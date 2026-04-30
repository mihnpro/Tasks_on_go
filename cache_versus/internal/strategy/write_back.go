package strategy

import (
	"context"
	"log"
	"sync"
	"time"

	"cache-comparison/internal/cache"
	"cache-comparison/internal/db"
	"cache-comparison/internal/metrics"
)

// WriteBack batches writes in the cache and flushes them to DB asynchronously.
//
// Read path:  cache → (on miss) DB → populate cache
// Write path: cache only — key is marked dirty and persisted to DB by the
//
//	background flusher on every flushInterval tick.
//
// Trade-off: highest write throughput, but data not yet flushed is at risk of
// loss if the process crashes before the next flush.
type WriteBack struct {
	cache         cache.Cache
	db            db.DB
	ttl           time.Duration
	flushInterval time.Duration
	metrics       *metrics.Metrics

	dirtyMu sync.Mutex
	dirty   map[string]string // key → latest value pending DB write

	stopCh chan struct{}
	wg     sync.WaitGroup

	// PendingWrites is exported for observability — shows writes queued in cache.
	PendingWrites int64
}

// NewWriteBack constructs a WriteBack strategy and starts the background flusher.
// Call Stop() when the benchmark is done to flush remaining dirty entries.
func NewWriteBack(c cache.Cache, d db.DB, ttl, flushInterval time.Duration, m *metrics.Metrics) *WriteBack {
	wb := &WriteBack{
		cache:         c,
		db:            d,
		ttl:           ttl,
		flushInterval: flushInterval,
		metrics:       m,
		dirty:         make(map[string]string),
		stopCh:        make(chan struct{}),
	}
	wb.wg.Add(1)
	go wb.flusher()
	return wb
}

func (w *WriteBack) Name() string { return "Write-Back" }

func (w *WriteBack) Read(ctx context.Context, key string) (string, error) {
	start := time.Now()

	val, err := w.cache.Get(ctx, key)
	if err == nil {
		w.metrics.RecordRead(time.Since(start), true)
		return val, nil
	}
	if err != cache.ErrCacheMiss {
		return "", err
	}

	// Check the dirty map — a key may have been written but not yet flushed,
	// and then evicted from Redis (e.g. under memory pressure).
	w.dirtyMu.Lock()
	if v, ok := w.dirty[key]; ok {
		w.dirtyMu.Unlock()
		_ = w.cache.Set(ctx, key, v, w.ttl)
		w.metrics.RecordRead(time.Since(start), false)
		return v, nil
	}
	w.dirtyMu.Unlock()

	val, err = w.db.Get(ctx, key)
	if err != nil {
		return "", err
	}
	_ = w.cache.Set(ctx, key, val, w.ttl)

	w.metrics.RecordRead(time.Since(start), false)
	return val, nil
}

func (w *WriteBack) Write(ctx context.Context, key, value string) error {
	start := time.Now()

	// Write only to the cache; mark the key dirty for the next flush.
	if err := w.cache.Set(ctx, key, value, w.ttl); err != nil {
		return err
	}
	w.dirtyMu.Lock()
	w.dirty[key] = value
	pending := int64(len(w.dirty))
	w.dirtyMu.Unlock()

	w.PendingWrites = pending
	w.metrics.RecordWrite(time.Since(start))
	return nil
}

// Stop signals the flusher to exit and performs a final flush to DB.
func (w *WriteBack) Stop(ctx context.Context) {
	close(w.stopCh)
	w.wg.Wait()
	w.flush(ctx) // drain any remaining dirty entries
}

// flusher runs in a dedicated goroutine and periodically persists dirty keys.
func (w *WriteBack) flusher() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.flush(context.Background())
		case <-w.stopCh:
			return
		}
	}
}

// flush atomically swaps the dirty map and writes all accumulated entries to DB.
func (w *WriteBack) flush(ctx context.Context) {
	w.dirtyMu.Lock()
	if len(w.dirty) == 0 {
		w.dirtyMu.Unlock()
		return
	}
	toFlush := w.dirty
	w.dirty = make(map[string]string)
	w.dirtyMu.Unlock()

	log.Printf("[Write-Back] flushing %d dirty keys to DB", len(toFlush))
	for key, value := range toFlush {
		if err := w.db.Set(ctx, key, value); err != nil {
			log.Printf("[Write-Back] flush error for key %q: %v", key, err)
		}
	}
}

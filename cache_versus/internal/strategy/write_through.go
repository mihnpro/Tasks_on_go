package strategy

import (
	"context"
	"time"

	"cache-comparison/internal/cache"
	"cache-comparison/internal/db"
	"cache-comparison/internal/metrics"
)

// WriteThrough keeps cache and DB always in sync.
//
// Read path:  cache → (on miss) DB → populate cache
// Write path: DB first, then cache — both are updated synchronously before
//
//	returning to the caller.
type WriteThrough struct {
	cache   cache.Cache
	db      db.DB
	ttl     time.Duration
	metrics *metrics.Metrics
}

// NewWriteThrough constructs a WriteThrough strategy.
func NewWriteThrough(c cache.Cache, d db.DB, ttl time.Duration, m *metrics.Metrics) *WriteThrough {
	return &WriteThrough{cache: c, db: d, ttl: ttl, metrics: m}
}

func (w *WriteThrough) Name() string { return "Write-Through" }

func (w *WriteThrough) Read(ctx context.Context, key string) (string, error) {
	start := time.Now()

	val, err := w.cache.Get(ctx, key)
	if err == nil {
		w.metrics.RecordRead(time.Since(start), true)
		return val, nil
	}
	if err != cache.ErrCacheMiss {
		return "", err
	}

	val, err = w.db.Get(ctx, key)
	if err != nil {
		return "", err
	}
	_ = w.cache.Set(ctx, key, val, w.ttl)

	w.metrics.RecordRead(time.Since(start), false)
	return val, nil
}

func (w *WriteThrough) Write(ctx context.Context, key, value string) error {
	start := time.Now()

	// Write to DB first to keep the DB as the authoritative source.
	if err := w.db.Set(ctx, key, value); err != nil {
		return err
	}
	// Mirror the write into the cache so subsequent reads are cache hits.
	_ = w.cache.Set(ctx, key, value, w.ttl)

	w.metrics.RecordWrite(time.Since(start))
	return nil
}

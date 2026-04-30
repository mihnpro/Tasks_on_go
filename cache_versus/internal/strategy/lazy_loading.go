package strategy

import (
	"context"
	"time"

	"cache-comparison/internal/cache"
	"cache-comparison/internal/db"
	"cache-comparison/internal/metrics"
)

// LazyLoading implements the Cache-Aside / Write-Around strategy.
//
// Read path:  cache → (on miss) DB → populate cache
// Write path: DB only; the stale cache entry is invalidated so the next read
//
//	will reload fresh data from DB.
type LazyLoading struct {
	cache   cache.Cache
	db      db.DB
	ttl     time.Duration
	metrics *metrics.Metrics
}

// NewLazyLoading constructs a LazyLoading strategy.
func NewLazyLoading(c cache.Cache, d db.DB, ttl time.Duration, m *metrics.Metrics) *LazyLoading {
	return &LazyLoading{cache: c, db: d, ttl: ttl, metrics: m}
}

func (l *LazyLoading) Name() string { return "Lazy Loading / Cache-Aside" }

func (l *LazyLoading) Read(ctx context.Context, key string) (string, error) {
	start := time.Now()

	val, err := l.cache.Get(ctx, key)
	if err == nil {
		// Cache hit — fast path.
		l.metrics.RecordRead(time.Since(start), true)
		return val, nil
	}
	if err != cache.ErrCacheMiss {
		return "", err
	}

	// Cache miss — load from DB and backfill the cache.
	val, err = l.db.Get(ctx, key)
	if err != nil {
		return "", err
	}
	_ = l.cache.Set(ctx, key, val, l.ttl)

	l.metrics.RecordRead(time.Since(start), false)
	return val, nil
}

func (l *LazyLoading) Write(ctx context.Context, key, value string) error {
	start := time.Now()

	if err := l.db.Set(ctx, key, value); err != nil {
		return err
	}
	// Invalidate the cache entry so the next read picks up the fresh DB value.
	_ = l.cache.Delete(ctx, key)

	l.metrics.RecordWrite(time.Since(start))
	return nil
}

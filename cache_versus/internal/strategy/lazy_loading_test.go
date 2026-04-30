package strategy_test

import (
	"context"
	"testing"
	"time"

	"cache-comparison/internal/cache"
	"cache-comparison/internal/db"
	"cache-comparison/internal/metrics"
	"cache-comparison/internal/strategy"
)

func newLazy(c cache.Cache, d db.DB) *strategy.LazyLoading {
	return strategy.NewLazyLoading(c, d, time.Minute, metrics.New())
}

func TestLazyLoading_Name(t *testing.T) {
	s := newLazy(cache.NewMockCache(), db.NewMockDB(nil))
	if s.Name() == "" {
		t.Error("Name must not be empty")
	}
}

// Read: key is in cache → returned immediately, DB is never called.
func TestLazyLoading_Read_CacheHit(t *testing.T) {
	c := cache.NewMockCache()
	d := db.NewMockDB(nil)
	ctx := context.Background()
	_ = c.Set(ctx, "k", "v", time.Minute)

	val, err := newLazy(c, d).Read(ctx, "k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "v" {
		t.Errorf("want %q, got %q", "v", val)
	}
	if d.GetCallCount() != 0 {
		t.Errorf("DB should not be called on cache hit, got %d calls", d.GetCallCount())
	}
}

// Read: key absent from cache → loaded from DB, then cached for subsequent reads.
func TestLazyLoading_Read_CacheMiss_BackfillsCache(t *testing.T) {
	c := cache.NewMockCache()
	d := db.NewMockDB(map[string]string{"k": "v"})
	ctx := context.Background()
	strat := newLazy(c, d)

	val, err := strat.Read(ctx, "k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "v" {
		t.Errorf("want %q, got %q", "v", val)
	}
	if d.GetCallCount() != 1 {
		t.Errorf("DB should be called once on miss, got %d", d.GetCallCount())
	}

	// Second read must come from cache (no extra DB call).
	_, _ = strat.Read(ctx, "k")
	if d.GetCallCount() != 1 {
		t.Errorf("second read should hit cache; DB call count should still be 1, got %d", d.GetCallCount())
	}
}

// Write: value goes to DB; stale cache entry is invalidated.
func TestLazyLoading_Write_InvalidatesCache(t *testing.T) {
	c := cache.NewMockCache()
	d := db.NewMockDB(nil)
	ctx := context.Background()
	_ = c.Set(ctx, "k", "old", time.Minute)

	if err := newLazy(c, d).Write(ctx, "k", "new"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Has("k") {
		t.Error("cache entry should be invalidated after write")
	}
	if v, ok := d.Value("k"); !ok || v != "new" {
		t.Errorf("DB should have 'new', got %q (ok=%v)", v, ok)
	}
}

// Write: cache miss before write — DB still gets the value, no panic.
func TestLazyLoading_Write_NoCacheEntry(t *testing.T) {
	c := cache.NewMockCache()
	d := db.NewMockDB(nil)
	ctx := context.Background()

	if err := newLazy(c, d).Write(ctx, "k", "v"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, ok := d.Value("k"); !ok || v != "v" {
		t.Errorf("DB should have 'v', got %q (ok=%v)", v, ok)
	}
}

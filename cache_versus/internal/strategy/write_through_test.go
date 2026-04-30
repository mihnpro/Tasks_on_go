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

func newWT(c cache.Cache, d db.DB) *strategy.WriteThrough {
	return strategy.NewWriteThrough(c, d, time.Minute, metrics.New())
}

func TestWriteThrough_Name(t *testing.T) {
	s := newWT(cache.NewMockCache(), db.NewMockDB(nil))
	if s.Name() == "" {
		t.Error("Name must not be empty")
	}
}

// Read: key in cache → served from cache, DB not touched.
func TestWriteThrough_Read_CacheHit(t *testing.T) {
	c := cache.NewMockCache()
	d := db.NewMockDB(nil)
	ctx := context.Background()
	_ = c.Set(ctx, "k", "v", time.Minute)

	val, err := newWT(c, d).Read(ctx, "k")
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

// Read: key absent from cache → loaded from DB and backfilled into cache.
func TestWriteThrough_Read_CacheMiss_BackfillsCache(t *testing.T) {
	c := cache.NewMockCache()
	d := db.NewMockDB(map[string]string{"k": "v"})
	ctx := context.Background()
	strat := newWT(c, d)

	val, err := strat.Read(ctx, "k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "v" {
		t.Errorf("want %q, got %q", "v", val)
	}
	if d.GetCallCount() != 1 {
		t.Errorf("expected 1 DB call on miss, got %d", d.GetCallCount())
	}

	// Subsequent read must come from cache.
	_, _ = strat.Read(ctx, "k")
	if d.GetCallCount() != 1 {
		t.Errorf("second read should be a cache hit; DB call count should remain 1, got %d", d.GetCallCount())
	}
}

// Write: value must land in BOTH cache and DB synchronously.
func TestWriteThrough_Write_UpdatesBothCacheAndDB(t *testing.T) {
	c := cache.NewMockCache()
	d := db.NewMockDB(nil)
	ctx := context.Background()

	if err := newWT(c, d).Write(ctx, "k", "v"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !c.Has("k") {
		t.Error("write-through must populate cache after write")
	}
	if v, ok := d.Value("k"); !ok || v != "v" {
		t.Errorf("DB should have 'v', got %q (ok=%v)", v, ok)
	}
}

// Write: overwrite an existing cache entry — both sides reflect the new value.
func TestWriteThrough_Write_OverwritesExisting(t *testing.T) {
	c := cache.NewMockCache()
	d := db.NewMockDB(map[string]string{"k": "old"})
	ctx := context.Background()
	_ = c.Set(ctx, "k", "old", time.Minute)

	if err := newWT(c, d).Write(ctx, "k", "new"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read from cache without going through strategy to verify cache value.
	val, err := c.Get(ctx, "k")
	if err != nil || val != "new" {
		t.Errorf("cache should hold 'new', got %q (err=%v)", val, err)
	}
	if v, _ := d.Value("k"); v != "new" {
		t.Errorf("DB should hold 'new', got %q", v)
	}
}

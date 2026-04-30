package metrics_test

import (
	"testing"
	"time"

	"cache-comparison/internal/metrics"
)

func TestNew_StartsWithZeros(t *testing.T) {
	m := metrics.New()
	m.Stop()

	if m.TotalOps() != 0 {
		t.Errorf("TotalOps: want 0, got %d", m.TotalOps())
	}
	if m.CacheHitRate() != 0 {
		t.Errorf("CacheHitRate: want 0, got %.2f", m.CacheHitRate())
	}
	if m.AvgLatencyMs() != 0 {
		t.Errorf("AvgLatencyMs: want 0, got %.2f", m.AvgLatencyMs())
	}
}

func TestRecordRead_HitRate(t *testing.T) {
	m := metrics.New()
	m.RecordRead(1*time.Millisecond, true)
	m.RecordRead(1*time.Millisecond, true)
	m.RecordRead(1*time.Millisecond, false)
	m.Stop()

	want := 100.0 * 2 / 3
	got := m.CacheHitRate()
	if abs(got-want) > 0.01 {
		t.Errorf("CacheHitRate: want %.2f, got %.2f", want, got)
	}
}

func TestRecordRead_TotalOps(t *testing.T) {
	m := metrics.New()
	for range 5 {
		m.RecordRead(time.Millisecond, true)
	}
	for range 3 {
		m.RecordWrite(time.Millisecond)
	}
	m.Stop()

	if m.TotalOps() != 8 {
		t.Errorf("TotalOps: want 8, got %d", m.TotalOps())
	}
}

func TestAvgLatencyMs(t *testing.T) {
	m := metrics.New()
	m.RecordRead(2*time.Millisecond, true)
	m.RecordRead(4*time.Millisecond, false)
	m.Stop()

	want := 3.0 // (2+4)/2
	got := m.AvgLatencyMs()
	if abs(got-want) > 0.1 {
		t.Errorf("AvgLatencyMs: want %.1f, got %.2f", want, got)
	}
}

func TestP99LatencyMs(t *testing.T) {
	m := metrics.New()
	// 99 fast ops + 1 slow → p99 ≈ 1 ms
	for range 99 {
		m.RecordRead(1*time.Millisecond, true)
	}
	m.RecordRead(100*time.Millisecond, false)
	m.Stop()

	p99 := m.P99LatencyMs()
	if p99 < 1 || p99 > 100 {
		t.Errorf("P99LatencyMs out of expected range: %.2f", p99)
	}
}

func TestThroughput_Positive(t *testing.T) {
	m := metrics.New()
	time.Sleep(50 * time.Millisecond)
	m.RecordRead(time.Millisecond, true)
	m.RecordRead(time.Millisecond, true)
	m.Stop()

	tp := m.Throughput()
	if tp <= 0 {
		t.Errorf("Throughput should be > 0, got %.2f", tp)
	}
}

func TestCacheHitRate_AllMisses(t *testing.T) {
	m := metrics.New()
	for range 10 {
		m.RecordRead(time.Millisecond, false)
	}
	m.Stop()

	if m.CacheHitRate() != 0 {
		t.Errorf("CacheHitRate: want 0, got %.2f", m.CacheHitRate())
	}
}

func TestCacheHitRate_AllHits(t *testing.T) {
	m := metrics.New()
	for range 10 {
		m.RecordRead(time.Millisecond, true)
	}
	m.Stop()

	if m.CacheHitRate() != 100 {
		t.Errorf("CacheHitRate: want 100, got %.2f", m.CacheHitRate())
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// Package metrics provides a thread-safe collector for benchmark measurements.
package metrics

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics collects throughput, latency, and cache-hit data for one benchmark run.
type Metrics struct {
	mu          sync.Mutex
	latencies   []time.Duration

	totalReads  int64 // atomic
	totalWrites int64 // atomic
	cacheHits   int64 // atomic — reads served from cache
	cacheMisses int64 // atomic — reads that fell through to DB

	startTime time.Time
	endTime   time.Time
}

// New returns a Metrics instance with the clock started.
func New() *Metrics {
	return &Metrics{
		latencies: make([]time.Duration, 0, 50_000),
		startTime: time.Now(),
	}
}

// RecordRead records a completed read operation.
// hit must be true when the value was served from the cache.
func (m *Metrics) RecordRead(latency time.Duration, hit bool) {
	atomic.AddInt64(&m.totalReads, 1)
	if hit {
		atomic.AddInt64(&m.cacheHits, 1)
	} else {
		atomic.AddInt64(&m.cacheMisses, 1)
	}
	m.mu.Lock()
	m.latencies = append(m.latencies, latency)
	m.mu.Unlock()
}

// RecordWrite records a completed write operation.
func (m *Metrics) RecordWrite(latency time.Duration) {
	atomic.AddInt64(&m.totalWrites, 1)
	m.mu.Lock()
	m.latencies = append(m.latencies, latency)
	m.mu.Unlock()
}

// Stop marks the end of the measurement window.
func (m *Metrics) Stop() {
	m.endTime = time.Now()
}

// Throughput returns completed operations per second.
func (m *Metrics) Throughput() float64 {
	elapsed := m.endTime.Sub(m.startTime).Seconds()
	if elapsed == 0 {
		return 0
	}
	total := atomic.LoadInt64(&m.totalReads) + atomic.LoadInt64(&m.totalWrites)
	return float64(total) / elapsed
}

// AvgLatencyMs returns the mean latency across all operations in milliseconds.
func (m *Metrics) AvgLatencyMs() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.latencies) == 0 {
		return 0
	}
	var sum time.Duration
	for _, l := range m.latencies {
		sum += l
	}
	return float64(sum) / float64(len(m.latencies)) / float64(time.Millisecond)
}

// P99LatencyMs returns the 99th-percentile latency in milliseconds.
func (m *Metrics) P99LatencyMs() float64 {
	m.mu.Lock()
	cp := make([]time.Duration, len(m.latencies))
	copy(cp, m.latencies)
	m.mu.Unlock()

	if len(cp) == 0 {
		return 0
	}
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	idx := int(float64(len(cp)) * 0.99)
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	return float64(cp[idx]) / float64(time.Millisecond)
}

// CacheHitRate returns the percentage of reads served from the cache (0–100).
func (m *Metrics) CacheHitRate() float64 {
	hits := atomic.LoadInt64(&m.cacheHits)
	misses := atomic.LoadInt64(&m.cacheMisses)
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total) * 100
}

// TotalOps returns the total number of operations recorded.
func (m *Metrics) TotalOps() int64 {
	return atomic.LoadInt64(&m.totalReads) + atomic.LoadInt64(&m.totalWrites)
}

// Report is the final, serialisable summary of a single benchmark run.
type Report struct {
	Strategy     string
	Scenario     string
	Throughput   float64 // req/s
	AvgLatencyMs float64
	P99LatencyMs float64
	DBCalls      int64
	CacheHitRate float64 // 0–100 %
	TotalOps     int64
}

// Print writes a human-readable summary of the report to stdout.
func (r *Report) Print() {
	fmt.Printf(
		"  %-24s | %-12s | %8.0f req/s | avg %6.2f ms | p99 %6.2f ms | DB calls %6d | hit rate %5.1f%% | ops %d\n",
		r.Strategy, r.Scenario,
		r.Throughput, r.AvgLatencyMs, r.P99LatencyMs,
		r.DBCalls, r.CacheHitRate, r.TotalOps,
	)
}

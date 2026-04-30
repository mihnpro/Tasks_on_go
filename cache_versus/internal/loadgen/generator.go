// Package loadgen runs a configurable read/write workload against a CacheStrategy.
package loadgen

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"sync"
	"time"

	"cache-comparison/internal/strategy"
)

// Scenario describes a single benchmark run.
type Scenario struct {
	// Name is a short label shown in the results table.
	Name string
	// ReadRatio is the fraction of operations that are reads (0.0–1.0).
	// The remaining fraction becomes writes.
	ReadRatio float64
}

// Predefined scenarios used across all strategy benchmarks.
var Scenarios = []Scenario{
	{Name: "read-heavy  (80/20)", ReadRatio: 0.80},
	{Name: "balanced    (50/50)", ReadRatio: 0.50},
	{Name: "write-heavy (20/80)", ReadRatio: 0.20},
}

// Config holds the parameters for a load-generation run.
type Config struct {
	// Duration is how long to keep generating load.
	Duration time.Duration
	// Concurrency is the number of parallel goroutines.
	Concurrency int
	// Keys is the pool of keys to read from / write to.
	Keys []string
}

// Run drives load against strat for the given scenario and returns when
// Duration has elapsed.  The caller is responsible for resetting metrics and
// the DB call counter before calling Run.
func Run(ctx context.Context, strat strategy.CacheStrategy, cfg Config, sc Scenario) {
	deadline := time.Now().Add(cfg.Duration)

	var wg sync.WaitGroup
	wg.Add(cfg.Concurrency)

	for range cfg.Concurrency {
		go func() {
			defer wg.Done()
			rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))

			for time.Now().Before(deadline) {
				key := cfg.Keys[rng.IntN(len(cfg.Keys))]

				if rng.Float64() < sc.ReadRatio {
					if _, err := strat.Read(ctx, key); err != nil {
						log.Printf("read error: %v", err)
					}
				} else {
					value := fmt.Sprintf("val-%d", rng.Int())
					if err := strat.Write(ctx, key, value); err != nil {
						log.Printf("write error: %v", err)
					}
				}
			}
		}()
	}

	wg.Wait()
}

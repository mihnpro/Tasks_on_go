// cache-comparison benchmarks three cache strategies — Lazy Loading,
// Write-Through, and Write-Back — across three load scenarios, prints a
// side-by-side comparison table, and saves a Markdown report to disk.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"

	"cache-comparison/config"
	"cache-comparison/internal/cache"
	"cache-comparison/internal/db"
	"cache-comparison/internal/loadgen"
	"cache-comparison/internal/metrics"
	"cache-comparison/internal/reporter"
	"cache-comparison/internal/strategy"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	// ── Infrastructure ──────────────────────────────────────────────────────
	log.Println("connecting to Redis…")
	redisCache := cache.NewRedisCache(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err := redisCache.Ping(ctx); err != nil {
		log.Fatalf("Redis ping failed: %v", err)
	}

	log.Println("connecting to PostgreSQL…")
	postgresDB, err := db.NewPostgresDB(ctx, cfg.Postgres.DSN)
	if err != nil {
		log.Fatalf("PostgreSQL connect failed: %v", err)
	}
	if err = postgresDB.Init(ctx); err != nil {
		log.Fatalf("PostgreSQL init failed: %v", err)
	}

	// ── Seed ────────────────────────────────────────────────────────────────
	log.Printf("seeding %d keys into PostgreSQL…", cfg.Bench.TotalKeys)
	keys, seedData := generateSeedData(cfg.Bench.TotalKeys)
	if err = postgresDB.Seed(ctx, seedData); err != nil {
		log.Fatalf("seed failed: %v", err)
	}

	genCfg := loadgen.Config{
		Duration:    cfg.Bench.Duration,
		Concurrency: cfg.Bench.Concurrency,
		Keys:        keys,
	}

	// ── Benchmark ───────────────────────────────────────────────────────────
	var reports []metrics.Report

	for _, sc := range loadgen.Scenarios {
		// --- Lazy Loading ---------------------------------------------------
		reports = append(reports, runLazy(ctx, redisCache, postgresDB, cfg, genCfg, sc))

		// --- Write-Through --------------------------------------------------
		reports = append(reports, runWriteThrough(ctx, redisCache, postgresDB, cfg, genCfg, sc))

		// --- Write-Back -----------------------------------------------------
		reports = append(reports, runWriteBack(ctx, redisCache, postgresDB, cfg, genCfg, sc))
	}

	// ── Results ─────────────────────────────────────────────────────────────
	printTable(reports)
	printConclusions(reports)

	// ── Auto-generate Markdown report ────────────────────────────────────────
	path, err := reporter.Generate(reports, reporter.ReportConfig{
		Duration:      cfg.Bench.Duration,
		Concurrency:   cfg.Bench.Concurrency,
		TotalKeys:     cfg.Bench.TotalKeys,
		CacheTTL:      cfg.Bench.CacheTTL,
		FlushInterval: cfg.Bench.WriteBackFlushInterval,
	})
	if err != nil {
		log.Printf("warning: could not save report: %v", err)
	} else {
		log.Printf("report saved → %s", path)
	}
}

// ── Strategy runners ────────────────────────────────────────────────────────

func runLazy(
	ctx context.Context,
	c *cache.RedisCache, d *db.PostgresDB,
	cfg *config.Config, genCfg loadgen.Config, sc loadgen.Scenario,
) metrics.Report {
	log.Printf("[Lazy Loading] scenario: %s", sc.Name)
	reset(ctx, c, d)

	m := metrics.New()
	strat := strategy.NewLazyLoading(c, d, cfg.Bench.CacheTTL, m)
	loadgen.Run(ctx, strat, genCfg, sc)
	m.Stop()

	return metrics.Report{
		Strategy:     strat.Name(),
		Scenario:     sc.Name,
		Throughput:   m.Throughput(),
		AvgLatencyMs: m.AvgLatencyMs(),
		P99LatencyMs: m.P99LatencyMs(),
		DBCalls:      d.GetCallCount(),
		CacheHitRate: m.CacheHitRate(),
		TotalOps:     m.TotalOps(),
	}
}

func runWriteThrough(
	ctx context.Context,
	c *cache.RedisCache, d *db.PostgresDB,
	cfg *config.Config, genCfg loadgen.Config, sc loadgen.Scenario,
) metrics.Report {
	log.Printf("[Write-Through] scenario: %s", sc.Name)
	reset(ctx, c, d)

	m := metrics.New()
	strat := strategy.NewWriteThrough(c, d, cfg.Bench.CacheTTL, m)
	loadgen.Run(ctx, strat, genCfg, sc)
	m.Stop()

	return metrics.Report{
		Strategy:     strat.Name(),
		Scenario:     sc.Name,
		Throughput:   m.Throughput(),
		AvgLatencyMs: m.AvgLatencyMs(),
		P99LatencyMs: m.P99LatencyMs(),
		DBCalls:      d.GetCallCount(),
		CacheHitRate: m.CacheHitRate(),
		TotalOps:     m.TotalOps(),
	}
}

func runWriteBack(
	ctx context.Context,
	c *cache.RedisCache, d *db.PostgresDB,
	cfg *config.Config, genCfg loadgen.Config, sc loadgen.Scenario,
) metrics.Report {
	log.Printf("[Write-Back] scenario: %s", sc.Name)
	reset(ctx, c, d)

	m := metrics.New()
	strat := strategy.NewWriteBack(c, d, cfg.Bench.CacheTTL, cfg.Bench.WriteBackFlushInterval, m)

	loadgen.Run(ctx, strat, genCfg, sc)

	// Stop the flusher and drain remaining dirty keys to DB before measuring.
	strat.Stop(ctx)
	m.Stop()

	log.Printf("[Write-Back] pending writes at stop: %d", strat.PendingWrites)

	return metrics.Report{
		Strategy:     strat.Name(),
		Scenario:     sc.Name,
		Throughput:   m.Throughput(),
		AvgLatencyMs: m.AvgLatencyMs(),
		P99LatencyMs: m.P99LatencyMs(),
		DBCalls:      d.GetCallCount(),
		CacheHitRate: m.CacheHitRate(),
		TotalOps:     m.TotalOps(),
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// reset flushes the cache and zeroes the DB call counter between runs.
func reset(ctx context.Context, c *cache.RedisCache, d *db.PostgresDB) {
	if err := c.FlushAll(ctx); err != nil {
		log.Printf("cache flush error: %v", err)
	}
	d.ResetCallCount()
	// Brief pause so Redis can process the flush before the next run starts.
	time.Sleep(200 * time.Millisecond)
}

// generateSeedData creates TotalKeys items and returns the key slice plus the
// map used for seeding PostgreSQL.
func generateSeedData(n int) ([]string, map[string]string) {
	keys := make([]string, n)
	data := make(map[string]string, n)
	for i := range n {
		k := fmt.Sprintf("item:%04d", i)
		keys[i] = k
		data[k] = fmt.Sprintf("value-%d", i)
	}
	return keys, data
}

// ── Output ───────────────────────────────────────────────────────────────────

func printTable(reports []metrics.Report) {
	fmt.Println()
	fmt.Println(strings.Repeat("─", 110))
	fmt.Println("  BENCHMARK RESULTS")
	fmt.Println(strings.Repeat("─", 110))

	tbl := tablewriter.NewWriter(os.Stdout)
	tbl.SetHeader([]string{
		"Strategy", "Scenario",
		"Throughput\n(req/s)", "Avg Lat\n(ms)", "P99 Lat\n(ms)",
		"DB Calls", "Cache Hit\nRate (%)", "Total Ops",
	})
	tbl.SetBorder(true)
	tbl.SetRowLine(true)
	tbl.SetAlignment(tablewriter.ALIGN_RIGHT)
	tbl.SetHeaderAlignment(tablewriter.ALIGN_CENTER)
	tbl.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT,
	})

	for _, r := range reports {
		tbl.Append([]string{
			r.Strategy,
			r.Scenario,
			fmt.Sprintf("%.0f", r.Throughput),
			fmt.Sprintf("%.2f", r.AvgLatencyMs),
			fmt.Sprintf("%.2f", r.P99LatencyMs),
			fmt.Sprintf("%d", r.DBCalls),
			fmt.Sprintf("%.1f%%", r.CacheHitRate),
			fmt.Sprintf("%d", r.TotalOps),
		})
	}
	tbl.Render()
}

func printConclusions(reports []metrics.Report) {
	fmt.Println()
	fmt.Println(strings.Repeat("─", 110))
	fmt.Println("  CONCLUSIONS")
	fmt.Println(strings.Repeat("─", 110))

	// Group by scenario for easy comparison.
	type key struct{ scenario string }
	grouped := map[string][]metrics.Report{}
	for _, r := range reports {
		grouped[r.Scenario] = append(grouped[r.Scenario], r)
	}

	for _, sc := range loadgen.Scenarios {
		reps := grouped[sc.Name]
		if len(reps) == 0 {
			continue
		}
		fmt.Printf("\n  Scenario: %s\n", sc.Name)

		best := func(cmp func(a, b metrics.Report) bool) string {
			winner := reps[0]
			for _, r := range reps[1:] {
				if cmp(r, winner) {
					winner = r
				}
			}
			return winner.Strategy
		}

		fmt.Printf("    Best throughput : %s\n",
			best(func(a, b metrics.Report) bool { return a.Throughput > b.Throughput }))
		fmt.Printf("    Lowest avg lat  : %s\n",
			best(func(a, b metrics.Report) bool { return a.AvgLatencyMs < b.AvgLatencyMs }))
		fmt.Printf("    Fewest DB calls : %s\n",
			best(func(a, b metrics.Report) bool { return a.DBCalls < b.DBCalls }))
		fmt.Printf("    Best cache hit  : %s\n",
			best(func(a, b metrics.Report) bool { return a.CacheHitRate > b.CacheHitRate }))
	}

	fmt.Println()
	fmt.Println("  General guidance:")
	fmt.Println("  • Lazy Loading   — best for read-heavy, tolerates stale data, simple to implement.")
	fmt.Println("  • Write-Through  — strong consistency; extra DB write per update; ideal when reads dominate after writes.")
	fmt.Println("  • Write-Back     — highest write throughput; risk of data loss on crash; ideal for write-heavy bursts.")
	fmt.Println()
}

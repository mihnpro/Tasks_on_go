// Package config loads runtime configuration from environment variables with
// sensible defaults for local Docker Compose development.
package config

import (
	"os"
	"time"
)

// Config holds all tunable parameters for the benchmark.
type Config struct {
	Redis    RedisConfig
	Postgres PostgresConfig
	Bench    BenchConfig
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type PostgresConfig struct {
	DSN string
}

// BenchConfig controls the shape of each benchmark run.
type BenchConfig struct {
	// Duration is how long each scenario runs.
	Duration time.Duration
	// Concurrency is the number of parallel workers.
	Concurrency int
	// TotalKeys is the size of the key-space used during the test.
	TotalKeys int
	// CacheTTL is the TTL applied to every cache entry.
	CacheTTL time.Duration
	// WriteBackFlushInterval is the period between Write-Back flushes to DB.
	WriteBackFlushInterval time.Duration
}

// Load returns a Config populated from environment variables.
func Load() *Config {
	return &Config{
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       0,
		},
		Postgres: PostgresConfig{
			DSN: getEnv(
				"POSTGRES_DSN",
				"postgres://cache_user:cache_pass@localhost:5432/cache_db?sslmode=disable",
			),
		},
		Bench: BenchConfig{
			Duration:               30 * time.Second,
			Concurrency:            50,
			TotalKeys:              1000,
			CacheTTL:               5 * time.Minute,
			WriteBackFlushInterval: 2 * time.Second,
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

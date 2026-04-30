package db

import (
	"context"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresDB is a DB backed by PostgreSQL.
// It also exposes a call counter so the benchmark can measure how often the
// database is actually hit.
type PostgresDB struct {
	pool      *pgxpool.Pool
	callCount int64 // accessed atomically
}

// NewPostgresDB connects to PostgreSQL and returns a ready-to-use PostgresDB.
func NewPostgresDB(ctx context.Context, dsn string) (*PostgresDB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		return nil, err
	}
	return &PostgresDB{pool: pool}, nil
}

// Init creates the items table if it does not already exist.
func (p *PostgresDB) Init(ctx context.Context) error {
	_, err := p.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS items (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	return err
}

// Seed populates the database with the provided key-value pairs and resets the
// call counter so seeding is not counted in benchmark metrics.
func (p *PostgresDB) Seed(ctx context.Context, items map[string]string) error {
	for k, v := range items {
		if err := p.set(ctx, k, v); err != nil {
			return err
		}
	}
	p.ResetCallCount()
	return nil
}

func (p *PostgresDB) Get(ctx context.Context, key string) (string, error) {
	atomic.AddInt64(&p.callCount, 1)
	var value string
	err := p.pool.QueryRow(ctx, "SELECT value FROM items WHERE key = $1", key).Scan(&value)
	if err == pgx.ErrNoRows {
		return "", ErrNotFound
	}
	return value, err
}

func (p *PostgresDB) Set(ctx context.Context, key, value string) error {
	atomic.AddInt64(&p.callCount, 1)
	return p.set(ctx, key, value)
}

// set is the internal upsert that does not increment the call counter.
func (p *PostgresDB) set(ctx context.Context, key, value string) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO items (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value
	`, key, value)
	return err
}

// GetCallCount returns the current DB access count.
func (p *PostgresDB) GetCallCount() int64 {
	return atomic.LoadInt64(&p.callCount)
}

// ResetCallCount zeroes the counter — called between benchmark runs.
func (p *PostgresDB) ResetCallCount() {
	atomic.StoreInt64(&p.callCount, 0)
}

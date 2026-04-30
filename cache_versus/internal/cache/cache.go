// Package cache defines the generic cache interface used by all strategies.
package cache

import (
	"context"
	"errors"
	"time"
)

// ErrCacheMiss is returned by Get when the key does not exist in the cache.
var ErrCacheMiss = errors.New("cache miss")

// Cache is the minimal set of operations every cache backend must implement.
type Cache interface {
	// Get returns the value for key, or ErrCacheMiss if the key is absent.
	Get(ctx context.Context, key string) (string, error)
	// Set stores value under key with an expiry of ttl (0 = no expiry).
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	// Delete removes key from the cache; a missing key is not an error.
	Delete(ctx context.Context, key string) error
	// FlushAll removes every key — used to reset state between test runs.
	FlushAll(ctx context.Context) error
}

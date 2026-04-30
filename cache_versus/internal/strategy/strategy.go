// Package strategy provides implementations of three cache strategies:
// Lazy Loading (Cache-Aside), Write-Through, and Write-Back.
package strategy

import "context"

// CacheStrategy is the common interface for all cache strategies.
// Both Read and Write must record their own metrics internally.
type CacheStrategy interface {
	// Read retrieves the value for key, consulting the cache per the strategy.
	Read(ctx context.Context, key string) (string, error)
	// Write stores key→value, writing to cache and/or DB per the strategy.
	Write(ctx context.Context, key, value string) error
	// Name returns a human-readable strategy identifier.
	Name() string
}

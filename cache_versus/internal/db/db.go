// Package db defines the generic database interface used by all strategies.
package db

import (
	"context"
	"errors"
)

// ErrNotFound is returned by Get when the key does not exist in the database.
var ErrNotFound = errors.New("not found")

// DB is the minimal set of operations every database backend must implement.
type DB interface {
	// Get returns the value for key, or ErrNotFound if the key is absent.
	Get(ctx context.Context, key string) (string, error)
	// Set upserts key→value.
	Set(ctx context.Context, key, value string) error
}

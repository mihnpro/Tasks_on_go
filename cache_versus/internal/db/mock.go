package db

import (
	"context"
	"sync"
	"sync/atomic"
)

// MockDB is an in-memory DB for use in tests.
type MockDB struct {
	mu        sync.RWMutex
	store     map[string]string
	callCount int64
}

// NewMockDB creates a MockDB pre-populated with initial data.
func NewMockDB(initial map[string]string) *MockDB {
	store := make(map[string]string, len(initial))
	for k, v := range initial {
		store[k] = v
	}
	return &MockDB{store: store}
}

func (m *MockDB) Get(_ context.Context, key string) (string, error) {
	atomic.AddInt64(&m.callCount, 1)
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.store[key]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (m *MockDB) Set(_ context.Context, key, value string) error {
	atomic.AddInt64(&m.callCount, 1)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[key] = value
	return nil
}

// GetCallCount returns the total number of Get+Set calls.
func (m *MockDB) GetCallCount() int64 {
	return atomic.LoadInt64(&m.callCount)
}

// Value returns the current stored value for key and whether it exists.
func (m *MockDB) Value(key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.store[key]
	return v, ok
}

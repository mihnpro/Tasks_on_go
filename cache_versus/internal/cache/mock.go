package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// MockCache is an in-memory Cache for use in tests.
type MockCache struct {
	mu       sync.RWMutex
	store    map[string]string
	GetCalls int64
	SetCalls int64
	DelCalls int64
}

func NewMockCache() *MockCache {
	return &MockCache{store: make(map[string]string)}
}

func (m *MockCache) Get(_ context.Context, key string) (string, error) {
	atomic.AddInt64(&m.GetCalls, 1)
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.store[key]
	if !ok {
		return "", ErrCacheMiss
	}
	return v, nil
}

func (m *MockCache) Set(_ context.Context, key, value string, _ time.Duration) error {
	atomic.AddInt64(&m.SetCalls, 1)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[key] = value
	return nil
}

func (m *MockCache) Delete(_ context.Context, key string) error {
	atomic.AddInt64(&m.DelCalls, 1)
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.store, key)
	return nil
}

func (m *MockCache) FlushAll(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store = make(map[string]string)
	return nil
}

// Has reports whether key is currently in the mock cache.
func (m *MockCache) Has(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.store[key]
	return ok
}

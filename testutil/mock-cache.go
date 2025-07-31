package testutil

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/metrics"
)

type MockCache struct {
	data                      map[string]any
	metadata                  map[string]*cache.CacheEntryMetadata
	OnGetCallback             func(ctx context.Context, key string) (any, bool, error)
	OnSetCallback             func(ctx context.Context, key string, value any, ttl time.Duration) error
	OnDeleteCallback          func(ctx context.Context, key string) error
	OnClearCallback           func(ctx context.Context) error
	OnHasCallback             func(ctx context.Context, key string) bool
	OnGetKeysCallback         func(ctx context.Context) []string
	OnCloseCallback           func() error
	OnGetManyCallback         func(ctx context.Context, keys []string) (map[string]any, error)
	OnSetManyCallback         func(ctx context.Context, items map[string]any, ttl time.Duration) error
	OnDeleteManyCallback      func(ctx context.Context, keys []string) error
	OnGetMetadataCallback     func(ctx context.Context, key string) (*cache.CacheEntryMetadata, error)
	OnGetManyMetadataCallback func(ctx context.Context, keys []string) (map[string]*cache.CacheEntryMetadata, error)
	OnGetMetricsCallback      func() *metrics.CacheMetricsSnapshot
	mu                        sync.RWMutex
}

func NewMockCache() *MockCache {
	return &MockCache{
		data:     make(map[string]any),
		metadata: make(map[string]*cache.CacheEntryMetadata),
	}
}

// Get implements cache.Cache.
func (m *MockCache) Get(ctx context.Context, key string) (any, bool, error) {
	if m.OnGetCallback != nil {
		return m.OnGetCallback(ctx, key)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	value, exists := m.data[key]
	if exists && m.metadata[key] != nil {
		m.metadata[key].LastAccessed = time.Now()
		m.metadata[key].AccessCount++
	}
	return value, exists, nil
}

// Set implements cache.Cache.
func (m *MockCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if m.OnSetCallback != nil {
		return m.OnSetCallback(ctx, key, value, ttl)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[key] = value
	m.metadata[key] = &cache.CacheEntryMetadata{
		Key:          key,
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		AccessCount:  0,
		TTL:          ttl,
		Size:         int64(len(key)) + 8, // approximate size
	}
	return nil
}

// Delete implements cache.Cache.
func (m *MockCache) Delete(ctx context.Context, key string) error {
	if m.OnDeleteCallback != nil {
		return m.OnDeleteCallback(ctx, key)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, key)
	delete(m.metadata, key)
	return nil
}

// Clear implements cache.Cache.
func (m *MockCache) Clear(ctx context.Context) error {
	if m.OnClearCallback != nil {
		return m.OnClearCallback(ctx)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = make(map[string]any)
	m.metadata = make(map[string]*cache.CacheEntryMetadata)
	return nil
}

// Has implements cache.Cache.
func (m *MockCache) Has(ctx context.Context, key string) bool {
	if m.OnHasCallback != nil {
		return m.OnHasCallback(ctx, key)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.data[key]
	return exists
}

// GetKeys implements cache.Cache.
func (m *MockCache) GetKeys(ctx context.Context) []string {
	if m.OnGetKeysCallback != nil {
		return m.OnGetKeysCallback(ctx)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]string, 0, len(m.data))
	for key := range m.data {
		keys = append(keys, key)
	}
	return keys
}

// Close implements cache.Cache.
func (m *MockCache) Close() error {
	if m.OnCloseCallback != nil {
		return m.OnCloseCallback()
	}
	return nil
}

// GetMany implements cache.Cache.
func (m *MockCache) GetMany(ctx context.Context, keys []string) (map[string]any, error) {
	if m.OnGetManyCallback != nil {
		return m.OnGetManyCallback(ctx, keys)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]any)
	for _, key := range keys {
		if value, exists := m.data[key]; exists {
			result[key] = value
			if m.metadata[key] != nil {
				m.metadata[key].LastAccessed = time.Now()
				m.metadata[key].AccessCount++
			}
		}
	}
	return result, nil
}

// SetMany implements cache.Cache.
func (m *MockCache) SetMany(ctx context.Context, items map[string]any, ttl time.Duration) error {
	if m.OnSetManyCallback != nil {
		return m.OnSetManyCallback(ctx, items, ttl)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for key, value := range items {
		m.data[key] = value
		m.metadata[key] = &cache.CacheEntryMetadata{
			Key:          key,
			CreatedAt:    now,
			LastAccessed: now,
			AccessCount:  0,
			TTL:          ttl,
			Size:         int64(len(key)) + 8,
		}
	}
	return nil
}

// DeleteMany implements cache.Cache.
func (m *MockCache) DeleteMany(ctx context.Context, keys []string) error {
	if m.OnDeleteManyCallback != nil {
		return m.OnDeleteManyCallback(ctx, keys)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, key := range keys {
		delete(m.data, key)
		delete(m.metadata, key)
	}
	return nil
}

// GetMetadata implements cache.Cache.
func (m *MockCache) GetMetadata(ctx context.Context, key string) (*cache.CacheEntryMetadata, error) {
	if m.OnGetMetadataCallback != nil {
		return m.OnGetMetadataCallback(ctx, key)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	metadata, exists := m.metadata[key]
	if !exists {
		return nil, nil
	}
	return metadata, nil
}

// GetManyMetadata implements cache.Cache.
func (m *MockCache) GetManyMetadata(ctx context.Context, keys []string) (map[string]*cache.CacheEntryMetadata, error) {
	if m.OnGetManyMetadataCallback != nil {
		return m.OnGetManyMetadataCallback(ctx, keys)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*cache.CacheEntryMetadata)
	for _, key := range keys {
		if metadata, exists := m.metadata[key]; exists {
			result[key] = metadata
		}
	}
	return result, nil
}

// GetMetrics implements cache.Cache.
func (m *MockCache) GetMetrics() *metrics.CacheMetricsSnapshot {
	if m.OnGetMetricsCallback != nil {
		return m.OnGetMetricsCallback()
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalAccessCount int64
	for _, metadata := range m.metadata {
		totalAccessCount += metadata.AccessCount
	}

	return &metrics.CacheMetricsSnapshot{
		Hits:          totalAccessCount,
		Misses:        0,
		HitRatio:      1.0,
		GetLatency:    0,
		SetLatency:    0,
		DeleteLatency: 0,
		CacheSize:     int64(len(m.data)),
		EntryCount:    int64(len(m.data)),
	}
}

// Increment atomically increments a numeric value in the mock cache
func (m *MockCache) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for context cancellation
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}

	value, exists := m.data[key]
	var currentValue int64 = 0

	if exists {
		// Convert existing value to int64
		switch v := value.(type) {
		case int64:
			currentValue = v
		case int:
			currentValue = int64(v)
		case int32:
			currentValue = int64(v)
		case float64:
			currentValue = int64(v)
		default:
			return 0, fmt.Errorf("mock cache: cannot increment non-numeric value")
		}
	}

	// Calculate new value
	newValue := currentValue + delta

	// Store new value
	m.data[key] = newValue

	// Update metadata
	now := time.Now()
	if m.metadata[key] == nil {
		m.metadata[key] = &cache.CacheEntryMetadata{
			Key:          key,
			CreatedAt:    now,
			LastAccessed: now,
			AccessCount:  0,
			TTL:          ttl,
			Size:         8, // int64 is 8 bytes
			Tags:         []string{},
		}
	} else {
		m.metadata[key].LastAccessed = now
		m.metadata[key].AccessCount++
		m.metadata[key].TTL = ttl
	}

	return newValue, nil
}

// Decrement atomically decrements a numeric value in the mock cache
func (m *MockCache) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return m.Increment(ctx, key, -delta, ttl)
}

// SetIfNotExists sets a value only if the key doesn't exist in the mock cache
func (m *MockCache) SetIfNotExists(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for context cancellation
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	// Check if key exists (simple existence check for mock)
	_, exists := m.data[key]
	if exists {
		// Key exists
		return false, nil
	}

	// Key doesn't exist, set it
	m.data[key] = value

	// Update metadata
	now := time.Now()
	m.metadata[key] = &cache.CacheEntryMetadata{
		Key:          key,
		CreatedAt:    now,
		LastAccessed: now,
		AccessCount:  0,
		TTL:          ttl,
		Size:         int64(len(fmt.Sprintf("%v", value))), // Rough size estimate
		Tags:         []string{},
	}

	return true, nil
}

// SetIfExists sets a value only if the key already exists in the mock cache
func (m *MockCache) SetIfExists(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for context cancellation
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	// Check if key exists
	_, exists := m.data[key]
	if !exists {
		// Key doesn't exist
		return false, nil
	}

	// Key exists, update it
	m.data[key] = value

	// Update metadata
	now := time.Now()
	if m.metadata[key] != nil {
		m.metadata[key].LastAccessed = now
		m.metadata[key].AccessCount++
		m.metadata[key].TTL = ttl
		m.metadata[key].Size = int64(len(fmt.Sprintf("%v", value)))
	} else {
		m.metadata[key] = &cache.CacheEntryMetadata{
			Key:          key,
			CreatedAt:    now,
			LastAccessed: now,
			AccessCount:  0,
			TTL:          ttl,
			Size:         int64(len(fmt.Sprintf("%v", value))),
			Tags:         []string{},
		}
	}

	return true, nil
}

// AddIndex adds a secondary index - STUB IMPLEMENTATION for testing
func (m *MockCache) AddIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error {
	// TODO: Implement mock secondary indexing for testing
	return fmt.Errorf("AddIndex not yet implemented in mock cache")
}

// RemoveIndex removes a secondary index - STUB IMPLEMENTATION for testing
func (m *MockCache) RemoveIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error {
	// TODO: Implement mock secondary index removal for testing
	return fmt.Errorf("RemoveIndex not yet implemented in mock cache")
}

// GetByIndex retrieves keys by index - STUB IMPLEMENTATION for testing
func (m *MockCache) GetByIndex(ctx context.Context, indexName string, indexKey string) ([]string, error) {
	// TODO: Implement mock index querying for testing
	return []string{}, fmt.Errorf("GetByIndex not yet implemented in mock cache")
}

// DeleteByIndex deletes keys by index - STUB IMPLEMENTATION for testing
func (m *MockCache) DeleteByIndex(ctx context.Context, indexName string, indexKey string) error {
	// TODO: Implement mock bulk deletion by index for testing
	return fmt.Errorf("DeleteByIndex not yet implemented in mock cache")
}

// GetKeysByPattern returns keys matching pattern - STUB IMPLEMENTATION for testing
func (m *MockCache) GetKeysByPattern(ctx context.Context, pattern string) ([]string, error) {
	// TODO: Implement mock pattern matching for testing
	return []string{}, fmt.Errorf("GetKeysByPattern not yet implemented in mock cache")
}

// DeleteByPattern deletes keys matching pattern - STUB IMPLEMENTATION for testing
func (m *MockCache) DeleteByPattern(ctx context.Context, pattern string) (int, error) {
	// TODO: Implement mock pattern-based bulk deletion for testing
	return 0, fmt.Errorf("DeleteByPattern not yet implemented in mock cache")
}

// UpdateMetadata updates cache entry metadata - STUB IMPLEMENTATION for testing
func (m *MockCache) UpdateMetadata(ctx context.Context, key string, updater cache.MetadataUpdater) error {
	// TODO: Implement mock metadata updates for testing
	return fmt.Errorf("UpdateMetadata not yet implemented in mock cache")
}

// GetAndUpdate atomically gets and updates a cache entry - STUB IMPLEMENTATION for testing
func (m *MockCache) GetAndUpdate(ctx context.Context, key string, updater cache.ValueUpdater, ttl time.Duration) (any, error) {
	// TODO: Implement mock atomic get-and-update for testing
	return nil, fmt.Errorf("GetAndUpdate not yet implemented in mock cache")
}

var _ cache.Cache = (*MockCache)(nil)

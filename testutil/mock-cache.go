package testutil

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/MichaelAJay/go-cache"
)

type MockCache struct {
	data                      map[string]any
	metadata                  map[string]*cache.CacheEntryMetadata
	indexes                   map[string]map[string][]string // indexName -> indexKey -> []keys
	indexPatterns             map[string]string              // indexName -> keyPattern
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
	mu                        sync.RWMutex
}

func NewMockCache() *MockCache {
	return &MockCache{
		data:          make(map[string]any),
		metadata:      make(map[string]*cache.CacheEntryMetadata),
		indexes:       make(map[string]map[string][]string),
		indexPatterns: make(map[string]string),
	}
}

// Get implements Cache.
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

// Set implements Cache.
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
		Tags:         []string{},
	}

	// Update indexes if key matches any patterns
	m.updateIndexesForKey(key)

	return nil
}

// Delete implements Cache.
func (m *MockCache) Delete(ctx context.Context, key string) error {
	if m.OnDeleteCallback != nil {
		return m.OnDeleteCallback(ctx, key)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove from indexes before deleting
	m.removeFromAllIndexes(key)

	delete(m.data, key)
	delete(m.metadata, key)
	return nil
}

// Clear implements Cache.
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

// Has implements Cache.
func (m *MockCache) Has(ctx context.Context, key string) bool {
	if m.OnHasCallback != nil {
		return m.OnHasCallback(ctx, key)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.data[key]
	return exists
}

// GetKeys implements Cache.
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

// Close implements Cache.
func (m *MockCache) Close() error {
	if m.OnCloseCallback != nil {
		return m.OnCloseCallback()
	}
	return nil
}

// GetMany implements Cache.
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

// SetMany implements Cache.
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

// DeleteMany implements Cache.
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

// GetMetadata implements Cache.
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

// GetManyMetadata implements Cache.
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

// initializeIndex initializes an empty index for testing
func (m *MockCache) initializeIndex(indexName string, keyPattern string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.indexes[indexName] == nil {
		m.indexes[indexName] = make(map[string][]string)
	}
	m.indexPatterns[indexName] = keyPattern
}

// AddIndex adds a secondary index
func (m *MockCache) AddIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for context cancellation
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Initialize index if it doesn't exist
	if m.indexes[indexName] == nil {
		m.indexes[indexName] = make(map[string][]string)
		m.indexPatterns[indexName] = keyPattern
	}

	// Find keys matching the pattern
	matchingKeys := m.getKeysByPatternInternal(keyPattern)
	
	// Add matching keys to the index
	for _, key := range matchingKeys {
		m.addKeyToIndexInternal(indexName, indexKey, key)
	}

	return nil
}

// RemoveIndex removes a secondary index
func (m *MockCache) RemoveIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for context cancellation
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Remove specific indexKey from the index
	if index, exists := m.indexes[indexName]; exists {
		delete(index, indexKey)
		
		// If index is empty, remove the entire index
		if len(index) == 0 {
			delete(m.indexes, indexName)
			delete(m.indexPatterns, indexName)
		}
	}

	return nil
}

// GetByIndex retrieves keys by index
func (m *MockCache) GetByIndex(ctx context.Context, indexName string, indexKey string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Get keys from index
	if index, exists := m.indexes[indexName]; exists {
		if keys, exists := index[indexKey]; exists {
			// Return a copy to avoid race conditions
			result := make([]string, len(keys))
			copy(result, keys)
			return result, nil
		}
	}

	return []string{}, nil
}

// DeleteByIndex deletes keys by index
func (m *MockCache) DeleteByIndex(ctx context.Context, indexName string, indexKey string) error {
	// First get the keys to delete
	keys, err := m.GetByIndex(ctx, indexName, indexKey)
	if err != nil {
		return err
	}

	// Delete each key
	for _, key := range keys {
		if err := m.Delete(ctx, key); err != nil {
			return err
		}
	}

	// Remove the index entry
	m.mu.Lock()
	if index, exists := m.indexes[indexName]; exists {
		delete(index, indexKey)
	}
	m.mu.Unlock()

	return nil
}

// getKeysByPatternInternal returns keys matching pattern (internal helper)
func (m *MockCache) getKeysByPatternInternal(pattern string) []string {
	var matchingKeys []string
	
	// Convert glob pattern to regex
	regexPattern := strings.ReplaceAll(pattern, "*", ".*")
	regexPattern = "^" + regexPattern + "$"
	
	regex, err := regexp.Compile(regexPattern)
	if err != nil {
		return matchingKeys
	}
	
	// Find matching keys
	for key := range m.data {
		if regex.MatchString(key) {
			matchingKeys = append(matchingKeys, key)
		}
	}
	
	return matchingKeys
}

// addKeyToIndexInternal adds a key to an index (internal helper)
func (m *MockCache) addKeyToIndexInternal(indexName, indexKey, key string) {
	if m.indexes[indexName] == nil {
		m.indexes[indexName] = make(map[string][]string)
	}
	
	// Check if key already exists in the index
	keys := m.indexes[indexName][indexKey]
	for _, existingKey := range keys {
		if existingKey == key {
			return // Key already exists
		}
	}
	
	// Add key to index
	m.indexes[indexName][indexKey] = append(keys, key)
}

// removeKeyFromIndexInternal removes a key from an index (internal helper)
func (m *MockCache) removeKeyFromIndexInternal(indexName, indexKey, key string) {
	if index, exists := m.indexes[indexName]; exists {
		if keys, exists := index[indexKey]; exists {
			// Find and remove the key
			for i, existingKey := range keys {
				if existingKey == key {
					// Remove key by swapping with last element
					keys[i] = keys[len(keys)-1]
					keys = keys[:len(keys)-1]
					index[indexKey] = keys
					
					// If no keys left for this indexKey, remove it
					if len(keys) == 0 {
						delete(index, indexKey)
					}
					break
				}
			}
		}
	}
}

// updateIndexesForKey updates all indexes when a key is added
func (m *MockCache) updateIndexesForKey(key string) {
	// Check each index pattern to see if this key should be included
	for _, pattern := range m.indexPatterns {
		// Convert glob pattern to regex
		regexPattern := strings.ReplaceAll(pattern, "*", ".*")
		regexPattern = "^" + regexPattern + "$"
		
		if regex, err := regexp.Compile(regexPattern); err == nil {
			if regex.MatchString(key) {
				// This key matches the pattern, but we need to determine which indexKey to use
				// For now, we'll skip automatic indexing and require explicit AddIndex calls
				// since we don't know what the indexKey should be
			}
		}
	}
}

// removeFromAllIndexes removes a key from all indexes
func (m *MockCache) removeFromAllIndexes(key string) {
	for indexName, index := range m.indexes {
		for indexKey := range index {
			m.removeKeyFromIndexInternal(indexName, indexKey, key)
		}
	}
}

// GetKeysByPattern returns keys matching pattern
func (m *MockCache) GetKeysByPattern(ctx context.Context, pattern string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return m.getKeysByPatternInternal(pattern), nil
}

// DeleteByPattern deletes keys matching pattern
func (m *MockCache) DeleteByPattern(ctx context.Context, pattern string) (int, error) {
	// First get matching keys
	keys, err := m.GetKeysByPattern(ctx, pattern)
	if err != nil {
		return 0, err
	}

	// Delete each key
	deleted := 0
	for _, key := range keys {
		if err := m.Delete(ctx, key); err != nil {
			return deleted, err
		}
		deleted++
	}

	return deleted, nil
}

// UpdateMetadata updates cache entry metadata
func (m *MockCache) UpdateMetadata(ctx context.Context, key string, updater cache.MetadataUpdater) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for context cancellation
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Check if key exists
	if _, exists := m.data[key]; !exists {
		return nil // Key doesn't exist, nothing to update
	}

	// Get current metadata or create new one
	currentMetadata := m.metadata[key]
	if currentMetadata == nil {
		currentMetadata = &cache.CacheEntryMetadata{
			Key:          key,
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
			AccessCount:  0,
			Size:         int64(len(fmt.Sprintf("%v", m.data[key]))),
			Tags:         []string{},
		}
	}

	// Apply updater
	updatedMetadata := updater(currentMetadata)
	if updatedMetadata != nil {
		m.metadata[key] = updatedMetadata
	}

	return nil
}

// GetAndUpdate atomically gets and updates a cache entry
func (m *MockCache) GetAndUpdate(ctx context.Context, key string, updater cache.ValueUpdater, ttl time.Duration) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Get current value
	currentValue, exists := m.data[key]
	if !exists {
		currentValue = nil
	}

	// Apply updater
	newValue, shouldUpdate := updater(currentValue)
	
	// Update if requested
	if shouldUpdate {
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
				Size:         int64(len(fmt.Sprintf("%v", newValue))),
				Tags:         []string{},
			}
		} else {
			m.metadata[key].LastAccessed = now
			m.metadata[key].AccessCount++
			m.metadata[key].TTL = ttl
			m.metadata[key].Size = int64(len(fmt.Sprintf("%v", newValue)))
		}
	}

	return currentValue, nil
}

var _ cache.Cache = (*MockCache)(nil)

// MockProvider implements the CacheProvider interface for testing
type MockProvider struct {
	createCallback func(options *cache.CacheOptions) (cache.Cache, error)
}

// NewMockProvider creates a new MockProvider
func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

// Create implements CacheProvider.
func (p *MockProvider) Create(options *cache.CacheOptions) (cache.Cache, error) {
	if p.createCallback != nil {
		return p.createCallback(options)
	}

	mockCache := NewMockCache()
	
	// Apply configuration options to the mock cache
	if options != nil {
		// Pre-configure indexes if specified
		if options.Indexes != nil {
			for indexName, keyPattern := range options.Indexes {
				// Initialize empty index for each configured index
				mockCache.initializeIndex(indexName, keyPattern)
			}
		}
	}
	
	return mockCache, nil
}

// SetCreateCallback allows customizing cache creation behavior in tests
func (p *MockProvider) SetCreateCallback(callback func(options *cache.CacheOptions) (cache.Cache, error)) {
	p.createCallback = callback
}

var _ cache.CacheProvider = (*MockProvider)(nil)

// MockCacheManager implements the CacheManager interface for testing
type MockCacheManager struct {
	providers map[string]cache.CacheProvider
	caches    map[string]cache.Cache
	mu        sync.RWMutex
}

// NewMockCacheManager creates a new MockCacheManager
func NewMockCacheManager() *MockCacheManager {
	return &MockCacheManager{
		providers: make(map[string]cache.CacheProvider),
		caches:    make(map[string]cache.Cache),
	}
}

// GetCache returns a named cache instance with specified options
func (m *MockCacheManager) GetCache(name string, options ...cache.CacheOption) (cache.Cache, error) {
	m.mu.RLock()
	cacheInstance, exists := m.caches[name]
	m.mu.RUnlock()

	if exists {
		return cacheInstance, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if cacheInstance, exists = m.caches[name]; exists {
		return cacheInstance, nil
	}

	// Create cache options
	opts := &cache.CacheOptions{}
	for _, opt := range options {
		opt(opts)
	}

	// Get provider for the cache type
	provider, exists := m.providers[name]
	if !exists {
		return nil, fmt.Errorf("cache provider not found: %s", name)
	}

	// Create new cache instance
	cacheInstance, err := provider.Create(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	m.caches[name] = cacheInstance
	return cacheInstance, nil
}

// RegisterProvider registers a new cache provider
func (m *MockCacheManager) RegisterProvider(name string, provider cache.CacheProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.providers[name] = provider
}

// GetCaches returns all registered cache instances
func (m *MockCacheManager) GetCaches() map[string]cache.Cache {
	m.mu.RLock()
	defer m.mu.RUnlock()

	caches := make(map[string]cache.Cache, len(m.caches))
	for k, v := range m.caches {
		caches[k] = v
	}
	return caches
}

// Close closes all managed caches
func (m *MockCacheManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, cache := range m.caches {
		if err := cache.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close cache %s: %w", name, err)
		}
		delete(m.caches, name)
	}

	return lastErr
}

var _ cache.CacheManager = (*MockCacheManager)(nil)

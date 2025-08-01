package cache

import (
	"fmt"
	"sync"
)

// CacheManager defines the interface for managing cache instances
type CacheManager interface {
	// GetCache returns a named cache instance with specified options
	GetCache(name string, options ...CacheOption) (Cache, error)

	// RegisterProvider registers a new cache provider
	RegisterProvider(name string, provider CacheProvider)

	// GetCaches returns all registered cache instances
	GetCaches() map[string]Cache

	// Close closes all managed caches
	Close() error
}

// cacheManager implements the CacheManager interface
type cacheManager struct {
	providers map[string]CacheProvider
	caches    map[string]Cache
	mu        sync.RWMutex
}

// NewCacheManager creates a new cache manager instance.
// The cache manager provides centralized management of multiple cache instances,
// supporting different providers (memory, Redis) and configurations.
// It handles provider registration, cache lifecycle, and resource cleanup.
func NewCacheManager() CacheManager {
	return &cacheManager{
		providers: make(map[string]CacheProvider),
		caches:    make(map[string]Cache),
	}
}

// GetCache returns a named cache instance with specified options
func (m *cacheManager) GetCache(name string, options ...CacheOption) (Cache, error) {
	m.mu.RLock()
	cache, exists := m.caches[name]
	m.mu.RUnlock()

	if exists {
		return cache, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if cache, exists = m.caches[name]; exists {
		return cache, nil
	}

	// Create cache options
	opts := &CacheOptions{}
	for _, opt := range options {
		opt(opts)
	}

	// Get provider for the cache type
	provider, exists := m.providers[name]
	if !exists {
		return nil, fmt.Errorf("cache provider not found: %s", name)
	}

	// Create new cache instance
	cache, err := provider.Create(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	m.caches[name] = cache
	return cache, nil
}

// RegisterProvider registers a new cache provider
func (m *cacheManager) RegisterProvider(name string, provider CacheProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.providers[name] = provider
}

// GetCaches returns all registered cache instances
func (m *cacheManager) GetCaches() map[string]Cache {
	m.mu.RLock()
	defer m.mu.RUnlock()

	caches := make(map[string]Cache, len(m.caches))
	for k, v := range m.caches {
		caches[k] = v
	}
	return caches
}

// Close closes all managed caches
func (m *cacheManager) Close() error {
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

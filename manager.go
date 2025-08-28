package cache

import (
	"fmt"
	"sync"

	"github.com/MichaelAJay/go-cache/interfaces"
)

// CacheManager_OLD defines the legacy interface for managing cache instances
// This is being replaced by the new generic CacheManager
type CacheManager_OLD interface {
	// GetCache returns a named cache instance with specified options
	GetCache(name string, options ...interfaces.CacheOption) (interfaces.Cache_OLD, error)

	// RegisterProvider registers a new cache provider
	RegisterProvider(name string, provider interfaces.CacheProvider_OLD)

	// GetCaches returns all registered cache instances
	GetCaches() map[string]interfaces.Cache_OLD

	// Close closes all managed caches
	Close() error
}

// legacyCacheManager implements the CacheManager_OLD interface
type legacyCacheManager struct {
	providers map[string]interfaces.CacheProvider_OLD
	caches    map[string]interfaces.Cache_OLD
	mu        sync.RWMutex
}

// NewLegacyCacheManager creates a legacy cache manager instance.
// This is being replaced by the new generic CacheManager.
func NewLegacyCacheManager() CacheManager_OLD {
	return &legacyCacheManager{
		providers: make(map[string]interfaces.CacheProvider_OLD),
		caches:    make(map[string]interfaces.Cache_OLD),
	}
}

// GetCache returns a named cache instance with specified options
func (m *legacyCacheManager) GetCache(name string, options ...interfaces.CacheOption) (interfaces.Cache_OLD, error) {
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
	opts := &interfaces.CacheOptions{}
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
func (m *legacyCacheManager) RegisterProvider(name string, provider interfaces.CacheProvider_OLD) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.providers[name] = provider
}

// GetCaches returns all registered cache instances
func (m *legacyCacheManager) GetCaches() map[string]interfaces.Cache_OLD {
	m.mu.RLock()
	defer m.mu.RUnlock()

	caches := make(map[string]interfaces.Cache_OLD, len(m.caches))
	for k, v := range m.caches {
		caches[k] = v
	}
	return caches
}

// Close closes all managed caches
func (m *legacyCacheManager) Close() error {
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

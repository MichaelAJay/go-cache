package cache

import (
	"github.com/MichaelAJay/go-cache/interfaces"
)

// CacheManager provides typed cache creation methods
// This is the concrete type that applications will use
type CacheManager struct {
	providers map[string]interfaces.CacheProvider
	caches    map[string]any // stores Cache[T] instances with various T types
}

// NewCacheManager creates a new cache manager
func NewCacheManager() *CacheManager {
	return &CacheManager{
		providers: make(map[string]interfaces.CacheProvider),
		caches:    make(map[string]any),
	}
}

// RegisterProvider registers a cache provider
func (m *CacheManager) RegisterProvider(name string, provider interfaces.CacheProvider) {
	m.providers[name] = provider
}

// Close closes all managed caches
func (m *CacheManager) Close() error {
	// Implementation will iterate through caches and close them
	panic("not implemented in Phase 0 - interface design only")
}

// NewCache creates a new typed cache instance
// This is implemented as a package-level generic function
func NewCache[T any](manager *CacheManager, providerName string, opts ...interfaces.Option) (interfaces.Cache[T], error) {
	// This function will be implemented to create and return Cache[T] instances
	// The implementation will need provider-specific logic
	panic("not implemented in Phase 0 - interface design only")
}
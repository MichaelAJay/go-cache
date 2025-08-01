package memory

import (
	"github.com/MichaelAJay/go-cache/interfaces"
)

// memoryProvider implements the CacheProvider interface
type memoryProvider struct{}

// NewProvider creates a new memory cache provider instance.
// This provider offers in-memory caching with thread-safe operations,
// automatic cleanup, and secondary indexing capabilities.
func NewProvider() interfaces.CacheProvider {
	return &memoryProvider{}
}

// Create creates a new memory cache instance with the provided options.
// If options is nil, default configuration will be used.
// Returns an error if the cache cannot be initialized (e.g., serializer issues).
func (p *memoryProvider) Create(options *interfaces.CacheOptions) (interfaces.Cache, error) {
	if options == nil {
		options = &interfaces.CacheOptions{}
	}

	return NewMemoryCache(options)
}

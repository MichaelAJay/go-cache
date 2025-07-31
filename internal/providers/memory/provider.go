package memory

import (
	"github.com/MichaelAJay/go-cache"
)

// memoryProvider implements the CacheProvider interface
type memoryProvider struct{}

// NewProvider creates a new memory cache provider
func NewProvider() cache.CacheProvider {
	return &memoryProvider{}
}

// Create creates a new memory cache instance
func (p *memoryProvider) Create(options *cache.CacheOptions) (cache.Cache, error) {
	if options == nil {
		options = &cache.CacheOptions{}
	}

	return NewMemoryCache(options)
}

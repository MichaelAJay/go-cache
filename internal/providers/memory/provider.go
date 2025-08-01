package memory

import (
	"github.com/MichaelAJay/go-cache/interfaces"
)

// memoryProvider implements the CacheProvider interface
type memoryProvider struct{}

// NewProvider creates a new memory cache provider
func NewProvider() interfaces.CacheProvider {
	return &memoryProvider{}
}

// Create creates a new memory cache instance
func (p *memoryProvider) Create(options *interfaces.CacheOptions) (interfaces.Cache, error) {
	if options == nil {
		options = &interfaces.CacheOptions{}
	}

	return NewMemoryCache(options)
}

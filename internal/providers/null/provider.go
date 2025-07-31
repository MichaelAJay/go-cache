package null

import (
	"github.com/MichaelAJay/go-cache"
)

// nullProvider implements the CacheProvider interface
type nullProvider struct{}

// NewProvider creates a new null cache provider
func NewProvider() cache.CacheProvider {
	return &nullProvider{}
}

// Create creates a new null cache instance
func (p *nullProvider) Create(options *cache.CacheOptions) (cache.Cache, error) {
	return NewNullCache(options), nil
}
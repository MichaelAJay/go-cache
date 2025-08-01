package null

import (
	"github.com/MichaelAJay/go-cache/interfaces"
)

// nullProvider implements the CacheProvider interface
type nullProvider struct{}

// NewProvider creates a new null cache provider
func NewProvider() interfaces.CacheProvider {
	return &nullProvider{}
}

// Create creates a new null cache instance
func (p *nullProvider) Create(options *interfaces.CacheOptions) (interfaces.Cache, error) {
	return NewNullCache(options), nil
}
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

// Name returns the provider name for registration
func (p *nullProvider) Name() string {
	return "null"
}

// Validate checks if the provided options are compatible with null provider
func (p *nullProvider) Validate(options *interfaces.CacheOptions) error {
	// Null provider accepts all options (ignores them)
	return nil
}

// Close cleans up any provider-level resources
func (p *nullProvider) Close() error {
	// Null provider has no resources to clean up
	return nil
}
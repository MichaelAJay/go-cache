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

// Name returns the provider name for registration
func (p *memoryProvider) Name() string {
	return "memory"
}

// Validate checks if the provided options are compatible with memory provider
func (p *memoryProvider) Validate(options *interfaces.CacheOptions) error {
	// Memory provider accepts all options
	return nil
}

// Close cleans up any provider-level resources
func (p *memoryProvider) Close() error {
	// Memory provider has no global resources to clean up
	return nil
}

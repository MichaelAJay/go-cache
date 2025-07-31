package providers

import (
	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/internal/providers/null"
)

// NewNullProvider creates a new null cache provider
// This provides a clean public API while keeping the implementation internal
func NewNullProvider() cache.CacheProvider {
	return null.NewProvider()
}
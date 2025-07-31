package providers

import (
	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/internal/providers/memory"
)

// NewMemoryProvider creates a new memory cache provider
// This provides a clean public API while keeping the implementation internal
func NewMemoryProvider() cache.CacheProvider {
	return memory.NewProvider()
}
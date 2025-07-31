package providers

import (
	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/internal/providers/redis"
)

// NewRedisProvider creates a new Redis cache provider
// This provides a clean public API while keeping the implementation internal
func NewRedisProvider() cache.CacheProvider {
	return redis.NewProvider()
}
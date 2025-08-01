package cache

import (
	memoryprovider "github.com/MichaelAJay/go-cache/internal/providers/memory"
	redisprovider "github.com/MichaelAJay/go-cache/internal/providers/redis"
)

// NewMemoryProvider creates a new memory cache provider
func NewMemoryProvider() CacheProvider {
	return memoryprovider.NewProvider()
}

// NewRedisProvider creates a new Redis cache provider
func NewRedisProvider() CacheProvider {
	return redisprovider.NewProvider()
}
package redis

import (
	"github.com/MichaelAJay/go-cache/interfaces"
)

// redisProvider implements the CacheProvider interface
type redisProvider struct{}

// NewProvider creates a new Redis cache provider instance.
// This provider offers distributed caching with thread-safe operations,
// Redis-based secondary indexing, distributed locks, and enterprise features.
func NewProvider() interfaces.CacheProvider {
	return &redisProvider{}
}

// Name returns the provider name for registration
func (p *redisProvider) Name() string {
	return "redis"
}

// Validate checks if the provided options are compatible with Redis provider
func (p *redisProvider) Validate(options *interfaces.CacheOptions) error {
	if options == nil {
		return nil
	}
	
	if options.RedisOptions == nil {
		options.RedisOptions = &interfaces.RedisOptions{
			Address: "localhost:6379",
			DB:      0,
		}
	}
	
	// Validate Redis connection parameters
	if options.RedisOptions.Address == "" {
		options.RedisOptions.Address = "localhost:6379"
	}
	
	return nil
}

// Close cleans up any provider-level resources
func (p *redisProvider) Close() error {
	// Provider-level cleanup - individual cache instances manage their own connections
	return nil
}
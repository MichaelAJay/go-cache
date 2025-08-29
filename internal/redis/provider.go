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
// DEPRECATED: Provider validation is being removed in Redis-only refactoring
// Redis client should be injected directly via CacheOptions.RedisClient
func (p *redisProvider) Validate(options *interfaces.CacheOptions) error {
	// No-op validation - Redis client is expected to be pre-configured
	// and injected via options.RedisClient in the new consolidated approach
	return nil
}

// Close cleans up any provider-level resources
func (p *redisProvider) Close() error {
	// Provider-level cleanup - individual cache instances manage their own connections
	return nil
}
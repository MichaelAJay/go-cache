package redis

import (
	"fmt"
	
	"github.com/MichaelAJay/go-cache/interfaces"
)

// NewRedisCache creates a new Redis cache instance with the given options
// This is the factory function that implements CacheFactory[T] signature
func NewRedisCacheFactory[T any](options *interfaces.CacheOptions) (interfaces.Cache[T], error) {
	return NewRedisCache[T](options)
}

// RegisterRedisProvider registers the Redis provider with a cache manager
// This is a convenience function to register the provider and its factory
func RegisterRedisProvider(manager interfaces.Manager) error {
	provider := NewProvider()
	manager.RegisterProvider("redis", provider)
	return nil
}

// NewRedisFactory creates a factory function for Redis caches of a specific type
// This allows for type-safe creation of Redis caches
func NewRedisFactory[T any]() interfaces.CacheFactory[T] {
	return func(options *interfaces.CacheOptions) (interfaces.Cache[T], error) {
		return NewRedisCache[T](options)
	}
}

// ValidateRedisConfiguration validates Redis-specific configuration
// DEPRECATED: Provider-specific validation is being removed in Redis-only refactoring  
// Redis client validation should be handled at the client creation level, not here
func ValidateRedisConfiguration(options *interfaces.CacheOptions) error {
	if options == nil {
		return fmt.Errorf("options cannot be nil")
	}
	
	// In the new consolidated approach, we expect RedisClient to be pre-configured
	// Validation of Redis connection should happen when creating the Redis client,
	// not in the cache configuration validation
	if options.RedisClient == nil && options.RedisOptions == nil {
		return fmt.Errorf("either RedisClient or RedisOptions must be provided")
	}
	
	return nil
}
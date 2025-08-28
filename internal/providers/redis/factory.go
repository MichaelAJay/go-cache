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
func ValidateRedisConfiguration(options *interfaces.CacheOptions) error {
	if options == nil {
		return fmt.Errorf("options cannot be nil")
	}
	
	if options.RedisOptions == nil {
		return fmt.Errorf("RedisOptions cannot be nil for Redis provider")
	}
	
	if options.RedisOptions.Address == "" {
		return fmt.Errorf("Redis address cannot be empty")
	}
	
	// Validate DB number
	if options.RedisOptions.DB < 0 || options.RedisOptions.DB > 15 {
		return fmt.Errorf("Redis DB must be between 0 and 15")
	}
	
	// Validate pool size
	if options.RedisOptions.PoolSize < 0 {
		return fmt.Errorf("Redis pool size cannot be negative")
	}
	
	if options.RedisOptions.PoolSize == 0 {
		options.RedisOptions.PoolSize = 10 // Default pool size
	}
	
	return nil
}
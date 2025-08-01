package redis

import (
	"errors"

	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/go-redis/redis/v8"
)

// RedisClientFactory is the function used to create a new Redis client.
// This can be replaced in tests to inject a mock client.
var RedisClientFactory = redis.NewClient

// redisProvider implements the CacheProvider interface
type redisProvider struct{}

// NewProvider creates a new Redis cache provider instance.
// This provider offers distributed caching with Redis backend,
// supporting atomic operations, pipelines, and connection pooling.
func NewProvider() interfaces.CacheProvider {
	return &redisProvider{}
}

// Create creates a new Redis cache instance with the provided options.
// Requires CacheOptions with RedisOptions configured. If RedisOptions
// is nil, attempts to load configuration from environment variables.
// Returns an error if Redis connection cannot be established or
// configuration is invalid.
func (p *redisProvider) Create(options *interfaces.CacheOptions) (interfaces.Cache, error) {
	if options == nil {
		return nil, errors.New("cache options cannot be nil")
	}

	// If Redis options aren't provided, try to load from environment
	if options.RedisOptions == nil {
		options.RedisOptions = LoadRedisOptionsFromEnv()
	}

	// Validate that we have an address at minimum
	if options.RedisOptions.Address == "" {
		return nil, ErrInvalidRedisOptions
	}

	// Configure Redis client
	redisOpts := &redis.Options{
		Addr:     options.RedisOptions.Address,
		Password: options.RedisOptions.Password,
		DB:       options.RedisOptions.DB,
	}

	// Set pool size if specified
	if options.RedisOptions.PoolSize > 0 {
		redisOpts.PoolSize = options.RedisOptions.PoolSize
	}

	// Use the factory function to create the client
	client := RedisClientFactory(redisOpts)

	return NewRedisCache(client, options)
}

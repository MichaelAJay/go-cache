package redis

import (
	"errors"

	"github.com/MichaelAJay/go-cache"
	"github.com/go-redis/redis/v8"
)

// redisProvider implements the CacheProvider interface
type redisProvider struct{}

// NewProvider creates a new Redis cache provider
func NewProvider() cache.CacheProvider {
	return &redisProvider{}
}

// Create creates a new Redis cache instance
func (p *redisProvider) Create(options *cache.CacheOptions) (cache.Cache, error) {
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

	client := redis.NewClient(redisOpts)

	return NewRedisCache(client, options)
}

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-logger"
)

func main() {
	// Create a logger
	log := logger.New(logger.DefaultConfig)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Method 1: Using explicit configuration
	explicitConfig(ctx, log)

	// Method 2: Using environment-based configuration
	envConfig(ctx, log)
}

// explicitConfig demonstrates using the Redis cache with explicit configuration
func explicitConfig(ctx context.Context, log logger.Logger) {
	// Create Redis provider
	provider := cache.NewRedisProvider()

	// Configure cache with explicit Redis options
	cacheOptions := &cache.CacheOptions{
		TTL:    time.Hour,
		Logger: log,
		RedisOptions: &cache.RedisOptions{
			Address:  "localhost:6379",
			Password: "", // No password
			DB:       0,  // Default DB
			PoolSize: 10, // Connection pool size
		},
	}

	// Create cache instance
	c, err := provider.Create(cacheOptions)
	if err != nil {
		log.Error("Failed to create cache with explicit config",
			logger.Field{Key: "error", Value: err})
		return
	}
	defer c.Close()

	// Use the cache
	basicCacheOperations(ctx, c, log, "explicit")
}

// envConfig demonstrates using the Redis cache with environment-based configuration
func envConfig(ctx context.Context, log logger.Logger) {
	// Create Redis provider
	provider := cache.NewRedisProvider()

	// Configure cache without specifying Redis options (will use environment variables)
	cacheOptions := &cache.CacheOptions{
		TTL:    time.Hour,
		Logger: log,
		// RedisOptions will be loaded from environment variables
	}

	// Create cache instance
	c, err := provider.Create(cacheOptions)
	if err != nil {
		log.Error("Failed to create cache with environment config",
			logger.Field{Key: "error", Value: err})
		return
	}
	defer c.Close()

	// Use the cache
	basicCacheOperations(ctx, c, log, "env")
}

// basicCacheOperations demonstrates common cache operations
func basicCacheOperations(ctx context.Context, c cache.Cache, log logger.Logger, configType string) {
	prefix := fmt.Sprintf("demo:%s", configType)

	// Set value
	key := fmt.Sprintf("%s:greeting", prefix)
	err := c.Set(ctx, key, "Hello from Redis!", time.Minute*5)
	if err != nil {
		log.Error("Failed to set value",
			logger.Field{Key: "error", Value: err},
			logger.Field{Key: "configType", Value: configType})
		return
	}

	// Get value
	value, exists, err := c.Get(ctx, key)
	if err != nil {
		log.Error("Failed to get value",
			logger.Field{Key: "error", Value: err},
			logger.Field{Key: "configType", Value: configType})
		return
	}

	if exists {
		log.Info("Retrieved value",
			logger.Field{Key: "value", Value: value},
			logger.Field{Key: "configType", Value: configType})
	} else {
		log.Warn("Value not found",
			logger.Field{Key: "key", Value: key},
			logger.Field{Key: "configType", Value: configType})
	}

	// Set multiple values
	items := map[string]any{
		fmt.Sprintf("%s:item1", prefix): "First item",
		fmt.Sprintf("%s:item2", prefix): "Second item",
	}
	if err := c.SetMany(ctx, items, time.Minute*5); err != nil {
		log.Error("Failed to set multiple values",
			logger.Field{Key: "error", Value: err},
			logger.Field{Key: "configType", Value: configType})
		return
	}

	// Get multiple values
	keys := []string{
		fmt.Sprintf("%s:item1", prefix),
		fmt.Sprintf("%s:item2", prefix),
	}
	results, err := c.GetMany(ctx, keys)
	if err != nil {
		log.Error("Failed to get multiple values",
			logger.Field{Key: "error", Value: err},
			logger.Field{Key: "configType", Value: configType})
		return
	}

	log.Info("Retrieved multiple values",
		logger.Field{Key: "count", Value: len(results)},
		logger.Field{Key: "configType", Value: configType})


	// Delete our test data
	keys = append(keys, key)
	if err := c.DeleteMany(ctx, keys); err != nil {
		log.Error("Failed to clean up test data",
			logger.Field{Key: "error", Value: err},
			logger.Field{Key: "configType", Value: configType})
	}
}

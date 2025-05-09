# Redis Cache Provider

This package implements a Redis-based cache provider for the go-cache library.

## Features

- Full implementation of the `cache.Cache` interface
- Persistent data storage in Redis
- Support for TTL-based expiration
- Thread-safe operations
- Metadata tracking (creation time, access count, etc.)
- Metrics collection

## Setup

### Required Dependencies

To use this package, ensure you have the following dependencies:

```sh
# Redis client
go get github.com/go-redis/redis/v8

# For testing
go get github.com/alicebob/miniredis/v2
```

### Configuration Options

The Redis provider can be configured using the following options in the `cache.RedisOptions` struct:

- `Address` - Redis server address (required)
- `Password` - Redis server password (optional)
- `DB` - Redis database index (optional, defaults to 0)
- `PoolSize` - Connection pool size (optional)

## Usage

```go
package main

import (
	"context"
	"time"
	
	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/providers/redis"
	"github.com/MichaelAJay/go-logger"
)

func main() {
	// Create a logger
	log := logger.New(logger.DefaultConfig)
	
	// Create Redis provider
	provider := redis.NewProvider()
	
	// Configure cache with Redis options
	cacheOptions := &cache.CacheOptions{
		TTL: time.Hour,
		Logger: log,
		RedisOptions: &cache.RedisOptions{
			Address:  "localhost:6379",
			Password: "",  // No password
			DB:       0,   // Default DB
			PoolSize: 10,  // Connection pool size
		},
	}
	
	// Create cache instance
	c, err := provider.Create(cacheOptions)
	if err != nil {
		log.Fatal("Failed to create cache", logger.Field{Key: "error", Value: err})
	}
	defer c.Close()
	
	// Use the cache
	ctx := context.Background()
	c.Set(ctx, "greeting", "Hello, World!", time.Minute*5)
	
	value, exists, _ := c.Get(ctx, "greeting")
	if exists {
		log.Info("Retrieved value", logger.Field{Key: "value", Value: value})
	}
}
```

## Environment Variables

When using this provider, you can configure Redis connection details via environment variables:

- `REDIS_ADDR` - Redis server address (default: "localhost:6379")
- `REDIS_PASSWORD` - Redis server password (default: "")
- `REDIS_DB` - Redis database index (default: 0)
- `REDIS_POOL_SIZE` - Connection pool size (default: 10)

To use environment-based configuration, use the `LoadRedisOptionsFromEnv()` function.

## Testing

This package includes comprehensive tests, including:
- Unit tests with in-memory Redis (miniredis)
- Optional integration tests with a real Redis server

To run tests with a real Redis instance:

```sh
REDIS_ADDR=localhost:6379 go test -v ./providers/redis
``` 
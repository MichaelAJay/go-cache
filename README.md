# Go Cache

A flexible and extensible caching library for Go applications.

## Features

- Uniform interface for different cache implementations
- Support for multiple cache providers (memory, Redis, etc.)
- Thread-safe operations
- Context-aware operations
- TTL (Time-To-Live) support
- Bulk operations
- Cache entry metadata
- Metrics collection
- Logging middleware
- Metrics middleware

## Installation

```bash
go get github.com/MichaelAJay/go-cache
```

## Quick Start

```go
package main

import (
    "context"
    "time"

    "github.com/MichaelAJay/go-cache"
    "github.com/MichaelAJay/go-cache/providers/memory"
)

func main() {
    // Create a cache manager
    manager := cache.NewCacheManager()

    // Register the memory cache provider
    manager.RegisterProvider("memory", memory.NewProvider())

    // Create a cache instance
    cache, err := manager.GetCache("memory",
        cache.WithTTL(5*time.Minute),
        cache.WithMaxEntries(1000),
    )
    if err != nil {
        panic(err)
    }

    // Use the cache
    ctx := context.Background()
    cache.Set(ctx, "key", "value", 0)
    value, exists, _ := cache.Get(ctx, "key")
}
```

## Cache Providers

### Memory Cache

The memory cache provider uses an in-memory map to store cache entries. It's suitable for single-instance applications.

```go
manager.RegisterProvider("memory", memory.NewProvider())
```

### Redis Cache

The Redis cache provider uses Redis as the backend storage. It's suitable for distributed applications.

```go
import "github.com/MichaelAJay/go-cache/providers/redis"

// Register Redis provider
manager.RegisterProvider("redis", redis.NewProvider())

// Create a Redis cache
redisCache, err := manager.GetCache("redis",
    cache.WithTTL(5*time.Minute),
    cache.WithRedisOptions(&cache.RedisOptions{
        Address:  "localhost:6379",
        Password: "",
        DB:       0,
        PoolSize: 10,
    }),
)
```

See the [Redis Provider README](providers/redis/README.md) for more details and examples.

## Configuration Options

- `WithTTL`: Set the default time-to-live for cache entries
- `WithMaxEntries`: Set the maximum number of entries (for memory cache)
- `WithCleanupInterval`: Set the interval for cleaning up expired entries
- `WithLogger`: Set a logger for the cache
- `WithRedisOptions`: Set Redis-specific options

## Middleware

### Logging Middleware

The logging middleware logs all cache operations with timing information.

```go
cache := middleware.NewLoggingMiddleware(logger)(cache)
```

### Metrics Middleware

The metrics middleware collects performance metrics for cache operations.

```go
metrics := cache.NewMetrics()
cache := middleware.NewMetricsMiddleware(metrics)(cache)
```

## Examples

See the `examples` directory for complete examples:

- `examples/simple`: Basic cache usage
- `examples/redis_cache.go`: Basic Redis cache usage
- `examples/redis_msgpack_example`: Redis with MessagePack serialization
- `examples/redis_examples`: Advanced Redis examples

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
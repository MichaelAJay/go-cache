# go-cache

A high-performance, production-ready Redis cache implementation for Go with full generic type support and comprehensive atomic operations.

## Features

- **Generic Type Support**: Full type safety with Go generics (`Cache[T any]`)
- **Redis-Only Backend**: Optimized for Redis with Lua script-based atomic operations
- **Goroutine-Safe**: All operations are concurrent-safe without external synchronization
- **Advanced Indexing**: Owner-based indexing for grouped cache entries (e.g., user sessions)
- **Atomic Operations**: `GetOrSet`, `Update`, `SetIfExists`, `SetIfNotExists` with zero race conditions
- **Batch Operations**: High-performance multi-key operations (`GetMany`, `SetMany`, `DeleteMany`)
- **Counter Operations**: Atomic increment/decrement for integers and floats
- **Circuit Breaker**: Built-in resilience pattern for Redis connectivity
- **Enterprise Features**: Metrics, hooks, serialization options (JSON, Gob, MessagePack)
- **Comprehensive Testing**: Container-based integration tests with latency simulation

## Quick Start

```go
package main

import (
    "context"
    "time"
    
    "github.com/MichaelAJay/go-cache"
    "github.com/MichaelAJay/go-cache/config"
    "github.com/redis/go-redis/v9"
)

type User struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

func main() {
    ctx := context.Background()
    
    // Create Redis client
    rdb := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
    
    // Configure cache
    opts := config.NewCacheOptions(rdb)
    
    // Create cache instance
    userCache, err := cache.NewRedisCache[User](opts)
    if err != nil {
        panic(err)
    }
    defer userCache.Close()
    
    // Store a user
    user := User{ID: "123", Name: "Alice"}
    err = userCache.Set(ctx, user, time.Hour)
    if err != nil {
        panic(err)
    }
    
    // Retrieve by ID
    retrieved, found, err := userCache.Get(ctx, "123")
    if err != nil {
        panic(err)
    }
    
    if found {
        fmt.Printf("Found user: %+v\n", retrieved)
    }
}
```

## Advanced Usage

### With Owner-Based Indexing

```go
// Define extractors for session management
extractor := &cache.IndexExtractor[Session]{
    GetEntryKey: func(s Session) string { return s.SessionID },
    GetOwnerKey: func(s Session) string { return s.UserID },
}

// Create cache with indexing
sessionCache, err := cache.NewRedisCache[Session](
    opts,
    cache.WithIndexExtractor(extractor),
    cache.WithRedisOptions(&cache.RedisOptions{
        DataPrefix:  "sessions:data:",
        IndexPrefix: "sessions:index:",
        MetaPrefix:  "sessions:meta:",
    }),
)

// Get all sessions for a user
userSessions, err := sessionCache.GetByOwner(ctx, "user123")

// Delete all sessions for a user
deletedCount, err := sessionCache.DeleteByOwner(ctx, "user123")
```

### Atomic Operations

```go
// Atomic get-or-set with loader function
session, err := sessionCache.GetOrSet(ctx, "session123", func(ctx context.Context) (Session, error) {
    return createNewSession("user123"), nil
}, time.Hour)

// Atomic update with updater function
updatedSession, err := sessionCache.Update(ctx, "session123", func(old Session, exists bool) (Session, error) {
    if !exists {
        return Session{}, errors.New("session not found")
    }
    old.LastAccessed = time.Now()
    return old, nil
}, time.Hour)
```

### Batch Operations

```go
// Batch get multiple users
userIDs := []string{"123", "456", "789"}
users, err := userCache.GetMany(ctx, userIDs)

// Batch set multiple users
newUsers := []User{
    {ID: "111", Name: "Bob"},
    {ID: "222", Name: "Carol"},
}
err = userCache.SetMany(ctx, newUsers, time.Hour)
```

### Counter Operations

```go
// Increment page views
newCount, err := userCache.Increment(ctx, "page:views", 1)

// Rate limiting counter
currentRequests, err := userCache.Increment(ctx, "rate:user123", 1)
if currentRequests > 100 {
    // Rate limit exceeded
}
```

## Configuration Options

```go
opts := config.NewCacheOptions(redisClient).
    WithTTL(time.Hour).
    WithSerializer("msgpack"). // json, gob, msgpack
    WithMetrics(customMetrics).
    WithHooks(&config.CacheHooks{
        PreSet: func(ctx context.Context, key string, value any) error {
            // Custom validation
            return nil
        },
        PostGet: func(ctx context.Context, key string, found bool, err error) {
            // Custom logging
        },
    })
```

## Testing

The project includes comprehensive testing infrastructure:

```bash
# Unit tests
make test

# Integration tests with containers
make test-containers

# Docker compose tests
make docker-up && make docker-test

# Tests with network latency simulation
make docker-test-latency
```

### Test Modes

- **Direct**: Use existing Redis instance
- **Containers**: Cold start with testcontainers
- **Compose**: Use docker-compose services

Set environment variables for testing:
```bash
export GOCACHE_TEST_MODE=containers
export GOCACHE_TEST_LATENCY=enabled
export GOCACHE_TEST_REDIS_LATENCY_MS=100
```

## Architecture

### Goroutine Safety
All operations are designed to be goroutine-safe without external locking:
- Lua scripts ensure atomicity for complex operations
- Singleflight pattern prevents duplicate loader calls in `GetOrSet`
- Circuit breaker provides resilience against Redis failures

### Redis Key Structure
```
{DataPrefix}{EntryKey}           # Actual data storage
{IndexPrefix}{OwnerKey}          # Owner -> []EntryKey mapping
{MetaPrefix}{EntryKey}           # Entry metadata
{LockPrefix}{EntryKey}           # Distributed locks
```

### Serialization
- **MessagePack**: Default, compact binary format
- **JSON**: Human-readable, cross-language
- **Gob**: Go-native, fastest for Go-to-Go

## Requirements

- Go 1.23.3+
- Redis 6.0+ (for Lua script support)

## Dependencies

- `github.com/redis/go-redis/v9` - Redis client
- `github.com/MichaelAJay/go-serializer` - Pluggable serialization
- `github.com/MichaelAJay/go-metrics` - Enhanced metrics support
- `github.com/MichaelAJay/go-logger` - Structured logging

## License

[License information]

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Run tests: `make test-containers`
4. Commit your changes (`git commit -m 'Add amazing feature'`)
5. Push to the branch (`git push origin feature/amazing-feature`)
6. Open a Pull Request

## Performance

Benchmarks show excellent performance characteristics:
- Sub-millisecond operations for typical cache sizes
- Efficient batch operations with pipelining
- Minimal memory overhead with circuit breaker protection
- Lua script-based atomicity without coordination overhead

For detailed benchmarks, see the `*_benchmark_test.go` files.
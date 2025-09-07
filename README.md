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
    
    // Define key extractor
    extractor := &cache.IndexExtractor[User]{
        GetEntryKey: func(u User) string { return u.ID },
        GetOwnerKey: func(u User) string { return u.ID }, // Same as entry for simple caching
    }
    
    // Create cache instance (indexing disabled for simple use case)
    userCache, err := cache.NewCache[User](ctx, rdb, false, extractor)
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

// Create cache with indexing enabled
sessionCache, err := cache.NewCache[Session](
    ctx,
    rdb, // Redis client
    true, // Enable indexing
    extractor,
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
// Using functional options with constructor
cache, err := cache.NewCache[User](
    ctx,
    redisClient,
    false, // indexing mode
    extractor,
    cache.WithTTL(time.Hour),
    cache.WithSerializer("msgpack"), // json, gob, msgpack
    cache.WithMetrics(customMetrics),
    cache.WithHooks(&config.CacheHooks{
        PreSet: func(ctx context.Context, key string, value any) error {
            // Custom validation
            return nil
        },
        PostGet: func(ctx context.Context, key string, found bool, err error) {
            // Custom logging
        },
    }),
)
```

### Memory Tracking and Pressure Monitoring

Enable memory tracking to monitor cache memory usage and receive pressure alerts for capacity planning:

```go
// Enable memory tracking with custom thresholds
cache, err := cache.NewCache[User](
    ctx,
    redisClient,
    false,
    extractor,
    cache.WithMemoryTracking(true),                          // Enable memory tracking
    cache.WithMemoryUsageSamplingRate(50),                   // Sample every 50 operations
    cache.WithMemoryUsageSamplingInterval(30*time.Second),   // Background sampling every 30s
    cache.WithMemoryPressureThresholdBytes(100*1024*1024),   // Alert at 100MB
    cache.WithMemoryPressureThresholdPercent(75.0),          // Alert at 75% of Redis maxmemory
    cache.WithMetrics(customMetrics), // Required for memory metrics
)
```

**Memory Tracking Features:**
- **Hybrid Tracking**: Incremental tracking with periodic Redis MEMORY USAGE corrections for accuracy
- **Configurable Sampling**: Balance performance vs accuracy with operation-based and time-based sampling
- **Pressure Alerts**: Dual thresholds - absolute bytes and percentage of Redis maxmemory
- **Automatic Metrics**: Memory usage and pressure metrics recorded via enhanced metrics interface

**Memory Pressure Detection:**
```go
// Memory pressure alerts are automatically triggered when:
// 1. Cache memory usage exceeds MemoryPressureThresholdBytes (if > 0)
// 2. Cache memory usage exceeds MemoryPressureThresholdPercent of Redis maxmemory

// Metrics recorded:
// - MemoryUsage: Current cache memory usage in bytes and entry count
// - MemoryPressure: Boolean flag when thresholds are exceeded
```

**Performance Considerations:**
- Memory tracking adds minimal overhead when properly configured
- Default sampling rate (100 operations) balances accuracy with performance  
- Disabled by default - enable only when memory monitoring is needed

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

## Public Interface

### Core Interface

The cache implements the `Cache[T any]` interface with full generic type safety:

```go
type Cache[T any] interface {
    // Basic operations
    Set(ctx context.Context, value T, ttl time.Duration) error
    Get(ctx context.Context, key string) (value T, found bool, err error)
    Delete(ctx context.Context, key string) error
    Clear(ctx context.Context) error
    Has(ctx context.Context, key string) bool

    // Owner-based operations (requires indexing)
    GetByOwner(ctx context.Context, ownerKey string) ([]T, error)
    DeleteByOwner(ctx context.Context, ownerKey string) (deletedCount int, err error)

    // Atomic operations
    GetOrSet(ctx context.Context, key string, loader func(ctx context.Context) (T, error), ttl time.Duration) (T, error)
    Update(ctx context.Context, key string, updater func(old T, exists bool) (T, error), ttl time.Duration) (T, error)

    // Batch operations
    GetMany(ctx context.Context, keys []string) (map[string]T, error)
    SetMany(ctx context.Context, values []T, ttl time.Duration) error
    DeleteMany(ctx context.Context, keys []string) error

    // Conditional operations
    SetIfNotExists(ctx context.Context, value T, ttl time.Duration) (wasSet bool, err error)
    SetIfExists(ctx context.Context, value T, ttl time.Duration) (wasSet bool, err error)

    // Pattern operations
    GetKeysByPattern(ctx context.Context, pattern string) ([]string, error)

    // Counter operations
    Increment(ctx context.Context, key string, delta int64) (int64, error)
    Decrement(ctx context.Context, key string, delta int64) (int64, error)
    IncrementFloat(ctx context.Context, key string, delta float64) (float64, error)

    // Metadata operations
    GetMetadata(ctx context.Context, key string) (*CacheEntryMetadata, error)

    // Lifecycle management
    Close() error
}
```

### Constructor and Configuration

```go
// Create cache instance
func NewCache[T any](
    ctx context.Context, 
    client redis.Cmdable, 
    indexingMode bool, 
    extractor *IndexExtractor[T], 
    opts ...Option[T]
) (Cache[T], error)

// Index extractor for key extraction
type IndexExtractor[T any] struct {
    GetEntryKey func(T) string // Required: primary cache key
    GetOwnerKey func(T) string // Required for indexing: grouping key
}

// Configuration options (functional options pattern)
func WithRedisOptions[T any](redisOpts *RedisOptions) Option[T]
func WithTTL[T any](ttl time.Duration) Option[T]
func WithMetrics[T any](metrics metrics.EnhancedCacheMetrics) Option[T]
func WithHooks[T any](hooks *config.CacheHooks) Option[T]
func WithSerializer[T any](format string) Option[T] // "json", "gob", "msgpack"
func WithMaxEntries[T any](max int) Option[T]
func WithCleanupInterval[T any](interval time.Duration) Option[T]
func WithGoMetrics[T any](registry metric.Registry, tags metric.Tags) Option[T]
func WithVersion[T any](version string) Option[T]
func WithWarmLuaScripts[T any](warmScripts bool) Option[T]

// Memory tracking options (requires enhanced metrics)
func WithMemoryTracking[T any](enabled bool) Option[T]
func WithMemoryUsageSamplingRate[T any](rate int) Option[T]
func WithMemoryUsageSamplingInterval[T any](interval time.Duration) Option[T] 
func WithMemoryPressureThresholdBytes[T any](bytes int64) Option[T]
func WithMemoryPressureThresholdPercent[T any](percent float64) Option[T]

// Redis-specific options
type RedisOptions struct {
    DataPrefix  string // Prefix for data keys
    IndexPrefix string // Prefix for index keys  
    MetaPrefix  string // Prefix for metadata keys
    LockPrefix  string // Prefix for lock keys
    Version     string // Optional version suffix
}
```

### Configuration Builder (Chainable)

```go
// Create base configuration
opts := config.NewCacheOptions(redisClient)

// Chain configuration methods
opts = opts.
    WithTTL(time.Hour).
    WithSerializer("msgpack").
    WithMetrics(customMetrics).
    WithHooks(&config.CacheHooks{
        PreSet: func(ctx context.Context, key string, value any) error {
            return nil // Custom validation
        },
        PostGet: func(ctx context.Context, key string, found bool, err error) {
            // Custom logging
        },
    })
```

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

### Current Performance Characteristics

Benchmarks show excellent performance characteristics:
- Sub-millisecond operations for typical cache sizes
- Efficient batch operations with pipelining
- Minimal memory overhead with circuit breaker protection
- Lua script-based atomicity without coordination overhead

For detailed benchmarks, see the `*_benchmark_test.go` files.

### Recent Optimizations (September 2025)

**Major allocation optimizations completed** as part of the [ALLOCATION_OPTIMIZATION_PLAN.md](ALLOCATION_OPTIMIZATION_PLAN.md):

#### ✅ Phase 1: Comprehensive Allocation Benchmarking & Analysis
- **Baseline Establishment**: Complete allocation benchmark suite for all core operations
- **Analysis Tooling**: Automated allocation regression detection with configurable thresholds
- **Initial Metrics**: Has()=28 allocs, Delete()=51 allocs, Get()=58 allocs (hit)/43 allocs (miss), Set()=52 allocs, GetOrSet()=89 allocs (miss)/66 allocs (hit)

#### ✅ Phase 2: Pre-Computed Metrics Architecture (75% Allocation Reduction)
- **Zero-Allocation Metrics**: Implemented pre-computed metrics system with 87 individual metrics
- **🏆 Has() Method Optimization**: **28 → 7 allocations (75% reduction)**, **1256 B → 228 B (82% memory reduction)**
- **Boats Burned Approach**: Eliminated ALL legacy metrics fallback - GoMetricsRegistry is now **required**
- **Architecture**: All cache instances must provide metrics registry - no optional behavior, no legacy cruft

#### ✅ Phase 3: String Pool & Key Building Optimizations
- **Zero-Allocation Key Building**: Fast path achieves **2.06ns/op, 0 B/op, 0 allocs/op** for simple keys
- **Pooled Complex Keys**: **143ns/op, 128 B/op, 7 allocs/op** for keys with versions/prefixes using sync.Pool
- **Security Hardening**: Added sensitive data clearing in string pool operations
- **Circuit Breaker Optimization**: Already optimal at **0 allocations per call** across all scenarios

#### 🔄 In Progress: Core Operations Optimization
Currently extending the proven pre-computed metrics pattern to all core operations (Get, Set, Delete, GetOrSet) following the successful Has() optimization that achieved 75% allocation reduction.

**Performance Impact**: The `Has()` method serves as proof-of-concept for the optimization approach, demonstrating that **dramatic performance improvements are achievable** while maintaining full observability and functionality.
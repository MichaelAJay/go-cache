# Go Cache

A pluggable, high-performance caching abstraction for Go applications that provides unified interfaces for multiple cache providers with comprehensive observability, security features, and advanced functionality like secondary indexing.

## Features

### Core Features

- **Provider Abstraction**: Unified interface supporting memory, Redis, and extensible to other providers
- **Thread-Safe Operations**: All providers designed for high-concurrency usage
- **Context-Aware Operations**: Full context support for cancellation and timeouts
- **TTL (Time-To-Live) Support**: Automatic expiration with configurable cleanup
- **Bulk Operations**: Efficient batch get/set/delete operations

### Advanced Features

- **Secondary Indexing**: Support for complex queries beyond simple key-value operations
- **Atomic Operations**: Increment/decrement with conditional set operations
- **Pattern Operations**: Get/delete by key patterns with wildcard support
- **Metadata Operations**: Rich metadata tracking with access counts and timestamps
- **Cache Entry Lifecycle**: Update hooks and value transformation capabilities

### Security & Observability

- **Security-First Design**: Timing attack protection and secure cleanup
- **Comprehensive Metrics**: Detailed performance monitoring with Prometheus integration
- **Enhanced Logging**: Structured logging with operation tracing
- **Memory Safety**: Automatic cleanup with no memory leaks

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
)

func main() {
    // Create a cache manager
    manager := cache.NewCacheManager()

    // Register the memory cache provider
    manager.RegisterProvider("memory", cache.NewMemoryProvider())

    // Create a cache instance
    myCache, err := manager.GetCache("memory",
        cache.WithTTL(5*time.Minute),
        cache.WithMaxEntries(1000),
        cache.WithCleanupInterval(1*time.Minute),
    )
    if err != nil {
        panic(err)
    }
    defer myCache.Close()

    // Basic operations
    ctx := context.Background()

    // Set with custom TTL
    err = myCache.Set(ctx, "user:123", map[string]string{"name": "John"}, time.Hour)

    // Get value
    value, exists, err := myCache.Get(ctx, "user:123")
    if exists {
        user := value.(map[string]string)
        fmt.Printf("User: %s\n", user["name"])
    }

    // Bulk operations
    items := map[string]any{
        "session:1": "token1",
        "session:2": "token2",
    }
    err = myCache.SetMany(ctx, items, time.Hour)

    // Secondary indexing for complex queries
    err = myCache.AddIndex(ctx, "sessions_by_user", "session:*", "user:123")
    sessionKeys, err := myCache.GetByIndex(ctx, "sessions_by_user", "user:123")
}
```

## Cache Providers

### Memory Cache

The memory cache provider uses an in-memory map with advanced features like secondary indexing and efficient cleanup. Ideal for single-instance applications requiring high performance.

```go
import "github.com/MichaelAJay/go-cache"

// Register memory provider
manager.RegisterProvider("memory", cache.NewMemoryProvider())

// Create with advanced options
memCache, err := manager.GetCache("memory",
    cache.WithTTL(10*time.Minute),
    cache.WithMaxEntries(10000),
    cache.WithCleanupInterval(5*time.Minute),
    cache.WithSecurityConfig(&cache.SecurityConfig{
        EnableTimingProtection: true,
        MinProcessingTime:      5*time.Millisecond,
    }),
)
```

**Features:**

- Secondary indexing for complex queries
- Automatic cleanup with configurable intervals
- Memory-efficient storage with metadata tracking
- Thread-safe concurrent operations

### Redis Cache

The Redis cache provider offers distributed caching with persistence, clustering, and Lua script support. Perfect for multi-instance distributed applications.

```go
import "github.com/MichaelAJay/go-cache"

// Register Redis provider
manager.RegisterProvider("redis", cache.NewRedisProvider())

// Create with comprehensive options
redisCache, err := manager.GetCache("redis",
    cache.WithTTL(1*time.Hour),
    cache.WithRedisOptions(&cache.RedisOptions{
        Address:     "localhost:6379",
        Password:    "",
        DB:          0,
        PoolSize:    20,
        MaxRetries:  3,
        DialTimeout: 5*time.Second,
    }),
    // Optional serialization formats available
)
```

**Features:**

- Full Redis feature support (clustering, persistence, pub/sub)
- Connection pooling with automatic failover
- Lua script execution for atomic operations
- Multiple serialization formats (JSON, MessagePack, Gob)
- Pipeline operations for batch processing

See the [Redis Provider Documentation](https://godoc.org/github.com/MichaelAJay/go-cache#RedisProvider) for detailed examples and advanced configuration.

## Configuration Options

### Core Options

- `WithTTL(duration)`: Set default TTL for cache entries
- `WithMaxEntries(int)`: Maximum entries (memory cache)
- `WithCleanupInterval(duration)`: Cleanup interval for expired entries
- `WithLogger(logger)`: Custom logger instance
- `WithMetrics(metrics)`: Custom metrics collector

### Provider-Specific Options

- `WithRedisOptions(*RedisOptions)`: Redis connection and pool settings

### Security & Advanced Options

- `WithSecurityConfig(*SecurityConfig)`: Enable timing protection and secure cleanup
- `WithHooks(*CacheHooks)`: Pre/post operation hooks for validation
- `WithIndexes(map[string]string)`: Pre-configure secondary indexes

### Metrics & Observability Options

- `WithGoMetricsRegistry(gometrics.Registry)`: Set go-metrics registry for comprehensive metrics
- `WithMetricsEnabled(bool)`: Enable/disable metrics collection
- `WithGlobalMetricsTags(gometrics.Tags)`: Global tags applied to all metrics
- `WithPrometheusMetrics(registry, tags)`: Configure Prometheus metrics
- `WithOpenTelemetryMetrics(registry, tags)`: Configure OpenTelemetry metrics
- `WithServiceTags(serviceName, version, environment)`: Add standard service tags
- `WithDetailedMetrics(bool)`: Enable comprehensive metrics collection
- `WithMetricsPrefix(string)`: Set prefix for all metric names

### Example: Full Configuration

```go
import (
    gometrics "github.com/MichaelAJay/go-metrics"
    "github.com/MichaelAJay/go-cache"
)

// Create go-metrics registry
registry := gometrics.NewRegistry()

cache, err := manager.GetCache("sessions",
    // Core settings
    cache.WithTTL(24*time.Hour),
    cache.WithMaxEntries(100000),
    cache.WithCleanupInterval(10*time.Minute),

    // Security
    cache.WithSecurityConfig(&cache.SecurityConfig{
        EnableTimingProtection: true,
        MinProcessingTime:      5*time.Millisecond,
        SecureCleanup:         true,
    }),

    // Pre-configure indexes
    cache.WithIndexes(map[string]string{
        "sessions_by_user": "session:*",
        "admin_sessions":   "admin:*",
    }),

    // Comprehensive metrics configuration
    cache.WithGoMetricsRegistry(registry),
    cache.WithGlobalMetricsTags(gometrics.Tags{
        "service":     "app_cache",
        "version":     "1.0.0",
        "environment": "production",
        "node_id":     "cache-node-1",
    }),
    cache.WithDetailedMetrics(true),
    cache.WithMetricsPrefix("myapp_"),
    cache.WithLogger(logger),
)
```

## Advanced Usage

### Secondary Indexing

Build complex queries beyond simple key-value operations:

```go
// Add indexes for efficient queries
err := cache.AddIndex(ctx, "users_by_role", "user:*", "admin")
err = cache.AddIndex(ctx, "users_by_role", "user:*", "user")

// Query by index
adminUsers, err := cache.GetByIndex(ctx, "users_by_role", "admin")

// Bulk delete by index
err = cache.DeleteByIndex(ctx, "users_by_role", "admin")
```

### Atomic Operations

Thread-safe counter operations:

```go
// Increment counter
count, err := cache.Increment(ctx, "page_views", 1, time.Hour)

// Conditional operations
success, err := cache.SetIfNotExists(ctx, "lock:resource", "locked", time.Minute)
```

### Pattern Operations

Work with multiple keys using patterns:

```go
// Get all session keys
sessionKeys, err := cache.GetKeysByPattern(ctx, "session:*")

// Bulk delete by pattern
deletedCount, err := cache.DeleteByPattern(ctx, "temp:*")
```

### Middleware

Chain middleware for enhanced functionality:

```go
import (
    gometrics "github.com/MichaelAJay/go-metrics"
    "github.com/MichaelAJay/go-cache/middleware"
)

// Create base cache
baseCache, _ := manager.GetCache("redis", options...)

// Add metrics middleware
registry := gometrics.NewRegistry()
cacheWithMetrics := middleware.NewMetricsMiddleware(registry, gometrics.Tags{
    "service": "myapp_cache",
})(baseCache)

// Add logging middleware
finalCache := middleware.NewLoggingMiddleware(logger)(cacheWithMetrics)
```

## Examples

See the `cmd/examples/` directory for complete examples:

- **`simple/`**: Basic cache operations and setup
- **`redis/`**: Redis provider configuration and usage examples
- **`redis_msgpack_example/`**: Redis with MessagePack serialization
- **`redis_examples/`**: Advanced Redis features (pipelines, Lua scripts)
- **`prometheus/`**: Metrics collection with Prometheus integration using go-metrics
- **`comprehensive_metrics/`**: Advanced metrics collection examples
- **`opentelemetry/`**: OpenTelemetry integration examples

### Example: Session Management with Secondary Indexing

```go
// Real-world example: session management for web applications
type SessionManager struct {
    cache cache.Cache
}

func NewSessionManager() *SessionManager {
    manager := cache.NewCacheManager()
    manager.RegisterProvider("redis", cache.NewRedisProvider())

    sessionCache, _ := manager.GetCache("redis",
        cache.WithTTL(24*time.Hour),
        cache.WithRedisOptions(&cache.RedisOptions{
            Address:  "localhost:6379",
            PoolSize: 20,
        }),
        cache.WithSecurity(&cache.SecurityConfig{
            EnableTimingProtection: true,
            SecureCleanup:         true,
        }),
    )

    return &SessionManager{cache: sessionCache}
}

func (sm *SessionManager) CreateSession(ctx context.Context, userID string, data map[string]any) (string, error) {
    sessionID := generateSessionID()
    sessionKey := fmt.Sprintf("session:%s", sessionID)

    // Store session data
    err := sm.cache.Set(ctx, sessionKey, data, 0) // Use default TTL
    if err != nil {
        return "", err
    }

    // Add to user index for efficient "get all user sessions" queries
    err = sm.cache.AddIndex(ctx, "sessions_by_user", "session:*", userID)
    if err != nil {
        return "", err
    }

    return sessionID, nil
}

func (sm *SessionManager) GetUserSessions(ctx context.Context, userID string) ([]map[string]any, error) {
    // Get all session keys for user
    sessionKeys, err := sm.cache.GetByIndex(ctx, "sessions_by_user", userID)
    if err != nil {
        return nil, err
    }

    // Bulk fetch session data
    sessions, err := sm.cache.GetMany(ctx, sessionKeys)
    if err != nil {
        return nil, err
    }

    var result []map[string]any
    for _, session := range sessions {
        result = append(result, session.(map[string]any))
    }

    return result, nil
}

func (sm *SessionManager) InvalidateUserSessions(ctx context.Context, userID string) error {
    // Efficiently delete all sessions for a user
    return sm.cache.DeleteByIndex(ctx, "sessions_by_user", userID)
}
```

## Architecture & Integration

### Integration with go-auth

This cache module is designed as the foundational storage layer for the [go-auth](https://github.com/MichaelAJay/go-auth) authentication system:

- **Session Storage**: Primary storage for authentication sessions
- **User Indexing**: Efficient user-to-sessions mapping via secondary indexes
- **Security Features**: Timing attack protection for session validation
- **Automatic Cleanup**: TTL-based session expiration with secure memory cleanup
- **Observability**: Comprehensive metrics for session management operations

### Design Philosophy

- **Interface-First**: Clean abstractions that hide provider complexity
- **Security-Conscious**: Built-in protections against common attack vectors
- **Performance-Optimized**: Efficient algorithms with minimal overhead
- **Observability-Ready**: Comprehensive metrics and logging throughout
- **Extensible**: Easy to add new providers, serializers, and features

## License

This project is licensed under the MIT License - see the LICENSE file for details.

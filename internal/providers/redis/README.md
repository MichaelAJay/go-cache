# Redis Cache Provider

This package implements a Redis-based cache provider for the `go-cache` library. It provides a distributed caching solution using Redis as the backend storage.

## Features

- Full implementation of the `Cache` interface
- Default MessagePack serialization for optimal performance
- Support for TTL (Time-to-Live)
- Metadata tracking for cache entries
- Connection pooling for efficient Redis connections
- Comprehensive metrics collection
- Support for bulk operations (GetMany, SetMany, DeleteMany)

## Installation

```bash
go get github.com/MichaelAJay/go-cache
```

## Usage

### Basic Usage

```go
package main

import (
	"context"
	"fmt"
	"time"
	
	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/providers/redis"
	"github.com/MichaelAJay/go-serializer"
)

func main() {
	// Create cache options
	options := &cache.CacheOptions{
		TTL:              time.Hour,
		SerializerFormat: serializer.Msgpack, // Use MessagePack for better performance
		RedisOptions: &cache.RedisOptions{
			Address:  "localhost:6379",
			Password: "",
			DB:       0,
			PoolSize: 10,
		},
	}
	
	// Create provider
	provider := redis.NewProvider()
	
	// Create cache
	redisCache, err := provider.Create(options)
	if err != nil {
		panic(err)
	}
	defer redisCache.Close()
	
	// Use the cache
	ctx := context.Background()
	
	// Set a value
	err = redisCache.Set(ctx, "greeting", "Hello, World!", time.Minute*5)
	if err != nil {
		panic(err)
	}
	
	// Get a value
	value, found, err := redisCache.Get(ctx, "greeting")
	if err != nil {
		panic(err)
	}
	
	if found {
		fmt.Println("Value:", value)
	} else {
		fmt.Println("Value not found")
	}
}
```

### Storing Custom Structs with MessagePack

The Redis cache provider uses MessagePack serialization by default for better performance. When storing custom structs, adding MessagePack tags is recommended:

```go
type User struct {
	ID        int       `json:"id" msgpack:"id"`
	Name      string    `json:"name" msgpack:"name"`
	Email     string    `json:"email" msgpack:"email"`
	CreatedAt time.Time `json:"created_at" msgpack:"created_at"`
	Roles     []string  `json:"roles" msgpack:"roles"`
}

// Store a user
user := &User{
	ID:        1,
	Name:      "John Doe",
	Email:     "john@example.com",
	CreatedAt: time.Now(),
	Roles:     []string{"admin", "user"},
}

err = redisCache.Set(ctx, "user:1", user, time.Hour)

// Retrieve the user
data, found, err := redisCache.Get(ctx, "user:1")
if found {
	// MessagePack deserializes to map[string]any by default
	// You may need to convert back to your struct if needed
	userData := data.(map[string]any)
	fmt.Printf("User: %s (%s)\n", userData["name"], userData["email"])
}
```

### Bulk Operations

The Redis cache provider supports efficient bulk operations:

```go
// Store multiple values
items := map[string]any{
	"key1": "value1",
	"key2": "value2",
	"key3": "value3",
}
err = redisCache.SetMany(ctx, items, time.Hour)

// Retrieve multiple values
keys := []string{"key1", "key2", "key3"}
values, err := redisCache.GetMany(ctx, keys)

// Delete multiple values
err = redisCache.DeleteMany(ctx, keys)
```

### Configuration from Environment Variables

The Redis provider can load configuration from environment variables:

```go
// Environment variables:
// - REDIS_ADDR: Redis server address (default: 127.0.0.1:6379)
// - REDIS_PASSWORD: Redis password (default: empty)
// - REDIS_DB: Redis database number (default: 0)
// - REDIS_POOL_SIZE: Connection pool size (default: 10)

// Create options with nil RedisOptions to load from environment
options := &cache.CacheOptions{
	TTL:              time.Hour,
	SerializerFormat: serializer.Msgpack,
	RedisOptions:     nil, // Will load from environment
}

provider := redis.NewProvider()
redisCache, err := provider.Create(options)
```

### Testing with the Redis Provider

When testing applications that use the Redis cache, you can follow these patterns:

```go
package myapp_test

import (
    "context"
    "testing"
    "time"

    "github.com/MichaelAJay/go-cache"
    "github.com/MichaelAJay/go-cache/providers/redis"
)

// setupRedisCache creates a cache for testing, with Redis connection checks
func setupRedisCache(t *testing.T) (cache.Cache, func()) {
    t.Helper()

    // Check if Redis is available
    rClient := goredis.NewClient(&goredis.Options{
        Addr: "localhost:6379",
    })
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    if err := rClient.Ping(ctx).Err(); err != nil {
        t.Skipf("Skipping test: Cannot connect to Redis: %v", err)
    }
    rClient.Close()

    // Create a unique test prefix to avoid collisions between tests
    testID := fmt.Sprintf("test:%s:", time.Now().Format("20060102150405"))

    // Create the cache for this test
    provider := redis.NewProvider()
    c, err := provider.Create(&cache.CacheOptions{
        TTL: time.Minute,
        RedisOptions: &cache.RedisOptions{
            Address: "localhost:6379",
        },
    })
    if err != nil {
        t.Fatalf("Failed to create cache: %v", err)
    }

    // Return the cache and a cleanup function
    cleanup := func() {
        ctx := context.Background()
        if err := c.Clear(ctx); err != nil {
            t.Logf("Warning: Failed to clear cache during cleanup: %v", err)
        }
        c.Close()
    }

    return c, cleanup
}

func TestWithRedisCache(t *testing.T) {
    // Set up the cache with automatic cleanup
    cache, cleanup := setupRedisCache(t)
    defer cleanup()

    // Run the test
    ctx := context.Background()
    err := cache.Set(ctx, "test-key", "test-value", time.Minute)
    if err != nil {
        t.Fatalf("Failed to set value: %v", err)
    }

    value, exists, err := cache.Get(ctx, "test-key")
    if err != nil {
        t.Fatalf("Failed to get value: %v", err)
    }
    if !exists {
        t.Fatal("Value should exist")
    }
    if value != "test-value" {
        t.Fatalf("Expected 'test-value', got '%v'", value)
    }
}
```

## Examples in Repository

The repository contains several working examples that demonstrate Redis cache usage:

1. **Basic Redis Cache Usage**: 
   - File: `/examples/redis_cache.go`
   - Features: Basic operations, environment-based configuration

2. **Redis with MessagePack Serialization**:
   - Directory: `/examples/redis_msgpack_example/`
   - Features: Using MessagePack serialization, storing and retrieving custom structures

3. **Advanced Redis Examples**:
   - Directory: `/examples/redis_examples/`
   - Features: Pipeline operations and other advanced features

To run an example:

```bash
# Basic example
go run examples/redis_cache.go

# MessagePack example
go run examples/redis_msgpack_example/main.go
```

## MessagePack vs JSON Serialization

This provider uses MessagePack serialization by default for better performance, but you can switch to JSON if needed:

```go
// Use JSON serialization instead
options := &cache.CacheOptions{
	SerializerFormat: serializer.JSON,
	// other options...
}
```

### Performance Comparison

MessagePack generally offers several advantages over JSON for caching:

1. **Smaller size**: MessagePack typically produces 20-30% smaller payloads than JSON
2. **Faster serialization/deserialization**: MessagePack is more efficient to encode/decode
3. **Better type preservation**: MessagePack preserves numeric types (int vs float)

For Redis caching, these benefits translate to:
- Reduced network bandwidth
- Lower Redis memory usage
- Faster overall cache operations

## Advanced Features

### Metadata Tracking

The Redis cache tracks metadata for each cache entry:

```go
metadata, err := redisCache.GetMetadata(ctx, "user:1")
if err == nil {
	fmt.Printf("Created: %v\n", metadata.CreatedAt)
	fmt.Printf("Last Accessed: %v\n", metadata.LastAccessed)
	fmt.Printf("Access Count: %d\n", metadata.AccessCount)
	fmt.Printf("TTL: %v\n", metadata.TTL)
	fmt.Printf("Size: %d bytes\n", metadata.Size)
}
```

### Cache Metrics

The Redis cache provider collects performance metrics:

```go
metrics := redisCache.GetMetrics()
fmt.Printf("Hits: %d\n", metrics.Hits)
fmt.Printf("Misses: %d\n", metrics.Misses)
fmt.Printf("Hit Ratio: %.2f\n", metrics.HitRatio)
fmt.Printf("Get Latency: %v\n", metrics.GetLatency)
fmt.Printf("Set Latency: %v\n", metrics.SetLatency)
fmt.Printf("Delete Latency: %v\n", metrics.DeleteLatency)
fmt.Printf("Cache Size: %d bytes\n", metrics.CacheSize)
fmt.Printf("Entry Count: %d\n", metrics.EntryCount)
```

## Implementation Details

### Key Formatting

All keys are prefixed with `cache:` to avoid collisions with other data in the Redis database:

```go
// Internal key format: cache:your-key
// Metadata key format: cache:meta:your-key
```

### Thread Safety

All operations are thread-safe and can be called concurrently from multiple goroutines.

### Error Handling

The Redis cache provider implements comprehensive error handling:

- Redis connection errors
- Serialization/deserialization errors
- Context cancellation handling
- Redis command errors

## Best Practices

1. **Use MessagePack serialization**: It's faster and more efficient for most use cases
2. **Set appropriate TTL values**: Avoid indefinite caching unless necessary
3. **Consider connection pooling**: Adjust `PoolSize` based on your application needs
4. **Monitor cache metrics**: Use `GetMetrics()` to optimize your cache usage
5. **Use bulk operations**: `GetMany`/`SetMany`/`DeleteMany` for better performance
6. **Handle errors properly**: Check for errors on all operations 
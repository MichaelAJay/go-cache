package testintegration

import (
	"context"
	"time"

	cache "github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/redis/go-redis/v9"
)

const (
	serializer = "msgpack"
)

// TestSession represents a simple test data structure for cache testing
type TestSession struct {
	ID       string
	UserID   string
	Username string
	Created  time.Time
}

// RotateTestSession represents a test session structure with fields compatible with RotateKey
// This matches the expected fields from go-auth session rotation use case
type RotateTestSession struct {
	ID           string `msgpack:"id"`
	UserID       string `msgpack:"user_id"`
	Username     string `msgpack:"username"`
	Created      int64  `msgpack:"created_at"`
	ExpiresAt    int64  `msgpack:"expires_at"`
	LastActivity int64  `msgpack:"last_activity"`
}

// TestSessionExtractor provides key extraction for TestSession
var TestSessionExtractor = &cache.IndexExtractor[*TestSession]{
	GetEntryKey: func(session *TestSession) string {
		return session.ID
	},
	GetOwnerKey: func(session *TestSession) string {
		return session.UserID
	},
}

// RotateTestSessionExtractor provides key extraction for RotateTestSession
var RotateTestSessionExtractor = &cache.IndexExtractor[*RotateTestSession]{
	GetEntryKey: func(session *RotateTestSession) string {
		return session.ID
	},
	GetOwnerKey: func(session *RotateTestSession) string {
		return session.UserID
	},
}

// CacheConfig represents configuration options for test cache creation
type CacheConfig struct {
	IndexingMode     bool
	SerializerFormat string
	WarmLuaScripts   bool
	TTL              time.Duration
}

// DefaultCacheConfig returns a basic cache configuration for testing
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		IndexingMode:     false,
		SerializerFormat: serializer,
		WarmLuaScripts:   true,
		TTL:              5 * time.Minute,
	}
}

// IndexedCacheConfig returns a cache configuration with indexing enabled
func IndexedCacheConfig() *CacheConfig {
	return &CacheConfig{
		IndexingMode:     true,
		SerializerFormat: serializer,
		WarmLuaScripts:   true,
		TTL:              5 * time.Minute,
	}
}

// CreateTestSessionCache creates a RedisCache[*TestSession] with the specified configuration
func CreateTestSessionCache(ctx context.Context, client redis.Cmdable, config *CacheConfig) (interfaces.Cache[*TestSession], error) {
	opts := []cache.Option[*TestSession]{
		cache.WithTTL[*TestSession](config.TTL),
		cache.WithSerializer[*TestSession](config.SerializerFormat),
		cache.WithWarmLuaScripts[*TestSession](config.WarmLuaScripts),
	}

	return cache.NewCache(ctx, client, config.IndexingMode, TestSessionExtractor, 128, opts...)
}

// CreateRotateTestSessionCache creates a RedisCache[*RotateTestSession] for RotateKey testing
func CreateRotateTestSessionCache(ctx context.Context, client redis.Cmdable, config *CacheConfig) (interfaces.Cache[*RotateTestSession], error) {
	opts := []cache.Option[*RotateTestSession]{
		cache.WithTTL[*RotateTestSession](config.TTL),
		cache.WithSerializer[*RotateTestSession](config.SerializerFormat),
		cache.WithWarmLuaScripts[*RotateTestSession](config.WarmLuaScripts),
	}

	return cache.NewCache(ctx, client, config.IndexingMode, RotateTestSessionExtractor, 128, opts...)
}

// CreateStringCache creates a simple string-based cache for basic testing
func CreateStringCache(ctx context.Context, client redis.Cmdable, config *CacheConfig) (interfaces.Cache[string], error) {
	stringExtractor := &cache.IndexExtractor[string]{
		GetEntryKey: func(value string) string {
			return value // Use the string value itself as the key
		},
		GetOwnerKey: func(value string) string {
			return "default" // All strings belong to "default" owner
		},
	}

	opts := []cache.Option[string]{
		cache.WithTTL[string](config.TTL),
		cache.WithSerializer[string](config.SerializerFormat),
	}

	// Note: WarmLuaScripts is handled internally by RedisCache during initialization

	return cache.NewCache(ctx, client, config.IndexingMode, stringExtractor, 0, opts...)
}

// CreateTestCounterCache creates a RedisCache[int64] for counter testing
func CreateTestCounterCache(ctx context.Context, client redis.Cmdable, config *CacheConfig) (interfaces.Cache[int64], error) {
	counterExtractor := &cache.IndexExtractor[int64]{
		GetEntryKey: func(value int64) string {
			return "counter" // Simple key since counters are accessed by explicit keys
		},
		GetOwnerKey: func(value int64) string {
			return "default" // All counters belong to "default" owner for simplicity
		},
	}

	opts := []cache.Option[int64]{
		cache.WithTTL[int64](config.TTL),
		cache.WithSerializer[int64](config.SerializerFormat),
	}

	// Note: WarmLuaScripts is handled internally by RedisCache during initialization

	return cache.NewCache(ctx, client, config.IndexingMode, counterExtractor, 0, opts...)
}

// CreateTestFloatCounterCache creates a RedisCache[float64] for float counter testing
func CreateTestFloatCounterCache(ctx context.Context, client redis.Cmdable, config *CacheConfig) (interfaces.Cache[float64], error) {
	floatCounterExtractor := &cache.IndexExtractor[float64]{
		GetEntryKey: func(value float64) string {
			return "float_counter" // Simple key since counters are accessed by explicit keys
		},
		GetOwnerKey: func(value float64) string {
			return "default" // All float counters belong to "default" owner for simplicity
		},
	}

	opts := []cache.Option[float64]{
		cache.WithTTL[float64](config.TTL),
		cache.WithSerializer[float64](config.SerializerFormat),
	}

	// Note: WarmLuaScripts is handled internally by RedisCache during initialization

	return cache.NewCache(ctx, client, config.IndexingMode, floatCounterExtractor, 0, opts...)
}

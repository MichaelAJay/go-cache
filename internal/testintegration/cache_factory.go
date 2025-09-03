package testintegration

import (
	"context"
	"time"

	cache "github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/go-redis/redis/v8"
)

// TestSession represents a simple test data structure for cache testing
type TestSession struct {
	ID       string
	UserID   string
	Username string
	Created  time.Time
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
		SerializerFormat: "msgpack",
		WarmLuaScripts:   true,
		TTL:              5 * time.Minute,
	}
}

// IndexedCacheConfig returns a cache configuration with indexing enabled
func IndexedCacheConfig() *CacheConfig {
	return &CacheConfig{
		IndexingMode:     true,
		SerializerFormat: "msgpack",
		WarmLuaScripts:   true,
		TTL:              5 * time.Minute,
	}
}

// CreateTestSessionCache creates a RedisCache[*TestSession] with the specified configuration
func CreateTestSessionCache(ctx context.Context, client redis.Cmdable, config *CacheConfig) (interfaces.Cache[*TestSession], error) {
	opts := []cache.Option[*TestSession]{
		cache.WithTTL[*TestSession](config.TTL),
		cache.WithSerializer[*TestSession](config.SerializerFormat),
	}

	// Note: WarmLuaScripts is handled internally by RedisCache during initialization
	// We don't need to explicitly set it via options for now

	return cache.NewCache(ctx, client, config.IndexingMode, TestSessionExtractor, opts...)
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

	return cache.NewCache(ctx, client, config.IndexingMode, stringExtractor, opts...)
}

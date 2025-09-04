package config

import (
	"context"
	"time"

	"github.com/MichaelAJay/go-cache/metrics"
	"github.com/MichaelAJay/go-metrics/metric"
	"github.com/redis/go-redis/v9"
)

// CacheOptions contains configuration settings for Redis-only cache implementation.
type CacheOptions struct {
	// Core Redis settings
	RedisClient redis.Cmdable // Injected Redis client

	// Cache behavior
	DefaultTTL      time.Duration // Default TTL for entries (0 = no expiration)
	MaxEntries      int           // Maximum number of entries (0 = no limit)
	CleanupInterval time.Duration // How often to clean expired entries

	// Serialization
	SerializerFormat string // "json", "gob", "msgpack"

	// Enterprise features
	EnhancedMetrics   metrics.EnhancedCacheMetrics // Custom metrics implementation
	GoMetricsRegistry metric.Registry              // go-metrics registry for built-in metrics
	GlobalMetricsTags metric.Tags                  // Tags applied to all metrics
	Hooks             *CacheHooks                  // Lifecycle hooks for custom behavior

	WarmLuaScripts bool
}

// CacheHooks provides lifecycle hooks for extending cache behavior
type CacheHooks struct {
	// Pre-operation hooks (can prevent operation by returning error)
	PreGet    func(ctx context.Context, key string) error
	PreSet    func(ctx context.Context, key string, value any) error
	PreDelete func(ctx context.Context, key string) error

	// Post-operation hooks (for logging, metrics, notifications)
	PostGet    func(ctx context.Context, key string, found bool, err error)
	PostSet    func(ctx context.Context, key string, value any, err error)
	PostDelete func(ctx context.Context, key string, deleted bool, err error)
}

// NewCacheOptions creates cache options with the provided Redis client and sensible defaults
func NewCacheOptions(redisClient redis.Cmdable) *CacheOptions {
	return &CacheOptions{
		RedisClient:       redisClient,
		DefaultTTL:        0, // No expiration by default
		MaxEntries:        0, // No limit by default
		CleanupInterval:   5 * time.Minute,
		SerializerFormat:  "msgpack", // Optimal for Redis - compact, cross-language
		GlobalMetricsTags: make(metric.Tags),
	}
}

// DefaultOptions returns sensible defaults for cache options
func DefaultOptions() *CacheOptions {
	return &CacheOptions{
		DefaultTTL:        0, // No expiration by default
		MaxEntries:        0, // No limit by default
		CleanupInterval:   5 * time.Minute,
		SerializerFormat:  "msgpack", // Optimal for Redis - compact, cross-language
		GlobalMetricsTags: make(metric.Tags),
	}
}

// WithTTL sets the default TTL for cache entries
func (o *CacheOptions) WithTTL(ttl time.Duration) *CacheOptions {
	o.DefaultTTL = ttl
	return o
}

// WithMaxEntries sets the maximum number of cache entries
func (o *CacheOptions) WithMaxEntries(max int) *CacheOptions {
	o.MaxEntries = max
	return o
}

// WithCleanupInterval sets how often expired entries are cleaned
func (o *CacheOptions) WithCleanupInterval(interval time.Duration) *CacheOptions {
	o.CleanupInterval = interval
	return o
}

// WithMetrics sets custom metrics implementation
func (o *CacheOptions) WithMetrics(metrics metrics.EnhancedCacheMetrics) *CacheOptions {
	o.EnhancedMetrics = metrics
	return o
}

// WithGoMetrics sets go-metrics registry for built-in metrics
func (o *CacheOptions) WithGoMetrics(registry metric.Registry, tags metric.Tags) *CacheOptions {
	o.GoMetricsRegistry = registry
	o.GlobalMetricsTags = tags
	return o
}

// WithHooks sets lifecycle hooks
func (o *CacheOptions) WithHooks(hooks *CacheHooks) *CacheOptions {
	o.Hooks = hooks
	return o
}

// WithSerializer sets serialization format (for Redis)
func (o *CacheOptions) WithSerializer(format string) *CacheOptions {
	o.SerializerFormat = format
	return o
}

// WithRedisClient sets the Redis client (required for cache creation)
func (o *CacheOptions) WithRedisClient(client redis.Cmdable) *CacheOptions {
	o.RedisClient = client
	return o
}

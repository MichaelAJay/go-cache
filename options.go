package cache

import (
	"context"
	"time"

	"github.com/MichaelAJay/go-logger"
	"github.com/MichaelAJay/go-serializer"
)

// CacheOption defines a function type for configuring cache options
type CacheOption func(*CacheOptions)

// WithTTL sets the default time-to-live for cache entries
func WithTTL(ttl time.Duration) CacheOption {
	return func(o *CacheOptions) {
		o.TTL = ttl
	}
}

// WithMaxEntries sets the maximum number of entries for the cache
func WithMaxEntries(max int) CacheOption {
	return func(o *CacheOptions) {
		o.MaxEntries = max
	}
}

// WithCleanupInterval sets the interval for cleaning up expired entries
func WithCleanupInterval(interval time.Duration) CacheOption {
	return func(o *CacheOptions) {
		o.CleanupInterval = interval
	}
}

// WithLogger sets the logger for the cache
func WithLogger(logger logger.Logger) CacheOption {
	return func(o *CacheOptions) {
		o.Logger = logger
	}
}

// WithRedisOptions sets the Redis-specific options
func WithRedisOptions(options *RedisOptions) CacheOption {
	return func(o *CacheOptions) {
		o.RedisOptions = options
	}
}

// WithMaxSize sets the maximum size in bytes for the cache
func WithMaxSize(maxSize int64) CacheOption {
	return func(o *CacheOptions) {
		o.MaxSize = maxSize
	}
}

// Configuration structs

// SecurityConfig defines security settings for cache operations
type SecurityConfig struct {
	EnableTimingProtection bool          // Enable consistent response times
	MinProcessingTime      time.Duration // Minimum time for operations (for timing protection)
	SecureCleanup          bool          // Enable secure memory wiping during cleanup
}

// CacheHooks defines lifecycle hooks for cache operations
type CacheHooks struct {
	PreGet     func(ctx context.Context, key string) error
	PostGet    func(ctx context.Context, key string, value any, found bool, err error)
	PreSet     func(ctx context.Context, key string, value any, ttl time.Duration) error
	PostSet    func(ctx context.Context, key string, value any, ttl time.Duration, err error)
	PreDelete  func(ctx context.Context, key string) error
	PostDelete func(ctx context.Context, key string, existed bool, err error)
	OnCleanup  func(ctx context.Context, key string, reason CleanupReason)
}

// CleanupConfig defines enhanced cleanup configuration
type CleanupConfig struct {
	Interval          time.Duration                                     // Cleanup interval
	BatchSize         int                                               // Maximum number of entries to clean per batch
	MaxCleanupTime    time.Duration                                     // Maximum time to spend on cleanup per cycle
	CustomCleanupFunc func(key string, entry *CacheEntryMetadata) bool // Custom cleanup logic
}

// MetricsConfig defines enhanced metrics configuration
type MetricsConfig struct {
	EnableDetailedMetrics   bool   // Enable detailed operation metrics
	EnableSecurityMetrics   bool   // Enable security-related metrics
	EnableLatencyHistograms bool   // Enable latency percentile tracking
	MetricsPrefix           string // Prefix for metric names
}

// CacheOptions represents configuration options for cache instances
type CacheOptions struct {
	TTL              time.Duration
	MaxEntries       int
	MaxSize          int64
	CleanupInterval  time.Duration
	Logger           logger.Logger
	RedisOptions     *RedisOptions
	SerializerFormat serializer.Format // Format to use for serialization
	Metrics          CacheMetrics      // Custom metrics implementation

	// Enhanced configuration options
	Security        *SecurityConfig           // Security configuration
	Hooks           *CacheHooks               // Lifecycle hooks
	Indexes         map[string]string         // indexName -> keyPattern for secondary indexes
	CleanupConfig   *CleanupConfig            // Enhanced cleanup configuration
	MetricsConfig   *MetricsConfig            // Enhanced metrics configuration
	EnhancedMetrics EnhancedCacheMetrics      // Enhanced metrics implementation
}

// RedisOptions represents configuration options for Redis cache
type RedisOptions struct {
	Address  string
	Password string
	DB       int
	PoolSize int
}

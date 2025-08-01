package interfaces

import (
	"context"
	"time"

	"github.com/MichaelAJay/go-cache/metrics"
	"github.com/MichaelAJay/go-logger"
	"github.com/MichaelAJay/go-metrics/metric"
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

// WithGoMetricsRegistry sets the go-metrics registry for comprehensive metrics
func WithGoMetricsRegistry(registry metric.Registry) CacheOption {
	return func(o *CacheOptions) {
		o.GoMetricsRegistry = registry
		o.EnhancedMetrics = metrics.NewEnhancedCacheMetrics(registry, o.GlobalMetricsTags)
	}
}

// WithMetricsEnabled enables or disables metrics collection
func WithMetricsEnabled(enabled bool) CacheOption {
	return func(o *CacheOptions) {
		o.MetricsEnabled = enabled
		if !enabled {
			o.GoMetricsRegistry = metric.NewNoop()
			o.EnhancedMetrics = metrics.NewNoopEnhancedCacheMetrics()
		}
	}
}

// WithGlobalMetricsTags sets global tags that will be applied to all metrics
func WithGlobalMetricsTags(tags metric.Tags) CacheOption {
	return func(o *CacheOptions) {
		o.GlobalMetricsTags = tags
	}
}

// WithSecurityConfig sets the security configuration
func WithSecurityConfig(config *SecurityConfig) CacheOption {
	return func(o *CacheOptions) {
		o.Security = config
	}
}

// WithHooks sets the lifecycle hooks
func WithHooks(hooks *CacheHooks) CacheOption {
	return func(o *CacheOptions) {
		o.Hooks = hooks
	}
}

// WithIndexes sets the secondary indexes
func WithIndexes(indexes map[string]string) CacheOption {
	return func(o *CacheOptions) {
		o.Indexes = indexes
	}
}

// WithCleanupConfig sets enhanced cleanup configuration
func WithCleanupConfig(config *CleanupConfig) CacheOption {
	return func(o *CacheOptions) {
		o.CleanupConfig = config
	}
}

// WithMetricsConfig sets enhanced metrics configuration
func WithMetricsConfig(config *MetricsConfig) CacheOption {
	return func(o *CacheOptions) {
		o.MetricsConfig = config
	}
}

// Advanced metrics configuration helpers

// WithPrometheusMetrics configures the cache to use Prometheus metrics
func WithPrometheusMetrics(registry metric.Registry, tags metric.Tags) CacheOption {
	return func(o *CacheOptions) {
		if registry == nil {
			registry = metric.NewDefaultRegistry()
		}
		o.GoMetricsRegistry = registry
		o.GlobalMetricsTags = tags
		o.MetricsEnabled = true
	}
}

// WithOpenTelemetryMetrics configures the cache to use OpenTelemetry metrics
func WithOpenTelemetryMetrics(registry metric.Registry, tags metric.Tags) CacheOption {
	return func(o *CacheOptions) {
		if registry == nil {
			registry = metric.NewDefaultRegistry()
		}
		o.GoMetricsRegistry = registry
		o.GlobalMetricsTags = tags
		o.MetricsEnabled = true
	}
}

// WithServiceTags adds standard service tags to metrics
func WithServiceTags(serviceName, version, environment string) CacheOption {
	return func(o *CacheOptions) {
		if o.GlobalMetricsTags == nil {
			o.GlobalMetricsTags = make(metric.Tags)
		}
		o.GlobalMetricsTags["service"] = serviceName
		o.GlobalMetricsTags["version"] = version
		o.GlobalMetricsTags["environment"] = environment
	}
}

// WithDetailedMetrics enables comprehensive metrics collection
func WithDetailedMetrics(enabled bool) CacheOption {
	return func(o *CacheOptions) {
		if o.MetricsConfig == nil {
			o.MetricsConfig = &MetricsConfig{}
		}
		o.MetricsConfig.EnableDetailedMetrics = enabled
		o.MetricsConfig.EnableSecurityMetrics = enabled
		o.MetricsConfig.EnableLatencyHistograms = enabled
	}
}

// WithMetricsPrefix sets a prefix for all metric names
func WithMetricsPrefix(prefix string) CacheOption {
	return func(o *CacheOptions) {
		if o.MetricsConfig == nil {
			o.MetricsConfig = &MetricsConfig{}
		}
		o.MetricsConfig.MetricsPrefix = prefix
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
	Interval          time.Duration                                    // Cleanup interval
	BatchSize         int                                              // Maximum number of entries to clean per batch
	MaxCleanupTime    time.Duration                                    // Maximum time to spend on cleanup per cycle
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

	// go-metrics integration
	GoMetricsRegistry metric.Registry      // go-metrics registry for comprehensive metrics
	MetricsEnabled    bool                 // Enable/disable metrics collection
	GlobalMetricsTags metric.Tags          // Global tags applied to all metrics
	EnhancedMetrics   EnhancedCacheMetrics // Enhanced metrics implementation using go-metrics

	// Enhanced configuration options
	Security      *SecurityConfig   // Security configuration
	Hooks         *CacheHooks       // Lifecycle hooks
	Indexes       map[string]string // indexName -> keyPattern for secondary indexes
	CleanupConfig *CleanupConfig    // Enhanced cleanup configuration
	MetricsConfig *MetricsConfig    // Enhanced metrics configuration
}

// RedisOptions represents configuration options for Redis cache
type RedisOptions struct {
	Address  string
	Password string
	DB       int
	PoolSize int
}
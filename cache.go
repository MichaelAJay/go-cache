package cache

import (
	"context"
	"time"

	"github.com/MichaelAJay/go-logger"
	"github.com/MichaelAJay/go-serializer"
)

// Cache defines the interface for all cache implementations
type Cache interface {
	// Basic operations
	Get(ctx context.Context, key string) (any, bool, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
	Has(ctx context.Context, key string) bool
	GetKeys(ctx context.Context) []string
	Close() error

	// Bulk operations
	GetMany(ctx context.Context, keys []string) (map[string]any, error)
	SetMany(ctx context.Context, items map[string]any, ttl time.Duration) error
	DeleteMany(ctx context.Context, keys []string) error

	// Metadata operations
	GetMetadata(ctx context.Context, key string) (*CacheEntryMetadata, error)
	GetManyMetadata(ctx context.Context, keys []string) (map[string]*CacheEntryMetadata, error)

	// Metrics
	GetMetrics() *CacheMetricsSnapshot

	// Atomic operations for counters
	Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error)
	Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error)

	// Conditional operations
	SetIfNotExists(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)
	SetIfExists(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)

	// Secondary indexing
	AddIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error
	RemoveIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error
	GetByIndex(ctx context.Context, indexName string, indexKey string) ([]string, error)
	DeleteByIndex(ctx context.Context, indexName string, indexKey string) error

	// Pattern operations
	GetKeysByPattern(ctx context.Context, pattern string) ([]string, error)
	DeleteByPattern(ctx context.Context, pattern string) (int, error)

	// Advanced operations
	UpdateMetadata(ctx context.Context, key string, updater MetadataUpdater) error
	GetAndUpdate(ctx context.Context, key string, updater ValueUpdater, ttl time.Duration) (any, error)
}

// MetadataUpdater defines a function type for updating cache entry metadata
type MetadataUpdater func(metadata *CacheEntryMetadata) *CacheEntryMetadata

// ValueUpdater defines a function type for updating cache entry values
type ValueUpdater func(currentValue any) (newValue any, shouldUpdate bool)

// CacheEntryMetadata represents metadata for a cache entry
type CacheEntryMetadata struct {
	Key          string
	CreatedAt    time.Time
	LastAccessed time.Time
	AccessCount  int64
	TTL          time.Duration
	Size         int64
	Tags         []string
}

// CacheEvent represents different types of cache events
type CacheEvent string

const (
	CacheHit    CacheEvent = "hit"
	CacheMiss   CacheEvent = "miss"
	CacheSet    CacheEvent = "set"
	CacheDelete CacheEvent = "delete"
	CacheClear  CacheEvent = "clear"
)

// CleanupReason represents the reason for cache entry cleanup
type CleanupReason string

const (
	CleanupExpired CleanupReason = "expired"
	CleanupEvicted CleanupReason = "evicted"
	CleanupManual  CleanupReason = "manual"
)

// SecurityConfig defines security settings for cache operations
type SecurityConfig struct {
	EnableTimingProtection bool          // Enable consistent response times
	MinProcessingTime      time.Duration // Minimum time for operations (for timing protection)
	SecureCleanup          bool          // Enable secure memory wiping during cleanup
}

// CacheHooks defines lifecycle hooks for cache operations
type CacheHooks struct {
	PreGet    func(ctx context.Context, key string) error
	PostGet   func(ctx context.Context, key string, value any, found bool, err error)
	PreSet    func(ctx context.Context, key string, value any, ttl time.Duration) error
	PostSet   func(ctx context.Context, key string, value any, ttl time.Duration, err error)
	PreDelete func(ctx context.Context, key string) error
	PostDelete func(ctx context.Context, key string, existed bool, err error)
	OnCleanup func(ctx context.Context, key string, reason CleanupReason)
}

// CleanupConfig defines enhanced cleanup configuration
type CleanupConfig struct {
	Interval           time.Duration                                        // Cleanup interval
	BatchSize          int                                                  // Maximum number of entries to clean per batch
	MaxCleanupTime     time.Duration                                        // Maximum time to spend on cleanup per cycle
	CustomCleanupFunc  func(key string, entry *CacheEntryMetadata) bool    // Custom cleanup logic
}

// MetricsConfig defines enhanced metrics configuration
type MetricsConfig struct {
	EnableDetailedMetrics   bool   // Enable detailed operation metrics
	EnableSecurityMetrics   bool   // Enable security-related metrics
	EnableLatencyHistograms bool   // Enable latency percentile tracking
	MetricsPrefix           string // Prefix for metric names
}

// CacheMiddleware defines a function type for cache middleware
type CacheMiddleware func(next Cache) Cache

// CacheMetrics defines the interface for cache metrics
type CacheMetrics interface {
	RecordHit()
	RecordMiss()
	RecordGetLatency(duration time.Duration)
	RecordSetLatency(duration time.Duration)
	RecordDeleteLatency(duration time.Duration)
	RecordCacheSize(size int64)
	RecordEntryCount(count int64)
	GetMetrics() *CacheMetricsSnapshot
}

// EnhancedCacheMetrics extends CacheMetrics with additional functionality
type EnhancedCacheMetrics interface {
	CacheMetrics // Inherit existing interface

	// Operation-specific metrics
	RecordOperation(operation string, status string, duration time.Duration)
	RecordOperationError(operation string, errorType string, err error)

	// Security metrics
	RecordSecurityEvent(eventType string, severity string, metadata map[string]any)
	RecordTimingProtection(operation string, actualTime, adjustedTime time.Duration)

	// Index metrics
	RecordIndexOperation(indexName string, operation string, keyCount int, duration time.Duration)

	// Cleanup metrics
	RecordCleanup(reason CleanupReason, itemCount int, duration time.Duration)
	RecordMemoryUsage(totalSize int64, entryCount int64)

	// Advanced metrics
	GetOperationLatencyPercentiles(operation string) map[string]time.Duration // P50, P95, P99
	GetErrorRates() map[string]float64                                        // operation -> error rate
}

// CacheMetricsSnapshot represents a snapshot of cache metrics
type CacheMetricsSnapshot struct {
	Hits          int64
	Misses        int64
	HitRatio      float64
	GetLatency    time.Duration
	SetLatency    time.Duration
	DeleteLatency time.Duration
	CacheSize     int64
	EntryCount    int64
}

// CacheProvider defines the interface for cache providers
type CacheProvider interface {
	// Create creates a new cache instance with the given options
	Create(options *CacheOptions) (Cache, error)
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

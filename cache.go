package cache

import (
	"context"
	"time"

	"github.com/MichaelAJay/go-cache/metrics"
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


// CacheMiddleware defines a function type for cache middleware
type CacheMiddleware func(next Cache) Cache

// CacheMetrics defines the interface for cache metrics
// Deprecated: Use github.com/MichaelAJay/go-cache/metrics.CacheMetrics instead
type CacheMetrics = metrics.CacheMetrics

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
// Deprecated: Use github.com/MichaelAJay/go-cache/metrics.CacheMetricsSnapshot instead
type CacheMetricsSnapshot = metrics.CacheMetricsSnapshot

// CacheProvider defines the interface for cache providers
type CacheProvider interface {
	// Create creates a new cache instance with the given options
	Create(options *CacheOptions) (Cache, error)
}


// Re-exported functions for backward compatibility

// NewMetrics creates a new metrics instance
// Deprecated: Use github.com/MichaelAJay/go-cache/metrics.NewMetrics instead
var NewMetrics = metrics.NewMetrics

// NewPrometheusMetrics creates a new metrics instance using Prometheus
// Deprecated: Use github.com/MichaelAJay/go-cache/metrics.NewPrometheusMetrics instead
var NewPrometheusMetrics = metrics.NewPrometheusMetrics

// StartPrometheusServer starts a HTTP server to expose metrics
// Deprecated: Use github.com/MichaelAJay/go-cache/metrics.StartPrometheusServer instead
var StartPrometheusServer = metrics.StartPrometheusServer

// PrometheusExposer is an interface for types that can expose a Prometheus HTTP handler
// Deprecated: Use github.com/MichaelAJay/go-cache/metrics.PrometheusExposer instead
type PrometheusExposer = metrics.PrometheusExposer

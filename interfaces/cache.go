package interfaces

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

// CacheProvider defines the interface for cache providers
type CacheProvider interface {
	// Create creates a new cache instance with the given options
	Create(options *CacheOptions) (Cache, error)
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
type CleanupReason = metrics.CleanupReason

// CleanupReason constants
const (
	CleanupExpired = metrics.CleanupExpired
	CleanupEvicted = metrics.CleanupEvicted
	CleanupManual  = metrics.CleanupManual
)

// CacheMiddleware defines a function type for cache middleware
type CacheMiddleware func(next Cache) Cache

// EnhancedCacheMetrics is an alias to the metrics package interface to avoid duplication
type EnhancedCacheMetrics = metrics.EnhancedCacheMetrics
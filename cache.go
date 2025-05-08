package cache

import (
	"context"
	"time"

	"github.com/MichaelAJay/go-logger"
)

// Cache defines the interface for all cache implementations
type Cache interface {
	// Basic operations
	Get(ctx context.Context, key string) (interface{}, bool, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
	Has(ctx context.Context, key string) bool
	GetKeys(ctx context.Context) []string
	Close() error

	// Bulk operations
	GetMany(ctx context.Context, keys []string) (map[string]interface{}, error)
	SetMany(ctx context.Context, items map[string]interface{}, ttl time.Duration) error
	DeleteMany(ctx context.Context, keys []string) error

	// Metadata operations
	GetMetadata(ctx context.Context, key string) (*CacheEntryMetadata, error)
	GetManyMetadata(ctx context.Context, keys []string) (map[string]*CacheEntryMetadata, error)

	// Metrics
	GetMetrics() *CacheMetricsSnapshot
}

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
	TTL             time.Duration
	MaxEntries      int
	MaxSize         int64
	CleanupInterval time.Duration
	Logger          logger.Logger
	RedisOptions    *RedisOptions
}

// RedisOptions represents configuration options for Redis cache
type RedisOptions struct {
	Address  string
	Password string
	DB       int
	PoolSize int
}

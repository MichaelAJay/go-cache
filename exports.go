package cache

//go:generate go run tools/generate_exports.go

import (
	cacheErrors "github.com/MichaelAJay/go-cache/cache_errors"
	"github.com/MichaelAJay/go-cache/interfaces"
)

// =============================================================================
// CONFIGURATION
// =============================================================================

type (
	CleanupConfig = interfaces.CleanupConfig
	MetricsConfig = interfaces.MetricsConfig
	RedisOptions = interfaces.RedisOptions
	SecurityConfig = interfaces.SecurityConfig
)

// =============================================================================
// CONSTANTS
// =============================================================================

const (
	CacheClear = interfaces.CacheClear
	CacheDelete = interfaces.CacheDelete
	CacheHit = interfaces.CacheHit
	CacheMiss = interfaces.CacheMiss
	CacheSet = interfaces.CacheSet
	CleanupEvicted = interfaces.CleanupEvicted
	CleanupExpired = interfaces.CleanupExpired
	CleanupManual = interfaces.CleanupManual
)

// =============================================================================
// ERRORS
// =============================================================================

var (
	ErrCacheFull = cacheErrors.ErrCacheFull
	ErrContextCanceled = cacheErrors.ErrContextCanceled
	ErrDeserialization = cacheErrors.ErrDeserialization
	ErrInvalidKey = cacheErrors.ErrInvalidKey
	ErrInvalidTTL = cacheErrors.ErrInvalidTTL
	ErrInvalidValue = cacheErrors.ErrInvalidValue
	ErrKeyNotFound = cacheErrors.ErrKeyNotFound
	ErrSerialization = cacheErrors.ErrSerialization
)

// =============================================================================
// EVENTS
// =============================================================================

type (
	CleanupReason = interfaces.CleanupReason
)

// =============================================================================
// FUNCTIONS
// =============================================================================

var (
	WithCleanupConfig = interfaces.WithCleanupConfig
	WithCleanupInterval = interfaces.WithCleanupInterval
	WithDetailedMetrics = interfaces.WithDetailedMetrics
	WithGlobalMetricsTags = interfaces.WithGlobalMetricsTags
	WithGoMetricsRegistry = interfaces.WithGoMetricsRegistry
	WithHooks = interfaces.WithHooks
	WithIndexes = interfaces.WithIndexes
	WithLogger = interfaces.WithLogger
	WithMaxEntries = interfaces.WithMaxEntries
	WithMaxSize = interfaces.WithMaxSize
	WithMetricsConfig = interfaces.WithMetricsConfig
	WithMetricsEnabled = interfaces.WithMetricsEnabled
	WithMetricsPrefix = interfaces.WithMetricsPrefix
	WithOpenTelemetryMetrics = interfaces.WithOpenTelemetryMetrics
	WithPrometheusMetrics = interfaces.WithPrometheusMetrics
	WithRedisOptions = interfaces.WithRedisOptions
	WithSecurityConfig = interfaces.WithSecurityConfig
	WithServiceTags = interfaces.WithServiceTags
	WithTTL = interfaces.WithTTL
)

// =============================================================================
// INTERFACES
// =============================================================================

type (
	Cache = interfaces.Cache
	CacheEntryMetadata = interfaces.CacheEntryMetadata
	CacheEvent = interfaces.CacheEvent
	CacheHooks = interfaces.CacheHooks
	CacheMiddleware = interfaces.CacheMiddleware
	CacheOption = interfaces.CacheOption
	CacheOptions = interfaces.CacheOptions
	CacheProvider = interfaces.CacheProvider
	EnhancedCacheMetrics = interfaces.EnhancedCacheMetrics
	TypedCacheProvider = interfaces.TypedCacheProvider
	TypeInfo = interfaces.TypeInfo
)

// =============================================================================
// TYPES
// =============================================================================

type (
	MetadataUpdater = interfaces.MetadataUpdater
	ValueUpdater = interfaces.ValueUpdater
)

// =============================================================================
// COMPILE-TIME VALIDATION
// =============================================================================
// Ensure re-exported types maintain compatibility

var (
	_ CacheOption = interfaces.WithTTL(0)
	_ CacheOption = interfaces.WithMaxEntries(0)
	_ CacheOption = interfaces.WithCleanupInterval(0)
	_ CacheEvent = interfaces.CacheHit
	_ CacheEvent = interfaces.CacheMiss
)

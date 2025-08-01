package cache

import (
	"github.com/MichaelAJay/go-cache/interfaces"
)

// Re-export types from interfaces package for backward compatibility
type Cache = interfaces.Cache
type CacheProvider = interfaces.CacheProvider
type MetadataUpdater = interfaces.MetadataUpdater
type ValueUpdater = interfaces.ValueUpdater
type CacheEntryMetadata = interfaces.CacheEntryMetadata
type CacheEvent = interfaces.CacheEvent
type CleanupReason = interfaces.CleanupReason
type CacheMiddleware = interfaces.CacheMiddleware
type EnhancedCacheMetrics = interfaces.EnhancedCacheMetrics

// Re-export constants from interfaces package
const (
	CacheHit    = interfaces.CacheHit
	CacheMiss   = interfaces.CacheMiss
	CacheSet    = interfaces.CacheSet
	CacheDelete = interfaces.CacheDelete
	CacheClear  = interfaces.CacheClear
)

// Re-export cleanup reason constants
const (
	CleanupExpired = interfaces.CleanupExpired
	CleanupEvicted = interfaces.CleanupEvicted
	CleanupManual  = interfaces.CleanupManual
)

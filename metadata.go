package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	cacheErrors "github.com/MichaelAJay/go-cache/cache_errors"
	"github.com/MichaelAJay/go-cache/interfaces"
)

// GetMetadata returns metadata for a cache entry
func (c *RedisCache[T]) GetMetadata(ctx context.Context, key string) (*interfaces.CacheEntryMetadata, error) {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.precomputedMetrics.GetMetadataCircuitBreakerErrorCounter().Inc()
		return nil, cacheErrors.ErrCircuitBreakerOpen
	}

	dataKey := c.buildDataKey(key)
	metaKey := c.buildMetaKey(key)

	// Check if key exists first
	exists, err := c.client.Exists(ctx, dataKey).Result()
	if err != nil {
		c.handleError("getmetadata", err)
		c.precomputedMetrics.GetMetadataRedisErrorCounter().Inc()
		return nil, fmt.Errorf("Redis GetMetadata existence check error: %w", err)
	}

	if exists == 0 {
		// Data key doesn't exist - check for and cleanup any orphaned metadata
		reverseKey := c.buildReverseIndexKey(key)
		lruTrackerKey := c.buildLRUTrackerKey()
		indexPrefix := "cache:index:"
		if c.redisOptions != nil && c.redisOptions.IndexPrefix != "" {
			indexPrefix = c.redisOptions.IndexPrefix
		}

		cleaned, err := c.cleanupOrphanedMetadataScript.Run(ctx, c.client, 
			[]string{metaKey, reverseKey, lruTrackerKey}, 
			key, indexPrefix).Result()
		
		if err != nil {
			// Log cleanup error but don't fail the operation since cleanup is supplementary
			c.handleError("cleanup_orphaned_metadata", err)
		} else if cleanedInt, ok := cleaned.(int64); ok && cleanedInt > 0 {
			// Record successful cleanup for monitoring
			c.precomputedMetrics.CleanupOrphanedMetadataSuccessCounter().Inc()
		}

		// Key doesn't exist, return nil (not an error according to interface contract)
		c.precomputedMetrics.GetMetadataTimer().Record(time.Since(start))
		c.precomputedMetrics.GetMetadataNotFoundCounter().Inc()
		return nil, nil
	}

	// Get all metadata fields
	metadataFields, err := c.client.HGetAll(ctx, metaKey).Result()
	if err != nil {
		c.handleError("getmetadata", err)
		c.precomputedMetrics.GetMetadataRedisErrorCounter().Inc()
		return nil, fmt.Errorf("Redis GetMetadata HGetAll error: %w", err)
	}

	// Parse metadata fields
	metadata := &interfaces.CacheEntryMetadata{
		Key: key,
	}

	// Parse timestamps (handle both second and microsecond precision)
	if createdAtStr, ok := metadataFields["created_at"]; ok {
		if createdAtUnix, err := strconv.ParseInt(createdAtStr, 10, 64); err == nil {
			if createdAtUnix > 1e12 { // Microsecond timestamp (> year 2001 in microseconds)
				seconds := createdAtUnix / 1000000
				nanoseconds := (createdAtUnix % 1000000) * 1000
				metadata.CreatedAt = time.Unix(seconds, nanoseconds)
			} else { // Second timestamp
				metadata.CreatedAt = time.Unix(createdAtUnix, 0)
			}
		}
	}

	if lastAccessedStr, ok := metadataFields["last_accessed"]; ok {
		if lastAccessedUnix, err := strconv.ParseInt(lastAccessedStr, 10, 64); err == nil {
			if lastAccessedUnix > 1e12 { // Microsecond timestamp (> year 2001 in microseconds)
				seconds := lastAccessedUnix / 1000000
				nanoseconds := (lastAccessedUnix % 1000000) * 1000
				metadata.LastAccessed = time.Unix(seconds, nanoseconds)
			} else { // Second timestamp
				metadata.LastAccessed = time.Unix(lastAccessedUnix, 0)
			}
		}
	}

	// Parse access count
	if accessCountStr, ok := metadataFields["access_count"]; ok {
		if accessCount, err := strconv.ParseInt(accessCountStr, 10, 64); err == nil {
			metadata.AccessCount = accessCount
		}
	}

	// Parse TTL
	if ttlStr, ok := metadataFields["ttl"]; ok {
		if ttlSeconds, err := strconv.ParseInt(ttlStr, 10, 64); err == nil {
			metadata.TTL = time.Duration(ttlSeconds) * time.Second
		}
	}

	// Parse size
	if sizeStr, ok := metadataFields["size"]; ok {
		if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
			metadata.Size = size
		}
	}

	// Get remaining TTL from Redis
	if ttl, err := c.client.TTL(ctx, dataKey).Result(); err == nil {
		// Update TTL with actual remaining time if key has expiration
		if ttl > 0 {
			metadata.TTL = ttl
		}
	}

	// Parse tags if they exist
	if tagsStr, ok := metadataFields["tags"]; ok && tagsStr != "" {
		// Simple comma-separated tags implementation
		// In a full implementation, this might use a more sophisticated format
		tags := make([]string, 0)
		if tagsStr != "" {
			// Split by comma and trim spaces (simplified)
			parts := []string{tagsStr} // Simplified - would need proper CSV parsing
			tags = append(tags, parts...)
		}
		metadata.Tags = tags
	}

	c.precomputedMetrics.GetMetadataTimer().Record(time.Since(start))
	c.precomputedMetrics.GetMetadataSuccessCounter().Inc()
	return metadata, nil
}

package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/MichaelAJay/go-cache/interfaces"
	cacheErrors "github.com/MichaelAJay/go-cache/cache_errors"
)

// GetMetadata returns metadata for a cache entry
func (c *redisCache[T]) GetMetadata(ctx context.Context, key string) (*interfaces.CacheEntryMetadata, error) {
	start := time.Now()
	
	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "getmetadata", "circuit_breaker", "availability", c.getMetricTags())
		return nil, cacheErrors.ErrCircuitBreakerOpen
	}
	
	// Apply security timing protection if enabled
	defer func() {
		if c.options.Security != nil && c.options.Security.EnableTimingProtection {
			c.applyTimingProtection("getmetadata", start)
		}
	}()
	
	dataKey := c.buildDataKey(key)
	metaKey := c.buildMetaKey(key)
	
	// Check if key exists first
	exists, err := c.client.Exists(ctx, dataKey).Result()
	if err != nil {
		c.handleError("getmetadata", err)
		c.metrics.RecordError("redis", "getmetadata", "redis_error", "infrastructure", c.getMetricTags())
		return nil, fmt.Errorf("Redis GetMetadata existence check error: %w", err)
	}
	
	if exists == 0 {
		// Key doesn't exist, return nil (not an error according to interface contract)
		c.metrics.RecordOperation("redis", "getmetadata", "not_found", time.Since(start), c.getMetricTags())
		return nil, nil
	}
	
	// Get all metadata fields
	metadataFields, err := c.client.HGetAll(ctx, metaKey).Result()
	if err != nil {
		c.handleError("getmetadata", err)
		c.metrics.RecordError("redis", "getmetadata", "redis_error", "infrastructure", c.getMetricTags())
		return nil, fmt.Errorf("Redis GetMetadata HGetAll error: %w", err)
	}
	
	// Parse metadata fields
	metadata := &interfaces.CacheEntryMetadata{
		Key: key,
	}
	
	// Parse timestamps
	if createdAtStr, ok := metadataFields["created_at"]; ok {
		if createdAtUnix, err := strconv.ParseInt(createdAtStr, 10, 64); err == nil {
			metadata.CreatedAt = time.Unix(createdAtUnix, 0)
		}
	}
	
	if lastAccessedStr, ok := metadataFields["last_accessed"]; ok {
		if lastAccessedUnix, err := strconv.ParseInt(lastAccessedStr, 10, 64); err == nil {
			metadata.LastAccessed = time.Unix(lastAccessedUnix, 0)
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
	
	c.metrics.RecordOperation("redis", "getmetadata", "success", time.Since(start), c.getMetricTags())
	return metadata, nil
}

// setMetadata creates or updates metadata for a cache entry
func (c *redisCache[T]) setMetadata(ctx context.Context, key string, ttl time.Duration, size int64) error {
	metaKey := c.buildMetaKey(key)
	now := time.Now().Unix()
	
	// Check if metadata already exists to decide between create vs update
	exists, err := c.client.Exists(ctx, metaKey).Result()
	if err != nil {
		return fmt.Errorf("metadata existence check error: %w", err)
	}
	
	if exists > 0 {
		// Update existing metadata
		pipe := c.client.TxPipeline()
		pipe.HIncrBy(ctx, metaKey, "access_count", 1)
		pipe.HSet(ctx, metaKey, "last_accessed", now)
		pipe.HSet(ctx, metaKey, "ttl", int64(ttl.Seconds()))
		pipe.HSet(ctx, metaKey, "size", size)
		
		if ttl > 0 {
			pipe.Expire(ctx, metaKey, ttl)
		}
		
		_, err := pipe.Exec(ctx)
		return err
	} else {
		// Create new metadata
		metadataMap := map[string]interface{}{
			"created_at":    now,
			"last_accessed": now,
			"access_count":  1,
			"ttl":           int64(ttl.Seconds()),
			"size":          size,
		}
		
		err := c.client.HMSet(ctx, metaKey, metadataMap).Err()
		if err != nil {
			return err
		}
		
		if ttl > 0 {
			return c.client.Expire(ctx, metaKey, ttl).Err()
		}
		
		return nil
	}
}

// updateMetadataOnAccess updates metadata when a key is accessed
func (c *redisCache[T]) updateMetadataOnAccess(ctx context.Context, key string) error {
	metaKey := c.buildMetaKey(key)
	now := time.Now().Unix()
	
	pipe := c.client.TxPipeline()
	pipe.HIncrBy(ctx, metaKey, "access_count", 1)
	pipe.HSet(ctx, metaKey, "last_accessed", now)
	
	_, err := pipe.Exec(ctx)
	return err
}

// deleteMetadata removes metadata for a cache entry
func (c *redisCache[T]) deleteMetadata(ctx context.Context, key string) error {
	metaKey := c.buildMetaKey(key)
	return c.client.Del(ctx, metaKey).Err()
}

// getMetadataStats returns aggregated statistics about cache metadata
// This is an internal helper method for monitoring and diagnostics
func (c *redisCache[T]) getMetadataStats(ctx context.Context) (map[string]interface{}, error) {
	// Get all metadata keys
	metaPattern := c.buildMetaKey("*")
	metaKeys, err := c.client.Keys(ctx, metaPattern).Result()
	if err != nil {
		return nil, fmt.Errorf("error getting metadata keys: %w", err)
	}
	
	stats := map[string]interface{}{
		"total_entries":    len(metaKeys),
		"total_size":       int64(0),
		"total_accesses":   int64(0),
		"average_age":      time.Duration(0),
		"oldest_entry":     time.Time{},
		"newest_entry":     time.Time{},
	}
	
	if len(metaKeys) == 0 {
		return stats, nil
	}
	
	// Sample metadata to calculate aggregates
	// In a production system, this might be done more efficiently with sampling
	var totalSize, totalAccesses int64
	var oldestTime, newestTime time.Time
	var ageSum time.Duration
	
	now := time.Now()
	sampleSize := 100 // Sample up to 100 entries for performance
	step := len(metaKeys) / sampleSize
	if step == 0 {
		step = 1
	}
	
	for i := 0; i < len(metaKeys); i += step {
		metaKey := metaKeys[i]
		
		// Get metadata fields for this key
		fields, err := c.client.HGetAll(ctx, metaKey).Result()
		if err != nil {
			continue // Skip this entry
		}
		
		// Parse and accumulate statistics
		if sizeStr, ok := fields["size"]; ok {
			if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
				totalSize += size
			}
		}
		
		if accessCountStr, ok := fields["access_count"]; ok {
			if accessCount, err := strconv.ParseInt(accessCountStr, 10, 64); err == nil {
				totalAccesses += accessCount
			}
		}
		
		if createdAtStr, ok := fields["created_at"]; ok {
			if createdAtUnix, err := strconv.ParseInt(createdAtStr, 10, 64); err == nil {
				createdAt := time.Unix(createdAtUnix, 0)
				age := now.Sub(createdAt)
				ageSum += age
				
				if oldestTime.IsZero() || createdAt.Before(oldestTime) {
					oldestTime = createdAt
				}
				if newestTime.IsZero() || createdAt.After(newestTime) {
					newestTime = createdAt
				}
			}
		}
	}
	
	// Calculate averages
	sampleCount := (len(metaKeys) + step - 1) / step // Ceiling division
	if sampleCount > 0 {
		stats["total_size"] = totalSize * int64(step) // Extrapolate from sample
		stats["total_accesses"] = totalAccesses * int64(step)
		stats["average_age"] = time.Duration(int64(ageSum) / int64(sampleCount))
		stats["oldest_entry"] = oldestTime
		stats["newest_entry"] = newestTime
	}
	
	return stats, nil
}
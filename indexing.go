package cache

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	cacheErrors "github.com/MichaelAJay/go-cache/cache_errors"
)

// AddIndex associates a key matching keyPattern with an index entry
func (c *RedisCache[T]) AddIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "addindex", "circuit_breaker", "availability", c.getMetricTags())
		return cacheErrors.ErrCircuitBreakerOpen
	}

	// Validate parameters
	if indexName == "" || keyPattern == "" || indexKey == "" {
		return fmt.Errorf("indexName, keyPattern, and indexKey cannot be empty")
	}

	// Find all keys matching the pattern
	dataPattern := c.buildDataKey(keyPattern)
	matchingDataKeys, err := c.client.Keys(ctx, dataPattern).Result()
	if err != nil {
		c.handleError("addindex", err)
		c.metrics.RecordError("redis", "addindex", "redis_error", "infrastructure", c.getMetricTags())
		return fmt.Errorf("Redis AddIndex pattern matching error: %w", err)
	}

	if len(matchingDataKeys) == 0 {
		// No matching keys found, but this is not an error
		c.metrics.RecordIndexOperation("redis", "addindex", indexName, time.Since(start), c.getMetricTags())
		return nil
	}

	// Extract actual cache keys from Redis data keys
	redisIndexKey := c.buildIndexKey(indexName, indexKey)
	cacheKeys := make([]string, 0, len(matchingDataKeys))

	for _, dataKey := range matchingDataKeys {
		// Extract cache key from data key (remove prefix)
		cacheKey := dataKey[len(dataPrefix):]
		cacheKeys = append(cacheKeys, cacheKey)
	}

	// Add all matching keys to the index using Redis SET
	if len(cacheKeys) > 0 {
		// Convert to interface{} slice for Redis SAdd
		members := make([]interface{}, len(cacheKeys))
		for i, key := range cacheKeys {
			members[i] = key
		}

		if err := c.client.SAdd(ctx, redisIndexKey, members...).Err(); err != nil {
			c.handleError("addindex", err)
			c.metrics.RecordError("redis", "addindex", "redis_error", "infrastructure", c.getMetricTags())
			return fmt.Errorf("Redis AddIndex SAdd error: %w", err)
		}
	}

	c.metrics.RecordIndexOperation("redis", "addindex", indexName, time.Since(start), c.getMetricTags())
	return nil
}

// RemoveIndex removes association between key pattern and index entry
func (c *RedisCache[T]) RemoveIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "removeindex", "circuit_breaker", "availability", c.getMetricTags())
		return cacheErrors.ErrCircuitBreakerOpen
	}

	// Validate parameters
	if indexName == "" || keyPattern == "" || indexKey == "" {
		return fmt.Errorf("indexName, keyPattern, and indexKey cannot be empty")
	}

	// Find all keys matching the pattern
	dataPattern := c.buildDataKey(keyPattern)
	matchingDataKeys, err := c.client.Keys(ctx, dataPattern).Result()
	if err != nil {
		c.handleError("removeindex", err)
		c.metrics.RecordError("redis", "removeindex", "redis_error", "infrastructure", c.getMetricTags())
		return fmt.Errorf("Redis RemoveIndex pattern matching error: %w", err)
	}

	if len(matchingDataKeys) == 0 {
		// No matching keys found, but this is not an error
		c.metrics.RecordIndexOperation("redis", "removeindex", indexName, time.Since(start), c.getMetricTags())
		return nil
	}

	// Extract actual cache keys from Redis data keys and remove from index
	redisIndexKey := c.buildIndexKey(indexName, indexKey)

	for _, dataKey := range matchingDataKeys {
		// Extract cache key from data key (remove prefix)
		cacheKey := dataKey[len(dataPrefix):]

		if err := c.client.SRem(ctx, redisIndexKey, cacheKey).Err(); err != nil {
			c.handleError("removeindex", err)
			c.metrics.RecordError("redis", "removeindex", "redis_error", "infrastructure", c.getMetricTags())
			return fmt.Errorf("Redis RemoveIndex SRem error: %w", err)
		}
	}

	c.metrics.RecordIndexOperation("redis", "removeindex", indexName, time.Since(start), c.getMetricTags())
	return nil
}

// GetByIndex returns all keys associated with an index entry
func (c *RedisCache[T]) GetByIndex(ctx context.Context, indexName string, indexKey string) ([]string, error) {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "getbyindex", "circuit_breaker", "availability", c.getMetricTags())
		return nil, cacheErrors.ErrCircuitBreakerOpen
	}

	// Validate parameters
	if indexName == "" || indexKey == "" {
		return nil, fmt.Errorf("indexName and indexKey cannot be empty")
	}

	redisIndexKey := c.buildIndexKey(indexName, indexKey)

	// Get all members from the Redis SET
	members, err := c.client.SMembers(ctx, redisIndexKey).Result()
	if err != nil {
		c.handleError("getbyindex", err)
		c.metrics.RecordError("redis", "getbyindex", "redis_error", "infrastructure", c.getMetricTags())
		return nil, fmt.Errorf("Redis GetByIndex error: %w", err)
	}

	// Filter out any keys that no longer exist in the cache
	existingKeys := make([]string, 0, len(members))
	if len(members) > 0 {
		// Check existence in batches for efficiency
		pipe := c.client.TxPipeline()
		existsResults := make([]interface{}, len(members))

		for i, member := range members {
			dataKey := c.buildDataKey(member)
			existsResults[i] = pipe.Exists(ctx, dataKey)
		}

		_, err := pipe.Exec(ctx)
		if err != nil {
			c.handleError("getbyindex", err)
			c.metrics.RecordError("redis", "getbyindex", "redis_error", "infrastructure", c.getMetricTags())
			return nil, fmt.Errorf("Redis GetByIndex existence check error: %w", err)
		}

		// Collect keys that still exist and clean up index
		keysToRemove := make([]interface{}, 0)
		for i, member := range members {
			if cmdResult, ok := existsResults[i].(interface {
				Val() int64
				Err() error
			}); ok {
				if cmdResult.Err() == nil && cmdResult.Val() > 0 {
					existingKeys = append(existingKeys, member)
				} else {
					// Key no longer exists, mark for removal from index
					keysToRemove = append(keysToRemove, member)
				}
			}
		}

		// Clean up stale index entries
		if len(keysToRemove) > 0 {
			c.client.SRem(ctx, redisIndexKey, keysToRemove...)
		}
	}

	c.metrics.RecordIndexOperation("redis", "getbyindex", indexName, time.Since(start), c.getMetricTags())
	return existingKeys, nil
}

// DeleteByIndex removes all keys associated with an index entry
func (c *RedisCache[T]) DeleteByIndex(ctx context.Context, indexName string, indexKey string) error {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "deletebyindex", "circuit_breaker", "availability", c.getMetricTags())
		return cacheErrors.ErrCircuitBreakerOpen
	}

	// Validate parameters
	if indexName == "" || indexKey == "" {
		return fmt.Errorf("indexName and indexKey cannot be empty")
	}

	redisIndexKey := c.buildIndexKey(indexName, indexKey)

	// Use Lua script for atomic delete operation
	deletedCount, err := c.deleteByIndexScript.Run(ctx, c.client, []string{redisIndexKey},
		dataPrefix, metaPrefix).Result()

	if err != nil {
		c.handleError("deletebyindex", err)
		c.metrics.RecordError("redis", "deletebyindex", "redis_error", "infrastructure", c.getMetricTags())
		return fmt.Errorf("Redis DeleteByIndex error: %w", err)
	}

	count, ok := deletedCount.(int64)
	if ok {
		c.metrics.RecordIndexOperation("redis", "deletebyindex", indexName, time.Since(start), c.getMetricTags())

		// Apply hooks if configured
		if c.options.Hooks != nil && c.options.Hooks.PostDelete != nil && count > 0 {
			// We can't know individual key names after deletion, so we'll log the count
			c.options.Hooks.PostDelete(ctx, fmt.Sprintf("%s:%s", indexName, indexKey), count > 0, nil)
		}
	}

	return nil
}

// GetKeysByPattern returns keys matching pattern (e.g., "user:*")
func (c *RedisCache[T]) GetKeysByPattern(ctx context.Context, pattern string) ([]string, error) {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "getkeysbypattern", "circuit_breaker", "availability", c.getMetricTags())
		return nil, cacheErrors.ErrCircuitBreakerOpen
	}

	if pattern == "" {
		return nil, fmt.Errorf("pattern cannot be empty")
	}

	// Build Redis pattern
	dataPattern := c.buildDataKey(pattern)

	// Get matching data keys
	matchingDataKeys, err := c.client.Keys(ctx, dataPattern).Result()
	if err != nil {
		c.handleError("getkeysbypattern", err)
		c.metrics.RecordError("redis", "getkeysbypattern", "redis_error", "infrastructure", c.getMetricTags())
		return nil, fmt.Errorf("Redis GetKeysByPattern error: %w", err)
	}

	// Extract cache keys from data keys
	cacheKeys := make([]string, 0, len(matchingDataKeys))
	for _, dataKey := range matchingDataKeys {
		// Extract cache key from data key (remove prefix)
		cacheKey := dataKey[len(dataPrefix):]
		cacheKeys = append(cacheKeys, cacheKey)
	}

	c.metrics.RecordOperation("redis", "getkeysbypattern", "success", time.Since(start), c.getMetricTags())
	return cacheKeys, nil
}

// DeleteByPattern removes all keys matching pattern
func (c *RedisCache[T]) DeleteByPattern(ctx context.Context, pattern string) (int, error) {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "deletebypattern", "circuit_breaker", "availability", c.getMetricTags())
		return 0, cacheErrors.ErrCircuitBreakerOpen
	}

	if pattern == "" {
		return 0, fmt.Errorf("pattern cannot be empty")
	}

	// Build Redis pattern
	dataPattern := c.buildDataKey(pattern)

	// Use Lua script for atomic delete operation
	deletedCount, err := c.deleteByPatternScript.Run(ctx, c.client, []string{},
		dataPattern, dataPrefix, metaPrefix).Result()

	if err != nil {
		c.handleError("deletebypattern", err)
		c.metrics.RecordError("redis", "deletebypattern", "redis_error", "infrastructure", c.getMetricTags())
		return 0, fmt.Errorf("Redis DeleteByPattern error: %w", err)
	}

	count, ok := deletedCount.(int64)
	if !ok {
		count = 0
	}

	c.metrics.RecordOperation("redis", "deletebypattern", "success", time.Since(start), c.getMetricTags())
	return int(count), nil
}

// maintainIndexConsistency ensures indexes remain consistent with actual data
// This is a maintenance operation that should be called periodically
func (c *RedisCache[T]) maintainIndexConsistency(ctx context.Context) error {
	// Get all index keys
	indexPattern := indexPrefix + "*"
	indexKeys, err := c.client.Keys(ctx, indexPattern).Result()
	if err != nil {
		return fmt.Errorf("error getting index keys: %w", err)
	}

	// For each index, verify all members still exist and remove stale entries
	for _, indexKey := range indexKeys {
		members, err := c.client.SMembers(ctx, indexKey).Result()
		if err != nil {
			continue // Skip this index and continue with others
		}

		staleMembers := make([]interface{}, 0)
		for _, member := range members {
			dataKey := c.buildDataKey(member)
			exists, err := c.client.Exists(ctx, dataKey).Result()
			if err != nil || exists == 0 {
				staleMembers = append(staleMembers, member)
			}
		}

		// Remove stale members
		if len(staleMembers) > 0 {
			c.client.SRem(ctx, indexKey, staleMembers...)
		}

		// If index is now empty, remove it entirely
		if len(members) == len(staleMembers) {
			c.client.Del(ctx, indexKey)
		}
	}

	return nil
}

// addToIndexesOnSet adds a key to relevant indexes when it's set
func (c *RedisCache[T]) addToIndexesOnSet(ctx context.Context, key string) {
	if c.options.Indexes == nil {
		return
	}

	// Check each configured index to see if this key should be added
	for indexName, keyPattern := range c.options.Indexes {
		if matched, err := filepath.Match(keyPattern, key); err == nil && matched {
			// This key matches the pattern, but we need to determine the index key
			// For now, we'll use a default approach - in a full implementation,
			// this would be configurable or derived from the key structure

			// Extract index key from the cache key (simple heuristic)
			// For example, "user:123" with pattern "user:*" might index under "123"
			// This is a simplified implementation
			indexKey := key
			if len(keyPattern) > 2 && keyPattern[len(keyPattern)-1] == '*' {
				prefix := keyPattern[:len(keyPattern)-1]
				if len(key) > len(prefix) && key[:len(prefix)] == prefix {
					indexKey = key[len(prefix):]
				}
			}

			redisIndexKey := c.buildIndexKey(indexName, indexKey)
			c.client.SAdd(ctx, redisIndexKey, key)
		}
	}
}

package cache

import (
	"context"
	"fmt"
	"time"

	cacheErrors "github.com/MichaelAJay/go-cache/cache_errors"
)

// GetMany retrieves multiple keys in a single operation
// @TODO Lua script
func (c *RedisCache[T]) GetMany(ctx context.Context, keys []string) (map[string]T, error) {
	start := time.Now()
	result := make(map[string]T)

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "getmany", "circuit_breaker", "availability", c.getMetricTags())
		return result, cacheErrors.ErrCircuitBreakerOpen
	}

	if len(keys) == 0 {
		return result, nil
	}

	// Build Redis keys for pipeline
	dataKeys := make([]string, len(keys))
	metaKeys := make([]string, len(keys))
	for i, key := range keys {
		dataKeys[i] = c.buildDataKey(key)
		metaKeys[i] = c.buildMetaKey(key)
	}

	// Use pipeline for efficient batch retrieval
	pipe := c.client.TxPipeline()

	// Get all data values
	dataResults := make([]interface{}, len(keys))
	for i, dataKey := range dataKeys {
		dataResults[i] = pipe.Get(ctx, dataKey)
	}

	// Update metadata for accessed keys (increment access count, update last accessed)
	for _, metaKey := range metaKeys {
		pipe.HIncrBy(ctx, metaKey, "access_count", 1)
		pipe.HSet(ctx, metaKey, "last_accessed", time.Now().Unix())
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil && err.Error() != "redis: nil" {
		c.handleError("getmany", err)
		c.metrics.RecordError("redis", "getmany", "redis_error", "infrastructure", c.getMetricTags())
		return result, fmt.Errorf("Redis GetMany error: %w", err)
	}

	// Process results
	hits := 0
	misses := 0

	for i, key := range keys {
		if cmdResult, ok := dataResults[i].(interface {
			Val() string
			Err() error
		}); ok {
			serializedValue := cmdResult.Val()
			if cmdResult.Err() == nil && serializedValue != "" {
				// Deserialize value
				var value T
				if err := c.serializer.Deserialize([]byte(serializedValue), &value); err != nil {
					c.metrics.RecordError("redis", "getmany", "serialization_error", "data", c.getMetricTags())
					// Log the error but continue processing other keys
					continue
				}

				result[key] = value
				hits++
			} else {
				misses++
			}
		} else {
			misses++
		}
	}

	// Record metrics
	c.metrics.RecordBatchOperation("redis", "getmany", len(keys), time.Since(start), c.getMetricTags())

	// Record individual hits/misses for accurate statistics
	for i := 0; i < hits; i++ {
		c.metrics.RecordHit("redis", c.getMetricTags())
	}
	for i := 0; i < misses; i++ {
		c.metrics.RecordMiss("redis", c.getMetricTags())
	}

	return result, nil
}

// @TODO Lua script
// SetMany stores multiple values with same TTL, using extractors for keys
func (c *RedisCache[T]) SetMany(ctx context.Context, values []T, ttl time.Duration) error {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "setmany", "circuit_breaker", "availability", c.getMetricTags())
		return cacheErrors.ErrCircuitBreakerOpen
	}

	if len(values) == 0 {
		return nil
	}

	// Require key extractor for SetMany
	if c.extractor.GetEntryKey == nil {
		return fmt.Errorf("IndexExtractor.GetEntryKey is required for SetMany operation")
	}

	// Use pipeline for efficient batch setting
	pipe := c.client.TxPipeline()
	now := time.Now().Unix()

	// Process all values
	for _, value := range values {
		key := c.extractor.GetEntryKey(value)
		// Serialize value
		serializedValue, err := c.serializer.Serialize(value)
		if err != nil {
			c.metrics.RecordError("redis", "setmany", "serialization_error", "data", c.getMetricTags())
			return fmt.Errorf("serialization error for key %s: %w", key, err)
		}

		dataKey := c.buildDataKey(key)
		metaKey := c.buildMetaKey(key)

		// Set data
		if ttl > 0 {
			pipe.SetEX(ctx, dataKey, serializedValue, ttl)
			pipe.Expire(ctx, metaKey, ttl)
		} else {
			pipe.Set(ctx, dataKey, serializedValue, 0)
		}

		// Set metadata
		pipe.HMSet(ctx, metaKey, map[string]interface{}{
			"created_at":    now,
			"last_accessed": now,
			"access_count":  1,
			"ttl":           ttlToMilliseconds(ttl),
			"size":          len(serializedValue),
		})

		// Update indexes if configured
		if c.extractor.GetOwnerKey != nil {
			ownerKey := c.extractor.GetOwnerKey(value)
			indexKey := c.buildIndexKey("owner", ownerKey)
			pipe.SAdd(ctx, indexKey, key)
			if ttl > 0 {
				pipe.Expire(ctx, indexKey, ttl)
			}
		}
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		c.handleError("setmany", err)
		c.metrics.RecordError("redis", "setmany", "redis_error", "infrastructure", c.getMetricTags())
		return fmt.Errorf("Redis SetMany error: %w", err)
	}

	c.metrics.RecordBatchOperation("redis", "setmany", len(values), time.Since(start), c.getMetricTags())
	return nil
}

// @TODO Lua script
// DeleteMany removes multiple keys
func (c *RedisCache[T]) DeleteMany(ctx context.Context, keys []string) error {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "deletemany", "circuit_breaker", "availability", c.getMetricTags())
		return cacheErrors.ErrCircuitBreakerOpen
	}

	if len(keys) == 0 {
		return nil
	}

	// Build Redis keys
	allKeys := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		dataKey := c.buildDataKey(key)
		metaKey := c.buildMetaKey(key)
		allKeys = append(allKeys, dataKey, metaKey)
	}

	// Delete in batches to avoid blocking Redis
	batchSize := 100
	for i := 0; i < len(allKeys); i += batchSize {
		end := i + batchSize
		if end > len(allKeys) {
			end = len(allKeys)
		}

		if err := c.client.Del(ctx, allKeys[i:end]...).Err(); err != nil {
			c.handleError("deletemany", err)
			c.metrics.RecordError("redis", "deletemany", "redis_error", "infrastructure", c.getMetricTags())
			return fmt.Errorf("Redis DeleteMany error: %w", err)
		}
	}

	c.metrics.RecordBatchOperation("redis", "deletemany", len(keys), time.Since(start), c.getMetricTags())

	// Apply hooks if configured
	if c.options.Hooks != nil && c.options.Hooks.PostDelete != nil {
		for _, key := range keys {
			c.options.Hooks.PostDelete(ctx, key, true, nil) // Assume all were deleted
		}
	}

	return nil
}

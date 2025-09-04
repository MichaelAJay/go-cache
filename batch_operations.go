package cache

import (
	"context"
	"fmt"
	"time"

	cacheErrors "github.com/MichaelAJay/go-cache/cache_errors"
)

// GetMany retrieves multiple keys efficiently using optimized pipeline operations
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

	// Use optimized pipeline approach
	pipe := c.client.TxPipeline()

	// Get all data values
	dataResults := make([]any, len(keys))
	for i, dataKey := range dataKeys {
		dataResults[i] = pipe.Get(ctx, dataKey)
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil && err.Error() != "redis: nil" {
		c.handleError("getmany", err)
		c.metrics.RecordError("redis", "getmany", "redis_error", "infrastructure", c.getMetricTags())
		return result, fmt.Errorf("redis GetMany error: %w", err)
	}

	// Process results and update metadata for hits only
	hits := 0
	misses := 0
	hitMetaKeys := make([]string, 0, len(keys))

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
					continue
				}

				result[key] = value
				hits++
				hitMetaKeys = append(hitMetaKeys, metaKeys[i])
			} else {
				misses++
			}
		} else {
			misses++
		}
	}

	// Update metadata for successful retrievals in separate pipeline
	if len(hitMetaKeys) > 0 {
		metaPipe := c.client.TxPipeline()
		now := time.Now().Unix()

		for _, metaKey := range hitMetaKeys {
			metaPipe.HIncrBy(ctx, metaKey, "access_count", 1)
			metaPipe.HSet(ctx, metaKey, "last_accessed", now)
		}

		// Execute metadata updates (ignore errors as they're not critical)
		metaPipe.Exec(ctx)
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

// SetMany stores multiple values with same TTL using optimized pipeline operations
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

	// Pre-serialize all values to catch errors early
	type setItem struct {
		key             string
		serializedValue []byte
		dataKey         string
		metaKey         string
		ownerKey        string
		indexKey        string
	}

	items := make([]setItem, 0, len(values))

	for _, value := range values {
		key := c.extractor.GetEntryKey(value)

		// Serialize value
		serializedValue, err := c.serializer.Serialize(value)
		if err != nil {
			c.metrics.RecordError("redis", "setmany", "serialization_error", "data", c.getMetricTags())
			return fmt.Errorf("serialization error for key %s: %w", key, err)
		}

		item := setItem{
			key:             key,
			serializedValue: serializedValue,
			dataKey:         c.buildDataKey(key),
			metaKey:         c.buildMetaKey(key),
		}

		// Build index keys if configured
		if c.indexingMode && c.extractor.GetOwnerKey != nil {
			item.ownerKey = c.extractor.GetOwnerKey(value)
			item.indexKey = c.buildIndexKey("owner", item.ownerKey)
		}

		items = append(items, item)
	}

	// Use optimized pipeline for batch setting
	pipe := c.client.TxPipeline()
	now := time.Now().Unix()

	// Add all operations to pipeline
	for _, item := range items {
		// Set data with TTL
		if ttl > 0 {
			pipe.SetEX(ctx, item.dataKey, item.serializedValue, ttl)
		} else {
			pipe.Set(ctx, item.dataKey, item.serializedValue, 0)
		}

		// Set metadata
		pipe.HSet(ctx, item.metaKey, map[string]any{
			"created_at":    now,
			"last_accessed": now,
			"access_count":  1,
			"ttl":           ttlToMilliseconds(ttl),
			"size":          len(item.serializedValue),
		})
		if ttl > 0 {
			pipe.Expire(ctx, item.metaKey, ttl)
		}

		// Update indexes if configured
		if c.indexingMode && item.ownerKey != "" {
			pipe.SAdd(ctx, item.indexKey, item.key)
			if ttl > 0 {
				pipe.Expire(ctx, item.indexKey, ttl)
			}
		}
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		c.handleError("setmany", err)
		c.metrics.RecordError("redis", "setmany", "redis_error", "infrastructure", c.getMetricTags())
		return fmt.Errorf("redis SetMany error: %w", err)
	}

	c.metrics.RecordBatchOperation("redis", "setmany", len(values), time.Since(start), c.getMetricTags())
	return nil
}

// DeleteMany removes multiple keys using optimized pipeline operations
func (c *RedisCache[T]) DeleteMany(ctx context.Context, keys []string) error {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "deletemany", "circuit_breaker", "availability", c.getMetricTags())
		return cacheErrors.ErrCircuitBreakerOpen
	}

	if len(keys) == 0 {
		return nil
	}

	// Use optimized pipeline for batch deletion
	pipe := c.client.TxPipeline()

	// Add all deletions to pipeline
	for _, key := range keys {
		dataKey := c.buildDataKey(key)
		metaKey := c.buildMetaKey(key)
		pipe.Del(ctx, dataKey, metaKey)
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		c.handleError("deletemany", err)
		c.metrics.RecordError("redis", "deletemany", "redis_error", "infrastructure", c.getMetricTags())
		return fmt.Errorf("redis DeleteMany error: %w", err)
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

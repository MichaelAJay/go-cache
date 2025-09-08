package cache

import (
	"context"
	"fmt"
	"time"

	cacheErrors "github.com/MichaelAJay/go-cache/cache_errors"
	"github.com/MichaelAJay/go-serializer"
)

// GetMany retrieves multiple keys efficiently using optimized pipeline operations
func (c *RedisCache[T]) GetMany(ctx context.Context, keys []string) (map[string]T, error) {
	start := time.Now()
	result := make(map[string]T, len(keys)) // Pre-size map to avoid growth reallocations

	if c.isCircuitBreakerOpen() {
		c.precomputedMetrics.GetManyCircuitBreakerErrorCounter().Inc()
		return result, cacheErrors.ErrCircuitBreakerOpen
	}

	if len(keys) == 0 {
		return result, nil
	}

	// Build Redis keys for pipeline using pooled slices and batch string building
	dataKeys := c.slicePool.GetStringSliceWithLength(len(keys))
	defer c.slicePool.PutStringSlice(dataKeys)
	metaKeys := c.slicePool.GetStringSliceWithLength(len(keys))
	defer c.slicePool.PutStringSlice(metaKeys)
	
	// Use batch key building to reduce string builder allocation overhead
	c.buildDataKeysMany(keys, dataKeys)
	c.buildMetaKeysMany(keys, metaKeys)

	// Use optimized pipeline approach
	pipe := c.client.TxPipeline()

	// Get all data values using pooled slice
	dataResults := c.slicePool.GetAnySliceWithLength(len(keys))
	defer c.slicePool.PutAnySlice(dataResults)
	for i, dataKey := range dataKeys {
		dataResults[i] = pipe.Get(ctx, dataKey)
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil && err.Error() != "redis: nil" {
		c.handleError("getmany", err)
		c.precomputedMetrics.GetManyRedisErrorCounter().Inc()
		return result, fmt.Errorf("redis GetMany error: %w", err)
	}

	// Process results and update metadata for hits only
	hits := 0
	misses := 0
	hitMetaKeys := c.slicePool.GetStringSlice(len(keys)) // Pre-allocate with capacity
	defer c.slicePool.PutStringSlice(hitMetaKeys)

	for i, key := range keys {
		if cmdResult, ok := dataResults[i].(interface {
			Val() string
			Err() error
		}); ok {
			serializedValue := cmdResult.Val()
			if cmdResult.Err() == nil && serializedValue != "" {
				// Deserialize value using StringDeserializer optimization if available
				var value T
				var err error
				if stringDeser, ok := c.serializer.(serializer.StringDeserializer); ok {
					err = stringDeser.DeserializeString(serializedValue, &value)
				} else {
					err = c.serializer.Deserialize([]byte(serializedValue), &value)
				}
				if err != nil {
					c.precomputedMetrics.GetManySerializationErrorCounter().Inc()
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

	// Update metadata for successful retrievals using Lua script for atomicity
	if len(hitMetaKeys) > 0 {
		_, err := c.getManyMetadataUpdateScript.Run(ctx, c.client, hitMetaKeys).Result()
		if err != nil {
			// Log error but don't fail the operation since metadata updates are supplementary
			c.handleError("getmany_metadata", err)
		}
	}

	// Record metrics using precomputed metrics for zero-allocation performance
	duration := time.Since(start)
	c.precomputedMetrics.GetManyTimer().Record(duration)
	c.precomputedMetrics.GetManyBatchCounter().Inc()

	// Record batch hits/misses - eliminate loops to reduce allocations
	// Note: We sacrifice granular per-key metrics for performance
	if hits > 0 {
		c.precomputedMetrics.GetManyHitCounter().Inc()
	}
	if misses > 0 {
		c.precomputedMetrics.GeneralMissCounter().Inc()
	}

	return result, nil
}

// SetMany stores multiple values with same TTL using optimized pipeline operations
func (c *RedisCache[T]) SetMany(ctx context.Context, values []T, ttl time.Duration) error {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.precomputedMetrics.SetManyCircuitBreakerErrorCounter().Inc()
		return cacheErrors.ErrCircuitBreakerOpen
	}

	if len(values) == 0 {
		return nil
	}

	// Require key extractor for SetMany
	if c.extractor.GetEntryKey == nil {
		return fmt.Errorf("IndexExtractor.GetEntryKey is required for SetMany operation")
	}

	// Pre-serialize all values and batch key building to reduce allocations
	type setItem struct {
		key             string
		serializedValue []byte
		dataKey         string
		metaKey         string
		ownerKey        string
		indexKey        string
	}

	items := make([]setItem, 0, len(values))
	keys := make([]string, len(values))

	// First pass: extract keys and serialize values
	for i, value := range values {
		key := c.extractor.GetEntryKey(value)
		keys[i] = key

		// Serialize value
		serializedValue, err := c.serializer.Serialize(value)
		if err != nil {
			c.precomputedMetrics.SetManySerializationErrorCounter().Inc()
			return fmt.Errorf("serialization error for key %s: %w", key, err)
		}

		items = append(items, setItem{
			key:             key,
			serializedValue: serializedValue,
		})
	}

	// Batch key building using pooled slices
	dataKeys := c.slicePool.GetStringSliceWithLength(len(values))
	defer c.slicePool.PutStringSlice(dataKeys)
	metaKeys := c.slicePool.GetStringSliceWithLength(len(values))
	defer c.slicePool.PutStringSlice(metaKeys)

	// Use batch key building to reduce string builder allocation overhead
	c.buildDataKeysMany(keys, dataKeys)
	c.buildMetaKeysMany(keys, metaKeys)

	// Second pass: assign batch-built keys and handle indexing
	for i := range items {
		items[i].dataKey = dataKeys[i]
		items[i].metaKey = metaKeys[i]

		// Build index keys if configured
		if c.indexingMode && c.extractor.GetOwnerKey != nil {
			items[i].ownerKey = c.extractor.GetOwnerKey(values[i])
			items[i].indexKey = c.buildIndexKey("owner", items[i].ownerKey)
		}
	}

	// Use optimized pipeline for batch setting
	pipe := c.client.TxPipeline()
	now := time.Now().Unix()
	ttlMs := ttlToMilliseconds(ttl)

	// Create single reusable metadata map to eliminate per-item map allocations
	metadataTemplate := map[string]any{
		"created_at":    now,
		"last_accessed": now,
		"access_count":  1,
		"ttl":           ttlMs,
		"size":          0, // Will be updated per item
	}

	// Add all operations to pipeline
	for _, item := range items {
		// Set data with TTL
		if ttl > 0 {
			pipe.SetEX(ctx, item.dataKey, item.serializedValue, ttl)
		} else {
			pipe.Set(ctx, item.dataKey, item.serializedValue, 0)
		}

		// Set metadata using shared template (update size field)
		metadataTemplate["size"] = len(item.serializedValue)
		pipe.HSet(ctx, item.metaKey, metadataTemplate)
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
		c.precomputedMetrics.SetManyRedisErrorCounter().Inc()
		return fmt.Errorf("redis SetMany error: %w", err)
	}

	// Record metrics using precomputed metrics for zero-allocation performance
	duration := time.Since(start)
	c.precomputedMetrics.SetManyTimer().Record(duration)
	c.precomputedMetrics.SetManyBatchCounter().Inc()
	return nil
}

// DeleteMany removes multiple keys using optimized pipeline operations
func (c *RedisCache[T]) DeleteMany(ctx context.Context, keys []string) error {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.precomputedMetrics.DeleteManyCircuitBreakerErrorCounter().Inc()
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
		c.precomputedMetrics.DeleteManyRedisErrorCounter().Inc()
		return fmt.Errorf("redis DeleteMany error: %w", err)
	}

	c.precomputedMetrics.DeleteManyTimer().Record(time.Since(start))
	c.precomputedMetrics.DeleteManyBatchCounter().Inc()

	// Apply hooks if configured
	if c.options.Hooks != nil && c.options.Hooks.PostDelete != nil {
		for _, key := range keys {
			c.options.Hooks.PostDelete(ctx, key, true, nil) // Assume all were deleted
		}
	}

	return nil
}

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
				// Deserialize value using best available method
				var value T
				var err error

				// Try StringDeserializer first (already optimized for strings)
				if stringDeser, ok := c.serializer.(serializer.StringDeserializer); ok {
					err = stringDeser.DeserializeString(serializedValue, &value)
				} else {
					// Use standard Deserialize (which for MsgPack uses pooled decoders internally)
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

// GetManyRaw retrieves multiple keys as raw serialized strings, bypassing deserialization
// This eliminates deserialization overhead for performance-critical scenarios
func (c *RedisCache[T]) GetManyRaw(ctx context.Context, keys []string) (map[string]string, error) {
	start := time.Now()
	result := make(map[string]string, len(keys))

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

	// Use batch key building to reduce string builder allocation overhead
	c.buildDataKeysMany(keys, dataKeys)

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

	// Process results - NO DESERIALIZATION
	hits := 0
	misses := 0

	for i, key := range keys {
		if cmdResult, ok := dataResults[i].(interface {
			Val() string
			Err() error
		}); ok {
			serializedValue := cmdResult.Val()
			if cmdResult.Err() == nil && serializedValue != "" {
				// NO DESERIALIZATION - just return raw string
				result[key] = serializedValue
				hits++
			} else {
				misses++
			}
		} else {
			misses++
		}
	}

	// Record metrics using precomputed metrics for zero-allocation performance
	duration := time.Since(start)
	c.precomputedMetrics.GetManyTimer().Record(duration)
	c.precomputedMetrics.GetManyBatchCounter().Inc()

	// Record batch hits/misses
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
			pipe.SetEx(ctx, item.dataKey, item.serializedValue, ttl)
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
			// Forward index (owner -> entry keys)
			pipe.SAdd(ctx, item.indexKey, item.key)
			if ttl > 0 {
				pipe.Expire(ctx, item.indexKey, ttl)
			}

			// Reverse index (entry -> owner key)
			reverseKey := c.reversePrefix + item.key
			if ttl > 0 {
				pipe.SetEx(ctx, reverseKey, item.ownerKey, ttl)
			} else {
				pipe.Set(ctx, reverseKey, item.ownerKey, 0)
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

// SetManySafe stores multiple values using pooled encoders internally but returns owned bytes
// Provides allocation reduction over individual Set calls while maintaining simple ownership
func (c *RedisCache[T]) SetManySafe(ctx context.Context, values []T, ttl time.Duration) error {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.precomputedMetrics.SetManyCircuitBreakerErrorCounter().Inc()
		return cacheErrors.ErrCircuitBreakerOpen
	}

	if len(values) == 0 {
		return nil
	}

	// Require key extractor for SetManySafe
	if c.extractor.GetEntryKey == nil {
		return fmt.Errorf("IndexExtractor.GetEntryKey is required for SetManySafe operation")
	}

	// Check if serializer supports SerializeSafe
	type safeSer interface {
		SerializeSafe(v any) ([]byte, error)
	}
	safeSerializer, hasSafe := c.serializer.(safeSer)

	// Pre-serialize all values using SerializeSafe if available, otherwise fallback to standard
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

	// First pass: extract keys and serialize values using SerializeSafe
	for i, value := range values {
		key := c.extractor.GetEntryKey(value)
		keys[i] = key

		// Use SerializeSafe if available for better performance
		var serializedValue []byte
		var err error
		if hasSafe {
			serializedValue, err = safeSerializer.SerializeSafe(value)
		} else {
			serializedValue, err = c.serializer.Serialize(value)
		}

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
			pipe.SetEx(ctx, item.dataKey, item.serializedValue, ttl)
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
			// Forward index (owner -> entry keys)
			pipe.SAdd(ctx, item.indexKey, item.key)
			if ttl > 0 {
				pipe.Expire(ctx, item.indexKey, ttl)
			}

			// Reverse index (entry -> owner key)
			reverseKey := c.reversePrefix + item.key
			if ttl > 0 {
				pipe.SetEx(ctx, reverseKey, item.ownerKey, ttl)
			} else {
				pipe.Set(ctx, reverseKey, item.ownerKey, 0)
			}
		}
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		c.handleError("setmanysafe", err)
		c.precomputedMetrics.SetManyRedisErrorCounter().Inc()
		return fmt.Errorf("redis SetManySafe error: %w", err)
	}

	// Record metrics using precomputed metrics for zero-allocation performance
	duration := time.Since(start)
	c.precomputedMetrics.SetManyTimer().Record(duration)
	c.precomputedMetrics.SetManyBatchCounter().Inc()
	return nil
}

// SetManyPooled stores multiple values using zero-copy pooled serialization
// Aggressive optimization path with maximum performance and minimal allocations
func (c *RedisCache[T]) SetManyPooled(ctx context.Context, values []T, ttl time.Duration) error {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.precomputedMetrics.SetManyCircuitBreakerErrorCounter().Inc()
		return cacheErrors.ErrCircuitBreakerOpen
	}

	if len(values) == 0 {
		return nil
	}

	// Require key extractor for SetManyPooled
	if c.extractor.GetEntryKey == nil {
		return fmt.Errorf("IndexExtractor.GetEntryKey is required for SetManyPooled operation")
	}

	// Check if serializer supports SerializePooled
	type pooledSer interface {
		SerializePooled(v any) (*serializer.PooledBuf, error)
	}
	pooledSerializer, hasPooled := c.serializer.(pooledSer)
	if !hasPooled {
		// Fallback to SetManySafe if pooled serialization not available
		return c.SetManySafe(ctx, values, ttl)
	}

	// Pre-serialize all values using SerializePooled with proper lifecycle management
	type pooledSetItem struct {
		key       string
		pooledBuf *serializer.PooledBuf
		dataKey   string
		metaKey   string
		ownerKey  string
		indexKey  string
	}

	items := make([]pooledSetItem, 0, len(values))
	keys := make([]string, len(values))

	// First pass: extract keys and serialize values using SerializePooled
	for i, value := range values {
		key := c.extractor.GetEntryKey(value)
		keys[i] = key

		// Use SerializePooled for zero-copy performance
		pooledBuf, err := pooledSerializer.SerializePooled(value)
		if err != nil {
			// Release any already-serialized buffers on error
			for _, item := range items {
				if item.pooledBuf != nil {
					item.pooledBuf.Release()
				}
			}
			c.precomputedMetrics.SetManySerializationErrorCounter().Inc()
			return fmt.Errorf("serialization error for key %s: %w", key, err)
		}

		items = append(items, pooledSetItem{
			key:       key,
			pooledBuf: pooledBuf,
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

	// Execute pipeline with pooled buffers - ensure Release is called even on errors
	defer func() {
		// Release all pooled buffers after pipeline execution
		for _, item := range items {
			if item.pooledBuf != nil {
				item.pooledBuf.Release()
			}
		}
	}()

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

	// Add all operations to pipeline using pooled buffer bytes
	for _, item := range items {
		bytes := item.pooledBuf.Bytes()

		// Set data with TTL
		if ttl > 0 {
			pipe.SetEx(ctx, item.dataKey, bytes, ttl)
		} else {
			pipe.Set(ctx, item.dataKey, bytes, 0)
		}

		// Set metadata using shared template (update size field)
		metadataTemplate["size"] = len(bytes)
		pipe.HSet(ctx, item.metaKey, metadataTemplate)
		if ttl > 0 {
			pipe.Expire(ctx, item.metaKey, ttl)
		}

		// Update indexes if configured
		if c.indexingMode && item.ownerKey != "" {
			// Forward index (owner -> entry keys)
			pipe.SAdd(ctx, item.indexKey, item.key)
			if ttl > 0 {
				pipe.Expire(ctx, item.indexKey, ttl)
			}

			// Reverse index (entry -> owner key)
			reverseKey := c.reversePrefix + item.key
			if ttl > 0 {
				pipe.SetEx(ctx, reverseKey, item.ownerKey, ttl)
			} else {
				pipe.Set(ctx, reverseKey, item.ownerKey, 0)
			}
		}
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		c.handleError("setmanypooled", err)
		c.precomputedMetrics.SetManyRedisErrorCounter().Inc()
		return fmt.Errorf("redis SetManyPooled error: %w", err)
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

	// Build all keys properly using the actual key building methods
	lruTrackerKey := c.buildLRUTrackerKey()
	indexPrefix := "cache:index:"
	if c.redisOptions != nil && c.redisOptions.IndexPrefix != "" {
		indexPrefix = c.redisOptions.IndexPrefix
	}
	indexingEnabled := fmt.Sprintf("%t", c.indexingMode)

	// Build script arguments: indexPrefix + indexingEnabled + reversePrefix + all fully-built keys
	scriptArgs := make([]interface{}, 3+len(keys)*3)
	scriptArgs[0] = indexPrefix
	scriptArgs[1] = indexingEnabled
	scriptArgs[2] = c.reversePrefix

	// Add pre-built keys for each entry
	for i, key := range keys {
		scriptArgs[3+i*3] = c.buildDataKey(key)     // dataKey
		scriptArgs[3+i*3+1] = c.buildMetaKey(key)   // metaKey
		scriptArgs[3+i*3+2] = c.reversePrefix + key // reverseKey
	}

	// Execute atomic deletion with proper index cleanup
	_, err := c.deleteManyScript.Run(ctx, c.client,
		[]string{lruTrackerKey},
		scriptArgs...).Result()
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

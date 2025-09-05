package cache

import (
	"context"
	"fmt"
	"time"

	cacheErrors "github.com/MichaelAJay/go-cache/cache_errors"
)

// GetOrSet atomically gets existing value or sets new value from loader
func (c *RedisCache[T]) GetOrSet(ctx context.Context, key string, loader func(ctx context.Context) (T, error), ttl time.Duration) (T, error) {
	start := time.Now()
	var zero T

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "getorset", "circuit_breaker", "availability", c.getMetricTags())
		return zero, cacheErrors.ErrCircuitBreakerOpen
	}

	// Use singleflight to ensure only one goroutine per key executes the loader
	result, err, _ := c.sf.Do(key, func() (interface{}, error) {
		return c.getOrSetInternal(ctx, key, loader, ttl, start)
	})

	if err != nil {
		return zero, err
	}

	return result.(T), nil
}

// getOrSetInternal implements the actual GetOrSet logic without singleflight
func (c *RedisCache[T]) getOrSetInternal(ctx context.Context, key string, loader func(ctx context.Context) (T, error), ttl time.Duration, start time.Time) (T, error) {
	var zero T

	dataKey := c.buildDataKey(key)
	lockKey := c.buildLockKey(key)
	metaKey := c.buildMetaKey(key)
	lockValue := c.instanceID + ":" + fmt.Sprintf("%d", time.Now().UnixNano())

	// Retry logic for distributed coordination
	maxRetries := lockMaxRetries
	for range maxRetries {
		// Try the Lua script for atomic GetOrSet
		result, err := c.getOrSetScript.Run(ctx, c.client, []string{lockKey, dataKey, metaKey},
			lockValue, ttlToMilliseconds(ttl), "", ttlToMilliseconds(defaultLockTimeout)).Result()

		if err != nil {
			c.handleError("getorset", err)
			c.metrics.RecordError("redis", "getorset", "redis_error", "infrastructure", c.getMetricTags())
			return zero, fmt.Errorf("redis GetOrSet error: %w", err)
		}

		resultSlice, ok := result.([]any)
		if !ok {
			// Try alternative types
			if resultSlice2, ok2 := result.([]any); ok2 {
				resultSlice = resultSlice2
				ok = true
			}
		}

		if !ok || len(resultSlice) < 2 {
			return zero, fmt.Errorf("unexpected script result format")
		}

		existingValue := resultSlice[0]
		resultCode := resultSlice[1].(string)

		shouldRetry := resultCode == "1"
		noDataAvailable := resultCode == "2"

		if shouldRetry {
			// Another process is holding the lock, wait and retry
			time.Sleep(lockRetryDelay)
			continue
		}

		if noDataAvailable {
			// Cache miss - proceed to load the value using the loader function
			// This is the normal path for GetOrSet when key doesn't exist
			c.metrics.RecordMiss("redis", c.getMetricTags())
		}

		if existingValue != nil && existingValue != "" && existingValue != false {
			// Value exists, deserialize and return it
			var value T
			if err := c.serializer.Deserialize([]byte(existingValue.(string)), &value); err != nil {
				c.metrics.RecordError("redis", "getorset", "serialization_error", "data", c.getMetricTags())
				return zero, fmt.Errorf("deserialization error for key %s: %w", key, err)
			}

			c.metrics.RecordHit("redis", c.getMetricTags())
			c.metrics.RecordOperation("redis", "getorset", "hit", time.Since(start), c.getMetricTags())
			return value, nil
		}

		// Value doesn't exist, we need to load it
		loadedValue, err := loader(ctx)
		if err != nil {
			c.metrics.RecordError("redis", "getorset", "loader_error", "application", c.getMetricTags())
			return zero, fmt.Errorf("loader function failed: %w", err)
		}

		// Serialize the loaded value
		serializedValue, err := c.serializer.Serialize(loadedValue)
		if err != nil {
			c.metrics.RecordError("redis", "getorset", "serialization_error", "data", c.getMetricTags())
			return zero, fmt.Errorf("serialization error: %w", err)
		}

		// Try the script again with the loaded value
		result, err = c.getOrSetScript.Run(ctx, c.client, []string{lockKey, dataKey, metaKey},
			lockValue, ttlToMilliseconds(ttl), string(serializedValue), ttlToMilliseconds(defaultLockTimeout)).Result()

		if err != nil {
			c.handleError("getorset", err)
			return zero, fmt.Errorf("Redis GetOrSet set error: %w", err)
		}

		c.metrics.RecordMiss("redis", c.getMetricTags())
		c.metrics.RecordOperation("redis", "getorset", "loaded", time.Since(start), c.getMetricTags())
		return loadedValue, nil
	}

	// Max retries exceeded
	c.metrics.RecordError("redis", "getorset", "max_retries_exceeded", "coordination", c.getMetricTags())
	return zero, fmt.Errorf("GetOrSet max retries exceeded for key %s", key)
}

// REMOVED: Update method has been removed due to fundamental race conditions.
//
// For truly atomic operations, use the following alternatives:
//   - Increment(ctx, key, delta) for numeric increments
//   - Decrement(ctx, key, delta) for numeric decrements  
//   - IncrementFloat(ctx, key, delta) for float increments
//   - ExtendTTL(ctx, key, ttl) for TTL extension
//   - Touch(ctx, key, ttl) for activity tracking + TTL extension
//   - AppendToField(ctx, key, fieldPath, value, ttl) for string appends
//
// For complex updates requiring read-modify-write semantics, 
// consider using optimistic concurrency patterns or accept eventual consistency.

// SetIfNotExists atomically sets value only if key doesn't exist
func (c *RedisCache[T]) SetIfNotExists(ctx context.Context, value T, ttl time.Duration) (bool, error) {
	// Extract key from value using configured extractor
	if c.extractor.GetEntryKey == nil {
		return false, fmt.Errorf("IndexExtractor.GetEntryKey is required for SetIfNotExists operation")
	}
	key := c.extractor.GetEntryKey(value)
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "setifnotexists", "circuit_breaker", "availability", c.getMetricTags())
		return false, cacheErrors.ErrCircuitBreakerOpen
	}

	// Serialize value
	serializedValue, err := c.serializer.Serialize(value)
	if err != nil {
		c.metrics.RecordError("redis", "setifnotexists", "serialization_error", "data", c.getMetricTags())
		return false, fmt.Errorf("serialization error: %w", err)
	}

	// Build all keys (some may be unused if indexing is disabled)
	dataKey := c.buildDataKey(key)
	metaKey := c.buildMetaKey(key)
	var indexKey, reverseKey string
	var ownerKey string

	indexingEnabled := c.indexingMode && c.extractor.GetOwnerKey != nil
	if indexingEnabled {
		ownerKey = c.extractor.GetOwnerKey(value)
		indexKey = c.buildIndexKey("owner", ownerKey)
		reverseKey = c.buildReverseIndexKey(key)
	}

	// Always use the same script, pass indexing flag as parameter
	result, err := c.setIfNotExistsScript.Run(ctx, c.client,
		[]string{dataKey, metaKey, indexKey, reverseKey},
		string(serializedValue),
		ttlToMilliseconds(ttl),
		key,
		ownerKey,
		fmt.Sprintf("%t", indexingEnabled)).Result()

	if err != nil {
		c.handleError("setifnotexists", err)
		c.metrics.RecordError("redis", "setifnotexists", "redis_error", "infrastructure", c.getMetricTags())
		return false, fmt.Errorf("redis SetIfNotExists error: %w", err)
	}

	wasSet := result.(int64) == 1

	c.metrics.RecordOperation("redis", "setifnotexists", "success", time.Since(start), c.getMetricTags())
	return wasSet, nil
}

// SetIfExists atomically sets value only if key exists
func (c *RedisCache[T]) SetIfExists(ctx context.Context, value T, ttl time.Duration) (bool, error) {
	// Extract key from value using configured extractor
	if c.extractor.GetEntryKey == nil {
		return false, fmt.Errorf("IndexExtractor.GetEntryKey is required for SetIfExists operation")
	}
	key := c.extractor.GetEntryKey(value)
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "setifexists", "circuit_breaker", "availability", c.getMetricTags())
		return false, cacheErrors.ErrCircuitBreakerOpen
	}

	// Serialize value
	serializedValue, err := c.serializer.Serialize(value)
	if err != nil {
		c.metrics.RecordError("redis", "setifexists", "serialization_error", "data", c.getMetricTags())
		return false, fmt.Errorf("serialization error: %w", err)
	}

	// Build all keys (some may be unused if indexing is disabled)
	dataKey := c.buildDataKey(key)
	metaKey := c.buildMetaKey(key)
	var indexKey, reverseKey string
	var ownerKey string

	indexingEnabled := c.indexingMode && c.extractor.GetOwnerKey != nil
	if indexingEnabled {
		ownerKey = c.extractor.GetOwnerKey(value)
		indexKey = c.buildIndexKey("owner", ownerKey)
		reverseKey = c.buildReverseIndexKey(key)
	}

	// Always use the same script, pass indexing flag as parameter
	result, err := c.setIfExistsScript.Run(ctx, c.client,
		[]string{dataKey, metaKey, indexKey, reverseKey},
		string(serializedValue),
		ttlToMilliseconds(ttl),
		key,
		ownerKey,
		fmt.Sprintf("%t", indexingEnabled)).Result()

	if err != nil {
		c.handleError("setifexists", err)
		c.metrics.RecordError("redis", "setifexists", "redis_error", "infrastructure", c.getMetricTags())
		return false, fmt.Errorf("redis SetIfExists error: %w", err)
	}

	wasSet := result.(int64) == 1

	c.metrics.RecordOperation("redis", "setifexists", "success", time.Since(start), c.getMetricTags())
	return wasSet, nil
}

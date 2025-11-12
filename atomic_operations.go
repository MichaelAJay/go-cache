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
		}

		if existingValue != nil && existingValue != "" && existingValue != false {
			// Value exists, deserialize and return it
			var value T
			if err := c.serializer.Deserialize([]byte(existingValue.(string)), &value); err != nil {
				return zero, fmt.Errorf("deserialization error for key %s: %w", key, err)
			}

			// TODO: Record metrics when metrics system is available (hit, duration)
			_ = time.Since(start)
			return value, nil
		}

		// Value doesn't exist, we need to load it
		loadedValue, err := loader(ctx)
		if err != nil {
			return zero, fmt.Errorf("loader function failed: %w", err)
		}

		// Serialize the loaded value
		serializedValue, err := c.serializer.Serialize(loadedValue)
		if err != nil {
			return zero, fmt.Errorf("serialization error: %w", err)
		}

		// Try the script again with the loaded value
		result, err = c.getOrSetScript.Run(ctx, c.client, []string{lockKey, dataKey, metaKey},
			lockValue, ttlToMilliseconds(ttl), string(serializedValue), ttlToMilliseconds(defaultLockTimeout)).Result()

		if err != nil {
			c.handleError("getorset", err)
			return zero, fmt.Errorf("Redis GetOrSet set error: %w", err)
		}

		// TODO: Record metrics when metrics system is available (miss, loaded, duration)
		_ = time.Since(start)
		return loadedValue, nil
	}

	// Max retries exceeded
	return zero, fmt.Errorf("GetOrSet max retries exceeded for key %s", key)
}

// RotateKey atomically rotates the cache entry key for a record, updating specific fields
// This operation uses a Lua script to perform all updates atomically in a SINGLE round-trip:
// - Gets existing entry from oldKey
// - Updates id, expires_at, last_activity fields
// - Stores updated entry at newKey with new TTL
// - Deletes oldKey
// - Rotates metadata key (preserves created_at, updates last_accessed, access_count, ttl, size)
// - Updates LRU tracker (removes old entry, adds new entry)
// - Updates forward index (removes old entry key, adds new entry key)
// - Updates reverse index (oldKey -> newKey mapping to same owner)
//
// IMPORTANT: This method currently only supports msgpack serialization because the Lua script
// uses cmsgpack.unpack/pack for atomic field updates.
func (c *RedisCache[T]) RotateKey(ctx context.Context, oldKey, newKey, newID string, newExpiresAt, newLastActivity int64, newTTL time.Duration) (T, error) {
	var zero T
	start := time.Now()

	// Validate msgpack serialization (required for Lua cmsgpack operations)
	if c.options.SerializerFormat != "msgpack" {
		return zero, fmt.Errorf("RotateKey requires msgpack serialization (current: %s). The Lua script uses cmsgpack for atomic field updates", c.options.SerializerFormat)
	}

	if c.isCircuitBreakerOpen() {
		return zero, cacheErrors.ErrCircuitBreakerOpen
	}

	// Build all required keys
	oldDataKey := c.buildDataKey(oldKey)
	newDataKey := c.buildDataKey(newKey)
	oldMetaKey := c.buildMetaKey(oldKey)
	newMetaKey := c.buildMetaKey(newKey)
	oldReverseKey := c.buildReverseIndexKey(oldKey)
	newReverseKey := c.buildReverseIndexKey(newKey)
	lruTrackerKey := c.buildLRUTrackerKey()

	// Determine indexing parameters
	// Note: Unlike Set/Delete operations, RotateKey builds the forward index key dynamically
	// within the Lua script by getting the owner from the reverse index. This is necessary
	// because the caller only has entry keys, not the full value object needed to extract
	// the owner key. The reverse index already exists and provides the owner key without
	// requiring an external GET operation.
	indexingEnabled := c.indexingMode && c.extractor.GetOwnerKey != nil
	var indexPrefix string

	if indexingEnabled {
		// Provide index prefix for Lua script to build the forward index key dynamically
		indexPrefix = "cache:index:"
		if c.redisOptions != nil && c.redisOptions.IndexPrefix != "" {
			indexPrefix = c.redisOptions.IndexPrefix
		}
	} else {
		// Provide empty prefix if indexing disabled
		indexPrefix = ""
	}

	// Execute the Lua script
	ttlMs := ttlToMilliseconds(newTTL)
	scriptResult, err := c.rotateEntryScript.Run(ctx, c.client,
		[]string{
			oldDataKey,      // KEYS[1]
			newDataKey,      // KEYS[2]
			oldMetaKey,      // KEYS[3]
			newMetaKey,      // KEYS[4]
			oldReverseKey,   // KEYS[5]
			newReverseKey,   // KEYS[6]
			lruTrackerKey,   // KEYS[7]
		},
		ttlMs,                              // ARGV[1]
		newID,                              // ARGV[2]
		newExpiresAt,                       // ARGV[3]
		newLastActivity,                    // ARGV[4]
		oldKey,                             // ARGV[5] - old entry key
		newKey,                             // ARGV[6] - new entry key
		fmt.Sprintf("%t", indexingEnabled), // ARGV[7]
		indexPrefix,                        // ARGV[8]
	).Result()

	if err != nil {
		c.handleError("rotatekey", err)
		return zero, fmt.Errorf("redis RotateKey error: %w", err)
	}

	// Deserialize the returned msgpack value
	msgpackBytes, ok := scriptResult.(string)
	if !ok {
		return zero, fmt.Errorf("unexpected script result type: %T", scriptResult)
	}

	// Deserialize using the cache's serializer
	var result T
	if err := c.serializer.Deserialize([]byte(msgpackBytes), &result); err != nil {
		return zero, fmt.Errorf("failed to deserialize rotated entry: %w", err)
	}

	// TODO: Add specific RotateKey metrics to PrecomputedCacheMetrics
	// For now, track execution time using a generic operation counter
	_ = time.Since(start) // Track duration for future metrics

	return result, nil
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
		return false, cacheErrors.ErrCircuitBreakerOpen
	}

	// Serialize value
	serializedValue, err := c.serializer.Serialize(value)
	if err != nil {
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
		return false, fmt.Errorf("redis SetIfNotExists error: %w", err)
	}

	wasSet := result.(int64) == 1

	// TODO: Record metrics when metrics system is available (success, duration)
	_ = time.Since(start)
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
		return false, cacheErrors.ErrCircuitBreakerOpen
	}

	// Serialize value
	serializedValue, err := c.serializer.Serialize(value)
	if err != nil {
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
		return false, fmt.Errorf("redis SetIfExists error: %w", err)
	}

	wasSet := result.(int64) == 1

	// TODO: Record metrics when metrics system is available (success, duration)
	_ = time.Since(start)
	return wasSet, nil
}

// CheckAndIncrement atomically checks if incrementing would exceed a limit and increments if allowed
// This is essential for rate limiting and quota enforcement without race conditions.
//
// Parameters:
//   - key: Cache key for the counter
//   - limit: Maximum allowed value (inclusive)
//   - delta: Amount to increment (typically 1 for rate limiting)
//   - ttl: Time-to-live for the counter key
//
// Returns:
//   - newValue: The value after increment (if allowed), or current value (if not allowed)
//   - allowed: true if increment was performed, false if limit would be exceeded
//   - error: Any cache operation error
//
// Behavior:
//   - If key doesn't exist, creates it with value of delta (if delta <= limit)
//   - If current + delta > limit, returns (current, false, nil) - no increment
//   - If current + delta <= limit, increments and returns (current+delta, true, nil)
//   - Sets/refreshes TTL on every successful increment
//   - MUST be atomic - no race conditions under concurrent access
func (c *RedisCache[T]) CheckAndIncrement(ctx context.Context, key string, limit int64, delta int64, ttl time.Duration) (int64, bool, error) {
	// Circuit breaker check
	if c.isCircuitBreakerOpen() {
		return 0, false, cacheErrors.ErrCircuitBreakerOpen
	}

	start := time.Now()
	dataKey := c.buildDataKey(key)

	// Execute Lua script for atomic check-and-increment
	result, err := c.checkAndIncrementScript.Run(ctx, c.client,
		[]string{dataKey},
		limit,
		delta,
		ttlToMilliseconds(ttl)).Result()

	if err != nil {
		c.handleError("check_and_increment", err)
		return 0, false, fmt.Errorf("redis CheckAndIncrement error: %w", err)
	}

	// Parse result from Lua script
	resultSlice, ok := result.([]any)
	if !ok || len(resultSlice) < 2 {
		return 0, false, fmt.Errorf("unexpected script result format")
	}

	newValue := resultSlice[0].(int64)
	allowed := resultSlice[1].(int64) == 1 // 0 if not allowed, 1 if allowed

	duration := time.Since(start)
	_ = duration // TODO: Add metrics when available

	return newValue, allowed, nil
}

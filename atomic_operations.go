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

	dataKey := c.buildDataKey(key)
	lockKey := c.buildLockKey(key)
	metaKey := c.buildMetaKey(key)
	lockValue := c.instanceID + ":" + fmt.Sprintf("%d", time.Now().UnixNano())

	// Retry logic for distributed coordination
	maxRetries := lockMaxRetries
	for range maxRetries {
		// Try the Lua script for atomic GetOrSet
		result, err := c.getOrSetScript.Run(ctx, c.client, []string{key, lockKey, dataKey, metaKey},
			lockValue, int64(ttl.Seconds()), "", int64(defaultLockTimeout.Seconds())).Result()

		if err != nil {
			c.handleError("getorset", err)
			c.metrics.RecordError("redis", "getorset", "redis_error", "infrastructure", c.getMetricTags())
			return zero, fmt.Errorf("redis GetOrSet error: %w", err)
		}

		resultSlice, ok := result.([]any)
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
			// Cache miss and no backup data source configured
			c.metrics.RecordMiss("redis", c.getMetricTags())
			c.metrics.RecordError("redis", "getorset", "no_backup_source", "configuration", c.getMetricTags())
			return zero, fmt.Errorf("cache miss for key %s and no backup data source available", key)
		}

		if existingValue != nil && existingValue != "" {
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
		result, err = c.getOrSetScript.Run(ctx, c.client, []string{key, lockKey, dataKey, metaKey},
			lockValue, int64(ttl.Seconds()), string(serializedValue), int64(defaultLockTimeout.Seconds())).Result()

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

// Update atomically updates existing value or creates new value
func (c *RedisCache[T]) Update(ctx context.Context, key string, updater func(old T, exists bool) (T, error), ttl time.Duration) (T, error) {
	start := time.Now()
	var zero T

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "update", "circuit_breaker", "availability", c.getMetricTags())
		return zero, cacheErrors.ErrCircuitBreakerOpen
	}

	dataKey := c.buildDataKey(key)
	lockKey := c.buildLockKey(key)
	metaKey := c.buildMetaKey(key)
	lockValue := c.instanceID + ":" + fmt.Sprintf("%d", time.Now().UnixNano())

	// Retry logic for distributed coordination
	maxRetries := lockMaxRetries
	for attempt := 0; attempt < maxRetries; attempt++ {
		// First, try to get the current value and acquire lock
		result, err := c.updateScript.Run(ctx, c.client, []string{key, lockKey, dataKey, metaKey},
			lockValue, int64(ttl.Seconds()), "", int64(defaultLockTimeout.Seconds())).Result()

		if err != nil && err.Error() != "NOSCRIPT" {
			c.handleError("update", err)
			c.metrics.RecordError("redis", "update", "redis_error", "infrastructure", c.getMetricTags())
			return zero, fmt.Errorf("Redis Update error: %w", err)
		}

		resultSlice, ok := result.([]interface{})
		if !ok || len(resultSlice) < 2 {
			// Lock not acquired, retry
			time.Sleep(lockRetryDelay)
			continue
		}

		shouldRetry := resultSlice[1].(string) == "1"
		if shouldRetry {
			time.Sleep(lockRetryDelay)
			continue
		}

		// We have the lock, get old value and call updater
		var oldValue T
		var exists bool

		if resultSlice[0] != nil && resultSlice[0] != "" {
			exists = true
			if err := c.serializer.Deserialize([]byte(resultSlice[0].(string)), &oldValue); err != nil {
				c.metrics.RecordError("redis", "update", "serialization_error", "data", c.getMetricTags())
				// Release lock
				c.client.Del(ctx, lockKey)
				return zero, fmt.Errorf("deserialization error: %w", err)
			}
		}

		// Call the updater function
		newValue, err := updater(oldValue, exists)
		if err != nil {
			c.metrics.RecordError("redis", "update", "updater_error", "application", c.getMetricTags())
			// Release lock
			c.client.Del(ctx, lockKey)
			return zero, fmt.Errorf("updater function failed: %w", err)
		}

		// Serialize new value
		serializedNewValue, err := c.serializer.Serialize(newValue)
		if err != nil {
			c.metrics.RecordError("redis", "update", "serialization_error", "data", c.getMetricTags())
			// Release lock
			c.client.Del(ctx, lockKey)
			return zero, fmt.Errorf("serialization error: %w", err)
		}

		// Execute the update script with the new value
		result, err = c.updateScript.Run(ctx, c.client, []string{key, lockKey, dataKey, metaKey},
			lockValue, int64(ttl.Seconds()), string(serializedNewValue), int64(defaultLockTimeout.Seconds())).Result()

		if err != nil {
			c.handleError("update", err)
			return zero, fmt.Errorf("Redis Update set error: %w", err)
		}

		c.metrics.RecordOperation("redis", "update", "success", time.Since(start), c.getMetricTags())
		return newValue, nil
	}

	// Max retries exceeded
	c.metrics.RecordError("redis", "update", "max_retries_exceeded", "coordination", c.getMetricTags())
	return zero, fmt.Errorf("Update max retries exceeded for key %s", key)
}

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

	dataKey := c.buildDataKey(key)
	metaKey := c.buildMetaKey(key)

	// Use Lua script for atomic SetIfNotExists with metadata
	result, err := c.setIfNotExistsScript.Run(ctx, c.client, 
		[]string{dataKey, metaKey}, 
		string(serializedValue), 
		int64(ttl.Seconds())).Result()

	if err != nil {
		c.handleError("setifnotexists", err)
		c.metrics.RecordError("redis", "setifnotexists", "redis_error", "infrastructure", c.getMetricTags())
		return false, fmt.Errorf("Redis SetIfNotExists error: %w", err)
	}

	wasSet := result.(int64) == 1

	// Handle indexing if the SET was successful and indexing is configured
	if wasSet && c.indexingMode && c.extractor.GetOwnerKey != nil {
		ownerKey := c.extractor.GetOwnerKey(value)
		indexKey := c.buildIndexKey("owner", ownerKey)
		reverseKey := c.buildReverseIndexKey(key)

		pipe := c.client.TxPipeline()
		pipe.SAdd(ctx, indexKey, key)
		pipe.Set(ctx, reverseKey, ownerKey, ttl)
		if ttl > 0 {
			pipe.Expire(ctx, indexKey, ttl)
		}
		pipe.Exec(ctx)
	}

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

	dataKey := c.buildDataKey(key)
	metaKey := c.buildMetaKey(key)

	// Use Lua script for atomic SetIfExists with metadata
	result, err := c.setIfExistsScript.Run(ctx, c.client, 
		[]string{dataKey, metaKey}, 
		string(serializedValue), 
		int64(ttl.Seconds())).Result()

	if err != nil {
		c.handleError("setifexists", err)
		c.metrics.RecordError("redis", "setifexists", "redis_error", "infrastructure", c.getMetricTags())
		return false, fmt.Errorf("Redis SetIfExists error: %w", err)
	}

	wasSet := result.(int64) == 1

	// Handle indexing if the SET was successful and indexing is configured
	// Note: For SetIfExists, we may need to handle owner changes
	if wasSet && c.indexingMode && c.extractor.GetOwnerKey != nil {
		ownerKey := c.extractor.GetOwnerKey(value)
		indexKey := c.buildIndexKey("owner", ownerKey)
		reverseKey := c.buildReverseIndexKey(key)

		// For existing entries, we should handle potential owner changes
		pipe := c.client.TxPipeline()
		
		// Get old owner key to clean up old indexes
		oldOwnerResult := pipe.Get(ctx, reverseKey)
		pipe.Exec(ctx)
		
		oldOwner := oldOwnerResult.Val()
		if oldOwner != "" && oldOwner != ownerKey {
			// Owner changed, remove from old index
			oldIndexKey := c.buildIndexKey("owner", oldOwner)
			c.client.SRem(ctx, oldIndexKey, key)
		}

		// Add to new owner index and update reverse mapping
		pipe = c.client.TxPipeline()
		pipe.SAdd(ctx, indexKey, key)
		pipe.Set(ctx, reverseKey, ownerKey, ttl)
		if ttl > 0 {
			pipe.Expire(ctx, indexKey, ttl)
		}
		pipe.Exec(ctx)
	}

	c.metrics.RecordOperation("redis", "setifexists", "success", time.Since(start), c.getMetricTags())
	return wasSet, nil
}

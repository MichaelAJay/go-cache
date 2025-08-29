package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	cacheErrors "github.com/MichaelAJay/go-cache/cache_errors"
)

// GetOrSet atomically gets existing value or sets new value from loader
func (c *redisCache[T]) GetOrSet(ctx context.Context, key string, loader func(ctx context.Context) (T, error), ttl time.Duration) (T, error) {
	start := time.Now()
	var zero T
	
	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "getorset", "circuit_breaker", "availability", c.getMetricTags())
		return zero, cacheErrors.ErrCircuitBreakerOpen
	}
	
	// Apply security timing protection if enabled
	defer func() {
		if c.options.Security != nil && c.options.Security.EnableTimingProtection {
			c.applyTimingProtection("getorset", start)
		}
	}()
	
	dataKey := c.buildDataKey(key)
	lockKey := c.buildLockKey(key)
	metaKey := c.buildMetaKey(key)
	lockValue := c.instanceID + ":" + fmt.Sprintf("%d", time.Now().UnixNano())
	
	// Retry logic for distributed coordination
	maxRetries := lockMaxRetries
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Try the Lua script for atomic GetOrSet
		result, err := c.getOrSetScript.Run(ctx, c.client, []string{key, lockKey, dataKey, metaKey}, 
			lockValue, int64(ttl.Seconds()), "", int64(defaultLockTimeout.Seconds())).Result()
		
		if err != nil {
			c.handleError("getorset", err)
			c.metrics.RecordError("redis", "getorset", "redis_error", "infrastructure", c.getMetricTags())
			return zero, fmt.Errorf("Redis GetOrSet error: %w", err)
		}
		
		resultSlice, ok := result.([]interface{})
		if !ok || len(resultSlice) < 2 {
			return zero, fmt.Errorf("unexpected script result format")
		}
		
		existingValue := resultSlice[0]
		shouldRetry := resultSlice[1].(string) == "1"
		
		if shouldRetry {
			// Another process is holding the lock, wait and retry
			time.Sleep(lockRetryDelay)
			continue
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
func (c *redisCache[T]) Update(ctx context.Context, key string, updater func(old T, exists bool) (T, error), ttl time.Duration) (T, error) {
	start := time.Now()
	var zero T
	
	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "update", "circuit_breaker", "availability", c.getMetricTags())
		return zero, cacheErrors.ErrCircuitBreakerOpen
	}
	
	// Apply security timing protection if enabled
	defer func() {
		if c.options.Security != nil && c.options.Security.EnableTimingProtection {
			c.applyTimingProtection("update", start)
		}
	}()
	
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
func (c *redisCache[T]) SetIfNotExists(ctx context.Context, key string, value T, ttl time.Duration) (bool, error) {
	start := time.Now()
	
	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "setifnotexists", "circuit_breaker", "availability", c.getMetricTags())
		return false, cacheErrors.ErrCircuitBreakerOpen
	}
	
	// Apply security timing protection if enabled
	defer func() {
		if c.options.Security != nil && c.options.Security.EnableTimingProtection {
			c.applyTimingProtection("setifnotexists", start)
		}
	}()
	
	// Serialize value
	serializedValue, err := c.serializer.Serialize(value)
	if err != nil {
		c.metrics.RecordError("redis", "setifnotexists", "serialization_error", "data", c.getMetricTags())
		return false, fmt.Errorf("serialization error: %w", err)
	}
	
	dataKey := c.buildDataKey(key)
	metaKey := c.buildMetaKey(key)
	
	// Use Redis SETNX (SET if Not eXists)
	var wasSet bool
	if ttl > 0 {
		// Use SET with NX and EX options for atomic operation with TTL
		result := c.client.Set(ctx, dataKey, serializedValue, ttl)
		wasSet = result.Err() == nil && result.Val() == "OK"
	} else {
		// Use SETNX for atomic operation without TTL
		result, err := c.client.SetNX(ctx, dataKey, serializedValue, 0).Result()
		if err != nil {
			c.handleError("setifnotexists", err)
			c.metrics.RecordError("redis", "setifnotexists", "redis_error", "infrastructure", c.getMetricTags())
			return false, fmt.Errorf("Redis SetIfNotExists error: %w", err)
		}
		wasSet = result
	}
	
	if wasSet {
		// Set metadata
		now := time.Now().Unix()
		c.client.HMSet(ctx, metaKey, map[string]interface{}{
			"created_at":    now,
			"last_accessed": now,
			"access_count":  1,
			"ttl":           int64(ttl.Seconds()),
			"size":          len(serializedValue),
		})
		
		if ttl > 0 {
			c.client.Expire(ctx, metaKey, ttl)
		}
	}
	
	c.metrics.RecordOperation("redis", "setifnotexists", "success", time.Since(start), c.getMetricTags())
	return wasSet, nil
}

// SetIfExists atomically sets value only if key exists
func (c *redisCache[T]) SetIfExists(ctx context.Context, key string, value T, ttl time.Duration) (bool, error) {
	start := time.Now()
	
	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "setifexists", "circuit_breaker", "availability", c.getMetricTags())
		return false, cacheErrors.ErrCircuitBreakerOpen
	}
	
	// Apply security timing protection if enabled
	defer func() {
		if c.options.Security != nil && c.options.Security.EnableTimingProtection {
			c.applyTimingProtection("setifexists", start)
		}
	}()
	
	// Serialize value
	serializedValue, err := c.serializer.Serialize(value)
	if err != nil {
		c.metrics.RecordError("redis", "setifexists", "serialization_error", "data", c.getMetricTags())
		return false, fmt.Errorf("serialization error: %w", err)
	}
	
	dataKey := c.buildDataKey(key)
	metaKey := c.buildMetaKey(key)
	
	// Use Redis SET with XX option (SET if eXists)
	var cmd *redis.StatusCmd
	if ttl > 0 {
		cmd = c.client.Set(ctx, dataKey, serializedValue, ttl)
	} else {
		cmd = c.client.Set(ctx, dataKey, serializedValue, 0)
	}
	
	result := cmd.Val()
	wasSet := result == "OK"
	
	if wasSet {
		// Update metadata
		now := time.Now().Unix()
		accessCount, _ := c.client.HIncrBy(ctx, metaKey, "access_count", 1).Result()
		
		c.client.HMSet(ctx, metaKey, map[string]interface{}{
			"last_accessed": now,
			"access_count":  accessCount,
			"ttl":           int64(ttl.Seconds()),
			"size":          len(serializedValue),
		})
		
		if ttl > 0 {
			c.client.Expire(ctx, metaKey, ttl)
		}
	}
	
	c.metrics.RecordOperation("redis", "setifexists", "success", time.Since(start), c.getMetricTags())
	return wasSet, nil
}
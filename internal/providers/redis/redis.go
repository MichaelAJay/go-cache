package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"

	cacheErrors "github.com/MichaelAJay/go-cache/cache_errors"
	"github.com/MichaelAJay/go-cache/config"
	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/MichaelAJay/go-cache/metrics"
	"github.com/MichaelAJay/go-metrics/metric"
	"github.com/MichaelAJay/go-serializer"
)

const (
	// Redis key prefixes
	dataPrefix  = "cache:data:"
	indexPrefix = "cache:index:"
	metaPrefix  = "cache:meta:"
	lockPrefix  = "cache:lock:"

	// Lock configuration
	defaultLockTimeout = 30 * time.Second
	lockRetryDelay     = 10 * time.Millisecond
	lockMaxRetries     = 100

	// Circuit breaker thresholds
	circuitBreakerThreshold = 10
	circuitBreakerTimeout   = 60 * time.Second
)

// redisCache implements the Cache[T] interface with distributed Redis backend
type redisCache[T any] struct {
	client     redis.Cmdable
	serializer serializer.Serializer
	options    *config.CacheOptions
	metrics    metrics.EnhancedCacheMetrics

	// Circuit breaker state
	mu                 sync.RWMutex
	circuitBreakerOpen bool
	lastFailureTime    time.Time
	failureCount       int

	// Instance identifier for distributed coordination
	instanceID string

	// Lua scripts for atomic operations
	getOrSetScript        *redis.Script
	updateScript          *redis.Script
	deleteByIndexScript   *redis.Script
	deleteByPatternScript *redis.Script
}

// NewRedisCache creates a new Redis cache instance with an injected Redis client
func NewRedisCache[T any](client redis.Cmdable, options *config.CacheOptions) (interfaces.Cache[T], error) {
	if client == nil {
		return nil, fmt.Errorf("Redis client cannot be nil")
	}
	if options == nil {
		options = config.DefaultOptions()
	}

	// Initialize serializer
	var ser serializer.Serializer
	if options.SerializerFormat != "" {
		switch options.SerializerFormat {
		case "json":
			ser = serializer.NewJSONSerializer()
		case "gob", "binary":
			ser = serializer.NewGobSerializer()
		case "msgpack":
			ser = serializer.NewMsgpackSerializer()
		default:
			return nil, fmt.Errorf("unsupported serializer format: %s", options.SerializerFormat)
		}
	} else {
		ser = serializer.NewMsgpackSerializer() // Default to msgpack for Redis
	}

	// Generate unique instance ID for this cache instance
	instanceID := generateInstanceID()

	// Initialize metrics
	metricsImpl := options.EnhancedMetrics
	if metricsImpl == nil && options.GoMetricsRegistry != nil {
		metricsImpl = metrics.NewEnhancedCacheMetrics(options.GoMetricsRegistry, options.GlobalMetricsTags)
	}
	if metricsImpl == nil {
		metricsImpl = metrics.NewNoopEnhancedCacheMetrics()
	}

	cache := &redisCache[T]{
		client:     client,
		serializer: ser,
		options:    options,
		metrics:    metricsImpl,
		instanceID: instanceID,
	}

	// Initialize Lua scripts for atomic operations
	cache.initLuaScripts()

	return cache, nil
}

// initLuaScripts initializes Lua scripts for atomic operations
func (c *redisCache[T]) initLuaScripts() {
	// GetOrSet script - atomically get existing value or execute loader
	c.getOrSetScript = redis.NewScript(`
		local key = KEYS[1]
		local lockKey = KEYS[2] 
		local dataKey = KEYS[3]
		local metaKey = KEYS[4]
		local lockValue = ARGV[1]
		local ttl = tonumber(ARGV[2])
		local serializedValue = ARGV[3]
		local lockTimeout = tonumber(ARGV[4])
		
		-- Try to get existing value first
		local existingValue = redis.call('GET', dataKey)
		if existingValue then
			return {existingValue, '0'} -- Value exists, no need to set
		end
		
		-- Try to acquire lock
		local lockAcquired = redis.call('SET', lockKey, lockValue, 'EX', lockTimeout, 'NX')
		if not lockAcquired then
			-- Lock not acquired, check again for value (in case another process set it)
			existingValue = redis.call('GET', dataKey)
			if existingValue then
				return {existingValue, '0'}
			end
			return {nil, '1'} -- Signal to retry
		end
		
		-- Lock acquired, set the value
		if ttl > 0 then
			redis.call('SETEX', dataKey, ttl, serializedValue)
		else
			redis.call('SET', dataKey, serializedValue)
		end
		
		-- Set metadata
		local now = redis.call('TIME')
		local timestamp = now[1]
		redis.call('HMSET', metaKey, 
			'created_at', timestamp,
			'last_accessed', timestamp,
			'access_count', '1',
			'ttl', ttl,
			'size', string.len(serializedValue)
		)
		
		if ttl > 0 then
			redis.call('EXPIRE', metaKey, ttl)
		end
		
		-- Release lock
		redis.call('DEL', lockKey)
		
		return {serializedValue, '0'}
	`)

	// Update script - atomically update existing value
	c.updateScript = redis.NewScript(`
		local key = KEYS[1]
		local lockKey = KEYS[2]
		local dataKey = KEYS[3] 
		local metaKey = KEYS[4]
		local lockValue = ARGV[1]
		local ttl = tonumber(ARGV[2])
		local newSerializedValue = ARGV[3]
		local lockTimeout = tonumber(ARGV[4])
		
		-- Try to acquire lock
		local lockAcquired = redis.call('SET', lockKey, lockValue, 'EX', lockTimeout, 'NX')
		if not lockAcquired then
			return {nil, '1'} -- Signal to retry
		end
		
		-- Get existing value
		local oldValue = redis.call('GET', dataKey)
		local exists = oldValue and '1' or '0'
		
		-- Set new value
		if ttl > 0 then
			redis.call('SETEX', dataKey, ttl, newSerializedValue)
		else
			redis.call('SET', dataKey, newSerializedValue)
		end
		
		-- Update metadata
		local now = redis.call('TIME')
		local timestamp = now[1]
		local accessCount = redis.call('HGET', metaKey, 'access_count') or '0'
		accessCount = tostring(tonumber(accessCount) + 1)
		
		redis.call('HMSET', metaKey,
			'last_accessed', timestamp,
			'access_count', accessCount,
			'ttl', ttl,
			'size', string.len(newSerializedValue)
		)
		
		if ttl > 0 then
			redis.call('EXPIRE', metaKey, ttl)
		end
		
		-- Release lock
		redis.call('DEL', lockKey)
		
		return {oldValue, exists, newSerializedValue}
	`)

	// Delete by index script
	c.deleteByIndexScript = redis.NewScript(`
		local indexKey = KEYS[1]
		local dataPrefix = ARGV[1]
		local metaPrefix = ARGV[2]
		
		-- Get all keys from index
		local keys = redis.call('SMEMBERS', indexKey)
		local deletedCount = 0
		
		for i = 1, #keys do
			local key = keys[i]
			local dataKey = dataPrefix .. key
			local metaKey = metaPrefix .. key
			
			-- Delete data and metadata
			local deleted = redis.call('DEL', dataKey, metaKey)
			if deleted > 0 then
				deletedCount = deletedCount + 1
			end
		end
		
		-- Clear the index
		redis.call('DEL', indexKey)
		
		return deletedCount
	`)

	// Delete by pattern script
	c.deleteByPatternScript = redis.NewScript(`
		local pattern = ARGV[1]
		local dataPrefix = ARGV[2]
		local metaPrefix = ARGV[3]
		
		-- Get all matching keys
		local keys = redis.call('KEYS', pattern)
		local deletedCount = 0
		
		for i = 1, #keys do
			local fullKey = keys[i]
			-- Extract the actual cache key (remove prefix)
			local key = string.sub(fullKey, string.len(dataPrefix) + 1)
			local metaKey = metaPrefix .. key
			
			-- Delete data and metadata
			local deleted = redis.call('DEL', fullKey, metaKey)
			if deleted > 0 then
				deletedCount = deletedCount + 1
			end
		end
		
		return deletedCount
	`)
}

// generateInstanceID creates a unique identifier for this cache instance
func generateInstanceID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// Basic Operations

// Get retrieves a value by key
func (c *redisCache[T]) Get(ctx context.Context, key string) (T, bool, error) {
	start := time.Now()
	var zero T

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "get", "circuit_breaker", "availability", c.getMetricTags())
		return zero, false, cacheErrors.ErrCircuitBreakerOpen
	}


	dataKey := c.buildDataKey(key)
	metaKey := c.buildMetaKey(key)

	// Get data and update metadata atomically
	pipe := c.client.TxPipeline()
	getResult := pipe.Get(ctx, dataKey)
	pipe.HIncrBy(ctx, metaKey, "access_count", 1)
	pipe.HSet(ctx, metaKey, "last_accessed", time.Now().Unix())

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		c.handleError("get", err)
		c.metrics.RecordError("redis", "get", "redis_error", "infrastructure", c.getMetricTags())
		return zero, false, fmt.Errorf("Redis get error: %w", err)
	}

	serializedValue := getResult.Val()
	if serializedValue == "" {
		c.metrics.RecordMiss("redis", c.getMetricTags())
		c.metrics.RecordOperation("redis", "get", "miss", time.Since(start), c.getMetricTags())
		return zero, false, nil
	}

	// Deserialize value
	var value T
	if err := c.serializer.Deserialize([]byte(serializedValue), &value); err != nil {
		c.metrics.RecordError("redis", "get", "serialization_error", "data", c.getMetricTags())
		return zero, false, fmt.Errorf("deserialization error for key %s: %w", key, err)
	}

	c.metrics.RecordHit("redis", c.getMetricTags())
	c.metrics.RecordOperation("redis", "get", "success", time.Since(start), c.getMetricTags())
	return value, true, nil
}

// Set stores a value with TTL
func (c *redisCache[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "set", "circuit_breaker", "availability", c.getMetricTags())
		return cacheErrors.ErrCircuitBreakerOpen
	}


	// Serialize value
	serializedValue, err := c.serializer.Serialize(value)
	if err != nil {
		c.metrics.RecordError("redis", "set", "serialization_error", "data", c.getMetricTags())
		return fmt.Errorf("serialization error for key %s: %w", key, err)
	}

	dataKey := c.buildDataKey(key)
	metaKey := c.buildMetaKey(key)

	// Set data and metadata atomically
	pipe := c.client.TxPipeline()

	if ttl > 0 {
		pipe.SetEX(ctx, dataKey, serializedValue, ttl)
		pipe.Expire(ctx, metaKey, ttl)
	} else {
		pipe.Set(ctx, dataKey, serializedValue, 0)
	}

	// Set metadata
	now := time.Now().Unix()
	pipe.HMSet(ctx, metaKey, map[string]interface{}{
		"created_at":    now,
		"last_accessed": now,
		"access_count":  1,
		"ttl":           int64(ttl.Seconds()),
		"size":          len(serializedValue),
	})

	_, err = pipe.Exec(ctx)
	if err != nil {
		c.handleError("set", err)
		c.metrics.RecordError("redis", "set", "redis_error", "infrastructure", c.getMetricTags())
		return fmt.Errorf("Redis set error: %w", err)
	}

	c.metrics.RecordOperation("redis", "set", "success", time.Since(start), c.getMetricTags())
	return nil
}

// Delete removes a key
func (c *redisCache[T]) Delete(ctx context.Context, key string) error {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "delete", "circuit_breaker", "availability", c.getMetricTags())
		return cacheErrors.ErrCircuitBreakerOpen
	}

	dataKey := c.buildDataKey(key)
	metaKey := c.buildMetaKey(key)

	// Delete data and metadata atomically
	deleted, err := c.client.Del(ctx, dataKey, metaKey).Result()
	if err != nil {
		c.handleError("delete", err)
		c.metrics.RecordError("redis", "delete", "redis_error", "infrastructure", c.getMetricTags())
		return fmt.Errorf("Redis delete error: %w", err)
	}

	// Remove from any indexes
	c.removeFromIndexes(ctx, key)

	c.metrics.RecordOperation("redis", "delete", "success", time.Since(start), c.getMetricTags())

	// Apply hooks if configured
	if c.options.Hooks != nil && c.options.Hooks.PostDelete != nil {
		c.options.Hooks.PostDelete(ctx, key, deleted > 0, nil)
	}

	return nil
}

// Clear removes all entries
func (c *redisCache[T]) Clear(ctx context.Context) error {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "clear", "circuit_breaker", "availability", c.getMetricTags())
		return cacheErrors.ErrCircuitBreakerOpen
	}

	// Get all data keys
	dataPattern := c.buildDataKey("*")
	dataKeys, err := c.client.Keys(ctx, dataPattern).Result()
	if err != nil {
		c.handleError("clear", err)
		return fmt.Errorf("Redis clear error getting keys: %w", err)
	}

	if len(dataKeys) == 0 {
		return nil
	}

	// Also get metadata and index keys to clear
	metaPattern := c.buildMetaKey("*")
	metaKeys, _ := c.client.Keys(ctx, metaPattern).Result()

	indexPattern := c.buildIndexKey("*", "*")
	indexKeys, _ := c.client.Keys(ctx, indexPattern).Result()

	// Combine all keys to delete
	allKeys := append(dataKeys, metaKeys...)
	allKeys = append(allKeys, indexKeys...)

	// Delete in batches to avoid blocking Redis
	batchSize := 100
	for i := 0; i < len(allKeys); i += batchSize {
		end := i + batchSize
		if end > len(allKeys) {
			end = len(allKeys)
		}

		if err := c.client.Del(ctx, allKeys[i:end]...).Err(); err != nil {
			c.handleError("clear", err)
			return fmt.Errorf("Redis clear error deleting batch: %w", err)
		}
	}

	c.metrics.RecordOperation("redis", "clear", "success", time.Since(start), c.getMetricTags())
	return nil
}

// Has checks if key exists without retrieving value
func (c *redisCache[T]) Has(ctx context.Context, key string) bool {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "has", "circuit_breaker", "availability", c.getMetricTags())
		return false
	}

	dataKey := c.buildDataKey(key)
	exists, err := c.client.Exists(ctx, dataKey).Result()
	if err != nil {
		c.handleError("has", err)
		c.metrics.RecordError("redis", "has", "redis_error", "infrastructure", c.getMetricTags())
		return false
	}

	c.metrics.RecordOperation("redis", "has", "success", time.Since(start), c.getMetricTags())
	return exists > 0
}

// Atomic Operations Implementation continues...
// (Due to length limits, I'll continue with the remaining methods in the next part)

// Helper methods

// buildDataKey constructs the Redis key for data storage
func (c *redisCache[T]) buildDataKey(key string) string {
	return dataPrefix + key
}

// buildMetaKey constructs the Redis key for metadata storage
func (c *redisCache[T]) buildMetaKey(key string) string {
	return metaPrefix + key
}

// buildIndexKey constructs the Redis key for index storage
func (c *redisCache[T]) buildIndexKey(indexName, indexKey string) string {
	return fmt.Sprintf("%s%s:%s", indexPrefix, indexName, indexKey)
}

// buildLockKey constructs the Redis key for distributed locks
func (c *redisCache[T]) buildLockKey(key string) string {
	return lockPrefix + key
}

// getMetricTags returns metric tags for this cache instance
func (c *redisCache[T]) getMetricTags() metric.Tags {
	tags := make(metric.Tags)
	if c.options.GlobalMetricsTags != nil {
		for k, v := range c.options.GlobalMetricsTags {
			tags[k] = v
		}
	}
	tags["provider"] = "redis"
	tags["instance_id"] = c.instanceID
	return tags
}

// Circuit breaker implementation

// isCircuitBreakerOpen checks if the circuit breaker is open
func (c *redisCache[T]) isCircuitBreakerOpen() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.circuitBreakerOpen {
		return false
	}

	// Check if timeout has passed
	if time.Since(c.lastFailureTime) > circuitBreakerTimeout {
		c.mu.RUnlock()
		c.mu.Lock()
		c.circuitBreakerOpen = false
		c.failureCount = 0
		c.mu.Unlock()
		c.mu.RLock()
		return false
	}

	return true
}

// handleError processes errors and manages circuit breaker state
func (c *redisCache[T]) handleError(operation string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.failureCount++
	c.lastFailureTime = time.Now()

	if c.failureCount >= circuitBreakerThreshold {
		c.circuitBreakerOpen = true
		c.metrics.RecordSecurityEvent("redis", "circuit_breaker_opened", "warning", c.getMetricTags())
	}
}


// removeFromIndexes removes a key from all relevant indexes
func (c *redisCache[T]) removeFromIndexes(ctx context.Context, key string) {
	// This is a simplified version - in a full implementation,
	// we would track which indexes contain which keys
	if c.options.Indexes != nil {
		for indexName, keyPattern := range c.options.Indexes {
			// Check if key matches pattern
			if matched, _ := filepath.Match(keyPattern, key); matched {
				// Remove from all index keys (this is inefficient but correct)
				// In a full implementation, we'd maintain reverse indexes
				indexPattern := c.buildIndexKey(indexName, "*")
				indexKeys, _ := c.client.Keys(ctx, indexPattern).Result()

				for _, indexKey := range indexKeys {
					c.client.SRem(ctx, indexKey, key)
				}
			}
		}
	}
}

// Lifecycle management

// Close shuts down the cache and cleans up resources
func (c *redisCache[T]) Close() error {
	if rdb, ok := c.client.(*redis.Client); ok {
		return rdb.Close()
	}
	return nil
}

package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gometrics "github.com/MichaelAJay/go-metrics"
	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/metrics"
	"github.com/MichaelAJay/go-logger"
	"github.com/MichaelAJay/go-serializer"
	"github.com/go-redis/redis/v8"
)

const (
	// Key prefixes and separators
	keyPrefix      = "cache:"
	metadataPrefix = "cache:meta:"
	keySeparator   = ":"

	// Default values
	defaultKeyTTL = time.Hour * 24 * 7 // 1 week
)

// redisCache implements the Cache interface using Redis
type redisCache struct {
	client     *redis.Client
	options    *cache.CacheOptions
	serializer serializer.Serializer
	
	// Metrics - support both old and new systems
	legacyMetrics   *cacheMetrics                // Legacy metrics for backward compatibility
	enhancedMetrics cache.EnhancedCacheMetrics   // New go-metrics based system
	providerName    string                       // Provider name for metrics tagging
}

// cacheMetrics implements a thread-safe metrics collector
type cacheMetrics struct {
	hits          int64
	misses        int64
	getLatency    time.Duration
	setLatency    time.Duration
	deleteLatency time.Duration
	mu            sync.RWMutex
}

// metadataEntry represents Redis cache entry metadata
type metadataEntry struct {
	CreatedAt    time.Time     `json:"created_at"`
	LastAccessed time.Time     `json:"last_accessed"`
	AccessCount  int64         `json:"access_count"`
	TTL          time.Duration `json:"ttl"`
	Size         int64         `json:"size"`
	Tags         []string      `json:"tags"`
}

// RedisCache exposes the Redis cache implementation
type RedisCache struct {
	*redisCache
}

// NewRedisCache creates a new Redis cache instance with an existing client
func NewRedisCache(client *redis.Client, options *cache.CacheOptions) (cache.Cache, error) {
	// Validate client
	if client == nil {
		return nil, fmt.Errorf("redis client is required")
	}

	// Determine serializer format
	format := serializer.Msgpack // Default to MessagePack for better performance
	if options != nil && options.SerializerFormat != "" {
		format = options.SerializerFormat
	}

	// Get serializer from default registry
	s, err := serializer.DefaultRegistry.New(format)
	if err != nil {
		return nil, fmt.Errorf("failed to create serializer: %w", err)
	}

	// Initialize metrics systems
	if options == nil {
		options = &cache.CacheOptions{}
	}
	
	legacyMetrics := &cacheMetrics{}
	
	// Determine which metrics system to use
	var enhancedMetrics cache.EnhancedCacheMetrics
	if options.EnhancedMetrics != nil {
		enhancedMetrics = options.EnhancedMetrics
	} else if options.GoMetricsRegistry != nil {
		enhancedMetrics = metrics.NewEnhancedCacheMetrics(options.GoMetricsRegistry, options.GlobalMetricsTags)
	} else {
		// Use no-op metrics if disabled or no registry provided
		if options.MetricsEnabled == false {
			enhancedMetrics = metrics.NewNoopEnhancedCacheMetrics()
		} else {
			// Create default registry
			registry := gometrics.NewRegistry()
			enhancedMetrics = metrics.NewEnhancedCacheMetrics(registry, options.GlobalMetricsTags)
		}
	}

	// Create Redis cache
	c := &redisCache{
		client:          client,
		options:         options,
		serializer:      s,
		legacyMetrics:   legacyMetrics,
		enhancedMetrics: enhancedMetrics,
		providerName:    "redis",
	}

	// Check connection to Redis
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{c}, nil
}

// NewRedisCacheWithConfig creates a new Redis cache instance with its own client
func NewRedisCacheWithConfig(config *redis.Options, options *cache.CacheOptions) (cache.Cache, error) {
	// Create Redis client
	client := redis.NewClient(config)

	// Create cache with the new client
	return NewRedisCache(client, options)
}

// formatKey creates a properly formatted key with prefix
func formatKey(key string) string {
	return keyPrefix + key
}

// formatMetadataKey creates a properly formatted metadata key
func formatMetadataKey(key string) string {
	return metadataPrefix + key
}

// Get retrieves a value from the cache
func (c *redisCache) Get(ctx context.Context, key string) (any, bool, error) {
	start := time.Now()
	defer func() {
		c.recordGetLatency(time.Since(start))
	}()

	// Validate key
	if key == "" {
		return nil, false, cache.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, false, cache.ErrContextCanceled
	}

	// Get value from Redis
	formattedKey := formatKey(key)
	data, err := c.client.Get(ctx, formattedKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			c.recordMiss()
			return nil, false, nil
		}
		return nil, false, err
	}

	// Deserialize value
	var value any
	if err := c.serializer.Deserialize(data, &value); err != nil {
		return nil, false, cache.ErrDeserialization
	}

	// Update metadata
	c.updateAccessMetadata(ctx, key)

	c.recordHit()
	return value, true, nil
}

// Set stores a value in the cache
func (c *redisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		c.recordSetLatency(time.Since(start))
	}()

	// Validate key
	if key == "" {
		return cache.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return cache.ErrContextCanceled
	}

	// Use default TTL if not specified
	if ttl == 0 {
		ttl = c.options.TTL
	}
	if ttl <= 0 {
		ttl = defaultKeyTTL
	}

	// Serialize value
	data, err := c.serializer.Serialize(value)
	if err != nil {
		return cache.ErrSerialization
	}

	// Store value in Redis
	formattedKey := formatKey(key)
	if err := c.client.Set(ctx, formattedKey, data, ttl).Err(); err != nil {
		return err
	}

	// Store metadata
	if err := c.storeMetadata(ctx, key, len(data), ttl); err != nil {
		// Non-fatal, just log if logger is available
		if c.options.Logger != nil {
			c.options.Logger.Error("Failed to store metadata",
				logger.Field{Key: "error", Value: err.Error()})
		}
	}

	return nil
}

// Delete removes a value from the cache
func (c *redisCache) Delete(ctx context.Context, key string) error {
	start := time.Now()
	defer func() {
		c.recordDeleteLatency(time.Since(start))
	}()

	// Validate key
	if key == "" {
		return cache.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return cache.ErrContextCanceled
	}

	// Remove value from Redis
	formattedKey := formatKey(key)
	if err := c.client.Del(ctx, formattedKey).Err(); err != nil {
		return err
	}

	// Remove metadata
	metaKey := formatMetadataKey(key)
	if err := c.client.Del(ctx, metaKey).Err(); err != nil {
		// Non-fatal, just log if logger is available
		if c.options.Logger != nil {
			c.options.Logger.Error("Failed to delete metadata",
				logger.Field{Key: "error", Value: err.Error()})
		}
	}

	return nil
}

// Clear removes all values from the cache with our prefix
func (c *redisCache) Clear(ctx context.Context) error {
	// Check for context cancellation
	if ctx.Err() != nil {
		return cache.ErrContextCanceled
	}

	// Use scan to find all keys matching our prefix
	var cursor uint64
	var err error
	var keys []string

	for {
		var scanKeys []string
		scanKeys, cursor, err = c.client.Scan(ctx, cursor, keyPrefix+"*", 100).Result()
		if err != nil {
			return err
		}

		keys = append(keys, scanKeys...)

		if cursor == 0 {
			break
		}
	}

	// Delete found keys in batches
	if len(keys) > 0 {
		// Delete in batches of 100
		for i := 0; i < len(keys); i += 100 {
			end := i + 100
			if end > len(keys) {
				end = len(keys)
			}
			batch := keys[i:end]
			if err := c.client.Del(ctx, batch...).Err(); err != nil {
				return err
			}
		}
	}

	// Also clear metadata keys
	cursor = 0
	keys = []string{}

	for {
		var scanKeys []string
		scanKeys, cursor, err = c.client.Scan(ctx, cursor, metadataPrefix+"*", 100).Result()
		if err != nil {
			return err
		}

		keys = append(keys, scanKeys...)

		if cursor == 0 {
			break
		}
	}

	// Delete found metadata keys in batches
	if len(keys) > 0 {
		// Delete in batches of 100
		for i := 0; i < len(keys); i += 100 {
			end := i + 100
			if end > len(keys) {
				end = len(keys)
			}
			batch := keys[i:end]
			if err := c.client.Del(ctx, batch...).Err(); err != nil {
				return err
			}
		}
	}

	return nil
}

// Has checks if a key exists in the cache
func (c *redisCache) Has(ctx context.Context, key string) bool {
	// Validate key
	if key == "" {
		return false
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return false
	}

	// Check if key exists in Redis
	formattedKey := formatKey(key)
	exists, err := c.client.Exists(ctx, formattedKey).Result()
	if err != nil {
		return false
	}

	return exists > 0
}

// GetKeys returns all keys in the cache
func (c *redisCache) GetKeys(ctx context.Context) []string {
	// Check for context cancellation
	if ctx.Err() != nil {
		return []string{}
	}

	// Use scan to find all keys matching our prefix
	var cursor uint64
	var err error
	var keys []string

	for {
		var scanKeys []string
		scanKeys, cursor, err = c.client.Scan(ctx, cursor, keyPrefix+"*", 100).Result()
		if err != nil {
			return []string{}
		}

		// Remove prefix from keys
		for i, fullKey := range scanKeys {
			scanKeys[i] = strings.TrimPrefix(fullKey, keyPrefix)
		}

		keys = append(keys, scanKeys...)

		if cursor == 0 {
			break
		}
	}

	return keys
}

// Close closes the Redis client
func (c *redisCache) Close() error {
	return c.client.Close()
}

// GetMany retrieves multiple values from the cache
func (c *redisCache) GetMany(ctx context.Context, keys []string) (map[string]any, error) {
	// Validate keys
	if len(keys) == 0 {
		return map[string]any{}, nil
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, cache.ErrContextCanceled
	}

	// Format keys
	formattedKeys := make([]string, len(keys))
	for i, key := range keys {
		formattedKeys[i] = formatKey(key)
	}

	// Get values using MGET
	values, err := c.client.MGet(ctx, formattedKeys...).Result()
	if err != nil {
		return nil, err
	}

	// Process results
	result := make(map[string]any)
	for i, val := range values {
		if val != nil {
			// Deserialize value
			var value any
			if data, ok := val.(string); ok {
				if err := c.serializer.Deserialize([]byte(data), &value); err == nil {
					result[keys[i]] = value
					// Update metadata in background
					go c.updateAccessMetadata(context.Background(), keys[i])
					c.recordHit()
				} else {
					// Deserialization error, just log
					if c.options.Logger != nil {
						c.options.Logger.Error("Failed to deserialize value",
							logger.Field{Key: "key", Value: keys[i]},
							logger.Field{Key: "error", Value: err.Error()})
					}
					c.recordMiss()
				}
			}
		} else {
			c.recordMiss()
		}
	}

	return result, nil
}

// SetMany stores multiple values in the cache
func (c *redisCache) SetMany(ctx context.Context, items map[string]any, ttl time.Duration) error {
	// Validate items
	if len(items) == 0 {
		return nil
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return cache.ErrContextCanceled
	}

	// Use default TTL if not specified
	if ttl == 0 {
		ttl = c.options.TTL
	}
	if ttl <= 0 {
		ttl = defaultKeyTTL
	}

	// Use pipeline for better performance
	pipe := c.client.Pipeline()

	// Add each item to the pipeline
	for key, value := range items {
		// Serialize value
		data, err := c.serializer.Serialize(value)
		if err != nil {
			return cache.ErrSerialization
		}

		// Add to pipeline
		formattedKey := formatKey(key)
		pipe.Set(ctx, formattedKey, data, ttl)

		// Store metadata async to not block the main operation
		go c.storeMetadata(context.Background(), key, len(data), ttl)
	}

	// Execute the pipeline
	_, err := pipe.Exec(ctx)
	return err
}

// DeleteMany removes multiple values from the cache
func (c *redisCache) DeleteMany(ctx context.Context, keys []string) error {
	// Validate keys
	if len(keys) == 0 {
		return nil
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return cache.ErrContextCanceled
	}

	// Format keys
	formattedKeys := make([]string, len(keys))
	metaKeys := make([]string, len(keys))
	for i, key := range keys {
		formattedKeys[i] = formatKey(key)
		metaKeys[i] = formatMetadataKey(key)
	}

	// Use pipeline for better performance
	pipe := c.client.Pipeline()

	// Delete values
	pipe.Del(ctx, formattedKeys...)

	// Delete metadata
	pipe.Del(ctx, metaKeys...)

	// Execute the pipeline
	_, err := pipe.Exec(ctx)
	return err
}

// GetMetadata retrieves metadata for a cache entry
func (c *redisCache) GetMetadata(ctx context.Context, key string) (*cache.CacheEntryMetadata, error) {
	// Validate key
	if key == "" {
		return nil, cache.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, cache.ErrContextCanceled
	}

	// First check if the key exists
	if !c.Has(ctx, key) {
		return nil, cache.ErrKeyNotFound
	}

	// Get metadata from Redis
	metaKey := formatMetadataKey(key)
	data, err := c.client.Get(ctx, metaKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			// Key exists but metadata doesn't, return minimal metadata
			return &cache.CacheEntryMetadata{
				Key:       key,
				CreatedAt: time.Now(),
			}, nil
		}
		return nil, err
	}

	// Deserialize metadata
	var meta metadataEntry
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, cache.ErrDeserialization
	}

	// Get TTL from Redis
	formattedKey := formatKey(key)
	ttl, err := c.client.TTL(ctx, formattedKey).Result()
	if err != nil {
		ttl = meta.TTL // Fallback to stored TTL
	}

	// Create cache entry metadata
	return &cache.CacheEntryMetadata{
		Key:          key,
		CreatedAt:    meta.CreatedAt,
		LastAccessed: meta.LastAccessed,
		AccessCount:  meta.AccessCount,
		TTL:          ttl,
		Size:         meta.Size,
		Tags:         meta.Tags,
	}, nil
}

// GetManyMetadata retrieves metadata for multiple cache entries
func (c *redisCache) GetManyMetadata(ctx context.Context, keys []string) (map[string]*cache.CacheEntryMetadata, error) {
	// Validate keys
	if len(keys) == 0 {
		return map[string]*cache.CacheEntryMetadata{}, nil
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, cache.ErrContextCanceled
	}

	results := make(map[string]*cache.CacheEntryMetadata)

	// Format metadata keys
	metaKeys := make([]string, 0, len(keys))
	keyMap := make(map[string]string) // Map formatted keys back to original keys

	// First check which keys exist
	existingKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		if c.Has(ctx, key) {
			existingKeys = append(existingKeys, key)
			metaKey := formatMetadataKey(key)
			metaKeys = append(metaKeys, metaKey)
			keyMap[metaKey] = key
		}
	}

	// Get TTLs for all existing keys
	ttls := make(map[string]time.Duration)
	if len(existingKeys) > 0 {
		// Use pipeline for better performance
		pipe := c.client.Pipeline()

		for _, key := range existingKeys {
			formattedKey := formatKey(key)
			pipe.TTL(ctx, formattedKey)
		}

		cmds, err := pipe.Exec(ctx)
		if err != nil {
			return nil, err
		}

		for i, cmd := range cmds {
			if ttlCmd, ok := cmd.(*redis.DurationCmd); ok {
				ttls[existingKeys[i]] = ttlCmd.Val()
			}
		}
	}

	// Get metadata for all existing keys
	if len(metaKeys) > 0 {
		// Use pipeline for better performance
		pipe := c.client.Pipeline()

		for _, metaKey := range metaKeys {
			pipe.Get(ctx, metaKey)
		}

		cmds, err := pipe.Exec(ctx)
		if err != nil {
			return nil, err
		}

		for i, cmd := range cmds {
			if getCmd, ok := cmd.(*redis.StringCmd); ok {
				data, err := getCmd.Bytes()
				originalKey := keyMap[metaKeys[i]]

				switch err {
				case nil:
					// Deserialize metadata
					var meta metadataEntry
					if err := json.Unmarshal(data, &meta); err == nil {
						results[originalKey] = &cache.CacheEntryMetadata{
							Key:          originalKey,
							CreatedAt:    meta.CreatedAt,
							LastAccessed: meta.LastAccessed,
							AccessCount:  meta.AccessCount,
							TTL:          ttls[originalKey],
							Size:         meta.Size,
							Tags:         meta.Tags,
						}
					} else {
						// Return minimal metadata on deserialization error
						results[originalKey] = &cache.CacheEntryMetadata{
							Key:       originalKey,
							CreatedAt: time.Now(),
							TTL:       ttls[originalKey],
						}
					}
				case redis.Nil:
					// Key exists but metadata doesn't, return minimal metadata
					results[originalKey] = &cache.CacheEntryMetadata{
						Key:       originalKey,
						CreatedAt: time.Now(),
						TTL:       ttls[originalKey],
					}
				}
			}
		}
	}

	return results, nil
}


// Helper function to parse Redis INFO command output
func parseRedisInfo(info string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(info, "\r\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}

	return result
}

// storeMetadata stores metadata for a cache entry
func (c *redisCache) storeMetadata(ctx context.Context, key string, size int, ttl time.Duration) error {
	// Create metadata
	meta := metadataEntry{
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		AccessCount:  0,
		TTL:          ttl,
		Size:         int64(size),
		Tags:         []string{},
	}

	// Serialize metadata
	data, err := json.Marshal(meta)
	if err != nil {
		return cache.ErrSerialization
	}

	// Store metadata in Redis with the same TTL as the main key
	metaKey := formatMetadataKey(key)
	return c.client.Set(ctx, metaKey, data, ttl).Err()
}

// updateAccessMetadata updates access metadata for a cache entry
func (c *redisCache) updateAccessMetadata(ctx context.Context, key string) {
	// Get existing metadata
	metaKey := formatMetadataKey(key)
	data, err := c.client.Get(ctx, metaKey).Bytes()
	if err != nil {
		// Metadata doesn't exist, nothing to update
		return
	}

	// Deserialize metadata
	var meta metadataEntry
	if err := json.Unmarshal(data, &meta); err != nil {
		// Corrupted metadata, nothing to update
		return
	}

	// Update access information
	meta.LastAccessed = time.Now()
	meta.AccessCount++

	// Serialize updated metadata
	updatedData, err := json.Marshal(meta)
	if err != nil {
		return
	}

	// Get remaining TTL of the value
	formattedKey := formatKey(key)
	ttl, err := c.client.TTL(ctx, formattedKey).Result()
	if err != nil || ttl < 0 {
		// Use default TTL if we can't get the TTL
		ttl = defaultKeyTTL
	}

	// Store updated metadata
	c.client.Set(ctx, metaKey, updatedData, ttl)
}

// Metrics recording functions
// New metrics recording functions that use both legacy and enhanced metrics
func (c *redisCache) recordHit() {
	// Legacy metrics
	c.legacyMetrics.mu.Lock()
	c.legacyMetrics.hits++
	c.legacyMetrics.mu.Unlock()
	
	// Enhanced metrics
	tags := c.getBaseTags()
	c.enhancedMetrics.RecordHit(c.providerName, tags)
}

func (c *redisCache) recordMiss() {
	// Legacy metrics
	c.legacyMetrics.mu.Lock()
	c.legacyMetrics.misses++
	c.legacyMetrics.mu.Unlock()
	
	// Enhanced metrics
	tags := c.getBaseTags()
	c.enhancedMetrics.RecordMiss(c.providerName, tags)
}

func (c *redisCache) recordGetLatency(duration time.Duration) {
	// Legacy metrics
	c.legacyMetrics.mu.Lock()
	c.legacyMetrics.getLatency = duration
	c.legacyMetrics.mu.Unlock()
	
	// Enhanced metrics
	tags := c.getBaseTags()
	c.enhancedMetrics.RecordOperation(c.providerName, "get", "completed", duration, tags)
}

func (c *redisCache) recordSetLatency(duration time.Duration) {
	// Legacy metrics
	c.legacyMetrics.mu.Lock()
	c.legacyMetrics.setLatency = duration
	c.legacyMetrics.mu.Unlock()
	
	// Enhanced metrics
	tags := c.getBaseTags()
	c.enhancedMetrics.RecordOperation(c.providerName, "set", "completed", duration, tags)
}

func (c *redisCache) recordDeleteLatency(duration time.Duration) {
	// Legacy metrics
	c.legacyMetrics.mu.Lock()
	c.legacyMetrics.deleteLatency = duration
	c.legacyMetrics.mu.Unlock()
	
	// Enhanced metrics
	tags := c.getBaseTags()
	c.enhancedMetrics.RecordOperation(c.providerName, "delete", "completed", duration, tags)
}

// Helper function to get base tags for metrics
func (c *redisCache) getBaseTags() gometrics.Tags {
	tags := make(gometrics.Tags)
	if c.options.GlobalMetricsTags != nil {
		for k, v := range c.options.GlobalMetricsTags {
			tags[k] = v
		}
	}
	return tags
}

// recordRedisCommand records metrics for a Redis command
func (c *redisCache) recordRedisCommand(command string, duration time.Duration, err error) {
	tags := c.getBaseTags()
	tags["command"] = command
	
	status := "success"
	if err != nil {
		status = "error"
		// Record Redis-specific errors
		errorType := "redis_error"
		errorCategory := "unknown"
		
		// Categorize common Redis errors
		if err == redis.Nil {
			errorType = "not_found"
			errorCategory = "key_not_found"
		} else if strings.Contains(err.Error(), "connection refused") {
			errorType = "connection_error"
			errorCategory = "connection_refused"
		} else if strings.Contains(err.Error(), "timeout") {
			errorType = "connection_error"
			errorCategory = "timeout"
		}
		
		c.enhancedMetrics.RecordError(c.providerName, command, errorType, errorCategory, tags)
	}
	
	tags["status"] = status
	c.enhancedMetrics.RecordOperation(c.providerName, "redis_"+command, status, duration, tags)
}

// recordConnectionPoolStats records Redis connection pool metrics
func (c *redisCache) recordConnectionPoolStats() {
	stats := c.client.PoolStats()
	tags := c.getBaseTags()
	
	// Record various pool statistics
	c.enhancedMetrics.RecordProviderSpecific(c.providerName, "connection_pool_hits", float64(stats.Hits), tags)
	c.enhancedMetrics.RecordProviderSpecific(c.providerName, "connection_pool_misses", float64(stats.Misses), tags)
	c.enhancedMetrics.RecordProviderSpecific(c.providerName, "connection_pool_timeouts", float64(stats.Timeouts), tags)
	c.enhancedMetrics.RecordProviderSpecific(c.providerName, "connection_pool_total_conns", float64(stats.TotalConns), tags)
	c.enhancedMetrics.RecordProviderSpecific(c.providerName, "connection_pool_idle_conns", float64(stats.IdleConns), tags)
	c.enhancedMetrics.RecordProviderSpecific(c.providerName, "connection_pool_stale_conns", float64(stats.StaleConns), tags)
}

// Increment atomically increments a numeric value in Redis
func (c *redisCache) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	// Validate key
	if key == "" {
		return 0, cache.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return 0, cache.ErrContextCanceled
	}

	// Use default TTL if not specified
	if ttl == 0 && c.options != nil {
		ttl = c.options.TTL
	}
	if ttl <= 0 {
		ttl = defaultKeyTTL
	}

	formattedKey := formatKey(key)

	// Use Redis atomic INCRBY operation
	result, err := c.client.IncrBy(ctx, formattedKey, delta).Result()
	if err != nil {
		return 0, fmt.Errorf("redis increment failed: %w", err)
	}

	// Set TTL on the key if it's newly created or doesn't have one
	if result == delta {
		// This is likely a new key (first increment resulted in delta value)
		// Set TTL on the key
		c.client.Expire(ctx, formattedKey, ttl)

		// Store metadata for the new key
		go c.storeMetadata(ctx, key, 8, ttl) // int64 is 8 bytes
	}

	return result, nil
}

// Decrement atomically decrements a numeric value in Redis
func (c *redisCache) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return c.Increment(ctx, key, -delta, ttl)
}

// SetIfNotExists sets a value only if the key doesn't exist (Redis SETNX)
func (c *redisCache) SetIfNotExists(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	// Validate key
	if key == "" {
		return false, cache.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return false, cache.ErrContextCanceled
	}

	// Use default TTL if not specified
	if ttl == 0 && c.options != nil {
		ttl = c.options.TTL
	}
	if ttl <= 0 {
		ttl = defaultKeyTTL
	}

	// Serialize value
	data, err := c.serializer.Serialize(value)
	if err != nil {
		return false, cache.ErrSerialization
	}

	formattedKey := formatKey(key)

	// Use Redis SET NX command (set if not exists)
	success, err := c.client.SetNX(ctx, formattedKey, data, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("redis setnx failed: %w", err)
	}

	// If successful, store metadata
	if success {
		go c.storeMetadata(ctx, key, len(data), ttl)
	}

	return success, nil
}

// SetIfExists sets a value only if the key already exists (Redis SET XX)
func (c *redisCache) SetIfExists(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	// Validate key
	if key == "" {
		return false, cache.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return false, cache.ErrContextCanceled
	}

	// Use default TTL if not specified
	if ttl == 0 && c.options != nil {
		ttl = c.options.TTL
	}
	if ttl <= 0 {
		ttl = defaultKeyTTL
	}

	// Serialize value
	data, err := c.serializer.Serialize(value)
	if err != nil {
		return false, cache.ErrSerialization
	}

	formattedKey := formatKey(key)

	// Use Redis SET XX command with a Lua script for atomic operation
	// This is more reliable than EXISTS + SET because it's atomic
	luaScript := `
		if redis.call("EXISTS", KEYS[1]) == 1 then
			redis.call("SET", KEYS[1], ARGV[1])
			if ARGV[2] ~= "0" then
				redis.call("EXPIRE", KEYS[1], ARGV[2])
			end
			return 1
		else
			return 0
		end
	`

	ttlSeconds := int64(ttl.Seconds())
	result, err := c.client.Eval(ctx, luaScript, []string{formattedKey}, data, ttlSeconds).Result()
	if err != nil {
		return false, fmt.Errorf("redis set if exists failed: %w", err)
	}

	success := result.(int64) == 1

	// If successful, update metadata
	if success {
		go c.storeMetadata(ctx, key, len(data), ttl)
	}

	return success, nil
}

// AddIndex adds a secondary index for the given key pattern
func (c *redisCache) AddIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error {
	// Validate parameters
	if indexName == "" || keyPattern == "" || indexKey == "" {
		return cache.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return cache.ErrContextCanceled
	}

	// Get all existing cache keys and filter by pattern
	allKeys := c.GetKeys(ctx)
	var matchingKeys []string
	
	// Use filepath.Match to match keys against pattern (same as memory provider)
	for _, key := range allKeys {
		matched, err := filepath.Match(keyPattern, key)
		if err != nil {
			// If filepath.Match fails, fall back to simple string matching
			if strings.Contains(key, strings.ReplaceAll(keyPattern, "*", "")) {
				matchingKeys = append(matchingKeys, key)
			}
		} else if matched {
			matchingKeys = append(matchingKeys, key)
		}
	}

	// Create index set key
	indexSetKey := fmt.Sprintf("cache:index:%s:%s", indexName, indexKey)

	// Add all matching keys to the Redis set for this index
	if len(matchingKeys) > 0 {
		// Use pipeline for better performance
		pipe := c.client.Pipeline()

		for _, key := range matchingKeys {
			// Add the key to the index set
			pipe.SAdd(ctx, indexSetKey, key)
		}

		// Set a reasonable TTL on the index set (default to cache TTL)
		ttl := c.options.TTL
		if ttl <= 0 {
			ttl = defaultKeyTTL
		}
		pipe.Expire(ctx, indexSetKey, ttl)

		_, err := pipe.Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// RemoveIndex removes a secondary index entry - STUB IMPLEMENTATION
func (c *redisCache) RemoveIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error {
	// TODO: Implement Redis-based secondary index removal
	return fmt.Errorf("RemoveIndex not yet implemented for Redis provider")
}

// GetByIndex retrieves all keys associated with an index key
func (c *redisCache) GetByIndex(ctx context.Context, indexName string, indexKey string) ([]string, error) {
	// Validate parameters
	if indexName == "" || indexKey == "" {
		return []string{}, cache.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return []string{}, cache.ErrContextCanceled
	}

	// Create index set key
	indexSetKey := fmt.Sprintf("cache:index:%s:%s", indexName, indexKey)

	// Get all members from the Redis set
	keys, err := c.client.SMembers(ctx, indexSetKey).Result()
	if err != nil {
		if err == redis.Nil {
			// Index doesn't exist, return empty slice
			return []string{}, nil
		}
		return []string{}, fmt.Errorf("failed to get index members: %w", err)
	}

	// Filter out keys that no longer exist in the cache
	existingKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		if c.Has(ctx, key) {
			existingKeys = append(existingKeys, key)
		} else {
			// Key no longer exists, remove it from the index in background
			go c.client.SRem(context.Background(), indexSetKey, key)
		}
	}

	return existingKeys, nil
}

// DeleteByIndex deletes all keys associated with an index key - STUB IMPLEMENTATION
func (c *redisCache) DeleteByIndex(ctx context.Context, indexName string, indexKey string) error {
	// TODO: Implement Redis-based bulk deletion by index
	return fmt.Errorf("DeleteByIndex not yet implemented for Redis provider")
}

// GetKeysByPattern returns all keys matching the given pattern
func (c *redisCache) GetKeysByPattern(ctx context.Context, pattern string) ([]string, error) {
	// Validate pattern
	if pattern == "" {
		return []string{}, cache.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return []string{}, cache.ErrContextCanceled
	}

	// Add cache prefix to pattern for Redis scan
	searchPattern := keyPrefix + pattern

	// Use SCAN to find all keys matching the pattern
	var cursor uint64
	var err error
	var keys []string

	for {
		var scanKeys []string
		scanKeys, cursor, err = c.client.Scan(ctx, cursor, searchPattern, 100).Result()
		if err != nil {
			return []string{}, fmt.Errorf("redis scan failed: %w", err)
		}

		// Remove prefix from keys
		for _, fullKey := range scanKeys {
			key := strings.TrimPrefix(fullKey, keyPrefix)
			keys = append(keys, key)
		}

		if cursor == 0 {
			break
		}
	}

	return keys, nil
}

// DeleteByPattern deletes all keys matching the given pattern
func (c *redisCache) DeleteByPattern(ctx context.Context, pattern string) (int, error) {
	// Validate pattern
	if pattern == "" {
		return 0, cache.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return 0, cache.ErrContextCanceled
	}

	// Get all keys matching the pattern
	keys, err := c.GetKeysByPattern(ctx, pattern)
	if err != nil {
		return 0, fmt.Errorf("failed to get keys by pattern: %w", err)
	}

	if len(keys) == 0 {
		return 0, nil
	}

	// Delete all matching keys
	err = c.DeleteMany(ctx, keys)
	if err != nil {
		return 0, fmt.Errorf("failed to delete keys: %w", err)
	}

	return len(keys), nil
}

// UpdateMetadata updates metadata for a cache entry - STUB IMPLEMENTATION
func (c *redisCache) UpdateMetadata(ctx context.Context, key string, updater cache.MetadataUpdater) error {
	// TODO: Implement Redis-based metadata updates
	return fmt.Errorf("UpdateMetadata not yet implemented for Redis provider")
}

// GetAndUpdate atomically gets and updates a cache entry - STUB IMPLEMENTATION
func (c *redisCache) GetAndUpdate(ctx context.Context, key string, updater cache.ValueUpdater, ttl time.Duration) (any, error) {
	// TODO: Implement Redis-based atomic get-and-update using Lua scripts
	return nil, fmt.Errorf("GetAndUpdate not yet implemented for Redis provider")
}

package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MichaelAJay/go-cache"
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
	metrics    *cacheMetrics
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

	// Create Redis cache
	c := &redisCache{
		client:     client,
		options:    options,
		serializer: s,
		metrics:    &cacheMetrics{},
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
		c.metrics.recordGetLatency(time.Since(start))
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
			c.metrics.recordMiss()
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

	c.metrics.recordHit()
	return value, true, nil
}

// Set stores a value in the cache
func (c *redisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		c.metrics.recordSetLatency(time.Since(start))
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
		c.metrics.recordDeleteLatency(time.Since(start))
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
					c.metrics.recordHit()
				} else {
					// Deserialization error, just log
					if c.options.Logger != nil {
						c.options.Logger.Error("Failed to deserialize value",
							logger.Field{Key: "key", Value: keys[i]},
							logger.Field{Key: "error", Value: err.Error()})
					}
					c.metrics.recordMiss()
				}
			}
		} else {
			c.metrics.recordMiss()
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

				if err == nil {
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
				} else if err == redis.Nil {
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

// GetMetrics returns a snapshot of the metrics
func (c *redisCache) GetMetrics() *cache.CacheMetricsSnapshot {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()

	hitRate := float64(0)
	totalOps := c.metrics.hits + c.metrics.misses
	if totalOps > 0 {
		hitRate = float64(c.metrics.hits) / float64(totalOps)
	}

	// Get stats from Redis
	ctx := context.Background()
	var dbSize int64
	var info map[string]string

	// Get DB size (approximate)
	dbSizeCmd := c.client.DBSize(ctx)
	if dbSizeCmd.Err() == nil {
		dbSize = dbSizeCmd.Val()
	}

	// Get Redis info
	infoCmd := c.client.Info(ctx)
	if infoCmd.Err() == nil {
		info = parseRedisInfo(infoCmd.Val())
	}

	// Calculate memory usage for our cache (approximate)
	usedMemory := int64(0)
	if memoryStr, ok := info["used_memory"]; ok {
		if parsedMem, err := strconv.ParseInt(memoryStr, 10, 64); err == nil {
			usedMemory = parsedMem
		}
	}

	// If we have keys with our prefix, adjust memory usage proportionally
	if dbSize > 0 {
		keysWithPrefix := int64(len(c.GetKeys(ctx)))
		if keysWithPrefix > 0 && keysWithPrefix < dbSize {
			usedMemory = int64(float64(usedMemory) * (float64(keysWithPrefix) / float64(dbSize)))
		}
	}

	return &cache.CacheMetricsSnapshot{
		Hits:          c.metrics.hits,
		Misses:        c.metrics.misses,
		HitRatio:      hitRate,
		GetLatency:    c.metrics.getLatency,
		SetLatency:    c.metrics.setLatency,
		DeleteLatency: c.metrics.deleteLatency,
		CacheSize:     usedMemory,
		EntryCount:    int64(len(c.GetKeys(ctx))), // Only count keys with our prefix
	}
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
func (m *cacheMetrics) recordHit() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hits++
}

func (m *cacheMetrics) recordMiss() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.misses++
}

func (m *cacheMetrics) recordGetLatency(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getLatency = duration
}

func (m *cacheMetrics) recordSetLatency(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setLatency = duration
}

func (m *cacheMetrics) recordDeleteLatency(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteLatency = duration
}

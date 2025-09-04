package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"maps"
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

// RedisOptions configures Redis-specific cache behavior
type RedisOptions struct {
	DataPrefix  string // Prefix for data keys (e.g., "sessions:data:")
	IndexPrefix string // Prefix for index keys (e.g., "sessions:index:")
	MetaPrefix  string // Prefix for metadata keys (e.g., "sessions:meta:")
	LockPrefix  string // Prefix for lock keys (e.g., "sessions:lock:")
}

// IndexExtractor defines how to extract keys from values for indexing
// GetEntryKey extracts the primary cache entry key (e.g., SessionID from Session)
// GetOwnerKey extracts the owner/grouping key (e.g., UserID from Session)
type IndexExtractor[T any] struct {
	GetEntryKey func(T) string // Required: extracts primary cache entry key
	GetOwnerKey func(T) string // Required: extracts primary index key for cache entry
}

const (
	// Lock configuration
	defaultLockTimeout = 30 * time.Second
	lockRetryDelay     = 10 * time.Millisecond
	lockMaxRetries     = 100

	// Circuit breaker thresholds
	circuitBreakerThreshold = 10
	circuitBreakerTimeout   = 60 * time.Second
)

// RedisCache implements the Cache[T] interface with distributed Redis backend
type RedisCache[T any] struct {
	client     redis.Cmdable
	serializer serializer.Serializer
	options    *config.CacheOptions
	metrics    metrics.EnhancedCacheMetrics

	extractor    *IndexExtractor[T] // nil = no indexing
	indexingMode bool               // derived from extractor != nil

	// Redis-specific options
	redisOptions *RedisOptions

	// Circuit breaker state
	mu                 sync.RWMutex
	circuitBreakerOpen bool
	lastFailureTime    time.Time
	failureCount       int

	// Instance identifier for distributed coordination
	instanceID string

	// Lua scripts for atomic operations
	getScript            *redis.Script
	setScript            *redis.Script
	getOrSetScript       *redis.Script
	updateScript         *redis.Script
	deleteByIndexScript  *redis.Script
	deleteByEntryScript  *redis.Script
	getByOwnerScript     *redis.Script
	deleteByOwnerScript  *redis.Script
	setIfExistsScript    *redis.Script
	setIfNotExistsScript *redis.Script
}

// Option defines a functional option for configuring cache behavior
type Option[T any] func(*RedisCache[T])

// WithRedisOptions sets Redis-specific configuration
func WithRedisOptions[T any](redisOpts *RedisOptions) Option[T] {
	return func(cache *RedisCache[T]) {
		cache.redisOptions = redisOpts
	}
}

// WithTTL sets the default TTL for cache entries
func WithTTL[T any](ttl time.Duration) Option[T] {
	return func(cache *RedisCache[T]) {
		cache.options.DefaultTTL = ttl
	}
}

// WithMetrics sets custom metrics implementation
func WithMetrics[T any](metrics metrics.EnhancedCacheMetrics) Option[T] {
	return func(cache *RedisCache[T]) {
		cache.options.EnhancedMetrics = metrics
		cache.metrics = metrics
	}
}

// WithHooks sets lifecycle hooks
func WithHooks[T any](hooks *config.CacheHooks) Option[T] {
	return func(cache *RedisCache[T]) {
		cache.options.Hooks = hooks
	}
}

// WithSerializer sets serialization format
func WithSerializer[T any](format string) Option[T] {
	return func(cache *RedisCache[T]) {
		cache.options.SerializerFormat = format
	}
}

// WithMaxEntries sets the maximum number of cache entries
func WithMaxEntries[T any](max int) Option[T] {
	return func(cache *RedisCache[T]) {
		cache.options.MaxEntries = max
	}
}

// WithCleanupInterval sets how often expired entries are cleaned
func WithCleanupInterval[T any](interval time.Duration) Option[T] {
	return func(cache *RedisCache[T]) {
		cache.options.CleanupInterval = interval
	}
}

// WithGoMetrics sets go-metrics registry for built-in metrics
func WithGoMetrics[T any](registry metric.Registry, tags metric.Tags) Option[T] {
	return func(cache *RedisCache[T]) {
		cache.options.GoMetricsRegistry = registry
		cache.options.GlobalMetricsTags = tags
	}
}

// NewCache creates a new Redis cache instance with clean abstraction
// indexingMode explicitly controls whether indexing features are enabled
// extractor provides key extraction functions - GetEntryKey always required, GetOwnerKey only when indexing enabled
// Valid combinations:
//   - indexingMode=false, extractor with GetEntryKey (basic caching)
//   - indexingMode=true, extractor with GetEntryKey & GetOwnerKey (indexed caching)
func NewCache[T any](ctx context.Context, client redis.Cmdable, indexingMode bool, extractor *IndexExtractor[T], opts ...Option[T]) (interfaces.Cache[T], error) {
	if client == nil {
		return nil, fmt.Errorf("redis client cannot be nil")
	}

	// Create cache instance with defaults
	cache := &RedisCache[T]{
		client:       client,
		extractor:    extractor,
		indexingMode: indexingMode,
		options:      config.DefaultOptions(),
		instanceID:   generateInstanceID(),
		redisOptions: nil,
	}

	// Apply generic options
	for _, opt := range opts {
		opt(cache)
	}

	// CRITICAL: Validate indexing configuration at initialization
	if err := cache.validateIndexingConfig(); err != nil {
		return nil, fmt.Errorf("indexing configuration error: %w", err)
	}

	// Initialize the cache (serializer, metrics, scripts, etc.)
	if err := cache.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize cache: %w", err)
	}

	return cache, nil
}

// validateIndexingConfig validates indexing configuration at initialization time
func (c *RedisCache[T]) validateIndexingConfig() error {
	// GetEntryKey is ALWAYS required to determine storage key from value
	if c.extractor == nil {
		return fmt.Errorf("extractor is required - provide valid IndexExtractor with GetEntryKey")
	}
	if c.extractor.GetEntryKey == nil {
		return fmt.Errorf("IndexExtractor.GetEntryKey is required to determine storage key from value")
	}

	// GetOwnerKey is only required when indexing is enabled
	if c.indexingMode && c.extractor.GetOwnerKey == nil {
		return fmt.Errorf("indexingMode=true but IndexExtractor.GetOwnerKey is nil - required for owner-to-entries indexing")
	}

	return nil
}

// initialize sets up the cache instance with serializer, metrics, and Lua scripts
func (c *RedisCache[T]) initialize() error {
	// Initialize serializer
	var ser serializer.Serializer
	if c.options.SerializerFormat != "" {
		switch c.options.SerializerFormat {
		case "json":
			ser = serializer.NewJSONSerializer()
		case "gob", "binary":
			ser = serializer.NewGobSerializer()
		case "msgpack":
			ser = serializer.NewMsgpackSerializer()
		default:
			return fmt.Errorf("unsupported serializer format: %s", c.options.SerializerFormat)
		}
	} else {
		ser = serializer.NewMsgpackSerializer()
	}
	c.serializer = ser

	// Initialize metrics
	if c.metrics == nil {
		if c.options.EnhancedMetrics != nil {
			c.metrics = c.options.EnhancedMetrics
		} else if c.options.GoMetricsRegistry != nil {
			c.metrics = metrics.NewEnhancedCacheMetrics(c.options.GoMetricsRegistry, c.options.GlobalMetricsTags)
		} else {
			c.metrics = metrics.NewNoopEnhancedCacheMetrics()
		}
	}

	// Initialize Lua scripts for atomic operations
	c.initLuaScripts()

	return nil
}

// generateInstanceID creates a unique identifier for this cache instance
func generateInstanceID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// ttlToMilliseconds converts a duration to milliseconds for Redis PEXPIRE commands
func ttlToMilliseconds(ttl time.Duration) int64 {
	if ttl <= 0 {
		return 0
	}
	return int64(ttl.Milliseconds())
}

// Basic Operations

// Get retrieves a value by key
func (c *RedisCache[T]) Get(ctx context.Context, key string) (T, bool, error) {
	start := time.Now()
	var zero T

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "get", "circuit_breaker", "availability", c.getMetricTags())
		return zero, false, cacheErrors.ErrCircuitBreakerOpen
	}

	dataKey := c.buildDataKey(key)
	metaKey := c.buildMetaKey(key)

	// Use Lua script for atomic get and metadata update
	result, err := c.getScript.Run(ctx, c.client, []string{dataKey, metaKey}).Result()
	if err != nil {
		c.handleError("get", err)
		c.metrics.RecordError("redis", "get", "redis_error", "infrastructure", c.getMetricTags())
		return zero, false, fmt.Errorf("redis get error: %w", err)
	}

	resultSlice, ok := result.([]any)
	if !ok {
		return zero, false, fmt.Errorf("unexpected script result type: %T", result)
	}

	// Handle empty result (cache miss)
	if len(resultSlice) == 0 {
		c.metrics.RecordMiss("redis", c.getMetricTags())
		c.metrics.RecordOperation("redis", "get", "miss", time.Since(start), c.getMetricTags())
		return zero, false, nil
	}

	if len(resultSlice) < 2 {
		return zero, false, fmt.Errorf("unexpected script result length: %d", len(resultSlice))
	}

	serializedValue := resultSlice[0]
	found := resultSlice[1].(string) == "1"

	if !found {
		c.metrics.RecordMiss("redis", c.getMetricTags())
		c.metrics.RecordOperation("redis", "get", "miss", time.Since(start), c.getMetricTags())
		return zero, false, nil
	}

	// Deserialize value
	var value T
	if err := c.serializer.Deserialize([]byte(serializedValue.(string)), &value); err != nil {
		c.metrics.RecordError("redis", "get", "serialization_error", "data", c.getMetricTags())
		return zero, false, fmt.Errorf("deserialization error for key %s: %w", key, err)
	}

	c.metrics.RecordHit("redis", c.getMetricTags())
	c.metrics.RecordOperation("redis", "get", "success", time.Since(start), c.getMetricTags())
	return value, true, nil
}

// Set stores a value with TTL, using configured extractors to determine storage key
func (c *RedisCache[T]) Set(ctx context.Context, value T, ttl time.Duration) error {
	// Extract key from value using configured extractor
	if c.extractor == nil || c.extractor.GetEntryKey == nil {
		return fmt.Errorf("IndexExtractor.GetEntryKey is required for Set operation")
	}
	key := c.extractor.GetEntryKey(value)
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

	ttlInMilliseconds := ttlToMilliseconds(ttl)

	// Always use unified SET script with conditional indexing
	var ownerKey string
	var indexKey string
	var reverseKey string
	
	if c.indexingMode {
		// Indexing enabled - validate and extract owner key
		if c.extractor.GetOwnerKey == nil {
			return fmt.Errorf("IndexExtractor.GetOwnerKey is required when indexing is enabled")
		}
		ownerKey = c.extractor.GetOwnerKey(value)
		indexKey = c.buildIndexKey("owner", ownerKey)
		reverseKey = c.buildReverseIndexKey(key)
	} else {
		// Provide dummy values for non-indexing mode (will be ignored by script)
		ownerKey = ""
		indexKey = ""
		reverseKey = ""
	}

	result, err := c.setScript.Run(ctx, c.client,
		[]string{dataKey, metaKey, indexKey, reverseKey},
		string(serializedValue), ttlInMilliseconds, key, ownerKey, fmt.Sprintf("%t", c.indexingMode)).Result()

	if err != nil {
		c.handleError("set", err)
		c.metrics.RecordError("redis", "set", "redis_error", "infrastructure", c.getMetricTags())
		return fmt.Errorf("redis set error: %w", err)
	}

	// Validate script result
	if result != "OK" {
		return fmt.Errorf("unexpected set script result: %v", result)
	}

	c.metrics.RecordOperation("redis", "set", "success", time.Since(start), c.getMetricTags())
	return nil
}

// Delete removes a key
func (c *RedisCache[T]) Delete(ctx context.Context, key string) error {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "delete", "circuit_breaker", "availability", c.getMetricTags())
		return cacheErrors.ErrCircuitBreakerOpen
	}

	dataKey := c.buildDataKey(key)
	metaKey := c.buildMetaKey(key)

	var deleted int64
	var err error

	if c.indexingMode {
		// Use atomic script to clean up indexes
		reverseKey := c.buildReverseIndexKey(key)
		indexPrefix := "cache:index:"
		if c.redisOptions != nil && c.redisOptions.IndexPrefix != "" {
			indexPrefix = c.redisOptions.IndexPrefix
		}

		result, scriptErr := c.deleteByEntryScript.Run(ctx, c.client,
			[]string{dataKey, metaKey, reverseKey},
			key, indexPrefix).Result()

		if scriptErr != nil {
			c.handleError("delete", scriptErr)
			c.metrics.RecordError("redis", "delete", "redis_error", "infrastructure", c.getMetricTags())
			return fmt.Errorf("redis delete error: %w", scriptErr)
		}
		deleted = result.(int64)
	} else {
		// Simple delete without index cleanup
		deleted, err = c.client.Del(ctx, dataKey, metaKey).Result()
		if err != nil {
			c.handleError("delete", err)
			c.metrics.RecordError("redis", "delete", "redis_error", "infrastructure", c.getMetricTags())
			return fmt.Errorf("redis delete error: %w", err)
		}
	}

	c.metrics.RecordOperation("redis", "delete", "success", time.Since(start), c.getMetricTags())

	// Apply hooks if configured
	if c.options.Hooks != nil && c.options.Hooks.PostDelete != nil {
		c.options.Hooks.PostDelete(ctx, key, deleted > 0, nil)
	}

	return nil
}

// Clear removes all entries
func (c *RedisCache[T]) Clear(ctx context.Context) error {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "clear", "circuit_breaker", "availability", c.getMetricTags())
		return cacheErrors.ErrCircuitBreakerOpen
	}

	// Use SCAN to iteratively find keys instead of KEYS command
	var allKeys []string

	// Scan for data keys
	dataPattern := c.buildDataKey("*")
	if err := c.scanAndCollectKeys(ctx, dataPattern, &allKeys); err != nil {
		c.handleError("clear", err)
		return fmt.Errorf("redis clear error scanning data keys: %w", err)
	}

	// Scan for metadata keys
	metaPattern := c.buildMetaKey("*")
	if err := c.scanAndCollectKeys(ctx, metaPattern, &allKeys); err != nil {
		c.handleError("clear", err)
		return fmt.Errorf("redis clear error scanning meta keys: %w", err)
	}

	// Scan for index keys
	indexPattern := c.buildIndexKey("*", "*")
	if err := c.scanAndCollectKeys(ctx, indexPattern, &allKeys); err != nil {
		c.handleError("clear", err)
		return fmt.Errorf("redis clear error scanning index keys: %w", err)
	}

	if len(allKeys) == 0 {
		c.metrics.RecordOperation("redis", "clear", "empty", time.Since(start), c.getMetricTags())
		return nil
	}

	// Delete in batches to avoid blocking Redis
	batchSize := 100
	for i := 0; i < len(allKeys); i += batchSize {
		end := i + batchSize
		if end > len(allKeys) {
			end = len(allKeys)
		}

		if err := c.client.Del(ctx, allKeys[i:end]...).Err(); err != nil {
			c.handleError("clear", err)
			return fmt.Errorf("redis clear error deleting batch: %w", err)
		}
	}

	c.metrics.RecordOperation("redis", "clear", "success", time.Since(start), c.getMetricTags())
	return nil
}

// scanAndCollectKeys uses SCAN to collect keys matching pattern (instead of KEYS)
func (c *RedisCache[T]) scanAndCollectKeys(ctx context.Context, pattern string, keys *[]string) error {
	iter := c.client.Scan(ctx, 0, pattern, 1000).Iterator()
	for iter.Next(ctx) {
		*keys = append(*keys, iter.Val())
	}
	return iter.Err()
}

// Has checks if key exists without retrieving value
func (c *RedisCache[T]) Has(ctx context.Context, key string) bool {
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

// Atomic Operations - NOTE: GetOrSet and Update are implemented in atomic_operations.go

// Owner-based operations

// GetByOwner retrieves all entries for a given owner key using atomic Lua script
func (c *RedisCache[T]) GetByOwner(ctx context.Context, ownerKey string) ([]T, error) {
	start := time.Now()
	result := make([]T, 0)

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "getbyowner", "circuit_breaker", "availability", c.getMetricTags())
		return result, cacheErrors.ErrCircuitBreakerOpen
	}

	// Require indexing to be enabled
	if !c.indexingMode {
		return result, fmt.Errorf("GetByOwner requires indexing to be enabled")
	}

	indexKey := c.buildIndexKey("owner", ownerKey)
	dataPrefix := c.buildDataKey("")
	metaPrefix := c.buildMetaKey("")

	// Use atomic Lua script to get all entries and update access metadata
	scriptResult, err := c.getByOwnerScript.Run(ctx, c.client,
		[]string{indexKey},
		dataPrefix, metaPrefix).Result()

	if err != nil {
		c.handleError("getbyowner", err)
		c.metrics.RecordError("redis", "getbyowner", "redis_error", "infrastructure", c.getMetricTags())
		return result, fmt.Errorf("redis GetByOwner error: %w", err)
	}

	// Handle empty result
	resultSlice, ok := scriptResult.([]any)
	if !ok || len(resultSlice) == 0 {
		c.metrics.RecordOperation("redis", "getbyowner", "empty", time.Since(start), c.getMetricTags())
		return result, nil
	}

	// Process script results - each entry is [entryKey, serializedValue]
	for _, entry := range resultSlice {
		entryData, ok := entry.([]any)
		if !ok || len(entryData) < 2 {
			continue // Skip malformed entries
		}

		serializedValue, ok := entryData[1].(string)
		if !ok {
			continue // Skip if value is not a string
		}

		// Deserialize the value
		var value T
		if err := c.serializer.Deserialize([]byte(serializedValue), &value); err != nil {
			c.metrics.RecordError("redis", "getbyowner", "serialization_error", "data", c.getMetricTags())
			// Continue processing other entries instead of failing completely
			continue
		}

		result = append(result, value)
	}

	// Record appropriate metrics
	if len(result) > 0 {
		c.metrics.RecordHit("redis", c.getMetricTags())
		c.metrics.RecordOperation("redis", "getbyowner", "success", time.Since(start), c.getMetricTags())
	} else {
		c.metrics.RecordMiss("redis", c.getMetricTags())
		c.metrics.RecordOperation("redis", "getbyowner", "empty", time.Since(start), c.getMetricTags())
	}

	return result, nil
}

// DeleteByOwner removes all entries for a given owner key using atomic Lua script.
// Returns the number of sessions/entries deleted (not the total number of Redis keys deleted).
// For example, if 3 sessions are deleted, this returns 3, even though internally it may
// delete 10+ Redis keys (data, metadata, reverse indexes, etc.).
//
// Requires indexing to be enabled on the cache instance.
//
// Parameters:
//   - ctx: context for the operation
//   - ownerKey: the owner identifier (e.g., user ID)
//
// Returns:
//   - deletedCount: number of sessions/entries deleted (not Redis keys)
//   - err: error if operation fails or indexing is disabled
func (c *RedisCache[T]) DeleteByOwner(ctx context.Context, ownerKey string) (deletedCount int, err error) {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "deletebyowner", "circuit_breaker", "availability", c.getMetricTags())
		return 0, cacheErrors.ErrCircuitBreakerOpen
	}

	// Require indexing to be enabled
	if !c.indexingMode {
		return 0, fmt.Errorf("DeleteByOwner requires indexing to be enabled")
	}

	indexKey := c.buildIndexKey("owner", ownerKey)
	dataPrefix := c.buildDataKey("")
	metaPrefix := c.buildMetaKey("")
	reversePrefix := c.buildReverseIndexKey("")

	// Use atomic Lua script for complete deletion
	result, err := c.deleteByOwnerScript.Run(ctx, c.client,
		[]string{indexKey},
		dataPrefix, metaPrefix, reversePrefix).Result()

	if err != nil {
		c.handleError("deletebyowner", err)
		c.metrics.RecordError("redis", "deletebyowner", "redis_error", "infrastructure", c.getMetricTags())
		return 0, fmt.Errorf("redis DeleteByOwner error: %w", err)
	}

	deletedCount = int(result.(int64))
	c.metrics.RecordOperation("redis", "deletebyowner", "success", time.Since(start), c.getMetricTags())
	return deletedCount, nil
}

// Pattern operations

// GetKeysByPattern returns entry keys matching pattern - PERFORMANCE FIX: Uses SCAN instead of KEYS
func (c *RedisCache[T]) GetKeysByPattern(ctx context.Context, pattern string) ([]string, error) {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.metrics.RecordError("redis", "getkeysbypattern", "circuit_breaker", "availability", c.getMetricTags())
		return nil, cacheErrors.ErrCircuitBreakerOpen
	}

	// Build the full pattern with data prefix
	dataPattern := c.buildDataKey(pattern)

	var keys []string
	if err := c.scanAndCollectKeys(ctx, dataPattern, &keys); err != nil {
		c.handleError("getkeysbypattern", err)
		c.metrics.RecordError("redis", "getkeysbypattern", "redis_error", "infrastructure", c.getMetricTags())
		return nil, fmt.Errorf("redis GetKeysByPattern error: %w", err)
	}

	// Remove data prefix from keys to return clean keys
	result := make([]string, len(keys))
	dataPrefix := c.buildDataKey("")
	for i, fullKey := range keys {
		if len(fullKey) > len(dataPrefix) {
			result[i] = fullKey[len(dataPrefix):]
		} else {
			result[i] = fullKey
		}
	}

	c.metrics.RecordOperation("redis", "getkeysbypattern", "success", time.Since(start), c.getMetricTags())
	return result, nil
}

// Helper methods

// buildDataKey constructs the Redis key for data storage
func (c *RedisCache[T]) buildDataKey(key string) string {
	if c.redisOptions != nil && c.redisOptions.DataPrefix != "" {
		return c.redisOptions.DataPrefix + key
	}
	return "cache:data:" + key
}

// buildMetaKey constructs the Redis key for metadata storage
func (c *RedisCache[T]) buildMetaKey(key string) string {
	if c.redisOptions != nil && c.redisOptions.MetaPrefix != "" {
		return c.redisOptions.MetaPrefix + key
	}
	return "cache:meta:" + key
}

// buildIndexKey constructs the Redis key for index storage
func (c *RedisCache[T]) buildIndexKey(indexName, indexKey string) string {
	prefix := "cache:index:"
	if c.redisOptions != nil && c.redisOptions.IndexPrefix != "" {
		prefix = c.redisOptions.IndexPrefix
	}
	return fmt.Sprintf("%s%s:%s", prefix, indexName, indexKey)
}

// buildReverseIndexKey constructs the Redis key for reverse indexing (entry -> owner)
func (c *RedisCache[T]) buildReverseIndexKey(entryKey string) string {
	prefix := "cache:reverse:"
	if c.redisOptions != nil && c.redisOptions.IndexPrefix != "" {
		prefix = c.redisOptions.IndexPrefix + "reverse:"
	}
	return prefix + entryKey
}

// buildLockKey constructs the Redis key for distributed locks
func (c *RedisCache[T]) buildLockKey(key string) string {
	if c.redisOptions != nil && c.redisOptions.LockPrefix != "" {
		return c.redisOptions.LockPrefix + key
	}
	return "cache:lock:" + key
}

// getMetricTags returns metric tags for this cache instance
func (c *RedisCache[T]) getMetricTags() metric.Tags {
	tags := make(metric.Tags)
	if c.options.GlobalMetricsTags != nil {
		maps.Copy(tags, c.options.GlobalMetricsTags)
	}
	tags["provider"] = "redis"
	tags["instance_id"] = c.instanceID
	return tags
}

// Circuit breaker implementation

// isCircuitBreakerOpen checks if the circuit breaker is open
func (c *RedisCache[T]) isCircuitBreakerOpen() bool {
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
func (c *RedisCache[T]) handleError(operation string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.failureCount++
	c.lastFailureTime = time.Now()

	if c.failureCount >= circuitBreakerThreshold {
		c.circuitBreakerOpen = true
		c.metrics.RecordSecurityEvent("redis", "circuit_breaker_opened", "warning", c.getMetricTags())
	}
}

// Lifecycle management

// Close shuts down the cache and cleans up resources
// Note: This does NOT close the Redis client since it's provided externally.
// The caller who provided the client is responsible for closing it.
func (c *RedisCache[T]) Close() error {
	// Cache doesn't own the Redis client, so it doesn't close it
	// The client lifecycle is managed by the caller
	return nil
}

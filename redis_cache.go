package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/singleflight"

	cacheErrors "github.com/MichaelAJay/go-cache/cache_errors"
	"github.com/MichaelAJay/go-cache/config"
	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/MichaelAJay/go-cache/internal/slicepool"
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
	Version     string // Optional version suffix for schema migrations (e.g., "v2" -> "session:abc:v2")
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
	client             redis.Cmdable
	serializer         serializer.Serializer
	options            *config.CacheOptions
	precomputedMetrics *metrics.PrecomputedCacheMetrics

	extractor    *IndexExtractor[T] // nil = no indexing
	indexingMode bool               // derived from extractor != nil

	// Redis-specific options
	redisOptions *RedisOptions
	
	// Precomputed prefixes for optimal key building (1 allocation per key)
	dataPrefix     string
	metaPrefix     string
	indexPrefix    string
	lockPrefix     string
	reversePrefix  string
	lruTrackerKey  string
	versionSuffix  string

	// Circuit breaker state
	mu                 sync.RWMutex
	circuitBreakerOpen bool
	lastFailureTime    time.Time
	failureCount       int

	// Instance identifier for distributed coordination
	instanceID string

	// Singleflight group for coordinating concurrent GetOrSet operations
	sf singleflight.Group

	// Memory tracking
	memoryTracker *memoryTracker

	// Slice pool for batch operation optimization
	slicePool *slicepool.SlicePool

	// String builder pool for key construction optimization
	builderPool sync.Pool

	// Lua scripts for atomic operations
	getScript                     *redis.Script
	setScript                     *redis.Script
	getOrSetScript                *redis.Script
	updateScript                  *redis.Script
	deleteByIndexScript           *redis.Script
	deleteByEntryScript           *redis.Script
	getByOwnerScript              *redis.Script
	deleteByOwnerScript           *redis.Script
	setIfExistsScript             *redis.Script
	setIfNotExistsScript          *redis.Script
	getManyMetadataUpdateScript   *redis.Script
	cleanupOrphanedMetadataScript *redis.Script
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

// WithVersion sets a version suffix for cache keys for schema migrations
// Usage: WithVersion("v2") will append ":v2" to all cache keys
func WithVersion[T any](version string) Option[T] {
	return func(cache *RedisCache[T]) {
		if cache.redisOptions == nil {
			cache.redisOptions = &RedisOptions{}
		}
		cache.redisOptions.Version = version
	}
}

// WithWarmLuaScripts enables or disables Lua script pre-loading
func WithWarmLuaScripts[T any](warmScripts bool) Option[T] {
	return func(cache *RedisCache[T]) {
		cache.options.WarmLuaScripts = warmScripts
	}
}

// NewCache creates a new Redis cache instance with clean abstraction
// indexingMode explicitly controls whether indexing features are enabled
// extractor provides key extraction functions - GetEntryKey always required, GetOwnerKey only when indexing enabled
// warmPoolCount pre-warms pools with specified number of items to reduce initial allocation overhead
// Valid combinations:
//   - indexingMode=false, extractor with GetEntryKey (basic caching)
//   - indexingMode=true, extractor with GetEntryKey & GetOwnerKey (indexed caching)
func NewCache[T any](ctx context.Context, client redis.Cmdable, indexingMode bool, extractor *IndexExtractor[T], warmPoolCount int, opts ...Option[T]) (interfaces.Cache[T], error) {
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
	if err := cache.initialize(warmPoolCount); err != nil {
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
func (c *RedisCache[T]) initialize(warmPoolCount int) error {
	// Initialize serializer
	var ser serializer.Serializer
	if c.options.SerializerFormat != "" {
		switch c.options.SerializerFormat {
		case "json":
			ser = serializer.NewJSONSerializer(32 * 1064)
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

	// Initialize pre-computed metrics for zero-allocation operations
	if c.options.GoMetricsRegistry == nil {
		return fmt.Errorf("GoMetricsRegistry is required")
	}
	// Create final tags once during initialization to avoid runtime allocation
	finalTags := make(metric.Tags)
	if c.options.GlobalMetricsTags != nil {
		maps.Copy(finalTags, c.options.GlobalMetricsTags)
	}
	finalTags["provider"] = "redis"
	finalTags["instance_id"] = c.instanceID
	c.precomputedMetrics = metrics.NewPrecomputedCacheMetrics(c.options.GoMetricsRegistry, finalTags)

	// Initialize memory tracker if enabled
	if c.options.MemoryTrackingEnabled {
		config := memoryTrackerConfig{
			samplingRate:     c.options.MemoryUsageSamplingRate,
			samplingInterval: c.options.MemoryUsageSamplingInterval,
			thresholdBytes:   c.options.MemoryPressureThresholdBytes,
			thresholdPercent: c.options.MemoryPressureThresholdPercent,
		}
		c.memoryTracker = NewMemoryTracker(c.client, config)
	}

	// Initialize slice pool for batch operation optimization
	c.slicePool = slicepool.NewSlicePool()

	// Initialize string builder pool for key construction optimization
	c.builderPool = sync.Pool{
		New: func() any {
			return &strings.Builder{}
		},
	}

	// Initialize Lua scripts for atomic operations
	c.initLuaScripts()

	// Pre-warm pools to reduce initial allocation overhead
	if warmPoolCount > 0 {
		c.warmPools(warmPoolCount)
	}

	// Precompute prefixes for optimal key building (1 allocation per key instead of 2-3)
	c.precomputePrefixes()

	return nil
}

// warmPools pre-warms both slice pool and string pool with the specified number of items
// This reduces allocation overhead during the first operations by pre-allocating pool items
func (c *RedisCache[T]) warmPools(warmCount int) {
	// Warm []string pool
	for range warmCount {
		s := make([]string, 0, 16) // capacity = 16
		c.slicePool.PutStringSlice(s)
	}

	// Warm []any pool
	for range warmCount {
		a := make([]any, 0, 16) // capacity = 16
		c.slicePool.PutAnySlice(a)
	}

	// Warm string builder pool
	for range warmCount {
		b := &strings.Builder{}
		c.builderPool.Put(b)
	}
}

// precomputePrefixes calculates final prefixes once at initialization to enable 1-allocation key building
func (c *RedisCache[T]) precomputePrefixes() {
	// Determine base prefixes
	var baseDataPrefix, baseMetaPrefix, baseIndexPrefix, baseLockPrefix string
	
	if c.redisOptions != nil {
		if c.redisOptions.DataPrefix != "" {
			baseDataPrefix = c.redisOptions.DataPrefix
		} else {
			baseDataPrefix = "cache:data:"
		}
		
		if c.redisOptions.MetaPrefix != "" {
			baseMetaPrefix = c.redisOptions.MetaPrefix
		} else {
			baseMetaPrefix = "cache:meta:"
		}
		
		if c.redisOptions.IndexPrefix != "" {
			baseIndexPrefix = c.redisOptions.IndexPrefix
		} else {
			baseIndexPrefix = "cache:index:"
		}
		
		if c.redisOptions.LockPrefix != "" {
			baseLockPrefix = c.redisOptions.LockPrefix
		} else {
			baseLockPrefix = "cache:lock:"
		}
	} else {
		baseDataPrefix = "cache:data:"
		baseMetaPrefix = "cache:meta:"
		baseIndexPrefix = "cache:index:"
		baseLockPrefix = "cache:lock:"
	}
	
	// Add version suffix if configured
	var versionSuffix string
	if c.redisOptions != nil && c.redisOptions.Version != "" {
		versionSuffix = ":" + c.redisOptions.Version
	}
	
	// Precompute final prefixes (these become the effective prefixes for runtime concatenation)
	c.dataPrefix = baseDataPrefix    // Will concatenate: dataPrefix + key + versionSuffix
	c.metaPrefix = baseMetaPrefix    // Will concatenate: metaPrefix + key + versionSuffix  
	c.indexPrefix = baseIndexPrefix
	c.lockPrefix = baseLockPrefix    
	c.reversePrefix = "cache:reverse:"
	if c.redisOptions != nil && c.redisOptions.IndexPrefix != "" {
		c.reversePrefix = c.redisOptions.IndexPrefix + "reverse:"
	}
	
	// Precompute LRU tracker key (fully static, no per-operation building needed)
	if baseDataPrefix == "cache:data:" {
		c.lruTrackerKey = "cache:lru:tracker" + versionSuffix
	} else {
		c.lruTrackerKey = baseDataPrefix + "lru:tracker" + versionSuffix
	}
	
	// Store version suffix for use in key building methods
	c.versionSuffix = versionSuffix
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
		c.precomputedMetrics.GetCircuitBreakerErrorCounter().Inc()
		return zero, false, cacheErrors.ErrCircuitBreakerOpen
	}

	dataKey := c.buildDataKey(key)
	metaKey := c.buildMetaKey(key)
	lruTrackerKey := c.buildLRUTrackerKey()

	// Get pooled slice for script arguments to avoid allocation
	scriptArgs := c.slicePool.GetStringSlice(3)
	defer c.slicePool.PutStringSlice(scriptArgs)

	// Extend slice to required length
	scriptArgs = scriptArgs[:3]
	scriptArgs[0] = dataKey
	scriptArgs[1] = metaKey
	scriptArgs[2] = lruTrackerKey

	// Use Lua script for atomic get and metadata update with LRU tracking
	result, err := c.getScript.Run(ctx, c.client, scriptArgs, key).Result()
	if err != nil {
		c.handleError("get", err)
		c.precomputedMetrics.GetRedisErrorCounter().Inc()
		return zero, false, fmt.Errorf("redis get error: %w", err)
	}

	resultSlice, ok := result.([]any)
	if !ok {
		return zero, false, fmt.Errorf("unexpected script result type: %T", result)
	}

	// Handle empty result (cache miss)
	if len(resultSlice) == 0 {
		duration := time.Since(start)
		c.precomputedMetrics.GetMissCounter().Inc()
		c.precomputedMetrics.GeneralMissCounter().Inc()
		c.precomputedMetrics.GetTimer().Record(duration)
		return zero, false, nil
	}

	if len(resultSlice) < 2 {
		return zero, false, fmt.Errorf("unexpected script result length: %d", len(resultSlice))
	}

	serializedValue := resultSlice[0]
	found := resultSlice[1].(string) == "1"

	if !found {
		duration := time.Since(start)
		c.precomputedMetrics.GetMissCounter().Inc()
		c.precomputedMetrics.GeneralMissCounter().Inc()
		c.precomputedMetrics.GetTimer().Record(duration)
		return zero, false, nil
	}

	// Deserialize value using StringDeserializer optimization if available
	var value T
	serializedString := serializedValue.(string)
	if stringDeser, ok := c.serializer.(serializer.StringDeserializer); ok {
		err = stringDeser.DeserializeString(serializedString, &value)
	} else {
		err = c.serializer.Deserialize([]byte(serializedString), &value)
	}
	if err != nil {
		c.precomputedMetrics.GetSerializationErrorCounter().Inc()
		return zero, false, fmt.Errorf("deserialization error for key %s: %w", key, err)
	}

	duration := time.Since(start)
	c.precomputedMetrics.GetHitCounter().Inc()
	c.precomputedMetrics.GetTimer().Record(duration)
	c.precomputedMetrics.GetSuccessCounter().Inc()
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
		c.precomputedMetrics.SetCircuitBreakerErrorCounter().Inc()
		return cacheErrors.ErrCircuitBreakerOpen
	}

	// Serialize value
	serializedValue, err := c.serializer.Serialize(value)
	if err != nil {
		c.precomputedMetrics.SetSerializationErrorCounter().Inc()
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

	// LRU tracking parameters
	lruTrackerKey := c.buildLRUTrackerKey()
	dataPrefix := c.buildDataKey("")
	metaPrefix := c.buildMetaKey("")

	result, err := c.setScript.Run(ctx, c.client,
		[]string{dataKey, metaKey, indexKey, reverseKey, lruTrackerKey},
		string(serializedValue), ttlInMilliseconds, key, ownerKey, fmt.Sprintf("%t", c.indexingMode), c.options.MaxEntries, dataPrefix, metaPrefix).Result()

	if err != nil {
		c.handleError("set", err)
		c.precomputedMetrics.SetRedisErrorCounter().Inc()
		return fmt.Errorf("redis set error: %w", err)
	}

	// Validate script result
	if result != "OK" {
		return fmt.Errorf("unexpected set script result: %v", result)
	}

	// Record memory usage after successful set operation
	if c.memoryTracker != nil {
		c.memoryTracker.RecordSet(key, serializedValue)

		// Check if memory sampling should occur
		if c.memoryTracker.ShouldSample() {
			dataPrefix := c.buildDataKey("")
			if err := c.memoryTracker.PerformMemorySample(ctx, dataPrefix); err != nil {
				// Log sampling error but don't fail the operation
				c.precomputedMetrics.SetMemorySamplingErrorCounter().Inc()
			}
		}

		// Record memory usage metrics
		c.recordMemoryUsageMetrics(ctx)
	}

	duration := time.Since(start)
	c.precomputedMetrics.SetTimer().Record(duration)
	c.precomputedMetrics.SetSuccessCounter().Inc()
	return nil
}

// Delete removes a key
func (c *RedisCache[T]) Delete(ctx context.Context, key string) error {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.precomputedMetrics.DeleteCircuitBreakerErrorCounter().Inc()
		return cacheErrors.ErrCircuitBreakerOpen
	}

	dataKey := c.buildDataKey(key)
	metaKey := c.buildMetaKey(key)

	var deleted int64

	if c.indexingMode {
		// Use atomic script to clean up indexes
		reverseKey := c.buildReverseIndexKey(key)
		indexPrefix := "cache:index:"
		if c.redisOptions != nil && c.redisOptions.IndexPrefix != "" {
			indexPrefix = c.redisOptions.IndexPrefix
		}

		lruTrackerKey := c.buildLRUTrackerKey()
		result, scriptErr := c.deleteByEntryScript.Run(ctx, c.client,
			[]string{dataKey, metaKey, reverseKey, lruTrackerKey},
			key, indexPrefix).Result()

		if scriptErr != nil {
			c.handleError("delete", scriptErr)
			c.precomputedMetrics.DeleteRedisErrorCounter().Inc()
			return fmt.Errorf("redis delete error: %w", scriptErr)
		}
		deleted = result.(int64)
	} else {
		// Simple delete without index cleanup but with LRU tracker cleanup
		lruTrackerKey := c.buildLRUTrackerKey()
		pipe := c.client.TxPipeline()
		pipe.Del(ctx, dataKey, metaKey)
		pipe.ZRem(ctx, lruTrackerKey, key)
		results, err := pipe.Exec(ctx)
		if err != nil {
			c.handleError("delete", err)
			c.precomputedMetrics.DeleteRedisErrorCounter().Inc()
			return fmt.Errorf("redis delete error: %w", err)
		}
		deleted = results[0].(*redis.IntCmd).Val()
	}

	// Record memory usage after successful delete operation
	if c.memoryTracker != nil && deleted > 0 {
		c.memoryTracker.RecordDelete(key)

		// Check if memory sampling should occur
		if c.memoryTracker.ShouldSample() {
			dataPrefix := c.buildDataKey("")
			if err := c.memoryTracker.PerformMemorySample(ctx, dataPrefix); err != nil {
				// Log sampling error but don't fail the operation
				c.precomputedMetrics.DeleteMemorySamplingErrorCounter().Inc()
			}
		}

		// Record memory usage metrics
		c.recordMemoryUsageMetrics(ctx)
	}

	duration := time.Since(start)
	c.precomputedMetrics.DeleteTimer().Record(duration)
	c.precomputedMetrics.DeleteSuccessCounter().Inc()

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
		c.precomputedMetrics.ClearCircuitBreakerErrorCounter().Inc()
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
		c.precomputedMetrics.ClearTimer().Record(time.Since(start))
		c.precomputedMetrics.ClearEmptyCounter().Inc()
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

	// Reset memory tracking and record zero memory usage
	if c.memoryTracker != nil {
		c.memoryTracker.Reset()
		c.recordMemoryUsageMetrics(ctx)
	}

	c.precomputedMetrics.ClearTimer().Record(time.Since(start))
	c.precomputedMetrics.ClearSuccessCounter().Inc()
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
		c.precomputedMetrics.HasCircuitBreakerErrorCounter().Inc()
		return false
	}

	dataKey := c.buildDataKey(key)
	exists, err := c.client.Exists(ctx, dataKey).Result()
	if err != nil {
		c.handleError("has", err)
		c.precomputedMetrics.HasRedisErrorCounter().Inc()
		return false
	}

	duration := time.Since(start)
	c.precomputedMetrics.HasTimer().Record(duration)
	c.precomputedMetrics.HasSuccessCounter().Inc()
	return exists > 0
}

// Atomic Operations - NOTE: GetOrSet and Update are implemented in atomic_operations.go

// Owner-based operations

// GetByOwner retrieves all entries for a given owner key using atomic Lua script
func (c *RedisCache[T]) GetByOwner(ctx context.Context, ownerKey string) ([]T, error) {
	start := time.Now()
	result := make([]T, 0)

	if c.isCircuitBreakerOpen() {
		c.precomputedMetrics.GetByOwnerCircuitBreakerErrorCounter().Inc()
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
		c.precomputedMetrics.GetByOwnerRedisErrorCounter().Inc()
		return result, fmt.Errorf("redis GetByOwner error: %w", err)
	}

	// Handle empty result
	resultSlice, ok := scriptResult.([]any)
	if !ok || len(resultSlice) == 0 {
		c.precomputedMetrics.GetByOwnerTimer().Record(time.Since(start))
		c.precomputedMetrics.GetByOwnerEmptyCounter().Inc()
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
			c.precomputedMetrics.GetByOwnerSerializationErrorCounter().Inc()
			// Continue processing other entries instead of failing completely
			continue
		}

		result = append(result, value)
	}

	// Record appropriate metrics
	if len(result) > 0 {
		c.precomputedMetrics.GetByOwnerHitCounter().Inc()
		c.precomputedMetrics.GetByOwnerTimer().Record(time.Since(start))
		c.precomputedMetrics.GetByOwnerSuccessCounter().Inc()
	} else {
		c.precomputedMetrics.GetByOwnerMissCounter().Inc()
		c.precomputedMetrics.GetByOwnerTimer().Record(time.Since(start))
		c.precomputedMetrics.GetByOwnerEmptyCounter().Inc()
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
		c.precomputedMetrics.DeleteByOwnerCircuitBreakerErrorCounter().Inc()
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
		c.precomputedMetrics.DeleteByOwnerRedisErrorCounter().Inc()
		return 0, fmt.Errorf("redis DeleteByOwner error: %w", err)
	}

	deletedCount = int(result.(int64))
	c.precomputedMetrics.DeleteByOwnerTimer().Record(time.Since(start))
	c.precomputedMetrics.DeleteByOwnerSuccessCounter().Inc()
	return deletedCount, nil
}

// Pattern operations

// GetKeysByPattern returns entry keys matching pattern - PERFORMANCE FIX: Uses SCAN instead of KEYS
func (c *RedisCache[T]) GetKeysByPattern(ctx context.Context, pattern string) ([]string, error) {
	start := time.Now()

	if c.isCircuitBreakerOpen() {
		c.precomputedMetrics.GetKeysByPatternCircuitBreakerErrorCounter().Inc()
		return nil, cacheErrors.ErrCircuitBreakerOpen
	}

	// Build the full pattern with data prefix
	dataPattern := c.buildDataKey(pattern)

	var keys []string
	if err := c.scanAndCollectKeys(ctx, dataPattern, &keys); err != nil {
		c.handleError("getkeysbypattern", err)
		c.precomputedMetrics.GetKeysByPatternRedisErrorCounter().Inc()
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

	c.precomputedMetrics.GetKeysByPatternTimer().Record(time.Since(start))
	c.precomputedMetrics.GetKeysByPatternSuccessCounter().Inc()
	return result, nil
}

// Helper methods

// buildDataKey constructs the Redis key for data storage using precomputed prefix (1 allocation)
func (c *RedisCache[T]) buildDataKey(key string) string {
	if c.versionSuffix == "" {
		// Fast path: prefix + key (1 allocation)
		return c.dataPrefix + key
	}
	// Version path: prefix + key + versionSuffix (1 allocation)
	return c.dataPrefix + key + c.versionSuffix
}

// buildMetaKey constructs the Redis key for metadata storage using precomputed prefix (1 allocation)
func (c *RedisCache[T]) buildMetaKey(key string) string {
	if c.versionSuffix == "" {
		// Fast path: prefix + key (1 allocation)
		return c.metaPrefix + key
	}
	// Version path: prefix + key + versionSuffix (1 allocation)
	return c.metaPrefix + key + c.versionSuffix
}

// buildIndexKey constructs the Redis key for index storage using precomputed prefix (1 allocation)
func (c *RedisCache[T]) buildIndexKey(indexName, indexKey string) string {
	return c.indexPrefix + indexName + ":" + indexKey
}

// buildReverseIndexKey constructs the Redis key for reverse indexing using precomputed prefix (1 allocation)
func (c *RedisCache[T]) buildReverseIndexKey(entryKey string) string {
	if c.versionSuffix == "" {
		return c.reversePrefix + entryKey
	}
	return c.reversePrefix + entryKey + c.versionSuffix
}

// buildLockKey constructs the Redis key for distributed locks using precomputed prefix (1 allocation)
func (c *RedisCache[T]) buildLockKey(key string) string {
	if c.versionSuffix == "" {
		return c.lockPrefix + key
	}
	return c.lockPrefix + key + c.versionSuffix
}

// buildLRUTrackerKey returns the precomputed LRU tracker key (0 allocations)
func (c *RedisCache[T]) buildLRUTrackerKey() string {
	return c.lruTrackerKey
}

// buildDataKeysMany efficiently builds multiple data keys using a single string builder
// This reduces allocation overhead compared to calling buildDataKey individually
func (c *RedisCache[T]) buildDataKeysMany(keys []string, dataKeys []string) {
	if len(keys) != len(dataKeys) {
		panic("keys and dataKeys slices must have same length")
	}

	// Get single string builder for all operations
	builder := c.builderPool.Get().(*strings.Builder)
	defer func() {
		builder.Reset()
		c.builderPool.Put(builder)
	}()

	// Determine if we have version/prefix to avoid repeated checks
	hasVersion := c.redisOptions != nil && c.redisOptions.Version != ""
	hasPrefix := c.redisOptions != nil && c.redisOptions.DataPrefix != ""

	var dataPrefix string
	if hasPrefix {
		dataPrefix = c.redisOptions.DataPrefix
	} else {
		dataPrefix = "cache:data:"
	}

	// Build all keys efficiently
	for i, key := range keys {
		builder.Reset()

		// Build the key with version if needed
		builder.WriteString(key)
		if hasVersion {
			builder.WriteString(":")
			builder.WriteString(c.redisOptions.Version)
		}

		// Get intermediate result
		keyWithVersion := builder.String()
		builder.Reset()

		// Add prefix and store final result
		builder.WriteString(dataPrefix)
		builder.WriteString(keyWithVersion)
		dataKeys[i] = builder.String()
	}
}

// buildMetaKeysMany efficiently builds multiple metadata keys using a single string builder
// This reduces allocation overhead compared to calling buildMetaKey individually
func (c *RedisCache[T]) buildMetaKeysMany(keys []string, metaKeys []string) {
	if len(keys) != len(metaKeys) {
		panic("keys and metaKeys slices must have same length")
	}

	// Get single string builder for all operations
	builder := c.builderPool.Get().(*strings.Builder)
	defer func() {
		builder.Reset()
		c.builderPool.Put(builder)
	}()

	// Determine if we have version/prefix to avoid repeated checks
	hasVersion := c.redisOptions != nil && c.redisOptions.Version != ""
	hasPrefix := c.redisOptions != nil && c.redisOptions.MetaPrefix != ""

	var metaPrefix string
	if hasPrefix {
		metaPrefix = c.redisOptions.MetaPrefix
	} else {
		metaPrefix = "cache:meta:"
	}

	// Build all keys efficiently
	for i, key := range keys {
		builder.Reset()

		// Build the key with version if needed
		builder.WriteString(key)
		if hasVersion {
			builder.WriteString(":")
			builder.WriteString(c.redisOptions.Version)
		}

		// Get intermediate result
		keyWithVersion := builder.String()
		builder.Reset()

		// Add prefix and store final result
		builder.WriteString(metaPrefix)
		builder.WriteString(keyWithVersion)
		metaKeys[i] = builder.String()
	}
}

// recordMemoryUsageMetrics records current memory usage and checks for pressure
func (c *RedisCache[T]) recordMemoryUsageMetrics(ctx context.Context) {
	if c.memoryTracker == nil {
		return
	}

	// Get current memory usage from tracker
	memoryBytes, _ := c.memoryTracker.GetCurrentUsage()

	// Record memory usage metrics
	c.precomputedMetrics.MemoryUsageGauge().Set(float64(memoryBytes))

	// Check for memory pressure and record alerts if threshold exceeded
	if c.memoryTracker.IsMemoryPressure(ctx) {
		// Determine which threshold was exceeded for metrics recording
		var threshold int64

		// Check absolute threshold first
		if c.options.MemoryPressureThresholdBytes > 0 && memoryBytes >= c.options.MemoryPressureThresholdBytes {
			threshold = c.options.MemoryPressureThresholdBytes
		} else if c.options.MemoryPressureThresholdPercent > 0 {
			// For percentage threshold, we need the Redis maxmemory
			// Since getRedisMaxMemory is not exported, we'll use the current memory as threshold approximation
			// This is not perfect but provides useful metrics
			threshold = int64(float64(memoryBytes) / (c.options.MemoryPressureThresholdPercent / 100.0))
		}

		// If we couldn't determine a specific threshold, use the current memory as the threshold
		// This ensures we always record the pressure event
		if threshold == 0 {
			threshold = memoryBytes
		}

		c.precomputedMetrics.MemoryPressureCounter().Inc()
	}
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
		c.precomputedMetrics.SecurityEventCounter().Inc()
	}
}

// Atomic counter operations

// Increment atomically increments a counter key by the specified delta
func (c *RedisCache[T]) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	// Circuit breaker check
	if c.isCircuitBreakerOpen() {
		c.precomputedMetrics.IncrementCircuitBreakerErrorCounter().Inc()
		return 0, cacheErrors.ErrCircuitBreakerOpen
	}

	start := time.Now()

	// Build the data key with proper prefix
	dataKey := c.buildDataKey(key)

	// Use Redis INCRBY for atomic increment
	result, err := c.client.IncrBy(ctx, dataKey, delta).Result()

	duration := time.Since(start)

	if err != nil {
		c.handleError("increment", err)

		// Categorize error for metrics
		errorType := "redis_error"
		if err == redis.Nil {
			errorType = "key_not_found"
		}

		if errorType == "redis_error" {
			c.precomputedMetrics.IncrementRedisErrorCounter().Inc()
		} else {
			c.precomputedMetrics.IncrementTimeoutErrorCounter().Inc()
		}
		return 0, fmt.Errorf("increment operation failed: %w", err)
	}

	// Record successful operation
	c.precomputedMetrics.IncrementTimer().Record(duration)
	c.precomputedMetrics.IncrementSuccessCounter().Inc()

	return result, nil
}

// Decrement atomically decrements a counter key by the specified delta
func (c *RedisCache[T]) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	// Circuit breaker check
	if c.isCircuitBreakerOpen() {
		c.precomputedMetrics.DecrementCircuitBreakerErrorCounter().Inc()
		return 0, cacheErrors.ErrCircuitBreakerOpen
	}

	start := time.Now()

	// Build the data key with proper prefix
	dataKey := c.buildDataKey(key)

	// Use Redis DECRBY for atomic decrement
	result, err := c.client.DecrBy(ctx, dataKey, delta).Result()

	duration := time.Since(start)

	if err != nil {
		c.handleError("decrement", err)

		// Categorize error for metrics
		errorType := "redis_error"
		if err == redis.Nil {
			errorType = "key_not_found"
		}

		if errorType == "redis_error" {
			c.precomputedMetrics.DecrementRedisErrorCounter().Inc()
		} else {
			c.precomputedMetrics.DecrementTimeoutErrorCounter().Inc()
		}
		return 0, fmt.Errorf("decrement operation failed: %w", err)
	}

	// Record successful operation
	c.precomputedMetrics.DecrementTimer().Record(duration)
	c.precomputedMetrics.DecrementSuccessCounter().Inc()

	return result, nil
}

// IncrementFloat atomically increments a floating-point counter key by the specified delta
func (c *RedisCache[T]) IncrementFloat(ctx context.Context, key string, delta float64) (float64, error) {
	// Circuit breaker check
	if c.isCircuitBreakerOpen() {
		c.precomputedMetrics.IncrementFloatCircuitBreakerErrorCounter().Inc()
		return 0, cacheErrors.ErrCircuitBreakerOpen
	}

	start := time.Now()

	// Build the data key with proper prefix
	dataKey := c.buildDataKey(key)

	// Use Redis INCRBYFLOAT for atomic float increment
	result, err := c.client.IncrByFloat(ctx, dataKey, delta).Result()

	duration := time.Since(start)

	if err != nil {
		c.handleError("increment_float", err)

		// Categorize error for metrics
		errorType := "redis_error"
		if err == redis.Nil {
			errorType = "key_not_found"
		}

		if errorType == "redis_error" {
			c.precomputedMetrics.IncrementFloatRedisErrorCounter().Inc()
		} else {
			c.precomputedMetrics.IncrementFloatTimeoutErrorCounter().Inc()
		}
		return 0, fmt.Errorf("increment float operation failed: %w", err)
	}

	// Record successful operation
	c.precomputedMetrics.IncrementFloatTimer().Record(duration)
	c.precomputedMetrics.IncrementFloatSuccessCounter().Inc()

	return result, nil
}

// Session management operations

// ExtendTTL atomically extends the TTL of a cache entry without modifying its data
func (c *RedisCache[T]) ExtendTTL(ctx context.Context, key string, ttl time.Duration) error {
	// Circuit breaker check
	if c.isCircuitBreakerOpen() {
		c.precomputedMetrics.ExtendTTLCircuitBreakerErrorCounter().Inc()
		return cacheErrors.ErrCircuitBreakerOpen
	}

	start := time.Now()

	// Build the data key with proper prefix
	dataKey := c.buildDataKey(key)
	metaKey := c.buildMetaKey(key)

	// Use Redis PEXPIRE for atomic TTL extension with millisecond precision
	ttlMs := ttl.Milliseconds()

	// Set TTL on both data and metadata keys
	pipe := c.client.Pipeline()
	dataResult := pipe.PExpire(ctx, dataKey, ttl)
	metaResult := pipe.PExpire(ctx, metaKey, ttl)
	_, err := pipe.Exec(ctx)

	duration := time.Since(start)

	if err != nil {
		c.handleError("extend_ttl", err)
		c.precomputedMetrics.ExtendTTLRedisErrorCounter().Inc()
		return fmt.Errorf("extend TTL operation failed: %w", err)
	}

	// Check if the key actually existed
	dataExists, _ := dataResult.Result()
	if !dataExists {
		c.precomputedMetrics.ExtendTTLKeyNotFoundErrorCounter().Inc()
		return fmt.Errorf("key does not exist: %s", key)
	}

	// Update metadata TTL field
	if metaExists, _ := metaResult.Result(); metaExists {
		c.client.HSet(ctx, metaKey, "ttl", fmt.Sprintf("%d", ttlMs))
	}

	// Record successful operation
	c.precomputedMetrics.ExtendTTLTimer().Record(duration)
	c.precomputedMetrics.ExtendTTLSuccessCounter().Inc()

	return nil
}

// Touch atomically updates the last-accessed timestamp and extends TTL for a cache entry
func (c *RedisCache[T]) Touch(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	// Circuit breaker check
	if c.isCircuitBreakerOpen() {
		c.precomputedMetrics.TouchCircuitBreakerErrorCounter().Inc()
		return false, cacheErrors.ErrCircuitBreakerOpen
	}

	start := time.Now()

	// Build the keys
	dataKey := c.buildDataKey(key)
	metaKey := c.buildMetaKey(key)

	// Use Lua script for atomic touch operation
	script := `
		local dataKey = KEYS[1]
		local metaKey = KEYS[2]
		local ttlMs = tonumber(ARGV[1])
		
		-- Check if data key exists
		if redis.call('EXISTS', dataKey) == 0 then
			return 0
		end
		
		-- Extend TTL on both keys
		redis.call('PEXPIRE', dataKey, ttlMs)
		redis.call('PEXPIRE', metaKey, ttlMs)
		
		-- Update metadata with current timestamp and access count
		local now = redis.call('TIME')
		local ts = now[1]
		local acc = redis.call('HGET', metaKey, 'access_count') or '0'
		acc = tostring((tonumber(acc) or 0) + 1)
		
		redis.call('HSET', metaKey,
			'last_accessed', ts,
			'access_count', acc,
			'ttl', tostring(ttlMs)
		)
		
		return 1
	`

	ttlMs := ttl.Milliseconds()
	result, err := c.client.Eval(ctx, script, []string{dataKey, metaKey}, ttlMs).Result()

	duration := time.Since(start)

	if err != nil {
		c.handleError("touch", err)
		c.precomputedMetrics.TouchRedisErrorCounter().Inc()
		return false, fmt.Errorf("touch operation failed: %w", err)
	}

	exists := result.(int64) == 1

	if !exists {
		c.precomputedMetrics.TouchKeyNotFoundErrorCounter().Inc()
	} else {
		c.precomputedMetrics.TouchTimer().Record(duration)
		c.precomputedMetrics.TouchSuccessCounter().Inc()
	}

	return exists, nil
}

// AppendToField atomically appends a value to a string field within a cached entry
// This is useful for activity logs, session traces, etc.
func (c *RedisCache[T]) AppendToField(ctx context.Context, key, fieldPath, value string, ttl time.Duration) error {
	// Circuit breaker check
	if c.isCircuitBreakerOpen() {
		c.precomputedMetrics.AppendFieldCircuitBreakerErrorCounter().Inc()
		return cacheErrors.ErrCircuitBreakerOpen
	}

	start := time.Now()

	// Build the keys
	dataKey := c.buildDataKey(key)
	metaKey := c.buildMetaKey(key)

	// For simple string append, use Redis APPEND command
	// This works for simple string fields, not complex JSON paths
	if fieldPath == "" {
		// Append to the entire value (treat as string)
		pipe := c.client.Pipeline()
		appendResult := pipe.Append(ctx, dataKey, value)
		pipe.PExpire(ctx, dataKey, ttl)
		pipe.PExpire(ctx, metaKey, ttl)
		_, err := pipe.Exec(ctx)

		duration := time.Since(start)

		if err != nil {
			c.handleError("append_field", err)
			c.precomputedMetrics.AppendFieldRedisErrorCounter().Inc()
			return fmt.Errorf("append field operation failed: %w", err)
		}

		// Update metadata
		if appendResult.Val() > 0 {
			now := time.Now().Unix()
			c.client.HSet(ctx, metaKey,
				"last_accessed", fmt.Sprintf("%d", now),
				"ttl", fmt.Sprintf("%d", ttl.Milliseconds()),
				"size", fmt.Sprintf("%d", appendResult.Val()),
			)
		}

		c.precomputedMetrics.AppendFieldTimer().Record(duration)
		c.precomputedMetrics.AppendFieldSuccessCounter().Inc()
		return nil
	}

	// For complex field paths, this would require JSON manipulation
	// For now, return an error indicating this feature needs implementation
	c.precomputedMetrics.AppendFieldUnsupportedOperationErrorCounter().Inc()
	return fmt.Errorf("complex field path operations not yet implemented: %s", fieldPath)
}

// Lifecycle management

// Close shuts down the cache and cleans up resources
// Note: This does NOT close the Redis client since it's provided externally.
// The caller who provided the client is responsible for closing it.
func (c *RedisCache[T]) Close() error {
	// Clean up memory tracker if it exists
	if c.memoryTracker != nil {
		// Memory tracker doesn't have background goroutines to clean up in this implementation
		// If background sampling is added later, cancellation logic would go here
		c.memoryTracker = nil
	}

	// Cache doesn't own the Redis client, so it doesn't close it
	// The client lifecycle is managed by the caller
	return nil
}

package memory

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	cacheErrors "github.com/MichaelAJay/go-cache/cache_errors"
	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/MichaelAJay/go-cache/metrics"
	"github.com/MichaelAJay/go-metrics/metric"
	"github.com/MichaelAJay/go-serializer"
)

const (
	// Default values
	defaultKeyTTL = time.Hour * 24 * 7 // 1 week
)

// memoryCache implements the Cache interface using an in-memory map
type memoryCache struct {
	items      map[string]*cacheEntry
	mu         sync.RWMutex
	options    *interfaces.CacheOptions
	serializer serializer.Serializer

	// Metrics - support both old and new systems
	legacyMetrics   *cacheMetrics                   // Legacy metrics for backward compatibility
	enhancedMetrics interfaces.EnhancedCacheMetrics // New go-metrics based system
	providerName    string                          // Provider name for metrics tagging

	cleanupTicker   *time.Ticker
	cleanupStopChan chan struct{}

	// Enhanced features
	indexes        map[string]map[string][]string // indexName -> indexKey -> []primaryKeys
	indexPatterns  map[string]string              // indexName -> keyPattern
	hooks          *interfaces.CacheHooks
	securityConfig *interfaces.SecurityConfig
}

// cacheMetrics implements a thread-safe metrics collector
type cacheMetrics struct {
	hits          int64
	misses        int64
	getLatency    time.Duration
	setLatency    time.Duration
	deleteLatency time.Duration
	cacheSize     int64
	entryCount    int64
	mu            sync.RWMutex
}

// cacheEntry represents a single cache entry with metadata
type cacheEntry struct {
	value       []byte // Serialized value
	createdAt   time.Time
	expiresAt   time.Time
	accessCount int64
	lastAccess  time.Time
	size        int64
	tags        []string
}

// MemoryCache exposes the Memory cache implementation
type MemoryCache struct {
	*memoryCache
}

// NewMemoryCache creates a new memory cache instance
func NewMemoryCache(options *interfaces.CacheOptions) (interfaces.Cache, error) {
	if options == nil {
		options = &interfaces.CacheOptions{}
	}

	// Determine serializer format
	format := serializer.Binary // Default to Gob for native Go performance in memory cache
	if options.SerializerFormat != "" {
		format = options.SerializerFormat
	}

	// Get serializer from default registry
	s, err := serializer.DefaultRegistry.New(format)
	if err != nil {
		return nil, err
	}

	// Initialize metrics systems
	legacyMetrics := &cacheMetrics{}

	// Determine which metrics system to use
	var enhancedMetrics interfaces.EnhancedCacheMetrics
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
			registry := metric.NewDefaultRegistry()
			enhancedMetrics = metrics.NewEnhancedCacheMetrics(registry, options.GlobalMetricsTags)
		}
	}

	c := &memoryCache{
		items:           make(map[string]*cacheEntry),
		options:         options,
		serializer:      s,
		legacyMetrics:   legacyMetrics,
		enhancedMetrics: enhancedMetrics,
		providerName:    "memory",
		cleanupStopChan: make(chan struct{}),

		// Initialize enhanced features
		indexes:        make(map[string]map[string][]string),
		indexPatterns:  make(map[string]string),
		hooks:          options.Hooks,
		securityConfig: options.Security,
	}

	// Initialize pre-configured indexes
	if options.Indexes != nil {
		for indexName, keyPattern := range options.Indexes {
			c.indexPatterns[indexName] = keyPattern
			c.indexes[indexName] = make(map[string][]string)
		}
	}

	// Start cleanup goroutine if cleanup interval is set
	if options.CleanupInterval > 0 {
		c.cleanupTicker = time.NewTicker(options.CleanupInterval)
		go c.cleanupLoop()
	}

	return &MemoryCache{c}, nil
}

// Get retrieves a value from the cache
func (c *memoryCache) Get(ctx context.Context, key string) (any, bool, error) {
	start := time.Now()
	defer func() {
		c.recordGetLatency(time.Since(start))
		// Apply timing protection if enabled
		if c.securityConfig != nil && c.securityConfig.EnableTimingProtection {
			elapsed := time.Since(start)
			if elapsed < c.securityConfig.MinProcessingTime {
				actualTime := elapsed
				adjustedTime := c.securityConfig.MinProcessingTime
				time.Sleep(adjustedTime - actualTime)

				// Record timing protection metrics
				tags := c.getBaseTags()
				c.enhancedMetrics.RecordTimingProtection(c.providerName, "get", actualTime, adjustedTime, tags)
			}
		}
	}()

	// Validate key
	if key == "" {
		return nil, false, cacheErrors.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, false, cacheErrors.ErrContextCanceled
	}

	// Call PreGet hook
	if c.hooks != nil && c.hooks.PreGet != nil {
		if err := c.hooks.PreGet(ctx, key); err != nil {
			return nil, false, err
		}
	}

	c.mu.RLock()
	entry, exists := c.items[key]
	c.mu.RUnlock()

	if !exists {
		c.recordMiss()
		// Call PostGet hook for miss
		if c.hooks != nil && c.hooks.PostGet != nil {
			c.hooks.PostGet(ctx, key, nil, false, nil)
		}
		return nil, false, nil
	}

	// Check if entry has expired
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		c.recordMiss()
		// Call PostGet hook for expired entry
		if c.hooks != nil && c.hooks.PostGet != nil {
			c.hooks.PostGet(ctx, key, nil, false, nil)
		}
		return nil, false, nil
	}

	// Deserialize value
	var value any

	// Try standard deserialization first
	if err := c.serializer.Deserialize(entry.value, &value); err != nil {
		// If standard deserialization fails, try with specific types
		// This helps with Binary format which may need concrete types

		// Try as string
		var strVal string
		if err := c.serializer.Deserialize(entry.value, &strVal); err == nil {
			value = strVal
		} else {
			// Try as int
			var intVal int
			if err := c.serializer.Deserialize(entry.value, &intVal); err == nil {
				value = intVal
			} else {
				// Try as float64
				var floatVal float64
				if err := c.serializer.Deserialize(entry.value, &floatVal); err == nil {
					value = floatVal
				} else {
					// Try as bool
					var boolVal bool
					if err := c.serializer.Deserialize(entry.value, &boolVal); err == nil {
						value = boolVal
					} else {
						// If all else fails, return error
						return nil, false, cacheErrors.ErrDeserialization
					}
				}
			}
		}
	}

	// Update access metadata
	c.mu.Lock()
	entry.accessCount++
	entry.lastAccess = time.Now()
	c.mu.Unlock()

	c.recordHit()
	// Call PostGet hook for successful get
	if c.hooks != nil && c.hooks.PostGet != nil {
		c.hooks.PostGet(ctx, key, value, true, nil)
	}
	return value, true, nil
}

// Set stores a value in the cache
func (c *memoryCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	start := time.Now()
	var err error
	defer func() {
		c.recordSetLatency(time.Since(start))
		// Apply timing protection if enabled
		if c.securityConfig != nil && c.securityConfig.EnableTimingProtection {
			elapsed := time.Since(start)
			if elapsed < c.securityConfig.MinProcessingTime {
				actualTime := elapsed
				adjustedTime := c.securityConfig.MinProcessingTime
				time.Sleep(adjustedTime - actualTime)

				// Record timing protection metrics
				tags := c.getBaseTags()
				c.enhancedMetrics.RecordTimingProtection(c.providerName, "get", actualTime, adjustedTime, tags)
			}
		}
		// Call PostSet hook
		if c.hooks != nil && c.hooks.PostSet != nil {
			c.hooks.PostSet(ctx, key, value, ttl, err)
		}
	}()

	// Validate key
	if key == "" {
		err = cacheErrors.ErrInvalidKey
		return err
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		err = cacheErrors.ErrContextCanceled
		return err
	}

	// Call PreSet hook
	if c.hooks != nil && c.hooks.PreSet != nil {
		if hookErr := c.hooks.PreSet(ctx, key, value, ttl); hookErr != nil {
			err = hookErr
			return err
		}
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
		return cacheErrors.ErrSerialization
	}

	// Create the cache entry
	now := time.Now()
	entry := &cacheEntry{
		value:     data,
		createdAt: now,
		size:      int64(len(data)),
		tags:      []string{},
	}

	if ttl > 0 {
		entry.expiresAt = now.Add(ttl)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if cache is full (either by entry count or size)
	if c.options.MaxEntries > 0 && len(c.items) >= c.options.MaxEntries {
		return cacheErrors.ErrCacheFull
	}

	// Check if adding this entry would exceed max size
	if c.options.MaxSize > 0 {
		currentSize := int64(0)
		for _, e := range c.items {
			currentSize += e.size
		}
		if currentSize+entry.size > c.options.MaxSize {
			return cacheErrors.ErrCacheFull
		}
	}

	c.items[key] = entry

	// Add to appropriate indexes
	c.addToIndexes(key)

	// Update metrics
	c.updateSizeMetrics()

	return nil
}

// Delete removes a value from the cache
func (c *memoryCache) Delete(ctx context.Context, key string) error {
	start := time.Now()
	defer func() {
		c.recordDeleteLatency(time.Since(start))
	}()

	// Validate key
	if key == "" {
		return cacheErrors.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return cacheErrors.ErrContextCanceled
	}

	c.mu.Lock()
	delete(c.items, key)
	c.removeFromAllIndexes(key)
	c.mu.Unlock()

	// Update metrics
	c.updateSizeMetrics()

	return nil
}

// Clear removes all values from the cache
func (c *memoryCache) Clear(ctx context.Context) error {
	// Check for context cancellation
	if ctx.Err() != nil {
		return cacheErrors.ErrContextCanceled
	}

	c.mu.Lock()
	c.items = make(map[string]*cacheEntry)
	// Clear all indexes
	for indexName := range c.indexes {
		c.indexes[indexName] = make(map[string][]string)
	}
	c.mu.Unlock()

	// Update metrics
	c.updateSizeMetrics()

	return nil
}

// Has checks if a key exists in the cache
func (c *memoryCache) Has(ctx context.Context, key string) bool {
	// Validate key
	if key == "" {
		return false
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return false
	}

	c.mu.RLock()
	entry, exists := c.items[key]
	c.mu.RUnlock()

	if !exists {
		return false
	}

	// Check if entry has expired
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()

		// Update metrics
		c.updateSizeMetrics()

		return false
	}

	return true
}

// GetKeys returns all keys in the cache
func (c *memoryCache) GetKeys(ctx context.Context) []string {
	// Check for context cancellation
	if ctx.Err() != nil {
		return []string{}
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.items))
	now := time.Now()

	for k, entry := range c.items {
		// Skip expired entries
		if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
			continue
		}
		keys = append(keys, k)
	}

	return keys
}

// Close stops the cleanup goroutine and releases resources
func (c *memoryCache) Close() error {
	if c.cleanupTicker != nil {
		c.cleanupTicker.Stop()
		close(c.cleanupStopChan)
	}
	return nil
}

// GetMany retrieves multiple values from the cache
func (c *memoryCache) GetMany(ctx context.Context, keys []string) (map[string]any, error) {
	// Validate keys
	if len(keys) == 0 {
		return map[string]any{}, nil
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, cacheErrors.ErrContextCanceled
	}

	start := time.Now()
	defer func() {
		duration := time.Since(start)
		c.recordGetLatency(duration)

		// Record batch operation metrics
		tags := c.getBaseTags()
		c.enhancedMetrics.RecordBatchOperation(c.providerName, "get", len(keys), duration, tags)
	}()

	result := make(map[string]any, len(keys))
	now := time.Now()

	c.mu.RLock()
	for _, key := range keys {
		entry, exists := c.items[key]

		if exists {
			// Skip expired entries
			if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
				c.recordMiss()
				continue
			}

			// Deserialize value
			var value any
			if err := c.serializer.Deserialize(entry.value, &value); err == nil {
				result[key] = value

				// Update entry metadata in a non-blocking way
				go func(k string) {
					c.mu.Lock()
					if e, ok := c.items[k]; ok {
						e.accessCount++
						e.lastAccess = time.Now()
					}
					c.mu.Unlock()
				}(key)

				c.recordHit()
			} else {
				c.recordMiss()
			}
		} else {
			c.recordMiss()
		}
	}
	c.mu.RUnlock()

	return result, nil
}

// SetMany stores multiple values in the cache
func (c *memoryCache) SetMany(ctx context.Context, items map[string]any, ttl time.Duration) error {
	// Validate items
	if len(items) == 0 {
		return nil
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return cacheErrors.ErrContextCanceled
	}

	start := time.Now()
	defer func() {
		duration := time.Since(start)
		c.recordSetLatency(duration)

		// Record batch operation metrics
		tags := c.getBaseTags()
		c.enhancedMetrics.RecordBatchOperation(c.providerName, "set", len(items), duration, tags)
	}()

	// Use default TTL if not specified
	if ttl == 0 {
		ttl = c.options.TTL
	}
	if ttl <= 0 {
		ttl = defaultKeyTTL
	}

	now := time.Now()
	newEntries := make(map[string]*cacheEntry, len(items))

	// First, serialize all values and check size constraints
	totalNewSize := int64(0)
	for key, value := range items {
		// Validate key
		if key == "" {
			return cacheErrors.ErrInvalidKey
		}

		// Serialize value
		data, err := c.serializer.Serialize(value)
		if err != nil {
			return cacheErrors.ErrSerialization
		}

		// Create entry
		entry := &cacheEntry{
			value:     data,
			createdAt: now,
			size:      int64(len(data)),
			tags:      []string{},
		}

		if ttl > 0 {
			entry.expiresAt = now.Add(ttl)
		}

		newEntries[key] = entry
		totalNewSize += entry.size
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if adding these entries would exceed limits
	if c.options.MaxEntries > 0 {
		availableSlots := c.options.MaxEntries - len(c.items)
		if availableSlots < len(newEntries) {
			return cacheErrors.ErrCacheFull
		}
	}

	// Check size constraints
	if c.options.MaxSize > 0 {
		currentSize := int64(0)
		for _, e := range c.items {
			currentSize += e.size
		}
		if currentSize+totalNewSize > c.options.MaxSize {
			return cacheErrors.ErrCacheFull
		}
	}

	// Add all entries
	for key, entry := range newEntries {
		c.items[key] = entry
	}

	// Update metrics
	c.updateSizeMetrics()

	return nil
}

// DeleteMany removes multiple values from the cache
func (c *memoryCache) DeleteMany(ctx context.Context, keys []string) error {
	// Validate keys
	if len(keys) == 0 {
		return nil
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return cacheErrors.ErrContextCanceled
	}

	start := time.Now()
	defer func() {
		duration := time.Since(start)
		c.recordDeleteLatency(duration)

		// Record batch operation metrics
		tags := c.getBaseTags()
		c.enhancedMetrics.RecordBatchOperation(c.providerName, "delete", len(keys), duration, tags)
	}()

	c.mu.Lock()
	for _, key := range keys {
		delete(c.items, key)
	}
	c.mu.Unlock()

	// Update metrics
	c.updateSizeMetrics()

	return nil
}

// GetMetadata retrieves metadata for a cache entry
func (c *memoryCache) GetMetadata(ctx context.Context, key string) (*interfaces.CacheEntryMetadata, error) {
	// Validate key
	if key == "" {
		return nil, cacheErrors.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, cacheErrors.ErrContextCanceled
	}

	// First check if the key exists
	if !c.Has(ctx, key) {
		return nil, cacheErrors.ErrKeyNotFound
	}

	c.mu.RLock()
	entry := c.items[key]
	c.mu.RUnlock()

	// Calculate TTL
	var ttl time.Duration
	if !entry.expiresAt.IsZero() {
		now := time.Now()
		if now.Before(entry.expiresAt) {
			ttl = entry.expiresAt.Sub(now)
		}
	}

	return &interfaces.CacheEntryMetadata{
		Key:          key,
		CreatedAt:    entry.createdAt,
		LastAccessed: entry.lastAccess,
		AccessCount:  entry.accessCount,
		TTL:          ttl,
		Size:         entry.size,
		Tags:         entry.tags,
	}, nil
}

// GetManyMetadata retrieves metadata for multiple cache entries
func (c *memoryCache) GetManyMetadata(ctx context.Context, keys []string) (map[string]*interfaces.CacheEntryMetadata, error) {
	// Validate keys
	if len(keys) == 0 {
		return map[string]*interfaces.CacheEntryMetadata{}, nil
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, cacheErrors.ErrContextCanceled
	}

	result := make(map[string]*interfaces.CacheEntryMetadata)
	now := time.Now()

	c.mu.RLock()
	for _, key := range keys {
		entry, exists := c.items[key]
		if exists {
			// Skip expired entries
			if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
				continue
			}

			// Calculate TTL
			var ttl time.Duration
			if !entry.expiresAt.IsZero() && now.Before(entry.expiresAt) {
				ttl = entry.expiresAt.Sub(now)
			}

			result[key] = &interfaces.CacheEntryMetadata{
				Key:          key,
				CreatedAt:    entry.createdAt,
				LastAccessed: entry.lastAccess,
				AccessCount:  entry.accessCount,
				TTL:          ttl,
				Size:         entry.size,
				Tags:         entry.tags,
			}
		}
	}
	c.mu.RUnlock()

	return result, nil
}

// cleanupLoop runs the cleanup process at regular intervals
func (c *memoryCache) cleanupLoop() {
	for {
		select {
		case <-c.cleanupTicker.C:
			c.cleanup()
		case <-c.cleanupStopChan:
			return
		}
	}
}

// cleanup removes expired entries from the cache
func (c *memoryCache) cleanup() {
	start := time.Now()
	now := time.Now()

	c.mu.Lock()
	expiredKeys := make([]string, 0)
	for key, entry := range c.items {
		if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	// Remove expired entries and their index references
	for _, key := range expiredKeys {
		delete(c.items, key)
		c.removeFromAllIndexes(key)
	}
	c.mu.Unlock()

	duration := time.Since(start)
	itemCount := len(expiredKeys)

	// Record cleanup metrics
	tags := c.getBaseTags()
	c.enhancedMetrics.RecordCleanup(c.providerName, "expired", itemCount, duration, tags)

	// Update size metrics
	c.updateSizeMetrics()
}

// updateSizeMetrics updates the size and entry count metrics
func (c *memoryCache) updateSizeMetrics() {
	var totalSize int64
	entryCount := int64(len(c.items))

	for _, entry := range c.items {
		totalSize += entry.size
	}

	// Update legacy metrics
	c.legacyMetrics.mu.Lock()
	c.legacyMetrics.cacheSize = totalSize
	c.legacyMetrics.entryCount = entryCount
	c.legacyMetrics.mu.Unlock()

	// Update enhanced metrics
	tags := c.getBaseTags()
	c.enhancedMetrics.RecordCacheSize(c.providerName, totalSize, tags)
	c.enhancedMetrics.RecordEntryCount(c.providerName, entryCount, tags)
}

// New metrics recording functions that use both legacy and enhanced metrics
func (c *memoryCache) recordHit() {
	// Legacy metrics
	c.legacyMetrics.mu.Lock()
	c.legacyMetrics.hits++
	c.legacyMetrics.mu.Unlock()

	// Enhanced metrics
	tags := c.getBaseTags()
	c.enhancedMetrics.RecordHit(c.providerName, tags)
}

func (c *memoryCache) recordMiss() {
	// Legacy metrics
	c.legacyMetrics.mu.Lock()
	c.legacyMetrics.misses++
	c.legacyMetrics.mu.Unlock()

	// Enhanced metrics
	tags := c.getBaseTags()
	c.enhancedMetrics.RecordMiss(c.providerName, tags)
}

func (c *memoryCache) recordGetLatency(duration time.Duration) {
	// Legacy metrics
	c.legacyMetrics.mu.Lock()
	c.legacyMetrics.getLatency = duration
	c.legacyMetrics.mu.Unlock()

	// Enhanced metrics
	tags := c.getBaseTags()
	c.enhancedMetrics.RecordOperation(c.providerName, "get", "completed", duration, tags)
}

func (c *memoryCache) recordSetLatency(duration time.Duration) {
	// Legacy metrics
	c.legacyMetrics.mu.Lock()
	c.legacyMetrics.setLatency = duration
	c.legacyMetrics.mu.Unlock()

	// Enhanced metrics
	tags := c.getBaseTags()
	c.enhancedMetrics.RecordOperation(c.providerName, "set", "completed", duration, tags)
}

func (c *memoryCache) recordDeleteLatency(duration time.Duration) {
	// Legacy metrics
	c.legacyMetrics.mu.Lock()
	c.legacyMetrics.deleteLatency = duration
	c.legacyMetrics.mu.Unlock()

	// Enhanced metrics
	tags := c.getBaseTags()
	c.enhancedMetrics.RecordOperation(c.providerName, "delete", "completed", duration, tags)
}

// Helper function to get base tags for metrics
func (c *memoryCache) getBaseTags() metric.Tags {
	tags := make(metric.Tags)
	if c.options.GlobalMetricsTags != nil {
		for k, v := range c.options.GlobalMetricsTags {
			tags[k] = v
		}
	}
	return tags
}

// Increment atomically increments a numeric value in the cache
func (c *memoryCache) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	// Validate key
	if key == "" {
		return 0, cacheErrors.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return 0, cacheErrors.ErrContextCanceled
	}

	// Use default TTL if not specified
	if ttl == 0 {
		ttl = c.options.TTL
	}
	if ttl <= 0 {
		ttl = defaultKeyTTL
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	entry, exists := c.items[key]

	var currentValue int64 = 0

	if exists {
		// Check if entry has expired
		if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
			// Entry expired, treat as if it doesn't exist
			delete(c.items, key)
			exists = false
		} else {
			// Deserialize current value
			var value any
			if err := c.serializer.Deserialize(entry.value, &value); err != nil {
				// If can't deserialize as generic, try as int64
				var intVal int64
				if err := c.serializer.Deserialize(entry.value, &intVal); err != nil {
					return 0, cacheErrors.ErrDeserialization
				}
				currentValue = intVal
			} else {
				// Convert value to int64
				switch v := value.(type) {
				case int64:
					currentValue = v
				case int:
					currentValue = int64(v)
				case int32:
					currentValue = int64(v)
				case float64:
					currentValue = int64(v)
				case float32:
					currentValue = int64(v)
				default:
					return 0, cacheErrors.ErrInvalidValue
				}
			}
		}
	}

	// Calculate new value
	newValue := currentValue + delta

	// Serialize new value
	data, err := c.serializer.Serialize(newValue)
	if err != nil {
		return 0, cacheErrors.ErrSerialization
	}

	// Create or update entry
	newEntry := &cacheEntry{
		value:     data,
		createdAt: now,
		size:      int64(len(data)),
		tags:      []string{},
	}

	if exists && entry != nil {
		// Preserve original creation time and access data
		newEntry.createdAt = entry.createdAt
		newEntry.accessCount = entry.accessCount
		newEntry.lastAccess = entry.lastAccess
	}

	if ttl > 0 {
		newEntry.expiresAt = now.Add(ttl)
	}

	c.items[key] = newEntry

	// Update metrics
	c.updateSizeMetrics()

	return newValue, nil
}

// Decrement atomically decrements a numeric value in the cache
func (c *memoryCache) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return c.Increment(ctx, key, -delta, ttl)
}

// SetIfNotExists sets a value only if the key doesn't exist
func (c *memoryCache) SetIfNotExists(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	// Validate key
	if key == "" {
		return false, cacheErrors.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return false, cacheErrors.ErrContextCanceled
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	entry, exists := c.items[key]

	if exists {
		// Check if entry has expired
		if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
			// Entry expired, remove it and proceed with set
			delete(c.items, key)
		} else {
			// Key exists and is not expired
			return false, nil
		}
	}

	// Key doesn't exist, set it
	return true, c.setLocked(key, value, ttl, now)
}

// SetIfExists sets a value only if the key already exists
func (c *memoryCache) SetIfExists(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	// Validate key
	if key == "" {
		return false, cacheErrors.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return false, cacheErrors.ErrContextCanceled
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	entry, exists := c.items[key]

	if !exists {
		return false, nil
	}

	// Check if entry has expired
	if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
		// Entry expired, remove it
		delete(c.items, key)
		return false, nil
	}

	// Key exists and is not expired, update it
	return true, c.setLocked(key, value, ttl, now)
}

// setLocked is a helper method that sets a value while holding the lock
func (c *memoryCache) setLocked(key string, value any, ttl time.Duration, now time.Time) error {
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
		return cacheErrors.ErrSerialization
	}

	// Create the cache entry
	entry := &cacheEntry{
		value:     data,
		createdAt: now,
		size:      int64(len(data)),
		tags:      []string{},
	}

	if ttl > 0 {
		entry.expiresAt = now.Add(ttl)
	}

	// Check cache limits
	if c.options.MaxEntries > 0 && len(c.items) >= c.options.MaxEntries {
		return cacheErrors.ErrCacheFull
	}

	if c.options.MaxSize > 0 {
		currentSize := int64(0)
		for _, e := range c.items {
			currentSize += e.size
		}
		if currentSize+entry.size > c.options.MaxSize {
			return cacheErrors.ErrCacheFull
		}
	}

	c.items[key] = entry

	// Update metrics
	c.updateSizeMetrics()

	return nil
}

// AddIndex adds a secondary index mapping
func (c *memoryCache) AddIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error {
	// Check for context cancellation
	if ctx.Err() != nil {
		return cacheErrors.ErrContextCanceled
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Register the index pattern if not already registered
	if _, exists := c.indexPatterns[indexName]; !exists {
		c.indexPatterns[indexName] = keyPattern
		c.indexes[indexName] = make(map[string][]string)
	}

	// Find all existing keys that match the pattern and should be indexed under this indexKey
	for key := range c.items {
		if c.matchesPattern(key, keyPattern) {
			// Add to index
			if c.indexes[indexName][indexKey] == nil {
				c.indexes[indexName][indexKey] = make([]string, 0)
			}

			// Check if key is already in the index
			found := false
			for _, existingKey := range c.indexes[indexName][indexKey] {
				if existingKey == key {
					found = true
					break
				}
			}

			if !found {
				c.indexes[indexName][indexKey] = append(c.indexes[indexName][indexKey], key)
			}
		}
	}

	return nil
}

// RemoveIndex removes a secondary index mapping
func (c *memoryCache) RemoveIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error {
	// Check for context cancellation
	if ctx.Err() != nil {
		return cacheErrors.ErrContextCanceled
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if indexMap, exists := c.indexes[indexName]; exists {
		if keys, exists := indexMap[indexKey]; exists {
			// Remove all keys that match the pattern
			newKeys := make([]string, 0)
			for _, key := range keys {
				if !c.matchesPattern(key, keyPattern) {
					newKeys = append(newKeys, key)
				}
			}

			if len(newKeys) == 0 {
				delete(indexMap, indexKey)
			} else {
				indexMap[indexKey] = newKeys
			}
		}
	}

	return nil
}

// GetByIndex retrieves all keys associated with an index key
func (c *memoryCache) GetByIndex(ctx context.Context, indexName string, indexKey string) ([]string, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		// Record index operation metrics
		tags := c.getBaseTags()
		c.enhancedMetrics.RecordIndexOperation(c.providerName, "get", indexName, duration, tags)
	}()

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, cacheErrors.ErrContextCanceled
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if indexMap, exists := c.indexes[indexName]; exists {
		if keys, exists := indexMap[indexKey]; exists {
			// Filter out expired keys
			validKeys := make([]string, 0, len(keys))
			now := time.Now()

			for _, key := range keys {
				if entry, exists := c.items[key]; exists {
					if entry.expiresAt.IsZero() || now.Before(entry.expiresAt) {
						validKeys = append(validKeys, key)
					}
				}
			}

			return validKeys, nil
		}
	}

	return []string{}, nil
}

// DeleteByIndex deletes all entries associated with an index key
func (c *memoryCache) DeleteByIndex(ctx context.Context, indexName string, indexKey string) error {
	// Check for context cancellation
	if ctx.Err() != nil {
		return cacheErrors.ErrContextCanceled
	}

	// Get the keys first
	keys, err := c.GetByIndex(ctx, indexName, indexKey)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Delete each key and remove from all indexes
	for _, key := range keys {
		delete(c.items, key)
		c.removeFromAllIndexes(key)
	}

	// Update metrics
	c.updateSizeMetrics()

	return nil
}

// GetKeysByPattern retrieves all keys matching a pattern
func (c *memoryCache) GetKeysByPattern(ctx context.Context, pattern string) ([]string, error) {
	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, cacheErrors.ErrContextCanceled
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	matchingKeys := make([]string, 0)
	now := time.Now()

	for key, entry := range c.items {
		// Skip expired entries
		if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
			continue
		}

		if c.matchesPattern(key, pattern) {
			matchingKeys = append(matchingKeys, key)
		}
	}

	return matchingKeys, nil
}

// DeleteByPattern deletes all entries matching a pattern
func (c *memoryCache) DeleteByPattern(ctx context.Context, pattern string) (int, error) {
	// Check for context cancellation
	if ctx.Err() != nil {
		return 0, cacheErrors.ErrContextCanceled
	}

	// Get matching keys first
	keys, err := c.GetKeysByPattern(ctx, pattern)
	if err != nil {
		return 0, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Delete each key and remove from indexes
	deletedCount := 0
	for _, key := range keys {
		if _, exists := c.items[key]; exists {
			delete(c.items, key)
			c.removeFromAllIndexes(key)
			deletedCount++
		}
	}

	// Update metrics
	c.updateSizeMetrics()

	return deletedCount, nil
}

// UpdateMetadata updates the metadata of a cache entry
func (c *memoryCache) UpdateMetadata(ctx context.Context, key string, updater interfaces.MetadataUpdater) error {
	// Validate key
	if key == "" {
		return cacheErrors.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return cacheErrors.ErrContextCanceled
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.items[key]
	if !exists {
		return cacheErrors.ErrKeyNotFound
	}

	// Check if entry has expired
	now := time.Now()
	if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
		delete(c.items, key)
		c.removeFromAllIndexes(key)
		return cacheErrors.ErrKeyNotFound
	}

	// Create current metadata
	var ttl time.Duration
	if !entry.expiresAt.IsZero() && now.Before(entry.expiresAt) {
		ttl = entry.expiresAt.Sub(now)
	}

	currentMetadata := &interfaces.CacheEntryMetadata{
		Key:          key,
		CreatedAt:    entry.createdAt,
		LastAccessed: entry.lastAccess,
		AccessCount:  entry.accessCount,
		TTL:          ttl,
		Size:         entry.size,
		Tags:         entry.tags,
	}

	// Apply the updater
	newMetadata := updater(currentMetadata)
	if newMetadata == nil {
		return nil // No update needed
	}

	// Update the entry
	entry.lastAccess = newMetadata.LastAccessed
	entry.accessCount = newMetadata.AccessCount
	entry.tags = newMetadata.Tags

	// Update TTL if changed
	if newMetadata.TTL != ttl {
		if newMetadata.TTL > 0 {
			entry.expiresAt = now.Add(newMetadata.TTL)
		} else {
			entry.expiresAt = time.Time{}
		}
	}

	return nil
}

// GetAndUpdate atomically gets and updates a cache entry
func (c *memoryCache) GetAndUpdate(ctx context.Context, key string, updater interfaces.ValueUpdater, ttl time.Duration) (any, error) {
	// Validate key
	if key == "" {
		return nil, cacheErrors.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, cacheErrors.ErrContextCanceled
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.items[key]
	var currentValue any

	if exists {
		// Check if entry has expired
		now := time.Now()
		if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
			delete(c.items, key)
			c.removeFromAllIndexes(key)
			exists = false
		} else {
			// Deserialize current value
			if err := c.serializer.Deserialize(entry.value, &currentValue); err != nil {
				return nil, cacheErrors.ErrDeserialization
			}
		}
	}

	// Apply the updater
	newValue, shouldUpdate := updater(currentValue)

	if shouldUpdate {
		// Update the entry
		err := c.setLocked(key, newValue, ttl, time.Now())
		if err != nil {
			return nil, err
		}
		return newValue, nil
	}

	return currentValue, nil
}

// Helper methods

// matchesPattern checks if a key matches a glob pattern
func (c *memoryCache) matchesPattern(key, pattern string) bool {
	matched, err := filepath.Match(pattern, key)
	if err != nil {
		// If filepath.Match fails, fall back to simple string matching
		return strings.Contains(key, strings.ReplaceAll(pattern, "*", ""))
	}
	return matched
}

// removeFromAllIndexes removes a key from all indexes
func (c *memoryCache) removeFromAllIndexes(key string) {
	for indexName, indexMap := range c.indexes {
		pattern := c.indexPatterns[indexName]
		if c.matchesPattern(key, pattern) {
			for indexKey, keys := range indexMap {
				newKeys := make([]string, 0, len(keys))
				for _, k := range keys {
					if k != key {
						newKeys = append(newKeys, k)
					}
				}
				if len(newKeys) == 0 {
					delete(indexMap, indexKey)
				} else {
					indexMap[indexKey] = newKeys
				}
			}
		}
	}
}

// addToIndexes adds a key to appropriate indexes when setting entries
func (c *memoryCache) addToIndexes(key string) {
	// This method is called when a key is added to the cache
	// The actual indexing happens when AddIndex is called explicitly
	// This is a placeholder for automatic indexing logic that would
	// need to be implemented based on specific requirements
}

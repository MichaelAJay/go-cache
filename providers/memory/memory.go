package memory

import (
	"context"
	"sync"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-serializer"
)

const (
	// Default values
	defaultKeyTTL = time.Hour * 24 * 7 // 1 week
)

// memoryCache implements the Cache interface using an in-memory map
type memoryCache struct {
	items           map[string]*cacheEntry
	mu              sync.RWMutex
	options         *cache.CacheOptions
	serializer      serializer.Serializer
	metrics         *cacheMetrics
	cleanupTicker   *time.Ticker
	cleanupStopChan chan struct{}
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
func NewMemoryCache(options *cache.CacheOptions) (cache.Cache, error) {
	if options == nil {
		options = &cache.CacheOptions{}
	}

	// Determine serializer format
	format := serializer.Msgpack // Default to MessagePack for better performance
	if options.SerializerFormat != "" {
		format = options.SerializerFormat
	}

	// Get serializer from default registry
	s, err := serializer.DefaultRegistry.New(format)
	if err != nil {
		return nil, err
	}

	c := &memoryCache{
		items:           make(map[string]*cacheEntry),
		options:         options,
		serializer:      s,
		metrics:         &cacheMetrics{},
		cleanupStopChan: make(chan struct{}),
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

	c.mu.RLock()
	entry, exists := c.items[key]
	c.mu.RUnlock()

	if !exists {
		c.metrics.recordMiss()
		return nil, false, nil
	}

	// Check if entry has expired
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		c.metrics.recordMiss()
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
						return nil, false, cache.ErrDeserialization
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

	c.metrics.recordHit()
	return value, true, nil
}

// Set stores a value in the cache
func (c *memoryCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
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
		return cache.ErrCacheFull
	}

	// Check if adding this entry would exceed max size
	if c.options.MaxSize > 0 {
		currentSize := int64(0)
		for _, e := range c.items {
			currentSize += e.size
		}
		if currentSize+entry.size > c.options.MaxSize {
			return cache.ErrCacheFull
		}
	}

	c.items[key] = entry

	// Update metrics
	c.updateSizeMetrics()

	return nil
}

// Delete removes a value from the cache
func (c *memoryCache) Delete(ctx context.Context, key string) error {
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

	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()

	// Update metrics
	c.updateSizeMetrics()

	return nil
}

// Clear removes all values from the cache
func (c *memoryCache) Clear(ctx context.Context) error {
	// Check for context cancellation
	if ctx.Err() != nil {
		return cache.ErrContextCanceled
	}

	c.mu.Lock()
	c.items = make(map[string]*cacheEntry)
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
		return nil, cache.ErrContextCanceled
	}

	start := time.Now()
	defer func() {
		c.metrics.recordGetLatency(time.Since(start))
	}()

	result := make(map[string]any, len(keys))
	now := time.Now()

	c.mu.RLock()
	for _, key := range keys {
		entry, exists := c.items[key]

		if exists {
			// Skip expired entries
			if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
				c.metrics.recordMiss()
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

				c.metrics.recordHit()
			} else {
				c.metrics.recordMiss()
			}
		} else {
			c.metrics.recordMiss()
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
		return cache.ErrContextCanceled
	}

	start := time.Now()
	defer func() {
		c.metrics.recordSetLatency(time.Since(start))
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
			return cache.ErrInvalidKey
		}

		// Serialize value
		data, err := c.serializer.Serialize(value)
		if err != nil {
			return cache.ErrSerialization
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
			return cache.ErrCacheFull
		}
	}

	// Check size constraints
	if c.options.MaxSize > 0 {
		currentSize := int64(0)
		for _, e := range c.items {
			currentSize += e.size
		}
		if currentSize+totalNewSize > c.options.MaxSize {
			return cache.ErrCacheFull
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
		return cache.ErrContextCanceled
	}

	start := time.Now()
	defer func() {
		c.metrics.recordDeleteLatency(time.Since(start))
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
func (c *memoryCache) GetMetadata(ctx context.Context, key string) (*cache.CacheEntryMetadata, error) {
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

	return &cache.CacheEntryMetadata{
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
func (c *memoryCache) GetManyMetadata(ctx context.Context, keys []string) (map[string]*cache.CacheEntryMetadata, error) {
	// Validate keys
	if len(keys) == 0 {
		return map[string]*cache.CacheEntryMetadata{}, nil
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, cache.ErrContextCanceled
	}

	result := make(map[string]*cache.CacheEntryMetadata)
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

			result[key] = &cache.CacheEntryMetadata{
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

// GetMetrics returns a snapshot of the metrics
func (c *memoryCache) GetMetrics() *cache.CacheMetricsSnapshot {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()

	hitRate := float64(0)
	totalOps := c.metrics.hits + c.metrics.misses
	if totalOps > 0 {
		hitRate = float64(c.metrics.hits) / float64(totalOps)
	}

	return &cache.CacheMetricsSnapshot{
		Hits:          c.metrics.hits,
		Misses:        c.metrics.misses,
		HitRatio:      hitRate,
		GetLatency:    c.metrics.getLatency,
		SetLatency:    c.metrics.setLatency,
		DeleteLatency: c.metrics.deleteLatency,
		CacheSize:     c.metrics.cacheSize,
		EntryCount:    c.metrics.entryCount,
	}
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
	now := time.Now()

	c.mu.Lock()
	for key, entry := range c.items {
		if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
			delete(c.items, key)
		}
	}
	c.mu.Unlock()

	// Update metrics
	c.updateSizeMetrics()
}

// updateSizeMetrics updates the size and entry count metrics
func (c *memoryCache) updateSizeMetrics() {
	var totalSize int64
	entryCount := int64(len(c.items))

	for _, entry := range c.items {
		totalSize += entry.size
	}

	c.metrics.mu.Lock()
	c.metrics.cacheSize = totalSize
	c.metrics.entryCount = entryCount
	c.metrics.mu.Unlock()
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

// Increment atomically increments a numeric value in the cache
func (c *memoryCache) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	// Validate key
	if key == "" {
		return 0, cache.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return 0, cache.ErrContextCanceled
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
					return 0, cache.ErrDeserialization
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
					return 0, cache.ErrInvalidValue
				}
			}
		}
	}

	// Calculate new value
	newValue := currentValue + delta

	// Serialize new value
	data, err := c.serializer.Serialize(newValue)
	if err != nil {
		return 0, cache.ErrSerialization
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
		return false, cache.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return false, cache.ErrContextCanceled
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
		return false, cache.ErrInvalidKey
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return false, cache.ErrContextCanceled
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
		return cache.ErrSerialization
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
		return cache.ErrCacheFull
	}

	if c.options.MaxSize > 0 {
		currentSize := int64(0)
		for _, e := range c.items {
			currentSize += e.size
		}
		if currentSize+entry.size > c.options.MaxSize {
			return cache.ErrCacheFull
		}
	}

	c.items[key] = entry

	// Update metrics
	c.updateSizeMetrics()

	return nil
}

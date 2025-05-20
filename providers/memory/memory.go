package memory

import (
	"context"
	"sync"
	"time"

	"github.com/MichaelAJay/go-cache"
)

// memoryCache implements the Cache interface using an in-memory map
type memoryCache struct {
	items           map[string]*cacheEntry
	mu              sync.RWMutex
	options         *cache.CacheOptions
	metrics         cache.CacheMetrics
	cleanupTicker   *time.Ticker
	cleanupStopChan chan struct{}
}

// cacheEntry represents a single cache entry with metadata
type cacheEntry struct {
	value       any
	createdAt   time.Time
	expiresAt   time.Time
	accessCount int64
	lastAccess  time.Time
	size        int64
	tags        []string
}

// NewMemoryCache creates a new memory cache instance
func NewMemoryCache(options *cache.CacheOptions) cache.Cache {
	var metrics cache.CacheMetrics
	if options != nil && options.Metrics != nil {
		metrics = options.Metrics
	} else {
		metrics = cache.NewMetrics()
	}

	c := &memoryCache{
		items:           make(map[string]*cacheEntry),
		options:         options,
		metrics:         metrics,
		cleanupStopChan: make(chan struct{}),
	}

	// Start cleanup goroutine if cleanup interval is set
	if options.CleanupInterval > 0 {
		c.cleanupTicker = time.NewTicker(options.CleanupInterval)
		go c.cleanupLoop()
	}

	return c
}

// Get retrieves a value from the cache
func (c *memoryCache) Get(ctx context.Context, key string) (any, bool, error) {
	start := time.Now()
	defer func() {
		c.metrics.RecordGetLatency(time.Since(start))
	}()

	c.mu.RLock()
	entry, exists := c.items[key]
	c.mu.RUnlock()

	if !exists {
		c.metrics.RecordMiss()
		return nil, false, nil
	}

	// Check if entry has expired
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		c.metrics.RecordMiss()
		return nil, false, nil
	}

	// Update access metadata
	c.mu.Lock()
	entry.accessCount++
	entry.lastAccess = time.Now()
	c.mu.Unlock()

	c.metrics.RecordHit()
	return entry.value, true, nil
}

// Set stores a value in the cache
func (c *memoryCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		c.metrics.RecordSetLatency(time.Since(start))
	}()

	// Use default TTL if not specified
	if ttl == 0 {
		ttl = c.options.TTL
	}

	// Calculate size of new entry
	size := calculateSize(value)

	entry := &cacheEntry{
		value:      value,
		createdAt:  time.Now(),
		lastAccess: time.Now(),
		size:       size,
	}

	if ttl > 0 {
		entry.expiresAt = entry.createdAt.Add(ttl)
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
		if currentSize+size > c.options.MaxSize {
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
		c.metrics.RecordDeleteLatency(time.Since(start))
	}()

	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()

	// Update metrics
	c.updateSizeMetrics()

	return nil
}

// Clear removes all values from the cache
func (c *memoryCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	c.items = make(map[string]*cacheEntry)
	c.mu.Unlock()

	// Update metrics
	c.updateSizeMetrics()

	return nil
}

// Has checks if a key exists in the cache
func (c *memoryCache) Has(ctx context.Context, key string) bool {
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
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.items))
	for k := range c.items {
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
	start := time.Now()
	defer func() {
		c.metrics.RecordGetLatency(time.Since(start))
	}()

	result := make(map[string]any, len(keys))
	for _, key := range keys {
		value, found, _ := c.Get(ctx, key)
		if found {
			result[key] = value
		}
	}
	return result, nil
}

// SetMany stores multiple values in the cache
func (c *memoryCache) SetMany(ctx context.Context, items map[string]any, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		c.metrics.RecordSetLatency(time.Since(start))
	}()

	for key, value := range items {
		if err := c.Set(ctx, key, value, ttl); err != nil {
			return err
		}
	}
	return nil
}

// DeleteMany removes multiple values from the cache
func (c *memoryCache) DeleteMany(ctx context.Context, keys []string) error {
	start := time.Now()
	defer func() {
		c.metrics.RecordDeleteLatency(time.Since(start))
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
	c.mu.RLock()
	entry, exists := c.items[key]
	c.mu.RUnlock()

	if !exists {
		return nil, cache.ErrKeyNotFound
	}

	return &cache.CacheEntryMetadata{
		Key:          key,
		CreatedAt:    entry.createdAt,
		LastAccessed: entry.lastAccess,
		AccessCount:  entry.accessCount,
		TTL:          c.getTTL(entry),
		Size:         entry.size,
		Tags:         entry.tags,
	}, nil
}

// GetManyMetadata retrieves metadata for multiple cache entries
func (c *memoryCache) GetManyMetadata(ctx context.Context, keys []string) (map[string]*cache.CacheEntryMetadata, error) {
	result := make(map[string]*cache.CacheEntryMetadata, len(keys))
	for _, key := range keys {
		metadata, err := c.GetMetadata(ctx, key)
		if err == nil {
			result[key] = metadata
		}
	}
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

	c.metrics.RecordCacheSize(totalSize)
	c.metrics.RecordEntryCount(entryCount)
}

// getTTL calculates the TTL for an entry
func (c *memoryCache) getTTL(entry *cacheEntry) time.Duration {
	if entry.expiresAt.IsZero() {
		return 0
	}

	now := time.Now()
	if now.After(entry.expiresAt) {
		return 0
	}

	return entry.expiresAt.Sub(now)
}

// GetMetrics returns the current metrics snapshot
func (c *memoryCache) GetMetrics() *cache.CacheMetricsSnapshot {
	return c.metrics.GetMetrics()
}

// calculateSize estimates the size of a value in bytes
func calculateSize(value any) int64 {
	// Simple size estimation - in a real implementation you would use
	// more accurate size calculation or serialization
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	case int, int32, float32, bool:
		return 4
	case int64, float64:
		return 8
	default:
		// Default estimate for other types
		return 64
	}
}

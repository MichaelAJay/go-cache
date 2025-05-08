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
	metrics         *cacheMetrics
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

// cacheMetrics implements the CacheMetrics interface
type cacheMetrics struct {
	hits          int64
	misses        int64
	getLatency    time.Duration
	setLatency    time.Duration
	deleteLatency time.Duration
	mu            sync.RWMutex
}

// NewMemoryCache creates a new memory cache instance
func NewMemoryCache(options *cache.CacheOptions) cache.Cache {
	c := &memoryCache{
		items:           make(map[string]*cacheEntry),
		options:         options,
		metrics:         &cacheMetrics{},
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
		c.metrics.recordGetLatency(time.Since(start))
	}()

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

	// Update access metadata
	c.mu.Lock()
	entry.accessCount++
	entry.lastAccess = time.Now()
	c.mu.Unlock()

	c.metrics.recordHit()
	return entry.value, true, nil
}

// Set stores a value in the cache
func (c *memoryCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		c.metrics.recordSetLatency(time.Since(start))
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
	return nil
}

// Delete removes a value from the cache
func (c *memoryCache) Delete(ctx context.Context, key string) error {
	start := time.Now()
	defer func() {
		c.metrics.recordDeleteLatency(time.Since(start))
	}()

	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
	return nil
}

// Clear removes all values from the cache
func (c *memoryCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	c.items = make(map[string]*cacheEntry)
	c.mu.Unlock()
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
	result := make(map[string]any)
	for _, key := range keys {
		if value, exists, err := c.Get(ctx, key); err == nil && exists {
			result[key] = value
		}
	}
	return result, nil
}

// SetMany stores multiple values in the cache
func (c *memoryCache) SetMany(ctx context.Context, items map[string]any, ttl time.Duration) error {
	for key, value := range items {
		if err := c.Set(ctx, key, value, ttl); err != nil {
			return err
		}
	}
	return nil
}

// DeleteMany removes multiple values from the cache
func (c *memoryCache) DeleteMany(ctx context.Context, keys []string) error {
	for _, key := range keys {
		if err := c.Delete(ctx, key); err != nil {
			return err
		}
	}
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
		TTL:          entry.expiresAt.Sub(entry.createdAt),
		Size:         entry.size,
		Tags:         entry.tags,
	}, nil
}

// GetManyMetadata retrieves metadata for multiple cache entries
func (c *memoryCache) GetManyMetadata(ctx context.Context, keys []string) (map[string]*cache.CacheEntryMetadata, error) {
	result := make(map[string]*cache.CacheEntryMetadata)
	for _, key := range keys {
		if metadata, err := c.GetMetadata(ctx, key); err == nil {
			result[key] = metadata
		}
	}
	return result, nil
}

// cleanupLoop periodically removes expired entries
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
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.items {
		if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
			delete(c.items, key)
		}
	}
}

// recordHit records a cache hit
func (m *cacheMetrics) recordHit() {
	m.mu.Lock()
	m.hits++
	m.mu.Unlock()
}

// recordMiss records a cache miss
func (m *cacheMetrics) recordMiss() {
	m.mu.Lock()
	m.misses++
	m.mu.Unlock()
}

// recordGetLatency records the latency of a Get operation
func (m *cacheMetrics) recordGetLatency(duration time.Duration) {
	m.mu.Lock()
	m.getLatency = duration
	m.mu.Unlock()
}

// recordSetLatency records the latency of a Set operation
func (m *cacheMetrics) recordSetLatency(duration time.Duration) {
	m.mu.Lock()
	m.setLatency = duration
	m.mu.Unlock()
}

// recordDeleteLatency records the latency of a Delete operation
func (m *cacheMetrics) recordDeleteLatency(duration time.Duration) {
	m.mu.Lock()
	m.deleteLatency = duration
	m.mu.Unlock()
}

// GetMetrics returns the current metrics snapshot
func (c *memoryCache) GetMetrics() *cache.CacheMetricsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()

	total := c.metrics.hits + c.metrics.misses
	hitRatio := 0.0
	if total > 0 {
		hitRatio = float64(c.metrics.hits) / float64(total)
	}

	var totalSize int64
	for _, entry := range c.items {
		totalSize += entry.size
	}

	return &cache.CacheMetricsSnapshot{
		Hits:          c.metrics.hits,
		Misses:        c.metrics.misses,
		HitRatio:      hitRatio,
		GetLatency:    c.metrics.getLatency,
		SetLatency:    c.metrics.setLatency,
		DeleteLatency: c.metrics.deleteLatency,
		CacheSize:     totalSize,
		EntryCount:    int64(len(c.items)),
	}
}

// calculateSize estimates the size of a value in bytes
func calculateSize(value any) int64 {
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	case int:
		return 8
	case int64:
		return 8
	case float64:
		return 8
	case bool:
		return 1
	case nil:
		return 0
	default:
		// For complex types, use a rough estimate
		return 64
	}
}

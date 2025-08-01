package null

import (
	"context"
	"sync"
	"time"

	"github.com/MichaelAJay/go-metrics/metric"
	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/MichaelAJay/go-cache/metrics"
)

// nullCache implements the Cache interface but doesn't store anything
// Useful for testing or when you want to disable caching
type nullCache struct {
	enhancedMetrics interfaces.EnhancedCacheMetrics
	providerName    string
	legacyMetrics   *cacheMetrics
}

// cacheMetrics implements a thread-safe metrics collector for the null cache
type cacheMetrics struct {
	hits          int64
	misses        int64
	getLatency    time.Duration
	setLatency    time.Duration
	deleteLatency time.Duration
	mu            sync.RWMutex
}

// NullCache exposes the null cache implementation
type NullCache struct {
	*nullCache
}

// NewNullCache creates a new null cache instance
func NewNullCache(options *interfaces.CacheOptions) interfaces.Cache {
	// Initialize metrics systems
	if options == nil {
		options = &interfaces.CacheOptions{}
	}
	
	legacyMetrics := &cacheMetrics{}
	
	// Determine which metrics system to use
	var enhancedMetrics interfaces.EnhancedCacheMetrics
	if options.EnhancedMetrics != nil {
		enhancedMetrics = options.EnhancedMetrics
	} else if options.GoMetricsRegistry != nil {
		enhancedMetrics = metrics.NewEnhancedCacheMetrics(options.GoMetricsRegistry, options.GlobalMetricsTags)
	} else {
		// Use no-op metrics
		enhancedMetrics = metrics.NewNoopEnhancedCacheMetrics()
	}

	return &NullCache{&nullCache{
		enhancedMetrics: enhancedMetrics,
		providerName:    "null",
		legacyMetrics:   legacyMetrics,
	}}
}


// Get always returns not found
func (c *nullCache) Get(ctx context.Context, key string) (any, bool, error) {
	start := time.Now()
	defer func() {
		c.recordGetLatency(time.Since(start))
	}()

	c.recordMiss()
	return nil, false, nil
}

// Set does nothing but records metrics
func (c *nullCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		c.recordSetLatency(time.Since(start))
	}()

	return nil
}

// Delete does nothing
func (c *nullCache) Delete(ctx context.Context, key string) error {
	start := time.Now()
	defer func() {
		c.recordDeleteLatency(time.Since(start))
	}()

	return nil
}

// Clear does nothing
func (c *nullCache) Clear(ctx context.Context) error {
	return nil
}

// Has always returns false
func (c *nullCache) Has(ctx context.Context, key string) bool {
	return false
}

// GetKeys always returns empty
func (c *nullCache) GetKeys(ctx context.Context) []string {
	return []string{}
}

// Close does nothing
func (c *nullCache) Close() error {
	return nil
}

// GetMany returns empty map
func (c *nullCache) GetMany(ctx context.Context, keys []string) (map[string]any, error) {
	for range keys {
		c.recordMiss()
	}
	return map[string]any{}, nil
}

// SetMany does nothing
func (c *nullCache) SetMany(ctx context.Context, items map[string]any, ttl time.Duration) error {
	return nil
}

// DeleteMany does nothing
func (c *nullCache) DeleteMany(ctx context.Context, keys []string) error {
	return nil
}

// GetMetadata returns nil
func (c *nullCache) GetMetadata(ctx context.Context, key string) (*interfaces.CacheEntryMetadata, error) {
	return nil, interfaces.ErrKeyNotFound
}

// GetManyMetadata returns empty map
func (c *nullCache) GetManyMetadata(ctx context.Context, keys []string) (map[string]*interfaces.CacheEntryMetadata, error) {
	return map[string]*interfaces.CacheEntryMetadata{}, nil
}

// Increment does nothing
func (c *nullCache) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return 0, nil
}

// Decrement does nothing
func (c *nullCache) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return 0, nil
}

// SetIfNotExists does nothing
func (c *nullCache) SetIfNotExists(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	return false, nil
}

// SetIfExists does nothing
func (c *nullCache) SetIfExists(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	return false, nil
}

// AddIndex does nothing
func (c *nullCache) AddIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error {
	return nil
}

// RemoveIndex does nothing
func (c *nullCache) RemoveIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error {
	return nil
}

// GetByIndex returns empty slice
func (c *nullCache) GetByIndex(ctx context.Context, indexName string, indexKey string) ([]string, error) {
	return []string{}, nil
}

// DeleteByIndex does nothing
func (c *nullCache) DeleteByIndex(ctx context.Context, indexName string, indexKey string) error {
	return nil
}

// GetKeysByPattern returns empty slice
func (c *nullCache) GetKeysByPattern(ctx context.Context, pattern string) ([]string, error) {
	return []string{}, nil
}

// DeleteByPattern does nothing
func (c *nullCache) DeleteByPattern(ctx context.Context, pattern string) (int, error) {
	return 0, nil
}

// UpdateMetadata does nothing
func (c *nullCache) UpdateMetadata(ctx context.Context, key string, updater interfaces.MetadataUpdater) error {
	return interfaces.ErrKeyNotFound
}

// GetAndUpdate does nothing
func (c *nullCache) GetAndUpdate(ctx context.Context, key string, updater interfaces.ValueUpdater, ttl time.Duration) (any, error) {
	return nil, interfaces.ErrKeyNotFound
}

// Metrics recording functions
func (c *nullCache) recordHit() {
	c.legacyMetrics.mu.Lock()
	c.legacyMetrics.hits++
	c.legacyMetrics.mu.Unlock()
	
	tags := c.getBaseTags()
	c.enhancedMetrics.RecordHit(c.providerName, tags)
}

func (c *nullCache) recordMiss() {
	c.legacyMetrics.mu.Lock()
	c.legacyMetrics.misses++
	c.legacyMetrics.mu.Unlock()
	
	tags := c.getBaseTags()
	c.enhancedMetrics.RecordMiss(c.providerName, tags)
}

func (c *nullCache) recordGetLatency(duration time.Duration) {
	c.legacyMetrics.mu.Lock()
	c.legacyMetrics.getLatency = duration
	c.legacyMetrics.mu.Unlock()
	
	tags := c.getBaseTags()
	c.enhancedMetrics.RecordOperation(c.providerName, "get", "completed", duration, tags)
}

func (c *nullCache) recordSetLatency(duration time.Duration) {
	c.legacyMetrics.mu.Lock()
	c.legacyMetrics.setLatency = duration
	c.legacyMetrics.mu.Unlock()
	
	tags := c.getBaseTags()
	c.enhancedMetrics.RecordOperation(c.providerName, "set", "completed", duration, tags)
}

func (c *nullCache) recordDeleteLatency(duration time.Duration) {
	c.legacyMetrics.mu.Lock()
	c.legacyMetrics.deleteLatency = duration
	c.legacyMetrics.mu.Unlock()
	
	tags := c.getBaseTags()
	c.enhancedMetrics.RecordOperation(c.providerName, "delete", "completed", duration, tags)
}

// Helper function to get base tags for metrics
func (c *nullCache) getBaseTags() metric.Tags {
	// Null cache typically doesn't need tags, but we'll provide empty ones
	return make(metric.Tags)
}
package middleware

import (
	"context"
	"time"

	gometrics "github.com/MichaelAJay/go-metrics"
	"github.com/MichaelAJay/go-cache"
)


// enhancedMetricsCache wraps a Cache with enhanced metrics capabilities
type enhancedMetricsCache struct {
	cache           cache.Cache
	enhancedMetrics cache.EnhancedCacheMetrics
	providerName    string
	tags            gometrics.Tags
}

// NewEnhancedMetricsMiddleware creates a new enhanced metrics middleware
// This middleware adds an additional layer of metrics on top of the provider's built-in metrics
func NewEnhancedMetricsMiddleware(enhancedMetrics cache.EnhancedCacheMetrics, providerName string, tags gometrics.Tags) cache.CacheMiddleware {
	return func(next cache.Cache) cache.Cache {
		return &enhancedMetricsCache{
			cache:           next,
			enhancedMetrics: enhancedMetrics,
			providerName:    providerName,
			tags:            tags,
		}
	}
}

// Enhanced metrics middleware implementation

// Get retrieves a value from the cache with enhanced metrics
func (c *enhancedMetricsCache) Get(ctx context.Context, key string) (any, bool, error) {
	start := time.Now()
	value, exists, err := c.cache.Get(ctx, key)
	duration := time.Since(start)

	// Record operation metrics
	status := "success"
	if err != nil {
		status = "error"
		c.enhancedMetrics.RecordError(c.providerName, "get", "cache_error", "unknown", c.tags)
	}
	
	c.enhancedMetrics.RecordOperation(c.providerName, "middleware_get", status, duration, c.tags)
	
	// Record hit/miss metrics
	if err == nil {
		if exists {
			c.enhancedMetrics.RecordHit(c.providerName, c.tags)
		} else {
			c.enhancedMetrics.RecordMiss(c.providerName, c.tags)
		}
	}

	return value, exists, err
}

// Set stores a value in the cache with enhanced metrics
func (c *enhancedMetricsCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	start := time.Now()
	err := c.cache.Set(ctx, key, value, ttl)
	duration := time.Since(start)
	
	status := "success"
	if err != nil {
		status = "error"
		c.enhancedMetrics.RecordError(c.providerName, "set", "cache_error", "unknown", c.tags)
	}
	
	c.enhancedMetrics.RecordOperation(c.providerName, "middleware_set", status, duration, c.tags)
	return err
}

// Delete removes a value from the cache with enhanced metrics
func (c *enhancedMetricsCache) Delete(ctx context.Context, key string) error {
	start := time.Now()
	err := c.cache.Delete(ctx, key)
	duration := time.Since(start)
	
	status := "success"
	if err != nil {
		status = "error"
		c.enhancedMetrics.RecordError(c.providerName, "delete", "cache_error", "unknown", c.tags)
	}
	
	c.enhancedMetrics.RecordOperation(c.providerName, "middleware_delete", status, duration, c.tags)
	return err
}

// Clear removes all values from the cache with enhanced metrics
func (c *enhancedMetricsCache) Clear(ctx context.Context) error {
	start := time.Now()
	err := c.cache.Clear(ctx)
	duration := time.Since(start)
	
	status := "success"
	if err != nil {
		status = "error"
		c.enhancedMetrics.RecordError(c.providerName, "clear", "cache_error", "unknown", c.tags)
	}
	
	c.enhancedMetrics.RecordOperation(c.providerName, "middleware_clear", status, duration, c.tags)
	return err
}

// Delegate all other methods to the underlying cache
func (c *enhancedMetricsCache) Has(ctx context.Context, key string) bool {
	return c.cache.Has(ctx, key)
}

func (c *enhancedMetricsCache) GetKeys(ctx context.Context) []string {
	return c.cache.GetKeys(ctx)
}

func (c *enhancedMetricsCache) Close() error {
	return c.cache.Close()
}

func (c *enhancedMetricsCache) GetMany(ctx context.Context, keys []string) (map[string]any, error) {
	start := time.Now()
	result, err := c.cache.GetMany(ctx, keys)
	duration := time.Since(start)
	
	// Record batch operation metrics
	c.enhancedMetrics.RecordBatchOperation(c.providerName, "middleware_get", len(keys), duration, c.tags)
	return result, err
}

func (c *enhancedMetricsCache) SetMany(ctx context.Context, items map[string]any, ttl time.Duration) error {
	start := time.Now()
	err := c.cache.SetMany(ctx, items, ttl)
	duration := time.Since(start)
	
	// Record batch operation metrics
	c.enhancedMetrics.RecordBatchOperation(c.providerName, "middleware_set", len(items), duration, c.tags)
	return err
}

func (c *enhancedMetricsCache) DeleteMany(ctx context.Context, keys []string) error {
	start := time.Now()
	err := c.cache.DeleteMany(ctx, keys)
	duration := time.Since(start)
	
	// Record batch operation metrics
	c.enhancedMetrics.RecordBatchOperation(c.providerName, "middleware_delete", len(keys), duration, c.tags)
	return err
}

func (c *enhancedMetricsCache) GetMetadata(ctx context.Context, key string) (*cache.CacheEntryMetadata, error) {
	return c.cache.GetMetadata(ctx, key)
}

func (c *enhancedMetricsCache) GetManyMetadata(ctx context.Context, keys []string) (map[string]*cache.CacheEntryMetadata, error) {
	return c.cache.GetManyMetadata(ctx, keys)
}


func (c *enhancedMetricsCache) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return c.cache.Increment(ctx, key, delta, ttl)
}

func (c *enhancedMetricsCache) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return c.cache.Decrement(ctx, key, delta, ttl)
}

func (c *enhancedMetricsCache) SetIfNotExists(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	return c.cache.SetIfNotExists(ctx, key, value, ttl)
}

func (c *enhancedMetricsCache) SetIfExists(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	return c.cache.SetIfExists(ctx, key, value, ttl)
}

func (c *enhancedMetricsCache) AddIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error {
	start := time.Now()
	err := c.cache.AddIndex(ctx, indexName, keyPattern, indexKey)
	duration := time.Since(start)
	
	c.enhancedMetrics.RecordIndexOperation(c.providerName, "middleware_add", indexName, duration, c.tags)
	return err
}

func (c *enhancedMetricsCache) RemoveIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error {
	start := time.Now()
	err := c.cache.RemoveIndex(ctx, indexName, keyPattern, indexKey)
	duration := time.Since(start)
	
	c.enhancedMetrics.RecordIndexOperation(c.providerName, "middleware_remove", indexName, duration, c.tags)
	return err
}

func (c *enhancedMetricsCache) GetByIndex(ctx context.Context, indexName string, indexKey string) ([]string, error) {
	start := time.Now()
	keys, err := c.cache.GetByIndex(ctx, indexName, indexKey)
	duration := time.Since(start)
	
	c.enhancedMetrics.RecordIndexOperation(c.providerName, "middleware_get", indexName, duration, c.tags)
	return keys, err
}

func (c *enhancedMetricsCache) DeleteByIndex(ctx context.Context, indexName string, indexKey string) error {
	start := time.Now()
	err := c.cache.DeleteByIndex(ctx, indexName, indexKey)
	duration := time.Since(start)
	
	c.enhancedMetrics.RecordIndexOperation(c.providerName, "middleware_delete", indexName, duration, c.tags)
	return err
}

func (c *enhancedMetricsCache) GetKeysByPattern(ctx context.Context, pattern string) ([]string, error) {
	return c.cache.GetKeysByPattern(ctx, pattern)
}

func (c *enhancedMetricsCache) DeleteByPattern(ctx context.Context, pattern string) (int, error) {
	return c.cache.DeleteByPattern(ctx, pattern)
}

func (c *enhancedMetricsCache) UpdateMetadata(ctx context.Context, key string, updater cache.MetadataUpdater) error {
	return c.cache.UpdateMetadata(ctx, key, updater)
}

func (c *enhancedMetricsCache) GetAndUpdate(ctx context.Context, key string, updater cache.ValueUpdater, ttl time.Duration) (any, error) {
	return c.cache.GetAndUpdate(ctx, key, updater, ttl)
}

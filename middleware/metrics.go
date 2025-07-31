package middleware

import (
	"context"
	"time"

	"github.com/MichaelAJay/go-cache"
)

// metricsCache wraps a Cache with metrics capabilities
type metricsCache struct {
	cache   cache.Cache
	metrics cache.CacheMetrics
}

// NewMetricsMiddleware creates a new metrics middleware
func NewMetricsMiddleware(metrics cache.CacheMetrics) cache.CacheMiddleware {
	return func(next cache.Cache) cache.Cache {
		return &metricsCache{
			cache:   next,
			metrics: metrics,
		}
	}
}

// Get retrieves a value from the cache with metrics
func (c *metricsCache) Get(ctx context.Context, key string) (any, bool, error) {
	start := time.Now()
	value, exists, err := c.cache.Get(ctx, key)
	duration := time.Since(start)

	c.metrics.RecordGetLatency(duration)
	if err == nil {
		if exists {
			c.metrics.RecordHit()
		} else {
			c.metrics.RecordMiss()
		}
	}

	return value, exists, err
}

// Set stores a value in the cache with metrics
func (c *metricsCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	start := time.Now()
	err := c.cache.Set(ctx, key, value, ttl)
	c.metrics.RecordSetLatency(time.Since(start))
	return err
}

// Delete removes a value from the cache with metrics
func (c *metricsCache) Delete(ctx context.Context, key string) error {
	start := time.Now()
	err := c.cache.Delete(ctx, key)
	c.metrics.RecordDeleteLatency(time.Since(start))
	return err
}

// Clear removes all values from the cache with metrics
func (c *metricsCache) Clear(ctx context.Context) error {
	return c.cache.Clear(ctx)
}

// Has checks if a key exists in the cache with metrics
func (c *metricsCache) Has(ctx context.Context, key string) bool {
	return c.cache.Has(ctx, key)
}

// GetKeys returns all keys in the cache with metrics
func (c *metricsCache) GetKeys(ctx context.Context) []string {
	return c.cache.GetKeys(ctx)
}

// Close closes the cache with metrics
func (c *metricsCache) Close() error {
	return c.cache.Close()
}

// GetMany retrieves multiple values from the cache with metrics
func (c *metricsCache) GetMany(ctx context.Context, keys []string) (map[string]any, error) {
	start := time.Now()
	values, err := c.cache.GetMany(ctx, keys)
	c.metrics.RecordGetLatency(time.Since(start))
	return values, err
}

// SetMany stores multiple values in the cache with metrics
func (c *metricsCache) SetMany(ctx context.Context, items map[string]any, ttl time.Duration) error {
	start := time.Now()
	err := c.cache.SetMany(ctx, items, ttl)
	c.metrics.RecordSetLatency(time.Since(start))
	return err
}

// DeleteMany removes multiple values from the cache with metrics
func (c *metricsCache) DeleteMany(ctx context.Context, keys []string) error {
	start := time.Now()
	err := c.cache.DeleteMany(ctx, keys)
	c.metrics.RecordDeleteLatency(time.Since(start))
	return err
}

// GetMetadata retrieves metadata for a cache entry with metrics
func (c *metricsCache) GetMetadata(ctx context.Context, key string) (*cache.CacheEntryMetadata, error) {
	start := time.Now()
	metadata, err := c.cache.GetMetadata(ctx, key)
	c.metrics.RecordGetLatency(time.Since(start))
	return metadata, err
}

// GetManyMetadata retrieves metadata for multiple cache entries with metrics
func (c *metricsCache) GetManyMetadata(ctx context.Context, keys []string) (map[string]*cache.CacheEntryMetadata, error) {
	start := time.Now()
	metadata, err := c.cache.GetManyMetadata(ctx, keys)
	c.metrics.RecordGetLatency(time.Since(start))
	return metadata, err
}

// GetMetrics returns the current metrics snapshot
func (c *metricsCache) GetMetrics() *cache.CacheMetricsSnapshot {
	return c.cache.GetMetrics()
}

// Increment atomically increments a numeric value with metrics
func (c *metricsCache) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	start := time.Now()
	result, err := c.cache.Increment(ctx, key, delta, ttl)
	c.metrics.RecordSetLatency(time.Since(start))
	return result, err
}

// Decrement atomically decrements a numeric value with metrics
func (c *metricsCache) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	start := time.Now()
	result, err := c.cache.Decrement(ctx, key, delta, ttl)
	c.metrics.RecordSetLatency(time.Since(start))
	return result, err
}

// SetIfNotExists sets a value only if the key doesn't exist with metrics
func (c *metricsCache) SetIfNotExists(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	start := time.Now()
	success, err := c.cache.SetIfNotExists(ctx, key, value, ttl)
	c.metrics.RecordSetLatency(time.Since(start))
	return success, err
}

// SetIfExists sets a value only if the key already exists with metrics
func (c *metricsCache) SetIfExists(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	start := time.Now()
	success, err := c.cache.SetIfExists(ctx, key, value, ttl)
	c.metrics.RecordSetLatency(time.Since(start))
	return success, err
}

// AddIndex adds a secondary index with metrics
func (c *metricsCache) AddIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error {
	start := time.Now()
	err := c.cache.AddIndex(ctx, indexName, keyPattern, indexKey)
	c.metrics.RecordSetLatency(time.Since(start))
	return err
}

// RemoveIndex removes a secondary index with metrics
func (c *metricsCache) RemoveIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error {
	start := time.Now()
	err := c.cache.RemoveIndex(ctx, indexName, keyPattern, indexKey)
	c.metrics.RecordDeleteLatency(time.Since(start))
	return err
}

// GetByIndex retrieves keys by index with metrics
func (c *metricsCache) GetByIndex(ctx context.Context, indexName string, indexKey string) ([]string, error) {
	start := time.Now()
	keys, err := c.cache.GetByIndex(ctx, indexName, indexKey)
	c.metrics.RecordGetLatency(time.Since(start))
	return keys, err
}

// DeleteByIndex deletes keys by index with metrics
func (c *metricsCache) DeleteByIndex(ctx context.Context, indexName string, indexKey string) error {
	start := time.Now()
	err := c.cache.DeleteByIndex(ctx, indexName, indexKey)
	c.metrics.RecordDeleteLatency(time.Since(start))
	return err
}

// GetKeysByPattern retrieves keys by pattern with metrics
func (c *metricsCache) GetKeysByPattern(ctx context.Context, pattern string) ([]string, error) {
	start := time.Now()
	keys, err := c.cache.GetKeysByPattern(ctx, pattern)
	c.metrics.RecordGetLatency(time.Since(start))
	return keys, err
}

// DeleteByPattern deletes keys by pattern with metrics
func (c *metricsCache) DeleteByPattern(ctx context.Context, pattern string) (int, error) {
	start := time.Now()
	count, err := c.cache.DeleteByPattern(ctx, pattern)
	c.metrics.RecordDeleteLatency(time.Since(start))
	return count, err
}

// UpdateMetadata updates cache entry metadata with metrics
func (c *metricsCache) UpdateMetadata(ctx context.Context, key string, updater cache.MetadataUpdater) error {
	start := time.Now()
	err := c.cache.UpdateMetadata(ctx, key, updater)
	c.metrics.RecordSetLatency(time.Since(start))
	return err
}

// GetAndUpdate atomically gets and updates a cache entry with metrics
func (c *metricsCache) GetAndUpdate(ctx context.Context, key string, updater cache.ValueUpdater, ttl time.Duration) (any, error) {
	start := time.Now()
	value, err := c.cache.GetAndUpdate(ctx, key, updater, ttl)
	c.metrics.RecordGetLatency(time.Since(start))
	return value, err
}

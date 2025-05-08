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

package null

import (
	"context"
	"sync"
	"time"

	"github.com/MichaelAJay/go-cache"
)

// nullCache implements the Cache interface but does nothing
type nullCache struct {
	metrics *cacheMetrics
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

// NewNullCache creates a new null cache instance
func NewNullCache(options *cache.CacheOptions) cache.Cache {
	return &nullCache{
		metrics: &cacheMetrics{},
	}
}

// Get always returns not found
func (c *nullCache) Get(ctx context.Context, key string) (any, bool, error) {
	start := time.Now()
	defer func() {
		c.metrics.recordGetLatency(time.Since(start))
		c.metrics.recordMiss()
	}()
	return nil, false, nil
}

// Set does nothing
func (c *nullCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		c.metrics.recordSetLatency(time.Since(start))
	}()
	return nil
}

// Delete does nothing
func (c *nullCache) Delete(ctx context.Context, key string) error {
	start := time.Now()
	defer func() {
		c.metrics.recordDeleteLatency(time.Since(start))
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

// GetKeys returns an empty slice
func (c *nullCache) GetKeys(ctx context.Context) []string {
	return []string{}
}

// Close does nothing
func (c *nullCache) Close() error {
	return nil
}

// GetMany returns an empty map
func (c *nullCache) GetMany(ctx context.Context, keys []string) (map[string]any, error) {
	return make(map[string]any), nil
}

// SetMany does nothing
func (c *nullCache) SetMany(ctx context.Context, items map[string]any, ttl time.Duration) error {
	return nil
}

// DeleteMany does nothing
func (c *nullCache) DeleteMany(ctx context.Context, keys []string) error {
	return nil
}

// GetMetadata always returns ErrKeyNotFound
func (c *nullCache) GetMetadata(ctx context.Context, key string) (*cache.CacheEntryMetadata, error) {
	return nil, cache.ErrKeyNotFound
}

// GetManyMetadata returns an empty map
func (c *nullCache) GetManyMetadata(ctx context.Context, keys []string) (map[string]*cache.CacheEntryMetadata, error) {
	return make(map[string]*cache.CacheEntryMetadata), nil
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
func (m *cacheMetrics) GetMetrics() *cache.CacheMetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := m.hits + m.misses
	hitRatio := 0.0
	if total > 0 {
		hitRatio = float64(m.hits) / float64(total)
	}

	return &cache.CacheMetricsSnapshot{
		Hits:          m.hits,
		Misses:        m.misses,
		HitRatio:      hitRatio,
		GetLatency:    m.getLatency,
		SetLatency:    m.setLatency,
		DeleteLatency: m.deleteLatency,
		CacheSize:     0,
		EntryCount:    0,
	}
}

// GetMetrics returns the current metrics snapshot
func (c *nullCache) GetMetrics() *cache.CacheMetricsSnapshot {
	return c.metrics.GetMetrics()
}

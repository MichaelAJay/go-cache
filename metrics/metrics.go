package metrics

import (
	"sync"
	"time"
)

// CacheMetrics defines the interface for cache metrics
type CacheMetrics interface {
	RecordHit()
	RecordMiss()
	RecordGetLatency(duration time.Duration)
	RecordSetLatency(duration time.Duration)
	RecordDeleteLatency(duration time.Duration)
	RecordCacheSize(size int64)
	RecordEntryCount(count int64)
	GetMetrics() *CacheMetricsSnapshot
}

// CacheMetricsSnapshot represents a snapshot of cache metrics
type CacheMetricsSnapshot struct {
	Hits          int64
	Misses        int64
	HitRatio      float64
	GetLatency    time.Duration
	SetLatency    time.Duration
	DeleteLatency time.Duration
	CacheSize     int64
	EntryCount    int64
}

// defaultMetrics implements the CacheMetrics interface
type defaultMetrics struct {
	hits          int64
	misses        int64
	getLatency    time.Duration
	setLatency    time.Duration
	deleteLatency time.Duration
	cacheSize     int64
	entryCount    int64
	mu            sync.RWMutex
}

// NewMetrics creates a new metrics instance
func NewMetrics() CacheMetrics {
	return &defaultMetrics{}
}

// RecordHit records a cache hit
func (m *defaultMetrics) RecordHit() {
	m.mu.Lock()
	m.hits++
	m.mu.Unlock()
}

// RecordMiss records a cache miss
func (m *defaultMetrics) RecordMiss() {
	m.mu.Lock()
	m.misses++
	m.mu.Unlock()
}

// RecordGetLatency records the latency of a Get operation
func (m *defaultMetrics) RecordGetLatency(duration time.Duration) {
	m.mu.Lock()
	m.getLatency = duration
	m.mu.Unlock()
}

// RecordSetLatency records the latency of a Set operation
func (m *defaultMetrics) RecordSetLatency(duration time.Duration) {
	m.mu.Lock()
	m.setLatency = duration
	m.mu.Unlock()
}

// RecordDeleteLatency records the latency of a Delete operation
func (m *defaultMetrics) RecordDeleteLatency(duration time.Duration) {
	m.mu.Lock()
	m.deleteLatency = duration
	m.mu.Unlock()
}

// RecordCacheSize records the current cache size
func (m *defaultMetrics) RecordCacheSize(size int64) {
	m.mu.Lock()
	m.cacheSize = size
	m.mu.Unlock()
}

// RecordEntryCount records the current number of entries
func (m *defaultMetrics) RecordEntryCount(count int64) {
	m.mu.Lock()
	m.entryCount = count
	m.mu.Unlock()
}

// GetMetrics returns the current metrics snapshot
func (m *defaultMetrics) GetMetrics() *CacheMetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := m.hits + m.misses
	hitRatio := 0.0
	if total > 0 {
		hitRatio = float64(m.hits) / float64(total)
	}

	return &CacheMetricsSnapshot{
		Hits:          m.hits,
		Misses:        m.misses,
		HitRatio:      hitRatio,
		GetLatency:    m.getLatency,
		SetLatency:    m.setLatency,
		DeleteLatency: m.deleteLatency,
		CacheSize:     m.cacheSize,
		EntryCount:    m.entryCount,
	}
}
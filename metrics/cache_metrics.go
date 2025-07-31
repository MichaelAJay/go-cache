package metrics

import (
	"sync"
	"time"

	gometrics "github.com/MichaelAJay/go-metrics"
	"github.com/MichaelAJay/go-metrics/operational"
)

// CacheMetrics provides comprehensive cache metrics using go-metrics
type CacheMetrics interface {
	// Core cache operations
	RecordHit(provider string, tags gometrics.Tags)
	RecordMiss(provider string, tags gometrics.Tags)
	RecordOperation(provider, operation, status string, duration time.Duration, tags gometrics.Tags)
	RecordError(provider, operation, errorType, errorCategory string, tags gometrics.Tags)
	
	// Cache state metrics
	RecordCacheSize(provider string, size int64, tags gometrics.Tags)
	RecordEntryCount(provider string, count int64, tags gometrics.Tags)
	
	// Advanced operations
	RecordBatchOperation(provider, operation string, batchSize int, duration time.Duration, tags gometrics.Tags)
	RecordIndexOperation(provider, operation, indexName string, duration time.Duration, tags gometrics.Tags)
	
	// Security and reliability
	RecordSecurityEvent(provider, eventType, severity string, tags gometrics.Tags)
	RecordTimingProtection(provider, operation string, actualTime, adjustedTime time.Duration, tags gometrics.Tags)
	
	// Registry access for advanced use cases
	Registry() gometrics.Registry
	
}


// cacheMetrics implements CacheMetrics using go-metrics
type cacheMetrics struct {
	registry       gometrics.Registry
	operational    operational.OperationalMetrics
	
	// Cached metric instances for performance
	hitCounters       map[string]gometrics.Counter
	missCounters      map[string]gometrics.Counter
	operationTimers   map[string]gometrics.Timer
	operationCounters map[string]gometrics.Counter
	sizeGauges        map[string]gometrics.Gauge
	entryGauges       map[string]gometrics.Gauge
	securityCounters  map[string]gometrics.Counter
	
	// Mutex for thread-safe metric caching
	mu sync.RWMutex
	
}

// NewCacheMetrics creates a new CacheMetrics instance
func NewCacheMetrics(registry gometrics.Registry) CacheMetrics {
	if registry == nil {
		registry = gometrics.NewRegistry()
	}
	
	return &cacheMetrics{
		registry:          registry,
		operational:       operational.New(registry),
		hitCounters:       make(map[string]gometrics.Counter),
		missCounters:      make(map[string]gometrics.Counter),
		operationTimers:   make(map[string]gometrics.Timer),
		operationCounters: make(map[string]gometrics.Counter),
		sizeGauges:        make(map[string]gometrics.Gauge),
		entryGauges:       make(map[string]gometrics.Gauge),
		securityCounters:  make(map[string]gometrics.Counter),
	}
}

// NewNoopCacheMetrics creates a cache metrics instance that discards all metrics
func NewNoopCacheMetrics() CacheMetrics {
	return NewCacheMetrics(gometrics.NewNoop())
}

// RecordHit records a cache hit
func (c *cacheMetrics) RecordHit(provider string, tags gometrics.Tags) {
	counter := c.getOrCreateHitCounter(provider, tags)
	counter.Inc()
}

// RecordMiss records a cache miss
func (c *cacheMetrics) RecordMiss(provider string, tags gometrics.Tags) {
	counter := c.getOrCreateMissCounter(provider, tags)
	counter.Inc()
}

// RecordOperation records a cache operation with timing
func (c *cacheMetrics) RecordOperation(provider, operation, status string, duration time.Duration, tags gometrics.Tags) {
	// Record with operational metrics
	c.operational.RecordOperation(operation, status, duration)
	
	// Record with detailed metrics
	timer := c.getOrCreateOperationTimer(provider, operation, tags)
	timer.Record(duration)
	
	counter := c.getOrCreateOperationCounter(provider, operation, status, tags)
	counter.Inc()
}

// RecordError records an error during cache operations
func (c *cacheMetrics) RecordError(provider, operation, errorType, errorCategory string, tags gometrics.Tags) {
	// Record with operational metrics
	c.operational.RecordError(operation, errorType, errorCategory)
	
	// Record with detailed provider metrics
	if tags == nil {
		tags = make(gometrics.Tags)
	}
	tags["provider"] = provider
	tags["error_type"] = errorType
	tags["error_category"] = errorCategory
	
	key := c.cacheKey("error", provider, operation, errorType, errorCategory)
	counter := c.getOrCreateSecurityCounter(key, tags)
	counter.Inc()
}

// RecordCacheSize records the current cache size
func (c *cacheMetrics) RecordCacheSize(provider string, size int64, tags gometrics.Tags) {
	gauge := c.getOrCreateSizeGauge(provider, tags)
	gauge.Set(float64(size))
}

// RecordEntryCount records the current number of entries
func (c *cacheMetrics) RecordEntryCount(provider string, count int64, tags gometrics.Tags) {
	gauge := c.getOrCreateEntryGauge(provider, tags)
	gauge.Set(float64(count))
}

// RecordBatchOperation records metrics for batch operations
func (c *cacheMetrics) RecordBatchOperation(provider, operation string, batchSize int, duration time.Duration, tags gometrics.Tags) {
	if tags == nil {
		tags = make(gometrics.Tags)
	}
	tags["provider"] = provider
	tags["batch_size"] = string(rune(batchSize))
	
	timer := c.getOrCreateOperationTimer(provider, "batch_"+operation, tags)
	timer.Record(duration)
	
	counter := c.getOrCreateOperationCounter(provider, "batch_"+operation, "completed", tags)
	counter.Inc()
}

// RecordIndexOperation records metrics for index operations
func (c *cacheMetrics) RecordIndexOperation(provider, operation, indexName string, duration time.Duration, tags gometrics.Tags) {
	if tags == nil {
		tags = make(gometrics.Tags)
	}
	tags["provider"] = provider
	tags["index_name"] = indexName
	
	timer := c.getOrCreateOperationTimer(provider, "index_"+operation, tags)
	timer.Record(duration)
	
	counter := c.getOrCreateOperationCounter(provider, "index_"+operation, "completed", tags)
	counter.Inc()
}

// RecordSecurityEvent records security-related events
func (c *cacheMetrics) RecordSecurityEvent(provider, eventType, severity string, tags gometrics.Tags) {
	if tags == nil {
		tags = make(gometrics.Tags)
	}
	tags["provider"] = provider
	tags["event_type"] = eventType
	tags["severity"] = severity
	
	key := c.cacheKey("security", provider, eventType, severity)
	counter := c.getOrCreateSecurityCounter(key, tags)
	counter.Inc()
}

// RecordTimingProtection records timing attack protection adjustments
func (c *cacheMetrics) RecordTimingProtection(provider, operation string, actualTime, adjustedTime time.Duration, tags gometrics.Tags) {
	if tags == nil {
		tags = make(gometrics.Tags)
	}
	tags["provider"] = provider
	tags["operation"] = operation
	
	// Record the adjustment
	key := c.cacheKey("timing_protection", provider, operation)
	counter := c.getOrCreateSecurityCounter(key, tags)
	counter.Inc()
	
	// Record actual vs adjusted time difference
	adjustmentTags := make(gometrics.Tags)
	for k, v := range tags {
		adjustmentTags[k] = v
	}
	adjustmentTags["type"] = "adjustment"
	
	timer := c.getOrCreateOperationTimer(provider, "timing_adjustment", adjustmentTags)
	timer.Record(adjustedTime - actualTime)
}

// Registry returns the underlying metrics registry
func (c *cacheMetrics) Registry() gometrics.Registry {
	return c.registry
}


// Helper methods for metric creation and caching

func (c *cacheMetrics) cacheKey(parts ...string) string {
	key := ""
	for i, part := range parts {
		if i > 0 {
			key += ":"
		}
		key += part
	}
	return key
}

func (c *cacheMetrics) getOrCreateHitCounter(provider string, tags gometrics.Tags) gometrics.Counter {
	key := c.cacheKey("hit", provider)
	
	c.mu.RLock()
	if counter, exists := c.hitCounters[key]; exists {
		c.mu.RUnlock()
		return counter.With(tags)
	}
	c.mu.RUnlock()
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if counter, exists := c.hitCounters[key]; exists {
		return counter.With(tags)
	}
	
	baseTags := gometrics.Tags{"provider": provider}
	counter := c.registry.Counter(gometrics.Options{
		Name:        "cache_hits_total",
		Description: "Total number of cache hits",
		Unit:        "count",
		Tags:        baseTags,
	})
	
	c.hitCounters[key] = counter
	return counter.With(tags)
}

func (c *cacheMetrics) getOrCreateMissCounter(provider string, tags gometrics.Tags) gometrics.Counter {
	key := c.cacheKey("miss", provider)
	
	c.mu.RLock()
	if counter, exists := c.missCounters[key]; exists {
		c.mu.RUnlock()
		return counter.With(tags)
	}
	c.mu.RUnlock()
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if counter, exists := c.missCounters[key]; exists {
		return counter.With(tags)
	}
	
	baseTags := gometrics.Tags{"provider": provider}
	counter := c.registry.Counter(gometrics.Options{
		Name:        "cache_misses_total",
		Description: "Total number of cache misses",
		Unit:        "count",
		Tags:        baseTags,
	})
	
	c.missCounters[key] = counter
	return counter.With(tags)
}

func (c *cacheMetrics) getOrCreateOperationTimer(provider, operation string, tags gometrics.Tags) gometrics.Timer {
	key := c.cacheKey("timer", provider, operation)
	
	c.mu.RLock()
	if timer, exists := c.operationTimers[key]; exists {
		c.mu.RUnlock()
		return timer.With(tags)
	}
	c.mu.RUnlock()
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if timer, exists := c.operationTimers[key]; exists {
		return timer.With(tags)
	}
	
	baseTags := gometrics.Tags{"provider": provider, "operation": operation}
	timer := c.registry.Timer(gometrics.Options{
		Name:        "cache_operation_duration",
		Description: "Duration of cache operations",
		Unit:        "nanoseconds",
		Tags:        baseTags,
	})
	
	c.operationTimers[key] = timer
	return timer.With(tags)
}

func (c *cacheMetrics) getOrCreateOperationCounter(provider, operation, status string, tags gometrics.Tags) gometrics.Counter {
	key := c.cacheKey("counter", provider, operation, status)
	
	c.mu.RLock()
	if counter, exists := c.operationCounters[key]; exists {
		c.mu.RUnlock()
		return counter.With(tags)
	}
	c.mu.RUnlock()
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if counter, exists := c.operationCounters[key]; exists {
		return counter.With(tags)
	}
	
	baseTags := gometrics.Tags{"provider": provider, "operation": operation, "status": status}
	counter := c.registry.Counter(gometrics.Options{
		Name:        "cache_operations_total",
		Description: "Total number of cache operations",
		Unit:        "count",
		Tags:        baseTags,
	})
	
	c.operationCounters[key] = counter
	return counter.With(tags)
}

func (c *cacheMetrics) getOrCreateSizeGauge(provider string, tags gometrics.Tags) gometrics.Gauge {
	key := c.cacheKey("size", provider)
	
	c.mu.RLock()
	if gauge, exists := c.sizeGauges[key]; exists {
		c.mu.RUnlock()
		return gauge.With(tags)
	}
	c.mu.RUnlock()
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if gauge, exists := c.sizeGauges[key]; exists {
		return gauge.With(tags)
	}
	
	baseTags := gometrics.Tags{"provider": provider}
	gauge := c.registry.Gauge(gometrics.Options{
		Name:        "cache_size_bytes",
		Description: "Current cache size in bytes",
		Unit:        "bytes",
		Tags:        baseTags,
	})
	
	c.sizeGauges[key] = gauge
	return gauge.With(tags)
}

func (c *cacheMetrics) getOrCreateEntryGauge(provider string, tags gometrics.Tags) gometrics.Gauge {
	key := c.cacheKey("entries", provider)
	
	c.mu.RLock()
	if gauge, exists := c.entryGauges[key]; exists {
		c.mu.RUnlock()
		return gauge.With(tags)
	}
	c.mu.RUnlock()
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if gauge, exists := c.entryGauges[key]; exists {
		return gauge.With(tags)
	}
	
	baseTags := gometrics.Tags{"provider": provider}
	gauge := c.registry.Gauge(gometrics.Options{
		Name:        "cache_entries_total",
		Description: "Current number of cache entries",
		Unit:        "count",
		Tags:        baseTags,
	})
	
	c.entryGauges[key] = gauge
	return gauge.With(tags)
}

func (c *cacheMetrics) getOrCreateSecurityCounter(key string, tags gometrics.Tags) gometrics.Counter {
	c.mu.RLock()
	if counter, exists := c.securityCounters[key]; exists {
		c.mu.RUnlock()
		return counter.With(tags)
	}
	c.mu.RUnlock()
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if counter, exists := c.securityCounters[key]; exists {
		return counter.With(tags)
	}
	
	counter := c.registry.Counter(gometrics.Options{
		Name:        "cache_security_events_total",
		Description: "Total number of cache security events",
		Unit:        "count",
		Tags:        tags,
	})
	
	c.securityCounters[key] = counter
	return counter
}


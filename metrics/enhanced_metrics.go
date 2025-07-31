package metrics

import (
	"time"

	"github.com/MichaelAJay/go-metrics/metric"
)

// EnhancedCacheMetrics interface (local definition to avoid circular imports)
type EnhancedCacheMetrics interface {
	Registry() metric.Registry
	RecordHit(provider string, tags metric.Tags)
	RecordMiss(provider string, tags metric.Tags)
	RecordOperation(provider, operation, status string, duration time.Duration, tags metric.Tags)
	RecordError(provider, operation, errorType, errorCategory string, tags metric.Tags)
	RecordCacheSize(provider string, size int64, tags metric.Tags)
	RecordEntryCount(provider string, count int64, tags metric.Tags)
	RecordBatchOperation(provider, operation string, batchSize int, duration time.Duration, tags metric.Tags)
	RecordIndexOperation(provider, operation, indexName string, duration time.Duration, tags metric.Tags)
	RecordSecurityEvent(provider, eventType, severity string, tags metric.Tags)
	RecordTimingProtection(provider, operation string, actualTime, adjustedTime time.Duration, tags metric.Tags)
	RecordCleanup(provider string, reason CleanupReason, itemCount int, duration time.Duration, tags metric.Tags)
	RecordMemoryUsage(provider string, totalSize int64, entryCount int64, tags metric.Tags)
	RecordProviderSpecific(provider, metricName string, value float64, tags metric.Tags)
}

// CleanupReason is imported from the main cache package to avoid duplication
type CleanupReason = string

const (
	CleanupExpired CleanupReason = "expired"
	CleanupEvicted CleanupReason = "evicted"
	CleanupManual  CleanupReason = "manual"
)

// enhancedCacheMetrics implements the EnhancedCacheMetrics interface
// It wraps the core CacheMetrics and adds cache-specific functionality
type enhancedCacheMetrics struct {
	core        CacheMetrics
	registry    metric.Registry
	globalTags  metric.Tags
}

// NewEnhancedCacheMetrics creates a new EnhancedCacheMetrics instance
func NewEnhancedCacheMetrics(registry metric.Registry, globalTags metric.Tags) EnhancedCacheMetrics {
	if registry == nil {
		registry = metric.NewDefaultRegistry()
	}
	
	core := NewCacheMetrics(registry)
	
	return &enhancedCacheMetrics{
		core:       core,
		registry:   registry,
		globalTags: globalTags,
	}
}

// NewNoopEnhancedCacheMetrics creates a no-op enhanced cache metrics instance
func NewNoopEnhancedCacheMetrics() EnhancedCacheMetrics {
	return &enhancedCacheMetrics{
		core:       NewNoopCacheMetrics(),
		registry:   metric.NewNoop(),
		globalTags: make(metric.Tags),
	}
}

// Registry returns the underlying go-metrics registry
func (e *enhancedCacheMetrics) Registry() metric.Registry {
	return e.registry
}

// RecordHit records a cache hit
func (e *enhancedCacheMetrics) RecordHit(provider string, tags metric.Tags) {
	mergedTags := e.mergeTags(tags)
	e.core.RecordHit(provider, mergedTags)
}

// RecordMiss records a cache miss
func (e *enhancedCacheMetrics) RecordMiss(provider string, tags metric.Tags) {
	mergedTags := e.mergeTags(tags)
	e.core.RecordMiss(provider, mergedTags)
}

// RecordOperation records a cache operation with timing
func (e *enhancedCacheMetrics) RecordOperation(provider, operation, status string, duration time.Duration, tags metric.Tags) {
	mergedTags := e.mergeTags(tags)
	e.core.RecordOperation(provider, operation, status, duration, mergedTags)
}

// RecordError records an error during cache operations
func (e *enhancedCacheMetrics) RecordError(provider, operation, errorType, errorCategory string, tags metric.Tags) {
	mergedTags := e.mergeTags(tags)
	e.core.RecordError(provider, operation, errorType, errorCategory, mergedTags)
}

// RecordCacheSize records the current cache size
func (e *enhancedCacheMetrics) RecordCacheSize(provider string, size int64, tags metric.Tags) {
	mergedTags := e.mergeTags(tags)
	e.core.RecordCacheSize(provider, size, mergedTags)
}

// RecordEntryCount records the current number of entries
func (e *enhancedCacheMetrics) RecordEntryCount(provider string, count int64, tags metric.Tags) {
	mergedTags := e.mergeTags(tags)
	e.core.RecordEntryCount(provider, count, mergedTags)
}

// RecordBatchOperation records metrics for batch operations
func (e *enhancedCacheMetrics) RecordBatchOperation(provider, operation string, batchSize int, duration time.Duration, tags metric.Tags) {
	mergedTags := e.mergeTags(tags)
	e.core.RecordBatchOperation(provider, operation, batchSize, duration, mergedTags)
}

// RecordIndexOperation records metrics for index operations
func (e *enhancedCacheMetrics) RecordIndexOperation(provider, operation, indexName string, duration time.Duration, tags metric.Tags) {
	mergedTags := e.mergeTags(tags)
	e.core.RecordIndexOperation(provider, operation, indexName, duration, mergedTags)
}

// RecordSecurityEvent records security-related events
func (e *enhancedCacheMetrics) RecordSecurityEvent(provider, eventType, severity string, tags metric.Tags) {
	mergedTags := e.mergeTags(tags)
	e.core.RecordSecurityEvent(provider, eventType, severity, mergedTags)
}

// RecordTimingProtection records timing attack protection adjustments
func (e *enhancedCacheMetrics) RecordTimingProtection(provider, operation string, actualTime, adjustedTime time.Duration, tags metric.Tags) {
	mergedTags := e.mergeTags(tags)
	e.core.RecordTimingProtection(provider, operation, actualTime, adjustedTime, mergedTags)
}

// RecordCleanup records cleanup operations
func (e *enhancedCacheMetrics) RecordCleanup(provider string, reason CleanupReason, itemCount int, duration time.Duration, tags metric.Tags) {
	if tags == nil {
		tags = make(metric.Tags)
	}
	tags["provider"] = provider
	tags["reason"] = string(reason)
	tags["item_count"] = string(rune(itemCount))
	
	mergedTags := e.mergeTags(tags)
	
	// Record cleanup operation
	counter := e.registry.Counter(metric.Options{
		Name:        "cache_cleanup_operations_total",
		Description: "Total number of cache cleanup operations",
		Unit:        "count",
		Tags:        mergedTags,
	})
	counter.Inc()
	
	// Record cleanup duration
	timer := e.registry.Timer(metric.Options{
		Name:        "cache_cleanup_duration",
		Description: "Duration of cache cleanup operations",
		Unit:        "nanoseconds",
		Tags:        mergedTags,
	})
	timer.Record(duration)
}

// RecordMemoryUsage records memory usage metrics
func (e *enhancedCacheMetrics) RecordMemoryUsage(provider string, totalSize int64, entryCount int64, tags metric.Tags) {
	if tags == nil {
		tags = make(metric.Tags)
	}
	tags["provider"] = provider
	
	mergedTags := e.mergeTags(tags)
	
	// Record total memory usage
	sizeGauge := e.registry.Gauge(metric.Options{
		Name:        "cache_memory_usage_bytes",
		Description: "Current cache memory usage in bytes",
		Unit:        "bytes",
		Tags:        mergedTags,
	})
	sizeGauge.Set(float64(totalSize))
	
	// Record entry count
	entryGauge := e.registry.Gauge(metric.Options{
		Name:        "cache_entries_current",
		Description: "Current number of cache entries",
		Unit:        "count",
		Tags:        mergedTags,
	})
	entryGauge.Set(float64(entryCount))
}

// RecordProviderSpecific records provider-specific metrics
func (e *enhancedCacheMetrics) RecordProviderSpecific(provider, metricName string, value float64, tags metric.Tags) {
	if tags == nil {
		tags = make(metric.Tags)
	}
	tags["provider"] = provider
	
	mergedTags := e.mergeTags(tags)
	
	gauge := e.registry.Gauge(metric.Options{
		Name:        "cache_provider_" + metricName,
		Description: "Provider-specific metric: " + metricName,
		Tags:        mergedTags,
	})
	gauge.Set(value)
}


// Note: CleanupReason is defined in the main cache package to avoid circular imports

// mergeTags merges global tags with provided tags, with provided tags taking precedence
func (e *enhancedCacheMetrics) mergeTags(tags metric.Tags) metric.Tags {
	if e.globalTags == nil && tags == nil {
		return nil
	}
	
	merged := make(metric.Tags)
	
	// Add global tags first
	if e.globalTags != nil {
		for k, v := range e.globalTags {
			merged[k] = v
		}
	}
	
	// Add provided tags, overriding global tags
	if tags != nil {
		for k, v := range tags {
			merged[k] = v
		}
	}
	
	return merged
}
// Package metrics provides comprehensive cache metrics using go-metrics
// This package serves as the main entry point for cache metrics functionality,
// providing both new go-metrics based interfaces and backward compatibility
package metrics

import (
	"github.com/MichaelAJay/go-metrics/metric"
)

// Factory functions for creating metrics instances

// NewComprehensiveCacheMetrics creates a new comprehensive cache metrics instance
func NewComprehensiveCacheMetrics(registry metric.Registry) CacheMetrics {
	return NewCacheMetrics(registry)
}

// NewDiscardingCacheMetrics creates a no-op cache metrics instance for testing
func NewDiscardingCacheMetrics() CacheMetrics {
	return NewNoopCacheMetrics()
}

// Legacy functions are now defined in legacy.go to avoid redeclaration

// Convenience functions for common use cases

// NewMemoryProviderMetrics creates metrics specifically for the memory provider
func NewMemoryProviderMetrics(registry metric.Registry) CacheMetrics {
	if registry == nil {
		registry = metric.NewDefaultRegistry()
	}
	return NewCacheMetrics(registry)
}

// NewRedisProviderMetrics creates metrics specifically for the Redis provider
func NewRedisProviderMetrics(registry metric.Registry) CacheMetrics {
	if registry == nil {
		registry = metric.NewDefaultRegistry()
	}
	return NewCacheMetrics(registry)
}

// NewProviderMetrics creates metrics for a specific provider type
func NewProviderMetrics(provider string, registry metric.Registry) CacheMetrics {
	if registry == nil {
		registry = metric.NewDefaultRegistry()
	}
	return NewCacheMetrics(registry)
}

// Helper functions for working with metrics tags

// ProviderTags creates standard tags for a cache provider
func ProviderTags(provider string) metric.Tags {
	return metric.Tags{
		"provider": provider,
	}
}

// OperationTags creates standard tags for a cache operation
func OperationTags(provider, operation string) metric.Tags {
	return metric.Tags{
		"provider":  provider,
		"operation": operation,
	}
}

// ErrorTags creates standard tags for error tracking
func ErrorTags(provider, operation, errorType, errorCategory string) metric.Tags {
	return metric.Tags{
		"provider":       provider,
		"operation":      operation,
		"error_type":     errorType,
		"error_category": errorCategory,
	}
}

// SecurityTags creates standard tags for security events
func SecurityTags(provider, eventType, severity string) metric.Tags {
	return metric.Tags{
		"provider":   provider,
		"event_type": eventType,
		"severity":   severity,
	}
}
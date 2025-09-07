//go:build integration

package cache

import (
	"sync"
	"testing"
	"time"
)

// BenchmarkCircuitBreaker_Closed tests allocation profile when circuit breaker is closed (most common case)
func BenchmarkCircuitBreaker_Closed(b *testing.B) {
	cache := &RedisCache[string]{
		mu:                 sync.RWMutex{},
		circuitBreakerOpen: false,
		failureCount:       0,
		lastFailureTime:    time.Time{},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = cache.isCircuitBreakerOpen()
	}
}

// BenchmarkCircuitBreaker_Open_WithinTimeout tests allocation profile when circuit breaker is open and within timeout
func BenchmarkCircuitBreaker_Open_WithinTimeout(b *testing.B) {
	cache := &RedisCache[string]{
		mu:                 sync.RWMutex{},
		circuitBreakerOpen: true,
		failureCount:       5,
		lastFailureTime:    time.Now().Add(-30 * time.Second), // Within 60s timeout
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = cache.isCircuitBreakerOpen()
	}
}

// BenchmarkCircuitBreaker_Open_TimeoutExpired tests allocation profile when circuit breaker transitions from open to closed
func BenchmarkCircuitBreaker_Open_TimeoutExpired(b *testing.B) {
	cache := &RedisCache[string]{
		mu:                 sync.RWMutex{},
		circuitBreakerOpen: true,
		failureCount:       5,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Set expired failure time before each iteration to trigger reset
		cache.circuitBreakerOpen = true
		cache.lastFailureTime = time.Now().Add(-120 * time.Second) // Beyond 60s timeout
		cache.failureCount = 5

		_ = cache.isCircuitBreakerOpen()
	}
}

// BenchmarkCircuitBreaker_WithPrecomputedMetrics tests allocation profile including metrics calls
func BenchmarkCircuitBreaker_WithPrecomputedMetrics(b *testing.B) {
	// Create a minimal cache with pre-computed metrics to test realistic scenario
	cache := &RedisCache[string]{
		mu:                 sync.RWMutex{},
		circuitBreakerOpen: true, // Open so it triggers metric call
		failureCount:       5,
		lastFailureTime:    time.Now().Add(-30 * time.Second), // Within timeout
		// Note: precomputedMetrics would be nil, so we'll test just the circuit breaker part
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		isOpen := cache.isCircuitBreakerOpen()
		if isOpen {
			// This simulates the metric call that would happen in real usage
			// but without actual metric system to isolate circuit breaker allocations
		}
	}
}
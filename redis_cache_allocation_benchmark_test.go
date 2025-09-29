//go:build integration

package cache_test

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// ==============================================================================
// ALLOCATION TRACKING BENCHMARKS - Task 1.1
// ==============================================================================
//
// These benchmarks are specifically designed to measure memory allocation overhead
// for core cache operations, establishing baseline metrics for optimization tracking.
//
// Each benchmark uses b.ReportAllocs() to capture:
// - Number of allocations per operation (allocs/op)
// - Bytes allocated per operation (B/op)
//
// Target baseline metrics from ALLOCATION_OPTIMIZATION_PLAN.md:
// - Has():     28 allocations per call
// - Delete():  51 allocations per call
// - Get():     TBD allocations per call
// - Set():     TBD allocations per call
// - GetOrSet(): TBD allocations per call
//
// ==============================================================================

// BenchmarkRedisCache_Has_Allocations measures memory allocations for Has() operation
func BenchmarkRedisCache_Has_Allocations(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data for consistent Has() behavior
	testData := generateBenchmarkData1KB("allocation:has:test")
	err := cache.Set(ctx, testData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	// Enable allocation tracking
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = cache.Has(ctx, "allocation:has:test")
	}
}

// BenchmarkRedisCache_Has_Allocations_Miss measures allocations when key doesn't exist
func BenchmarkRedisCache_Has_Allocations_Miss(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Enable allocation tracking
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = cache.Has(ctx, fmt.Sprintf("allocation:has:miss:%d", i))
	}
}

// BenchmarkRedisCache_Get_Allocations measures memory allocations for Get() operation
func BenchmarkRedisCache_Get_Allocations(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	testData := generateBenchmarkData1KB("allocation:get:test")
	err := cache.Set(ctx, testData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	// Enable allocation tracking
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, err := cache.Get(ctx, "allocation:get:test")
		if err != nil {
			b.Errorf("GET error: %v", err)
		}
	}
}

// BenchmarkRedisCache_Get_Allocations_Miss measures allocations for cache misses
func BenchmarkRedisCache_Get_Allocations_Miss(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Enable allocation tracking
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, err := cache.Get(ctx, fmt.Sprintf("allocation:get:miss:%d", i))
		if err != nil {
			b.Errorf("GET error: %v", err)
		}
	}
}

// BenchmarkRedisCache_Set_Allocations measures memory allocations for Set() operation
func BenchmarkRedisCache_Set_Allocations(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Enable allocation tracking
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testData := generateBenchmarkData1KB(fmt.Sprintf("allocation:set:%d", i))
		err := cache.Set(ctx, testData, 10*time.Minute)
		if err != nil {
			b.Errorf("SET error: %v", err)
		}
	}
}

// BenchmarkRedisCache_Set_Allocations_LargeData measures allocations with 10KB data
func BenchmarkRedisCache_Set_Allocations_LargeData(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Enable allocation tracking
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testData := generateBenchmarkData10KB(fmt.Sprintf("allocation:set:large:%d", i))
		err := cache.Set(ctx, testData, 10*time.Minute)
		if err != nil {
			b.Errorf("SET error: %v", err)
		}
	}
}

// BenchmarkRedisCache_Delete_Allocations measures memory allocations for Delete() operation
func BenchmarkRedisCache_Delete_Allocations(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data for deletion
	for i := 0; i < b.N; i++ {
		testData := generateBenchmarkData1KB(fmt.Sprintf("allocation:delete:%d", i))
		err := cache.Set(ctx, testData, 10*time.Minute)
		if err != nil {
			b.Fatalf("Failed to pre-populate cache: %v", err)
		}
	}

	// Enable allocation tracking
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := cache.Delete(ctx, fmt.Sprintf("allocation:delete:%d", i))
		if err != nil {
			b.Errorf("DELETE error: %v", err)
		}
	}
}

// BenchmarkRedisCache_Delete_Allocations_Missing measures allocations when deleting non-existent keys
func BenchmarkRedisCache_Delete_Allocations_Missing(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Enable allocation tracking
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := cache.Delete(ctx, fmt.Sprintf("allocation:delete:missing:%d", i))
		if err != nil {
			b.Errorf("DELETE error: %v", err)
		}
	}
}

// BenchmarkRedisCache_GetOrSet_Allocations_CacheMiss measures allocations for GetOrSet cache miss
func BenchmarkRedisCache_GetOrSet_Allocations_CacheMiss(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Loader function for GetOrSet
	loader := func(ctx context.Context) (benchmarkData, error) {
		return generateBenchmarkData1KB("allocation:getorset:loaded"), nil
	}

	// Enable allocation tracking
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := cache.GetOrSet(ctx, fmt.Sprintf("allocation:getorset:miss:%d", i), loader, 10*time.Minute)
		if err != nil {
			b.Errorf("GETORSET error: %v", err)
		}
	}
}

// BenchmarkRedisCache_GetOrSet_Allocations_CacheHit measures allocations for GetOrSet cache hit
func BenchmarkRedisCache_GetOrSet_Allocations_CacheHit(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	testData := generateBenchmarkData1KB("allocation:getorset:hit")
	err := cache.Set(ctx, testData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	// Loader function for GetOrSet (should not be called)
	loader := func(ctx context.Context) (benchmarkData, error) {
		return generateBenchmarkData1KB("allocation:getorset:loaded"), nil
	}

	// Enable allocation tracking
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := cache.GetOrSet(ctx, "allocation:getorset:hit", loader, 10*time.Minute)
		if err != nil {
			b.Errorf("GETORSET error: %v", err)
		}
	}
}

// ==============================================================================
// COMPREHENSIVE ALLOCATION PROFILES
// ==============================================================================

// BenchmarkRedisCache_AllOperations_AllocationProfile provides a combined view
// of allocation patterns across all core operations for baseline comparison
func BenchmarkRedisCache_AllOperations_AllocationProfile(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate some data for operations that need it
	for i := 0; i < 100; i++ {
		testData := generateBenchmarkData1KB(fmt.Sprintf("allocation:profile:existing:%d", i))
		cache.Set(ctx, testData, 10*time.Minute)
	}

	// Loader function for GetOrSet
	loader := func(ctx context.Context) (benchmarkData, error) {
		return generateBenchmarkData1KB("allocation:profile:loaded"), nil
	}

	// Enable allocation tracking
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("allocation:profile:%d", i)
		
		// Cycle through operations to get a representative allocation profile
		switch i % 5 {
		case 0: // Has operation
			_ = cache.Has(ctx, key)
		case 1: // Get operation  
			_, _, _ = cache.Get(ctx, key)
		case 2: // Set operation
			testData := generateBenchmarkData1KB(key)
			_ = cache.Set(ctx, testData, 10*time.Minute)
		case 3: // Delete operation
			_, _ = cache.Delete(ctx, key)
		case 4: // GetOrSet operation
			_, _ = cache.GetOrSet(ctx, key, loader, 10*time.Minute)
		}
	}
}

// ==============================================================================
// CIRCUIT BREAKER ALLOCATION IMPACT 
// ==============================================================================

// BenchmarkRedisCache_Has_Allocations_CircuitBreakerCheck measures the allocation
// overhead specifically from circuit breaker checks in the Has() method
func BenchmarkRedisCache_Has_Allocations_CircuitBreakerCheck(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	testData := generateBenchmarkData1KB("allocation:cb:test")
	err := cache.Set(ctx, testData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	// Enable allocation tracking
	b.ReportAllocs() 
	b.ResetTimer()

	// This benchmark helps isolate circuit breaker allocation overhead
	// by running the same key repeatedly (circuit breaker should remain closed)
	for i := 0; i < b.N; i++ {
		_ = cache.Has(ctx, "allocation:cb:test")
	}
}

// ==============================================================================
// KEY BUILDING ALLOCATION OVERHEAD
// ==============================================================================

// BenchmarkRedisCache_KeyBuilding_Allocations measures allocation overhead from
// buildDataKey() method - this helps isolate string construction costs
func BenchmarkRedisCache_KeyBuilding_Allocations(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Enable allocation tracking
	b.ReportAllocs()
	b.ResetTimer()

	// Use varying key lengths to test string building allocation patterns
	for i := 0; i < b.N; i++ {
		keyLength := (i % 10) + 1 // Keys from length 1 to 10
		key := fmt.Sprintf("allocation:keybuild:%0*d", keyLength, i)
		_ = cache.Has(ctx, key)
	}
}
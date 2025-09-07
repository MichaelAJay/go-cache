//go:build integration

package cache_test

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// BenchmarkGetMany_DebugAllocations - Step by step allocation analysis
func BenchmarkGetMany_DebugAllocations(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	benchPrefix := fmt.Sprintf("bench:debug:%d", time.Now().UnixNano())
	keys := make([]string, 10)
	for i := range 10 {
		testData := generateBenchmarkData1KB(fmt.Sprintf("%s:%d", benchPrefix, i))
		err := cache.Set(ctx, testData, 10*time.Minute)
		if err != nil {
			b.Fatalf("Failed to pre-populate cache: %v", err)
		}
		keys[i] = testData.GetID()
	}

	b.ResetTimer()

	// Step 1: Baseline - just return empty map
	b.Run("Step1_EmptyReturn", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			result := make(map[string]any, len(keys))
			_ = result
		}
	})

	// Step 2: Add slice allocations (what we optimized)
	b.Run("Step2_SliceAllocations", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			result := make(map[string]any, len(keys))
			dataKeys := make([]string, len(keys))
			metaKeys := make([]string, len(keys))
			dataResults := make([]any, len(keys))
			_ = result
			_ = dataKeys
			_ = metaKeys  
			_ = dataResults
		}
	})

	// Step 3: Add string building (what we optimized)  
	b.Run("Step3_StringBuilding", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			result := make(map[string]any, len(keys))
			dataKeys := make([]string, len(keys))
			metaKeys := make([]string, len(keys))
			dataResults := make([]any, len(keys))
			
			// Simulate string building  
			for j, key := range keys {
				dataKeys[j] = "cache:data:" + key
				metaKeys[j] = "cache:meta:" + key
			}
			
			_ = result
			_ = dataKeys
			_ = metaKeys  
			_ = dataResults
		}
	})

	// Step 4: Add Redis operations (biggest unknown)
	b.Run("Step4_RedisOperations", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := cache.GetMany(ctx, keys)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
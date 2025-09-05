//go:build integration

package cache_test

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// ==============================================================================
// BASIC OPERATIONS BENCHMARKS
// ==============================================================================

// BenchmarkRedisCache_Get_1KB tests GET performance with 1KB data
func BenchmarkRedisCache_Get_1KB(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	testData := generateBenchmarkData1KB("bench:test:1kb")
	err := cache.Set(ctx, testData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, err := cache.Get(ctx, "bench:test:1kb")
			if err != nil {
				b.Errorf("GET error: %v", err)
			}
		}
	})
}

// BenchmarkRedisCache_Get_10KB tests GET performance with 10KB data
func BenchmarkRedisCache_Get_10KB(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	testData := generateBenchmarkData10KB("bench:test:10kb")
	err := cache.Set(ctx, testData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, err := cache.Get(ctx, "bench:test:10kb")
			if err != nil {
				b.Errorf("GET error: %v", err)
			}
		}
	})
}

// BenchmarkRedisCache_Get_100KB tests GET performance with 100KB data
func BenchmarkRedisCache_Get_100KB(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	testData := generateBenchmarkData100KB("bench:test:100kb")
	err := cache.Set(ctx, testData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, err := cache.Get(ctx, "bench:test:100kb")
			if err != nil {
				b.Errorf("GET error: %v", err)
			}
		}
	})
}

// BenchmarkRedisCache_Set_1KB tests SET performance with 1KB data
func BenchmarkRedisCache_Set_1KB(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			testData := generateBenchmarkData1KB(fmt.Sprintf("bench:set:1kb:%d", i))
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SET error: %v", err)
			}
			i++
		}
	})
}

// BenchmarkRedisCache_Set_10KB tests SET performance with 10KB data
func BenchmarkRedisCache_Set_10KB(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			testData := generateBenchmarkData10KB(fmt.Sprintf("bench:set:10kb:%d", i))
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SET error: %v", err)
			}
			i++
		}
	})
}

// BenchmarkRedisCache_Set_100KB tests SET performance with 100KB data
func BenchmarkRedisCache_Set_100KB(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			testData := generateBenchmarkData100KB(fmt.Sprintf("bench:set:100kb:%d", i))
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SET error: %v", err)
			}
			i++
		}
	})
}

// BenchmarkRedisCache_Delete tests DELETE performance
func BenchmarkRedisCache_Delete(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	for i := 0; i < b.N; i++ {
		testData := generateBenchmarkData1KB(fmt.Sprintf("bench:delete:%d", i))
		err := cache.Set(ctx, testData, 10*time.Minute)
		if err != nil {
			b.Fatalf("Failed to pre-populate cache: %v", err)
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			err := cache.Delete(ctx, fmt.Sprintf("bench:delete:%d", i))
			if err != nil {
				b.Errorf("DELETE error: %v", err)
			}
			i++
		}
	})
}

// BenchmarkRedisCache_Has tests HAS/EXISTS performance
func BenchmarkRedisCache_Has(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	testData := generateBenchmarkData1KB("bench:has:test")
	err := cache.Set(ctx, testData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			exists := cache.Has(ctx, "bench:has:test")
			if !exists {
				b.Error("Expected key to exist")
			}
		}
	})
}

// BenchmarkRedisCache_Clear tests CLEAR performance
func BenchmarkRedisCache_Clear(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	b.ResetTimer()
	for i := range b.N {
		b.StopTimer()
		// Pre-populate cache with test data for each iteration
		for j := range 100 {
			testData := generateBenchmarkData1KB(fmt.Sprintf("bench:clear:%d:%d", i, j))
			cache.Set(ctx, testData, 10*time.Minute)
		}
		b.StartTimer()

		err := cache.Clear(ctx)
		if err != nil {
			b.Errorf("CLEAR error: %v", err)
		}
	}
}

// ==============================================================================
// ATOMIC OPERATIONS BENCHMARKS
// ==============================================================================

// BenchmarkRedisCache_GetOrSet_CacheMiss tests GetOrSet performance when cache miss occurs
func BenchmarkRedisCache_GetOrSet_CacheMiss(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Loader function that generates new data
	loader := func(ctx context.Context) (benchmarkData, error) {
		return generateBenchmarkData1KB("bench:getorset:miss:generated"), nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench:getorset:miss:%d", i)
			_, err := cache.GetOrSet(ctx, key, loader, 10*time.Minute)
			if err != nil {
				b.Errorf("GetOrSet cache miss error: %v", err)
			}
			// Clean up to ensure each iteration is a cache miss
			cache.Delete(ctx, key)
			i++
		}
	})
}

// BenchmarkRedisCache_GetOrSet_CacheHit tests GetOrSet performance when cache hit occurs
func BenchmarkRedisCache_GetOrSet_CacheHit(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	testData := generateBenchmarkData1KB("bench:getorset:hit")
	err := cache.Set(ctx, testData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	// Loader function (should not be called)
	loader := func(ctx context.Context) (benchmarkData, error) {
		return generateBenchmarkData1KB("should:not:be:called"), nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := cache.GetOrSet(ctx, "bench:getorset:hit", loader, 10*time.Minute)
			if err != nil {
				b.Errorf("GetOrSet cache hit error: %v", err)
			}
		}
	})
}

// BenchmarkRedisCache_GetOrSet_HighContention tests GetOrSet with multiple goroutines on same key
func BenchmarkRedisCache_GetOrSet_HighContention(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Loader function that simulates work
	loader := func(ctx context.Context) (benchmarkData, error) {
		// Simulate some work
		time.Sleep(1 * time.Millisecond)
		return generateBenchmarkData1KB("bench:getorset:contention:generated"), nil
	}

	b.ResetTimer()
	b.SetParallelism(100) // High contention with many goroutines
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// All goroutines compete for the same key
			_, err := cache.GetOrSet(ctx, "bench:getorset:contention", loader, 10*time.Minute)
			if err != nil {
				b.Errorf("GetOrSet high contention error: %v", err)
			}
		}
	})
}

// BenchmarkRedisCache_Update_Existing tests Update performance on existing keys
func BenchmarkRedisCache_Update_Existing(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	for i := 0; i < b.N; i++ {
		testData := generateBenchmarkData1KB(fmt.Sprintf("bench:update:existing:%d", i))
		err := cache.Set(ctx, testData, 10*time.Minute)
		if err != nil {
			b.Fatalf("Failed to pre-populate cache: %v", err)
		}
	}

	// Updater function that modifies existing data
	updater := func(old benchmarkData, exists bool) (benchmarkData, error) {
		if !exists {
			return old, fmt.Errorf("key should exist")
		}
		old.Content = "UPDATED: " + old.Content[:100] // Truncate to avoid growing too large
		return old, nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench:update:existing:%d", i%b.N)
			_, err := cache.Update(ctx, key, updater, 10*time.Minute)
			if err != nil {
				b.Errorf("Update existing error: %v", err)
			}
			i++
		}
	})
}

// BenchmarkRedisCache_Update_NonExistent tests Update performance on non-existent keys
func BenchmarkRedisCache_Update_NonExistent(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Updater function that creates new data if key doesn't exist
	updater := func(old benchmarkData, exists bool) (benchmarkData, error) {
		if exists {
			return old, fmt.Errorf("key should not exist")
		}
		return generateBenchmarkData1KB("bench:update:nonexistent:created"), nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench:update:nonexistent:%d", i)
			_, err := cache.Update(ctx, key, updater, 10*time.Minute)
			if err != nil {
				b.Errorf("Update non-existent error: %v", err)
			}
			// Clean up to ensure each iteration deals with non-existent key
			cache.Delete(ctx, key)
			i++
		}
	})
}

// BenchmarkRedisCache_Update_HighContention tests Update with multiple goroutines on same key
func BenchmarkRedisCache_Update_HighContention(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with initial data
	initialData := generateBenchmarkData1KB("bench:update:contention")
	err := cache.Set(ctx, initialData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	// Updater function that increments a counter in the data
	updater := func(old benchmarkData, exists bool) (benchmarkData, error) {
		if !exists {
			// Create new if doesn't exist
			return generateBenchmarkData1KB("bench:update:contention:created"), nil
		}
		// Simulate updating existing data
		old.Data["counter"] = fmt.Sprintf("%d", time.Now().UnixNano())
		return old, nil
	}

	b.ResetTimer()
	b.SetParallelism(100) // High contention with many goroutines
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// All goroutines compete to update the same key
			_, err := cache.Update(ctx, "bench:update:contention", updater, 10*time.Minute)
			if err != nil {
				b.Errorf("Update high contention error: %v", err)
			}
		}
	})
}

// ==============================================================================
// CONDITIONAL OPERATIONS BENCHMARKS
// ==============================================================================

// BenchmarkRedisCache_SetIfNotExists_NewKey tests SetIfNotExists performance on new/non-existent keys
func BenchmarkRedisCache_SetIfNotExists_NewKey(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			testData := generateBenchmarkData1KB(fmt.Sprintf("bench:setifnotexists:new:%d", i))
			wasSet, err := cache.SetIfNotExists(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SetIfNotExists new key error: %v", err)
			}
			if !wasSet {
				b.Errorf("SetIfNotExists should return true for new key")
			}
			// Clean up to ensure next iteration is also a new key
			cache.Delete(ctx, testData.ID)
			i++
		}
	})
}

// BenchmarkRedisCache_SetIfNotExists_ExistingKey tests SetIfNotExists performance when key already exists
func BenchmarkRedisCache_SetIfNotExists_ExistingKey(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with existing data
	existingData := generateBenchmarkData1KB("bench:setifnotexists:existing")
	err := cache.Set(ctx, existingData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Try to set with different data but same key
			testData := generateBenchmarkData1KB("bench:setifnotexists:existing")
			testData.Content = "DIFFERENT_CONTENT"
			wasSet, err := cache.SetIfNotExists(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SetIfNotExists existing key error: %v", err)
			}
			if wasSet {
				b.Errorf("SetIfNotExists should return false for existing key")
			}
		}
	})
}

// BenchmarkRedisCache_SetIfExists_ExistingKey tests SetIfExists performance when key exists
func BenchmarkRedisCache_SetIfExists_ExistingKey(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	for i := 0; i < b.N; i++ {
		testData := generateBenchmarkData1KB(fmt.Sprintf("bench:setifexists:existing:%d", i))
		err := cache.Set(ctx, testData, 10*time.Minute)
		if err != nil {
			b.Fatalf("Failed to pre-populate cache: %v", err)
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Update existing key with new data
			testData := generateBenchmarkData1KB(fmt.Sprintf("bench:setifexists:existing:%d", i%b.N))
			testData.Content = "UPDATED_CONTENT"
			wasSet, err := cache.SetIfExists(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SetIfExists existing key error: %v", err)
			}
			if !wasSet {
				b.Errorf("SetIfExists should return true for existing key")
			}
			i++
		}
	})
}

// BenchmarkRedisCache_SetIfExists_NonExistentKey tests SetIfExists performance on non-existent keys
func BenchmarkRedisCache_SetIfExists_NonExistentKey(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			testData := generateBenchmarkData1KB(fmt.Sprintf("bench:setifexists:nonexistent:%d", i))
			wasSet, err := cache.SetIfExists(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SetIfExists non-existent key error: %v", err)
			}
			if wasSet {
				b.Errorf("SetIfExists should return false for non-existent key")
			}
			i++
		}
	})
}

// BenchmarkRedisCache_SetIfNotExists_HighContention tests SetIfNotExists with many goroutines competing for same key
func BenchmarkRedisCache_SetIfNotExists_HighContention(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Ensure the key doesn't exist initially
	cache.Delete(ctx, "bench:setifnotexists:contention")

	b.ResetTimer()
	b.SetParallelism(100) // High contention with many goroutines
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			testData := generateBenchmarkData1KB("bench:setifnotexists:contention")
			// Only one goroutine should succeed in setting the key
			_, err := cache.SetIfNotExists(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SetIfNotExists high contention error: %v", err)
			}
			// In high contention, wasSet can be either true or false
			// Both are valid outcomes, so we don't assert on the return value
		}
	})
}

// BenchmarkRedisCache_SetIfExists_HighContention tests SetIfExists with many goroutines updating same existing key
func BenchmarkRedisCache_SetIfExists_HighContention(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate with initial data
	initialData := generateBenchmarkData1KB("bench:setifexists:contention")
	err := cache.Set(ctx, initialData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	b.ResetTimer()
	b.SetParallelism(100) // High contention with many goroutines
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// All goroutines compete to update the same existing key
			testData := generateBenchmarkData1KB("bench:setifexists:contention")
			testData.Content = fmt.Sprintf("UPDATED_%d", time.Now().UnixNano())
			wasSet, err := cache.SetIfExists(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SetIfExists high contention error: %v", err)
			}
			if !wasSet {
				b.Errorf("SetIfExists should return true for existing key in high contention")
			}
		}
	})
}
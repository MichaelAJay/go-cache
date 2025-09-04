package cache_test

import (
	"context"
	"fmt"
	"testing"
	"time"
)

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

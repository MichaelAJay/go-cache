package cache_test

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// BenchmarkRedisCache_GetMany_10Keys tests GetMany performance with 10 keys
func BenchmarkRedisCache_GetMany_10Keys(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	keys := make([]string, 10)
	for i := 0; i < 10; i++ {
		testData := generateBenchmarkData1KB(fmt.Sprintf("bench:getmany:10:%d", i))
		err := cache.Set(ctx, testData, 10*time.Minute)
		if err != nil {
			b.Fatalf("Failed to pre-populate cache: %v", err)
		}
		keys[i] = testData.GetID()
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			results, err := cache.GetMany(ctx, keys)
			if err != nil {
				b.Errorf("GetMany error: %v", err)
			}
			if len(results) != 10 {
				b.Errorf("Expected 10 results, got %d", len(results))
			}
		}
	})
}

// BenchmarkRedisCache_GetMany_100Keys tests GetMany performance with 100 keys
func BenchmarkRedisCache_GetMany_100Keys(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	keys := make([]string, 100)
	for i := 0; i < 100; i++ {
		testData := generateBenchmarkData1KB(fmt.Sprintf("bench:getmany:100:%d", i))
		err := cache.Set(ctx, testData, 10*time.Minute)
		if err != nil {
			b.Fatalf("Failed to pre-populate cache: %v", err)
		}
		keys[i] = testData.GetID()
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			results, err := cache.GetMany(ctx, keys)
			if err != nil {
				b.Errorf("GetMany error: %v", err)
			}
			if len(results) != 100 {
				b.Errorf("Expected 100 results, got %d", len(results))
			}
		}
	})
}

// BenchmarkRedisCache_GetMany_1000Keys tests GetMany performance with 1000 keys
func BenchmarkRedisCache_GetMany_1000Keys(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	keys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		testData := generateBenchmarkData1KB(fmt.Sprintf("bench:getmany:1000:%d", i))
		err := cache.Set(ctx, testData, 10*time.Minute)
		if err != nil {
			b.Fatalf("Failed to pre-populate cache: %v", err)
		}
		keys[i] = testData.GetID()
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			results, err := cache.GetMany(ctx, keys)
			if err != nil {
				b.Errorf("GetMany error: %v", err)
			}
			if len(results) != 1000 {
				b.Errorf("Expected 1000 results, got %d", len(results))
			}
		}
	})
}

// BenchmarkRedisCache_SetMany_10Values tests SetMany performance with 10 values
func BenchmarkRedisCache_SetMany_10Values(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Create test data for batch
			values := make([]benchmarkData, 10)
			for j := 0; j < 10; j++ {
				values[j] = generateBenchmarkData1KB(fmt.Sprintf("bench:setmany:10:%d:%d", i, j))
			}

			err := cache.SetMany(ctx, values, 10*time.Minute)
			if err != nil {
				b.Errorf("SetMany error: %v", err)
			}
			i++
		}
	})
}

// BenchmarkRedisCache_SetMany_100Values tests SetMany performance with 100 values
func BenchmarkRedisCache_SetMany_100Values(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Create test data for batch
			values := make([]benchmarkData, 100)
			for j := 0; j < 100; j++ {
				values[j] = generateBenchmarkData1KB(fmt.Sprintf("bench:setmany:100:%d:%d", i, j))
			}

			err := cache.SetMany(ctx, values, 10*time.Minute)
			if err != nil {
				b.Errorf("SetMany error: %v", err)
			}
			i++
		}
	})
}

// BenchmarkRedisCache_DeleteMany_100Keys tests DeleteMany performance with 100 keys
func BenchmarkRedisCache_DeleteMany_100Keys(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	b.ResetTimer()
	for i := range b.N {
		b.StopTimer()
		// Pre-populate cache with test data for each iteration
		keys := make([]string, 100)
		for j := 0; j < 100; j++ {
			testData := generateBenchmarkData1KB(fmt.Sprintf("bench:deletemany:100:%d:%d", i, j))
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Fatalf("Failed to pre-populate cache: %v", err)
			}
			keys[j] = testData.GetID()
		}
		b.StartTimer()

		err := cache.DeleteMany(ctx, keys)
		if err != nil {
			b.Errorf("DeleteMany error: %v", err)
		}
	}
}

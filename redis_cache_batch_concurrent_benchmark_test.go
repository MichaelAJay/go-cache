//go:build integration

package cache_test

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// ==============================================================================
// BATCH OPERATIONS BENCHMARKS
// ==============================================================================

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

// ==============================================================================
// CONCURRENT OPERATIONS BENCHMARKS
// ==============================================================================

// BenchmarkRedisCache_Get_Concurrent_10 tests GET performance with 10 concurrent goroutines
func BenchmarkRedisCache_Get_Concurrent_10(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	testData := generateBenchmarkData1KB("bench:concurrent:get:10")
	err := cache.Set(ctx, testData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	b.ResetTimer()
	b.SetParallelism(10)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, err := cache.Get(ctx, "bench:concurrent:get:10")
			if err != nil {
				b.Errorf("GET error: %v", err)
			}
		}
	})
}

// BenchmarkRedisCache_Get_Concurrent_100 tests GET performance with 100 concurrent goroutines
func BenchmarkRedisCache_Get_Concurrent_100(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	testData := generateBenchmarkData1KB("bench:concurrent:get:100")
	err := cache.Set(ctx, testData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	b.ResetTimer()
	b.SetParallelism(100)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, err := cache.Get(ctx, "bench:concurrent:get:100")
			if err != nil {
				b.Errorf("GET error: %v", err)
			}
		}
	})
}

// BenchmarkRedisCache_Get_Concurrent_1000 tests GET performance with 1000 concurrent goroutines
func BenchmarkRedisCache_Get_Concurrent_1000(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	testData := generateBenchmarkData1KB("bench:concurrent:get:1000")
	err := cache.Set(ctx, testData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	b.ResetTimer()
	b.SetParallelism(1000)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, err := cache.Get(ctx, "bench:concurrent:get:1000")
			if err != nil {
				b.Errorf("GET error: %v", err)
			}
		}
	})
}

// BenchmarkRedisCache_Set_Concurrent_10 tests SET performance with 10 concurrent goroutines
func BenchmarkRedisCache_Set_Concurrent_10(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	b.ResetTimer()
	b.SetParallelism(10)
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			testData := generateBenchmarkData1KB(fmt.Sprintf("bench:concurrent:set:10:%d", i))
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SET error: %v", err)
			}
			i++
		}
	})
}

// BenchmarkRedisCache_Set_Concurrent_100 tests SET performance with 100 concurrent goroutines
func BenchmarkRedisCache_Set_Concurrent_100(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	b.ResetTimer()
	b.SetParallelism(100)
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			testData := generateBenchmarkData1KB(fmt.Sprintf("bench:concurrent:set:100:%d", i))
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SET error: %v", err)
			}
			i++
		}
	})
}

// BenchmarkRedisCache_Mixed_Operations_Concurrent tests mixed operations under high concurrency
// Simulates realistic workload: 70% reads, 20% writes, 10% deletes
func BenchmarkRedisCache_Mixed_Operations_Concurrent(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with initial data
	for i := range 1000 {
		testData := generateBenchmarkData1KB(fmt.Sprintf("bench:mixed:initial:%d", i))
		cache.Set(ctx, testData, 10*time.Minute)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			operation := i % 10

			switch {
			case operation < 7: // 70% reads
				keyIndex := i % 1000
				_, _, err := cache.Get(ctx, fmt.Sprintf("bench:mixed:initial:%d", keyIndex))
				if err != nil {
					b.Errorf("Mixed GET error: %v", err)
				}

			case operation < 9: // 20% writes
				testData := generateBenchmarkData1KB(fmt.Sprintf("bench:mixed:write:%d", i))
				err := cache.Set(ctx, testData, 10*time.Minute)
				if err != nil {
					b.Errorf("Mixed SET error: %v", err)
				}

			default: // 10% deletes
				keyIndex := i % 1000
				err := cache.Delete(ctx, fmt.Sprintf("bench:mixed:initial:%d", keyIndex))
				if err != nil {
					b.Errorf("Mixed DELETE error: %v", err)
				}
				// Replenish deleted data to maintain dataset size
				testData := generateBenchmarkData1KB(fmt.Sprintf("bench:mixed:initial:%d", keyIndex))
				cache.Set(ctx, testData, 10*time.Minute)
			}

			i++
		}
	})
}
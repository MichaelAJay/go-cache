package cache_test

import (
	"context"
	"fmt"
	"testing"
	"time"
)

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

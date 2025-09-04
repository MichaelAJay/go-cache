package cache_test

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"
)

// BenchmarkRedisCache_MemoryAllocations tests memory allocation patterns during cache operations
func BenchmarkRedisCache_MemoryAllocations(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Force GC and get baseline memory stats
	runtime.GC()
	runtime.GC() // Call twice to ensure clean state
	var memStatsBefore, memStatsAfter runtime.MemStats
	runtime.ReadMemStats(&memStatsBefore)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Perform typical cache operations that should allocate memory
		testData := generateBenchmarkData1KB(fmt.Sprintf("bench:alloc:%d", i))
		
		// Set operation
		err := cache.Set(ctx, testData, 5*time.Minute)
		if err != nil {
			b.Errorf("SET error: %v", err)
		}

		// Get operation
		_, _, err = cache.Get(ctx, testData.GetID())
		if err != nil {
			b.Errorf("GET error: %v", err)
		}

		// Delete operation to clean up
		err = cache.Delete(ctx, testData.GetID())
		if err != nil {
			b.Errorf("DELETE error: %v", err)
		}
	}

	b.StopTimer()
	
	// Measure memory after operations
	runtime.GC()
	runtime.GC()
	runtime.ReadMemStats(&memStatsAfter)
	
	// Report memory statistics
	allocsDiff := memStatsAfter.TotalAlloc - memStatsBefore.TotalAlloc
	b.ReportMetric(float64(allocsDiff)/float64(b.N), "allocs/op")
	b.ReportMetric(float64(memStatsAfter.Mallocs - memStatsBefore.Mallocs)/float64(b.N), "mallocs/op")
}

// BenchmarkRedisCache_GCPressure tests garbage collection pressure under sustained load
func BenchmarkRedisCache_GCPressure(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Get baseline GC stats
	var gcStatsBefore, gcStatsAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&gcStatsBefore)

	b.ResetTimer()

	// Perform sustained operations that create garbage
	for i := 0; i < b.N; i++ {
		// Create multiple objects of different sizes to stress GC
		testData1KB := generateBenchmarkData1KB(fmt.Sprintf("bench:gc:1kb:%d", i))
		testData10KB := generateBenchmarkData10KB(fmt.Sprintf("bench:gc:10kb:%d", i))
		
		// Set operations (creates serialization overhead)
		err := cache.Set(ctx, testData1KB, 1*time.Minute)
		if err != nil {
			b.Errorf("SET 1KB error: %v", err)
		}
		
		err = cache.Set(ctx, testData10KB, 1*time.Minute)
		if err != nil {
			b.Errorf("SET 10KB error: %v", err)
		}
		
		// Get operations (creates deserialization overhead)
		_, _, err = cache.Get(ctx, testData1KB.GetID())
		if err != nil {
			b.Errorf("GET 1KB error: %v", err)
		}
		
		_, _, err = cache.Get(ctx, testData10KB.GetID())
		if err != nil {
			b.Errorf("GET 10KB error: %v", err)
		}

		// Periodically clean up to avoid memory exhaustion
		if i%100 == 0 {
			_ = cache.Delete(ctx, testData1KB.GetID())
			_ = cache.Delete(ctx, testData10KB.GetID())
		}
	}

	b.StopTimer()

	// Measure GC impact
	runtime.ReadMemStats(&gcStatsAfter)
	
	gcCycles := gcStatsAfter.NumGC - gcStatsBefore.NumGC
	gcPauseTotal := gcStatsAfter.PauseTotalNs - gcStatsBefore.PauseTotalNs
	
	b.ReportMetric(float64(gcCycles), "gc-cycles")
	b.ReportMetric(float64(gcPauseTotal)/float64(time.Millisecond), "gc-pause-ms")
	
	if gcCycles > 0 {
		avgGCPause := float64(gcPauseTotal) / float64(gcCycles) / float64(time.Microsecond)
		b.ReportMetric(avgGCPause, "avg-gc-pause-μs")
	}
}

// BenchmarkRedisCache_ConnectionPooling tests Redis connection pool efficiency under concurrent load
func BenchmarkRedisCache_ConnectionPooling(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate some data for get operations
	for i := range 10 {
		testData := generateBenchmarkData1KB(fmt.Sprintf("bench:pool:preload:%d", i))
		err := cache.Set(ctx, testData, 10*time.Minute)
		if err != nil {
			b.Fatalf("Failed to pre-populate cache: %v", err)
		}
	}

	b.ResetTimer()

	// Run with high concurrency to test connection pooling
	b.SetParallelism(50) // Higher concurrency to stress connection pool
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Mix of read and write operations to utilize connections
			switch i % 4 {
			case 0:
				// GET operation
				_, _, err := cache.Get(ctx, fmt.Sprintf("bench:pool:preload:%d", i%10))
				if err != nil {
					b.Errorf("GET error: %v", err)
				}
			case 1:
				// SET operation
				testData := generateBenchmarkData1KB(fmt.Sprintf("bench:pool:set:%d", i))
				err := cache.Set(ctx, testData, 1*time.Minute)
				if err != nil {
					b.Errorf("SET error: %v", err)
				}
			case 2:
				// HAS operation
				exists := cache.Has(ctx, fmt.Sprintf("bench:pool:preload:%d", i%10))
				if !exists {
					// This is expected for some keys, don't error
				}
			case 3:
				// DELETE operation (cleanup some keys we set)
				if i > 10 {
					_ = cache.Delete(ctx, fmt.Sprintf("bench:pool:set:%d", i-10))
				}
			}
			i++
		}
	})
}

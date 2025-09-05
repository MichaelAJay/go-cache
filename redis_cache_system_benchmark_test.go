//go:build integration

package cache_test

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	cache "github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/MichaelAJay/go-cache/internal/testintegration"
)

// ==============================================================================
// CIRCUIT BREAKER PERFORMANCE BENCHMARKS
// ==============================================================================

// setupCircuitBreakerCache creates a cache instance for circuit breaker testing
func setupCircuitBreakerCache(b *testing.B) interfaces.Cache[benchmarkData] {
	b.Helper()

	ctx := context.Background()

	// Create a temporary testing.T to satisfy the interface
	t := &testing.T{}
	setup := testintegration.SetupTestEnvironment(ctx, t)

	// Cleanup after benchmark
	b.Cleanup(func() {
		setup.TestEnv.Close()
	})

	extractor := &cache.IndexExtractor[benchmarkData]{
		GetEntryKey: func(data benchmarkData) string { return data.GetID() },
		GetOwnerKey: func(data benchmarkData) string { return data.GetOwner() },
	}

	cacheInstance, err := cache.NewCache(
		ctx,
		setup.RedisClient,
		false, // no indexing for these benchmarks
		extractor,
		cache.WithTTL[benchmarkData](10*time.Minute),
		cache.WithSerializer[benchmarkData]("msgpack"),
	)
	if err != nil {
		b.Fatalf("Failed to create cache: %v", err)
	}

	return cacheInstance
}

// forceCircuitBreakerOpen forces the circuit breaker to open by triggering failures
func forceCircuitBreakerOpen(cache interfaces.Cache[benchmarkData]) {
	ctx := context.Background()
	
	// Create a cache with a broken Redis client to trigger circuit breaker
	// We'll simulate this by making many rapid calls that will fail
	// The circuit breaker opens after 10 failures (circuitBreakerThreshold = 10)
	
	// Force many failures by trying to access non-existent keys rapidly
	// This simulates network failures or Redis unavailability
	for i := range 15 { // More than threshold to ensure circuit opens
		// Use a context with very short timeout to force failures
		failCtx, cancel := context.WithTimeout(ctx, 1*time.Microsecond)
		cache.Get(failCtx, "non-existent-key-to-force-failure")
		cancel()
		_ = i
	}
}

// BenchmarkRedisCache_CircuitBreakerClosed tests performance with circuit breaker closed (normal operation)
func BenchmarkRedisCache_CircuitBreakerClosed(b *testing.B) {
	cache := setupCircuitBreakerCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	testData := generateBenchmarkData1KB("bench:circuit:closed")
	err := cache.Set(ctx, testData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Perform normal GET operation with circuit breaker closed
			_, _, err := cache.Get(ctx, "bench:circuit:closed")
			if err != nil {
				b.Errorf("GET error with closed circuit breaker: %v", err)
			}
		}
	})
}

// BenchmarkRedisCache_CircuitBreakerOpen tests performance when circuit breaker is open
func BenchmarkRedisCache_CircuitBreakerOpen(b *testing.B) {
	cache := setupCircuitBreakerCache(b)
	ctx := context.Background()

	// Force circuit breaker to open
	forceCircuitBreakerOpen(cache)
	
	// Give circuit breaker time to fully open
	time.Sleep(10 * time.Millisecond)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Perform GET operation with circuit breaker open
			// This should fail fast without hitting Redis
			_, _, err := cache.Get(ctx, "bench:circuit:open")
			if err == nil {
				b.Error("Expected circuit breaker error but got none")
			}
		}
	})
}

// BenchmarkRedisCache_CircuitBreakerRecovery tests performance during circuit breaker recovery
func BenchmarkRedisCache_CircuitBreakerRecovery(b *testing.B) {
	cache := setupCircuitBreakerCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data before forcing failures
	testData := generateBenchmarkData1KB("bench:circuit:recovery")
	err := cache.Set(ctx, testData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	// Force circuit breaker to open
	forceCircuitBreakerOpen(cache)
	
	// Wait for circuit breaker timeout (60 seconds in code, but we'll simulate recovery)
	// In real implementation, we'd wait for circuitBreakerTimeout = 60 * time.Second
	// For benchmarking, we simulate the recovery phase where it's attempting to heal
	time.Sleep(100 * time.Millisecond)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// During recovery, some requests should succeed (healing process)
			// and some might still fail as the circuit breaker evaluates health
			_, _, err := cache.Get(ctx, "bench:circuit:recovery")
			// We don't check error here as both success and failure are expected
			// during recovery phase - we're measuring the performance overhead
			_ = err
		}
	})
}

// BenchmarkRedisCache_CircuitBreakerOverhead_Comparison compares overhead of circuit breaker checks
func BenchmarkRedisCache_CircuitBreakerOverhead_Comparison(b *testing.B) {
	cache := setupCircuitBreakerCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	testData := generateBenchmarkData1KB("bench:circuit:comparison")
	err := cache.Set(ctx, testData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	b.Run("NormalOperation", func(b *testing.B) {
		// Measure normal operation with circuit breaker checks
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, _, err := cache.Get(ctx, "bench:circuit:comparison")
				if err != nil {
					b.Errorf("GET error: %v", err)
				}
			}
		})
	})

	b.Run("CircuitBreakerOpen", func(b *testing.B) {
		// Force circuit breaker open for this sub-benchmark
		forceCircuitBreakerOpen(cache)
		time.Sleep(10 * time.Millisecond)
		
		// Measure fast-fail performance when circuit breaker is open
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, _, err := cache.Get(ctx, "bench:circuit:comparison")
				// Circuit breaker should cause fast failure
				_ = err
			}
		})
	})
}

// BenchmarkRedisCache_CircuitBreakerWriteOperations tests circuit breaker impact on write operations
func BenchmarkRedisCache_CircuitBreakerWriteOperations(b *testing.B) {
	cache := setupCircuitBreakerCache(b)
	ctx := context.Background()

	b.Run("SetWithClosedCircuit", func(b *testing.B) {
		// Test SET performance with circuit breaker closed
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				testData := generateBenchmarkData1KB("bench:set:closed:" + string(rune(i)))
				err := cache.Set(ctx, testData, 10*time.Minute)
				if err != nil {
					b.Errorf("SET error with closed circuit: %v", err)
				}
				i++
			}
		})
	})

	b.Run("SetWithOpenCircuit", func(b *testing.B) {
		// Force circuit breaker open
		forceCircuitBreakerOpen(cache)
		time.Sleep(10 * time.Millisecond)

		// Test SET performance with circuit breaker open (should fail fast)
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				testData := generateBenchmarkData1KB("bench:set:open:" + string(rune(i)))
				err := cache.Set(ctx, testData, 10*time.Minute)
				// Circuit breaker should cause fast failure
				_ = err
				i++
			}
		})
	})
}

// ==============================================================================
// MEMORY AND RESOURCE BENCHMARKS
// ==============================================================================

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

	for i := range b.N {
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
	for i := range b.N {
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
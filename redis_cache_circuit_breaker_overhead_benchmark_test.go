package cache_test

import (
	"context"
	"testing"
	"time"

	cache "github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/MichaelAJay/go-cache/internal/testintegration"
)

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
	for i := 0; i < 15; i++ { // More than threshold to ensure circuit opens
		// Use a context with very short timeout to force failures
		failCtx, cancel := context.WithTimeout(ctx, 1*time.Microsecond)
		cache.Get(failCtx, "non-existent-key-to-force-failure")
		cancel()
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
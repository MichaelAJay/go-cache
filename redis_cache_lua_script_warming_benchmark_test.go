package cache_test

import (
	"context"
	"testing"
	"time"

	cache "github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/MichaelAJay/go-cache/internal/testintegration"
)

// setupBenchmarkCacheWithScriptWarming creates cache with Lua script warming enabled/disabled
func setupBenchmarkCacheWithScriptWarming(b *testing.B, warmScripts bool) interfaces.Cache[benchmarkData] {
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
		cache.WithWarmLuaScripts[benchmarkData](warmScripts),
	)
	if err != nil {
		b.Fatalf("Failed to create cache: %v", err)
	}

	return cacheInstance
}

// BenchmarkRedisCache_WithScriptWarming tests performance with Lua scripts pre-warmed
func BenchmarkRedisCache_WithScriptWarming(b *testing.B) {
	cache := setupBenchmarkCacheWithScriptWarming(b, true)
	ctx := context.Background()

	testData := generateBenchmarkData1KB("bench:script:warm")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Test a mix of operations that use different Lua scripts
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SET error: %v", err)
			}

			_, _, err = cache.Get(ctx, "bench:script:warm")
			if err != nil {
				b.Errorf("GET error: %v", err)
			}

			err = cache.Delete(ctx, "bench:script:warm")
			if err != nil {
				b.Errorf("DELETE error: %v", err)
			}
		}
	})
}

// BenchmarkRedisCache_WithoutScriptWarming tests performance with scripts loaded on-demand
func BenchmarkRedisCache_WithoutScriptWarming(b *testing.B) {
	cache := setupBenchmarkCacheWithScriptWarming(b, false)
	ctx := context.Background()

	testData := generateBenchmarkData1KB("bench:script:cold")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Test a mix of operations that use different Lua scripts
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SET error: %v", err)
			}

			_, _, err = cache.Get(ctx, "bench:script:cold")
			if err != nil {
				b.Errorf("GET error: %v", err)
			}

			err = cache.Delete(ctx, "bench:script:cold")
			if err != nil {
				b.Errorf("DELETE error: %v", err)
			}
		}
	})
}

// BenchmarkRedisCache_ColdStart_FirstCall measures the overhead of first script execution
func BenchmarkRedisCache_ColdStart_FirstCall(b *testing.B) {
	ctx := context.Background()
	
	// Setup shared test environment once
	t := &testing.T{}
	setup := testintegration.SetupTestEnvironment(ctx, t)
	defer setup.TestEnv.Close()

	extractor := &cache.IndexExtractor[benchmarkData]{
		GetEntryKey: func(data benchmarkData) string { return data.GetID() },
		GetOwnerKey: func(data benchmarkData) string { return data.GetOwner() },
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Create fresh cache without script warming (but reuse Redis client)
		cacheInstance, err := cache.NewCache(
			ctx,
			setup.RedisClient,
			false, // no indexing for these benchmarks
			extractor,
			cache.WithTTL[benchmarkData](10*time.Minute),
			cache.WithSerializer[benchmarkData]("msgpack"),
			cache.WithWarmLuaScripts[benchmarkData](false),
		)
		if err != nil {
			b.Fatalf("Failed to create cache: %v", err)
		}

		testData := generateBenchmarkData1KB("bench:script:coldstart")
		b.StartTimer()

		// Measure time for first operation (which compiles scripts on first use)
		err = cacheInstance.Set(ctx, testData, 10*time.Minute)
		if err != nil {
			b.Errorf("SET error: %v", err)
		}
		
		b.StopTimer()
		cacheInstance.Close()
	}
}


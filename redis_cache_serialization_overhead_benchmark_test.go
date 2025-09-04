package cache_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	cache "github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/MichaelAJay/go-cache/internal/testintegration"
)

// setupBenchmarkCacheWithSerializer creates cache with specific serialization format
func setupBenchmarkCacheWithSerializer(b *testing.B, format string) interfaces.Cache[benchmarkData] {
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
		false, // no indexing for serialization tests
		extractor,
		cache.WithTTL[benchmarkData](10*time.Minute),
		cache.WithSerializer[benchmarkData](format), // specific serialization format
	)
	if err != nil {
		b.Fatalf("Failed to create cache with %s serialization: %v", format, err)
	}

	return cacheInstance
}

// BenchmarkRedisCache_JSON_Serialization tests performance with JSON serialization
func BenchmarkRedisCache_JSON_Serialization(b *testing.B) {
	cache := setupBenchmarkCacheWithSerializer(b, "json")
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Test Set operation with JSON serialization
			testData := generateBenchmarkData1KB(fmt.Sprintf("bench:json:set:%d", i))
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("JSON Set error: %v", err)
			}

			// Test Get operation with JSON deserialization
			_, found, err := cache.Get(ctx, testData.GetID())
			if err != nil {
				b.Errorf("JSON Get error: %v", err)
			}
			if !found {
				b.Errorf("Expected to find key after JSON set")
			}

			i++
		}
	})
}

// BenchmarkRedisCache_Gob_Serialization tests performance with Gob serialization
func BenchmarkRedisCache_Gob_Serialization(b *testing.B) {
	cache := setupBenchmarkCacheWithSerializer(b, "gob")
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Test Set operation with Gob serialization
			testData := generateBenchmarkData1KB(fmt.Sprintf("bench:gob:set:%d", i))
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("Gob Set error: %v", err)
			}

			// Test Get operation with Gob deserialization
			_, found, err := cache.Get(ctx, testData.GetID())
			if err != nil {
				b.Errorf("Gob Get error: %v", err)
			}
			if !found {
				b.Errorf("Expected to find key after Gob set")
			}

			i++
		}
	})
}

// BenchmarkRedisCache_Msgpack_Serialization tests performance with MessagePack serialization
func BenchmarkRedisCache_Msgpack_Serialization(b *testing.B) {
	cache := setupBenchmarkCacheWithSerializer(b, "msgpack")
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Test Set operation with MessagePack serialization
			testData := generateBenchmarkData1KB(fmt.Sprintf("bench:msgpack:set:%d", i))
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("MessagePack Set error: %v", err)
			}

			// Test Get operation with MessagePack deserialization
			_, found, err := cache.Get(ctx, testData.GetID())
			if err != nil {
				b.Errorf("MessagePack Get error: %v", err)
			}
			if !found {
				b.Errorf("Expected to find key after MessagePack set")
			}

			i++
		}
	})
}

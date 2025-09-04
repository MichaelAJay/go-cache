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

// setupBenchmarkCacheWithIndexing creates cache with indexing enabled for comparison benchmarks
func setupBenchmarkCacheWithIndexing(b *testing.B) interfaces.Cache[benchmarkData] {
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
		true, // indexing enabled
		extractor,
		cache.WithTTL[benchmarkData](10*time.Minute),
		cache.WithSerializer[benchmarkData]("msgpack"),
	)
	if err != nil {
		b.Fatalf("Failed to create cache with indexing: %v", err)
	}

	return cacheInstance
}

// BenchmarkRedisCache_Set_WithIndexing tests Set performance with indexing enabled
func BenchmarkRedisCache_Set_WithIndexing(b *testing.B) {
	cache := setupBenchmarkCacheWithIndexing(b)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			testData := generateBenchmarkData1KB(fmt.Sprintf("user%d:session%d", i%10, i))
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("Set error: %v", err)
			}
			i++
		}
	})
}

// BenchmarkRedisCache_Set_WithoutIndexing tests Set performance without indexing
func BenchmarkRedisCache_Set_WithoutIndexing(b *testing.B) {
	cache := setupBenchmarkCache(b) // Uses indexing=false
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			testData := generateBenchmarkData1KB(fmt.Sprintf("user%d:session%d", i%10, i))
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("Set error: %v", err)
			}
			i++
		}
	})
}

// BenchmarkRedisCache_GetByOwner_10Entries tests GetByOwner performance with 10 entries per owner
func BenchmarkRedisCache_GetByOwner_10Entries(b *testing.B) {
	cache := setupBenchmarkCacheWithIndexing(b)
	ctx := context.Background()

	// Pre-populate cache with 10 entries per owner across 100 owners
	for ownerID := 0; ownerID < 100; ownerID++ {
		for entryID := 0; entryID < 10; entryID++ {
			testData := generateBenchmarkData1KB(fmt.Sprintf("user%d:entry%d", ownerID, entryID))
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Fatalf("Failed to pre-populate cache: %v", err)
			}
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ownerID := 0
		for pb.Next() {
			results, err := cache.GetByOwner(ctx, fmt.Sprintf("user%d", ownerID%100))
			if err != nil {
				b.Errorf("GetByOwner error: %v", err)
			}
			if len(results) != 10 {
				b.Errorf("Expected 10 results, got %d", len(results))
			}
			ownerID++
		}
	})
}

// BenchmarkRedisCache_GetByOwner_100Entries tests GetByOwner performance with 100 entries per owner
func BenchmarkRedisCache_GetByOwner_100Entries(b *testing.B) {
	cache := setupBenchmarkCacheWithIndexing(b)
	ctx := context.Background()

	// Pre-populate cache with 100 entries per owner across 10 owners
	for ownerID := 0; ownerID < 10; ownerID++ {
		for entryID := 0; entryID < 100; entryID++ {
			testData := generateBenchmarkData1KB(fmt.Sprintf("user%d:entry%d", ownerID, entryID))
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Fatalf("Failed to pre-populate cache: %v", err)
			}
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ownerID := 0
		for pb.Next() {
			results, err := cache.GetByOwner(ctx, fmt.Sprintf("user%d", ownerID%10))
			if err != nil {
				b.Errorf("GetByOwner error: %v", err)
			}
			if len(results) != 100 {
				b.Errorf("Expected 100 results, got %d", len(results))
			}
			ownerID++
		}
	})
}

// BenchmarkRedisCache_DeleteByOwner_100Entries tests DeleteByOwner performance with 100 entries per owner
func BenchmarkRedisCache_DeleteByOwner_100Entries(b *testing.B) {
	cache := setupBenchmarkCacheWithIndexing(b)
	ctx := context.Background()

	b.ResetTimer()
	for i := range b.N {
		b.StopTimer()
		// Pre-populate cache with 100 entries for this iteration
		ownerKey := fmt.Sprintf("deletebench:user%d", i)
		for entryID := 0; entryID < 100; entryID++ {
			testData := generateBenchmarkData1KB(fmt.Sprintf("%s:entry%d", ownerKey, entryID))
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Fatalf("Failed to pre-populate cache: %v", err)
			}
		}
		b.StartTimer()

		deletedCount, err := cache.DeleteByOwner(ctx, ownerKey)
		if err != nil {
			b.Errorf("DeleteByOwner error: %v", err)
		}
		if deletedCount != 100 {
			b.Errorf("Expected to delete 100 entries, deleted %d", deletedCount)
		}
	}
}

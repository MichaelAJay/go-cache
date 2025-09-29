//go:build integration

package cache_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	cache "github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/MichaelAJay/go-cache/internal/testintegration"
	"github.com/MichaelAJay/go-metrics/metric"
)

// ==============================================================================
// SHARED CACHE INSTANCES FOR FEATURE BENCHMARKS
// ==============================================================================

var (
	sharedIndexingCache     interfaces.Cache[benchmarkData]
	sharedIndexingCacheOnce sync.Once
	
	sharedJSONCache         interfaces.Cache[benchmarkData]
	sharedJSONCacheOnce     sync.Once
	
	sharedGobCache          interfaces.Cache[benchmarkData]
	sharedGobCacheOnce      sync.Once
	
	sharedMsgPackCache      interfaces.Cache[benchmarkData]
	sharedMsgPackCacheOnce  sync.Once
	
	sharedScriptWarmCache   interfaces.Cache[benchmarkData]
	sharedScriptWarmOnce    sync.Once
	
	sharedNoScriptWarmCache interfaces.Cache[benchmarkData]
	sharedNoScriptWarmOnce  sync.Once
)

// getSharedIndexingCache creates and returns a shared cache with indexing enabled
func getSharedIndexingCache() interfaces.Cache[benchmarkData] {
	sharedIndexingCacheOnce.Do(func() {
		ctx := context.Background()
		setup := getSharedBenchmarkSetup()

		extractor := &cache.IndexExtractor[benchmarkData]{
			GetEntryKey: func(data benchmarkData) string { return data.GetID() },
			GetOwnerKey: func(data benchmarkData) string { return data.GetOwner() },
		}

		// Create metrics registry for pre-computed metrics
		registry := metric.NewDefaultRegistry()
		tags := metric.Tags{"environment": "benchmark", "indexing": "enabled"}

		cacheInstance, err := cache.NewCache(
			ctx,
			setup.RedisClient,
			true, // indexing enabled
			extractor,
			0, // no pool warming for baseline benchmarks
			cache.WithTTL[benchmarkData](10*time.Minute),
			cache.WithSerializer[benchmarkData]("msgpack"),
			cache.WithGoMetrics[benchmarkData](registry, tags),
		)
		if err != nil {
			panic("Failed to create shared indexing cache: " + err.Error())
		}
		
		sharedIndexingCache = cacheInstance
	})
	return sharedIndexingCache
}

// getSharedSerializerCache creates and returns a shared cache with specific serializer
func getSharedSerializerCache(format string) interfaces.Cache[benchmarkData] {
	switch format {
	case "json":
		sharedJSONCacheOnce.Do(func() {
			sharedJSONCache = createSerializerCache("json")
		})
		return sharedJSONCache
	case "gob":
		sharedGobCacheOnce.Do(func() {
			sharedGobCache = createSerializerCache("gob")
		})
		return sharedGobCache
	case "msgpack":
		sharedMsgPackCacheOnce.Do(func() {
			sharedMsgPackCache = createSerializerCache("msgpack")
		})
		return sharedMsgPackCache
	default:
		panic("Unsupported serializer format: " + format)
	}
}

// getSharedScriptWarmingCache creates and returns a shared cache with script warming setting
func getSharedScriptWarmingCache(warmScripts bool) interfaces.Cache[benchmarkData] {
	if warmScripts {
		sharedScriptWarmOnce.Do(func() {
			sharedScriptWarmCache = createScriptWarmingCache(true)
		})
		return sharedScriptWarmCache
	} else {
		sharedNoScriptWarmOnce.Do(func() {
			sharedNoScriptWarmCache = createScriptWarmingCache(false)
		})
		return sharedNoScriptWarmCache
	}
}

// Helper to create serializer cache
func createSerializerCache(format string) interfaces.Cache[benchmarkData] {
	ctx := context.Background()
	setup := getSharedBenchmarkSetup()

	extractor := &cache.IndexExtractor[benchmarkData]{
		GetEntryKey: func(data benchmarkData) string { return data.GetID() },
		GetOwnerKey: func(data benchmarkData) string { return data.GetOwner() },
	}

	// Create metrics registry for serializer benchmarks
	registry := metric.NewDefaultRegistry()
	tags := metric.Tags{"environment": "benchmark", "serializer": format}

	cacheInstance, err := cache.NewCache(
		ctx,
		setup.RedisClient,
		false, // no indexing for serializer benchmarks
		extractor,
		0, // no pool warming for baseline benchmarks
		cache.WithTTL[benchmarkData](10*time.Minute),
		cache.WithSerializer[benchmarkData](format),
		cache.WithGoMetrics[benchmarkData](registry, tags),
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to create shared %s cache: %s", format, err.Error()))
	}
	
	return cacheInstance
}

// Helper to create script warming cache
func createScriptWarmingCache(warmScripts bool) interfaces.Cache[benchmarkData] {
	ctx := context.Background()
	setup := getSharedBenchmarkSetup()

	extractor := &cache.IndexExtractor[benchmarkData]{
		GetEntryKey: func(data benchmarkData) string { return data.GetID() },
		GetOwnerKey: func(data benchmarkData) string { return data.GetOwner() },
	}

	// Create metrics registry for script warming benchmarks
	registry := metric.NewDefaultRegistry()
	tags := metric.Tags{"environment": "benchmark", "script_warming": fmt.Sprintf("%v", warmScripts)}

	options := []cache.Option[benchmarkData]{
		cache.WithTTL[benchmarkData](10*time.Minute),
		cache.WithSerializer[benchmarkData]("msgpack"),
		cache.WithGoMetrics[benchmarkData](registry, tags),
	}
	
	// Add script warming option if available
	if warmScripts {
		options = append(options, cache.WithWarmLuaScripts[benchmarkData](true))
	}

	cacheInstance, err := cache.NewCache(
		ctx,
		setup.RedisClient,
		false, // no indexing for script warming benchmarks
		extractor,
		0, // no pool warming for baseline benchmarks
		options...,
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to create shared script warming cache (warm=%v): %s", warmScripts, err.Error()))
	}
	
	return cacheInstance
}

// ==============================================================================
// HELPER FUNCTIONS FOR FEATURE BENCHMARKS
// ==============================================================================

// setupBenchmarkCacheWithIndexing creates cache with indexing enabled for comparison benchmarks
func setupBenchmarkCacheWithIndexing(b *testing.B) interfaces.Cache[benchmarkData] {
	b.Helper()
	return getSharedIndexingCache()
}

// setupBenchmarkCacheWithSerializer creates cache with specific serialization format
func setupBenchmarkCacheWithSerializer(b *testing.B, format string) interfaces.Cache[benchmarkData] {
	b.Helper()
	return getSharedSerializerCache(format)
}

// setupBenchmarkCacheWithScriptWarming creates cache with Lua script warming enabled/disabled
func setupBenchmarkCacheWithScriptWarming(b *testing.B, warmScripts bool) interfaces.Cache[benchmarkData] {
	b.Helper()
	return getSharedScriptWarmingCache(warmScripts)
}

// ==============================================================================
// INDEXING PERFORMANCE BENCHMARKS
// ==============================================================================

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

	// Use timestamp to ensure unique keys for this benchmark run
	benchPrefix := fmt.Sprintf("bench10_%d", time.Now().UnixNano())

	// Pre-populate cache with 10 entries per owner across 100 owners
	for ownerID := range 100 {
		for entryID := range 10 {
			testData := generateBenchmarkData1KB(fmt.Sprintf("%s_user%d:entry%d", benchPrefix, ownerID, entryID))
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
			results, err := cache.GetByOwner(ctx, fmt.Sprintf("%s_user%d", benchPrefix, ownerID%100))
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

	// Use timestamp to ensure unique keys for this benchmark run
	benchPrefix := fmt.Sprintf("bench100_%d", time.Now().UnixNano())

	// Pre-populate cache with 100 entries per owner across 10 owners
	for ownerID := range 10 {
		for entryID := range 100 {
			testData := generateBenchmarkData1KB(fmt.Sprintf("%s_user%d:entry%d", benchPrefix, ownerID, entryID))
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
			results, err := cache.GetByOwner(ctx, fmt.Sprintf("%s_user%d", benchPrefix, ownerID%10))
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
		ownerKey := fmt.Sprintf("deletebench-user%d", i)
		for entryID := range 100 {
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

// ==============================================================================
// SERIALIZATION OVERHEAD BENCHMARKS
// ==============================================================================

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

// ==============================================================================
// LUA SCRIPT WARMING BENCHMARKS
// ==============================================================================

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

			_, err = cache.Delete(ctx, "bench:script:warm")
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

			_, err = cache.Delete(ctx, "bench:script:cold")
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
			0, // no pool warming for baseline benchmarks
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

// ==============================================================================
// GET COUNT BY OWNER BENCHMARKS
// ==============================================================================

// BenchmarkRedisCache_GetCountByOwner_10Entries tests GetCountByOwner performance with 10 entries per owner
func BenchmarkRedisCache_GetCountByOwner_10Entries(b *testing.B) {
	cache := setupBenchmarkCacheWithIndexing(b)
	ctx := context.Background()

	// Use timestamp to ensure unique keys for this benchmark run
	benchPrefix := fmt.Sprintf("countbench10_%d", time.Now().UnixNano())

	// Pre-populate cache with 10 entries per owner across 100 owners
	for ownerID := range 100 {
		for entryID := range 10 {
			testData := generateBenchmarkData1KB(fmt.Sprintf("%s_user%d:entry%d", benchPrefix, ownerID, entryID))
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
			count, err := cache.GetCountByOwner(ctx, fmt.Sprintf("%s_user%d", benchPrefix, ownerID%100))
			if err != nil {
				b.Errorf("GetCountByOwner error: %v", err)
			}
			if count != 10 {
				b.Errorf("Expected count 10, got %d", count)
			}
			ownerID++
		}
	})
}

// BenchmarkRedisCache_GetCountByOwner_100Entries tests GetCountByOwner performance with 100 entries per owner
func BenchmarkRedisCache_GetCountByOwner_100Entries(b *testing.B) {
	cache := setupBenchmarkCacheWithIndexing(b)
	ctx := context.Background()

	// Use timestamp to ensure unique keys for this benchmark run
	benchPrefix := fmt.Sprintf("countbench100_%d", time.Now().UnixNano())

	// Pre-populate cache with 100 entries per owner across 10 owners
	for ownerID := range 10 {
		for entryID := range 100 {
			testData := generateBenchmarkData1KB(fmt.Sprintf("%s_user%d:entry%d", benchPrefix, ownerID, entryID))
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
			count, err := cache.GetCountByOwner(ctx, fmt.Sprintf("%s_user%d", benchPrefix, ownerID%10))
			if err != nil {
				b.Errorf("GetCountByOwner error: %v", err)
			}
			if count != 100 {
				b.Errorf("Expected count 100, got %d", count)
			}
			ownerID++
		}
	})
}

// BenchmarkRedisCache_GetCountByOwner_1000Entries tests GetCountByOwner performance with 1000 entries per owner
func BenchmarkRedisCache_GetCountByOwner_1000Entries(b *testing.B) {
	cache := setupBenchmarkCacheWithIndexing(b)
	ctx := context.Background()

	// Use timestamp to ensure unique keys for this benchmark run
	benchPrefix := fmt.Sprintf("countbench1000_%d", time.Now().UnixNano())

	// Pre-populate cache with 1000 entries per owner across 5 owners
	for ownerID := range 5 {
		for entryID := range 1000 {
			testData := generateBenchmarkData1KB(fmt.Sprintf("%s_user%d:entry%d", benchPrefix, ownerID, entryID))
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
			count, err := cache.GetCountByOwner(ctx, fmt.Sprintf("%s_user%d", benchPrefix, ownerID%5))
			if err != nil {
				b.Errorf("GetCountByOwner error: %v", err)
			}
			if count != 1000 {
				b.Errorf("Expected count 1000, got %d", count)
			}
			ownerID++
		}
	})
}

// BenchmarkRedisCache_GetCountByOwner_EmptyOwner tests GetCountByOwner performance for non-existent owners
func BenchmarkRedisCache_GetCountByOwner_EmptyOwner(b *testing.B) {
	cache := setupBenchmarkCacheWithIndexing(b)
	ctx := context.Background()

	// Use timestamp to ensure unique keys for this benchmark run
	benchPrefix := fmt.Sprintf("countbenchempty_%d", time.Now().UnixNano())

	// Pre-populate cache with some data to make it realistic
	for ownerID := range 10 {
		for entryID := range 50 {
			testData := generateBenchmarkData1KB(fmt.Sprintf("%s_existing_user%d:entry%d", benchPrefix, ownerID, entryID))
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
			// Query for non-existent owners
			count, err := cache.GetCountByOwner(ctx, fmt.Sprintf("%s_nonexistent_user%d", benchPrefix, ownerID%1000))
			if err != nil {
				b.Errorf("GetCountByOwner error: %v", err)
			}
			if count != 0 {
				b.Errorf("Expected count 0 for non-existent owner, got %d", count)
			}
			ownerID++
		}
	})
}

// BenchmarkRedisCache_GetCountByOwner_Mixed tests GetCountByOwner performance with mixed scenarios
func BenchmarkRedisCache_GetCountByOwner_Mixed(b *testing.B) {
	cache := setupBenchmarkCacheWithIndexing(b)
	ctx := context.Background()

	// Use timestamp to ensure unique keys for this benchmark run
	benchPrefix := fmt.Sprintf("countbenchmixed_%d", time.Now().UnixNano())

	// Pre-populate cache with varied entry counts per owner
	ownerEntries := []int{1, 5, 10, 25, 50, 100, 200, 500} // Different entry counts
	for i, entryCount := range ownerEntries {
		for entryID := range entryCount {
			testData := generateBenchmarkData1KB(fmt.Sprintf("%s_mixeduser%d:entry%d", benchPrefix, i, entryID))
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
			ownerIndex := ownerID % len(ownerEntries)
			expectedCount := ownerEntries[ownerIndex]
			
			count, err := cache.GetCountByOwner(ctx, fmt.Sprintf("%s_mixeduser%d", benchPrefix, ownerIndex))
			if err != nil {
				b.Errorf("GetCountByOwner error: %v", err)
			}
			if count != expectedCount {
				b.Errorf("Expected count %d for owner %d, got %d", expectedCount, ownerIndex, count)
			}
			ownerID++
		}
	})
}

// BenchmarkRedisCache_GetCountByOwner_ConcurrentWrites tests GetCountByOwner performance under concurrent write load
func BenchmarkRedisCache_GetCountByOwner_ConcurrentWrites(b *testing.B) {
	cache := setupBenchmarkCacheWithIndexing(b)
	ctx := context.Background()

	// Use timestamp to ensure unique keys for this benchmark run
	benchPrefix := fmt.Sprintf("countbenchconcurrent_%d", time.Now().UnixNano())

	// Pre-populate cache with initial data
	for ownerID := range 20 {
		for entryID := range 50 {
			testData := generateBenchmarkData1KB(fmt.Sprintf("%s_concurrent_user%d:entry%d", benchPrefix, ownerID, entryID))
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Fatalf("Failed to pre-populate cache: %v", err)
			}
		}
	}

	// Start background writers to simulate concurrent modification load
	stopWriters := make(chan struct{})
	defer close(stopWriters)

	go func() {
		writerID := 0
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		
		for {
			select {
			case <-stopWriters:
				return
			case <-ticker.C:
				// Add new entries occasionally
				testData := generateBenchmarkData1KB(fmt.Sprintf("%s_concurrent_user%d:dynamic%d", 
					benchPrefix, writerID%20, time.Now().UnixNano()))
				cache.Set(ctx, testData, 10*time.Minute)
				writerID++
			}
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ownerID := 0
		for pb.Next() {
			// Query counts while concurrent writes are happening
			count, err := cache.GetCountByOwner(ctx, fmt.Sprintf("%s_concurrent_user%d", benchPrefix, ownerID%20))
			if err != nil {
				b.Errorf("GetCountByOwner error: %v", err)
			}
			// Count should be at least 50 (initial entries), but may be higher due to concurrent writes
			if count < 50 {
				b.Errorf("Expected count >= 50 for owner %d, got %d", ownerID%20, count)
			}
			ownerID++
		}
	})
}

// BenchmarkRedisCache_GetCountByOwner_MemoryAllocation tests GetCountByOwner memory allocation patterns
func BenchmarkRedisCache_GetCountByOwner_MemoryAllocation(b *testing.B) {
	cache := setupBenchmarkCacheWithIndexing(b)
	ctx := context.Background()

	// Use timestamp to ensure unique keys for this benchmark run
	benchPrefix := fmt.Sprintf("countbenchalloc_%d", time.Now().UnixNano())

	// Pre-populate cache with data
	for ownerID := range 20 {
		for entryID := range 25 {
			testData := generateBenchmarkData1KB(fmt.Sprintf("%s_alloc_user%d:entry%d", benchPrefix, ownerID, entryID))
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Fatalf("Failed to pre-populate cache: %v", err)
			}
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		count, err := cache.GetCountByOwner(ctx, fmt.Sprintf("%s_alloc_user%d", benchPrefix, i%20))
		if err != nil {
			b.Errorf("GetCountByOwner error: %v", err)
		}
		if count != 25 {
			b.Errorf("Expected count 25, got %d", count)
		}
	}
}
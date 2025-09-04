package cache_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	cache "github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/MichaelAJay/go-cache/internal/testintegration"
)

// Test data structures for benchmarks
type benchmarkData struct {
	ID      string            `json:"id"`
	Content string            `json:"content"`
	Data    map[string]string `json:"data"`
}

func (bd benchmarkData) GetID() string {
	return bd.ID
}

func (bd benchmarkData) GetOwner() string {
	// Extract owner from ID (format: "owner:id")
	parts := strings.Split(bd.ID, ":")
	if len(parts) >= 2 {
		return parts[0]
	}
	return "default"
}

// Data generators for different sizes
func generateBenchmarkData1KB(id string) benchmarkData {
	content := strings.Repeat("a", 900) // ~1KB with overhead
	return benchmarkData{
		ID:      id,
		Content: content,
		Data:    map[string]string{"key1": "value1", "key2": "value2"},
	}
}

func generateBenchmarkData10KB(id string) benchmarkData {
	content := strings.Repeat("a", 9800) // ~10KB with overhead
	return benchmarkData{
		ID:      id,
		Content: content,
		Data:    map[string]string{"key1": "value1", "key2": "value2"},
	}
}

func generateBenchmarkData100KB(id string) benchmarkData {
	content := strings.Repeat("a", 99800) // ~100KB with overhead
	return benchmarkData{
		ID:      id,
		Content: content,
		Data:    map[string]string{"key1": "value1", "key2": "value2"},
	}
}

// Setup helper for benchmarks
func setupBenchmarkCache(b *testing.B) interfaces.Cache[benchmarkData] {
	b.Helper()

	ctx := context.Background()
	
	// Create a temporary testing.T to satisfy the interface
	// This is a workaround for the setup function expecting a *testing.T
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
		false, // no indexing for basic benchmarks
		extractor,
		cache.WithTTL[benchmarkData](10*time.Minute),
		cache.WithSerializer[benchmarkData]("msgpack"),
	)
	if err != nil {
		b.Fatalf("Failed to create cache: %v", err)
	}

	return cacheInstance
}

// 1. Basic Operation Benchmarks

// BenchmarkRedisCache_Get_1KB tests GET performance with 1KB data
func BenchmarkRedisCache_Get_1KB(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()
	
	// Pre-populate cache with test data
	testData := generateBenchmarkData1KB("bench:test:1kb")
	err := cache.Set(ctx, testData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, err := cache.Get(ctx, "bench:test:1kb")
			if err != nil {
				b.Errorf("GET error: %v", err)
			}
		}
	})
}

// BenchmarkRedisCache_Get_10KB tests GET performance with 10KB data
func BenchmarkRedisCache_Get_10KB(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()
	
	// Pre-populate cache with test data
	testData := generateBenchmarkData10KB("bench:test:10kb")
	err := cache.Set(ctx, testData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, err := cache.Get(ctx, "bench:test:10kb")
			if err != nil {
				b.Errorf("GET error: %v", err)
			}
		}
	})
}

// BenchmarkRedisCache_Get_100KB tests GET performance with 100KB data
func BenchmarkRedisCache_Get_100KB(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()
	
	// Pre-populate cache with test data
	testData := generateBenchmarkData100KB("bench:test:100kb")
	err := cache.Set(ctx, testData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, err := cache.Get(ctx, "bench:test:100kb")
			if err != nil {
				b.Errorf("GET error: %v", err)
			}
		}
	})
}

// BenchmarkRedisCache_Set_1KB tests SET performance with 1KB data
func BenchmarkRedisCache_Set_1KB(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			testData := generateBenchmarkData1KB(fmt.Sprintf("bench:set:1kb:%d", i))
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SET error: %v", err)
			}
			i++
		}
	})
}

// BenchmarkRedisCache_Set_10KB tests SET performance with 10KB data
func BenchmarkRedisCache_Set_10KB(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			testData := generateBenchmarkData10KB(fmt.Sprintf("bench:set:10kb:%d", i))
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SET error: %v", err)
			}
			i++
		}
	})
}

// BenchmarkRedisCache_Set_100KB tests SET performance with 100KB data
func BenchmarkRedisCache_Set_100KB(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			testData := generateBenchmarkData100KB(fmt.Sprintf("bench:set:100kb:%d", i))
			err := cache.Set(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SET error: %v", err)
			}
			i++
		}
	})
}

// BenchmarkRedisCache_Delete tests DELETE performance
func BenchmarkRedisCache_Delete(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	for i := 0; i < b.N; i++ {
		testData := generateBenchmarkData1KB(fmt.Sprintf("bench:delete:%d", i))
		err := cache.Set(ctx, testData, 10*time.Minute)
		if err != nil {
			b.Fatalf("Failed to pre-populate cache: %v", err)
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			err := cache.Delete(ctx, fmt.Sprintf("bench:delete:%d", i))
			if err != nil {
				b.Errorf("DELETE error: %v", err)
			}
			i++
		}
	})
}

// BenchmarkRedisCache_Has tests HAS/EXISTS performance
func BenchmarkRedisCache_Has(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()
	
	// Pre-populate cache with test data
	testData := generateBenchmarkData1KB("bench:has:test")
	err := cache.Set(ctx, testData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			exists := cache.Has(ctx, "bench:has:test")
			if !exists {
				b.Error("Expected key to exist")
			}
		}
	})
}

// BenchmarkRedisCache_Clear tests CLEAR performance
func BenchmarkRedisCache_Clear(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	b.ResetTimer()
	for i := range b.N {
		b.StopTimer()
		// Pre-populate cache with test data for each iteration
		for j := range 100 {
			testData := generateBenchmarkData1KB(fmt.Sprintf("bench:clear:%d:%d", i, j))
			cache.Set(ctx, testData, 10*time.Minute)
		}
		b.StartTimer()

		err := cache.Clear(ctx)
		if err != nil {
			b.Errorf("CLEAR error: %v", err)
		}
	}
}

// 2. Concurrency Stress Tests

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

// 3. Atomic Operations Performance

// BenchmarkRedisCache_GetOrSet_CacheMiss tests GetOrSet performance when cache miss occurs
func BenchmarkRedisCache_GetOrSet_CacheMiss(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()
	
	// Loader function that generates new data
	loader := func(ctx context.Context) (benchmarkData, error) {
		return generateBenchmarkData1KB("bench:getorset:miss:generated"), nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench:getorset:miss:%d", i)
			_, err := cache.GetOrSet(ctx, key, loader, 10*time.Minute)
			if err != nil {
				b.Errorf("GetOrSet cache miss error: %v", err)
			}
			// Clean up to ensure each iteration is a cache miss
			cache.Delete(ctx, key)
			i++
		}
	})
}

// BenchmarkRedisCache_GetOrSet_CacheHit tests GetOrSet performance when cache hit occurs
func BenchmarkRedisCache_GetOrSet_CacheHit(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()
	
	// Pre-populate cache with test data
	testData := generateBenchmarkData1KB("bench:getorset:hit")
	err := cache.Set(ctx, testData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}
	
	// Loader function (should not be called)
	loader := func(ctx context.Context) (benchmarkData, error) {
		return generateBenchmarkData1KB("should:not:be:called"), nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := cache.GetOrSet(ctx, "bench:getorset:hit", loader, 10*time.Minute)
			if err != nil {
				b.Errorf("GetOrSet cache hit error: %v", err)
			}
		}
	})
}

// BenchmarkRedisCache_GetOrSet_HighContention tests GetOrSet with multiple goroutines on same key
func BenchmarkRedisCache_GetOrSet_HighContention(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()
	
	// Loader function that simulates work
	loader := func(ctx context.Context) (benchmarkData, error) {
		// Simulate some work
		time.Sleep(1 * time.Millisecond)
		return generateBenchmarkData1KB("bench:getorset:contention:generated"), nil
	}

	b.ResetTimer()
	b.SetParallelism(100) // High contention with many goroutines
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// All goroutines compete for the same key
			_, err := cache.GetOrSet(ctx, "bench:getorset:contention", loader, 10*time.Minute)
			if err != nil {
				b.Errorf("GetOrSet high contention error: %v", err)
			}
		}
	})
}

// BenchmarkRedisCache_Update_Existing tests Update performance on existing keys
func BenchmarkRedisCache_Update_Existing(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()
	
	// Pre-populate cache with test data
	for i := 0; i < b.N; i++ {
		testData := generateBenchmarkData1KB(fmt.Sprintf("bench:update:existing:%d", i))
		err := cache.Set(ctx, testData, 10*time.Minute)
		if err != nil {
			b.Fatalf("Failed to pre-populate cache: %v", err)
		}
	}
	
	// Updater function that modifies existing data
	updater := func(old benchmarkData, exists bool) (benchmarkData, error) {
		if !exists {
			return old, fmt.Errorf("key should exist")
		}
		old.Content = "UPDATED: " + old.Content[:100] // Truncate to avoid growing too large
		return old, nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench:update:existing:%d", i%b.N)
			_, err := cache.Update(ctx, key, updater, 10*time.Minute)
			if err != nil {
				b.Errorf("Update existing error: %v", err)
			}
			i++
		}
	})
}

// BenchmarkRedisCache_Update_NonExistent tests Update performance on non-existent keys
func BenchmarkRedisCache_Update_NonExistent(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()
	
	// Updater function that creates new data if key doesn't exist
	updater := func(old benchmarkData, exists bool) (benchmarkData, error) {
		if exists {
			return old, fmt.Errorf("key should not exist")
		}
		return generateBenchmarkData1KB("bench:update:nonexistent:created"), nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench:update:nonexistent:%d", i)
			_, err := cache.Update(ctx, key, updater, 10*time.Minute)
			if err != nil {
				b.Errorf("Update non-existent error: %v", err)
			}
			// Clean up to ensure each iteration deals with non-existent key
			cache.Delete(ctx, key)
			i++
		}
	})
}

// BenchmarkRedisCache_Update_HighContention tests Update with multiple goroutines on same key
func BenchmarkRedisCache_Update_HighContention(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()
	
	// Pre-populate cache with initial data
	initialData := generateBenchmarkData1KB("bench:update:contention")
	err := cache.Set(ctx, initialData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}
	
	// Updater function that increments a counter in the data
	updater := func(old benchmarkData, exists bool) (benchmarkData, error) {
		if !exists {
			// Create new if doesn't exist
			return generateBenchmarkData1KB("bench:update:contention:created"), nil
		}
		// Simulate updating existing data
		old.Data["counter"] = fmt.Sprintf("%d", time.Now().UnixNano())
		return old, nil
	}

	b.ResetTimer()
	b.SetParallelism(100) // High contention with many goroutines
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// All goroutines compete to update the same key
			_, err := cache.Update(ctx, "bench:update:contention", updater, 10*time.Minute)
			if err != nil {
				b.Errorf("Update high contention error: %v", err)
			}
		}
	})
}

// 4. Batch Operations Efficiency

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

// 5. Indexing Performance Impact

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
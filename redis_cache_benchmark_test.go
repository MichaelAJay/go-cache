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
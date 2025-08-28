//go:build integration
// +build integration

package redis

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/MichaelAJay/go-serializer"
)

// BenchmarkBasicOperations benchmarks basic cache operations for different types
func BenchmarkBasicOperations(b *testing.B) {
	container := SetupRedisContainer(&testing.T{}) // Convert to testing.T for setup
	defer container.Close()
	
	b.Run("String", func(b *testing.B) {
		benchmarkBasicOperationsForType(b, container, "test-value", "updated-value")
	})
	
	b.Run("Int64", func(b *testing.B) {
		benchmarkBasicOperationsForType(b, container, int64(42), int64(100))
	})
	
	b.Run("User", func(b *testing.B) {
		user1 := &User{
			ID:      "user1",
			Name:    "John Doe",
			Email:   "john@test.com",
			Created: time.Now(),
			Tags:    []string{"admin", "active"},
		}
		user2 := &User{
			ID:      "user2", 
			Name:    "Jane Smith",
			Email:   "jane@test.com",
			Created: time.Now(),
			Tags:    []string{"user"},
		}
		benchmarkBasicOperationsForType(b, container, user1, user2)
	})
	
	b.Run("LargeObject", func(b *testing.B) {
		large1 := GenerateLargeObjects(1, 1)[0] // 1KB object
		large2 := GenerateLargeObjects(1, 1)[0] // Another 1KB object
		large2.ID = "large2"
		benchmarkBasicOperationsForType(b, container, large1, large2)
	})
}

// benchmarkBasicOperationsForType benchmarks basic operations for a specific type
func benchmarkBasicOperationsForType[T any](b *testing.B, container *TestRedisContainer, value1 T, value2 T) {
	cache := CreateCacheForTesting[T](container)
	defer cache.Close()
	
	ctx := context.Background()
	key := "benchmark-key"
	
	b.Run("Set", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := cache.Set(ctx, key, value1, time.Hour)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	// Pre-populate for Get benchmark
	cache.Set(ctx, key, value1, time.Hour)
	
	b.Run("Get", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, err := cache.Get(ctx, key)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("Has", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = cache.Has(ctx, key)
		}
	})
	
	b.Run("Update", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var newVal T
			if i%2 == 0 {
				newVal = value1
			} else {
				newVal = value2
			}
			err := cache.Set(ctx, key, newVal, time.Hour)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("Delete", func(b *testing.B) {
		// Pre-populate keys for deletion
		keys := make([]string, b.N)
		for i := 0; i < b.N; i++ {
			keys[i] = fmt.Sprintf("delete-key-%d", i)
			cache.Set(ctx, keys[i], value1, time.Hour)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := cache.Delete(ctx, keys[i])
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkBatchOperations benchmarks batch operations vs individual operations
func BenchmarkBatchOperations(b *testing.B) {
	container := SetupRedisContainer(&testing.T{})
	defer container.Close()
	
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	ctx := context.Background()
	
	batchSizes := []int{10, 100, 1000}
	
	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("BatchSize_%d", batchSize), func(b *testing.B) {
			b.Run("GetMany", func(b *testing.B) {
				// Pre-populate data
				keys := make([]string, batchSize)
				for i := 0; i < batchSize; i++ {
					keys[i] = fmt.Sprintf("batch-key-%d", i)
					cache.Set(ctx, keys[i], fmt.Sprintf("value-%d", i), time.Hour)
				}
				
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, err := cache.GetMany(ctx, keys)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
			
			b.Run("SetMany", func(b *testing.B) {
				items := make(map[string]string, batchSize)
				for i := 0; i < batchSize; i++ {
					items[fmt.Sprintf("batch-set-key-%d", i)] = fmt.Sprintf("value-%d", i)
				}
				
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					err := cache.SetMany(ctx, items, time.Hour)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
			
			b.Run("DeleteMany", func(b *testing.B) {
				b.StopTimer()
				// Setup keys for each iteration
				allKeys := make([][]string, b.N)
				for n := 0; n < b.N; n++ {
					keys := make([]string, batchSize)
					for i := 0; i < batchSize; i++ {
						key := fmt.Sprintf("delete-batch-key-%d-%d", n, i)
						keys[i] = key
						cache.Set(ctx, key, fmt.Sprintf("value-%d", i), time.Hour)
					}
					allKeys[n] = keys
				}
				
				b.StartTimer()
				for i := 0; i < b.N; i++ {
					err := cache.DeleteMany(ctx, allKeys[i])
					if err != nil {
						b.Fatal(err)
					}
				}
			})
			
			// Compare with individual operations
			b.Run("IndividualGets", func(b *testing.B) {
				// Pre-populate data
				keys := make([]string, batchSize)
				for i := 0; i < batchSize; i++ {
					keys[i] = fmt.Sprintf("individual-key-%d", i)
					cache.Set(ctx, keys[i], fmt.Sprintf("value-%d", i), time.Hour)
				}
				
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					for _, key := range keys {
						_, _, err := cache.Get(ctx, key)
						if err != nil {
							b.Fatal(err)
						}
					}
				}
			})
		})
	}
}

// BenchmarkConcurrentOperations benchmarks operations under different concurrency levels
func BenchmarkConcurrentOperations(b *testing.B) {
	container := SetupRedisContainer(&testing.T{})
	defer container.Close()
	
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	ctx := context.Background()
	
	concurrencyLevels := []int{1, 10, 50, 100, 500}
	
	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			b.Run("ConcurrentSets", func(b *testing.B) {
				b.SetParallelism(concurrency)
				b.RunParallel(func(pb *testing.PB) {
					i := 0
					for pb.Next() {
						key := fmt.Sprintf("concurrent-set-%d-%d", concurrency, i)
						value := fmt.Sprintf("value-%d", i)
						err := cache.Set(ctx, key, value, time.Hour)
						if err != nil {
							b.Fatal(err)
						}
						i++
					}
				})
			})
			
			b.Run("ConcurrentGets", func(b *testing.B) {
				// Pre-populate data
				numKeys := 1000
				for i := 0; i < numKeys; i++ {
					key := fmt.Sprintf("get-key-%d", i)
					cache.Set(ctx, key, fmt.Sprintf("value-%d", i), time.Hour)
				}
				
				b.SetParallelism(concurrency)
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					i := 0
					for pb.Next() {
						key := fmt.Sprintf("get-key-%d", i%numKeys)
						_, _, err := cache.Get(ctx, key)
						if err != nil {
							b.Fatal(err)
						}
						i++
					}
				})
			})
			
			b.Run("MixedOperations", func(b *testing.B) {
				// Pre-populate some data
				numKeys := 500
				for i := 0; i < numKeys; i++ {
					key := fmt.Sprintf("mixed-key-%d", i)
					cache.Set(ctx, key, fmt.Sprintf("value-%d", i), time.Hour)
				}
				
				b.SetParallelism(concurrency)
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					i := 0
					for pb.Next() {
						key := fmt.Sprintf("mixed-key-%d", i%numKeys)
						
						// 70% reads, 30% writes
						if i%10 < 7 {
							_, _, err := cache.Get(ctx, key)
							if err != nil {
								b.Fatal(err)
							}
						} else {
							value := fmt.Sprintf("updated-value-%d", i)
							err := cache.Set(ctx, key, value, time.Hour)
							if err != nil {
								b.Fatal(err)
							}
						}
						i++
					}
				})
			})
		})
	}
}

// BenchmarkSerializationOverhead benchmarks different serialization formats
func BenchmarkSerializationOverhead(b *testing.B) {
	container := SetupRedisContainer(&testing.T{})
	defer container.Close()
	
	ctx := context.Background()
	
	// Test with different data types and serialization formats
	user := &User{
		ID:      "benchmark-user",
		Name:    "Benchmark User",
		Email:   "benchmark@test.com",
		Created: time.Now(),
		Tags:    []string{"tag1", "tag2", "tag3", "tag4", "tag5"},
		Metadata: map[string]interface{}{
			"department": "engineering",
			"level":      5,
			"active":     true,
			"skills":     []string{"go", "redis", "docker"},
		},
	}
	
	formats := []string{
		string(serializer.JSON),
		string(serializer.Binary),
		string(serializer.Msgpack),
	}
	
	for _, format := range formats {
		b.Run(fmt.Sprintf("Format_%s", format), func(b *testing.B) {
			cache := CreateCacheWithOptions[*User](container, &interfaces.CacheOptions{
				SerializerFormat: format,
			})
			defer cache.Close()
			
			key := "serialization-benchmark"
			
			b.Run("Serialize_Set", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					err := cache.Set(ctx, key, user, time.Hour)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
			
			// Pre-populate for deserialization benchmark
			cache.Set(ctx, key, user, time.Hour)
			
			b.Run("Deserialize_Get", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, _, err := cache.Get(ctx, key)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

// BenchmarkAtomicOperations benchmarks GetOrSet and Update operations
func BenchmarkAtomicOperations(b *testing.B) {
	container := SetupRedisContainer(&testing.T{})
	defer container.Close()
	
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	ctx := context.Background()
	
	b.Run("GetOrSet", func(b *testing.B) {
		loader := func(ctx context.Context) (string, error) {
			return "loaded-value", nil
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("getorset-key-%d", i)
			_, err := cache.GetOrSet(ctx, key, loader, time.Hour)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("GetOrSet_ExistingKey", func(b *testing.B) {
		key := "existing-getorset-key"
		cache.Set(ctx, key, "existing-value", time.Hour)
		
		loader := func(ctx context.Context) (string, error) {
			return "should-not-be-called", nil
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := cache.GetOrSet(ctx, key, loader, time.Hour)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("Update", func(b *testing.B) {
		// Pre-populate keys
		keys := make([]string, b.N)
		for i := 0; i < b.N; i++ {
			keys[i] = fmt.Sprintf("update-key-%d", i)
			cache.Set(ctx, keys[i], "original-value", time.Hour)
		}
		
		updater := func(old string, exists bool) (string, error) {
			if !exists {
				return "new-value", nil
			}
			return old + "-updated", nil
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := cache.Update(ctx, keys[i], updater, time.Hour)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("ConcurrentGetOrSet", func(b *testing.B) {
		key := "concurrent-getorset-key"
		var loaderCallCount int64
		
		loader := func(ctx context.Context) (string, error) {
			atomic.AddInt64(&loaderCallCount, 1)
			time.Sleep(time.Millisecond) // Simulate expensive operation
			return "loaded-value", nil
		}
		
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := cache.GetOrSet(ctx, key, loader, time.Hour)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		
		// Verify singleflight behavior - loader should be called exactly once
		if loaderCallCount != 1 {
			b.Errorf("Expected loader to be called once, but was called %d times", loaderCallCount)
		}
	})
}

// BenchmarkIndexingOperations benchmarks indexing-related operations
func BenchmarkIndexingOperations(b *testing.B) {
	container := SetupRedisContainer(&testing.T{})
	defer container.Close()
	
	cache := CreateCacheForTesting[*User](container)
	defer cache.Close()
	
	ctx := context.Background()
	
	b.Run("AddIndex", func(b *testing.B) {
		// Pre-populate data
		numUsers := 1000
		for i := 0; i < numUsers; i++ {
			key := fmt.Sprintf("user:%d", i)
			user := &User{ID: fmt.Sprintf("user-%d", i), Name: fmt.Sprintf("User %d", i)}
			cache.Set(ctx, key, user, time.Hour)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			indexName := fmt.Sprintf("index-%d", i)
			err := cache.AddIndex(ctx, indexName, "user:*", "all")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("GetByIndex", func(b *testing.B) {
		// Pre-populate data and index
		numUsers := 1000
		for i := 0; i < numUsers; i++ {
			key := fmt.Sprintf("indexed-user:%d", i)
			user := &User{ID: fmt.Sprintf("user-%d", i), Name: fmt.Sprintf("User %d", i)}
			cache.Set(ctx, key, user, time.Hour)
		}
		cache.AddIndex(ctx, "benchmark-index", "indexed-user:*", "all")
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := cache.GetByIndex(ctx, "benchmark-index", "all")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("GetKeysByPattern", func(b *testing.B) {
		// Pre-populate data
		numKeys := 1000
		for i := 0; i < numKeys; i++ {
			key := fmt.Sprintf("pattern-key:%d", i)
			user := &User{ID: fmt.Sprintf("user-%d", i), Name: fmt.Sprintf("User %d", i)}
			cache.Set(ctx, key, user, time.Hour)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := cache.GetKeysByPattern(ctx, "pattern-key:*")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkMemoryEfficiency measures memory usage patterns
func BenchmarkMemoryEfficiency(b *testing.B) {
	container := SetupRedisContainer(&testing.T{})
	defer container.Close()
	
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	ctx := context.Background()
	
	b.Run("MemoryUsage", func(b *testing.B) {
		runtime.GC()
		var m1 runtime.MemStats
		runtime.ReadMemStats(&m1)
		
		// Perform operations
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("memory-key-%d", i)
			value := fmt.Sprintf("memory-value-%d", i)
			
			err := cache.Set(ctx, key, value, time.Hour)
			if err != nil {
				b.Fatal(err)
			}
			
			_, _, err = cache.Get(ctx, key)
			if err != nil {
				b.Fatal(err)
			}
			
			if i%100 == 0 {
				err = cache.Delete(ctx, key)
				if err != nil {
					b.Fatal(err)
				}
			}
		}
		
		runtime.GC()
		var m2 runtime.MemStats
		runtime.ReadMemStats(&m2)
		
		bytesPerOp := (m2.TotalAlloc - m1.TotalAlloc) / uint64(b.N)
		b.ReportMetric(float64(bytesPerOp), "bytes/op")
		b.ReportMetric(float64(m2.Mallocs-m1.Mallocs), "allocs/op")
	})
}

// BenchmarkThroughputAndLatency measures throughput and latency characteristics
func BenchmarkThroughputAndLatency(b *testing.B) {
	container := SetupRedisContainer(&testing.T{})
	defer container.Close()
	
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	ctx := context.Background()
	
	// Pre-populate some data
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("throughput-key-%d", i)
		cache.Set(ctx, key, fmt.Sprintf("value-%d", i), time.Hour)
	}
	
	b.Run("ThroughputTest", func(b *testing.B) {
		start := time.Now()
		
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				key := fmt.Sprintf("throughput-key-%d", i%1000)
				
				// Mix of read and write operations (80% read, 20% write)
				if i%5 == 0 {
					// Write operation
					value := fmt.Sprintf("updated-value-%d", i)
					err := cache.Set(ctx, key, value, time.Hour)
					if err != nil {
						b.Fatal(err)
					}
				} else {
					// Read operation
					_, _, err := cache.Get(ctx, key)
					if err != nil {
						b.Fatal(err)
					}
				}
				i++
			}
		})
		
		duration := time.Since(start)
		opsPerSecond := float64(b.N) / duration.Seconds()
		b.ReportMetric(opsPerSecond, "ops/sec")
	})
}

// BenchmarkScalingBehavior tests performance scaling with increasing data size
func BenchmarkScalingBehavior(b *testing.B) {
	container := SetupRedisContainer(&testing.T{})
	defer container.Close()
	
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	ctx := context.Background()
	
	dataSizes := []int{1000, 10000, 100000}
	
	for _, dataSize := range dataSizes {
		b.Run(fmt.Sprintf("DataSize_%d", dataSize), func(b *testing.B) {
			// Pre-populate data
			for i := 0; i < dataSize; i++ {
				key := fmt.Sprintf("scale-key-%d-%d", dataSize, i)
				cache.Set(ctx, key, fmt.Sprintf("value-%d", i), time.Hour)
			}
			
			b.Run("RandomGets", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					key := fmt.Sprintf("scale-key-%d-%d", dataSize, i%dataSize)
					_, _, err := cache.Get(ctx, key)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
			
			b.Run("PatternMatch", func(b *testing.B) {
				pattern := fmt.Sprintf("scale-key-%d-*", dataSize)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, err := cache.GetKeysByPattern(ctx, pattern)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
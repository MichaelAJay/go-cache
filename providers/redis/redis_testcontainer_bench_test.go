package redis

import (
	"context"
	"fmt"  
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-serializer"
)

// BenchmarkRedis8Operations benchmarks Redis 8.0 operations
func BenchmarkRedis8Operations(b *testing.B) {
	redis, err := NewRedisTestContainer(&testing.T{},
		WithRedisVersion("redis:8.0"),
		WithCacheOptions(&cache.CacheOptions{
			TTL:              time.Hour,
			SerializerFormat: serializer.Msgpack,
		}),
	)
	if err != nil {
		b.Fatalf("Failed to start Redis container: %v", err)
	}
	defer redis.Cleanup()

	ctx := context.Background()
	testPrefix := CreateUniqueTestID("bench")

	b.Run("Set", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("%sset:%d", testPrefix, i)
			redis.Cache.Set(ctx, key, "benchmark-value", time.Hour)
		}
	})

	b.Run("Get", func(b *testing.B) {
		// Pre-populate with data
		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("%sget:%d", testPrefix, i)
			redis.Cache.Set(ctx, key, "benchmark-value", time.Hour)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("%sget:%d", testPrefix, i%1000)
			redis.Cache.Get(ctx, key)
		}
	})

	b.Run("Delete", func(b *testing.B) {
		// Pre-populate with data
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("%sdelete:%d", testPrefix, i)
			redis.Cache.Set(ctx, key, "benchmark-value", time.Hour)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("%sdelete:%d", testPrefix, i)
			redis.Cache.Delete(ctx, key)
		}
	})

	b.Run("SetMany", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			items := make(map[string]any)
			for j := 0; j < 10; j++ {
				key := fmt.Sprintf("%ssetmany:%d:%d", testPrefix, i, j)
				items[key] = fmt.Sprintf("batch-value-%d", j)
			}
			redis.Cache.SetMany(ctx, items, time.Hour)
		}
	})

	b.Run("GetMany", func(b *testing.B) {
		// Pre-populate with batch data
		for i := 0; i < 100; i++ {
			items := make(map[string]any)
			for j := 0; j < 10; j++ {
				key := fmt.Sprintf("%sgetmany:%d:%d", testPrefix, i, j)
				items[key] = fmt.Sprintf("batch-value-%d", j)
			}
			redis.Cache.SetMany(ctx, items, time.Hour)
		}

		// Prepare keys for benchmarking
		keys := make([]string, 10)
		for j := 0; j < 10; j++ {
			keys[j] = fmt.Sprintf("%sgetmany:0:%d", testPrefix, j)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			redis.Cache.GetMany(ctx, keys)
		}
	})

	b.Run("Increment", func(b *testing.B) {
		counterKey := testPrefix + "counter"
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			redis.Cache.Increment(ctx, counterKey, 1, time.Hour)
		}
	})
}

// BenchmarkRedisVersionComparison compares performance across Redis versions
func BenchmarkRedisVersionComparison(b *testing.B) {
	versions := []string{"redis:7.2", "redis:8.0"}
	
	for _, version := range versions {
		b.Run(fmt.Sprintf("Redis-%s", version), func(b *testing.B) {
			redis, err := NewRedisTestContainer(&testing.T{},
				WithRedisVersion(version),
				WithCacheOptions(&cache.CacheOptions{
					TTL:              time.Hour,
					SerializerFormat: serializer.Msgpack,
				}),
			)
			if err != nil {
				b.Fatalf("Failed to start Redis %s: %v", version, err)
			}
			defer redis.Cleanup()

			ctx := context.Background()
			testPrefix := CreateUniqueTestID(fmt.Sprintf("ver-bench-%s", version))

			b.Run("SetGet", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					key := fmt.Sprintf("%ssetget:%d", testPrefix, i)
					
					// Set
					err := redis.Cache.Set(ctx, key, "benchmark-value", time.Hour)
					if err != nil {
						b.Fatalf("Set failed: %v", err)
					}
					
					// Get
					_, _, err = redis.Cache.Get(ctx, key)
					if err != nil {
						b.Fatalf("Get failed: %v", err)
					}
				}
			})
		})
	}
}

// BenchmarkRedisMemoryUsage benchmarks memory usage patterns
func BenchmarkRedisMemoryUsage(b *testing.B) {
	redis, err := NewRedisTestContainer(&testing.T{},
		WithRedisVersion("redis:8.0"),
		WithCacheOptions(&cache.CacheOptions{
			TTL:              time.Hour,
			SerializerFormat: serializer.Msgpack,
		}),
	)
	if err != nil {
		b.Fatalf("Failed to start Redis container: %v", err)
	}
	defer redis.Cleanup()

	ctx := context.Background()
	testPrefix := CreateUniqueTestID("mem-bench")

	// Test different data sizes
	dataSizes := []int{100, 1000, 10000, 100000}
	
	for _, size := range dataSizes {
		b.Run(fmt.Sprintf("DataSize-%d", size), func(b *testing.B) {
			// Create test data of specified size
			data := make([]byte, size)
			for i := range data {
				data[i] = byte(i % 256)
			}

			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("%smem:%d:%d", testPrefix, size, i)
				err := redis.Cache.Set(ctx, key, data, time.Hour)
				if err != nil {
					b.Fatalf("Set failed: %v", err)
				}
			}
		})
	}
}
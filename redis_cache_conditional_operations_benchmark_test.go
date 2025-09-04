package cache_test

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// BenchmarkRedisCache_SetIfNotExists_NewKey tests SetIfNotExists performance on new/non-existent keys
func BenchmarkRedisCache_SetIfNotExists_NewKey(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			testData := generateBenchmarkData1KB(fmt.Sprintf("bench:setifnotexists:new:%d", i))
			wasSet, err := cache.SetIfNotExists(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SetIfNotExists new key error: %v", err)
			}
			if !wasSet {
				b.Errorf("SetIfNotExists should return true for new key")
			}
			// Clean up to ensure next iteration is also a new key
			cache.Delete(ctx, testData.ID)
			i++
		}
	})
}

// BenchmarkRedisCache_SetIfNotExists_ExistingKey tests SetIfNotExists performance when key already exists
func BenchmarkRedisCache_SetIfNotExists_ExistingKey(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with existing data
	existingData := generateBenchmarkData1KB("bench:setifnotexists:existing")
	err := cache.Set(ctx, existingData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Try to set with different data but same key
			testData := generateBenchmarkData1KB("bench:setifnotexists:existing")
			testData.Content = "DIFFERENT_CONTENT"
			wasSet, err := cache.SetIfNotExists(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SetIfNotExists existing key error: %v", err)
			}
			if wasSet {
				b.Errorf("SetIfNotExists should return false for existing key")
			}
		}
	})
}

// BenchmarkRedisCache_SetIfExists_ExistingKey tests SetIfExists performance when key exists
func BenchmarkRedisCache_SetIfExists_ExistingKey(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache with test data
	for i := 0; i < b.N; i++ {
		testData := generateBenchmarkData1KB(fmt.Sprintf("bench:setifexists:existing:%d", i))
		err := cache.Set(ctx, testData, 10*time.Minute)
		if err != nil {
			b.Fatalf("Failed to pre-populate cache: %v", err)
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Update existing key with new data
			testData := generateBenchmarkData1KB(fmt.Sprintf("bench:setifexists:existing:%d", i%b.N))
			testData.Content = "UPDATED_CONTENT"
			wasSet, err := cache.SetIfExists(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SetIfExists existing key error: %v", err)
			}
			if !wasSet {
				b.Errorf("SetIfExists should return true for existing key")
			}
			i++
		}
	})
}

// BenchmarkRedisCache_SetIfExists_NonExistentKey tests SetIfExists performance on non-existent keys
func BenchmarkRedisCache_SetIfExists_NonExistentKey(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			testData := generateBenchmarkData1KB(fmt.Sprintf("bench:setifexists:nonexistent:%d", i))
			wasSet, err := cache.SetIfExists(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SetIfExists non-existent key error: %v", err)
			}
			if wasSet {
				b.Errorf("SetIfExists should return false for non-existent key")
			}
			i++
		}
	})
}

// BenchmarkRedisCache_SetIfNotExists_HighContention tests SetIfNotExists with many goroutines competing for same key
func BenchmarkRedisCache_SetIfNotExists_HighContention(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Ensure the key doesn't exist initially
	cache.Delete(ctx, "bench:setifnotexists:contention")

	b.ResetTimer()
	b.SetParallelism(100) // High contention with many goroutines
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			testData := generateBenchmarkData1KB("bench:setifnotexists:contention")
			// Only one goroutine should succeed in setting the key
			_, err := cache.SetIfNotExists(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SetIfNotExists high contention error: %v", err)
			}
			// In high contention, wasSet can be either true or false
			// Both are valid outcomes, so we don't assert on the return value
		}
	})
}

// BenchmarkRedisCache_SetIfExists_HighContention tests SetIfExists with many goroutines updating same existing key
func BenchmarkRedisCache_SetIfExists_HighContention(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate with initial data
	initialData := generateBenchmarkData1KB("bench:setifexists:contention")
	err := cache.Set(ctx, initialData, 10*time.Minute)
	if err != nil {
		b.Fatalf("Failed to pre-populate cache: %v", err)
	}

	b.ResetTimer()
	b.SetParallelism(100) // High contention with many goroutines
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// All goroutines compete to update the same existing key
			testData := generateBenchmarkData1KB("bench:setifexists:contention")
			testData.Content = fmt.Sprintf("UPDATED_%d", time.Now().UnixNano())
			wasSet, err := cache.SetIfExists(ctx, testData, 10*time.Minute)
			if err != nil {
				b.Errorf("SetIfExists high contention error: %v", err)
			}
			if !wasSet {
				b.Errorf("SetIfExists should return true for existing key in high contention")
			}
		}
	})
}
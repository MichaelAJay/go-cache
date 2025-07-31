//go:build integration
// +build integration

package redis

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-serializer"
)

// TestCacheStressScenarios tests cache under extreme stress conditions
func TestCacheStressScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress tests in short mode")
	}

	redis, err := NewRedisTestContainer(t,
		WithRedisVersion("redis:8.0"),
		WithCacheOptions(&cache.CacheOptions{
			TTL:              time.Hour,
			SerializerFormat: serializer.Msgpack,
		}),
	)
	if err != nil {
		t.Fatalf("Failed to start Redis container: %v", err)
	}
	defer redis.Cleanup()

	ctx := context.Background()
	testPrefix := CreateUniqueTestID("stress")

	t.Run("MassiveConcurrentOperations", func(t *testing.T) {
		const numGoroutines = 100
		const operationsPerGoroutine = 100
		const timeoutDuration = 2 * time.Minute

		var (
			successfulOps int64
			failedOps     int64
			wg            sync.WaitGroup
		)

		// Create a context with timeout to prevent test hanging
		testCtx, cancel := context.WithTimeout(ctx, timeoutDuration)
		defer cancel()

		startTime := time.Now()

		// Launch concurrent workers
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for j := 0; j < operationsPerGoroutine; j++ {
					key := fmt.Sprintf("%smassive:%d:%d", testPrefix, workerID, j)
					value := fmt.Sprintf("stress-value-%d-%d", workerID, j)

					// Random operation type
					switch rand.Intn(4) {
					case 0: // Set
						if err := redis.Cache.Set(testCtx, key, value, time.Hour); err != nil {
							atomic.AddInt64(&failedOps, 1)
						} else {
							atomic.AddInt64(&successfulOps, 1)
						}

					case 1: // Get
						_, _, err := redis.Cache.Get(testCtx, key)
						if err != nil {
							atomic.AddInt64(&failedOps, 1)
						} else {
							atomic.AddInt64(&successfulOps, 1)
						}

					case 2: // Delete
						if err := redis.Cache.Delete(testCtx, key); err != nil {
							atomic.AddInt64(&failedOps, 1)
						} else {
							atomic.AddInt64(&successfulOps, 1)
						}

					case 3: // Increment
						_, err := redis.Cache.Increment(testCtx, key, 1, time.Hour)
						if err != nil {
							atomic.AddInt64(&failedOps, 1)
						} else {
							atomic.AddInt64(&successfulOps, 1)
						}
					}

					// Check for context cancellation
					select {
					case <-testCtx.Done():
						return
					default:
					}
				}
			}(i)
		}

		wg.Wait()
		duration := time.Since(startTime)

		totalOps := atomic.LoadInt64(&successfulOps) + atomic.LoadInt64(&failedOps)
		successRate := float64(atomic.LoadInt64(&successfulOps)) / float64(totalOps) * 100
		opsPerSecond := float64(totalOps) / duration.Seconds()

		t.Logf("Massive concurrent operations completed:")
		t.Logf("  Total operations: %d", totalOps)
		t.Logf("  Successful: %d (%.2f%%)", atomic.LoadInt64(&successfulOps), successRate)
		t.Logf("  Failed: %d", atomic.LoadInt64(&failedOps))
		t.Logf("  Duration: %v", duration)
		t.Logf("  Throughput: %.2f ops/sec", opsPerSecond)

		// We expect at least 95% success rate under stress
		if successRate < 95.0 {
			t.Errorf("Success rate %.2f%% is below acceptable threshold of 95%%", successRate)
		}
	})

	t.Run("MemoryPressureTest", func(t *testing.T) {
		const numLargeObjects = 50
		const objectSizeKB = 100 // 100KB per object

		largeObjects := make(map[string][]byte)

		// Create large objects
		for i := 0; i < numLargeObjects; i++ {
			key := fmt.Sprintf("%smemory-pressure:%d", testPrefix, i)
			data := make([]byte, objectSizeKB*1024) // Convert KB to bytes

			// Fill with random data
			for j := range data {
				data[j] = byte(rand.Intn(256))
			}

			largeObjects[key] = data
		}

		// Store all large objects
		start := time.Now()
		for key, data := range largeObjects {
			err := redis.Cache.Set(ctx, key, data, time.Hour)
			if err != nil {
				t.Fatalf("Failed to store large object %s: %v", key, err)
			}
		}
		storeDuration := time.Since(start)

		// Retrieve and verify all large objects
		start = time.Now()
		for key, originalData := range largeObjects {
			retrievedValue, found, err := redis.Cache.Get(ctx, key)
			if err != nil {
				t.Fatalf("Failed to retrieve large object %s: %v", key, err)
			}
			if !found {
				t.Fatalf("Large object %s should exist", key)
			}

			retrievedData, ok := retrievedValue.([]byte)
			if !ok {
				t.Fatalf("Expected []byte for %s, got %T", key, retrievedValue)
			}

			if len(retrievedData) != len(originalData) {
				t.Errorf("Object %s: size mismatch, expected %d, got %d",
					key, len(originalData), len(retrievedData))
			}
		}
		retrieveDuration := time.Since(start)

		totalDataMB := float64(numLargeObjects*objectSizeKB) / 1024
		t.Logf("Memory pressure test completed:")
		t.Logf("  Objects: %d (%.2f MB total)", numLargeObjects, totalDataMB)
		t.Logf("  Store time: %v (%.2f MB/s)", storeDuration, totalDataMB/storeDuration.Seconds())
		t.Logf("  Retrieve time: %v (%.2f MB/s)", retrieveDuration, totalDataMB/retrieveDuration.Seconds())
	})

	t.Run("RapidTTLExpiration", func(t *testing.T) {
		const numKeys = 100
		const baseExpiration = 100 * time.Millisecond

		expiredKeys := make([]string, 0, numKeys)

		// Create keys with staggered expiration times
		for i := 0; i < numKeys; i++ {
			key := fmt.Sprintf("%sttl-rapid:%d", testPrefix, i)
			value := fmt.Sprintf("expiring-value-%d", i)

			// Stagger expiration times between 50ms and 200ms
			ttl := baseExpiration + time.Duration(rand.Intn(100))*time.Millisecond

			err := redis.Cache.Set(ctx, key, value, ttl)
			if err != nil {
				t.Fatalf("Failed to set key %s: %v", key, err)
			}

			expiredKeys = append(expiredKeys, key)
		}

		// Verify all keys exist initially
		existingCount := 0
		for _, key := range expiredKeys {
			_, found, _ := redis.Cache.Get(ctx, key)
			if found {
				existingCount++
			}
		}
		t.Logf("Initial state: %d/%d keys exist", existingCount, numKeys)

		// Wait for some keys to expire
		time.Sleep(150 * time.Millisecond)

		// Check intermediate state
		midCount := 0
		for _, key := range expiredKeys {
			_, found, _ := redis.Cache.Get(ctx, key)
			if found {
				midCount++
			}
		}
		t.Logf("After 150ms: %d/%d keys exist", midCount, numKeys)

		// Wait for all keys to expire
		time.Sleep(200 * time.Millisecond)

		// Verify all keys have expired
		finalCount := 0
		for _, key := range expiredKeys {
			_, found, _ := redis.Cache.Get(ctx, key)
			if found {
				finalCount++
			}
		}

		t.Logf("After 350ms: %d/%d keys exist", finalCount, numKeys)

		// Allow some tolerance for timing precision
		if finalCount > numKeys/10 { // Allow up to 10% to still exist due to timing
			t.Errorf("Too many keys still exist after expiration: %d/%d", finalCount, numKeys)
		}
	})
}

// TestCacheEdgeCases tests unusual and edge case scenarios
func TestCacheEdgeCases(t *testing.T) {
	redis, err := NewRedisTestContainer(t,
		WithRedisVersion("redis:8.0"),
		WithCacheOptions(&cache.CacheOptions{
			TTL:              time.Hour,
			SerializerFormat: serializer.Msgpack,
		}),
	)
	if err != nil {
		t.Fatalf("Failed to start Redis container: %v", err)
	}
	defer redis.Cleanup()

	ctx := context.Background()
	testPrefix := CreateUniqueTestID("edge-cases")

	t.Run("ExtremeKeyNames", func(t *testing.T) {
		edgeKeys := []struct {
			name       string
			key        string
			shouldWork bool
		}{
			{"Empty key", "", false},
			{"Very long key", testPrefix + "very-long-key-" + generateLongString(1000), true},
			{"Unicode key", testPrefix + "键值-🔑-クイ", true},
			{"Special chars", testPrefix + "key:with@special#chars$and%symbols", true},
			{"Whitespace key", testPrefix + "key with spaces and\ttabs\n", true},
			{"JSON-like key", testPrefix + `{"nested":"key","with":"special"}`, true},
		}

		for _, tc := range edgeKeys {
			t.Run(tc.name, func(t *testing.T) {
				value := fmt.Sprintf("value-for-%s", tc.name)

				err := redis.Cache.Set(ctx, tc.key, value, time.Hour)
				if tc.shouldWork {
					if err != nil {
						t.Errorf("Expected key '%s' to work, got error: %v", tc.key, err)
						return
					}

					// Try to retrieve it
					retrievedValue, found, err := redis.Cache.Get(ctx, tc.key)
					if err != nil {
						t.Errorf("Failed to retrieve key '%s': %v", tc.key, err)
						return
					}
					if !found {
						t.Errorf("Key '%s' should exist", tc.key)
						return
					}
					if retrievedValue != value {
						t.Errorf("Key '%s': expected '%s', got '%v'", tc.key, value, retrievedValue)
					}
				} else {
					if err == nil {
						t.Errorf("Expected key '%s' to fail, but it succeeded", tc.key)
					}
				}
			})
		}
	})

	t.Run("ExtremeValues", func(t *testing.T) {
		extremeValues := []struct {
			name       string
			value      any
			shouldWork bool
		}{
			{"Nil value", nil, false}, // Nil serialization may fail
			{"Empty string", "", true},
			{"Empty slice", []string{}, true},
			{"Empty map", map[string]string{}, true},
			{"Very large string", generateLongString(10000), true},
			{"Very large number", int64(9223372036854775807), true},  // Max int64
			{"Very small number", int64(-9223372036854775808), true}, // Min int64
			{"Unicode string", "🌟🚀💯 Unicode test with emojis 中文 العربية", true},
			{"Complex nested structure", map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"level3": []any{
							"string",
							123,
							true,
							map[string]any{"deep": "value"},
						},
					},
				},
			}, true},
		}

		for i, tc := range extremeValues {
			t.Run(tc.name, func(t *testing.T) {
				key := fmt.Sprintf("%sextreme-value:%d", testPrefix, i)

				err := redis.Cache.Set(ctx, key, tc.value, time.Hour)
				if tc.shouldWork {
					if err != nil {
						t.Errorf("Failed to set extreme value '%s': %v", tc.name, err)
						return
					}

					retrievedValue, found, err := redis.Cache.Get(ctx, key)
					if err != nil {
						t.Errorf("Failed to get extreme value '%s': %v", tc.name, err)
						return
					}
					if !found {
						t.Errorf("Extreme value '%s' should exist", tc.name)
						return
					}

					// Basic validation (exact comparison might not work for complex types)
					if tc.value == nil && retrievedValue != nil {
						t.Errorf("Extreme value '%s': expected nil, got %v", tc.name, retrievedValue)
					}

					t.Logf("Extreme value '%s': stored and retrieved successfully", tc.name)
				} else {
					if err == nil {
						t.Errorf("Expected extreme value '%s' to fail, but it succeeded", tc.name)
					} else {
						t.Logf("Extreme value '%s' correctly failed: %v", tc.name, err)
					}
				}
			})
		}
	})

	t.Run("ConcurrentSameKeyOperations", func(t *testing.T) {
		const numGoroutines = 50
		const operationsPerGoroutine = 20

		sharedKey := fmt.Sprintf("%sshared-key", testPrefix)
		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines*operationsPerGoroutine)

		// Initialize shared key
		err := redis.Cache.Set(ctx, sharedKey, 0, time.Hour)
		if err != nil {
			t.Fatalf("Failed to initialize shared key: %v", err)
		}

		// Launch concurrent workers all operating on the same key
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for j := 0; j < operationsPerGoroutine; j++ {
					switch j % 4 {
					case 0: // Set
						value := fmt.Sprintf("worker-%d-op-%d", workerID, j)
						if err := redis.Cache.Set(ctx, sharedKey, value, time.Hour); err != nil {
							errors <- fmt.Errorf("worker %d set: %w", workerID, err)
						}

					case 1: // Get
						if _, _, err := redis.Cache.Get(ctx, sharedKey); err != nil {
							errors <- fmt.Errorf("worker %d get: %w", workerID, err)
						}

					case 2: // Increment (if possible)
						if _, err := redis.Cache.Increment(ctx, sharedKey, 1, time.Hour); err != nil {
							// Increment might fail if value is not numeric, that's OK
						}

					case 3: // Delete and recreate
						_ = redis.Cache.Delete(ctx, sharedKey)
						if err := redis.Cache.Set(ctx, sharedKey, workerID, time.Hour); err != nil {
							errors <- fmt.Errorf("worker %d recreate: %w", workerID, err)
						}
					}
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		errorCount := 0
		for err := range errors {
			t.Log(err) // Log but don't fail - some contention is expected
			errorCount++
		}

		t.Logf("Concurrent same-key operations: %d errors out of %d operations",
			errorCount, numGoroutines*operationsPerGoroutine)

		// Verify the key still exists and is accessible
		_, found, err := redis.Cache.Get(ctx, sharedKey)
		if err != nil {
			t.Errorf("Failed to get shared key after concurrent operations: %v", err)
		}
		if !found {
			t.Error("Shared key should exist after concurrent operations")
		}
	})

	t.Run("RapidCreateDeleteCycles", func(t *testing.T) {
		const cycles = 100
		key := fmt.Sprintf("%srapid-cycle", testPrefix)

		for i := 0; i < cycles; i++ {
			value := fmt.Sprintf("cycle-value-%d", i)

			// Create
			err := redis.Cache.Set(ctx, key, value, time.Hour)
			if err != nil {
				t.Fatalf("Cycle %d: failed to create: %v", i, err)
			}

			// Verify exists
			retrievedValue, found, err := redis.Cache.Get(ctx, key)
			if err != nil {
				t.Fatalf("Cycle %d: failed to get: %v", i, err)
			}
			if !found {
				t.Fatalf("Cycle %d: key should exist", i)
			}
			if retrievedValue != value {
				t.Fatalf("Cycle %d: value mismatch", i)
			}

			// Delete
			err = redis.Cache.Delete(ctx, key)
			if err != nil {
				t.Fatalf("Cycle %d: failed to delete: %v", i, err)
			}

			// Verify deleted
			_, found, err = redis.Cache.Get(ctx, key)
			if err != nil {
				t.Fatalf("Cycle %d: unexpected error after delete: %v", i, err)
			}
			if found {
				t.Fatalf("Cycle %d: key should be deleted", i)
			}
		}

		t.Logf("Completed %d rapid create-delete cycles", cycles)
	})
}

// TestFailureScenarios tests behavior under various failure conditions
func TestFailureScenarios(t *testing.T) {
	redis, err := NewRedisTestContainer(t,
		WithRedisVersion("redis:8.0"),
		WithCacheOptions(&cache.CacheOptions{
			TTL:              time.Hour,
			SerializerFormat: serializer.Msgpack,
		}),
	)
	if err != nil {
		t.Fatalf("Failed to start Redis container: %v", err)
	}
	defer redis.Cleanup()

	ctx := context.Background()
	testPrefix := CreateUniqueTestID("failures")

	t.Run("ContextTimeoutRecovery", func(t *testing.T) {
		key := fmt.Sprintf("%stimeout-recovery", testPrefix)

		// Set a value normally
		err := redis.Cache.Set(ctx, key, "normal-value", time.Hour)
		if err != nil {
			t.Fatalf("Failed to set normal value: %v", err)
		}

		// Try operation with very short timeout (should fail)
		shortCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
		time.Sleep(1 * time.Millisecond) // Ensure timeout

		err = redis.Cache.Set(shortCtx, key, "timeout-value", time.Hour)
		cancel()

		if err == nil {
			t.Error("Expected timeout error, got nil")
		}

		// Verify original value is still accessible with normal context
		value, found, err := redis.Cache.Get(ctx, key)
		if err != nil {
			t.Fatalf("Failed to recover after timeout: %v", err)
		}
		if !found {
			t.Fatal("Key should still exist after timeout")
		}
		if value != "normal-value" {
			t.Errorf("Expected 'normal-value', got '%v'", value)
		}

		t.Log("Successfully recovered from context timeout")
	})

	t.Run("InvalidSerializationData", func(t *testing.T) {
		// This test is harder to trigger with our current implementation
		// but we can test with various data types that might cause issues

		problematicData := []struct {
			name string
			data any
		}{
			{"Function pointer", fmt.Sprintf}, // This will fail to serialize
			{"Channel", make(chan int)},       // This will fail to serialize
		}

		for _, tc := range problematicData {
			t.Run(tc.name, func(t *testing.T) {
				key := fmt.Sprintf("%sinvalid-data:%s", testPrefix, tc.name)

				err := redis.Cache.Set(ctx, key, tc.data, time.Hour)
				// We expect this to fail with serialization error
				if err == nil {
					t.Errorf("Expected serialization error for %s, got nil", tc.name)

					// If it somehow succeeded, try to get it back
					_, found, err := redis.Cache.Get(ctx, key)
					if err != nil {
						t.Logf("Failed to retrieve %s (expected): %v", tc.name, err)
					} else if !found {
						t.Logf("%s was not found (might be expected)", tc.name)
					}
				} else {
					t.Logf("Correctly rejected invalid data %s: %v", tc.name, err)
				}
			})
		}
	})
}

// Helper function to generate long strings for testing
func generateLongString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

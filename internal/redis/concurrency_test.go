//go:build integration
// +build integration

package redis

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDistributedGetOrSet validates that GetOrSet operations maintain atomicity
// across multiple processes and high concurrency
func TestDistributedGetOrSet(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	
	t.Run("SingleflightBehavior", func(t *testing.T) {
		testGetOrSetSingleflightBehavior(t)
	})
	
	t.Run("LoaderExecutionCount", func(t *testing.T) {
		testGetOrSetLoaderExecutionCount(t, container)
	})
	
	t.Run("ConcurrentGetOrSet", func(t *testing.T) {
		testConcurrentGetOrSet(t, container)
	})
	
	t.Run("GetOrSetWithExpensiveLoader", func(t *testing.T) {
		testGetOrSetWithExpensiveLoader(t, container)
	})
}

// testGetOrSetSingleflightBehavior ensures loader function executes exactly once under high concurrency
func testGetOrSetSingleflightBehavior(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	
	cache := CreateCacheForTesting[*User](container)
	defer cache.Close()
	
	ctx := context.Background()
	key := "singleflight-test"
	
	var loaderCallCount int64
	var wg sync.WaitGroup
	numGoroutines := 1000
	
	// Create a loader that tracks how many times it's called
	loader := func(ctx context.Context) (*User, error) {
		atomic.AddInt64(&loaderCallCount, 1)
		// Add some delay to increase chance of race condition
		time.Sleep(10 * time.Millisecond)
		return &User{
			ID:      "loaded-user",
			Name:    "Loaded User",
			Created: time.Now(),
		}, nil
	}
	
	// Launch many goroutines calling GetOrSet simultaneously
	results := make([]*User, numGoroutines)
	errors := make([]error, numGoroutines)
	
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()
			user, err := cache.GetOrSet(ctx, key, loader, time.Hour)
			results[index] = user
			errors[index] = err
		}(i)
	}
	
	wg.Wait()
	
	// Verify loader was called exactly once
	assert.Equal(t, int64(1), loaderCallCount, "Loader should be called exactly once")
	
	// Verify all goroutines got the same result
	for i := 0; i < numGoroutines; i++ {
		require.NoError(t, errors[i], "GetOrSet should not error for goroutine %d", i)
		require.NotNil(t, results[i], "Result should not be nil for goroutine %d", i)
		assert.Equal(t, "loaded-user", results[i].ID, "All results should have same ID")
		assert.Equal(t, "Loaded User", results[i].Name, "All results should have same name")
	}
}

// testGetOrSetLoaderExecutionCount tests loader execution counting with different scenarios
func testGetOrSetLoaderExecutionCount(t *testing.T, container *TestRedisContainer) {
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	ctx := context.Background()
	
	t.Run("MultipleKeys", func(t *testing.T) {
		var loaderCallCount int64
		loader := func(ctx context.Context) (string, error) {
			atomic.AddInt64(&loaderCallCount, 1)
			return "loaded-value", nil
		}
		
		keys := []string{"key1", "key2", "key3"}
		
		// Each key should trigger loader once
		for _, key := range keys {
			value, err := cache.GetOrSet(ctx, key, loader, time.Hour)
			require.NoError(t, err)
			assert.Equal(t, "loaded-value", value)
		}
		
		// Loader should be called once per unique key
		assert.Equal(t, int64(len(keys)), loaderCallCount)
	})
	
	t.Run("ExistingKey", func(t *testing.T) {
		key := "existing-key"
		existingValue := "existing-value"
		
		// Pre-populate the key
		err := cache.Set(ctx, key, existingValue, time.Hour)
		require.NoError(t, err)
		
		var loaderCallCount int64
		loader := func(ctx context.Context) (string, error) {
			atomic.AddInt64(&loaderCallCount, 1)
			return "should-not-be-called", nil
		}
		
		// GetOrSet should return existing value without calling loader
		value, err := cache.GetOrSet(ctx, key, loader, time.Hour)
		require.NoError(t, err)
		assert.Equal(t, existingValue, value)
		
		// Loader should not be called
		assert.Equal(t, int64(0), loaderCallCount)
	})
}

// testConcurrentGetOrSet tests GetOrSet under various concurrency patterns
func testConcurrentGetOrSet(t *testing.T, container *TestRedisContainer) {
	cache := CreateCacheForTesting[int](container)
	defer cache.Close()
	
	ctx := context.Background()
	
	configs := []struct {
		name       string
		goroutines int
		keys       int
		iterations int
	}{
		{"LowConcurrency", 10, 5, 10},
		{"MediumConcurrency", 100, 20, 5},
		{"HighConcurrency", 500, 50, 2},
	}
	
	for _, config := range configs {
		t.Run(config.name, func(t *testing.T) {
			var totalLoaderCalls int64
			var wg sync.WaitGroup
			
			loader := func(ctx context.Context) (int, error) {
				atomic.AddInt64(&totalLoaderCalls, 1)
				time.Sleep(time.Millisecond) // Simulate work
				return 42, nil
			}
			
			totalOperations := config.goroutines * config.iterations
			wg.Add(totalOperations)
			
			for g := 0; g < config.goroutines; g++ {
				go func(goroutineID int) {
					for i := 0; i < config.iterations; i++ {
						defer wg.Done()
						
						// Use modulo to create contention on keys
						key := fmt.Sprintf("key-%d", goroutineID%config.keys)
						
						value, err := cache.GetOrSet(ctx, key, loader, time.Hour)
						assert.NoError(t, err)
						assert.Equal(t, 42, value)
					}
				}(g)
			}
			
			wg.Wait()
			
			// Loader should be called at most once per unique key
			assert.LessOrEqual(t, totalLoaderCalls, int64(config.keys), 
				"Loader calls should not exceed number of unique keys")
			
			// Should be at least 1 call (assuming at least one key was accessed)
			assert.GreaterOrEqual(t, totalLoaderCalls, int64(1), 
				"Loader should be called at least once")
		})
	}
}

// testGetOrSetWithExpensiveLoader tests behavior with slow loaders
func testGetOrSetWithExpensiveLoader(t *testing.T, container *TestRedisContainer) {
	cache := CreateCacheForTesting[*User](container)
	defer cache.Close()
	
	ctx := context.Background()
	key := "expensive-resource"
	
	var loaderCallCount int64
	expensiveLoader := func(ctx context.Context) (*User, error) {
		atomic.AddInt64(&loaderCallCount, 1)
		// Simulate expensive database operation
		time.Sleep(100 * time.Millisecond)
		return &User{
			ID:      "expensive-user",
			Name:    "Expensive User",
			Created: time.Now(),
		}, nil
	}
	
	// Start multiple goroutines that will all wait for the expensive operation
	numGoroutines := 50
	start := time.Now()
	
	var wg sync.WaitGroup
	results := make([]*User, numGoroutines)
	errors := make([]error, numGoroutines)
	
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()
			user, err := cache.GetOrSet(ctx, key, expensiveLoader, time.Hour)
			results[index] = user
			errors[index] = err
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	// Verify expensive loader was called exactly once
	assert.Equal(t, int64(1), loaderCallCount, "Expensive loader should be called exactly once")
	
	// Verify all operations completed in reasonable time
	// Should be close to the loader time (100ms) rather than 50 * 100ms
	assert.Less(t, duration, 500*time.Millisecond, 
		"All operations should complete efficiently")
	
	// Verify all results are consistent
	for i := 0; i < numGoroutines; i++ {
		require.NoError(t, errors[i], "GetOrSet should not error")
		require.NotNil(t, results[i], "Result should not be nil")
		assert.Equal(t, "expensive-user", results[i].ID)
	}
}

// TestUpdateAtomicity tests the atomicity of Update operations
func TestUpdateAtomicity(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	
	t.Run("CounterIncrement", func(t *testing.T) {
		testUpdateCounterIncrement(t, container)
	})
	
	t.Run("ComplexObjectUpdate", func(t *testing.T) {
		testUpdateComplexObject(t, container)
	})
	
	t.Run("ConcurrentUpdates", func(t *testing.T) {
		testConcurrentUpdates(t, container)
	})
}

// testUpdateCounterIncrement tests atomic counter increments using Update
func testUpdateCounterIncrement(t *testing.T, container *TestRedisContainer) {
	cache := CreateCacheForTesting[int64](container)
	defer cache.Close()
	
	ctx := context.Background()
	key := "counter"
	
	// Initialize counter
	err := cache.Set(ctx, key, int64(0), time.Hour)
	require.NoError(t, err)
	
	numGoroutines := 100
	incrementsPerGoroutine := 10
	expectedTotal := int64(numGoroutines * incrementsPerGoroutine)
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	// Concurrent increments
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPerGoroutine; j++ {
				_, err := cache.Update(ctx, key, func(old int64, exists bool) (int64, error) {
					if !exists {
						return 1, nil
					}
					return old + 1, nil
				}, time.Hour)
				assert.NoError(t, err)
			}
		}()
	}
	
	wg.Wait()
	
	// Verify final count
	finalValue, found, err := cache.Get(ctx, key)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, expectedTotal, finalValue, 
		"Final counter value should equal expected total (no lost updates)")
}

// testUpdateComplexObject tests atomic updates of complex objects
func testUpdateComplexObject(t *testing.T, container *TestRedisContainer) {
	cache := CreateCacheForTesting[*User](container)
	defer cache.Close()
	
	ctx := context.Background()
	key := "user-update"
	
	// Initialize user
	initialUser := &User{
		ID:      "user1",
		Name:    "Initial User",
		Created: time.Now(),
		Tags:    []string{"initial"},
	}
	
	err := cache.Set(ctx, key, initialUser, time.Hour)
	require.NoError(t, err)
	
	// Concurrent updates adding tags
	numGoroutines := 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			
			tag := fmt.Sprintf("tag-%d", goroutineID)
			
			_, err := cache.Update(ctx, key, func(old *User, exists bool) (*User, error) {
				if !exists {
					return nil, fmt.Errorf("user should exist")
				}
				
				// Create new user with additional tag
				newUser := *old // Copy struct
				newUser.Tags = append(newUser.Tags, tag)
				return &newUser, nil
			}, time.Hour)
			
			assert.NoError(t, err)
		}(i)
	}
	
	wg.Wait()
	
	// Verify final state
	finalUser, found, err := cache.Get(ctx, key)
	require.NoError(t, err)
	assert.True(t, found)
	
	// Should have initial tag plus all added tags
	expectedTagCount := 1 + numGoroutines // "initial" + all goroutine tags
	assert.Equal(t, expectedTagCount, len(finalUser.Tags), 
		"All tag updates should be preserved")
	
	// Verify initial tag is still there
	assert.Contains(t, finalUser.Tags, "initial")
	
	// Verify all goroutine tags are there
	tagSet := make(map[string]bool)
	for _, tag := range finalUser.Tags {
		tagSet[tag] = true
	}
	
	for i := 0; i < numGoroutines; i++ {
		expectedTag := fmt.Sprintf("tag-%d", i)
		assert.True(t, tagSet[expectedTag], 
			"Tag %s should be present in final user", expectedTag)
	}
}

// testConcurrentUpdates tests updates under high concurrency
func testConcurrentUpdates(t *testing.T, container *TestRedisContainer) {
	cache := CreateCacheForTesting[map[string]int](container)
	defer cache.Close()
	
	ctx := context.Background()
	key := "concurrent-map"
	
	// Initialize map
	initialMap := make(map[string]int)
	err := cache.Set(ctx, key, initialMap, time.Hour)
	require.NoError(t, err)
	
	numGoroutines := 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	// Each goroutine updates a different field in the map
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			
			fieldName := fmt.Sprintf("field-%d", goroutineID)
			
			_, err := cache.Update(ctx, key, func(old map[string]int, exists bool) (map[string]int, error) {
				if !exists {
					return nil, fmt.Errorf("map should exist")
				}
				
				// Create new map with updated field
				newMap := make(map[string]int)
				for k, v := range old {
					newMap[k] = v
				}
				newMap[fieldName] = goroutineID
				return newMap, nil
			}, time.Hour)
			
			assert.NoError(t, err)
		}(i)
	}
	
	wg.Wait()
	
	// Verify final state
	finalMap, found, err := cache.Get(ctx, key)
	require.NoError(t, err)
	assert.True(t, found)
	
	// Should have all fields
	assert.Equal(t, numGoroutines, len(finalMap), 
		"All fields should be present in final map")
	
	// Verify all fields have correct values
	for i := 0; i < numGoroutines; i++ {
		fieldName := fmt.Sprintf("field-%d", i)
		value, exists := finalMap[fieldName]
		assert.True(t, exists, "Field %s should exist", fieldName)
		assert.Equal(t, i, value, "Field %s should have correct value", fieldName)
	}
}

// TestDistributedLockBehavior tests distributed lock mechanisms
func TestDistributedLockBehavior(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	
	t.Run("LockTimeout", func(t *testing.T) {
		testLockTimeout(t, container)
	})
	
	t.Run("LockContention", func(t *testing.T) {
		testLockContention(t, container)
	})
}

// testLockTimeout tests lock timeout behavior
func testLockTimeout(t *testing.T, container *TestRedisContainer) {
	// This test would require access to internal locking mechanisms
	// For now, we test indirectly through GetOrSet behavior
	
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	ctx := context.Background()
	key := "lock-timeout-test"
	
	// Test that operations don't hang indefinitely
	timeout := 5 * time.Second
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	loader := func(ctx context.Context) (string, error) {
		time.Sleep(100 * time.Millisecond)
		return "loaded", nil
	}
	
	start := time.Now()
	value, err := cache.GetOrSet(timeoutCtx, key, loader, time.Hour)
	duration := time.Since(start)
	
	require.NoError(t, err)
	assert.Equal(t, "loaded", value)
	assert.Less(t, duration, timeout, "Operation should complete before timeout")
}

// testLockContention tests behavior under lock contention
func testLockContention(t *testing.T, container *TestRedisContainer) {
	cache := CreateCacheForTesting[int](container)
	defer cache.Close()
	
	ctx := context.Background()
	key := "contention-test"
	
	numGoroutines := 200
	var successCount int64
	var errorCount int64
	var wg sync.WaitGroup
	
	wg.Add(numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			
			value, err := cache.GetOrSet(ctx, key, func(ctx context.Context) (int, error) {
				// Add small delay to increase contention
				time.Sleep(10 * time.Millisecond)
				return id, nil
			}, time.Hour)
			
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
			} else {
				atomic.AddInt64(&successCount, 1)
				// All successful operations should get the same value
				// (from the first goroutine that acquired the lock)
				assert.GreaterOrEqual(t, value, 0)
				assert.Less(t, value, numGoroutines)
			}
		}(i)
	}
	
	wg.Wait()
	
	// All operations should succeed
	assert.Equal(t, int64(numGoroutines), successCount, 
		"All operations should succeed")
	assert.Equal(t, int64(0), errorCount, 
		"No operations should error")
}

// TestRaceConditionDetection tests for race conditions in cache operations
func TestRaceConditionDetection(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	
	// This test should be run with -race flag to detect race conditions
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	ctx := context.Background()
	numGoroutines := 100
	numOperations := 50
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("race-test-%d-%d", goroutineID, j)
				value := fmt.Sprintf("value-%d-%d", goroutineID, j)
				
				// Mix of operations to create potential race conditions
				switch j % 4 {
				case 0:
					err := cache.Set(ctx, key, value, time.Hour)
					assert.NoError(t, err)
					
				case 1:
					_, _, err := cache.Get(ctx, key)
					assert.NoError(t, err)
					
				case 2:
					err := cache.Delete(ctx, key)
					assert.NoError(t, err)
					
				case 3:
					exists := cache.Has(ctx, key)
					_ = exists // Don't assert - key may or may not exist
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// If we get here without race detector complaints, test passes
	t.Log("Race condition test completed successfully")
}
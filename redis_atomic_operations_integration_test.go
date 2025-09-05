//go:build integration

package cache_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/internal/testintegration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==============================================================================
// GETORSET INTEGRATION TESTS
// ==============================================================================

// TestRedisCache_GetOrSet_CacheMiss tests GetOrSet when key doesn't exist
func TestRedisCache_GetOrSet_CacheMiss(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create cache")
	defer cache.Close()

	testKey := "session:getorset:miss"
	expectedSession := &testintegration.TestSession{
		ID:       testKey,
		UserID:   "user123",
		Username: "testuser",
		Created:  time.Now().Truncate(time.Second), // Truncate for comparison precision
	}

	// Track loader calls
	var loaderCallCount int64
	loader := func(ctx context.Context) (*testintegration.TestSession, error) {
		atomic.AddInt64(&loaderCallCount, 1)
		t.Logf("💡 Loader called for cache miss")
		return expectedSession, nil
	}

	t.Logf("🔍 Testing GetOrSet cache miss scenario")

	// Call GetOrSet - should trigger loader
	result, err := cache.GetOrSet(ctx, testKey, loader, 5*time.Minute)

	// Assertions
	assert.NoError(t, err, "GetOrSet should not error on cache miss")
	assert.NotNil(t, result, "GetOrSet should return loaded value")
	assert.Equal(t, expectedSession.ID, result.ID, "Result should match loaded session ID")
	assert.Equal(t, expectedSession.UserID, result.UserID, "Result should match loaded session UserID")
	assert.Equal(t, int64(1), atomic.LoadInt64(&loaderCallCount), "Loader should be called exactly once")

	// Verify value was stored in cache
	cachedSession, found, err := cache.Get(ctx, testKey)
	assert.NoError(t, err, "Follow-up GET should not error")
	assert.True(t, found, "Session should be found in cache after GetOrSet")
	assert.Equal(t, expectedSession.ID, cachedSession.ID, "Cached session should match original")

	t.Logf("✅ GetOrSet cache miss test successful")
}

// TestRedisCache_GetOrSet_CacheHit tests GetOrSet when key exists
func TestRedisCache_GetOrSet_CacheHit(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create cache")
	defer cache.Close()

	testKey := "session:getorset:hit"
	existingSession := &testintegration.TestSession{
		ID:       testKey,
		UserID:   "existing123",
		Username: "existinguser",
		Created:  time.Now().Truncate(time.Second),
	}

	// Pre-populate cache
	err = cache.Set(ctx, existingSession, 5*time.Minute)
	require.NoError(t, err, "Failed to pre-populate cache")

	// Track loader calls - should not be called
	var loaderCallCount int64
	loader := func(ctx context.Context) (*testintegration.TestSession, error) {
		atomic.AddInt64(&loaderCallCount, 1)
		t.Errorf("❌ Loader should not be called for cache hit")
		return &testintegration.TestSession{
			ID:       testKey,
			UserID:   "should-not-be-loaded",
			Username: "should-not-be-loaded",
			Created:  time.Now(),
		}, nil
	}

	t.Logf("🎯 Testing GetOrSet cache hit scenario")

	// Call GetOrSet - should return existing value without calling loader
	result, err := cache.GetOrSet(ctx, testKey, loader, 5*time.Minute)

	// Assertions
	assert.NoError(t, err, "GetOrSet should not error on cache hit")
	assert.NotNil(t, result, "GetOrSet should return existing value")
	assert.Equal(t, existingSession.ID, result.ID, "Result should match existing session ID")
	assert.Equal(t, existingSession.UserID, result.UserID, "Result should match existing session UserID")
	assert.Equal(t, "existinguser", result.Username, "Result should match existing session Username")
	assert.Equal(t, int64(0), atomic.LoadInt64(&loaderCallCount), "Loader should not be called for cache hit")

	t.Logf("✅ GetOrSet cache hit test successful")
}

// TestRedisCache_GetOrSet_ConcurrentCacheMiss tests GetOrSet race conditions on same key
func TestRedisCache_GetOrSet_ConcurrentCacheMiss(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create cache")
	defer cache.Close()

	testKey := "session:getorset:concurrent"
	expectedSession := &testintegration.TestSession{
		ID:       testKey,
		UserID:   "concurrent123",
		Username: "concurrentuser",
		Created:  time.Now().Truncate(time.Second),
	}

	// Track loader calls - should be called exactly once despite concurrent access
	var loaderCallCount int64
	var loaderDuration time.Duration = 50 * time.Millisecond // Simulate some work

	loader := func(ctx context.Context) (*testintegration.TestSession, error) {
		callNumber := atomic.AddInt64(&loaderCallCount, 1)
		t.Logf("💡 Loader call #%d started", callNumber)

		// Simulate some work to increase likelihood of race conditions
		time.Sleep(loaderDuration)

		t.Logf("💡 Loader call #%d completed", callNumber)
		return expectedSession, nil
	}

	const numGoroutines = 10
	t.Logf("🏁 Testing GetOrSet with %d concurrent goroutines on same key", numGoroutines)

	// Use channels to coordinate goroutines
	startSignal := make(chan struct{})
	results := make(chan *testintegration.TestSession, numGoroutines)
	errors := make(chan error, numGoroutines)

	// Start goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			// Wait for start signal to maximize concurrency
			<-startSignal

			t.Logf("🚀 Goroutine %d starting GetOrSet", goroutineID)
			result, err := cache.GetOrSet(ctx, testKey, loader, 5*time.Minute)

			if err != nil {
				t.Logf("❌ Goroutine %d error: %v", goroutineID, err)
				errors <- err
			} else {
				t.Logf("✅ Goroutine %d completed successfully", goroutineID)
				results <- result
			}
		}(i)
	}

	// Start all goroutines simultaneously
	close(startSignal)

	// Collect results with timeout
	timeout := time.After(10 * time.Second)
	var successCount int
	var errorCount int

	for i := 0; i < numGoroutines; i++ {
		select {
		case result := <-results:
			successCount++
			// Verify each result matches expected value
			assert.Equal(t, expectedSession.ID, result.ID, "Result should match expected session ID")
			assert.Equal(t, expectedSession.UserID, result.UserID, "Result should match expected session UserID")

		case err := <-errors:
			errorCount++
			t.Errorf("Goroutine error: %v", err)

		case <-timeout:
			t.Fatalf("Test timeout - deadlock or excessive wait time")
		}
	}

	// Critical assertion: loader should be called exactly once
	finalLoaderCallCount := atomic.LoadInt64(&loaderCallCount)
	assert.Equal(t, int64(1), finalLoaderCallCount, "Loader should be called exactly once despite concurrent access (singleflight behavior)")
	assert.Equal(t, numGoroutines, successCount, "All goroutines should succeed")
	assert.Equal(t, 0, errorCount, "No goroutines should error")

	// Verify final state in cache
	cachedSession, found, err := cache.Get(ctx, testKey)
	assert.NoError(t, err, "Final GET should not error")
	assert.True(t, found, "Session should exist in cache")
	assert.Equal(t, expectedSession.ID, cachedSession.ID, "Final cached session should match expected")

	t.Logf("✅ GetOrSet concurrent test successful - %d goroutines, 1 loader call", numGoroutines)
}

// TestRedisCache_GetOrSet_LoaderError tests GetOrSet when loader function fails
func TestRedisCache_GetOrSet_LoaderError(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create cache")
	defer cache.Close()

	testKey := "session:getorset:error"
	expectedError := errors.New("simulated loader failure")

	// Loader that always fails
	var loaderCallCount int64
	loader := func(ctx context.Context) (*testintegration.TestSession, error) {
		atomic.AddInt64(&loaderCallCount, 1)
		t.Logf("💥 Loader called and failing intentionally")
		return nil, expectedError
	}

	t.Logf("💥 Testing GetOrSet loader error scenario")

	// Call GetOrSet - should return loader error
	result, err := cache.GetOrSet(ctx, testKey, loader, 5*time.Minute)

	// Assertions
	assert.Error(t, err, "GetOrSet should return error when loader fails")
	assert.Contains(t, err.Error(), "loader function failed", "Error should indicate loader failure")
	assert.Nil(t, result, "GetOrSet should return nil on loader error")
	assert.Equal(t, int64(1), atomic.LoadInt64(&loaderCallCount), "Loader should be called once")

	// Verify nothing was stored in cache
	cachedSession, found, err := cache.Get(ctx, testKey)
	assert.NoError(t, err, "Follow-up GET should not error")
	assert.False(t, found, "Nothing should be cached when loader fails")
	assert.Nil(t, cachedSession, "Cached value should be nil")

	t.Logf("✅ GetOrSet loader error test successful")
}

// ==============================================================================
// UPDATE INTEGRATION TESTS
// ==============================================================================

// TestRedisCache_Update_ExistingKey tests Update on existing key
// REMOVED: All Update method tests have been removed due to fundamental race conditions.
// The Update method has been replaced with truly atomic operations:
// - Increment(ctx, key, delta) for numeric increments
// - Decrement(ctx, key, delta) for numeric decrements
// - ExtendTTL(ctx, key, ttl) for TTL extension
// - Touch(ctx, key, ttl) for activity tracking + TTL extension
// - AppendToField(ctx, key, fieldPath, value, ttl) for string appends

// TestRedisCache_Update_AtomicReadModifyWrite tests that Update provides atomic read-modify-write semantics
func TestRedisCache_Increment_AtomicCounter(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create cache")
	defer cache.Close()

	testKey := "counter:atomic:increment"

	const numGoroutines = 10
	const incrementsPerGoroutine = 5
	const expectedFinalCounter = numGoroutines * incrementsPerGoroutine

	// Track successful increments
	var successfulIncrements int64

	t.Logf("🔢 Testing Increment atomic counter with %d goroutines", numGoroutines)

	// Use channels to coordinate goroutines
	startSignal := make(chan struct{})
	var wg sync.WaitGroup

	// Start goroutines that increment the counter
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			// Wait for start signal
			<-startSignal

			// Perform multiple increments
			for i := 0; i < incrementsPerGoroutine; i++ {
				result, err := cache.Increment(ctx, testKey, 1)
				if err != nil {
					t.Errorf("Goroutine %d increment %d error: %v", goroutineID, i+1, err)
				} else {
					atomic.AddInt64(&successfulIncrements, 1)
					t.Logf("✅ Goroutine %d increment %d: counter-%d", goroutineID, i+1, result)
				}
			}
		}(g)
	}

	// Start all goroutines simultaneously
	close(startSignal)

	// Wait for all goroutines to complete
	wg.Wait()

	// Assertions
	assert.Equal(t, int64(expectedFinalCounter), successfulIncrements, "All increments should succeed")

	// Verify final counter value - this is the critical test for atomicity
	finalCounterValue, err := cache.Increment(ctx, testKey, 0) // Add 0 to get current value
	assert.NoError(t, err, "Final counter read should not error")

	// This is the key assertion: if Increment provides proper atomicity,
	// the final counter should exactly equal the number of increments
	assert.Equal(t, int64(expectedFinalCounter), finalCounterValue,
		"Final counter should equal total increments (proves atomicity)")

	t.Logf("✅ Increment atomic counter test successful")
	t.Logf("📊 Expected final counter: %d, Actual final counter: %d", expectedFinalCounter, finalCounterValue)
}

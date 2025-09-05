//go:build integration

package cache_test

import (
	"context"
	"errors"
	"fmt"
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
func TestRedisCache_Update_ExistingKey(t *testing.T) {
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

	testKey := "session:update:existing"
	originalSession := &testintegration.TestSession{
		ID:       testKey,
		UserID:   "user123",
		Username: "originaluser",
		Created:  time.Now().Truncate(time.Second),
	}

	// Pre-populate cache
	err = cache.Set(ctx, originalSession, 5*time.Minute)
	require.NoError(t, err, "Failed to pre-populate cache")

	// Track updater calls
	var updaterCallCount int64
	updater := func(old *testintegration.TestSession, exists bool) (*testintegration.TestSession, error) {
		atomic.AddInt64(&updaterCallCount, 1)
		t.Logf("🔄 Updater called - exists: %v, old username: %s", exists, old.Username)
		
		// Verify we received the existing value
		assert.True(t, exists, "Updater should receive exists=true for existing key")
		assert.Equal(t, originalSession.Username, old.Username, "Updater should receive original session")
		
		// Return updated session
		updated := *old // Copy
		updated.Username = "updateduser"
		return &updated, nil
	}

	t.Logf("🔄 Testing Update on existing key")

	// Call Update
	result, err := cache.Update(ctx, testKey, updater, 5*time.Minute)
	
	// Assertions
	assert.NoError(t, err, "Update should not error on existing key")
	assert.NotNil(t, result, "Update should return updated value")
	assert.Equal(t, originalSession.ID, result.ID, "Updated session should maintain same ID")
	assert.Equal(t, originalSession.UserID, result.UserID, "Updated session should maintain same UserID")
	assert.Equal(t, "updateduser", result.Username, "Updated session should have new username")
	assert.Equal(t, int64(1), atomic.LoadInt64(&updaterCallCount), "Updater should be called exactly once")

	// Verify updated value is stored in cache
	cachedSession, found, err := cache.Get(ctx, testKey)
	assert.NoError(t, err, "Follow-up GET should not error")
	assert.True(t, found, "Updated session should be found in cache")
	assert.Equal(t, "updateduser", cachedSession.Username, "Cached session should have updated username")

	t.Logf("✅ Update existing key test successful")
}

// TestRedisCache_Update_NonExistentKey tests Update on non-existent key
func TestRedisCache_Update_NonExistentKey(t *testing.T) {
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

	testKey := "session:update:nonexistent"
	newSession := &testintegration.TestSession{
		ID:       testKey,
		UserID:   "user456",
		Username: "newuser",
		Created:  time.Now().Truncate(time.Second),
	}

	// Track updater calls
	var updaterCallCount int64
	updater := func(old *testintegration.TestSession, exists bool) (*testintegration.TestSession, error) {
		atomic.AddInt64(&updaterCallCount, 1)
		t.Logf("🆕 Updater called - exists: %v", exists)
		
		// Verify we received exists=false
		assert.False(t, exists, "Updater should receive exists=false for non-existent key")
		assert.Nil(t, old, "Updater should receive nil for non-existent key")
		
		// Return new session
		return newSession, nil
	}

	t.Logf("🆕 Testing Update on non-existent key")

	// Call Update
	result, err := cache.Update(ctx, testKey, updater, 5*time.Minute)
	
	// Assertions
	assert.NoError(t, err, "Update should not error on non-existent key")
	assert.NotNil(t, result, "Update should return new value")
	assert.Equal(t, newSession.ID, result.ID, "Result should match new session")
	assert.Equal(t, newSession.UserID, result.UserID, "Result should match new session")
	assert.Equal(t, newSession.Username, result.Username, "Result should match new session")
	assert.Equal(t, int64(1), atomic.LoadInt64(&updaterCallCount), "Updater should be called exactly once")

	// Verify new value is stored in cache
	cachedSession, found, err := cache.Get(ctx, testKey)
	assert.NoError(t, err, "Follow-up GET should not error")
	assert.True(t, found, "New session should be found in cache")
	assert.Equal(t, newSession.Username, cachedSession.Username, "Cached session should match new session")

	t.Logf("✅ Update non-existent key test successful")
}

// TestRedisCache_Update_ConcurrentUpdates tests Update race conditions
func TestRedisCache_Update_ConcurrentUpdates(t *testing.T) {
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

	testKey := "session:update:concurrent"
	initialSession := &testintegration.TestSession{
		ID:       testKey,
		UserID:   "user789",
		Username: "initial",
		Created:  time.Now().Truncate(time.Second),
	}

	// Pre-populate cache with initial value
	err = cache.Set(ctx, initialSession, 5*time.Minute)
	require.NoError(t, err, "Failed to pre-populate cache")

	const numGoroutines = 5
	const incrementsPerGoroutine = 3
	const totalExpectedIncrements = numGoroutines * incrementsPerGoroutine

	// Track total updater calls across all goroutines
	var totalUpdaterCalls int64
	
	// Create updater that appends goroutine ID to username (simulating concurrent updates)
	createUpdater := func(goroutineID int) func(*testintegration.TestSession, bool) (*testintegration.TestSession, error) {
		return func(old *testintegration.TestSession, exists bool) (*testintegration.TestSession, error) {
			atomic.AddInt64(&totalUpdaterCalls, 1)
			
			assert.True(t, exists, "Key should exist for concurrent update test")
			assert.NotNil(t, old, "Old value should not be nil")
			
			// Simulate some processing time to increase chance of race conditions
			time.Sleep(10 * time.Millisecond)
			
			// Append goroutine marker to username
			updated := *old // Copy
			updated.Username = fmt.Sprintf("%s-g%d", old.Username, goroutineID)
			
			t.Logf("🔄 Goroutine %d updating: %s -> %s", goroutineID, old.Username, updated.Username)
			return &updated, nil
		}
	}

	t.Logf("🏁 Testing Update with %d concurrent goroutines (%d updates each)", numGoroutines, incrementsPerGoroutine)

	// Use channels to coordinate goroutines
	startSignal := make(chan struct{})
	results := make(chan *testintegration.TestSession, totalExpectedIncrements)
	errors := make(chan error, totalExpectedIncrements)

	// Start goroutines
	var wg sync.WaitGroup
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			updater := createUpdater(goroutineID)
			
			// Wait for start signal
			<-startSignal
			
			// Perform multiple updates from this goroutine
			for i := 0; i < incrementsPerGoroutine; i++ {
				t.Logf("🚀 Goroutine %d starting update %d", goroutineID, i+1)
				result, err := cache.Update(ctx, testKey, updater, 5*time.Minute)
				
				if err != nil {
					t.Logf("❌ Goroutine %d update %d error: %v", goroutineID, i+1, err)
					errors <- err
				} else {
					t.Logf("✅ Goroutine %d update %d success: %s", goroutineID, i+1, result.Username)
					results <- result
				}
			}
		}(g)
	}

	// Start all goroutines simultaneously
	close(startSignal)

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	// Collect results
	var successCount int
	var errorCount int

	// Process all results
	for result := range results {
		successCount++
		assert.NotNil(t, result, "Update result should not be nil")
		assert.Contains(t, result.Username, "g", "Username should contain goroutine marker")
	}

	// Process all errors
	for err := range errors {
		errorCount++
		t.Errorf("Update error: %v", err)
	}

	// Assertions
	assert.Equal(t, totalExpectedIncrements, successCount, "All updates should succeed")
	assert.Equal(t, 0, errorCount, "No updates should error")
	assert.Equal(t, int64(totalExpectedIncrements), atomic.LoadInt64(&totalUpdaterCalls), "All updater calls should complete")

	// Verify final state - should contain markers from all goroutines
	finalSession, found, err := cache.Get(ctx, testKey)
	assert.NoError(t, err, "Final GET should not error")
	assert.True(t, found, "Final session should exist")
	assert.NotEqual(t, "initial", finalSession.Username, "Username should be different from initial")
	assert.Contains(t, finalSession.Username, "g", "Final username should contain goroutine marker")

	t.Logf("✅ Update concurrent test successful - %d total updates completed", totalExpectedIncrements)
	t.Logf("📊 Final username: %s", finalSession.Username)
}

// TestRedisCache_Update_UpdaterError tests Update when updater function fails
func TestRedisCache_Update_UpdaterError(t *testing.T) {
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

	testKey := "session:update:error"
	originalSession := &testintegration.TestSession{
		ID:       testKey,
		UserID:   "user999",
		Username: "original",
		Created:  time.Now().Truncate(time.Second),
	}

	// Pre-populate cache
	err = cache.Set(ctx, originalSession, 5*time.Minute)
	require.NoError(t, err, "Failed to pre-populate cache")

	expectedError := errors.New("simulated updater failure")

	// Updater that always fails
	var updaterCallCount int64
	updater := func(old *testintegration.TestSession, exists bool) (*testintegration.TestSession, error) {
		atomic.AddInt64(&updaterCallCount, 1)
		t.Logf("💥 Updater called and failing intentionally")
		
		// Verify we received the existing value before failing
		assert.True(t, exists, "Updater should receive exists=true")
		assert.Equal(t, originalSession.Username, old.Username, "Updater should receive original session")
		
		return nil, expectedError
	}

	t.Logf("💥 Testing Update updater error scenario")

	// Call Update - should return updater error
	result, err := cache.Update(ctx, testKey, updater, 5*time.Minute)
	
	// Assertions
	assert.Error(t, err, "Update should return error when updater fails")
	assert.Contains(t, err.Error(), "updater function failed", "Error should indicate updater failure")
	assert.Nil(t, result, "Update should return nil on updater error")
	assert.Equal(t, int64(1), atomic.LoadInt64(&updaterCallCount), "Updater should be called once")

	// Verify original value is still in cache (transaction should be rolled back)
	cachedSession, found, err := cache.Get(ctx, testKey)
	assert.NoError(t, err, "Follow-up GET should not error")
	assert.True(t, found, "Original session should still exist in cache")
	assert.Equal(t, originalSession.Username, cachedSession.Username, "Cached session should still have original username")

	t.Logf("✅ Update updater error test successful - original value preserved")
}

// TestRedisCache_Update_AtomicReadModifyWrite tests that Update provides atomic read-modify-write semantics
func TestRedisCache_Update_AtomicReadModifyWrite(t *testing.T) {
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

	testKey := "session:update:atomic"
	
	// Create initial session with a counter in the username
	initialSession := &testintegration.TestSession{
		ID:       testKey,
		UserID:   "counteruser",
		Username: "counter-0", // Start with counter at 0
		Created:  time.Now().Truncate(time.Second),
	}

	// Pre-populate cache
	err = cache.Set(ctx, initialSession, 5*time.Minute)
	require.NoError(t, err, "Failed to pre-populate cache")

	const numGoroutines = 10
	const incrementsPerGoroutine = 5
	const expectedFinalCounter = numGoroutines * incrementsPerGoroutine

	// Track successful increments
	var successfulIncrements int64
	
	// Updater that increments counter in username
	incrementUpdater := func(old *testintegration.TestSession, exists bool) (*testintegration.TestSession, error) {
		if !exists {
			return nil, errors.New("session should exist for increment test")
		}
		
		// Parse current counter from username
		var currentCounter int
		_, err := fmt.Sscanf(old.Username, "counter-%d", &currentCounter)
		if err != nil {
			return nil, fmt.Errorf("failed to parse counter: %w", err)
		}
		
		// Increment counter
		newCounter := currentCounter + 1
		
		// Simulate some processing time to increase chance of race conditions
		time.Sleep(5 * time.Millisecond)
		
		updated := *old // Copy
		updated.Username = fmt.Sprintf("counter-%d", newCounter)
		
		return &updated, nil
	}

	t.Logf("🔢 Testing Update atomic read-modify-write with %d goroutines", numGoroutines)

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
				result, err := cache.Update(ctx, testKey, incrementUpdater, 5*time.Minute)
				if err != nil {
					t.Errorf("Goroutine %d increment %d error: %v", goroutineID, i+1, err)
				} else {
					atomic.AddInt64(&successfulIncrements, 1)
					t.Logf("✅ Goroutine %d increment %d: %s", goroutineID, i+1, result.Username)
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
	finalSession, found, err := cache.Get(ctx, testKey)
	assert.NoError(t, err, "Final GET should not error")
	assert.True(t, found, "Final session should exist")

	// Parse final counter
	var finalCounter int
	_, err = fmt.Sscanf(finalSession.Username, "counter-%d", &finalCounter)
	assert.NoError(t, err, "Should be able to parse final counter")

	// This is the key assertion: if Update provides proper atomicity,
	// the final counter should exactly equal the number of increments
	assert.Equal(t, expectedFinalCounter, finalCounter, 
		"Final counter should equal total increments (proves atomicity)")

	t.Logf("✅ Update atomic read-modify-write test successful")
	t.Logf("📊 Expected final counter: %d, Actual final counter: %d", expectedFinalCounter, finalCounter)
}
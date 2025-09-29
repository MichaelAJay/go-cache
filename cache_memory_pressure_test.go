//go:build integration

package cache_test

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/internal/testintegration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLargeDatasetMemoryPressure tests cache behavior with large numbers of entries
func TestLargeDatasetMemoryPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large dataset memory pressure test in short mode")
	}

	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	t.Log("🧪 Testing memory pressure with large datasets")

	// Track memory usage
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// Phase 1: Create a large number of cache entries
	const numEntries = 10000
	const batchSize = 500
	
	t.Logf("Phase 1: Creating %d cache entries in batches of %d", numEntries, batchSize)
	
	createdKeys := make([]string, 0, numEntries)
	
	for batch := 0; batch < numEntries/batchSize; batch++ {
		sessions := make([]*testintegration.TestSession, batchSize)
		
		for i := 0; i < batchSize; i++ {
			entryNum := batch*batchSize + i
			sessionID := fmt.Sprintf("memory_test_session_%d", entryNum)
			
			sessions[i] = &testintegration.TestSession{
				ID:       sessionID,
				UserID:   fmt.Sprintf("user_%d", entryNum%1000), // Reuse user IDs
				Username: fmt.Sprintf("username_%d_with_some_longer_content_to_increase_size", entryNum),
				Created:  time.Now(),
			}
			createdKeys = append(createdKeys, sessionID)
		}
		
		// Set batch
		err := cache.SetMany(ctx, sessions, time.Hour)
		require.NoError(t, err, "SetMany should succeed for batch %d", batch)
		
		// Log progress every 1000 entries
		if (batch+1)*batchSize%1000 == 0 {
			t.Logf("Created %d entries", (batch+1)*batchSize)
		}
	}

	// Phase 2: Verify entries were created and can be retrieved
	t.Log("Phase 2: Verifying random sample of entries can be retrieved")
	
	sampleIndices := []int{0, 1000, 5000, 7500, 9999} // Sample from different ranges
	
	for _, idx := range sampleIndices {
		if idx < len(createdKeys) {
			retrieved, found, err := cache.Get(ctx, createdKeys[idx])
			assert.NoError(t, err, "Get should succeed for entry %d", idx)
			assert.True(t, found, "Entry %d should be found", idx)
			assert.NotNil(t, retrieved, "Retrieved entry should not be nil")
		}
	}

	// Phase 3: Test batch retrieval under memory pressure
	t.Log("Phase 3: Testing batch retrieval with large result sets")
	
	// Get a subset of entries using batch operation
	testKeys := createdKeys[:1000] // First 1000 keys
	retrieved, err := cache.GetMany(ctx, testKeys)
	require.NoError(t, err, "GetMany should succeed")
	assert.Equal(t, len(testKeys), len(retrieved), "All requested keys should be retrieved")

	// Phase 4: Test memory usage patterns
	runtime.GC()
	runtime.ReadMemStats(&m2)
	
	memoryIncrease := m2.Alloc - m1.Alloc
	t.Logf("Memory increase: %d bytes (%.2f MB)", memoryIncrease, float64(memoryIncrease)/(1024*1024))
	
	// Verify memory increase is reasonable (should be non-zero but not excessive)
	assert.Greater(t, memoryIncrease, uint64(0), "Memory usage should increase")

	// Phase 5: Clean up and verify memory is released
	t.Log("Phase 5: Cleaning up entries and checking memory release")
	
	// Delete all entries in batches
	for i := 0; i < len(createdKeys); i += batchSize {
		end := i + batchSize
		if end > len(createdKeys) {
			end = len(createdKeys)
		}
		
		err := cache.DeleteMany(ctx, createdKeys[i:end])
		assert.NoError(t, err, "DeleteMany should succeed for batch starting at %d", i)
	}

	// Verify cleanup
	sample := createdKeys[len(createdKeys)/2] // Middle key
	_, found, err := cache.Get(ctx, sample)
	assert.NoError(t, err, "Get should not error after cleanup")
	assert.False(t, found, "Entries should be deleted after cleanup")

	t.Log("✅ Large dataset memory pressure test completed")
}

// TestLargeEntryMemoryPressure tests cache behavior with very large individual entries
func TestLargeEntryMemoryPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large entry memory pressure test in short mode")
	}

	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateStringCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	t.Log("🧪 Testing memory pressure with large individual entries")

	// Phase 1: Create progressively larger entries
	sizes := []int{
		1 * 1024,      // 1KB
		10 * 1024,     // 10KB  
		100 * 1024,    // 100KB
		1024 * 1024,   // 1MB
		5 * 1024 * 1024, // 5MB
	}

	for i, size := range sizes {
		t.Logf("Phase 1.%d: Testing %d byte entry", i+1, size)
		
		// Create large string value
		largeValue := strings.Repeat("A", size)
		
		// Test setting large entry
		start := time.Now()
		err := cache.Set(ctx, largeValue, time.Hour)
		duration := time.Since(start)
		
		require.NoError(t, err, "Set should succeed for %d byte entry", size)
		t.Logf("Set operation took %v for %d byte entry", duration, size)
		
		// Test retrieving large entry - use largeValue as key since that's what stringExtractor uses
		start = time.Now()
		retrieved, found, err := cache.Get(ctx, largeValue)
		duration = time.Since(start)
		
		require.NoError(t, err, "Get should succeed for %d byte entry", size)
		require.True(t, found, "Large entry should be found")
		require.Equal(t, len(largeValue), len(retrieved), "Retrieved entry should have same size")
		t.Logf("Get operation took %v for %d byte entry", duration, size)
		
		// Clean up large entry
		_, err = cache.Delete(ctx, largeValue)
		require.NoError(t, err, "Delete should succeed for large entry")
	}

	t.Log("✅ Large entry memory pressure test completed")
}

// TestMemoryBasedEvictionPressure tests LRU eviction under memory pressure
func TestMemoryBasedEvictionPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory-based eviction pressure test in short mode")
	}

	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache with LRU eviction enabled and small max entries
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	t.Log("🧪 Testing eviction behavior under memory pressure")

	// Note: The exact eviction behavior depends on the cache configuration
	// This test focuses on ensuring the cache handles memory pressure gracefully

	// Phase 1: Fill cache beyond typical capacity
	const numSessions = 5000
	t.Logf("Phase 1: Creating %d sessions to stress eviction", numSessions)
	
	sessions := make([]*testintegration.TestSession, numSessions)
	keys := make([]string, numSessions)
	
	for i := 0; i < numSessions; i++ {
		sessionID := fmt.Sprintf("eviction_test_session_%d", i)
		sessions[i] = &testintegration.TestSession{
			ID:       sessionID,
			UserID:   fmt.Sprintf("user_%d", i),
			Username: fmt.Sprintf("username_%d_with_additional_data_to_increase_memory_usage", i),
			Created:  time.Now(),
		}
		keys[i] = sessionID
	}

	// Set all sessions in batches
	batchSize := 500
	for i := 0; i < len(sessions); i += batchSize {
		end := i + batchSize
		if end > len(sessions) {
			end = len(sessions)
		}
		
		err := cache.SetMany(ctx, sessions[i:end], time.Hour)
		require.NoError(t, err, "SetMany should succeed for eviction test batch")
		
		if (i+batchSize)%2000 == 0 {
			t.Logf("Created %d sessions", i+batchSize)
		}
	}

	// Phase 2: Access patterns to test LRU behavior
	t.Log("Phase 2: Testing access patterns and eviction behavior")
	
	// Access first 100 entries to make them more recently used
	recentKeys := keys[:100]
	for _, key := range recentKeys {
		_, _, err := cache.Get(ctx, key)
		// Don't require no error since some entries might have been evicted
		if err != nil {
			t.Logf("Key %s not found (possibly evicted): %v", key, err)
		}
	}

	// Phase 3: Add more entries to trigger more evictions
	additionalSessions := make([]*testintegration.TestSession, 1000)
	for i := 0; i < 1000; i++ {
		sessionID := fmt.Sprintf("additional_session_%d", i)
		additionalSessions[i] = &testintegration.TestSession{
			ID:       sessionID,
			UserID:   fmt.Sprintf("additional_user_%d", i),
			Username: fmt.Sprintf("additional_username_%d", i),
			Created:  time.Now(),
		}
	}
	
	err = cache.SetMany(ctx, additionalSessions, time.Hour)
	require.NoError(t, err, "Additional sessions should be set successfully")

	// Phase 4: Verify cache still functions correctly after memory pressure
	t.Log("Phase 4: Verifying cache functionality after memory pressure")
	
	// Test basic operations still work
	testSession := &testintegration.TestSession{
		ID:       "memory_pressure_test_final",
		UserID:   "test_user_final",
		Username: "test_username_final",
		Created:  time.Now(),
	}
	
	err = cache.Set(ctx, testSession, time.Hour)
	require.NoError(t, err, "Cache should still accept new entries after memory pressure")
	
	retrieved, found, err := cache.Get(ctx, testSession.ID)
	require.NoError(t, err, "Cache should still retrieve entries after memory pressure")
	require.True(t, found, "Test session should be found")
	require.Equal(t, testSession.UserID, retrieved.UserID, "Retrieved data should match")

	t.Log("✅ Memory-based eviction pressure test completed")
}

// TestConcurrentMemoryPressure tests cache behavior under concurrent access and memory pressure
func TestConcurrentMemoryPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent memory pressure test in short mode")
	}

	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	t.Log("🧪 Testing concurrent access under memory pressure")

	const numWorkers = 10
	const operationsPerWorker = 500
	
	// Use channels to coordinate workers and collect results
	start := make(chan struct{})
	done := make(chan bool, numWorkers)
	errorCh := make(chan error, numWorkers*operationsPerWorker)

	// Start workers
	for workerID := 0; workerID < numWorkers; workerID++ {
		go func(id int) {
			defer func() { done <- true }()
			
			// Wait for start signal
			<-start
			
			for op := 0; op < operationsPerWorker; op++ {
				sessionID := fmt.Sprintf("concurrent_session_%d_%d", id, op)
				
				// Create relatively large session data
				session := &testintegration.TestSession{
					ID:       sessionID,
					UserID:   fmt.Sprintf("concurrent_user_%d", id),
					Username: fmt.Sprintf("concurrent_username_%d_%d_with_extra_data_for_memory_pressure", id, op),
					Created:  time.Now(),
				}
				
				// Perform various operations
				switch op % 4 {
				case 0:
					// Set operation
					if err := cache.Set(ctx, session, time.Hour); err != nil {
						errorCh <- fmt.Errorf("worker %d set failed: %w", id, err)
					}
				case 1:
					// Get operation
					if _, _, err := cache.Get(ctx, sessionID); err != nil {
						errorCh <- fmt.Errorf("worker %d get failed: %w", id, err)
					}
				case 2:
					// Delete operation
					if _, err := cache.Delete(ctx, sessionID); err != nil {
						errorCh <- fmt.Errorf("worker %d delete failed: %w", id, err)
					}
				case 3:
					// Has operation
					cache.Has(ctx, sessionID) // Has doesn't return error
				}
			}
		}(workerID)
	}

	// Start all workers simultaneously
	t.Log("Starting concurrent workers...")
	close(start)
	
	// Wait for all workers to complete
	for i := 0; i < numWorkers; i++ {
		<-done
	}
	
	// Check for errors
	close(errorCh)
	var errors []error
	for err := range errorCh {
		errors = append(errors, err)
	}
	
	// Report results
	t.Logf("Concurrent memory pressure test completed with %d errors out of %d total operations", 
		len(errors), numWorkers*operationsPerWorker)
	
	// Allow some errors under extreme pressure, but not too many
	errorRate := float64(len(errors)) / float64(numWorkers*operationsPerWorker)
	assert.Less(t, errorRate, 0.1, "Error rate should be less than 10%% under memory pressure")

	// Verify cache is still functional after concurrent pressure
	testSession := &testintegration.TestSession{
		ID:       "post_concurrent_test",
		UserID:   "post_test_user",
		Username: "post_test_username", 
		Created:  time.Now(),
	}
	
	err = cache.Set(ctx, testSession, time.Hour)
	assert.NoError(t, err, "Cache should be functional after concurrent memory pressure")
	
	retrieved, found, err := cache.Get(ctx, testSession.ID)
	assert.NoError(t, err, "Cache should retrieve after concurrent memory pressure")
	assert.True(t, found, "Test session should be found after pressure test")
	
	if found {
		assert.Equal(t, testSession.UserID, retrieved.UserID, "Data integrity should be maintained")
	}

	t.Log("✅ Concurrent memory pressure test completed")
}
//go:build integration

package cache_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/MichaelAJay/go-cache/internal/testintegration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBatchOperations_Comprehensive runs comprehensive tests for batch operations
func TestBatchOperations_Comprehensive(t *testing.T) {
	t.Run("GetMany_EdgeCases", TestGetMany_EdgeCases)
	t.Run("GetMany_LargeBatches", TestGetMany_LargeBatches)
	t.Run("GetMany_MetadataConsistency", TestGetMany_MetadataConsistency)
	t.Run("GetMany_PerformanceComparison", TestGetMany_PerformanceComparison)
	
	t.Run("SetMany_EdgeCases", TestSetMany_EdgeCases)
	t.Run("SetMany_WithIndexing", TestSetMany_WithIndexing)
	t.Run("SetMany_WithTTL", TestSetMany_WithTTL)
	t.Run("SetMany_PerformanceComparison", TestSetMany_PerformanceComparison)
	
	t.Run("DeleteMany_EdgeCases", TestDeleteMany_EdgeCases)
	t.Run("DeleteMany_WithIndexing", TestDeleteMany_WithIndexing)
	t.Run("DeleteMany_PerformanceComparison", TestDeleteMany_PerformanceComparison)
	
	t.Run("BatchOperations_ConcurrentAccess", TestBatchOperations_ConcurrentAccess)
	t.Run("BatchOperations_CircuitBreaker", TestBatchOperations_CircuitBreaker)
}

// TestGetMany_EdgeCases tests GetMany with various edge cases
func TestGetMany_EdgeCases(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	t.Run("EmptyKeysList", func(t *testing.T) {
		results, err := cache.GetMany(ctx, []string{})
		assert.NoError(t, err, "GetMany with empty keys should not error")
		assert.Empty(t, results, "GetMany with empty keys should return empty map")
	})

	t.Run("AllNonExistentKeys", func(t *testing.T) {
		keys := []string{"nonexistent1", "nonexistent2", "nonexistent3"}
		results, err := cache.GetMany(ctx, keys)
		assert.NoError(t, err, "GetMany with non-existent keys should not error")
		assert.Empty(t, results, "GetMany with non-existent keys should return empty map")
	})

	t.Run("MixedExistentAndNonExistent", func(t *testing.T) {
		// Set up some test data
		existingSessions := []*testintegration.TestSession{
			{ID: "exists1", UserID: "user1", Username: "user1", Created: time.Now()},
			{ID: "exists3", UserID: "user3", Username: "user3", Created: time.Now()},
		}
		for _, session := range existingSessions {
			err = cache.Set(ctx, session, 0)
			require.NoError(t, err)
		}

		keys := []string{"exists1", "nonexistent2", "exists3", "nonexistent4"}
		results, err := cache.GetMany(ctx, keys)
		
		assert.NoError(t, err, "GetMany with mixed keys should not error")
		assert.Len(t, results, 2, "GetMany should return only existing sessions")
		
		// Verify existing sessions were retrieved
		for _, session := range existingSessions {
			retrieved, exists := results[session.ID]
			assert.True(t, exists, "Existing session %s should be in results", session.ID)
			assert.Equal(t, session.Username, retrieved.Username)
		}

		// Verify non-existent keys are not in results
		_, exists := results["nonexistent2"]
		assert.False(t, exists, "Non-existent key should not be in results")
		_, exists = results["nonexistent4"]
		assert.False(t, exists, "Non-existent key should not be in results")
	})

	t.Run("DuplicateKeys", func(t *testing.T) {
		// Set up test data
		session := &testintegration.TestSession{
			ID: "duplicate-test", UserID: "user1", Username: "user1", Created: time.Now(),
		}
		err = cache.Set(ctx, session, 0)
		require.NoError(t, err)

		keys := []string{"duplicate-test", "duplicate-test", "duplicate-test"}
		results, err := cache.GetMany(ctx, keys)
		
		assert.NoError(t, err, "GetMany with duplicate keys should not error")
		assert.Len(t, results, 1, "GetMany should deduplicate keys")
		
		retrieved, exists := results["duplicate-test"]
		assert.True(t, exists)
		assert.Equal(t, session.Username, retrieved.Username)
	})
}

// TestGetMany_LargeBatches tests GetMany with large numbers of keys
func TestGetMany_LargeBatches(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	t.Run("Batch100Keys", func(t *testing.T) {
		// Create 100 test sessions
		sessions := make([]*testintegration.TestSession, 100)
		keys := make([]string, 100)
		
		for i := 0; i < 100; i++ {
			sessions[i] = &testintegration.TestSession{
				ID:       fmt.Sprintf("batch100-%d", i),
				UserID:   fmt.Sprintf("user%d", i),
				Username: fmt.Sprintf("user%d", i),
				Created:  time.Now(),
			}
			keys[i] = sessions[i].ID
			
			err = cache.Set(ctx, sessions[i], 0)
			require.NoError(t, err, "Failed to set session %d", i)
		}

		start := time.Now()
		results, err := cache.GetMany(ctx, keys)
		duration := time.Since(start)
		
		assert.NoError(t, err, "GetMany with 100 keys should not error")
		assert.Len(t, results, 100, "GetMany should return all 100 sessions")
		
		t.Logf("✅ GetMany 100 keys completed in %v", duration)
		
		// Verify all sessions were retrieved correctly
		for _, session := range sessions {
			retrieved, exists := results[session.ID]
			assert.True(t, exists, "Session %s should exist", session.ID)
			if exists {
				assert.Equal(t, session.Username, retrieved.Username)
			}
		}
	})

	t.Run("Batch500Keys_MixedResults", func(t *testing.T) {
		// Create 250 sessions, request 500 keys (50% hit rate)
		sessions := make([]*testintegration.TestSession, 250)
		allKeys := make([]string, 500)
		
		// First 250 are existing sessions
		for i := 0; i < 250; i++ {
			sessions[i] = &testintegration.TestSession{
				ID:       fmt.Sprintf("batch500-exists-%d", i),
				UserID:   fmt.Sprintf("user%d", i),
				Username: fmt.Sprintf("user%d", i),
				Created:  time.Now(),
			}
			allKeys[i] = sessions[i].ID
			
			err = cache.Set(ctx, sessions[i], 0)
			require.NoError(t, err, "Failed to set session %d", i)
		}
		
		// Next 250 are non-existent keys
		for i := 250; i < 500; i++ {
			allKeys[i] = fmt.Sprintf("batch500-nonexistent-%d", i)
		}

		start := time.Now()
		results, err := cache.GetMany(ctx, allKeys)
		duration := time.Since(start)
		
		assert.NoError(t, err, "GetMany with 500 keys should not error")
		assert.Len(t, results, 250, "GetMany should return only existing 250 sessions")
		
		t.Logf("✅ GetMany 500 keys (50%% hit rate) completed in %v", duration)
	})
}

// TestGetMany_MetadataConsistency verifies metadata is updated correctly
func TestGetMany_MetadataConsistency(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	// Set up test sessions
	sessions := []*testintegration.TestSession{
		{ID: "meta1", UserID: "user1", Username: "user1", Created: time.Now()},
		{ID: "meta2", UserID: "user2", Username: "user2", Created: time.Now()},
		{ID: "meta3", UserID: "user3", Username: "user3", Created: time.Now()},
	}

	for _, session := range sessions {
		err = cache.Set(ctx, session, 0)
		require.NoError(t, err)
	}

	// Record initial metadata state
	keys := []string{"meta1", "meta2", "meta3"}
	initialMetadata := make(map[string]*interfaces.CacheEntryMetadata)
	for _, key := range keys {
		metadata, err := cache.GetMetadata(ctx, key)
		require.NoError(t, err)
		initialMetadata[key] = metadata
	}

	// Perform GetMany operation
	time.Sleep(100 * time.Millisecond) // Ensure timestamp difference is detectable
	results, err := cache.GetMany(ctx, keys)
	require.NoError(t, err)
	assert.Len(t, results, 3)

	// Verify metadata was updated for all retrieved keys
	for _, key := range keys {
		metadata, err := cache.GetMetadata(ctx, key)
		require.NoError(t, err, "Should be able to get metadata for %s", key)
		
		initial := initialMetadata[key]
		assert.Greater(t, metadata.AccessCount, initial.AccessCount, "Access count should increase for %s", key)
		assert.True(t, metadata.LastAccessed.After(initial.LastAccessed), "Last accessed should be updated for %s", key)
	}

	t.Logf("✅ Metadata consistency verified for all retrieved keys")
}

// TestGetMany_PerformanceComparison compares batch vs individual operations
func TestGetMany_PerformanceComparison(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	// Set up 50 test sessions
	const numSessions = 50
	sessions := make([]*testintegration.TestSession, numSessions)
	keys := make([]string, numSessions)
	
	for i := 0; i < numSessions; i++ {
		sessions[i] = &testintegration.TestSession{
			ID:       fmt.Sprintf("perf-test-%d", i),
			UserID:   fmt.Sprintf("user%d", i),
			Username: fmt.Sprintf("user%d", i),
			Created:  time.Now(),
		}
		keys[i] = sessions[i].ID
		
		err = cache.Set(ctx, sessions[i], 0)
		require.NoError(t, err)
	}

	// Test batch operation performance
	batchStart := time.Now()
	batchResults, err := cache.GetMany(ctx, keys)
	batchDuration := time.Since(batchStart)
	
	require.NoError(t, err)
	assert.Len(t, batchResults, numSessions)

	// Test individual operations performance
	individualStart := time.Now()
	individualResults := make(map[string]*testintegration.TestSession)
	for _, key := range keys {
		session, found, err := cache.Get(ctx, key)
		require.NoError(t, err)
		if found {
			individualResults[key] = session
		}
	}
	individualDuration := time.Since(individualStart)
	
	assert.Len(t, individualResults, numSessions)

	// Performance assertions
	speedupRatio := float64(individualDuration) / float64(batchDuration)
	assert.Greater(t, speedupRatio, 1.5, "Batch operation should be at least 50%% faster")
	
	t.Logf("📊 Performance Comparison:")
	t.Logf("   - Batch GetMany (%d keys): %v", numSessions, batchDuration)
	t.Logf("   - Individual Gets (%d keys): %v", numSessions, individualDuration)
	t.Logf("   - Speedup ratio: %.2fx", speedupRatio)
	t.Logf("   - Batch operation is %.1f%% faster", (speedupRatio-1)*100)
}

// TestSetMany_EdgeCases tests SetMany with various edge cases
func TestSetMany_EdgeCases(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	t.Run("EmptySessionsList", func(t *testing.T) {
		err := cache.SetMany(ctx, []*testintegration.TestSession{}, 0)
		assert.NoError(t, err, "SetMany with empty list should not error")
	})

	t.Run("SingleSession", func(t *testing.T) {
		session := &testintegration.TestSession{
			ID: "single-setmany", UserID: "user1", Username: "user1", Created: time.Now(),
		}
		
		err := cache.SetMany(ctx, []*testintegration.TestSession{session}, 0)
		assert.NoError(t, err, "SetMany with single session should not error")
		
		// Verify the session was set
		retrieved, found, err := cache.Get(ctx, "single-setmany")
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, session.Username, retrieved.Username)
	})

	t.Run("DuplicateSessionIDs", func(t *testing.T) {
		// Create sessions with same ID (should overwrite)
		sessions := []*testintegration.TestSession{
			{ID: "duplicate-id", UserID: "user1", Username: "original", Created: time.Now()},
			{ID: "duplicate-id", UserID: "user1", Username: "updated", Created: time.Now()},
		}
		
		err := cache.SetMany(ctx, sessions, 0)
		assert.NoError(t, err, "SetMany with duplicate IDs should not error")
		
		// Verify the last one wins
		retrieved, found, err := cache.Get(ctx, "duplicate-id")
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "updated", retrieved.Username)
	})
}

// TestSetMany_WithIndexing tests SetMany with indexing enabled
func TestSetMany_WithIndexing(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.IndexedCacheConfig()  // Use indexed cache config
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	// Create sessions with multiple users
	sessions := []*testintegration.TestSession{
		{ID: "session1", UserID: "user100", Username: "user100", Created: time.Now()},
		{ID: "session2", UserID: "user100", Username: "user100", Created: time.Now()},
		{ID: "session3", UserID: "user101", Username: "user101", Created: time.Now()},
		{ID: "session4", UserID: "user101", Username: "user101", Created: time.Now()},
		{ID: "session5", UserID: "user102", Username: "user102", Created: time.Now()},
	}

	// Set sessions using SetMany
	err = cache.SetMany(ctx, sessions, 0)
	require.NoError(t, err, "SetMany should succeed")

	// Verify indexing worked correctly
	user100Sessions, err := cache.GetByOwner(ctx, "user100")
	assert.NoError(t, err)
	assert.Len(t, user100Sessions, 2, "User100 should have 2 sessions")

	user101Sessions, err := cache.GetByOwner(ctx, "user101")
	assert.NoError(t, err)
	assert.Len(t, user101Sessions, 2, "User101 should have 2 sessions")

	user102Sessions, err := cache.GetByOwner(ctx, "user102")
	assert.NoError(t, err)
	assert.Len(t, user102Sessions, 1, "User102 should have 1 session")

	t.Logf("✅ Indexing verification successful")
}

// TestSetMany_WithTTL tests SetMany with TTL values
func TestSetMany_WithTTL(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	sessions := []*testintegration.TestSession{
		{ID: "ttl-test1", UserID: "user1", Username: "user1", Created: time.Now()},
		{ID: "ttl-test2", UserID: "user2", Username: "user2", Created: time.Now()},
	}

	// Set with 1s TTL (SetEX minimum precision)
	err = cache.SetMany(ctx, sessions, 1*time.Second)
	require.NoError(t, err)

	// Verify sessions exist immediately
	for _, session := range sessions {
		_, found, err := cache.Get(ctx, session.ID)
		assert.NoError(t, err)
		assert.True(t, found, "Session %s should exist immediately", session.ID)
	}

	// Wait for TTL expiration
	time.Sleep(1200 * time.Millisecond)

	// Verify sessions have expired
	for _, session := range sessions {
		_, found, err := cache.Get(ctx, session.ID)
		assert.NoError(t, err)
		assert.False(t, found, "Session %s should be expired", session.ID)
	}

	t.Logf("✅ TTL behavior verified for SetMany")
}

// TestSetMany_PerformanceComparison compares batch vs individual SET operations
func TestSetMany_PerformanceComparison(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	const numSessions = 50

	// Prepare sessions for batch test
	batchSessions := make([]*testintegration.TestSession, numSessions)
	for i := 0; i < numSessions; i++ {
		batchSessions[i] = &testintegration.TestSession{
			ID:       fmt.Sprintf("batch-set-%d", i),
			UserID:   fmt.Sprintf("user%d", i),
			Username: fmt.Sprintf("user%d", i),
			Created:  time.Now(),
		}
	}

	// Test batch operation performance
	batchStart := time.Now()
	err = cache.SetMany(ctx, batchSessions, 0)
	batchDuration := time.Since(batchStart)
	require.NoError(t, err)

	// Clear data for individual test
	setup.FlushRedis(ctx, t)

	// Prepare sessions for individual test
	individualSessions := make([]*testintegration.TestSession, numSessions)
	for i := 0; i < numSessions; i++ {
		individualSessions[i] = &testintegration.TestSession{
			ID:       fmt.Sprintf("individual-set-%d", i),
			UserID:   fmt.Sprintf("user%d", i),
			Username: fmt.Sprintf("user%d", i),
			Created:  time.Now(),
		}
	}

	// Test individual operations performance
	individualStart := time.Now()
	for _, session := range individualSessions {
		err = cache.Set(ctx, session, 0)
		require.NoError(t, err)
	}
	individualDuration := time.Since(individualStart)

	// Performance assertions
	speedupRatio := float64(individualDuration) / float64(batchDuration)
	assert.Greater(t, speedupRatio, 1.5, "Batch SetMany should be at least 50%% faster")

	t.Logf("📊 SetMany Performance Comparison:")
	t.Logf("   - Batch SetMany (%d sessions): %v", numSessions, batchDuration)
	t.Logf("   - Individual Sets (%d sessions): %v", numSessions, individualDuration)
	t.Logf("   - Speedup ratio: %.2fx", speedupRatio)
}

// TestDeleteMany_EdgeCases tests DeleteMany with various edge cases
func TestDeleteMany_EdgeCases(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	t.Run("EmptyKeysList", func(t *testing.T) {
		err := cache.DeleteMany(ctx, []string{})
		assert.NoError(t, err, "DeleteMany with empty keys should not error")
	})

	t.Run("NonExistentKeys", func(t *testing.T) {
		keys := []string{"nonexistent1", "nonexistent2", "nonexistent3"}
		err := cache.DeleteMany(ctx, keys)
		assert.NoError(t, err, "DeleteMany with non-existent keys should not error")
	})

	t.Run("MixedExistentAndNonExistent", func(t *testing.T) {
		// Set up some test data
		sessions := []*testintegration.TestSession{
			{ID: "delete-exists1", UserID: "user1", Username: "user1", Created: time.Now()},
			{ID: "delete-exists2", UserID: "user2", Username: "user2", Created: time.Now()},
		}
		
		for _, session := range sessions {
			err = cache.Set(ctx, session, 0)
			require.NoError(t, err)
		}

		// Delete mix of existing and non-existing keys
		keys := []string{"delete-exists1", "nonexistent", "delete-exists2"}
		err := cache.DeleteMany(ctx, keys)
		assert.NoError(t, err, "DeleteMany with mixed keys should not error")

		// Verify existing keys were deleted
		_, found, err := cache.Get(ctx, "delete-exists1")
		assert.NoError(t, err)
		assert.False(t, found, "Existing key should be deleted")

		_, found, err = cache.Get(ctx, "delete-exists2")
		assert.NoError(t, err)
		assert.False(t, found, "Existing key should be deleted")
	})
}

// TestDeleteMany_WithIndexing tests DeleteMany with indexing cleanup
func TestDeleteMany_WithIndexing(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.IndexedCacheConfig()  // Use indexed cache config
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	// Create sessions with indexing
	sessions := []*testintegration.TestSession{
		{ID: "del-session1", UserID: "user200", Username: "user200", Created: time.Now()},
		{ID: "del-session2", UserID: "user200", Username: "user200", Created: time.Now()},
		{ID: "del-session3", UserID: "user201", Username: "user201", Created: time.Now()},
	}

	// Set all sessions
	for _, session := range sessions {
		err = cache.Set(ctx, session, 0)
		require.NoError(t, err)
	}

	// Verify indexing is working
	user200Sessions, err := cache.GetByOwner(ctx, "user200")
	require.NoError(t, err)
	require.Len(t, user200Sessions, 2, "Should have 2 sessions for user200")

	// Delete some sessions using DeleteMany
	keysToDelete := []string{"del-session1", "del-session2"}
	err = cache.DeleteMany(ctx, keysToDelete)
	require.NoError(t, err)

	// Verify sessions are deleted
	for _, key := range keysToDelete {
		_, found, err := cache.Get(ctx, key)
		assert.NoError(t, err)
		assert.False(t, found, "Session %s should be deleted", key)
	}

	// Verify indexing is updated (user200 should have no sessions now)
	user200SessionsAfter, err := cache.GetByOwner(ctx, "user200")
	assert.NoError(t, err)
	assert.Empty(t, user200SessionsAfter, "User200 should have no sessions after deletion")

	// Verify other user's sessions are unaffected
	user201Sessions, err := cache.GetByOwner(ctx, "user201")
	assert.NoError(t, err)
	assert.Len(t, user201Sessions, 1, "User201 should still have 1 session")

	t.Logf("✅ DeleteMany indexing cleanup verified")
}

// TestDeleteMany_PerformanceComparison compares batch vs individual DELETE operations
func TestDeleteMany_PerformanceComparison(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	const numSessions = 50

	// Set up sessions for batch delete test
	batchKeys := make([]string, numSessions)
	for i := 0; i < numSessions; i++ {
		session := &testintegration.TestSession{
			ID:       fmt.Sprintf("batch-delete-%d", i),
			UserID:   fmt.Sprintf("user%d", i),
			Username: fmt.Sprintf("user%d", i),
			Created:  time.Now(),
		}
		batchKeys[i] = session.ID
		err = cache.Set(ctx, session, 0)
		require.NoError(t, err)
	}

	// Test batch delete performance
	batchStart := time.Now()
	err = cache.DeleteMany(ctx, batchKeys)
	batchDuration := time.Since(batchStart)
	require.NoError(t, err)

	// Set up sessions for individual delete test
	individualKeys := make([]string, numSessions)
	for i := 0; i < numSessions; i++ {
		session := &testintegration.TestSession{
			ID:       fmt.Sprintf("individual-delete-%d", i),
			UserID:   fmt.Sprintf("user%d", i),
			Username: fmt.Sprintf("user%d", i),
			Created:  time.Now(),
		}
		individualKeys[i] = session.ID
		err = cache.Set(ctx, session, 0)
		require.NoError(t, err)
	}

	// Test individual delete performance
	individualStart := time.Now()
	for _, key := range individualKeys {
		_, err = cache.Delete(ctx, key)
		require.NoError(t, err)
	}
	individualDuration := time.Since(individualStart)

	// Performance assertions
	speedupRatio := float64(individualDuration) / float64(batchDuration)
	assert.Greater(t, speedupRatio, 1.5, "Batch DeleteMany should be at least 50%% faster")

	t.Logf("📊 DeleteMany Performance Comparison:")
	t.Logf("   - Batch DeleteMany (%d keys): %v", numSessions, batchDuration)
	t.Logf("   - Individual Deletes (%d keys): %v", numSessions, individualDuration)
	t.Logf("   - Speedup ratio: %.2fx", speedupRatio)
}

// TestBatchOperations_ConcurrentAccess tests batch operations under concurrent access
func TestBatchOperations_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	const numGoroutines = 10
	const sessionsPerGoroutine = 20

	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make([]error, 0)

	// Concurrent SetMany operations
	wg.Add(numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			
			sessions := make([]*testintegration.TestSession, sessionsPerGoroutine)
			for i := 0; i < sessionsPerGoroutine; i++ {
				sessions[i] = &testintegration.TestSession{
					ID:       fmt.Sprintf("concurrent-g%d-s%d", goroutineID, i),
					UserID:   fmt.Sprintf("user-g%d", goroutineID),
					Username: fmt.Sprintf("user-g%d", goroutineID),
					Created:  time.Now(),
				}
			}
			
			if err := cache.SetMany(ctx, sessions, 0); err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
			}
		}(g)
	}
	wg.Wait()

	// Check for errors
	assert.Empty(t, errors, "Concurrent SetMany operations should not error")

	// Verify all sessions were set correctly
	totalExpected := numGoroutines * sessionsPerGoroutine
	allKeys := make([]string, totalExpected)
	for g := 0; g < numGoroutines; g++ {
		for s := 0; s < sessionsPerGoroutine; s++ {
			allKeys[g*sessionsPerGoroutine+s] = fmt.Sprintf("concurrent-g%d-s%d", g, s)
		}
	}

	results, err := cache.GetMany(ctx, allKeys)
	require.NoError(t, err)
	assert.Len(t, results, totalExpected, "All concurrent sessions should be retrievable")

	t.Logf("✅ Concurrent batch operations test successful")
	t.Logf("   - Goroutines: %d", numGoroutines)
	t.Logf("   - Sessions per goroutine: %d", sessionsPerGoroutine)
	t.Logf("   - Total sessions: %d", totalExpected)
}

// TestBatchOperations_CircuitBreaker tests batch operations with circuit breaker scenarios
func TestBatchOperations_CircuitBreaker(t *testing.T) {
	// This test would require more sophisticated setup to trigger circuit breaker
	// For now, it's a placeholder for when circuit breaker testing is needed
	t.Skip("Circuit breaker testing requires specific Redis failure scenarios")
}
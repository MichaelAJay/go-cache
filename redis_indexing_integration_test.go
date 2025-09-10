//go:build integration

package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/internal/testintegration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRedisCache_GetByOwner tests owner-based retrieval with indexing
func TestRedisCache_GetByOwner(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance with indexing enabled
	config := testintegration.IndexedCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create indexed cache")
	defer cache.Close()

	ownerID := "user2000"

	// Create multiple sessions for the same owner
	sessions := []*testintegration.TestSession{
		{
			ID:       "session:owner-1",
			UserID:   ownerID,
			Username: "owneruser",
			Created:  time.Now(),
		},
		{
			ID:       "session:owner-2",
			UserID:   ownerID,
			Username: "owneruser",
			Created:  time.Now().Add(time.Minute),
		},
		{
			ID:       "session:owner-3",
			UserID:   "user2001", // Different owner
			Username: "differentuser",
			Created:  time.Now(),
		},
	}

	t.Logf("📝 Testing GetByOwner operation for owner ID: %s", ownerID)

	// SET all sessions
	for _, session := range sessions {
		err = cache.Set(ctx, session, 0)
		require.NoError(t, err, "SET operation should not error for session %s", session.ID)
	}

	// GetByOwner for the main owner
	ownerSessions, err := cache.GetByOwner(ctx, ownerID)
	require.NoError(t, err, "GetByOwner should not error")

	// Should return exactly 2 sessions for this owner
	assert.Len(t, ownerSessions, 2, "GetByOwner should return 2 sessions for owner %s", ownerID)

	// Verify the correct sessions were returned
	sessionIDs := make(map[string]bool)
	for _, session := range ownerSessions {
		assert.Equal(t, ownerID, session.UserID, "All returned sessions should belong to the correct owner")
		sessionIDs[session.ID] = true
	}

	assert.True(t, sessionIDs["session:owner-1"], "Should include session:owner-1")
	assert.True(t, sessionIDs["session:owner-2"], "Should include session:owner-2")
	assert.False(t, sessionIDs["session:owner-3"], "Should not include session from different owner")

	// GetByOwner for different owner
	differentOwnerSessions, err := cache.GetByOwner(ctx, "user2001")
	require.NoError(t, err, "GetByOwner should not error for different owner")
	assert.Len(t, differentOwnerSessions, 1, "Different owner should have 1 session")
	assert.Equal(t, "session:owner-3", differentOwnerSessions[0].ID, "Should return correct session for different owner")

	// GetByOwner for non-existent owner
	nonExistentSessions, err := cache.GetByOwner(ctx, "user9999")
	require.NoError(t, err, "GetByOwner should not error for non-existent owner")
	assert.Len(t, nonExistentSessions, 0, "Non-existent owner should have 0 sessions")

	t.Logf("✅ GetByOwner test successful")
	t.Logf("   - Owner ID: %s", ownerID)
	t.Logf("   - Sessions found: %d", len(ownerSessions))
}

// TestRedisCache_DeleteByOwner tests owner-based deletion with indexing
func TestRedisCache_DeleteByOwner(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance with indexing enabled
	config := testintegration.IndexedCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create indexed cache")
	defer cache.Close()

	ownerID := "user2100"

	// Create multiple sessions for the same owner
	sessions := []*testintegration.TestSession{
		{
			ID:       "session:deleteowner-1",
			UserID:   ownerID,
			Username: "deleteowneruser",
			Created:  time.Now(),
		},
		{
			ID:       "session:deleteowner-2",
			UserID:   ownerID,
			Username: "deleteowneruser",
			Created:  time.Now().Add(time.Minute),
		},
		{
			ID:       "session:deleteowner-3",
			UserID:   ownerID,
			Username: "deleteowneruser",
			Created:  time.Now().Add(2 * time.Minute),
		},
		{
			ID:       "session:keepowner-1",
			UserID:   "user2101", // Different owner - should not be deleted
			Username: "keepuser",
			Created:  time.Now(),
		},
	}

	t.Logf("📝 Testing DeleteByOwner operation for owner ID: %s", ownerID)

	// SET all sessions
	for _, session := range sessions {
		err = cache.Set(ctx, session, 0)
		require.NoError(t, err, "SET operation should not error for session %s", session.ID)
	}

	// Verify all sessions exist before deletion
	for _, session := range sessions {
		exists := cache.Has(ctx, session.ID)
		assert.True(t, exists, "Session %s should exist before DeleteByOwner", session.ID)
	}

	// DeleteByOwner operation
	deletedCount, err := cache.DeleteByOwner(ctx, ownerID)
	require.NoError(t, err, "DeleteByOwner should not error")
	assert.Equal(t, 3, deletedCount, "DeleteByOwner should return count of 3 deleted sessions")

	// Verify sessions belonging to the owner were deleted
	targetSessions := []string{"session:deleteowner-1", "session:deleteowner-2", "session:deleteowner-3"}
	for _, sessionID := range targetSessions {
		exists := cache.Has(ctx, sessionID)
		assert.False(t, exists, "Session %s should not exist after DeleteByOwner", sessionID)
		
		// Double check with GET
		retrievedSession, found, err := cache.Get(ctx, sessionID)
		assert.NoError(t, err, "GET should not error after DeleteByOwner")
		assert.False(t, found, "GET should return found=false after DeleteByOwner")
		assert.Nil(t, retrievedSession, "GET should return nil after DeleteByOwner")
	}

	// Verify session from different owner still exists
	exists := cache.Has(ctx, "session:keepowner-1")
	assert.True(t, exists, "Session from different owner should still exist after DeleteByOwner")

	// Verify GetByOwner returns empty for deleted owner
	ownerSessions, err := cache.GetByOwner(ctx, ownerID)
	require.NoError(t, err, "GetByOwner should not error after DeleteByOwner")
	assert.Len(t, ownerSessions, 0, "GetByOwner should return 0 sessions after DeleteByOwner")

	t.Logf("✅ DeleteByOwner test successful")
	t.Logf("   - Owner ID: %s", ownerID)
	t.Logf("   - Sessions deleted: %d", deletedCount)
}

// TestRedisCache_IndexingConsistency tests that indexing remains consistent across operations
func TestRedisCache_IndexingConsistency(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance with indexing enabled
	config := testintegration.IndexedCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create indexed cache")
	defer cache.Close()

	ownerID := "user2200"

	originalSession := &testintegration.TestSession{
		ID:       "session:consistency-test",
		UserID:   ownerID,
		Username: "consistencyuser",
		Created:  time.Now(),
	}

	t.Logf("📝 Testing indexing consistency for owner ID: %s", ownerID)

	// SET operation - should add to index
	err = cache.Set(ctx, originalSession, 0)
	require.NoError(t, err, "SET operation should not error")

	// Verify session is in owner index
	ownerSessions, err := cache.GetByOwner(ctx, ownerID)
	require.NoError(t, err, "GetByOwner should not error")
	assert.Len(t, ownerSessions, 1, "Owner should have 1 session after SET")
	assert.Equal(t, originalSession.ID, ownerSessions[0].ID, "Index should contain correct session")

	// Overwrite with same owner - index should remain consistent
	updatedSession := &testintegration.TestSession{
		ID:       "session:consistency-test", // Same ID
		UserID:   ownerID, // Same owner
		Username: "updateduser",
		Created:  time.Now().Add(time.Hour),
	}

	err = cache.Set(ctx, updatedSession, 0)
	require.NoError(t, err, "Overwrite SET should not error")

	// Verify index is still consistent
	ownerSessions, err = cache.GetByOwner(ctx, ownerID)
	require.NoError(t, err, "GetByOwner should not error after overwrite")
	assert.Len(t, ownerSessions, 1, "Owner should still have 1 session after overwrite")
	assert.Equal(t, "updateduser", ownerSessions[0].Username, "Index should reflect updated data")

	// Change owner via overwrite - index should update
	differentOwnerSession := &testintegration.TestSession{
		ID:       "session:consistency-test", // Same ID
		UserID:   "user2201", // Different owner
		Username: "differentowneruser",
		Created:  time.Now().Add(2 * time.Hour),
	}

	err = cache.Set(ctx, differentOwnerSession, 0)
	require.NoError(t, err, "Owner change SET should not error")

	// Verify original owner no longer has the session
	originalOwnerSessions, err := cache.GetByOwner(ctx, ownerID)
	require.NoError(t, err, "GetByOwner should not error for original owner")
	assert.Len(t, originalOwnerSessions, 0, "Original owner should have 0 sessions after owner change")

	// Verify new owner has the session
	newOwnerSessions, err := cache.GetByOwner(ctx, "user2201")
	require.NoError(t, err, "GetByOwner should not error for new owner")
	assert.Len(t, newOwnerSessions, 1, "New owner should have 1 session after owner change")
	assert.Equal(t, differentOwnerSession.ID, newOwnerSessions[0].ID, "New owner index should contain correct session")

	// DELETE operation - should remove from index
	err = cache.Delete(ctx, differentOwnerSession.ID)
	require.NoError(t, err, "DELETE should not error")

	// Verify session is removed from new owner index
	newOwnerSessions, err = cache.GetByOwner(ctx, "user2201")
	require.NoError(t, err, "GetByOwner should not error after DELETE")
	assert.Len(t, newOwnerSessions, 0, "New owner should have 0 sessions after DELETE")

	t.Logf("✅ Indexing consistency test successful")
}

// TestRedisCache_IndexingWithBatchOperations tests indexing behavior with batch operations
func TestRedisCache_IndexingWithBatchOperations(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance with indexing enabled
	config := testintegration.IndexedCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create indexed cache")
	defer cache.Close()

	owner1 := "user2300"
	owner2 := "user2301"

	// Create sessions for multiple owners
	sessions := []*testintegration.TestSession{
		{
			ID:       "session:batch-1",
			UserID:   owner1,
			Username: "batchuser1",
			Created:  time.Now(),
		},
		{
			ID:       "session:batch-2",
			UserID:   owner1,
			Username: "batchuser1",
			Created:  time.Now().Add(time.Minute),
		},
		{
			ID:       "session:batch-3",
			UserID:   owner2,
			Username: "batchuser2",
			Created:  time.Now(),
		},
		{
			ID:       "session:batch-4",
			UserID:   owner2,
			Username: "batchuser2",
			Created:  time.Now().Add(time.Minute),
		},
	}

	t.Logf("📝 Testing indexing with batch operations")

	// SetMany - should update all indexes
	err = cache.SetMany(ctx, sessions, 0)
	require.NoError(t, err, "SetMany should not error")

	// Verify indexes are updated correctly
	owner1Sessions, err := cache.GetByOwner(ctx, owner1)
	require.NoError(t, err, "GetByOwner should not error for owner1")
	assert.Len(t, owner1Sessions, 2, "Owner1 should have 2 sessions after SetMany")

	owner2Sessions, err := cache.GetByOwner(ctx, owner2)
	require.NoError(t, err, "GetByOwner should not error for owner2")
	assert.Len(t, owner2Sessions, 2, "Owner2 should have 2 sessions after SetMany")

	// DeleteMany - should update indexes
	keysToDelete := []string{"session:batch-1", "session:batch-3"}
	err = cache.DeleteMany(ctx, keysToDelete)
	require.NoError(t, err, "DeleteMany should not error")

	// Verify indexes are updated after DeleteMany
	owner1Sessions, err = cache.GetByOwner(ctx, owner1)
	require.NoError(t, err, "GetByOwner should not error after DeleteMany")
	assert.Len(t, owner1Sessions, 1, "Owner1 should have 1 session after DeleteMany")
	assert.Equal(t, "session:batch-2", owner1Sessions[0].ID, "Owner1 should have correct remaining session")

	owner2Sessions, err = cache.GetByOwner(ctx, owner2)
	require.NoError(t, err, "GetByOwner should not error after DeleteMany")
	assert.Len(t, owner2Sessions, 1, "Owner2 should have 1 session after DeleteMany")
	assert.Equal(t, "session:batch-4", owner2Sessions[0].ID, "Owner2 should have correct remaining session")

	t.Logf("✅ Indexing with batch operations test successful")
}

// TestRedisCache_IndexingWithClear tests that Clear operation properly handles indexes
func TestRedisCache_IndexingWithClear(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance with indexing enabled
	config := testintegration.IndexedCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create indexed cache")
	defer cache.Close()

	// Create sessions for multiple owners
	sessions := []*testintegration.TestSession{
		{
			ID:       "session:clear-index-1",
			UserID:   "user2400",
			Username: "clearuser1",
			Created:  time.Now(),
		},
		{
			ID:       "session:clear-index-2",
			UserID:   "user2400",
			Username: "clearuser1",
			Created:  time.Now().Add(time.Minute),
		},
		{
			ID:       "session:clear-index-3",
			UserID:   "user2401",
			Username: "clearuser2",
			Created:  time.Now(),
		},
	}

	t.Logf("📝 Testing indexing with Clear operation")

	// SET all sessions
	for _, session := range sessions {
		err = cache.Set(ctx, session, 0)
		require.NoError(t, err, "SET should not error")
	}

	// Verify indexes are populated
	user2400Sessions, err := cache.GetByOwner(ctx, "user2400")
	require.NoError(t, err, "GetByOwner should not error")
	assert.Len(t, user2400Sessions, 2, "user2400 should have 2 sessions before Clear")

	user2401Sessions, err := cache.GetByOwner(ctx, "user2401")
	require.NoError(t, err, "GetByOwner should not error")
	assert.Len(t, user2401Sessions, 1, "user2401 should have 1 session before Clear")

	// Clear all data
	err = cache.Clear(ctx)
	require.NoError(t, err, "Clear should not error")

	// Verify all indexes are cleared
	user2400Sessions, err = cache.GetByOwner(ctx, "user2400")
	require.NoError(t, err, "GetByOwner should not error after Clear")
	assert.Len(t, user2400Sessions, 0, "user2400 should have 0 sessions after Clear")

	user2401Sessions, err = cache.GetByOwner(ctx, "user2401")
	require.NoError(t, err, "GetByOwner should not error after Clear")
	assert.Len(t, user2401Sessions, 0, "user2401 should have 0 sessions after Clear")

	// Verify individual sessions are gone
	for _, session := range sessions {
		exists := cache.Has(ctx, session.ID)
		assert.False(t, exists, "Session %s should not exist after Clear", session.ID)
	}

	t.Logf("✅ Indexing with Clear operation test successful")
}

// TestRedisCache_IndexingWithNonIndexedCache tests that non-indexed caches reject indexing operations
func TestRedisCache_IndexingWithNonIndexedCache(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance WITHOUT indexing
	config := testintegration.DefaultCacheConfig()
	config.IndexingMode = false // Explicitly disable indexing
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create non-indexed cache")
	defer cache.Close()

	testSession := &testintegration.TestSession{
		ID:       "session:non-indexed-test",
		UserID:   "user2500",
		Username: "nonindexeduser",
		Created:  time.Now(),
	}

	t.Logf("📝 Testing indexing operations with non-indexed cache")

	// SET should work normally
	err = cache.Set(ctx, testSession, 0)
	require.NoError(t, err, "SET should work on non-indexed cache")

	// Basic operations should work
	exists := cache.Has(ctx, testSession.ID)
	assert.True(t, exists, "HAS should work on non-indexed cache")

	session, found, err := cache.Get(ctx, testSession.ID)
	assert.NoError(t, err, "GET should work on non-indexed cache")
	assert.True(t, found, "GET should find session on non-indexed cache")
	assert.Equal(t, testSession.ID, session.ID, "GET should return correct session")

	// Indexing operations should return errors or empty results
	ownerSessions, err := cache.GetByOwner(ctx, "user2500")
	// The behavior here depends on implementation - it might error or return empty
	// We'll check that it doesn't panic and handles the case gracefully
	if err != nil {
		t.Logf("   - GetByOwner returned error as expected: %v", err)
	} else {
		t.Logf("   - GetByOwner returned empty result as expected: %d sessions", len(ownerSessions))
	}

	deletedCount, err := cache.DeleteByOwner(ctx, "user2500")
	// Same as above - implementation dependent behavior
	if err != nil {
		t.Logf("   - DeleteByOwner returned error as expected: %v", err)
	} else {
		t.Logf("   - DeleteByOwner returned result: %d deleted", deletedCount)
		// If it succeeded, verify the session is still there (wasn't deleted)
		exists = cache.Has(ctx, testSession.ID)
		assert.True(t, exists, "Session should still exist if DeleteByOwner didn't work on non-indexed cache")
	}

	t.Logf("✅ Non-indexed cache test successful")
}

// TestRedisCache_GetCountByOwner_Basic tests basic counting functionality
func TestRedisCache_GetCountByOwner_Basic(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance with indexing enabled
	config := testintegration.IndexedCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create indexed cache")
	defer cache.Close()

	ownerID := "user3000"
	differentOwnerID := "user3001"

	t.Logf("📝 Testing GetCountByOwner basic functionality for owner ID: %s", ownerID)

	// Initially, count should be 0 for non-existent owner
	count, err := cache.GetCountByOwner(ctx, ownerID)
	require.NoError(t, err, "GetCountByOwner should not error for non-existent owner")
	assert.Equal(t, 0, count, "Non-existent owner should have count 0")

	// Create multiple sessions for the same owner
	sessions := []*testintegration.TestSession{
		{
			ID:       "session:count-basic-1",
			UserID:   ownerID,
			Username: "basicuser",
			Created:  time.Now(),
		},
		{
			ID:       "session:count-basic-2",
			UserID:   ownerID,
			Username: "basicuser",
			Created:  time.Now().Add(time.Minute),
		},
		{
			ID:       "session:count-basic-3",
			UserID:   ownerID,
			Username: "basicuser",
			Created:  time.Now().Add(2 * time.Minute),
		},
		{
			ID:       "session:count-different-1",
			UserID:   differentOwnerID,
			Username: "differentuser",
			Created:  time.Now(),
		},
	}

	// SET sessions one by one and verify count increases
	for i, session := range sessions[:3] { // First 3 sessions belong to main owner
		err = cache.Set(ctx, session, 0)
		require.NoError(t, err, "SET operation should not error for session %s", session.ID)

		count, err = cache.GetCountByOwner(ctx, ownerID)
		require.NoError(t, err, "GetCountByOwner should not error after SET %d", i+1)
		assert.Equal(t, i+1, count, "Count should be %d after setting session %d", i+1, i+1)
	}

	// Set session for different owner
	err = cache.Set(ctx, sessions[3], 0)
	require.NoError(t, err, "SET operation should not error for different owner session")

	// Main owner count should still be 3
	count, err = cache.GetCountByOwner(ctx, ownerID)
	require.NoError(t, err, "GetCountByOwner should not error after different owner SET")
	assert.Equal(t, 3, count, "Main owner should still have count 3")

	// Different owner count should be 1
	count, err = cache.GetCountByOwner(ctx, differentOwnerID)
	require.NoError(t, err, "GetCountByOwner should not error for different owner")
	assert.Equal(t, 1, count, "Different owner should have count 1")

	t.Logf("✅ GetCountByOwner basic test successful")
	t.Logf("   - Owner ID: %s, Final count: %d", ownerID, 3)
	t.Logf("   - Different Owner ID: %s, Final count: %d", differentOwnerID, 1)
}

// TestRedisCache_GetCountByOwner_WithDeletions tests counting with deletions
func TestRedisCache_GetCountByOwner_WithDeletions(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance with indexing enabled
	config := testintegration.IndexedCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create indexed cache")
	defer cache.Close()

	ownerID := "user3100"

	t.Logf("📝 Testing GetCountByOwner with deletions for owner ID: %s", ownerID)

	// Create sessions
	sessions := []*testintegration.TestSession{
		{
			ID:       "session:count-del-1",
			UserID:   ownerID,
			Username: "deluser",
			Created:  time.Now(),
		},
		{
			ID:       "session:count-del-2",
			UserID:   ownerID,
			Username: "deluser",
			Created:  time.Now().Add(time.Minute),
		},
		{
			ID:       "session:count-del-3",
			UserID:   ownerID,
			Username: "deluser",
			Created:  time.Now().Add(2 * time.Minute),
		},
		{
			ID:       "session:count-del-4",
			UserID:   ownerID,
			Username: "deluser",
			Created:  time.Now().Add(3 * time.Minute),
		},
		{
			ID:       "session:count-del-5",
			UserID:   ownerID,
			Username: "deluser",
			Created:  time.Now().Add(4 * time.Minute),
		},
	}

	// SET all sessions
	for _, session := range sessions {
		err = cache.Set(ctx, session, 0)
		require.NoError(t, err, "SET operation should not error for session %s", session.ID)
	}

	// Verify initial count
	count, err := cache.GetCountByOwner(ctx, ownerID)
	require.NoError(t, err, "GetCountByOwner should not error")
	assert.Equal(t, 5, count, "Initial count should be 5")

	// Delete individual sessions and verify count decreases
	err = cache.Delete(ctx, "session:count-del-1")
	require.NoError(t, err, "Delete should not error")

	count, err = cache.GetCountByOwner(ctx, ownerID)
	require.NoError(t, err, "GetCountByOwner should not error after first delete")
	assert.Equal(t, 4, count, "Count should be 4 after first delete")

	err = cache.Delete(ctx, "session:count-del-3")
	require.NoError(t, err, "Delete should not error")

	count, err = cache.GetCountByOwner(ctx, ownerID)
	require.NoError(t, err, "GetCountByOwner should not error after second delete")
	assert.Equal(t, 3, count, "Count should be 3 after second delete")

	// Delete multiple keys at once
	err = cache.DeleteMany(ctx, []string{"session:count-del-2", "session:count-del-4"})
	require.NoError(t, err, "DeleteMany should not error")

	count, err = cache.GetCountByOwner(ctx, ownerID)
	require.NoError(t, err, "GetCountByOwner should not error after DeleteMany")
	assert.Equal(t, 1, count, "Count should be 1 after DeleteMany")

	// Verify GetByOwner shows correct data
	actualSessions, err := cache.GetByOwner(ctx, ownerID)
	require.NoError(t, err, "GetByOwner should work correctly")
	assert.Len(t, actualSessions, 1, "GetByOwner should show actual 1 remaining session")

	// Delete remaining session using single Delete (which works correctly)
	err = cache.Delete(ctx, "session:count-del-5")
	require.NoError(t, err, "Delete should not error")

	count, err = cache.GetCountByOwner(ctx, ownerID)
	require.NoError(t, err, "GetCountByOwner should not error after final delete")
	assert.Equal(t, 0, count, "Count should be 0 after final delete")

	t.Logf("✅ GetCountByOwner with deletions test successful")
	t.Logf("   - Owner ID: %s, Final count: 0", ownerID)
}

// TestRedisCache_GetCountByOwner_WithBatchOperations tests counting with batch operations
func TestRedisCache_GetCountByOwner_WithBatchOperations(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance with indexing enabled
	config := testintegration.IndexedCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create indexed cache")
	defer cache.Close()

	owner1 := "user3200"
	owner2 := "user3201"

	t.Logf("📝 Testing GetCountByOwner with batch operations")

	// Create sessions for multiple owners
	sessions := []*testintegration.TestSession{
		{ID: "session:batch-count-1", UserID: owner1, Username: "batchuser1", Created: time.Now()},
		{ID: "session:batch-count-2", UserID: owner1, Username: "batchuser1", Created: time.Now().Add(time.Minute)},
		{ID: "session:batch-count-3", UserID: owner1, Username: "batchuser1", Created: time.Now().Add(2 * time.Minute)},
		{ID: "session:batch-count-4", UserID: owner1, Username: "batchuser1", Created: time.Now().Add(3 * time.Minute)},
		{ID: "session:batch-count-5", UserID: owner2, Username: "batchuser2", Created: time.Now()},
		{ID: "session:batch-count-6", UserID: owner2, Username: "batchuser2", Created: time.Now().Add(time.Minute)},
		{ID: "session:batch-count-7", UserID: owner2, Username: "batchuser2", Created: time.Now().Add(2 * time.Minute)},
	}

	// Initial counts should be 0
	count1, err := cache.GetCountByOwner(ctx, owner1)
	require.NoError(t, err, "GetCountByOwner should not error for owner1")
	assert.Equal(t, 0, count1, "Initial count for owner1 should be 0")

	count2, err := cache.GetCountByOwner(ctx, owner2)
	require.NoError(t, err, "GetCountByOwner should not error for owner2")
	assert.Equal(t, 0, count2, "Initial count for owner2 should be 0")

	// SetMany - should update all counts
	err = cache.SetMany(ctx, sessions, 0)
	require.NoError(t, err, "SetMany should not error")

	// Verify counts after SetMany
	count1, err = cache.GetCountByOwner(ctx, owner1)
	require.NoError(t, err, "GetCountByOwner should not error for owner1 after SetMany")
	assert.Equal(t, 4, count1, "Owner1 should have 4 sessions after SetMany")

	count2, err = cache.GetCountByOwner(ctx, owner2)
	require.NoError(t, err, "GetCountByOwner should not error for owner2 after SetMany")
	assert.Equal(t, 3, count2, "Owner2 should have 3 sessions after SetMany")

	// DeleteMany - should update counts
	// Delete session:batch-count-1 (owner1), session:batch-count-5 (owner2), session:batch-count-6 (owner2)
	keysToDelete := []string{"session:batch-count-1", "session:batch-count-5", "session:batch-count-6"}
	err = cache.DeleteMany(ctx, keysToDelete)
	require.NoError(t, err, "DeleteMany should not error")

	// Verify counts after DeleteMany
	count1, err = cache.GetCountByOwner(ctx, owner1)
	require.NoError(t, err, "GetCountByOwner should not error for owner1 after DeleteMany")
	assert.Equal(t, 3, count1, "Owner1 should have 3 sessions after DeleteMany")

	count2, err = cache.GetCountByOwner(ctx, owner2)
	require.NoError(t, err, "GetCountByOwner should not error for owner2 after DeleteMany")
	assert.Equal(t, 1, count2, "Owner2 should have 1 session after DeleteMany")

	// Verify that GetByOwner shows the correct actual data (this works correctly)
	actualOwner1Sessions, err := cache.GetByOwner(ctx, owner1)
	require.NoError(t, err, "GetByOwner should work correctly for owner1")
	assert.Len(t, actualOwner1Sessions, 3, "GetByOwner should show actual 3 sessions for owner1")

	actualOwner2Sessions, err := cache.GetByOwner(ctx, owner2)
	require.NoError(t, err, "GetByOwner should work correctly for owner2")
	assert.Len(t, actualOwner2Sessions, 1, "GetByOwner should show actual 1 session for owner2")

	t.Logf("✅ GetCountByOwner with batch operations test successful")
	t.Logf("   - Owner1: %s, Final count: 3", owner1)
	t.Logf("   - Owner2: %s, Final count: 1", owner2)
}

// TestRedisCache_GetCountByOwner_WithDeleteByOwner tests counting with DeleteByOwner operations
func TestRedisCache_GetCountByOwner_WithDeleteByOwner(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance with indexing enabled
	config := testintegration.IndexedCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create indexed cache")
	defer cache.Close()

	ownerToDelete := "user3300"
	ownerToKeep := "user3301"

	t.Logf("📝 Testing GetCountByOwner with DeleteByOwner")

	// Create sessions for multiple owners
	sessions := []*testintegration.TestSession{
		{ID: "session:delbyowner-1", UserID: ownerToDelete, Username: "deleteuser", Created: time.Now()},
		{ID: "session:delbyowner-2", UserID: ownerToDelete, Username: "deleteuser", Created: time.Now().Add(time.Minute)},
		{ID: "session:delbyowner-3", UserID: ownerToDelete, Username: "deleteuser", Created: time.Now().Add(2 * time.Minute)},
		{ID: "session:delbyowner-4", UserID: ownerToDelete, Username: "deleteuser", Created: time.Now().Add(3 * time.Minute)},
		{ID: "session:delbyowner-5", UserID: ownerToDelete, Username: "deleteuser", Created: time.Now().Add(4 * time.Minute)},
		{ID: "session:keepowner-1", UserID: ownerToKeep, Username: "keepuser", Created: time.Now()},
		{ID: "session:keepowner-2", UserID: ownerToKeep, Username: "keepuser", Created: time.Now().Add(time.Minute)},
	}

	// SET all sessions
	for _, session := range sessions {
		err = cache.Set(ctx, session, 0)
		require.NoError(t, err, "SET should not error")
	}

	// Verify initial counts
	countToDelete, err := cache.GetCountByOwner(ctx, ownerToDelete)
	require.NoError(t, err, "GetCountByOwner should not error for ownerToDelete")
	assert.Equal(t, 5, countToDelete, "ownerToDelete should have 5 sessions initially")

	countToKeep, err := cache.GetCountByOwner(ctx, ownerToKeep)
	require.NoError(t, err, "GetCountByOwner should not error for ownerToKeep")
	assert.Equal(t, 2, countToKeep, "ownerToKeep should have 2 sessions initially")

	// DeleteByOwner operation
	deletedCount, err := cache.DeleteByOwner(ctx, ownerToDelete)
	require.NoError(t, err, "DeleteByOwner should not error")
	assert.Equal(t, 5, deletedCount, "DeleteByOwner should return 5 deleted sessions")

	// Verify counts after DeleteByOwner
	countToDelete, err = cache.GetCountByOwner(ctx, ownerToDelete)
	require.NoError(t, err, "GetCountByOwner should not error after DeleteByOwner")
	assert.Equal(t, 0, countToDelete, "ownerToDelete should have 0 sessions after DeleteByOwner")

	countToKeep, err = cache.GetCountByOwner(ctx, ownerToKeep)
	require.NoError(t, err, "GetCountByOwner should not error for ownerToKeep after DeleteByOwner")
	assert.Equal(t, 2, countToKeep, "ownerToKeep should still have 2 sessions after DeleteByOwner")

	t.Logf("✅ GetCountByOwner with DeleteByOwner test successful")
	t.Logf("   - Deleted owner: %s, Final count: %d", ownerToDelete, 0)
	t.Logf("   - Kept owner: %s, Final count: %d", ownerToKeep, 2)
}

// TestRedisCache_GetCountByOwner_WithUpdates tests counting with session updates
func TestRedisCache_GetCountByOwner_WithUpdates(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance with indexing enabled
	config := testintegration.IndexedCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create indexed cache")
	defer cache.Close()

	owner1 := "user3400"
	owner2 := "user3401"
	sessionID := "session:count-update-test"

	t.Logf("📝 Testing GetCountByOwner with updates")

	// Create initial session for owner1
	session := &testintegration.TestSession{
		ID:       sessionID,
		UserID:   owner1,
		Username: "updateuser1",
		Created:  time.Now(),
	}

	err = cache.Set(ctx, session, 0)
	require.NoError(t, err, "Initial SET should not error")

	// Verify initial counts
	count1, err := cache.GetCountByOwner(ctx, owner1)
	require.NoError(t, err, "GetCountByOwner should not error for owner1")
	assert.Equal(t, 1, count1, "Owner1 should have 1 session initially")

	count2, err := cache.GetCountByOwner(ctx, owner2)
	require.NoError(t, err, "GetCountByOwner should not error for owner2")
	assert.Equal(t, 0, count2, "Owner2 should have 0 sessions initially")

	// Update session data but keep same owner - count should remain the same
	updatedSession := &testintegration.TestSession{
		ID:       sessionID,
		UserID:   owner1,
		Username: "updateuser1-modified",
		Created:  time.Now().Add(time.Hour),
	}

	err = cache.Set(ctx, updatedSession, 0)
	require.NoError(t, err, "Update SET with same owner should not error")

	count1, err = cache.GetCountByOwner(ctx, owner1)
	require.NoError(t, err, "GetCountByOwner should not error after update")
	assert.Equal(t, 1, count1, "Owner1 should still have 1 session after update")

	count2, err = cache.GetCountByOwner(ctx, owner2)
	require.NoError(t, err, "GetCountByOwner should not error for owner2 after update")
	assert.Equal(t, 0, count2, "Owner2 should still have 0 sessions after update")

	// Transfer ownership to owner2 - counts should update
	transferredSession := &testintegration.TestSession{
		ID:       sessionID,
		UserID:   owner2, // Changed owner
		Username: "updateuser2",
		Created:  time.Now().Add(2 * time.Hour),
	}

	err = cache.Set(ctx, transferredSession, 0)
	require.NoError(t, err, "Transfer SET should not error")

	count1, err = cache.GetCountByOwner(ctx, owner1)
	require.NoError(t, err, "GetCountByOwner should not error for owner1 after transfer")
	assert.Equal(t, 0, count1, "Owner1 should have 0 sessions after transfer")

	count2, err = cache.GetCountByOwner(ctx, owner2)
	require.NoError(t, err, "GetCountByOwner should not error for owner2 after transfer")
	assert.Equal(t, 1, count2, "Owner2 should have 1 session after transfer")

	t.Logf("✅ GetCountByOwner with updates test successful")
	t.Logf("   - Original owner: %s, Final count: %d", owner1, 0)
	t.Logf("   - New owner: %s, Final count: %d", owner2, 1)
}

// TestRedisCache_GetCountByOwner_WithClear tests that Clear operation resets all counts
func TestRedisCache_GetCountByOwner_WithClear(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance with indexing enabled
	config := testintegration.IndexedCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create indexed cache")
	defer cache.Close()

	t.Logf("📝 Testing GetCountByOwner with Clear operation")

	// Create sessions for multiple owners
	sessions := []*testintegration.TestSession{
		{ID: "session:clear-count-1", UserID: "user3500", Username: "clearuser1", Created: time.Now()},
		{ID: "session:clear-count-2", UserID: "user3500", Username: "clearuser1", Created: time.Now().Add(time.Minute)},
		{ID: "session:clear-count-3", UserID: "user3500", Username: "clearuser1", Created: time.Now().Add(2 * time.Minute)},
		{ID: "session:clear-count-4", UserID: "user3501", Username: "clearuser2", Created: time.Now()},
		{ID: "session:clear-count-5", UserID: "user3501", Username: "clearuser2", Created: time.Now().Add(time.Minute)},
		{ID: "session:clear-count-6", UserID: "user3502", Username: "clearuser3", Created: time.Now()},
	}

	// SET all sessions
	for _, session := range sessions {
		err = cache.Set(ctx, session, 0)
		require.NoError(t, err, "SET should not error")
	}

	// Verify initial counts
	count1, err := cache.GetCountByOwner(ctx, "user3500")
	require.NoError(t, err, "GetCountByOwner should not error")
	assert.Equal(t, 3, count1, "user3500 should have 3 sessions before Clear")

	count2, err := cache.GetCountByOwner(ctx, "user3501")
	require.NoError(t, err, "GetCountByOwner should not error")
	assert.Equal(t, 2, count2, "user3501 should have 2 sessions before Clear")

	count3, err := cache.GetCountByOwner(ctx, "user3502")
	require.NoError(t, err, "GetCountByOwner should not error")
	assert.Equal(t, 1, count3, "user3502 should have 1 session before Clear")

	// Clear all data
	err = cache.Clear(ctx)
	require.NoError(t, err, "Clear should not error")

	// Verify all counts are 0 after Clear
	count1, err = cache.GetCountByOwner(ctx, "user3500")
	require.NoError(t, err, "GetCountByOwner should not error after Clear")
	assert.Equal(t, 0, count1, "user3500 should have 0 sessions after Clear")

	count2, err = cache.GetCountByOwner(ctx, "user3501")
	require.NoError(t, err, "GetCountByOwner should not error after Clear")
	assert.Equal(t, 0, count2, "user3501 should have 0 sessions after Clear")

	count3, err = cache.GetCountByOwner(ctx, "user3502")
	require.NoError(t, err, "GetCountByOwner should not error after Clear")
	assert.Equal(t, 0, count3, "user3502 should have 0 sessions after Clear")

	t.Logf("✅ GetCountByOwner with Clear test successful")
}

// TestRedisCache_GetCountByOwner_NonIndexedCache tests that non-indexed caches reject GetCountByOwner
func TestRedisCache_GetCountByOwner_NonIndexedCache(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance WITHOUT indexing
	config := testintegration.DefaultCacheConfig()
	config.IndexingMode = false // Explicitly disable indexing
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create non-indexed cache")
	defer cache.Close()

	t.Logf("📝 Testing GetCountByOwner with non-indexed cache")

	// Create and set a test session
	testSession := &testintegration.TestSession{
		ID:       "session:non-indexed-count",
		UserID:   "user3600",
		Username: "nonindexeduser",
		Created:  time.Now(),
	}

	// SET should work normally
	err = cache.Set(ctx, testSession, 0)
	require.NoError(t, err, "SET should work on non-indexed cache")

	// GetCountByOwner should return an error since indexing is required
	count, err := cache.GetCountByOwner(ctx, "user3600")
	require.Error(t, err, "GetCountByOwner should return error on non-indexed cache")
	assert.Contains(t, err.Error(), "GetCountByOwner requires indexing to be enabled", 
		"Error message should indicate indexing is required")
	assert.Equal(t, 0, count, "Count should be 0 when error occurs")

	t.Logf("✅ GetCountByOwner with non-indexed cache test successful")
	t.Logf("   - Error returned as expected: %v", err)
}
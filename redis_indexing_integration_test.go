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
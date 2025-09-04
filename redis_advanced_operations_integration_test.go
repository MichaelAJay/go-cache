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

// TestRedisCache_GetMany tests batch GET operations
func TestRedisCache_GetMany(t *testing.T) {
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

	// Create multiple test sessions
	sessions := []*testintegration.TestSession{
		{
			ID:       "session:getmany-1",
			UserID:   "user1200",
			Username: "getmanyuser1",
			Created:  time.Now(),
		},
		{
			ID:       "session:getmany-2",
			UserID:   "user1201",
			Username: "getmanyuser2",
			Created:  time.Now(),
		},
		{
			ID:       "session:getmany-3",
			UserID:   "user1202",
			Username: "getmanyuser3",
			Created:  time.Now(),
		},
	}

	t.Logf("📝 Testing GetMany operation with %d sessions", len(sessions))

	// SET all sessions
	for _, session := range sessions {
		err = cache.Set(ctx, session, 0)
		require.NoError(t, err, "SET operation should not error for session %s", session.ID)
	}

	// Prepare keys for GetMany
	keys := make([]string, len(sessions))
	for i, session := range sessions {
		keys[i] = session.ID
	}

	// Add a non-existent key to test mixed results
	keys = append(keys, "session:nonexistent-getmany")

	// GetMany operation
	results, err := cache.GetMany(ctx, keys)
	
	// Assertions
	assert.NoError(t, err, "GetMany operation should not error")
	assert.Len(t, results, len(sessions), "GetMany should return only existing sessions")

	// Verify each session was retrieved correctly
	for _, originalSession := range sessions {
		retrievedSession, exists := results[originalSession.ID]
		assert.True(t, exists, "GetMany should return session %s", originalSession.ID)
		assert.Equal(t, originalSession.ID, retrievedSession.ID, "Retrieved session ID should match")
		assert.Equal(t, originalSession.UserID, retrievedSession.UserID, "Retrieved session UserID should match")
		assert.Equal(t, originalSession.Username, retrievedSession.Username, "Retrieved session Username should match")
	}

	// Verify non-existent key is not in results
	_, exists := results["session:nonexistent-getmany"]
	assert.False(t, exists, "GetMany should not return non-existent keys")

	t.Logf("✅ GetMany test successful")
	t.Logf("   - Keys requested: %d", len(keys))
	t.Logf("   - Sessions found: %d", len(results))
}

// TestRedisCache_SetMany tests batch SET operations
func TestRedisCache_SetMany(t *testing.T) {
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

	// Create multiple test sessions
	sessions := []*testintegration.TestSession{
		{
			ID:       "session:setmany-1",
			UserID:   "user1300",
			Username: "setmanyuser1",
			Created:  time.Now(),
		},
		{
			ID:       "session:setmany-2",
			UserID:   "user1301",
			Username: "setmanyuser2",
			Created:  time.Now(),
		},
		{
			ID:       "session:setmany-3",
			UserID:   "user1302",
			Username: "setmanyuser3",
			Created:  time.Now(),
		},
	}

	t.Logf("📝 Testing SetMany operation with %d sessions", len(sessions))

	// SetMany operation
	err = cache.SetMany(ctx, sessions, 0)
	require.NoError(t, err, "SetMany operation should not error")

	// Verify all sessions were set correctly
	for _, originalSession := range sessions {
		retrievedSession, found, err := cache.Get(ctx, originalSession.ID)
		assert.NoError(t, err, "GET should not error after SetMany")
		assert.True(t, found, "Session %s should exist after SetMany", originalSession.ID)
		assert.Equal(t, originalSession.ID, retrievedSession.ID, "Retrieved session ID should match")
		assert.Equal(t, originalSession.UserID, retrievedSession.UserID, "Retrieved session UserID should match")
		assert.Equal(t, originalSession.Username, retrievedSession.Username, "Retrieved session Username should match")
	}

	t.Logf("✅ SetMany test successful")
	t.Logf("   - Sessions set: %d", len(sessions))
}

// TestRedisCache_DeleteMany tests batch DELETE operations
func TestRedisCache_DeleteMany(t *testing.T) {
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

	// Create multiple test sessions
	sessions := []*testintegration.TestSession{
		{
			ID:       "session:deletemany-1",
			UserID:   "user1400",
			Username: "deletemanyuser1",
			Created:  time.Now(),
		},
		{
			ID:       "session:deletemany-2",
			UserID:   "user1401",
			Username: "deletemanyuser2",
			Created:  time.Now(),
		},
		{
			ID:       "session:deletemany-3",
			UserID:   "user1402",
			Username: "deletemanyuser3",
			Created:  time.Now(),
		},
	}

	t.Logf("📝 Testing DeleteMany operation with %d sessions", len(sessions))

	// SET all sessions first
	for _, session := range sessions {
		err = cache.Set(ctx, session, 0)
		require.NoError(t, err, "SET operation should not error")
	}

	// Verify all sessions exist
	for _, session := range sessions {
		exists := cache.Has(ctx, session.ID)
		assert.True(t, exists, "Session %s should exist before DeleteMany", session.ID)
	}

	// Prepare keys for DeleteMany
	keys := make([]string, len(sessions))
	for i, session := range sessions {
		keys[i] = session.ID
	}

	// Add a non-existent key to test mixed deletion
	keys = append(keys, "session:nonexistent-deletemany")

	// DeleteMany operation
	err = cache.DeleteMany(ctx, keys)
	require.NoError(t, err, "DeleteMany operation should not error")

	// Verify all sessions were deleted
	for _, session := range sessions {
		exists := cache.Has(ctx, session.ID)
		assert.False(t, exists, "Session %s should not exist after DeleteMany", session.ID)
		
		// Double check with GET
		retrievedSession, found, err := cache.Get(ctx, session.ID)
		assert.NoError(t, err, "GET should not error after DeleteMany")
		assert.False(t, found, "GET should return found=false after DeleteMany")
		assert.Nil(t, retrievedSession, "GET should return nil after DeleteMany")
	}

	t.Logf("✅ DeleteMany test successful")
	t.Logf("   - Keys deleted: %d", len(keys))
}

// TestRedisCache_SetIfNotExists tests conditional SET operation
func TestRedisCache_SetIfNotExists(t *testing.T) {
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

	testSession := &testintegration.TestSession{
		ID:       "session:setifnotexists",
		UserID:   "user1500",
		Username: "setifnotexistsuser",
		Created:  time.Now(),
	}

	t.Logf("📝 Testing SetIfNotExists operation for session ID: %s", testSession.ID)

	// First SetIfNotExists - should succeed
	wasSet, err := cache.SetIfNotExists(ctx, testSession, 0)
	require.NoError(t, err, "SetIfNotExists should not error on first call")
	assert.True(t, wasSet, "SetIfNotExists should return true on first call")

	// Verify session was set
	exists := cache.Has(ctx, testSession.ID)
	assert.True(t, exists, "Session should exist after successful SetIfNotExists")

	// Second SetIfNotExists with different data - should fail
	updatedSession := &testintegration.TestSession{
		ID:       "session:setifnotexists", // Same ID
		UserID:   "user1501", // Different data
		Username: "differentuser",
		Created:  time.Now().Add(time.Hour),
	}

	wasSet, err = cache.SetIfNotExists(ctx, updatedSession, 0)
	require.NoError(t, err, "SetIfNotExists should not error on second call")
	assert.False(t, wasSet, "SetIfNotExists should return false when key exists")

	// Verify original session is unchanged
	retrievedSession, found, err := cache.Get(ctx, testSession.ID)
	assert.NoError(t, err, "GET should not error")
	assert.True(t, found, "Session should still exist")
	assert.Equal(t, testSession.UserID, retrievedSession.UserID, "Original session data should be preserved")
	assert.Equal(t, testSession.Username, retrievedSession.Username, "Original session username should be preserved")

	t.Logf("✅ SetIfNotExists test successful")
	t.Logf("   - First call result: %v", true)
	t.Logf("   - Second call result: %v", false)
}

// TestRedisCache_SetIfExists tests conditional SET operation for existing keys
func TestRedisCache_SetIfExists(t *testing.T) {
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

	originalSession := &testintegration.TestSession{
		ID:       "session:setifexists",
		UserID:   "user1600",
		Username: "setifexistsuser",
		Created:  time.Now(),
	}

	updatedSession := &testintegration.TestSession{
		ID:       "session:setifexists", // Same ID
		UserID:   "user1601", // Different data
		Username: "updateduser",
		Created:  time.Now().Add(time.Hour),
	}

	t.Logf("📝 Testing SetIfExists operation for session ID: %s", originalSession.ID)

	// SetIfExists on non-existent key - should fail
	wasSet, err := cache.SetIfExists(ctx, originalSession, 0)
	require.NoError(t, err, "SetIfExists should not error on non-existent key")
	assert.False(t, wasSet, "SetIfExists should return false for non-existent key")

	// Verify no session was created
	exists := cache.Has(ctx, originalSession.ID)
	assert.False(t, exists, "Session should not exist after failed SetIfExists")

	// Create the session first
	err = cache.Set(ctx, originalSession, 0)
	require.NoError(t, err, "Initial SET should not error")

	// SetIfExists on existing key - should succeed
	wasSet, err = cache.SetIfExists(ctx, updatedSession, 0)
	require.NoError(t, err, "SetIfExists should not error on existing key")
	assert.True(t, wasSet, "SetIfExists should return true for existing key")

	// Verify session was updated
	retrievedSession, found, err := cache.Get(ctx, originalSession.ID)
	assert.NoError(t, err, "GET should not error")
	assert.True(t, found, "Session should exist after SetIfExists")
	assert.Equal(t, updatedSession.UserID, retrievedSession.UserID, "Session should have updated UserID")
	assert.Equal(t, updatedSession.Username, retrievedSession.Username, "Session should have updated Username")

	t.Logf("✅ SetIfExists test successful")
	t.Logf("   - Call on non-existent key: %v", false)
	t.Logf("   - Call on existing key: %v", true)
}

// TestRedisCache_GetKeysByPattern tests pattern-based key retrieval
func TestRedisCache_GetKeysByPattern(t *testing.T) {
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

	// Create sessions with different prefixes
	sessions := []*testintegration.TestSession{
		{
			ID:       "session:pattern-test-1",
			UserID:   "user1700",
			Username: "patternuser1",
			Created:  time.Now(),
		},
		{
			ID:       "session:pattern-test-2",
			UserID:   "user1701",
			Username: "patternuser2",
			Created:  time.Now(),
		},
		{
			ID:       "session:different-prefix-1",
			UserID:   "user1702",
			Username: "differentuser1",
			Created:  time.Now(),
		},
		{
			ID:       "other:pattern-test-3",
			UserID:   "user1703",
			Username: "otheruser",
			Created:  time.Now(),
		},
	}

	t.Logf("📝 Testing GetKeysByPattern operation")

	// SET all sessions
	for _, session := range sessions {
		err = cache.Set(ctx, session, 0)
		require.NoError(t, err, "SET operation should not error")
	}

	// Test pattern matching for "session:pattern-test-*"
	pattern := "session:pattern-test-*"
	matchingKeys, err := cache.GetKeysByPattern(ctx, pattern)
	require.NoError(t, err, "GetKeysByPattern should not error")

	// Should match exactly 2 keys
	assert.Len(t, matchingKeys, 2, "Pattern should match exactly 2 keys")
	assert.Contains(t, matchingKeys, "session:pattern-test-1", "Should contain first pattern match")
	assert.Contains(t, matchingKeys, "session:pattern-test-2", "Should contain second pattern match")
	assert.NotContains(t, matchingKeys, "session:different-prefix-1", "Should not contain different prefix")
	assert.NotContains(t, matchingKeys, "other:pattern-test-3", "Should not contain different namespace")

	// Test broader pattern "session:*"
	broadPattern := "session:*"
	broadMatchingKeys, err := cache.GetKeysByPattern(ctx, broadPattern)
	require.NoError(t, err, "GetKeysByPattern should not error for broad pattern")

	// Should match 3 keys (all session: prefixed keys)
	assert.Len(t, broadMatchingKeys, 3, "Broad pattern should match 3 keys")
	assert.Contains(t, broadMatchingKeys, "session:pattern-test-1")
	assert.Contains(t, broadMatchingKeys, "session:pattern-test-2")
	assert.Contains(t, broadMatchingKeys, "session:different-prefix-1")
	assert.NotContains(t, broadMatchingKeys, "other:pattern-test-3", "Should not contain different namespace")

	t.Logf("✅ GetKeysByPattern test successful")
	t.Logf("   - Pattern '%s' matched: %d keys", pattern, len(matchingKeys))
	t.Logf("   - Broad pattern '%s' matched: %d keys", broadPattern, len(broadMatchingKeys))
}

// TestRedisCache_GetMetadata tests metadata retrieval
func TestRedisCache_GetMetadata(t *testing.T) {
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

	testSession := &testintegration.TestSession{
		ID:       "session:metadata-test",
		UserID:   "user1800",
		Username: "metadatauser",
		Created:  time.Now(),
	}

	t.Logf("📝 Testing GetMetadata operation for session ID: %s", testSession.ID)

	// GetMetadata on non-existent key
	metadata, err := cache.GetMetadata(ctx, testSession.ID)
	require.NoError(t, err, "GetMetadata should not error for non-existent key")
	assert.Nil(t, metadata, "GetMetadata should return nil for non-existent key")

	// Set the session
	err = cache.Set(ctx, testSession, time.Hour)
	require.NoError(t, err, "SET operation should not error")

	// GetMetadata on existing key
	metadata, err = cache.GetMetadata(ctx, testSession.ID)
	require.NoError(t, err, "GetMetadata should not error for existing key")
	
	if metadata != nil {
		assert.Equal(t, testSession.ID, metadata.Key, "Metadata key should match session ID")
		assert.False(t, metadata.CreatedAt.IsZero(), "Metadata should have CreatedAt time")
		assert.False(t, metadata.LastAccessed.IsZero(), "Metadata should have LastAccessed time")
		assert.GreaterOrEqual(t, metadata.AccessCount, int64(0), "Access count should be non-negative")
		assert.Greater(t, metadata.Size, int64(0), "Metadata should have positive size")

		t.Logf("   - Key: %s", metadata.Key)
		t.Logf("   - CreatedAt: %v", metadata.CreatedAt)
		t.Logf("   - LastAccessed: %v", metadata.LastAccessed)
		t.Logf("   - AccessCount: %d", metadata.AccessCount)
		t.Logf("   - Size: %d bytes", metadata.Size)
	}

	t.Logf("✅ GetMetadata test successful")
}

// TestRedisCache_TTLBehavior tests TTL (Time To Live) functionality
func TestRedisCache_TTLBehavior(t *testing.T) {
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

	testSession := &testintegration.TestSession{
		ID:       "session:ttl-test",
		UserID:   "user1900",
		Username: "ttluser",
		Created:  time.Now(),
	}

	t.Logf("📝 Testing TTL behavior for session ID: %s", testSession.ID)

	// Set session with short TTL
	shortTTL := 100 * time.Millisecond
	err = cache.Set(ctx, testSession, shortTTL)
	require.NoError(t, err, "SET with TTL should not error")

	// Verify session exists immediately
	exists := cache.Has(ctx, testSession.ID)
	assert.True(t, exists, "Session should exist immediately after SET")

	// Wait for TTL to expire
	time.Sleep(shortTTL + 50*time.Millisecond)

	// Verify session no longer exists
	exists = cache.Has(ctx, testSession.ID)
	assert.False(t, exists, "Session should not exist after TTL expires")

	// Double check with GET
	retrievedSession, found, err := cache.Get(ctx, testSession.ID)
	assert.NoError(t, err, "GET should not error after TTL expiry")
	assert.False(t, found, "GET should return found=false after TTL expiry")
	assert.Nil(t, retrievedSession, "GET should return nil after TTL expiry")

	t.Logf("✅ TTL behavior test successful")
	t.Logf("   - TTL duration: %v", shortTTL)
	t.Logf("   - Session expired as expected")
}
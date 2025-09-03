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

// TestRedisCache_BasicGet tests a simple GET operation that should miss (no data present)
func TestRedisCache_BasicGet(t *testing.T) {
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

	// Test GET operation on non-existent key - should miss
	testKey := "session:nonexistent"

	t.Logf("🔍 Testing GET operation for key: %s", testKey)

	session, found, err := cache.Get(ctx, testKey)

	// Assertions
	assert.NoError(t, err, "GET operation should not error")
	assert.False(t, found, "GET should return found=false for non-existent key")
	assert.Nil(t, session, "GET should return nil value for non-existent key")

	t.Logf("✅ GET test successful - key not found as expected")
	t.Logf("   - Key: %s", testKey)
	t.Logf("   - Found: %v", found)
	t.Logf("   - Error: %v", err)
}

// TestRedisCache_BasicSet tests a simple SET operation
func TestRedisCache_BasicSet(t *testing.T) {
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

	// Create test session
	testSession := &testintegration.TestSession{
		ID:       "session:test-set",
		UserID:   "user123",
		Username: "testuser",
		Created:  time.Now(),
	}

	t.Logf("📝 Testing SET operation for session ID: %s", testSession.ID)

	err = cache.Set(ctx, testSession, 0)

	// Assertions
	assert.NoError(t, err, "SET operation should not error")

	t.Logf("✅ SET test successful")
	t.Logf("   - Session ID: %s", testSession.ID)
	t.Logf("   - User ID: %s", testSession.UserID)
	t.Logf("   - Error: %v", err)
}

// TestRedisCache_SetThenGet tests SET followed by GET in the same test
func TestRedisCache_SetThenGet(t *testing.T) {
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

	// Create test session
	testSession := &testintegration.TestSession{
		ID:       "session:test-set-get",
		UserID:   "user456",
		Username: "testuser2",
		Created:  time.Now(),
	}

	t.Logf("📝 Testing SET then GET operation for session ID: %s", testSession.ID)

	// SET operation
	err = cache.Set(ctx, testSession, 0)
	require.NoError(t, err, "SET operation should not error")

	// GET operation
	retrievedSession, found, err := cache.Get(ctx, testSession.ID)

	// Assertions
	assert.NoError(t, err, "GET operation should not error")
	assert.True(t, found, "GET should return found=true for existing key")
	assert.NotNil(t, retrievedSession, "GET should return non-nil value for existing key")
	assert.Equal(t, testSession.ID, retrievedSession.ID, "Retrieved session should match original")
	assert.Equal(t, testSession.UserID, retrievedSession.UserID, "Retrieved session user ID should match original")
	assert.Equal(t, testSession.Username, retrievedSession.Username, "Retrieved session username should match original")

	t.Logf("✅ SET then GET test successful")
	t.Logf("   - Session ID: %s", testSession.ID)
	t.Logf("   - Original Session ID: %s", testSession.ID)
	t.Logf("   - Retrieved Session ID: %s", retrievedSession.ID)
	t.Logf("   - Found: %v", found)
	t.Logf("   - Error: %v", err)
}

// TestRedisCache_SetDeleteGet tests SET > DELETE > GET sequence
func TestRedisCache_SetDeleteGet(t *testing.T) {
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

	// Create test session
	testSession := &testintegration.TestSession{
		ID:       "session:test-delete",
		UserID:   "user789",
		Username: "deletetest",
		Created:  time.Now(),
	}

	t.Logf("📝 Testing SET > DELETE > GET sequence for session ID: %s", testSession.ID)

	// SET operation
	err = cache.Set(ctx, testSession, 0)
	require.NoError(t, err, "SET operation should not error")

	// DELETE operation
	err = cache.Delete(ctx, testSession.ID)
	require.NoError(t, err, "DELETE operation should not error")

	// GET operation (should miss)
	retrievedSession, found, err := cache.Get(ctx, testSession.ID)

	// Assertions
	assert.NoError(t, err, "GET operation should not error")
	assert.False(t, found, "GET should return found=false for deleted key")
	assert.Nil(t, retrievedSession, "GET should return nil value for deleted key")

	t.Logf("✅ SET > DELETE > GET test successful")
	t.Logf("   - Session ID: %s", testSession.ID)
	t.Logf("   - Found after delete: %v", found)
	t.Logf("   - Error: %v", err)
}

// TestRedisCache_SetHas tests SET > HAS sequence
func TestRedisCache_SetHas(t *testing.T) {
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

	// Create test session
	testSession := &testintegration.TestSession{
		ID:       "session:test-has",
		UserID:   "user101",
		Username: "hastest",
		Created:  time.Now(),
	}

	t.Logf("📝 Testing SET > HAS sequence for session ID: %s", testSession.ID)

	// HAS operation (should be false before SET)
	exists := cache.Has(ctx, testSession.ID)
	assert.False(t, exists, "HAS should return false before SET")

	// SET operation
	err = cache.Set(ctx, testSession, 0)
	require.NoError(t, err, "SET operation should not error")

	// HAS operation (should be true after SET)
	exists = cache.Has(ctx, testSession.ID)
	assert.True(t, exists, "HAS should return true after SET")

	t.Logf("✅ SET > HAS test successful")
	t.Logf("   - Session ID: %s", testSession.ID)
	t.Logf("   - Exists after SET: %v", exists)
}

// TestRedisCache_SetHasDeleteHas tests SET > HAS > DELETE > HAS sequence
func TestRedisCache_SetHasDeleteHas(t *testing.T) {
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

	// Create test session
	testSession := &testintegration.TestSession{
		ID:       "session:test-has-delete-has",
		UserID:   "user202",
		Username: "hasdeletetest",
		Created:  time.Now(),
	}

	t.Logf("📝 Testing SET > HAS > DELETE > HAS sequence for session ID: %s", testSession.ID)

	// SET operation
	err = cache.Set(ctx, testSession, 0)
	require.NoError(t, err, "SET operation should not error")

	// HAS operation (should be true after SET)
	exists := cache.Has(ctx, testSession.ID)
	assert.True(t, exists, "HAS should return true after SET")

	// DELETE operation
	err = cache.Delete(ctx, testSession.ID)
	require.NoError(t, err, "DELETE operation should not error")

	// HAS operation (should be false after DELETE)
	exists = cache.Has(ctx, testSession.ID)
	assert.False(t, exists, "HAS should return false after DELETE")

	t.Logf("✅ SET > HAS > DELETE > HAS test successful")
	t.Logf("   - Session ID: %s", testSession.ID)
	t.Logf("   - Exists after DELETE: %v", exists)
}

// TestRedisCache_MultipleSetGet tests setting and getting multiple different sessions
func TestRedisCache_MultipleSetGet(t *testing.T) {
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
			ID:       "session:multi-1",
			UserID:   "user301",
			Username: "multiuser1",
			Created:  time.Now(),
		},
		{
			ID:       "session:multi-2",
			UserID:   "user302",
			Username: "multiuser2",
			Created:  time.Now(),
		},
	}

	t.Logf("📝 Testing multiple SET > GET operations")

	// SET all sessions
	for _, session := range sessions {
		err = cache.Set(ctx, session, 0)
		require.NoError(t, err, "SET operation should not error for session %s", session.ID)
	}

	// GET all sessions and verify
	for _, originalSession := range sessions {
		retrievedSession, found, err := cache.Get(ctx, originalSession.ID)
		
		assert.NoError(t, err, "GET operation should not error for session %s", originalSession.ID)
		assert.True(t, found, "GET should return found=true for session %s", originalSession.ID)
		assert.NotNil(t, retrievedSession, "GET should return non-nil value for session %s", originalSession.ID)
		assert.Equal(t, originalSession.ID, retrievedSession.ID, "Retrieved session ID should match original")
		assert.Equal(t, originalSession.UserID, retrievedSession.UserID, "Retrieved session UserID should match original")
		assert.Equal(t, originalSession.Username, retrievedSession.Username, "Retrieved session Username should match original")
	}

	t.Logf("✅ Multiple SET > GET test successful")
	t.Logf("   - Sessions tested: %d", len(sessions))
}

// TestRedisCache_DeleteNonExistent tests DELETE operation on non-existent key
func TestRedisCache_DeleteNonExistent(t *testing.T) {
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

	testKey := "session:nonexistent-delete"

	t.Logf("📝 Testing DELETE operation on non-existent key: %s", testKey)

	// DELETE operation on non-existent key - should not error
	err = cache.Delete(ctx, testKey)

	// Assertions
	assert.NoError(t, err, "DELETE operation should not error on non-existent key")

	t.Logf("✅ DELETE non-existent key test successful")
	t.Logf("   - Key: %s", testKey)
	t.Logf("   - Error: %v", err)
}

// TestRedisCache_HasNonExistent tests HAS operation on non-existent key
func TestRedisCache_HasNonExistent(t *testing.T) {
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

	testKey := "session:nonexistent-has"

	t.Logf("📝 Testing HAS operation on non-existent key: %s", testKey)

	// HAS operation on non-existent key - should return false
	exists := cache.Has(ctx, testKey)

	// Assertions
	assert.False(t, exists, "HAS should return false for non-existent key")

	t.Logf("✅ HAS non-existent key test successful")
	t.Logf("   - Key: %s", testKey)
	t.Logf("   - Exists: %v", exists)
}

// TestRedisCache_SetOverwrite tests overwriting an existing key
func TestRedisCache_SetOverwrite(t *testing.T) {
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

	sessionID := "session:overwrite-test"

	// Create first session
	originalSession := &testintegration.TestSession{
		ID:       sessionID,
		UserID:   "user500",
		Username: "originaluser",
		Created:  time.Now(),
	}

	// Create updated session with same ID
	updatedSession := &testintegration.TestSession{
		ID:       sessionID,
		UserID:   "user600",
		Username: "updateduser",
		Created:  time.Now().Add(time.Hour),
	}

	t.Logf("📝 Testing SET overwrite for session ID: %s", sessionID)

	// SET original session
	err = cache.Set(ctx, originalSession, 0)
	require.NoError(t, err, "Original SET operation should not error")

	// SET updated session (overwrite)
	err = cache.Set(ctx, updatedSession, 0)
	require.NoError(t, err, "Overwrite SET operation should not error")

	// GET to verify overwrite worked
	retrievedSession, found, err := cache.Get(ctx, sessionID)

	// Assertions
	assert.NoError(t, err, "GET operation should not error")
	assert.True(t, found, "GET should return found=true")
	assert.NotNil(t, retrievedSession, "GET should return non-nil value")
	assert.Equal(t, updatedSession.ID, retrievedSession.ID, "Retrieved session ID should match updated")
	assert.Equal(t, updatedSession.UserID, retrievedSession.UserID, "Retrieved session should have updated UserID")
	assert.Equal(t, updatedSession.Username, retrievedSession.Username, "Retrieved session should have updated Username")
	assert.NotEqual(t, originalSession.UserID, retrievedSession.UserID, "Retrieved session should not have original UserID")

	t.Logf("✅ SET overwrite test successful")
	t.Logf("   - Session ID: %s", sessionID)
	t.Logf("   - Original UserID: %s", originalSession.UserID)
	t.Logf("   - Updated UserID: %s", retrievedSession.UserID)
}

// TestRedisCache_Clear tests clearing all cache entries
func TestRedisCache_Clear(t *testing.T) {
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
			ID:       "session:clear-1",
			UserID:   "user700",
			Username: "clearuser1",
			Created:  time.Now(),
		},
		{
			ID:       "session:clear-2", 
			UserID:   "user701",
			Username: "clearuser2",
			Created:  time.Now(),
		},
		{
			ID:       "session:clear-3",
			UserID:   "user702",
			Username: "clearuser3",
			Created:  time.Now(),
		},
	}

	t.Logf("📝 Testing CLEAR operation with %d sessions", len(sessions))

	// SET all sessions
	for _, session := range sessions {
		err = cache.Set(ctx, session, 0)
		require.NoError(t, err, "SET operation should not error for session %s", session.ID)
		
		// Verify session was set
		exists := cache.Has(ctx, session.ID)
		assert.True(t, exists, "Session %s should exist after SET", session.ID)
	}

	// CLEAR all entries
	err = cache.Clear(ctx)
	require.NoError(t, err, "CLEAR operation should not error")

	// Verify all sessions were cleared
	for _, session := range sessions {
		exists := cache.Has(ctx, session.ID)
		assert.False(t, exists, "Session %s should not exist after CLEAR", session.ID)
		
		// Double check with GET
		retrievedSession, found, err := cache.Get(ctx, session.ID)
		assert.NoError(t, err, "GET should not error after CLEAR")
		assert.False(t, found, "GET should return found=false after CLEAR for session %s", session.ID)
		assert.Nil(t, retrievedSession, "GET should return nil after CLEAR for session %s", session.ID)
	}

	t.Logf("✅ CLEAR test successful")
	t.Logf("   - Sessions cleared: %d", len(sessions))
}

// TestRedisCache_EmptyValues tests handling of sessions with empty/zero values
func TestRedisCache_EmptyValues(t *testing.T) {
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

	// Create session with minimal values
	testSession := &testintegration.TestSession{
		ID:       "session:empty-values",
		UserID:   "", // Empty string
		Username: "", // Empty string
		Created:  time.Time{}, // Zero time
	}

	t.Logf("📝 Testing SET/GET with empty values for session ID: %s", testSession.ID)

	// SET operation
	err = cache.Set(ctx, testSession, 0)
	require.NoError(t, err, "SET operation should not error with empty values")

	// GET operation
	retrievedSession, found, err := cache.Get(ctx, testSession.ID)

	// Assertions
	assert.NoError(t, err, "GET operation should not error")
	assert.True(t, found, "GET should return found=true")
	assert.NotNil(t, retrievedSession, "GET should return non-nil value")
	assert.Equal(t, testSession.ID, retrievedSession.ID, "Retrieved session ID should match")
	assert.Equal(t, "", retrievedSession.UserID, "Retrieved session should preserve empty UserID")
	assert.Equal(t, "", retrievedSession.Username, "Retrieved session should preserve empty Username")
	assert.True(t, retrievedSession.Created.IsZero(), "Retrieved session should preserve zero Created time")

	t.Logf("✅ Empty values test successful")
	t.Logf("   - Session ID: %s", testSession.ID)
	t.Logf("   - Empty UserID preserved: %v", retrievedSession.UserID == "")
	t.Logf("   - Empty Username preserved: %v", retrievedSession.Username == "")
	t.Logf("   - Zero time preserved: %v", retrievedSession.Created.IsZero())
}
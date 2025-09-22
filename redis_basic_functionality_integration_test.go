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

// TestRedisCache_SetOverwriteTTL tests overwrite behavior with different TTL scenarios
func TestRedisCache_SetOverwriteTTL(t *testing.T) {
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

	sessionID := "session:ttl-overwrite-test"

	t.Run("TTL_to_Different_TTL", func(t *testing.T) {
		originalSession := &testintegration.TestSession{
			ID:       sessionID,
			UserID:   "user600",
			Username: "originalttl",
			Created:  time.Now(),
		}

		updatedSession := &testintegration.TestSession{
			ID:       sessionID,
			UserID:   "user601",
			Username: "updatedttl",
			Created:  time.Now().Add(time.Hour),
		}

		// SET with 200ms TTL
		err = cache.Set(ctx, originalSession, 200*time.Millisecond)
		require.NoError(t, err, "Original SET with TTL should not error")

		// Verify exists
		exists := cache.Has(ctx, sessionID)
		assert.True(t, exists, "Session should exist after SET")

		// Wait a bit but not full TTL
		time.Sleep(50 * time.Millisecond)

		// Overwrite with different 300ms TTL
		err = cache.Set(ctx, updatedSession, 300*time.Millisecond)
		require.NoError(t, err, "Overwrite SET with different TTL should not error")

		// Verify overwrite worked and data is updated
		retrievedSession, found, err := cache.Get(ctx, sessionID)
		assert.NoError(t, err, "GET should not error")
		assert.True(t, found, "Session should be found after overwrite")
		assert.Equal(t, updatedSession.UserID, retrievedSession.UserID, "Should have updated data")

		// Wait for original TTL to pass (should still exist due to new TTL)
		time.Sleep(200 * time.Millisecond)
		exists = cache.Has(ctx, sessionID)
		assert.True(t, exists, "Session should still exist - new TTL should apply")

		// Wait for new TTL to pass
		time.Sleep(150 * time.Millisecond)
		exists = cache.Has(ctx, sessionID)
		assert.False(t, exists, "Session should expire after new TTL")

		t.Logf("✅ TTL to different TTL overwrite test successful")
	})

	t.Run("TTL_to_No_TTL", func(t *testing.T) {
		setup.FlushRedis(ctx, t) // Clean slate

		originalSession := &testintegration.TestSession{
			ID:       sessionID,
			UserID:   "user602",
			Username: "originalttl2",
			Created:  time.Now(),
		}

		updatedSession := &testintegration.TestSession{
			ID:       sessionID,
			UserID:   "user603",
			Username: "nottl",
			Created:  time.Now().Add(time.Hour),
		}

		// SET with 100ms TTL
		err = cache.Set(ctx, originalSession, 100*time.Millisecond)
		require.NoError(t, err, "Original SET with TTL should not error")

		// Overwrite with no TTL (TTL=0)
		err = cache.Set(ctx, updatedSession, 0)
		require.NoError(t, err, "Overwrite SET with no TTL should not error")

		// Wait for original TTL to pass
		time.Sleep(150 * time.Millisecond)

		// Should still exist - no TTL means no expiration
		retrievedSession, found, err := cache.Get(ctx, sessionID)
		assert.NoError(t, err, "GET should not error")
		assert.True(t, found, "Session should still exist - no TTL")
		assert.Equal(t, updatedSession.UserID, retrievedSession.UserID, "Should have updated data")

		t.Logf("✅ TTL to no TTL overwrite test successful")
	})

	t.Run("No_TTL_to_TTL", func(t *testing.T) {
		setup.FlushRedis(ctx, t) // Clean slate

		originalSession := &testintegration.TestSession{
			ID:       sessionID,
			UserID:   "user604",
			Username: "nottle",
			Created:  time.Now(),
		}

		updatedSession := &testintegration.TestSession{
			ID:       sessionID,
			UserID:   "user605",
			Username: "withttl",
			Created:  time.Now().Add(time.Hour),
		}

		// SET with no TTL
		err = cache.Set(ctx, originalSession, 0)
		require.NoError(t, err, "Original SET with no TTL should not error")

		// Overwrite with TTL
		err = cache.Set(ctx, updatedSession, 100*time.Millisecond)
		require.NoError(t, err, "Overwrite SET with TTL should not error")

		// Verify overwrite worked
		retrievedSession, found, err := cache.Get(ctx, sessionID)
		assert.NoError(t, err, "GET should not error")
		assert.True(t, found, "Session should be found after overwrite")
		assert.Equal(t, updatedSession.UserID, retrievedSession.UserID, "Should have updated data")

		// Wait for TTL to expire
		time.Sleep(150 * time.Millisecond)
		exists := cache.Has(ctx, sessionID)
		assert.False(t, exists, "Session should expire after TTL")

		t.Logf("✅ No TTL to TTL overwrite test successful")
	})
}

// TestRedisCache_MultipleSequentialOverwrites tests multiple overwrites in sequence
func TestRedisCache_MultipleSequentialOverwrites(t *testing.T) {
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

	sessionID := "session:multiple-overwrites"

	// Create multiple versions
	sessions := []*testintegration.TestSession{
		{ID: sessionID, UserID: "user700", Username: "version1", Created: time.Now()},
		{ID: sessionID, UserID: "user701", Username: "version2", Created: time.Now().Add(time.Minute)},
		{ID: sessionID, UserID: "user702", Username: "version3", Created: time.Now().Add(2 * time.Minute)},
		{ID: sessionID, UserID: "user703", Username: "version4", Created: time.Now().Add(3 * time.Minute)},
	}

	t.Logf("📝 Testing multiple sequential overwrites for session ID: %s", sessionID)

	// Perform sequential overwrites
	for i, session := range sessions {
		err = cache.Set(ctx, session, 0)
		require.NoError(t, err, "Overwrite %d should not error", i+1)

		// Verify current version is correct
		retrievedSession, found, err := cache.Get(ctx, sessionID)
		assert.NoError(t, err, "GET should not error for overwrite %d", i+1)
		assert.True(t, found, "Session should be found for overwrite %d", i+1)
		assert.Equal(t, session.UserID, retrievedSession.UserID, "Should have version %d data", i+1)
		assert.Equal(t, session.Username, retrievedSession.Username, "Should have version %d username", i+1)
	}

	// Final verification - should have the last version
	finalSession, found, err := cache.Get(ctx, sessionID)
	assert.NoError(t, err, "Final GET should not error")
	assert.True(t, found, "Final session should be found")
	assert.Equal(t, sessions[len(sessions)-1].UserID, finalSession.UserID, "Should have final version data")
	assert.Equal(t, "version4", finalSession.Username, "Should have final version username")

	t.Logf("✅ Multiple sequential overwrites test successful")
	t.Logf("   - Total overwrites: %d", len(sessions))
	t.Logf("   - Final version: %s", finalSession.Username)
}

// TestRedisCache_OverwriteIdenticalData tests overwriting with identical data
func TestRedisCache_OverwriteIdenticalData(t *testing.T) {
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

	sessionID := "session:identical-overwrite"

	// Create identical sessions
	originalSession := &testintegration.TestSession{
		ID:       sessionID,
		UserID:   "user800",
		Username: "identicaluser",
		Created:  time.Now().Truncate(time.Second), // Truncate to avoid microsecond differences
	}

	identicalSession := &testintegration.TestSession{
		ID:       sessionID,
		UserID:   "user800",
		Username: "identicaluser",
		Created:  originalSession.Created, // Same time
	}

	t.Logf("📝 Testing overwrite with identical data for session ID: %s", sessionID)

	// SET original
	err = cache.Set(ctx, originalSession, 0)
	require.NoError(t, err, "Original SET should not error")

	// GET original
	retrievedSession1, found1, err := cache.Get(ctx, sessionID)
	require.NoError(t, err, "First GET should not error")
	require.True(t, found1, "First GET should find session")

	// "Overwrite" with identical data
	err = cache.Set(ctx, identicalSession, 0)
	require.NoError(t, err, "Identical overwrite SET should not error")

	// GET after overwrite
	retrievedSession2, found2, err := cache.Get(ctx, sessionID)
	assert.NoError(t, err, "Second GET should not error")
	assert.True(t, found2, "Second GET should find session")
	assert.Equal(t, retrievedSession1.UserID, retrievedSession2.UserID, "UserID should remain same")
	assert.Equal(t, retrievedSession1.Username, retrievedSession2.Username, "Username should remain same")
	assert.Equal(t, retrievedSession1.Created, retrievedSession2.Created, "Created time should remain same")

	t.Logf("✅ Identical data overwrite test successful")
	t.Logf("   - Data preserved correctly")
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
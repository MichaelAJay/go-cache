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

// TestRedisCache_BasicInitialization tests that a RedisCache can be created and initialized properly
func TestRedisCache_BasicInitialization(t *testing.T) {
	t.Skip()
	ctx := context.Background()

	// Setup test environment using existing container infrastructure
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Test basic cache creation without indexing
	config := testintegration.DefaultCacheConfig()
	config.IndexingMode = false

	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create basic cache")
	require.NotNil(t, cache, "Cache should not be nil")

	// Test cache can be closed
	err = cache.Close()
	assert.NoError(t, err, "Cache close should not error")

	t.Logf("✅ Basic cache initialization successful")
}

// TestRedisCache_IndexedInitialization tests cache creation with indexing enabled
func TestRedisCache_IndexedInitialization(t *testing.T) {
	t.Skip()
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Test indexed cache creation
	config := testintegration.IndexedCacheConfig()

	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create indexed cache")
	require.NotNil(t, cache, "Cache should not be nil")

	// Test cache can be closed
	err = cache.Close()
	assert.NoError(t, err, "Cache close should not error")

	t.Logf("✅ Indexed cache initialization successful")
}

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

// TestRedisCache_SerializationFormats tests cache creation with different serialization formats
func TestRedisCache_SerializationFormats(t *testing.T) {
	t.Skip()
	ctx := context.Background()

	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Test different serialization formats
	formats := []string{"json", "gob", "msgpack"}

	for _, format := range formats {
		t.Run("Format_"+format, func(t *testing.T) {
			config := testintegration.DefaultCacheConfig()
			config.SerializerFormat = format

			cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
			require.NoError(t, err, "Failed to create cache with %s serialization", format)
			require.NotNil(t, cache, "Cache should not be nil")

			// Test basic operation works
			session, found, err := cache.Get(ctx, "test:key")
			assert.NoError(t, err, "GET should work with %s serialization", format)
			assert.False(t, found, "GET should miss for non-existent key")
			assert.Nil(t, session, "Value should be nil for cache miss")

			cache.Close()
			t.Logf("✅ %s serialization test successful", format)
		})
	}
}

// TestRedisCache_ConnectionValidation tests that cache properly validates Redis connection
func TestRedisCache_ConnectionValidation(t *testing.T) {
	t.Skip()
	ctx := context.Background()

	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)

	t.Logf("🔗 Testing Redis connection validation")
	t.Logf("   - Redis Address: %s", setup.TestEnv.GetRedisAddr())
	t.Logf("   - Test Mode: %s", setup.TestEnv.GetMode().String())

	// Verify we can create a cache and it connects properly
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Cache creation should succeed with valid Redis connection")
	require.NotNil(t, cache, "Cache should not be nil")

	// Test a simple operation to ensure connection works
	exists := cache.Has(ctx, "test:connection:check")
	assert.False(t, exists, "Key should not exist in fresh Redis")

	cache.Close()
	t.Logf("✅ Connection validation successful")
}

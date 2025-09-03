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

// TestRedisCache_SerializationFormats tests cache creation with different serialization formats
func TestRedisCache_SerializationFormats(t *testing.T) {
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

// TestRedisCache_MultipleCacheInstances tests creating multiple cache instances
func TestRedisCache_MultipleCacheInstances(t *testing.T) {
	ctx := context.Background()

	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	t.Logf("📝 Testing multiple cache instance creation")

	// Create multiple cache instances with different configurations
	caches := make([]interface{ Close() error }, 0)
	defer func() {
		// Cleanup all caches
		for i, cache := range caches {
			if err := cache.Close(); err != nil {
				t.Errorf("Failed to close cache %d: %v", i, err)
			}
		}
	}()

	// Create basic cache
	basicConfig := testintegration.DefaultCacheConfig()
	basicConfig.IndexingMode = false
	basicCache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, basicConfig)
	require.NoError(t, err, "Failed to create basic cache")
	caches = append(caches, basicCache)

	// Create indexed cache
	indexedConfig := testintegration.IndexedCacheConfig()
	indexedCache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, indexedConfig)
	require.NoError(t, err, "Failed to create indexed cache")
	caches = append(caches, indexedCache)

	// Create cache with different serialization
	jsonConfig := testintegration.DefaultCacheConfig()
	jsonConfig.SerializerFormat = "json"
	jsonCache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, jsonConfig)
	require.NoError(t, err, "Failed to create JSON cache")
	caches = append(caches, jsonCache)

	// Test that all caches can perform basic operations
	testSession := &testintegration.TestSession{
		ID:       "session:multi-instance-test",
		UserID:   "user999",
		Username: "multiuser",
		Created:  time.Now(),
	}

	// Test basic cache
	err = basicCache.Set(ctx, testSession, 0)
	assert.NoError(t, err, "Basic cache SET should work")
	exists := basicCache.Has(ctx, testSession.ID)
	assert.True(t, exists, "Basic cache should find the session")

	// Test indexed cache (different session to avoid conflicts)
	indexedTestSession := &testintegration.TestSession{
		ID:       "session:multi-instance-indexed",
		UserID:   "user998",
		Username: "indexeduser",
		Created:  time.Now(),
	}
	err = indexedCache.Set(ctx, indexedTestSession, 0)
	assert.NoError(t, err, "Indexed cache SET should work")
	exists = indexedCache.Has(ctx, indexedTestSession.ID)
	assert.True(t, exists, "Indexed cache should find the session")

	// Test JSON cache (different session to avoid conflicts)
	jsonTestSession := &testintegration.TestSession{
		ID:       "session:multi-instance-json",
		UserID:   "user997",
		Username: "jsonuser",
		Created:  time.Now(),
	}
	err = jsonCache.Set(ctx, jsonTestSession, 0)
	assert.NoError(t, err, "JSON cache SET should work")
	exists = jsonCache.Has(ctx, jsonTestSession.ID)
	assert.True(t, exists, "JSON cache should find the session")

	t.Logf("✅ Multiple cache instances test successful")
	t.Logf("   - Cache instances created: %d", len(caches))
}

// TestRedisCache_ConfigurationEdgeCases tests cache creation with edge case configurations
func TestRedisCache_ConfigurationEdgeCases(t *testing.T) {
	ctx := context.Background()

	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	t.Run("ZeroTTL", func(t *testing.T) {
		config := testintegration.DefaultCacheConfig()
		config.TTL = 0 // Zero TTL (no expiration)

		cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
		require.NoError(t, err, "Cache creation should succeed with zero TTL")
		require.NotNil(t, cache, "Cache should not be nil")

		// Test that operations work with zero TTL
		testSession := &testintegration.TestSession{
			ID:       "session:zero-ttl",
			UserID:   "user800",
			Username: "zerouser",
			Created:  time.Now(),
		}

		err = cache.Set(ctx, testSession, 0)
		assert.NoError(t, err, "SET should work with zero TTL config")
		
		exists := cache.Has(ctx, testSession.ID)
		assert.True(t, exists, "Session should exist with zero TTL")

		cache.Close()
		t.Logf("✅ Zero TTL configuration test successful")
	})

	t.Run("VeryShortTTL", func(t *testing.T) {
		config := testintegration.DefaultCacheConfig()
		config.TTL = time.Millisecond // Very short TTL

		cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
		require.NoError(t, err, "Cache creation should succeed with short TTL")
		require.NotNil(t, cache, "Cache should not be nil")

		cache.Close()
		t.Logf("✅ Very short TTL configuration test successful")
	})

	t.Run("VeryLongTTL", func(t *testing.T) {
		config := testintegration.DefaultCacheConfig()
		config.TTL = 24 * time.Hour // Very long TTL

		cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
		require.NoError(t, err, "Cache creation should succeed with long TTL")
		require.NotNil(t, cache, "Cache should not be nil")

		cache.Close()
		t.Logf("✅ Very long TTL configuration test successful")
	})
}

// TestRedisCache_InitializationSequence tests the proper initialization sequence
func TestRedisCache_InitializationSequence(t *testing.T) {
	ctx := context.Background()

	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	t.Logf("📝 Testing cache initialization sequence")

	// Test 1: Create cache
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Cache creation should succeed")
	require.NotNil(t, cache, "Cache should not be nil")

	// Test 2: Verify cache is immediately usable
	testSession := &testintegration.TestSession{
		ID:       "session:init-sequence",
		UserID:   "user900",
		Username: "inituser",
		Created:  time.Now(),
	}

	// Should be able to use cache immediately after creation
	err = cache.Set(ctx, testSession, 0)
	assert.NoError(t, err, "Cache should be usable immediately after creation")

	exists := cache.Has(ctx, testSession.ID)
	assert.True(t, exists, "Cache operations should work immediately after creation")

	// Test 3: Clean close
	err = cache.Close()
	assert.NoError(t, err, "Cache should close cleanly")

	t.Logf("✅ Cache initialization sequence test successful")
}

// TestRedisCache_WarmLuaScripts tests cache creation with Lua script warming
func TestRedisCache_WarmLuaScripts(t *testing.T) {
	ctx := context.Background()

	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	t.Run("WarmLuaScriptsEnabled", func(t *testing.T) {
		config := testintegration.DefaultCacheConfig()
		config.WarmLuaScripts = true

		cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
		require.NoError(t, err, "Cache creation should succeed with Lua script warming enabled")
		require.NotNil(t, cache, "Cache should not be nil")

		// Verify cache works normally
		testSession := &testintegration.TestSession{
			ID:       "session:warm-lua-enabled",
			UserID:   "user1000",
			Username: "luawarmuser",
			Created:  time.Now(),
		}

		err = cache.Set(ctx, testSession, 0)
		assert.NoError(t, err, "SET should work with Lua warming enabled")
		
		exists := cache.Has(ctx, testSession.ID)
		assert.True(t, exists, "HAS should work with Lua warming enabled")

		cache.Close()
		t.Logf("✅ Warm Lua scripts enabled test successful")
	})

	t.Run("WarmLuaScriptsDisabled", func(t *testing.T) {
		config := testintegration.DefaultCacheConfig()
		config.WarmLuaScripts = false

		cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
		require.NoError(t, err, "Cache creation should succeed with Lua script warming disabled")
		require.NotNil(t, cache, "Cache should not be nil")

		// Verify cache works normally
		testSession := &testintegration.TestSession{
			ID:       "session:warm-lua-disabled",
			UserID:   "user1001",
			Username: "luacolduser",
			Created:  time.Now(),
		}

		err = cache.Set(ctx, testSession, 0)
		assert.NoError(t, err, "SET should work with Lua warming disabled")
		
		exists := cache.Has(ctx, testSession.ID)
		assert.True(t, exists, "HAS should work with Lua warming disabled")

		cache.Close()
		t.Logf("✅ Warm Lua scripts disabled test successful")
	})
}

// TestRedisCache_CleanShutdown tests proper cache shutdown behavior
func TestRedisCache_CleanShutdown(t *testing.T) {
	ctx := context.Background()

	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	t.Logf("📝 Testing cache clean shutdown")

	// Create cache and add some data
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Cache creation should succeed")
	require.NotNil(t, cache, "Cache should not be nil")

	// Add test data
	testSession := &testintegration.TestSession{
		ID:       "session:shutdown-test",
		UserID:   "user1100",
		Username: "shutdownuser",
		Created:  time.Now(),
	}

	err = cache.Set(ctx, testSession, 0)
	require.NoError(t, err, "SET should work before shutdown")

	// Test clean shutdown
	err = cache.Close()
	assert.NoError(t, err, "Cache should close cleanly")

	// Test that close is idempotent
	err = cache.Close()
	assert.NoError(t, err, "Second close should not error (idempotent)")

	t.Logf("✅ Clean shutdown test successful")
}
//go:build integration

package cache_test

import (
	"context"
	"testing"

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

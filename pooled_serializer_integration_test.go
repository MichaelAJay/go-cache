//go:build integration

package cache_test

import (
	"context"
	"testing"
	"time"

	cache "github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/internal/testintegration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestStruct struct {
	ID   int    `msgpack:"id"`
	Name string `msgpack:"name"`
}

func TestSetManySafePooledIntegration(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create custom extractor for TestStruct
	testExtractor := &cache.IndexExtractor[TestStruct]{
		GetEntryKey: func(ts TestStruct) string { return ts.Name },
		GetOwnerKey: nil, // No indexing needed for this test
	}

	// Create cache with msgpack serializer (which has pooled APIs)
	config := testintegration.DefaultCacheConfig()
	config.SerializerFormat = "msgpack"

	cacheInstance, err := cache.NewCache(ctx, setup.RedisClient, false, testExtractor, 0,
		cache.WithTTL[TestStruct](time.Minute),
		cache.WithSerializer[TestStruct](config.SerializerFormat),
	)
	require.NoError(t, err)
	defer cacheInstance.Close()

	// Test data
	testValues := []TestStruct{
		{ID: 1, Name: "alice"},
		{ID: 2, Name: "bob"},
		{ID: 3, Name: "charlie"},
	}

	// Test SetManySafe
	t.Run("SetManySafe", func(t *testing.T) {
		err := cacheInstance.SetManySafe(ctx, testValues, time.Minute)
		assert.NoError(t, err)

		// Verify values were stored correctly
		for _, expected := range testValues {
			value, found, err := cacheInstance.Get(ctx, expected.Name)
			assert.NoError(t, err)
			assert.True(t, found)
			assert.Equal(t, expected, value)
		}
	})

	// Test SetManyPooled
	t.Run("SetManyPooled", func(t *testing.T) {
		// Clear previous data
		keys := []string{"alice", "bob", "charlie"}
		err := cacheInstance.DeleteMany(ctx, keys)
		assert.NoError(t, err)

		// Test pooled version
		err = cacheInstance.SetManyPooled(ctx, testValues, time.Minute)
		assert.NoError(t, err)

		// Verify values were stored correctly
		for _, expected := range testValues {
			value, found, err := cacheInstance.Get(ctx, expected.Name)
			assert.NoError(t, err)
			assert.True(t, found)
			assert.Equal(t, expected, value)
		}
	})

	// Test GetMany still works correctly after SetManyPooled
	t.Run("GetManyAfterSetManyPooled", func(t *testing.T) {
		keys := []string{"alice", "bob", "charlie"}
		results, err := cacheInstance.GetMany(ctx, keys)
		assert.NoError(t, err)
		assert.Len(t, results, 3)

		for _, expected := range testValues {
			actual, found := results[expected.Name]
			assert.True(t, found)
			assert.Equal(t, expected, actual)
		}
	})
}

func TestSerializerFallback(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create custom extractor for TestStruct
	testExtractor := &cache.IndexExtractor[TestStruct]{
		GetEntryKey: func(ts TestStruct) string { return ts.Name },
		GetOwnerKey: nil,
	}

	// Create cache with JSON serializer (which does NOT have pooled APIs)
	cacheInstance, err := cache.NewCache(ctx, setup.RedisClient, false, testExtractor, 0,
		cache.WithTTL[TestStruct](time.Minute),
		cache.WithSerializer[TestStruct]("json"), // JSON doesn't have pooled APIs
	)
	require.NoError(t, err)
	defer cacheInstance.Close()

	testValues := []TestStruct{
		{ID: 1, Name: "fallback_test"},
	}

	// SetManySafe should work with fallback to standard Serialize
	err = cacheInstance.SetManySafe(ctx, testValues, time.Minute)
	assert.NoError(t, err)

	// SetManyPooled should fallback to SetManySafe
	err = cacheInstance.SetManyPooled(ctx, testValues, time.Minute)
	assert.NoError(t, err)

	// Verify it works
	value, found, err := cacheInstance.Get(ctx, "fallback_test")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, testValues[0], value)
}

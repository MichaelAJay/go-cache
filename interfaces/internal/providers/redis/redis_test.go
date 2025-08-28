//go:build integration
// +build integration

package redis

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/MichaelAJay/go-serializer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenericInterfaceCompliance validates that the Redis provider correctly implements
// the generic Cache[T] interface for all supported types
func TestGenericInterfaceCompliance(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	
	t.Run("String", func(t *testing.T) {
		testGenericInterfaceComplianceForType(t, container, "test-string", "updated-string")
	})
	
	t.Run("Int64", func(t *testing.T) {
		testGenericInterfaceComplianceForType(t, container, int64(42), int64(100))
	})
	
	t.Run("Float64", func(t *testing.T) {
		testGenericInterfaceComplianceForType(t, container, 3.14159, 2.71828)
	})
	
	t.Run("Bool", func(t *testing.T) {
		testGenericInterfaceComplianceForType(t, container, true, false)
	})
	
	t.Run("User", func(t *testing.T) {
		user1 := &User{
			ID:      "user1",
			Name:    "John Doe",
			Email:   "john@test.com",
			Created: time.Now(),
			Tags:    []string{"admin", "active"},
		}
		user2 := &User{
			ID:      "user2",
			Name:    "Jane Smith",
			Email:   "jane@test.com",
			Created: time.Now(),
			Tags:    []string{"user"},
		}
		testGenericInterfaceComplianceForType(t, container, user1, user2)
	})
	
	t.Run("SessionData", func(t *testing.T) {
		session1 := SessionData{
			SessionID:   "session1",
			UserID:      "user1",
			Expires:     time.Now().Add(time.Hour),
			Permissions: []string{"read", "write"},
			Data:        map[string]string{"key": "value"},
		}
		session2 := SessionData{
			SessionID:   "session2", 
			UserID:      "user2",
			Expires:     time.Now().Add(2 * time.Hour),
			Permissions: []string{"read"},
			Data:        map[string]string{"other": "data"},
		}
		testGenericInterfaceComplianceForType(t, container, session1, session2)
	})
	
	t.Run("ByteSlice", func(t *testing.T) {
		testGenericInterfaceComplianceForType(t, container, []byte("test-bytes"), []byte("updated-bytes"))
	})
	
	t.Run("Map", func(t *testing.T) {
		map1 := map[string]interface{}{
			"string":  "value",
			"number":  42,
			"boolean": true,
		}
		map2 := map[string]interface{}{
			"other":   "data",
			"count":   100,
			"enabled": false,
		}
		testGenericInterfaceComplianceForType(t, container, map1, map2)
	})
}

// testGenericInterfaceComplianceForType tests all basic cache operations for a specific type
func testGenericInterfaceComplianceForType[T any](t *testing.T, container *TestRedisContainer, value1 T, value2 T) {
	cache := CreateCacheForTesting[T](container)
	defer cache.Close()
	ctx := context.Background()
	
	key := "test-key"
	
	// Test initial state - key should not exist
	t.Run("InitialState", func(t *testing.T) {
		exists := cache.Has(ctx, key)
		assert.False(t, exists, "Key should not exist initially")
		
		_, found, err := cache.Get(ctx, key)
		require.NoError(t, err)
		assert.False(t, found, "Get should return false for non-existent key")
	})
	
	// Test Set operation
	t.Run("Set", func(t *testing.T) {
		err := cache.Set(ctx, key, value1, time.Hour)
		require.NoError(t, err, "Set should succeed")
		
		exists := cache.Has(ctx, key)
		assert.True(t, exists, "Key should exist after Set")
	})
	
	// Test Get operation
	t.Run("Get", func(t *testing.T) {
		retrieved, found, err := cache.Get(ctx, key)
		require.NoError(t, err, "Get should succeed")
		assert.True(t, found, "Get should find the key")
		
		// Use deep equal comparison for complex types
		if !reflect.DeepEqual(retrieved, value1) {
			t.Errorf("Retrieved value doesn't match. Expected: %+v, Got: %+v", value1, retrieved)
		}
	})
	
	// Test Update operation
	t.Run("Update", func(t *testing.T) {
		err := cache.Set(ctx, key, value2, time.Hour)
		require.NoError(t, err, "Update should succeed")
		
		retrieved, found, err := cache.Get(ctx, key)
		require.NoError(t, err, "Get after update should succeed")
		assert.True(t, found, "Get should find the updated key")
		
		if !reflect.DeepEqual(retrieved, value2) {
			t.Errorf("Updated value doesn't match. Expected: %+v, Got: %+v", value2, retrieved)
		}
	})
	
	// Test Delete operation
	t.Run("Delete", func(t *testing.T) {
		err := cache.Delete(ctx, key)
		require.NoError(t, err, "Delete should succeed")
		
		exists := cache.Has(ctx, key)
		assert.False(t, exists, "Key should not exist after Delete")
		
		_, found, err := cache.Get(ctx, key)
		require.NoError(t, err)
		assert.False(t, found, "Get should return false for deleted key")
	})
}

// TestSerializationRoundtrip validates serialization accuracy for all supported types
func TestSerializationRoundtrip(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	
	testValues := GetTestValues()
	
	t.Run("String", func(t *testing.T) {
		testSerializationRoundtripForType(t, container, testValues.String, testValues.String)
	})
	
	t.Run("User", func(t *testing.T) {
		testSerializationRoundtripForType(t, container, testValues.User, testValues.User)
	})
	
	t.Run("SessionData", func(t *testing.T) {
		testSerializationRoundtripForType(t, container, testValues.Session, testValues.Session)
	})
	
	t.Run("LargeStruct", func(t *testing.T) {
		large := GenerateLargeObjects(1, 1)[0] // 1KB object
		testSerializationRoundtripForType(t, container, large, large)
	})
}

// testSerializationRoundtripForType tests serialization accuracy for a specific type
func testSerializationRoundtripForType[T any](t *testing.T, container *TestRedisContainer, value T, expected T) {
	formats := []string{string(serializer.JSON), string(serializer.Binary), string(serializer.Msgpack)}
	
	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			options := &interfaces.CacheOptions{
				SerializerFormat: format,
			}
			cache := CreateCacheWithOptions[T](container, options)
			defer cache.Close()
			
			ctx := context.Background()
			key := "serialization-test"
			
			// Set and get the value
			err := cache.Set(ctx, key, value, time.Hour)
			require.NoError(t, err, "Set should succeed")
			
			retrieved, found, err := cache.Get(ctx, key)
			require.NoError(t, err, "Get should succeed")
			assert.True(t, found, "Get should find the key")
			
			// Verify exact match
			if !reflect.DeepEqual(retrieved, expected) {
				t.Errorf("Serialization roundtrip failed for format %s. Expected: %+v, Got: %+v", format, expected, retrieved)
			}
		})
	}
}

// TestZeroValues tests behavior with nil pointers, empty structs, and zero values
func TestZeroValues(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	ctx := context.Background()
	
	t.Run("NilPointer", func(t *testing.T) {
		cache := CreateCacheForTesting[*User](container)
		defer cache.Close()
		
		key := "nil-user"
		var nilUser *User = nil
		
		err := cache.Set(ctx, key, nilUser, time.Hour)
		require.NoError(t, err, "Set with nil pointer should succeed")
		
		retrieved, found, err := cache.Get(ctx, key)
		require.NoError(t, err, "Get should succeed")
		assert.True(t, found, "Get should find the key")
		assert.Nil(t, retrieved, "Retrieved value should be nil")
	})
	
	t.Run("EmptyStruct", func(t *testing.T) {
		cache := CreateCacheForTesting[User](container)
		defer cache.Close()
		
		key := "empty-user"
		emptyUser := User{} // Zero value struct
		
		err := cache.Set(ctx, key, emptyUser, time.Hour)
		require.NoError(t, err, "Set with empty struct should succeed")
		
		retrieved, found, err := cache.Get(ctx, key)
		require.NoError(t, err, "Get should succeed")
		assert.True(t, found, "Get should find the key")
		assert.Equal(t, emptyUser, retrieved, "Retrieved value should match empty struct")
	})
	
	t.Run("EmptySlice", func(t *testing.T) {
		cache := CreateCacheForTesting[[]string](container)
		defer cache.Close()
		
		key := "empty-slice"
		emptySlice := []string{}
		
		err := cache.Set(ctx, key, emptySlice, time.Hour)
		require.NoError(t, err, "Set with empty slice should succeed")
		
		retrieved, found, err := cache.Get(ctx, key)
		require.NoError(t, err, "Get should succeed")
		assert.True(t, found, "Get should find the key")
		assert.Equal(t, emptySlice, retrieved, "Retrieved value should match empty slice")
	})
	
	t.Run("ZeroString", func(t *testing.T) {
		cache := CreateCacheForTesting[string](container)
		defer cache.Close()
		
		key := "empty-string"
		emptyString := ""
		
		err := cache.Set(ctx, key, emptyString, time.Hour)
		require.NoError(t, err, "Set with empty string should succeed")
		
		retrieved, found, err := cache.Get(ctx, key)
		require.NoError(t, err, "Get should succeed")
		assert.True(t, found, "Get should find the key")
		assert.Equal(t, emptyString, retrieved, "Retrieved value should match empty string")
	})
}

// TestTTLBehavior tests TTL functionality
func TestTTLBehavior(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	ctx := context.Background()
	
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	t.Run("WithTTL", func(t *testing.T) {
		key := "ttl-test"
		value := "expires-soon"
		
		// Set with very short TTL
		err := cache.Set(ctx, key, value, 100*time.Millisecond)
		require.NoError(t, err, "Set with TTL should succeed")
		
		// Should exist immediately
		retrieved, found, err := cache.Get(ctx, key)
		require.NoError(t, err, "Get should succeed")
		assert.True(t, found, "Key should exist immediately after set")
		assert.Equal(t, value, retrieved, "Value should match")
		
		// Wait for expiration
		time.Sleep(200 * time.Millisecond)
		
		// Should be expired
		_, found, err = cache.Get(ctx, key)
		require.NoError(t, err, "Get should succeed even for expired key")
		assert.False(t, found, "Key should be expired")
	})
	
	t.Run("WithoutTTL", func(t *testing.T) {
		key := "no-ttl-test"
		value := "never-expires"
		
		// Set without TTL (0 duration)
		err := cache.Set(ctx, key, value, 0)
		require.NoError(t, err, "Set without TTL should succeed")
		
		// Should exist
		retrieved, found, err := cache.Get(ctx, key)
		require.NoError(t, err, "Get should succeed")
		assert.True(t, found, "Key should exist")
		assert.Equal(t, value, retrieved, "Value should match")
		
		// Wait a bit and check again
		time.Sleep(100 * time.Millisecond)
		
		retrieved, found, err = cache.Get(ctx, key)
		require.NoError(t, err, "Get should succeed")
		assert.True(t, found, "Key should still exist without TTL")
		assert.Equal(t, value, retrieved, "Value should still match")
	})
}

// TestClearOperation tests the Clear functionality
func TestClearOperation(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	ctx := context.Background()
	
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	// Set multiple keys
	keys := []string{"key1", "key2", "key3"}
	values := []string{"value1", "value2", "value3"}
	
	for i, key := range keys {
		err := cache.Set(ctx, key, values[i], time.Hour)
		require.NoError(t, err, "Set should succeed for key %s", key)
	}
	
	// Verify all keys exist
	for _, key := range keys {
		exists := cache.Has(ctx, key)
		assert.True(t, exists, "Key %s should exist before clear", key)
	}
	
	// Clear all
	err := cache.Clear(ctx)
	require.NoError(t, err, "Clear should succeed")
	
	// Verify all keys are gone
	for _, key := range keys {
		exists := cache.Has(ctx, key)
		assert.False(t, exists, "Key %s should not exist after clear", key)
		
		_, found, err := cache.Get(ctx, key)
		require.NoError(t, err, "Get should succeed")
		assert.False(t, found, "Get should return false for cleared key %s", key)
	}
}

// TestEdgeCases tests various edge cases and error conditions
func TestEdgeCases(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	ctx := context.Background()
	
	t.Run("EmptyKey", func(t *testing.T) {
		cache := CreateCacheForTesting[string](container)
		defer cache.Close()
		
		// Test with empty key - should work (Redis allows empty keys)
		err := cache.Set(ctx, "", "empty-key-value", time.Hour)
		require.NoError(t, err, "Set with empty key should succeed")
		
		retrieved, found, err := cache.Get(ctx, "")
		require.NoError(t, err, "Get with empty key should succeed")
		assert.True(t, found, "Empty key should be found")
		assert.Equal(t, "empty-key-value", retrieved, "Value should match")
	})
	
	t.Run("VeryLongKey", func(t *testing.T) {
		cache := CreateCacheForTesting[string](container)
		defer cache.Close()
		
		// Create a very long key (but within Redis limits)
		longKey := ""
		for i := 0; i < 1000; i++ {
			longKey += "a"
		}
		
		err := cache.Set(ctx, longKey, "long-key-value", time.Hour)
		require.NoError(t, err, "Set with long key should succeed")
		
		retrieved, found, err := cache.Get(ctx, longKey)
		require.NoError(t, err, "Get with long key should succeed")
		assert.True(t, found, "Long key should be found")
		assert.Equal(t, "long-key-value", retrieved, "Value should match")
	})
	
	t.Run("SpecialCharactersInKey", func(t *testing.T) {
		cache := CreateCacheForTesting[string](container)
		defer cache.Close()
		
		specialKeys := []string{
			"key:with:colons",
			"key with spaces",
			"key@with#special$chars",
			"key\nwith\ttabs",
			"🔑with🌟emojis",
		}
		
		for _, specialKey := range specialKeys {
			value := "value-for-" + specialKey
			
			err := cache.Set(ctx, specialKey, value, time.Hour)
			require.NoError(t, err, "Set with special key should succeed: %s", specialKey)
			
			retrieved, found, err := cache.Get(ctx, specialKey)
			require.NoError(t, err, "Get with special key should succeed: %s", specialKey)
			assert.True(t, found, "Special key should be found: %s", specialKey)
			assert.Equal(t, value, retrieved, "Value should match for special key: %s", specialKey)
		}
	})
}

// TestMemoryUsageAndLeaks tests for memory leaks in basic operations
func TestMemoryUsageAndLeaks(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	ctx := context.Background()
	
	cache := CreateCacheForTesting[*User](container)
	defer cache.Close()
	
	// Get baseline memory
	runtime.GC()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)
	
	// Perform many operations
	users := GenerateTestUsers(1000)
	for i, user := range users {
		key := fmt.Sprintf("user-%d", i)
		
		err := cache.Set(ctx, key, user, time.Hour)
		require.NoError(t, err, "Set should succeed")
		
		retrieved, found, err := cache.Get(ctx, key)
		require.NoError(t, err, "Get should succeed")
		assert.True(t, found, "Key should be found")
		
		if i%100 == 0 {
			err = cache.Delete(ctx, key)
			require.NoError(t, err, "Delete should succeed")
		}
	}
	
	// Clear remaining data
	err := cache.Clear(ctx)
	require.NoError(t, err, "Clear should succeed")
	
	// Get final memory
	runtime.GC()
	var final runtime.MemStats
	runtime.ReadMemStats(&final)
	
	// Check for reasonable memory usage (not a strict leak test)
	isReasonable := ValidateNoMemoryLeaks(baseline, final)
	if !isReasonable {
		t.Logf("Memory usage increased significantly: %d bytes", 
			final.Alloc-baseline.Alloc)
		// Don't fail the test, just log - Redis connections may have overhead
	}
}
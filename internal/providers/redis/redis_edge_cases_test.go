package redis_test

import (
	"context"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/internal/providers/redis"
	"github.com/MichaelAJay/go-serializer"
)

// TestRedisCache_SetWithEdgeCases tests the Set method with various edge cases
func TestRedisCache_SetWithEdgeCases(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Test with empty key
	err := c.Set(ctx, "", "value", time.Minute)
	if err == nil {
		t.Error("Expected error when setting empty key")
	}

	// Test with zero TTL (should use default TTL)
	err = c.Set(ctx, "zero_ttl_key", "value", 0)
	if err != nil {
		t.Errorf("Set with zero TTL failed: %v", err)
	}

	// Test with negative TTL (should use default TTL)
	err = c.Set(ctx, "negative_ttl_key", "value", -1*time.Second)
	if err != nil {
		t.Errorf("Set with negative TTL failed: %v", err)
	}

	// Test with canceled context
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel() // Cancel immediately
	err = c.Set(canceledCtx, "key", "value", time.Minute)
	if err == nil {
		t.Error("Expected error with canceled context")
	}

	// Test with a value that can't be serialized
	type unserializableValue struct {
		Ch chan int // channels can't be serialized
	}
	err = c.Set(ctx, "unserializable_key", unserializableValue{Ch: make(chan int)}, time.Minute)
	if err == nil {
		t.Error("Expected serialization error")
	}
}

// TestRedisCache_GetManyWithEdgeCases tests the GetMany method with various edge cases
func TestRedisCache_GetManyWithEdgeCases(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Setup test data
	items := map[string]any{
		"gm_key1": "value1",
		"gm_key2": "value2",
		"gm_key3": 123,
		"gm_key4": true,
	}
	err := c.SetMany(ctx, items, time.Hour)
	if err != nil {
		t.Fatalf("SetMany failed: %v", err)
	}

	// Test with empty key list
	values, err := c.GetMany(ctx, []string{})
	if err != nil {
		t.Errorf("GetMany with empty keys failed: %v", err)
	}
	if len(values) != 0 {
		t.Errorf("Expected empty result, got %d items", len(values))
	}

	// Test with nil keys
	values, err = c.GetMany(ctx, nil)
	if err != nil {
		t.Errorf("GetMany with nil keys failed: %v", err)
	}
	if len(values) != 0 {
		t.Errorf("Expected empty result, got %d items", len(values))
	}

	// Test with some existing and some non-existing keys
	values, err = c.GetMany(ctx, []string{"gm_key1", "gm_key2", "gm_nonexistent"})
	if err != nil {
		t.Errorf("GetMany failed: %v", err)
	}
	if len(values) != 2 {
		t.Errorf("Expected 2 values, got %d", len(values))
	}
	if values["gm_key1"] != "value1" || values["gm_key2"] != "value2" {
		t.Errorf("Expected correct values, got: %v", values)
	}

	// Test with canceled context
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel() // Cancel immediately
	_, err = c.GetMany(canceledCtx, []string{"gm_key1"})
	if err == nil {
		t.Error("Expected error with canceled context")
	}
}

// TestRedisCache_SetManyWithEdgeCases tests the SetMany method with various edge cases
func TestRedisCache_SetManyWithEdgeCases(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Test with empty map
	err := c.SetMany(ctx, map[string]any{}, time.Hour)
	if err != nil {
		t.Errorf("SetMany with empty map failed: %v", err)
	}

	// Test with nil map
	err = c.SetMany(ctx, nil, time.Hour)
	if err != nil {
		t.Errorf("SetMany with nil map failed: %v", err)
	}

	// Test with zero TTL (should use default TTL)
	err = c.SetMany(ctx, map[string]any{"sm_key1": "value1"}, 0)
	if err != nil {
		t.Errorf("SetMany with zero TTL failed: %v", err)
	}

	// Test with negative TTL (should use default TTL)
	err = c.SetMany(ctx, map[string]any{"sm_key2": "value2"}, -1*time.Second)
	if err != nil {
		t.Errorf("SetMany with negative TTL failed: %v", err)
	}

	// Test with various value types
	mixedItems := map[string]any{
		"sm_string": "string_value",
		"sm_int":    123,
		"sm_bool":   true,
		"sm_float":  123.456,
		"sm_slice":  []int{1, 2, 3},
		"sm_map":    map[string]string{"a": "b"},
	}
	err = c.SetMany(ctx, mixedItems, time.Hour)
	if err != nil {
		t.Errorf("SetMany with mixed types failed: %v", err)
	}

	// Verify the values
	values, err := c.GetMany(ctx, []string{"sm_string", "sm_int", "sm_bool", "sm_float", "sm_slice", "sm_map"})
	if err != nil {
		t.Errorf("Failed to retrieve values: %v", err)
	}
	if len(values) != 6 {
		t.Errorf("Expected 6 values, got %d", len(values))
	}

	// Test with canceled context
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel() // Cancel immediately
	err = c.SetMany(canceledCtx, map[string]any{"key": "value"}, time.Minute)
	if err == nil {
		t.Error("Expected error with canceled context")
	}

	// Test with a value that can't be serialized
	type unserializableValue struct {
		Ch chan int // channels can't be serialized
	}
	err = c.SetMany(ctx, map[string]any{"unserializable_key": unserializableValue{Ch: make(chan int)}}, time.Minute)
	if err == nil {
		t.Error("Expected serialization error")
	}
}

// TestRedisCache_GetMetadataWithEdgeCases tests the GetMetadata method with various edge cases
func TestRedisCache_GetMetadataWithEdgeCases(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Test with empty key
	_, err := c.GetMetadata(ctx, "")
	if err == nil {
		t.Error("Expected error with empty key")
	}

	// Test with non-existing key
	_, err = c.GetMetadata(ctx, "gmd_nonexistent")
	if err == nil {
		t.Error("Expected error with non-existing key")
	}

	// Set a value and check metadata existence
	err = c.Set(ctx, "gmd_key1", "value1", time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	metadata, err := c.GetMetadata(ctx, "gmd_key1")
	if err != nil {
		t.Errorf("GetMetadata failed: %v", err)
	}
	if metadata.Key != "gmd_key1" {
		t.Errorf("Expected key 'gmd_key1', got '%s'", metadata.Key)
	}
	if metadata.CreatedAt.IsZero() {
		t.Error("Expected non-zero creation time")
	}

	// Set a value with specific TTL and verify
	err = c.Set(ctx, "gmd_ttl_key", "value", 10*time.Second)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get metadata and verify TTL
	metadata, err = c.GetMetadata(ctx, "gmd_ttl_key")
	if err != nil {
		t.Errorf("GetMetadata failed: %v", err)
	}
	if metadata.TTL != 10*time.Second {
		t.Errorf("Expected TTL of 10s, got %v", metadata.TTL)
	}

	// Test with canceled context
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel() // Cancel immediately
	_, err = c.GetMetadata(canceledCtx, "gmd_key1")
	if err == nil {
		t.Error("Expected error with canceled context")
	}

	// Test with a key that exists but has no metadata
	err = c.Set(ctx, "key_without_metadata", "value", time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Delete just the metadata
	client := setupRedisClient(t)
	metaKey := "cache:meta:key_without_metadata"
	err = client.Del(ctx, metaKey).Err()
	if err != nil {
		t.Fatalf("Failed to delete metadata: %v", err)
	}
	client.Close()

	// Try to get metadata - should either return error or constructed metadata
	metadata, err = c.GetMetadata(ctx, "key_without_metadata")
	if err != nil {
		// If error, that's one valid outcome
		t.Logf("GetMetadata returned error as expected: %v", err)
	} else {
		// If no error, should still have the key name set
		if metadata.Key != "key_without_metadata" {
			t.Errorf("Expected key name in metadata, got: %v", metadata)
		}
	}
}

// TestRedisCache_GetManyMetadataWithEdgeCases tests the GetManyMetadata method with edge cases
func TestRedisCache_GetManyMetadataWithEdgeCases(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Set up test data
	err := c.Set(ctx, "gmmd_key1", "value1", time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	err = c.Set(ctx, "gmmd_key2", "value2", time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Test with empty keys
	metadataMap, err := c.GetManyMetadata(ctx, []string{})
	if err != nil {
		t.Errorf("GetManyMetadata with empty keys failed: %v", err)
	}
	if len(metadataMap) != 0 {
		t.Errorf("Expected empty map, got %d entries", len(metadataMap))
	}

	// Test with nil keys
	metadataMap, err = c.GetManyMetadata(ctx, nil)
	if err != nil {
		t.Errorf("GetManyMetadata with nil keys failed: %v", err)
	}
	if len(metadataMap) != 0 {
		t.Errorf("Expected empty map, got %d entries", len(metadataMap))
	}

	// Test with mix of existing and non-existing keys
	metadataMap, err = c.GetManyMetadata(ctx, []string{"gmmd_key1", "gmmd_nonexistent", "gmmd_key2"})
	if err != nil {
		t.Errorf("GetManyMetadata failed: %v", err)
	}
	if len(metadataMap) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(metadataMap))
	}
	if _, ok := metadataMap["gmmd_key1"]; !ok {
		t.Error("Expected metadata for 'gmmd_key1'")
	}
	if _, ok := metadataMap["gmmd_key2"]; !ok {
		t.Error("Expected metadata for 'gmmd_key2'")
	}

	// Test with canceled context
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel() // Cancel immediately
	_, err = c.GetManyMetadata(canceledCtx, []string{"gmmd_key1"})
	if err == nil {
		t.Error("Expected error with canceled context")
	}
}

// TestRedisCache_DeleteWithEdgeCases tests the Delete method with various edge cases
func TestRedisCache_DeleteWithEdgeCases(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Test with empty key
	err := c.Delete(ctx, "")
	if err == nil {
		t.Error("Expected error when deleting empty key")
	}

	// Test with canceled context
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel() // Cancel immediately
	err = c.Delete(canceledCtx, "some_key")
	if err == nil {
		t.Error("Expected error with canceled context")
	}

	// Test Delete on non-existent key (should not error)
	err = c.Delete(ctx, "nonexistent_delete_key")
	if err != nil {
		t.Errorf("Delete on non-existent key should not error: %v", err)
	}

	// Set and delete valid key
	err = c.Set(ctx, "delete_test_key", "value", time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify it exists
	_, exists, err := c.Get(ctx, "delete_test_key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !exists {
		t.Fatal("Key should exist before deletion")
	}

	// Delete it
	err = c.Delete(ctx, "delete_test_key")
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	// Verify it's gone
	_, exists, err = c.Get(ctx, "delete_test_key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if exists {
		t.Fatal("Key should not exist after deletion")
	}
}

// TestRedisCache_DeleteManyWithEdgeCases tests the DeleteMany method with various edge cases
func TestRedisCache_DeleteManyWithEdgeCases(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Set up test data
	items := map[string]any{
		"dm_key1": "value1",
		"dm_key2": "value2",
		"dm_key3": "value3",
	}
	err := c.SetMany(ctx, items, time.Hour)
	if err != nil {
		t.Fatalf("SetMany failed: %v", err)
	}

	// Test with empty keys
	err = c.DeleteMany(ctx, []string{})
	if err != nil {
		t.Errorf("DeleteMany with empty keys failed: %v", err)
	}

	// Test with nil keys
	err = c.DeleteMany(ctx, nil)
	if err != nil {
		t.Errorf("DeleteMany with nil keys failed: %v", err)
	}

	// Test with mix of existing and non-existing keys
	err = c.DeleteMany(ctx, []string{"dm_key1", "dm_nonexistent", "dm_key2"})
	if err != nil {
		t.Errorf("DeleteMany failed: %v", err)
	}

	// Verify correct keys were deleted
	values, err := c.GetMany(ctx, []string{"dm_key1", "dm_key2", "dm_key3"})
	if err != nil {
		t.Errorf("GetMany failed: %v", err)
	}
	if len(values) != 1 {
		t.Errorf("Expected 1 value, got %d", len(values))
	}
	if _, ok := values["dm_key3"]; !ok {
		t.Error("Expected dm_key3 to still exist")
	}

	// Test with canceled context
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel() // Cancel immediately
	err = c.DeleteMany(canceledCtx, []string{"dm_key3"})
	if err == nil {
		t.Error("Expected error with canceled context")
	}
}

// TestRedisCache_ClearWithEdgeCases tests the Clear method with various edge cases
func TestRedisCache_ClearWithEdgeCases(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Set up test data
	items := map[string]any{
		"clear_key1": "value1",
		"clear_key2": "value2",
	}
	err := c.SetMany(ctx, items, time.Hour)
	if err != nil {
		t.Fatalf("SetMany failed: %v", err)
	}

	// Test with canceled context
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel() // Cancel immediately
	err = c.Clear(canceledCtx)
	if err == nil {
		t.Error("Expected error with canceled context")
	}

	// Clear the cache
	err = c.Clear(ctx)
	if err != nil {
		t.Errorf("Clear failed: %v", err)
	}

	// Verify all keys are gone
	values, err := c.GetMany(ctx, []string{"clear_key1", "clear_key2"})
	if err != nil {
		t.Errorf("GetMany failed: %v", err)
	}
	if len(values) != 0 {
		t.Errorf("Expected empty result after clear, got %d items", len(values))
	}
}

// TestRedisCache_NewWithNilClient tests creating a Redis cache with a nil client
func TestRedisCache_NewWithNilClient(t *testing.T) {
	_, err := redis.NewRedisCache(nil, &cache.CacheOptions{
		TTL: time.Minute,
	})
	if err == nil {
		t.Error("Expected error when creating cache with nil client")
	}
}

// TestRedisCache_NewWithInvalidSerializer tests creating a cache with an invalid serializer format
func TestRedisCache_NewWithInvalidSerializer(t *testing.T) {
	client := setupRedisClient(t)
	defer client.Close()

	// Create cache with invalid serializer format
	_, err := redis.NewRedisCache(client, &cache.CacheOptions{
		SerializerFormat: "invalid_format",
	})
	if err == nil {
		t.Error("Expected error with invalid serializer format")
	}
}

// TestRedisCache_OptionsWithVariousCombinations tests creating a Redis cache with various option combinations
func TestRedisCache_OptionsWithVariousCombinations(t *testing.T) {
	client := setupRedisClient(t)
	defer client.Close()

	testCases := []struct {
		name          string
		options       *cache.CacheOptions
		expectSuccess bool
	}{
		{
			name: "Default Options",
			options: &cache.CacheOptions{
				TTL: time.Minute,
			},
			expectSuccess: true,
		},
		{
			name: "JSON Serializer",
			options: &cache.CacheOptions{
				TTL:              time.Minute,
				SerializerFormat: serializer.JSON,
			},
			expectSuccess: true,
		},
		{
			name: "MsgPack Serializer",
			options: &cache.CacheOptions{
				TTL:              time.Minute,
				SerializerFormat: serializer.Msgpack,
			},
			expectSuccess: true,
		},
		{
			name:          "Nil Options",
			options:       nil,
			expectSuccess: true, // Should use default values
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := redis.NewRedisCache(client, tc.options)
			if tc.expectSuccess {
				if err != nil {
					t.Errorf("Expected success, got error: %v", err)
				}
				if c == nil {
					t.Error("Expected non-nil cache instance")
				}
			} else {
				if err == nil {
					t.Error("Expected error, got success")
				}
			}
		})
	}
}

// TestRedisCache_HandlingCorruptedMetadata tests handling of corrupted metadata
func TestRedisCache_HandlingCorruptedMetadata(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Set a value
	err := c.Set(ctx, "corrupted_meta_key", "value", time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Corrupt the metadata by setting an invalid JSON value
	client := setupRedisClient(t)
	metaKey := "cache:meta:corrupted_meta_key"
	err = client.Set(ctx, metaKey, "invalid json", time.Hour).Err()
	if err != nil {
		t.Fatalf("Failed to corrupt metadata: %v", err)
	}
	client.Close()

	// Now try to get the metadata
	metadata, err := c.GetMetadata(ctx, "corrupted_meta_key")
	// Either we'll get an error or we'll get empty/default metadata
	if err == nil {
		// If no error, at least the metadata should have the key set
		if metadata.Key != "corrupted_meta_key" {
			t.Errorf("Expected key to be set in metadata: %v", metadata)
		}
	}

	// The main value should still be accessible
	value, exists, err := c.Get(ctx, "corrupted_meta_key")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if !exists {
		t.Error("Expected value to exist despite corrupted metadata")
	}
	if value != "value" {
		t.Errorf("Expected 'value', got '%v'", value)
	}
}

package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/providers/memory"
	"github.com/MichaelAJay/go-logger"
	"github.com/MichaelAJay/go-serializer"
)

// TestMemoryCache_Get_Comprehensive tests edge cases for the Get method
func TestMemoryCache_Get_Comprehensive(t *testing.T) {
	// Test 1: Deserialization fallbacks - test our improved deserialization handling
	t.Run("DeserializationFallbacks", func(t *testing.T) {
		c, err := memory.NewMemoryCache(&cache.CacheOptions{
			SerializerFormat: serializer.Binary, // Binary format is prone to deserialization issues
		})
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		ctx := context.Background()

		// Test with different types to verify fallback deserialization
		testCases := []struct {
			key   string
			value any
		}{
			{"string-key", "string-value"},
			{"int-key", 42},
			{"bool-key", true},
			{"float-key", 3.14159},
		}

		// Set all values
		for _, tc := range testCases {
			err = c.Set(ctx, tc.key, tc.value, time.Hour)
			if err != nil {
				t.Fatalf("Failed to set key %s: %v", tc.key, err)
			}
		}

		// Get and verify values
		for _, tc := range testCases {
			value, exists, err := c.Get(ctx, tc.key)
			if err != nil {
				t.Errorf("Failed to get key %s: %v", tc.key, err)
				continue
			}
			if !exists {
				t.Errorf("Key %s should exist", tc.key)
				continue
			}

			// Value should be retrievable regardless of serializer's quirks
			switch tc.key {
			case "string-key":
				str, ok := value.(string)
				if !ok {
					t.Errorf("Expected string value, got %T", value)
				} else if str != "string-value" {
					t.Errorf("Expected 'string-value', got '%s'", str)
				}
			case "bool-key":
				b, ok := value.(bool)
				if !ok {
					t.Errorf("Expected bool value, got %T", value)
				} else if !b {
					t.Errorf("Expected true, got %v", b)
				}
			}
		}
	})
}

// TestMemoryCache_GetMany_Comprehensive tests edge cases for the GetMany method
func TestMemoryCache_GetMany_Comprehensive(t *testing.T) {
	t.Run("MixedKeysAndExpirations", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		ctx := context.Background()

		// Create some regular keys
		err = c.Set(ctx, "regular1", "value1", time.Hour)
		if err != nil {
			t.Fatalf("Failed to set key: %v", err)
		}
		err = c.Set(ctx, "regular2", "value2", time.Hour)
		if err != nil {
			t.Fatalf("Failed to set key: %v", err)
		}

		// Create an expired key
		err = c.Set(ctx, "expired", "expired-value", 10*time.Millisecond)
		if err != nil {
			t.Fatalf("Failed to set key: %v", err)
		}

		// Wait for the key to expire
		time.Sleep(20 * time.Millisecond)

		// Get mixed keys including non-existent and expired
		result, err := c.GetMany(ctx, []string{
			"regular1",     // exists
			"regular2",     // exists
			"expired",      // expired
			"non-existent", // never existed
		})
		if err != nil {
			t.Fatalf("GetMany failed: %v", err)
		}

		// Should only have two results
		if len(result) != 2 {
			t.Errorf("Expected 2 results, got %d", len(result))
		}

		// Check specific keys
		if val, exists := result["regular1"]; !exists {
			t.Error("regular1 should exist in results")
		} else if val != "value1" {
			t.Errorf("Expected 'value1', got '%v'", val)
		}

		if val, exists := result["regular2"]; !exists {
			t.Error("regular2 should exist in results")
		} else if val != "value2" {
			t.Errorf("Expected 'value2', got '%v'", val)
		}

		// Expired and non-existent keys should not be in results
		if _, exists := result["expired"]; exists {
			t.Error("expired key should not be in results")
		}
		if _, exists := result["non-existent"]; exists {
			t.Error("non-existent key should not be in results")
		}
	})

	t.Run("ConcurrentUpdate", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		ctx := context.Background()

		// Create a key
		err = c.Set(ctx, "concurrent-key", "original-value", time.Hour)
		if err != nil {
			t.Fatalf("Failed to set key: %v", err)
		}

		// Get the key asynchronously and then update it in a goroutine
		// to test the concurrent access pattern in GetMany
		resultCh := make(chan map[string]any, 1)
		go func() {
			// Small delay to ensure the main thread updates the value
			time.Sleep(10 * time.Millisecond)
			result, err := c.GetMany(ctx, []string{"concurrent-key"})
			if err != nil {
				t.Errorf("GetMany failed: %v", err)
				resultCh <- nil
				return
			}
			resultCh <- result
		}()

		// Update the key while GetMany is potentially accessing it
		err = c.Set(ctx, "concurrent-key", "updated-value", time.Hour)
		if err != nil {
			t.Fatalf("Failed to update key: %v", err)
		}

		// Get the result
		result := <-resultCh
		if result == nil {
			t.Fatal("GetMany failed")
		}

		// The value could be either the original or updated one depending on timing
		// We just verify that we got a value without errors
		if val, exists := result["concurrent-key"]; !exists {
			t.Error("concurrent-key should exist in results")
		} else if val != "original-value" && val != "updated-value" {
			t.Errorf("Expected either 'original-value' or 'updated-value', got '%v'", val)
		}
	})
}

// TestMemoryCache_GetManyMetadata_Comprehensive tests edge cases for GetManyMetadata
func TestMemoryCache_GetManyMetadata_Comprehensive(t *testing.T) {
	t.Run("MixedKeysAndExpirations", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		ctx := context.Background()

		// Create some regular keys
		err = c.Set(ctx, "meta1", "value1", time.Hour)
		if err != nil {
			t.Fatalf("Failed to set key: %v", err)
		}
		err = c.Set(ctx, "meta2", "value2", time.Hour)
		if err != nil {
			t.Fatalf("Failed to set key: %v", err)
		}

		// Create an expired key
		err = c.Set(ctx, "meta-expired", "expired-value", 10*time.Millisecond)
		if err != nil {
			t.Fatalf("Failed to set key: %v", err)
		}

		// Wait for the key to expire
		time.Sleep(20 * time.Millisecond)

		// Get metadata for all keys
		metadata, err := c.GetManyMetadata(ctx, []string{
			"meta1",        // exists
			"meta2",        // exists
			"meta-expired", // expired
			"non-existent", // never existed
		})
		if err != nil {
			t.Fatalf("GetManyMetadata failed: %v", err)
		}

		// Should only have two results
		if len(metadata) != 2 {
			t.Errorf("Expected 2 metadata entries, got %d", len(metadata))
		}

		// Check specific keys
		if md, exists := metadata["meta1"]; !exists {
			t.Error("meta1 should exist in metadata")
		} else if md.Key != "meta1" {
			t.Errorf("Expected key 'meta1', got '%s'", md.Key)
		}

		if md, exists := metadata["meta2"]; !exists {
			t.Error("meta2 should exist in metadata")
		} else if md.Key != "meta2" {
			t.Errorf("Expected key 'meta2', got '%s'", md.Key)
		}

		// Expired and non-existent keys should not be in results
		if _, exists := metadata["meta-expired"]; exists {
			t.Error("meta-expired key should not be in metadata")
		}
		if _, exists := metadata["non-existent"]; exists {
			t.Error("non-existent key should not be in metadata")
		}
	})

	t.Run("AccessCountAndLastAccessed", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		ctx := context.Background()

		// Create a key
		err = c.Set(ctx, "access-key", "value", time.Hour)
		if err != nil {
			t.Fatalf("Failed to set key: %v", err)
		}

		// Initial metadata should have access count 0
		initialMd, err := c.GetManyMetadata(ctx, []string{"access-key"})
		if err != nil {
			t.Fatalf("GetManyMetadata failed: %v", err)
		}
		if md, exists := initialMd["access-key"]; !exists {
			t.Error("access-key should exist in metadata")
		} else if md.AccessCount != 0 {
			t.Errorf("Expected access count 0, got %d", md.AccessCount)
		}

		// Access the key
		_, _, err = c.Get(ctx, "access-key")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		// Metadata should now reflect the access
		updatedMd, err := c.GetManyMetadata(ctx, []string{"access-key"})
		if err != nil {
			t.Fatalf("GetManyMetadata failed: %v", err)
		}
		if md, exists := updatedMd["access-key"]; !exists {
			t.Error("access-key should exist in metadata")
		} else if md.AccessCount != 1 {
			t.Errorf("Expected access count 1, got %d", md.AccessCount)
		}
	})
}

// TestMemoryCache_NewMemoryCache_Comprehensive tests all options and edge cases for NewMemoryCache
func TestMemoryCache_NewMemoryCache_Comprehensive(t *testing.T) {
	t.Run("AllCustomOptions", func(t *testing.T) {
		// Create a logger with default config
		log := logger.New(logger.DefaultConfig)

		// Create with all custom options
		options := &cache.CacheOptions{
			TTL:              time.Hour,
			MaxEntries:       1000,
			MaxSize:          1024 * 1024 * 10, // 10MB
			CleanupInterval:  time.Minute,
			Logger:           log,
			SerializerFormat: serializer.JSON,
		}

		c, err := memory.NewMemoryCache(options)
		if err != nil {
			t.Fatalf("Failed to create memory cache with custom options: %v", err)
		}
		defer c.Close()

		// Test that options were applied by using the cache
		ctx := context.Background()
		err = c.Set(ctx, "test-key", "test-value", 0)
		if err != nil {
			t.Fatalf("Failed to set key: %v", err)
		}

		// Get the value
		val, exists, err := c.Get(ctx, "test-key")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if !exists {
			t.Fatal("Key should exist")
		}
		if val != "test-value" {
			t.Errorf("Expected 'test-value', got '%v'", val)
		}
	})

	t.Run("InvalidSerializerFormat", func(t *testing.T) {
		// Create with invalid serializer format
		options := &cache.CacheOptions{
			SerializerFormat: "invalid-format",
		}

		_, err := memory.NewMemoryCache(options)
		if err == nil {
			t.Fatal("Expected error for invalid serializer format, got nil")
		}
	})
}

// TestMemoryCache_DeleteMany_Comprehensive tests edge cases for DeleteMany
func TestMemoryCache_DeleteMany_Comprehensive(t *testing.T) {
	t.Run("EmptyKeysList", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		ctx := context.Background()

		// Empty keys list should return nil
		err = c.DeleteMany(ctx, []string{})
		if err != nil {
			t.Errorf("Expected nil error for empty keys list, got: %v", err)
		}
	})

	t.Run("CanceledContext", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		// Create canceled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Should return error for canceled context
		err = c.DeleteMany(ctx, []string{"any-key"})
		if err != cache.ErrContextCanceled {
			t.Errorf("Expected ErrContextCanceled for canceled context, got: %v", err)
		}
	})

	t.Run("MixedExistingAndNonExistingKeys", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		ctx := context.Background()

		// Create some keys
		err = c.Set(ctx, "delete1", "value1", time.Hour)
		if err != nil {
			t.Fatalf("Failed to set key: %v", err)
		}
		err = c.Set(ctx, "delete2", "value2", time.Hour)
		if err != nil {
			t.Fatalf("Failed to set key: %v", err)
		}

		// Delete a mix of existing and non-existing keys
		err = c.DeleteMany(ctx, []string{
			"delete1",      // exists
			"delete2",      // exists
			"non-existent", // never existed
		})
		if err != nil {
			t.Errorf("DeleteMany failed: %v", err)
		}

		// None of the keys should exist after DeleteMany
		for _, key := range []string{"delete1", "delete2", "non-existent"} {
			if c.Has(ctx, key) {
				t.Errorf("Key %s should not exist after DeleteMany", key)
			}
		}
	})

	t.Run("UpdateSizeMetrics", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		ctx := context.Background()

		// Add several keys with different sizes
		keys := []string{"key1", "key2", "key3", "key4", "key5"}
		for i, key := range keys {
			// Make values of different sizes
			value := make([]byte, 100*(i+1))
			err = c.Set(ctx, key, value, time.Hour)
			if err != nil {
				t.Fatalf("Failed to set key %s: %v", key, err)
			}
		}

		// Get initial metrics
		initialMetrics := c.GetMetrics()
		initialSize := initialMetrics.CacheSize
		initialCount := initialMetrics.EntryCount

		if initialCount != int64(len(keys)) {
			t.Errorf("Expected initial entry count %d, got %d", len(keys), initialCount)
		}

		// Delete a subset of keys
		err = c.DeleteMany(ctx, keys[:3]) // Delete first 3 keys
		if err != nil {
			t.Errorf("DeleteMany failed: %v", err)
		}

		// Check updated metrics
		updatedMetrics := c.GetMetrics()
		updatedSize := updatedMetrics.CacheSize
		updatedCount := updatedMetrics.EntryCount

		if updatedCount != initialCount-3 {
			t.Errorf("Expected entry count to decrease by 3, got change from %d to %d",
				initialCount, updatedCount)
		}

		if updatedSize >= initialSize {
			t.Errorf("Expected cache size to decrease, got change from %d to %d",
				initialSize, updatedSize)
		}
	})
}

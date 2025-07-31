package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/internal/providers/memory"
)

func TestMemoryCache_SetMany_Comprehensive(t *testing.T) {
	ctx := context.Background()

	// Test 1: Empty map of items
	t.Run("EmptyMap", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		// Empty map should return nil (success)
		err = c.SetMany(ctx, map[string]any{}, time.Hour)
		if err != nil {
			t.Errorf("Expected nil error for empty map, got: %v", err)
		}
	})

	// Test 2: Canceled context
	t.Run("CanceledContext", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		// Canceled context should return error
		err = c.SetMany(canceledCtx, map[string]any{"key": "value"}, time.Hour)
		if err != cache.ErrContextCanceled {
			t.Errorf("Expected ErrContextCanceled for canceled context, got: %v", err)
		}
	})

	// Test 3: Empty key in the map
	t.Run("EmptyKey", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		// Empty key should return error
		err = c.SetMany(ctx, map[string]any{"": "value"}, time.Hour)
		if err != cache.ErrInvalidKey {
			t.Errorf("Expected ErrInvalidKey for empty key, got: %v", err)
		}
	})

	// Test 4: Serialization error
	t.Run("SerializationError", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		// Create a value that can't be serialized
		invalidValue := make(chan int) // channels can't be serialized

		// Should return serialization error
		err = c.SetMany(ctx, map[string]any{"key": invalidValue}, time.Hour)
		if err != cache.ErrSerialization {
			t.Errorf("Expected ErrSerialization for unserializable value, got: %v", err)
		}
	})

	// Test 5: Max entries limit
	t.Run("MaxEntriesLimit", func(t *testing.T) {
		c, err := memory.NewMemoryCache(&cache.CacheOptions{
			MaxEntries: 2,
		})
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		// First add one key
		err = c.Set(ctx, "existing", "value", time.Hour)
		if err != nil {
			t.Fatalf("Failed to set key: %v", err)
		}

		// Try to add two more keys (would exceed MaxEntries=2)
		err = c.SetMany(ctx, map[string]any{
			"new1": "value1",
			"new2": "value2",
		}, time.Hour)
		if err != cache.ErrCacheFull {
			t.Errorf("Expected ErrCacheFull when exceeding max entries, got: %v", err)
		}

		// Add just one key (should work)
		err = c.SetMany(ctx, map[string]any{"new1": "value1"}, time.Hour)
		if err != nil {
			t.Errorf("Expected success when adding one key, got: %v", err)
		}
	})

	// Test 6: Max size limit
	t.Run("MaxSizeLimit", func(t *testing.T) {
		c, err := memory.NewMemoryCache(&cache.CacheOptions{
			MaxSize: 100, // Small max size
		})
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		// Try to add large values
		largeValue := make([]byte, 150) // Exceeds MaxSize
		err = c.SetMany(ctx, map[string]any{"large": largeValue}, time.Hour)
		if err != cache.ErrCacheFull {
			t.Errorf("Expected ErrCacheFull when exceeding max size, got: %v", err)
		}

		// Add small value (should work)
		err = c.SetMany(ctx, map[string]any{"small": "value"}, time.Hour)
		if err != nil {
			t.Errorf("Expected success when adding small value, got: %v", err)
		}
	})

	// Test 7: Default TTL handling
	t.Run("DefaultTTL", func(t *testing.T) {
		// Create cache with custom default TTL
		customTTL := time.Minute * 5
		c, err := memory.NewMemoryCache(&cache.CacheOptions{
			TTL: customTTL,
		})
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		// Add with zero TTL (should use default)
		err = c.SetMany(ctx, map[string]any{"default-ttl": "value"}, 0)
		if err != nil {
			t.Errorf("Failed to set with default TTL: %v", err)
		}

		// Verify key exists
		metadata, err := c.GetMetadata(ctx, "default-ttl")
		if err != nil {
			t.Fatalf("Failed to get metadata: %v", err)
		}

		// TTL should be close to our default (allow some execution time variation)
		if metadata.TTL < customTTL-time.Second || metadata.TTL > customTTL {
			t.Errorf("Expected TTL around %v, got %v", customTTL, metadata.TTL)
		}
	})

	// Test 8: Check that all values were actually stored
	t.Run("VerifyAllStored", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		// Multiple values of different types
		itemsToStore := map[string]any{
			"string-key": "string-value",
			"int-key":    42,
			"bool-key":   true,
			"float-key":  3.14,
			"slice-key":  []string{"a", "b", "c"},
		}

		// Store all values
		err = c.SetMany(ctx, itemsToStore, time.Hour)
		if err != nil {
			t.Fatalf("Failed to store multiple values: %v", err)
		}

		// Check all values were stored correctly
		for key, expectedValue := range itemsToStore {
			value, exists, err := c.Get(ctx, key)
			if err != nil {
				t.Errorf("Failed to get key %s: %v", key, err)
			}
			if !exists {
				t.Errorf("Expected key %s to exist", key)
			}

			// For simple types we can do direct comparison
			// For complex types we just check they exist since serialization might change representation
			switch key {
			case "string-key", "bool-key":
				if value != expectedValue {
					t.Errorf("For key %s, expected %v, got %v", key, expectedValue, value)
				}
			}
		}
	})
}

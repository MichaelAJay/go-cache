package memory_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/providers/memory"
)

func TestMemoryCache_GetKeys_Comprehensive(t *testing.T) {
	// Test 1: Empty cache
	t.Run("EmptyCache", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		ctx := context.Background()
		keys := c.GetKeys(ctx)

		if len(keys) != 0 {
			t.Errorf("Expected empty keys list for empty cache, got %d keys", len(keys))
		}
	})

	// Test 2: Canceled context
	t.Run("CanceledContext", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		// Add a key
		ctx := context.Background()
		err = c.Set(ctx, "test-key", "test-value", time.Hour)
		if err != nil {
			t.Fatalf("Failed to set key: %v", err)
		}

		// Create canceled context
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		// Should return empty list with canceled context
		keys := c.GetKeys(canceledCtx)
		if len(keys) != 0 {
			t.Errorf("Expected empty keys list for canceled context, got %d keys", len(keys))
		}
	})

	// Test 3: Multiple keys
	t.Run("MultipleKeys", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		ctx := context.Background()

		// Add multiple keys
		keysToAdd := []string{"key1", "key2", "key3", "key4", "key5"}
		for i, key := range keysToAdd {
			err = c.Set(ctx, key, i, time.Hour)
			if err != nil {
				t.Fatalf("Failed to set key %s: %v", key, err)
			}
		}

		// Get all keys
		retrievedKeys := c.GetKeys(ctx)

		// Check count
		if len(retrievedKeys) != len(keysToAdd) {
			t.Errorf("Expected %d keys, got %d keys", len(keysToAdd), len(retrievedKeys))
		}

		// Check all expected keys are present
		keyMap := make(map[string]bool)
		for _, key := range retrievedKeys {
			keyMap[key] = true
		}

		for _, expectedKey := range keysToAdd {
			if !keyMap[expectedKey] {
				t.Errorf("Expected key %s is missing from retrieved keys", expectedKey)
			}
		}
	})

	// Test 4: Mixed regular and expired keys
	t.Run("MixedExpirationKeys", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		ctx := context.Background()

		// Add some regular keys
		regularKeys := []string{"regular1", "regular2", "regular3"}
		for i, key := range regularKeys {
			err = c.Set(ctx, key, i, time.Hour)
			if err != nil {
				t.Fatalf("Failed to set regular key %s: %v", key, err)
			}
		}

		// Add some keys with short expiration
		expiredKeys := []string{"expired1", "expired2"}
		for i, key := range expiredKeys {
			err = c.Set(ctx, key, i, 10*time.Millisecond)
			if err != nil {
				t.Fatalf("Failed to set expired key %s: %v", key, err)
			}
		}

		// Wait for the keys to expire
		time.Sleep(50 * time.Millisecond)

		// Get all keys - should only return regular keys
		retrievedKeys := c.GetKeys(ctx)

		// Check count
		if len(retrievedKeys) != len(regularKeys) {
			t.Errorf("Expected %d keys (only regular keys), got %d keys",
				len(regularKeys), len(retrievedKeys))
		}

		// Check all regular keys are present and no expired keys are present
		keyMap := make(map[string]bool)
		for _, key := range retrievedKeys {
			keyMap[key] = true
		}

		for _, expectedKey := range regularKeys {
			if !keyMap[expectedKey] {
				t.Errorf("Expected regular key %s is missing from retrieved keys", expectedKey)
			}
		}

		for _, expiredKey := range expiredKeys {
			if keyMap[expiredKey] {
				t.Errorf("Expired key %s should not be in retrieved keys", expiredKey)
			}
		}
	})

	// Test 5: Keys with zero TTL (default TTL)
	t.Run("DefaultTTLKeys", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		ctx := context.Background()

		// Add key with default TTL
		err = c.Set(ctx, "default-ttl-key", "value", 0)
		if err != nil {
			t.Fatalf("Failed to set key: %v", err)
		}

		// Key should be returned by GetKeys
		keys := c.GetKeys(ctx)

		if len(keys) != 1 {
			t.Errorf("Expected 1 key, got %d keys", len(keys))
		}

		if len(keys) > 0 && keys[0] != "default-ttl-key" {
			t.Errorf("Expected key 'default-ttl-key', got '%s'", keys[0])
		}
	})

	// Test 6: Large number of keys
	t.Run("LargeNumberOfKeys", func(t *testing.T) {
		// Skip this test in normal runs as it's more of a benchmark
		if testing.Short() {
			t.Skip("Skipping large key test in short mode")
		}

		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		ctx := context.Background()

		// Add a large number of keys
		keyCount := 1000
		for i := 0; i < keyCount; i++ {
			key := fmt.Sprintf("key-%d", i)
			err = c.Set(ctx, key, i, time.Hour)
			if err != nil {
				t.Fatalf("Failed to set key %s: %v", key, err)
			}
		}

		// Get all keys
		retrievedKeys := c.GetKeys(ctx)

		// Check count
		if len(retrievedKeys) != keyCount {
			t.Errorf("Expected %d keys, got %d keys", keyCount, len(retrievedKeys))
		}
	})
}

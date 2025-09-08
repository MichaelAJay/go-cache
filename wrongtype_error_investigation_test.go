package cache_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/internal/testintegration"
)

// TestWRONGTYPE_Investigation systematically investigates WRONGTYPE errors
func TestWRONGTYPE_Investigation(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)

	t.Run("CleanState_SingleSetOperation", func(t *testing.T) {
		// Start with completely clean Redis state
		setup.FlushRedis(ctx, t)
		
		// Create cache instance
		cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, testintegration.DefaultCacheConfig())
		if err != nil {
			t.Fatalf("Failed to create cache: %v", err)
		}

		// Perform single SET operation
		session := &testintegration.TestSession{
			ID:       "test-session-1",
			UserID:   "user-1", 
			Username: "testuser",
			Created:  time.Now(),
		}

		err = cache.Set(ctx, session, 5*time.Minute)
		if err != nil {
			t.Fatalf("Single SET operation failed: %v", err)
		}
		t.Log("✅ Single SET operation succeeded with clean state")
	})

	t.Run("CleanState_SingleGetOperation", func(t *testing.T) {
		// Start with completely clean Redis state
		setup.FlushRedis(ctx, t)
		
		// Create cache instance
		cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, testintegration.DefaultCacheConfig())
		if err != nil {
			t.Fatalf("Failed to create cache: %v", err)
		}

		// First set a value
		session := &testintegration.TestSession{
			ID:       "test-session-2",
			UserID:   "user-2", 
			Username: "testuser2",
			Created:  time.Now(),
		}

		err = cache.Set(ctx, session, 5*time.Minute)
		if err != nil {
			t.Fatalf("SET operation failed: %v", err)
		}

		// Now try GET operation (use session ID as key)
		result, found, err := cache.Get(ctx, session.ID)
		if err != nil {
			t.Fatalf("Single GET operation failed: %v", err)
		}
		if !found {
			t.Fatalf("Value not found after SET")
		}
		t.Logf("✅ Single GET operation succeeded: %+v", result)
	})

	t.Run("InvestigateKeyCollision_ManualPollution", func(t *testing.T) {
		// Start with clean state
		setup.FlushRedis(ctx, t)
		
		// Create cache instance
		cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, testintegration.DefaultCacheConfig())
		if err != nil {
			t.Fatalf("Failed to create cache: %v", err)
		}

		testKey := "collision-test"
		
		// First, let's see what keys get created by a normal operation
		session := &testintegration.TestSession{
			ID:       testKey,
			UserID:   "collision-user", 
			Username: "collision-test",
			Created:  time.Now(),
		}

		// Perform a SET operation to see what keys are created
		err = cache.Set(ctx, session, 5*time.Minute)
		if err != nil {
			t.Fatalf("Initial SET failed: %v", err)
		}

		// List all keys to see what was created
		keys, err := setup.RedisClient.Keys(ctx, "*").Result()
		if err != nil {
			t.Fatalf("Failed to list keys: %v", err)
		}
		
		t.Logf("Keys created by cache.Set: %v", keys)
		
		// Check the type of each key
		for _, key := range keys {
			keyType, err := setup.RedisClient.Type(ctx, key).Result()
			if err != nil {
				continue
			}
			t.Logf("Key: %s, Type: %s", key, keyType)
		}

		// Clean up
		setup.FlushRedis(ctx, t)

		// Now manually create a STRING key where we expect a metadata HASH to be created
		// We'll guess that the metadata key pattern and pollute it
		possibleMetaKeys := []string{
			testKey + ":meta",
			"meta:" + testKey,
			testKey + "_meta", 
			"cache:meta:" + testKey,
		}

		for _, metaKey := range possibleMetaKeys {
			t.Logf("Testing pollution of potential metaKey: %s", metaKey)
			
			// Clean state
			setup.FlushRedis(ctx, t)
			
			// Pollute with STRING
			err = setup.RedisClient.Set(ctx, metaKey, "polluted-string-value", 0).Err()
			if err != nil {
				t.Fatalf("Failed to create polluted key: %v", err)
			}

			// Try cache operation
			err = cache.Set(ctx, session, 5*time.Minute)
			if err != nil {
				if strings.Contains(err.Error(), "WRONGTYPE") {
					t.Logf("✅ WRONGTYPE error reproduced with metaKey pattern: %s", metaKey)
					t.Logf("Error: %v", err)
					break // Found the pattern!
				} else {
					t.Logf("Unexpected error with %s: %v", metaKey, err)
				}
			} else {
				t.Logf("No error with %s - not the right pattern", metaKey)
			}
		}
	})

	t.Run("InvestigateKeyBuilding_DifferentKeys", func(t *testing.T) {
		// Test different key scenarios to see if they create different Redis key patterns
		setup.FlushRedis(ctx, t)

		// Create cache with minimal configuration
		cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, testintegration.DefaultCacheConfig())
		if err != nil {
			t.Fatalf("Failed to create cache: %v", err)
		}

		// Test different key scenarios
		testCases := []string{
			"simple",     // Simple key
			"with:colon", // Key with separator
			"data",       // Key that might conflict with prefixes
			"meta",       // Key that might conflict with prefixes
			"",           // Empty key - might cause issues
		}

		for _, testKey := range testCases {
			t.Logf("Testing with session ID: '%s'", testKey)
			
			// Clean state for each test
			setup.FlushRedis(ctx, t)
			
			session := &testintegration.TestSession{
				ID:       testKey,
				UserID:   "test-user", 
				Username: "test-username",
				Created:  time.Now(),
			}

			err = cache.Set(ctx, session, 5*time.Minute)
			if err != nil {
				t.Logf("❌ Error with key '%s': %v", testKey, err)
			} else {
				// List keys created
				keys, err := setup.RedisClient.Keys(ctx, "*").Result()
				if err == nil {
					t.Logf("✅ Key '%s' -> Redis keys: %v", testKey, keys)
				}
			}
		}
	})

	t.Run("ReproduceMultipleOperations_SameKey", func(t *testing.T) {
		// Test multiple operations on the same key
		setup.FlushRedis(ctx, t)
		
		cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, testintegration.DefaultCacheConfig())
		if err != nil {
			t.Fatalf("Failed to create cache: %v", err)
		}

		// Both sessions use the same ID so they operate on the same cache key
		testKey := "multi-ops-key"
		session1 := &testintegration.TestSession{
			ID:       testKey,
			UserID:   "multi-user", 
			Username: "multitest1",
			Created:  time.Now(),
		}

		session2 := &testintegration.TestSession{
			ID:       testKey,
			UserID:   "multi-user", 
			Username: "multitest2",
			Created:  time.Now(),
		}

		// First SET
		err = cache.Set(ctx, session1, 5*time.Minute)
		if err != nil {
			t.Fatalf("First SET failed: %v", err)
		}
		t.Log("✅ First SET succeeded")

		// GET
		result, found, err := cache.Get(ctx, testKey)
		if err != nil {
			t.Fatalf("GET after first SET failed: %v", err)
		}
		if !found {
			t.Fatalf("Value not found after first SET")
		}
		t.Logf("✅ GET succeeded: %s", result.Username)

		// Second SET (overwrite)
		err = cache.Set(ctx, session2, 5*time.Minute)
		if err != nil {
			t.Fatalf("Second SET failed: %v", err)
		}
		t.Log("✅ Second SET succeeded")

		// GET after overwrite
		result, found, err = cache.Get(ctx, testKey)
		if err != nil {
			t.Fatalf("GET after second SET failed: %v", err)
		}
		if !found {
			t.Fatalf("Value not found after second SET")
		}
		t.Logf("✅ GET after overwrite succeeded: %s", result.Username)
	})

	t.Run("InspectRedisKeysDirectly", func(t *testing.T) {
		// Direct Redis inspection during cache operations
		setup.FlushRedis(ctx, t)
		
		cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, testintegration.DefaultCacheConfig())
		if err != nil {
			t.Fatalf("Failed to create cache: %v", err)
		}

		testKey := "inspect-key"

		// Before any operations
		keys, err := setup.RedisClient.Keys(ctx, "*").Result()
		if err != nil {
			t.Fatalf("Failed to list keys: %v", err)
		}
		t.Logf("Keys before operation: %v", keys)

		// Perform SET operation
		session := &testintegration.TestSession{
			ID:       testKey,
			UserID:   "inspect-user", 
			Username: "inspect-test",
			Created:  time.Now(),
		}

		err = cache.Set(ctx, session, 5*time.Minute)
		if err != nil {
			t.Fatalf("SET operation failed: %v", err)
		}

		// After SET operation
		keys, err = setup.RedisClient.Keys(ctx, "*").Result()
		if err != nil {
			t.Fatalf("Failed to list keys: %v", err)
		}
		t.Logf("Keys after SET: %v", keys)

		// Check types of all created keys
		for _, key := range keys {
			keyType, err := setup.RedisClient.Type(ctx, key).Result()
			if err != nil {
				t.Logf("Failed to get type for key %s: %v", key, err)
				continue
			}
			t.Logf("Key: %s, Type: %s", key, keyType)
			
			// Try to identify the pattern based on key content and type
			if strings.Contains(key, testKey) {
				t.Logf("Key contains testKey '%s': %s (type: %s)", testKey, key, keyType)
			}
		}
	})
}
//go:build integration

package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/internal/testintegration"
	"github.com/stretchr/testify/require"
)

// TestMetadataCleanup_OrphanedMetadata tests the metadata cleanup gap fix
func TestMetadataCleanup_OrphanedMetadata(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	sessionCache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create cache")
	defer sessionCache.Close()

	sessionKey := "session:orphaned-metadata-test"
	
	session := &testintegration.TestSession{
		ID:       sessionKey,
		UserID:   "user123",
		Username: "cleanup-test-user",
		Created:  time.Now(),
	}

	t.Logf("📝 Testing orphaned metadata cleanup for session ID: %s", sessionKey)

	// 1. Set a session to create metadata
	err = sessionCache.Set(ctx, session, 5*time.Minute)
	require.NoError(t, err, "Failed to set session")

	// 2. Verify metadata exists initially
	metadata, err := sessionCache.GetMetadata(ctx, sessionKey)
	if err != nil {
		t.Fatalf("Failed to get metadata: %v", err)
	}
	if metadata == nil {
		t.Fatalf("Expected metadata to exist")
	}

	// 3. Get the actual data key pattern and verify it exists
	allKeys, err := setup.RedisClient.Keys(ctx, "*").Result()
	require.NoError(t, err, "Failed to list keys")
	t.Logf("   - Available keys: %v", allKeys)
	
	// Find the data key for our session (not the metadata key)
	var actualDataKey string
	for _, key := range allKeys {
		if key != "" && (key == "cache:data:"+sessionKey) {
			actualDataKey = key
			break
		}
	}
	
	require.NotEmpty(t, actualDataKey, "No data key found for session")
	t.Logf("   - Using data key: %s", actualDataKey)
	
	// 4. Directly delete the data key from Redis (simulating orphaned metadata)
	deleted, err := setup.RedisClient.Del(ctx, actualDataKey).Result()
	require.NoError(t, err, "Failed to delete data key directly")
	require.Equal(t, int64(1), deleted, "Expected to delete 1 key")

	t.Log("   - Data key deleted directly to create orphaned metadata")

	// 5. Verify metadata key still exists in Redis (orphaned state) 
	metaKey := "cache:meta:" + sessionKey
	exists, err := setup.RedisClient.Exists(ctx, metaKey).Result()
	if err != nil {
		t.Fatalf("Failed to check metadata existence: %v", err)
	}
	if exists == 0 {
		t.Fatalf("Expected orphaned metadata to still exist")
	}

	t.Log("   - Confirmed orphaned metadata exists")

	// 6. Call GetMetadata - this should trigger cleanup and return nil
	metadata, err = sessionCache.GetMetadata(ctx, sessionKey)
	if err != nil {
		t.Fatalf("Failed to get metadata: %v", err)
	}
	if metadata != nil {
		t.Fatalf("Expected metadata to be nil after cleanup, got: %+v", metadata)
	}

	t.Log("   - GetMetadata returned nil as expected")

	// 7. Verify metadata was cleaned up
	exists, err = setup.RedisClient.Exists(ctx, metaKey).Result()
	if err != nil {
		t.Fatalf("Failed to check metadata existence after cleanup: %v", err)
	}
	if exists != 0 {
		t.Fatalf("Expected orphaned metadata to be cleaned up, but it still exists")
	}

	t.Log("✅ Orphaned metadata cleanup test successful")
	t.Log("   - Orphaned metadata was automatically cleaned up")
	t.Log("   - GetMetadata correctly returned nil for missing data")
}
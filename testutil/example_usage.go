package testutil

import (
	"context"
	"time"

	"github.com/MichaelAJay/go-cache"
)

// ExampleUsageWithMockManager demonstrates how to use the MockCacheManager
// to test cache-dependent code using the same patterns as the production code
func ExampleUsageWithMockManager() (cache.Cache, error) {
	// Create a mock cache manager (same interface as real CacheManager)
	manager := NewMockCacheManager()

	// Register the mock provider (same API as real providers)
	manager.RegisterProvider("memory", NewMockProvider())

	// Create a cache instance using the same API as production code
	myCache, err := manager.GetCache("memory",
		cache.WithTTL(5*time.Minute),
		cache.WithMaxEntries(1000),
		cache.WithCleanupInterval(1*time.Minute),
	)
	if err != nil {
		return nil, err
	}

	return myCache, nil
}

// ExampleSessionManagerTest demonstrates testing a session manager
// that uses the cache with secondary indexing, just like in the README
func ExampleSessionManagerTest() error {
	// Setup mock cache manager
	manager := NewMockCacheManager()
	manager.RegisterProvider("redis", NewMockProvider())

	// Create cache with same configuration as production
	sessionCache, err := manager.GetCache("redis",
		cache.WithTTL(24*time.Hour),
		cache.WithIndexes(map[string]string{
			"sessions_by_user": "session:*",
		}),
	)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Test session creation
	sessionID := "session:123"
	userID := "user:456"
	sessionData := map[string]string{"userID": userID, "role": "user"}

	// Store session
	err = sessionCache.Set(ctx, sessionID, sessionData, time.Hour)
	if err != nil {
		return err
	}

	// Add to user index for bulk operations
	err = sessionCache.AddIndex(ctx, "sessions_by_user", "session:*", userID)
	if err != nil {
		return err
	}

	// Query by index (same as production code)
	sessionKeys, err := sessionCache.GetByIndex(ctx, "sessions_by_user", userID)
	if err != nil {
		return err
	}

	// Should find our session
	if len(sessionKeys) == 0 {
		return err
	}

	// Test bulk deletion by index
	err = sessionCache.DeleteByIndex(ctx, "sessions_by_user", userID)
	if err != nil {
		return err
	}

	return nil
}

// ExamplePatternOperationsTest demonstrates testing pattern-based operations
func ExamplePatternOperationsTest() error {
	manager := NewMockCacheManager()
	manager.RegisterProvider("memory", NewMockProvider())

	cache, err := manager.GetCache("memory", cache.WithTTL(time.Hour))
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Set up test data
	cache.Set(ctx, "session:user1:123", "data1", time.Hour)
	cache.Set(ctx, "session:user1:456", "data2", time.Hour)
	cache.Set(ctx, "session:user2:789", "data3", time.Hour)
	cache.Set(ctx, "temp:cache:abc", "temp1", time.Hour)

	// Test pattern matching
	sessionKeys, err := cache.GetKeysByPattern(ctx, "session:user1:*")
	if err != nil {
		return err
	}

	// Should find 2 sessions for user1
	if len(sessionKeys) != 2 {
		return err
	}

	// Test bulk deletion by pattern
	deletedCount, err := cache.DeleteByPattern(ctx, "temp:*")
	if err != nil {
		return err
	}

	// Should have deleted 1 temp key
	if deletedCount != 1 {
		return err
	}

	return nil
}
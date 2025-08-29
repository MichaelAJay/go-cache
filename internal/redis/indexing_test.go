//go:build integration
// +build integration

package redis

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSecondaryIndexing tests all aspects of the secondary indexing system
func TestSecondaryIndexing(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	
	t.Run("IndexConsistency", func(t *testing.T) {
		testIndexConsistency(t, container)
	})
	
	t.Run("IndexConcurrency", func(t *testing.T) {
		testIndexConcurrency(t, container)
	})
	
	t.Run("IndexPatternMatching", func(t *testing.T) {
		testIndexPatternMatching(t, container)
	})
	
	t.Run("IndexCleanup", func(t *testing.T) {
		testIndexCleanup(t, container)
	})
	
	t.Run("CrossProcessIndexing", func(t *testing.T) {
		testCrossProcessIndexing(t, container)
	})
}

// testIndexConsistency tests that indexes remain consistent under concurrent operations
func testIndexConsistency(t *testing.T, container *TestRedisContainer) {
	cache := CreateCacheForTesting[*User](container)
	defer cache.Close()
	
	ctx := context.Background()
	indexName := "users_by_role"
	indexKey := "admin"
	
	// Create test users
	users := GenerateTestUsers(50)
	for i, user := range users {
		key := fmt.Sprintf("user:%d", i)
		
		// Set the user in cache
		err := cache.Set(ctx, key, user, time.Hour)
		require.NoError(t, err)
		
		// Add to index
		err = cache.AddIndex(ctx, indexName, "user:*", indexKey)
		require.NoError(t, err)
	}
	
	// Verify index contains all keys
	indexedKeys, err := cache.GetByIndex(ctx, indexName, indexKey)
	require.NoError(t, err)
	assert.Equal(t, len(users), len(indexedKeys), "Index should contain all user keys")
	
	// Verify all indexed keys exist in cache
	for _, key := range indexedKeys {
		exists := cache.Has(ctx, key)
		assert.True(t, exists, "Indexed key %s should exist in cache", key)
		
		_, found, err := cache.Get(ctx, key)
		require.NoError(t, err)
		assert.True(t, found, "Should be able to retrieve indexed key %s", key)
	}
	
	// Delete some keys and verify index consistency
	keysToDelete := indexedKeys[:10] // Delete first 10 keys
	for _, key := range keysToDelete {
		err := cache.Delete(ctx, key)
		require.NoError(t, err)
	}
	
	// Refresh index view (index cleanup happens automatically in GetByIndex)
	updatedIndexedKeys, err := cache.GetByIndex(ctx, indexName, indexKey)
	require.NoError(t, err)
	
	// Should have fewer keys now
	assert.Equal(t, len(users)-len(keysToDelete), len(updatedIndexedKeys), 
		"Index should reflect deleted keys")
	
	// Verify remaining keys still exist
	for _, key := range updatedIndexedKeys {
		exists := cache.Has(ctx, key)
		assert.True(t, exists, "Remaining indexed key %s should still exist", key)
	}
}

// testIndexConcurrency tests index operations under high concurrency
func testIndexConcurrency(t *testing.T, container *TestRedisContainer) {
	cache := CreateCacheForTesting[*User](container)
	defer cache.Close()
	
	ctx := context.Background()
	numGoroutines := 50
	usersPerGoroutine := 10
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	// Each goroutine creates users and adds them to different indexes
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			
			indexName := fmt.Sprintf("users_by_dept_%d", goroutineID)
			indexKey := fmt.Sprintf("dept_%d", goroutineID)
			
			for u := 0; u < usersPerGoroutine; u++ {
				userID := goroutineID*usersPerGoroutine + u
				key := fmt.Sprintf("user:%d", userID)
				
				user := &User{
					ID:      fmt.Sprintf("user-%d", userID),
					Name:    fmt.Sprintf("User %d", userID),
					Email:   fmt.Sprintf("user%d@dept%d.com", userID, goroutineID),
					Created: time.Now(),
				}
				
				// Set user
				err := cache.Set(ctx, key, user, time.Hour)
				assert.NoError(t, err, "Set should succeed for goroutine %d user %d", 
					goroutineID, u)
				
				// Add to index
				err = cache.AddIndex(ctx, indexName, "user:*", indexKey)
				assert.NoError(t, err, "AddIndex should succeed for goroutine %d user %d", 
					goroutineID, u)
			}
		}(g)
	}
	
	wg.Wait()
	
	// Verify each index has the expected number of keys
	for g := 0; g < numGoroutines; g++ {
		indexName := fmt.Sprintf("users_by_dept_%d", g)
		indexKey := fmt.Sprintf("dept_%d", g)
		
		indexedKeys, err := cache.GetByIndex(ctx, indexName, indexKey)
		require.NoError(t, err, "GetByIndex should succeed for goroutine %d", g)
		
		// Should contain all users from this goroutine
		expectedCount := usersPerGoroutine
		actualCount := len(indexedKeys)
		
		assert.GreaterOrEqual(t, actualCount, expectedCount, 
			"Index for goroutine %d should have at least %d keys, got %d", 
			g, expectedCount, actualCount)
	}
}

// testIndexPatternMatching tests pattern matching accuracy for indexing
func testIndexPatternMatching(t *testing.T, container *TestRedisContainer) {
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	ctx := context.Background()
	
	testCases := []struct {
		name        string
		keys        []string
		pattern     string
		indexName   string
		indexKey    string
		expectedMatch int
	}{
		{
			name:         "SimpleWildcard",
			keys:         []string{"user:1", "user:2", "admin:1", "user:3"},
			pattern:      "user:*",
			indexName:    "users",
			indexKey:     "all",
			expectedMatch: 3,
		},
		{
			name:         "PrefixMatch",
			keys:         []string{"session:abc", "session:def", "token:xyz", "session:ghi"},
			pattern:      "session:*",
			indexName:    "sessions",
			indexKey:     "active",
			expectedMatch: 3,
		},
		{
			name:         "NoMatch",
			keys:         []string{"user:1", "user:2", "user:3"},
			pattern:      "admin:*",
			indexName:    "admins",
			indexKey:     "all",
			expectedMatch: 0,
		},
		{
			name:         "ExactMatch",
			keys:         []string{"cache:data", "cache:meta", "other:data"},
			pattern:      "cache:data",
			indexName:    "cache_data",
			indexKey:     "exact",
			expectedMatch: 1,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up test data
			for i, key := range tc.keys {
				value := fmt.Sprintf("value-%d", i)
				err := cache.Set(ctx, key, value, time.Hour)
				require.NoError(t, err, "Set should succeed for key %s", key)
			}
			
			// Add to index using pattern
			err := cache.AddIndex(ctx, tc.indexName, tc.pattern, tc.indexKey)
			require.NoError(t, err, "AddIndex should succeed")
			
			// Verify index contains expected keys
			indexedKeys, err := cache.GetByIndex(ctx, tc.indexName, tc.indexKey)
			require.NoError(t, err, "GetByIndex should succeed")
			
			assert.Equal(t, tc.expectedMatch, len(indexedKeys), 
				"Index should contain %d keys for pattern %s", 
				tc.expectedMatch, tc.pattern)
			
			// Verify indexed keys actually match the pattern
			for _, key := range indexedKeys {
				// Simple pattern matching verification
				if tc.pattern[len(tc.pattern)-1] == '*' {
					prefix := tc.pattern[:len(tc.pattern)-1]
					assert.True(t, len(key) >= len(prefix) && key[:len(prefix)] == prefix,
						"Key %s should match pattern %s", key, tc.pattern)
				} else {
					assert.Equal(t, tc.pattern, key, 
						"Key %s should exactly match pattern %s", key, tc.pattern)
				}
			}
			
			// Clean up for next test
			for _, key := range tc.keys {
				cache.Delete(ctx, key)
			}
		})
	}
}

// testIndexCleanup tests automatic cleanup of orphaned index entries
func testIndexCleanup(t *testing.T, container *TestRedisContainer) {
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	ctx := context.Background()
	indexName := "cleanup_test"
	indexKey := "orphaned"
	
	// Create some keys and index them
	keys := []string{"cleanup:1", "cleanup:2", "cleanup:3", "cleanup:4", "cleanup:5"}
	for i, key := range keys {
		value := fmt.Sprintf("value-%d", i)
		err := cache.Set(ctx, key, value, time.Hour)
		require.NoError(t, err)
	}
	
	// Add all keys to index
	err := cache.AddIndex(ctx, indexName, "cleanup:*", indexKey)
	require.NoError(t, err)
	
	// Verify index contains all keys
	indexedKeys, err := cache.GetByIndex(ctx, indexName, indexKey)
	require.NoError(t, err)
	assert.Equal(t, len(keys), len(indexedKeys))
	
	// Delete some keys directly (bypassing index removal)
	keysToDelete := keys[:3]
	for _, key := range keysToDelete {
		err := cache.Delete(ctx, key)
		require.NoError(t, err)
	}
	
	// GetByIndex should automatically clean up orphaned entries
	updatedIndexedKeys, err := cache.GetByIndex(ctx, indexName, indexKey)
	require.NoError(t, err)
	
	expectedRemaining := len(keys) - len(keysToDelete)
	assert.Equal(t, expectedRemaining, len(updatedIndexedKeys), 
		"Index should automatically clean up orphaned entries")
	
	// Verify remaining keys are the correct ones
	remainingKeys := keys[3:] // Should be cleanup:4 and cleanup:5
	sort.Strings(updatedIndexedKeys)
	sort.Strings(remainingKeys)
	
	for i, key := range remainingKeys {
		assert.Equal(t, key, updatedIndexedKeys[i], 
			"Remaining keys should match expected keys")
	}
}

// testCrossProcessIndexing tests index coordination across multiple cache instances
func testCrossProcessIndexing(t *testing.T, container *TestRedisContainer) {
	// Create two cache instances to simulate cross-process behavior
	cache1 := CreateCacheForTesting[*User](container)
	defer cache1.Close()
	
	cache2 := CreateCacheForTesting[*User](container)
	defer cache2.Close()
	
	ctx := context.Background()
	indexName := "cross_process_users"
	indexKey := "team_a"
	
	// Cache1 creates some users
	users1 := GenerateTestUsers(25)
	for i, user := range users1 {
		key := fmt.Sprintf("user:cache1:%d", i)
		err := cache1.Set(ctx, key, user, time.Hour)
		require.NoError(t, err)
	}
	
	// Cache2 creates some users
	users2 := GenerateTestUsers(25)
	for i, user := range users2 {
		key := fmt.Sprintf("user:cache2:%d", i)
		err := cache2.Set(ctx, key, user, time.Hour)
		require.NoError(t, err)
	}
	
	// Cache1 adds its users to the index
	err := cache1.AddIndex(ctx, indexName, "user:cache1:*", indexKey)
	require.NoError(t, err)
	
	// Cache2 adds its users to the same index
	err = cache2.AddIndex(ctx, indexName, "user:cache2:*", indexKey)
	require.NoError(t, err)
	
	// Both caches should see all indexed keys
	indexedKeys1, err := cache1.GetByIndex(ctx, indexName, indexKey)
	require.NoError(t, err)
	
	indexedKeys2, err := cache2.GetByIndex(ctx, indexName, indexKey)
	require.NoError(t, err)
	
	// Both should see the same index contents
	assert.Equal(t, len(indexedKeys1), len(indexedKeys2), 
		"Both cache instances should see same index contents")
	
	expectedTotal := len(users1) + len(users2)
	assert.Equal(t, expectedTotal, len(indexedKeys1), 
		"Index should contain users from both cache instances")
	
	// Verify cross-visibility: cache1 should be able to see cache2's keys in index
	cache2Keys := 0
	for _, key := range indexedKeys1 {
		if len(key) > 12 && key[:12] == "user:cache2:" {
			cache2Keys++
		}
	}
	assert.Equal(t, len(users2), cache2Keys, 
		"Cache1 should see Cache2's keys in shared index")
	
	// Test index removal coordination
	// Cache1 removes some of its entries from the index
	err = cache1.RemoveIndex(ctx, indexName, "user:cache1:*", indexKey)
	require.NoError(t, err)
	
	// Cache2 should see the updated index (with only cache2 keys)
	updatedIndexedKeys, err := cache2.GetByIndex(ctx, indexName, indexKey)
	require.NoError(t, err)
	
	// Should only contain cache2 keys now
	cache2KeysAfterRemoval := 0
	cache1KeysAfterRemoval := 0
	for _, key := range updatedIndexedKeys {
		if len(key) > 12 && key[:12] == "user:cache2:" {
			cache2KeysAfterRemoval++
		} else if len(key) > 12 && key[:12] == "user:cache1:" {
			cache1KeysAfterRemoval++
		}
	}
	
	assert.Equal(t, len(users2), cache2KeysAfterRemoval, 
		"Cache2 keys should remain in index")
	assert.Equal(t, 0, cache1KeysAfterRemoval, 
		"Cache1 keys should be removed from index")
}

// TestGetKeysByPattern tests pattern-based key retrieval
func TestGetKeysByPattern(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	ctx := context.Background()
	
	// Set up test data with various patterns
	testData := map[string]string{
		"user:1":      "user1",
		"user:2":      "user2", 
		"user:10":     "user10",
		"admin:1":     "admin1",
		"admin:2":     "admin2",
		"session:abc": "session1",
		"session:def": "session2",
		"cache:data":  "cached",
		"other":       "other",
	}
	
	// Populate cache
	for key, value := range testData {
		err := cache.Set(ctx, key, value, time.Hour)
		require.NoError(t, err, "Set should succeed for key %s", key)
	}
	
	testCases := []struct {
		pattern      string
		expectedKeys []string
		description  string
	}{
		{
			pattern:      "user:*",
			expectedKeys: []string{"user:1", "user:2", "user:10"},
			description:  "User keys with wildcard",
		},
		{
			pattern:      "admin:*",
			expectedKeys: []string{"admin:1", "admin:2"},
			description:  "Admin keys with wildcard",
		},
		{
			pattern:      "session:*",
			expectedKeys: []string{"session:abc", "session:def"},
			description:  "Session keys with wildcard",
		},
		{
			pattern:      "*:1",
			expectedKeys: []string{"user:1", "admin:1"},
			description:  "Keys ending with :1",
		},
		{
			pattern:      "cache:data",
			expectedKeys: []string{"cache:data"},
			description:  "Exact match",
		},
		{
			pattern:      "nonexistent:*",
			expectedKeys: []string{},
			description:  "No matching keys",
		},
		{
			pattern:      "*",
			expectedKeys: []string{"user:1", "user:2", "user:10", "admin:1", "admin:2", 
				"session:abc", "session:def", "cache:data", "other"},
			description:  "All keys",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			matchedKeys, err := cache.GetKeysByPattern(ctx, tc.pattern)
			require.NoError(t, err, "GetKeysByPattern should succeed for pattern %s", tc.pattern)
			
			// Sort both slices for comparison
			sort.Strings(matchedKeys)
			sort.Strings(tc.expectedKeys)
			
			assert.Equal(t, tc.expectedKeys, matchedKeys, 
				"Pattern %s should match expected keys", tc.pattern)
			
			// Verify all matched keys actually exist and have correct values
			for _, key := range matchedKeys {
				value, found, err := cache.Get(ctx, key)
				require.NoError(t, err, "Get should succeed for matched key %s", key)
				assert.True(t, found, "Matched key %s should exist", key)
				
				expectedValue, exists := testData[key]
				assert.True(t, exists, "Matched key %s should be in test data", key)
				assert.Equal(t, expectedValue, value, 
					"Value for key %s should match expected", key)
			}
		})
	}
}

// TestDeleteByPattern tests pattern-based deletion
func TestDeleteByPattern(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	ctx := context.Background()
	
	// Set up test data
	testData := map[string]string{
		"user:1":      "user1",
		"user:2":      "user2",
		"user:3":      "user3",
		"admin:1":     "admin1", 
		"admin:2":     "admin2",
		"session:abc": "session1",
		"temp:1":      "temp1",
		"temp:2":      "temp2",
		"permanent":   "perm",
	}
	
	// Populate cache
	for key, value := range testData {
		err := cache.Set(ctx, key, value, time.Hour)
		require.NoError(t, err)
	}
	
	// Test deleting user keys
	deletedCount, err := cache.DeleteByPattern(ctx, "user:*")
	require.NoError(t, err, "DeleteByPattern should succeed")
	assert.Equal(t, 3, deletedCount, "Should delete 3 user keys")
	
	// Verify user keys are gone
	userKeys := []string{"user:1", "user:2", "user:3"}
	for _, key := range userKeys {
		exists := cache.Has(ctx, key)
		assert.False(t, exists, "User key %s should be deleted", key)
	}
	
	// Verify other keys still exist
	remainingKeys := []string{"admin:1", "admin:2", "session:abc", "temp:1", "temp:2", "permanent"}
	for _, key := range remainingKeys {
		exists := cache.Has(ctx, key)
		assert.True(t, exists, "Key %s should still exist", key)
	}
	
	// Test deleting temp keys
	deletedCount, err = cache.DeleteByPattern(ctx, "temp:*")
	require.NoError(t, err, "DeleteByPattern should succeed for temp keys")
	assert.Equal(t, 2, deletedCount, "Should delete 2 temp keys")
	
	// Test exact pattern match
	deletedCount, err = cache.DeleteByPattern(ctx, "permanent")
	require.NoError(t, err, "DeleteByPattern should succeed for exact match")
	assert.Equal(t, 1, deletedCount, "Should delete 1 exact match key")
	
	// Test non-matching pattern
	deletedCount, err = cache.DeleteByPattern(ctx, "nonexistent:*")
	require.NoError(t, err, "DeleteByPattern should succeed even for non-matching pattern")
	assert.Equal(t, 0, deletedCount, "Should delete 0 keys for non-matching pattern")
	
	// Verify final state
	finalKeys := []string{"admin:1", "admin:2", "session:abc"}
	for _, key := range finalKeys {
		exists := cache.Has(ctx, key)
		assert.True(t, exists, "Final key %s should still exist", key)
	}
}

// TestDeleteByIndex tests deletion of all keys associated with an index
func TestDeleteByIndex(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	
	cache := CreateCacheForTesting[*User](container)
	defer cache.Close()
	
	ctx := context.Background()
	indexName := "users_by_status"
	
	// Create users with different statuses
	activeUsers := GenerateTestUsers(10)
	inactiveUsers := GenerateTestUsers(5)
	
	// Set active users
	for i, user := range activeUsers {
		key := fmt.Sprintf("user:active:%d", i)
		err := cache.Set(ctx, key, user, time.Hour)
		require.NoError(t, err)
	}
	
	// Set inactive users  
	for i, user := range inactiveUsers {
		key := fmt.Sprintf("user:inactive:%d", i)
		err := cache.Set(ctx, key, user, time.Hour)
		require.NoError(t, err)
	}
	
	// Add to indexes
	err := cache.AddIndex(ctx, indexName, "user:active:*", "active")
	require.NoError(t, err)
	
	err = cache.AddIndex(ctx, indexName, "user:inactive:*", "inactive")
	require.NoError(t, err)
	
	// Verify indexes are populated
	activeKeys, err := cache.GetByIndex(ctx, indexName, "active")
	require.NoError(t, err)
	assert.Equal(t, len(activeUsers), len(activeKeys))
	
	inactiveKeys, err := cache.GetByIndex(ctx, indexName, "inactive")
	require.NoError(t, err)
	assert.Equal(t, len(inactiveUsers), len(inactiveKeys))
	
	// Delete all inactive users by index
	err = cache.DeleteByIndex(ctx, indexName, "inactive")
	require.NoError(t, err, "DeleteByIndex should succeed")
	
	// Verify inactive users are deleted
	for i := 0; i < len(inactiveUsers); i++ {
		key := fmt.Sprintf("user:inactive:%d", i)
		exists := cache.Has(ctx, key)
		assert.False(t, exists, "Inactive user key %s should be deleted", key)
	}
	
	// Verify active users still exist
	for i := 0; i < len(activeUsers); i++ {
		key := fmt.Sprintf("user:active:%d", i)
		exists := cache.Has(ctx, key)
		assert.True(t, exists, "Active user key %s should still exist", key)
	}
	
	// Verify inactive index is cleaned up
	inactiveKeysAfterDelete, err := cache.GetByIndex(ctx, indexName, "inactive")
	require.NoError(t, err)
	assert.Equal(t, 0, len(inactiveKeysAfterDelete), 
		"Inactive index should be empty after deletion")
	
	// Verify active index is unchanged
	activeKeysAfterDelete, err := cache.GetByIndex(ctx, indexName, "active")
	require.NoError(t, err)
	assert.Equal(t, len(activeUsers), len(activeKeysAfterDelete), 
		"Active index should be unchanged")
}
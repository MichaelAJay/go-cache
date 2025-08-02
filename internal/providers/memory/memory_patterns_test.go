package memory_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
)

func TestMemoryCache_PatternOperations(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Set up test data with various patterns
	testData := map[string]string{
		"user:1":        "user1",
		"user:2":        "user2",
		"user:3":        "user3",
		"session:abc":   "session_abc",
		"session:def":   "session_def",
		"session:ghi":   "session_ghi",
		"cache:temp:1":  "temp1",
		"cache:temp:2":  "temp2",
		"cache:perm:1":  "perm1",
		"cache:perm:2":  "perm2",
		"other:data":    "other",
	}

	for key, value := range testData {
		err := c.Set(ctx, key, value, time.Hour)
		if err != nil {
			t.Fatalf("Failed to set %s: %v", key, err)
		}
	}

	// Test GetKeysByPattern
	t.Run("GetUserKeys", func(t *testing.T) {
		keys, err := c.GetKeysByPattern(ctx, "user:*")
		if err != nil {
			t.Errorf("GetKeysByPattern failed: %v", err)
		}
		
		expected := []string{"user:1", "user:2", "user:3"}
		sort.Strings(keys)
		sort.Strings(expected)
		
		if len(keys) != len(expected) {
			t.Errorf("Expected %d keys, got %d: %v", len(expected), len(keys), keys)
		}
		
		for i, key := range keys {
			if i < len(expected) && key != expected[i] {
				t.Errorf("Expected key %s, got %s", expected[i], key)
			}
		}
	})

	t.Run("GetSessionKeys", func(t *testing.T) {
		keys, err := c.GetKeysByPattern(ctx, "session:*")
		if err != nil {
			t.Errorf("GetKeysByPattern failed: %v", err)
		}

		expected := []string{"session:abc", "session:def", "session:ghi"}
		sort.Strings(keys)
		sort.Strings(expected)

		if len(keys) != len(expected) {
			t.Errorf("Expected %d keys, got %d: %v", len(expected), len(keys), keys)
		}

		for i, key := range keys {
			if i < len(expected) && key != expected[i] {
				t.Errorf("Expected key %s, got %s", expected[i], key)
			}
		}
	})

	t.Run("GetCacheTempKeys", func(t *testing.T) {
		keys, err := c.GetKeysByPattern(ctx, "cache:temp:*")
		if err != nil {
			t.Errorf("GetKeysByPattern failed: %v", err)
		}

		expected := []string{"cache:temp:1", "cache:temp:2"}
		sort.Strings(keys)
		sort.Strings(expected)

		if len(keys) != len(expected) {
			t.Errorf("Expected %d keys, got %d: %v", len(expected), len(keys), keys)
		}
	})

	// Test DeleteByPattern
	t.Run("DeleteUserPattern", func(t *testing.T) {
		count, err := c.DeleteByPattern(ctx, "user:*")
		if err != nil {
			t.Errorf("DeleteByPattern failed: %v", err)
		}
		if count != 3 {
			t.Errorf("Expected to delete 3 keys, got %d", count)
		}

		// Verify keys are gone
		keys, err := c.GetKeysByPattern(ctx, "user:*")
		if err != nil {
			t.Errorf("GetKeysByPattern after delete failed: %v", err)
		}
		if len(keys) != 0 {
			t.Errorf("Expected no keys after delete, got %d: %v", len(keys), keys)
		}

		// Verify other keys still exist
		keys, err = c.GetKeysByPattern(ctx, "session:*")
		if err != nil {
			t.Errorf("GetKeysByPattern for sessions failed: %v", err)
		}
		if len(keys) != 3 {
			t.Errorf("Expected 3 session keys to remain, got %d", len(keys))
		}
	})
}

func TestMemoryCache_PatternOperationsEdgeCases(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Test with empty pattern
	t.Run("EmptyPattern", func(t *testing.T) {
		keys, err := c.GetKeysByPattern(ctx, "")
		if err != nil {
			t.Errorf("GetKeysByPattern with empty pattern failed: %v", err)
		}
		// Should return all keys or no keys, both are valid behaviors
		t.Logf("Empty pattern returned %d keys", len(keys))
	})

	// Test with pattern that matches nothing
	t.Run("NoMatchPattern", func(t *testing.T) {
		keys, err := c.GetKeysByPattern(ctx, "nonexistent:*")
		if err != nil {
			t.Errorf("GetKeysByPattern with no matches failed: %v", err)
		}
		if len(keys) != 0 {
			t.Errorf("Expected no keys for non-matching pattern, got %d", len(keys))
		}

		count, err := c.DeleteByPattern(ctx, "nonexistent:*")
		if err != nil {
			t.Errorf("DeleteByPattern with no matches failed: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected 0 deletes for non-matching pattern, got %d", count)
		}
	})

	// Test with wildcard-only pattern
	t.Run("WildcardOnlyPattern", func(t *testing.T) {
		// Set some test data first
		err := c.Set(ctx, "test1", "value1", time.Hour)
		if err != nil {
			t.Fatalf("Failed to set test data: %v", err)
		}
		err = c.Set(ctx, "test2", "value2", time.Hour)
		if err != nil {
			t.Fatalf("Failed to set test data: %v", err)
		}

		keys, err := c.GetKeysByPattern(ctx, "*")
		if err != nil {
			t.Errorf("GetKeysByPattern with wildcard failed: %v", err)
		}
		if len(keys) < 2 {
			t.Errorf("Expected at least 2 keys for wildcard pattern, got %d", len(keys))
		}
	})

	// Test with canceled context
	t.Run("CanceledContext", func(t *testing.T) {
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		_, err := c.GetKeysByPattern(canceledCtx, "test:*")
		if err != cache.ErrContextCanceled {
			t.Errorf("Expected ErrContextCanceled, got %v", err)
		}

		_, err = c.DeleteByPattern(canceledCtx, "test:*")
		if err != cache.ErrContextCanceled {
			t.Errorf("Expected ErrContextCanceled, got %v", err)
		}
	})

	// Test with patterns containing special characters
	t.Run("SpecialCharacterPatterns", func(t *testing.T) {
		specialKeys := []string{
			"key:with:colons",
			"key-with-dashes",
			"key_with_underscores",
			"key.with.dots",
			"key@with@ats",
			"key#with#hashes",
		}

		for _, key := range specialKeys {
			err := c.Set(ctx, key, "value", time.Hour)
			if err != nil {
				t.Fatalf("Failed to set key %s: %v", key, err)
			}
		}

		// Test pattern matching with colons
		keys, err := c.GetKeysByPattern(ctx, "key:*")
		if err != nil {
			t.Errorf("Pattern with colons failed: %v", err)
		}
		if len(keys) < 1 {
			t.Errorf("Expected at least 1 key with colon pattern, got %d", len(keys))
		}

		// Test pattern matching with dashes
		keys, err = c.GetKeysByPattern(ctx, "key-*")
		if err != nil {
			t.Errorf("Pattern with dashes failed: %v", err)
		}
		if len(keys) < 1 {
			t.Errorf("Expected at least 1 key with dash pattern, got %d", len(keys))
		}
	})
}

func TestMemoryCache_PatternOperationsWithExpiry(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	shortTTL := 100 * time.Millisecond

	// Set keys with short TTL
	for i := 0; i < 5; i++ {
		key := "expiry:test:" + string(rune(i+'0'))
		err := c.Set(ctx, key, "value", shortTTL)
		if err != nil {
			t.Fatalf("Failed to set key %s: %v", key, err)
		}
	}

	// Verify all keys exist
	keys, err := c.GetKeysByPattern(ctx, "expiry:test:*")
	if err != nil {
		t.Errorf("GetKeysByPattern failed: %v", err)
	}
	if len(keys) != 5 {
		t.Errorf("Expected 5 keys before expiry, got %d", len(keys))
	}

	// Wait for expiry
	time.Sleep(150 * time.Millisecond)

	// Keys should be gone
	keys, err = c.GetKeysByPattern(ctx, "expiry:test:*")
	if err != nil {
		t.Errorf("GetKeysByPattern after expiry failed: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("Expected 0 keys after expiry, got %d", len(keys))
	}

	// DeleteByPattern on expired keys should return 0
	count, err := c.DeleteByPattern(ctx, "expiry:test:*")
	if err != nil {
		t.Errorf("DeleteByPattern on expired keys failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 deletes on expired keys, got %d", count)
	}
}

func TestMemoryCache_PatternOperationsLargeDataset(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Create a large dataset
	const numKeys = 1000
	const numPatterns = 10

	// Set up keys in multiple patterns
	for pattern := 0; pattern < numPatterns; pattern++ {
		for i := 0; i < numKeys/numPatterns; i++ {
			key := "pattern" + string(rune(pattern+'0')) + ":key:" + string(rune(i/10+'0')) + string(rune(i%10+'0'))
			err := c.Set(ctx, key, "value", time.Hour)
			if err != nil {
				t.Fatalf("Failed to set key %s: %v", key, err)
			}
		}
	}

	// Test pattern matching on large dataset
	for pattern := 0; pattern < numPatterns; pattern++ {
		patternStr := "pattern" + string(rune(pattern+'0')) + ":*"
		keys, err := c.GetKeysByPattern(ctx, patternStr)
		if err != nil {
			t.Errorf("GetKeysByPattern failed for %s: %v", patternStr, err)
		}
		expected := numKeys / numPatterns
		if len(keys) != expected {
			t.Errorf("Expected %d keys for pattern %s, got %d", expected, patternStr, len(keys))
		}
	}

	// Test deleting one pattern
	count, err := c.DeleteByPattern(ctx, "pattern0:*")
	if err != nil {
		t.Errorf("DeleteByPattern failed: %v", err)
	}
	expected := numKeys / numPatterns
	if count != expected {
		t.Errorf("Expected to delete %d keys, got %d", expected, count)
	}

	// Verify the pattern was deleted
	keys, err := c.GetKeysByPattern(ctx, "pattern0:*")
	if err != nil {
		t.Errorf("GetKeysByPattern after delete failed: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("Expected 0 keys after delete, got %d", len(keys))
	}

	// Verify other patterns still exist
	keys, err = c.GetKeysByPattern(ctx, "pattern1:*")
	if err != nil {
		t.Errorf("GetKeysByPattern for remaining pattern failed: %v", err)
	}
	if len(keys) != expected {
		t.Errorf("Expected %d keys for remaining pattern, got %d", expected, len(keys))
	}
}
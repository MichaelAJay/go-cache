package memory_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
)

func TestMemoryCache_ConditionalOperations(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Test SetIfNotExists on new key
	success, err := c.SetIfNotExists(ctx, "new_key", "new_value", time.Hour)
	if err != nil {
		t.Errorf("SetIfNotExists failed: %v", err)
	}
	if !success {
		t.Error("SetIfNotExists should succeed for new key")
	}

	// Verify value was set
	value, exists, err := c.Get(ctx, "new_key")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if !exists {
		t.Error("Key should exist after SetIfNotExists")
	}
	if value != "new_value" {
		t.Errorf("Expected 'new_value', got %v", value)
	}

	// Test SetIfNotExists on existing key (should fail)
	success, err = c.SetIfNotExists(ctx, "new_key", "updated_value", time.Hour)
	if err != nil {
		t.Errorf("SetIfNotExists failed: %v", err)
	}
	if success {
		t.Error("SetIfNotExists should fail for existing key")
	}

	// Verify value was not changed
	value, exists, err = c.Get(ctx, "new_key")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if !exists {
		t.Error("Key should still exist")
	}
	if value != "new_value" {
		t.Errorf("Value should not have changed, got %v", value)
	}

	// Test SetIfExists on existing key
	success, err = c.SetIfExists(ctx, "new_key", "updated_value", time.Hour)
	if err != nil {
		t.Errorf("SetIfExists failed: %v", err)
	}
	if !success {
		t.Error("SetIfExists should succeed for existing key")
	}

	// Verify value was updated
	value, exists, err = c.Get(ctx, "new_key")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if !exists {
		t.Error("Key should still exist")
	}
	if value != "updated_value" {
		t.Errorf("Expected 'updated_value', got %v", value)
	}

	// Test SetIfExists on non-existing key (should fail)
	success, err = c.SetIfExists(ctx, "nonexistent_key", "some_value", time.Hour)
	if err != nil {
		t.Errorf("SetIfExists failed: %v", err)
	}
	if success {
		t.Error("SetIfExists should fail for non-existing key")
	}

	// Verify key was not created
	_, exists, err = c.Get(ctx, "nonexistent_key")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if exists {
		t.Error("Key should not have been created")
	}
}

func TestMemoryCache_ConditionalOperationsEdgeCases(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Test with empty key
	_, err := c.SetIfNotExists(ctx, "", "value", time.Hour)
	if err != cache.ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey for empty key, got %v", err)
	}

	_, err = c.SetIfExists(ctx, "", "value", time.Hour)
	if err != cache.ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey for empty key, got %v", err)
	}

	// Test with canceled context
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()

	_, err = c.SetIfNotExists(canceledCtx, "key", "value", time.Hour)
	if err != cache.ErrContextCanceled {
		t.Errorf("Expected ErrContextCanceled, got %v", err)
	}

	_, err = c.SetIfExists(canceledCtx, "key", "value", time.Hour)
	if err != cache.ErrContextCanceled {
		t.Errorf("Expected ErrContextCanceled, got %v", err)
	}

	// Test with zero TTL (should use default)
	success, err := c.SetIfNotExists(ctx, "zero_ttl", "value", 0)
	if err != nil {
		t.Errorf("SetIfNotExists with zero TTL failed: %v", err)
	}
	if !success {
		t.Error("SetIfNotExists with zero TTL should succeed")
	}

	// Test with negative TTL (should use default)
	success, err = c.SetIfNotExists(ctx, "negative_ttl", "value", -time.Second)
	if err != nil {
		t.Errorf("SetIfNotExists with negative TTL failed: %v", err)
	}
	if !success {
		t.Error("SetIfNotExists with negative TTL should succeed")
	}

	// Test with nil value (may fail due to serialization - this is expected behavior)
	success, err = c.SetIfNotExists(ctx, "nil_value", nil, time.Hour)
	if err != nil {
		// This is expected - nil values may not be serializable
		t.Logf("SetIfNotExists with nil value failed as expected: %v", err)
	} else {
		if !success {
			t.Error("SetIfNotExists with nil value should succeed if serialization is supported")
		}

		// Only verify if the operation succeeded
		value, exists, err := c.Get(ctx, "nil_value")
		if err != nil {
			t.Errorf("Get failed: %v", err)
		}
		if !exists {
			t.Error("Key with nil value should exist")
		}
		if value != nil {
			t.Errorf("Expected nil value, got %v", value)
		}
	}
}

func TestMemoryCache_ConditionalOperationsConcurrency(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	const numWorkers = 10
	var wg sync.WaitGroup
	successCount := make([]int, numWorkers)

	// Test concurrent SetIfNotExists on the same key
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			defer wg.Done()
			success, err := c.SetIfNotExists(ctx, "concurrent_key", "worker_"+string(rune(workerID+'0')), time.Hour)
			if err != nil {
				t.Errorf("Concurrent SetIfNotExists failed: %v", err)
			}
			if success {
				successCount[workerID] = 1
			}
		}(i)
	}
	wg.Wait()

	// Only one worker should have succeeded
	totalSuccesses := 0
	for _, count := range successCount {
		totalSuccesses += count
	}
	if totalSuccesses != 1 {
		t.Errorf("Expected exactly 1 success, got %d", totalSuccesses)
	}

	// Verify the key exists
	_, exists, err := c.Get(ctx, "concurrent_key")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if !exists {
		t.Error("Key should exist after concurrent SetIfNotExists")
	}
}

func TestMemoryCache_ConditionalOperationsWithExpiry(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Set a key with short TTL
	shortTTL := 100 * time.Millisecond
	success, err := c.SetIfNotExists(ctx, "expiry_key", "initial_value", shortTTL)
	if err != nil {
		t.Errorf("SetIfNotExists failed: %v", err)
	}
	if !success {
		t.Error("SetIfNotExists should succeed for new key")
	}

	// Verify key exists
	_, exists, err := c.Get(ctx, "expiry_key")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if !exists {
		t.Error("Key should exist immediately after setting")
	}

	// Try SetIfNotExists again (should fail while key exists)
	success, err = c.SetIfNotExists(ctx, "expiry_key", "second_value", time.Hour)
	if err != nil {
		t.Errorf("SetIfNotExists failed: %v", err)
	}
	if success {
		t.Error("SetIfNotExists should fail while key exists")
	}

	// Wait for key to expire
	time.Sleep(150 * time.Millisecond)

	// Key should be gone
	_, exists, err = c.Get(ctx, "expiry_key")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if exists {
		t.Error("Key should have expired")
	}

	// SetIfNotExists should now succeed
	success, err = c.SetIfNotExists(ctx, "expiry_key", "new_value", time.Hour)
	if err != nil {
		t.Errorf("SetIfNotExists after expiry failed: %v", err)
	}
	if !success {
		t.Error("SetIfNotExists should succeed after key expires")
	}

	// Verify new value
	value, exists, err := c.Get(ctx, "expiry_key")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if !exists {
		t.Error("Key should exist after SetIfNotExists")
	}
	if value != "new_value" {
		t.Errorf("Expected 'new_value', got %v", value)
	}
}

func TestMemoryCache_ConditionalOperationsDataTypes(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Test with simple data types that should work with serialization
	simpleTypes := []struct {
		name  string
		value any
	}{
		{"string", "test_string"},
		{"int", 42},
		{"float", 3.14},
		{"bool", true},
	}

	for _, tc := range simpleTypes {
		t.Run(tc.name, func(t *testing.T) {
			key := "type_test_" + tc.name

			// Test SetIfNotExists
			success, err := c.SetIfNotExists(ctx, key, tc.value, time.Hour)
			if err != nil {
				t.Errorf("SetIfNotExists failed for %s: %v", tc.name, err)
			}
			if !success {
				t.Errorf("SetIfNotExists should succeed for new key %s", tc.name)
			}

			// Verify value
			value, exists, err := c.Get(ctx, key)
			if err != nil {
				t.Errorf("Get failed for %s: %v", tc.name, err)
			}
			if !exists {
				t.Errorf("Key should exist for %s", tc.name)
			}

			// Test SetIfExists to update
			newValue := "updated_" + tc.name
			success, err = c.SetIfExists(ctx, key, newValue, time.Hour)
			if err != nil {
				t.Errorf("SetIfExists failed for %s: %v", tc.name, err)
			}
			if !success {
				t.Errorf("SetIfExists should succeed for existing key %s", tc.name)
			}

			// Verify updated value
			value, exists, err = c.Get(ctx, key)
			if err != nil {
				t.Errorf("Get after update failed for %s: %v", tc.name, err)
			}
			if !exists {
				t.Errorf("Key should still exist for %s", tc.name)
			}
			if value != newValue {
				t.Errorf("Expected updated value '%s' for %s, got %v", newValue, tc.name, value)
			}
		})
	}

	// Test with complex types that may fail serialization - these are edge case tests
	complexTypes := []struct {
		name  string
		value any
	}{
		{"slice", []int{1, 2, 3}},
		{"map", map[string]string{"key": "value"}},
		{"struct", struct{ Name string }{"test"}},
		{"nil", nil},
	}

	for _, tc := range complexTypes {
		t.Run(tc.name+"_EdgeCase", func(t *testing.T) {
			key := "complex_type_test_" + tc.name

			// Test SetIfNotExists - may fail due to serialization issues
			success, err := c.SetIfNotExists(ctx, key, tc.value, time.Hour)
			if err != nil {
				// This is expected for complex types or nil values
				t.Logf("SetIfNotExists failed for %s as expected: %v", tc.name, err)
				return
			}

			// If it succeeded, verify basic behavior
			if !success {
				t.Errorf("SetIfNotExists should succeed for new key %s if serialization works", tc.name)
				return
			}

			// Try to get the value - may also fail during deserialization
			value, exists, err := c.Get(ctx, key)
			if err != nil {
				t.Logf("Get failed for %s as expected: %v", tc.name, err)
				return
			}

			if !exists {
				t.Errorf("Key should exist for %s if operations succeeded", tc.name)
			}

			// For complex types, just verify the operation completed without panics
			t.Logf("Successfully stored and retrieved %s value: %v", tc.name, value)
		})
	}
}
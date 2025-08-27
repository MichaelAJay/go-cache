package redis_test

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRedisCache_AtomicOperations(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Test Increment with new key
	result, err := c.Increment(ctx, "redis_counter1", 5, time.Hour)
	if err != nil {
		t.Errorf("Increment failed: %v", err)
	}
	if result != 5 {
		t.Errorf("Expected 5, got %d", result)
	}

	// Test Increment with existing key
	result, err = c.Increment(ctx, "redis_counter1", 3, time.Hour)
	if err != nil {
		t.Errorf("Increment failed: %v", err)
	}
	if result != 8 {
		t.Errorf("Expected 8, got %d", result)
	}

	// Test Decrement with existing key
	result, err = c.Decrement(ctx, "redis_counter1", 2, time.Hour)
	if err != nil {
		t.Errorf("Decrement failed: %v", err)
	}
	if result != 6 {
		t.Errorf("Expected 6, got %d", result)
	}

	// Test Decrement with new key
	result, err = c.Decrement(ctx, "redis_counter2", 10, time.Hour)
	if err != nil {
		t.Errorf("Decrement failed: %v", err)
	}
	if result != -10 {
		t.Errorf("Expected -10, got %d", result)
	}
}

func TestRedisCache_AtomicOperationsEdgeCases(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Test with empty key
	_, err := c.Increment(ctx, "", 1, time.Hour)
	if err == nil {
		t.Error("Expected error for empty key")
	}

	_, err = c.Decrement(ctx, "", 1, time.Hour)
	if err == nil {
		t.Error("Expected error for empty key")
	}

	// Test with zero delta
	result, err := c.Increment(ctx, "redis_zero_test", 0, time.Hour)
	if err != nil {
		t.Errorf("Increment with zero delta failed: %v", err)
	}
	if result != 0 {
		t.Errorf("Expected 0, got %d", result)
	}

	// Test with negative delta for increment
	result, err = c.Increment(ctx, "redis_negative_inc", -5, time.Hour)
	if err != nil {
		t.Errorf("Increment with negative delta failed: %v", err)
	}
	if result != -5 {
		t.Errorf("Expected -5, got %d", result)
	}

	// Test with canceled context
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, err = c.Increment(canceledCtx, "redis_canceled", 1, time.Hour)
	if err == nil {
		t.Error("Expected error with canceled context")
	}

	// Test with zero TTL (should use default)
	result, err = c.Increment(ctx, "redis_zero_ttl", 1, 0)
	if err != nil {
		t.Errorf("Increment with zero TTL failed: %v", err)
	}
	if result != 1 {
		t.Errorf("Expected 1, got %d", result)
	}

	// Test with existing non-numeric value
	err = c.Set(ctx, "redis_string_key", "not_a_number", time.Hour)
	if err != nil {
		t.Fatalf("Failed to set string value: %v", err)
	}

	_, err = c.Increment(ctx, "redis_string_key", 1, time.Hour)
	if err == nil {
		t.Error("Expected error when incrementing non-numeric value")
	}
}

func TestRedisCache_AtomicOperationsConcurrency(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	const numWorkers = 10
	const numOps = 50 // Reduced for Redis to avoid overwhelming the connection
	var wg sync.WaitGroup

	// Test concurrent increments on the same key
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				_, err := c.Increment(ctx, "redis_concurrent_counter", 1, time.Hour)
				if err != nil {
					t.Errorf("Concurrent Increment failed: %v", err)
				}
			}
		}()
	}
	wg.Wait()

	// Final value should be numWorkers * numOps
	expected := int64(numWorkers * numOps)
	value, exists, err := c.Get(ctx, "redis_concurrent_counter")
	if err != nil {
		t.Fatalf("Failed to get final counter value: %v", err)
	}
	if !exists {
		t.Fatal("Counter key should exist")
	}

	// Handle different integer types that might be returned
	var actualValue int64
	switch v := value.(type) {
	case int64:
		actualValue = v
	case int:
		actualValue = int64(v)
	case int32:
		actualValue = int64(v)
	case float64:
		actualValue = int64(v)
	default:
		t.Fatalf("Unexpected value type: %T, value: %v", value, value)
	}

	if actualValue != expected {
		t.Errorf("Expected final counter value %d, got %d", expected, actualValue)
	}
}

func TestRedisCache_ConditionalOperations(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Test SetIfNotExists on new key
	success, err := c.SetIfNotExists(ctx, "redis_new_key", "new_value", time.Hour)
	if err != nil {
		t.Errorf("SetIfNotExists failed: %v", err)
	}
	if !success {
		t.Error("SetIfNotExists should succeed for new key")
	}

	// Verify value was set
	value, exists, err := c.Get(ctx, "redis_new_key")
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
	success, err = c.SetIfNotExists(ctx, "redis_new_key", "updated_value", time.Hour)
	if err != nil {
		t.Errorf("SetIfNotExists failed: %v", err)
	}
	if success {
		t.Error("SetIfNotExists should fail for existing key")
	}

	// Test SetIfExists on existing key
	success, err = c.SetIfExists(ctx, "redis_new_key", "updated_value", time.Hour)
	if err != nil {
		t.Errorf("SetIfExists failed: %v", err)
	}
	if !success {
		t.Error("SetIfExists should succeed for existing key")
	}

	// Verify value was updated
	value, exists, err = c.Get(ctx, "redis_new_key")
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
	success, err = c.SetIfExists(ctx, "redis_nonexistent_key", "some_value", time.Hour)
	if err != nil {
		t.Errorf("SetIfExists failed: %v", err)
	}
	if success {
		t.Error("SetIfExists should fail for non-existing key")
	}
}

func TestRedisCache_ConditionalOperationsEdgeCases(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Test with empty key
	_, err := c.SetIfNotExists(ctx, "", "value", time.Hour)
	if err == nil {
		t.Error("Expected error for empty key")
	}

	_, err = c.SetIfExists(ctx, "", "value", time.Hour)
	if err == nil {
		t.Error("Expected error for empty key")
	}

	// Test with canceled context
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()

	_, err = c.SetIfNotExists(canceledCtx, "redis_key", "value", time.Hour)
	if err == nil {
		t.Error("Expected error with canceled context")
	}

	_, err = c.SetIfExists(canceledCtx, "redis_key", "value", time.Hour)
	if err == nil {
		t.Error("Expected error with canceled context")
	}

	// Test with zero TTL (should use default)
	success, err := c.SetIfNotExists(ctx, "redis_zero_ttl", "value", 0)
	if err != nil {
		t.Errorf("SetIfNotExists with zero TTL failed: %v", err)
	}
	if !success {
		t.Error("SetIfNotExists with zero TTL should succeed")
	}

	// Test with nil value
	success, err = c.SetIfNotExists(ctx, "redis_nil_value", nil, time.Hour)
	if err != nil {
		t.Errorf("SetIfNotExists with nil value failed: %v", err)
	}
	if !success {
		t.Error("SetIfNotExists with nil value should succeed")
	}

	// Verify nil value was stored
	value, exists, err := c.Get(ctx, "redis_nil_value")
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

func TestRedisCache_ConditionalOperationsConcurrency(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	const numWorkers = 10
	var wg sync.WaitGroup
	successCount := make([]int, numWorkers)

	// Test concurrent SetIfNotExists on the same key
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			defer wg.Done()
			success, err := c.SetIfNotExists(ctx, "redis_concurrent_key", "worker_"+string(rune(workerID+'0')), time.Hour)
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
	_, exists, err := c.Get(ctx, "redis_concurrent_key")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if !exists {
		t.Error("Key should exist after concurrent SetIfNotExists")
	}
}

func TestRedisCache_ConditionalOperationsWithExpiry(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Set a key with short TTL
	shortTTL := 2 * time.Second // Redis TTL precision is in seconds
	success, err := c.SetIfNotExists(ctx, "redis_expiry_key", "initial_value", shortTTL)
	if err != nil {
		t.Errorf("SetIfNotExists failed: %v", err)
	}
	if !success {
		t.Error("SetIfNotExists should succeed for new key")
	}

	// Try SetIfNotExists again (should fail while key exists)
	success, err = c.SetIfNotExists(ctx, "redis_expiry_key", "second_value", time.Hour)
	if err != nil {
		t.Errorf("SetIfNotExists failed: %v", err)
	}
	if success {
		t.Error("SetIfNotExists should fail while key exists")
	}

	// Wait for key to expire
	time.Sleep(3 * time.Second)

	// SetIfNotExists should now succeed
	success, err = c.SetIfNotExists(ctx, "redis_expiry_key", "new_value", time.Hour)
	if err != nil {
		t.Errorf("SetIfNotExists after expiry failed: %v", err)
	}
	if !success {
		t.Error("SetIfNotExists should succeed after key expires")
	}

	// Verify new value
	value, exists, err := c.Get(ctx, "redis_expiry_key")
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
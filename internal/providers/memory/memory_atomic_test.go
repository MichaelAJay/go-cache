package memory_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
)

func TestMemoryCache_AtomicOperations(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Test Increment with new key
	result, err := c.Increment(ctx, "counter1", 5, time.Hour)
	if err != nil {
		t.Errorf("Increment failed: %v", err)
	}
	if result != 5 {
		t.Errorf("Expected 5, got %d", result)
	}

	// Test Increment with existing key
	result, err = c.Increment(ctx, "counter1", 3, time.Hour)
	if err != nil {
		t.Errorf("Increment failed: %v", err)
	}
	if result != 8 {
		t.Errorf("Expected 8, got %d", result)
	}

	// Test Decrement with existing key
	result, err = c.Decrement(ctx, "counter1", 2, time.Hour)
	if err != nil {
		t.Errorf("Decrement failed: %v", err)
	}
	if result != 6 {
		t.Errorf("Expected 6, got %d", result)
	}

	// Test Decrement with new key
	result, err = c.Decrement(ctx, "counter2", 10, time.Hour)
	if err != nil {
		t.Errorf("Decrement failed: %v", err)
	}
	if result != -10 {
		t.Errorf("Expected -10, got %d", result)
	}
}

func TestMemoryCache_AtomicOperationsEdgeCases(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Test with empty key
	_, err := c.Increment(ctx, "", 1, time.Hour)
	if err != cache.ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey for empty key, got %v", err)
	}

	_, err = c.Decrement(ctx, "", 1, time.Hour)
	if err != cache.ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey for empty key, got %v", err)
	}

	// Test with zero delta
	result, err := c.Increment(ctx, "zero_test", 0, time.Hour)
	if err != nil {
		t.Errorf("Increment with zero delta failed: %v", err)
	}
	if result != 0 {
		t.Errorf("Expected 0, got %d", result)
	}

	// Test with negative delta for increment
	result, err = c.Increment(ctx, "negative_inc", -5, time.Hour)
	if err != nil {
		t.Errorf("Increment with negative delta failed: %v", err)
	}
	if result != -5 {
		t.Errorf("Expected -5, got %d", result)
	}

	// Test with negative delta for decrement
	result, err = c.Decrement(ctx, "negative_dec", -5, time.Hour)
	if err != nil {
		t.Errorf("Decrement with negative delta failed: %v", err)
	}
	if result != 5 {
		t.Errorf("Expected 5, got %d", result)
	}

	// Test with canceled context
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, err = c.Increment(canceledCtx, "canceled", 1, time.Hour)
	if err != cache.ErrContextCanceled {
		t.Errorf("Expected ErrContextCanceled, got %v", err)
	}

	// Test with zero TTL (should use default)
	result, err = c.Increment(ctx, "zero_ttl", 1, 0)
	if err != nil {
		t.Errorf("Increment with zero TTL failed: %v", err)
	}
	if result != 1 {
		t.Errorf("Expected 1, got %d", result)
	}

	// Test with existing non-numeric value
	err = c.Set(ctx, "string_key", "not_a_number", time.Hour)
	if err != nil {
		t.Fatalf("Failed to set string value: %v", err)
	}

	_, err = c.Increment(ctx, "string_key", 1, time.Hour)
	if err == nil {
		t.Error("Expected error when incrementing non-numeric value")
	}
}

func TestMemoryCache_AtomicOperationsConcurrency(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	const numWorkers = 10
	const numOps = 100
	var wg sync.WaitGroup

	// Test concurrent increments on the same key
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				_, err := c.Increment(ctx, "concurrent_counter", 1, time.Hour)
				if err != nil {
					t.Errorf("Concurrent Increment failed: %v", err)
				}
			}
		}()
	}
	wg.Wait()

	// Final value should be numWorkers * numOps
	expected := int64(numWorkers * numOps)
	value, exists, err := c.Get(ctx, "concurrent_counter")
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
	default:
		t.Fatalf("Unexpected value type: %T", value)
	}

	if actualValue != expected {
		t.Errorf("Expected final counter value %d, got %d", expected, actualValue)
	}
}

func TestMemoryCache_AtomicOperationsTTL(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Test that atomic operations respect TTL
	shortTTL := 100 * time.Millisecond
	result, err := c.Increment(ctx, "ttl_counter", 5, shortTTL)
	if err != nil {
		t.Errorf("Increment failed: %v", err)
	}
	if result != 5 {
		t.Errorf("Expected 5, got %d", result)
	}

	// Verify key exists
	_, exists, err := c.Get(ctx, "ttl_counter")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if !exists {
		t.Error("Key should exist immediately after increment")
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Key should be gone
	_, exists, err = c.Get(ctx, "ttl_counter")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if exists {
		t.Error("Key should have expired")
	}

	// Increment on expired key should start fresh
	result, err = c.Increment(ctx, "ttl_counter", 10, time.Hour)
	if err != nil {
		t.Errorf("Increment after expiry failed: %v", err)
	}
	if result != 10 {
		t.Errorf("Expected 10 (fresh start), got %d", result)
	}
}

func TestMemoryCache_AtomicOperationsOverflow(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Test near max int64 value
	const maxInt64 = 9223372036854775807
	const nearMax = maxInt64 - 10

	// Set counter to near max value
	_, err := c.Increment(ctx, "overflow_test", nearMax, time.Hour)
	if err != nil {
		t.Errorf("Increment to near max failed: %v", err)
	}

	// Small increment should work
	result, err := c.Increment(ctx, "overflow_test", 5, time.Hour)
	if err != nil {
		t.Errorf("Small increment failed: %v", err)
	}
	expected := int64(nearMax + 5)
	if result != expected {
		t.Errorf("Expected %d, got %d", expected, result)
	}

	// Test underflow with min int64
	const minInt64 = -9223372036854775808
	const nearMin = minInt64 + 10

	_, err = c.Increment(ctx, "underflow_test", nearMin, time.Hour)
	if err != nil {
		t.Errorf("Increment to near min failed: %v", err)
	}

	result, err = c.Decrement(ctx, "underflow_test", 5, time.Hour)
	if err != nil {
		t.Errorf("Small decrement failed: %v", err)
	}
	expected = int64(nearMin - 5)
	if result != expected {
		t.Errorf("Expected %d, got %d", expected, result)
	}
}
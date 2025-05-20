package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/providers/memory"
)

func TestMemoryCache_Has_Comprehensive(t *testing.T) {
	// Create a cache with a short cleanup interval
	c, err := memory.NewMemoryCache(&cache.CacheOptions{
		CleanupInterval: time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create memory cache: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	// Test 1: Empty key
	if c.Has(ctx, "") {
		t.Error("Expected Has to return false for empty key")
	}

	// Test 2: Canceled context
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	if c.Has(canceledCtx, "any-key") {
		t.Error("Expected Has to return false for canceled context")
	}

	// Test 3: Non-existent key
	if c.Has(ctx, "non-existent") {
		t.Error("Expected Has to return false for non-existent key")
	}

	// Test 4: Existing key
	err = c.Set(ctx, "existing", "value", time.Hour)
	if err != nil {
		t.Fatalf("Failed to set key: %v", err)
	}
	if !c.Has(ctx, "existing") {
		t.Error("Expected Has to return true for existing key")
	}

	// Test 5: Expired key (but still in cache)
	err = c.Set(ctx, "expired", "value", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to set key: %v", err)
	}

	// First verify it exists
	if !c.Has(ctx, "expired") {
		t.Error("Expected Has to return true for newly added key with TTL")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Now it should be considered expired
	if c.Has(ctx, "expired") {
		t.Error("Expected Has to return false for expired key")
	}

	// Test 6: Check that expired key was removed from cache during Has check
	// Getting the value directly from items map would require accessing unexported fields
	// Instead, let's try setting the same key and check there's no conflict
	err = c.Set(ctx, "expired", "new-value", time.Hour)
	if err != nil {
		t.Fatalf("Failed to set key that should have been removed: %v", err)
	}

	// Value should exist and be the new value
	value, exists, err := c.Get(ctx, "expired")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !exists {
		t.Error("Expected key to exist after re-adding")
	}
	if value != "new-value" {
		t.Errorf("Expected value 'new-value', got '%v'", value)
	}
}

func TestMemoryCache_Has_WithCleanup(t *testing.T) {
	// Create a cache with automatic cleanup
	c, err := memory.NewMemoryCache(&cache.CacheOptions{
		CleanupInterval: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to create memory cache: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	// Add a key with short TTL
	err = c.Set(ctx, "temp-key", "temp-value", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to set key: %v", err)
	}

	// Verify it exists
	if !c.Has(ctx, "temp-key") {
		t.Error("Expected key to exist immediately after setting")
	}

	// Wait for both TTL expiration and cleanup to run
	time.Sleep(300 * time.Millisecond)

	// Now the key should be gone due to the cleanup routine
	if c.Has(ctx, "temp-key") {
		t.Error("Expected key to be gone after cleanup")
	}

	// Add a key with zero TTL (should use default)
	err = c.Set(ctx, "default-ttl", "value", 0)
	if err != nil {
		t.Fatalf("Failed to set key: %v", err)
	}

	// Verify it exists and uses default TTL
	if !c.Has(ctx, "default-ttl") {
		t.Error("Expected key with default TTL to exist")
	}
}

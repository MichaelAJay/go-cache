package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/providers/memory"
)

// TestMemoryCache_GetMetadata_Comprehensive tests edge cases for the GetMetadata method
func TestMemoryCache_GetMetadata_Comprehensive(t *testing.T) {
	t.Run("EmptyKey", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		ctx := context.Background()

		// Empty key should return error
		_, err = c.GetMetadata(ctx, "")
		if err != cache.ErrInvalidKey {
			t.Errorf("Expected ErrInvalidKey for empty key, got: %v", err)
		}
	})

	t.Run("CanceledContext", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		// Create canceled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Should return error for canceled context
		_, err = c.GetMetadata(ctx, "any-key")
		if err != cache.ErrContextCanceled {
			t.Errorf("Expected ErrContextCanceled for canceled context, got: %v", err)
		}
	})

	t.Run("NonExistentKey", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		ctx := context.Background()

		// Non-existent key should return KeyNotFound error
		_, err = c.GetMetadata(ctx, "non-existent")
		if err != cache.ErrKeyNotFound {
			t.Errorf("Expected ErrKeyNotFound for non-existent key, got: %v", err)
		}
	})

	t.Run("ExpiredKey", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		ctx := context.Background()

		// Add key with short TTL
		err = c.Set(ctx, "expired-key", "value", 10*time.Millisecond)
		if err != nil {
			t.Fatalf("Failed to set key: %v", err)
		}

		// Initially it should have metadata
		initialMd, err := c.GetMetadata(ctx, "expired-key")
		if err != nil {
			t.Fatalf("GetMetadata failed for fresh key: %v", err)
		}
		if initialMd.Key != "expired-key" {
			t.Errorf("Expected key 'expired-key', got '%s'", initialMd.Key)
		}

		// Wait for the key to expire
		time.Sleep(20 * time.Millisecond)

		// After expiration, should return KeyNotFound
		_, err = c.GetMetadata(ctx, "expired-key")
		if err != cache.ErrKeyNotFound {
			t.Errorf("Expected ErrKeyNotFound for expired key, got: %v", err)
		}
	})

	t.Run("TTLCalculation", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		ctx := context.Background()

		// Add key with specific TTL
		ttl := time.Minute
		err = c.Set(ctx, "ttl-key", "value", ttl)
		if err != nil {
			t.Fatalf("Failed to set key: %v", err)
		}

		// Get metadata
		md, err := c.GetMetadata(ctx, "ttl-key")
		if err != nil {
			t.Fatalf("GetMetadata failed: %v", err)
		}

		// TTL should be close to what we set
		if md.TTL > ttl || md.TTL < ttl-5*time.Second {
			t.Errorf("Expected TTL close to %v, got %v", ttl, md.TTL)
		}

		// Check other metadata fields
		if md.CreatedAt.IsZero() {
			t.Error("CreatedAt should not be zero")
		}
		if md.Size <= 0 {
			t.Errorf("Size should be positive, got %d", md.Size)
		}
		if md.AccessCount != 0 {
			t.Errorf("Initial AccessCount should be 0, got %d", md.AccessCount)
		}

		// Access the key
		_, _, err = c.Get(ctx, "ttl-key")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		// Get updated metadata
		updatedMd, err := c.GetMetadata(ctx, "ttl-key")
		if err != nil {
			t.Fatalf("GetMetadata failed after access: %v", err)
		}

		// Access count should be updated
		if updatedMd.AccessCount != 1 {
			t.Errorf("Expected AccessCount 1 after access, got %d", updatedMd.AccessCount)
		}
		if updatedMd.LastAccessed.IsZero() {
			t.Error("LastAccessed should be set after access")
		}
	})

	t.Run("PermanentTTL", func(t *testing.T) {
		c, err := memory.NewMemoryCache(nil)
		if err != nil {
			t.Fatalf("Failed to create memory cache: %v", err)
		}
		defer c.Close()

		ctx := context.Background()

		// Set a key with no explicit expiration by using negative TTL
		err = c.Set(ctx, "permanent-key", "value", -1)
		if err != nil {
			t.Fatalf("Failed to set permanent key: %v", err)
		}

		// Get metadata
		md, err := c.GetMetadata(ctx, "permanent-key")
		if err != nil {
			t.Fatalf("GetMetadata failed for permanent key: %v", err)
		}

		// Should have default TTL
		if md.TTL <= 0 {
			t.Errorf("Expected positive TTL for permanent key, got %v", md.TTL)
		}
	})
}

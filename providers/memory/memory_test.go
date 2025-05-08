package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/providers/memory"
)

func TestMemoryCache_BasicOperations(t *testing.T) {
	ctx := context.Background()
	c := memory.NewMemoryCache(&cache.CacheOptions{})

	// Test Set and Get
	err := c.Set(ctx, "key1", "value1", 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	value, exists, err := c.Get(ctx, "key1")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if !exists {
		t.Error("Expected key to exist")
	}
	if value != "value1" {
		t.Errorf("Expected value 'value1', got '%v'", value)
	}

	// Test Delete
	err = c.Delete(ctx, "key1")
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	_, exists, _ = c.Get(ctx, "key1")
	if exists {
		t.Error("Expected key to be deleted")
	}
}

func TestMemoryCache_TTL(t *testing.T) {
	ctx := context.Background()
	c := memory.NewMemoryCache(&cache.CacheOptions{})

	// Set value with short TTL
	err := c.Set(ctx, "key1", "value1", 100*time.Millisecond)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Value should exist immediately
	_, exists, _ := c.Get(ctx, "key1")
	if !exists {
		t.Error("Expected key to exist immediately after setting")
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Value should be gone
	_, exists, _ = c.Get(ctx, "key1")
	if exists {
		t.Error("Expected key to be expired")
	}
}

func TestMemoryCache_MaxEntries(t *testing.T) {
	ctx := context.Background()
	c := memory.NewMemoryCache(&cache.CacheOptions{
		MaxEntries: 2,
	})

	// Set two values
	err := c.Set(ctx, "key1", "value1", 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}
	err = c.Set(ctx, "key2", "value2", 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Third value should fail
	err = c.Set(ctx, "key3", "value3", 0)
	if err != cache.ErrCacheFull {
		t.Errorf("Expected ErrCacheFull, got %v", err)
	}
}

func TestMemoryCache_MaxSize(t *testing.T) {
	ctx := context.Background()
	c := memory.NewMemoryCache(&cache.CacheOptions{
		MaxSize: 10,
	})

	// Set a large value
	err := c.Set(ctx, "key1", "this is a very long string", 0)
	if err != cache.ErrCacheFull {
		t.Errorf("Expected ErrCacheFull, got %v", err)
	}

	// Set a small value
	err = c.Set(ctx, "key1", "small", 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}
}

func TestMemoryCache_BulkOperations(t *testing.T) {
	ctx := context.Background()
	c := memory.NewMemoryCache(&cache.CacheOptions{})

	// Test SetMany
	items := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}
	err := c.SetMany(ctx, items, 0)
	if err != nil {
		t.Errorf("SetMany failed: %v", err)
	}

	// Test GetMany
	values, err := c.GetMany(ctx, []string{"key1", "key2", "key3"})
	if err != nil {
		t.Errorf("GetMany failed: %v", err)
	}
	if len(values) != 2 {
		t.Errorf("Expected 2 values, got %d", len(values))
	}
	if values["key1"] != "value1" || values["key2"] != "value2" {
		t.Error("Expected values to match")
	}

	// Test DeleteMany
	err = c.DeleteMany(ctx, []string{"key1", "key2"})
	if err != nil {
		t.Errorf("DeleteMany failed: %v", err)
	}

	values, _ = c.GetMany(ctx, []string{"key1", "key2"})
	if len(values) != 0 {
		t.Error("Expected all keys to be deleted")
	}
}

func TestMemoryCache_Metadata(t *testing.T) {
	ctx := context.Background()
	c := memory.NewMemoryCache(&cache.CacheOptions{})

	// Set value
	err := c.Set(ctx, "key1", "value1", time.Hour)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Get metadata
	metadata, err := c.GetMetadata(ctx, "key1")
	if err != nil {
		t.Errorf("GetMetadata failed: %v", err)
	}

	if metadata.Key != "key1" {
		t.Errorf("Expected key 'key1', got '%s'", metadata.Key)
	}
	if metadata.TTL != time.Hour {
		t.Errorf("Expected TTL 1h, got %v", metadata.TTL)
	}
	if metadata.Size == 0 {
		t.Error("Expected non-zero size")
	}
	if metadata.AccessCount != 0 {
		t.Errorf("Expected access count 0, got %d", metadata.AccessCount)
	}

	// Access the value
	_, _, _ = c.Get(ctx, "key1")

	// Check updated metadata
	metadata, _ = c.GetMetadata(ctx, "key1")
	if metadata.AccessCount != 1 {
		t.Errorf("Expected access count 1, got %d", metadata.AccessCount)
	}
}

func TestMemoryCache_Cleanup(t *testing.T) {
	ctx := context.Background()
	c := memory.NewMemoryCache(&cache.CacheOptions{
		CleanupInterval: 100 * time.Millisecond,
	})

	// Set value with short TTL
	err := c.Set(ctx, "key1", "value1", 100*time.Millisecond)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Wait for cleanup
	time.Sleep(250 * time.Millisecond)

	// Value should be gone
	_, exists, _ := c.Get(ctx, "key1")
	if exists {
		t.Error("Expected key to be cleaned up")
	}

	// Close the cache
	err = c.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

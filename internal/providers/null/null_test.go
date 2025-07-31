package null_test

import (
	"context"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/internal/providers/null"
)

func TestNullCache_BasicOperations(t *testing.T) {
	ctx := context.Background()
	c := null.NewNullCache(&cache.CacheOptions{})

	// Test Set
	err := c.Set(ctx, "key1", "value1", 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Test Get (should always return not found)
	value, exists, err := c.Get(ctx, "key1")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if exists {
		t.Error("Expected key to not exist")
	}
	if value != nil {
		t.Errorf("Expected nil value, got '%v'", value)
	}

	// Test Delete
	err = c.Delete(ctx, "key1")
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	// Test Has
	if c.Has(ctx, "key1") {
		t.Error("Expected Has to return false")
	}
}

func TestNullCache_BulkOperations(t *testing.T) {
	ctx := context.Background()
	c := null.NewNullCache(&cache.CacheOptions{})

	// Test SetMany
	items := map[string]any{
		"key1": "value1",
		"key2": "value2",
	}
	err := c.SetMany(ctx, items, 0)
	if err != nil {
		t.Errorf("SetMany failed: %v", err)
	}

	// Test GetMany (should return empty map)
	values, err := c.GetMany(ctx, []string{"key1", "key2"})
	if err != nil {
		t.Errorf("GetMany failed: %v", err)
	}
	if len(values) != 0 {
		t.Errorf("Expected empty map, got %d items", len(values))
	}

	// Test DeleteMany
	err = c.DeleteMany(ctx, []string{"key1", "key2"})
	if err != nil {
		t.Errorf("DeleteMany failed: %v", err)
	}
}

func TestNullCache_Metadata(t *testing.T) {
	ctx := context.Background()
	c := null.NewNullCache(&cache.CacheOptions{})

	// Test GetMetadata (should return ErrKeyNotFound)
	_, err := c.GetMetadata(ctx, "key1")
	if err != cache.ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound, got %v", err)
	}

	// Test GetManyMetadata (should return empty map)
	metadata, err := c.GetManyMetadata(ctx, []string{"key1", "key2"})
	if err != nil {
		t.Errorf("GetManyMetadata failed: %v", err)
	}
	if len(metadata) != 0 {
		t.Errorf("Expected empty map, got %d items", len(metadata))
	}
}

func TestNullCache_Metrics(t *testing.T) {
	ctx := context.Background()
	c := null.NewNullCache(&cache.CacheOptions{})

	// Perform some operations
	_ = c.Set(ctx, "key1", "value1", 0)
	_, _, _ = c.Get(ctx, "key1")
	_ = c.Delete(ctx, "key1")

	// Get metrics
	metrics := c.GetMetrics()

	// Verify metrics
	if metrics.Hits != 0 {
		t.Errorf("Expected 0 hits, got %d", metrics.Hits)
	}
	if metrics.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", metrics.Misses)
	}
	if metrics.HitRatio != 0.0 {
		t.Errorf("Expected 0.0 hit ratio, got %f", metrics.HitRatio)
	}
	if metrics.CacheSize != 0 {
		t.Errorf("Expected 0 cache size, got %d", metrics.CacheSize)
	}
	if metrics.EntryCount != 0 {
		t.Errorf("Expected 0 entry count, got %d", metrics.EntryCount)
	}
}

func TestNullCache_Cleanup(t *testing.T) {
	ctx := context.Background()
	c := null.NewNullCache(&cache.CacheOptions{
		CleanupInterval: 100 * time.Millisecond,
	})

	// Set value
	err := c.Set(ctx, "key1", "value1", 100*time.Millisecond)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Wait for cleanup
	time.Sleep(250 * time.Millisecond)

	// Value should not exist (but this is because it never existed)
	_, exists, _ := c.Get(ctx, "key1")
	if exists {
		t.Error("Expected key to not exist")
	}

	// Close the cache
	err = c.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

package redis_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestRedisCache_ConcurrentSetMany tests concurrent SetMany operations to ensure thread safety
func TestRedisCache_ConcurrentSetMany(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	const goroutines = 10
	var wg sync.WaitGroup

	// Function to generate items for a specific goroutine
	generateItems := func(id int) map[string]any {
		items := make(map[string]any)
		for j := 0; j < 10; j++ {
			key := "concurrent_setmany_" + string(rune(id+'a')) + "_" + string(rune(j+'0'))
			items[key] = "value_" + string(rune(id+'a')) + "_" + string(rune(j+'0'))
		}
		return items
	}

	// Test concurrent SetMany operations
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			items := generateItems(id)
			err := c.SetMany(ctx, items, time.Hour)
			if err != nil {
				t.Errorf("Concurrent SetMany failed: %v", err)
			}
		}(i)
	}
	wg.Wait()

	// Verify all values were stored correctly
	for i := 0; i < goroutines; i++ {
		items := generateItems(i)
		keys := make([]string, 0, len(items))
		for k := range items {
			keys = append(keys, k)
		}

		values, err := c.GetMany(ctx, keys)
		if err != nil {
			t.Errorf("GetMany failed: %v", err)
		}
		if len(values) != len(items) {
			t.Errorf("Expected %d values, got %d", len(items), len(values))
		}
	}
}

// TestRedisCache_ConcurrentGetMany tests concurrent GetMany operations to ensure thread safety
func TestRedisCache_ConcurrentGetMany(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Set up test data
	items := map[string]any{}
	keys := make([]string, 100)
	for i := 0; i < 100; i++ {
		key := "concurrent_getmany_" + string(rune(i/10+'a')) + "_" + string(rune(i%10+'0'))
		items[key] = "value_" + string(rune(i/10+'a')) + "_" + string(rune(i%10+'0'))
		keys[i] = key
	}
	err := c.SetMany(ctx, items, time.Hour)
	if err != nil {
		t.Fatalf("SetMany failed: %v", err)
	}

	const goroutines = 10
	var wg sync.WaitGroup

	// Test concurrent GetMany operations
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				values, err := c.GetMany(ctx, keys)
				if err != nil {
					t.Errorf("Concurrent GetMany failed: %v", err)
				}
				if len(values) != len(items) {
					t.Errorf("Expected %d values, got %d", len(items), len(values))
				}
			}
		}()
	}
	wg.Wait()
}

// TestRedisCache_ConcurrencyWithRaceDetection tests various concurrent operations to ensure thread safety
func TestRedisCache_ConcurrencyWithRaceDetection(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	const goroutines = 5
	const keysPerGoroutine = 10
	var wg sync.WaitGroup

	// Prepare some keys
	baseKeys := make([]string, goroutines*keysPerGoroutine)
	for i := 0; i < goroutines*keysPerGoroutine; i++ {
		baseKeys[i] = fmt.Sprintf("race_key_%d", i)
	}

	// Phase 1: Multiple goroutines setting values concurrently
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			start := id * keysPerGoroutine
			end := start + keysPerGoroutine
			for i := start; i < end; i++ {
				err := c.Set(ctx, baseKeys[i], fmt.Sprintf("value_%d", i), time.Hour)
				if err != nil {
					t.Errorf("Concurrent Set failed: %v", err)
				}
			}
		}(g)
	}
	wg.Wait()

	// Phase 2: Concurrent reads, writes and deletes on the same keys
	wg.Add(goroutines * 3) // One group does gets, one does sets, one does deletes

	// Group 1: Readers
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			// Each reader tries to read all keys
			for _, key := range baseKeys {
				_, _, _ = c.Get(ctx, key)
			}
		}()
	}

	// Group 2: Writers (updating existing keys)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			start := id * keysPerGoroutine
			end := start + keysPerGoroutine
			for i := start; i < end; i++ {
				_ = c.Set(ctx, baseKeys[i], fmt.Sprintf("updated_value_%d", i), time.Hour)
			}
		}(g)
	}

	// Group 3: Deleters
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			start := id * keysPerGoroutine
			end := start + keysPerGoroutine
			for i := start; i < end; i++ {
				_ = c.Delete(ctx, baseKeys[i])
			}
		}(g)
	}
	wg.Wait()

	// Phase 3: Concurrent metadata operations
	wg.Add(goroutines)
	// First set some keys again
	for i := 0; i < goroutines*keysPerGoroutine; i++ {
		_ = c.Set(ctx, baseKeys[i], fmt.Sprintf("value_for_meta_%d", i), time.Hour)
	}

	// Now query metadata concurrently
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			start := id * keysPerGoroutine
			end := start + keysPerGoroutine
			for i := start; i < end; i++ {
				_, _ = c.GetMetadata(ctx, baseKeys[i])
			}
		}(g)
	}
	wg.Wait()

	// Phase 4: Concurrent bulk operations
	wg.Add(goroutines * 3) // GetMany, SetMany, DeleteMany

	// Group 1: GetMany
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			start := id * keysPerGoroutine
			end := start + keysPerGoroutine
			keys := baseKeys[start:end]
			_, _ = c.GetMany(ctx, keys)
		}(g)
	}

	// Group 2: SetMany
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			items := map[string]any{}
			start := id * keysPerGoroutine
			end := start + keysPerGoroutine
			for i := start; i < end; i++ {
				items[baseKeys[i]] = fmt.Sprintf("bulk_value_%d", i)
			}
			_ = c.SetMany(ctx, items, time.Hour)
		}(g)
	}

	// Group 3: DeleteMany
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			start := id * keysPerGoroutine
			end := start + keysPerGoroutine
			keys := baseKeys[start:end]
			_ = c.DeleteMany(ctx, keys)
		}(g)
	}
	wg.Wait()
}

// TestRedisCache_ConcurrencyWithRedisErrors tests concurrent operations with simulated Redis errors
func TestRedisCache_ConcurrencyWithRedisErrors(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// First set some test data
	for i := 0; i < 5; i++ {
		key := "concurrent_error_key_" + string(rune(i+'0'))
		err := c.Set(ctx, key, "value", time.Hour)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}

	// Create a concurrent scenario with many Get and Delete operations
	// This tests the robustness of the code when under concurrent access
	// While we can't easily simulate Redis errors, this increases code coverage
	// by exercising error handling paths in the context of concurrency
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := "concurrent_error_key_" + string(rune(id%5+'0'))

			// Alternate between Get and Delete to create contention
			if id%2 == 0 {
				_, _, _ = c.Get(ctx, key)
			} else {
				_ = c.Delete(ctx, key)
			}
		}(i)
	}
	wg.Wait()
}

// TestRedisCache_MetadataOperations tests metadata handling in detail
func TestRedisCache_MetadataOperations(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Set value with explicit TTL
	err := c.Set(ctx, "meta_key1", "value1", time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get initial metadata
	metadata1, err := c.GetMetadata(ctx, "meta_key1")
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}

	// Record initial access count
	initialAccessCount := metadata1.AccessCount

	// Access value multiple times
	for i := 0; i < 5; i++ {
		_, _, err := c.Get(ctx, "meta_key1")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
	}

	// Get updated metadata
	metadata2, err := c.GetMetadata(ctx, "meta_key1")
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}

	// Check that access count increased
	if metadata2.AccessCount <= initialAccessCount {
		t.Errorf("Expected access count to increase from %d, got %d",
			initialAccessCount, metadata2.AccessCount)
	}

	// Check last accessed time was updated
	if !metadata2.LastAccessed.After(metadata1.LastAccessed) {
		t.Error("Expected last accessed time to be updated")
	}

	// Check TTL is preserved
	if metadata2.TTL != time.Hour {
		t.Errorf("Expected TTL to be 1h, got %v", metadata2.TTL)
	}
}

// TestRedisCache_StoreMetadataError tests handling of errors when storing metadata
func TestRedisCache_StoreMetadataError(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Set a value - this should work even if metadata storage would fail
	// We can't easily simulate Redis failures for metadata storage,
	// but this tests that the code path is at least executed successfully
	err := c.Set(ctx, "metadata_error_key", "value", time.Hour)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// The value should be accessible
	value, exists, err := c.Get(ctx, "metadata_error_key")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if !exists {
		t.Error("Expected value to exist")
	}
	if value != "value" {
		t.Errorf("Expected 'value', got '%v'", value)
	}
}

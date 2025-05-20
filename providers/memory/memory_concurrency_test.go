package memory_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/providers/memory"
)

// TestMemoryCache_ConcurrentAccess tests the memory cache under concurrent access
func TestMemoryCache_ConcurrentAccess(t *testing.T) {
	c, err := memory.NewMemoryCache(&cache.CacheOptions{
		MaxEntries: 10000, // Enough for our test
	})
	if err != nil {
		t.Fatalf("Failed to create memory cache: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	var wg sync.WaitGroup
	numWorkers := 10
	numOps := 100

	// Test concurrent reads and writes
	wg.Add(numWorkers * 2) // For both readers and writers

	// Create concurrent writers
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := fmt.Sprintf("key-%d-%d", workerID, j)
				value := fmt.Sprintf("value-%d-%d", workerID, j)
				err := c.Set(ctx, key, value, time.Minute)
				if err != nil {
					t.Errorf("Concurrent Set failed: %v", err)
				}
			}
		}(i)
	}

	// Create concurrent readers (reading keys that may or may not exist yet)
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				// Read in a pattern that will cause some cache hits and some misses
				readWorker := (workerID + 1) % numWorkers
				key := fmt.Sprintf("key-%d-%d", readWorker, j)
				_, _, err := c.Get(ctx, key)
				if err != nil {
					t.Errorf("Concurrent Get failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify all keys were written correctly
	for i := 0; i < numWorkers; i++ {
		for j := 0; j < numOps; j++ {
			key := fmt.Sprintf("key-%d-%d", i, j)
			expectedValue := fmt.Sprintf("value-%d-%d", i, j)
			value, exists, err := c.Get(ctx, key)
			if err != nil {
				t.Errorf("Verification Get failed: %v", err)
			}
			if !exists {
				t.Errorf("Key %s doesn't exist after concurrent operations", key)
			} else if value != expectedValue {
				t.Errorf("Key %s has value %v, expected %s", key, value, expectedValue)
			}
		}
	}
}

// TestMemoryCache_ConcurrentBulkOperations tests concurrent bulk operations
func TestMemoryCache_ConcurrentBulkOperations(t *testing.T) {
	c, err := memory.NewMemoryCache(&cache.CacheOptions{
		MaxEntries: 10000, // Enough for our test
	})
	if err != nil {
		t.Fatalf("Failed to create memory cache: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	var wg sync.WaitGroup
	numWorkers := 5
	numOps := 20

	// Test concurrent SetMany and GetMany
	wg.Add(numWorkers * 2) // For both bulk setters and getters

	// Create concurrent bulk setters
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				items := make(map[string]any, 5)
				// Each operation sets 5 keys
				for k := 0; k < 5; k++ {
					key := fmt.Sprintf("bulk-key-%d-%d-%d", workerID, j, k)
					value := fmt.Sprintf("bulk-value-%d-%d-%d", workerID, j, k)
					items[key] = value
				}
				err := c.SetMany(ctx, items, time.Minute)
				if err != nil {
					t.Errorf("Concurrent SetMany failed: %v", err)
				}
			}
		}(i)
	}

	// Create concurrent bulk getters (reading keys that may or may not exist yet)
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				// Read from the previous worker's keys (or wrap around)
				readWorker := (workerID + 1) % numWorkers
				keys := make([]string, 5)
				for k := 0; k < 5; k++ {
					keys[k] = fmt.Sprintf("bulk-key-%d-%d-%d", readWorker, j, k)
				}
				_, err := c.GetMany(ctx, keys)
				if err != nil {
					t.Errorf("Concurrent GetMany failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify some keys were written correctly
	for i := 0; i < numWorkers; i++ {
		for j := 0; j < numOps; j += 5 { // Check every 5th operation to save time
			for k := 0; k < 5; k++ {
				key := fmt.Sprintf("bulk-key-%d-%d-%d", i, j, k)
				expectedValue := fmt.Sprintf("bulk-value-%d-%d-%d", i, j, k)
				value, exists, err := c.Get(ctx, key)
				if err != nil {
					t.Errorf("Verification Get failed: %v", err)
				}
				if !exists {
					t.Errorf("Key %s doesn't exist after concurrent operations", key)
				} else if value != expectedValue {
					t.Errorf("Key %s has value %v, expected %s", key, value, expectedValue)
				}
			}
		}
	}
}

// TestMemoryCache_ConcurrentMetadata tests concurrent metadata operations
func TestMemoryCache_ConcurrentMetadata(t *testing.T) {
	c, err := memory.NewMemoryCache(&cache.CacheOptions{
		MaxEntries: 1000, // Enough for our test
	})
	if err != nil {
		t.Fatalf("Failed to create memory cache: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	var wg sync.WaitGroup
	numWorkers := 5
	numKeys := 20

	// First create all the keys we'll be testing
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("meta-key-%d", i)
		err := c.Set(ctx, key, fmt.Sprintf("meta-value-%d", i), time.Minute)
		if err != nil {
			t.Fatalf("Failed to set up test keys: %v", err)
		}
	}

	// Test concurrent GetMetadata and GetManyMetadata
	wg.Add(numWorkers * 2) // For both metadata accessors

	// Concurrent individual metadata accessors
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < numKeys; j++ {
				key := fmt.Sprintf("meta-key-%d", j)
				_, err := c.GetMetadata(ctx, key)
				if err != nil && err != cache.ErrKeyNotFound {
					t.Errorf("Concurrent GetMetadata failed: %v", err)
				}
			}
		}(i)
	}

	// Concurrent bulk metadata accessors
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			defer wg.Done()
			keys := make([]string, numKeys)
			for j := 0; j < numKeys; j++ {
				keys[j] = fmt.Sprintf("meta-key-%d", j)
			}
			_, err := c.GetManyMetadata(ctx, keys)
			if err != nil {
				t.Errorf("Concurrent GetManyMetadata failed: %v", err)
			}
		}(i)
	}

	wg.Wait()

	// No explicit verification needed here since we're just testing for race conditions
}

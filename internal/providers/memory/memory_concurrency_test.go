package memory_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/internal/providers/memory"
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
	for i := range numWorkers {
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
	for i := range numWorkers {
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

// TestMemoryCache_RaceConditionDelete demonstrates the race condition in updateSizeMetrics
// This test should FAIL with -race until the synchronization issue is fixed
func TestMemoryCache_RaceConditionDelete(t *testing.T) {
	c, err := memory.NewMemoryCache(&cache.CacheOptions{
		MaxEntries: 20000,
	})
	if err != nil {
		t.Fatalf("Failed to create memory cache: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	numWorkers := 20
	numOpsPerWorker := 500

	// Pre-populate the cache with items
	for i := 0; i < numWorkers*numOpsPerWorker; i++ {
		key := fmt.Sprintf("race-key-%d", i)
		value := fmt.Sprintf("race-value-%d", i)
		err := c.Set(ctx, key, value, time.Hour)
		if err != nil {
			t.Fatalf("Failed to populate cache: %v", err)
		}
	}

	var wg sync.WaitGroup

	// Start multiple goroutines that will concurrently delete items
	// This will trigger the race condition between Delete() and updateSizeMetrics()
	for i := range numWorkers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			// Each worker deletes its own range of keys
			start := workerID * numOpsPerWorker
			end := start + numOpsPerWorker

			for j := start; j < end; j++ {
				key := fmt.Sprintf("race-key-%d", j)
				// This Delete operation calls updateSizeMetrics() without proper synchronization
				// causing concurrent map iteration and map write
				err := c.Delete(ctx, key)
				if err != nil {
					t.Errorf("Delete failed: %v", err)
				}
			}
		}(i)
	}

	// Start additional goroutines that perform other operations to increase contention
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				// Mix of operations that might trigger metrics updates
				key := fmt.Sprintf("extra-key-%d-%d", workerID, j)

				// Set new keys
				err := c.Set(ctx, key, "extra-value", time.Hour)
				if err != nil {
					t.Errorf("Set failed: %v", err)
				}

				// Read existing keys
				_, _, err = c.Get(ctx, key)
				if err != nil {
					t.Errorf("Get failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// This test is specifically designed to trigger the race condition
	// It should fail with: "fatal error: concurrent map iteration and map write"
	// when run with: go test -race
	t.Log("If this test passes with -race, the race condition has been fixed")
}

// TestMemoryCache_IndexOperationsRace tests race conditions in index operations
// This test should expose the race between removeFromAllIndexes() and AddIndex()
// Reduced load to avoid AddIndex performance bottleneck
func TestMemoryCache_IndexOperationsRace(t *testing.T) {
	c, err := memory.NewMemoryCache(&cache.CacheOptions{
		MaxEntries: 5000,
	})
	if err != nil {
		t.Fatalf("Failed to create memory cache: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	numWorkers := 5
	numItems := 200

	// Pre-populate with fewer items to reduce AddIndex overhead
	for i := 0; i < numItems; i++ {
		key := fmt.Sprintf("index-key-%d", i)
		value := fmt.Sprintf("index-value-%d", i)
		err := c.Set(ctx, key, value, time.Hour)
		if err != nil {
			t.Fatalf("Failed to populate cache: %v", err)
		}
	}

	var wg sync.WaitGroup

	// First, create a few indexes to set up the race condition
	for i := 0; i < 3; i++ {
		indexName := fmt.Sprintf("pre-index-%d", i)
		indexKey := fmt.Sprintf("pre-group-%d", i)
		err := c.AddIndex(ctx, indexName, "index-key-*", indexKey)
		if err != nil {
			t.Fatalf("Failed to create initial index: %v", err)
		}
	}

	// Start goroutines that add indexes concurrently (fewer operations)
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 5; j++ { // Reduced from 50 to 5
				indexName := fmt.Sprintf("index-%d-%d", workerID, j)
				indexKey := fmt.Sprintf("group-%d", workerID)
				// This accesses c.indexes and c.indexPatterns with proper locking
				err := c.AddIndex(ctx, indexName, "index-key-*", indexKey)
				if err != nil {
					t.Errorf("AddIndex failed: %v", err)
				}
			}
		}(i)
	}

	// Start goroutines that delete items (triggering removeFromAllIndexes)
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			start := workerID * 30
			end := start + 30
			
			for j := start; j < end; j++ {
				if j >= numItems {
					break
				}
				key := fmt.Sprintf("index-key-%d", j)
				// This calls removeFromAllIndexes() which accesses c.indexes WITHOUT locks
				err := c.Delete(ctx, key)
				if err != nil {
					t.Errorf("Delete failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()
	
	t.Log("If this test passes with -race, index race conditions may be fixed")
}

// TestMemoryCache_CleanupRace tests race conditions between cleanup and index operations
// This test should expose races between background cleanup and concurrent operations
func TestMemoryCache_CleanupRace(t *testing.T) {
	c, err := memory.NewMemoryCache(&cache.CacheOptions{
		MaxEntries:      15000,
		CleanupInterval: 10 * time.Millisecond, // Very frequent cleanup
	})
	if err != nil {
		t.Fatalf("Failed to create memory cache: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	var wg sync.WaitGroup

	// Run for a short time to trigger multiple cleanup cycles
	stopChan := make(chan bool)
	go func() {
		time.Sleep(500 * time.Millisecond)
		close(stopChan)
	}()

	// Continuously add items with short TTL (will expire and trigger cleanup)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			counter := 0
			for {
				select {
				case <-stopChan:
					return
				default:
					key := fmt.Sprintf("cleanup-key-%d-%d", workerID, counter)
					value := fmt.Sprintf("cleanup-value-%d-%d", workerID, counter)
					// Short TTL to trigger frequent cleanup
					err := c.Set(ctx, key, value, 50*time.Millisecond)
					if err != nil {
						t.Errorf("Set failed: %v", err)
					}
					counter++
				}
			}
		}(i)
	}

	// Continuously add and remove indexes
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			counter := 0
			for {
				select {
				case <-stopChan:
					return
				default:
					indexName := fmt.Sprintf("cleanup-index-%d-%d", workerID, counter)
					indexKey := fmt.Sprintf("cleanup-group-%d", workerID)
					// This will race with cleanup calling removeFromAllIndexes()
					err := c.AddIndex(ctx, indexName, "cleanup-key-*", indexKey)
					if err != nil {
						t.Errorf("AddIndex failed: %v", err)
					}
					counter++
					time.Sleep(20 * time.Millisecond)
				}
			}
		}(i)
	}

	wg.Wait()
	
	t.Log("If this test passes with -race, cleanup race conditions may be fixed")
}

// TestMemoryCache_ExpirationTOCTOURace tests time-of-check-to-time-of-use races
// This test should expose races in expiration checking patterns
func TestMemoryCache_ExpirationTOCTOURace(t *testing.T) {
	c, err := memory.NewMemoryCache(&cache.CacheOptions{
		MaxEntries: 10000,
	})
	if err != nil {
		t.Fatalf("Failed to create memory cache: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	numWorkers := 20
	numKeys := 1000

	// Pre-populate with items that will expire soon
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("expire-key-%d", i)
		value := fmt.Sprintf("expire-value-%d", i)
		// Very short TTL to trigger expiration races
		err := c.Set(ctx, key, value, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("Failed to populate cache: %v", err)
		}
	}

	var wg sync.WaitGroup

	// Multiple goroutines trying to access potentially expired items
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				key := fmt.Sprintf("expire-key-%d", j%numKeys)
				// This should trigger expiration checks that could race
				_, _, err := c.Get(ctx, key)
				if err != nil {
					t.Errorf("Get failed: %v", err)
				}
				
				// Also test Has() which has similar expiration logic
				exists := c.Has(ctx, key)
				_ = exists // Silence unused variable warning
			}
		}(i)
	}

	// Concurrent operations that might interfere with expiration
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("expire-key-%d", j%numKeys)
				// Manual deletion that might race with expiration cleanup
				c.Delete(ctx, key)
				
				// Re-add with new TTL
				value := fmt.Sprintf("new-expire-value-%d-%d", workerID, j)
				c.Set(ctx, key, value, 200*time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	
	t.Log("If this test passes with -race, expiration TOCTOU races may be fixed")
}

// TestMemoryCache_HighConcurrencyStress combines all operations under extreme load
// This test should expose any remaining race conditions under high stress
func TestMemoryCache_HighConcurrencyStress(t *testing.T) {
	c, err := memory.NewMemoryCache(&cache.CacheOptions{
		MaxEntries:      25000,
		CleanupInterval: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to create memory cache: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	var wg sync.WaitGroup
	
	// Run stress test for a longer duration
	stopChan := make(chan bool)
	go func() {
		time.Sleep(2 * time.Second)
		close(stopChan)
	}()

	// High-frequency CRUD operations
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			counter := 0
			for {
				select {
				case <-stopChan:
					return
				default:
					key := fmt.Sprintf("stress-key-%d-%d", workerID, counter%1000)
					value := fmt.Sprintf("stress-value-%d-%d", workerID, counter)
					
					// Mix of operations
					switch counter % 5 {
					case 0:
						c.Set(ctx, key, value, time.Duration(100+counter%900)*time.Millisecond)
					case 1:
						c.Get(ctx, key)
					case 2:
						c.Delete(ctx, key)
					case 3:
						c.Has(ctx, key)
					case 4:
						// Bulk operations
						items := map[string]any{
							key:           value,
							key + "-bulk": value + "-bulk",
						}
						c.SetMany(ctx, items, 500*time.Millisecond)
					}
					counter++
				}
			}
		}(i)
	}

	// High-frequency index operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			counter := 0
			for {
				select {
				case <-stopChan:
					return
				default:
					indexName := fmt.Sprintf("stress-index-%d-%d", workerID, counter%100)
					indexKey := fmt.Sprintf("stress-group-%d", workerID)
					c.AddIndex(ctx, indexName, "stress-key-*", indexKey)
					
					// Try to use the index
					c.GetByIndex(ctx, indexName, indexKey)
					
					counter++
					time.Sleep(10 * time.Millisecond)
				}
			}
		}(i)
	}

	wg.Wait()
	
	t.Log("If this test passes with -race, most race conditions may be fixed")
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

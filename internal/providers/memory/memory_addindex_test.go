package memory_test

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/internal/providers/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupCacheWithItems creates a cache with the specified number of items
func setupCacheWithItems(count int) cache.Cache {
	c, err := memory.NewMemoryCache(&cache.CacheOptions{
		MaxEntries: count * 2, // Extra room to avoid cache full errors
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to create cache: %v", err))
	}

	ctx := context.Background()
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		err := c.Set(ctx, key, value, time.Hour)
		if err != nil {
			panic(fmt.Sprintf("Failed to set item %d: %v", i, err))
		}
	}

	return c
}

// BenchmarkAddIndex_ScaleTest measures performance across different cache sizes
func BenchmarkAddIndex_ScaleTest(b *testing.B) {
	cacheSizes := []int{100, 1000, 5000, 10000}
	
	for _, size := range cacheSizes {
		b.Run(fmt.Sprintf("CacheSize_%d", size), func(b *testing.B) {
			c := setupCacheWithItems(size)
			defer c.Close()
			
			ctx := context.Background()
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				indexName := fmt.Sprintf("bench-index-%d", i)
				err := c.AddIndex(ctx, indexName, "key-*", "group-1")
				if err != nil {
					b.Fatalf("AddIndex failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkAddIndex_PatternComplexity measures impact of pattern complexity
func BenchmarkAddIndex_PatternComplexity(b *testing.B) {
	patterns := []struct {
		name    string
		pattern string
	}{
		{"Simple", "key-*"},
		{"Multiple", "key-*-*"},
		{"Complex", "key-[0-9]*"},
		{"NoMatch", "nomatch-*"},
	}
	
	c := setupCacheWithItems(5000)
	defer c.Close()
	ctx := context.Background()
	
	for _, p := range patterns {
		b.Run(p.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				indexName := fmt.Sprintf("pattern-%s-%d", p.name, i)
				err := c.AddIndex(ctx, indexName, p.pattern, "test-group")
				if err != nil {
					b.Fatalf("AddIndex failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkAddIndex_IndexSize measures impact of existing index size
func BenchmarkAddIndex_IndexSize(b *testing.B) {
	indexSizes := []int{0, 100, 1000, 5000}
	
	for _, indexSize := range indexSizes {
		b.Run(fmt.Sprintf("ExistingIndexSize_%d", indexSize), func(b *testing.B) {
			c := setupCacheWithItems(10000)
			defer c.Close()
			
			ctx := context.Background()
			
			// Pre-populate with existing index entries
			if indexSize > 0 {
				err := c.AddIndex(ctx, "existing-index", "key-*", "existing-group")
				require.NoError(b, err)
			}
			
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				indexName := fmt.Sprintf("size-bench-%d", i)
				err := c.AddIndex(ctx, indexName, "key-*", "new-group")
				if err != nil {
					b.Fatalf("AddIndex failed: %v", err)
				}
			}
		})
	}
}

// TestAddIndex_MemoryUsage ensures memory usage is reasonable
func TestAddIndex_MemoryUsage(t *testing.T) {
	var memBefore, memAfter runtime.MemStats
	
	c := setupCacheWithItems(10000)
	defer c.Close()
	
	ctx := context.Background()
	
	runtime.GC()
	runtime.ReadMemStats(&memBefore)
	
	// Add multiple indexes
	for i := 0; i < 100; i++ {
		indexName := fmt.Sprintf("memory-test-%d", i)
		err := c.AddIndex(ctx, indexName, "key-*", fmt.Sprintf("group-%d", i%10))
		require.NoError(t, err)
	}
	
	runtime.GC()
	runtime.ReadMemStats(&memAfter)
	
	memIncrease := memAfter.Alloc - memBefore.Alloc
	maxAcceptable := uint64(50 * 1024 * 1024) // 50MB max
	
	t.Logf("Memory increase: %d bytes (%.2f MB)", memIncrease, float64(memIncrease)/(1024*1024))
	
	if memIncrease > maxAcceptable {
		t.Errorf("Memory usage too high: %d bytes (max: %d)", memIncrease, maxAcceptable)
	}
}

// TestAddIndex_LockContention measures lock contention impact
func TestAddIndex_LockContention(t *testing.T) {
	c := setupCacheWithItems(5000)
	defer c.Close()
	
	ctx := context.Background()
	
	var (
		addIndexDuration  time.Duration
		getOperationCount int64
		setOperationCount int64
		testDuration      = 2 * time.Second
	)
	
	// Start background read operations
	go func() {
		startTime := time.Now()
		for time.Since(startTime) < testDuration {
			start := time.Now()
			_, _, _ = c.Get(ctx, fmt.Sprintf("key-%d", time.Now().Nanosecond()%100))
			if time.Since(start) < 100*time.Millisecond { // Only count if not severely blocked
				atomic.AddInt64(&getOperationCount, 1)
			}
		}
	}()
	
	// Start background write operations
	go func() {
		startTime := time.Now()
		counter := 0
		for time.Since(startTime) < testDuration {
			start := time.Now()
			_ = c.Set(ctx, fmt.Sprintf("new-key-%d", counter), "value", time.Hour)
			if time.Since(start) < 100*time.Millisecond { // Only count if not severely blocked
				atomic.AddInt64(&setOperationCount, 1)
			}
			counter++
		}
	}()
	
	// Give background operations time to start
	time.Sleep(100 * time.Millisecond)
	
	// Measure AddIndex duration
	start := time.Now()
	err := c.AddIndex(ctx, "contention-test", "key-*", "test-group")
	addIndexDuration = time.Since(start)
	
	require.NoError(t, err)
	
	// Wait for background operations to complete
	time.Sleep(testDuration - time.Since(start) + 100*time.Millisecond)
	
	t.Logf("AddIndex duration: %v", addIndexDuration)
	t.Logf("Concurrent Get operations: %d", getOperationCount)
	t.Logf("Concurrent Set operations: %d", setOperationCount)
	
	// Assert acceptable performance
	assert.Less(t, addIndexDuration, 100*time.Millisecond, "AddIndex took too long")
	assert.Greater(t, getOperationCount, int64(100), "Too many Gets were blocked")
	assert.Greater(t, setOperationCount, int64(100), "Too many Sets were blocked")
}

// TestAddIndex_ConsistencyUnderConcurrency ensures index consistency
func TestAddIndex_ConsistencyUnderConcurrency(t *testing.T) {
	c := setupCacheWithItems(1000)
	defer c.Close()
	
	ctx := context.Background()
	var wg sync.WaitGroup
	
	// Concurrent AddIndex operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				indexName := fmt.Sprintf("consistency-%d-%d", worker, j)
				err := c.AddIndex(ctx, indexName, "key-*", fmt.Sprintf("group-%d", worker))
				assert.NoError(t, err)
			}
		}(i)
	}
	
	// Concurrent cache operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("concurrent-key-%d-%d", worker, j)
				_ = c.Set(ctx, key, "value", time.Hour)
				_ = c.Delete(ctx, key)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify index consistency
	for i := 0; i < 10; i++ {
		for j := 0; j < 5; j++ {
			indexName := fmt.Sprintf("consistency-%d-%d", i, j)
			keys, err := c.GetByIndex(ctx, indexName, fmt.Sprintf("group-%d", i))
			assert.NoError(t, err)
			
			// Verify all returned keys actually exist and match pattern
			for _, key := range keys {
				exists := c.Has(ctx, key)
				assert.True(t, exists, "Index references non-existent key: %s", key)
				
				// Verify key matches pattern
				assert.True(t, len(key) > 4 && key[:4] == "key-", "Key doesn't match pattern: %s", key)
			}
		}
	}
}

// TestAddIndex_OptimizationRegression ensures optimizations don't break functionality
func TestAddIndex_OptimizationRegression(t *testing.T) {
	testCases := []struct {
		name        string
		cacheSize   int
		pattern     string
		expectCount int
	}{
		{"Small cache simple pattern", 100, "key-*", 100},
		{"Large cache complex pattern", 5000, "session:*:*", 0}, // No matching keys
		{"Medium cache partial match", 1000, "key-5*", 111},     // key-5, key-50-59, key-500-599
		{"Edge case empty pattern", 500, "", 0},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := setupCacheWithItems(tc.cacheSize)
			defer c.Close()
			
			ctx := context.Background()
			
			err := c.AddIndex(ctx, "regression-test", tc.pattern, "test-group")
			assert.NoError(t, err)
			
			keys, err := c.GetByIndex(ctx, "regression-test", "test-group")
			assert.NoError(t, err)
			assert.Len(t, keys, tc.expectCount, "Unexpected number of keys for pattern %s", tc.pattern)
		})
	}
}

// TestAddIndex_DuplicateHandling tests duplicate key handling
func TestAddIndex_DuplicateHandling(t *testing.T) {
	c := setupCacheWithItems(100)
	defer c.Close()
	
	ctx := context.Background()
	
	// Add the same index multiple times
	for i := 0; i < 5; i++ {
		err := c.AddIndex(ctx, "duplicate-test", "key-*", "test-group")
		require.NoError(t, err)
	}
	
	// Verify no duplicates in the index
	keys, err := c.GetByIndex(ctx, "duplicate-test", "test-group")
	require.NoError(t, err)
	
	// Check for duplicates
	seen := make(map[string]bool)
	for _, key := range keys {
		if seen[key] {
			t.Errorf("Duplicate key found in index: %s", key)
		}
		seen[key] = true
	}
	
	// Should have exactly 100 unique keys
	assert.Len(t, keys, 100)
}

// TestAddIndex_EmptyCache tests AddIndex on empty cache
func TestAddIndex_EmptyCache(t *testing.T) {
	c, err := memory.NewMemoryCache(&cache.CacheOptions{
		MaxEntries: 1000,
	})
	require.NoError(t, err)
	defer c.Close()
	
	ctx := context.Background()
	
	err = c.AddIndex(ctx, "empty-test", "key-*", "test-group")
	require.NoError(t, err)
	
	keys, err := c.GetByIndex(ctx, "empty-test", "test-group")
	require.NoError(t, err)
	assert.Len(t, keys, 0)
}

// TestAddIndex_ContextCancellation tests context cancellation handling
func TestAddIndex_ContextCancellation(t *testing.T) {
	c := setupCacheWithItems(1000)
	defer c.Close()
	
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	
	err := c.AddIndex(ctx, "cancelled-test", "key-*", "test-group")
	assert.Error(t, err)
	// Check for either "context" or "canceled" in the error message
	errMsg := err.Error()
	assert.True(t, 
		strings.Contains(errMsg, "context") || strings.Contains(errMsg, "canceled") || strings.Contains(errMsg, "cancelled"),
		"Error should indicate cancellation, got: %s", errMsg)
}

// TestAddIndex_ChunkedProcessing tests chunked processing for large datasets
func TestAddIndex_ChunkedProcessing(t *testing.T) {
	// Create cache with > 5000 items to trigger chunked processing
	c := setupCacheWithItems(6000)
	defer c.Close()
	
	ctx := context.Background()
	
	start := time.Now()
	err := c.AddIndex(ctx, "chunked-test", "key-*", "large-group")
	duration := time.Since(start)
	
	require.NoError(t, err)
	
	// Verify the index was created correctly
	keys, err := c.GetByIndex(ctx, "chunked-test", "large-group")
	require.NoError(t, err)
	assert.Len(t, keys, 6000, "Should index all 6000 keys")
	
	t.Logf("Chunked processing took: %v", duration)
	
	// Should be reasonably fast even for large datasets
	assert.Less(t, duration, 50*time.Millisecond, "Chunked processing should be fast")
}

// TestAddIndex_ChunkedVsDirect compares performance of chunked vs direct processing
func TestAddIndex_ChunkedVsDirect(t *testing.T) {
	// Test both below and above threshold
	testCases := []struct {
		name     string
		size     int
		expected string
	}{
		{"Direct processing", 4000, "Should use direct processing"},
		{"Chunked processing", 6000, "Should use chunked processing"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := setupCacheWithItems(tc.size)
			defer c.Close()
			
			ctx := context.Background()
			
			start := time.Now()
			err := c.AddIndex(ctx, "comparison-test", "key-*", "test-group")
			duration := time.Since(start)
			
			require.NoError(t, err)
			
			// Verify correctness
			keys, err := c.GetByIndex(ctx, "comparison-test", "test-group")
			require.NoError(t, err)
			assert.Len(t, keys, tc.size)
			
			t.Logf("%s (size=%d) took: %v", tc.name, tc.size, duration)
		})
	}
}

// TestAddIndex_ConcurrentModificationDuringChunked tests that chunked processing handles concurrent modifications
func TestAddIndex_ConcurrentModificationDuringChunked(t *testing.T) {
	// Create large cache to trigger chunked processing
	c := setupCacheWithItems(8000)
	defer c.Close()
	
	ctx := context.Background()
	var wg sync.WaitGroup
	
	// Start AddIndex operation in background
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := c.AddIndex(ctx, "concurrent-chunked", "key-*", "modified-group")
		assert.NoError(t, err)
	}()
	
	// Concurrently modify cache during index creation
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Millisecond) // Let AddIndex start
		
		// Delete some keys
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("key-%d", i)
			c.Delete(ctx, key)
		}
		
		// Add some new keys
		for i := 8000; i < 8100; i++ {
			key := fmt.Sprintf("key-%d", i)
			c.Set(ctx, key, "new-value", time.Hour)
		}
	}()
	
	wg.Wait()
	
	// Verify index was created and is consistent
	keys, err := c.GetByIndex(ctx, "concurrent-chunked", "modified-group")
	require.NoError(t, err)
	
	// Should have most keys (some deleted, some added)
	assert.GreaterOrEqual(t, len(keys), 7900, "Should have most keys indexed")
	assert.LessOrEqual(t, len(keys), 8100, "Should not exceed original + new keys")
	
	// All indexed keys should actually exist in cache
	for _, key := range keys {
		exists := c.Has(ctx, key)
		assert.True(t, exists, "Indexed key should exist: %s", key)
	}
}
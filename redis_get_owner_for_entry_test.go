//go:build integration

package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/MichaelAJay/go-metrics/metric"
	"github.com/go-redis/redis/v8"
)

type TestEntryForOwner struct {
	ID      string
	OwnerID string
	Value   string
}

func setupTestCacheForOwner(t *testing.T, indexingMode bool) (interfaces.Cache[TestEntryForOwner], *redis.Client, func()) {
	t.Helper()

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use DB 1 for tests
	})

	// Clear test database
	ctx := context.Background()
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("failed to flush test database: %v", err)
	}

	// Create extractor
	extractor := &IndexExtractor[TestEntryForOwner]{
		GetEntryKey: func(e TestEntryForOwner) string {
			return e.ID
		},
		GetOwnerKey: func(e TestEntryForOwner) string {
			return e.OwnerID
		},
	}

	// Create cache with indexing
	registry := metric.NewDefaultRegistry()
	cache, err := NewCache[TestEntryForOwner](
		ctx,
		client,
		indexingMode,
		extractor,
		10, // warmPoolCount
		WithGoMetrics[TestEntryForOwner](registry, nil),
		WithRedisOptions[TestEntryForOwner](&RedisOptions{
			DataPrefix:  "test:data:",
			IndexPrefix: "test:index:",
			MetaPrefix:  "test:meta:",
			LockPrefix:  "test:lock:",
		}),
	)

	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	cleanup := func() {
		client.FlushDB(ctx)
		client.Close()
	}

	return cache, client, cleanup
}

// TestGetOwnerForEntry_Basic tests the basic functionality of GetOwnerForEntry
func TestGetOwnerForEntry_Basic(t *testing.T) {
	cache, _, cleanup := setupTestCacheForOwner(t, true)
	defer cleanup()

	ctx := context.Background()

	// Create test entry
	entry := TestEntryForOwner{
		ID:      "entry123",
		OwnerID: "owner456",
		Value:   "test value",
	}

	// Set the entry
	err := cache.Set(ctx, entry, 5*time.Minute)
	if err != nil {
		t.Fatalf("failed to set entry: %v", err)
	}

	// Test GetOwnerForEntry - should return the owner
	ownerKey, found, err := cache.GetOwnerForEntry(ctx, "entry123")
	if err != nil {
		t.Fatalf("GetOwnerForEntry failed: %v", err)
	}
	if !found {
		t.Fatal("expected entry to be found")
	}
	if ownerKey != "owner456" {
		t.Fatalf("expected ownerKey 'owner456', got '%s'", ownerKey)
	}

	// Test GetOwnerForEntry for non-existent entry - should return false
	ownerKey, found, err = cache.GetOwnerForEntry(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetOwnerForEntry failed for non-existent entry: %v", err)
	}
	if found {
		t.Fatal("expected entry not to be found")
	}
	if ownerKey != "" {
		t.Fatalf("expected empty ownerKey, got '%s'", ownerKey)
	}
}

// TestGetOwnerForEntry_RequiresIndexing tests that GetOwnerForEntry requires indexing
func TestGetOwnerForEntry_RequiresIndexing(t *testing.T) {
	cache, _, cleanup := setupTestCacheForOwner(t, false)
	defer cleanup()

	ctx := context.Background()

	// Test GetOwnerForEntry without indexing - should return error
	_, _, err := cache.GetOwnerForEntry(ctx, "entry123")
	if err == nil {
		t.Fatal("expected error when indexing is disabled")
	}
	if err.Error() != "GetOwnerForEntry requires indexing to be enabled" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// TestGetOwnerForEntry_AfterDelete tests that GetOwnerForEntry returns not found after deletion
func TestGetOwnerForEntry_AfterDelete(t *testing.T) {
	cache, _, cleanup := setupTestCacheForOwner(t, true)
	defer cleanup()

	ctx := context.Background()

	// Create and set test entry
	entry := TestEntryForOwner{
		ID:      "entry789",
		OwnerID: "owner999",
		Value:   "test value",
	}

	err := cache.Set(ctx, entry, 5*time.Minute)
	if err != nil {
		t.Fatalf("failed to set entry: %v", err)
	}

	// Verify entry exists
	ownerKey, found, err := cache.GetOwnerForEntry(ctx, "entry789")
	if err != nil {
		t.Fatalf("GetOwnerForEntry failed: %v", err)
	}
	if !found || ownerKey != "owner999" {
		t.Fatal("expected entry to be found before deletion")
	}

	// Delete the entry
	_, err = cache.Delete(ctx, "entry789")
	if err != nil {
		t.Fatalf("failed to delete entry: %v", err)
	}

	// Verify GetOwnerForEntry returns not found
	ownerKey, found, err = cache.GetOwnerForEntry(ctx, "entry789")
	if err != nil {
		t.Fatalf("GetOwnerForEntry failed after deletion: %v", err)
	}
	if found {
		t.Fatal("expected entry not to be found after deletion")
	}
	if ownerKey != "" {
		t.Fatalf("expected empty ownerKey after deletion, got '%s'", ownerKey)
	}
}

// TestGetOwnerForEntry_ConcurrentAccess tests concurrent access to GetOwnerForEntry
func TestGetOwnerForEntry_ConcurrentAccess(t *testing.T) {
	cache, _, cleanup := setupTestCacheForOwner(t, true)
	defer cleanup()

	ctx := context.Background()

	// Create multiple test entries
	numEntries := 100
	for i := 0; i < numEntries; i++ {
		entry := TestEntryForOwner{
			ID:      fmt.Sprintf("entry%d", i),
			OwnerID: fmt.Sprintf("owner%d", i%10), // 10 different owners
			Value:   fmt.Sprintf("value%d", i),
		}
		err := cache.Set(ctx, entry, 5*time.Minute)
		if err != nil {
			t.Fatalf("failed to set entry %d: %v", i, err)
		}
	}

	// Concurrent reads
	var wg sync.WaitGroup
	errChan := make(chan error, numEntries)

	for i := 0; i < numEntries; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			entryKey := fmt.Sprintf("entry%d", idx)
			expectedOwner := fmt.Sprintf("owner%d", idx%10)

			ownerKey, found, err := cache.GetOwnerForEntry(ctx, entryKey)
			if err != nil {
				errChan <- fmt.Errorf("GetOwnerForEntry failed for %s: %v", entryKey, err)
				return
			}
			if !found {
				errChan <- fmt.Errorf("expected entry %s to be found", entryKey)
				return
			}
			if ownerKey != expectedOwner {
				errChan <- fmt.Errorf("expected owner %s for entry %s, got %s", expectedOwner, entryKey, ownerKey)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		t.Error(err)
	}
}

// TestGetOwnerForEntry_TTLExpiration tests behavior when the reverse index TTL expires
func TestGetOwnerForEntry_TTLExpiration(t *testing.T) {
	cache, client, cleanup := setupTestCacheForOwner(t, true)
	defer cleanup()

	ctx := context.Background()

	// Create test entry with short TTL
	entry := TestEntryForOwner{
		ID:      "short_ttl_entry",
		OwnerID: "short_ttl_owner",
		Value:   "test value",
	}

	// Set with very short TTL (1 second)
	err := cache.Set(ctx, entry, 1*time.Second)
	if err != nil {
		t.Fatalf("failed to set entry: %v", err)
	}

	// Verify entry exists
	ownerKey, found, err := cache.GetOwnerForEntry(ctx, "short_ttl_entry")
	if err != nil {
		t.Fatalf("GetOwnerForEntry failed: %v", err)
	}
	if !found || ownerKey != "short_ttl_owner" {
		t.Fatal("expected entry to be found before TTL expiration")
	}

	// Wait for TTL to expire
	time.Sleep(2 * time.Second)

	// Verify GetOwnerForEntry returns not found after TTL expiration
	ownerKey, found, err = cache.GetOwnerForEntry(ctx, "short_ttl_entry")
	if err != nil {
		t.Fatalf("GetOwnerForEntry failed after TTL expiration: %v", err)
	}
	if found {
		t.Fatal("expected entry not to be found after TTL expiration")
	}
	if ownerKey != "" {
		t.Fatalf("expected empty ownerKey after TTL expiration, got '%s'", ownerKey)
	}

	// Verify the reverse index key is actually gone from Redis
	redisCache := cache.(*RedisCache[TestEntryForOwner])
	reverseKey := redisCache.buildReverseIndexKey("short_ttl_entry")
	exists, err := client.Exists(ctx, reverseKey).Result()
	if err != nil {
		t.Fatalf("failed to check reverse key existence: %v", err)
	}
	if exists > 0 {
		t.Fatal("expected reverse index key to be expired")
	}
}

// TestGetOwnerForEntry_WithSetGetDeleteOperations tests integration with Set/Get/Delete
func TestGetOwnerForEntry_WithSetGetDeleteOperations(t *testing.T) {
	cache, _, cleanup := setupTestCacheForOwner(t, true)
	defer cleanup()

	ctx := context.Background()

	// Create multiple entries for the same owner
	owner := "shared_owner"
	entries := []TestEntryForOwner{
		{ID: "entry1", OwnerID: owner, Value: "value1"},
		{ID: "entry2", OwnerID: owner, Value: "value2"},
		{ID: "entry3", OwnerID: owner, Value: "value3"},
	}

	// Set all entries
	for _, entry := range entries {
		err := cache.Set(ctx, entry, 5*time.Minute)
		if err != nil {
			t.Fatalf("failed to set entry %s: %v", entry.ID, err)
		}
	}

	// Verify all entries have the correct owner
	for _, entry := range entries {
		ownerKey, found, err := cache.GetOwnerForEntry(ctx, entry.ID)
		if err != nil {
			t.Fatalf("GetOwnerForEntry failed for %s: %v", entry.ID, err)
		}
		if !found || ownerKey != owner {
			t.Fatalf("expected owner %s for entry %s, got %s (found: %v)", owner, entry.ID, ownerKey, found)
		}
	}

	// Verify GetByOwner returns all entries
	ownerEntries, err := cache.GetByOwner(ctx, owner)
	if err != nil {
		t.Fatalf("GetByOwner failed: %v", err)
	}
	if len(ownerEntries) != 3 {
		t.Fatalf("expected 3 entries for owner, got %d", len(ownerEntries))
	}

	// Delete one entry
	_, err = cache.Delete(ctx, "entry2")
	if err != nil {
		t.Fatalf("failed to delete entry: %v", err)
	}

	// Verify deleted entry has no owner
	ownerKey, found, err := cache.GetOwnerForEntry(ctx, "entry2")
	if err != nil {
		t.Fatalf("GetOwnerForEntry failed for deleted entry: %v", err)
	}
	if found {
		t.Fatal("expected deleted entry not to be found")
	}

	// Verify other entries still have the owner
	ownerKey, found, err = cache.GetOwnerForEntry(ctx, "entry1")
	if err != nil {
		t.Fatalf("GetOwnerForEntry failed for entry1: %v", err)
	}
	if !found || ownerKey != owner {
		t.Fatal("expected entry1 to still have owner after entry2 deletion")
	}
}

// TestGetOwnerForEntry_CircuitBreakerHandling tests circuit breaker behavior
func TestGetOwnerForEntry_CircuitBreakerHandling(t *testing.T) {
	cache, client, cleanup := setupTestCacheForOwner(t, true)
	defer cleanup()

	ctx := context.Background()

	// Create test entry
	entry := TestEntryForOwner{
		ID:      "circuit_test",
		OwnerID: "circuit_owner",
		Value:   "test value",
	}

	err := cache.Set(ctx, entry, 5*time.Minute)
	if err != nil {
		t.Fatalf("failed to set entry: %v", err)
	}

	// Close the Redis connection to trigger errors
	client.Close()

	// Try to get owner - should trigger circuit breaker eventually
	redisCache := cache.(*RedisCache[TestEntryForOwner])
	for i := 0; i < circuitBreakerThreshold+1; i++ {
		_, _, _ = cache.GetOwnerForEntry(ctx, "circuit_test")
	}

	// Verify circuit breaker is open
	if !redisCache.isCircuitBreakerOpen() {
		t.Fatal("expected circuit breaker to be open after multiple failures")
	}

	// Try GetOwnerForEntry with circuit breaker open - should return circuit breaker error
	_, _, err = cache.GetOwnerForEntry(ctx, "circuit_test")
	if err == nil {
		t.Fatal("expected error when circuit breaker is open")
	}
	if err.Error() != "cache: circuit breaker is open" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestGetOwnerForEntry_MultipleOwners tests with entries having different owners
func TestGetOwnerForEntry_MultipleOwners(t *testing.T) {
	cache, _, cleanup := setupTestCacheForOwner(t, true)
	defer cleanup()

	ctx := context.Background()

	// Create entries with different owners
	testCases := []struct {
		entryID string
		ownerID string
	}{
		{"entry_a", "owner_1"},
		{"entry_b", "owner_2"},
		{"entry_c", "owner_1"},
		{"entry_d", "owner_3"},
		{"entry_e", "owner_2"},
	}

	// Set all entries
	for _, tc := range testCases {
		entry := TestEntryForOwner{
			ID:      tc.entryID,
			OwnerID: tc.ownerID,
			Value:   "test value",
		}
		err := cache.Set(ctx, entry, 5*time.Minute)
		if err != nil {
			t.Fatalf("failed to set entry %s: %v", tc.entryID, err)
		}
	}

	// Verify each entry has the correct owner
	for _, tc := range testCases {
		ownerKey, found, err := cache.GetOwnerForEntry(ctx, tc.entryID)
		if err != nil {
			t.Fatalf("GetOwnerForEntry failed for %s: %v", tc.entryID, err)
		}
		if !found {
			t.Fatalf("expected entry %s to be found", tc.entryID)
		}
		if ownerKey != tc.ownerID {
			t.Fatalf("expected owner %s for entry %s, got %s", tc.ownerID, tc.entryID, ownerKey)
		}
	}
}

// TestGetOwnerForEntry_PerformanceBaseline benchmarks GetOwnerForEntry vs Get+deserialize
func TestGetOwnerForEntry_PerformanceBaseline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}

	cache, _, cleanup := setupTestCacheForOwner(t, true)
	defer cleanup()

	ctx := context.Background()

	// Create test entry
	entry := TestEntryForOwner{
		ID:      "perf_entry",
		OwnerID: "perf_owner",
		Value:   "test value with some longer content to simulate real data",
	}

	err := cache.Set(ctx, entry, 5*time.Minute)
	if err != nil {
		t.Fatalf("failed to set entry: %v", err)
	}

	// Benchmark GetOwnerForEntry (O(1) reverse index lookup)
	iterations := 1000
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, _, err := cache.GetOwnerForEntry(ctx, "perf_entry")
		if err != nil {
			t.Fatalf("GetOwnerForEntry failed: %v", err)
		}
	}
	ownerLookupDuration := time.Since(start)

	// Benchmark Get (O(N) full object deserialization)
	start = time.Now()
	for i := 0; i < iterations; i++ {
		entry, found, err := cache.Get(ctx, "perf_entry")
		if err != nil || !found {
			t.Fatalf("Get failed: %v (found: %v)", err, found)
		}
		_ = entry.OwnerID // Extract owner from full object
	}
	fullGetDuration := time.Since(start)

	t.Logf("GetOwnerForEntry: %v (%v/op)", ownerLookupDuration, ownerLookupDuration/time.Duration(iterations))
	t.Logf("Get+extract: %v (%v/op)", fullGetDuration, fullGetDuration/time.Duration(iterations))
	t.Logf("Performance improvement: %.2fx faster", float64(fullGetDuration)/float64(ownerLookupDuration))

	// GetOwnerForEntry should be faster than Get since it doesn't deserialize
	if ownerLookupDuration > fullGetDuration {
		t.Logf("Warning: GetOwnerForEntry was slower than Get - this might indicate an issue")
	}
}

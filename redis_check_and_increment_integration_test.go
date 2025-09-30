//go:build integration

package cache_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/internal/testintegration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==============================================================================
// CHECK AND INCREMENT INTEGRATION TESTS
// ==============================================================================

// TestRedisCache_CheckAndIncrement_WithinLimit tests basic increment within limit
func TestRedisCache_CheckAndIncrement_WithinLimit(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestCounterCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create counter cache")
	defer cache.Close()

	testKey := "ratelimit:user123:email:challenges"
	limit := int64(20)
	delta := int64(1)
	ttl := 5 * time.Minute

	t.Logf("🔢 Testing CheckAndIncrement within limit")
	t.Logf("   - Key: %s", testKey)
	t.Logf("   - Limit: %d", limit)

	// First increment on non-existent key - should succeed
	newValue, allowed, err := cache.CheckAndIncrement(ctx, testKey, limit, delta, ttl)
	assert.NoError(t, err, "First CheckAndIncrement should not error")
	assert.True(t, allowed, "First increment should be allowed")
	assert.Equal(t, int64(1), newValue, "First increment should return 1")

	// Second increment - should succeed
	newValue, allowed, err = cache.CheckAndIncrement(ctx, testKey, limit, delta, ttl)
	assert.NoError(t, err, "Second CheckAndIncrement should not error")
	assert.True(t, allowed, "Second increment should be allowed")
	assert.Equal(t, int64(2), newValue, "Second increment should return 2")

	// Increment 18 more times to reach limit
	for i := 0; i < 18; i++ {
		newValue, allowed, err = cache.CheckAndIncrement(ctx, testKey, limit, delta, ttl)
		require.NoError(t, err, "Increment %d should not error", i+3)
		require.True(t, allowed, "Increment %d should be allowed", i+3)
	}

	// Final value should be exactly at limit
	assert.Equal(t, int64(20), newValue, "Final value should equal limit")

	t.Logf("✅ CheckAndIncrement within limit test successful")
	t.Logf("   - Final value: %d", newValue)
}

// TestRedisCache_CheckAndIncrement_ExceedingLimit tests increment that exceeds limit
func TestRedisCache_CheckAndIncrement_ExceedingLimit(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestCounterCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create counter cache")
	defer cache.Close()

	testKey := "ratelimit:user456:sms:challenges"
	limit := int64(5)
	delta := int64(1)
	ttl := 5 * time.Minute

	t.Logf("🚫 Testing CheckAndIncrement exceeding limit")
	t.Logf("   - Key: %s", testKey)
	t.Logf("   - Limit: %d", limit)

	// Increment to limit
	var newValue int64
	var allowed bool
	for i := 0; i < 5; i++ {
		newValue, allowed, err = cache.CheckAndIncrement(ctx, testKey, limit, delta, ttl)
		require.NoError(t, err, "Increment %d should not error", i+1)
		require.True(t, allowed, "Increment %d should be allowed", i+1)
	}
	assert.Equal(t, int64(5), newValue, "Counter should be at limit")

	// Attempt to exceed limit - should be rejected
	newValue, allowed, err = cache.CheckAndIncrement(ctx, testKey, limit, delta, ttl)
	assert.NoError(t, err, "CheckAndIncrement should not error when exceeding limit")
	assert.False(t, allowed, "Increment should be rejected when exceeding limit")
	assert.Equal(t, int64(5), newValue, "Value should remain at limit")

	// Additional attempts should continue to be rejected
	newValue, allowed, err = cache.CheckAndIncrement(ctx, testKey, limit, delta, ttl)
	assert.NoError(t, err, "Second rejection should not error")
	assert.False(t, allowed, "Second attempt should also be rejected")
	assert.Equal(t, int64(5), newValue, "Value should still be at limit")

	t.Logf("✅ CheckAndIncrement exceeding limit test successful")
	t.Logf("   - Final value: %d (at limit)", newValue)
}

// TestRedisCache_CheckAndIncrement_NonExistentKey tests increment on non-existent key
func TestRedisCache_CheckAndIncrement_NonExistentKey(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestCounterCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create counter cache")
	defer cache.Close()

	testKey := "ratelimit:user789:new:challenges"
	limit := int64(10)
	delta := int64(1)
	ttl := 5 * time.Minute

	t.Logf("🆕 Testing CheckAndIncrement on non-existent key")
	t.Logf("   - Key: %s", testKey)
	t.Logf("   - Limit: %d", limit)

	// First increment should create key with delta value
	newValue, allowed, err := cache.CheckAndIncrement(ctx, testKey, limit, delta, ttl)
	assert.NoError(t, err, "CheckAndIncrement on non-existent key should not error")
	assert.True(t, allowed, "First increment should be allowed")
	assert.Equal(t, int64(1), newValue, "Non-existent key should be created with delta value")

	t.Logf("✅ CheckAndIncrement non-existent key test successful")
	t.Logf("   - Created value: %d", newValue)
}

// TestRedisCache_CheckAndIncrement_DeltaExceedsLimit tests delta larger than limit
func TestRedisCache_CheckAndIncrement_DeltaExceedsLimit(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestCounterCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create counter cache")
	defer cache.Close()

	testKey := "ratelimit:user999:large:delta"
	limit := int64(10)
	delta := int64(15) // Delta exceeds limit
	ttl := 5 * time.Minute

	t.Logf("⚠️  Testing CheckAndIncrement with delta > limit")
	t.Logf("   - Key: %s", testKey)
	t.Logf("   - Limit: %d", limit)
	t.Logf("   - Delta: %d", delta)

	// Attempt to increment with delta > limit on non-existent key - should be rejected
	newValue, allowed, err := cache.CheckAndIncrement(ctx, testKey, limit, delta, ttl)
	assert.NoError(t, err, "CheckAndIncrement should not error")
	assert.False(t, allowed, "Increment should be rejected when delta > limit")
	assert.Equal(t, int64(0), newValue, "Key should not be created when delta > limit")

	t.Logf("✅ CheckAndIncrement delta exceeds limit test successful")
}

// TestRedisCache_CheckAndIncrement_VariableDeltas tests increments with different delta values
func TestRedisCache_CheckAndIncrement_VariableDeltas(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestCounterCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create counter cache")
	defer cache.Close()

	testKey := "ratelimit:user111:variable:deltas"
	limit := int64(20)
	ttl := 5 * time.Minute

	t.Logf("🔢 Testing CheckAndIncrement with variable deltas")
	t.Logf("   - Key: %s", testKey)
	t.Logf("   - Limit: %d", limit)

	// Increment by 5
	newValue, allowed, err := cache.CheckAndIncrement(ctx, testKey, limit, 5, ttl)
	assert.NoError(t, err, "First increment should not error")
	assert.True(t, allowed, "First increment should be allowed")
	assert.Equal(t, int64(5), newValue, "Value should be 5")

	// Increment by 3
	newValue, allowed, err = cache.CheckAndIncrement(ctx, testKey, limit, 3, ttl)
	assert.NoError(t, err, "Second increment should not error")
	assert.True(t, allowed, "Second increment should be allowed")
	assert.Equal(t, int64(8), newValue, "Value should be 8")

	// Increment by 10
	newValue, allowed, err = cache.CheckAndIncrement(ctx, testKey, limit, 10, ttl)
	assert.NoError(t, err, "Third increment should not error")
	assert.True(t, allowed, "Third increment should be allowed")
	assert.Equal(t, int64(18), newValue, "Value should be 18")

	// Attempt to increment by 5 (would exceed limit of 20)
	newValue, allowed, err = cache.CheckAndIncrement(ctx, testKey, limit, 5, ttl)
	assert.NoError(t, err, "Fourth increment should not error")
	assert.False(t, allowed, "Fourth increment should be rejected (18 + 5 > 20)")
	assert.Equal(t, int64(18), newValue, "Value should remain at 18")

	// Increment by 2 (exactly reaches limit)
	newValue, allowed, err = cache.CheckAndIncrement(ctx, testKey, limit, 2, ttl)
	assert.NoError(t, err, "Fifth increment should not error")
	assert.True(t, allowed, "Fifth increment should be allowed (18 + 2 = 20)")
	assert.Equal(t, int64(20), newValue, "Value should be exactly at limit")

	t.Logf("✅ CheckAndIncrement variable deltas test successful")
	t.Logf("   - Final value: %d", newValue)
}

// TestRedisCache_CheckAndIncrement_ConcurrentAccess tests atomicity under concurrent load
func TestRedisCache_CheckAndIncrement_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestCounterCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create counter cache")
	defer cache.Close()

	testKey := "ratelimit:concurrent:test"
	limit := int64(20)
	delta := int64(1)
	ttl := 5 * time.Minute
	numGoroutines := 100

	t.Logf("🏁 Testing CheckAndIncrement atomicity under concurrent load")
	t.Logf("   - Key: %s", testKey)
	t.Logf("   - Limit: %d", limit)
	t.Logf("   - Concurrent goroutines: %d", numGoroutines)

	// Use channels to coordinate goroutines
	startSignal := make(chan struct{})
	var wg sync.WaitGroup

	// Track results
	successCount := make(chan int64, numGoroutines)
	rejectionCount := make(chan int64, numGoroutines)

	// Start goroutines that all try to increment simultaneously
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			// Wait for start signal to maximize concurrency
			<-startSignal

			// Attempt increment
			newValue, allowed, err := cache.CheckAndIncrement(ctx, testKey, limit, delta, ttl)
			if err != nil {
				t.Errorf("Goroutine %d error: %v", goroutineID, err)
				return
			}

			if allowed {
				successCount <- newValue
				t.Logf("✅ Goroutine %d: increment allowed, new value: %d", goroutineID, newValue)
			} else {
				rejectionCount <- newValue
				t.Logf("🚫 Goroutine %d: increment rejected, current value: %d", goroutineID, newValue)
			}
		}(i)
	}

	// Start all goroutines simultaneously
	close(startSignal)

	// Wait for all goroutines to complete
	wg.Wait()
	close(successCount)
	close(rejectionCount)

	// Count results
	var successes, rejections int64
	for range successCount {
		successes++
	}
	for range rejectionCount {
		rejections++
	}

	t.Logf("📊 Concurrent test results:")
	t.Logf("   - Successful increments: %d", successes)
	t.Logf("   - Rejected increments: %d", rejections)
	t.Logf("   - Total attempts: %d", successes+rejections)

	// Critical assertion: exactly limit number of increments should succeed
	assert.Equal(t, limit, successes, "Exactly limit number of increments should succeed (proves atomicity)")
	assert.Equal(t, int64(numGoroutines)-limit, rejections, "Remaining attempts should be rejected")

	// Verify final counter value matches limit by attempting one more increment
	finalValue, allowed, err := cache.CheckAndIncrement(ctx, testKey, limit, delta, ttl)
	assert.NoError(t, err, "Final value check should not error")
	assert.False(t, allowed, "No more increments should be allowed")
	assert.Equal(t, limit, finalValue, "Final value should equal limit")

	t.Logf("✅ CheckAndIncrement concurrent access test successful")
	t.Logf("   - No race conditions detected")
	t.Logf("   - Atomicity verified: %d allowed, %d rejected", successes, rejections)
}

// TestRedisCache_CheckAndIncrement_TTLBehavior tests TTL setting and refresh behavior
func TestRedisCache_CheckAndIncrement_TTLBehavior(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestCounterCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create counter cache")
	defer cache.Close()

	testKey := "ratelimit:ttl:test"
	limit := int64(10)
	delta := int64(1)
	ttl := 2 * time.Second

	t.Logf("⏱️  Testing CheckAndIncrement TTL behavior")
	t.Logf("   - Key: %s", testKey)
	t.Logf("   - TTL: %v", ttl)

	// First increment - should set TTL
	newValue, allowed, err := cache.CheckAndIncrement(ctx, testKey, limit, delta, ttl)
	assert.NoError(t, err, "First increment should not error")
	assert.True(t, allowed, "First increment should be allowed")
	assert.Equal(t, int64(1), newValue, "Value should be 1")

	// Second increment to verify counter is working
	newValue, allowed, err = cache.CheckAndIncrement(ctx, testKey, limit, delta, ttl)
	assert.NoError(t, err, "Second increment should not error")
	assert.True(t, allowed, "Second increment should be allowed")
	assert.Equal(t, int64(2), newValue, "Value should be 2")

	// Wait for TTL to expire
	t.Logf("⏳ Waiting for TTL to expire...")
	time.Sleep(ttl + 500*time.Millisecond)

	// Next increment should start fresh from 1 (counter should have expired)
	newValue, allowed, err = cache.CheckAndIncrement(ctx, testKey, limit, delta, ttl)
	assert.NoError(t, err, "Increment after expiry should not error")
	assert.True(t, allowed, "Increment after expiry should be allowed")
	assert.Equal(t, int64(1), newValue, "Counter should reset to 1 after expiry")

	t.Logf("✅ CheckAndIncrement TTL behavior test successful")
}

// TestRedisCache_CheckAndIncrement_MultipleKeys tests independent counters for different keys
func TestRedisCache_CheckAndIncrement_MultipleKeys(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestCounterCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create counter cache")
	defer cache.Close()

	keys := []string{
		"ratelimit:user1:email:challenges",
		"ratelimit:user1:sms:challenges",
		"ratelimit:user1:all:challenges",
	}
	limit := int64(10)
	delta := int64(1)
	ttl := 5 * time.Minute

	t.Logf("🔑 Testing CheckAndIncrement with multiple independent keys")

	// Increment each key to different values
	for i, key := range keys {
		incrementCount := int64(i + 1) * 3 // 3, 6, 9
		t.Logf("   - Incrementing %s to %d", key, incrementCount)

		for j := int64(0); j < incrementCount; j++ {
			newValue, allowed, err := cache.CheckAndIncrement(ctx, key, limit, delta, ttl)
			require.NoError(t, err, "Increment should not error for key %s", key)
			require.True(t, allowed, "Increment should be allowed for key %s", key)
			if j == incrementCount-1 {
				assert.Equal(t, incrementCount, newValue, "Final value should match for key %s", key)
			}
		}
	}

	// Verify each key maintained its independent value
	expectedValues := []int64{3, 6, 9}
	for i, key := range keys {
		newValue, allowed, err := cache.CheckAndIncrement(ctx, key, limit, delta, ttl)
		assert.NoError(t, err, "Verification should not error for key %s", key)
		assert.True(t, allowed, "Increment should still be allowed for key %s", key)
		assert.Equal(t, expectedValues[i]+1, newValue, "Key %s should have independent counter", key)
	}

	t.Logf("✅ CheckAndIncrement multiple keys test successful")
	t.Logf("   - All keys maintained independent counters")
}
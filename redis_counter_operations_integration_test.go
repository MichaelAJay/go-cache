//go:build integration

package cache_test

import (
	"context"
	"testing"

	"github.com/MichaelAJay/go-cache/internal/testintegration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRedisCache_BasicIncrement tests basic increment operations
func TestRedisCache_BasicIncrement(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance with int64 type for counter operations
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestCounterCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create counter cache")
	defer cache.Close()

	testKey := "counter:basic-increment"

	t.Logf("🔢 Testing basic INCREMENT operation for key: %s", testKey)

	// Test increment on non-existent key (should create with delta value)
	result, err := cache.Increment(ctx, testKey, 5)

	// Assertions
	assert.NoError(t, err, "Increment operation should not error")
	assert.Equal(t, int64(5), result, "First increment should return delta value")

	// Test another increment on existing key
	result, err = cache.Increment(ctx, testKey, 3)
	assert.NoError(t, err, "Second increment should not error")
	assert.Equal(t, int64(8), result, "Second increment should return cumulative value")

	t.Logf("✅ Basic increment test successful")
	t.Logf("   - Key: %s", testKey)
	t.Logf("   - Final value: %d", result)
}

// TestRedisCache_BasicDecrement tests basic decrement operations
func TestRedisCache_BasicDecrement(t *testing.T) {
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

	testKey := "counter:basic-decrement"

	t.Logf("🔢 Testing basic DECREMENT operation for key: %s", testKey)

	// Test decrement on non-existent key (should create with negative delta value)
	result, err := cache.Decrement(ctx, testKey, 5)

	// Assertions
	assert.NoError(t, err, "Decrement operation should not error")
	assert.Equal(t, int64(-5), result, "First decrement should return negative delta value")

	// Test another decrement on existing key
	result, err = cache.Decrement(ctx, testKey, 3)
	assert.NoError(t, err, "Second decrement should not error")
	assert.Equal(t, int64(-8), result, "Second decrement should return cumulative negative value")

	t.Logf("✅ Basic decrement test successful")
	t.Logf("   - Key: %s", testKey)
	t.Logf("   - Final value: %d", result)
}

// TestRedisCache_IncrementFloat tests floating-point increment operations
func TestRedisCache_IncrementFloat(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestFloatCounterCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create float counter cache")
	defer cache.Close()

	testKey := "counter:float-increment"

	t.Logf("🔢 Testing FLOAT INCREMENT operation for key: %s", testKey)

	// Test float increment on non-existent key
	result, err := cache.IncrementFloat(ctx, testKey, 2.5)

	// Assertions
	assert.NoError(t, err, "IncrementFloat operation should not error")
	assert.InDelta(t, 2.5, result, 0.001, "First float increment should return delta value")

	// Test another float increment on existing key
	result, err = cache.IncrementFloat(ctx, testKey, 1.3)
	assert.NoError(t, err, "Second IncrementFloat should not error")
	assert.InDelta(t, 3.8, result, 0.001, "Second float increment should return cumulative value")

	// Test negative float increment (effectively decrement)
	result, err = cache.IncrementFloat(ctx, testKey, -0.8)
	assert.NoError(t, err, "Negative IncrementFloat should not error")
	assert.InDelta(t, 3.0, result, 0.001, "Negative float increment should reduce value")

	t.Logf("✅ Float increment test successful")
	t.Logf("   - Key: %s", testKey)
	t.Logf("   - Final value: %.3f", result)
}

// TestRedisCache_CounterSequence tests a sequence of counter operations
func TestRedisCache_CounterSequence(t *testing.T) {
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

	testKey := "counter:sequence"

	t.Logf("🔢 Testing counter operation SEQUENCE for key: %s", testKey)

	// Start with increment
	result, err := cache.Increment(ctx, testKey, 10)
	require.NoError(t, err, "First increment should not error")
	assert.Equal(t, int64(10), result, "First increment result")

	// Add more
	result, err = cache.Increment(ctx, testKey, 5)
	require.NoError(t, err, "Second increment should not error")
	assert.Equal(t, int64(15), result, "Second increment result")

	// Decrement some
	result, err = cache.Decrement(ctx, testKey, 3)
	require.NoError(t, err, "First decrement should not error")
	assert.Equal(t, int64(12), result, "First decrement result")

	// Another increment
	result, err = cache.Increment(ctx, testKey, 8)
	require.NoError(t, err, "Third increment should not error")
	assert.Equal(t, int64(20), result, "Third increment result")

	// Large decrement
	result, err = cache.Decrement(ctx, testKey, 25)
	require.NoError(t, err, "Second decrement should not error")
	assert.Equal(t, int64(-5), result, "Second decrement should result in negative value")

	t.Logf("✅ Counter sequence test successful")
	t.Logf("   - Key: %s", testKey)
	t.Logf("   - Final value: %d", result)
}

// TestRedisCache_MultipleCounters tests multiple independent counters
func TestRedisCache_MultipleCounters(t *testing.T) {
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
		"counter:multi-1",
		"counter:multi-2",
		"counter:multi-3",
	}

	t.Logf("🔢 Testing MULTIPLE independent counters")

	// Initialize all counters with different values
	expectedValues := make(map[string]int64)
	
	result, err := cache.Increment(ctx, keys[0], 100)
	require.NoError(t, err, "Counter 1 initialization should not error")
	expectedValues[keys[0]] = result
	assert.Equal(t, int64(100), result, "Counter 1 should initialize to 100")

	result, err = cache.Increment(ctx, keys[1], 200)
	require.NoError(t, err, "Counter 2 initialization should not error")
	expectedValues[keys[1]] = result
	assert.Equal(t, int64(200), result, "Counter 2 should initialize to 200")

	result, err = cache.Increment(ctx, keys[2], 50)
	require.NoError(t, err, "Counter 3 initialization should not error")
	expectedValues[keys[2]] = result
	assert.Equal(t, int64(50), result, "Counter 3 should initialize to 50")

	// Perform operations on each counter independently
	result, err = cache.Increment(ctx, keys[0], 10)
	require.NoError(t, err, "Counter 1 increment should not error")
	expectedValues[keys[0]] = 110
	assert.Equal(t, int64(110), result, "Counter 1 should be 110")

	result, err = cache.Decrement(ctx, keys[1], 50)
	require.NoError(t, err, "Counter 2 decrement should not error")
	expectedValues[keys[1]] = 150
	assert.Equal(t, int64(150), result, "Counter 2 should be 150")

	result, err = cache.Increment(ctx, keys[2], 25)
	require.NoError(t, err, "Counter 3 increment should not error")
	expectedValues[keys[2]] = 75
	assert.Equal(t, int64(75), result, "Counter 3 should be 75")

	// Verify all counters have maintained independent values
	for _, key := range keys {
		// We can't directly get counter values from a counter cache, but we can increment by 0
		result, err := cache.Increment(ctx, key, 0)
		require.NoError(t, err, "Zero increment should not error for key %s", key)
		assert.Equal(t, expectedValues[key], result, "Counter %s should maintain its value", key)
	}

	t.Logf("✅ Multiple counters test successful")
	t.Logf("   - Counter 1: %d", expectedValues[keys[0]])
	t.Logf("   - Counter 2: %d", expectedValues[keys[1]])
	t.Logf("   - Counter 3: %d", expectedValues[keys[2]])
}

// TestRedisCache_CounterZeroDelta tests counter operations with zero delta
func TestRedisCache_CounterZeroDelta(t *testing.T) {
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

	testKey := "counter:zero-delta"

	t.Logf("🔢 Testing counter operations with ZERO DELTA for key: %s", testKey)

	// Test increment with zero delta on non-existent key (should create with 0)
	result, err := cache.Increment(ctx, testKey, 0)
	assert.NoError(t, err, "Zero increment on non-existent key should not error")
	assert.Equal(t, int64(0), result, "Zero increment should create key with value 0")

	// Set counter to a specific value first
	result, err = cache.Increment(ctx, testKey, 42)
	require.NoError(t, err, "Setup increment should not error")
	assert.Equal(t, int64(42), result, "Setup should set counter to 42")

	// Test zero increment (should return current value without changing it)
	result, err = cache.Increment(ctx, testKey, 0)
	assert.NoError(t, err, "Zero increment should not error")
	assert.Equal(t, int64(42), result, "Zero increment should return current value unchanged")

	// Test zero decrement (should return current value without changing it)
	result, err = cache.Decrement(ctx, testKey, 0)
	assert.NoError(t, err, "Zero decrement should not error")
	assert.Equal(t, int64(42), result, "Zero decrement should return current value unchanged")

	t.Logf("✅ Zero delta test successful")
	t.Logf("   - Key: %s", testKey)
	t.Logf("   - Final value: %d", result)
}

// TestRedisCache_CounterNegativeValues tests counter operations with negative values
func TestRedisCache_CounterNegativeValues(t *testing.T) {
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

	testKey := "counter:negative-values"

	t.Logf("🔢 Testing counter operations with NEGATIVE VALUES for key: %s", testKey)

	// Start with a negative increment (creates negative counter)
	result, err := cache.Increment(ctx, testKey, -15)
	assert.NoError(t, err, "Negative increment should not error")
	assert.Equal(t, int64(-15), result, "Negative increment should create negative counter")

	// Increment by positive value (should increase toward zero)
	result, err = cache.Increment(ctx, testKey, 5)
	assert.NoError(t, err, "Positive increment on negative counter should not error")
	assert.Equal(t, int64(-10), result, "Positive increment should move counter toward zero")

	// Decrement by negative value (effectively an increment)
	result, err = cache.Decrement(ctx, testKey, -3)
	assert.NoError(t, err, "Negative decrement should not error")
	assert.Equal(t, int64(-7), result, "Negative decrement should effectively increment")

	// Cross zero with increment
	result, err = cache.Increment(ctx, testKey, 20)
	assert.NoError(t, err, "Large increment crossing zero should not error")
	assert.Equal(t, int64(13), result, "Counter should cross zero into positive territory")

	t.Logf("✅ Negative values test successful")
	t.Logf("   - Key: %s", testKey)
	t.Logf("   - Final value: %d", result)
}

// TestRedisCache_CounterLargeValues tests counter operations with large values
func TestRedisCache_CounterLargeValues(t *testing.T) {
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

	testKey := "counter:large-values"

	t.Logf("🔢 Testing counter operations with LARGE VALUES for key: %s", testKey)

	// Test with large positive value
	largeValue := int64(1000000000) // 1 billion
	result, err := cache.Increment(ctx, testKey, largeValue)
	assert.NoError(t, err, "Large increment should not error")
	assert.Equal(t, largeValue, result, "Large increment should work correctly")

	// Add another large value
	result, err = cache.Increment(ctx, testKey, largeValue)
	assert.NoError(t, err, "Second large increment should not error")
	assert.Equal(t, largeValue*2, result, "Second large increment should add correctly")

	// Decrement a large value
	result, err = cache.Decrement(ctx, testKey, largeValue/2)
	assert.NoError(t, err, "Large decrement should not error")
	assert.Equal(t, largeValue+largeValue/2, result, "Large decrement should subtract correctly")

	t.Logf("✅ Large values test successful")
	t.Logf("   - Key: %s", testKey)
	t.Logf("   - Final value: %d", result)
}

// TestRedisCache_FloatCounterPrecision tests floating-point precision in counter operations
func TestRedisCache_FloatCounterPrecision(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestFloatCounterCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create float counter cache")
	defer cache.Close()

	testKey := "counter:float-precision"

	t.Logf("🔢 Testing FLOAT PRECISION in counter operations for key: %s", testKey)

	// Test with high precision values
	result, err := cache.IncrementFloat(ctx, testKey, 0.1)
	assert.NoError(t, err, "High precision increment should not error")
	assert.InDelta(t, 0.1, result, 0.00001, "High precision should be maintained")

	// Add more precision
	result, err = cache.IncrementFloat(ctx, testKey, 0.2)
	assert.NoError(t, err, "Second precision increment should not error")
	assert.InDelta(t, 0.3, result, 0.00001, "Combined precision should be maintained")

	// Test with very small values
	result, err = cache.IncrementFloat(ctx, testKey, 0.0001)
	assert.NoError(t, err, "Very small increment should not error")
	assert.InDelta(t, 0.3001, result, 0.00001, "Very small precision should be maintained")

	// Test with negative precision
	result, err = cache.IncrementFloat(ctx, testKey, -0.0001)
	assert.NoError(t, err, "Negative precision increment should not error")
	assert.InDelta(t, 0.3, result, 0.00001, "Negative precision should work correctly")

	t.Logf("✅ Float precision test successful")
	t.Logf("   - Key: %s", testKey)
	t.Logf("   - Final value: %.6f", result)
}

// TestRedisCache_ConcurrentCounterOperations tests concurrent counter operations for thread safety
func TestRedisCache_ConcurrentCounterOperations(t *testing.T) {
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

	testKey := "counter:concurrent-test"
	numGoroutines := 50
	incrementsPerGoroutine := 10
	expectedFinalValue := int64(numGoroutines * incrementsPerGoroutine)

	t.Logf("🔢 Testing CONCURRENT counter operations for key: %s", testKey)
	t.Logf("   - Goroutines: %d", numGoroutines)
	t.Logf("   - Increments per goroutine: %d", incrementsPerGoroutine)
	t.Logf("   - Expected final value: %d", expectedFinalValue)

	// Use channels to synchronize goroutines
	startSignal := make(chan struct{})
	done := make(chan struct{}, numGoroutines)

	// Start goroutines that will all increment the same counter
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer func() { done <- struct{}{} }()
			
			// Wait for start signal
			<-startSignal
			
			// Perform multiple increments
			for j := 0; j < incrementsPerGoroutine; j++ {
				_, err := cache.Increment(ctx, testKey, 1)
				if err != nil {
					t.Errorf("Goroutine %d increment %d failed: %v", goroutineID, j, err)
					return
				}
			}
		}(i)
	}

	// Start all goroutines simultaneously
	close(startSignal)

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Check final value - should be exactly the expected value if operations were atomic
	finalValue, err := cache.Increment(ctx, testKey, 0) // Add 0 to get current value
	require.NoError(t, err, "Failed to get final counter value")
	assert.Equal(t, expectedFinalValue, finalValue, "Final counter value should match expected value from concurrent operations")

	t.Logf("✅ Concurrent counter test successful")
	t.Logf("   - Expected: %d", expectedFinalValue)
	t.Logf("   - Actual: %d", finalValue)
	t.Logf("   - Race conditions: %s", func() string {
		if expectedFinalValue == finalValue {
			return "NONE DETECTED"
		}
		return "DETECTED - TEST FAILED"
	}())
}

// TestRedisCache_ConcurrentMixedCounterOperations tests concurrent increments and decrements
func TestRedisCache_ConcurrentMixedCounterOperations(t *testing.T) {
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

	testKey := "counter:mixed-concurrent"
	numIncrementGoroutines := 25
	numDecrementGoroutines := 25
	operationsPerGoroutine := 20
	
	// Each increment goroutine adds operationsPerGoroutine
	// Each decrement goroutine subtracts operationsPerGoroutine
	// Net result should be 0
	expectedFinalValue := int64(0)

	t.Logf("🔢 Testing CONCURRENT mixed counter operations for key: %s", testKey)
	t.Logf("   - Increment goroutines: %d", numIncrementGoroutines)
	t.Logf("   - Decrement goroutines: %d", numDecrementGoroutines)
	t.Logf("   - Operations per goroutine: %d", operationsPerGoroutine)
	t.Logf("   - Expected final value: %d", expectedFinalValue)

	// Use channels to synchronize goroutines
	startSignal := make(chan struct{})
	done := make(chan struct{}, numIncrementGoroutines+numDecrementGoroutines)

	// Start increment goroutines
	for i := 0; i < numIncrementGoroutines; i++ {
		go func(goroutineID int) {
			defer func() { done <- struct{}{} }()
			
			// Wait for start signal
			<-startSignal
			
			// Perform increments
			for j := 0; j < operationsPerGoroutine; j++ {
				_, err := cache.Increment(ctx, testKey, 1)
				if err != nil {
					t.Errorf("Increment goroutine %d operation %d failed: %v", goroutineID, j, err)
					return
				}
			}
		}(i)
	}

	// Start decrement goroutines
	for i := 0; i < numDecrementGoroutines; i++ {
		go func(goroutineID int) {
			defer func() { done <- struct{}{} }()
			
			// Wait for start signal
			<-startSignal
			
			// Perform decrements
			for j := 0; j < operationsPerGoroutine; j++ {
				_, err := cache.Decrement(ctx, testKey, 1)
				if err != nil {
					t.Errorf("Decrement goroutine %d operation %d failed: %v", goroutineID, j, err)
					return
				}
			}
		}(i)
	}

	// Start all goroutines simultaneously
	close(startSignal)

	// Wait for all goroutines to complete
	totalGoroutines := numIncrementGoroutines + numDecrementGoroutines
	for i := 0; i < totalGoroutines; i++ {
		<-done
	}

	// Check final value - should be exactly 0 if all operations were atomic
	finalValue, err := cache.Increment(ctx, testKey, 0) // Add 0 to get current value
	require.NoError(t, err, "Failed to get final counter value")
	assert.Equal(t, expectedFinalValue, finalValue, "Final counter value should be 0 from balanced concurrent operations")

	t.Logf("✅ Concurrent mixed operations test successful")
	t.Logf("   - Expected: %d", expectedFinalValue)
	t.Logf("   - Actual: %d", finalValue)
	t.Logf("   - Operations balanced correctly: %s", func() string {
		if expectedFinalValue == finalValue {
			return "YES"
		}
		return "NO - TEST FAILED"
	}())
}

// TestRedisCache_ConcurrentFloatCounterOperations tests concurrent float counter operations
func TestRedisCache_ConcurrentFloatCounterOperations(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestFloatCounterCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create float counter cache")
	defer cache.Close()

	testKey := "counter:concurrent-float"
	numGoroutines := 20
	incrementPerGoroutine := 0.5
	operationsPerGoroutine := 10
	expectedFinalValue := float64(numGoroutines) * incrementPerGoroutine * float64(operationsPerGoroutine)

	t.Logf("🔢 Testing CONCURRENT float counter operations for key: %s", testKey)
	t.Logf("   - Goroutines: %d", numGoroutines)
	t.Logf("   - Increment per operation: %.1f", incrementPerGoroutine)
	t.Logf("   - Operations per goroutine: %d", operationsPerGoroutine)
	t.Logf("   - Expected final value: %.1f", expectedFinalValue)

	// Use channels to synchronize goroutines
	startSignal := make(chan struct{})
	done := make(chan struct{}, numGoroutines)

	// Start goroutines that will all increment the same float counter
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer func() { done <- struct{}{} }()
			
			// Wait for start signal
			<-startSignal
			
			// Perform multiple float increments
			for j := 0; j < operationsPerGoroutine; j++ {
				_, err := cache.IncrementFloat(ctx, testKey, incrementPerGoroutine)
				if err != nil {
					t.Errorf("Float goroutine %d increment %d failed: %v", goroutineID, j, err)
					return
				}
			}
		}(i)
	}

	// Start all goroutines simultaneously
	close(startSignal)

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Check final value - should be close to expected value (allowing for floating point precision)
	finalValue, err := cache.IncrementFloat(ctx, testKey, 0) // Add 0 to get current value
	require.NoError(t, err, "Failed to get final float counter value")
	assert.InDelta(t, expectedFinalValue, finalValue, 0.01, "Final float counter value should match expected value from concurrent operations")

	t.Logf("✅ Concurrent float counter test successful")
	t.Logf("   - Expected: %.2f", expectedFinalValue)
	t.Logf("   - Actual: %.2f", finalValue)
	t.Logf("   - Difference: %.6f", expectedFinalValue-finalValue)
}
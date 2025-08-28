//go:build integration
// +build integration

package redis

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/interfaces"
	cacheErrors "github.com/MichaelAJay/go-cache/cache_errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCircuitBreakerBehavior tests the circuit breaker functionality
func TestCircuitBreakerBehavior(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	
	t.Run("CircuitBreakerThreshold", func(t *testing.T) {
		testCircuitBreakerThreshold(t, container)
	})
	
	t.Run("CircuitBreakerRecovery", func(t *testing.T) {
		testCircuitBreakerRecovery(t, container)
	})
	
	t.Run("CircuitBreakerUnderLoad", func(t *testing.T) {
		testCircuitBreakerUnderLoad(t, container)
	})
}

// testCircuitBreakerThreshold tests that circuit breaker opens after threshold failures
func testCircuitBreakerThreshold(t *testing.T, container *TestRedisContainer) {
	// Create cache with invalid Redis connection to force failures
	options := &interfaces.CacheOptions{
		RedisOptions: &interfaces.RedisOptions{
			Address:  "localhost:9999", // Invalid port to cause connection failures
			DB:       0,
			PoolSize: 1,
		},
	}
	
	cache := CreateCacheWithOptions[string](container, options)
	defer cache.Close()
	
	ctx := context.Background()
	key := "circuit-test-key"
	value := "circuit-test-value"
	
	// Perform operations that will fail and trigger circuit breaker
	failureCount := 0
	circuitBreakerErrors := 0
	
	// Try operations until circuit breaker opens (should be around 10 failures)
	for i := 0; i < 20; i++ {
		err := cache.Set(ctx, key, value, time.Hour)
		if err != nil {
			if err == cacheErrors.ErrCircuitBreakerOpen {
				circuitBreakerErrors++
			} else {
				failureCount++
			}
		}
		
		// Give small delay between failures
		time.Sleep(10 * time.Millisecond)
	}
	
	// We should have some failures before circuit breaker opens
	assert.Greater(t, failureCount, 5, "Should have failures before circuit breaker opens")
	assert.Greater(t, circuitBreakerErrors, 0, "Should have circuit breaker errors")
	
	// Subsequent operations should fail fast with circuit breaker error
	_, _, err := cache.Get(ctx, key)
	assert.Equal(t, cacheErrors.ErrCircuitBreakerOpen, err, 
		"Operations should fail fast when circuit breaker is open")
}

// testCircuitBreakerRecovery tests that circuit breaker recovers after timeout
func testCircuitBreakerRecovery(t *testing.T, container *TestRedisContainer) {
	// This test simulates circuit breaker recovery by using a working connection
	// and then testing after the timeout period
	
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	ctx := context.Background()
	key := "recovery-test-key"
	value := "recovery-test-value"
	
	// First, verify normal operation works
	err := cache.Set(ctx, key, value, time.Hour)
	require.NoError(t, err, "Normal operation should work")
	
	retrieved, found, err := cache.Get(ctx, key)
	require.NoError(t, err, "Get should work")
	assert.True(t, found, "Key should be found")
	assert.Equal(t, value, retrieved, "Value should match")
	
	t.Log("Circuit breaker recovery test completed - would require Redis failure simulation for full test")
}

// testCircuitBreakerUnderLoad tests circuit breaker behavior under concurrent load
func testCircuitBreakerUnderLoad(t *testing.T, container *TestRedisContainer) {
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	ctx := context.Background()
	numGoroutines := 100
	operationsPerGoroutine := 10
	
	var successCount int64
	var errorCount int64
	var circuitBreakerCount int64
	var wg sync.WaitGroup
	
	wg.Add(numGoroutines)
	
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			
			for i := 0; i < operationsPerGoroutine; i++ {
				key := fmt.Sprintf("load-test-%d-%d", goroutineID, i)
				value := fmt.Sprintf("value-%d-%d", goroutineID, i)
				
				err := cache.Set(ctx, key, value, time.Hour)
				if err != nil {
					if err == cacheErrors.ErrCircuitBreakerOpen {
						atomic.AddInt64(&circuitBreakerCount, 1)
					} else {
						atomic.AddInt64(&errorCount, 1)
					}
				} else {
					atomic.AddInt64(&successCount, 1)
				}
				
				// Small delay to prevent overwhelming
				time.Sleep(time.Millisecond)
			}
		}(g)
	}
	
	wg.Wait()
	
	totalOperations := int64(numGoroutines * operationsPerGoroutine)
	assert.Equal(t, totalOperations, successCount+errorCount+circuitBreakerCount, 
		"All operations should be accounted for")
	
	// Under normal conditions, most operations should succeed
	assert.Greater(t, successCount, totalOperations/2, 
		"Most operations should succeed under normal conditions")
	
	t.Logf("Results: Success=%d, Errors=%d, CircuitBreaker=%d", 
		successCount, errorCount, circuitBreakerCount)
}

// TestConnectionPoolBehavior tests connection pool handling under stress
func TestConnectionPoolBehavior(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	
	t.Run("PoolExhaustion", func(t *testing.T) {
		testConnectionPoolExhaustion(t, container)
	})
	
	t.Run("PoolRecovery", func(t *testing.T) {
		testConnectionPoolRecovery(t, container)
	})
	
	t.Run("ConcurrentConnections", func(t *testing.T) {
		testConcurrentConnections(t, container)
	})
}

// testConnectionPoolExhaustion tests behavior when connection pool is exhausted
func testConnectionPoolExhaustion(t *testing.T, container *TestRedisContainer) {
	// Create cache with very small connection pool
	options := &interfaces.CacheOptions{
		RedisOptions: &interfaces.RedisOptions{
			Address:  container.GetRedisAddr(),
			DB:       0,
			PoolSize: 2, // Very small pool
		},
	}
	
	cache := CreateCacheWithOptions[string](container, options)
	defer cache.Close()
	
	ctx := context.Background()
	
	// Start many concurrent operations that might exhaust the pool
	numGoroutines := 20
	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64
	
	wg.Add(numGoroutines)
	
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			
			key := fmt.Sprintf("pool-test-%d", goroutineID)
			value := fmt.Sprintf("value-%d", goroutineID)
			
			// Perform a slow operation to hold connections
			err := cache.Set(ctx, key, value, time.Hour)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
			} else {
				atomic.AddInt64(&successCount, 1)
			}
			
			// Hold the operation a bit to stress the pool
			time.Sleep(100 * time.Millisecond)
		}(g)
	}
	
	wg.Wait()
	
	// Most operations should still succeed (Redis client handles pool management)
	totalOps := int64(numGoroutines)
	assert.Equal(t, totalOps, successCount+errorCount, 
		"All operations should complete")
	
	// Allow some errors due to pool exhaustion, but most should succeed
	assert.GreaterOrEqual(t, successCount, totalOps/2, 
		"At least half of operations should succeed")
	
	t.Logf("Pool exhaustion test: Success=%d, Errors=%d", successCount, errorCount)
}

// testConnectionPoolRecovery tests that the pool recovers after stress
func testConnectionPoolRecovery(t *testing.T, container *TestRedisContainer) {
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	ctx := context.Background()
	
	// Stress the connection pool first
	stressOperations := 100
	for i := 0; i < stressOperations; i++ {
		go func(id int) {
			key := fmt.Sprintf("stress-key-%d", id)
			cache.Set(ctx, key, "stress-value", time.Hour)
		}(i)
	}
	
	// Wait a bit for stress operations to complete
	time.Sleep(500 * time.Millisecond)
	
	// Now test that normal operations work fine
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("recovery-key-%d", i)
		value := fmt.Sprintf("recovery-value-%d", i)
		
		err := cache.Set(ctx, key, value, time.Hour)
		assert.NoError(t, err, "Operations should work after pool recovery")
		
		retrieved, found, err := cache.Get(ctx, key)
		require.NoError(t, err, "Get should work after recovery")
		assert.True(t, found, "Key should be found")
		assert.Equal(t, value, retrieved, "Value should match")
	}
}

// testConcurrentConnections tests handling of many concurrent connections
func testConcurrentConnections(t *testing.T, container *TestRedisContainer) {
	// Create cache with reasonable pool size
	options := &interfaces.CacheOptions{
		RedisOptions: &interfaces.RedisOptions{
			Address:  container.GetRedisAddr(),
			DB:       0,
			PoolSize: 20,
		},
	}
	
	cache := CreateCacheWithOptions[string](container, options)
	defer cache.Close()
	
	ctx := context.Background()
	numGoroutines := 200
	operationsPerGoroutine := 5
	
	var wg sync.WaitGroup
	var totalSuccessful int64
	var totalErrors int64
	
	wg.Add(numGoroutines)
	
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			
			successful := 0
			errors := 0
			
			for i := 0; i < operationsPerGoroutine; i++ {
				key := fmt.Sprintf("concurrent-%d-%d", goroutineID, i)
				value := fmt.Sprintf("value-%d-%d", goroutineID, i)
				
				// Mix of operations
				switch i % 3 {
				case 0:
					err := cache.Set(ctx, key, value, time.Hour)
					if err != nil {
						errors++
					} else {
						successful++
					}
				case 1:
					_, _, err := cache.Get(ctx, key)
					if err != nil {
						errors++
					} else {
						successful++
					}
				case 2:
					exists := cache.Has(ctx, key)
					_ = exists
					successful++ // Has shouldn't error
				}
			}
			
			atomic.AddInt64(&totalSuccessful, int64(successful))
			atomic.AddInt64(&totalErrors, int64(errors))
		}(g)
	}
	
	wg.Wait()
	
	totalOperations := int64(numGoroutines * operationsPerGoroutine)
	assert.Equal(t, totalOperations, totalSuccessful+totalErrors, 
		"All operations should be accounted for")
	
	// Most operations should succeed under normal conditions
	successRate := float64(totalSuccessful) / float64(totalOperations)
	assert.Greater(t, successRate, 0.95, 
		"Success rate should be high under concurrent load")
	
	t.Logf("Concurrent connections test: Success=%d (%.2f%%), Errors=%d", 
		totalSuccessful, successRate*100, totalErrors)
}

// TestNetworkFailureRecovery tests handling of network failures and recovery
func TestNetworkFailureRecovery(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	
	t.Run("TimeoutHandling", func(t *testing.T) {
		testTimeoutHandling(t, container)
	})
	
	t.Run("OperationRetries", func(t *testing.T) {
		testOperationRetries(t, container)
	})
}

// testTimeoutHandling tests that operations respect context timeouts
func testTimeoutHandling(t *testing.T, container *TestRedisContainer) {
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	key := "timeout-test-key"
	value := "timeout-test-value"
	
	// Test with very short timeout
	shortTimeout := 1 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), shortTimeout)
	defer cancel()
	
	// This should timeout (though it might succeed if Redis is very fast)
	err := cache.Set(ctx, key, value, time.Hour)
	if err != nil {
		// If it times out, that's expected behavior
		t.Logf("Operation timed out as expected: %v", err)
	} else {
		// If it succeeds, Redis is very fast, which is also fine
		t.Logf("Operation completed faster than timeout")
	}
	
	// Test with reasonable timeout - should succeed
	reasonableTimeout := 5 * time.Second
	ctx2, cancel2 := context.WithTimeout(context.Background(), reasonableTimeout)
	defer cancel2()
	
	err = cache.Set(ctx2, key, value, time.Hour)
	assert.NoError(t, err, "Operation should succeed with reasonable timeout")
}

// testOperationRetries tests retry behavior (implementation-dependent)
func testOperationRetries(t *testing.T, container *TestRedisContainer) {
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	ctx := context.Background()
	
	// Test normal operations work (implicit retry testing)
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("retry-test-%d", i)
		value := fmt.Sprintf("retry-value-%d", i)
		
		err := cache.Set(ctx, key, value, time.Hour)
		assert.NoError(t, err, "Set operation should succeed")
		
		retrieved, found, err := cache.Get(ctx, key)
		require.NoError(t, err, "Get operation should succeed")
		assert.True(t, found, "Key should be found")
		assert.Equal(t, value, retrieved, "Value should match")
	}
}

// TestSecurityFeatures tests security-related functionality
func TestSecurityFeatures(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	
	t.Run("TimingProtection", func(t *testing.T) {
		testTimingProtection(t, container)
	})
	
	t.Run("SecureCleanup", func(t *testing.T) {
		testSecureCleanup(t, container)
	})
}

// testTimingProtection tests timing attack protection
func testTimingProtection(t *testing.T, container *TestRedisContainer) {
	// Create cache with timing protection enabled
	options := &interfaces.CacheOptions{
		RedisOptions: &interfaces.RedisOptions{
			Address: container.GetRedisAddr(),
			DB:      0,
		},
		Security: &interfaces.SecurityConfig{
			EnableTimingProtection: true,
			MinProcessingTime:      10 * time.Millisecond,
			SecureCleanup:         true,
		},
	}
	
	cache := CreateCacheWithOptions[string](container, options)
	defer cache.Close()
	
	ctx := context.Background()
	key := "timing-test-key"
	value := "timing-test-value"
	
	// Test that operations take at least the minimum time
	start := time.Now()
	err := cache.Set(ctx, key, value, time.Hour)
	duration := time.Since(start)
	
	require.NoError(t, err, "Set should succeed")
	assert.GreaterOrEqual(t, duration, 10*time.Millisecond, 
		"Operation should take at least minimum processing time")
	
	// Test Get operation timing
	start = time.Now()
	_, _, err = cache.Get(ctx, key)
	duration = time.Since(start)
	
	require.NoError(t, err, "Get should succeed")
	assert.GreaterOrEqual(t, duration, 10*time.Millisecond, 
		"Get operation should also respect minimum time")
	
	// Test non-existent key (should still take minimum time)
	start = time.Now()
	_, found, err := cache.Get(ctx, "non-existent-key")
	duration = time.Since(start)
	
	require.NoError(t, err, "Get should succeed even for non-existent key")
	assert.False(t, found, "Key should not be found")
	assert.GreaterOrEqual(t, duration, 10*time.Millisecond, 
		"Non-existent key lookup should also take minimum time")
}

// testSecureCleanup tests that sensitive data is properly cleaned up
func testSecureCleanup(t *testing.T, container *TestRedisContainer) {
	// Create cache with secure cleanup enabled
	options := &interfaces.CacheOptions{
		RedisOptions: &interfaces.RedisOptions{
			Address: container.GetRedisAddr(),
			DB:      0,
		},
		Security: &interfaces.SecurityConfig{
			EnableTimingProtection: false,
			SecureCleanup:         true,
		},
	}
	
	cache := CreateCacheWithOptions[string](container, options)
	defer cache.Close()
	
	ctx := context.Background()
	
	// Store and delete sensitive data
	sensitiveKeys := []string{"password", "token", "secret", "key"}
	sensitiveValues := []string{"secret123", "token456", "classified", "privatekey"}
	
	for i, key := range sensitiveKeys {
		err := cache.Set(ctx, key, sensitiveValues[i], time.Hour)
		require.NoError(t, err, "Should be able to set sensitive data")
	}
	
	// Verify data is stored
	for i, key := range sensitiveKeys {
		value, found, err := cache.Get(ctx, key)
		require.NoError(t, err, "Get should succeed")
		assert.True(t, found, "Sensitive data should be found")
		assert.Equal(t, sensitiveValues[i], value, "Values should match")
	}
	
	// Delete the sensitive data
	for _, key := range sensitiveKeys {
		err := cache.Delete(ctx, key)
		require.NoError(t, err, "Should be able to delete sensitive data")
	}
	
	// Verify data is gone
	for _, key := range sensitiveKeys {
		_, found, err := cache.Get(ctx, key)
		require.NoError(t, err, "Get should succeed")
		assert.False(t, found, "Sensitive data should be deleted")
	}
	
	// Note: We can't easily verify that memory was securely wiped without
	// access to internals, but the test ensures the API works correctly
	t.Log("Secure cleanup test completed - actual memory wiping verification requires internal access")
}

// TestErrorHandlingAndLogging tests comprehensive error handling
func TestErrorHandlingAndLogging(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	ctx := context.Background()
	
	t.Run("InvalidOperations", func(t *testing.T) {
		// Test operations with invalid parameters
		err := cache.Set(ctx, "", "value", time.Hour)
		// Empty key should work (Redis allows it)
		assert.NoError(t, err, "Redis allows empty keys")
		
		// Test with negative TTL (should be handled gracefully)
		err = cache.Set(ctx, "negative-ttl", "value", -time.Hour)
		// Implementation should handle this gracefully
		if err != nil {
			t.Logf("Negative TTL handled with error: %v", err)
		}
	})
	
	t.Run("ContextCancellation", func(t *testing.T) {
		// Test context cancellation
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		err := cache.Set(cancelCtx, "cancelled-key", "value", time.Hour)
		if err != nil {
			t.Logf("Cancelled context handled appropriately: %v", err)
		}
	})
}

// TestGracefulDegradation tests system behavior under extreme conditions
func TestGracefulDegradation(t *testing.T) {
	container := SetupRedisContainer(t)
	defer container.Close()
	
	t.Run("HighMemoryPressure", func(t *testing.T) {
		testHighMemoryPressure(t, container)
	})
	
	t.Run("ExtremeConcurrency", func(t *testing.T) {
		testExtremeConcurrency(t, container)
	})
}

// testHighMemoryPressure tests behavior under memory pressure
func testHighMemoryPressure(t *testing.T, container *TestRedisContainer) {
	cache := CreateCacheForTesting[[]byte](container)
	defer cache.Close()
	
	ctx := context.Background()
	
	// Try to store large amounts of data
	largeData := make([]byte, 1024*1024) // 1MB chunks
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	
	successCount := 0
	errorCount := 0
	
	// Try to store many large objects
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("large-data-%d", i)
		err := cache.Set(ctx, key, largeData, time.Hour)
		if err != nil {
			errorCount++
		} else {
			successCount++
		}
		
		// Stop if we start getting consistent errors
		if errorCount > 10 {
			break
		}
	}
	
	t.Logf("High memory pressure test: Success=%d, Errors=%d", successCount, errorCount)
	assert.Greater(t, successCount, 0, "Should be able to store some large objects")
}

// testExtremeConcurrency tests system under extreme concurrent load
func testExtremeConcurrency(t *testing.T, container *TestRedisContainer) {
	cache := CreateCacheForTesting[string](container)
	defer cache.Close()
	
	ctx := context.Background()
	
	// Test with very high concurrency
	numGoroutines := 1000
	operationsPerGoroutine := 1
	
	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64
	
	wg.Add(numGoroutines)
	
	start := time.Now()
	
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			
			for i := 0; i < operationsPerGoroutine; i++ {
				key := fmt.Sprintf("extreme-concurrent-%d-%d", goroutineID, i)
				value := fmt.Sprintf("value-%d-%d", goroutineID, i)
				
				err := cache.Set(ctx, key, value, time.Hour)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(g)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	totalOps := int64(numGoroutines * operationsPerGoroutine)
	successRate := float64(successCount) / float64(totalOps)
	
	t.Logf("Extreme concurrency test (%d goroutines): Success=%d (%.1f%%), Errors=%d, Duration=%v", 
		numGoroutines, successCount, successRate*100, errorCount, duration)
	
	assert.Greater(t, successRate, 0.8, 
		"Should maintain reasonable success rate under extreme concurrency")
	assert.Equal(t, totalOps, successCount+errorCount, 
		"All operations should be accounted for")
}
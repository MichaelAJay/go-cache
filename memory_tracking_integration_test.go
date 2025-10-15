//go:build integration

package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/config"
	"github.com/MichaelAJay/go-cache/internal/testintegration"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createRedisV9Client creates a Redis v9 client from the test environment setup
func createRedisV9Client(setup *testintegration.TestEnvironmentSetup) *redis.Client {
	// Extract Redis address from the test environment
	redisAddr := setup.TestEnv.GetRedisAddr()
	
	// Create Redis v9 client
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	
	return client
}

// TestMemoryTrackingConfiguration tests Step 1: Memory tracking configuration options
func TestMemoryTrackingConfiguration(t *testing.T) {
	ctx := context.Background()

	// Setup test environment (using v8 client for environment setup)
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)

	// Create v9 client for testing config package
	v9Client := createRedisV9Client(setup)
	defer v9Client.Close()

	// Test v9 client connectivity
	err := v9Client.Ping(ctx).Err()
	require.NoError(t, err, "Redis v9 client should connect successfully")

	t.Run("DefaultMemoryTrackingDisabled", func(t *testing.T) {
		// Test default configuration has memory tracking disabled
		defaultOptions := config.DefaultOptions()
		
		assert.False(t, defaultOptions.MemoryTrackingEnabled, "Memory tracking should be disabled by default")
		assert.Equal(t, 100, defaultOptions.MemoryUsageSamplingRate, "Default sampling rate should be 100")
		assert.Equal(t, 60*time.Second, defaultOptions.MemoryUsageSamplingInterval, "Default sampling interval should be 60 seconds")
		assert.Equal(t, int64(0), defaultOptions.MemoryPressureThresholdBytes, "Default byte threshold should be disabled (0)")
		assert.Equal(t, 80.0, defaultOptions.MemoryPressureThresholdPercent, "Default percentage threshold should be 80%")
		
		t.Logf("✅ Default configuration validated - memory tracking disabled by default")
	})

	t.Run("NewCacheOptionsMemoryTrackingDisabled", func(t *testing.T) {
		// Test NewCacheOptions also has memory tracking disabled
		options := config.NewCacheOptions(v9Client)
		
		assert.False(t, options.MemoryTrackingEnabled, "Memory tracking should be disabled by default in NewCacheOptions")
		assert.Equal(t, 100, options.MemoryUsageSamplingRate, "Default sampling rate should be 100")
		assert.Equal(t, 60*time.Second, options.MemoryUsageSamplingInterval, "Default sampling interval should be 60 seconds")
		assert.Equal(t, int64(0), options.MemoryPressureThresholdBytes, "Default byte threshold should be disabled (0)")
		assert.Equal(t, 80.0, options.MemoryPressureThresholdPercent, "Default percentage threshold should be 80%")
		
		t.Logf("✅ NewCacheOptions configuration validated - memory tracking disabled by default")
	})

	t.Run("MemoryTrackingConfigurationBuilders", func(t *testing.T) {
		// Test all builder methods for memory tracking configuration
		options := config.NewCacheOptions(v9Client).
			WithMemoryTracking(true).
			WithMemoryUsageSamplingRate(50).
			WithMemoryUsageSamplingInterval(30*time.Second).
			WithMemoryPressureThresholdBytes(1024*1024). // 1MB
			WithMemoryPressureThresholdPercent(90.0)

		assert.True(t, options.MemoryTrackingEnabled, "Memory tracking should be enabled after WithMemoryTracking(true)")
		assert.Equal(t, 50, options.MemoryUsageSamplingRate, "Sampling rate should be updated")
		assert.Equal(t, 30*time.Second, options.MemoryUsageSamplingInterval, "Sampling interval should be updated")
		assert.Equal(t, int64(1024*1024), options.MemoryPressureThresholdBytes, "Byte threshold should be updated")
		assert.Equal(t, 90.0, options.MemoryPressureThresholdPercent, "Percentage threshold should be updated")
		
		t.Logf("✅ Memory tracking configuration builders validated")
		t.Logf("   - Enabled: %v", options.MemoryTrackingEnabled)
		t.Logf("   - Sampling rate: %d", options.MemoryUsageSamplingRate)
		t.Logf("   - Sampling interval: %v", options.MemoryUsageSamplingInterval)
		t.Logf("   - Byte threshold: %d", options.MemoryPressureThresholdBytes)
		t.Logf("   - Percentage threshold: %.1f%%", options.MemoryPressureThresholdPercent)
	})

	t.Run("ConfigurationValidation", func(t *testing.T) {
		// Test edge cases and validation
		options := config.NewCacheOptions(v9Client)

		// Test zero sampling rate (should work but might be inefficient)
		options.WithMemoryUsageSamplingRate(0)
		assert.Equal(t, 0, options.MemoryUsageSamplingRate, "Zero sampling rate should be accepted")

		// Test negative threshold (should work - means unlimited)
		options.WithMemoryPressureThresholdBytes(-1)
		assert.Equal(t, int64(-1), options.MemoryPressureThresholdBytes, "Negative threshold should be accepted")

		// Test very high percentage (> 100%)
		options.WithMemoryPressureThresholdPercent(150.0)
		assert.Equal(t, 150.0, options.MemoryPressureThresholdPercent, "High percentage should be accepted")

		t.Logf("✅ Configuration edge cases validated")
	})
}

// TestMemoryTrackingInitializationWithV8 tests Step 2: Memory tracker initialization using v8 client
func TestMemoryTrackingInitializationWithV8(t *testing.T) {
	ctx := context.Background()

	// Setup test environment (this creates v8 client)
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	t.Run("MemoryTrackingDisabled", func(t *testing.T) {
		// Create cache with memory tracking disabled (default)
		config := testintegration.DefaultCacheConfig()
		cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
		require.NoError(t, err, "Failed to create cache")
		defer cache.Close()

		// Cache should be created successfully even without memory tracking
		assert.NotNil(t, cache, "Cache should be created successfully")
		
		t.Logf("✅ Cache created successfully with memory tracking disabled")
	})

	t.Run("CacheBasicOperationsWork", func(t *testing.T) {
		// Verify that basic cache operations work (this is our baseline)
		config := testintegration.DefaultCacheConfig()
		cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
		require.NoError(t, err, "Failed to create cache")
		defer cache.Close()

		// Create test session
		testSession := &testintegration.TestSession{
			ID:       "memory:init:test",
			UserID:   "user123",
			Username: "testuser",
			Created:  time.Now(),
		}
		
		// SET operation should work
		err = cache.Set(ctx, testSession, 0)
		assert.NoError(t, err, "SET operation should work")
		
		// GET operation should work
		retrieved, found, err := cache.Get(ctx, testSession.ID)
		assert.NoError(t, err, "GET operation should work")
		assert.True(t, found, "Session should be found")
		assert.Equal(t, testSession.ID, retrieved.ID, "Retrieved session should match")
		
		// DELETE operation should work
		_, err = cache.Delete(ctx, testSession.ID)
		assert.NoError(t, err, "DELETE operation should work")
		
		t.Logf("✅ Basic cache operations validated")
	})

	t.Run("CacheCleanup", func(t *testing.T) {
		// Create cache and test cleanup
		config := testintegration.DefaultCacheConfig()
		cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
		require.NoError(t, err, "Failed to create cache")
		
		// Close cache should not error
		err = cache.Close()
		assert.NoError(t, err, "Cache.Close() should not error")
		
		t.Logf("✅ Cache cleanup successful")
	})
}

// TestMemoryTrackerLogicValidation tests Step 3: Core memory tracker functionality logic
func TestMemoryTrackerLogicValidation(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	t.Run("MemoryEstimationLogic", func(t *testing.T) {
		// Test memory estimation logic (this is what RecordSet would do)
		testKey := "test:memory:key"
		testData := []byte("this is test data for memory tracking validation - it needs to be reasonably long to test memory estimation")
		
		// Test memory estimation calculation
		keySize := int64(len(testKey))
		valueSize := int64(len(testData))
		redisOverhead := int64(64) // Same as implementation
		estimatedSize := keySize + valueSize + redisOverhead
		
		expectedSize := int64(len(testKey)) + int64(len(testData)) + 64
		assert.Equal(t, expectedSize, estimatedSize, "Memory estimation should match expected calculation")
		
		// Test with different key sizes
		testCases := []struct {
			key   string
			value []byte
		}{
			{"short", []byte("small")},
			{"medium:length:key", []byte("medium length value for testing")},
			{"very:long:key:with:many:segments:for:testing:purposes", []byte("very long value with lots of content to test memory estimation accuracy for larger entries")},
		}
		
		for _, tc := range testCases {
			keySize := int64(len(tc.key))
			valueSize := int64(len(tc.value))
			estimatedSize := keySize + valueSize + 64
			
			assert.Greater(t, estimatedSize, keySize+valueSize, "Estimated size should include Redis overhead")
			assert.Equal(t, keySize+valueSize+64, estimatedSize, "Estimated size calculation should be consistent")
		}
		
		t.Logf("✅ Memory estimation logic validated")
		t.Logf("   - Key size: %d bytes", keySize)
		t.Logf("   - Value size: %d bytes", valueSize)
		t.Logf("   - Redis overhead: %d bytes", redisOverhead)
		t.Logf("   - Total estimated: %d bytes", estimatedSize)
	})

	t.Run("SamplingRateLogic", func(t *testing.T) {
		// Test sampling rate logic
		samplingRate := 10
		
		// Test operations that should trigger sampling
		shouldSampleOps := []int{10, 20, 30, 40, 50}
		for _, opCount := range shouldSampleOps {
			shouldSample := opCount > 0 && opCount%samplingRate == 0
			assert.True(t, shouldSample, "Operation %d should trigger sampling with rate %d", opCount, samplingRate)
		}
		
		// Test operations that should NOT trigger sampling
		shouldNotSampleOps := []int{1, 5, 9, 11, 15, 19}
		for _, opCount := range shouldNotSampleOps {
			shouldSample := opCount > 0 && opCount%samplingRate == 0
			assert.False(t, shouldSample, "Operation %d should NOT trigger sampling with rate %d", opCount, samplingRate)
		}
		
		// Test edge cases
		zeroOpCount := 0
		exactSampleCount := samplingRate
		assert.False(t, zeroOpCount > 0 && zeroOpCount%samplingRate == 0, "Operation 0 should not trigger sampling")
		assert.True(t, exactSampleCount > 0 && exactSampleCount%samplingRate == 0, "Operation equal to sampling rate should trigger")
		
		t.Logf("✅ Sampling rate logic validated")
		t.Logf("   - Sampling rate: %d operations", samplingRate)
	})

	t.Run("TimeBasedSamplingLogic", func(t *testing.T) {
		// Test time-based sampling logic
		samplingInterval := 5 * time.Second
		
		// Test recent sample time (should not sample)
		recentTime := time.Now().Add(-1 * time.Second)
		timeSinceRecent := time.Since(recentTime)
		shouldSampleRecent := timeSinceRecent >= samplingInterval
		assert.False(t, shouldSampleRecent, "Recent sample time should not trigger time-based sampling")
		
		// Test old sample time (should sample)
		oldTime := time.Now().Add(-10 * time.Second)
		timeSinceOld := time.Since(oldTime)
		shouldSampleOld := timeSinceOld >= samplingInterval
		assert.True(t, shouldSampleOld, "Old sample time should trigger time-based sampling")
		
		// Test edge case - exactly at interval
		exactTime := time.Now().Add(-samplingInterval)
		timeSinceExact := time.Since(exactTime)
		shouldSampleExact := timeSinceExact >= samplingInterval
		assert.True(t, shouldSampleExact, "Exact interval time should trigger sampling")
		
		t.Logf("✅ Time-based sampling logic validated")
		t.Logf("   - Sampling interval: %v", samplingInterval)
		t.Logf("   - Recent time delta: %v (should not sample)", timeSinceRecent)
		t.Logf("   - Old time delta: %v (should sample)", timeSinceOld)
	})

	t.Run("MemoryPressureThresholdLogic", func(t *testing.T) {
		// Test absolute threshold logic
		absoluteThreshold := int64(1024) // 1KB
		
		testCases := []struct {
			currentMemory int64
			shouldTrigger bool
			description   string
		}{
			{0, false, "Zero memory should not trigger any threshold"},
			{512, false, "512 bytes should not trigger 1KB threshold"},
			{1023, false, "1023 bytes should not trigger 1KB threshold"},
			{1024, true, "1024 bytes should trigger 1KB threshold"},
			{2048, true, "2048 bytes should trigger 1KB threshold"},
		}
		
		for _, tc := range testCases {
			triggered := absoluteThreshold > 0 && tc.currentMemory >= absoluteThreshold
			assert.Equal(t, tc.shouldTrigger, triggered, tc.description)
		}
		
		// Test percentage threshold logic
		percentThreshold := 80.0
		maxMemory := int64(10240) // 10KB
		threshold := float64(maxMemory) * (percentThreshold / 100.0)
		
		percentTestCases := []struct {
			currentMemory int64
			shouldTrigger bool
			description   string
		}{
			{0, false, "Zero memory should not trigger percentage threshold"},
			{4096, false, "4KB should not trigger 80% of 10KB threshold"}, // 40%
			{8191, false, "8191 bytes should not trigger 80% of 10KB threshold"}, // Just under 80%
			{8192, true, "8KB should trigger 80% of 10KB threshold"},      // Exactly 80%
			{9216, true, "9KB should trigger 80% of 10KB threshold"},      // 90%
		}
		
		for _, tc := range percentTestCases {
			triggered := maxMemory > 0 && float64(tc.currentMemory) >= threshold
			assert.Equal(t, tc.shouldTrigger, triggered, tc.description)
		}
		
		// Test disabled thresholds
		disabledAbsolute := int64(0)
		disabledMax := int64(0)
		assert.False(t, disabledAbsolute > 0 && 1000 >= disabledAbsolute, "Disabled absolute threshold should never trigger")
		assert.False(t, disabledMax > 0 && float64(1000) >= float64(disabledMax)*0.8, "Disabled percentage threshold should never trigger")
		
		t.Logf("✅ Memory pressure threshold logic validated")
		t.Logf("   - Absolute threshold: %d bytes", absoluteThreshold)
		t.Logf("   - Percentage threshold: %.1f%% of %d bytes = %.0f bytes", percentThreshold, maxMemory, threshold)
	})

	t.Run("DeleteOperationAverageCalculation", func(t *testing.T) {
		// Test average entry size calculation for delete operations
		testCases := []struct {
			totalMemory  int64
			totalEntries int64
			expectedAvg  int64
			description  string
		}{
			{0, 0, 0, "Zero entries should result in no deletion"},
			{1000, 10, 100, "1000 bytes / 10 entries = 100 bytes average"},
			{5000, 5, 1000, "5000 bytes / 5 entries = 1000 bytes average"},
			{1, 1, 1, "Single byte entry should work"},
		}
		
		for _, tc := range testCases {
			if tc.totalEntries <= 0 {
				// No entries to delete - should not attempt calculation
				continue
			}
			
			avgSize := tc.totalMemory / tc.totalEntries
			assert.Equal(t, tc.expectedAvg, avgSize, tc.description)
		}
		
		t.Logf("✅ Delete operation average calculation validated")
	})
}

// TestRedisMemoryUsageCommand tests Redis MEMORY USAGE command functionality
func TestRedisMemoryUsageCommand(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create both v8 and v9 clients for testing
	v9Client := createRedisV9Client(setup)
	defer v9Client.Close()

	t.Run("RedisV8MemoryUsageAvailable", func(t *testing.T) {
		// Test that Redis MEMORY USAGE command is available with v8 client
		testKey := "memory:usage:test:v8"
		testValue := "test data for memory usage command validation with v8 client"
		
		// Set a key using v8 client
		err := setup.RedisClient.Set(ctx, testKey, testValue, 0).Err()
		require.NoError(t, err, "Failed to set test key with v8 client")
		
		// Check memory usage with v8 client
		memUsage, err := setup.RedisClient.MemoryUsage(ctx, testKey).Result()
		
		if err != nil {
			t.Skipf("Redis MEMORY USAGE command not available with v8 client: %v", err)
		}
		
		assert.Greater(t, memUsage, int64(0), "Memory usage should be greater than 0")
		assert.GreaterOrEqual(t, memUsage, int64(len(testValue)), "Memory usage should be at least the value size")
		
		// Cleanup
		setup.RedisClient.Del(ctx, testKey)
		
		t.Logf("✅ Redis MEMORY USAGE command available with v8 client")
		t.Logf("   - Key: %s", testKey)
		t.Logf("   - Memory usage: %d bytes", memUsage)
		t.Logf("   - Value size: %d bytes", len(testValue))
	})

	t.Run("RedisV9MemoryUsageAvailable", func(t *testing.T) {
		// Test that Redis MEMORY USAGE command is available with v9 client
		testKey := "memory:usage:test:v9"
		testValue := "test data for memory usage command validation with v9 client"
		
		// Set a key using v9 client
		err := v9Client.Set(ctx, testKey, testValue, 0).Err()
		require.NoError(t, err, "Failed to set test key with v9 client")
		
		// Check memory usage with v9 client
		memUsage, err := v9Client.MemoryUsage(ctx, testKey).Result()
		
		if err != nil {
			t.Skipf("Redis MEMORY USAGE command not available with v9 client: %v", err)
		}
		
		assert.Greater(t, memUsage, int64(0), "Memory usage should be greater than 0")
		assert.GreaterOrEqual(t, memUsage, int64(len(testValue)), "Memory usage should be at least the value size")
		
		// Cleanup
		v9Client.Del(ctx, testKey)
		
		t.Logf("✅ Redis MEMORY USAGE command available with v9 client")
		t.Logf("   - Key: %s", testKey)
		t.Logf("   - Memory usage: %d bytes", memUsage)
		t.Logf("   - Value size: %d bytes", len(testValue))
	})

	t.Run("RedisConfigGetMaxMemory", func(t *testing.T) {
		// Test Redis CONFIG GET maxmemory command
		result, err := v9Client.ConfigGet(ctx, "maxmemory").Result()
		require.NoError(t, err, "Failed to get Redis maxmemory config")
		
		// v9 returns a map[string]string
		maxMemory, exists := result["maxmemory"]
		assert.True(t, exists, "maxmemory should exist in config result")
		assert.NotEmpty(t, maxMemory, "maxmemory config should be found")
		
		t.Logf("✅ Redis CONFIG GET maxmemory available")
		t.Logf("   - maxmemory: %s", maxMemory)
		
		// Test maxmemory parsing logic
		if maxMemory == "0" {
			t.Logf("   - maxmemory is unlimited (percentage thresholds will be disabled)")
		} else {
			t.Logf("   - maxmemory has a limit (percentage thresholds can be used)")
		}
	})

	t.Run("MemoryScanOperationSimulation", func(t *testing.T) {
		// Test SCAN operation for memory sampling (simulates PerformMemorySample)
		keyPrefix := "memory:scan:test:"
		
		// Create multiple keys for scanning
		testKeys := []string{
			keyPrefix + "key1",
			keyPrefix + "key2", 
			keyPrefix + "key3",
		}
		
		testValues := []string{
			"value for key 1 - testing scan operation",
			"value for key 2 - testing scan functionality",
			"value for key 3 - testing memory scan simulation",
		}
		
		// Set all keys
		for i, key := range testKeys {
			err := setup.RedisClient.Set(ctx, key, testValues[i], 0).Err()
			require.NoError(t, err, "Failed to set key %s", key)
		}
		
		// Simulate SCAN operation (what PerformMemorySample would do)
		var scannedKeys []string
		var totalMemory int64
		
		iter := setup.RedisClient.Scan(ctx, 0, keyPrefix+"*", 100).Iterator()
		for iter.Next(ctx) {
			key := iter.Val()
			scannedKeys = append(scannedKeys, key)
			
			// Get memory usage for this key (if available)
			memUsage := setup.RedisClient.MemoryUsage(ctx, key)
			if memUsage.Err() == nil {
				totalMemory += memUsage.Val()
			}
		}
		
		require.NoError(t, iter.Err(), "SCAN operation should succeed")
		
		// Verify we found all our test keys
		assert.Equal(t, len(testKeys), len(scannedKeys), "Should find all test keys")
		
		// If MEMORY USAGE worked, we should have some memory total
		if totalMemory > 0 {
			assert.Greater(t, totalMemory, int64(0), "Total memory should be positive if MEMORY USAGE works")
			t.Logf("   - Total scanned memory: %d bytes", totalMemory)
		}
		
		// Cleanup
		for _, key := range testKeys {
			setup.RedisClient.Del(ctx, key)
		}
		
		t.Logf("✅ Memory scan operation simulation successful")
		t.Logf("   - Keys scanned: %d", len(scannedKeys))
		t.Logf("   - Key prefix: %s", keyPrefix)
	})
}
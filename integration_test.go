//go:build integration
// +build integration

package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/internal/testenv"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheIntegration_ContainerModes(t *testing.T) {
	ctx := context.Background()

	// Create test environment based on environment variables
	testEnv, err := testenv.NewTestEnvironmentFromEnv(ctx)
	require.NoError(t, err, "Failed to create test environment")
	defer testEnv.Close()

	t.Run("RedisConnection", func(t *testing.T) {
		// Test basic Redis connection
		rdb := redis.NewClient(&redis.Options{
			Addr: testEnv.GetRedisAddr(),
		})
		defer rdb.Close()

		// Test basic operations
		err := rdb.Set(ctx, "test:key", "test:value", time.Minute).Err()
		assert.NoError(t, err, "Failed to set Redis key")

		val, err := rdb.Get(ctx, "test:key").Result()
		assert.NoError(t, err, "Failed to get Redis key")
		assert.Equal(t, "test:value", val, "Unexpected Redis value")

		// Cleanup
		err = rdb.Del(ctx, "test:key").Err()
		assert.NoError(t, err, "Failed to delete Redis key")
	})

	t.Run("ToxiproxyLatency", func(t *testing.T) {
		if !testEnv.HasToxiproxy() {
			t.Skip("Toxiproxy not available in this test mode")
		}

		toxiController := testEnv.GetToxiproxyController()
		require.NotNil(t, toxiController, "Toxiproxy controller should be available")

		// Test adding latency
		err := toxiController.AddLatencyToxic(ctx, "redis_proxy", 50)
		assert.NoError(t, err, "Failed to add latency toxic")

		// Test Redis operations with latency
		rdb := redis.NewClient(&redis.Options{
			Addr: testEnv.GetRedisAddr(),
		})
		defer rdb.Close()

		// Measure operation time
		start := time.Now()
		err = rdb.Set(ctx, "test:latency", "value", time.Minute).Err()
		duration := time.Since(start)

		assert.NoError(t, err, "Redis operation should work with latency")
		// Should take at least 50ms due to added latency
		assert.GreaterOrEqual(t, duration.Milliseconds(), int64(40), "Operation should be slower with latency")

		// Cleanup
		err = rdb.Del(ctx, "test:latency").Err()
		assert.NoError(t, err, "Failed to cleanup test key")

		// Remove latency toxic
		err = toxiController.RemoveToxic(ctx, "redis_proxy", "redis_proxy_latency")
		assert.NoError(t, err, "Failed to remove latency toxic")
	})
}

func TestCacheIntegration_TestModes(t *testing.T) {
	tests := []struct {
		name string
		mode testenv.TestMode
	}{
		{"Direct", testenv.ModeDirect},
		{"Containers", testenv.ModeContainers},
		{"Compose", testenv.ModeCompose},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Skip containers test if we're not in the right environment
			if tt.mode == testenv.ModeContainers {
				mode := testenv.GetTestModeFromEnv()
				if mode != testenv.ModeContainers {
					t.Skip("Container mode test skipped - set GOCACHE_TEST_MODE=containers")
				}
			}

			testEnv, err := testenv.NewTestEnvironment(ctx, tt.mode)
			if err != nil {
				if tt.mode == testenv.ModeDirect {
					t.Skipf("Direct mode test skipped - Redis not available: %v", err)
				} else {
					require.NoError(t, err, "Failed to create test environment")
				}
				return
			}
			defer testEnv.Close()

			// Test Redis operations
			rdb := redis.NewClient(&redis.Options{
				Addr: testEnv.GetRedisAddr(),
			})
			defer rdb.Close()

			// Basic functionality test
			key := "test:mode:" + tt.name
			value := "value:" + tt.name

			err = rdb.Set(ctx, key, value, time.Minute).Err()
			assert.NoError(t, err, "Failed to set key in %s mode", tt.name)

			retrievedValue, err := rdb.Get(ctx, key).Result()
			assert.NoError(t, err, "Failed to get key in %s mode", tt.name)
			assert.Equal(t, value, retrievedValue, "Value mismatch in %s mode", tt.name)

			// Cleanup
			err = rdb.Del(ctx, key).Err()
			assert.NoError(t, err, "Failed to cleanup key in %s mode", tt.name)
		})
	}
}
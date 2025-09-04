package testintegration

import (
	"context"
	"testing"

	"github.com/MichaelAJay/go-cache/internal/testenv"
	"github.com/go-redis/redis/v8"
)

// TestEnvironmentSetup encapsulates test environment configuration
type TestEnvironmentSetup struct {
	TestEnv   *testenv.TestEnvironment
	RedisClient redis.Cmdable
}

// SetupTestEnvironment creates and configures a test environment for Redis cache integration tests
func SetupTestEnvironment(ctx context.Context, t *testing.T) *TestEnvironmentSetup {
	t.Helper()

	// Create test environment from environment variables
	testEnv, err := testenv.NewTestEnvironmentFromEnv(ctx)
	if err != nil {
		t.Fatalf("Failed to create test environment: %v", err)
	}

	// Ensure cleanup happens when test completes
	t.Cleanup(func() {
		testEnv.Close()
	})

	// Create Redis client for the test environment
	redisClient := redis.NewClient(&redis.Options{
		Addr: testEnv.GetRedisAddr(),
	})

	// Verify Redis connectivity
	if err := redisClient.Ping(ctx).Err(); err != nil {
		testEnv.Close()
		t.Fatalf("Failed to connect to Redis at %s: %v", testEnv.GetRedisAddr(), err)
	}

	t.Logf("Test environment ready: mode=%s, redis=%s", 
		testEnv.GetMode().String(), testEnv.GetRedisAddr())

	return &TestEnvironmentSetup{
		TestEnv:     testEnv,
		RedisClient: redisClient,
	}
}

// FlushRedis clears all Redis data for clean test state
func (setup *TestEnvironmentSetup) FlushRedis(ctx context.Context, t *testing.T) {
	t.Helper()
	
	if err := setup.RedisClient.FlushAll(ctx).Err(); err != nil {
		t.Fatalf("Failed to flush Redis: %v", err)
	}
}

// ValidateEnvironment ensures the test environment is properly configured
func (setup *TestEnvironmentSetup) ValidateEnvironment(ctx context.Context, t *testing.T) {
	t.Helper()

	// Test basic Redis operations
	testKey := "integration:test:ping"
	testValue := "test-value"

	// SET operation
	if err := setup.RedisClient.Set(ctx, testKey, testValue, 0).Err(); err != nil {
		t.Fatalf("Redis SET validation failed: %v", err)
	}

	// GET operation
	result, err := setup.RedisClient.Get(ctx, testKey).Result()
	if err != nil {
		t.Fatalf("Redis GET validation failed: %v", err)
	}
	if result != testValue {
		t.Fatalf("Redis validation mismatch: expected %q, got %q", testValue, result)
	}

	// Cleanup test key
	setup.RedisClient.Del(ctx, testKey)

	t.Log("Test environment validation successful")
}
package redis_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/providers/redis"
	"github.com/MichaelAJay/go-logger"
	"github.com/MichaelAJay/go-serializer"
)

// getLocalRedisAddr is kept for backward compatibility
// It now just calls getRedisAddr() from redis_test.go
func getLocalRedisAddr() string {
	return getRedisAddr()
}

// TestLoadFromEnvironment tests loading Redis options from environment variables
func TestLoadFromEnvironment(t *testing.T) {
	// Skip test if Redis is not available
	client := SkipIfRedisUnavailable(t)
	if client != nil {
		client.Close()
	}

	// Save original environment values
	origAddr := os.Getenv("REDIS_ADDR")
	origPassword := os.Getenv("REDIS_PASSWORD")
	origDB := os.Getenv("REDIS_DB")
	origPoolSize := os.Getenv("REDIS_POOL_SIZE")

	// Restore environment at the end of the test
	defer func() {
		os.Setenv("REDIS_ADDR", origAddr)
		os.Setenv("REDIS_PASSWORD", origPassword)
		os.Setenv("REDIS_DB", origDB)
		os.Setenv("REDIS_POOL_SIZE", origPoolSize)
	}()

	// Set environment variables for the test
	redisAddr := getLocalRedisAddr()
	os.Setenv("REDIS_ADDR", redisAddr)
	os.Setenv("REDIS_PASSWORD", "")
	os.Setenv("REDIS_DB", "0")
	os.Setenv("REDIS_POOL_SIZE", "3")

	// Create logger
	loggerCfg := logger.Config{
		Level:  logger.InfoLevel,
		Output: os.Stdout,
	}
	log := logger.New(loggerCfg)

	// Create cache options without explicit Redis options
	// This should force loading from environment
	cacheOptions := &cache.CacheOptions{
		TTL:              time.Minute,
		Logger:           log,
		SerializerFormat: serializer.Msgpack,
		// RedisOptions is nil, so it should load from environment
	}

	// Create Redis provider
	provider := redis.NewProvider()

	// Create cache - this should use the environment variables
	redisCache, err := provider.Create(cacheOptions)
	if err != nil {
		t.Fatalf("Failed to create Redis cache from environment: %v", err)
	}
	defer redisCache.Close()

	// Test the cache to ensure it's working
	ctx := context.Background()
	testKey := "env-test-key"
	testValue := "loaded-from-environment"

	// Set a value
	err = redisCache.Set(ctx, testKey, testValue, time.Minute)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Get the value back
	value, found, err := redisCache.Get(ctx, testKey)
	if err != nil {
		t.Fatalf("Failed to get value: %v", err)
	}
	if !found {
		t.Fatal("Expected value to be found")
	}
	if value != testValue {
		t.Errorf("Expected '%s', got '%v'", testValue, value)
	}

	// Clean up
	redisCache.Delete(ctx, testKey)

	// Test with LoadRedisOptionsFromEnv directly
	redisOptions := redis.LoadRedisOptionsFromEnv()
	if redisOptions.Address != redisAddr {
		t.Errorf("Expected address %s, got %s", redisAddr, redisOptions.Address)
	}
	if redisOptions.PoolSize != 3 {
		t.Errorf("Expected pool size 3, got %d", redisOptions.PoolSize)
	}

	// Test with custom environment variable values
	os.Setenv("REDIS_ADDR", "custom.redis.server:6379")
	os.Setenv("REDIS_POOL_SIZE", "15")

	customOptions := redis.LoadRedisOptionsFromEnv()
	if customOptions.Address != "custom.redis.server:6379" {
		t.Errorf("Expected custom address, got %s", customOptions.Address)
	}
	if customOptions.PoolSize != 15 {
		t.Errorf("Expected custom pool size 15, got %d", customOptions.PoolSize)
	}
}

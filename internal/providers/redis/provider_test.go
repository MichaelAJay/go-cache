package redis

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-logger"
	"github.com/MichaelAJay/go-serializer"
	goredis "github.com/go-redis/redis/v8"
)

// getRedisAddr returns the Redis address for testing
func getRedisAddr() string {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	return addr
}

// TestRedisProviderUnit conducts unit tests for the Redis provider without requiring
// an actual Redis connection
func TestRedisProviderUnit(t *testing.T) {
	provider := NewProvider()

	// Test with nil options
	t.Run("NilOptions", func(t *testing.T) {
		_, err := provider.Create(nil)
		if err == nil {
			t.Error("Expected error with nil options")
		}
	})

	// Test with empty Redis options - this will use default addresses from
	// LoadRedisOptionsFromEnv, which sets a default address of "127.0.0.1:6379"
	t.Run("EmptyRedisOptions", func(t *testing.T) {
		// Save original environment variables
		origAddr := os.Getenv("REDIS_ADDR")

		// Unset environment variables to ensure consistent test behavior
		os.Unsetenv("REDIS_ADDR")

		// Restore environment variables after test
		defer func() {
			if origAddr != "" {
				os.Setenv("REDIS_ADDR", origAddr)
			}
		}()

		c, err := provider.Create(&cache.CacheOptions{
			TTL: 0, // No TTL
		})

		// LoadRedisOptionsFromEnv will set a default address of "127.0.0.1:6379"
		// so this should NOT return ErrInvalidRedisOptions.
		// The connection may fail if Redis is not running, but that's a different error.
		if err == ErrInvalidRedisOptions {
			t.Errorf("Got ErrInvalidRedisOptions but should be using default Redis address from environment loader")
		}

		// Note that the cache creation might still fail with a connection error
		// if Redis is not running, but that's expected and not part of this test
		if err == nil && c == nil {
			t.Error("Got nil error but cache is also nil")
		}
	})

	// Test with invalid Redis address
	t.Run("InvalidRedisAddress", func(t *testing.T) {
		_, err := provider.Create(&cache.CacheOptions{
			RedisOptions: &cache.RedisOptions{
				Address: "", // Empty address should cause an error
			},
		})

		if err != ErrInvalidRedisOptions {
			t.Errorf("Expected ErrInvalidRedisOptions, got %v", err)
		}
	})

	// Test with minimal valid options
	t.Run("MinimalValidOptions", func(t *testing.T) {
		// This test doesn't actually connect to Redis, it just makes sure
		// the provider attempts to create a Redis client and cache
		_, err := provider.Create(&cache.CacheOptions{
			RedisOptions: &cache.RedisOptions{
				Address: "localhost:6379", // Valid address format
			},
		})

		// The connection may fail (if Redis isn't running), but the provider
		// should at least attempt to create a cache without returning ErrInvalidRedisOptions
		if err == ErrInvalidRedisOptions {
			t.Error("Provider rejected valid Redis options")
		}
	})
}

// TestProvider_Create tests the cache provider's Create method
func TestProvider_Create(t *testing.T) {
	// Check if Redis is available first
	client := goredis.NewClient(&goredis.Options{
		Addr: getRedisAddr(),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Skipping test: Cannot connect to Redis at %s: %v", getRedisAddr(), err)
	}
	client.Close()

	// Test with valid options
	options := &cache.CacheOptions{
		TTL: time.Minute,
		RedisOptions: &cache.RedisOptions{
			Address:  getRedisAddr(),
			Password: "",
			DB:       0,
			PoolSize: 10,
		},
	}

	provider := NewProvider()
	c, err := provider.Create(options)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	// Test basic operations to ensure it works
	ctx = context.Background()
	err = c.Set(ctx, "test_key", "test_value", time.Hour)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	value, exists, err := c.Get(ctx, "test_key")
	if err != nil || !exists {
		t.Errorf("Get failed: %v", err)
	}
	if value != "test_value" {
		t.Errorf("Expected 'test_value', got %v", value)
	}

	// Test with invalid options
	invalidOptions := &cache.CacheOptions{
		RedisOptions: &cache.RedisOptions{
			Address: "invalid:6379",
		},
	}
	_, err = provider.Create(invalidOptions)
	if err == nil {
		t.Error("Expected error with invalid options")
	}

	// Test with nil options
	_, err = provider.Create(nil)
	if err == nil {
		t.Error("Expected error with nil options")
	}

	// Test with custom logger
	log := logger.New(logger.DefaultConfig)
	optionsWithLogger := &cache.CacheOptions{
		TTL: time.Minute,
		RedisOptions: &cache.RedisOptions{
			Address:  getRedisAddr(),
			Password: "",
			DB:       0,
			PoolSize: 10,
		},
		Logger: log,
	}

	c, err = provider.Create(optionsWithLogger)
	if err != nil {
		t.Fatalf("Failed to create cache with logger: %v", err)
	}
	defer c.Close()

	// Test with enhanced metrics
	optionsWithMetrics := &cache.CacheOptions{
		TTL: time.Minute,
		RedisOptions: &cache.RedisOptions{
			Address:  getRedisAddr(),
			Password: "",
			DB:       0,
			PoolSize: 10,
		},
		MetricsEnabled: true,
	}

	c, err = provider.Create(optionsWithMetrics)
	if err != nil {
		t.Fatalf("Failed to create cache with metrics: %v", err)
	}
	defer c.Close()

	// Test with custom serializer format
	optionsWithSerializer := &cache.CacheOptions{
		TTL: time.Minute,
		RedisOptions: &cache.RedisOptions{
			Address:  getRedisAddr(),
			Password: "",
			DB:       0,
			PoolSize: 10,
		},
		SerializerFormat: serializer.JSON,
	}

	c, err = provider.Create(optionsWithSerializer)
	if err != nil {
		t.Fatalf("Failed to create cache with custom serializer: %v", err)
	}
	defer c.Close()
}

// TestProvider_LoadRedisOptionsFromEnv tests loading Redis options from environment
func TestProvider_LoadRedisOptionsFromEnv(t *testing.T) {
	// Test with default values
	options := LoadRedisOptionsFromEnv()
	if options.Address != "127.0.0.1:6379" {
		t.Errorf("Expected default address '127.0.0.1:6379', got %s", options.Address)
	}
	if options.Password != "" {
		t.Errorf("Expected empty password, got %s", options.Password)
	}
	if options.DB != 0 {
		t.Errorf("Expected default DB 0, got %d", options.DB)
	}
	if options.PoolSize != 10 {
		t.Errorf("Expected default pool size 10, got %d", options.PoolSize)
	}

	// Test with environment variables
	t.Setenv("REDIS_ADDR", "test:6379")
	t.Setenv("REDIS_PASSWORD", "testpass")
	t.Setenv("REDIS_DB", "1")
	t.Setenv("REDIS_POOL_SIZE", "20")

	options = LoadRedisOptionsFromEnv()
	if options.Address != "test:6379" {
		t.Errorf("Expected address 'test:6379', got %s", options.Address)
	}
	if options.Password != "testpass" {
		t.Errorf("Expected password 'testpass', got %s", options.Password)
	}
	if options.DB != 1 {
		t.Errorf("Expected DB 1, got %d", options.DB)
	}
	if options.PoolSize != 20 {
		t.Errorf("Expected pool size 20, got %d", options.PoolSize)
	}
}

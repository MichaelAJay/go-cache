package redis_test

import (
	"os"
	"testing"

	"github.com/MichaelAJay/go-cache/providers/redis"
)

func TestLoadRedisOptionsFromEnv(t *testing.T) {
	// Test with default values (no environment variables set)
	t.Run("DefaultValues", func(t *testing.T) {
		// Clear any existing env vars
		os.Unsetenv("REDIS_ADDR")
		os.Unsetenv("REDIS_PASSWORD")
		os.Unsetenv("REDIS_DB")
		os.Unsetenv("REDIS_POOL_SIZE")

		options := redis.LoadRedisOptionsFromEnv()

		// Check default values
		if options.Address != "127.0.0.1:6379" {
			t.Errorf("Expected default address localhost:6379, got %s", options.Address)
		}
		if options.Password != "" {
			t.Errorf("Expected empty default password, got %s", options.Password)
		}
		if options.DB != 0 {
			t.Errorf("Expected default DB 0, got %d", options.DB)
		}
		if options.PoolSize != 10 {
			t.Errorf("Expected default pool size 10, got %d", options.PoolSize)
		}
	})

	// Test with custom environment variables
	t.Run("CustomValues", func(t *testing.T) {
		// Set environment variables
		os.Setenv("REDIS_ADDR", "redis.example.com:6379")
		os.Setenv("REDIS_PASSWORD", "secret-password")
		os.Setenv("REDIS_DB", "2")
		os.Setenv("REDIS_POOL_SIZE", "20")

		options := redis.LoadRedisOptionsFromEnv()

		// Check custom values
		if options.Address != "redis.example.com:6379" {
			t.Errorf("Expected address redis.example.com:6379, got %s", options.Address)
		}
		if options.Password != "secret-password" {
			t.Errorf("Expected password secret-password, got %s", options.Password)
		}
		if options.DB != 2 {
			t.Errorf("Expected DB 2, got %d", options.DB)
		}
		if options.PoolSize != 20 {
			t.Errorf("Expected pool size 20, got %d", options.PoolSize)
		}

		// Clean up environment variables
		os.Unsetenv("REDIS_ADDR")
		os.Unsetenv("REDIS_PASSWORD")
		os.Unsetenv("REDIS_DB")
		os.Unsetenv("REDIS_POOL_SIZE")
	})

	// Test with invalid numeric values
	t.Run("InvalidNumericValues", func(t *testing.T) {
		os.Setenv("REDIS_DB", "not-a-number")
		os.Setenv("REDIS_POOL_SIZE", "also-not-a-number")

		options := redis.LoadRedisOptionsFromEnv()

		// Should fall back to defaults for invalid numbers
		if options.DB != 0 {
			t.Errorf("Expected default DB 0 for invalid input, got %d", options.DB)
		}
		if options.PoolSize != 10 {
			t.Errorf("Expected default pool size 10 for invalid input, got %d", options.PoolSize)
		}

		// Clean up environment variables
		os.Unsetenv("REDIS_DB")
		os.Unsetenv("REDIS_POOL_SIZE")
	})
}

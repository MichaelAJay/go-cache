package redis_test

import (
	"testing"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/providers/redis"
	goredis "github.com/go-redis/redis/v8"
)

// MockRedisClientFactory is used to inject a mock Redis client into the cache creation process
type MockRedisClientFactory struct {
	// Store the provided options for inspection during tests
	options *goredis.Options
}

// NewMockRedisClient creates a new mock Redis client
func (m *MockRedisClientFactory) NewClient(opts *goredis.Options) *goredis.Client {
	m.options = opts

	// For testing purposes - we're not actually using the client in these unit tests
	// The actual functionality is tested in integration tests
	client := goredis.NewClient(opts)

	return client
}

// TestRedisProviderMocked tests the Redis provider with a mock Redis client factory
func TestRedisProviderMocked(t *testing.T) {
	// Original redis.NewClient function
	originalNewClient := redis.RedisClientFactory

	// Create mock factory
	mockFactory := &MockRedisClientFactory{}

	// Replace the NewClient function to use our mock
	// Note: This requires exposing the RedisClientFactory variable in the redis package
	redis.RedisClientFactory = mockFactory.NewClient

	// Restore the original function at the end of the test
	defer func() {
		redis.RedisClientFactory = originalNewClient
	}()

	provider := redis.NewProvider()

	// Test with all configuration options
	t.Run("FullConfigOptions", func(t *testing.T) {
		cacheOptions := &cache.CacheOptions{
			TTL: 0,
			RedisOptions: &cache.RedisOptions{
				Address:  "custom.redis.server:6380",
				Password: "secret",
				DB:       5,
				PoolSize: 25,
			},
		}

		_, err := provider.Create(cacheOptions)
		// We expect this to fail in real usage because we're not actually connecting to Redis,
		// but that's not relevant for this unit test which only checks the options passing
		if err == nil {
			t.Log("Note: No error returned when creating cache - this is unexpected in a unit test")
		}

		// Verify the options were passed correctly to the Redis client
		if mockFactory.options.Addr != "custom.redis.server:6380" {
			t.Errorf("Expected address custom.redis.server:6380, got %s", mockFactory.options.Addr)
		}

		if mockFactory.options.Password != "secret" {
			t.Errorf("Expected password 'secret', got %s", mockFactory.options.Password)
		}

		if mockFactory.options.DB != 5 {
			t.Errorf("Expected DB 5, got %d", mockFactory.options.DB)
		}

		if mockFactory.options.PoolSize != 25 {
			t.Errorf("Expected pool size 25, got %d", mockFactory.options.PoolSize)
		}
	})
}

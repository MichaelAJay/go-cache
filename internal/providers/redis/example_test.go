package redis_test

import (
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/MichaelAJay/go-cache/internal/providers/redis"
	"github.com/MichaelAJay/go-serializer"
)

// TestRedisProviderCreation tests that the Redis provider can be created
func TestRedisProviderCreation(t *testing.T) {
	provider := redis.NewProvider()
	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}
	
	if provider.Name() != "redis" {
		t.Errorf("Expected provider name 'redis', got %s", provider.Name())
	}
}

// TestRedisProviderValidation tests the provider validation logic
func TestRedisProviderValidation(t *testing.T) {
	provider := redis.NewProvider()
	
	// Test validation with nil options (should set defaults)
	err := provider.Validate(nil)
	if err != nil {
		t.Errorf("Expected no error for nil options, got %v", err)
	}
	
	// Test validation with empty options (should set defaults)
	options := &interfaces.CacheOptions{}
	err = provider.Validate(options)
	if err != nil {
		t.Errorf("Expected no error for empty options, got %v", err)
	}
	
	// Check that defaults were set
	if options.RedisOptions == nil {
		t.Error("Expected RedisOptions to be set by validation")
	}
	
	if options.RedisOptions.Address != "localhost:6379" {
		t.Errorf("Expected default address 'localhost:6379', got %s", options.RedisOptions.Address)
	}
}

// TestRedisFactoryFunctions tests the factory functions
func TestRedisFactoryFunctions(t *testing.T) {
	// Test creating a factory function
	factory := redis.NewRedisFactory[string]()
	if factory == nil {
		t.Fatal("Expected non-nil factory")
	}
	
	// Note: We can't test actual cache creation without a Redis instance
	// This test just verifies the factory function interface
}

// Example demonstrates how to use the Redis cache provider
func Example() {
	// Create Redis options
	options := &interfaces.CacheOptions{
		RedisOptions: &interfaces.RedisOptions{
			Address: "localhost:6379",
			DB:      0,
		},
		SerializerFormat: serializer.JSON,
		TTL:             time.Hour,
	}
	
	// Create cache instance (this would require a running Redis instance)
	// cache, err := redis.NewRedisCache[string](options)
	// if err != nil {
	//     panic(err)
	// }
	// defer cache.Close()
	
	// Use the cache
	// ctx := context.Background()
	// err = cache.Set(ctx, "key", "value", time.Hour)
	// value, found, err := cache.Get(ctx, "key")
	
	// This is just a documentation example
	_ = options
}

// BenchmarkRedisProvider benchmarks the provider creation
func BenchmarkRedisProvider(b *testing.B) {
	for i := 0; i < b.N; i++ {
		provider := redis.NewProvider()
		_ = provider.Name()
	}
}
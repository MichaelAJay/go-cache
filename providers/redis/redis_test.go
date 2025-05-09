package redis_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/providers/redis"
	goredis "github.com/go-redis/redis/v8"
)

// getRedisAddr gets the Redis address from environment or uses default
func getRedisAddr() string {
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		return addr
	}
	return "127.0.0.1:6379"
}

// setupRedisClient creates a Redis client for testing
func setupRedisClient(t *testing.T) *goredis.Client {
	client := goredis.NewClient(&goredis.Options{
		Addr: getRedisAddr(),
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Skipping test: Cannot connect to Redis at %s: %v", getRedisAddr(), err)
	}

	return client
}

// setupTestPrefix creates a unique prefix for test keys
func setupTestPrefix() string {
	return "test:" + time.Now().Format("20060102150405") + ":"
}

// setupRedisCache creates a Redis cache for testing
func setupRedisCache(t *testing.T) (cache.Cache, func()) {
	client := setupRedisClient(t)

	// Create a unique prefix string for logging purposes
	testID := setupTestPrefix()
	t.Logf("Creating Redis cache for test with ID: %s", testID)

	cacheOptions := &cache.CacheOptions{
		TTL: time.Minute,
		RedisOptions: &cache.RedisOptions{
			Address: getRedisAddr(),
		},
	}

	c, err := redis.NewRedisCache(client, cacheOptions)
	if err != nil {
		t.Fatalf("Failed to create Redis cache: %v", err)
	}

	cleanup := func() {
		// Clean up test keys
		ctx := context.Background()
		if err := c.Clear(ctx); err != nil {
			t.Logf("Warning: Failed to clear cache during cleanup: %v", err)
		}
		c.Close()
	}

	return c, cleanup
}

// Test basic operations
func TestRedisCache_BasicOperations(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Test Set and Get
	err := c.Set(ctx, "key1", "value1", 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	value, exists, err := c.Get(ctx, "key1")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if !exists {
		t.Error("Expected key to exist")
	}
	if value != "value1" {
		t.Errorf("Expected value 'value1', got '%v'", value)
	}

	// Test Delete
	err = c.Delete(ctx, "key1")
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	_, exists, _ = c.Get(ctx, "key1")
	if exists {
		t.Error("Expected key to be deleted")
	}
}

// Test TTL functionality
func TestRedisCache_TTL(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Set value with short TTL
	err := c.Set(ctx, "key1", "value1", 100*time.Millisecond)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Value should exist immediately
	_, exists, _ := c.Get(ctx, "key1")
	if !exists {
		t.Error("Expected key to exist immediately after setting")
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Value should be gone
	_, exists, _ = c.Get(ctx, "key1")
	if exists {
		t.Error("Expected key to be expired")
	}
}

// Test bulk operations
func TestRedisCache_BulkOperations(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Test SetMany
	items := map[string]any{
		"key1": "value1",
		"key2": "value2",
	}
	err := c.SetMany(ctx, items, 0)
	if err != nil {
		t.Errorf("SetMany failed: %v", err)
	}

	// Test GetMany
	values, err := c.GetMany(ctx, []string{"key1", "key2", "key3"})
	if err != nil {
		t.Errorf("GetMany failed: %v", err)
	}
	if len(values) != 2 {
		t.Errorf("Expected 2 values, got %d", len(values))
	}
	if values["key1"] != "value1" || values["key2"] != "value2" {
		t.Error("Expected values to match")
	}

	// Test DeleteMany
	err = c.DeleteMany(ctx, []string{"key1", "key2"})
	if err != nil {
		t.Errorf("DeleteMany failed: %v", err)
	}

	values, _ = c.GetMany(ctx, []string{"key1", "key2"})
	if len(values) != 0 {
		t.Error("Expected all keys to be deleted")
	}
}

// Test metadata operations
func TestRedisCache_Metadata(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Set value
	err := c.Set(ctx, "key1", "value1", time.Hour)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Get metadata
	metadata, err := c.GetMetadata(ctx, "key1")
	if err != nil {
		t.Errorf("GetMetadata failed: %v", err)
	}

	if metadata.Key != "key1" {
		t.Errorf("Expected key 'key1', got '%s'", metadata.Key)
	}

	// Access the value
	_, _, _ = c.Get(ctx, "key1")

	// Check updated metadata
	updatedMetadata, _ := c.GetMetadata(ctx, "key1")
	if updatedMetadata.AccessCount <= metadata.AccessCount {
		t.Errorf("Expected increased access count, got %d", updatedMetadata.AccessCount)
	}
}

// Test provider creation
func TestRedisProvider(t *testing.T) {
	provider := redis.NewProvider()

	// Check if Redis is available
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
	c, err := provider.Create(&cache.CacheOptions{
		TTL: time.Minute,
		RedisOptions: &cache.RedisOptions{
			Address: getRedisAddr(),
		},
	})

	if err != nil {
		t.Errorf("Provider creation failed: %v", err)
	}
	if c == nil {
		t.Error("Expected cache instance to be created")
	}

	// Test with nil options
	_, err = provider.Create(nil)
	if err == nil {
		t.Error("Expected error with nil options")
	}

	// Test environment-based configuration
	oldEnv := os.Getenv("REDIS_ADDR")
	os.Setenv("REDIS_ADDR", getRedisAddr())
	defer os.Setenv("REDIS_ADDR", oldEnv)

	c, err = provider.Create(&cache.CacheOptions{
		TTL: time.Minute,
	})

	if err != nil {
		t.Errorf("Provider creation with env config failed: %v", err)
	}
	if c == nil {
		t.Error("Expected cache instance to be created with env config")
	}
}

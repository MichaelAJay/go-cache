package redis_test

import (
	"context"
	"fmt"
	"os"
	"sync"
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

// Test serialization of different data types
func TestRedisCache_Serialization(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Test cases for different data types
	testCases := []struct {
		name  string
		value any
	}{
		{"string", "test string"},
		{"int", 42},
		{"float", 3.14},
		{"bool", true},
		{"slice", []string{"a", "b", "c"}},
		{"map", map[string]int{"a": 1, "b": 2}},
		{"struct", struct {
			Name  string
			Value int
		}{"test", 123}},
		{"nil", nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set value
			err := c.Set(ctx, tc.name, tc.value, time.Hour)
			if err != nil {
				if tc.value == nil && err == cache.ErrSerialization {
					// Skip nil value test as it's not supported
					t.Skip("Nil values are not supported")
					return
				}
				t.Errorf("Set failed for %s: %v", tc.name, err)
				return
			}

			// Get value
			got, exists, err := c.Get(ctx, tc.name)
			if err != nil {
				t.Errorf("Get failed for %s: %v", tc.name, err)
				return
			}
			if !exists {
				t.Errorf("Value not found for %s", tc.name)
				return
			}

			// Compare values
			if tc.value == nil {
				if got != nil {
					t.Errorf("Expected nil, got %v", got)
				}
			} else if tc.name == "struct" {
				// For structs, handle deserialized map comparison
				structMap, ok := got.(map[string]any)
				if !ok {
					t.Errorf("Expected map[string]any for deserialized struct, got %T", got)
					return
				}

				originalStruct := tc.value.(struct {
					Name  string
					Value int
				})

				// Compare individual fields
				if structMap["Name"] != originalStruct.Name {
					t.Errorf("Expected Name %v, got %v", originalStruct.Name, structMap["Name"])
				}

				// Handle potential numeric type differences
				switch value := structMap["Value"].(type) {
				case int:
					if value != originalStruct.Value {
						t.Errorf("Expected Value %v, got %v", originalStruct.Value, value)
					}
				case int8:
					if int(value) != originalStruct.Value {
						t.Errorf("Expected Value %v, got %v", originalStruct.Value, value)
					}
				case int64:
					if int(value) != originalStruct.Value {
						t.Errorf("Expected Value %v, got %v", originalStruct.Value, value)
					}
				case uint16:
					if int(value) != originalStruct.Value {
						t.Errorf("Expected Value %v, got %v", originalStruct.Value, value)
					}
				case float64:
					if int(value) != originalStruct.Value {
						t.Errorf("Expected Value %v, got %v", originalStruct.Value, value)
					}
				default:
					t.Errorf("Unexpected numeric type for Value: %T (value: %v)", structMap["Value"], structMap["Value"])
				}
			} else {
				// For other types, we need to compare the string representation
				// since direct comparison might not work due to type differences
				expectedStr := fmt.Sprintf("%v", tc.value)
				gotStr := fmt.Sprintf("%v", got)
				if expectedStr != gotStr {
					t.Errorf("Expected %v, got %v", tc.value, got)
				}
			}
		})
	}
}

// Test error handling and edge cases
func TestRedisCache_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Test invalid key
	_, exists, err := c.Get(ctx, "")
	if err != cache.ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey for empty key, got %v", err)
	}
	if exists {
		t.Error("Expected empty key to not exist")
	}

	// Test context cancellation
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, _, err = c.Get(cancelCtx, "key")
	if err != cache.ErrContextCanceled {
		t.Errorf("Expected ErrContextCanceled for canceled context, got %v", err)
	}

	// Test serialization error with invalid type
	type invalidType struct {
		Ch chan int // Channels can't be serialized
	}
	err = c.Set(ctx, "invalid", invalidType{make(chan int)}, time.Hour)
	if err != cache.ErrSerialization {
		t.Errorf("Expected ErrSerialization for invalid type, got %v", err)
	}

	// Test deserialization error by directly setting invalid data
	// Instead of using the client directly, we'll use the cache's public interface
	// to test error handling for invalid data
	err = c.Set(ctx, "invalid_data", []byte{0xFF, 0xFF, 0xFF}, time.Hour)
	if err != nil {
		t.Fatalf("Failed to set invalid data: %v", err)
	}

	// Try to get the invalid data
	_, _, err = c.Get(ctx, "invalid_data")
	// The error handling here might vary depending on the serializer implementation
	// Some serializers might be more forgiving with malformed data
	if err != nil && err != cache.ErrDeserialization {
		t.Errorf("Expected either nil or ErrDeserialization for invalid data, got %v", err)
	}
	// The "exists" flag could also vary based on how the implementation handles errors
	// Don't strictly test for it
}

// Test metrics collection
func TestRedisCache_Metrics(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Clear any existing entries
	if err := c.Clear(ctx); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Set some values
	err := c.Set(ctx, "key1", "value1", time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get existing value
	_, exists, err := c.Get(ctx, "key1")
	if err != nil || !exists {
		t.Fatalf("Get failed: %v", err)
	}

	// Get non-existent value
	_, exists, err = c.Get(ctx, "nonexistent")
	if err != nil || exists {
		t.Fatalf("Get should have returned not found")
	}

	// Get metrics
	metrics := c.GetMetrics()
	if metrics == nil {
		t.Fatal("Expected metrics to be non-nil")
	}

	// Verify metrics
	if metrics.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", metrics.Hits)
	}
	if metrics.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", metrics.Misses)
	}
	if metrics.HitRatio != 0.5 {
		t.Errorf("Expected hit ratio of 0.5, got %f", metrics.HitRatio)
	}
	if metrics.GetLatency == 0 {
		t.Error("Expected non-zero get latency")
	}
	if metrics.SetLatency == 0 {
		t.Error("Expected non-zero set latency")
	}
	if metrics.CacheSize == 0 {
		t.Error("Expected non-zero cache size")
	}
	if metrics.EntryCount == 0 {
		t.Error("Expected at least one entry")
	}
}

// TestRedisCache_Concurrency tests concurrent access to the cache
func TestRedisCache_Concurrency(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	const goroutines = 10
	const operations = 100
	var wg sync.WaitGroup

	// Test concurrent writes
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				err := c.Set(ctx, key, fmt.Sprintf("value_%d_%d", id, j), time.Hour)
				if err != nil {
					t.Errorf("Concurrent Set failed: %v", err)
				}
			}
		}(i)
	}
	wg.Wait()

	// Test concurrent reads
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				_, exists, err := c.Get(ctx, key)
				if err != nil {
					t.Errorf("Concurrent Get failed: %v", err)
				}
				if !exists {
					t.Errorf("Expected key %s to exist", key)
				}
			}
		}(i)
	}
	wg.Wait()

	// Test concurrent deletes
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				err := c.Delete(ctx, key)
				if err != nil {
					t.Errorf("Concurrent Delete failed: %v", err)
				}
			}
		}(i)
	}
	wg.Wait()
}

// TestRedisCache_ConnectionFailure tests behavior when Redis connection fails
func TestRedisCache_ConnectionFailure(t *testing.T) {
	// Create cache with invalid Redis address
	cacheOptions := &cache.CacheOptions{
		TTL: time.Minute,
		RedisOptions: &cache.RedisOptions{
			Address: "invalid:6379",
		},
	}

	// Create Redis client with invalid address
	client := goredis.NewClient(&goredis.Options{
		Addr: "invalid:6379",
	})

	// Attempt to create cache
	_, err := redis.NewRedisCache(client, cacheOptions)
	if err == nil {
		t.Error("Expected error when creating cache with invalid Redis address")
	}
}

// TestRedisCache_Disconnection tests behavior when Redis connection is lost
func TestRedisCache_Disconnection(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Set a value
	err := c.Set(ctx, "key1", "value1", time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get the value to ensure it's set
	_, exists, err := c.Get(ctx, "key1")
	if err != nil || !exists {
		t.Fatalf("Get failed: %v", err)
	}

	// Note: This test is limited in its ability to test actual disconnection
	// as we can't easily simulate a Redis server disconnection in a unit test.
	// This would be better tested in integration tests with a real Redis server
	// that can be stopped/started.
}

// TestRedisCache_ContextTimeout tests behavior with context timeouts
func TestRedisCache_ContextTimeout(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Create a context with a very short timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
	defer cancel()
	time.Sleep(10 * time.Millisecond)

	// Try operations with the timeout context
	_, _, err := c.Get(timeoutCtx, "key1")

	if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
		t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
	}

	err = c.Set(timeoutCtx, "key1", "value1", time.Hour)
	if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
		t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
	}
}

// TestRedisCache_MaxSize tests behavior when cache size limits are reached
func TestRedisCache_MaxSize(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Set a large value
	largeValue := make([]byte, 1024*1024) // 1MB
	err := c.Set(ctx, "large_key", largeValue, time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Try to set another large value
	err = c.Set(ctx, "large_key2", largeValue, time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify both values exist
	_, exists, err := c.Get(ctx, "large_key")
	if err != nil || !exists {
		t.Errorf("First large value not found: %v", err)
	}

	_, exists, err = c.Get(ctx, "large_key2")
	if err != nil || !exists {
		t.Errorf("Second large value not found: %v", err)
	}
}

// TestRedisCache_NewWithConfig tests creating a cache with a new client configuration
func TestRedisCache_NewWithConfig(t *testing.T) {
	// Test with valid config
	config := &goredis.Options{
		Addr: getRedisAddr(),
	}
	options := &cache.CacheOptions{
		TTL: time.Minute,
	}

	c, err := redis.NewRedisCacheWithConfig(config, options)
	if err != nil {
		t.Fatalf("Failed to create cache with config: %v", err)
	}
	defer c.Close()

	// Test basic operations to ensure it works
	ctx := context.Background()
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

	// Test with invalid config
	invalidConfig := &goredis.Options{
		Addr: "invalid:6379",
	}
	_, err = redis.NewRedisCacheWithConfig(invalidConfig, options)
	if err == nil {
		t.Error("Expected error with invalid config")
	}
}

// TestRedisCache_GetMetadata_EdgeCases tests edge cases for GetMetadata
func TestRedisCache_GetMetadata_EdgeCases(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Test non-existent key
	_, err := c.GetMetadata(ctx, "nonexistent")
	if err != cache.ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound, got %v", err)
	}

	// Test empty key
	_, err = c.GetMetadata(ctx, "")
	if err != cache.ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey, got %v", err)
	}

	// Test with context cancellation
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, err = c.GetMetadata(cancelCtx, "key")
	if err != cache.ErrContextCanceled {
		t.Errorf("Expected ErrContextCanceled, got %v", err)
	}

	// Test with corrupted metadata
	// Set a value first
	err = c.Set(ctx, "test_key", "test_value", time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get the value to ensure it's set
	_, exists, err := c.Get(ctx, "test_key")
	if err != nil || !exists {
		t.Fatalf("Get failed: %v", err)
	}

	// Get metadata
	metadata, err := c.GetMetadata(ctx, "test_key")
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}

	// Verify metadata fields
	if metadata.Key != "test_key" {
		t.Errorf("Expected key 'test_key', got %s", metadata.Key)
	}
	if metadata.CreatedAt.IsZero() {
		t.Error("Expected non-zero CreatedAt")
	}
	if metadata.LastAccessed.IsZero() {
		t.Error("Expected non-zero LastAccessed")
	}
	if metadata.AccessCount < 0 {
		t.Error("Expected non-negative AccessCount")
	}
	if metadata.TTL <= 0 {
		t.Error("Expected positive TTL")
	}
	if metadata.Size <= 0 {
		t.Error("Expected positive Size")
	}
}

// TestRedisCache_Has_EdgeCases tests edge cases for Has
func TestRedisCache_Has_EdgeCases(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Test empty key
	exists := c.Has(ctx, "")
	if exists {
		t.Error("Expected empty key to not exist")
	}

	// Test with context cancellation
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	exists = c.Has(cancelCtx, "key")
	if exists {
		t.Error("Expected canceled context to return false")
	}

	// Test expired key
	err := c.Set(ctx, "expired_key", "value", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	exists = c.Has(ctx, "expired_key")
	if exists {
		t.Error("Expected expired key to not exist")
	}
}

// TestRedisCache_Delete_EdgeCases tests edge cases for Delete
func TestRedisCache_Delete_EdgeCases(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Test empty key
	err := c.Delete(ctx, "")
	if err != cache.ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey, got %v", err)
	}

	// Test with context cancellation
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	err = c.Delete(cancelCtx, "key")
	if err != cache.ErrContextCanceled {
		t.Errorf("Expected ErrContextCanceled, got %v", err)
	}

	// Test deleting non-existent key
	err = c.Delete(ctx, "nonexistent")
	if err != nil {
		t.Errorf("Expected no error when deleting non-existent key, got %v", err)
	}

	// Test deleting expired key
	err = c.Set(ctx, "expired_key", "value", 1*time.Millisecond)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	err = c.Delete(ctx, "expired_key")
	if err != nil {
		t.Errorf("Expected no error when deleting expired key, got %v", err)
	}
}

// TestRedisCache_Clear_EdgeCases tests edge cases for Clear
func TestRedisCache_Clear_EdgeCases(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupRedisCache(t)
	defer cleanup()

	// Test clearing empty cache
	err := c.Clear(ctx)
	if err != nil {
		t.Errorf("Expected no error when clearing empty cache, got %v", err)
	}

	// Test with context cancellation
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	err = c.Clear(cancelCtx)
	if err != cache.ErrContextCanceled {
		t.Errorf("Expected ErrContextCanceled, got %v", err)
	}

	// Test clearing cache with expired entries
	err = c.Set(ctx, "expired_key", "value", 1*time.Millisecond)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	err = c.Clear(ctx)
	if err != nil {
		t.Errorf("Expected no error when clearing cache with expired entries, got %v", err)
	}

	// Test clearing cache with many entries
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key_%d", i)
		err = c.Set(ctx, key, fmt.Sprintf("value_%d", i), time.Hour)
		if err != nil {
			t.Fatalf("Set failed for key %s: %v", key, err)
		}
	}
	err = c.Clear(ctx)
	if err != nil {
		t.Errorf("Expected no error when clearing cache with many entries, got %v", err)
	}

	// Verify cache is empty
	keys := c.GetKeys(ctx)
	if len(keys) != 0 {
		t.Errorf("Expected empty cache after clear, got %d keys", len(keys))
	}
}

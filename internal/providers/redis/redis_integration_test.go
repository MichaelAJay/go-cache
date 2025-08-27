//go:build integration
// +build integration

package redis_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/internal/providers/redis"
	"github.com/MichaelAJay/go-serializer"
	goredis "github.com/go-redis/redis/v8"
)

// Define a flag to explicitly run integration tests
var runIntegration = flag.Bool("integration", false, "Run Redis integration tests")

// TestMain setup the testing environment and process test flags
func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

// getRedisAddrForIntegration gets the Redis address from environment or uses default
func getRedisAddrForIntegration() string {
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		return addr
	}
	return "127.0.0.1:6379"
}

// isRedisAvailable checks if Redis is available at the given address
func isRedisAvailable() bool {
	client := goredis.NewClient(&goredis.Options{
		Addr: getRedisAddrForIntegration(),
	})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := client.Ping(ctx).Err()
	return err == nil
}

// setupIntegrationCache creates a Redis cache for testing
func setupIntegrationCache(t *testing.T) (cache.Cache, func()) {
	t.Helper()

	// Skip test if integration flag is not set
	if !*runIntegration {
		t.Skip("Skipping integration test; use -integration flag to run")
	}

	// Skip if Redis is not available
	if !isRedisAvailable() {
		t.Skipf("Redis server not available at %s", getRedisAddrForIntegration())
	}

	client := goredis.NewClient(&goredis.Options{
		Addr: getRedisAddrForIntegration(),
	})

	// Create a unique prefix string for test isolation
	testID := fmt.Sprintf("integration:%s:", time.Now().Format("20060102150405"))
	t.Logf("Creating Redis cache for integration test with ID: %s", testID)

	cacheOptions := &cache.CacheOptions{
		TTL: time.Minute,
		RedisOptions: &cache.RedisOptions{
			Address: getRedisAddrForIntegration(),
		},
		SerializerFormat: serializer.Msgpack,
	}

	c, err := redis.NewRedisCache(client, cacheOptions)
	if err != nil {
		t.Fatalf("Failed to create Redis cache: %v", err)
	}

	cleanup := func() {
		ctx := context.Background()
		if err := c.Clear(ctx); err != nil {
			t.Logf("Warning: Failed to clear cache during cleanup: %v", err)
		}
		c.Close()
	}

	return c, cleanup
}

// TestRedisIntegration_Complete is a comprehensive integration test
func TestRedisIntegration_Complete(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupIntegrationCache(t)
	defer cleanup()

	// 1. Test Set, Get and exists operations
	t.Run("SetAndGet", func(t *testing.T) {
		// Set a value
		err := c.Set(ctx, "integration:key1", "value1", time.Minute)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		// Get the value
		value, exists, err := c.Get(ctx, "integration:key1")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if !exists {
			t.Fatal("Key should exist")
		}
		if value != "value1" {
			t.Fatalf("Expected 'value1', got '%v'", value)
		}
	})

	// 2. Test with complex data types
	t.Run("ComplexTypes", func(t *testing.T) {
		type User struct {
			ID      int     `json:"id" msgpack:"id"`
			Name    string  `json:"name" msgpack:"name"`
			Active  bool    `json:"active" msgpack:"active"`
			Balance float64 `json:"balance" msgpack:"balance"`
		}

		user := User{
			ID:      1,
			Name:    "Test User",
			Active:  true,
			Balance: 100.50,
		}

		// Set object
		err := c.Set(ctx, "integration:user:1", user, time.Minute)
		if err != nil {
			t.Fatalf("Set complex type failed: %v", err)
		}

		// Get object
		result, exists, err := c.Get(ctx, "integration:user:1")
		if err != nil {
			t.Fatalf("Get complex type failed: %v", err)
		}
		if !exists {
			t.Fatal("Complex key should exist")
		}

		// The result will be a map due to JSON/MessagePack serialization
		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("Expected map[string]any, got %T", result)
		}

		// Print all keys in the resultMap for debugging
		t.Logf("Result map keys: %v", resultMap)

		// Verify name field exists with expected value
		nameValue, nameExists := resultMap["name"]
		if !nameExists {
			t.Errorf("Name field missing from result map, got keys: %v", getMapKeys(resultMap))
		} else if nameValue != "Test User" {
			t.Errorf("Expected name 'Test User', got '%v'", nameValue)
		}

		// Verify active field exists with expected value
		activeValue, activeExists := resultMap["active"]
		if !activeExists {
			t.Errorf("Active field missing from result map, got keys: %v", getMapKeys(resultMap))
		} else if activeValue != true {
			t.Errorf("Expected active true, got %v", activeValue)
		}
	})

	// 3. Test TTL functionality
	t.Run("TTL", func(t *testing.T) {
		// Set with short TTL
		err := c.Set(ctx, "integration:ttl-key", "expires-soon", 200*time.Millisecond)
		if err != nil {
			t.Fatalf("Set with TTL failed: %v", err)
		}

		// Value should exist initially
		_, exists, _ := c.Get(ctx, "integration:ttl-key")
		if !exists {
			t.Fatal("Key should exist initially")
		}

		// Wait for expiration
		time.Sleep(300 * time.Millisecond)

		// Value should be gone
		_, exists, _ = c.Get(ctx, "integration:ttl-key")
		if exists {
			t.Fatal("Key should have expired")
		}
	})

	// 4. Test batch operations
	t.Run("BatchOperations", func(t *testing.T) {
		// SetMany
		items := map[string]any{
			"integration:batch:1": "value1",
			"integration:batch:2": "value2",
			"integration:batch:3": "value3",
		}
		err := c.SetMany(ctx, items, time.Minute)
		if err != nil {
			t.Fatalf("SetMany failed: %v", err)
		}

		// GetMany
		keys := []string{
			"integration:batch:1",
			"integration:batch:2",
			"integration:batch:3",
			"integration:batch:4", // This one doesn't exist
		}
		results, err := c.GetMany(ctx, keys)
		if err != nil {
			t.Fatalf("GetMany failed: %v", err)
		}

		// Verify results
		if len(results) != 3 {
			t.Fatalf("Expected 3 results, got %d", len(results))
		}
		if results["integration:batch:1"] != "value1" {
			t.Errorf("Wrong value for batch:1: %v", results["integration:batch:1"])
		}

		// DeleteMany
		err = c.DeleteMany(ctx, keys[:3]) // Delete the first 3 keys
		if err != nil {
			t.Fatalf("DeleteMany failed: %v", err)
		}

		// Verify deletion
		results, err = c.GetMany(ctx, keys[:3])
		if err != nil {
			t.Fatalf("GetMany after delete failed: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("Expected 0 results after deletion, got %d", len(results))
		}
	})

	// 5. Test Clear operation
	t.Run("Clear", func(t *testing.T) {
		// Set multiple values
		for i := 1; i <= 5; i++ {
			key := fmt.Sprintf("integration:clear:%d", i)
			err := c.Set(ctx, key, fmt.Sprintf("value%d", i), time.Minute)
			if err != nil {
				t.Fatalf("Set for clear test failed: %v", err)
			}
		}

		// Clear the cache
		err := c.Clear(ctx)
		if err != nil {
			t.Fatalf("Clear failed: %v", err)
		}

		// Verify all keys are gone
		for i := 1; i <= 5; i++ {
			key := fmt.Sprintf("integration:clear:%d", i)
			_, exists, _ := c.Get(ctx, key)
			if exists {
				t.Errorf("Key %s should have been cleared", key)
			}
		}
	})
}

// Helper function to get map keys for easier debugging
func getMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Helper functions that can be used by other test files

// SkipIfRedisUnavailable checks if Redis is available and skips the test if not
func SkipIfRedisUnavailable(t *testing.T) *goredis.Client {
	t.Helper()

	// Create Redis client with default options
	client := goredis.NewClient(&goredis.Options{
		Addr: getRedisAddrForIntegration(),
	})

	// Try to connect with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Ping Redis to check connection
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Skipping test: Cannot connect to Redis at %s: %v", getRedisAddrForIntegration(), err)
		return nil
	}

	return client
}

// CreateUniqueTestID creates a unique prefix for test isolation
func CreateUniqueTestID(prefix string) string {
	timestamp := time.Now().Format("20060102150405")
	return fmt.Sprintf("%s:%s:", prefix, timestamp)
}

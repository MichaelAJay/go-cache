package redis_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/internal/providers/redis"
	"github.com/MichaelAJay/go-logger"
	"github.com/MichaelAJay/go-serializer"
	goredis "github.com/go-redis/redis/v8"
)

// TestWithMessagePackSerializer tests the Redis cache with MessagePack serializer
func TestWithMessagePackSerializer(t *testing.T) {
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

	// Create logger
	loggerCfg := logger.Config{
		Level:      logger.InfoLevel,
		Output:     os.Stdout,
		TimeFormat: time.RFC3339,
		Prefix:     "[REDIS-TEST] ",
	}
	log := logger.New(loggerCfg)

	// Create cache options with MessagePack serializer
	cacheOptions := &cache.CacheOptions{
		TTL:              time.Minute,
		SerializerFormat: serializer.Msgpack,
		Logger:           log,
		RedisOptions: &cache.RedisOptions{
			Address:  getRedisAddr(),
			Password: "",
			DB:       0,
			PoolSize: 5,
		},
	}

	// Create Redis provider
	provider := redis.NewProvider()

	// Create cache
	redisCache, err := provider.Create(cacheOptions)
	if err != nil {
		t.Fatalf("Failed to create Redis cache: %v", err)
	}
	defer redisCache.Close()

	// Test with complex structured data
	type ComplexStruct struct {
		ID         int            `msgpack:"id"`
		Name       string         `msgpack:"name"`
		Created    time.Time      `msgpack:"created"`
		Tags       []string       `msgpack:"tags"`
		Properties map[string]any `msgpack:"properties"`
		Active     bool           `msgpack:"active"`
		Count      int64          `msgpack:"count"`
		Score      float64        `msgpack:"score"`
		Nested     *struct {
			Value string `msgpack:"value"`
		} `msgpack:"nested"`
	}

	// Create test data
	testData := &ComplexStruct{
		ID:      1,
		Name:    "Test Item",
		Created: time.Now(),
		Tags:    []string{"test", "msgpack", "redis"},
		Properties: map[string]any{
			"priority": 3,
			"category": "testing",
			"enabled":  true,
		},
		Active: true,
		Count:  1234567890,
		Score:  98.6,
		Nested: &struct {
			Value string `msgpack:"value"`
		}{
			Value: "nested value",
		},
	}

	// Set data with MessagePack
	err = redisCache.Set(ctx, "complex:1", testData, time.Hour)
	if err != nil {
		t.Fatalf("Failed to set complex data: %v", err)
	}

	// Get data
	result, found, err := redisCache.Get(ctx, "complex:1")
	if err != nil {
		t.Fatalf("Failed to get complex data: %v", err)
	}
	if !found {
		t.Fatal("Complex data not found")
	}

	// Verify data (type will be map[string]any due to MessagePack deserialization)
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("Expected map[string]any, got %T", result)
	}

	// Verify some fields
	if name, ok := resultMap["name"].(string); !ok || name != "Test Item" {
		t.Errorf("Name field mismatch, got: %v", resultMap["name"])
	}

	if active, ok := resultMap["active"].(bool); !ok || !active {
		t.Errorf("Active field mismatch, got: %v", resultMap["active"])
	}

	// Test metadata
	metadata, err := redisCache.GetMetadata(ctx, "complex:1")
	if err != nil {
		t.Fatalf("Failed to get metadata: %v", err)
	}

	if metadata.TTL < time.Minute*59 {
		t.Errorf("TTL too short: %v", metadata.TTL)
	}

	// Cleanup
	err = redisCache.Delete(ctx, "complex:1")
	if err != nil {
		t.Errorf("Failed to delete test data: %v", err)
	}
}

// TestWithCustomConfiguration tests loading Redis options from custom configuration
func TestWithCustomConfiguration(t *testing.T) {
	// Skip test if Redis is not available
	client := SkipIfRedisUnavailable(t)
	if client != nil {
		client.Close()
	}

	// Set Redis configuration directly
	redisAddress := getRedisAddr()
	redisPassword := ""
	redisDB := 0
	redisPoolSize := 3

	// Create logger
	loggerCfg := logger.Config{
		Level:  logger.DebugLevel,
		Output: os.Stdout,
	}
	log := logger.New(loggerCfg)

	// Create Redis options
	redisOptions := &cache.RedisOptions{
		Address:  redisAddress,
		Password: redisPassword,
		DB:       redisDB,
		PoolSize: redisPoolSize,
	}

	// Create cache options
	cacheOptions := &cache.CacheOptions{
		TTL:              time.Minute * 30,
		Logger:           log,
		SerializerFormat: serializer.Msgpack,
		RedisOptions:     redisOptions,
	}

	// Create Redis provider and cache
	provider := redis.NewProvider()
	redisCache, err := provider.Create(cacheOptions)
	if err != nil {
		t.Fatalf("Failed to create Redis cache: %v", err)
	}
	defer redisCache.Close()

	// Test the cache
	ctx := context.Background()

	// Test with simple key-value
	err = redisCache.Set(ctx, "config-test-key", "config-test-value", time.Minute)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	value, found, err := redisCache.Get(ctx, "config-test-key")
	if err != nil {
		t.Fatalf("Failed to get value: %v", err)
	}
	if !found {
		t.Fatal("Expected value to be found")
	}
	if value != "config-test-value" {
		t.Errorf("Expected 'config-test-value', got '%v'", value)
	}

	// Clean up
	redisCache.Delete(ctx, "config-test-key")
}

// TestTransactions tests Redis cache operations in a transaction-like scenario
func TestTransactions(t *testing.T) {
	// Skip test if Redis is not available
	client := SkipIfRedisUnavailable(t)
	if client != nil {
		client.Close()
	}

	// Setup
	ctx := context.Background()

	// Create logger
	loggerCfg := logger.Config{
		Level:  logger.DebugLevel,
		Output: os.Stdout,
	}
	log := logger.New(loggerCfg)

	// Create cache with short TTL
	cacheOptions := &cache.CacheOptions{
		TTL:              time.Second * 5,
		Logger:           log,
		SerializerFormat: serializer.Msgpack,
		RedisOptions: &cache.RedisOptions{
			Address:  getRedisAddr(),
			PoolSize: 5,
		},
	}

	// Create Redis provider
	provider := redis.NewProvider()
	redisCache, err := provider.Create(cacheOptions)
	if err != nil {
		t.Fatalf("Failed to create Redis cache: %v", err)
	}
	defer redisCache.Close()

	// Clear any existing test data
	for _, key := range []string{"tx:account1", "tx:account2"} {
		redisCache.Delete(ctx, key)
	}

	// Simulate a transaction scenario with account balances
	// Initial balances
	err = redisCache.Set(ctx, "tx:account1", 1000, 0) // Use default TTL
	if err != nil {
		t.Fatalf("Failed to set account1 balance: %v", err)
	}

	err = redisCache.Set(ctx, "tx:account2", 500, 0)
	if err != nil {
		t.Fatalf("Failed to set account2 balance: %v", err)
	}

	// Perform a "transaction" - transfer 300 from account1 to account2
	account1, found, err := redisCache.Get(ctx, "tx:account1")
	if err != nil || !found {
		t.Fatalf("Failed to get account1: %v, found: %v", err, found)
	}

	account2, found, err := redisCache.Get(ctx, "tx:account2")
	if err != nil || !found {
		t.Fatalf("Failed to get account2: %v, found: %v", err, found)
	}

	// Convert to integers (MessagePack preserves numeric types better than JSON)
	balance1 := convertToInt(account1)
	balance2 := convertToInt(account2)

	// Update balances
	balance1 -= 300
	balance2 += 300

	// Store updated balances
	items := map[string]any{
		"tx:account1": balance1,
		"tx:account2": balance2,
	}

	err = redisCache.SetMany(ctx, items, 0)
	if err != nil {
		t.Fatalf("Failed to update balances: %v", err)
	}

	// Verify the results
	accounts, err := redisCache.GetMany(ctx, []string{"tx:account1", "tx:account2"})
	if err != nil {
		t.Fatalf("Failed to get accounts: %v", err)
	}

	// Check balances - convert to int for comparison if needed
	account1Val := accounts["tx:account1"]
	account1Int := convertToInt(account1Val)
	t.Logf("Account1 balance: %v (type: %T)", account1Val, account1Val)
	if account1Int != 700 {
		t.Errorf("Expected account1 balance to be 700, got %v", account1Int)
	}

	account2Val := accounts["tx:account2"]
	account2Int := convertToInt(account2Val)
	t.Logf("Account2 balance: %v (type: %T)", account2Val, account2Val)
	if account2Int != 800 {
		t.Errorf("Expected account2 balance to be 800, got %v", account2Int)
	}

	// Test metadata
	metadata, err := redisCache.GetManyMetadata(ctx, []string{"tx:account1", "tx:account2"})
	if err != nil {
		t.Fatalf("Failed to get metadata: %v", err)
	}

	if len(metadata) != 2 {
		t.Errorf("Expected 2 metadata entries, got %d", len(metadata))
	}

	// Clean up
	redisCache.DeleteMany(ctx, []string{"tx:account1", "tx:account2"})
}

// TestConcurrentOperations tests Redis cache operations in a concurrent environment
func TestConcurrentOperations(t *testing.T) {
	// Skip test if Redis is not available
	client := SkipIfRedisUnavailable(t)
	if client != nil {
		client.Close()
	}

	// Setup
	ctx := context.Background()

	// Create logger
	loggerCfg := logger.Config{
		Level:  logger.InfoLevel,
		Output: os.Stdout,
	}
	log := logger.New(loggerCfg)

	// Create cache
	cacheOptions := &cache.CacheOptions{
		TTL:              time.Minute,
		Logger:           log,
		SerializerFormat: serializer.Msgpack,
		RedisOptions: &cache.RedisOptions{
			Address:  getRedisAddr(),
			PoolSize: 10, // Higher pool size for concurrent operations
		},
	}

	provider := redis.NewProvider()
	redisCache, err := provider.Create(cacheOptions)
	if err != nil {
		t.Fatalf("Failed to create Redis cache: %v", err)
	}
	defer redisCache.Close()

	// Clear any previous test data
	for i := 0; i < 10; i++ {
		redisCache.Delete(ctx, fmt.Sprintf("concurrent:%d", i))
	}

	// Run concurrent operations
	const numOperations = 100
	const numKeys = 10

	done := make(chan bool, numOperations)
	for i := 0; i < numOperations; i++ {
		go func(n int) {
			// Randomly choose an operation: set, get, or delete
			op := n % 3
			key := fmt.Sprintf("concurrent:%d", n%numKeys)

			switch op {
			case 0: // Set
				err := redisCache.Set(ctx, key, n, time.Minute)
				if err != nil {
					t.Errorf("Concurrent Set failed: %v", err)
				}
			case 1: // Get
				_, _, err := redisCache.Get(ctx, key)
				if err != nil && err != cache.ErrKeyNotFound {
					t.Errorf("Concurrent Get failed: %v", err)
				}
			case 2: // Delete
				err := redisCache.Delete(ctx, key)
				if err != nil {
					t.Errorf("Concurrent Delete failed: %v", err)
				}
			}

			done <- true
		}(i)
	}

	// Wait for all operations to complete
	for i := 0; i < numOperations; i++ {
		<-done
	}

	// Final verification - get metrics
	metrics := redisCache.GetMetrics()
	t.Logf("Cache metrics after concurrent operations: hits=%d, misses=%d, ratio=%.2f",
		metrics.Hits, metrics.Misses, metrics.HitRatio)

	// Clean up
	for i := 0; i < numKeys; i++ {
		redisCache.Delete(ctx, fmt.Sprintf("concurrent:%d", i))
	}
}

// TestErrorRecovery tests recovery from Redis connection errors
func TestErrorRecovery(t *testing.T) {
	// Skip test if Redis is not available
	client := SkipIfRedisUnavailable(t)
	if client != nil {
		client.Close()
	}

	// Setup
	ctx := context.Background()

	// Create logger that captures logs
	logOutput := &mockWriter{}
	loggerCfg := logger.Config{
		Level:  logger.ErrorLevel,
		Output: logOutput,
	}
	log := logger.New(loggerCfg)

	// Create cache
	cacheOptions := &cache.CacheOptions{
		TTL:              time.Minute,
		Logger:           log,
		SerializerFormat: serializer.Msgpack,
		RedisOptions: &cache.RedisOptions{
			Address: getRedisAddr(),
		},
	}

	provider := redis.NewProvider()
	redisCache, err := provider.Create(cacheOptions)
	if err != nil {
		t.Fatalf("Failed to create Redis cache: %v", err)
	}
	defer redisCache.Close()

	// Test recovery from serialization error
	type UnserializableType struct {
		Channel chan int
	}

	// This should fail with a serialization error
	err = redisCache.Set(ctx, "error:1", UnserializableType{Channel: make(chan int)}, 0)
	if err == nil {
		t.Error("Expected serialization error, got nil")
	}

	// Verify we can still use the cache after error
	err = redisCache.Set(ctx, "recovery-test", "still-working", 0)
	if err != nil {
		t.Errorf("Failed to use cache after error: %v", err)
	}

	// Clean up
	redisCache.Delete(ctx, "recovery-test")
}

// Helper functions

// mockWriter is a simple io.Writer that captures written content
type mockWriter struct {
	content []byte
}

func (w *mockWriter) Write(p []byte) (n int, err error) {
	w.content = append(w.content, p...)
	return len(p), nil
}

func (w *mockWriter) String() string {
	return string(w.content)
}

// Helper function to convert various numeric types to int
func convertToInt(val any) int {
	switch v := val.(type) {
	case int:
		return v
	case int8:
		return int(v)
	case int16:
		return int(v)
	case int32:
		return int(v)
	case int64:
		return int(v)
	case uint:
		return int(v)
	case uint8:
		return int(v)
	case uint16:
		return int(v)
	case uint32:
		return int(v)
	case uint64:
		return int(v)
	case float32:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0 // Default value if conversion fails
	}
}

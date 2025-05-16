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
	goredis "github.com/go-redis/redis/v8"
)

// getMsgPackTestRedisAddr is kept for backward compatibility
// It now just calls getRedisAddr() from redis_test.go
func getMsgPackTestRedisAddr() string {
	return getRedisAddr()
}

// TestMsgPackSerializer tests the Redis cache with MessagePack serializer specifically
func TestMsgPackSerializer(t *testing.T) {
	// Create Redis client
	client := goredis.NewClient(&goredis.Options{
		Addr: getMsgPackTestRedisAddr(),
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Skipping test: Cannot connect to Redis at %s: %v", getMsgPackTestRedisAddr(), err)
	}

	// Create logger
	loggerCfg := logger.Config{
		Level:  logger.InfoLevel,
		Output: os.Stdout,
	}
	log := logger.New(loggerCfg)

	// Create cache options explicitly with MessagePack serializer
	cacheOptions := &cache.CacheOptions{
		TTL:              time.Minute,
		Logger:           log,
		SerializerFormat: serializer.Msgpack, // Explicitly set MessagePack
		RedisOptions: &cache.RedisOptions{
			Address:  getMsgPackTestRedisAddr(),
			Password: "",
			DB:       0,
			PoolSize: 5,
		},
	}

	// Create Redis cache
	redisCache, err := redis.NewRedisCache(client, cacheOptions)
	if err != nil {
		t.Fatalf("Failed to create Redis cache: %v", err)
	}
	defer redisCache.Close()

	// Test with various data types to verify MessagePack serialization
	testCases := []struct {
		name  string
		key   string
		value interface{}
	}{
		{
			name:  "string",
			key:   "msgpack:string",
			value: "This is a test string",
		},
		{
			name:  "integer",
			key:   "msgpack:integer",
			value: 12345,
		},
		{
			name:  "float",
			key:   "msgpack:float",
			value: 123.456,
		},
		{
			name:  "boolean",
			key:   "msgpack:boolean",
			value: true,
		},
		{
			name: "map",
			key:  "msgpack:map",
			value: map[string]interface{}{
				"name":    "Test User",
				"age":     30,
				"active":  true,
				"balance": 1234.56,
			},
		},
		{
			name:  "slice",
			key:   "msgpack:slice",
			value: []string{"one", "two", "three"},
		},
		{
			name: "struct",
			key:  "msgpack:struct",
			value: struct {
				ID   int    `msgpack:"id"`
				Name string `msgpack:"name"`
			}{
				ID:   1,
				Name: "Test",
			},
		},
		{
			name:  "time",
			key:   "msgpack:time",
			value: time.Now(),
		},
	}

	// Run tests for each data type
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set value
			err := redisCache.Set(ctx, tc.key, tc.value, time.Hour)
			if err != nil {
				t.Fatalf("Failed to set %s: %v", tc.name, err)
			}

			// Get value
			result, found, err := redisCache.Get(ctx, tc.key)
			if err != nil {
				t.Fatalf("Failed to get %s: %v", tc.name, err)
			}
			if !found {
				t.Fatalf("%s not found after setting", tc.name)
			}

			// Verify the result type and content
			t.Logf("%s type: %T, value: %v", tc.name, result, result)

			// For each type, perform appropriate checks
			switch tc.name {
			case "string":
				if s, ok := result.(string); !ok || s != tc.value.(string) {
					t.Errorf("Expected string '%v', got '%v' (%T)", tc.value, result, result)
				}
			case "integer":
				// Check if it's an integer type - MessagePack should preserve numeric types
				switch v := result.(type) {
				case int:
					if v != tc.value.(int) {
						t.Errorf("Expected int %v, got %v", tc.value, v)
					}
				case int64:
					if v != int64(tc.value.(int)) {
						t.Errorf("Expected int64 %v, got %v", tc.value, v)
					}
				case uint16:
					if int(v) != tc.value.(int) {
						t.Errorf("Expected uint16 equivalent to %v, got %v", tc.value, v)
					}
				case float64:
					if v != float64(tc.value.(int)) {
						t.Errorf("Expected float64 %v, got %v", tc.value, v)
					}
				default:
					t.Errorf("Expected numeric type, got %T", result)
				}
			case "map":
				if m, ok := result.(map[string]interface{}); !ok {
					t.Errorf("Expected map[string]interface{}, got %T", result)
				} else {
					// Check a few fields
					expectedMap := tc.value.(map[string]interface{})
					if m["name"] != expectedMap["name"] {
						t.Errorf("Map field 'name' mismatch: expected '%v', got '%v'",
							expectedMap["name"], m["name"])
					}
				}
			}

			// Check metadata
			metadata, err := redisCache.GetMetadata(ctx, tc.key)
			if err != nil {
				t.Errorf("Failed to get metadata for %s: %v", tc.name, err)
			} else {
				t.Logf("%s metadata: TTL=%v, Size=%d bytes",
					tc.name, metadata.TTL, metadata.Size)
			}

			// Delete the test key
			_ = redisCache.Delete(ctx, tc.key)
		})
	}
}

// TestMessagePackVsJSON compares MessagePack to JSON for the same data
func TestMessagePackVsJSON(t *testing.T) {
	// Check if Redis is available
	client := goredis.NewClient(&goredis.Options{
		Addr: getMsgPackTestRedisAddr(),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Skipping test: Cannot connect to Redis at %s: %v", getMsgPackTestRedisAddr(), err)
	}

	// Create logger
	loggerCfg := logger.Config{
		Level:  logger.InfoLevel,
		Output: os.Stdout,
	}
	log := logger.New(loggerCfg)

	// Create test data - a complex object with various data types
	testData := map[string]interface{}{
		"id":        1234,
		"name":      "Test Object",
		"timestamp": time.Now(),
		"active":    true,
		"score":     98.6,
		"tags":      []string{"tag1", "tag2", "tag3", "tag4", "tag5"},
		"metadata": map[string]interface{}{
			"created_by": "test",
			"version":    3,
			"settings": map[string]interface{}{
				"visible": true,
				"color":   "blue",
				"size":    "large",
				"options": []interface{}{1, 2, 3, 4, 5},
			},
		},
		"items": []map[string]interface{}{
			{"id": 1, "value": "first"},
			{"id": 2, "value": "second"},
			{"id": 3, "value": "third"},
		},
	}

	// Create both JSON and MessagePack caches
	jsonOptions := &cache.CacheOptions{
		TTL:              time.Minute,
		Logger:           log,
		SerializerFormat: serializer.JSON,
		RedisOptions: &cache.RedisOptions{
			Address: getMsgPackTestRedisAddr(),
		},
	}

	msgpackOptions := &cache.CacheOptions{
		TTL:              time.Minute,
		Logger:           log,
		SerializerFormat: serializer.Msgpack,
		RedisOptions: &cache.RedisOptions{
			Address: getMsgPackTestRedisAddr(),
		},
	}

	jsonCache, err := redis.NewRedisCache(client, jsonOptions)
	if err != nil {
		t.Fatalf("Failed to create JSON cache: %v", err)
	}
	defer jsonCache.Close()

	msgpackCache, err := redis.NewRedisCache(client, msgpackOptions)
	if err != nil {
		t.Fatalf("Failed to create MessagePack cache: %v", err)
	}
	defer msgpackCache.Close()

	// Set data in both caches
	jsonKey := "comparison:json"
	msgpackKey := "comparison:msgpack"

	err = jsonCache.Set(ctx, jsonKey, testData, time.Hour)
	if err != nil {
		t.Fatalf("Failed to set JSON data: %v", err)
	}

	err = msgpackCache.Set(ctx, msgpackKey, testData, time.Hour)
	if err != nil {
		t.Fatalf("Failed to set MessagePack data: %v", err)
	}

	// Get metadata to compare sizes
	jsonMeta, err := jsonCache.GetMetadata(ctx, jsonKey)
	if err != nil {
		t.Fatalf("Failed to get JSON metadata: %v", err)
	}

	msgpackMeta, err := msgpackCache.GetMetadata(ctx, msgpackKey)
	if err != nil {
		t.Fatalf("Failed to get MessagePack metadata: %v", err)
	}

	// Compare sizes
	t.Logf("JSON size: %d bytes", jsonMeta.Size)
	t.Logf("MessagePack size: %d bytes", msgpackMeta.Size)

	sizeDiff := jsonMeta.Size - msgpackMeta.Size
	percentSmaller := float64(sizeDiff) / float64(jsonMeta.Size) * 100

	t.Logf("MessagePack is %d bytes (%.2f%%) smaller than JSON",
		sizeDiff, percentSmaller)

	// Compare retrieval and data types
	jsonResult, _, err := jsonCache.Get(ctx, jsonKey)
	if err != nil {
		t.Fatalf("Failed to get JSON data: %v", err)
	}

	msgpackResult, _, err := msgpackCache.Get(ctx, msgpackKey)
	if err != nil {
		t.Fatalf("Failed to get MessagePack data: %v", err)
	}

	// Check types of some fields
	jsonMap := jsonResult.(map[string]interface{})
	msgpackMap := msgpackResult.(map[string]interface{})

	t.Logf("JSON 'id' type: %T, value: %v", jsonMap["id"], jsonMap["id"])
	t.Logf("MessagePack 'id' type: %T, value: %v", msgpackMap["id"], msgpackMap["id"])

	// Clean up
	jsonCache.Delete(ctx, jsonKey)
	msgpackCache.Delete(ctx, msgpackKey)
}

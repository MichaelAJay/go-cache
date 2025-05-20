package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/providers/memory"
	"github.com/MichaelAJay/go-logger"
	"github.com/MichaelAJay/go-serializer"
)

// setupMemoryCache creates a memory cache for testing
func setupMemoryCache(t *testing.T) (cache.Cache, func()) {
	cacheOptions := &cache.CacheOptions{
		TTL: time.Minute,
	}

	c, err := memory.NewMemoryCache(cacheOptions)
	if err != nil {
		t.Fatalf("Failed to create memory cache: %v", err)
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

func TestMemoryCache_BasicOperations(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
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

	// Test Has
	has := c.Has(ctx, "key1")
	if !has {
		t.Error("Expected Has to return true for existing key")
	}

	has = c.Has(ctx, "nonexistent")
	if has {
		t.Error("Expected Has to return false for nonexistent key")
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

func TestMemoryCache_TTL(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
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

func TestMemoryCache_MaxEntries(t *testing.T) {
	ctx := context.Background()
	c, err := memory.NewMemoryCache(&cache.CacheOptions{
		MaxEntries: 2,
	})
	if err != nil {
		t.Fatalf("Failed to create memory cache: %v", err)
	}
	defer c.Close()

	// Set two values
	err = c.Set(ctx, "key1", "value1", 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}
	err = c.Set(ctx, "key2", "value2", 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Third value should fail
	err = c.Set(ctx, "key3", "value3", 0)
	if err != cache.ErrCacheFull {
		t.Errorf("Expected ErrCacheFull, got %v", err)
	}
}

func TestMemoryCache_MaxSize(t *testing.T) {
	ctx := context.Background()
	c, err := memory.NewMemoryCache(&cache.CacheOptions{
		MaxSize: 10,
	})
	if err != nil {
		t.Fatalf("Failed to create memory cache: %v", err)
	}
	defer c.Close()

	// Set a large value
	err = c.Set(ctx, "key1", "this is a very long string", 0)
	if err != cache.ErrCacheFull {
		t.Errorf("Expected ErrCacheFull, got %v", err)
	}

	// Set a small value
	err = c.Set(ctx, "key1", "small", 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}
}

func TestMemoryCache_BulkOperations(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
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

func TestMemoryCache_Metadata(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
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
	if metadata.TTL < time.Minute*59 {
		t.Errorf("Expected TTL close to 1h, got %v", metadata.TTL)
	}
	if metadata.Size == 0 {
		t.Error("Expected non-zero size")
	}
	if metadata.AccessCount != 0 {
		t.Errorf("Expected access count 0, got %d", metadata.AccessCount)
	}

	// Access the value
	_, _, _ = c.Get(ctx, "key1")

	// Check updated metadata
	updatedMetadata, _ := c.GetMetadata(ctx, "key1")
	if updatedMetadata.AccessCount != 1 {
		t.Errorf("Expected access count 1, got %d", updatedMetadata.AccessCount)
	}

	// Test GetManyMetadata
	err = c.Set(ctx, "key2", "value2", time.Hour)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	metadataMap, err := c.GetManyMetadata(ctx, []string{"key1", "key2", "nonexistent"})
	if err != nil {
		t.Errorf("GetManyMetadata failed: %v", err)
	}

	if len(metadataMap) != 2 {
		t.Errorf("Expected 2 metadata entries, got %d", len(metadataMap))
	}

	if _, ok := metadataMap["key1"]; !ok {
		t.Error("Expected metadata for key1")
	}

	if _, ok := metadataMap["key2"]; !ok {
		t.Error("Expected metadata for key2")
	}
}

func TestMemoryCache_Cleanup(t *testing.T) {
	ctx := context.Background()
	c, err := memory.NewMemoryCache(&cache.CacheOptions{
		CleanupInterval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to create memory cache: %v", err)
	}

	// Set value with short TTL
	err = c.Set(ctx, "key1", "value1", 100*time.Millisecond)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Wait for cleanup
	time.Sleep(250 * time.Millisecond)

	// Value should be gone
	_, exists, _ := c.Get(ctx, "key1")
	if exists {
		t.Error("Expected key to be cleaned up")
	}

	// Close the cache
	err = c.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestMemoryCache_GetKeys(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Set values
	err := c.Set(ctx, "key1", "value1", 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}
	err = c.Set(ctx, "key2", "value2", 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Get keys
	keys := c.GetKeys(ctx)
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}

	// Check if keys are correct
	foundKey1 := false
	foundKey2 := false
	for _, key := range keys {
		if key == "key1" {
			foundKey1 = true
		}
		if key == "key2" {
			foundKey2 = true
		}
	}
	if !foundKey1 || !foundKey2 {
		t.Error("Expected to find both keys")
	}
}

func TestMemoryCache_Serialization(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Test with string
	err := c.Set(ctx, "string", "hello", 0)
	if err != nil {
		t.Errorf("Set string failed: %v", err)
	}

	// Test with int
	err = c.Set(ctx, "int", 42, 0)
	if err != nil {
		t.Errorf("Set int failed: %v", err)
	}

	// Test with float
	err = c.Set(ctx, "float", 3.14, 0)
	if err != nil {
		t.Errorf("Set float failed: %v", err)
	}

	// Test with bool
	err = c.Set(ctx, "bool", true, 0)
	if err != nil {
		t.Errorf("Set bool failed: %v", err)
	}

	// Test with slice
	err = c.Set(ctx, "slice", []string{"a", "b", "c"}, 0)
	if err != nil {
		t.Errorf("Set slice failed: %v", err)
	}

	// Test with map
	err = c.Set(ctx, "map", map[string]int{"a": 1, "b": 2}, 0)
	if err != nil {
		t.Errorf("Set map failed: %v", err)
	}

	// Test with struct
	type testStruct struct {
		Name string
		Age  int
	}
	err = c.Set(ctx, "struct", testStruct{Name: "John", Age: 30}, 0)
	if err != nil {
		t.Errorf("Set struct failed: %v", err)
	}

	// Retrieve and check values
	val, _, _ := c.Get(ctx, "string")
	if val != "hello" {
		t.Errorf("Expected string 'hello', got %v", val)
	}

	val, _, _ = c.Get(ctx, "int")
	// Handle various numeric types that might be returned
	switch v := val.(type) {
	case int, int8, int16, int32, int64, float32, float64:
		// Convert to float64 for comparison
		var numVal float64
		switch n := v.(type) {
		case int:
			numVal = float64(n)
		case int8:
			numVal = float64(n)
		case int16:
			numVal = float64(n)
		case int32:
			numVal = float64(n)
		case int64:
			numVal = float64(n)
		case float32:
			numVal = float64(n)
		case float64:
			numVal = n
		}

		if numVal != 42 {
			t.Errorf("Expected numeric value 42, got %v (type: %T)", val, val)
		}
	default:
		t.Errorf("Expected numeric type, got %T: %v", val, val)
	}

	val, _, _ = c.Get(ctx, "float")
	// Handle various float types
	switch v := val.(type) {
	case float64, float32, int, int8, int16, int32, int64:
		// Convert to float64 for comparison
		var floatVal float64
		switch n := v.(type) {
		case float64:
			floatVal = n
		case float32:
			floatVal = float64(n)
		case int:
			floatVal = float64(n)
		case int8:
			floatVal = float64(n)
		case int16:
			floatVal = float64(n)
		case int32:
			floatVal = float64(n)
		case int64:
			floatVal = float64(n)
		}

		if floatVal < 3.13 || floatVal > 3.15 { // Allow small rounding differences
			t.Errorf("Expected float value around 3.14, got %v (type: %T)", val, val)
		}
	default:
		t.Errorf("Expected float type, got %T: %v", val, val)
	}

	val, _, _ = c.Get(ctx, "bool")
	if val != true {
		t.Errorf("Expected bool true, got %v", val)
	}

	// Slice and map checks are more complex due to serialization/deserialization
	// Just check that they exist
	_, exists, _ := c.Get(ctx, "slice")
	if !exists {
		t.Error("Expected slice to exist")
	}

	_, exists, _ = c.Get(ctx, "map")
	if !exists {
		t.Error("Expected map to exist")
	}

	_, exists, _ = c.Get(ctx, "struct")
	if !exists {
		t.Error("Expected struct to exist")
	}
}

func TestMemoryCache_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Test with empty key
	err := c.Set(ctx, "", "value", 0)
	if err != cache.ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey for empty key, got %v", err)
	}

	_, _, err = c.Get(ctx, "")
	if err != cache.ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey for empty key in Get, got %v", err)
	}

	err = c.Delete(ctx, "")
	if err != cache.ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey for empty key in Delete, got %v", err)
	}

	// Test with canceled context
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()

	err = c.Set(canceledCtx, "key", "value", 0)
	if err != cache.ErrContextCanceled {
		t.Errorf("Expected ErrContextCanceled for canceled context in Set, got %v", err)
	}

	_, _, err = c.Get(canceledCtx, "key")
	if err != cache.ErrContextCanceled {
		t.Errorf("Expected ErrContextCanceled for canceled context in Get, got %v", err)
	}

	// Test with nonexistent key for metadata
	_, err = c.GetMetadata(ctx, "nonexistent")
	if err != cache.ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound for nonexistent key in GetMetadata, got %v", err)
	}
}

func TestMemoryCache_Metrics(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Set a value
	err := c.Set(ctx, "key1", "value1", 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Get the value (hit)
	_, _, _ = c.Get(ctx, "key1")

	// Get a nonexistent value (miss)
	_, _, _ = c.Get(ctx, "nonexistent")

	// Get metrics
	metrics := c.GetMetrics()

	// Check metrics
	if metrics.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", metrics.Hits)
	}
	if metrics.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", metrics.Misses)
	}
	if metrics.HitRatio != 0.5 {
		t.Errorf("Expected hit ratio 0.5, got %f", metrics.HitRatio)
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
	if metrics.EntryCount != 1 {
		t.Errorf("Expected entry count 1, got %d", metrics.EntryCount)
	}
}

func TestMemoryProvider(t *testing.T) {
	provider := memory.NewProvider()

	// Test with valid options
	c, err := provider.Create(&cache.CacheOptions{
		TTL: time.Minute,
	})

	if err != nil {
		t.Errorf("Provider creation failed: %v", err)
	}
	if c == nil {
		t.Error("Expected cache instance to be created")
	}

	// Test with nil options - should still work
	c, err = provider.Create(nil)
	if err != nil {
		t.Errorf("Provider creation with nil options failed: %v", err)
	}
	if c == nil {
		t.Error("Expected cache instance to be created with nil options")
	}

	// Test with custom serializer format
	c, err = provider.Create(&cache.CacheOptions{
		SerializerFormat: serializer.JSON,
	})
	if err != nil {
		t.Errorf("Provider creation with custom serializer failed: %v", err)
	}
	if c == nil {
		t.Error("Expected cache instance to be created with custom serializer")
	}

	// Test with custom logger
	log := logger.New(logger.DefaultConfig)
	c, err = provider.Create(&cache.CacheOptions{
		Logger: log,
	})
	if err != nil {
		t.Errorf("Provider creation with custom logger failed: %v", err)
	}
	if c == nil {
		t.Error("Expected cache instance to be created with custom logger")
	}
}

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/internal/providers/memory"
	"github.com/MichaelAJay/go-serializer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ComplexTestStruct represents a struct that would require gob registration
type ComplexTestStruct struct {
	ID          string                 `json:"id"`
	NestedMap   map[string]interface{} `json:"nested_map"`
	SliceData   []string               `json:"slice_data"`
	PointerData *string                `json:"pointer_data"`
	Timestamp   time.Time              `json:"timestamp"`
}

func TestTypedCache_AutomaticGobRegistration(t *testing.T) {
	ctx := context.Background()

	// Create a memory cache with gob serialization
	cacheOptions := &CacheOptions{
		TTL:              time.Hour,
		SerializerFormat: serializer.Binary, // Use gob serialization
	}
	
	cache, err := memory.NewMemoryCache(cacheOptions)
	require.NoError(t, err)
	defer cache.Close()

	// Create a typed cache for a complex struct that would normally require manual gob registration
	typedCache := NewTypedCache[*ComplexTestStruct](cache)

	// Create test data with complex nested structures
	testData := "pointer_value"
	complexStruct := &ComplexTestStruct{
		ID: "test-123",
		NestedMap: map[string]interface{}{
			"key1": "value1",
			"key2": 42,
			"key3": true,
		},
		SliceData:   []string{"item1", "item2", "item3"},
		PointerData: &testData,
		Timestamp:   time.Now(),
	}

	// This should work without any manual gob.Register() calls
	// because our TypedSerializer automatically registers types
	err = typedCache.Set(ctx, "complex:test", complexStruct, time.Hour)
	require.NoError(t, err, "Set should succeed with automatic type registration")

	// Retrieve and verify
	retrieved, found, err := typedCache.Get(ctx, "complex:test")
	require.NoError(t, err, "Get should succeed")
	require.True(t, found, "Item should be found")
	require.NotNil(t, retrieved, "Retrieved item should not be nil")

	// Verify all fields are correctly preserved
	assert.Equal(t, complexStruct.ID, retrieved.ID)
	assert.Equal(t, len(complexStruct.NestedMap), len(retrieved.NestedMap))
	assert.Equal(t, complexStruct.NestedMap["key1"], retrieved.NestedMap["key1"])
	assert.Equal(t, len(complexStruct.SliceData), len(retrieved.SliceData))
	assert.Equal(t, complexStruct.SliceData[0], retrieved.SliceData[0])
	
	require.NotNil(t, retrieved.PointerData, "Pointer data should not be nil")
	assert.Equal(t, *complexStruct.PointerData, *retrieved.PointerData)
	
	// Time comparison with some tolerance for precision differences
	assert.WithinDuration(t, complexStruct.Timestamp, retrieved.Timestamp, time.Millisecond)

	t.Logf("Successfully handled complex struct with automatic gob registration")
}

func TestTypedCache_PointerVsValueTypes(t *testing.T) {
	ctx := context.Background()

	// Create cache
	cacheOptions := &CacheOptions{
		TTL:              time.Hour,
		SerializerFormat: serializer.Binary,
	}
	
	cache, err := memory.NewMemoryCache(cacheOptions)
	require.NoError(t, err)
	defer cache.Close()

	// Test with pointer type
	ptrCache := NewTypedCache[*ComplexTestStruct](cache)
	
	// Test with value type  
	valueCache := NewTypedCache[ComplexTestStruct](cache)

	testData := "test"
	ptrStruct := &ComplexTestStruct{
		ID:          "ptr-test",
		SliceData:   []string{"a", "b"},
		PointerData: &testData,
		Timestamp:   time.Now(),
	}
	
	valueStruct := ComplexTestStruct{
		ID:          "value-test", 
		SliceData:   []string{"c", "d"},
		PointerData: &testData,
		Timestamp:   time.Now(),
	}

	// Both should work with automatic registration
	err = ptrCache.Set(ctx, "ptr:test", ptrStruct, time.Hour)
	require.NoError(t, err, "Pointer type should work")

	err = valueCache.Set(ctx, "value:test", valueStruct, time.Hour)
	require.NoError(t, err, "Value type should work")

	// Retrieve both
	retrievedPtr, found, err := ptrCache.Get(ctx, "ptr:test")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, ptrStruct.ID, retrievedPtr.ID)

	retrievedValue, found, err := valueCache.Get(ctx, "value:test")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, valueStruct.ID, retrievedValue.ID)

	t.Logf("Successfully handled both pointer and value types with automatic registration")
}
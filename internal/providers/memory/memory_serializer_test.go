package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/internal/providers/memory"
	"github.com/MichaelAJay/go-serializer"
)

// TestMemoryCache_Serializers tests the memory cache with different serializer formats
func TestMemoryCache_Serializers(t *testing.T) {
	// Test serialization formats
	formats := []serializer.Format{
		serializer.JSON,
		serializer.Msgpack,
		// Binary format requires special handling - tested separately
	}

	// Define a struct with various field types
	type TestStruct struct {
		String  string
		Int     int
		Float   float64
		Bool    bool
		Slice   []string
		Map     map[string]int
		Pointer *string
	}

	// Create test value
	stringVal := "pointer-value"
	testValue := TestStruct{
		String:  "test",
		Int:     123,
		Float:   3.14159,
		Bool:    true,
		Slice:   []string{"a", "b", "c"},
		Map:     map[string]int{"a": 1, "b": 2},
		Pointer: &stringVal,
	}

	for _, format := range formats {
		t.Run(string(format), func(t *testing.T) {
			// Create cache with the specific serializer format
			c, err := memory.NewMemoryCache(&cache.CacheOptions{
				SerializerFormat: format,
			})
			if err != nil {
				t.Fatalf("Failed to create memory cache with %s serializer: %v", format, err)
			}
			defer c.Close()

			ctx := context.Background()

			// Store the test struct
			err = c.Set(ctx, "test-struct", testValue, time.Hour)
			if err != nil {
				t.Errorf("Failed to set value with %s serializer: %v", format, err)
				return
			}

			// Retrieve the test struct
			retrieved, exists, err := c.Get(ctx, "test-struct")
			if err != nil {
				t.Errorf("Failed to get value with %s serializer: %v", format, err)
				return
			}
			if !exists {
				t.Errorf("Value doesn't exist with %s serializer", format)
				return
			}

			// While we can't check everything due to type conversion during deserialization,
			// we can verify that key fields were preserved based on the serializer
			if retrievedMap, ok := retrieved.(map[string]any); ok {
				// Most serializers like JSON/msgpack convert to map[string]any
				if retrievedMap["String"] != testValue.String && retrievedMap["string"] != testValue.String {
					t.Errorf("String value mismatch with %s serializer", format)
				}
				// Additional checks could be added here
			} else {
				// For any other format, just ensure we got something
				if retrieved == nil {
					t.Errorf("Retrieved nil with %s serializer", format)
				}
			}
		})
	}

	// Test binary serializer separately with simple primitive types
	t.Run("binary-simple-types", func(t *testing.T) {
		c, err := memory.NewMemoryCache(&cache.CacheOptions{
			SerializerFormat: serializer.Binary,
		})
		if err != nil {
			t.Fatalf("Failed to create memory cache with binary serializer: %v", err)
		}
		defer c.Close()

		ctx := context.Background()

		// Test with string
		if err := c.Set(ctx, "string-key", "string-value", time.Hour); err != nil {
			t.Errorf("Failed to set string with binary serializer: %v", err)
		}

		// Retrieve string
		if val, exists, err := c.Get(ctx, "string-key"); err != nil {
			t.Errorf("Failed to get string with binary serializer: %v", err)
		} else if !exists {
			t.Error("String doesn't exist with binary serializer")
		} else if val != "string-value" {
			t.Errorf("String value mismatch with binary serializer: got %v", val)
		}

		// Test with int
		if err := c.Set(ctx, "int-key", 42, time.Hour); err != nil {
			t.Errorf("Failed to set int with binary serializer: %v", err)
		}

		// Retrieve int (don't check exact type, just value equivalence)
		if val, exists, err := c.Get(ctx, "int-key"); err != nil {
			t.Errorf("Failed to get int with binary serializer: %v", err)
		} else if !exists {
			t.Error("Int doesn't exist with binary serializer")
		} else {
			// Convert to float64 for comparison to handle various numeric types
			var numValue float64
			switch v := val.(type) {
			case int:
				numValue = float64(v)
			case int8:
				numValue = float64(v)
			case int16:
				numValue = float64(v)
			case int32:
				numValue = float64(v)
			case int64:
				numValue = float64(v)
			case float32:
				numValue = float64(v)
			case float64:
				numValue = v
			default:
				t.Errorf("Unexpected type for int value: %T", val)
			}

			if numValue != 42 {
				t.Errorf("Int value mismatch with binary serializer: got %v", numValue)
			}
		}
	})
}

// TestMemoryCache_SerializationErrors tests error cases with serialization
func TestMemoryCache_SerializationErrors(t *testing.T) {
	c, err := memory.NewMemoryCache(&cache.CacheOptions{
		SerializerFormat: serializer.JSON,
	})
	if err != nil {
		t.Fatalf("Failed to create memory cache: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	// Try to serialize something that can't be serialized
	type InvalidType struct {
		Chan chan int // Channels can't be serialized in most formats
	}

	err = c.Set(ctx, "invalid", InvalidType{Chan: make(chan int)}, 0)
	if err == nil {
		t.Error("Expected serialization error for invalid type, got nil")
	}
}

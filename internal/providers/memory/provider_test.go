package memory

import (
	"context"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-logger"
	"github.com/MichaelAJay/go-serializer"
)

// TestMemoryProviderUnit conducts unit tests for the memory provider
func TestMemoryProviderUnit(t *testing.T) {
	provider := NewProvider()

	// Test with nil options - memory provider should handle this gracefully
	t.Run("NilOptions", func(t *testing.T) {
		c, err := provider.Create(nil)
		if err != nil {
			t.Errorf("Expected nil error with nil options, got %v", err)
		}
		if c == nil {
			t.Error("Expected non-nil cache instance with nil options")
		}
	})

	// Test with minimal valid options
	t.Run("MinimalValidOptions", func(t *testing.T) {
		c, err := provider.Create(&cache.CacheOptions{
			TTL: time.Hour,
		})
		if err != nil {
			t.Errorf("Provider creation failed with minimal options: %v", err)
		}
		if c == nil {
			t.Error("Expected cache instance to be created with minimal options")
		}
	})
}

// TestProvider_Create tests the cache provider's Create method
func TestProvider_Create(t *testing.T) {
	// Test with valid options
	options := &cache.CacheOptions{
		TTL:             time.Minute,
		MaxEntries:      1000,
		MaxSize:         1024 * 1024 * 10, // 10MB
		CleanupInterval: time.Minute,
	}

	provider := NewProvider()
	c, err := provider.Create(options)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
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

	// Test with custom logger
	log := logger.New(logger.DefaultConfig)
	optionsWithLogger := &cache.CacheOptions{
		TTL:             time.Minute,
		MaxEntries:      1000,
		MaxSize:         1024 * 1024 * 10, // 10MB
		CleanupInterval: time.Minute,
		Logger:          log,
	}

	c, err = provider.Create(optionsWithLogger)
	if err != nil {
		t.Fatalf("Failed to create cache with logger: %v", err)
	}
	defer c.Close()

	// Test with custom serializer format
	optionsWithSerializer := &cache.CacheOptions{
		TTL:              time.Minute,
		MaxEntries:       1000,
		MaxSize:          1024 * 1024 * 10, // 10MB
		CleanupInterval:  time.Minute,
		SerializerFormat: serializer.JSON,
	}

	c, err = provider.Create(optionsWithSerializer)
	if err != nil {
		t.Fatalf("Failed to create cache with custom serializer: %v", err)
	}
	defer c.Close()

	// Test serialization with the custom format cache
	ctx = context.Background()
	type CustomType struct {
		Name  string
		Value int
	}
	custom := CustomType{Name: "test", Value: 42}

	err = c.Set(ctx, "custom", custom, time.Hour)
	if err != nil {
		t.Errorf("Set failed for custom type: %v", err)
	}

	retrieved, exists, err := c.Get(ctx, "custom")
	if err != nil || !exists {
		t.Errorf("Get failed for custom type: %v", err)
	}
	if retrieved == nil {
		t.Error("Retrieved value is nil")
	}
}

// TestProvider_ZeroValues tests the provider with zero or unspecified values
func TestProvider_ZeroValues(t *testing.T) {
	provider := NewProvider()

	// Test with empty options struct
	c, err := provider.Create(&cache.CacheOptions{})
	if err != nil {
		t.Fatalf("Failed to create cache with empty options: %v", err)
	}
	defer c.Close()

	// Test basic operations
	ctx := context.Background()
	err = c.Set(ctx, "key", "value", 0) // Using default TTL
	if err != nil {
		t.Errorf("Set failed with default TTL: %v", err)
	}

	val, exists, err := c.Get(ctx, "key")
	if err != nil || !exists {
		t.Errorf("Get failed: %v", err)
	}
	if val != "value" {
		t.Errorf("Expected 'value', got %v", val)
	}
}

// TestProvider_Limits tests the provider with various limit configurations
func TestProvider_Limits(t *testing.T) {
	provider := NewProvider()

	// Test with small max entries
	t.Run("SmallMaxEntries", func(t *testing.T) {
		c, err := provider.Create(&cache.CacheOptions{
			MaxEntries: 2,
		})
		if err != nil {
			t.Fatalf("Failed to create cache: %v", err)
		}
		defer c.Close()

		ctx := context.Background()

		// Add two entries (should succeed)
		if err := c.Set(ctx, "key1", "value1", 0); err != nil {
			t.Errorf("Failed to set key1: %v", err)
		}
		if err := c.Set(ctx, "key2", "value2", 0); err != nil {
			t.Errorf("Failed to set key2: %v", err)
		}

		// Try to add a third entry (should fail)
		err = c.Set(ctx, "key3", "value3", 0)
		if err != cache.ErrCacheFull {
			t.Errorf("Expected ErrCacheFull, got %v", err)
		}
	})

	// Test with small max size
	t.Run("SmallMaxSize", func(t *testing.T) {
		c, err := provider.Create(&cache.CacheOptions{
			MaxSize: 20, // Very small size limit
		})
		if err != nil {
			t.Fatalf("Failed to create cache: %v", err)
		}
		defer c.Close()

		ctx := context.Background()

		// Add a small entry (should succeed)
		if err := c.Set(ctx, "small", "abc", 0); err != nil {
			t.Errorf("Failed to set small value: %v", err)
		}

		// Try to add a large entry (should fail)
		err = c.Set(ctx, "large", "this is a very long string that exceeds the size limit", 0)
		if err != cache.ErrCacheFull {
			t.Errorf("Expected ErrCacheFull for large value, got %v", err)
		}
	})
}

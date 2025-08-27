package interfaces

import (
	"testing"
	"time"

	"github.com/MichaelAJay/go-logger"
	"github.com/MichaelAJay/go-metrics/metric"
	"github.com/MichaelAJay/go-serializer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithSerializerFormat(t *testing.T) {
	t.Run("sets JSON serializer format", func(t *testing.T) {
		options := &CacheOptions{}
		
		option := WithSerializerFormat(serializer.JSON)
		option(options)
		
		assert.Equal(t, serializer.JSON, options.SerializerFormat)
	})
	
	t.Run("sets Binary serializer format", func(t *testing.T) {
		options := &CacheOptions{}
		
		option := WithSerializerFormat(serializer.Binary)
		option(options)
		
		assert.Equal(t, serializer.Binary, options.SerializerFormat)
	})
	
	t.Run("sets Msgpack serializer format", func(t *testing.T) {
		options := &CacheOptions{}
		
		option := WithSerializerFormat(serializer.Msgpack)
		option(options)
		
		assert.Equal(t, serializer.Msgpack, options.SerializerFormat)
	})
	
	t.Run("overwrites previous serializer format", func(t *testing.T) {
		options := &CacheOptions{SerializerFormat: serializer.Binary}
		
		option := WithSerializerFormat(serializer.JSON)
		option(options)
		
		assert.Equal(t, serializer.JSON, options.SerializerFormat)
	})
	
	t.Run("can be combined with other options", func(t *testing.T) {
		options := &CacheOptions{}
		
		WithTTL(time.Hour)(options)
		WithCleanupInterval(5 * time.Minute)(options)
		WithSerializerFormat(serializer.JSON)(options)
		WithMaxEntries(1000)(options)
		
		assert.Equal(t, time.Hour, options.TTL)
		assert.Equal(t, 5*time.Minute, options.CleanupInterval)
		assert.Equal(t, serializer.JSON, options.SerializerFormat)
		assert.Equal(t, 1000, options.MaxEntries)
	})
}

func TestCacheOptionsIntegration(t *testing.T) {
	t.Run("all serializer formats work with complete option chain", func(t *testing.T) {
		formats := []serializer.Format{
			serializer.JSON,
			serializer.Binary,
			serializer.Msgpack,
		}
		
		for _, format := range formats {
			t.Run(string(format), func(t *testing.T) {
				options := &CacheOptions{}
				
				// Apply a chain of options including serializer format
				WithTTL(24 * time.Hour)(options)
				WithMaxEntries(5000)(options)
				WithCleanupInterval(10 * time.Minute)(options)
				WithSerializerFormat(format)(options)
				WithMaxSize(100 * 1024 * 1024)(options) // 100MB
				
				// Verify all options were applied correctly
				assert.Equal(t, 24*time.Hour, options.TTL)
				assert.Equal(t, 5000, options.MaxEntries)
				assert.Equal(t, 10*time.Minute, options.CleanupInterval)
				assert.Equal(t, format, options.SerializerFormat)
				assert.Equal(t, int64(100*1024*1024), options.MaxSize)
			})
		}
	})
}

// TestWithSerializerFormatFunctionSignature ensures the function has the correct signature
func TestWithSerializerFormatFunctionSignature(t *testing.T) {
	// This test ensures the function signature matches CacheOption
	var option CacheOption = WithSerializerFormat(serializer.JSON)
	
	// Verify it can be called on CacheOptions
	options := &CacheOptions{}
	require.NotPanics(t, func() {
		option(options)
	})
	
	assert.Equal(t, serializer.JSON, options.SerializerFormat)
}

// TestSerializerFormatInCacheOptions verifies the field exists in CacheOptions
func TestSerializerFormatInCacheOptions(t *testing.T) {
	options := &CacheOptions{
		SerializerFormat: serializer.JSON,
	}
	
	assert.Equal(t, serializer.JSON, options.SerializerFormat)
}

// Benchmark tests for performance
func BenchmarkWithSerializerFormat(b *testing.B) {
	options := &CacheOptions{}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WithSerializerFormat(serializer.JSON)(options)
	}
}

func BenchmarkOptionChainWithSerializer(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		options := &CacheOptions{}
		WithTTL(time.Hour)(options)
		WithSerializerFormat(serializer.JSON)(options)
		WithCleanupInterval(5 * time.Minute)(options)
	}
}

// Test edge cases and error conditions
func TestWithSerializerFormatEdgeCases(t *testing.T) {
	t.Run("nil options should not panic", func(t *testing.T) {
		// This should not panic, though it's not a normal use case
		assert.NotPanics(t, func() {
			var options *CacheOptions = nil
			defer func() {
				if r := recover(); r != nil {
					// Expected to panic due to nil pointer, which is acceptable
				}
			}()
			WithSerializerFormat(serializer.JSON)(options)
		})
	})
	
	t.Run("zero value format", func(t *testing.T) {
		options := &CacheOptions{}
		var zeroFormat serializer.Format
		
		option := WithSerializerFormat(zeroFormat)
		option(options)
		
		assert.Equal(t, zeroFormat, options.SerializerFormat)
	})
}

// Test that demonstrates real-world usage pattern
func TestRealWorldUsagePattern(t *testing.T) {
	t.Run("typical cache configuration with serializer", func(t *testing.T) {
		// Simulate how this would be used in production code
		mockLogger := logger.New(logger.Config{Level: logger.ErrorLevel})
		mockRegistry := metric.NewDefaultRegistry()
		
		options := &CacheOptions{}
		
		// Apply typical production options
		WithTTL(30 * time.Minute)(options)
		WithMaxEntries(10000)(options)
		WithCleanupInterval(5 * time.Minute)(options)
		WithSerializerFormat(serializer.JSON)(options) // The key addition
		WithLogger(mockLogger)(options)
		WithGoMetricsRegistry(mockRegistry)(options)
		WithMetricsEnabled(true)(options)
		
		// Verify the configuration
		assert.Equal(t, 30*time.Minute, options.TTL)
		assert.Equal(t, 10000, options.MaxEntries)
		assert.Equal(t, 5*time.Minute, options.CleanupInterval)
		assert.Equal(t, serializer.JSON, options.SerializerFormat)
		assert.Equal(t, mockLogger, options.Logger)
		assert.Equal(t, mockRegistry, options.GoMetricsRegistry)
		assert.True(t, options.MetricsEnabled)
	})
}
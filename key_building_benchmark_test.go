package cache

import (
	"strings"
	"testing"
)

func BenchmarkBuildDataKey_FastPath(b *testing.B) {
	cache := &RedisCache[string]{
		redisOptions: nil, // Fast path - no prefix or version
	}
	cache.precomputePrefixes() // Initialize precomputed prefixes
	
	key := "test_key"
	var result string
	
	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		result = cache.buildDataKey(key)
	}
	
	// Force the compiler to keep the result
	if len(result) == 0 {
		b.Fatal("unexpected empty result")
	}
}

func BenchmarkBuildDataKey_WithVersion(b *testing.B) {
	cache := &RedisCache[string]{
		redisOptions: &RedisOptions{
			Version: "v1",
		},
	}
	cache.precomputePrefixes() // Initialize precomputed prefixes
	
	key := "test_key"
	var result string
	
	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		result = cache.buildDataKey(key)
	}
	
	// Force the compiler to keep the result
	if len(result) == 0 {
		b.Fatal("unexpected empty result")
	}
}

func BenchmarkBuildDataKey_WithPrefix(b *testing.B) {
	cache := &RedisCache[string]{
		redisOptions: &RedisOptions{
			DataPrefix: "myapp:",
		},
	}
	
	key := "test_key"
	
	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_ = cache.buildDataKey(key)
	}
}

func BenchmarkBuildDataKey_WithPrefixAndVersion(b *testing.B) {
	cache := &RedisCache[string]{
		redisOptions: &RedisOptions{
			DataPrefix: "myapp:",
			Version:    "v1",
		},
	}
	
	key := "test_key"
	
	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_ = cache.buildDataKey(key)
	}
}

// Comparison with old implementation using string concatenation
func BenchmarkBuildDataKey_OldImplementation_FastPath(b *testing.B) {
	key := "test_key"
	
	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Old fast path would have been: "cache:data:" + key
		_ = "cache:data:" + key
	}
}

func BenchmarkBuildDataKey_OldImplementation_WithVersion(b *testing.B) {
	key := "test_key"
	version := "v1"
	
	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Old implementation
		finalKey := key + ":" + version
		_ = "cache:data:" + finalKey
	}
}

func BenchmarkBuildMetaKey_FastPath(b *testing.B) {
	cache := &RedisCache[string]{
		redisOptions: nil, // Fast path
	}
	
	key := "test_key"
	
	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_ = cache.buildMetaKey(key)
	}
}

func BenchmarkBuildMetaKey_WithVersion(b *testing.B) {
	cache := &RedisCache[string]{
		redisOptions: &RedisOptions{
			Version: "v1",
		},
	}
	
	key := "test_key"
	
	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_ = cache.buildMetaKey(key)
	}
}

func BenchmarkBuildLockKey_FastPath(b *testing.B) {
	cache := &RedisCache[string]{
		redisOptions: nil, // Fast path
	}
	
	key := "test_key"
	
	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_ = cache.buildLockKey(key)
	}
}

func BenchmarkBuildLockKey_WithVersion(b *testing.B) {
	cache := &RedisCache[string]{
		redisOptions: &RedisOptions{
			Version: "v1",
		},
	}
	
	key := "test_key"
	
	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_ = cache.buildLockKey(key)
	}
}

// Direct comparison: pooled vs manual string building
func BenchmarkKeyBuilding_Methods(b *testing.B) {
	key := "test_key"
	version := "v1"
	prefix := "myapp:"
	
	b.Run("StringConcatenation", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			finalKey := key + ":" + version
			_ = prefix + finalKey
		}
	})
	
	b.Run("StringsBuilder", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var builder strings.Builder
			builder.WriteString(key)
			builder.WriteString(":")
			builder.WriteString(version)
			finalKey := builder.String()
			
			builder.Reset()
			builder.WriteString(prefix)
			builder.WriteString(finalKey)
			_ = builder.String()
		}
	})
}
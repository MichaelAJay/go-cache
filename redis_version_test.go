package cache

import (
	"testing"
)

// TestVersionSuffixKeyBuilding verifies that key building functions append version suffixes
func TestVersionSuffixKeyBuilding(t *testing.T) {

	t.Run("buildDataKey with version suffix", func(t *testing.T) {
		// Create cache with version
		cache := &RedisCache[string]{
			redisOptions: &RedisOptions{
				Version: "v2",
			},
		}

		key := cache.buildDataKey("testkey")
		expected := "cache:data:testkey:v2"
		if key != expected {
			t.Errorf("Expected key '%s', got '%s'", expected, key)
		}
	})

	t.Run("buildDataKey without version suffix", func(t *testing.T) {
		// Create cache without version
		cache := &RedisCache[string]{
			redisOptions: nil,
		}

		key := cache.buildDataKey("testkey")
		expected := "cache:data:testkey"
		if key != expected {
			t.Errorf("Expected key '%s', got '%s'", expected, key)
		}
	})

	t.Run("buildDataKey with custom prefix and version", func(t *testing.T) {
		// Create cache with custom prefix and version
		cache := &RedisCache[string]{
			redisOptions: &RedisOptions{
				DataPrefix: "session:",
				Version:    "v3",
			},
		}

		key := cache.buildDataKey("abc123")
		expected := "session:abc123:v3"
		if key != expected {
			t.Errorf("Expected key '%s', got '%s'", expected, key)
		}
	})

	t.Run("buildMetaKey with version suffix", func(t *testing.T) {
		// Create cache with version
		cache := &RedisCache[string]{
			redisOptions: &RedisOptions{
				Version: "v2",
			},
		}

		key := cache.buildMetaKey("testkey")
		expected := "cache:meta:testkey:v2"
		if key != expected {
			t.Errorf("Expected key '%s', got '%s'", expected, key)
		}
	})

	t.Run("buildReverseIndexKey with version suffix", func(t *testing.T) {
		// Create cache with version
		cache := &RedisCache[string]{
			redisOptions: &RedisOptions{
				Version: "v2",
			},
		}

		key := cache.buildReverseIndexKey("testkey")
		expected := "cache:reverse:testkey:v2"
		if key != expected {
			t.Errorf("Expected key '%s', got '%s'", expected, key)
		}
	})

	t.Run("buildLockKey with version suffix", func(t *testing.T) {
		// Create cache with version
		cache := &RedisCache[string]{
			redisOptions: &RedisOptions{
				Version: "v2",
			},
		}

		key := cache.buildLockKey("testkey")
		expected := "cache:lock:testkey:v2"
		if key != expected {
			t.Errorf("Expected key '%s', got '%s'", expected, key)
		}
	})

	t.Run("Different versions produce different keys", func(t *testing.T) {
		cache1 := &RedisCache[string]{
			redisOptions: &RedisOptions{Version: "v1"},
		}
		cache2 := &RedisCache[string]{
			redisOptions: &RedisOptions{Version: "v2"},
		}

		key1 := cache1.buildDataKey("samekey")
		key2 := cache2.buildDataKey("samekey")

		expected1 := "cache:data:samekey:v1"
		expected2 := "cache:data:samekey:v2"

		if key1 != expected1 {
			t.Errorf("Expected v1 key '%s', got '%s'", expected1, key1)
		}
		if key2 != expected2 {
			t.Errorf("Expected v2 key '%s', got '%s'", expected2, key2)
		}
		if key1 == key2 {
			t.Error("Different versions should produce different keys")
		}
	})
}
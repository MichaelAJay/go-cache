package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/providers/memory"
)

func TestSecondaryIndexing(t *testing.T) {
	// Create cache with indexes configured
	options := &cache.CacheOptions{
		TTL:         time.Hour,
		MaxEntries:  100,
		CleanupInterval: time.Minute,
		Indexes: map[string]string{
			"sessions_by_user": "session:*",
		},
	}

	memCache, err := memory.NewMemoryCache(options)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer memCache.Close()

	ctx := context.Background()

	// Set some session entries
	err = memCache.Set(ctx, "session:123", "user1_session", time.Hour)
	if err != nil {
		t.Fatalf("Failed to set session:123: %v", err)
	}

	err = memCache.Set(ctx, "session:456", "user1_session2", time.Hour)
	if err != nil {
		t.Fatalf("Failed to set session:456: %v", err)
	}

	err = memCache.Set(ctx, "session:789", "user2_session", time.Hour)
	if err != nil {
		t.Fatalf("Failed to set session:789: %v", err)
	}

	// Add index entries
	err = memCache.AddIndex(ctx, "sessions_by_user", "session:*", "user1")
	if err != nil {
		t.Fatalf("Failed to add index for user1: %v", err)
	}

	err = memCache.AddIndex(ctx, "sessions_by_user", "session:*", "user2")
	if err != nil {
		t.Fatalf("Failed to add index for user2: %v", err)
	}

	// Test GetByIndex
	user1Sessions, err := memCache.GetByIndex(ctx, "sessions_by_user", "user1")
	if err != nil {
		t.Fatalf("Failed to get sessions for user1: %v", err)
	}

	if len(user1Sessions) != 3 {
		t.Errorf("Expected 3 sessions for user1, got %d: %v", len(user1Sessions), user1Sessions)
	}

	// Test pattern operations
	sessionKeys, err := memCache.GetKeysByPattern(ctx, "session:*")
	if err != nil {
		t.Fatalf("Failed to get keys by pattern: %v", err)
	}

	if len(sessionKeys) != 3 {
		t.Errorf("Expected 3 session keys, got %d: %v", len(sessionKeys), sessionKeys)
	}
}

func TestTimingProtection(t *testing.T) {
	// Create cache with timing protection enabled
	options := &cache.CacheOptions{
		TTL:         time.Hour,
		MaxEntries:  100,
		Security: &cache.SecurityConfig{
			EnableTimingProtection: true,
			MinProcessingTime:      5 * time.Millisecond,
		},
	}

	memCache, err := memory.NewMemoryCache(options)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer memCache.Close()

	ctx := context.Background()

	// Test that operations take at least the minimum processing time
	start := time.Now()
	_, _, err = memCache.Get(ctx, "nonexistent")
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if duration < 5*time.Millisecond {
		t.Errorf("Operation took %v, expected at least 5ms", duration)
	}
}

func TestHooks(t *testing.T) {
	preGetCalled := false
	postGetCalled := false

	// Create cache with hooks
	options := &cache.CacheOptions{
		TTL:         time.Hour,
		MaxEntries:  100,
		Hooks: &cache.CacheHooks{
			PreGet: func(ctx context.Context, key string) error {
				preGetCalled = true
				return nil
			},
			PostGet: func(ctx context.Context, key string, value any, found bool, err error) {
				postGetCalled = true
			},
		},
	}

	memCache, err := memory.NewMemoryCache(options)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer memCache.Close()

	ctx := context.Background()

	// Perform a Get operation
	_, _, err = memCache.Get(ctx, "test")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !preGetCalled {
		t.Error("PreGet hook was not called")
	}

	if !postGetCalled {
		t.Error("PostGet hook was not called")
	}
}
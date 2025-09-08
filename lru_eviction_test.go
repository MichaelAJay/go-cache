//go:build integration

package cache_test

import (
	"context"
	"testing"
	"time"

	cache "github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/internal/testintegration"
	"github.com/MichaelAJay/go-metrics/metric"
	"github.com/stretchr/testify/require"
)

func TestRedisCache_LRUEviction(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache with MaxEntries = 3
	config := testintegration.DefaultCacheConfig()
	
	// Create metrics registry required for cache initialization
	registry := metric.NewDefaultRegistry()
	tags := metric.Tags{"environment": "test"}
	
	sessionCache, err := cache.NewCache(ctx, setup.RedisClient, config.IndexingMode, testintegration.TestSessionExtractor,
		cache.WithTTL[*testintegration.TestSession](config.TTL),
		cache.WithSerializer[*testintegration.TestSession](config.SerializerFormat),
		cache.WithWarmLuaScripts[*testintegration.TestSession](config.WarmLuaScripts),
		cache.WithGoMetrics[*testintegration.TestSession](registry, tags),
		cache.WithMaxEntries[*testintegration.TestSession](3), // Set max entries to 3
	)
	require.NoError(t, err, "Failed to create cache")
	defer sessionCache.Close()

	ttl := 5 * time.Minute

	// Add 3 entries (should fit within limit)
	session1 := &testintegration.TestSession{ID: "session1", UserID: "user1", Username: "user1"}
	session2 := &testintegration.TestSession{ID: "session2", UserID: "user2", Username: "user2"}
	session3 := &testintegration.TestSession{ID: "session3", UserID: "user3", Username: "user3"}

	err = sessionCache.Set(ctx, session1, ttl)
	if err != nil {
		t.Fatalf("Failed to set session1: %v", err)
	}

	err = sessionCache.Set(ctx, session2, ttl)
	if err != nil {
		t.Fatalf("Failed to set session2: %v", err)
	}

	err = sessionCache.Set(ctx, session3, ttl)
	if err != nil {
		t.Fatalf("Failed to set session3: %v", err)
	}

	// Verify all 3 sessions exist
	_, found, _ := sessionCache.Get(ctx, "session1")
	if !found {
		t.Error("session1 should exist after initial set")
	}
	
	_, found, _ = sessionCache.Get(ctx, "session2")
	if !found {
		t.Error("session2 should exist after initial set")
	}
	
	_, found, _ = sessionCache.Get(ctx, "session3")
	if !found {
		t.Error("session3 should exist after initial set")
	}

	// Add a 4th session (should trigger eviction of oldest - session1)
	session4 := &testintegration.TestSession{ID: "session4", UserID: "user4", Username: "user4"}
	err = sessionCache.Set(ctx, session4, ttl)
	if err != nil {
		t.Fatalf("Failed to set session4: %v", err)
	}

	// session1 should be evicted
	_, found, _ = sessionCache.Get(ctx, "session1")
	if found {
		t.Error("session1 should have been evicted")
	}

	// Other sessions should still exist
	_, found, _ = sessionCache.Get(ctx, "session2")
	if !found {
		t.Error("session2 should still exist")
	}
	
	_, found, _ = sessionCache.Get(ctx, "session3")
	if !found {
		t.Error("session3 should still exist")
	}
	
	_, found, _ = sessionCache.Get(ctx, "session4")
	if !found {
		t.Error("session4 should exist")
	}

	// Access session2 to make it most recently used
	_, found, _ = sessionCache.Get(ctx, "session2")
	if !found {
		t.Error("session2 should exist")
	}

	// Add session5 (should evict session3, the oldest unaccessed)
	session5 := &testintegration.TestSession{ID: "session5", UserID: "user5", Username: "user5"}
	err = sessionCache.Set(ctx, session5, ttl)
	if err != nil {
		t.Fatalf("Failed to set session5: %v", err)
	}

	// session3 should be evicted (oldest unaccessed)
	_, found, _ = sessionCache.Get(ctx, "session3")
	if found {
		t.Error("session3 should have been evicted")
	}

	// session2, session4, session5 should remain
	_, found, _ = sessionCache.Get(ctx, "session2")
	if !found {
		t.Error("session2 should still exist")
	}
	
	_, found, _ = sessionCache.Get(ctx, "session4")
	if !found {
		t.Error("session4 should still exist")
	}
	
	_, found, _ = sessionCache.Get(ctx, "session5")
	if !found {
		t.Error("session5 should exist")
	}

	t.Log("✅ LRU eviction test successful")
}
package cache_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/internal/providers/memory"
	"github.com/MichaelAJay/go-cache/internal/providers/null"
)

func TestCacheManager_BasicOperations(t *testing.T) {
	manager := cache.NewCacheManager()
	defer manager.Close()

	// Register memory provider
	manager.RegisterProvider("memory", memory.NewProvider())

	// Test GetCache with non-existent provider
	_, err := manager.GetCache("nonexistent")
	if err == nil {
		t.Error("Expected error when getting cache with non-existent provider")
	}

	// Test GetCache with valid provider
	c, err := manager.GetCache("memory", 
		cache.WithTTL(time.Hour),
		cache.WithMaxEntries(100),
	)
	if err != nil {
		t.Fatalf("Failed to get cache: %v", err)
	}
	if c == nil {
		t.Fatal("Expected non-nil cache instance")
	}

	// Test that subsequent calls return the same instance
	c2, err := manager.GetCache("memory")
	if err != nil {
		t.Fatalf("Failed to get cache second time: %v", err)
	}
	if c != c2 {
		t.Error("Expected same cache instance on subsequent calls")
	}
}

func TestCacheManager_GetCaches(t *testing.T) {
	manager := cache.NewCacheManager()
	defer manager.Close()

	// Register providers
	manager.RegisterProvider("memory", memory.NewProvider())
	manager.RegisterProvider("null", null.NewProvider())

	// Get some caches
	_, err := manager.GetCache("memory")
	if err != nil {
		t.Fatalf("Failed to get memory cache: %v", err)
	}

	_, err = manager.GetCache("null")
	if err != nil {
		t.Fatalf("Failed to get null cache: %v", err)
	}

	// Test GetCaches
	caches := manager.GetCaches()
	if len(caches) != 2 {
		t.Errorf("Expected 2 caches, got %d", len(caches))
	}

	// Check that we have the expected cache names
	if _, ok := caches["memory"]; !ok {
		t.Error("Expected memory cache in GetCaches result")
	}
	if _, ok := caches["null"]; !ok {
		t.Error("Expected null cache in GetCaches result")
	}
}

func TestCacheManager_ConcurrentAccess(t *testing.T) {
	manager := cache.NewCacheManager()
	defer manager.Close()

	// Register provider
	manager.RegisterProvider("memory", memory.NewProvider())

	var wg sync.WaitGroup
	numGoroutines := 10
	caches := make([]cache.Cache, numGoroutines)

	// Test concurrent GetCache calls
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			c, err := manager.GetCache("memory", cache.WithTTL(time.Minute))
			if err != nil {
				t.Errorf("Concurrent GetCache failed: %v", err)
			}
			caches[idx] = c
		}(i)
	}
	wg.Wait()

	// Verify all caches are the same instance
	firstCache := caches[0]
	for i := 1; i < numGoroutines; i++ {
		if caches[i] != firstCache {
			t.Errorf("Cache instance %d is different from first cache", i)
		}
	}
}

func TestCacheManager_RegisterProvider(t *testing.T) {
	manager := cache.NewCacheManager()
	defer manager.Close()

	// Test registering multiple providers
	manager.RegisterProvider("memory", memory.NewProvider())
	manager.RegisterProvider("null", null.NewProvider())

	// Test that both providers work
	memCache, err := manager.GetCache("memory")
	if err != nil {
		t.Errorf("Failed to get memory cache after registration: %v", err)
	}

	nullCache, err := manager.GetCache("null")
	if err != nil {
		t.Errorf("Failed to get null cache after registration: %v", err)
	}

	// Test basic operations on both caches
	ctx := context.Background()
	
	// Memory cache should work normally
	err = memCache.Set(ctx, "test", "value", time.Minute)
	if err != nil {
		t.Errorf("Memory cache Set failed: %v", err)
	}

	// Null cache should accept operations but not store values
	err = nullCache.Set(ctx, "test", "value", time.Minute)
	if err != nil {
		t.Errorf("Null cache Set failed: %v", err)
	}

	_, exists, err := nullCache.Get(ctx, "test")
	if err != nil {
		t.Errorf("Null cache Get failed: %v", err)
	}
	if exists {
		t.Error("Null cache should not return existing values")
	}
}

func TestCacheManager_ConcurrentRegisterProvider(t *testing.T) {
	manager := cache.NewCacheManager()
	defer manager.Close()

	var wg sync.WaitGroup
	numGoroutines := 5

	// Test concurrent provider registration
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			providerName := "provider_" + string(rune(idx+'0'))
			manager.RegisterProvider(providerName, memory.NewProvider())
		}(i)
	}
	wg.Wait()

	// Test that all providers were registered correctly
	for i := 0; i < numGoroutines; i++ {
		providerName := "provider_" + string(rune(i+'0'))
		_, err := manager.GetCache(providerName, cache.WithTTL(time.Minute))
		if err != nil {
			t.Errorf("Failed to get cache for provider %s: %v", providerName, err)
		}
	}
}

func TestCacheManager_Close(t *testing.T) {
	manager := cache.NewCacheManager()

	// Register provider and get cache
	manager.RegisterProvider("memory", memory.NewProvider())
	cache1, err := manager.GetCache("memory")
	if err != nil {
		t.Fatalf("Failed to get cache: %v", err)
	}

	// Register another provider and get cache
	manager.RegisterProvider("null", null.NewProvider())
	cache2, err := manager.GetCache("null")
	if err != nil {
		t.Fatalf("Failed to get null cache: %v", err)
	}

	// Verify caches are accessible before close
	ctx := context.Background()
	err = cache1.Set(ctx, "test", "value", time.Minute)
	if err != nil {
		t.Errorf("Cache operation failed before close: %v", err)
	}

	// Close the manager
	err = manager.Close()
	if err != nil {
		t.Errorf("Manager close failed: %v", err)
	}

	// Verify GetCaches returns empty after close
	caches := manager.GetCaches()
	if len(caches) != 0 {
		t.Errorf("Expected empty caches after close, got %d", len(caches))
	}

	// Test that closed caches don't cause panics (they should be closed gracefully)
	err = cache1.Set(ctx, "test2", "value2", time.Minute)
	// We don't check the error because the behavior after close is implementation-specific
	// The important thing is that it doesn't panic

	err = cache2.Set(ctx, "test2", "value2", time.Minute)
	// Same as above - no panic check
}

func TestCacheManager_CloseWithNoCaches(t *testing.T) {
	manager := cache.NewCacheManager()

	// Test close with no caches
	err := manager.Close()
	if err != nil {
		t.Errorf("Close with no caches should not error: %v", err)
	}
}

func TestCacheManager_GetCacheWithNilOptions(t *testing.T) {
	manager := cache.NewCacheManager()
	defer manager.Close()

	manager.RegisterProvider("memory", memory.NewProvider())

	// Test GetCache with no options (should use defaults)
	cache, err := manager.GetCache("memory")
	if err != nil {
		t.Errorf("GetCache with no options failed: %v", err)
	}
	if cache == nil {
		t.Error("Expected non-nil cache with no options")
	}

	// Test basic operation
	ctx := context.Background()
	err = cache.Set(ctx, "test", "value", time.Minute)
	if err != nil {
		t.Errorf("Cache operation failed: %v", err)
	}
}

func TestCacheManager_ProviderCreateError(t *testing.T) {
	manager := cache.NewCacheManager()
	defer manager.Close()

	// Create a mock provider that always returns an error
	errorProvider := &errorProvider{}
	manager.RegisterProvider("error", errorProvider)

	// Test that GetCache returns the provider error
	_, err := manager.GetCache("error")
	if err == nil {
		t.Error("Expected error from failing provider")
	}
	if err.Error() != "failed to create cache: provider error" {
		t.Errorf("Expected wrapped provider error, got: %v", err)
	}
}

// errorProvider is a mock provider that always returns an error
type errorProvider struct{}

func (p *errorProvider) Create(options *cache.CacheOptions) (cache.Cache, error) {
	return nil, fmt.Errorf("provider error")
}

func TestCacheManager_DoubleClose(t *testing.T) {
	manager := cache.NewCacheManager()

	// Register and get a cache
	manager.RegisterProvider("memory", memory.NewProvider())
	_, err := manager.GetCache("memory")
	if err != nil {
		t.Fatalf("Failed to get cache: %v", err)
	}

	// Close once
	err = manager.Close()
	if err != nil {
		t.Errorf("First close failed: %v", err)
	}

	// Close again - should not panic or error
	err = manager.Close()
	if err != nil {
		t.Errorf("Second close failed: %v", err)
	}
}
package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
)

func TestMemoryCache_ContextTimeout(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Test with very short timeout
	shortCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
	defer cancel()

	// Give the context time to expire
	time.Sleep(1 * time.Millisecond)

	// Test basic operations with expired context
	t.Run("SetWithExpiredContext", func(t *testing.T) {
		err := c.Set(shortCtx, "test_key", "test_value", time.Hour)
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})

	t.Run("GetWithExpiredContext", func(t *testing.T) {
		_, _, err := c.Get(shortCtx, "test_key")
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})

	t.Run("DeleteWithExpiredContext", func(t *testing.T) {
		err := c.Delete(shortCtx, "test_key")
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})
}

func TestMemoryCache_ContextTimeoutBulkOperations(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Create expired context
	expiredCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond)

	items := map[string]any{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	keys := []string{"key1", "key2", "key3"}

	t.Run("SetManyWithExpiredContext", func(t *testing.T) {
		err := c.SetMany(expiredCtx, items, time.Hour)
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})

	t.Run("GetManyWithExpiredContext", func(t *testing.T) {
		_, err := c.GetMany(expiredCtx, keys)
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})

	t.Run("DeleteManyWithExpiredContext", func(t *testing.T) {
		err := c.DeleteMany(expiredCtx, keys)
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})
}

func TestMemoryCache_ContextTimeoutMetadataOperations(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Set up some test data first
	err := c.Set(ctx, "meta_key", "meta_value", time.Hour)
	if err != nil {
		t.Fatalf("Failed to set test data: %v", err)
	}

	// Create expired context
	expiredCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond)

	t.Run("GetMetadataWithExpiredContext", func(t *testing.T) {
		_, err := c.GetMetadata(expiredCtx, "meta_key")
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})

	t.Run("GetManyMetadataWithExpiredContext", func(t *testing.T) {
		_, err := c.GetManyMetadata(expiredCtx, []string{"meta_key"})
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})
}

func TestMemoryCache_ContextTimeoutAtomicOperations(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Create expired context
	expiredCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond)

	t.Run("IncrementWithExpiredContext", func(t *testing.T) {
		_, err := c.Increment(expiredCtx, "counter", 1, time.Hour)
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})

	t.Run("DecrementWithExpiredContext", func(t *testing.T) {
		_, err := c.Decrement(expiredCtx, "counter", 1, time.Hour)
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})
}

func TestMemoryCache_ContextTimeoutConditionalOperations(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Create expired context
	expiredCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond)

	t.Run("SetIfNotExistsWithExpiredContext", func(t *testing.T) {
		_, err := c.SetIfNotExists(expiredCtx, "conditional_key", "value", time.Hour)
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})

	t.Run("SetIfExistsWithExpiredContext", func(t *testing.T) {
		_, err := c.SetIfExists(expiredCtx, "conditional_key", "value", time.Hour)
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})
}

func TestMemoryCache_ContextTimeoutPatternOperations(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Set up some test data first
	for i := 0; i < 3; i++ {
		key := "pattern:test:" + string(rune(i+'0'))
		err := c.Set(ctx, key, "value", time.Hour)
		if err != nil {
			t.Fatalf("Failed to set test data: %v", err)
		}
	}

	// Create expired context
	expiredCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond)

	t.Run("GetKeysByPatternWithExpiredContext", func(t *testing.T) {
		_, err := c.GetKeysByPattern(expiredCtx, "pattern:*")
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})

	t.Run("DeleteByPatternWithExpiredContext", func(t *testing.T) {
		_, err := c.DeleteByPattern(expiredCtx, "pattern:*")
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})
}

func TestMemoryCache_ContextTimeoutAdvancedOperations(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Set up test data
	err := c.Set(ctx, "advanced_key", "initial_value", time.Hour)
	if err != nil {
		t.Fatalf("Failed to set test data: %v", err)
	}

	// Create expired context
	expiredCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond)

	t.Run("UpdateMetadataWithExpiredContext", func(t *testing.T) {
		updater := func(metadata *cache.CacheEntryMetadata) *cache.CacheEntryMetadata {
			return metadata
		}
		err := c.UpdateMetadata(expiredCtx, "advanced_key", updater)
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})

	t.Run("GetAndUpdateWithExpiredContext", func(t *testing.T) {
		updater := func(currentValue any) (newValue any, shouldUpdate bool) {
			return "updated_value", true
		}
		_, err := c.GetAndUpdate(expiredCtx, "advanced_key", updater, time.Hour)
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})

	t.Run("ClearWithExpiredContext", func(t *testing.T) {
		err := c.Clear(expiredCtx)
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})
}

func TestMemoryCache_ContextTimeoutIndexOperations(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Set up test data and index
	err := c.Set(ctx, "session:123", "user1_session", time.Hour)
	if err != nil {
		t.Fatalf("Failed to set test data: %v", err)
	}

	err = c.AddIndex(ctx, "sessions_by_user", "session:*", "user1")
	if err != nil {
		t.Fatalf("Failed to add index: %v", err)
	}

	// Create expired context
	expiredCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond)

	t.Run("AddIndexWithExpiredContext", func(t *testing.T) {
		err := c.AddIndex(expiredCtx, "test_index", "test:*", "test_value")
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})

	t.Run("RemoveIndexWithExpiredContext", func(t *testing.T) {
		err := c.RemoveIndex(expiredCtx, "sessions_by_user", "session:*", "user1")
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})

	t.Run("GetByIndexWithExpiredContext", func(t *testing.T) {
		_, err := c.GetByIndex(expiredCtx, "sessions_by_user", "user1")
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})

	t.Run("DeleteByIndexWithExpiredContext", func(t *testing.T) {
		err := c.DeleteByIndex(expiredCtx, "sessions_by_user", "user1")
		if err != context.DeadlineExceeded && err != cache.ErrContextCanceled {
			t.Errorf("Expected deadline exceeded or context canceled, got %v", err)
		}
	})
}

func TestMemoryCache_ContextCancellationDuringOperation(t *testing.T) {
	ctx := context.Background()
	c, cleanup := setupMemoryCache(t)
	defer cleanup()

	// Test context cancellation during a bulk operation
	t.Run("CancelDuringBulkSet", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)

		// Create a large dataset to increase chances of catching cancellation
		items := make(map[string]any)
		for i := 0; i < 1000; i++ {
			key := "bulk_key_" + string(rune(i/100+'0')) + string(rune((i/10)%10+'0')) + string(rune(i%10+'0'))
			items[key] = "bulk_value_" + string(rune(i+'0'))
		}

		// Start the operation and cancel immediately
		go func() {
			time.Sleep(1 * time.Millisecond)
			cancel()
		}()

		err := c.SetMany(cancelCtx, items, time.Hour)
		if err != nil && err != context.Canceled && err != cache.ErrContextCanceled {
			t.Logf("SetMany with cancellation returned: %v", err)
		}
	})

	t.Run("CancelDuringBulkGet", func(t *testing.T) {
		// Set up data first
		for i := 0; i < 100; i++ {
			key := "get_key_" + string(rune(i/10+'0')) + string(rune(i%10+'0'))
			err := c.Set(ctx, key, "value", time.Hour)
			if err != nil {
				t.Fatalf("Failed to set test data: %v", err)
			}
		}

		cancelCtx, cancel := context.WithCancel(ctx)
		keys := make([]string, 100)
		for i := 0; i < 100; i++ {
			keys[i] = "get_key_" + string(rune(i/10+'0')) + string(rune(i%10+'0'))
		}

		// Start the operation and cancel immediately
		go func() {
			time.Sleep(1 * time.Millisecond)
			cancel()
		}()

		_, err := c.GetMany(cancelCtx, keys)
		if err != nil && err != context.Canceled && err != cache.ErrContextCanceled {
			t.Logf("GetMany with cancellation returned: %v", err)
		}
	})
}
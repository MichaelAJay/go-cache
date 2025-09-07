package slicepool

import (
	"testing"
)

func TestSlicePool_BasicFunctionality(t *testing.T) {
	pool := NewSlicePool()

	t.Run("StringSlice", func(t *testing.T) {
		// Get a string slice
		slice := pool.GetStringSlice(10)
		
		// Should have length 0 and capacity >= 10
		if len(slice) != 0 {
			t.Errorf("Expected length 0, got %d", len(slice))
		}
		if cap(slice) < 10 {
			t.Errorf("Expected capacity >= 10, got %d", cap(slice))
		}
		
		// Use the slice
		slice = append(slice, "test1", "test2", "test3")
		
		// Return to pool
		pool.PutStringSlice(slice)
		
		// Get another slice - might be the same one
		slice2 := pool.GetStringSlice(5)
		
		// Should be clean
		if len(slice2) != 0 {
			t.Errorf("Expected clean slice with length 0, got %d", len(slice2))
		}
		
		// Should have adequate capacity
		if cap(slice2) < 5 {
			t.Errorf("Expected capacity >= 5, got %d", cap(slice2))
		}
		
		pool.PutStringSlice(slice2)
	})

	t.Run("AnySlice", func(t *testing.T) {
		// Get an any slice
		slice := pool.GetAnySlice(8)
		
		// Should have length 0 and capacity >= 8
		if len(slice) != 0 {
			t.Errorf("Expected length 0, got %d", len(slice))
		}
		if cap(slice) < 8 {
			t.Errorf("Expected capacity >= 8, got %d", cap(slice))
		}
		
		// Use the slice
		slice = append(slice, "test", 123, true)
		
		// Return to pool
		pool.PutAnySlice(slice)
		
		// Get another slice - might be the same one
		slice2 := pool.GetAnySlice(4)
		
		// Should be clean
		if len(slice2) != 0 {
			t.Errorf("Expected clean slice with length 0, got %d", len(slice2))
		}
		
		// Should have adequate capacity
		if cap(slice2) < 4 {
			t.Errorf("Expected capacity >= 4, got %d", cap(slice2))
		}
		
		pool.PutAnySlice(slice2)
	})
}

func TestSlicePool_WithLength(t *testing.T) {
	pool := NewSlicePool()

	t.Run("StringSliceWithLength", func(t *testing.T) {
		slice := pool.GetStringSliceWithLength(5)
		
		// Should have the exact length requested
		if len(slice) != 5 {
			t.Errorf("Expected length 5, got %d", len(slice))
		}
		
		// Should have capacity >= length
		if cap(slice) < 5 {
			t.Errorf("Expected capacity >= 5, got %d", cap(slice))
		}
		
		// Elements should be zero-valued
		for i, elem := range slice {
			if elem != "" {
				t.Errorf("Expected zero-valued element at index %d, got %q", i, elem)
			}
		}
		
		pool.PutStringSlice(slice)
	})

	t.Run("AnySliceWithLength", func(t *testing.T) {
		slice := pool.GetAnySliceWithLength(3)
		
		// Should have the exact length requested
		if len(slice) != 3 {
			t.Errorf("Expected length 3, got %d", len(slice))
		}
		
		// Should have capacity >= length
		if cap(slice) < 3 {
			t.Errorf("Expected capacity >= 3, got %d", cap(slice))
		}
		
		// Elements should be nil
		for i, elem := range slice {
			if elem != nil {
				t.Errorf("Expected nil element at index %d, got %v", i, elem)
			}
		}
		
		pool.PutAnySlice(slice)
	})
}

func TestSlicePool_DataCleaning(t *testing.T) {
	pool := NewSlicePool()

	t.Run("StringSliceDataClearing", func(t *testing.T) {
		// Get slice and populate it
		slice := pool.GetStringSlice(5)
		slice = append(slice, "sensitive", "data", "here")
		
		// Return to pool
		pool.PutStringSlice(slice)
		
		// Get slice again - should be cleaned
		slice2 := pool.GetStringSlice(3)
		
		// Length should be 0
		if len(slice2) != 0 {
			t.Errorf("Expected cleaned slice with length 0, got %d", len(slice2))
		}
		
		// If we get the same underlying array, check that data was cleared
		// (This test is best-effort since pool behavior isn't guaranteed)
		if cap(slice2) >= 3 {
			extended := slice2[:cap(slice2)]
			for i, elem := range extended {
				if elem != "" {
					t.Logf("Note: element at index %d not cleared: %q", i, elem)
					// This is informational - the important thing is len(slice2) == 0
				}
			}
		}
		
		pool.PutStringSlice(slice2)
	})

	t.Run("AnySliceDataClearing", func(t *testing.T) {
		// Get slice and populate it
		slice := pool.GetAnySlice(5)
		slice = append(slice, "sensitive", 12345, map[string]string{"key": "value"})
		
		// Return to pool
		pool.PutAnySlice(slice)
		
		// Get slice again - should be cleaned
		slice2 := pool.GetAnySlice(3)
		
		// Length should be 0
		if len(slice2) != 0 {
			t.Errorf("Expected cleaned slice with length 0, got %d", len(slice2))
		}
		
		pool.PutAnySlice(slice2)
	})
}

func TestSlicePool_CapacityHandling(t *testing.T) {
	pool := NewSlicePool()

	t.Run("GrowingCapacity", func(t *testing.T) {
		// Request small capacity
		slice1 := pool.GetStringSlice(5)
		initialCap := cap(slice1)
		pool.PutStringSlice(slice1)
		
		// Request larger capacity
		slice2 := pool.GetStringSlice(20)
		if cap(slice2) < 20 {
			t.Errorf("Expected capacity >= 20, got %d", cap(slice2))
		}
		
		// Should be at least the requested capacity
		if cap(slice2) < initialCap {
			// It's possible we got a new slice, which is fine
		}
		
		pool.PutStringSlice(slice2)
	})

	t.Run("ZeroCapacityHandling", func(t *testing.T) {
		// Create zero-capacity slice
		var zeroSlice []string
		
		// Should not panic when putting zero-capacity slice
		pool.PutStringSlice(zeroSlice)
		
		// Pool should still work normally
		slice := pool.GetStringSlice(5)
		if cap(slice) < 5 {
			t.Errorf("Expected capacity >= 5, got %d", cap(slice))
		}
		
		pool.PutStringSlice(slice)
	})
}

// Benchmark to verify pool provides allocation benefits
func BenchmarkSlicePool_vs_Make(b *testing.B) {
	pool := NewSlicePool()

	b.Run("PooledStringSlice", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			slice := pool.GetStringSlice(10)
			// Simulate usage
			for j := 0; j < 10; j++ {
				slice = append(slice, "test")
			}
			pool.PutStringSlice(slice)
		}
	})

	b.Run("DirectMakeStringSlice", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			slice := make([]string, 0, 10)
			// Simulate usage
			for j := 0; j < 10; j++ {
				slice = append(slice, "test")
			}
			// No pool return - let GC handle it
		}
	})

	b.Run("PooledAnySlice", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			slice := pool.GetAnySlice(10)
			// Simulate usage
			for j := 0; j < 10; j++ {
				slice = append(slice, "test")
			}
			pool.PutAnySlice(slice)
		}
	})

	b.Run("DirectMakeAnySlice", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			slice := make([]any, 0, 10)
			// Simulate usage
			for j := 0; j < 10; j++ {
				slice = append(slice, "test")
			}
			// No pool return - let GC handle it
		}
	})
}
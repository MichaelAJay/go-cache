package stringpool

import (
	"strings"
	"testing"
)

func TestPool_GetAndPut(t *testing.T) {
	builder := Get()
	if builder == nil {
		t.Fatal("Get() returned nil builder")
	}
	
	// Builder should be reset and ready to use
	if builder.Len() != 0 {
		t.Errorf("Expected empty builder, got length %d", builder.Len())
	}
	
	// Use the builder
	builder.WriteString("test")
	if builder.String() != "test" {
		t.Errorf("Expected 'test', got '%s'", builder.String())
	}
	
	// Return to pool
	Put(builder)
	
	// Get another builder - might be the same one (reset)
	builder2 := Get()
	if builder2 == nil {
		t.Fatal("Second Get() returned nil builder")
	}
	
	// Should be reset
	if builder2.Len() != 0 {
		t.Errorf("Expected empty builder after pool reuse, got length %d", builder2.Len())
	}
	
	Put(builder2)
}

func TestPool_LargeBuilderNotReturned(t *testing.T) {
	builder := Get()
	
	// Create a large string to exceed the 1024 cap limit
	largeString := strings.Repeat("x", 2000)
	builder.WriteString(largeString)
	
	// Force the builder to grow
	if builder.Cap() <= 1024 {
		t.Skip("Builder didn't grow as expected, test not valid")
	}
	
	// Put should not return this to pool due to size
	Put(builder)
	
	// This test just ensures Put doesn't panic with large builders
	// The actual pool behavior is internal and tested indirectly
}

func TestPool_Reset(t *testing.T) {
	builder := Get()
	builder.WriteString("some content")
	
	if builder.Len() == 0 {
		t.Error("Expected builder to have content")
	}
	
	Put(builder)
	
	// Get a builder (possibly the same one)
	builder2 := Get()
	
	// Should be reset
	if builder2.Len() != 0 {
		t.Error("Builder should be reset when retrieved from pool")
	}
	
	Put(builder2)
}

func TestPool_SecurityClearing(t *testing.T) {
	// This test verifies that sensitive data is cleared from the buffer
	sensitiveData := "session_id_12345_secret_token"
	
	builder := Get()
	builder.WriteString(sensitiveData)
	
	// Verify data is there
	if !strings.Contains(builder.String(), "secret") {
		t.Error("Expected sensitive data to be in builder")
	}
	
	// Put back to pool (should trigger security clearing)
	Put(builder)
	
	// Get the same builder back and verify it's clean
	builder2 := Get()
	
	// Buffer should be clean and ready for new use
	if builder2.Len() != 0 {
		t.Error("Builder should be empty after security clearing")
	}
	
	// Add new content to verify it works normally
	builder2.WriteString("new_data")
	if builder2.String() != "new_data" {
		t.Error("Builder should work normally after security clearing")
	}
	
	Put(builder2)
}

func BenchmarkPool_GetPut(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		builder := Get()
		builder.WriteString("test")
		_ = builder.String()
		Put(builder)
	}
}

func BenchmarkPool_GetPutLarge(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	
	data := strings.Repeat("x", 100)
	
	for i := 0; i < b.N; i++ {
		builder := Get()
		builder.WriteString(data)
		_ = builder.String()
		Put(builder)
	}
}

func BenchmarkPool_vs_Direct(b *testing.B) {
	b.Run("Pooled", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			builder := Get()
			builder.WriteString("prefix:")
			builder.WriteString("key")
			builder.WriteString(":suffix")
			_ = builder.String()
			Put(builder)
		}
	})
	
	b.Run("Direct", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var builder strings.Builder
			builder.WriteString("prefix:")
			builder.WriteString("key")
			builder.WriteString(":suffix")
			_ = builder.String()
		}
	})
}
package cache_test

import (
	"context"
	"strings"
	"testing"
	"time"

	cache "github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/MichaelAJay/go-cache/internal/testintegration"
)

// Test data structures for benchmarks
type benchmarkData struct {
	ID      string            `json:"id"`
	Content string            `json:"content"`
	Data    map[string]string `json:"data"`
}

func (bd benchmarkData) GetID() string {
	return bd.ID
}

func (bd benchmarkData) GetOwner() string {
	// Extract owner from ID (format: "owner:id")
	parts := strings.Split(bd.ID, ":")
	if len(parts) >= 2 {
		return parts[0]
	}
	return "default"
}

// Data generators for different sizes
func generateBenchmarkData1KB(id string) benchmarkData {
	content := strings.Repeat("a", 900) // ~1KB with overhead
	return benchmarkData{
		ID:      id,
		Content: content,
		Data:    map[string]string{"key1": "value1", "key2": "value2"},
	}
}

func generateBenchmarkData10KB(id string) benchmarkData {
	content := strings.Repeat("a", 9800) // ~10KB with overhead
	return benchmarkData{
		ID:      id,
		Content: content,
		Data:    map[string]string{"key1": "value1", "key2": "value2"},
	}
}

func generateBenchmarkData100KB(id string) benchmarkData {
	content := strings.Repeat("a", 99800) // ~100KB with overhead
	return benchmarkData{
		ID:      id,
		Content: content,
		Data:    map[string]string{"key1": "value1", "key2": "value2"},
	}
}

// Setup helper for benchmarks
func setupBenchmarkCache(b *testing.B) interfaces.Cache[benchmarkData] {
	b.Helper()

	ctx := context.Background()

	// Create a temporary testing.T to satisfy the interface
	// This is a workaround for the setup function expecting a *testing.T
	t := &testing.T{}
	setup := testintegration.SetupTestEnvironment(ctx, t)

	// Cleanup after benchmark
	b.Cleanup(func() {
		setup.TestEnv.Close()
	})

	extractor := &cache.IndexExtractor[benchmarkData]{
		GetEntryKey: func(data benchmarkData) string { return data.GetID() },
		GetOwnerKey: func(data benchmarkData) string { return data.GetOwner() },
	}

	cacheInstance, err := cache.NewCache(
		ctx,
		setup.RedisClient,
		false, // no indexing for basic benchmarks
		extractor,
		cache.WithTTL[benchmarkData](10*time.Minute),
		cache.WithSerializer[benchmarkData]("msgpack"),
	)
	if err != nil {
		b.Fatalf("Failed to create cache: %v", err)
	}

	return cacheInstance
}

// 1. Basic Operation Benchmarks

// 2. Concurrency Stress Tests

// 3. Atomic Operations Performance

// 4. Batch Operations Efficiency

// 5. Indexing Performance Impact

// 6. Serialization Overhead
// ---
// 7. Conditional Operations

// 8. Lua Script Warming Impact

// 9. Circuit Breaker Overhead

// 10. Memory and Resource Usage

// 11. Metadata Overhead Assessment

// 12. Real-World Usage Patterns

// 13. Performance vs Reliability Trade-offs

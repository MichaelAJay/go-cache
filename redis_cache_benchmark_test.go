package cache_test

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
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

// Global benchmark environment for container reuse
var (
	globalBenchmarkEnv   *testintegration.TestEnvironmentSetup
	globalBenchmarkMutex sync.Mutex
)

// getSharedBenchmarkEnvironment returns a shared test environment for all benchmarks
func getSharedBenchmarkEnvironment(b *testing.B) *testintegration.TestEnvironmentSetup {
	globalBenchmarkMutex.Lock()
	defer globalBenchmarkMutex.Unlock()
	
	if globalBenchmarkEnv == nil {
		ctx := context.Background()
		t := &testing.T{}
		globalBenchmarkEnv = testintegration.SetupTestEnvironment(ctx, t)
		
		// Setup cleanup to run when all benchmarks are done
		// Note: This is a simplification - in a real implementation you might want
		// more sophisticated lifecycle management
		b.Cleanup(func() {
			if globalBenchmarkEnv != nil {
				globalBenchmarkEnv.TestEnv.Close()
				globalBenchmarkEnv = nil
			}
		})
	}
	
	return globalBenchmarkEnv
}

// Setup helper for benchmarks with shared containers
func setupBenchmarkCache(b *testing.B) interfaces.Cache[benchmarkData] {
	b.Helper()

	ctx := context.Background()
	
	// Get shared environment (creates containers once)
	setup := getSharedBenchmarkEnvironment(b)
	
	// Reset environment for clean state
	latencyMs := getLatencyFromEnv()
	if err := setup.TestEnv.ResetForNewBenchmark(ctx, latencyMs); err != nil {
		b.Fatalf("Failed to reset benchmark environment: %v", err)
	}

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

// getLatencyFromEnv extracts latency setting from environment variables
func getLatencyFromEnv() int {
	if latencyStr := os.Getenv("GOCACHE_TEST_REDIS_LATENCY_MS"); latencyStr != "" {
		if latency, err := strconv.Atoi(latencyStr); err == nil {
			return latency
		}
	}
	return 0
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

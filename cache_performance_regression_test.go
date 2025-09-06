//go:build integration

package cache_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/internal/testintegration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PerformanceBaseline represents expected performance characteristics
type PerformanceBaseline struct {
	Operation       string                 `json:"operation"`
	DataSize        string                 `json:"dataSize"`
	ExpectedLatency time.Duration          `json:"expectedLatency"`
	TolerancePercent float64               `json:"tolerancePercent"`
	Metadata        map[string]interface{} `json:"metadata"`
	Timestamp       time.Time              `json:"timestamp"`
	Version         string                 `json:"version"`
}

// PerformanceResult captures actual performance measurements
type PerformanceResult struct {
	Operation        string        `json:"operation"`
	DataSize         string        `json:"dataSize"`
	ActualLatency    time.Duration `json:"actualLatency"`
	OperationsCount  int           `json:"operationsCount"`
	MinLatency       time.Duration `json:"minLatency"`
	MaxLatency       time.Duration `json:"maxLatency"`
	P50Latency       time.Duration `json:"p50Latency"`
	P95Latency       time.Duration `json:"p95Latency"`
	P99Latency       time.Duration `json:"p99Latency"`
	ThroughputPerSec float64       `json:"throughputPerSec"`
	Timestamp        time.Time     `json:"timestamp"`
}

// loadPerformanceBaselines loads expected performance baselines from file
func loadPerformanceBaselines(t *testing.T) map[string]PerformanceBaseline {
	baselines := make(map[string]PerformanceBaseline)
	
	// Try to load from file if it exists
	baselineFile := filepath.Join(".", "performance_baselines.json")
	if _, err := os.Stat(baselineFile); err == nil {
		data, err := os.ReadFile(baselineFile)
		if err == nil {
			var baselineSlice []PerformanceBaseline
			if json.Unmarshal(data, &baselineSlice) == nil {
				for _, baseline := range baselineSlice {
					key := fmt.Sprintf("%s_%s", baseline.Operation, baseline.DataSize)
					baselines[key] = baseline
				}
				t.Logf("Loaded %d performance baselines from file", len(baselines))
				return baselines
			}
		}
	}
	
	// Default baselines if file doesn't exist
	t.Log("Using default performance baselines")
	baselines["Set_small"] = PerformanceBaseline{
		Operation:       "Set",
		DataSize:        "small",
		ExpectedLatency: 2 * time.Millisecond,
		TolerancePercent: 50.0,
		Version:         "default",
		Timestamp:       time.Now(),
	}
	baselines["Get_small"] = PerformanceBaseline{
		Operation:       "Get",
		DataSize:        "small",
		ExpectedLatency: 1 * time.Millisecond,
		TolerancePercent: 50.0,
		Version:         "default",
		Timestamp:       time.Now(),
	}
	baselines["SetMany_medium"] = PerformanceBaseline{
		Operation:       "SetMany",
		DataSize:        "medium",
		ExpectedLatency: 10 * time.Millisecond,
		TolerancePercent: 50.0,
		Version:         "default",
		Timestamp:       time.Now(),
	}
	baselines["GetMany_medium"] = PerformanceBaseline{
		Operation:       "GetMany", 
		DataSize:        "medium",
		ExpectedLatency: 8 * time.Millisecond,
		TolerancePercent: 50.0,
		Version:         "default",
		Timestamp:       time.Now(),
	}
	
	return baselines
}

// savePerformanceResults saves performance results for future reference
func savePerformanceResults(t *testing.T, results []PerformanceResult) {
	resultsFile := filepath.Join(".", "performance_results.json")
	
	// Load existing results if they exist
	var existingResults []PerformanceResult
	if data, err := os.ReadFile(resultsFile); err == nil {
		json.Unmarshal(data, &existingResults)
	}
	
	// Append new results
	allResults := append(existingResults, results...)
	
	// Keep only recent results (last 100 test runs)
	if len(allResults) > 100 {
		allResults = allResults[len(allResults)-100:]
	}
	
	// Save to file
	if data, err := json.MarshalIndent(allResults, "", "  "); err == nil {
		os.WriteFile(resultsFile, data, 0644)
		t.Logf("Saved %d performance results to %s", len(results), resultsFile)
	}
}

// measureLatencies measures operation latencies and returns statistics
func measureLatencies(operation func() error, iterations int) ([]time.Duration, error) {
	latencies := make([]time.Duration, 0, iterations)
	
	for i := 0; i < iterations; i++ {
		start := time.Now()
		err := operation()
		duration := time.Since(start)
		
		if err != nil {
			return nil, fmt.Errorf("operation failed on iteration %d: %w", i, err)
		}
		
		latencies = append(latencies, duration)
	}
	
	return latencies, nil
}

// calculateLatencyStats calculates latency statistics from measurements
func calculateLatencyStats(latencies []time.Duration) PerformanceResult {
	if len(latencies) == 0 {
		return PerformanceResult{}
	}
	
	// Sort for percentile calculations
	sortedLatencies := make([]time.Duration, len(latencies))
	copy(sortedLatencies, latencies)
	sort.Slice(sortedLatencies, func(i, j int) bool {
		return sortedLatencies[i] < sortedLatencies[j]
	})
	
	// Calculate statistics
	var total time.Duration
	min := sortedLatencies[0]
	max := sortedLatencies[len(sortedLatencies)-1]
	
	for _, lat := range latencies {
		total += lat
	}
	
	avg := total / time.Duration(len(latencies))
	p50 := sortedLatencies[len(sortedLatencies)*50/100]
	p95 := sortedLatencies[len(sortedLatencies)*95/100]
	p99 := sortedLatencies[len(sortedLatencies)*99/100]
	
	throughput := float64(len(latencies)) / total.Seconds()
	
	return PerformanceResult{
		ActualLatency:    avg,
		OperationsCount:  len(latencies),
		MinLatency:       min,
		MaxLatency:       max,
		P50Latency:       p50,
		P95Latency:       p95,
		P99Latency:       p99,
		ThroughputPerSec: throughput,
		Timestamp:        time.Now(),
	}
}

// TestBasicOperationPerformanceRegression tests performance of basic cache operations
func TestBasicOperationPerformanceRegression(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	t.Log("🧪 Testing basic operation performance regression")

	baselines := loadPerformanceBaselines(t)
	var results []PerformanceResult
	
	// Test data
	smallSession := &testintegration.TestSession{
		ID:       "perf_test_small",
		UserID:   "perf_user",
		Username: "perf_username",
		Created:  time.Now(),
	}

	// Test Set operation performance
	t.Log("Measuring Set operation performance...")
	const setIterations = 100
	
	setLatencies, err := measureLatencies(func() error {
		smallSession.ID = fmt.Sprintf("perf_test_set_%d", time.Now().UnixNano())
		return cache.Set(ctx, smallSession, time.Hour)
	}, setIterations)
	require.NoError(t, err, "Set performance measurement should succeed")
	
	setStats := calculateLatencyStats(setLatencies)
	setStats.Operation = "Set"
	setStats.DataSize = "small"
	results = append(results, setStats)
	
	t.Logf("Set performance: avg=%v, p50=%v, p95=%v, p99=%v, throughput=%.1f ops/sec",
		setStats.ActualLatency, setStats.P50Latency, setStats.P95Latency, 
		setStats.P99Latency, setStats.ThroughputPerSec)

	// Test Get operation performance
	t.Log("Measuring Get operation performance...")
	const getIterations = 200
	
	// Pre-populate with test data
	testKeys := make([]string, getIterations)
	for i := 0; i < getIterations; i++ {
		key := fmt.Sprintf("perf_test_get_%d", i)
		session := &testintegration.TestSession{
			ID:       key,
			UserID:   fmt.Sprintf("perf_user_%d", i),
			Username: fmt.Sprintf("perf_username_%d", i),
			Created:  time.Now(),
		}
		cache.Set(ctx, session, time.Hour)
		testKeys[i] = key
	}
	
	keyIndex := 0
	getLatencies, err := measureLatencies(func() error {
		key := testKeys[keyIndex%len(testKeys)]
		keyIndex++
		_, _, err := cache.Get(ctx, key)
		return err
	}, getIterations)
	require.NoError(t, err, "Get performance measurement should succeed")
	
	getStats := calculateLatencyStats(getLatencies)
	getStats.Operation = "Get"
	getStats.DataSize = "small"
	results = append(results, getStats)
	
	t.Logf("Get performance: avg=%v, p50=%v, p95=%v, p99=%v, throughput=%.1f ops/sec",
		getStats.ActualLatency, getStats.P50Latency, getStats.P95Latency, 
		getStats.P99Latency, getStats.ThroughputPerSec)

	// Compare against baselines
	checkPerformanceRegression := func(result PerformanceResult) {
		baselineKey := fmt.Sprintf("%s_%s", result.Operation, result.DataSize)
		if baseline, exists := baselines[baselineKey]; exists {
			tolerance := baseline.ExpectedLatency * time.Duration(baseline.TolerancePercent) / 100
			maxAcceptable := baseline.ExpectedLatency + tolerance
			
			t.Logf("Performance check for %s: baseline=%v, actual=%v, max_acceptable=%v",
				baselineKey, baseline.ExpectedLatency, result.ActualLatency, maxAcceptable)
				
			assert.Less(t, result.ActualLatency, maxAcceptable, 
				"Performance regression detected for %s: actual %v > acceptable %v", 
				baselineKey, result.ActualLatency, maxAcceptable)
		} else {
			t.Logf("No baseline found for %s, current performance: %v", baselineKey, result.ActualLatency)
		}
	}
	
	checkPerformanceRegression(setStats)
	checkPerformanceRegression(getStats)

	// Clean up test data
	cache.DeleteMany(ctx, testKeys)
	
	// Save results
	savePerformanceResults(t, results)
	
	t.Log("✅ Basic operation performance regression test completed")
}

// TestBatchOperationPerformanceRegression tests performance of batch operations
func TestBatchOperationPerformanceRegression(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	t.Log("🧪 Testing batch operation performance regression")

	baselines := loadPerformanceBaselines(t)
	var results []PerformanceResult

	// Test SetMany performance
	t.Log("Measuring SetMany operation performance...")
	const batchSize = 50
	const batchIterations = 20
	
	setManyLatencies, err := measureLatencies(func() error {
		sessions := make([]*testintegration.TestSession, batchSize)
		for i := 0; i < batchSize; i++ {
			sessions[i] = &testintegration.TestSession{
				ID:       fmt.Sprintf("perf_batch_set_%d_%d", time.Now().UnixNano(), i),
				UserID:   fmt.Sprintf("perf_batch_user_%d", i),
				Username: fmt.Sprintf("perf_batch_username_%d", i),
				Created:  time.Now(),
			}
		}
		return cache.SetMany(ctx, sessions, time.Hour)
	}, batchIterations)
	require.NoError(t, err, "SetMany performance measurement should succeed")
	
	setManyStats := calculateLatencyStats(setManyLatencies)
	setManyStats.Operation = "SetMany"
	setManyStats.DataSize = "medium"
	results = append(results, setManyStats)
	
	t.Logf("SetMany performance (batch size %d): avg=%v, p50=%v, p95=%v, throughput=%.1f batches/sec",
		batchSize, setManyStats.ActualLatency, setManyStats.P50Latency, 
		setManyStats.P95Latency, setManyStats.ThroughputPerSec)

	// Test GetMany performance
	t.Log("Measuring GetMany operation performance...")
	
	// Pre-populate data for GetMany tests
	allKeys := make([]string, batchSize*batchIterations)
	for batch := 0; batch < batchIterations; batch++ {
		sessions := make([]*testintegration.TestSession, batchSize)
		for i := 0; i < batchSize; i++ {
			key := fmt.Sprintf("perf_batch_get_%d_%d", batch, i)
			sessions[i] = &testintegration.TestSession{
				ID:       key,
				UserID:   fmt.Sprintf("perf_get_user_%d_%d", batch, i),
				Username: fmt.Sprintf("perf_get_username_%d_%d", batch, i),
				Created:  time.Now(),
			}
			allKeys[batch*batchSize+i] = key
		}
		cache.SetMany(ctx, sessions, time.Hour)
	}
	
	batchIndex := 0
	getManyLatencies, err := measureLatencies(func() error {
		startIdx := (batchIndex * batchSize) % len(allKeys)
		endIdx := startIdx + batchSize
		if endIdx > len(allKeys) {
			endIdx = len(allKeys)
		}
		keys := allKeys[startIdx:endIdx]
		batchIndex++
		
		_, err := cache.GetMany(ctx, keys)
		return err
	}, batchIterations)
	require.NoError(t, err, "GetMany performance measurement should succeed")
	
	getManyStats := calculateLatencyStats(getManyLatencies)
	getManyStats.Operation = "GetMany"
	getManyStats.DataSize = "medium"
	results = append(results, getManyStats)
	
	t.Logf("GetMany performance (batch size %d): avg=%v, p50=%v, p95=%v, throughput=%.1f batches/sec",
		batchSize, getManyStats.ActualLatency, getManyStats.P50Latency, 
		getManyStats.P95Latency, getManyStats.ThroughputPerSec)

	// Compare against baselines
	for _, result := range results {
		baselineKey := fmt.Sprintf("%s_%s", result.Operation, result.DataSize)
		if baseline, exists := baselines[baselineKey]; exists {
			tolerance := baseline.ExpectedLatency * time.Duration(baseline.TolerancePercent) / 100
			maxAcceptable := baseline.ExpectedLatency + tolerance
			
			t.Logf("Performance check for %s: baseline=%v, actual=%v, max_acceptable=%v",
				baselineKey, baseline.ExpectedLatency, result.ActualLatency, maxAcceptable)
				
			assert.Less(t, result.ActualLatency, maxAcceptable,
				"Performance regression detected for %s: actual %v > acceptable %v",
				baselineKey, result.ActualLatency, maxAcceptable)
		} else {
			t.Logf("No baseline found for %s, current performance: %v", baselineKey, result.ActualLatency)
		}
	}

	// Clean up test data
	cache.DeleteMany(ctx, allKeys)
	
	// Save results
	savePerformanceResults(t, results)
	
	t.Log("✅ Batch operation performance regression test completed")
}

// TestScalabilityPerformanceRegression tests performance across different data scales
func TestScalabilityPerformanceRegression(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	t.Log("🧪 Testing scalability performance regression")

	var results []PerformanceResult
	
	// Test performance with different dataset sizes
	testCases := []struct {
		name        string
		datasetSize int
		batchSize   int
		iterations  int
	}{
		{"small_dataset", 100, 10, 10},
		{"medium_dataset", 1000, 50, 8},
		{"large_dataset", 5000, 100, 5},
	}

	for _, tc := range testCases {
		t.Logf("Testing %s: %d total entries, batch size %d", tc.name, tc.datasetSize, tc.batchSize)
		
		// Pre-populate dataset
		allSessions := make([]*testintegration.TestSession, tc.datasetSize)
		allKeys := make([]string, tc.datasetSize)
		
		for i := 0; i < tc.datasetSize; i++ {
			key := fmt.Sprintf("scalability_%s_session_%d", tc.name, i)
			allSessions[i] = &testintegration.TestSession{
				ID:       key,
				UserID:   fmt.Sprintf("scalability_user_%d", i),
				Username: fmt.Sprintf("scalability_username_%d_with_some_additional_data", i),
				Created:  time.Now(),
			}
			allKeys[i] = key
		}
		
		// Populate in batches
		for i := 0; i < len(allSessions); i += tc.batchSize {
			end := i + tc.batchSize
			if end > len(allSessions) {
				end = len(allSessions)
			}
			cache.SetMany(ctx, allSessions[i:end], time.Hour)
		}
		
		// Measure GetMany performance at this scale
		batchIndex := 0
		getManyLatencies, err := measureLatencies(func() error {
			startIdx := (batchIndex * tc.batchSize) % len(allKeys)
			endIdx := startIdx + tc.batchSize
			if endIdx > len(allKeys) {
				endIdx = len(allKeys)
			}
			keys := allKeys[startIdx:endIdx]
			batchIndex++
			
			_, err := cache.GetMany(ctx, keys)
			return err
		}, tc.iterations)
		
		if err != nil {
			t.Errorf("Performance measurement failed for %s: %v", tc.name, err)
			continue
		}
		
		stats := calculateLatencyStats(getManyLatencies)
		stats.Operation = "GetMany"
		stats.DataSize = tc.name
		results = append(results, stats)
		
		t.Logf("%s GetMany performance: avg=%v, p95=%v, throughput=%.1f batches/sec",
			tc.name, stats.ActualLatency, stats.P95Latency, stats.ThroughputPerSec)
		
		// Verify performance doesn't degrade significantly with scale
		// (This is a basic check - in production you'd have more sophisticated regression detection)
		maxAcceptableLatency := 100 * time.Millisecond
		assert.Less(t, stats.P95Latency, maxAcceptableLatency,
			"Performance degradation detected for %s: p95 latency %v > %v",
			tc.name, stats.P95Latency, maxAcceptableLatency)
		
		// Clean up dataset
		for i := 0; i < len(allKeys); i += tc.batchSize {
			end := i + tc.batchSize
			if end > len(allKeys) {
				end = len(allKeys)
			}
			cache.DeleteMany(ctx, allKeys[i:end])
		}
	}

	// Save results
	savePerformanceResults(t, results)
	
	t.Log("✅ Scalability performance regression test completed")
}
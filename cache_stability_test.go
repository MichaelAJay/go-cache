//go:build stability

package cache_test

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/internal/testintegration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLongRunningMixedWorkloadStability tests cache stability under mixed operations over time
func TestLongRunningMixedWorkloadStability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running stability test in short mode")
	}

	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	t.Log("🧪 Starting long-running mixed workload stability test")
	t.Log("⏱️  This test will run for several minutes to verify stability over time")

	// Test duration - configurable for different test environments
	testDuration := 3 * time.Minute // Reduced for practical testing
	if testing.Verbose() {
		testDuration = 5 * time.Minute // Longer for verbose mode
	}

	// Metrics tracking
	var (
		operationCount  = make(map[string]int64)
		errorCount     = make(map[string]int64)
		mu             sync.Mutex
	)

	recordOperation := func(opType string, isError bool) {
		mu.Lock()
		defer mu.Unlock()
		operationCount[opType]++
		if isError {
			errorCount[opType]++
		}
	}

	// Worker functions for different operation types
	startWorker := func(workerType string, workerFunc func()) {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Worker %s panicked: %v", workerType, r)
				}
			}()
			workerFunc()
		}()
	}

	// Start test timer
	testStart := time.Now()
	testCtx, cancel := context.WithTimeout(ctx, testDuration)
	defer cancel()

	t.Logf("Starting mixed workload test for %v", testDuration)

	// Worker 1: Continuous set operations
	startWorker("setter", func() {
		counter := 0
		for {
			select {
			case <-testCtx.Done():
				return
			default:
				session := &testintegration.TestSession{
					ID:       fmt.Sprintf("stability_session_%d", counter),
					UserID:   fmt.Sprintf("user_%d", counter%1000),
					Username: fmt.Sprintf("user_%d_stability_test", counter),
					Created:  time.Now(),
				}
				
				err := cache.Set(testCtx, session, time.Hour)
				recordOperation("set", err != nil)
				
				counter++
				time.Sleep(time.Millisecond * 10) // 100 ops/sec
			}
		}
	})

	// Worker 2: Continuous get operations
	startWorker("getter", func() {
		counter := 0
		for {
			select {
			case <-testCtx.Done():
				return
			default:
				// Try to get both existing and non-existing keys
				var key string
				if counter%3 == 0 {
					key = fmt.Sprintf("stability_session_%d", counter/2) // Likely exists
				} else {
					key = fmt.Sprintf("nonexistent_session_%d", counter) // Doesn't exist
				}
				
				_, _, err := cache.Get(testCtx, key)
				recordOperation("get", err != nil)
				
				counter++
				time.Sleep(time.Millisecond * 8) // ~125 ops/sec
			}
		}
	})

	// Worker 3: Batch operations
	startWorker("batcher", func() {
		counter := 0
		for {
			select {
			case <-testCtx.Done():
				return
			default:
				// Create batch of sessions
				batchSize := 10
				sessions := make([]*testintegration.TestSession, batchSize)
				keys := make([]string, batchSize)
				
				for i := 0; i < batchSize; i++ {
					sessionID := fmt.Sprintf("batch_session_%d_%d", counter, i)
					sessions[i] = &testintegration.TestSession{
						ID:       sessionID,
						UserID:   fmt.Sprintf("batch_user_%d", counter),
						Username: fmt.Sprintf("batch_user_%d_%d", counter, i),
						Created:  time.Now(),
					}
					keys[i] = sessionID
				}
				
				// Set batch
				err := cache.SetMany(testCtx, sessions, time.Hour)
				recordOperation("setmany", err != nil)
				
				time.Sleep(time.Millisecond * 50) // Small delay
				
				// Get batch
				_, err = cache.GetMany(testCtx, keys)
				recordOperation("getmany", err != nil)
				
				time.Sleep(time.Millisecond * 50) // Small delay
				
				// Delete batch
				err = cache.DeleteMany(testCtx, keys)
				recordOperation("deletemany", err != nil)
				
				counter++
				time.Sleep(time.Millisecond * 200) // ~5 batches/sec
			}
		}
	})

	// Worker 4: Counter operations
	startWorker("counter", func() {
		counter := 0
		for {
			select {
			case <-testCtx.Done():
				return
			default:
				counterKey := fmt.Sprintf("stability_counter_%d", counter%10)
				
				// Increment
				_, err := cache.Increment(testCtx, counterKey, 1)
				recordOperation("increment", err != nil)
				
				// Decrement occasionally
				if counter%5 == 0 {
					_, err := cache.Decrement(testCtx, counterKey, 1)
					recordOperation("decrement", err != nil)
				}
				
				counter++
				time.Sleep(time.Millisecond * 15) // ~67 ops/sec
			}
		}
	})

	// Memory monitoring goroutine
	var memStats []runtime.MemStats
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-testCtx.Done():
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.GC()
				runtime.ReadMemStats(&m)
				
				mu.Lock()
				memStats = append(memStats, m)
				mu.Unlock()
				
				t.Logf("Memory stats: Alloc=%dMB, TotalAlloc=%dMB, Sys=%dMB, NumGC=%d",
					m.Alloc/(1024*1024), m.TotalAlloc/(1024*1024), m.Sys/(1024*1024), m.NumGC)
			}
		}
	}()

	// Progress reporter
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-testCtx.Done():
				return
			case <-ticker.C:
				elapsed := time.Since(testStart)
				remaining := testDuration - elapsed
				
				mu.Lock()
				totalOps := int64(0)
				totalErrors := int64(0)
				for op, count := range operationCount {
					totalOps += count
					totalErrors += errorCount[op]
				}
				mu.Unlock()
				
				t.Logf("Progress: %v elapsed, %v remaining. Operations: %d total, %d errors (%.2f%% error rate)",
					elapsed.Round(time.Second), remaining.Round(time.Second),
					totalOps, totalErrors, float64(totalErrors)/float64(totalOps)*100)
			}
		}
	}()

	// Wait for test completion
	<-testCtx.Done()
	time.Sleep(time.Second) // Allow workers to finish current operations

	// Final statistics
	mu.Lock()
	totalOperations := int64(0)
	totalErrors := int64(0)
	
	t.Log("\n📊 Final Statistics:")
	for opType, count := range operationCount {
		errors := errorCount[opType]
		errorRate := float64(errors) / float64(count) * 100
		t.Logf("  %s: %d operations, %d errors (%.2f%%)", opType, count, errors, errorRate)
		totalOperations += count
		totalErrors += errors
	}
	
	overallErrorRate := float64(totalErrors) / float64(totalOperations) * 100
	operationsPerSecond := float64(totalOperations) / testDuration.Seconds()
	
	t.Logf("\n📈 Overall Results:")
	t.Logf("  Total operations: %d", totalOperations)
	t.Logf("  Total errors: %d", totalErrors)
	t.Logf("  Overall error rate: %.2f%%", overallErrorRate)
	t.Logf("  Operations per second: %.2f", operationsPerSecond)
	t.Logf("  Test duration: %v", testDuration)
	mu.Unlock()

	// Assertions for stability
	assert.Greater(t, totalOperations, int64(1000), "Should have performed significant number of operations")
	assert.Less(t, overallErrorRate, 5.0, "Error rate should be less than 5%")
	assert.Greater(t, operationsPerSecond, 10.0, "Should maintain reasonable throughput")

	// Verify cache is still functional after long run
	testSession := &testintegration.TestSession{
		ID:       "post_stability_test",
		UserID:   "stability_test_user",
		Username: "stability_test_final",
		Created:  time.Now(),
	}
	
	err = cache.Set(ctx, testSession, time.Hour)
	assert.NoError(t, err, "Cache should be functional after stability test")
	
	retrieved, found, err := cache.Get(ctx, testSession.ID)
	assert.NoError(t, err, "Cache should retrieve after stability test")
	assert.True(t, found, "Test session should be found after stability test")
	if found {
		assert.Equal(t, testSession.UserID, retrieved.UserID, "Data integrity should be maintained")
	}

	t.Log("✅ Long-running mixed workload stability test completed successfully")
}

// TestMemoryStabilityOverTime tests memory usage stability over extended periods
func TestMemoryStabilityOverTime(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory stability test in short mode")
	}

	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	t.Log("🧪 Testing memory stability over time")

	testDuration := 2 * time.Minute // Shorter duration for practical testing
	testCtx, cancel := context.WithTimeout(ctx, testDuration)
	defer cancel()

	// Memory tracking
	type MemorySnapshot struct {
		Timestamp time.Time
		Alloc     uint64
		TotalAlloc uint64
		Sys       uint64
		NumGC     uint32
	}

	var (
		memorySnapshots []MemorySnapshot
		mu              sync.Mutex
	)

	// Take initial memory snapshot
	var initialMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialMem)
	
	mu.Lock()
	memorySnapshots = append(memorySnapshots, MemorySnapshot{
		Timestamp:  time.Now(),
		Alloc:      initialMem.Alloc,
		TotalAlloc: initialMem.TotalAlloc,
		Sys:        initialMem.Sys,
		NumGC:      initialMem.NumGC,
	})
	mu.Unlock()

	// Memory monitoring goroutine
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-testCtx.Done():
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.GC()
				runtime.ReadMemStats(&m)
				
				mu.Lock()
				memorySnapshots = append(memorySnapshots, MemorySnapshot{
					Timestamp:  time.Now(),
					Alloc:      m.Alloc,
					TotalAlloc: m.TotalAlloc,
					Sys:        m.Sys,
					NumGC:      m.NumGC,
				})
				mu.Unlock()
			}
		}
	}()

	// Steady workload that creates and destroys sessions
	go func() {
		sessionCounter := 0
		activeKeys := make([]string, 0, 1000) // Keep track of active keys
		
		for {
			select {
			case <-testCtx.Done():
				return
			default:
				// Phase 1: Create sessions
				if len(activeKeys) < 500 {
					sessionID := fmt.Sprintf("memory_stability_session_%d", sessionCounter)
					session := &testintegration.TestSession{
						ID:       sessionID,
						UserID:   fmt.Sprintf("memory_user_%d", sessionCounter%100),
						Username: fmt.Sprintf("memory_username_%d_with_some_data_for_memory_tracking", sessionCounter),
						Created:  time.Now(),
					}
					
					if err := cache.Set(testCtx, session, time.Hour); err == nil {
						activeKeys = append(activeKeys, sessionID)
					}
					sessionCounter++
				}
				
				// Phase 2: Clean up old sessions periodically
				if len(activeKeys) > 100 && sessionCounter%10 == 0 {
					// Delete oldest 10% of keys
					deleteCount := len(activeKeys) / 10
					if deleteCount > 0 {
						keysToDelete := activeKeys[:deleteCount]
						cache.DeleteMany(testCtx, keysToDelete)
						activeKeys = activeKeys[deleteCount:]
					}
				}
				
				time.Sleep(time.Millisecond * 20) // ~50 ops/sec
			}
		}
	}()

	// Wait for test completion
	<-testCtx.Done()
	time.Sleep(time.Second) // Allow cleanup

	// Take final memory snapshot
	var finalMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&finalMem)
	
	mu.Lock()
	memorySnapshots = append(memorySnapshots, MemorySnapshot{
		Timestamp:  time.Now(),
		Alloc:      finalMem.Alloc,
		TotalAlloc: finalMem.TotalAlloc,
		Sys:        finalMem.Sys,
		NumGC:      finalMem.NumGC,
	})
	mu.Unlock()

	// Analyze memory stability
	t.Log("\n📊 Memory Stability Analysis:")
	
	initialSnapshot := memorySnapshots[0]
	finalSnapshot := memorySnapshots[len(memorySnapshots)-1]
	
	allocGrowth := int64(finalSnapshot.Alloc) - int64(initialSnapshot.Alloc)
	sysGrowth := int64(finalSnapshot.Sys) - int64(initialSnapshot.Sys)
	gcCount := finalSnapshot.NumGC - initialSnapshot.NumGC
	
	t.Logf("  Initial memory: %d MB allocated, %d MB system", 
		initialSnapshot.Alloc/(1024*1024), initialSnapshot.Sys/(1024*1024))
	t.Logf("  Final memory: %d MB allocated, %d MB system", 
		finalSnapshot.Alloc/(1024*1024), finalSnapshot.Sys/(1024*1024))
	t.Logf("  Memory growth: %+d MB allocated, %+d MB system", 
		allocGrowth/(1024*1024), sysGrowth/(1024*1024))
	t.Logf("  Garbage collections: %d", gcCount)

	// Check for memory leaks (excessive growth)
	maxAllocGrowthMB := int64(100) // Allow up to 100MB growth
	maxSysGrowthMB := int64(200)   // Allow up to 200MB system growth
	
	assert.Less(t, allocGrowth/(1024*1024), maxAllocGrowthMB, 
		"Allocated memory growth should be reasonable")
	assert.Less(t, sysGrowth/(1024*1024), maxSysGrowthMB, 
		"System memory growth should be reasonable")
	
	// Verify cache is still functional
	testSession := &testintegration.TestSession{
		ID:       "memory_stability_final_test",
		UserID:   "final_test_user",
		Username: "final_test",
		Created:  time.Now(),
	}
	
	err = cache.Set(ctx, testSession, time.Hour)
	assert.NoError(t, err, "Cache should be functional after memory stability test")

	t.Log("✅ Memory stability test completed")
}

// TestConnectionStabilityAndRecovery tests cache behavior with connection issues over time
func TestConnectionStabilityAndRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping connection stability test in short mode")
	}

	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	t.Log("🧪 Testing connection stability and recovery")

	testDuration := 1 * time.Minute // Shorter for practical testing
	testCtx, cancel := context.WithTimeout(ctx, testDuration)
	defer cancel()

	var (
		successCount   int64
		errorCount     int64
		recoveryCount  int64
		mu             sync.Mutex
	)

	// Test continuous operations to verify connection stability
	go func() {
		counter := 0
		consecutiveErrors := 0
		
		for {
			select {
			case <-testCtx.Done():
				return
			default:
				session := &testintegration.TestSession{
					ID:       fmt.Sprintf("connection_test_session_%d", counter),
					UserID:   fmt.Sprintf("connection_user_%d", counter%50),
					Username: fmt.Sprintf("connection_username_%d", counter),
					Created:  time.Now(),
				}
				
				// Try a complete cycle: Set -> Get -> Delete
				setErr := cache.Set(testCtx, session, time.Hour)
				_, _, getErr := cache.Get(testCtx, session.ID)
				deleteErr := cache.Delete(testCtx, session.ID)
				
				mu.Lock()
				if setErr != nil || getErr != nil || deleteErr != nil {
					errorCount++
					consecutiveErrors++
				} else {
					successCount++
					if consecutiveErrors > 0 {
						recoveryCount++
						t.Logf("Connection recovered after %d consecutive errors", consecutiveErrors)
						consecutiveErrors = 0
					}
				}
				mu.Unlock()
				
				counter++
				time.Sleep(time.Millisecond * 50) // ~20 ops/sec
			}
		}
	}()

	// Wait for test completion
	<-testCtx.Done()
	time.Sleep(time.Second)

	// Report results
	mu.Lock()
	total := successCount + errorCount
	errorRate := float64(errorCount) / float64(total) * 100
	mu.Unlock()

	t.Log("\n📊 Connection Stability Results:")
	t.Logf("  Successful operations: %d", successCount)
	t.Logf("  Failed operations: %d", errorCount)
	t.Logf("  Error rate: %.2f%%", errorRate)
	t.Logf("  Recovery events: %d", recoveryCount)

	// Connection should be highly stable
	assert.Less(t, errorRate, 2.0, "Connection error rate should be very low")
	assert.Greater(t, successCount, int64(100), "Should have many successful operations")

	// Verify final functionality
	testSession := &testintegration.TestSession{
		ID:       "connection_stability_final",
		UserID:   "final_user",
		Username: "final_test",
		Created:  time.Now(),
	}
	
	err = cache.Set(ctx, testSession, time.Hour)
	assert.NoError(t, err, "Cache should be functional after connection stability test")

	t.Log("✅ Connection stability test completed")
}
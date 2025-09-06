package cache

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
)

// memoryTrackerConfig holds configuration for memory tracking
type memoryTrackerConfig struct {
	samplingRate     int           // operations between Redis MEMORY USAGE samples
	samplingInterval time.Duration // background sampling interval
	thresholdBytes   int64         // absolute memory threshold (0 = disabled)
	thresholdPercent float64       // percentage memory threshold
}

// memoryTracker tracks cache memory usage with hybrid approach:
// - Incremental tracking for performance
// - Periodic Redis MEMORY USAGE corrections for accuracy
type memoryTracker struct {
	estimatedMemoryBytes int64     // atomic counter for estimated memory usage
	totalEntries         int64     // atomic counter for total entries
	operationCount       int64     // atomic counter for sampling rate tracking
	lastSampleTime       time.Time // last time Redis MEMORY USAGE was called
	redisMaxMemory       int64     // cached Redis maxmemory setting (atomic)

	client redis.Cmdable        // Redis client reference
	config memoryTrackerConfig  // configuration
	mu     sync.RWMutex          // coordination mutex
}

// NewMemoryTracker creates a new memory tracker with the given configuration
func NewMemoryTracker(client redis.Cmdable, config memoryTrackerConfig) *memoryTracker {
	return &memoryTracker{
		client:         client,
		config:         config,
		lastSampleTime: time.Now(),
	}
}

// RecordSet increments memory usage estimate after a successful set operation
func (mt *memoryTracker) RecordSet(key string, serializedData []byte) {
	// Estimate memory usage: key + value + Redis overhead
	// Redis overhead includes hash table entry, expiration data, etc.
	keySize := int64(len(key))
	valueSize := int64(len(serializedData))
	redisOverhead := int64(64) // Conservative estimate for Redis metadata per key
	
	estimatedSize := keySize + valueSize + redisOverhead
	
	atomic.AddInt64(&mt.estimatedMemoryBytes, estimatedSize)
	atomic.AddInt64(&mt.totalEntries, 1)
	atomic.AddInt64(&mt.operationCount, 1)
}

// RecordDelete decrements memory usage estimate after a successful delete operation
// Note: This may need Redis lookup for accurate size if we don't store sizes
func (mt *memoryTracker) RecordDelete(key string) {
	// For now, use average entry size estimation
	// In a more sophisticated implementation, we could store entry sizes in metadata
	currentEntries := atomic.LoadInt64(&mt.totalEntries)
	if currentEntries <= 0 {
		return // No entries to delete
	}
	
	currentMemory := atomic.LoadInt64(&mt.estimatedMemoryBytes)
	avgEntrySize := currentMemory / currentEntries
	
	// Decrement by average size (conservative approach)
	atomic.AddInt64(&mt.estimatedMemoryBytes, -avgEntrySize)
	atomic.AddInt64(&mt.totalEntries, -1)
	atomic.AddInt64(&mt.operationCount, 1)
}

// ShouldSample determines if memory sampling should occur based on rate and time
func (mt *memoryTracker) ShouldSample() bool {
	// Check operation-based sampling rate
	opCount := atomic.LoadInt64(&mt.operationCount)
	rateSample := opCount > 0 && opCount%int64(mt.config.samplingRate) == 0
	
	// Check time-based sampling interval
	mt.mu.RLock()
	timeSample := time.Since(mt.lastSampleTime) >= mt.config.samplingInterval
	mt.mu.RUnlock()
	
	return rateSample || timeSample
}

// PerformMemorySample executes Redis MEMORY USAGE command to correct estimates
func (mt *memoryTracker) PerformMemorySample(ctx context.Context, dataPrefix string) error {
	// Use Redis MEMORY USAGE command to get accurate memory consumption
	// This command returns the memory usage of keys matching the pattern
	
	// For keys with a prefix, we can use MEMORY USAGE with pattern matching
	// However, MEMORY USAGE doesn't support patterns directly, so we need to:
	// 1. Get all keys with the prefix using SCAN
	// 2. Sum up MEMORY USAGE for each key
	
	var totalMemory int64
	var keyCount int64
	
	// Use SCAN to iterate through keys with the prefix
	iter := mt.client.Scan(ctx, 0, dataPrefix+"*", 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		
		// Get memory usage for this specific key
		memUsage := mt.client.MemoryUsage(ctx, key)
		if memUsage.Err() != nil {
			// If MEMORY USAGE fails for a key, skip it but don't fail entirely
			continue
		}
		
		totalMemory += memUsage.Val()
		keyCount++
	}
	
	if iter.Err() != nil {
		// SCAN operation failed, don't update estimates
		return iter.Err()
	}
	
	// Update our estimates with the actual Redis measurements
	atomic.StoreInt64(&mt.estimatedMemoryBytes, totalMemory)
	atomic.StoreInt64(&mt.totalEntries, keyCount)
	
	// Update last sample time and reset operation counter
	mt.mu.Lock()
	mt.lastSampleTime = time.Now()
	mt.mu.Unlock()
	
	atomic.StoreInt64(&mt.operationCount, 0)
	
	return nil
}

// GetCurrentUsage returns current memory usage and entry count estimates
func (mt *memoryTracker) GetCurrentUsage() (memoryBytes, entryCount int64) {
	return atomic.LoadInt64(&mt.estimatedMemoryBytes), atomic.LoadInt64(&mt.totalEntries)
}

// Reset clears all memory tracking counters (used by Clear operation)
func (mt *memoryTracker) Reset() {
	atomic.StoreInt64(&mt.estimatedMemoryBytes, 0)
	atomic.StoreInt64(&mt.totalEntries, 0)
	atomic.StoreInt64(&mt.operationCount, 0)
	
	mt.mu.Lock()
	mt.lastSampleTime = time.Now()
	mt.mu.Unlock()
}

// IsMemoryPressure checks if current usage exceeds configured thresholds
func (mt *memoryTracker) IsMemoryPressure(ctx context.Context) bool {
	currentMemory := atomic.LoadInt64(&mt.estimatedMemoryBytes)
	
	// Check absolute threshold
	if mt.config.thresholdBytes > 0 && currentMemory >= mt.config.thresholdBytes {
		return true
	}
	
	// Check percentage threshold if enabled
	if mt.config.thresholdPercent > 0 {
		maxMemory := mt.getRedisMaxMemory(ctx)
		if maxMemory > 0 {
			threshold := float64(maxMemory) * (mt.config.thresholdPercent / 100.0)
			if float64(currentMemory) >= threshold {
				return true
			}
		}
	}
	
	return false
}

// getRedisMaxMemory retrieves Redis maxmemory setting, using cached value when possible
func (mt *memoryTracker) getRedisMaxMemory(ctx context.Context) int64 {
	// Return cached value if available
	cached := atomic.LoadInt64(&mt.redisMaxMemory)
	if cached > 0 {
		return cached
	}
	
	// Query Redis for maxmemory configuration
	result := mt.client.ConfigGet(ctx, "maxmemory")
	if result.Err() != nil {
		return 0
	}
	
	configSlice := result.Val()
	// ConfigGet returns a slice where every pair of elements forms key-value
	// ["maxmemory", "value", "another_config", "another_value", ...]
	for i := 0; i < len(configSlice)-1; i += 2 {
		if configSlice[i] == "maxmemory" {
			maxMemStr, ok := configSlice[i+1].(string)
			if !ok {
				return 0
			}
			
			if maxMemStr == "0" {
				// Redis maxmemory is unlimited, can't use percentage thresholds
				return 0
			}
			
			// Parse maxmemory value (string representation of bytes)
			var maxMem int64
			if _, err := fmt.Sscanf(maxMemStr, "%d", &maxMem); err == nil && maxMem > 0 {
				// Cache the result
				atomic.StoreInt64(&mt.redisMaxMemory, maxMem)
				return maxMem
			}
		}
	}
	
	return 0
}
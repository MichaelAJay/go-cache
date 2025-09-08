package cache

import (
	"context"
	"fmt"
	"time"

	cacheErrors "github.com/MichaelAJay/go-cache/cache_errors"
)

// ExperimentalGetManyRaw - bypasses deserialization entirely
// Returns raw serialized strings instead of deserialized objects
// This eliminates MsgPack deserialization overhead for performance testing
func (c *RedisCache[T]) ExperimentalGetManyRaw(ctx context.Context, keys []string) (map[string]string, error) {
	start := time.Now()
	result := make(map[string]string, len(keys))

	if c.isCircuitBreakerOpen() {
		c.precomputedMetrics.GetManyCircuitBreakerErrorCounter().Inc()
		return result, cacheErrors.ErrCircuitBreakerOpen
	}

	if len(keys) == 0 {
		return result, nil
	}

	// Build Redis keys for pipeline using pooled slices and batch string building
	dataKeys := c.slicePool.GetStringSliceWithLength(len(keys))
	defer c.slicePool.PutStringSlice(dataKeys)
	
	// Use batch key building to reduce string builder allocation overhead
	c.buildDataKeysMany(keys, dataKeys)

	// Use optimized pipeline approach
	pipe := c.client.TxPipeline()

	// Get all data values using pooled slice
	dataResults := c.slicePool.GetAnySliceWithLength(len(keys))
	defer c.slicePool.PutAnySlice(dataResults)
	for i, dataKey := range dataKeys {
		dataResults[i] = pipe.Get(ctx, dataKey)
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil && err.Error() != "redis: nil" {
		c.handleError("getmany", err)
		c.precomputedMetrics.GetManyRedisErrorCounter().Inc()
		return result, fmt.Errorf("redis GetMany error: %w", err)
	}

	// Process results - NO DESERIALIZATION
	hits := 0
	misses := 0

	for i, key := range keys {
		if cmdResult, ok := dataResults[i].(interface {
			Val() string
			Err() error
		}); ok {
			serializedValue := cmdResult.Val()
			if cmdResult.Err() == nil && serializedValue != "" {
				// NO DESERIALIZATION - just return raw string
				result[key] = serializedValue
				hits++
			} else {
				misses++
			}
		} else {
			misses++
		}
	}

	// Record metrics using precomputed metrics for zero-allocation performance
	duration := time.Since(start)
	c.precomputedMetrics.GetManyTimer().Record(duration)
	c.precomputedMetrics.GetManyBatchCounter().Inc()

	// Record batch hits/misses
	if hits > 0 {
		c.precomputedMetrics.GetManyHitCounter().Inc()
	}
	if misses > 0 {
		c.precomputedMetrics.GeneralMissCounter().Inc()
	}

	return result, nil
}
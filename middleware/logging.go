package middleware

import (
	"context"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-logger"
)

// loggingCache wraps a Cache with logging capabilities
type loggingCache struct {
	cache  cache.Cache
	logger logger.Logger
}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware(logger logger.Logger) cache.CacheMiddleware {
	return func(next cache.Cache) cache.Cache {
		return &loggingCache{
			cache:  next,
			logger: logger,
		}
	}
}

// Get retrieves a value from the cache with logging
func (c *loggingCache) Get(ctx context.Context, key string) (any, bool, error) {
	start := time.Now()
	value, exists, err := c.cache.Get(ctx, key)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache get error", logger.Field{Key: "key", Value: key}, logger.Field{Key: "error", Value: err})
		return nil, false, err
	}

	if exists {
		c.logger.Debug("Cache hit", logger.Field{Key: "key", Value: key}, logger.Field{Key: "duration", Value: duration})
	} else {
		c.logger.Debug("Cache miss", logger.Field{Key: "key", Value: key}, logger.Field{Key: "duration", Value: duration})
	}

	return value, exists, nil
}

// Set stores a value in the cache with logging
func (c *loggingCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	start := time.Now()
	err := c.cache.Set(ctx, key, value, ttl)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache set error", logger.Field{Key: "key", Value: key}, logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache set", logger.Field{Key: "key", Value: key}, logger.Field{Key: "ttl", Value: ttl}, logger.Field{Key: "duration", Value: duration})
	}

	return err
}

// Delete removes a value from the cache with logging
func (c *loggingCache) Delete(ctx context.Context, key string) error {
	start := time.Now()
	err := c.cache.Delete(ctx, key)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache delete error", logger.Field{Key: "key", Value: key}, logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache delete", logger.Field{Key: "key", Value: key}, logger.Field{Key: "duration", Value: duration})
	}

	return err
}

// Clear removes all values from the cache with logging
func (c *loggingCache) Clear(ctx context.Context) error {
	start := time.Now()
	err := c.cache.Clear(ctx)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache clear error", logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache clear", logger.Field{Key: "duration", Value: duration})
	}

	return err
}

// Has checks if a key exists in the cache with logging
func (c *loggingCache) Has(ctx context.Context, key string) bool {
	start := time.Now()
	exists := c.cache.Has(ctx, key)
	duration := time.Since(start)

	c.logger.Debug("Cache has", logger.Field{Key: "key", Value: key}, logger.Field{Key: "exists", Value: exists}, logger.Field{Key: "duration", Value: duration})

	return exists
}

// GetKeys returns all keys in the cache with logging
func (c *loggingCache) GetKeys(ctx context.Context) []string {
	start := time.Now()
	keys := c.cache.GetKeys(ctx)
	duration := time.Since(start)

	c.logger.Debug("Cache get keys", logger.Field{Key: "count", Value: len(keys)}, logger.Field{Key: "duration", Value: duration})

	return keys
}

// Close closes the cache with logging
func (c *loggingCache) Close() error {
	start := time.Now()
	err := c.cache.Close()
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache close error", logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache close", logger.Field{Key: "duration", Value: duration})
	}

	return err
}

// GetMany retrieves multiple values from the cache with logging
func (c *loggingCache) GetMany(ctx context.Context, keys []string) (map[string]any, error) {
	start := time.Now()
	values, err := c.cache.GetMany(ctx, keys)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache get many error", logger.Field{Key: "keys", Value: keys}, logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache get many", logger.Field{Key: "keys", Value: keys}, logger.Field{Key: "count", Value: len(values)}, logger.Field{Key: "duration", Value: duration})
	}

	return values, err
}

// SetMany stores multiple values in the cache with logging
func (c *loggingCache) SetMany(ctx context.Context, items map[string]any, ttl time.Duration) error {
	start := time.Now()
	err := c.cache.SetMany(ctx, items, ttl)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache set many error", logger.Field{Key: "count", Value: len(items)}, logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache set many", logger.Field{Key: "count", Value: len(items)}, logger.Field{Key: "ttl", Value: ttl}, logger.Field{Key: "duration", Value: duration})
	}

	return err
}

// DeleteMany removes multiple values from the cache with logging
func (c *loggingCache) DeleteMany(ctx context.Context, keys []string) error {
	start := time.Now()
	err := c.cache.DeleteMany(ctx, keys)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache delete many error", logger.Field{Key: "keys", Value: keys}, logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache delete many", logger.Field{Key: "keys", Value: keys}, logger.Field{Key: "duration", Value: duration})
	}

	return err
}

// GetMetadata retrieves metadata for a cache entry with logging
func (c *loggingCache) GetMetadata(ctx context.Context, key string) (*cache.CacheEntryMetadata, error) {
	start := time.Now()
	metadata, err := c.cache.GetMetadata(ctx, key)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache get metadata error", logger.Field{Key: "key", Value: key}, logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache get metadata", logger.Field{Key: "key", Value: key}, logger.Field{Key: "duration", Value: duration})
	}

	return metadata, err
}

// GetManyMetadata retrieves metadata for multiple cache entries with logging
func (c *loggingCache) GetManyMetadata(ctx context.Context, keys []string) (map[string]*cache.CacheEntryMetadata, error) {
	start := time.Now()
	metadata, err := c.cache.GetManyMetadata(ctx, keys)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache get many metadata error", logger.Field{Key: "keys", Value: keys}, logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache get many metadata", logger.Field{Key: "keys", Value: keys}, logger.Field{Key: "count", Value: len(metadata)}, logger.Field{Key: "duration", Value: duration})
	}

	return metadata, err
}

// Increment atomically increments a numeric value with logging
func (c *loggingCache) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	start := time.Now()
	result, err := c.cache.Increment(ctx, key, delta, ttl)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache increment error", logger.Field{Key: "key", Value: key}, logger.Field{Key: "delta", Value: delta}, logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache increment", logger.Field{Key: "key", Value: key}, logger.Field{Key: "delta", Value: delta}, logger.Field{Key: "result", Value: result}, logger.Field{Key: "duration", Value: duration})
	}

	return result, err
}

// Decrement atomically decrements a numeric value with logging
func (c *loggingCache) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	start := time.Now()
	result, err := c.cache.Decrement(ctx, key, delta, ttl)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache decrement error", logger.Field{Key: "key", Value: key}, logger.Field{Key: "delta", Value: delta}, logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache decrement", logger.Field{Key: "key", Value: key}, logger.Field{Key: "delta", Value: delta}, logger.Field{Key: "result", Value: result}, logger.Field{Key: "duration", Value: duration})
	}

	return result, err
}

// SetIfNotExists sets a value only if the key doesn't exist with logging
func (c *loggingCache) SetIfNotExists(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	start := time.Now()
	success, err := c.cache.SetIfNotExists(ctx, key, value, ttl)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache set if not exists error", logger.Field{Key: "key", Value: key}, logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache set if not exists", logger.Field{Key: "key", Value: key}, logger.Field{Key: "success", Value: success}, logger.Field{Key: "duration", Value: duration})
	}

	return success, err
}

// SetIfExists sets a value only if the key already exists with logging
func (c *loggingCache) SetIfExists(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	start := time.Now()
	success, err := c.cache.SetIfExists(ctx, key, value, ttl)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache set if exists error", logger.Field{Key: "key", Value: key}, logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache set if exists", logger.Field{Key: "key", Value: key}, logger.Field{Key: "success", Value: success}, logger.Field{Key: "duration", Value: duration})
	}

	return success, err
}

// AddIndex adds a secondary index with logging
func (c *loggingCache) AddIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error {
	start := time.Now()
	err := c.cache.AddIndex(ctx, indexName, keyPattern, indexKey)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache add index error", logger.Field{Key: "indexName", Value: indexName}, logger.Field{Key: "keyPattern", Value: keyPattern}, logger.Field{Key: "indexKey", Value: indexKey}, logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache add index", logger.Field{Key: "indexName", Value: indexName}, logger.Field{Key: "keyPattern", Value: keyPattern}, logger.Field{Key: "indexKey", Value: indexKey}, logger.Field{Key: "duration", Value: duration})
	}

	return err
}

// RemoveIndex removes a secondary index with logging
func (c *loggingCache) RemoveIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error {
	start := time.Now()
	err := c.cache.RemoveIndex(ctx, indexName, keyPattern, indexKey)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache remove index error", logger.Field{Key: "indexName", Value: indexName}, logger.Field{Key: "keyPattern", Value: keyPattern}, logger.Field{Key: "indexKey", Value: indexKey}, logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache remove index", logger.Field{Key: "indexName", Value: indexName}, logger.Field{Key: "keyPattern", Value: keyPattern}, logger.Field{Key: "indexKey", Value: indexKey}, logger.Field{Key: "duration", Value: duration})
	}

	return err
}

// GetByIndex retrieves keys by index with logging
func (c *loggingCache) GetByIndex(ctx context.Context, indexName string, indexKey string) ([]string, error) {
	start := time.Now()
	keys, err := c.cache.GetByIndex(ctx, indexName, indexKey)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache get by index error", logger.Field{Key: "indexName", Value: indexName}, logger.Field{Key: "indexKey", Value: indexKey}, logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache get by index", logger.Field{Key: "indexName", Value: indexName}, logger.Field{Key: "indexKey", Value: indexKey}, logger.Field{Key: "count", Value: len(keys)}, logger.Field{Key: "duration", Value: duration})
	}

	return keys, err
}

// DeleteByIndex deletes keys by index with logging
func (c *loggingCache) DeleteByIndex(ctx context.Context, indexName string, indexKey string) error {
	start := time.Now()
	err := c.cache.DeleteByIndex(ctx, indexName, indexKey)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache delete by index error", logger.Field{Key: "indexName", Value: indexName}, logger.Field{Key: "indexKey", Value: indexKey}, logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache delete by index", logger.Field{Key: "indexName", Value: indexName}, logger.Field{Key: "indexKey", Value: indexKey}, logger.Field{Key: "duration", Value: duration})
	}

	return err
}

// GetKeysByPattern retrieves keys by pattern with logging
func (c *loggingCache) GetKeysByPattern(ctx context.Context, pattern string) ([]string, error) {
	start := time.Now()
	keys, err := c.cache.GetKeysByPattern(ctx, pattern)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache get keys by pattern error", logger.Field{Key: "pattern", Value: pattern}, logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache get keys by pattern", logger.Field{Key: "pattern", Value: pattern}, logger.Field{Key: "count", Value: len(keys)}, logger.Field{Key: "duration", Value: duration})
	}

	return keys, err
}

// DeleteByPattern deletes keys by pattern with logging
func (c *loggingCache) DeleteByPattern(ctx context.Context, pattern string) (int, error) {
	start := time.Now()
	count, err := c.cache.DeleteByPattern(ctx, pattern)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache delete by pattern error", logger.Field{Key: "pattern", Value: pattern}, logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache delete by pattern", logger.Field{Key: "pattern", Value: pattern}, logger.Field{Key: "count", Value: count}, logger.Field{Key: "duration", Value: duration})
	}

	return count, err
}

// UpdateMetadata updates cache entry metadata with logging
func (c *loggingCache) UpdateMetadata(ctx context.Context, key string, updater cache.MetadataUpdater) error {
	start := time.Now()
	err := c.cache.UpdateMetadata(ctx, key, updater)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache update metadata error", logger.Field{Key: "key", Value: key}, logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache update metadata", logger.Field{Key: "key", Value: key}, logger.Field{Key: "duration", Value: duration})
	}

	return err
}

// GetAndUpdate atomically gets and updates a cache entry with logging
func (c *loggingCache) GetAndUpdate(ctx context.Context, key string, updater cache.ValueUpdater, ttl time.Duration) (any, error) {
	start := time.Now()
	value, err := c.cache.GetAndUpdate(ctx, key, updater, ttl)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Cache get and update error", logger.Field{Key: "key", Value: key}, logger.Field{Key: "error", Value: err})
	} else {
		c.logger.Debug("Cache get and update", logger.Field{Key: "key", Value: key}, logger.Field{Key: "duration", Value: duration})
	}

	return value, err
}

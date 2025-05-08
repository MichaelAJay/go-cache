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

// GetMetrics returns the current metrics snapshot
func (c *loggingCache) GetMetrics() *cache.CacheMetricsSnapshot {
	return c.cache.GetMetrics()
}

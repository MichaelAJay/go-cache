package cache

import (
	"time"

	"github.com/MichaelAJay/go-logger"
)

// CacheOption defines a function type for configuring cache options
type CacheOption func(*CacheOptions)

// WithTTL sets the default time-to-live for cache entries
func WithTTL(ttl time.Duration) CacheOption {
	return func(o *CacheOptions) {
		o.TTL = ttl
	}
}

// WithMaxEntries sets the maximum number of entries for the cache
func WithMaxEntries(max int) CacheOption {
	return func(o *CacheOptions) {
		o.MaxEntries = max
	}
}

// WithCleanupInterval sets the interval for cleaning up expired entries
func WithCleanupInterval(interval time.Duration) CacheOption {
	return func(o *CacheOptions) {
		o.CleanupInterval = interval
	}
}

// WithLogger sets the logger for the cache
func WithLogger(logger logger.Logger) CacheOption {
	return func(o *CacheOptions) {
		o.Logger = logger
	}
}

// WithRedisOptions sets the Redis-specific options
func WithRedisOptions(options *RedisOptions) CacheOption {
	return func(o *CacheOptions) {
		o.RedisOptions = options
	}
}

// WithMaxSize sets the maximum size in bytes for the cache
func WithMaxSize(maxSize int64) CacheOption {
	return func(o *CacheOptions) {
		o.MaxSize = maxSize
	}
}

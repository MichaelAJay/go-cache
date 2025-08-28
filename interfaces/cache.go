package interfaces

import (
	"context"
	"time"
)

// Cache defines the primary generic-first interface for all cache implementations
// 
// IMPLEMENTATION REQUIREMENTS:
// All methods MUST be implemented to be goroutine-safe - multiple goroutines can call
// any method concurrently without external synchronization. Consumers should never need
// mutexes, channels, or any synchronization primitives when using this interface.
type Cache[T any] interface {
	// Basic operations
	// IMPLEMENTATION REQUIREMENT: All basic operations must be goroutine-safe
	
	// Get retrieves a value by key
	// MUST return zero value of T if key doesn't exist (found=false)
	// MUST be goroutine-safe for concurrent access
	Get(ctx context.Context, key string) (value T, found bool, err error)
	
	// Set stores a value with TTL
	// MUST be goroutine-safe for concurrent access with other operations
	// MUST handle TTL=0 as "no expiration" consistently across providers
	Set(ctx context.Context, key string, value T, ttl time.Duration) error
	
	// Delete removes a key
	// MUST be goroutine-safe and idempotent (no error if key doesn't exist)
	Delete(ctx context.Context, key string) error
	
	// Clear removes all entries
	// MUST be goroutine-safe but may temporarily affect other operations
	Clear(ctx context.Context) error
	
	// Has checks if key exists without retrieving value
	// MUST be goroutine-safe for concurrent access
	Has(ctx context.Context, key string) bool

	// Atomic operations
	// IMPLEMENTATION REQUIREMENT: These operations MUST be atomic - no race conditions
	// even under extreme concurrent load. They eliminate the need for consumer-side locking.
	
	// GetOrSet atomically gets existing value or sets new value from loader
	// CRITICAL: loader function MUST be called exactly once per key under contention
	// MUST use singleflight pattern or equivalent to prevent duplicate loader calls
	// MUST be goroutine-safe across all concurrent calls
	GetOrSet(ctx context.Context, key string, loader func(ctx context.Context) (T, error), ttl time.Duration) (T, error)
	
	// Update atomically updates existing value or creates new value
	// CRITICAL: updater function MUST be called atomically - no lost updates
	// MUST handle the case where key doesn't exist (exists=false, old=zero value of T)
	// MUST be goroutine-safe with no possibility of race conditions
	Update(ctx context.Context, key string, updater func(old T, exists bool) (T, error), ttl time.Duration) (T, error)

	// Batch operations for performance
	// IMPLEMENTATION REQUIREMENT: All batch operations must be goroutine-safe and
	// should be optimized for better performance than individual operations
	
	// GetMany retrieves multiple keys in a single operation
	// MUST return map with only found keys, missing keys omitted from result
	// MUST be goroutine-safe for concurrent access
	GetMany(ctx context.Context, keys []string) (map[string]T, error)
	
	// SetMany stores multiple key-value pairs with same TTL
	// MUST be goroutine-safe and atomic where possible (all-or-nothing preferred)
	SetMany(ctx context.Context, items map[string]T, ttl time.Duration) error
	
	// DeleteMany removes multiple keys
	// MUST be goroutine-safe and idempotent (no errors for missing keys)
	DeleteMany(ctx context.Context, keys []string) error

	// Conditional operations
	// IMPLEMENTATION REQUIREMENT: Must be atomic checks - no race conditions between
	// existence check and set operation
	
	// SetIfNotExists atomically sets value only if key doesn't exist
	// MUST return true if value was set, false if key already existed
	// MUST be atomic - no race condition between check and set
	SetIfNotExists(ctx context.Context, key string, value T, ttl time.Duration) (wasSet bool, err error)
	
	// SetIfExists atomically sets value only if key exists
	// MUST return true if value was updated, false if key didn't exist
	// MUST be atomic - no race condition between check and set  
	SetIfExists(ctx context.Context, key string, value T, ttl time.Duration) (wasSet bool, err error)

	// Secondary indexing system
	// IMPLEMENTATION REQUIREMENT: All indexing operations must be goroutine-safe and
	// maintain consistency between primary data and indexes under concurrent access
	
	// AddIndex associates a key matching keyPattern with an index entry
	// MUST be goroutine-safe and maintain index consistency
	// MUST handle keyPattern globbing (e.g., "user:*")
	AddIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error
	
	// RemoveIndex removes association between key pattern and index entry
	// MUST be goroutine-safe and idempotent
	RemoveIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error
	
	// GetByIndex returns all keys associated with an index entry
	// MUST be goroutine-safe and return consistent snapshot
	GetByIndex(ctx context.Context, indexName string, indexKey string) ([]string, error)
	
	// DeleteByIndex removes all keys associated with an index entry
	// MUST be goroutine-safe and atomic where possible
	// MUST return count of deleted keys or error
	DeleteByIndex(ctx context.Context, indexName string, indexKey string) error

	// Pattern operations
	// IMPLEMENTATION REQUIREMENT: Must be goroutine-safe and provide consistent results
	
	// GetKeysByPattern returns keys matching pattern (e.g., "user:*")
	// MUST be goroutine-safe and return consistent snapshot
	GetKeysByPattern(ctx context.Context, pattern string) ([]string, error)
	
	// DeleteByPattern removes all keys matching pattern
	// MUST be goroutine-safe and return count of deleted keys
	DeleteByPattern(ctx context.Context, pattern string) (deletedCount int, err error)

	// Metadata operations
	// IMPLEMENTATION REQUIREMENT: Must provide consistent metadata view
	
	// GetMetadata returns metadata for a cache entry
	// MUST be goroutine-safe and return accurate metadata snapshot
	// MUST return nil if key doesn't exist (not an error)
	GetMetadata(ctx context.Context, key string) (*CacheEntryMetadata, error)

	// Lifecycle management
	// IMPLEMENTATION REQUIREMENT: Must cleanup all resources safely
	
	// Close shuts down the cache and cleans up resources
	// MUST be goroutine-safe and idempotent
	// MUST wait for ongoing operations to complete where possible
	Close() error
}

// Manager interface for basic lifecycle management of cache providers
// IMPLEMENTATION REQUIREMENT: Must be goroutine-safe for concurrent provider registration
type Manager interface {
	// RegisterProvider registers a cache provider for use
	// MUST be goroutine-safe for concurrent registration
	RegisterProvider(name string, provider CacheProvider)
	
	// Close shuts down all registered providers
	// MUST be goroutine-safe and idempotent
	Close() error
}

// CacheProvider interface for creating cache instances
// IMPLEMENTATION REQUIREMENT: Providers must support creating typed Cache[T] instances
// through external factory functions. Each provider handles its own resource management.
type CacheProvider interface {
	// Name returns the unique provider name (e.g., "memory", "redis")
	// MUST return consistent name across all calls
	Name() string
	
	// Validate checks if the provided options are compatible with this provider
	// MUST return detailed error messages for invalid configurations
	// MUST be goroutine-safe (read-only validation)
	Validate(options *CacheOptions) error
	
	// Close cleans up any provider-level resources (connection pools, etc.)
	// MUST be goroutine-safe and idempotent
	// MUST wait for provider-wide resources to cleanup gracefully
	Close() error
}

// Option defines functional options for cache configuration
// Uses the existing rich CacheOptions structure from options.go which includes
// TTL, serialization, metrics, security, indexing, and provider-specific options
type Option func(*CacheOptions)

// CacheFactory defines the signature for provider-specific cache creation functions
// IMPLEMENTATION REQUIREMENT: Each provider must implement factory functions with this signature
// to create typed Cache[T] instances. Factories must validate options and return configured cache.
type CacheFactory[T any] func(options *CacheOptions) (Cache[T], error)

// LoaderFunc defines a function type for loading values in GetOrSet operations
// USAGE: Loader functions are called when a key is not found in cache
// REQUIREMENT: Must be safe to call concurrently, but GetOrSet implementations must
// ensure the loader is called exactly once per key under contention (singleflight pattern)
type LoaderFunc[T any] func(ctx context.Context) (T, error)

// UpdaterFunc defines a function type for updating values in Update operations  
// USAGE: Updater functions receive the current value (or zero value if key doesn't exist)
// and return the new value to store. The exists parameter indicates if the key was found.
// REQUIREMENT: Must be safe to call concurrently, but Update implementations must
// ensure atomic execution with no lost updates under contention
type UpdaterFunc[T any] func(old T, exists bool) (T, error)
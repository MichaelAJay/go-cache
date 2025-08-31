package interfaces

import (
	"context"
	"time"
)

// Cache defines the primary generic-first interface for Redis-only cache implementation
//
// IMPLEMENTATION REQUIREMENTS:
// All methods MUST be implemented to be goroutine-safe - multiple goroutines can call
// any method concurrently without external synchronization. Consumers should never need
// mutexes, channels, or any synchronization primitives when using this interface.
//
// KEY EXTRACTION BEHAVIOR:
// If indexing is enabled, the cache uses configured extractors to automatically
// extract element keys and owner keys from values. This enables automatic indexing
// without requiring consumers to manually manage keys or indexes.
type Cache[T any] interface {
	// Basic operations - key extraction based
	// IMPLEMENTATION REQUIREMENT: All basic operations must be goroutine-safe

	// Set stores a value with TTL, using configured extractors to determine storage key
	// If indexing enabled: extracts key and reverseKey from value, updates indexes
	// If indexing disabled: uses key extractor for storage key only
	// MUST be goroutine-safe and atomic across value storage and index updates
	Set(ctx context.Context, value T, ttl time.Duration) error

	// Get retrieves a value by element key
	// MUST return zero value of T if key doesn't exist (found=false)
	// MUST be goroutine-safe for concurrent access
	Get(ctx context.Context, key string) (value T, found bool, err error)

	// Delete removes an element by key and cleans up all associated indexes
	// MUST be goroutine-safe and idempotent (no error if key doesn't exist)
	Delete(ctx context.Context, key string) error

	// Clear removes all entries
	// MUST be goroutine-safe but may temporarily affect other operations
	Clear(ctx context.Context) error

	// Has checks if key exists without retrieving value
	// MUST be goroutine-safe for concurrent access
	Has(ctx context.Context, key string) bool

	// Owner-based operations (requires indexing to be enabled)
	// IMPLEMENTATION REQUIREMENT: These methods require IndexExtractor configuration

	// GetByOwner retrieves all elements for a given owner
	// MUST return empty slice (not error) if owner has no elements
	// MUST be goroutine-safe for concurrent access
	GetByOwner(ctx context.Context, key string) ([]T, error)

	// DeleteByOwner removes all elements for a given owner
	// MUST return count of deleted elements
	// MUST be goroutine-safe and atomic across all deletions
	DeleteByOwner(ctx context.Context, key string) (deletedCount int, err error)

	// Atomic operations with key extraction
	// IMPLEMENTATION REQUIREMENT: These operations MUST be atomic - no race conditions
	// even under extreme concurrent load. They eliminate the need for consumer-side locking.

	// GetOrSet atomically gets existing value by key or sets new value from loader
	// CRITICAL: loader function MUST be called exactly once per key under contention
	// MUST use singleflight pattern or equivalent to prevent duplicate loader calls
	// MUST be goroutine-safe across all concurrent calls
	GetOrSet(ctx context.Context, key string, loader func(ctx context.Context) (T, error), ttl time.Duration) (T, error)

	// Update atomically updates existing value or creates new value by key
	// CRITICAL: updater function MUST be called atomically - no lost updates
	// MUST handle the case where key doesn't exist (exists=false, old=zero value of T)
	// MUST be goroutine-safe with no possibility of race conditions
	Update(ctx context.Context, key string, updater func(old T, exists bool) (T, error), ttl time.Duration) (T, error)

	// Batch operations for performance
	// IMPLEMENTATION REQUIREMENT: All batch operations must be goroutine-safe and
	// should be optimized for better performance than individual operations

	// GetMany retrieves multiple elements by keys in a single operation
	// MUST return map with only found keys, missing keys omitted from result
	// MUST be goroutine-safe for concurrent access
	GetMany(ctx context.Context, keys []string) (map[string]T, error)

	// SetMany stores multiple values with same TTL, using extractors for keys
	// MUST be goroutine-safe and atomic where possible (all-or-nothing preferred)
	// MUST update indexes for each value if indexing enabled
	SetMany(ctx context.Context, values []T, ttl time.Duration) error

	// DeleteMany removes multiple elements by keys
	// MUST be goroutine-safe and idempotent (no errors for missing keys)
	// MUST clean up indexes for all deleted elements
	DeleteMany(ctx context.Context, keys []string) error

	// Conditional operations
	// IMPLEMENTATION REQUIREMENT: Must be atomic checks - no race conditions between
	// existence check and set operation

	// SetIfNotExists atomically sets value only if key doesn't exist
	// Uses extractors to determine key from value
	// MUST return true if value was set, false if key already existed
	// MUST be atomic - no race condition between check and set
	SetIfNotExists(ctx context.Context, value T, ttl time.Duration) (wasSet bool, err error)

	// SetIfExists atomically sets value only if key exists
	// Uses extractors to determine key from value
	// MUST return true if value was updated, false if key didn't exist
	// MUST be atomic - no race condition between check and set
	SetIfExists(ctx context.Context, value T, ttl time.Duration) (wasSet bool, err error)

	// Pattern operations (for advanced use cases)
	// IMPLEMENTATION REQUIREMENT: Must be goroutine-safe and provide consistent results

	// GetKeysByPattern returns element keys matching pattern (e.g., "session:*")
	// MUST be goroutine-safe and return consistent snapshot
	GetKeysByPattern(ctx context.Context, pattern string) ([]string, error)

	// DeleteByPattern removes all elements with keys matching pattern
	// MUST be goroutine-safe and return count of deleted elements
	// MUST clean up indexes for all deleted elements
	DeleteByPattern(ctx context.Context, pattern string) (deletedCount int, err error)

	// Metadata operations
	// IMPLEMENTATION REQUIREMENT: Must provide consistent metadata view

	// GetMetadata returns metadata for a cache entry by key
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

// CacheEntryMetadata represents metadata for a cache entry
type CacheEntryMetadata struct {
	Key          string
	CreatedAt    time.Time
	LastAccessed time.Time
	AccessCount  int64
	TTL          time.Duration
	Size         int64
	Tags         []string
}

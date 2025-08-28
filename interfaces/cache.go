package interfaces

import (
	"context"
	"time"
)

// Cache defines the primary generic-first interface for all cache implementations
// All operations are thread-safe by design - consumers never need synchronization primitives
type Cache[T any] interface {
	// Basic operations - thread-safe and generic
	Get(ctx context.Context, key string) (T, bool, error)
	Set(ctx context.Context, key string, value T, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
	Has(ctx context.Context, key string) bool

	// Atomic operations (eliminate consumer-side locking)
	GetOrSet(ctx context.Context, key string, loader func(ctx context.Context) (T, error), ttl time.Duration) (T, error)
	Update(ctx context.Context, key string, updater func(old T, exists bool) (T, error), ttl time.Duration) (T, error)

	// Batch operations for performance
	GetMany(ctx context.Context, keys []string) (map[string]T, error)
	SetMany(ctx context.Context, items map[string]T, ttl time.Duration) error
	DeleteMany(ctx context.Context, keys []string) error

	// Conditional operations
	SetIfNotExists(ctx context.Context, key string, value T, ttl time.Duration) (bool, error)
	SetIfExists(ctx context.Context, key string, value T, ttl time.Duration) (bool, error)

	// Secondary indexing (thread-safe)
	AddIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error
	RemoveIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error
	GetByIndex(ctx context.Context, indexName string, indexKey string) ([]string, error)
	DeleteByIndex(ctx context.Context, indexName string, indexKey string) error

	// Pattern operations
	GetKeysByPattern(ctx context.Context, pattern string) ([]string, error)
	DeleteByPattern(ctx context.Context, pattern string) (int, error)

	// Metadata operations
	GetMetadata(ctx context.Context, key string) (*CacheEntryMetadata, error)

	// Lifecycle
	Close() error
}

// Manager interface for basic lifecycle management
type Manager interface {
	RegisterProvider(name string, provider CacheProvider)
	Close() error
}

// CacheManager provides typed cache creation methods
// This is the concrete type that applications will use
type CacheManager struct {
	providers map[string]CacheProvider
	caches    map[string]any // stores Cache[T] instances with various T types
}

// NewCacheManager creates a new cache manager
func NewCacheManager() *CacheManager {
	return &CacheManager{
		providers: make(map[string]CacheProvider),
		caches:    make(map[string]any),
	}
}

// RegisterProvider registers a cache provider
func (m *CacheManager) RegisterProvider(name string, provider CacheProvider) {
	m.providers[name] = provider
}

// Close closes all managed caches
func (m *CacheManager) Close() error {
	// Implementation will iterate through caches and close them
	panic("not implemented in Phase 0 - interface design only")
}

// NewCache creates a new typed cache instance
// This is implemented as a package-level generic function rather than a method
func NewCache[T any](manager *CacheManager, providerName string, opts ...Option) (Cache[T], error) {
	// This function will be implemented to create and return Cache[T] instances
	// The implementation will need provider-specific logic
	panic("not implemented in Phase 0 - interface design only")
}

// CacheProvider interface for creating cache instances
// Providers implement factory methods that will be called by the manager
// The actual cache creation will happen through factory functions
type CacheProvider interface {
	// Name returns the provider name (e.g., "memory", "redis")
	Name() string
	
	// Validate checks if the provided options are valid for this provider
	Validate(options *CacheOptions) error
	
	// Close cleans up any provider-level resources
	Close() error
}

// Option defines functional options for cache configuration
// Uses the existing CacheOptions from options.go
type Option func(*CacheOptions)

// CacheFactory defines the signature for provider-specific cache creation functions
// Each provider will implement this to create typed caches
type CacheFactory[T any] func(options *CacheOptions) (Cache[T], error)

// LoaderFunc defines a function type for loading values in GetOrSet operations
type LoaderFunc[T any] func(ctx context.Context) (T, error)

// UpdaterFunc defines a function type for updating values in Update operations
type UpdaterFunc[T any] func(old T, exists bool) (T, error)
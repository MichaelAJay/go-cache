package cache

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/MichaelAJay/go-cache/interfaces"
)

// TypedCache provides generic type-safe operations over a cache
// This allows the serializer to know the exact type for deserialization
type TypedCache[T any] struct {
	cache interfaces.Cache
}

// NewTypedCache creates a new typed cache wrapper
func NewTypedCache[T any](cache interfaces.Cache) *TypedCache[T] {
	return &TypedCache[T]{
		cache: cache,
	}
}

// Get retrieves a value with full type safety
// The serializer will know to deserialize to type T
func (tc *TypedCache[T]) Get(ctx context.Context, key string) (T, bool, error) {
	var zero T
	typeInfo := getTypeInfo[T]()
	
	// Check if the cache supports typed operations
	if typedCache, ok := tc.cache.(interfaces.TypedCacheProvider); ok {
		value, found, err := typedCache.GetWithTypeInfo(ctx, key, typeInfo)
		if err != nil {
			return zero, false, err
		}
		if !found {
			return zero, false, nil
		}
		
		// Type assertion with proper error handling
		typedValue, ok := value.(T)
		if !ok {
			return zero, false, fmt.Errorf("cache value is not of expected type %T, got %T", zero, value)
		}
		
		return typedValue, true, nil
	}
	
	// Fallback to regular cache with type assertion
	value, found, err := tc.cache.Get(ctx, key)
	if err != nil {
		return zero, false, err
	}
	if !found {
		return zero, false, nil
	}
	
	// Type assertion with proper error handling
	typedValue, ok := value.(T)
	if !ok {
		return zero, false, fmt.Errorf("cache value is not of expected type %T, got %T", zero, value)
	}
	
	return typedValue, true, nil
}

// Set stores a value with full type information
func (tc *TypedCache[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
	typeInfo := getTypeInfo[T]()
	
	// Check if the cache supports typed operations
	if typedCache, ok := tc.cache.(interfaces.TypedCacheProvider); ok {
		return typedCache.SetWithTypeInfo(ctx, key, value, typeInfo, ttl)
	}
	
	// Fallback to regular cache
	return tc.cache.Set(ctx, key, value, ttl)
}

// Delete removes a key from the cache
func (tc *TypedCache[T]) Delete(ctx context.Context, key string) error {
	return tc.cache.Delete(ctx, key)
}

// GetAndUpdate performs an atomic get-and-update operation with type safety
func (tc *TypedCache[T]) GetAndUpdate(ctx context.Context, key string, updater func(T) (T, bool), ttl time.Duration) (T, error) {
	var zero T
	typeInfo := getTypeInfo[T]()
	
	// Check if the cache supports typed operations
	if typedCache, ok := tc.cache.(interfaces.TypedCacheProvider); ok {
		genericUpdater := func(value any) (any, bool) {
			if value == nil {
				return updater(zero)
			}
			
			typedValue, ok := value.(T)
			if !ok {
				return value, false
			}
			
			return updater(typedValue)
		}
		
		result, err := typedCache.GetAndUpdateWithTypeInfo(ctx, key, typeInfo, genericUpdater, ttl)
		if err != nil {
			return zero, err
		}
		
		typedResult, ok := result.(T)
		if !ok {
			return zero, fmt.Errorf("cache result is not of expected type %T, got %T", zero, result)
		}
		
		return typedResult, nil
	}
	
	// Fallback implementation using regular cache
	genericUpdater := func(value any) (any, bool) {
		if value == nil {
			// Create zero value of T for new entries
			return updater(zero)
		}
		
		typedValue, ok := value.(T)
		if !ok {
			// Return unchanged if type mismatch
			return value, false
		}
		
		return updater(typedValue)
	}
	
	result, err := tc.cache.GetAndUpdate(ctx, key, genericUpdater, ttl)
	if err != nil {
		return zero, err
	}
	
	typedResult, ok := result.(T)
	if !ok {
		return zero, fmt.Errorf("cache result is not of expected type %T, got %T", zero, result)
	}
	
	return typedResult, nil
}

// GetMany retrieves multiple values with type safety
func (tc *TypedCache[T]) GetMany(ctx context.Context, keys []string) (map[string]T, error) {
	typeInfo := getTypeInfo[T]()
	
	// Check if the cache supports typed bulk operations
	if typedCache, ok := tc.cache.(interfaces.TypedCacheProvider); ok {
		values, err := typedCache.GetManyWithTypeInfo(ctx, keys, typeInfo)
		if err != nil {
			return nil, err
		}
		
		result := make(map[string]T, len(values))
		var zero T
		
		for key, value := range values {
			typedValue, ok := value.(T)
			if !ok {
				return nil, fmt.Errorf("cache value for key %q is not of expected type %T, got %T", key, zero, value)
			}
			result[key] = typedValue
		}
		
		return result, nil
	}
	
	// Fallback implementation
	values, err := tc.cache.GetMany(ctx, keys)
	if err != nil {
		return nil, err
	}
	
	result := make(map[string]T, len(values))
	var zero T
	
	for key, value := range values {
		typedValue, ok := value.(T)
		if !ok {
			return nil, fmt.Errorf("cache value for key %q is not of expected type %T, got %T", key, zero, value)
		}
		result[key] = typedValue
	}
	
	return result, nil
}

// SetMany stores multiple values with type information
func (tc *TypedCache[T]) SetMany(ctx context.Context, items map[string]T, ttl time.Duration) error {
	typeInfo := getTypeInfo[T]()
	
	// Check if the cache supports typed bulk operations
	if typedCache, ok := tc.cache.(interfaces.TypedCacheProvider); ok {
		genericItems := make(map[string]any, len(items))
		for key, value := range items {
			genericItems[key] = value
		}
		
		return typedCache.SetManyWithTypeInfo(ctx, genericItems, typeInfo, ttl)
	}
	
	// Fallback implementation
	genericItems := make(map[string]any, len(items))
	for key, value := range items {
		genericItems[key] = value
	}
	
	return tc.cache.SetMany(ctx, genericItems, ttl)
}

// Unwrap returns the underlying cache for operations that don't need type safety
func (tc *TypedCache[T]) Unwrap() interfaces.Cache {
	return tc.cache
}


// getTypeInfo returns type information for the generic type T (internal function)
func getTypeInfo[T any]() interfaces.TypeInfo {
	var zero T
	t := reflect.TypeOf(zero)
	if t == nil {
		// Handle interface{} case - this is problematic for type-safe operations
		// We should encourage users to use concrete types instead
		return interfaces.TypeInfo{
			Type:     nil,
			TypeName: "interface{}",
		}
	}
	
	// Ensure we have a concrete type name that distinguishes pointers from values
	typeName := t.String()
	if t.Kind() == reflect.Ptr {
		typeName = "*" + t.Elem().String()
	}
	
	return interfaces.TypeInfo{
		Type:     t,
		TypeName: typeName,
	}
}

// GetTypeInfo returns type information for the generic type T (public function)
func GetTypeInfo[T any]() interfaces.TypeInfo {
	return getTypeInfo[T]()
}
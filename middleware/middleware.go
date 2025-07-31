package middleware

import (
	"github.com/MichaelAJay/go-cache"
)

// CacheMiddleware defines a function type for cache middleware
type CacheMiddleware func(cache.Cache) cache.Cache

// Chain applies multiple middleware functions to a cache in order
// The middleware are applied from right to left (last to first)
// so the rightmost middleware wraps the cache directly
func Chain(cache cache.Cache, middlewares ...CacheMiddleware) cache.Cache {
	// Apply middleware in reverse order so the first middleware in the list
	// is the outermost (executed first)
	for i := len(middlewares) - 1; i >= 0; i-- {
		cache = middlewares[i](cache)
	}
	return cache
}

// Compose creates a single middleware from multiple middleware functions
// This is useful when you want to create a reusable middleware stack
func Compose(middlewares ...CacheMiddleware) CacheMiddleware {
	return func(cache cache.Cache) cache.Cache {
		return Chain(cache, middlewares...)
	}
}
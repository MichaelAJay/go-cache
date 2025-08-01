package middleware

import (
	"github.com/MichaelAJay/go-cache"
)

// CacheMiddleware defines a function type for cache middleware
type CacheMiddleware func(cache.Cache) cache.Cache

// Chain applies multiple middleware functions to a cache in order.
// The middleware are applied from right to left (last to first)
// so the rightmost middleware wraps the cache directly.
//
// Middleware Chain Algorithm:
// 1. Processes middleware slice in reverse order (last to first)
// 2. Each middleware wraps the result of the previous middleware
// 3. Creates an "onion" pattern where first middleware is outermost
//
// Example: Chain(cache, logging, metrics, auth)
// Results in: logging(metrics(auth(cache)))
// Execution order: logging -> metrics -> auth -> cache
func Chain(cache cache.Cache, middlewares ...CacheMiddleware) cache.Cache {
	// Apply middleware in reverse order so the first middleware in the list
	// is the outermost (executed first)
	for i := len(middlewares) - 1; i >= 0; i-- {
		cache = middlewares[i](cache)
	}
	return cache
}

// Compose creates a single middleware from multiple middleware functions.
// This is useful when you want to create a reusable middleware stack
// that can be applied to multiple cache instances.
//
// Example:
//   standardMiddleware := Compose(logging, metrics, auth)
//   cache1 := standardMiddleware(memoryCache)
//   cache2 := standardMiddleware(redisCache)
func Compose(middlewares ...CacheMiddleware) CacheMiddleware {
	return func(cache cache.Cache) cache.Cache {
		return Chain(cache, middlewares...)
	}
}
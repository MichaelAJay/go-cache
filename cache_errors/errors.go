package cache_errors

import "errors"

// Common cache error types
var (
	ErrKeyNotFound     = errors.New("cache: key not found")
	ErrInvalidTTL      = errors.New("cache: invalid TTL")
	ErrSerialization   = errors.New("cache: serialization error")
	ErrDeserialization = errors.New("cache: deserialization error")
	ErrInvalidKey      = errors.New("cache: invalid key")
	ErrCacheFull       = errors.New("cache: cache is full")
	ErrContextCanceled     = errors.New("cache: operation canceled")
	ErrInvalidValue        = errors.New("cache: invalid value type for operation")
	ErrCircuitBreakerOpen  = errors.New("cache: circuit breaker is open")

	// Counter-specific error types
	ErrNotNumeric = errors.New("cache: value is not numeric")
	ErrOverflow   = errors.New("cache: numeric overflow")
)

// IsNotFound checks if an error represents a key not found condition
func IsNotFound(err error) bool {
	return errors.Is(err, ErrKeyNotFound)
}

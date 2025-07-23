package cache

import "errors"

// Common error types
var (
	ErrKeyNotFound     = errors.New("cache: key not found")
	ErrInvalidTTL      = errors.New("cache: invalid TTL")
	ErrSerialization   = errors.New("cache: serialization error")
	ErrDeserialization = errors.New("cache: deserialization error")
	ErrInvalidKey      = errors.New("cache: invalid key")
	ErrCacheFull       = errors.New("cache: cache is full")
	ErrContextCanceled = errors.New("cache: operation canceled")
	ErrInvalidValue    = errors.New("cache: invalid value type for operation")
)

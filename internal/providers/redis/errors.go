package redis

import (
	"errors"
)

// Common errors that may be returned by Redis cache operations
var (
	// ErrInvalidRedisOptions is returned when Redis options are invalid
	ErrInvalidRedisOptions = errors.New("invalid Redis options")

	// ErrRedisConnectionFailed is returned when connection to Redis fails
	ErrRedisConnectionFailed = errors.New("failed to connect to Redis")

	// ErrRedisCommandFailed is returned when a Redis command fails
	ErrRedisCommandFailed = errors.New("redis command failed")

	// ErrRedisScriptFailed is returned when a Redis Lua script fails
	ErrRedisScriptFailed = errors.New("redis script execution failed")
)

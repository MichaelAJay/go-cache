package testenv

import (
	"context"
	"os"
)

// newDirectEnvironment creates a test environment using existing/external services
func newDirectEnvironment(ctx context.Context) (*TestEnvironment, error) {
	// Use external Redis if available, otherwise localhost
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	env := &TestEnvironment{
		RedisAddr: redisAddr,
		Mode:      ModeDirect,
		Cleanup: func() {
			// No cleanup needed for direct mode
		},
	}

	return env, nil
}
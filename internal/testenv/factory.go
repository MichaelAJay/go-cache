package testenv

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// TestMode represents different test environment modes
type TestMode int

const (
	ModeDirect TestMode = iota
	ModeContainers
	ModeCompose
)

func (tm TestMode) String() string {
	switch tm {
	case ModeDirect:
		return "direct"
	case ModeContainers:
		return "containers"
	case ModeCompose:
		return "compose"
	default:
		return "unknown"
	}
}

// TestEnvironment represents a test environment configuration
type TestEnvironment struct {
	RedisAddr           string
	Cleanup             func()
	Mode                TestMode
	ToxiproxyController *ToxiproxyController
}

// NewTestEnvironment creates a new test environment based on the specified mode
func NewTestEnvironment(ctx context.Context, mode TestMode) (*TestEnvironment, error) {
	switch mode {
	case ModeContainers:
		return newContainerEnvironment(ctx)
	case ModeDirect:
		return newDirectEnvironment(ctx)
	case ModeCompose:
		return newComposeEnvironment(ctx)
	default:
		return nil, fmt.Errorf("unsupported test mode: %s", mode.String())
	}
}

// NewTestEnvironmentFromEnv creates a test environment using environment variable configuration
func NewTestEnvironmentFromEnv(ctx context.Context) (*TestEnvironment, error) {
	mode := GetTestModeFromEnv()
	return NewTestEnvironment(ctx, mode)
}

// GetTestModeFromEnv determines the test mode from environment variables
func GetTestModeFromEnv() TestMode {
	mode := os.Getenv("GOCACHE_TEST_MODE")
	switch mode {
	case "containers":
		return ModeContainers
	case "compose":
		return ModeCompose
	default:
		return ModeDirect
	}
}

// GetContainerTimeoutFromEnv gets container startup timeout from environment
func GetContainerTimeoutFromEnv() time.Duration {
	timeoutStr := os.Getenv("GOCACHE_TEST_CONTAINER_TIMEOUT")
	if timeoutStr != "" {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			return timeout
		}
	}
	return 60 * time.Second // Default timeout
}

// Close cleans up the test environment
func (te *TestEnvironment) Close() {
	if te.Cleanup != nil {
		te.Cleanup()
	}
}

// GetRedisAddr returns the Redis connection address for this environment
func (te *TestEnvironment) GetRedisAddr() string {
	return te.RedisAddr
}

// GetMode returns the test mode for this environment
func (te *TestEnvironment) GetMode() TestMode {
	return te.Mode
}

// GetToxiproxyController returns the toxiproxy controller for this environment (may be nil)
func (te *TestEnvironment) GetToxiproxyController() *ToxiproxyController {
	return te.ToxiproxyController
}

// HasToxiproxy returns true if this environment has toxiproxy available
func (te *TestEnvironment) HasToxiproxy() bool {
	return te.ToxiproxyController != nil
}

// ResetForNewBenchmark clears Redis state and reconfigures toxiproxy for a new benchmark
func (te *TestEnvironment) ResetForNewBenchmark(ctx context.Context, latencyMs int) error {
	// First, get a Redis client to flush the database
	// Note: This is a simple approach - in production you might want to inject the client
	client := redis.NewClient(&redis.Options{
		Addr: te.GetRedisAddr(),
	})
	defer client.Close()

	// Clear all Redis data for clean state
	if err := client.FlushAll(ctx).Err(); err != nil {
		return fmt.Errorf("failed to flush Redis: %w", err)
	}

	// Reset toxiproxy latency if toxiproxy is available
	if te.HasToxiproxy() && latencyMs > 0 {
		if err := te.ToxiproxyController.ResetLatency(ctx, "redis_proxy", latencyMs); err != nil {
			return fmt.Errorf("failed to reset toxiproxy latency: %w", err)
		}
	}

	return nil
}

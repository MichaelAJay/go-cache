package testenv

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

// newContainerEnvironment creates a test environment using testcontainers
func newContainerEnvironment(ctx context.Context) (*TestEnvironment, error) {
	timeout := GetContainerTimeoutFromEnv()
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Start Redis container
	redisContainer, err := redis.RunContainer(ctxWithTimeout,
		testcontainers.WithImage("redis:8.0.3"),
		redis.WithSnapshotting(10, 1),
		redis.WithLogLevel(redis.LogLevelVerbose),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start Redis container: %w", err)
	}

	// Get Redis connection details (not used directly, but kept for reference)
	_, err = redisContainer.ConnectionString(ctxWithTimeout)
	if err != nil {
		redisContainer.Terminate(ctxWithTimeout)
		return nil, fmt.Errorf("failed to get Redis connection string: %w", err)
	}

	// Convert Redis URL to host:port format
	host, err := redisContainer.Host(ctxWithTimeout)
	if err != nil {
		redisContainer.Terminate(ctxWithTimeout)
		return nil, fmt.Errorf("failed to get Redis host: %w", err)
	}

	port, err := redisContainer.MappedPort(ctxWithTimeout, "6379")
	if err != nil {
		redisContainer.Terminate(ctxWithTimeout)
		return nil, fmt.Errorf("failed to get Redis port: %w", err)
	}

	redisHostPort := fmt.Sprintf("%s:%s", host, port.Port())

	// Start Toxiproxy container if latency testing is enabled
	var toxiController *ToxiproxyController
	if isLatencyTestingEnabled() {
		toxiContainer, err := testcontainers.GenericContainer(ctxWithTimeout, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        "shopify/toxiproxy:2.9.0",
				ExposedPorts: []string{"8474/tcp", "8080/tcp"},
				WaitingFor:   wait.ForHTTP("/version"),
			},
			Started: true,
		})
		if err != nil {
			redisContainer.Terminate(ctxWithTimeout)
			return nil, fmt.Errorf("failed to start Toxiproxy container: %w", err)
		}

		// Get Toxiproxy API endpoint
		toxiHost, err := toxiContainer.Host(ctxWithTimeout)
		if err != nil {
			redisContainer.Terminate(ctxWithTimeout)
			toxiContainer.Terminate(ctxWithTimeout)
			return nil, fmt.Errorf("failed to get Toxiproxy host: %w", err)
		}

		toxiPort, err := toxiContainer.MappedPort(ctxWithTimeout, "8474")
		if err != nil {
			redisContainer.Terminate(ctxWithTimeout)
			toxiContainer.Terminate(ctxWithTimeout)
			return nil, fmt.Errorf("failed to get Toxiproxy port: %w", err)
		}

		toxiAPI := fmt.Sprintf("http://%s:%s", toxiHost, toxiPort.Port())
		
		// Initialize toxiproxy controller
		toxiController, err = NewToxiproxyController(toxiAPI)
		if err != nil {
			redisContainer.Terminate(ctxWithTimeout)
			toxiContainer.Terminate(ctxWithTimeout)
			return nil, fmt.Errorf("failed to initialize Toxiproxy controller: %w", err)
		}

		// Setup proxies with actual Redis container details
		if err := toxiController.SetupCacheProxiesWithTarget(ctxWithTimeout, redisHostPort); err != nil {
			redisContainer.Terminate(ctxWithTimeout)
			toxiContainer.Terminate(ctxWithTimeout)
			return nil, fmt.Errorf("failed to setup Toxiproxy proxies: %w", err)
		}

		// Get proxy port for Redis
		proxyPort, err := toxiContainer.MappedPort(ctxWithTimeout, "8080")
		if err != nil {
			redisContainer.Terminate(ctxWithTimeout)
			toxiContainer.Terminate(ctxWithTimeout)
			return nil, fmt.Errorf("failed to get proxy port: %w", err)
		}

		// Use proxy for Redis connection
		redisHostPort = fmt.Sprintf("%s:%s", toxiHost, proxyPort.Port())

		// Store container for cleanup
		toxiController.container = toxiContainer
	}

	env := &TestEnvironment{
		RedisAddr:           redisHostPort,
		Mode:                ModeContainers,
		ToxiproxyController: toxiController,
		Cleanup: func() {
			// Cleanup with a reasonable timeout
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cleanupCancel()

			if toxiController != nil && toxiController.container != nil {
				toxiController.container.Terminate(cleanupCtx)
			}
			redisContainer.Terminate(cleanupCtx)
		},
	}

	return env, nil
}
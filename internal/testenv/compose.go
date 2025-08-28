package testenv

import (
	"context"
)

// newComposeEnvironment creates a test environment using docker-compose services
func newComposeEnvironment(ctx context.Context) (*TestEnvironment, error) {
	// Use docker-compose services
	redisAddr := "localhost:6379"

	// Setup toxiproxy if latency testing is enabled
	var toxiController *ToxiproxyController
	var err error
	
	if isLatencyTestingEnabled() {
		toxiController, err = NewToxiproxyController("http://localhost:8474")
		if err != nil {
			return nil, err
		}
		
		// Initialize proxies for cache testing
		if err := toxiController.SetupCacheProxies(ctx); err != nil {
			return nil, err
		}
		
		// If using proxies, redirect Redis connection through toxiproxy
		redisAddr = "localhost:8080" // Redis proxy port
	}

	env := &TestEnvironment{
		RedisAddr:           redisAddr,
		Mode:                ModeCompose,
		ToxiproxyController: toxiController,
		Cleanup: func() {
			if toxiController != nil {
				toxiController.Cleanup(context.Background())
			}
		},
	}

	return env, nil
}

// isLatencyTestingEnabled checks if latency testing is enabled
func isLatencyTestingEnabled() bool {
	validator := &ServiceValidator{}
	return validator.isLatencyModeEnabled()
}
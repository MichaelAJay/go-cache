package testenv

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// ServiceValidator validates that required services are running and accessible
type ServiceValidator struct {
	RedisAddr    string
	ToxiproxyAPI string
}

// ValidateServices validates services based on the test mode
func (sv *ServiceValidator) ValidateServices(ctx context.Context, mode TestMode) error {
	switch mode {
	case ModeCompose:
		return sv.validateComposeServices(ctx)
	case ModeDirect:
		return sv.validateDirectServices(ctx)
	default:
		return nil // Skip validation for container mode
	}
}

// validateComposeServices validates docker-compose services are running
func (sv *ServiceValidator) validateComposeServices(ctx context.Context) error {
	// 1. Check Redis connection
	if err := sv.validateRedis(ctx); err != nil {
		return fmt.Errorf("Redis validation failed - ensure 'docker-compose up' is running: %w", err)
	}

	// 2. Check Toxiproxy API (if latency mode enabled)
	if sv.isLatencyModeEnabled() {
		if err := sv.validateToxiproxy(ctx); err != nil {
			return fmt.Errorf("Toxiproxy validation failed - ensure proxy setup completed: %w", err)
		}
	}

	return nil
}

// validateDirectServices validates external services are accessible
func (sv *ServiceValidator) validateDirectServices(ctx context.Context) error {
	// Validate that external Redis is available at configured address
	if err := sv.validateRedis(ctx); err != nil {
		return fmt.Errorf("external Redis validation failed - check REDIS_ADDR: %w", err)
	}
	return nil
}

// validateRedis tests Redis connection
func (sv *ServiceValidator) validateRedis(ctx context.Context) error {
	// Create Redis client with timeout
	rdb := redis.NewClient(&redis.Options{
		Addr:         sv.RedisAddr,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	defer rdb.Close()

	// Test basic operation with context
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("Redis PING failed: %w", err)
	}

	return nil
}

// ValidateToxiproxy tests Toxiproxy API and required proxies (exported for scripts)
func (sv *ServiceValidator) ValidateToxiproxy(ctx context.Context) error {
	return sv.validateToxiproxy(ctx)
}

// validateToxiproxy tests Toxiproxy API and required proxies
func (sv *ServiceValidator) validateToxiproxy(ctx context.Context) error {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// 1. Check API availability
	req, err := http.NewRequestWithContext(ctx, "GET", sv.ToxiproxyAPI+"/proxies", nil)
	if err != nil {
		return fmt.Errorf("failed to create Toxiproxy request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Toxiproxy API not accessible: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("Toxiproxy API returned status %d", resp.StatusCode)
	}

	// 2. Verify required proxies exist
	requiredProxies := []string{"redis_proxy"}
	for _, proxy := range requiredProxies {
		if err := sv.checkProxyExists(ctx, client, proxy); err != nil {
			return fmt.Errorf("proxy %s not configured: %w", proxy, err)
		}
	}

	return nil
}

// checkProxyExists verifies a specific proxy is configured
func (sv *ServiceValidator) checkProxyExists(ctx context.Context, client *http.Client, proxyName string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", sv.ToxiproxyAPI+"/proxies/"+proxyName, nil)
	if err != nil {
		return fmt.Errorf("failed to create proxy check request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to check proxy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("proxy not found")
	} else if resp.StatusCode != 200 {
		return fmt.Errorf("proxy check returned status %d", resp.StatusCode)
	}

	return nil
}

// isLatencyModeEnabled checks if latency testing is enabled
func (sv *ServiceValidator) isLatencyModeEnabled() bool {
	// Check explicit latency flags
	if val := os.Getenv("GOCACHE_TEST_LATENCY"); val == "enabled" || val == "true" {
		return true
	}
	if val := os.Getenv("GOCACHE_TEST_TOXIPROXY"); val == "enabled" || val == "true" {
		return true
	}
	
	// For preset modes, only enable latency for specific presets
	if preset := os.Getenv("GOCACHE_TEST_PRESET"); preset != "" {
		switch preset {
		case "realistic", "production":
			// These presets may use latency, but not necessarily
			// Only enable if explicitly requested
			return os.Getenv("GOCACHE_TEST_LATENCY") == "enabled"
		default:
			return false
		}
	}

	return false
}

// NewServiceValidator creates a new service validator with default values
func NewServiceValidator(mode TestMode) *ServiceValidator {
	return &ServiceValidator{
		RedisAddr:    getRedisAddrForMode(mode),
		ToxiproxyAPI: "http://localhost:8474",
	}
}

// getRedisAddrForMode returns the appropriate Redis address for the mode
func getRedisAddrForMode(mode TestMode) string {
	switch mode {
	case ModeCompose:
		return "localhost:6379"
	case ModeDirect:
		if addr := os.Getenv("REDIS_ADDR"); addr != "" {
			return addr
		}
		return "localhost:6379"
	default:
		return ""
	}
}
package testenv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

// ToxiproxyController manages Toxiproxy for latency simulation
type ToxiproxyController struct {
	apiURL    string
	client    *http.Client
	container testcontainers.Container // For container cleanup
}

// ProxyConfig represents a Toxiproxy proxy configuration
type ProxyConfig struct {
	Name     string `json:"name"`
	Listen   string `json:"listen"`
	Upstream string `json:"upstream"`
	Enabled  bool   `json:"enabled"`
}

// ToxicConfig represents a Toxiproxy toxic configuration
type ToxicConfig struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Stream     string                 `json:"stream"`
	Toxicity   float64                `json:"toxicity"`
	Attributes map[string]interface{} `json:"attributes"`
}

// NewToxiproxyController creates a new Toxiproxy controller
func NewToxiproxyController(apiURL string) (*ToxiproxyController, error) {
	return &ToxiproxyController{
		apiURL: apiURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// SetupCacheProxies sets up proxies for cache testing with default Redis target
func (tc *ToxiproxyController) SetupCacheProxies(ctx context.Context) error {
	return tc.SetupCacheProxiesWithTarget(ctx, "redis:6379")
}

// SetupCacheProxiesWithTarget sets up proxies for cache testing with specified Redis target
func (tc *ToxiproxyController) SetupCacheProxiesWithTarget(ctx context.Context, redisTarget string) error {
	// Create Redis proxy
	redisProxy := ProxyConfig{
		Name:     "redis_proxy",
		Listen:   "0.0.0.0:8080",
		Upstream: redisTarget,
		Enabled:  true,
	}

	if err := tc.createProxy(ctx, redisProxy); err != nil {
		return fmt.Errorf("failed to create Redis proxy: %w", err)
	}

	// Apply latency configuration if specified
	if err := tc.applyLatencyFromEnv(ctx); err != nil {
		return fmt.Errorf("failed to apply latency configuration: %w", err)
	}

	return nil
}

// createProxy creates a new proxy
func (tc *ToxiproxyController) createProxy(ctx context.Context, config ProxyConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal proxy config: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tc.apiURL+"/proxies", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := tc.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create proxy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("failed to create proxy, status: %d", resp.StatusCode)
	}

	return nil
}

// applyLatencyFromEnv applies latency configuration from environment variables
func (tc *ToxiproxyController) applyLatencyFromEnv(ctx context.Context) error {
	// Check for Redis latency configuration
	if latencyStr := os.Getenv("GOCACHE_TEST_REDIS_LATENCY_MS"); latencyStr != "" {
		latencyMs, err := strconv.Atoi(latencyStr)
		if err != nil {
			return fmt.Errorf("invalid Redis latency value: %w", err)
		}
		
		if err := tc.AddLatencyToxic(ctx, "redis_proxy", latencyMs); err != nil {
			return fmt.Errorf("failed to add Redis latency toxic: %w", err)
		}
	}

	return nil
}

// AddLatencyToxic adds a latency toxic to a proxy
func (tc *ToxiproxyController) AddLatencyToxic(ctx context.Context, proxyName string, latencyMs int) error {
	toxic := ToxicConfig{
		Name:     fmt.Sprintf("%s_latency", proxyName),
		Type:     "latency",
		Stream:   "downstream",
		Toxicity: 1.0,
		Attributes: map[string]interface{}{
			"latency": latencyMs,
		},
	}

	return tc.addToxic(ctx, proxyName, toxic)
}

// addToxic adds a toxic to a proxy
func (tc *ToxiproxyController) addToxic(ctx context.Context, proxyName string, toxic ToxicConfig) error {
	data, err := json.Marshal(toxic)
	if err != nil {
		return fmt.Errorf("failed to marshal toxic config: %w", err)
	}

	url := fmt.Sprintf("%s/proxies/%s/toxics", tc.apiURL, proxyName)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := tc.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to add toxic: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to add toxic, status: %d", resp.StatusCode)
	}

	return nil
}

// RemoveToxic removes a toxic from a proxy
func (tc *ToxiproxyController) RemoveToxic(ctx context.Context, proxyName, toxicName string) error {
	url := fmt.Sprintf("%s/proxies/%s/toxics/%s", tc.apiURL, proxyName, toxicName)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := tc.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to remove toxic: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to remove toxic, status: %d", resp.StatusCode)
	}

	return nil
}

// Cleanup removes all proxies and toxics
func (tc *ToxiproxyController) Cleanup(ctx context.Context) error {
	// List all proxies
	req, err := http.NewRequestWithContext(ctx, "GET", tc.apiURL+"/proxies", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := tc.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to list proxies: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to list proxies, status: %d", resp.StatusCode)
	}

	var proxies map[string]ProxyConfig
	if err := json.NewDecoder(resp.Body).Decode(&proxies); err != nil {
		return fmt.Errorf("failed to decode proxies: %w", err)
	}

	// Delete each proxy
	for name := range proxies {
		deleteURL := fmt.Sprintf("%s/proxies/%s", tc.apiURL, name)
		deleteReq, err := http.NewRequestWithContext(ctx, "DELETE", deleteURL, nil)
		if err != nil {
			continue // Best effort cleanup
		}

		deleteResp, err := tc.client.Do(deleteReq)
		if err != nil {
			continue // Best effort cleanup
		}
		deleteResp.Body.Close()
	}

	return nil
}
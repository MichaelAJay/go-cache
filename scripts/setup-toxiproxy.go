//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/MichaelAJay/go-cache/internal/testenv"
)

// setup-toxiproxy.go - Setup toxiproxy proxies for latency testing
func main() {
	ctx := context.Background()

	controller, err := testenv.NewToxiproxyController("http://localhost:8474")
	if err != nil {
		fmt.Printf("❌ Failed to create Toxiproxy controller: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("🧪 Setting up toxiproxy proxies for cache testing...")

	// Setup cache proxies (Redis)
	if err := controller.SetupCacheProxies(ctx); err != nil {
		fmt.Printf("❌ Failed to setup cache proxies: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Toxiproxy proxies configured successfully")
	fmt.Println("   - Redis proxy: localhost:8080 -> redis:6379")
	
	// Show latency configuration if applied
	if latency := os.Getenv("GOCACHE_TEST_REDIS_LATENCY_MS"); latency != "" {
		fmt.Printf("   - Redis latency: %sms\n", latency)
	}
}
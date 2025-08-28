//go:build ignore
// +build ignore

// This is a standalone demo of the container testing functionality
// It doesn't depend on the go-cache interfaces, just demonstrates Redis connectivity
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/MichaelAJay/go-cache/internal/testenv"
	"github.com/redis/go-redis/v9"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-h" {
		fmt.Println("Container Testing Demo")
		fmt.Println("=====================")
		fmt.Println()
		fmt.Println("Environment Variables:")
		fmt.Println("  GOCACHE_TEST_MODE=containers    - Use testcontainers")
		fmt.Println("  GOCACHE_TEST_MODE=compose       - Use docker-compose")
		fmt.Println("  GOCACHE_TEST_MODE=direct        - Use existing Redis")
		fmt.Println("  GOCACHE_TEST_LATENCY=enabled    - Enable toxiproxy latency")
		fmt.Println("  GOCACHE_TEST_REDIS_LATENCY_MS=50 - Set latency in ms")
		fmt.Println()
		fmt.Println("Usage examples:")
		fmt.Println("  make test-containers              # Cold start containers")
		fmt.Println("  make docker-up && make docker-test # Use docker-compose")
		fmt.Println("  go run demo_container_test.go     # Direct mode")
		return
	}

	ctx := context.Background()

	fmt.Println("🧪 Go-Cache Container Testing Demo")
	fmt.Println("==================================")

	// Create test environment based on environment variables
	mode := testenv.GetTestModeFromEnv()
	fmt.Printf("📋 Test Mode: %s\n", mode.String())

	testEnv, err := testenv.NewTestEnvironmentFromEnv(ctx)
	if err != nil {
		log.Fatalf("❌ Failed to create test environment: %v", err)
	}
	defer testEnv.Close()

	fmt.Printf("🔗 Redis Address: %s\n", testEnv.GetRedisAddr())
	if testEnv.HasToxiproxy() {
		fmt.Println("🐌 Toxiproxy: Available for latency testing")
	}

	// Test Redis operations
	fmt.Println("\n🔧 Testing Redis Operations...")
	rdb := redis.NewClient(&redis.Options{
		Addr: testEnv.GetRedisAddr(),
	})
	defer rdb.Close()

	// Basic connectivity test
	start := time.Now()
	pong, err := rdb.Ping(ctx).Result()
	pingDuration := time.Since(start)
	if err != nil {
		log.Fatalf("❌ Redis PING failed: %v", err)
	}
	fmt.Printf("✅ Redis PING: %s (%v)\n", pong, pingDuration)

	// Set operation
	start = time.Now()
	err = rdb.Set(ctx, "demo:test", "Hello from container testing!", time.Minute).Err()
	setDuration := time.Since(start)
	if err != nil {
		log.Fatalf("❌ Redis SET failed: %v", err)
	}
	fmt.Printf("✅ Redis SET: success (%v)\n", setDuration)

	// Get operation  
	start = time.Now()
	value, err := rdb.Get(ctx, "demo:test").Result()
	getDuration := time.Since(start)
	if err != nil {
		log.Fatalf("❌ Redis GET failed: %v", err)
	}
	fmt.Printf("✅ Redis GET: %s (%v)\n", value, getDuration)

	// Test toxiproxy latency if available
	if testEnv.HasToxiproxy() {
		fmt.Println("\n🐌 Testing Toxiproxy Latency...")
		toxiController := testEnv.GetToxiproxyController()
		
		// Add 100ms latency
		err := toxiController.AddLatencyToxic(ctx, "redis_proxy", 100)
		if err != nil {
			log.Printf("⚠️  Failed to add latency toxic: %v", err)
		} else {
			fmt.Println("✅ Added 100ms latency to Redis proxy")

			// Test operation with latency
			start = time.Now()
			err = rdb.Set(ctx, "demo:latency", "slow operation", time.Minute).Err()
			latencyDuration := time.Since(start)
			if err != nil {
				log.Printf("❌ Latency test SET failed: %v", err)
			} else {
				fmt.Printf("✅ Latency test SET: success (%v)\n", latencyDuration)
				if latencyDuration.Milliseconds() >= 90 {
					fmt.Println("✅ Latency simulation working correctly")
				} else {
					fmt.Printf("⚠️  Expected >100ms latency, got %v\n", latencyDuration)
				}
			}

			// Remove latency
			err = toxiController.RemoveToxic(ctx, "redis_proxy", "redis_proxy_latency")
			if err != nil {
				log.Printf("⚠️  Failed to remove latency toxic: %v", err)
			}
		}
	}

	// Cleanup
	err = rdb.Del(ctx, "demo:test", "demo:latency").Err()
	if err != nil {
		log.Printf("⚠️  Failed to cleanup test keys: %v", err)
	}

	fmt.Println("\n🎉 Container testing demo completed successfully!")
	fmt.Printf("📊 Performance Summary:\n")
	fmt.Printf("   PING: %v\n", pingDuration)
	fmt.Printf("   SET:  %v\n", setDuration)
	fmt.Printf("   GET:  %v\n", getDuration)
}
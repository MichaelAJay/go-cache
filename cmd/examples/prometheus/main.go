// Example demonstrating how to use go-cache with Prometheus metrics
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/internal/providers/memory"
)

func main() {
	// Create Prometheus metrics
	metrics, err := cache.NewPrometheusMetrics("example-app", "example")
	if err != nil {
		fmt.Printf("Error creating metrics: %v\n", err)
		os.Exit(1)
	}

	// Create cache with metrics
	provider := memory.NewProvider()
	c, err := provider.Create(&cache.CacheOptions{
		TTL:             time.Minute * 5,
		MaxEntries:      1000,
		MaxSize:         1024 * 1024 * 10, // 10MB
		CleanupInterval: time.Minute,
		Metrics:         metrics, // Use our Prometheus metrics
	})
	if err != nil {
		fmt.Printf("Error creating cache: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	// Start Prometheus server
	server, err := cache.StartPrometheusServer(metrics, ":9090")
	if err != nil {
		fmt.Printf("Error starting Prometheus server: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Prometheus metrics server running at http://localhost:9090/metrics")

	// Perform some cache operations to generate metrics
	ctx := context.Background()
	go func() {
		for i := 0; i < 10000; i++ {
			// Simulate some random cache operations
			key := fmt.Sprintf("key-%d", i%100)

			// Every 3rd attempt is a hit (approximately)
			if i%3 == 0 {
				// Set operation
				value := fmt.Sprintf("value-%d", i)
				if err := c.Set(ctx, key, value, time.Minute); err != nil {
					fmt.Printf("Error setting key: %v\n", err)
				}
			} else {
				// Get operation - sometimes hit, sometimes miss
				if _, found, err := c.Get(ctx, key); err != nil {
					fmt.Printf("Error getting key: %v\n", err)
				} else if found {
					// Hit
				} else {
					// Miss
				}
			}

			// Every 10th operation is a delete
			if i%10 == 0 {
				if err := c.Delete(ctx, key); err != nil {
					fmt.Printf("Error deleting key: %v\n", err)
				}
			}

			// Sleep to simulate real workload
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Wait for signal to terminate
	termChan := make(chan os.Signal, 1)
	signal.Notify(termChan, syscall.SIGINT, syscall.SIGTERM)
	<-termChan

	fmt.Println("Shutting down...")

	// Graceful server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		fmt.Printf("Prometheus server shutdown error: %v\n", err)
	}
}

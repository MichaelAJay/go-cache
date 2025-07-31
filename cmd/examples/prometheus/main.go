// Example demonstrating how to use go-cache with Prometheus metrics using go-metrics
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	gometrics "github.com/MichaelAJay/go-metrics"
	"github.com/MichaelAJay/go-metrics/metric/prometheus"
	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/internal/providers/memory"
)

func main() {
	// Create go-metrics registry
	registry := gometrics.NewRegistry()
	
	// Create Prometheus reporter
	reporter := prometheus.NewReporter(
		prometheus.WithDefaultLabels(map[string]string{
			"service": "go-cache-example",
			"version": "1.0.0",
		}),
	)

	// Create cache with go-metrics integration
	provider := memory.NewProvider()
	c, err := provider.Create(&cache.CacheOptions{
		TTL:               time.Minute * 5,
		MaxEntries:        1000,
		MaxSize:           1024 * 1024 * 10, // 10MB
		CleanupInterval:   time.Minute,
		
		// New go-metrics integration
		GoMetricsRegistry: registry,
		MetricsEnabled:    true,
		GlobalMetricsTags: gometrics.Tags{
			"provider":    "memory",
			"environment": "development",
		},
	})
	if err != nil {
		fmt.Printf("Error creating cache: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	// Start reporting metrics to Prometheus
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := reporter.Report(registry); err != nil {
				fmt.Printf("Error reporting metrics: %v\n", err)
			}
		}
	}()

	// Create HTTP server for Prometheus scraping
	http.Handle("/metrics", reporter.Handler())
	server := &http.Server{Addr: ":9090"}
	
	go func() {
		fmt.Println("Prometheus metrics server running at http://localhost:9090/metrics")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Error starting server: %v\n", err)
		}
	}()

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

	// Final metrics report
	if err := reporter.Report(registry); err != nil {
		fmt.Printf("Error reporting final metrics: %v\n", err)
	}

	// Graceful server shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("Prometheus server shutdown error: %v\n", err)
	}
	
	fmt.Println("Cache metrics summary:")
}

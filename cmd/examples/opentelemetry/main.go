// Example demonstrating how to use go-cache with OpenTelemetry metrics using go-metrics
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	gometrics "github.com/MichaelAJay/go-metrics"
	"github.com/MichaelAJay/go-metrics/metric/otel"
	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/internal/providers/memory"
)

func main() {
	// Create go-metrics registry
	registry := gometrics.NewRegistry()
	
	// Create OpenTelemetry reporter
	reporter, err := otel.NewReporter(
		"go-cache-example",  // service name
		"1.0.0",            // version
		otel.WithAttributes(map[string]string{
			"environment": "development",
			"component":   "cache",
		}),
	)
	if err != nil {
		fmt.Printf("Error creating OpenTelemetry reporter: %v\n", err)
		os.Exit(1)
	}
	defer reporter.Close()

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
			"node":        "cache-node-1",
		},
	})
	if err != nil {
		fmt.Printf("Error creating cache: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	// Start reporting metrics to OpenTelemetry
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := reporter.Report(registry); err != nil {
				fmt.Printf("Error reporting metrics: %v\n", err)
			}
		}
	}()

	fmt.Println("OpenTelemetry metrics collection started")
	fmt.Println("Performing cache operations to generate metrics...")

	// Perform some cache operations to generate metrics
	ctx := context.Background()
	go func() {
		for i := 0; i < 5000; i++ {
			// Simulate different cache patterns
			key := fmt.Sprintf("key-%d", i%50) // 50 unique keys

			switch i % 7 {
			case 0, 1: // Set operations (2/7 chance)
				value := map[string]interface{}{
					"id":        i,
					"timestamp": time.Now().Unix(),
					"data":      fmt.Sprintf("payload-%d", i),
				}
				if err := c.Set(ctx, key, value, time.Minute*2); err != nil {
					fmt.Printf("Error setting key: %v\n", err)
				}
				
			case 2, 3, 4, 5: // Get operations (4/7 chance)
				if value, found, err := c.Get(ctx, key); err != nil {
					fmt.Printf("Error getting key: %v\n", err)
				} else if found {
					// Process the value
					_ = value
				}
				
			case 6: // Delete operations (1/7 chance)
				if err := c.Delete(ctx, key); err != nil {
					fmt.Printf("Error deleting key: %v\n", err)
				}
			}

			// Occasional batch operations
			if i%100 == 0 && i > 0 {
				// Batch set
				items := make(map[string]any)
				for j := 0; j < 5; j++ {
					batchKey := fmt.Sprintf("batch-%d-%d", i, j)
					items[batchKey] = fmt.Sprintf("batch-value-%d", j)
				}
				if err := c.SetMany(ctx, items, time.Minute); err != nil {
					fmt.Printf("Error setting batch: %v\n", err)
				}
			}

			// Simulate realistic operation spacing
			time.Sleep(20 * time.Millisecond)
		}
		fmt.Println("Cache operations completed")
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
	
	// Allow time for final metrics to be sent
	time.Sleep(2 * time.Second)
	
	fmt.Println("Cache metrics summary:")
}
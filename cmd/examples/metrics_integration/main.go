package main

import (
	"context"
	"fmt"
	"time"

	"github.com/MichaelAJay/go-metrics/metric"
	"github.com/MichaelAJay/go-cache"
)

func main() {
	// Create go-metrics registry
	registry := metric.NewDefaultRegistry()
	
	// Create cache with metrics
	provider := cache.NewMemoryProvider()
	c, err := provider.Create(&cache.CacheOptions{
		TTL:               time.Minute * 5,
		MaxEntries:        1000,
		CleanupInterval:   time.Minute,
		GoMetricsRegistry: registry,
		MetricsEnabled:    true,
		GlobalMetricsTags: metric.Tags{
			"provider": "memory",
			"test":     "integration",
		},
	})
	if err != nil {
		fmt.Printf("Error creating cache: %v\n", err)
		return
	}
	defer c.Close()

	ctx := context.Background()
	
	// Test basic operations
	fmt.Println("Testing metrics integration...")
	
	// Set some values
	err = c.Set(ctx, "key1", "value1", time.Minute)
	if err != nil {
		fmt.Printf("Error setting key1: %v\n", err)
		return
	}
	
	// Get values (hit and miss)
	val, found, err := c.Get(ctx, "key1")
	if err != nil {
		fmt.Printf("Error getting key1: %v\n", err)
		return
	}
	fmt.Printf("Got key1: %v (found: %v)\n", val, found)
	
	// Try to get non-existent key (miss)
	_, found, err = c.Get(ctx, "nonexistent")
	if err != nil {
		fmt.Printf("Error getting nonexistent: %v\n", err)
		return
	}
	fmt.Printf("Got nonexistent key (found: %v)\n", found)
	
	fmt.Println("Metrics integration test completed successfully!")
}
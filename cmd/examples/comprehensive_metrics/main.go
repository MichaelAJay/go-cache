// Comprehensive example demonstrating advanced go-cache metrics features
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MichaelAJay/go-metrics/metric"
	"github.com/MichaelAJay/go-metrics/metric/prometheus"
	"github.com/MichaelAJay/go-cache"
)

func main() {
	fmt.Println("=== Go-Cache Comprehensive Metrics Example ===")
	
	// Create go-metrics registry
	registry := metric.NewDefaultRegistry()
	
	// Create Prometheus reporter with custom labels
	reporter := prometheus.NewReporter(
		prometheus.WithDefaultLabels(map[string]string{
			"service":     "comprehensive-cache-demo",
			"version":     "2.0.0",
			"environment": "demo",
		}),
	)

	// Create cache with comprehensive metrics configuration
	provider := cache.NewMemoryProvider()
	c, err := provider.Create(&cache.CacheOptions{
		TTL:               time.Minute * 10,
		MaxEntries:        500,
		MaxSize:           1024 * 1024 * 5, // 5MB
		CleanupInterval:   time.Second * 30,
		
		// Advanced go-metrics integration
		GoMetricsRegistry: registry,
		MetricsEnabled:    true,
		GlobalMetricsTags: metric.Tags{
			"provider":      "memory",
			"node_id":       "cache-node-primary",
			"datacenter":    "us-east-1",
			"cluster":       "cache-cluster-1",
		},
		
		// Enhanced security configuration
		Security: &cache.SecurityConfig{
			EnableTimingProtection: true,
			MinProcessingTime:      5 * time.Millisecond,
			SecureCleanup:         true,
		},
		
		// Secondary indexing for advanced queries
		Indexes: map[string]string{
			"user_sessions": "session:*",
			"user_profiles": "profile:*",
		},
		
		// Detailed metrics configuration
		MetricsConfig: &cache.MetricsConfig{
			EnableDetailedMetrics:   true,
			EnableSecurityMetrics:   true,
			EnableLatencyHistograms: true,
			MetricsPrefix:          "demo_cache_",
		},
	})
	if err != nil {
		fmt.Printf("Error creating cache: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	// Start Prometheus metrics server
	http.Handle("/metrics", reporter.Handler())
	server := &http.Server{Addr: ":8080"}
	
	go func() {
		fmt.Println("📊 Prometheus metrics available at: http://localhost:8080/metrics")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Error starting metrics server: %v\n", err)
		}
	}()

	// Start periodic metrics reporting
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := reporter.Report(registry); err != nil {
				fmt.Printf("Error reporting metrics: %v\n", err)
			}
		}
	}()

	fmt.Println("🚀 Starting comprehensive cache operations...")
	
	// Demonstrate various cache operations with metrics
	ctx := context.Background()
	operationsChan := make(chan string, 100)
	
	// User session simulation
	go simulateUserSessions(ctx, c, operationsChan)
	
	// User profile operations
	go simulateUserProfiles(ctx, c, operationsChan)
	
	// Background cache maintenance
	go simulateCacheMaintenance(ctx, c, operationsChan)
	
	// Batch operations demo
	go simulateBatchOperations(ctx, c, operationsChan)
	
	// Index operations demo
	go simulateIndexOperations(ctx, c, operationsChan)

	// Operation logger
	go func() {
		for op := range operationsChan {
			fmt.Printf("📝 %s\n", op)
		}
	}()

	// Display live metrics every 10 seconds
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			displayLiveMetrics(c)
		}
	}()

	// Wait for termination signal
	fmt.Println("⏳ Running demo... Press Ctrl+C to stop")
	termChan := make(chan os.Signal, 1)
	signal.Notify(termChan, syscall.SIGINT, syscall.SIGTERM)
	<-termChan

	fmt.Println("\n🛑 Shutting down...")
	close(operationsChan)

	// Final metrics report
	if err := reporter.Report(registry); err != nil {
		fmt.Printf("Error reporting final metrics: %v\n", err)
	}

	// Graceful server shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("Server shutdown error: %v\n", err)
	}

	// Display final metrics summary
	displayFinalSummary(c)
}

func simulateUserSessions(ctx context.Context, c cache.Cache, ops chan<- string) {
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		
		sessionID := fmt.Sprintf("session:%d", i%20)
		
		switch i % 4 {
		case 0: // Create session
			session := map[string]interface{}{
				"user_id":    i % 10,
				"created_at": time.Now().Unix(),
				"ip_address": fmt.Sprintf("192.168.1.%d", i%255),
				"user_agent": "demo-client/1.0",
			}
			c.Set(ctx, sessionID, session, time.Minute*30)
			ops <- fmt.Sprintf("Created session %s", sessionID)
			
		case 1, 2: // Read session
			if _, found, _ := c.Get(ctx, sessionID); found {
				ops <- fmt.Sprintf("Retrieved session %s", sessionID)
			} else {
				ops <- fmt.Sprintf("Session %s not found", sessionID)
			}
			
		case 3: // Delete expired session
			c.Delete(ctx, sessionID)
			ops <- fmt.Sprintf("Deleted session %s", sessionID)
		}
		
		time.Sleep(100 * time.Millisecond)
	}
}

func simulateUserProfiles(ctx context.Context, c cache.Cache, ops chan<- string) {
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		
		profileID := fmt.Sprintf("profile:%d", i%15)
		
		switch i % 5 {
		case 0: // Create/update profile
			profile := map[string]interface{}{
				"user_id":      i % 15,
				"name":         fmt.Sprintf("User %d", i%15),
				"email":        fmt.Sprintf("user%d@example.com", i%15),
				"last_login":   time.Now().Unix(),
				"preferences":  map[string]bool{"notifications": true},
			}
			c.Set(ctx, profileID, profile, time.Hour*24)
			ops <- fmt.Sprintf("Updated profile %s", profileID)
			
		default: // Read profile
			if _, found, _ := c.Get(ctx, profileID); found {
				ops <- fmt.Sprintf("Retrieved profile %s", profileID)
			}
		}
		
		time.Sleep(150 * time.Millisecond)
	}
}

func simulateCacheMaintenance(ctx context.Context, c cache.Cache, ops chan<- string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Simulate cache maintenance operations
			keys := c.GetKeys(ctx)
			ops <- fmt.Sprintf("Cache maintenance: %d keys in cache", len(keys))
		}
	}
}

func simulateBatchOperations(ctx context.Context, c cache.Cache, ops chan<- string) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Batch set operation
			items := make(map[string]any)
			for j := 0; j < 10; j++ {
				key := fmt.Sprintf("batch:%d:%d", i, j)
				items[key] = fmt.Sprintf("batch-data-%d-%d", i, j)
			}
			c.SetMany(ctx, items, time.Minute*5)
			ops <- fmt.Sprintf("Batch set: %d items", len(items))
			
			// Batch get operation after a delay
			time.Sleep(2 * time.Second)
			keys := make([]string, 0, len(items))
			for key := range items {
				keys = append(keys, key)
			}
			results, _ := c.GetMany(ctx, keys)
			ops <- fmt.Sprintf("Batch get: retrieved %d items", len(results))
		}
	}
}

func simulateIndexOperations(ctx context.Context, c cache.Cache, ops chan<- string) {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Add some indexed entries
			userID := fmt.Sprintf("user:%d", i%5)
			for j := 0; j < 3; j++ {
				sessionKey := fmt.Sprintf("session:%d:%d", i, j)
				c.Set(ctx, sessionKey, fmt.Sprintf("session-data-%d", j), time.Minute*15)
				c.AddIndex(ctx, "user_sessions", sessionKey, userID)
			}
			
			// Query by index
			keys, _ := c.GetByIndex(ctx, "user_sessions", userID)
			ops <- fmt.Sprintf("Index query: found %d sessions for %s", len(keys), userID)
		}
	}
}

func displayLiveMetrics(c cache.Cache) {
	fmt.Printf("\n📈 Live Metrics Summary:\n")
	fmt.Printf("   Cache operations completed\n")
}

func displayFinalSummary(c cache.Cache) {
	fmt.Println("\n📊 Final Metrics Summary:")
	fmt.Println("====================================================")
	
	fmt.Printf("Total Operations:\n")
	fmt.Printf("\nCache State:\n")
	fmt.Printf("\nPerformance:\n")
	
	fmt.Println("\n✅ Demo completed successfully!")
	fmt.Println("Check the Prometheus metrics at http://localhost:8080/metrics for detailed metrics")
}
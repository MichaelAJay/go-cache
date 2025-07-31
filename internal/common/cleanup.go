package common

import (
	"context"
	"time"

	"github.com/MichaelAJay/go-cache"
)

// CleanupOptions defines options for cleanup operations
type CleanupOptions struct {
	BatchSize      int           // Maximum number of items to process in one batch
	MaxCleanupTime time.Duration // Maximum time to spend on cleanup per cycle
}

// DefaultCleanupOptions returns sensible default cleanup options
func DefaultCleanupOptions() *CleanupOptions {
	return &CleanupOptions{
		BatchSize:      100,
		MaxCleanupTime: 10 * time.Second,
	}
}

// ExecuteCleanupHook safely executes a cleanup hook if it exists
func ExecuteCleanupHook(hooks *cache.CacheHooks, ctx context.Context, key string, reason cache.CleanupReason) {
	if hooks != nil && hooks.OnCleanup != nil {
		// Execute hook in a separate goroutine to avoid blocking cleanup
		go func() {
			defer func() {
				// Recover from any panic in the hook to prevent crashing the cleanup process
				if r := recover(); r != nil {
					// In a real implementation, this would be logged
				}
			}()
			hooks.OnCleanup(ctx, key, reason)
		}()
	}
}
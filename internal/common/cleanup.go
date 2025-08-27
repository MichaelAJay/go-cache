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

// DefaultCleanupOptions returns sensible default cleanup options.
// These values balance cleanup effectiveness with system performance:
// - BatchSize: 100 items per cleanup cycle prevents long blocking operations
// - MaxCleanupTime: 10 seconds prevents cleanup from consuming too many resources
func DefaultCleanupOptions() *CleanupOptions {
	return &CleanupOptions{
		BatchSize:      100,
		MaxCleanupTime: 10 * time.Second,
	}
}

// ExecuteCleanupHook safely executes a cleanup hook if it exists.
// 
// Safety Implementation:
// 1. Executes the hook in a separate goroutine to prevent blocking cleanup operations
// 2. Uses defer-recover pattern to handle panics in user-provided hooks
// 3. Prevents hook failures from disrupting the cleanup process
// 4. Ensures cleanup continues even if hooks fail or panic
//
// This is critical for system stability since cleanup operations must complete
// to prevent memory leaks and maintain cache performance.
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
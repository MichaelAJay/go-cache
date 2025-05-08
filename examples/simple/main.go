package main

import (
	"context"
	"fmt"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/providers/memory"
	"github.com/MichaelAJay/go-logger"
)

func main() {
	// Create a logger
	log := logger.New(logger.Config{
		Level: logger.DebugLevel,
	})

	// Create a cache manager
	manager := cache.NewCacheManager()

	// Register the memory cache provider
	manager.RegisterProvider("memory", memory.NewProvider())

	// Create a cache instance with options
	cache, err := manager.GetCache("memory",
		cache.WithTTL(5*time.Minute),
		cache.WithMaxEntries(1000),
		cache.WithCleanupInterval(1*time.Minute),
		cache.WithLogger(log),
	)
	if err != nil {
		log.Fatal("Failed to create cache", logger.Field{Key: "error", Value: err})
	}

	// Create a context
	ctx := context.Background()

	// Set a value
	err = cache.Set(ctx, "key1", "value1", 0)
	if err != nil {
		log.Error("Failed to set value", logger.Field{Key: "error", Value: err})
	}

	// Get a value
	value, exists, err := cache.Get(ctx, "key1")
	if err != nil {
		log.Error("Failed to get value", logger.Field{Key: "error", Value: err})
	}
	if exists {
		fmt.Printf("Value: %v\n", value)
	}

	// Set multiple values
	items := map[string]any{
		"key2": "value2",
		"key3": "value3",
	}
	err = cache.SetMany(ctx, items, 0)
	if err != nil {
		log.Error("Failed to set multiple values", logger.Field{Key: "error", Value: err})
	}

	// Get multiple values
	values, err := cache.GetMany(ctx, []string{"key2", "key3"})
	if err != nil {
		log.Error("Failed to get multiple values", logger.Field{Key: "error", Value: err})
	}
	fmt.Printf("Values: %v\n", values)

	// Get metadata
	metadata, err := cache.GetMetadata(ctx, "key1")
	if err != nil {
		log.Error("Failed to get metadata", logger.Field{Key: "error", Value: err})
	}
	fmt.Printf("Metadata: %+v\n", metadata)

	// Close the cache
	err = cache.Close()
	if err != nil {
		log.Error("Failed to close cache", logger.Field{Key: "error", Value: err})
	}
}

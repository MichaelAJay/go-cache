package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

func main() {
	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "",
		DB:       0,
	})

	// Create context
	ctx := context.Background()

	// Test connection
	pong, err := client.Ping(ctx).Result()
	if err != nil {
		fmt.Printf("Error connecting to Redis: %v\n", err)
		return
	}
	fmt.Printf("Connected to Redis: %s\n", pong)

	// Clean up existing keys to start fresh
	client.Del(ctx, "pipeline_counter", "tx_counter")

	// Example 1: Using a pipeline
	fmt.Println("\n=== PIPELINE EXAMPLE ===")
	pipelineExample(client, ctx)

	// Example 2: Using a transaction
	fmt.Println("\n=== TRANSACTION EXAMPLE ===")
	transactionExample(client, ctx)

	// Close connection
	if err := client.Close(); err != nil {
		fmt.Printf("Error closing connection: %v\n", err)
	}
}

// pipelineExample demonstrates using Redis pipelines for batching commands
func pipelineExample(client *redis.Client, ctx context.Context) {
	// A pipeline batches multiple commands and sends them in one go,
	// reducing network round trips
	pipe := client.Pipeline()

	// Queue commands in the pipeline (these are not sent to Redis yet)
	incr := pipe.Incr(ctx, "pipeline_counter")
	pipe.Expire(ctx, "pipeline_counter", time.Hour)

	// Add 10 SET commands to the pipeline
	for i := 0; i < 10; i++ {
		pipe.Set(ctx, fmt.Sprintf("pipeline_key_%d", i), fmt.Sprintf("value_%d", i), time.Minute)
	}

	// Execute all commands in the pipeline at once
	_, err := pipe.Exec(ctx)
	if err != nil {
		fmt.Printf("Pipeline error: %v\n", err)
		return
	}

	// Get the result of the increment command
	fmt.Printf("Pipeline counter value: %d\n", incr.Val())

	// Get all the keys we created
	keys, err := client.Keys(ctx, "pipeline_key_*").Result()
	if err != nil {
		fmt.Printf("Error getting keys: %v\n", err)
		return
	}
	fmt.Printf("Created %d keys with pipeline\n", len(keys))

	// Get all values using another pipeline
	valuePipe := client.Pipeline()

	cmds := make([]*redis.StringCmd, len(keys))
	for i, key := range keys {
		cmds[i] = valuePipe.Get(ctx, key)
	}

	_, err = valuePipe.Exec(ctx)
	if err != nil {
		fmt.Printf("Value pipeline error: %v\n", err)
		return
	}

	fmt.Println("Pipeline key-value pairs:")
	for i, cmd := range cmds {
		fmt.Printf("  %s: %s\n", keys[i], cmd.Val())
	}
}

// transactionExample demonstrates using Redis transactions (MULTI/EXEC)
func transactionExample(client *redis.Client, ctx context.Context) {
	// A transaction ensures commands are executed atomically
	// Start a transaction
	tx := client.TxPipeline()

	// Queue commands in the transaction
	incr := tx.Incr(ctx, "tx_counter")
	tx.Expire(ctx, "tx_counter", time.Hour)

	// Add some key-value pairs in the transaction
	for i := 0; i < 5; i++ {
		tx.Set(ctx, fmt.Sprintf("tx_key_%d", i), fmt.Sprintf("tx_value_%d", i), time.Minute)
	}

	// Execute the transaction
	_, err := tx.Exec(ctx)
	if err != nil {
		fmt.Printf("Transaction error: %v\n", err)
		return
	}

	// Get the result of the increment command
	fmt.Printf("Transaction counter value: %d\n", incr.Val())

	// Get all the keys we created
	keys, err := client.Keys(ctx, "tx_key_*").Result()
	if err != nil {
		fmt.Printf("Error getting transaction keys: %v\n", err)
		return
	}
	fmt.Printf("Created %d keys with transaction\n", len(keys))

	// Watch a key for changes (optimistic locking)
	watchKey := "watched_key"
	client.Set(ctx, watchKey, "original_value", time.Minute)

	// Run a transaction with WATCH
	err = client.Watch(ctx, func(tx *redis.Tx) error {
		// Get current value
		current, err := tx.Get(ctx, watchKey).Result()
		if err != nil && err != redis.Nil {
			return err
		}
		fmt.Printf("Current watched value: %s\n", current)

		// Start a transaction
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			// Set new value
			pipe.Set(ctx, watchKey, "new_value", time.Minute)
			return nil
		})
		return err
	}, watchKey)

	if err != nil {
		fmt.Printf("Watch transaction error: %v\n", err)
		return
	}

	// Get the new value
	newValue, err := client.Get(ctx, watchKey).Result()
	if err != nil {
		fmt.Printf("Error getting new value: %v\n", err)
		return
	}
	fmt.Printf("New watched value: %s\n", newValue)
}

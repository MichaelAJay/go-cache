package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/providers/redis"
	"github.com/MichaelAJay/go-serializer"
)

// Example struct to store in the cache
type User struct {
	ID        int       `json:"id" msgpack:"id"`
	Name      string    `json:"name" msgpack:"name"`
	Email     string    `json:"email" msgpack:"email"`
	CreatedAt time.Time `json:"created_at" msgpack:"created_at"`
	Roles     []string  `json:"roles" msgpack:"roles"`
}

func main() {
	// Create a context
	ctx := context.Background()

	// Create cache options with MsgPack serializer explicitly set
	cacheOptions := &cache.CacheOptions{
		TTL:              time.Hour,
		MaxEntries:       1000,
		CleanupInterval:  time.Minute * 5,
		SerializerFormat: serializer.Msgpack, // Explicitly use MessagePack
		RedisOptions: &cache.RedisOptions{
			Address:  "localhost:6379", // Update with your Redis address
			Password: "",               // Update if Redis requires password
			DB:       0,                // Default Redis DB
			PoolSize: 10,               // Connection pool size
		},
	}

	// Create a Redis cache provider
	provider := redis.NewProvider()

	// Create a Redis cache instance
	redisCache, err := provider.Create(cacheOptions)
	if err != nil {
		log.Fatalf("Failed to create Redis cache: %v", err)
	}
	defer redisCache.Close()

	fmt.Println("Connected to Redis cache with MessagePack serializer")

	// Create sample user
	user := &User{
		ID:        1,
		Name:      "John Doe",
		Email:     "john@example.com",
		CreatedAt: time.Now(),
		Roles:     []string{"admin", "user"},
	}

	// Store user in cache with TTL of 10 minutes
	err = redisCache.Set(ctx, "user:1", user, time.Minute*10)
	if err != nil {
		log.Fatalf("Failed to store user in cache: %v", err)
	}
	fmt.Println("Stored user in cache")

	// Retrieve user from cache
	cachedData, found, err := redisCache.Get(ctx, "user:1")
	if err != nil {
		log.Fatalf("Error retrieving user: %v", err)
	}
	if !found {
		log.Fatalf("User not found in cache")
	}

	// Display the retrieved user data
	fmt.Printf("Retrieved user: %+v\n", cachedData)

	// Store multiple users
	users := map[string]any{
		"user:2": &User{ID: 2, Name: "Jane Smith", Email: "jane@example.com", Roles: []string{"user"}},
		"user:3": &User{ID: 3, Name: "Bob Johnson", Email: "bob@example.com", Roles: []string{"editor"}},
	}
	err = redisCache.SetMany(ctx, users, time.Minute*10)
	if err != nil {
		log.Fatalf("Failed to store multiple users: %v", err)
	}
	fmt.Println("Stored multiple users in cache")

	// Retrieve multiple users
	keys := []string{"user:1", "user:2", "user:3"}
	multiUsers, err := redisCache.GetMany(ctx, keys)
	if err != nil {
		log.Fatalf("Failed to retrieve multiple users: %v", err)
	}
	fmt.Printf("Retrieved %d users\n", len(multiUsers))

	// Check if a key exists
	exists := redisCache.Has(ctx, "user:1")
	fmt.Printf("User:1 exists in cache: %v\n", exists)

	// Get all keys
	allKeys := redisCache.GetKeys(ctx)
	fmt.Printf("All cache keys: %v\n", allKeys)

	// Get metadata
	metadata, err := redisCache.GetMetadata(ctx, "user:1")
	if err != nil {
		log.Printf("Warning: Failed to get metadata: %v", err)
	} else {
		fmt.Printf("Metadata for user:1: TTL=%v\n", metadata.TTL)
	}

	// Delete a key
	err = redisCache.Delete(ctx, "user:1")
	if err != nil {
		log.Fatalf("Failed to delete user: %v", err)
	}
	fmt.Println("Deleted user:1 from cache")

	// Verify deletion
	_, found, _ = redisCache.Get(ctx, "user:1")
	fmt.Printf("User:1 exists in cache after deletion: %v\n", found)

	fmt.Println("Example completed successfully")
}

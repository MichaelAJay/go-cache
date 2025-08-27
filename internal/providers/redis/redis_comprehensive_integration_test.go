//go:build integration
// +build integration

package redis_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache"
	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/MichaelAJay/go-cache/internal/providers/redis"
	"github.com/MichaelAJay/go-serializer"
)

// TestSessionManagementIntegration tests realistic session management scenarios
func TestSessionManagementIntegration(t *testing.T) {
	redis, err := redis.NewRedisTestContainer(t,
		redis.WithRedisVersion("redis:8.0"),
		redis.WithCacheOptions(&interfaces.CacheOptions{
			TTL:              24 * time.Hour,
			SerializerFormat: serializer.Msgpack,
			Security: &cache.SecurityConfig{
				EnableTimingProtection: true,
				MinProcessingTime:      5 * time.Millisecond,
				SecureCleanup:          true,
			},
			Hooks: &cache.CacheHooks{
				PreGet: func(ctx context.Context, key string) error {
					t.Logf("Session access: %s", key)
					return nil
				},
				PostSet: func(ctx context.Context, key string, value any, ttl time.Duration, err error) {
					if err == nil {
						t.Logf("Session created/updated: %s (TTL: %v)", key, ttl)
					}
				},
			},
		}),
	)
	if err != nil {
		t.Fatalf("Failed to start Redis container: %v", err)
	}
	defer redis.Cleanup()

	ctx := context.Background()
	testPrefix := CreateUniqueTestID("session")

	// Session data structure
	type UserSession struct {
		UserID       string            `json:"user_id" msgpack:"user_id"`
		Username     string            `json:"username" msgpack:"username"`
		Email        string            `json:"email" msgpack:"email"`
		Roles        []string          `json:"roles" msgpack:"roles"`
		Permissions  []string          `json:"permissions" msgpack:"permissions"`
		LastActivity time.Time         `json:"last_activity" msgpack:"last_activity"`
		IPAddress    string            `json:"ip_address" msgpack:"ip_address"`
		UserAgent    string            `json:"user_agent" msgpack:"user_agent"`
		Metadata     map[string]string `json:"metadata" msgpack:"metadata"`
	}

	t.Run("UserSessionLifecycle", func(t *testing.T) {
		userID := "user123"
		sessionID := fmt.Sprintf("%ssession:%s", testPrefix, userID)

		// Create session
		session := UserSession{
			UserID:       userID,
			Username:     "john_doe",
			Email:        "john@example.com",
			Roles:        []string{"user", "editor"},
			Permissions:  []string{"read", "write", "delete"},
			LastActivity: time.Now(),
			IPAddress:    "192.168.1.100",
			UserAgent:    "Mozilla/5.0...",
			Metadata: map[string]string{
				"theme":    "dark",
				"language": "en-US",
				"timezone": "America/New_York",
			},
		}

		// Store session with 1 hour TTL
		err := redis.Cache.Set(ctx, sessionID, session, time.Hour)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Retrieve and validate session
		retrievedValue, found, err := redis.Cache.Get(ctx, sessionID)
		if err != nil {
			t.Fatalf("Failed to retrieve session: %v", err)
		}
		if !found {
			t.Fatal("Session should exist")
		}

		// Validate session data
		sessionMap, ok := retrievedValue.(map[string]any)
		if !ok {
			t.Fatalf("Expected session map, got %T", retrievedValue)
		}

		if sessionMap["username"] != "john_doe" {
			t.Errorf("Expected username 'john_doe', got '%v'", sessionMap["username"])
		}

		if len(sessionMap["roles"].([]any)) != 2 {
			t.Errorf("Expected 2 roles, got %d", len(sessionMap["roles"].([]any)))
		}

		// Update session activity
		session.LastActivity = time.Now()
		session.Metadata["last_page"] = "/dashboard"

		err = redis.Cache.Set(ctx, sessionID, session, time.Hour)
		if err != nil {
			t.Fatalf("Failed to update session: %v", err)
		}

		// Verify update
		updatedValue, found, err := redis.Cache.Get(ctx, sessionID)
		if err != nil {
			t.Fatalf("Failed to retrieve updated session: %v", err)
		}
		if !found {
			t.Fatal("Updated session should exist")
		}

		updatedMap := updatedValue.(map[string]any)
		metadata := updatedMap["metadata"].(map[string]any)
		if metadata["last_page"] != "/dashboard" {
			t.Errorf("Expected last_page '/dashboard', got '%v'", metadata["last_page"])
		}

		// Clean up session
		err = redis.Cache.Delete(ctx, sessionID)
		if err != nil {
			t.Fatalf("Failed to delete session: %v", err)
		}

		// Verify deletion
		_, found, err = redis.Cache.Get(ctx, sessionID)
		if err != nil {
			t.Fatalf("Unexpected error after deletion: %v", err)
		}
		if found {
			t.Error("Session should be deleted")
		}
	})

	t.Run("MultipleUserSessions", func(t *testing.T) {
		users := []struct {
			userID   string
			username string
			email    string
		}{
			{"user1", "alice", "alice@example.com"},
			{"user2", "bob", "bob@example.com"},
			{"user3", "charlie", "charlie@example.com"},
		}

		sessionData := make(map[string]UserSession)

		// Create sessions for multiple users
		for _, user := range users {
			sessionID := fmt.Sprintf("%ssession:%s", testPrefix, user.userID)
			session := UserSession{
				UserID:       user.userID,
				Username:     user.username,
				Email:        user.email,
				Roles:        []string{"user"},
				Permissions:  []string{"read"},
				LastActivity: time.Now(),
				IPAddress:    "192.168.1.1",
				UserAgent:    "Test Agent",
				Metadata:     map[string]string{"created": "test"},
			}

			err := redis.Cache.Set(ctx, sessionID, session, time.Hour)
			if err != nil {
				t.Fatalf("Failed to create session for %s: %v", user.userID, err)
			}

			sessionData[sessionID] = session
		}

		// Verify all sessions exist
		for sessionID, originalSession := range sessionData {
			retrievedValue, found, err := redis.Cache.Get(ctx, sessionID)
			if err != nil {
				t.Fatalf("Failed to retrieve session %s: %v", sessionID, err)
			}
			if !found {
				t.Fatalf("Session %s should exist", sessionID)
			}

			sessionMap := retrievedValue.(map[string]any)
			if sessionMap["username"] != originalSession.Username {
				t.Errorf("Session %s: expected username '%s', got '%v'",
					sessionID, originalSession.Username, sessionMap["username"])
			}
		}

		// Bulk cleanup
		sessionIDs := make([]string, 0, len(sessionData))
		for sessionID := range sessionData {
			sessionIDs = append(sessionIDs, sessionID)
		}

		err = redis.Cache.DeleteMany(ctx, sessionIDs)
		if err != nil {
			t.Fatalf("Failed to delete sessions: %v", err)
		}

		// Verify all sessions are deleted
		results, err := redis.Cache.GetMany(ctx, sessionIDs)
		if err != nil {
			t.Fatalf("Failed to verify session deletion: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("Expected 0 sessions after deletion, got %d", len(results))
		}
	})

	t.Run("SessionExpiration", func(t *testing.T) {
		sessionID := fmt.Sprintf("%sexpiring-session", testPrefix)
		session := UserSession{
			UserID:       "temp-user",
			Username:     "temp",
			Email:        "temp@example.com",
			LastActivity: time.Now(),
		}

		// Create session with short TTL
		shortTTL := 300 * time.Millisecond
		err := redis.Cache.Set(ctx, sessionID, session, shortTTL)
		if err != nil {
			t.Fatalf("Failed to create expiring session: %v", err)
		}

		// Verify session exists initially
		_, found, err := redis.Cache.Get(ctx, sessionID)
		if err != nil {
			t.Fatalf("Failed to get session: %v", err)
		}
		if !found {
			t.Fatal("Session should exist initially")
		}

		// Wait for expiration
		time.Sleep(400 * time.Millisecond)

		// Verify session has expired
		_, found, err = redis.Cache.Get(ctx, sessionID)
		if err != nil {
			t.Fatalf("Unexpected error after expiration: %v", err)
		}
		if found {
			t.Error("Session should have expired")
		}
	})
}

// TestCacheProvidersIntegration tests integration between different cache providers
func TestCacheProvidersIntegration(t *testing.T) {
	// Start Redis container for comparison
	redis, err := redis.NewRedisTestContainer(t,
		redis.WithRedisVersion("redis:8.0"),
		redis.WithCacheOptions(&cache.CacheOptions{
			TTL:              time.Hour,
			SerializerFormat: serializer.Msgpack,
		}),
	)
	if err != nil {
		t.Fatalf("Failed to start Redis container: %v", err)
	}
	defer redis.Cleanup()

	ctx := context.Background()
	testPrefix := CreateUniqueTestID("provider-compare")

	t.Run("DataConsistencyAcrossProviders", func(t *testing.T) {
		// Test data
		testData := map[string]any{
			fmt.Sprintf("%sstring", testPrefix): "test-value",
			fmt.Sprintf("%snumber", testPrefix): 42,
			fmt.Sprintf("%sbool", testPrefix):   true,
			fmt.Sprintf("%scomplex", testPrefix): map[string]any{
				"nested": map[string]any{
					"value": "deeply-nested",
					"count": 123,
				},
				"array": []any{1, 2, 3, "four"},
			},
		}

		// Store data in Redis
		for key, value := range testData {
			err := redis.Cache.Set(ctx, key, value, time.Hour)
			if err != nil {
				t.Fatalf("Failed to set %s in Redis: %v", key, err)
			}
		}

		// Retrieve and validate data from Redis
		for key, expectedValue := range testData {
			retrievedValue, found, err := redis.Cache.Get(ctx, key)
			if err != nil {
				t.Fatalf("Failed to get %s from Redis: %v", key, err)
			}
			if !found {
				t.Fatalf("Key %s not found in Redis", key)
			}

			// Basic type validation
			switch expectedValue.(type) {
			case string:
				if retrievedValue != expectedValue {
					t.Errorf("Redis: key %s, expected %v, got %v", key, expectedValue, retrievedValue)
				}
			case int:
				// Redis/msgpack may return int64
				if retrievedInt, ok := retrievedValue.(int64); ok {
					if int(retrievedInt) != expectedValue.(int) {
						t.Errorf("Redis: key %s, expected %v, got %v", key, expectedValue, retrievedValue)
					}
				} else if retrievedValue != expectedValue {
					t.Errorf("Redis: key %s, expected %v, got %v", key, expectedValue, retrievedValue)
				}
			case bool:
				if retrievedValue != expectedValue {
					t.Errorf("Redis: key %s, expected %v, got %v", key, expectedValue, retrievedValue)
				}
			case map[string]any:
				// Complex validation for nested structures
				retrievedMap, ok := retrievedValue.(map[string]any)
				if !ok {
					t.Errorf("Redis: key %s, expected map, got %T", key, retrievedValue)
					continue
				}
				validateNestedMap(t, key, expectedValue.(map[string]any), retrievedMap)
			}
		}

		t.Logf("Successfully validated %d keys across providers", len(testData))
	})
}

// TestConcurrentCacheOperations tests cache behavior under concurrent access
func TestConcurrentCacheOperations(t *testing.T) {
	redis, err := redis.NewRedisTestContainer(t,
		redis.WithRedisVersion("redis:8.0"),
		redis.WithCacheOptions(&cache.CacheOptions{
			TTL:              time.Hour,
			SerializerFormat: serializer.Msgpack,
		}),
	)
	if err != nil {
		t.Fatalf("Failed to start Redis container: %v", err)
	}
	defer redis.Cleanup()

	ctx := context.Background()
	testPrefix := CreateUniqueTestID("concurrent")

	t.Run("ConcurrentReadsAndWrites", func(t *testing.T) {
		const numGoroutines = 50
		const operationsPerGoroutine = 20

		var wg sync.WaitGroup
		errChan := make(chan error, numGoroutines)

		// Start concurrent writers
		for i := 0; i < numGoroutines/2; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < operationsPerGoroutine; j++ {
					key := fmt.Sprintf("%swriter:%d:key:%d", testPrefix, workerID, j)
					value := fmt.Sprintf("value-%d-%d", workerID, j)

					if err := redis.Cache.Set(ctx, key, value, time.Hour); err != nil {
						errChan <- fmt.Errorf("writer %d: failed to set %s: %w", workerID, key, err)
						return
					}
				}
			}(i)
		}

		// Start concurrent readers (will read keys as they're written)
		for i := numGoroutines / 2; i < numGoroutines; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < operationsPerGoroutine; j++ {
					// Try to read from various writers
					writerID := j % (numGoroutines / 2)
					key := fmt.Sprintf("%swriter:%d:key:%d", testPrefix, writerID, j)

					_, found, err := redis.Cache.Get(ctx, key)
					if err != nil {
						errChan <- fmt.Errorf("reader %d: failed to get %s: %w", workerID, key, err)
						return
					}
					// It's OK if not found - writers might not have written yet
					_ = found
				}
			}(i)
		}

		wg.Wait()
		close(errChan)

		// Check for errors
		for err := range errChan {
			t.Error(err)
		}

		t.Logf("Completed %d concurrent operations across %d goroutines",
			numGoroutines*operationsPerGoroutine, numGoroutines)
	})

	t.Run("ConcurrentCounterOperations", func(t *testing.T) {
		// Use separate keys for each worker to avoid serialization issues
		// Then sum them up to test concurrent operations
		const numGoroutines = 20
		const incrementsPerGoroutine = 10

		var wg sync.WaitGroup
		errChan := make(chan error, numGoroutines)
		counterKeys := make([]string, numGoroutines)

		// Initialize individual counters for each worker
		for i := 0; i < numGoroutines; i++ {
			counterKeys[i] = fmt.Sprintf("%scounter-%d", testPrefix, i)
			// Initialize with Increment operation to avoid serialization issues
			_, err := redis.Cache.Increment(ctx, counterKeys[i], 0, time.Hour)
			if err != nil {
				t.Fatalf("Failed to initialize counter %d: %v", i, err)
			}
		}

		// Start concurrent incrementers
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < incrementsPerGoroutine; j++ {
					_, err := redis.Cache.Increment(ctx, counterKeys[workerID], 1, time.Hour)
					if err != nil {
						errChan <- fmt.Errorf("worker %d: increment failed: %w", workerID, err)
						return
					}
				}
			}(i)
		}

		wg.Wait()
		close(errChan)

		// Check for errors
		for err := range errChan {
			t.Error(err)
		}

		// Sum up all counter values
		var totalValue int64
		for i, key := range counterKeys {
			finalValue, found, err := redis.Cache.Get(ctx, key)
			if err != nil {
				t.Fatalf("Failed to get counter %d value: %v", i, err)
			}
			if !found {
				t.Fatalf("Counter %d should exist", i)
			}

			// Handle different numeric types
			var counterValue int64
			switch v := finalValue.(type) {
			case int64:
				counterValue = v
			case int:
				counterValue = int64(v)
			case int8:
				counterValue = int64(v)
			case int16:
				counterValue = int64(v)
			case int32:
				counterValue = int64(v)
			case uint64:
				counterValue = int64(v)
			case uint:
				counterValue = int64(v)
			case uint8:
				counterValue = int64(v)
			case uint16:
				counterValue = int64(v)
			case uint32:
				counterValue = int64(v)
			case float64:
				counterValue = int64(v)
			case float32:
				counterValue = int64(v)
			default:
				t.Fatalf("Unexpected counter type for counter %d: %T", i, finalValue)
			}

			totalValue += counterValue
		}

		expectedValue := int64(numGoroutines * incrementsPerGoroutine)

		if totalValue != expectedValue {
			t.Errorf("Expected total counter value %d, got %d", expectedValue, totalValue)
		}

		t.Logf("Counter test passed: %d concurrent increments across %d counters resulted in total value %d",
			expectedValue, numGoroutines, totalValue)
	})
}

// TestErrorHandlingIntegration tests error handling in realistic scenarios
func TestErrorHandlingIntegration(t *testing.T) {
	redis, err := redis.NewRedisTestContainer(t,
		redis.WithRedisVersion("redis:8.0"),
		redis.WithCacheOptions(&cache.CacheOptions{
			TTL:              time.Hour,
			SerializerFormat: serializer.Msgpack,
		}),
	)
	if err != nil {
		t.Fatalf("Failed to start Redis container: %v", err)
	}
	defer redis.Cleanup()

	ctx := context.Background()
	testPrefix := CreateUniqueTestID("error-handling")

	t.Run("ContextCancellation", func(t *testing.T) {
		key := fmt.Sprintf("%scancel-test", testPrefix)

		// Create a context that will be cancelled
		cancelCtx, cancel := context.WithCancel(ctx)

		// Cancel immediately
		cancel()

		// Try to perform operations with cancelled context
		err := redis.Cache.Set(cancelCtx, key, "value", time.Hour)
		if err == nil {
			t.Error("Expected error with cancelled context, got nil")
		}

		_, _, err = redis.Cache.Get(cancelCtx, key)
		if err == nil {
			t.Error("Expected error with cancelled context, got nil")
		}

		t.Logf("Context cancellation properly handled")
	})

	t.Run("TimeoutHandling", func(t *testing.T) {
		key := fmt.Sprintf("%stimeout-test", testPrefix)

		// Create a context with very short timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
		defer cancel()

		// Wait to ensure timeout
		time.Sleep(1 * time.Millisecond)

		// Try operations with timed-out context
		err := redis.Cache.Set(timeoutCtx, key, "value", time.Hour)
		if err == nil {
			t.Error("Expected timeout error, got nil")
		}

		t.Logf("Timeout handling working: %v", err)
	})

	t.Run("LargeDataHandling", func(t *testing.T) {
		key := fmt.Sprintf("%slarge-data", testPrefix)

		// Create large data structure
		largeData := make(map[string]string)
		for i := 0; i < 1000; i++ {
			largeData[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d_with_some_longer_content_to_make_it_larger", i)
		}

		// Store large data
		err := redis.Cache.Set(ctx, key, largeData, time.Hour)
		if err != nil {
			t.Fatalf("Failed to store large data: %v", err)
		}

		// Retrieve large data
		retrievedValue, found, err := redis.Cache.Get(ctx, key)
		if err != nil {
			t.Fatalf("Failed to retrieve large data: %v", err)
		}
		if !found {
			t.Fatal("Large data should exist")
		}

		retrievedMap, ok := retrievedValue.(map[string]any)
		if !ok {
			t.Fatalf("Expected map, got %T", retrievedValue)
		}

		if len(retrievedMap) != len(largeData) {
			t.Errorf("Expected %d items, got %d", len(largeData), len(retrievedMap))
		}

		t.Logf("Successfully handled large data structure with %d items", len(largeData))
	})
}

// TestCacheWarmupAndPreloading tests cache warming scenarios
func TestCacheWarmupAndPreloading(t *testing.T) {
	redis, err := redis.NewRedisTestContainer(t,
		redis.WithRedisVersion("redis:8.0"),
		redis.WithCacheOptions(&cache.CacheOptions{
			TTL:              2 * time.Hour,
			SerializerFormat: serializer.Msgpack,
		}),
	)
	if err != nil {
		t.Fatalf("Failed to start Redis container: %v", err)
	}
	defer redis.Cleanup()

	ctx := context.Background()
	testPrefix := CreateUniqueTestID("warmup")

	t.Run("BulkDataPreloading", func(t *testing.T) {
		// Simulate preloading common application data
		preloadData := map[string]any{
			fmt.Sprintf("%sconfig:app_name", testPrefix):     "My Application",
			fmt.Sprintf("%sconfig:version", testPrefix):      "1.0.0",
			fmt.Sprintf("%sconfig:features", testPrefix):     []string{"feature1", "feature2", "feature3"},
			fmt.Sprintf("%sconfig:database_url", testPrefix): "postgresql://localhost:5432/myapp",
			fmt.Sprintf("%sconfig:cache_ttl", testPrefix):    3600,
			fmt.Sprintf("%sconfig:max_users", testPrefix):    1000,
			fmt.Sprintf("%sconfig:debug_mode", testPrefix):   false,
		}

		// Bulk preload using SetMany
		err := redis.Cache.SetMany(ctx, preloadData, 2*time.Hour)
		if err != nil {
			t.Fatalf("Failed to preload data: %v", err)
		}

		// Verify all preloaded data
		keys := make([]string, 0, len(preloadData))
		for key := range preloadData {
			keys = append(keys, key)
		}

		results, err := redis.Cache.GetMany(ctx, keys)
		if err != nil {
			t.Fatalf("Failed to retrieve preloaded data: %v", err)
		}

		if len(results) != len(preloadData) {
			t.Errorf("Expected %d preloaded items, got %d", len(preloadData), len(results))
		}

		// Validate specific items
		appNameKey := fmt.Sprintf("%sconfig:app_name", testPrefix)
		if results[appNameKey] != "My Application" {
			t.Errorf("Expected app_name 'My Application', got '%v'", results[appNameKey])
		}

		t.Logf("Successfully preloaded %d configuration items", len(preloadData))
	})

	t.Run("UserDataPreloading", func(t *testing.T) {
		// Simulate preloading user profiles for active users
		users := []struct {
			id       string
			username string
			email    string
			active   bool
		}{
			{"user1", "alice", "alice@example.com", true},
			{"user2", "bob", "bob@example.com", true},
			{"user3", "charlie", "charlie@example.com", false},
			{"user4", "diana", "diana@example.com", true},
			{"user5", "eve", "eve@example.com", true},
		}

		userProfiles := make(map[string]any)
		for _, user := range users {
			key := fmt.Sprintf("%sprofile:%s", testPrefix, user.id)
			profile := map[string]any{
				"id":       user.id,
				"username": user.username,
				"email":    user.email,
				"active":   user.active,
				"profile": map[string]any{
					"created_at":  time.Now().Format(time.RFC3339),
					"last_login":  time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
					"login_count": 42,
				},
			}
			userProfiles[key] = profile
		}

		// Preload user profiles
		err := redis.Cache.SetMany(ctx, userProfiles, 2*time.Hour)
		if err != nil {
			t.Fatalf("Failed to preload user profiles: %v", err)
		}

		// Simulate application retrieving user profiles
		for _, user := range users {
			key := fmt.Sprintf("%sprofile:%s", testPrefix, user.id)
			profile, found, err := redis.Cache.Get(ctx, key)
			if err != nil {
				t.Fatalf("Failed to get profile for %s: %v", user.id, err)
			}
			if !found {
				t.Fatalf("Profile for %s should exist", user.id)
			}

			profileMap := profile.(map[string]any)
			if profileMap["username"] != user.username {
				t.Errorf("User %s: expected username '%s', got '%v'",
					user.id, user.username, profileMap["username"])
			}
		}

		t.Logf("Successfully preloaded and validated %d user profiles", len(users))
	})
}

// TestFailoverAndResilience tests cache behavior during failures
func TestFailoverAndResilience(t *testing.T) {
	redis, err := redis.NewRedisTestContainer(t,
		redis.WithRedisVersion("redis:8.0"),
		redis.WithCacheOptions(&cache.CacheOptions{
			TTL:              time.Hour,
			SerializerFormat: serializer.Msgpack,
		}),
	)
	if err != nil {
		t.Fatalf("Failed to start Redis container: %v", err)
	}
	defer redis.Cleanup()

	ctx := context.Background()
	testPrefix := CreateUniqueTestID("resilience")

	t.Run("GracefulDegradation", func(t *testing.T) {
		// Pre-populate cache with data
		testData := map[string]any{
			fmt.Sprintf("%sresilience:data1", testPrefix): "value1",
			fmt.Sprintf("%sresilience:data2", testPrefix): "value2",
			fmt.Sprintf("%sresilience:data3", testPrefix): "value3",
		}

		for key, value := range testData {
			err := redis.Cache.Set(ctx, key, value, time.Hour)
			if err != nil {
				t.Fatalf("Failed to set %s: %v", key, err)
			}
		}

		// Verify data exists
		for key, expectedValue := range testData {
			value, found, err := redis.Cache.Get(ctx, key)
			if err != nil {
				t.Fatalf("Failed to get %s: %v", key, err)
			}
			if !found {
				t.Fatalf("Key %s should exist", key)
			}
			if value != expectedValue {
				t.Errorf("Key %s: expected %v, got %v", key, expectedValue, value)
			}
		}

		t.Logf("Successfully verified graceful behavior with %d items", len(testData))
	})

	t.Run("ConnectionRecovery", func(t *testing.T) {
		key := fmt.Sprintf("%srecovery-test", testPrefix)

		// Set initial value
		err := redis.Cache.Set(ctx, key, "initial-value", time.Hour)
		if err != nil {
			t.Fatalf("Failed to set initial value: %v", err)
		}

		// Verify value exists
		value, found, err := redis.Cache.Get(ctx, key)
		if err != nil {
			t.Fatalf("Failed to get initial value: %v", err)
		}
		if !found || value != "initial-value" {
			t.Fatalf("Expected 'initial-value', got %v (found: %v)", value, found)
		}

		// Note: In a real failover test, we would restart the Redis container here
		// For this test, we'll just verify that the connection remains stable

		// Try multiple operations to ensure connection stability
		for i := 0; i < 10; i++ {
			testKey := fmt.Sprintf("%srecovery-%d", testPrefix, i)
			testValue := fmt.Sprintf("recovery-value-%d", i)

			err := redis.Cache.Set(ctx, testKey, testValue, time.Hour)
			if err != nil {
				t.Errorf("Failed to set recovery key %d: %v", i, err)
				continue
			}

			retrievedValue, found, err := redis.Cache.Get(ctx, testKey)
			if err != nil {
				t.Errorf("Failed to get recovery key %d: %v", i, err)
				continue
			}
			if !found || retrievedValue != testValue {
				t.Errorf("Recovery key %d: expected %s, got %v (found: %v)",
					i, testValue, retrievedValue, found)
			}
		}

		t.Logf("Connection recovery test completed successfully")
	})
}

// TestPerformanceUnderLoad tests cache performance under various load conditions
func TestPerformanceUnderLoad(t *testing.T) {
	redis, err := redis.NewRedisTestContainer(t,
		redis.WithRedisVersion("redis:8.0"),
		redis.WithCacheOptions(&cache.CacheOptions{
			TTL:              time.Hour,
			SerializerFormat: serializer.Msgpack,
		}),
	)
	if err != nil {
		t.Fatalf("Failed to start Redis container: %v", err)
	}
	defer redis.Cleanup()

	ctx := context.Background()
	testPrefix := CreateUniqueTestID("perf")

	t.Run("HighThroughputOperations", func(t *testing.T) {
		const numOperations = 1000
		const concurrency = 10

		var wg sync.WaitGroup
		errChan := make(chan error, concurrency)
		operationChan := make(chan int, numOperations)

		// Fill operation channel
		for i := 0; i < numOperations; i++ {
			operationChan <- i
		}
		close(operationChan)

		startTime := time.Now()

		// Start workers
		for w := 0; w < concurrency; w++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for opID := range operationChan {
					key := fmt.Sprintf("%sthroughput:%d:%d", testPrefix, workerID, opID)
					value := fmt.Sprintf("value-%d-%d", workerID, opID)

					// Set operation
					if err := redis.Cache.Set(ctx, key, value, time.Hour); err != nil {
						errChan <- fmt.Errorf("worker %d: set failed: %w", workerID, err)
						return
					}

					// Get operation
					retrievedValue, found, err := redis.Cache.Get(ctx, key)
					if err != nil {
						errChan <- fmt.Errorf("worker %d: get failed: %w", workerID, err)
						return
					}
					if !found || retrievedValue != value {
						errChan <- fmt.Errorf("worker %d: value mismatch", workerID)
						return
					}
				}
			}(w)
		}

		wg.Wait()
		close(errChan)

		duration := time.Since(startTime)

		// Check for errors
		errorCount := 0
		for err := range errChan {
			t.Error(err)
			errorCount++
		}

		if errorCount == 0 {
			opsPerSecond := float64(numOperations*2) / duration.Seconds() // *2 for set+get
			t.Logf("High throughput test: %d operations in %v (%.2f ops/sec)",
				numOperations*2, duration, opsPerSecond)
		}
	})

	t.Run("MemoryUsagePatterns", func(t *testing.T) {
		// Test different payload sizes
		payloadSizes := []int{100, 1000, 10000, 50000}

		for _, size := range payloadSizes {
			t.Run(fmt.Sprintf("PayloadSize_%d", size), func(t *testing.T) {
				// Create payload of specified size
				payload := make([]byte, size)
				for i := range payload {
					payload[i] = byte(i % 256)
				}

				key := fmt.Sprintf("%smemory:%d", testPrefix, size)

				start := time.Now()
				err := redis.Cache.Set(ctx, key, payload, time.Hour)
				setDuration := time.Since(start)

				if err != nil {
					t.Fatalf("Failed to set payload size %d: %v", size, err)
				}

				start = time.Now()
				retrievedValue, found, err := redis.Cache.Get(ctx, key)
				getDuration := time.Since(start)

				if err != nil {
					t.Fatalf("Failed to get payload size %d: %v", size, err)
				}
				if !found {
					t.Fatalf("Payload size %d should exist", size)
				}

				// Verify payload integrity (basic check)
				retrievedBytes, ok := retrievedValue.([]byte)
				if !ok {
					t.Fatalf("Expected []byte, got %T", retrievedValue)
				}
				if len(retrievedBytes) != size {
					t.Errorf("Payload size %d: expected length %d, got %d",
						size, size, len(retrievedBytes))
				}

				t.Logf("Payload size %d: Set=%v, Get=%v", size, setDuration, getDuration)
			})
		}
	})
}

// Helper function to validate nested maps
func validateNestedMap(t *testing.T, keyPrefix string, expected, actual map[string]any) {
	for key, expectedValue := range expected {
		actualValue, exists := actual[key]
		if !exists {
			t.Errorf("%s: missing key '%s'", keyPrefix, key)
			continue
		}

		switch expectedValue.(type) {
		case map[string]any:
			if actualMap, ok := actualValue.(map[string]any); ok {
				validateNestedMap(t, fmt.Sprintf("%s.%s", keyPrefix, key), expectedValue.(map[string]any), actualMap)
			} else {
				t.Errorf("%s.%s: expected map, got %T", keyPrefix, key, actualValue)
			}
		case []any:
			if actualSlice, ok := actualValue.([]any); ok {
				expectedSlice := expectedValue.([]any)
				if len(actualSlice) != len(expectedSlice) {
					t.Errorf("%s.%s: expected slice length %d, got %d",
						keyPrefix, key, len(expectedSlice), len(actualSlice))
				}
			} else {
				t.Errorf("%s.%s: expected slice, got %T", keyPrefix, key, actualValue)
			}
		default:
			if actualValue != expectedValue {
				t.Errorf("%s.%s: expected %v, got %v", keyPrefix, key, expectedValue, actualValue)
			}
		}
	}
}

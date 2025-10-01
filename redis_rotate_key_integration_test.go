//go:build integration

package cache_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/internal/testintegration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRedisCache_RotateKey_BasicOperation verifies basic rotation with field updates
func TestRedisCache_RotateKey_BasicOperation(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateRotateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create cache")
	defer cache.Close()

	oldKey := "session:old-123"
	newKey := "session:new-456"

	// Create original session
	now := time.Now()
	originalSession := &testintegration.RotateTestSession{
		ID:           oldKey,
		UserID:       "user-123",
		Username:     "testuser",
		Created:      now.Unix(),
		ExpiresAt:    now.Add(1 * time.Hour).Unix(),
		LastActivity: now.Unix(),
	}

	t.Logf("📝 Setting initial session with key: %s", oldKey)
	err = cache.Set(ctx, originalSession, 1*time.Hour)
	require.NoError(t, err)

	// Execute rotation
	newExpiresAt := now.Add(2 * time.Hour).Unix()
	newLastActivity := now.Unix()

	t.Logf("🔄 Rotating session from %s to %s", oldKey, newKey)
	rotated, err := cache.RotateKey(
		ctx,
		oldKey,
		newKey,
		newKey,          // newID
		newExpiresAt,    // newExpiresAt
		newLastActivity, // newLastActivity
		2*time.Hour,     // newTTL
	)

	// Verify operation succeeded
	require.NoError(t, err, "RotateKey should succeed")
	require.NotNil(t, rotated, "Rotated session should not be nil")

	// Verify updated fields
	assert.Equal(t, newKey, rotated.ID, "ID should be updated")
	assert.Equal(t, newExpiresAt, rotated.ExpiresAt, "ExpiresAt should be updated")
	assert.Equal(t, newLastActivity, rotated.LastActivity, "LastActivity should be updated")

	// Verify preserved fields
	assert.Equal(t, originalSession.UserID, rotated.UserID, "UserID should be preserved")
	assert.Equal(t, originalSession.Username, rotated.Username, "Username should be preserved")
	assert.Equal(t, originalSession.Created, rotated.Created, "Created should be preserved")

	// Verify old key deleted
	t.Logf("🔍 Verifying old key %s is deleted", oldKey)
	_, found, err := cache.Get(ctx, oldKey)
	require.NoError(t, err)
	assert.False(t, found, "Old key should be deleted")

	// Verify new key accessible
	t.Logf("🔍 Verifying new key %s exists", newKey)
	retrieved, found, err := cache.Get(ctx, newKey)
	require.NoError(t, err)
	assert.True(t, found, "New key should exist")
	assert.Equal(t, newKey, retrieved.ID)

	t.Logf("✅ RotateKey basic operation test passed")
}

// TestRedisCache_RotateKey_SessionNotFound verifies error handling for non-existent sessions
func TestRedisCache_RotateKey_SessionNotFound(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateRotateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create cache")
	defer cache.Close()

	now := time.Now()
	t.Logf("🔍 Attempting to rotate non-existent session")
	// Try to rotate non-existent session
	_, err = cache.RotateKey(
		ctx,
		"non-existent",
		"new-key",
		"new-key",                      // newID
		now.Add(1*time.Hour).Unix(),    // newExpiresAt
		now.Unix(),                     // newLastActivity
		1*time.Hour,                    // newTTL
	)

	assert.Error(t, err, "Should return error for non-existent session")
	t.Logf("✅ RotateKey error handling test passed - got expected error: %v", err)
}

// TestRedisCache_RotateKey_TTLPrecision verifies millisecond-precision TTL setting
func TestRedisCache_RotateKey_TTLPrecision(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateRotateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create cache")
	defer cache.Close()

	oldKey := "session:ttl-test"
	newKey := "session:ttl-test-new"

	now := time.Now()
	session := &testintegration.RotateTestSession{
		ID:           oldKey,
		UserID:       "user-ttl",
		ExpiresAt:    now.Add(1 * time.Hour).Unix(),
		LastActivity: now.Unix(),
	}

	t.Logf("📝 Setting session with 1-hour TTL")
	err = cache.Set(ctx, session, 1*time.Hour)
	require.NoError(t, err)

	// Rotate with millisecond-precision TTL
	ttl := 1500 * time.Millisecond
	newExpiresAt := now.Add(ttl).Unix()
	newLastActivity := now.Unix()

	t.Logf("🔄 Rotating with TTL: %v", ttl)
	_, err = cache.RotateKey(
		ctx,
		oldKey,
		newKey,
		newKey,          // newID
		newExpiresAt,    // newExpiresAt
		newLastActivity, // newLastActivity
		ttl,             // newTTL
	)

	require.NoError(t, err)

	// Verify key exists immediately
	t.Logf("🔍 Verifying key exists immediately")
	_, found, err := cache.Get(ctx, newKey)
	require.NoError(t, err)
	assert.True(t, found, "Key should exist immediately")

	// Wait for expiration
	t.Logf("⏳ Waiting for TTL expiration (1600ms)")
	time.Sleep(1600 * time.Millisecond)

	// Verify key expired
	t.Logf("🔍 Verifying key expired")
	_, found, err = cache.Get(ctx, newKey)
	require.NoError(t, err)
	assert.False(t, found, "Key should have expired after TTL")

	t.Logf("✅ TTL precision test passed")
}

// TestRedisCache_RotateKey_ConcurrentRotations verifies concurrent rotation operations
func TestRedisCache_RotateKey_ConcurrentRotations(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateRotateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create cache")
	defer cache.Close()

	// Create 100 sessions
	numSessions := 100
	now := time.Now()
	t.Logf("📝 Creating %d sessions", numSessions)

	for i := 0; i < numSessions; i++ {
		key := fmt.Sprintf("session:concurrent-%d", i)
		session := &testintegration.RotateTestSession{
			ID:           key,
			UserID:       fmt.Sprintf("user-%d", i),
			ExpiresAt:    now.Add(1 * time.Hour).Unix(),
			LastActivity: now.Unix(),
		}
		err := cache.Set(ctx, session, 1*time.Hour)
		require.NoError(t, err)
	}

	// Rotate all sessions concurrently
	t.Logf("🔄 Rotating %d sessions concurrently", numSessions)
	var wg sync.WaitGroup
	errors := make(chan error, numSessions)

	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			oldKey := fmt.Sprintf("session:concurrent-%d", idx)
			newKey := fmt.Sprintf("session:rotated-%d", idx)
			rotateTime := time.Now()

			_, err := cache.RotateKey(
				ctx,
				oldKey,
				newKey,
				newKey,                             // newID
				rotateTime.Add(1*time.Hour).Unix(), // newExpiresAt
				rotateTime.Unix(),                  // newLastActivity
				1*time.Hour,                        // newTTL
			)

			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Verify no errors
	errorCount := 0
	for err := range errors {
		t.Errorf("Concurrent rotation failed: %v", err)
		errorCount++
	}

	assert.Equal(t, 0, errorCount, "All concurrent rotations should succeed")

	// Verify all old keys deleted and new keys exist
	t.Logf("🔍 Verifying %d rotations completed successfully", numSessions)
	for i := 0; i < numSessions; i++ {
		oldKey := fmt.Sprintf("session:concurrent-%d", i)
		newKey := fmt.Sprintf("session:rotated-%d", i)

		_, found, _ := cache.Get(ctx, oldKey)
		assert.False(t, found, "Old key should be deleted: %s", oldKey)

		_, found, _ = cache.Get(ctx, newKey)
		assert.True(t, found, "New key should exist: %s", newKey)
	}

	t.Logf("✅ Concurrent rotations test passed")
}

// TestRedisCache_RotateKey_PreservesComplexData verifies preservation of complex data structures
func TestRedisCache_RotateKey_PreservesComplexData(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	// Create cache instance
	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateRotateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err, "Failed to create cache")
	defer cache.Close()

	oldKey := "session:complex-data"
	newKey := "session:complex-data-new"

	// Create session with complex data
	now := time.Now()
	originalCreated := now.Add(-1 * time.Hour).Unix()
	session := &testintegration.RotateTestSession{
		ID:           oldKey,
		UserID:       "user-complex",
		Username:     "complex_user_with_special_chars_!@#$%",
		Created:      originalCreated,
		ExpiresAt:    now.Add(1 * time.Hour).Unix(),
		LastActivity: now.Unix(),
	}

	t.Logf("📝 Setting session with complex data")
	err = cache.Set(ctx, session, 1*time.Hour)
	require.NoError(t, err)

	// Rotate - change ID, ExpiresAt, LastActivity
	newExpiresAt := now.Add(2 * time.Hour).Unix()
	newLastActivity := now.Unix()

	t.Logf("🔄 Rotating session")
	rotated, err := cache.RotateKey(
		ctx,
		oldKey,
		newKey,
		newKey,          // newID
		newExpiresAt,    // newExpiresAt
		newLastActivity, // newLastActivity
		1*time.Hour,     // newTTL
	)

	require.NoError(t, err)

	// Verify all fields preserved correctly
	assert.Equal(t, newKey, rotated.ID, "ID should be updated")
	assert.Equal(t, session.UserID, rotated.UserID, "UserID should be preserved")
	assert.Equal(t, session.Username, rotated.Username, "Username with special chars should be preserved")
	assert.Equal(t, originalCreated, rotated.Created, "Created timestamp should be preserved")
	assert.Equal(t, newExpiresAt, rotated.ExpiresAt, "ExpiresAt should be updated")
	assert.Equal(t, newLastActivity, rotated.LastActivity, "LastActivity should be updated")

	t.Logf("✅ Complex data preservation test passed")
}

//go:build integration

package cache_test

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"testing/quick"
	"time"

	"github.com/MichaelAJay/go-cache/internal/testintegration"
	"github.com/stretchr/testify/require"
)

// PropertyTestConfig defines configuration for property-based tests
type PropertyTestConfig struct {
	MaxCount int
	MaxSize  int
}

// DefaultPropertyConfig returns default configuration for property-based tests
func DefaultPropertyConfig() *PropertyTestConfig {
	return &PropertyTestConfig{
		MaxCount: 100, // Number of test cases to generate
		MaxSize:  10,  // Maximum size of generated data structures
	}
}

// TestCacheSetGetRoundTripProperty verifies that Set followed by Get returns the same value
func TestCacheSetGetRoundTripProperty(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	property := func(key string, userData string, ttl time.Duration) bool {
		// Skip empty keys and very short TTLs that might expire during test
		if key == "" || ttl < time.Millisecond*100 {
			return true
		}

		// Create a test session with the generated data
		session := &testintegration.TestSession{
			ID:       key,
			UserID:   fmt.Sprintf("user_%s", userData),
			Username: fmt.Sprintf("username_%s", userData),
			Created:  time.Now(),
		}

		// Set the value
		err := cache.Set(ctx, session, ttl)
		if err != nil {
			t.Logf("Set failed for key %s: %v", key, err)
			return false
		}

		// Get the value back
		retrieved, found, err := cache.Get(ctx, key)
		if err != nil {
			t.Logf("Get failed for key %s: %v", key, err)
			return false
		}

		if !found {
			t.Logf("Get returned found=false for key %s", key)
			return false
		}

		// Verify the round-trip property: what we set should equal what we get
		if retrieved.ID != session.ID || retrieved.UserID != session.UserID {
			t.Logf("Round-trip failed: set %+v, got %+v", session, retrieved)
			return false
		}

		return true
	}

	// Run property-based test
	config_quick := &quick.Config{
		MaxCount: DefaultPropertyConfig().MaxCount,
		Rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	if err := quick.Check(property, config_quick); err != nil {
		t.Errorf("Set-Get round-trip property failed: %v", err)
	}
}

// TestCacheDeleteIdempotencyProperty verifies that Delete operations are idempotent
func TestCacheDeleteIdempotencyProperty(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	property := func(key string) bool {
		// Skip empty keys
		if key == "" {
			return true
		}

		// First delete should succeed (even if key doesn't exist)
		err1 := cache.Delete(ctx, key)
		if err1 != nil {
			t.Logf("First Delete failed for key %s: %v", key, err1)
			return false
		}

		// Second delete should also succeed (idempotency)
		err2 := cache.Delete(ctx, key)
		if err2 != nil {
			t.Logf("Second Delete failed for key %s: %v", key, err2)
			return false
		}

		// Key should not exist after deletions
		exists := cache.Has(ctx, key)
		if exists {
			t.Logf("Key %s still exists after double delete", key)
			return false
		}

		return true
	}

	config_quick := &quick.Config{
		MaxCount: DefaultPropertyConfig().MaxCount,
		Rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	if err := quick.Check(property, config_quick); err != nil {
		t.Errorf("Delete idempotency property failed: %v", err)
	}
}

// TestCacheBatchOperationConsistencyProperty verifies SetMany/GetMany consistency
func TestCacheBatchOperationConsistencyProperty(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	property := func(sessionCount int, baseKey string) bool {
		// Limit the number of sessions to reasonable bounds
		if sessionCount < 1 || sessionCount > 10 || baseKey == "" {
			return true
		}

		// Generate test sessions
		sessions := make([]*testintegration.TestSession, sessionCount)
		keys := make([]string, sessionCount)

		for i := 0; i < sessionCount; i++ {
			sessionID := fmt.Sprintf("%s_session_%d", baseKey, i)
			sessions[i] = &testintegration.TestSession{
				ID:       sessionID,
				UserID:   fmt.Sprintf("user_%d", i),
				Username: fmt.Sprintf("username_%d", i),
				Created:  time.Now(),
			}
			keys[i] = sessionID
		}

		// Set all sessions using batch operation
		err := cache.SetMany(ctx, sessions, time.Hour)
		if err != nil {
			t.Logf("SetMany failed: %v", err)
			return false
		}

		// Get all sessions using batch operation
		retrieved, err := cache.GetMany(ctx, keys)
		if err != nil {
			t.Logf("GetMany failed: %v", err)
			return false
		}

		// Verify consistency: all set sessions should be retrieved
		if len(retrieved) != len(sessions) {
			t.Logf("Batch consistency failed: set %d sessions, got %d", len(sessions), len(retrieved))
			return false
		}

		// Verify each session matches what we set
		for _, session := range sessions {
			retrievedSession, exists := retrieved[session.ID]
			if !exists {
				t.Logf("Session %s not found in batch get results", session.ID)
				return false
			}
			if retrievedSession.UserID != session.UserID {
				t.Logf("Session %s UserID mismatch: expected %s, got %s",
					session.ID, session.UserID, retrievedSession.UserID)
				return false
			}
		}

		return true
	}

	config_quick := &quick.Config{
		MaxCount: DefaultPropertyConfig().MaxCount,
		Rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	if err := quick.Check(property, config_quick); err != nil {
		t.Errorf("Batch operation consistency property failed: %v", err)
	}
}

// TestCacheCounterInvariantsProperty verifies mathematical properties of counter operations
func TestCacheCounterInvariantsProperty(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	property := func(key string, delta1 int64, delta2 int64) bool {
		// Skip empty keys and very large deltas that might cause overflow
		if key == "" || delta1 > 1000000 || delta2 > 1000000 || delta1 < -1000000 || delta2 < -1000000 {
			return true
		}

		// Ensure clean state
		cache.Delete(ctx, key)

		// Test: Increment by delta1, then by delta2 should equal increment by (delta1 + delta2)
		val1, err := cache.Increment(ctx, key, delta1)
		if err != nil {
			t.Logf("First increment failed for key %s: %v", key, err)
			return false
		}
		if val1 != delta1 {
			t.Logf("First increment result mismatch: expected %d, got %d", delta1, val1)
			return false
		}

		val2, err := cache.Increment(ctx, key, delta2)
		if err != nil {
			t.Logf("Second increment failed for key %s: %v", key, err)
			return false
		}

		expectedFinal := delta1 + delta2
		if val2 != expectedFinal {
			t.Logf("Counter invariant failed: %d + %d = %d, but got %d", delta1, delta2, expectedFinal, val2)
			return false
		}

		// Test: Decrement should reverse the increment
		val3, err := cache.Decrement(ctx, key, expectedFinal)
		if err != nil {
			t.Logf("Decrement failed for key %s: %v", key, err)
			return false
		}

		if val3 != 0 {
			t.Logf("Decrement invariant failed: expected 0 after decrementing by total, got %d", val3)
			return false
		}

		return true
	}

	config_quick := &quick.Config{
		MaxCount: DefaultPropertyConfig().MaxCount,
		Rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	if err := quick.Check(property, config_quick); err != nil {
		t.Errorf("Counter invariants property failed: %v", err)
	}
}

// TestCacheConditionalOperationsProperty verifies SetIfExists/SetIfNotExists behavior
func TestCacheConditionalOperationsProperty(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	property := func(key string, userData string) bool {
		// Skip empty keys
		if key == "" {
			return true
		}

		// Ensure clean state
		cache.Delete(ctx, key)

		session1 := &testintegration.TestSession{
			ID:       key,
			UserID:   fmt.Sprintf("user1_%s", userData),
			Username: fmt.Sprintf("username1_%s", userData),
			Created:  time.Now(),
		}

		session2 := &testintegration.TestSession{
			ID:       key,
			UserID:   fmt.Sprintf("user2_%s", userData),
			Username: fmt.Sprintf("username2_%s", userData),
			Created:  time.Now(),
		}

		// Property 1: SetIfNotExists should succeed when key doesn't exist
		wasSet1, err := cache.SetIfNotExists(ctx, session1, time.Hour)
		if err != nil {
			t.Logf("SetIfNotExists failed for new key %s: %v", key, err)
			return false
		}
		if !wasSet1 {
			t.Logf("SetIfNotExists returned false for new key %s", key)
			return false
		}

		// Property 2: SetIfNotExists should fail when key exists
		wasSet2, err := cache.SetIfNotExists(ctx, session2, time.Hour)
		if err != nil {
			t.Logf("Second SetIfNotExists failed for existing key %s: %v", key, err)
			return false
		}
		if wasSet2 {
			t.Logf("SetIfNotExists returned true for existing key %s", key)
			return false
		}

		// Verify original value is unchanged
		retrieved, found, err := cache.Get(ctx, key)
		if err != nil || !found {
			t.Logf("Get failed after SetIfNotExists tests: err=%v, found=%v", err, found)
			return false
		}
		if retrieved.UserID != session1.UserID {
			t.Logf("SetIfNotExists modified existing value: expected %s, got %s", session1.UserID, retrieved.UserID)
			return false
		}

		// Property 3: SetIfExists should succeed when key exists
		wasSet3, err := cache.SetIfExists(ctx, session2, time.Hour)
		if err != nil {
			t.Logf("SetIfExists failed for existing key %s: %v", key, err)
			return false
		}
		if !wasSet3 {
			t.Logf("SetIfExists returned false for existing key %s", key)
			return false
		}

		// Delete the key for next test
		cache.Delete(ctx, key)

		// Property 4: SetIfExists should fail when key doesn't exist
		wasSet4, err := cache.SetIfExists(ctx, session1, time.Hour)
		if err != nil {
			t.Logf("SetIfExists failed for non-existent key %s: %v", key, err)
			return false
		}
		if wasSet4 {
			t.Logf("SetIfExists returned true for non-existent key %s", key)
			return false
		}

		return true
	}

	config_quick := &quick.Config{
		MaxCount: DefaultPropertyConfig().MaxCount,
		Rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	if err := quick.Check(property, config_quick); err != nil {
		t.Errorf("Conditional operations property failed: %v", err)
	}
}

// TestCacheKeyPatternProperty verifies pattern matching behavior
func TestCacheKeyPatternProperty(t *testing.T) {
	ctx := context.Background()
	setup := testintegration.SetupTestEnvironment(ctx, t)
	setup.ValidateEnvironment(ctx, t)
	setup.FlushRedis(ctx, t)

	config := testintegration.DefaultCacheConfig()
	cache, err := testintegration.CreateTestSessionCache(ctx, setup.RedisClient, config)
	require.NoError(t, err)
	defer cache.Close()

	property := func(prefix string, suffix string) bool {
		// Skip cases with special pattern characters or empty strings
		if prefix == "" || suffix == "" ||
			strings.Contains(prefix, "*") || strings.Contains(prefix, "?") ||
			strings.Contains(suffix, "*") || strings.Contains(suffix, "?") {
			return true
		}

		// Create test sessions with predictable keys
		testKeys := []string{
			fmt.Sprintf("%s_test_%s", prefix, suffix),
			fmt.Sprintf("%s_session_%s", prefix, suffix),
			fmt.Sprintf("other_%s_different", suffix),
		}

		sessions := make([]*testintegration.TestSession, len(testKeys))
		for i, key := range testKeys {
			sessions[i] = &testintegration.TestSession{
				ID:       key,
				UserID:   fmt.Sprintf("user_%d", i),
				Username: fmt.Sprintf("username_%d", i),
				Created:  time.Now(),
			}
		}

		// Set all sessions
		for _, session := range sessions {
			err := cache.Set(ctx, session, time.Hour)
			if err != nil {
				t.Logf("Set failed for key %s: %v", session.ID, err)
				return false
			}
		}

		// Test pattern matching
		pattern := fmt.Sprintf("%s_*", prefix)
		matchingKeys, err := cache.GetKeysByPattern(ctx, pattern)
		if err != nil {
			t.Logf("GetKeysByPattern failed for pattern %s: %v", pattern, err)
			return false
		}

		// Verify property: all keys matching the pattern should be returned
		expectedMatches := 0
		for _, key := range testKeys {
			if strings.HasPrefix(key, prefix+"_") {
				expectedMatches++
			}
		}

		if len(matchingKeys) != expectedMatches {
			t.Logf("Pattern matching failed: pattern %s expected %d matches, got %d",
				pattern, expectedMatches, len(matchingKeys))
			return false
		}

		// Cleanup
		for _, key := range testKeys {
			cache.Delete(ctx, key)
		}

		return true
	}

	config_quick := &quick.Config{
		MaxCount: DefaultPropertyConfig().MaxCount,
		Rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	if err := quick.Check(property, config_quick); err != nil {
		t.Errorf("Key pattern property failed: %v", err)
	}
}

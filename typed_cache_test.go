package cache

import (
	"context"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/internal/providers/memory"
	"github.com/MichaelAJay/go-serializer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSession represents a complex struct for testing gob serialization
type TestSession struct {
	ID        string            `json:"id"`
	UserID    string            `json:"user_id"`
	CreatedAt time.Time         `json:"created_at"`
	Metadata  map[string]string `json:"metadata"`
	Tags      []string          `json:"tags"`
}

func TestTypedCache_GobSerialization(t *testing.T) {
	ctx := context.Background()

	// Create a memory cache with gob (binary) serialization directly
	cacheOptions := &CacheOptions{
		TTL:              time.Hour,
		SerializerFormat: serializer.Binary, // Use gob serialization
	}
	
	cache, err := memory.NewMemoryCache(cacheOptions)
	require.NoError(t, err)
	defer cache.Close()

	// Create a typed cache wrapper for TestSession
	typedCache := NewTypedCache[*TestSession](cache)

	// Create a test session
	session := &TestSession{
		ID:        "test-123",
		UserID:    "user-456",
		CreatedAt: time.Now(),
		Metadata: map[string]string{
			"ip":     "192.168.1.1",
			"device": "desktop",
		},
		Tags: []string{"premium", "verified"},
	}

	// Test Set operation with type information
	err = typedCache.Set(ctx, "session:test", session, time.Hour)
	require.NoError(t, err, "Set should succeed with typed cache")

	// Test Get operation with type information
	retrievedSession, found, err := typedCache.Get(ctx, "session:test")
	require.NoError(t, err, "Get should succeed with typed cache")
	require.True(t, found, "Session should be found")
	require.NotNil(t, retrievedSession, "Retrieved session should not be nil")

	// Verify the retrieved session matches the original
	assert.Equal(t, session.ID, retrievedSession.ID)
	assert.Equal(t, session.UserID, retrievedSession.UserID)
	assert.Equal(t, session.Metadata["ip"], retrievedSession.Metadata["ip"])
	assert.Equal(t, session.Metadata["device"], retrievedSession.Metadata["device"])
	assert.Equal(t, len(session.Tags), len(retrievedSession.Tags))
	assert.Equal(t, session.Tags[0], retrievedSession.Tags[0])
	assert.Equal(t, session.Tags[1], retrievedSession.Tags[1])

	t.Logf("Successfully serialized and deserialized complex struct with gob using TypedCache")
}

func TestTypedCache_CompareWithRegularCache(t *testing.T) {
	ctx := context.Background()

	// Create a memory cache with gob (binary) serialization directly
	cacheOptions := &CacheOptions{
		TTL:              time.Hour,
		SerializerFormat: serializer.Binary, // Use gob serialization
	}
	
	regularCache, err := memory.NewMemoryCache(cacheOptions)
	require.NoError(t, err)
	defer regularCache.Close()

	// Create a typed cache wrapper for TestSession
	typedCache := NewTypedCache[*TestSession](regularCache)

	// Create a test session
	session := &TestSession{
		ID:        "test-456",
		UserID:    "user-789",
		CreatedAt: time.Now(),
		Metadata: map[string]string{
			"location": "NYC",
		},
		Tags: []string{"beta"},
	}

	// Test with regular cache (should fail with gob deserialization)
	err = regularCache.Set(ctx, "session:regular", session, time.Hour)
	require.NoError(t, err, "Set should work even with regular cache")

	// This should fail because gob can't deserialize to interface{} when it was serialized as a concrete type
	value, found, err := regularCache.Get(ctx, "session:regular")
	if found && err == nil {
		// If no error, check if we can type assert (usually fails with gob)
		_, ok := value.(*TestSession)
		if !ok {
			t.Logf("Regular cache found value but can't type assert to *TestSession: %T", value)
		}
	} else {
		t.Logf("Regular cache failed to deserialize: found=%v, err=%v", found, err)
	}

	// Test with typed cache (should succeed)
	err = typedCache.Set(ctx, "session:typed", session, time.Hour)
	require.NoError(t, err, "Typed cache Set should succeed")

	retrievedSession, found, err := typedCache.Get(ctx, "session:typed")
	require.NoError(t, err, "Typed cache Get should succeed")
	require.True(t, found, "Typed cache should find the session")
	require.NotNil(t, retrievedSession, "Typed cache should return valid session")
	
	assert.Equal(t, session.ID, retrievedSession.ID, "Typed cache should preserve all data correctly")
	
	t.Logf("TypedCache successfully handled complex struct serialization that regular cache struggled with")
}

func TestTypedCache_BulkOperations(t *testing.T) {
	ctx := context.Background()

	// Create a memory cache with gob (binary) serialization directly
	cacheOptions := &CacheOptions{
		TTL:              time.Hour,
		SerializerFormat: serializer.Binary, // Use gob serialization
	}
	
	cache, err := memory.NewMemoryCache(cacheOptions)
	require.NoError(t, err)
	defer cache.Close()

	// Create a typed cache wrapper for TestSession
	typedCache := NewTypedCache[*TestSession](cache)

	// Create multiple test sessions
	sessions := map[string]*TestSession{
		"session1": {
			ID:       "sess-1",
			UserID:   "user-1",
			Metadata: map[string]string{"type": "web"},
			Tags:     []string{"active"},
		},
		"session2": {
			ID:       "sess-2",
			UserID:   "user-2",
			Metadata: map[string]string{"type": "mobile"},
			Tags:     []string{"active", "premium"},
		},
	}

	// Test SetMany with type information
	err = typedCache.SetMany(ctx, sessions, time.Hour)
	require.NoError(t, err, "SetMany should succeed with typed cache")

	// Test GetMany with type information
	retrievedSessions, err := typedCache.GetMany(ctx, []string{"session1", "session2"})
	require.NoError(t, err, "GetMany should succeed with typed cache")
	require.Len(t, retrievedSessions, 2, "Should retrieve both sessions")

	// Verify the data integrity
	for key, originalSession := range sessions {
		retrievedSession, exists := retrievedSessions[key]
		require.True(t, exists, "Session %s should exist", key)
		assert.Equal(t, originalSession.ID, retrievedSession.ID)
		assert.Equal(t, originalSession.UserID, retrievedSession.UserID)
		assert.Equal(t, originalSession.Metadata["type"], retrievedSession.Metadata["type"])
		assert.Equal(t, len(originalSession.Tags), len(retrievedSession.Tags))
	}

	t.Logf("TypedCache bulk operations work correctly with gob serialization")
}

func TestTypedCache_GetAndUpdate(t *testing.T) {
	ctx := context.Background()

	// Create a memory cache with gob (binary) serialization directly
	cacheOptions := &CacheOptions{
		TTL:              time.Hour,
		SerializerFormat: serializer.Binary, // Use gob serialization
	}
	
	cache, err := memory.NewMemoryCache(cacheOptions)
	require.NoError(t, err)
	defer cache.Close()

	// Create a typed cache wrapper for TestSession
	typedCache := NewTypedCache[*TestSession](cache)

	// Create initial session
	session := &TestSession{
		ID:       "update-test",
		UserID:   "user-update",
		Metadata: map[string]string{"version": "1"},
		Tags:     []string{"initial"},
	}

	// Set initial session
	err = typedCache.Set(ctx, "session:update", session, time.Hour)
	require.NoError(t, err)

	// Test GetAndUpdate with type information
	updatedSession, err := typedCache.GetAndUpdate(ctx, "session:update", func(current *TestSession) (*TestSession, bool) {
		if current == nil {
			return nil, false
		}
		
		// Update the session
		updated := *current // Copy
		updated.Metadata["version"] = "2"
		updated.Tags = append(updated.Tags, "updated")
		return &updated, true
	}, time.Hour)

	require.NoError(t, err, "GetAndUpdate should succeed")
	require.NotNil(t, updatedSession, "Updated session should not be nil")
	
	// Verify the update
	assert.Equal(t, "update-test", updatedSession.ID)
	assert.Equal(t, "2", updatedSession.Metadata["version"])
	assert.Contains(t, updatedSession.Tags, "initial")
	assert.Contains(t, updatedSession.Tags, "updated")

	t.Logf("TypedCache GetAndUpdate works correctly with gob serialization")
}
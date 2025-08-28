//go:build integration
// +build integration

package redis

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/MichaelAJay/go-cache/internal/testenv"
	"github.com/MichaelAJay/go-serializer"
	"github.com/stretchr/testify/require"
)

// TestRedisContainer wraps testenv for Redis container management
type TestRedisContainer struct {
	testEnv *testenv.TestEnvironment
	ctx     context.Context
	t       *testing.T
}

// SetupRedisContainer creates a Redis container for testing
func SetupRedisContainer(t *testing.T) *TestRedisContainer {
	ctx := context.Background()

	// Create test environment (mode determined by environment variables)
	testEnv, err := testenv.NewTestEnvironmentFromEnv(ctx)
	if err != nil {
		t.Skipf("Test environment unavailable: %v", err)
	}

	return &TestRedisContainer{
		testEnv: testEnv,
		ctx:     ctx,
		t:       t,
	}
}

// GetRedisAddr returns the Redis address for testing
func (c *TestRedisContainer) GetRedisAddr() string {
	return c.testEnv.GetRedisAddr()
}

// FlushRedisData clears all data from Redis
func (c *TestRedisContainer) FlushRedisData() {
	// This would be implemented based on testenv's capabilities
	// For now, we assume the test environment provides clean state
}

// Close cleans up the test container
func (c *TestRedisContainer) Close() {
	c.testEnv.Close()
}

// CreateCacheForTesting creates a Redis cache instance for testing
func CreateCacheForTesting[T any](container *TestRedisContainer) interfaces.Cache[T] {
	return CreateCacheWithOptions[T](container, nil)
}

// CreateCacheWithOptions creates a Redis cache with specific options
func CreateCacheWithOptions[T any](container *TestRedisContainer, customOptions *interfaces.CacheOptions) interfaces.Cache[T] {
	options := &interfaces.CacheOptions{
		RedisOptions: &interfaces.RedisOptions{
			Address:  container.GetRedisAddr(),
			DB:       0,
			PoolSize: 10,
		},
		SerializerFormat: string(serializer.JSON),
		TTL:              time.Hour,
	}

	// Override with custom options if provided
	if customOptions != nil {
		if customOptions.RedisOptions != nil {
			if customOptions.RedisOptions.Address != "" {
				options.RedisOptions.Address = customOptions.RedisOptions.Address
			}
			if customOptions.RedisOptions.DB != 0 {
				options.RedisOptions.DB = customOptions.RedisOptions.DB
			}
			if customOptions.RedisOptions.PoolSize != 0 {
				options.RedisOptions.PoolSize = customOptions.RedisOptions.PoolSize
			}
			if customOptions.RedisOptions.Password != "" {
				options.RedisOptions.Password = customOptions.RedisOptions.Password
			}
		}
		if customOptions.SerializerFormat != "" {
			options.SerializerFormat = customOptions.SerializerFormat
		}
		if customOptions.TTL != 0 {
			options.TTL = customOptions.TTL
		}
		if customOptions.Security != nil {
			options.Security = customOptions.Security
		}
		if customOptions.Hooks != nil {
			options.Hooks = customOptions.Hooks
		}
		if customOptions.EnhancedMetrics != nil {
			options.EnhancedMetrics = customOptions.EnhancedMetrics
		}
	}

	cache, err := NewRedisCache[T](options)
	require.NoError(container.t, err)
	return cache
}

// Test data types and generators

type User struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Email    string                 `json:"email"`
	Created  time.Time              `json:"created"`
	Tags     []string               `json:"tags,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type SessionData struct {
	SessionID   string            `json:"session_id"`
	UserID      string            `json:"user_id"`
	Expires     time.Time         `json:"expires"`
	Permissions []string          `json:"permissions"`
	Data        map[string]string `json:"data"`
}

type LargeStruct struct {
	ID         string      `json:"id"`
	Data       [1024]byte  `json:"data"` // ~1KB
	Payload    []byte      `json:"payload"`
	Timestamps []time.Time `json:"timestamps"`
}

type HugeStruct struct {
	ID       string                 `json:"id"`
	Chunks   [100][1024]byte        `json:"chunks"` // ~100KB
	Metadata map[string]interface{} `json:"metadata"`
}

// GenerateTestUsers creates test user data
func GenerateTestUsers(count int) []*User {
	users := make([]*User, count)
	for i := 0; i < count; i++ {
		users[i] = &User{
			ID:      fmt.Sprintf("user-%d", i),
			Name:    fmt.Sprintf("User %d", i),
			Email:   fmt.Sprintf("user%d@test.com", i),
			Created: time.Now().Add(-time.Duration(i) * time.Hour),
			Tags:    []string{"tag1", "tag2"},
			Metadata: map[string]interface{}{
				"role":   "user",
				"active": true,
			},
		}
	}
	return users
}

// GenerateRandomStrings creates random string data for testing
func GenerateRandomStrings(count int, size int) []string {
	strings := make([]string, count)
	for i := 0; i < count; i++ {
		data := make([]byte, size)
		for j := range data {
			data[j] = byte(65 + (i+j)%26) // A-Z cycling
		}
		strings[i] = string(data)
	}
	return strings
}

// GenerateLargeObjects creates large test objects
func GenerateLargeObjects(count int, sizeKB int) []*LargeStruct {
	objects := make([]*LargeStruct, count)
	for i := 0; i < count; i++ {
		obj := &LargeStruct{
			ID:         fmt.Sprintf("large-%d", i),
			Payload:    make([]byte, sizeKB*1024),
			Timestamps: make([]time.Time, 100),
		}

		// Fill with test data
		for j := range obj.Payload {
			obj.Payload[j] = byte(i % 256)
		}
		for j := range obj.Timestamps {
			obj.Timestamps[j] = time.Now().Add(-time.Duration(j) * time.Second)
		}

		objects[i] = obj
	}
	return objects
}

// GenerateKeyPatterns creates key patterns for indexing tests
func GenerateKeyPatterns() []string {
	return []string{
		"user:*",
		"session:*",
		"cache:*",
		"data:user:*",
		"data:session:*",
		"index:*",
	}
}

// Test operation helpers

type Operation[T any] struct {
	Type     string
	Key      string
	Value    T
	Expected T
	TTL      time.Duration
}

// RunConcurrentOperations executes operations concurrently for stress testing
func RunConcurrentOperations[T any](cache interfaces.Cache[T], operations []Operation[T]) {
	// This would implement concurrent execution of operations
	// For testing race conditions and atomicity
}

// MeasureMemoryUsage measures memory usage before and after a function
func MeasureMemoryUsage(fn func()) (allocBytes int64, gcCount int) {
	runtime.GC()
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	fn()

	runtime.GC()
	runtime.ReadMemStats(&m2)

	return int64(m2.TotalAlloc - m1.TotalAlloc), int(m2.NumGC - m1.NumGC)
}

// ValidateNoMemoryLeaks checks for memory leaks between baseline and final measurements
func ValidateNoMemoryLeaks(baseline, final runtime.MemStats) bool {
	// Allow some tolerance for GC timing and small allocations
	const tolerance = 1024 * 1024 // 1MB tolerance

	allocated := int64(final.Alloc) - int64(baseline.Alloc)
	return allocated < tolerance
}

// TestTypes represents all the types we want to test
type TestTypes struct {
	String    string
	Int64     int64
	Float64   float64
	Bool      bool
	User      *User
	Session   SessionData
	ByteSlice []byte
	Map       map[string]interface{}
	Large     *LargeStruct
}

// GetTestValues returns sample values for each test type
func GetTestValues() TestTypes {
	return TestTypes{
		String:  "test-value",
		Int64:   42,
		Float64: 3.14159,
		Bool:    true,
		User: &User{
			ID:      "test-user",
			Name:    "Test User",
			Email:   "test@example.com",
			Created: time.Now(),
			Tags:    []string{"test", "user"},
		},
		Session: SessionData{
			SessionID:   "test-session",
			UserID:      "test-user",
			Expires:     time.Now().Add(time.Hour),
			Permissions: []string{"read", "write"},
			Data:        map[string]string{"key": "value"},
		},
		ByteSlice: []byte("test-bytes"),
		Map: map[string]interface{}{
			"string":  "value",
			"number":  42,
			"boolean": true,
		},
		Large: &LargeStruct{
			ID:         "test-large",
			Payload:    make([]byte, 1024),
			Timestamps: []time.Time{time.Now()},
		},
	}
}

// SerializationTestScenario defines a test case for serialization testing
type SerializationTestScenario[T any] struct {
	Name          string
	Value         T
	ExpectedValue T
	Format        string
	ShouldSucceed bool
}

// GetSerializationTestScenarios returns test scenarios for different serialization formats
func GetSerializationTestScenarios[T any](value T, expectedValue T) []SerializationTestScenario[T] {
	return []SerializationTestScenario[T]{
		{
			Name:          "JSON",
			Value:         value,
			ExpectedValue: expectedValue,
			Format:        string(serializer.JSON),
			ShouldSucceed: true,
		},
		{
			Name:          "Binary",
			Value:         value,
			ExpectedValue: expectedValue,
			Format:        string(serializer.Binary),
			ShouldSucceed: true,
		},
		{
			Name:          "MessagePack",
			Value:         value,
			ExpectedValue: expectedValue,
			Format:        string(serializer.Msgpack),
			ShouldSucceed: true,
		},
	}
}

// ConcurrencyTestConfig defines parameters for concurrency tests
type ConcurrencyTestConfig struct {
	Goroutines     int
	OperationsPerG int
	Duration       time.Duration
	ReadRatio      float64 // 0.0-1.0
	WriteRatio     float64 // 0.0-1.0 (should sum with ReadRatio to 1.0)
}

// DefaultConcurrencyConfigs returns standard concurrency test configurations
func DefaultConcurrencyConfigs() []ConcurrencyTestConfig {
	return []ConcurrencyTestConfig{
		{
			Goroutines:     10,
			OperationsPerG: 100,
			Duration:       5 * time.Second,
			ReadRatio:      0.7,
			WriteRatio:     0.3,
		},
		{
			Goroutines:     100,
			OperationsPerG: 50,
			Duration:       10 * time.Second,
			ReadRatio:      0.8,
			WriteRatio:     0.2,
		},
		{
			Goroutines:     1000,
			OperationsPerG: 10,
			Duration:       15 * time.Second,
			ReadRatio:      0.9,
			WriteRatio:     0.1,
		},
	}
}

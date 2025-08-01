package redis

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	goredis "github.com/go-redis/redis/v8"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/MichaelAJay/go-serializer"
)

// DefaultRedisVersion is the default Redis version to use for testing
const DefaultRedisVersion = "redis:8.0"

// SupportedRedisVersions lists all Redis versions we test against
var SupportedRedisVersions = []string{
	"redis:7.2", // Last stable 7.x for compatibility
	"redis:8.0", // Current stable (default)
	// "redis:8.2-rc", // Future compatibility (uncomment when stable)
}

// RedisTestContainer wraps testcontainer Redis instance with cache utilities
type RedisTestContainer struct {
	Container *tcredis.RedisContainer
	Client    *goredis.Client
	Cache     interfaces.Cache
	t         *testing.T
	cleanup   func()
}

// RedisContainerConfig holds configuration for Redis test container
type RedisContainerConfig struct {
	RedisVersion   string
	CacheOptions   *interfaces.CacheOptions
	ContainerOpts  []testcontainers.ContainerCustomizer
	EnableTLS      bool
	LogLevel       tcredis.LogLevel
	ConfigFile     string
	SnapshotConfig *SnapshotConfig
}

// SnapshotConfig configures Redis snapshotting
type SnapshotConfig struct {
	Seconds int
	Changes int
}

// RedisContainerOption allows customization of Redis container setup
type RedisContainerOption func(*RedisContainerConfig)

// WithRedisVersion sets the Redis Docker image version
func WithRedisVersion(version string) RedisContainerOption {
	return func(config *RedisContainerConfig) {
		config.RedisVersion = version
	}
}

// WithCacheOptions sets the cache configuration options
func WithCacheOptions(opts *interfaces.CacheOptions) RedisContainerOption {
	return func(config *RedisContainerConfig) {
		config.CacheOptions = opts
	}
}

// WithTLS enables TLS for the Redis container
func WithTLS() RedisContainerOption {
	return func(config *RedisContainerConfig) {
		config.EnableTLS = true
	}
}

// WithLogLevel sets the Redis log level
func WithLogLevel(level tcredis.LogLevel) RedisContainerOption {
	return func(config *RedisContainerConfig) {
		config.LogLevel = level
	}
}

// WithConfigFile sets a custom Redis configuration file
func WithConfigFile(path string) RedisContainerOption {
	return func(config *RedisContainerConfig) {
		config.ConfigFile = path
	}
}

// WithSnapshotting configures Redis snapshotting
func WithSnapshotting(seconds, changes int) RedisContainerOption {
	return func(config *RedisContainerConfig) {
		config.SnapshotConfig = &SnapshotConfig{
			Seconds: seconds,
			Changes: changes,
		}
	}
}

// WithCustomContainerOptions adds custom testcontainer options
func WithCustomContainerOptions(opts ...testcontainers.ContainerCustomizer) RedisContainerOption {
	return func(config *RedisContainerConfig) {
		config.ContainerOpts = append(config.ContainerOpts, opts...)
	}
}

// NewRedisTestContainer creates a new Redis test container with cache
func NewRedisTestContainer(t *testing.T, opts ...RedisContainerOption) (*RedisTestContainer, error) {
	t.Helper()

	// Default configuration
	config := &RedisContainerConfig{
		RedisVersion: DefaultRedisVersion,
		CacheOptions: &interfaces.CacheOptions{
			TTL:              time.Hour,
			SerializerFormat: serializer.Msgpack,
		},
		LogLevel: tcredis.LogLevelNotice,
	}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	ctx := context.Background()

	// Build testcontainer options
	containerOpts := []testcontainers.ContainerCustomizer{
		tcredis.WithLogLevel(config.LogLevel),
	}

	// Add TLS if enabled
	if config.EnableTLS {
		containerOpts = append(containerOpts, tcredis.WithTLS())
	}

	// Add snapshotting if configured
	if config.SnapshotConfig != nil {
		containerOpts = append(containerOpts,
			tcredis.WithSnapshotting(config.SnapshotConfig.Seconds, config.SnapshotConfig.Changes))
	}

	// Add config file if specified
	if config.ConfigFile != "" {
		containerOpts = append(containerOpts, tcredis.WithConfigFile(config.ConfigFile))
	}

	// Add custom container options
	containerOpts = append(containerOpts, config.ContainerOpts...)

	t.Logf("Starting Redis container with version %s", config.RedisVersion)

	// Start Redis container
	redisContainer, err := tcredis.Run(ctx, config.RedisVersion, containerOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to start Redis container: %w", err)
	}

	// Get connection string
	connectionString, err := redisContainer.ConnectionString(ctx)
	if err != nil {
		terminateContainer(redisContainer, t)
		return nil, fmt.Errorf("failed to get connection string: %w", err)
	}

	t.Logf("Redis container started, connection: %s", maskConnectionString(connectionString))

	// Parse connection options
	opts_redis, err := goredis.ParseURL(connectionString)
	if err != nil {
		terminateContainer(redisContainer, t)
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	// Configure TLS if enabled
	if config.EnableTLS {
		tlsConfig := redisContainer.TLSConfig()
		if tlsConfig != nil {
			opts_redis.TLSConfig = tlsConfig
		}
	}

	// Create Redis client
	client := goredis.NewClient(opts_redis)

	// Test connection with retry
	if err := testConnection(ctx, client, 3); err != nil {
		client.Close()
		terminateContainer(redisContainer, t)
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Create cache instance
	cacheInstance, err := NewRedisCache(client, config.CacheOptions)
	if err != nil {
		client.Close()
		terminateContainer(redisContainer, t)
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	// Create cleanup function
	cleanup := func() {
		if cacheInstance != nil {
			// Clear cache data
			if err := cacheInstance.Clear(context.Background()); err != nil {
				t.Logf("Warning: failed to clear cache during cleanup: %v", err)
			}
			cacheInstance.Close()
		}
		if client != nil {
			client.Close()
		}
		terminateContainer(redisContainer, t)
	}

	return &RedisTestContainer{
		Container: redisContainer,
		Client:    client,
		Cache:     cacheInstance,
		t:         t,
		cleanup:   cleanup,
	}, nil
}

// Cleanup terminates the container and cleans up resources
func (rtc *RedisTestContainer) Cleanup() {
	if rtc.cleanup != nil {
		rtc.cleanup()
	}
}

// GetConnectionString returns the Redis connection string
func (rtc *RedisTestContainer) GetConnectionString(ctx context.Context) (string, error) {
	return rtc.Container.ConnectionString(ctx)
}

// GetTLSConfig returns the TLS configuration if TLS is enabled
func (rtc *RedisTestContainer) GetTLSConfig() interface{} {
	return rtc.Container.TLSConfig()
}

// CreateUniqueTestID creates a unique prefix for test isolation
func CreateUniqueTestID(prefix string) string {
	timestamp := time.Now().Format("20060102150405.000")
	return fmt.Sprintf("%s:%s:", prefix, timestamp)
}

// Helper functions

// terminateContainer safely terminates a Redis container
func terminateContainer(container *tcredis.RedisContainer, t *testing.T) {
	if container != nil {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Logf("Warning: failed to terminate Redis container: %v", err)
		}
	}
}

// testConnection tests Redis connection with retry logic
func testConnection(ctx context.Context, client *goredis.Client, maxRetries int) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		ctxTimeout, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := client.Ping(ctxTimeout).Err()
		cancel()

		if err == nil {
			return nil
		}

		lastErr = err
		if i < maxRetries-1 {
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
		}
	}
	return fmt.Errorf("failed to connect after %d retries: %w", maxRetries, lastErr)
}

// maskConnectionString masks sensitive information in connection strings for logging
func maskConnectionString(connStr string) string {
	// Simple masking - in production, use more sophisticated masking
	if strings.Contains(connStr, "@") {
		parts := strings.Split(connStr, "@")
		if len(parts) == 2 {
			return fmt.Sprintf("redis://***@%s", parts[1])
		}
	}
	return connStr
}

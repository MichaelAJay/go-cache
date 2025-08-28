# testenv - Test Environment Management for go-cache

This package provides comprehensive test environment management for go-cache, enabling isolated Redis testing with multiple deployment scenarios including live local Redis instances, containerized environments, and latency simulation.

## Overview

The `testenv` package abstracts away the complexity of setting up different test environments, allowing tests to run against:

- **Live local Redis** instances (direct mode)
- **Isolated Redis containers** (container mode) 
- **Docker-compose services** (compose mode)
- **Latency-simulated connections** (via Toxiproxy)

## Quick Start

### Basic Usage

```go
//go:build integration
package mytest

import (
    "context"
    "testing"
    "github.com/MichaelAJay/go-cache/internal/testenv"
    "github.com/redis/go-redis/v9"
)

func TestRedisOperations(t *testing.T) {
    ctx := context.Background()
    
    // Create test environment (mode determined by environment variables)
    testEnv, err := testenv.NewTestEnvironmentFromEnv(ctx)
    if err != nil {
        t.Skipf("Test environment unavailable: %v", err)
    }
    defer testEnv.Close()
    
    // Connect to Redis using the test environment
    rdb := redis.NewClient(&redis.Options{
        Addr: testEnv.GetRedisAddr(),
    })
    defer rdb.Close()
    
    // Your test logic here
    err = rdb.Set(ctx, "test:key", "value", time.Minute).Err()
    require.NoError(t, err)
}
```

## Test Modes

### 1. Direct Mode (Live Local Redis)

**Use Case**: Development with existing Redis instance

```go
// Explicit direct mode
testEnv, err := testenv.NewTestEnvironment(ctx, testenv.ModeDirect)

// Or via environment
// GOCACHE_TEST_MODE=direct (or unset)
testEnv, err := testenv.NewTestEnvironmentFromEnv(ctx)
```

**Configuration**:
- Uses Redis at `localhost:6379` by default
- Override with `REDIS_ADDR` environment variable
- No cleanup required (external service)
- Fastest startup time

**Example**:
```bash
# Use local Redis (must be running)
redis-server --port 6379

# Run tests
GOCACHE_TEST_MODE=direct go test -tags=integration ./...

# Or with custom address
REDIS_ADDR=localhost:6380 go test -tags=integration ./...
```

### 2. Container Mode (Isolated Testing)

**Use Case**: Isolated testing with guaranteed clean state

```go
// Explicit container mode
testEnv, err := testenv.NewTestEnvironment(ctx, testenv.ModeContainers)

// Or via environment
// GOCACHE_TEST_MODE=containers
testEnv, err := testenv.NewTestEnvironmentFromEnv(ctx)
```

**Features**:
- Fresh Redis container for each test environment
- Complete isolation between test runs
- Automatic cleanup when `testEnv.Close()` is called
- Slower startup but guaranteed clean state

**Example**:
```bash
# Run with fresh containers (Docker required)
GOCACHE_TEST_MODE=containers go test -tags=integration ./...

# With custom timeout
GOCACHE_TEST_CONTAINER_TIMEOUT=120s GOCACHE_TEST_MODE=containers go test -tags=integration ./...
```

### 3. Compose Mode (Persistent Services)

**Use Case**: Faster testing with shared, persistent services

```go
// Explicit compose mode
testEnv, err := testenv.NewTestEnvironment(ctx, testenv.ModeCompose)

// Or via environment
// GOCACHE_TEST_MODE=compose
testEnv, err := testenv.NewTestEnvironmentFromEnv(ctx)
```

**Setup**:
```bash
# Start services once
docker compose up -d

# Wait for services to be ready
cd scripts && go run wait-for-services.go

# Run multiple test sessions
GOCACHE_TEST_MODE=compose go test -tags=integration ./...

# Cleanup when done
docker compose down -v
```

## Service Validation

The package includes robust service validation to ensure Redis connectivity before running tests:

### Manual Validation

```go
validator := testenv.NewServiceValidator(testenv.ModeDirect)
err := validator.ValidateServices(ctx, testenv.ModeDirect)
if err != nil {
    t.Skipf("Redis not available: %v", err)
}
```

### Automatic Validation

```bash
# Validate docker-compose services
cd scripts && go run validate-services.go

# From Makefile
make docker-validate
```

## Latency Simulation with Toxiproxy

For realistic network testing, the package supports latency injection via Toxiproxy:

### Basic Latency Testing

```go
func TestRedisWithLatency(t *testing.T) {
    // Enable latency mode
    os.Setenv("GOCACHE_TEST_LATENCY", "enabled")
    os.Setenv("GOCACHE_TEST_REDIS_LATENCY_MS", "50")
    
    testEnv, err := testenv.NewTestEnvironmentFromEnv(ctx)
    require.NoError(t, err)
    defer testEnv.Close()
    
    if !testEnv.HasToxiproxy() {
        t.Skip("Toxiproxy not available")
    }
    
    // Operations will now have added latency
    rdb := redis.NewClient(&redis.Options{
        Addr: testEnv.GetRedisAddr(), // Points to proxy
    })
    defer rdb.Close()
    
    start := time.Now()
    err = rdb.Ping(ctx).Err()
    duration := time.Since(start)
    
    // Should take at least 50ms due to latency
    assert.GreaterOrEqual(t, duration.Milliseconds(), int64(40))
}
```

### Dynamic Latency Control

```go
toxiController := testEnv.GetToxiproxyController()
if toxiController != nil {
    // Add 100ms latency
    err := toxiController.AddLatencyToxic(ctx, "redis_proxy", 100)
    
    // Test operations with latency...
    
    // Remove latency
    err = toxiController.RemoveToxic(ctx, "redis_proxy", "redis_proxy_latency")
}
```

### Command Line Latency Testing

```bash
# Using docker-compose with latency
docker compose up -d

# Configure latency
make docker-configure-latency LATENCY_MS=100

# Run tests with latency
GOCACHE_TEST_MODE=compose GOCACHE_TEST_LATENCY=enabled go test -tags=integration ./...

# Using containers with latency
GOCACHE_TEST_MODE=containers GOCACHE_TEST_LATENCY=enabled GOCACHE_TEST_REDIS_LATENCY_MS=50 go test -tags=integration ./...
```

## Environment Variables

### Test Mode Configuration
- `GOCACHE_TEST_MODE`: `direct|containers|compose` (default: `direct`)
- `GOCACHE_TEST_CONTAINER_TIMEOUT`: Container startup timeout (default: `60s`)

### Service Configuration  
- `REDIS_ADDR`: Redis address for direct mode (default: `localhost:6379`)

### Latency Configuration
- `GOCACHE_TEST_LATENCY`: `enabled` to activate Toxiproxy latency simulation
- `GOCACHE_TEST_REDIS_LATENCY_MS`: Latency to add in milliseconds
- `GOCACHE_TEST_TOXIPROXY`: Alternative to `GOCACHE_TEST_LATENCY`

## Advanced Usage

### Custom Test Environments

```go
func TestCustomRedis(t *testing.T) {
    // Create environment with specific mode
    testEnv, err := testenv.NewTestEnvironment(ctx, testenv.ModeContainers)
    require.NoError(t, err)
    defer testEnv.Close()
    
    // Get connection details
    redisAddr := testEnv.GetRedisAddr()
    mode := testEnv.GetMode()
    
    t.Logf("Testing with Redis at %s in %s mode", redisAddr, mode.String())
    
    // Custom Redis configuration
    rdb := redis.NewClient(&redis.Options{
        Addr:        redisAddr,
        DB:          1,  // Use different database
        MaxRetries:  3,
        PoolSize:    10,
    })
    defer rdb.Close()
    
    // Test operations...
}
```

### Conditional Testing Based on Mode

```go
func TestModeSpecificBehavior(t *testing.T) {
    testEnv, err := testenv.NewTestEnvironmentFromEnv(ctx)
    require.NoError(t, err)
    defer testEnv.Close()
    
    switch testEnv.GetMode() {
    case testenv.ModeDirect:
        // Test with assumption of persistent data
        t.Log("Testing against persistent Redis")
        
    case testenv.ModeContainers, testenv.ModeCompose:
        // Test with clean slate
        t.Log("Testing against clean Redis instance")
        
        // Can safely test destructive operations
        testDestructiveOperations(t, testEnv)
    }
}
```

### Service Health Checks

```go
func TestServiceHealth(t *testing.T) {
    testEnv, err := testenv.NewTestEnvironmentFromEnv(ctx)
    require.NoError(t, err)
    defer testEnv.Close()
    
    // Manual health check
    validator := &testenv.ServiceValidator{
        RedisAddr:    testEnv.GetRedisAddr(),
        ToxiproxyAPI: "http://localhost:8474",
    }
    
    err = validator.ValidateServices(ctx, testEnv.GetMode())
    require.NoError(t, err, "Services should be healthy")
    
    // Additional health checks...
}
```

## Integration with Test Suites

### Table-Driven Tests Across Modes

```go
func TestRedisAcrossModes(t *testing.T) {
    modes := []struct {
        name string
        mode testenv.TestMode
        skip bool
    }{
        {"Direct", testenv.ModeDirect, !isRedisAvailable()},
        {"Container", testenv.ModeContainers, !isDockerAvailable()},
        {"Compose", testenv.ModeCompose, !isComposeRunning()},
    }
    
    for _, tt := range modes {
        t.Run(tt.name, func(t *testing.T) {
            if tt.skip {
                t.Skipf("Skipping %s mode - requirements not met", tt.name)
            }
            
            testEnv, err := testenv.NewTestEnvironment(ctx, tt.mode)
            require.NoError(t, err)
            defer testEnv.Close()
            
            // Run the same test logic across all modes
            testRedisBasicOperations(t, testEnv)
        })
    }
}
```

### Benchmark Testing with Latency

```go
func BenchmarkRedisOperations(b *testing.B) {
    latencies := []int{0, 10, 50, 100}
    
    for _, latency := range latencies {
        b.Run(fmt.Sprintf("Latency%dms", latency), func(b *testing.B) {
            ctx := context.Background()
            testEnv := setupBenchmarkEnv(b, latency)
            defer testEnv.Close()
            
            rdb := redis.NewClient(&redis.Options{
                Addr: testEnv.GetRedisAddr(),
            })
            defer rdb.Close()
            
            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                key := fmt.Sprintf("bench:key:%d", i)
                err := rdb.Set(ctx, key, "value", time.Minute).Err()
                if err != nil {
                    b.Fatal(err)
                }
            }
        })
    }
}
```

## Error Handling and Debugging

### Common Issues and Solutions

1. **Redis Connection Refused**
   ```go
   testEnv, err := testenv.NewTestEnvironmentFromEnv(ctx)
   if err != nil {
       // In direct mode, this likely means Redis isn't running
       if testenv.GetTestModeFromEnv() == testenv.ModeDirect {
           t.Skip("Redis not available - start with: redis-server")
       }
       t.Fatalf("Failed to create test environment: %v", err)
   }
   ```

2. **Container Startup Timeout**
   ```bash
   # Increase timeout for slow systems
   GOCACHE_TEST_CONTAINER_TIMEOUT=120s go test -tags=integration ./...
   ```

3. **Docker Issues**
   ```go
   // Check if Docker is available before container tests
   if testenv.GetTestModeFromEnv() == testenv.ModeContainers {
       if !isDockerRunning() {
           t.Skip("Docker not available")
       }
   }
   ```

### Debug Logging

```go
func TestWithDebugInfo(t *testing.T) {
    testEnv, err := testenv.NewTestEnvironmentFromEnv(ctx)
    require.NoError(t, err)
    defer testEnv.Close()
    
    t.Logf("Test mode: %s", testEnv.GetMode().String())
    t.Logf("Redis address: %s", testEnv.GetRedisAddr())
    t.Logf("Has Toxiproxy: %t", testEnv.HasToxiproxy())
    
    if testEnv.HasToxiproxy() {
        controller := testEnv.GetToxiproxyController()
        t.Logf("Toxiproxy controller: %+v", controller)
    }
}
```

## Performance Considerations

### Mode Performance Characteristics

| Mode | Startup Time | Test Isolation | Best For |
|------|-------------|----------------|----------|
| Direct | ~1ms | Low | Development, fast iteration |
| Compose | ~1-2s | Medium | CI/CD, shared testing |
| Container | ~5-10s | High | Isolated testing, parallel tests |

### Optimization Tips

1. **Development**: Use direct mode with local Redis
2. **CI/CD**: Use compose mode for faster pipeline execution
3. **Integration**: Use container mode for complete isolation
4. **Parallel Tests**: Container mode prevents interference
5. **Latency Testing**: Only enable when specifically needed

## Files Overview

### Core Files
- `factory.go`: Test environment factory and mode management
- `validation.go`: Service validation and health checking
- `direct.go`: Direct connection to existing Redis
- `compose.go`: Docker-compose service integration
- `containers.go`: Testcontainers-based isolated environments
- `toxiproxy.go`: Latency simulation and proxy management

### Usage Examples
- `../demo_container.go`: Comprehensive demo of all features
- `../integration_test.go`: Integration test examples
- `../scripts/`: Helper scripts for service management

This package provides a robust foundation for testing Redis-based cache implementations across different deployment scenarios while maintaining test reliability and developer productivity.
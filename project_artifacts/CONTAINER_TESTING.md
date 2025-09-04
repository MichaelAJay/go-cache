# Container Testing for go-cache

This module now supports comprehensive container-based testing similar to go-auth, with multiple test environments and latency simulation capabilities.

## Test Modes

### 1. Container Mode (Testcontainers)
- **Environment**: `GOCACHE_TEST_MODE=containers`
- **Description**: Cold start fresh containers for each test run
- **Use Case**: Isolated testing with guaranteed clean state
- **Command**: `make test-containers`

### 2. Docker Compose Mode
- **Environment**: `GOCACHE_TEST_MODE=compose`
- **Description**: Uses persistent docker-compose services
- **Use Case**: Faster testing with shared infrastructure
- **Setup**: `make docker-up`
- **Command**: `make docker-test`

### 3. Direct Mode (Default)
- **Environment**: `GOCACHE_TEST_MODE=direct` (or unset)
- **Description**: Connects to existing Redis instance
- **Use Case**: Development with local Redis
- **Command**: `make test-redis`

## Latency Simulation

The testing framework includes Toxiproxy integration for network latency simulation:

### Basic Latency Testing
```bash
# Enable latency testing
export GOCACHE_TEST_LATENCY=enabled
export GOCACHE_TEST_REDIS_LATENCY_MS=100

# Run tests with latency
make docker-test-latency
```

### Configure Latency Dynamically
```bash
# Set specific latency values
make docker-configure-latency LATENCY_MS=50

# Test with custom latency
GOCACHE_TEST_MODE=compose GOCACHE_TEST_LATENCY=enabled go run demo_container.go
```

## Architecture

### Components
- **testenv.TestEnvironment**: Factory for creating test environments
- **testenv.ServiceValidator**: Validates service availability
- **testenv.ToxiproxyController**: Manages latency simulation
- **docker-compose.yml**: Defines Redis + Toxiproxy services
- **Makefile**: Provides convenient test commands

### File Structure
```
go-cache/
├── docker-compose.yml              # Service definitions
├── Makefile                       # Test commands
├── internal/testenv/              # Test environment management
│   ├── factory.go                 # Environment factory
│   ├── validation.go             # Service validation
│   ├── containers.go             # Testcontainers implementation
│   ├── compose.go                # Docker-compose implementation
│   ├── direct.go                 # Direct connection implementation
│   └── toxiproxy.go              # Latency simulation
├── scripts/                      # Helper scripts
│   ├── wait-for-services.go      # Service readiness checker
│   ├── validate-services.go      # Service validation
│   └── setup-toxiproxy.go        # Proxy configuration
└── demo_container.go             # Testing demo
```

## Usage Examples

### Quick Demo
```bash
# Test container functionality
go run demo_container.go -h                    # Show help
go run demo_container.go                       # Direct mode (may fail if no Redis)
GOCACHE_TEST_MODE=containers go run demo_container.go  # Container mode
```

### Development Workflow
```bash
# Start services
make docker-up

# Validate services
make docker-validate

# Run tests
make docker-test

# Test with latency
make docker-configure-latency LATENCY_MS=100
make docker-test-latency

# Cleanup
make docker-down
```

### Integration Testing
```bash
# Cold start containers (slow but isolated)
make test-containers

# Use persistent services (faster)
make docker-up && make docker-test

# Memory-only testing (fastest)
make test-fast
```

## Performance Characteristics

Based on demo results:

### Normal Redis Operations
- PING: ~2-10ms
- SET: ~300µs
- GET: ~200-300µs

### With 100ms Latency Simulation
- PING: ~100-330ms
- SET: ~100-105ms  
- GET: ~100-105ms

## Environment Variables

### Test Mode Control
- `GOCACHE_TEST_MODE`: `direct|containers|compose`
- `GOCACHE_TEST_CONTAINER_TIMEOUT`: Container startup timeout (default: 60s)

### Latency Control
- `GOCACHE_TEST_LATENCY`: `enabled` to activate latency simulation
- `GOCACHE_TEST_REDIS_LATENCY_MS`: Redis latency in milliseconds

### Service Configuration
- `REDIS_ADDR`: Redis address for direct mode (default: localhost:6379)

## Benefits

1. **Multiple Test Environments**: Choose the right environment for each use case
2. **Latency Simulation**: Test real-world network conditions
3. **Automated Setup**: Scripts handle service orchestration
4. **Performance Testing**: Built-in latency measurement
5. **Clean Isolation**: Container mode ensures no test interference
6. **Easy Integration**: Compatible with existing CI/CD pipelines

## Integration with CI/CD

The container testing can be easily integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions
- name: Run Container Tests
  run: make test-containers
  
- name: Run Docker Compose Tests
  run: |
    make docker-up
    make docker-test
    make docker-down
```

This testing infrastructure provides the foundation for robust, realistic testing of the go-cache module across different deployment scenarios.
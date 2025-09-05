# Go-Cache Testing Primer

## Quick Testing Commands

### Running Without Latency (Fast)

**Memory-only testing** (fastest):
```bash
make test-fast
# OR
GOCACHE_TEST_PRESET=fast go test -tags=integration ./...
```

**Docker-compose without latency**:
```bash
make docker-up && make docker-test-fast
# OR 
GOCACHE_TEST_MODE=compose GOCACHE_TEST_PRESET=fast go test -tags=integration ./...
```

**Container testing without latency**:
```bash
make test-containers
# OR
GOCACHE_TEST_MODE=containers go test -tags=integration ./...
```

### Running With Latency (Realistic Network Conditions)

**Docker-compose with latency**:
```bash
make docker-up
make docker-test-latency
# OR
GOCACHE_TEST_MODE=compose GOCACHE_TEST_LATENCY=enabled go test -tags=integration ./...
```

**Container testing with custom latency**:
```bash
GOCACHE_TEST_MODE=containers GOCACHE_TEST_LATENCY=enabled GOCACHE_TEST_REDIS_LATENCY_MS=50 go test -tags=integration ./...
```

**Configure specific latency**:
```bash
make docker-configure-latency LATENCY_MS=100
```

## Key Environment Variables

- `GOCACHE_TEST_MODE`: `direct|containers|compose` (testing mode)
- `GOCACHE_TEST_PRESET`: `fast|memory|realistic|production` (performance profile)
- `GOCACHE_TEST_LATENCY=enabled`: Enable network latency simulation
- `GOCACHE_TEST_REDIS_LATENCY_MS`: Latency in milliseconds (e.g., 50, 100)

## Demo Usage

**Run demo without latency**:
```bash
go run demo_container.go
```

**Run demo with latency**:
```bash
GOCACHE_TEST_MODE=compose GOCACHE_TEST_LATENCY=enabled go run demo_container.go
```
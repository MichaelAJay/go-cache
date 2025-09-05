# Go Cache Benchmarks

This directory contains benchmark results and scripts for performance testing of the go-cache library.

## Benchmark Structure

The benchmark tests are organized into 4 logical groups:

### 1. Core Operations (`redis_cache_core_operations_benchmark_test.go`)
- Basic operations: Get, Set, Delete, Has, Clear (various data sizes)
- Atomic operations: GetOrSet, Update (cache hits/misses, contention)
- Conditional operations: SetIfExists, SetIfNotExists (high contention scenarios)

### 2. Batch & Concurrent Operations (`redis_cache_batch_concurrent_benchmark_test.go`)
- Batch operations: GetMany, SetMany, DeleteMany (10, 100, 1000 items)
- Concurrent operations: High concurrency reads/writes (10, 100, 1000 goroutines)
- Mixed workloads: Realistic usage patterns (70% reads, 20% writes, 10% deletes)

### 3. Feature Performance (`redis_cache_features_benchmark_test.go`)
- Indexing impact: Performance with/without indexing enabled
- Serialization overhead: JSON vs Gob vs MessagePack comparison
- Lua script warming: Performance impact of script pre-loading vs on-demand

### 4. System Performance (`redis_cache_system_benchmark_test.go`)
- Circuit breaker overhead: Normal vs open vs recovery states
- Memory allocation patterns: GC pressure and allocation tracking
- Connection pooling: High concurrency connection efficiency

## Running Benchmarks

### Quick Start

Run all benchmarks:
```bash
./scripts/run_benchmarks.sh
```

> **Note**: Benchmarks require the `integration` build tag and depend on Redis/Docker infrastructure.

Run specific category:
```bash
./scripts/run_benchmarks.sh results_core.txt core
./scripts/run_benchmarks.sh results_batch.txt batch
./scripts/run_benchmarks.sh results_features.txt features
./scripts/run_benchmarks.sh results_system.txt system
```

### Customizing Benchmark Runs

Set environment variables to control benchmark execution:
```bash
# Run each benchmark for 5 seconds, repeat 10 times
BENCHTIME=5s COUNT=10 ./scripts/run_benchmarks.sh baseline.txt

# Quick benchmark run (100ms each, 1 iteration)
BENCHTIME=100ms COUNT=1 ./scripts/run_benchmarks.sh quick.txt
```

### Network Latency Testing with Toxiproxy

Test performance under realistic network conditions using containers mode with Toxiproxy:

```bash
# Run latency benchmarks with specific latency (fully automated)
./scripts/run_latency_benchmarks.sh 50 core      # 50ms latency, core operations
./scripts/run_latency_benchmarks.sh 100 batch    # 100ms latency, batch operations
./scripts/run_latency_benchmarks.sh 200 all      # 200ms latency, all benchmarks
```

**Available Latency Scenarios:**
- **Local/LAN**: 1-5ms latency
- **Regional**: 10-50ms latency  
- **Cross-country**: 50-100ms latency
- **International**: 100-300ms latency
- **Satellite/Poor network**: 500ms+ latency

**How It Works:**
- Uses **compose mode** for fast, reliable latency benchmarking
- Persistent Docker Compose services (Redis + Toxiproxy) shared across benchmarks
- Automatic state cleanup between benchmarks (Redis flush + toxiproxy reset)
- No container recreation overhead - services stay running throughout benchmark session

**Prerequisites for Latency Testing:**
- Docker and Docker Compose must be installed
- Services started automatically or manually with `docker compose up -d`
- Automatic toxiproxy conflict resolution and state management

### Comparing Results

Use the comparison script to analyze performance changes:
```bash
./scripts/compare_benchmarks.sh benchmarks/baseline.txt benchmarks/current.txt
```

This uses `benchstat` to provide statistical analysis of performance differences.

**Latency Comparison Example:**
```bash
# Compare baseline vs latency performance
./scripts/run_benchmarks.sh baseline.txt core
./scripts/run_latency_benchmarks.sh 100 core  # Creates latency_100ms_*.txt
./scripts/compare_benchmarks.sh benchmarks/baseline.txt benchmarks/latency_100ms_*.txt
```

**Expected Results:**
- **Baseline (no latency)**: ~50,000 ns/op for Get_1KB operations
- **100ms latency**: ~103,000,000 ns/op (100ms network + ~3ms Redis)
- **Batch operations**: Much better scaling under latency (GetMany vs individual Gets)

## Interpreting Results

### Benchmark Output Format
```
BenchmarkRedisCache_Get_1KB-10    	    2268	     50050 ns/op	    1024 B/op	      15 allocs/op
```

- `2268`: Number of iterations
- `50050 ns/op`: Nanoseconds per operation
- `1024 B/op`: Bytes allocated per operation  
- `15 allocs/op`: Number of allocations per operation

### Statistical Comparison
When using `benchstat`, look for:
- **Performance changes**: `+10.2%` (faster) or `-5.1%` (slower)
- **Statistical significance**: p-values < 0.05 indicate reliable changes
- **Symbols**: `+` (improvement), `-` (regression), `~` (no change)

### Understanding Latency Impact
Network latency typically affects operations differently:
- **Single operations**: Performance degrades linearly with latency (50ms latency ≈ +50ms per operation)
- **Batch operations**: Much better efficiency under latency (GetMany/SetMany scale better)
- **Concurrent operations**: May help amortize latency through connection pooling
- **Pipeline operations**: Show the biggest improvements under high latency

## Best Practices

1. **Establish baselines**: Run benchmarks before making changes
2. **Multiple iterations**: Use `COUNT=5` or higher for reliable results
3. **Consistent environment**: Run benchmarks on the same hardware
4. **Monitor trends**: Track performance over time, not just single comparisons
5. **Focus on important metrics**: Core operations and your specific use case
6. **Test realistic conditions**: Use latency testing to simulate production network conditions
7. **Consider operation patterns**: Batch operations become critical under high latency
8. **Integration setup**: Benchmarks require Redis infrastructure - ensure Docker services are running
9. **Benchmark duration**: Use longer `BENCHTIME` (1s+) for stable results, shorter (100ms) for quick tests
10. **Proxy persistence**: Toxiproxy configurations may be cleared between runs - scripts handle re-setup automatically
11. **Latency validation**: Test single operations first to verify latency is working before running full suites

## Benchmark Categories Explained

- **Core Operations**: Essential for everyday cache usage performance
- **Batch & Concurrent**: Important for high-throughput applications
- **Features**: Helps choose optimal configuration (serialization, indexing, etc.)
- **System**: Critical for production resilience and resource management

## Files in this Directory

- `baseline_YYYYMMDD_HHMMSS.txt`: Reference benchmark results (no latency)
- `latency_XXXms_YYYYMMDD_HHMMSS.txt`: Latency benchmark results 
- `feature_branch_YYYYMMDD_HHMMSS.txt`: Results from specific feature development
- `*_comparison.txt`: Benchmark comparison outputs
- `example_*.txt`: Sample benchmark files for testing comparison tools

## Available Scripts

- **`run_benchmarks.sh`**: Main benchmark runner with category filtering
  - Supports environment variables: `BENCHTIME`, `COUNT`, `LATENCY_MODE`, `LATENCY_MS`
  - Automatically uses integration build tags and redirects errors to output
- **`run_latency_benchmarks.sh`**: Automated latency testing with Toxiproxy
  - Handles Docker service startup and proxy configuration  
  - Intelligently checks and updates existing proxy settings
  - Automatically compares with baseline if available
- **`compare_benchmarks.sh`**: Statistical comparison using benchstat
  - Requires Go benchstat tool (auto-installed if missing)
  - Provides statistical significance testing and performance deltas

## Troubleshooting

**Benchmarks show `0` runs or `NaN` ns/op:**
- **Fixed in compose mode** - automatic state cleanup eliminates conflicts
- Verify integration tag is working: `go test -tags=integration -list "BenchmarkRedisCache_Get_1KB"`  
- Check Docker services: `docker compose ps` (should show Redis + Toxiproxy as Up)

**Latency benchmarks failing:**
- **Compose mode handles state automatically** - toxiproxy conflicts resolved
- Check services are running: `docker compose ps`
- Restart services if needed: `docker compose restart`
- Verify toxiproxy is accessible: `curl http://localhost:8474/version`

**Service startup issues:**
- Start services manually: `docker compose up -d`
- Check service health: `docker compose logs redis toxiproxy`
- Reset if needed: `docker compose down && docker compose up -d`

**Benchmarks take very long or hang:**
- **Large datasets with latency**: 1000-key operations can take 60+ seconds with network latency
- Reduce `BENCHTIME` for faster testing: `BENCHTIME=100ms`
- Use specific categories instead of `all`: `./scripts/run_benchmarks.sh test.txt core`
- Skip large benchmarks for quick testing (focus on 1KB, 10KB operations)

## Quick Diagnostic Commands

```bash
# Check Docker is running
docker info

# List available benchmarks
go test -tags=integration -list "BenchmarkRedisCache_"

# Test single benchmark (baseline - no latency)
go test -tags=integration -bench="BenchmarkRedisCache_Get_1KB" -run=^$ -benchtime=100ms -count=1

# Test single benchmark (with latency using containers)
GOCACHE_TEST_MODE=containers GOCACHE_TEST_LATENCY=enabled go test -tags=integration -bench="BenchmarkRedisCache_Get_1KB" -run=^$ -benchtime=100ms -count=1

# Quick latency benchmark test
BENCHTIME=100ms COUNT=1 ./scripts/run_latency_benchmarks.sh 50 core
```

> **Note**: Benchmark files are gitignored by default. Uncomment the `# benchmarks/` line in `.gitignore` if you want to track benchmark history in git.
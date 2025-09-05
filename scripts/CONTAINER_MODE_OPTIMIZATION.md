# Container Mode Benchmark Optimization

## Problem Statement

Benchmarks using containers mode are timing out after 11+ minutes due to excessive container creation/destruction overhead. Each benchmark iteration creates fresh Redis + Toxiproxy containers, adding 15-30 seconds per benchmark test.

## Current Status
- ✅ Benchmarks produce accurate results with realistic latency simulation
- ✅ Container networking and Toxiproxy integration working correctly
- ❌ Performance is unacceptable for regular use (11+ minutes for core category)
- ❌ Tests timeout before completion

## Optimization Goals

Transform container mode from 11+ minute execution to ~2-3 minute execution for practical daily use.

## Step-by-Step Optimization Plan

### Step 1: Container Reuse Within Single Benchmark Run
**Goal**: Reuse the same Redis+Toxiproxy containers across multiple benchmark functions in a single category run.

**Current Behavior**: 
- Each `BenchmarkRedisCache_X` function creates new containers
- Containers destroyed after each benchmark completes

**Target Behavior**:
- Create containers once at the start of benchmark run
- All benchmark functions in the category share the same containers
- Destroy containers only at the end of the full run

**Definition of Done**:
- Single category run (e.g., `core`) creates containers only once
- All benchmarks in that category reuse the same containers
- Total container creation time reduced from ~N×30s to ~30s per category
- Benchmarks still produce accurate latency-simulated results

**Files to Modify**:
- `internal/testenv/containers.go` - Modify container lifecycle management
- `redis_cache_*_benchmark_test.go` - Adjust benchmark setup/teardown

---

### Step 2: Container Lifecycle Optimization
**Goal**: Reduce container startup time and improve container management efficiency.

**Current Issues**:
- Container startup includes unnecessary waiting periods
- Redundant health checks and validation steps
- Network creation/destruction overhead

**Target Optimizations**:
- Minimize health check intervals and timeouts
- Optimize Docker network setup
- Reduce container image pull time (ensure images are cached)

**Definition of Done**:
- Container startup time reduced from 15-30s to 5-10s
- Network setup time minimized
- Containers start reliably with faster health checks

**Files to Modify**:
- `internal/testenv/containers.go` - Optimize wait conditions and timeouts
- Check Docker network setup efficiency

---

### Step 3: Benchmark Configuration Tuning  
**Goal**: Balance benchmark accuracy with execution speed for development workflow.

**Current Settings**: 
- `BENCHTIME=3s COUNT=3` (9 seconds per benchmark + container overhead)

**Target Settings**:
- Development mode: `BENCHTIME=100ms COUNT=1` (fast feedback)
- CI/Production mode: `BENCHTIME=1s COUNT=3` (more accurate)
- Environment variable to control benchmark depth

**Definition of Done**:
- Benchmark scripts accept `BENCHMARK_MODE=fast|normal|thorough`
- Fast mode completes core category in under 3 minutes total
- Normal mode balances speed vs accuracy for regular use
- Thorough mode for comprehensive performance analysis

**Files to Modify**:
- `scripts/run_latency_benchmarks.sh` - Add mode selection
- `scripts/run_benchmarks.sh` - Implement configurable benchmark times

---

### Step 4: Parallel Benchmark Execution (Advanced)
**Goal**: Run independent benchmark categories in parallel where possible.

**Current Behavior**: Sequential execution of all benchmarks

**Target Behavior**: 
- Independent benchmark functions run concurrently
- Shared container resources managed safely
- Maintain result accuracy and isolation

**Definition of Done**:
- Core operations can run multiple benchmark functions simultaneously
- No race conditions or resource conflicts
- Total execution time reduced through parallelization
- Results remain consistent and accurate

**Files to Modify**:
- Go benchmark setup to support concurrent execution
- Container resource management for thread safety

---

## Success Metrics

### Before Optimization
- ❌ Core category: 11+ minutes (timeout)
- ❌ Full benchmark suite: Impossible due to timeouts
- ❌ Development workflow: Too slow for regular use

### After Optimization (Target)
- ✅ Core category: 2-3 minutes in normal mode
- ✅ Core category: 30-60 seconds in fast mode  
- ✅ Full benchmark suite: 10-15 minutes maximum
- ✅ Development workflow: Fast enough for regular iteration

## Implementation Priority

1. **Step 1** (High Impact): Container reuse - biggest time savings
2. **Step 3** (Quick Win): Benchmark configuration tuning - immediate usability
3. **Step 2** (Medium Impact): Container optimization - incremental improvement  
4. **Step 4** (Advanced): Parallelization - additional optimization

## Validation Tests

After each step, verify with:
```bash
# Fast development test
BENCHTIME=100ms COUNT=1 ./scripts/run_latency_benchmarks.sh 50 core

# Normal accuracy test  
BENCHTIME=1s COUNT=2 ./scripts/run_latency_benchmarks.sh 100 core

# Full category test
./scripts/run_latency_benchmarks.sh 100 core
```

**Expected results**: Realistic latency values (50-100ms+ per operation) with dramatically reduced total execution time.

---

*Created: September 4, 2025*  
*Goal: Transform containers mode from unusable (11+ min) to practical (2-3 min) for regular development use*
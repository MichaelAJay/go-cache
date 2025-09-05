# TODO: Streamline Benchmark Output Filtering

## Current Problem

The benchmark scripts produce extremely noisy output files due to testcontainers verbose logging. When running latency benchmarks with `GOCACHE_TEST_MODE=containers`, testcontainers logs every container lifecycle event with timestamps and emojis:

```
2025/09/04 21:58:57 🐳 Creating container for image redis:8.0.3
2025/09/04 21:58:57 ✅ Container created: 19707dd61b35
2025/09/04 21:58:57 🐳 Starting container: 19707dd61b35
...hundreds more lines...
BenchmarkRedisCache_GetMany_10Keys-10    160    21422569 ns/op    56084 B/op    428 allocs/op
...more container logs mixed with results...
```

This makes benchmark files nearly unreadable and bloats them from ~20 useful lines to 500+ lines with 95% noise.

## Current Workaround

We have a two-step process:
1. `./scripts/run_latency_benchmarks.sh` - Creates noisy output files
2. `./scripts/clean_benchmark_output.sh` - Filters files after creation

The cleaning script works perfectly, reducing files from 500+ lines to ~20 clean lines, but requires a manual second step.

## Desired Solution

**One-step benchmark execution with clean output by default.**

The benchmark scripts should produce clean output files directly, without requiring post-processing.

## Potential Approaches (Future Investigation)

### Option 1: Real-time Filtering in Benchmark Scripts
- Modify `run_benchmarks.sh` to pipe through the same grep filter during execution
- **Pros**: Single-step solution, immediate clean results
- **Cons**: We attempted this but had issues with the filtering not working correctly in the script context
- **Investigation needed**: Why does `grep -E "(^Benchmark|^goos:|...)"` work in isolation but not in the script pipeline?

### Option 2: Testcontainers Logging Configuration
- Research testcontainers-go logging configuration options
- Look for environment variables or Go API calls to reduce verbosity
- **Potential variables to investigate**:
  - `TESTCONTAINERS_RYUK_VERBOSE=false` (we tried this briefly)
  - Other testcontainers logging environment variables
  - Go-level logging configuration in our test setup

### Option 3: Separate Log Streams
- Redirect testcontainers logs to stderr/dev/null while preserving benchmark stdout
- Use more sophisticated shell redirection to split log streams
- **Challenge**: testcontainers logs to stderr, but we need stderr for Go test errors

### Option 4: Custom Testcontainers Logger
- Implement a custom logger in our Go test setup to control testcontainers output
- Modify `internal/testenv/containers.go` to configure logging at the API level
- This would require more Go code changes but could be the cleanest solution

## Investigation Tasks

1. **Debug the grep pipeline issue**: Why doesn't real-time filtering work in the script?
   - Test intermediate pipeline steps
   - Check for buffering issues
   - Verify grep pattern matching in different shell contexts

2. **Research testcontainers configuration**: Find official ways to reduce verbosity
   - Check testcontainers-go documentation for logging options
   - Test environment variable configurations
   - Look for API-level logging controls

3. **Analyze log stream separation**: Can we cleanly separate benchmark output from container logs?
   - Test different redirection strategies
   - Investigate Go testing output streams
   - Consider using `tee` or other tools for stream management

## Success Criteria

- `./scripts/run_latency_benchmarks.sh 100 core` produces clean output files directly
- No need for post-processing with `clean_benchmark_output.sh`
- Maintain all current functionality (latency simulation, multiple benchmark categories)
- Preserve error reporting and debugging capabilities when needed

## Current Status

- **Workaround implemented**: Two-step process works reliably
- **Root cause identified**: testcontainers verbose logging
- **Solution attempted but failed**: Real-time grep filtering in scripts
- **Priority**: Nice-to-have improvement, not blocking current benchmarking workflow

---

*Created: September 4, 2025*  
*Last updated: September 4, 2025*
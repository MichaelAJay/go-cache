Fix Redis Cache Benchmark Failures

Task Overview

Fix failing benchmarks in the go-cache Redis implementation. Two specific benchmark functions are failing due to test isolation and data cleanup issues.

Current Working Directory

/Users/michaeljay/go-dev/go-cache

Failing Benchmarks

1. BenchmarkRedisCache_SetIfNotExists_NewKey

- File: redis_cache_core_operations_benchmark_test.go:316
- Error: "SetIfNotExists should return true for new key"
- Expected Behavior: Each benchmark iteration should work with truly new keys

2. BenchmarkRedisCache_GetByOwner_10Entries and BenchmarkRedisCache_GetByOwner_100Entries

- File: redis_cache_features_benchmark_test.go:193 and :225
- Error: "Expected 10 results, got 144" / "Expected 100 results, got 144"
- Expected Behavior: Each benchmark should only return the exact number of entries it creates

Investigation Steps

1. Examine the failing benchmark functions to understand their current key generation and cleanup strategies
2. Check if benchmarks are properly isolated - each b.N iteration should be independent
3. Identify data cleanup patterns used in working benchmarks vs failing ones
4. Look for Redis FLUSHDB/FLUSHALL usage or proper key prefixing strategies

Fix Requirements

For SetIfNotExists Benchmark

- Ensure each benchmark iteration uses truly unique keys
- Consider using b.ResetTimer() after setup if needed
- May need timestamp/iteration-specific key prefixes

For GetByOwner Benchmarks

- Ensure Redis is cleaned before each benchmark run
- Verify that owner keys are unique across benchmark iterations
- Check that indexed data doesn't accumulate between runs

Implementation Guidelines

1. Maintain benchmark accuracy - fixes should not impact performance measurements
2. Follow existing patterns - look at working benchmarks for successful cleanup strategies
3. Use proper Go benchmark patterns - leverage b.Cleanup(), b.ResetTimer(), etc. appropriately
4. Test the fixes - run the specific failing benchmarks to verify they pass

Validation

After implementing fixes, run:
go test -tags=integration -bench="BenchmarkRedisCache\_(SetIfNotExists_NewKey|GetByOwner)" -run=^$ -count=3

All benchmarks should pass consistently across multiple runs.

Constraints

- Do NOT modify any core cache functionality - this is purely a test isolation issue
- Preserve benchmark performance characteristics - cleanup should not impact timing measurements
- Follow the existing codebase patterns for Redis connection management and test utilities

Expected Deliverable

Working benchmark functions that:

1. Pass consistently on multiple runs
2. Properly isolate test data between iterations
3. Accurately measure the intended cache operations
4. Follow Go benchmarking best practices

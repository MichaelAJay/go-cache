RedisCache Implementation Analysis

Structure: The RedisCache spans 5 files with ~2,041 lines of code:

- redis_cache.go: Core operations, circuit breaker, distributed coordination
- atomic_operations.go: Complex GetOrSet/Update with distributed locking
- batch_operations.go: Pipeline-based batch operations
- metadata.go: Metadata tracking and statistics
- cache_lua_scripts.go: 11 Lua scripts for atomic operations

Performance Characteristics:

- Heavy Lua script usage for atomicity
- Complex distributed coordination with locks/retries
- Comprehensive metadata tracking overhead
- Dual indexing system (forward + reverse)
- Circuit breaker with failure tracking
- Serialization across multiple formats (JSON/Gob/Msgpack)

Comprehensive Benchmarking Plan

1. Basic Operation Benchmarks - COMPLETE

// Core CRUD operations with different data sizes
BenchmarkRedisCache_Get_1KB
BenchmarkRedisCache_Get_10KB
BenchmarkRedisCache_Get_100KB
BenchmarkRedisCache_Set_1KB
BenchmarkRedisCache_Set_10KB
BenchmarkRedisCache_Set_100KB
BenchmarkRedisCache_Delete
BenchmarkRedisCache_Has
BenchmarkRedisCache_Clear

2. Concurrency Stress Tests

// High-concurrency scenarios targeting goroutine safety
BenchmarkRedisCache_Get_Concurrent_10
BenchmarkRedisCache_Get_Concurrent_100
BenchmarkRedisCache_Get_Concurrent_1000
BenchmarkRedisCache_Set_Concurrent_10
BenchmarkRedisCache_Set_Concurrent_100
BenchmarkRedisCache_Mixed_Operations_Concurrent

3. Atomic Operations Performance

// Test singleflight pattern and distributed locking
BenchmarkRedisCache_GetOrSet_CacheMiss
BenchmarkRedisCache_GetOrSet_CacheHit
BenchmarkRedisCache_GetOrSet_HighContention // Multiple goroutines, same key
BenchmarkRedisCache_Update_Existing
BenchmarkRedisCache_Update_NonExistent
BenchmarkRedisCache_Update_HighContention

4. Batch Operations Efficiency

// Pipeline performance vs individual operations
BenchmarkRedisCache_GetMany_10Keys
BenchmarkRedisCache_GetMany_100Keys
BenchmarkRedisCache_GetMany_1000Keys
BenchmarkRedisCache_SetMany_10Values
BenchmarkRedisCache_SetMany_100Values
BenchmarkRedisCache_DeleteMany_100Keys

5. Indexing Performance Impact

// Compare indexed vs non-indexed performance
BenchmarkRedisCache_Set_WithIndexing
BenchmarkRedisCache_Set_WithoutIndexing
BenchmarkRedisCache_GetByOwner_10Entries
BenchmarkRedisCache_GetByOwner_100Entries
BenchmarkRedisCache_DeleteByOwner_100Entries

6. Serialization Overhead

// Compare different serialization formats
BenchmarkRedisCache_JSON_Serialization
BenchmarkRedisCache_Gob_Serialization
BenchmarkRedisCache_Msgpack_Serialization

7. Conditional Operations

// Atomic conditional operations
BenchmarkRedisCache_SetIfNotExists_NewKey
BenchmarkRedisCache_SetIfNotExists_ExistingKey
BenchmarkRedisCache_SetIfExists_ExistingKey
BenchmarkRedisCache_SetIfExists_NonExistentKey

8. Lua Script Warming Impact

// Test script loading overhead
BenchmarkRedisCache_WithScriptWarming
BenchmarkRedisCache_WithoutScriptWarming
BenchmarkRedisCache_ColdStart_FirstCall

9. Circuit Breaker Overhead

// Circuit breaker performance impact
BenchmarkRedisCache_CircuitBreakerOpen
BenchmarkRedisCache_CircuitBreakerClosed
BenchmarkRedisCache_CircuitBreakerRecovery

10. Memory and Resource Usage

// Memory allocation and GC pressure tests
BenchmarkRedisCache_MemoryAllocations
BenchmarkRedisCache_GCPressure
BenchmarkRedisCache_ConnectionPooling

11. Metadata Overhead Assessment

// Cost of comprehensive metadata tracking
BenchmarkRedisCache_MetadataEnabled
BenchmarkRedisCache_MetadataDisabled
BenchmarkRedisCache_GetMetadata

12. Real-World Usage Patterns

// Simulate go-auth session management patterns
BenchmarkRedisCache_SessionWorkload_Create
BenchmarkRedisCache_SessionWorkload_Access
BenchmarkRedisCache_SessionWorkload_Cleanup
BenchmarkRedisCache_SessionWorkload_Mixed // 70% reads, 20% writes, 10% deletes

13. Performance vs Reliability Trade-offs

// Test performance impact of safety features
BenchmarkRedisCache_FullFeatures // All features enabled
BenchmarkRedisCache_MinimalFeatures // Basic operations only
BenchmarkRedisCache_NoMetrics
BenchmarkRedisCache_NoIndexing

Benchmark Configuration Strategy

Test Data Variations:

- Small objects (100B-1KB) - typical session tokens
- Medium objects (1KB-10KB) - session data with metadata
- Large objects (10KB-100KB) - complex user profiles

Concurrency Patterns:

- Low (1-10 goroutines) - typical application load
- Medium (10-100 goroutines) - moderate load
- High (100-1000 goroutines) - stress testing

Redis Environments:

- Local Redis instance
- Redis cluster setup
- Network latency simulation with varying delays

Key Metrics to Capture:

- Operations per second (throughput)
- Latency percentiles (p50, p95, p99)
- Memory allocations per operation
- GC pressure impact
- Connection pool efficiency
- Lock contention frequency

This benchmarking plan will reveal the performance characteristics and bottlenecks of the complex RedisCache
implementation, particularly focusing on its distributed coordination, atomicity guarantees, and comprehensive feature
set.

Strategic Benchmarking Insights

This benchmarking plan addresses critical performance questions for the strategic refactoring:

Key Bottlenecks to Identify:

1. Distributed Locking Cost: GetOrSet/Update use expensive distributed coordination - benchmarks will reveal if the
   atomicity guarantees justify the ~10-50ms latency overhead
2. Metadata Tax: Every operation tracks access_count, timestamps, size - this doubles write overhead but provides
   observability
3. Lua Script Loading: 11 scripts × cold start penalty - script warming becomes critical for performance
4. Indexing Overhead: Forward+reverse indexes mean 3x write amplification for indexed operations

Performance vs Features Trade-offs:

- Circuit Breaker: Adds ~100μs overhead per operation but prevents cascade failures
- Comprehensive Metrics: Rich observability but ~20% throughput penalty
- Atomic Operations: Perfect consistency but 5-10x slower than simple GET/SET
- Batch Operations: Should show 5-10x throughput improvement via pipelining

Strategic Decisions This Plan Enables:

1. Feature Tiering: Identify which "enterprise features" should be optional vs mandatory
2. Optimization Priorities: Find the highest-impact performance improvements
3. Architecture Validation: Determine if Lua-heavy approach beats application-level coordination
4. Resource Planning: Understand Redis connection pool and memory requirements under load

Expected Findings:

- Basic operations: ~1M ops/sec (memory) vs ~100K ops/sec (Redis) - aligns with project goals
- Atomic operations: ~10K ops/sec due to distributed locking
- Batch operations: 5-10x better than individual calls
- Indexing penalty: ~50% throughput reduction but enables powerful queries

This benchmarking plan will provide the data-driven foundation for the "bold breaking changes" mentioned in the
refactoring plan, helping prioritize performance vs feature completeness trade-offs.

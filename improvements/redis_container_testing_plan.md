# Redis Provider Container Testing Plan

## Overview

This document outlines a comprehensive testing strategy for the Redis cache provider using containerized Redis instances. The testing plan focuses on validating the generic-first design, distributed operations, concurrency guarantees, and performance characteristics of the Redis provider.

## Test Environment Setup

### Redis Container Requirements

**Primary Test Container:**
- Redis 7.x with persistence disabled for speed
- Custom configuration optimized for testing
- Port mapping: 6379 (primary), 6380 (secondary for failover tests)
- Memory limit: 512MB for resource-constrained testing
- No authentication for basic tests
- Flush on startup to ensure clean state

**Secondary Test Containers:**
- Redis Cluster setup (3 nodes minimum) for distributed testing
- Redis Sentinel setup for failover testing
- Network partitioning test setup using container networking

### Test Data Types

The testing plan covers multiple generic type scenarios:

```go
// Primitive types
Cache[string]
Cache[int64]
Cache[float64]
Cache[bool]

// Complex types
Cache[*User]
Cache[SessionData] 
Cache[map[string]interface{}]
Cache[[]byte]

// Large types for memory/serialization testing  
Cache[*LargeStruct] // 1KB+ serialized size
Cache[*HugeStruct]  // 100KB+ serialized size
```

---

## Test Categories

### 1. Generic Interface Compliance Tests

**Objective:** Validate that the Redis provider correctly implements the generic `Cache[T]` interface for all supported types.

**Test Structure:**
```go
func TestGenericInterfaceCompliance[T any](t *testing.T, factory func() T, expectedValue T)
func TestSerializationRoundtrip[T comparable](t *testing.T, value T)
func TestTypeSpecificOperations[T comparable](t *testing.T, values []T)
```

**Test Cases:**
- **Basic CRUD Operations per Type**: Get, Set, Delete, Has, Clear
- **Serialization Accuracy**: Ensure complex types serialize/deserialize identically
- **Type Safety**: Verify compile-time type safety prevents type confusion
- **Zero Values**: Test behavior with nil pointers, empty structs, zero values
- **Edge Cases**: Very large objects, deeply nested structures, circular references (where applicable)

**Pass Criteria:**
- All generic types work identically regardless of underlying type
- Zero type assertions in implementation
- Serialization preserves all data fields accurately
- Memory usage is predictable across type complexity

### 2. Distributed Atomic Operations Tests

**Objective:** Validate that GetOrSet and Update operations maintain atomicity across multiple processes and high concurrency.

**Test Structure:**
```go
func TestDistributedGetOrSet(t *testing.T)
func TestMultiProcessAtomicity(t *testing.T)
func TestGetOrSetLoaderExecutionCount(t *testing.T)
func TestUpdateAtomicity(t *testing.T)
```

**Critical Test Scenarios:**

**2.1 GetOrSet Singleflight Behavior**
- **Setup**: 1000 goroutines simultaneously call GetOrSet with same key
- **Validation**: Loader function executes exactly once
- **Test Types**: `Cache[*ExpensiveResource]`, `Cache[DatabaseConnection]`
- **Metrics**: Track loader call count, measure coordination overhead

**2.2 Cross-Process GetOrSet**
- **Setup**: Multiple test processes (using container exec) call GetOrSet simultaneously
- **Validation**: Only one process's loader executes, all get same result
- **Network Simulation**: Test with network delays, packet loss

**2.3 Update Operation Race Conditions**
- **Setup**: High-concurrency updates to same key with increment operations
- **Validation**: Final value matches expected result (no lost updates)
- **Test Pattern**: Counter increment, complex object modification

**2.4 Distributed Lock Timeout Testing**
- **Setup**: Simulate lock holder crash/timeout scenarios
- **Validation**: Subsequent operations proceed after timeout
- **Recovery Testing**: Ensure system recovers gracefully from lock failures

**Pass Criteria:**
- Zero lost updates under maximum concurrency
- GetOrSet loader executes exactly once per key under contention
- Lock timeouts don't cause permanent deadlocks
- Performance degrades gracefully under extreme contention

### 3. Secondary Indexing System Tests

**Objective:** Validate distributed secondary indexing maintains consistency across operations and provides accurate query results.

**Test Structure:**
```go
func TestIndexConsistency[T comparable](t *testing.T, cache Cache[T])
func TestIndexConcurrency(t *testing.T)
func TestIndexPatternMatching(t *testing.T)
func TestIndexCleanup(t *testing.T)
```

**Key Test Scenarios:**

**3.1 Index Consistency Under Concurrency**
- **Setup**: Concurrent Set/Delete operations with active indexing
- **Validation**: Index always reflects current data state
- **Test Pattern**: User sessions indexed by user ID, frequent creation/deletion

**3.2 Pattern Matching Accuracy**
- **Setup**: Complex key patterns with wildcards
- **Validation**: GetByIndex returns exactly matching keys
- **Edge Cases**: Special characters in keys, unicode, very long keys

**3.3 Index Cleanup and Orphan Prevention**
- **Setup**: Delete keys that are indexed, restart Redis
- **Validation**: No orphaned index entries remain
- **Cleanup Testing**: Verify maintenance operations clean stale entries

**3.4 Cross-Process Index Coordination**
- **Setup**: Multiple processes modifying indexes simultaneously
- **Validation**: All processes see consistent index state

**Pass Criteria:**
- Index queries return 100% accurate results
- No orphaned index entries after cleanup
- Index performance scales linearly with entry count
- Cross-process index updates are eventually consistent

### 4. High-Concurrency Stress Tests

**Objective:** Validate system stability and performance under extreme concurrent load.

**Test Structure:**
```go
func TestConcurrencyStress(t *testing.T, goroutines int, duration time.Duration)
func TestMemoryLeakDetection(t *testing.T)
func TestCircuitBreakerBehavior(t *testing.T)
func TestConnectionPoolStress(t *testing.T)
```

**Stress Test Scenarios:**

**4.1 Extreme Goroutine Load**
- **Scale**: 10,000+ goroutines performing mixed operations
- **Duration**: 10+ minutes continuous operation
- **Operations**: 70% Gets, 20% Sets, 8% Updates, 2% Deletes
- **Validation**: Zero panics, memory leaks, or deadlocks

**4.2 Mixed Workload Torture Test**
- **Setup**: Complex operations mix with varying key distributions
- **Pattern**: Hot keys (high contention) + cold keys (low contention)
- **Types**: Mix of small and large objects, different serialization formats

**4.3 Memory Pressure Testing**
- **Setup**: Large datasets approaching Redis memory limits
- **Validation**: Graceful degradation, proper eviction behavior
- **Recovery**: Test system recovery after memory pressure relief

**4.4 Network Partition Simulation**
- **Setup**: Simulate Redis connection failures, network timeouts
- **Validation**: Circuit breaker triggers correctly, system recovers
- **Failover**: Test behavior when Redis container restarts

**Pass Criteria:**
- System remains stable under 10,000+ concurrent goroutines
- Memory usage is bounded and predictable
- Circuit breaker prevents cascade failures
- Zero data corruption under any failure scenario

### 5. Performance Benchmark Suite

**Objective:** Establish performance baselines and detect regressions across different usage patterns.

**Benchmark Structure:**
```go
func BenchmarkBasicOperations[T any](b *testing.B, value T)
func BenchmarkBatchOperations[T any](b *testing.B, values []T)
func BenchmarkConcurrentOperations(b *testing.B)
func BenchmarkSerializationOverhead[T any](b *testing.B, value T)
```

**Performance Test Scenarios:**

**5.1 Single-Operation Latency**
- **Target**: <1ms P99 latency for basic operations
- **Measurement**: Get, Set, Delete latency distribution
- **Conditions**: Local Redis, network Redis, loaded Redis
- **Types**: Small objects (<1KB), medium (1-100KB), large (>100KB)

**5.2 Throughput Testing**
- **Target**: >100,000 ops/sec for basic operations
- **Pattern**: Read-heavy (90% reads), write-heavy (90% writes), mixed
- **Concurrency**: Scale from 1 to 1000 concurrent clients
- **Measurement**: Operations per second vs. latency curve

**5.3 Batch Operation Efficiency**
- **Comparison**: Batch vs. individual operations for same data
- **Target**: >10x performance improvement for large batches
- **Sizes**: 10, 100, 1000, 10000 items per batch

**5.4 Memory Efficiency**
- **Measurement**: Memory overhead per cached item
- **Comparison**: Different serialization formats
- **Tracking**: GC pressure, allocation patterns

**5.5 Distributed Operation Overhead**
- **Measurement**: GetOrSet vs. simple Get/Set performance cost
- **Lock Contention**: Performance under increasing contention levels
- **Lua Script Performance**: Script execution time vs. multi-command operations

**Performance Targets:**
- **Basic Operations**: <1ms P99, >100k ops/sec
- **Distributed Operations**: <5ms P99, >50k ops/sec  
- **Batch Operations**: 10x+ improvement over individual operations
- **Memory Overhead**: <100 bytes per cached item
- **Connection Efficiency**: Support 1000+ concurrent connections

### 6. Serialization Format Testing

**Objective:** Validate all serialization formats work correctly and perform optimally for different data types.

**Test Structure:**
```go
func TestSerializationFormats[T any](t *testing.T, value T)
func BenchmarkSerializationFormats[T any](b *testing.B, value T)
func TestSerializationCompatibility[T any](t *testing.T, value T)
```

**Format Test Scenarios:**

**6.1 Format Accuracy Testing**
- **Formats**: JSON, Binary (Gob), MessagePack
- **Types**: All supported generic types
- **Validation**: Perfect round-trip accuracy
- **Edge Cases**: Nil pointers, empty collections, unicode data

**6.2 Performance Comparison**
- **Metrics**: Serialization speed, size, deserialization speed
- **Data Types**: Small structs, large structs, arrays, maps
- **Recommendations**: Optimal format per use case

**6.3 Cross-Language Compatibility**
- **Focus**: JSON and MessagePack formats
- **Validation**: Data written by Go can be read by other languages
- **Test Data**: Standard test vectors

**Pass Criteria:**
- Perfect accuracy for all formats and types
- Performance characteristics match expectations
- No serialization-related memory leaks

### 7. Circuit Breaker and Fault Tolerance

**Objective:** Validate fault tolerance mechanisms protect system integrity under failure conditions.

**Test Structure:**
```go
func TestCircuitBreakerThresholds(t *testing.T)
func TestRedisFailoverScenarios(t *testing.T)
func TestNetworkPartitionRecovery(t *testing.T)
func TestConnectionPoolExhaustion(t *testing.T)
```

**Fault Tolerance Scenarios:**

**7.1 Circuit Breaker Behavior**
- **Trigger**: Simulate 10 consecutive Redis failures
- **Validation**: Circuit opens, fast-fails subsequent requests
- **Recovery**: Circuit closes after timeout, normal operation resumes
- **Metrics**: Track circuit state changes, error rates

**7.2 Redis Container Failure**
- **Setup**: Kill Redis container during active operations
- **Validation**: Operations fail gracefully, no hanging requests
- **Recovery**: Operations resume when Redis restarts

**7.3 Network Partition Testing**
- **Setup**: Block network between application and Redis
- **Validation**: Circuit breaker activates, timeouts are respected
- **Recovery**: Normal operation when network restored

**7.4 Connection Pool Stress**
- **Setup**: Exhaust connection pool with slow operations
- **Validation**: New requests queue or fail gracefully
- **Recovery**: Pool recovers when operations complete

**Pass Criteria:**
- Circuit breaker prevents cascade failures
- System recovers automatically from transient failures
- No resource leaks during failure scenarios
- Graceful degradation under extreme conditions

---

## Test Infrastructure Requirements

### Container Setup Scripts

**docker-compose.test.yml**
```yaml
version: '3.8'
services:
  redis-primary:
    image: redis:7-alpine
    command: redis-server --save "" --appendonly no --maxmemory 512mb
    ports:
      - "6379:6379"
    
  redis-secondary:
    image: redis:7-alpine  
    command: redis-server --save "" --appendonly no --maxmemory 512mb
    ports:
      - "6380:6379"
      
  redis-cluster:
    image: redis:7-alpine
    command: redis-server --cluster-enabled yes --cluster-config-file nodes.conf --cluster-node-timeout 5000
    # Additional cluster setup...
```

### Test Helper Functions

```go
// Test utilities for container management
func SetupRedisContainer(t *testing.T) *TestRedisContainer
func FlushRedisData(container *TestRedisContainer)
func SimulateNetworkPartition(container *TestRedisContainer, duration time.Duration)
func CreateCacheForTesting[T any](container *TestRedisContainer) Cache[T]

// Generic test helpers
func RunConcurrentOperations[T any](cache Cache[T], operations []Operation[T])
func MeasureMemoryUsage(fn func()) (allocBytes int64, gcCount int)
func ValidateNoMemoryLeaks(baseline, final runtime.MemStats) bool
```

### Test Data Generators

```go
// Generate test data for different scenarios
func GenerateTestUsers(count int) []*User
func GenerateRandomStrings(count int, size int) []string
func GenerateLargeObjects(count int, sizeKB int) []*LargeStruct
func GenerateKeyPatterns() []string // For indexing tests
```

---

## Test Execution Strategy

### Test Organization

**Package Structure:**
```
internal/providers/redis/
├── redis_test.go           // Basic functionality tests
├── concurrency_test.go     // Concurrency and atomic operation tests
├── indexing_test.go        // Secondary indexing tests
├── performance_test.go     // Benchmarks and performance tests
├── serialization_test.go   // Serialization format tests
├── fault_tolerance_test.go // Circuit breaker and failure tests
├── integration_test.go     // Cross-process integration tests
└── helpers_test.go         // Test utilities and fixtures
```

### Continuous Integration

**Test Phases:**
1. **Unit Tests**: Fast, isolated, no external dependencies
2. **Container Tests**: Require Redis container, medium speed
3. **Integration Tests**: Multi-process, complex scenarios, slower
4. **Performance Tests**: Long-running benchmarks, performance validation
5. **Stress Tests**: Extreme load testing, longest duration

**CI Pipeline Integration:**
```yaml
test-redis-provider:
  runs-on: ubuntu-latest
  services:
    redis:
      image: redis:7-alpine
      ports:
        - 6379:6379
  steps:
    - name: Run Redis Provider Tests
      run: |
        go test ./internal/providers/redis/... -v -race -timeout 30m
        go test ./internal/providers/redis/... -bench=. -benchtime=10s
```

### Performance Regression Detection

**Baseline Establishment:**
- Run performance benchmarks on known-good implementation
- Store results in version control for comparison
- Set acceptable performance degradation thresholds (e.g., 5%)

**Automated Performance Validation:**
- Run benchmarks on every commit affecting Redis provider
- Compare results against baseline with statistical significance testing
- Alert on performance regressions exceeding thresholds

---

## Success Criteria Summary

### Functional Requirements
- ✅ All generic types work identically with zero type assertions
- ✅ Distributed atomic operations maintain consistency
- ✅ Secondary indexing provides accurate, consistent results
- ✅ Circuit breaker protects against cascade failures
- ✅ All serialization formats preserve data integrity

### Performance Requirements
- ✅ Basic operations: <1ms P99 latency, >100k ops/sec
- ✅ Distributed operations: <5ms P99 latency, >50k ops/sec
- ✅ Batch operations: 10x+ performance improvement
- ✅ Memory overhead: <100 bytes per cached item
- ✅ Support 1000+ concurrent connections

### Reliability Requirements  
- ✅ Zero data corruption under any failure scenario
- ✅ Zero memory leaks in long-running tests
- ✅ Graceful degradation under resource pressure
- ✅ Automatic recovery from transient failures
- ✅ Race detector passes with 10,000+ concurrent goroutines

### Enterprise Requirements
- ✅ Comprehensive metrics for observability
- ✅ Security features (timing protection) work correctly
- ✅ Cross-process coordination maintains consistency
- ✅ Support for multiple serialization formats
- ✅ Production-ready error handling and logging

---

This comprehensive testing plan ensures the Redis provider meets all requirements for high-performance, thread-safe, distributed caching while maintaining the generic-first design principles of the new cache architecture.
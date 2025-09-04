# Phase 1: Concurrency Guarantees & Architecture

## Core Concurrency Guarantees

### Thread-Safety Guarantees

**GUARANTEE 1: Goroutine Safety**
- **All cache operations are goroutine-safe** without external synchronization
- **No data races** under any concurrent usage pattern
- **No undefined behavior** when multiple goroutines access the same cache instance
- **Race detector must pass** under extreme concurrent load (10,000+ goroutines)

**GUARANTEE 2: Per-Key Atomicity**
- **GetOrSet operations are atomic per key**: Loader function executes exactly once for a given key under contention
- **Update operations are atomic per key**: No lost updates or inconsistent states
- **Conditional operations are atomic**: SetIfExists, SetIfNotExists guarantee atomic test-and-set behavior
- **Index operations are atomic**: AddIndex/RemoveIndex cannot leave indexes in inconsistent states

**GUARANTEE 3: Cross-Operation Consistency** 
- **Read-after-write consistency**: A successful Set is immediately visible to subsequent Gets from any goroutine
- **Delete consistency**: A successful Delete immediately makes the key unavailable to all operations
- **Index consistency**: Secondary indexes remain consistent with primary data at all times
- **Metadata consistency**: Entry metadata (TTL, creation time) remains consistent with the cached value

**GUARANTEE 4: Isolation Levels**
- **Operations appear atomic**: Concurrent operations on different keys proceed independently
- **No partial states visible**: Operations either complete fully or appear to have not started
- **Consistent ordering**: Operations from the same goroutine appear in program order to that goroutine
- **No phantom reads**: Pattern-based operations (GetKeysByPattern) see a consistent snapshot

**GUARANTEE 5: Deadlock Prevention**
- **Deadlock-free design**: No operation can cause a deadlock regardless of call ordering
- **Proper lock ordering**: When multiple locks are required, consistent ordering prevents deadlocks
- **Timeout mechanisms**: Long-running operations have timeouts to prevent indefinite blocking
- **Resource cleanup**: All resources are properly cleaned up even in error scenarios

## Provider-Specific Concurrency Models

### Memory Provider: Sharded Lock-Free Architecture

**Core Data Structure:**
```go
type MemoryProvider struct {
    // Sharded data storage (128+ shards for optimal distribution)
    shards []*memoryShard
    
    // Per-shard singleflight for GetOrSet deduplication  
    loaders []*singleflight.Group
    
    // Configuration
    shardCount int
    config     *MemoryConfig
}

type memoryShard struct {
    // Primary data store - sync.Map for lock-free reads
    data sync.Map // map[string]*cacheEntry
    
    // Secondary indexes - protected by RWMutex
    indexes map[string]*indexShard
    indexMu sync.RWMutex
    
    // TTL management
    ttlHeap *timeHeap // Min-heap for efficient cleanup
    ttlMu   sync.Mutex
    
    // Metrics per shard
    metrics *shardMetrics
}
```

**Concurrency Strategy:**

1. **Lock-Free Primary Operations**:
   - Use `sync.Map` for primary data storage enabling lock-free reads
   - Atomic operations for simple value updates where possible
   - Shard keys using consistent hashing to distribute load

2. **Minimal Locking for Complex Operations**:
   - **Index operations**: RWMutex per shard (read-heavy workloads benefit from shared reads)
   - **TTL management**: Mutex per shard for heap operations
   - **Cleanup operations**: Background goroutine with periodic coordination

3. **GetOrSet Deduplication**:
   - **singleflight.Group per shard**: Ensures loader executes once per key under contention
   - **Key-level deduplication**: Multiple goroutines requesting same key share result
   - **Error propagation**: Failed loads propagate to all waiting goroutines

4. **Update Atomicity**:
   - **Compare-and-swap patterns**: Use sync.Map's LoadOrStore and CompareAndSwap
   - **Retry mechanisms**: Handle concurrent modifications with exponential backoff
   - **Version-based updates**: Detect concurrent modifications and retry safely

**Memory Provider Performance Targets:**
- **Simple operations**: <10μs P99 latency
- **Complex operations**: <100μs P99 latency  
- **Throughput**: >1M operations/second on modern hardware
- **Scalability**: Linear scaling up to 32+ CPU cores

### Redis Provider: Distributed Coordination Architecture

**Core Data Structure:**
```go
type RedisProvider struct {
    // Connection management
    pool    *redis.Pool
    cluster *redis.ClusterClient // For Redis Cluster support
    
    // Distributed coordination
    lockManager *redisLockManager
    scriptCache map[string]*redis.Script
    
    // Per-key singleflight for cross-process deduplication
    loaders *singleflight.Group
    
    // Circuit breaker for resilience
    breaker *circuitbreaker.CircuitBreaker
    
    // Configuration
    config *RedisConfig
}

type redisLockManager struct {
    // Distributed lock implementation
    lockScript   *redis.Script
    unlockScript *redis.Script
    
    // Lock configuration
    defaultTTL time.Duration
    retryDelay time.Duration
    maxRetries int
}
```

**Concurrency Strategy:**

1. **Distributed Atomic Operations**:
   - **Lua scripts for atomicity**: Multi-operation transactions execute atomically on Redis server
   - **WATCH/MULTI/EXEC patterns**: Optimistic concurrency for complex updates
   - **Distributed locks**: Cross-process coordination for GetOrSet operations

2. **GetOrSet Cross-Process Deduplication**:
   ```lua
   -- Lua script for atomic GetOrSet
   local key = KEYS[1]
   local value = ARGV[1] 
   local ttl = ARGV[2]
   local lock_key = key .. ":lock"
   
   -- Try to acquire distributed lock
   if redis.call("SET", lock_key, "1", "PX", 5000, "NX") then
       -- Check if value already exists
       local existing = redis.call("GET", key)
       if existing then
           redis.call("DEL", lock_key)
           return existing
       end
       
       -- Set new value
       redis.call("SETEX", key, ttl, value)
       redis.call("DEL", lock_key)
       return value
   else
       -- Lock already held, wait and retry
       return nil
   end
   ```

3. **Update Operation Atomicity**:
   - **Version-based updates**: Use Redis WATCH command to detect concurrent modifications
   - **Retry with exponential backoff**: Handle contention gracefully
   - **Lua scripts for complex updates**: Ensure atomicity for multi-field updates

4. **Index Management**:
   - **Atomic index operations**: Lua scripts ensure index/data consistency
   - **Set-based indexes**: Use Redis Sets for efficient index storage
   - **Pattern matching**: Server-side pattern matching with SCAN commands

5. **Resilience and Circuit Breaking**:
   - **Connection pool management**: Automatic failover and retry
   - **Circuit breaker**: Prevent cascade failures during Redis outages
   - **Graceful degradation**: Fall back to local cache or error responses

**Redis Provider Performance Targets:**
- **Simple operations**: <1ms P99 latency (local Redis)
- **Complex operations**: <5ms P99 latency (local Redis)
- **Distributed operations**: <10ms P99 latency (network Redis)
- **Throughput**: >100k operations/second with pipelining
- **Network efficiency**: <100 round-trips/second through batching

## Cross-Provider Consistency Requirements

### Behavioral Consistency
- **Identical semantics**: Same operations produce same results across providers
- **Error handling**: Consistent error types and messages
- **TTL behavior**: Identical expiration semantics and precision
- **Index behavior**: Same secondary indexing results and performance characteristics

### Performance Consistency
- **Relative performance**: Redis ~10x slower than Memory (network overhead)
- **Scaling characteristics**: Both providers scale linearly with resources
- **Resource usage**: Predictable memory/connection usage patterns
- **Failure modes**: Graceful degradation under resource constraints

### API Consistency
- **Generic interface**: Both providers implement identical `Cache[T]` interface
- **Configuration**: Similar option patterns and validation
- **Observability**: Consistent metrics and logging across providers
- **Lifecycle**: Identical initialization and cleanup semantics

## Lock Hierarchy and Deadlock Prevention

### Memory Provider Lock Ordering

1. **Shard-level ordering**: Always acquire locks in shard index order (0, 1, 2, ...)
2. **Within-shard ordering**: 
   - Data operations (sync.Map - lock-free)
   - Index operations (indexMu RWMutex)
   - TTL operations (ttlMu Mutex)
3. **Cross-shard operations**: Acquire all required shard locks in index order
4. **Timeout mechanisms**: All lock acquisitions have timeouts to prevent deadlocks

### Redis Provider Coordination

1. **Distributed lock ordering**: Lexicographic key ordering for multi-key operations
2. **Lock timeouts**: All distributed locks have TTL to prevent orphaned locks
3. **Retry patterns**: Exponential backoff with jitter to prevent thundering herd
4. **Circuit breaker**: Prevents resource exhaustion during Redis failures

### Global Deadlock Prevention Rules

1. **No nested cache calls**: Operations never call other cache operations while holding locks
2. **Context timeout propagation**: All operations respect context timeouts
3. **Resource cleanup**: Defer statements ensure cleanup even in panic scenarios
4. **Lock-free where possible**: Prefer atomic operations over locks

## Performance Requirements and Targets

### Latency Requirements

**Memory Provider:**
- **Get/Set/Delete**: <10μs P99, <1μs P50
- **GetOrSet/Update**: <100μs P99, <10μs P50  
- **Batch operations**: <500μs P99 for 100 items
- **Index operations**: <50μs P99 for add/remove, <100μs P99 for query

**Redis Provider:**
- **Simple operations**: <1ms P99, <200μs P50 (local Redis)
- **Atomic operations**: <5ms P99, <1ms P50 (local Redis)
- **Batch operations**: <10ms P99 for 100 items (pipelined)
- **Network operations**: Add 1ms per network hop

### Throughput Requirements

**Memory Provider:**
- **Single-threaded**: >500k ops/sec per core
- **Multi-threaded**: >1M ops/sec scaling linearly with cores
- **Mixed workload**: >800k ops/sec with 70% reads, 30% writes
- **Under contention**: >100k ops/sec with 1000 concurrent goroutines

**Redis Provider:**
- **Pipelined operations**: >100k ops/sec
- **Single operations**: >50k ops/sec
- **Distributed operations**: >10k ops/sec (cross-process GetOrSet)
- **Batch operations**: >200k items/sec with batching

### Scalability Requirements

**Horizontal Scaling:**
- **Memory Provider**: Linear scaling up to 64 CPU cores
- **Redis Provider**: Linear scaling with Redis Cluster nodes
- **Connection efficiency**: <100 connections per process to Redis

**Vertical Scaling:**
- **Memory efficiency**: <1KB overhead per cached item
- **GC pressure**: <10% of CPU time spent in garbage collection
- **Lock contention**: <5% of operation time waiting for locks

### Resource Efficiency

**Memory Usage:**
- **Overhead per entry**: <200 bytes (including indexes and metadata)
- **Index memory**: <50% of primary data memory
- **Memory growth**: Linear with cached data size

**CPU Usage:**
- **Background cleanup**: <5% of total CPU under normal load
- **Lock contention**: <10% of operation time in lock acquisition
- **Serialization overhead**: <20% of operation time for complex types

## Error Handling Patterns

### Concurrent Error Scenarios

**Race Condition Errors:**
- **Type**: Transparent to user - handled internally with retries
- **Example**: Update operation detects concurrent modification
- **Handling**: Exponential backoff retry up to 3 attempts, then return error

**Resource Exhaustion:**
- **Type**: `ErrResourceExhausted` with specific resource type
- **Example**: Memory provider hits max entries, Redis connection pool exhausted
- **Handling**: Immediate failure with clear error message and retry guidance

**Timeout Errors:**
- **Type**: Context timeout or operation timeout
- **Example**: Distributed lock acquisition timeout in Redis
- **Handling**: Clean resource cleanup, return `context.DeadlineExceeded`

**Network Errors (Redis):**
- **Type**: Connection failures, Redis server errors
- **Example**: Redis server restart during operation
- **Handling**: Circuit breaker activation, automatic retry with exponential backoff

### Error Recovery Strategies

**Transient Errors:**
- **Automatic retry**: Up to 3 attempts with exponential backoff
- **Jitter**: Random delay to prevent thundering herd
- **Circuit breaker**: Temporary failure mode to prevent cascade failures

**Persistent Errors:**
- **Fail fast**: Return error immediately after retry limit
- **Error wrapping**: Preserve original error with operation context
- **Logging**: Structured logging with operation details and timing

**Partial Failures:**
- **Batch operations**: Return partial results with error details
- **Index operations**: Maintain data consistency, report index failures separately
- **Cleanup operations**: Continue cleanup, log failures for monitoring

### Error Types and Hierarchy

```go
// Base error types
var (
    ErrKeyNotFound      = errors.New("key not found")
    ErrKeyExists        = errors.New("key already exists") 
    ErrInvalidKey       = errors.New("invalid key format")
    ErrInvalidTTL       = errors.New("invalid TTL value")
    ErrCacheClosed      = errors.New("cache is closed")
    ErrResourceExhausted = errors.New("resource exhausted")
    ErrTimeout          = errors.New("operation timeout")
    ErrConcurrentModification = errors.New("concurrent modification detected")
)

// Provider-specific error types
type MemoryError struct {
    Op  string // Operation name
    Err error  // Underlying error
}

type RedisError struct {
    Op      string // Operation name  
    Network bool   // Network-related error
    Err     error  // Underlying Redis error
}
```

This comprehensive concurrency architecture ensures thread-safe, high-performance operation while maintaining clean, simple interfaces for consumers.
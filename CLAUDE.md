# CLAUDE.md - Go-Cache Module

## Project Overview

Go-Cache is undergoing **strategic refactoring** to become a high-performance, generic-first caching abstraction for Go applications. The new design eliminates legacy complexity while preserving all enterprise features through clean, thread-safe, type-safe interfaces.

### Key Features

- **Generic-First Design**: All interfaces are `Cache[T]` - no `interface{}` types anywhere
- **Thread-Safe by Design**: All concurrency handled at cache layer - consumers never need synchronization primitives
- **Enterprise Features Preserved**: Secondary indexing, comprehensive metrics, security features, middleware system
- **Aggressive Performance**: Lock-free operations where possible, atomic guarantees, sub-microsecond latency
- **Provider Abstraction**: Memory and Redis providers completely rewritten for optimal concurrency
- **Breaking Changes Embraced**: Clean design prioritized over backward compatibility

## Architecture

### Core Components

```
┌─────────────────────┐     ┌─────────────────────┐
│   Cache Manager     │────▶│  Cache Interface    │
│ (Factory Pattern)   │     │   (Unified API)     │
└─────────────────────┘     └─────────────────────┘
         │                           │
         │                  ┌────────┴────────┐─────────────────────┐
         │                  ▼                 ▼                     ▼
         │          ┌──────────────┐  ┌──────────────┐        ┌──────────────┐
         │          │   Memory     │  │    Redis     │        │ Additional   │
         │          │  Provider    │  │  Provider    │        │  Providers   │
         │          └──────────────┘  └──────────────┘        └──────────────┘
         │                  │                 │                 │
    ┌────┴────┐             │                 │                 │
    ▼         ▼             │                 │                 │
┌────────┐ ┌────────┐       │                 │                 │
│Metrics │ │ Hooks  │       │                 │                 │
│System  │ │ System │       │                 │                 │
└────────┘ └────────┘       │                 │                 │
    │         │             │                 │                 │
    └─────────┴─────────────┴─────────────────┘─────────────────┘
              │
    ┌─────────▼─────────┐
    │   Serialization   │
    │      System       │
    └───────────────────┘
```

### Design Principles

1. **Generic-First Architecture**: All interfaces use Go generics - compile-time type safety, zero runtime type assertions
2. **Thread-Safety at Cache Layer**: All concurrency complexity handled internally - consumers never need mutexes/locks
3. **Bold Breaking Changes**: Clean design prioritized over backward compatibility
4. **Enterprise Features Preserved**: Secondary indexing, metrics, security, middleware maintained and enhanced
5. **Observability First**: Comprehensive go-metrics integration, Prometheus, OpenTelemetry support  
6. **Security Conscious**: Built-in protections against timing attacks, secure cleanup, enhanced for concurrent access
7. **Performance Optimized**: Lock-free where possible, atomic operations, sub-microsecond latency targets

## Development Guidelines

### Code Style

1. **Interface-First Design**: All major functionality exposed through interfaces
2. **Error Handling**: Detailed, typed errors with proper context
3. **Concurrency**: All components must be thread-safe by design
4. **Testing**: Comprehensive unit, integration, and performance tests
5. **Documentation**: All public APIs thoroughly documented with examples

### Provider Implementation Requirements

**All providers must implement:**

- Complete `Cache[T]` generic interface with full type safety
- Atomic thread-safe operations (GetOrSet, Update with singleflight pattern)
- All enterprise features: secondary indexing, metrics, security, middleware
- Provider-optimized serialization (memory: native types, Redis: optimal encoding)
- Performance targets: >1M ops/sec (memory), >100k ops/sec (Redis)
- Zero memory leaks under sustained high-concurrency load

**Provider-Specific Features:**

- **Memory Provider**: Lock-free operations, sharded maps, hierarchical cleanup, zero-copy optimizations
- **Redis Provider**: Distributed locks, Lua scripts, pipelined operations, circuit breakers, cluster support

### Testing Patterns

**Unit Tests:**

- Provider-specific functionality
- Interface compliance
- Error conditions
- Edge cases
- Concurrency scenarios

**Integration Tests:**

- Cross-provider compatibility
- Real-world usage patterns
- Performance characteristics
- Memory leak detection

**Benchmark Tests:**

- Operation throughput
- Memory usage
- Latency percentiles
- Scaling characteristics

## Key Interfaces

### Core Cache Interface (NEW - Generic & Thread-Safe)

```go
type Cache[T any] interface {
    // Basic operations - thread-safe and generic
    Get(ctx context.Context, key string) (T, bool, error)
    Set(ctx context.Context, key string, value T, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Clear(ctx context.Context) error
    Has(ctx context.Context, key string) bool
    
    // Atomic operations (eliminate consumer-side locking)
    GetOrSet(ctx context.Context, key string, loader func(ctx context.Context) (T, error), ttl time.Duration) (T, error)
    Update(ctx context.Context, key string, updater func(old T, exists bool) (T, error), ttl time.Duration) (T, error)
    
    // Batch operations for performance
    GetMany(ctx context.Context, keys []string) (map[string]T, error)
    SetMany(ctx context.Context, items map[string]T, ttl time.Duration) error
    DeleteMany(ctx context.Context, keys []string) error
    
    // Secondary indexing (thread-safe)
    AddIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error
    RemoveIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error
    GetByIndex(ctx context.Context, indexName string, indexKey string) ([]string, error)
    DeleteByIndex(ctx context.Context, indexName string, indexKey string) error
    
    // Conditional operations
    SetIfNotExists(ctx context.Context, key string, value T, ttl time.Duration) (bool, error)
    SetIfExists(ctx context.Context, key string, value T, ttl time.Duration) (bool, error)
    
    // Pattern operations
    GetKeysByPattern(ctx context.Context, pattern string) ([]string, error)
    DeleteByPattern(ctx context.Context, pattern string) (int, error)
    
    // Metadata operations  
    GetMetadata(ctx context.Context, key string) (*CacheEntryMetadata, error)

    // Lifecycle
    Close() error
}

// Manager for creating provider-specific typed caches
type Manager interface {
    NewCache[T any](name string, opts ...Option) (Cache[T], error)
    RegisterProvider(name string, provider CacheProvider)
    Close() error
}
```

### Enhanced Metrics Interface

```go
type EnhancedCacheMetrics interface {
    // Basic metrics
    RecordHit()
    RecordMiss()
    RecordOperation(operation string, status string, duration time.Duration)

    // Security metrics
    RecordSecurityEvent(eventType string, severity string, metadata map[string]any)
    RecordTimingProtection(operation string, actualTime, adjustedTime time.Duration)

    // Performance metrics
    GetOperationLatencyPercentiles(operation string) map[string]time.Duration
}
```

## Configuration Architecture

### Unified Configuration

```go
type CacheOptions struct {
    // Basic settings
    TTL              time.Duration
    MaxEntries       int
    CleanupInterval  time.Duration

    // Security configuration
    Security *SecurityConfig

    // Provider-specific
    RedisOptions *RedisOptions

    // Advanced features
    Indexes map[string]string // indexName -> keyPattern
    Hooks   *CacheHooks
    Metrics EnhancedCacheMetrics
}
```

### Security Configuration

```go
type SecurityConfig struct {
    EnableTimingProtection bool
    MinProcessingTime      time.Duration
    SecureCleanup         bool
}
```

## Common Usage Patterns

### Basic Cache Operations (NEW - Generic & Thread-Safe)

```go
// Initialize typed cache - no more interface{} anywhere
manager := cache.NewManager()
sessions, err := manager.NewCache[*models.Session]("memory",
    WithTTL(24*time.Hour),
    WithCleanupInterval(5*time.Minute),
)

// Basic operations - fully type-safe
session := &models.Session{ID: "123", UserID: "456"}
err = sessions.Set(ctx, "session:123", session, time.Hour)
session, found, err := sessions.Get(ctx, "session:123") // Returns *models.Session, not interface{}
err = sessions.Delete(ctx, "session:123")

// Atomic operations replace consumer-side locking
session, err = sessions.GetOrSet(ctx, "session:123", func(ctx context.Context) (*models.Session, error) {
    return loadSessionFromDB(ctx, "123") // Only called once even under high concurrency
}, time.Hour)

// Update operations are atomic
session, err = sessions.Update(ctx, "session:123", func(old *models.Session, exists bool) (*models.Session, error) {
    if !exists {
        return nil, errors.New("session not found")
    }
    old.LastAccessed = time.Now()
    return old, nil
}, time.Hour)
```

### Secondary Indexing (Thread-Safe)

```go
// Add indexing for complex queries - all thread-safe
err = sessions.AddIndex(ctx, "sessions_by_user", "session:*", "user:456")

// Query by index - no external synchronization needed
sessionKeys, err := sessions.GetByIndex(ctx, "sessions_by_user", "user:456")

// Bulk operations using indexes - atomic operations
err = sessions.DeleteByIndex(ctx, "sessions_by_user", "user:456")

// Multiple indexes can be managed concurrently
err = sessions.AddIndex(ctx, "sessions_by_role", "session:*", "admin") // Safe to call concurrently
```

### With Security Features (Enhanced for Concurrency)

```go
secureTokens, err := manager.NewCache[*SecurityToken]("memory",
    WithSecurity(&SecurityConfig{
        EnableTimingProtection: true,  // Enhanced for concurrent access
        MinProcessingTime:      5*time.Millisecond,
        SecureCleanup:         true,   // Zero memory before deallocation
    }),
    WithHooks(&CacheHooks{
        PreGet: func(ctx context.Context, key string) error {
            // Validate access permissions - called atomically
            return validateAccess(ctx, key)
        },
    }),
    WithMetrics(metricsRegistry), // Comprehensive go-metrics integration
)

// All security features work seamlessly with generic interface
token, err := secureTokens.GetOrSet(ctx, "token:123", func(ctx context.Context) (*SecurityToken, error) {
    return generateSecureToken(ctx) // Timing protection applied automatically
}, 15*time.Minute)
```

## Integration with go-auth

**Primary Use Case: Session Management**

The go-cache module serves as the storage abstraction for go-auth's session management:

- **Session Storage**: Primary key-value storage for session objects
- **User Indexing**: Secondary indexes for "all sessions for user X" queries
- **TTL Management**: Automatic session expiration handling
- **Security**: Timing attack protection for session validation
- **Observability**: Detailed metrics for session operations
- **Cleanup**: Automatic cleanup of expired sessions

**New Integration Pattern (Dramatically Simplified):**

```go
// go-auth session manager - ZERO synchronization primitives
type SessionManager struct {
    sessions     Cache[*models.Session]  // Direct generic cache - no wrappers
    subjectIndex Cache[[]string]         // Thread-safe subject indexing
    // No mutexes, no channels, no TypedCache wrappers, no race conditions
}

func (s *SessionManager) Create(ctx context.Context, authResult *AuthResult, metadata map[string]any) (*Session, error) {
    session := &Session{...}

    // Atomic session creation - no race conditions possible
    session, err := s.sessions.GetOrSet(ctx, session.ID, func(ctx context.Context) (*Session, error) {
        return session, nil // Only executes once even under high concurrency
    }, s.defaultTTL)

    // Thread-safe index operations - no external locking needed
    err = s.sessions.AddIndex(ctx, "sessions_by_user", "session:*", session.SubjectID)

    return session, err
}

func (s *SessionManager) UpdateLastAccess(ctx context.Context, sessionID string) (*Session, error) {
    // Atomic update - no get-modify-set race conditions
    return s.sessions.Update(ctx, sessionID, func(old *Session, exists bool) (*Session, error) {
        if !exists {
            return nil, ErrSessionNotFound
        }
        old.LastAccessedAt = time.Now()
        return old, nil
    }, s.defaultTTL)
}

func (s *SessionManager) DeleteAllForUser(ctx context.Context, userID string) error {
    // Thread-safe bulk operations
    return s.sessions.DeleteByIndex(ctx, "sessions_by_user", userID)
}
```

## Performance Considerations

### Memory Provider Optimization

- **Efficient Maps**: Use of sync.Map for high-concurrency scenarios where appropriate
- **Batch Cleanup**: Process expired entries in batches to reduce lock contention
- **Index Maintenance**: Lazy index updates to minimize write amplification
- **Memory Pooling**: Reuse of frequently allocated objects

### Redis Provider Optimization

- **Connection Pooling**: Efficient connection management and reuse
- **Pipeline Operations**: Batch operations for better throughput
- **Lua Scripts**: Atomic operations for complex scenarios
- **Compression**: Optional compression for large values

### General Performance Guidelines

- **Avoid Hot Paths**: Keep critical path operations minimal
- **Measure Everything**: Comprehensive benchmarking and profiling
- **Memory Conscious**: Track and optimize memory allocation patterns
- **Concurrent Design**: Assume high-concurrency usage patterns

## Security Considerations

### Timing Attack Protection

All cache operations should have consistent response times regardless of:

- Key existence
- Value size
- Cache hit/miss status
- Error conditions

Implementation uses configurable minimum processing times and dummy operations.

### Secure Cleanup

Sensitive data (sessions, tokens, etc.) must be securely wiped from memory:

- Zero memory before deallocation
- Clear index references
- Audit cleanup operations

### Access Control Integration

Support for pre/post operation hooks enables:

- Permission validation
- Audit logging
- Rate limiting
- Security event collection

## Testing Strategy

### Unit Testing Requirements

- **Provider Compliance**: All providers pass identical interface tests
- **Concurrency Safety**: Tests under high-concurrency loads
- **Error Handling**: All error paths properly tested
- **Memory Safety**: No memory leaks under sustained operation

### Integration Testing Requirements

- **Cross-Provider Compatibility**: Same behavior across all providers
- **Real-World Scenarios**: Tests matching actual usage patterns
- **Performance Characteristics**: Throughput and latency validation
- **Security Features**: Timing protection and cleanup validation

### Benchmark Testing Requirements

- **Operation Performance**: Latency and throughput for all operations
- **Memory Usage**: Allocation patterns and peak usage
- **Scaling Behavior**: Performance under increasing load
- **Provider Comparison**: Relative performance between providers

## Common Patterns and Anti-Patterns

### ✅ Recommended Patterns (NEW - Generic & Atomic)

```go
// Use typed caches for compile-time safety
sessions := manager.NewCache[*models.Session]("memory")

// Use atomic operations instead of manual locking
session, err := sessions.GetOrSet(ctx, sessionID, loadFromDB, ttl)

// Leverage Update for atomic modifications
session, err := sessions.Update(ctx, sessionID, func(old *Session, exists bool) (*Session, error) {
    if !exists { return nil, ErrNotFound }
    old.LastAccessed = time.Now()
    return old, nil
}, ttl)

// Use batch operations for performance
sessionMap, err := sessions.GetMany(ctx, sessionIDs)

// Leverage thread-safe indexing
keys, err := sessions.GetByIndex(ctx, "sessions_by_user", userID)
```

### ❌ Anti-Patterns to Avoid (Updated for New Design)

```go
// DON'T: Try to use mutexes or locks with the cache
var mu sync.Mutex
mu.Lock() // ❌ WRONG - cache handles all concurrency internally
session, _ := cache.Get(ctx, key)
session.Update()
cache.Set(ctx, key, session, ttl)
mu.Unlock()

// DO: Use atomic Update operation instead
session, err := cache.Update(ctx, key, func(old *Session, exists bool) (*Session, error) {
    return old.WithUpdate(), nil // ✅ Atomic and thread-safe
}, ttl)

// DON'T: Try to use interface{} or type assertions
var cache Cache[any] // ❌ Defeats the purpose of generics
value := cache.Get(ctx, key).(Session) // ❌ Runtime type assertion

// DO: Use typed cache
var cache Cache[*Session] // ✅ Compile-time type safety
session, found, err := cache.Get(ctx, key) // ✅ Returns *Session directly

// DON'T: Implement your own caching patterns
if exists := cache.Has(ctx, key); !exists { // ❌ Race condition
    cache.Set(ctx, key, value, ttl)
}

// DO: Use atomic GetOrSet
value, err := cache.GetOrSet(ctx, key, loader, ttl) // ✅ Atomic
```

## Troubleshooting

### Common Issues

1. **Memory Leaks**: Usually caused by missing cleanup or index maintenance
2. **Performance Degradation**: Often due to inefficient indexing or large object serialization
3. **Inconsistent Behavior**: Different providers may have subtle behavioral differences
4. **Connection Issues**: Redis provider connection pool exhaustion or timeouts

### Debugging Tools

- **Metrics Dashboard**: Monitor hit ratios, latency, error rates
- **Memory Profiling**: Track allocation patterns and peak usage
- **Operation Tracing**: Detailed logging of cache operations
- **Health Checks**: Provider-specific health monitoring

### Performance Optimization

- **Index Strategy**: Choose indexes carefully to avoid maintenance overhead
- **Serialization**: Select appropriate serialization format for your data
- **TTL Tuning**: Balance cache effectiveness with memory usage
- **Provider Selection**: Choose provider based on deployment architecture

## Strategic Refactoring Status

### Current Implementation Status

🚧 **In Progress - Strategic Refactoring** 🚧

The go-cache module is undergoing aggressive refactoring to achieve:
- **Generic-first interfaces** with `Cache[T]` - **BREAKING CHANGES**
- **Thread-safe atomic operations** eliminating all consumer-side locking  
- **Performance targets**: >1M ops/sec (memory), >100k ops/sec (Redis)
- **Enterprise features preserved**: secondary indexing, metrics, security, middleware

### Migration Impact

⚠️ **Breaking Changes Planned** ⚠️
- **No backward compatibility** - clean design takes priority
- **Consumer refactoring required** - dramatic simplification expected
- **Performance improvements**: 5-10x faster concurrent operations
- **Code complexity reduction**: >50% fewer lines of code in consumers

### Implementation Plan

See: `/improvements/overhaul_improvement_plan.md` for detailed roadmap

**Phase Status:**
- [ ] Phase 0: Core Interface Design 
- [ ] Phase 1: Concurrency Architecture & Guarantees
- [ ] Phase 2: High-Performance In-Memory Provider
- [ ] Phase 3: Distributed Redis Provider  
- [ ] Phase 4: Extreme Testing & Performance Validation
- [ ] Phase 5: Consumer Migration & Optimization
- [ ] Phase 6: Production Readiness & Observability

### Future Integration Opportunities

- **Advanced Eviction Policies**: LRU, LFU, custom strategies with atomic operations
- **Multi-Level Caching**: L1 (memory) + L2 (Redis) with consistent generic interfaces
- **Cache Warming**: Type-safe preloading with batch operations
- **Distributed Coordination**: Enhanced Redis locks and consensus operations

---

This module is being refactored to be the **fastest, safest, most maintainable** caching layer for Go applications, with particular focus on authentication systems, session management, and high-concurrency use cases that demand both performance and reliability.

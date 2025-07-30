# CLAUDE.md - Go-Cache Module

## Project Overview

Go-Cache is a pluggable, high-performance caching abstraction for Go applications that provides unified interfaces for multiple cache providers (memory, Redis, etc.) with comprehensive observability, security features, and advanced functionality like secondary indexing.

### Key Features

- **Provider Abstraction**: Unified interface supporting memory, Redis, and extensible to other providers
- **Secondary Indexing**: Support for complex queries beyond simple key-value operations
- **Security-First Design**: Timing attack protection, secure cleanup, lifecycle hooks
- **Comprehensive Observability**: Detailed metrics, structured logging, performance monitoring
- **Thread-Safe Operations**: All providers designed for high-concurrency usage
- **Serialization Support**: Pluggable serialization with multiple format support

## Architecture

### Core Components

```
┌─────────────────────┐     ┌─────────────────────┐
│   Cache Manager     │────▶│  Cache Interface    │
│ (Factory Pattern)   │     │   (Unified API)     │
└─────────────────────┘     └─────────────────────┘
         │                           │
         │                  ┌────────┴────────┐
         │                  ▼                 ▼
         │          ┌──────────────┐  ┌──────────────┐
         │          │   Memory     │  │    Redis     │
         │          │  Provider    │  │  Provider    │
         │          └──────────────┘  └──────────────┘
         │                  │                 │
    ┌────┴────┐            │                 │
    ▼         ▼            │                 │
┌────────┐ ┌────────┐     │                 │
│Metrics │ │ Hooks  │     │                 │
│System  │ │ System │     │                 │
└────────┘ └────────┘     │                 │
    │         │            │                 │
    └─────────┴────────────┴─────────────────┘
              │
    ┌─────────▼─────────┐
    │   Serialization   │
    │      System       │
    └───────────────────┘
```

### Design Principles

1. **Interface Segregation**: Clean, focused interfaces that don't force unnecessary dependencies
2. **Provider Agnostic**: Applications shouldn't know or care which cache provider is used
3. **Observability First**: All operations instrumented with metrics and logging
4. **Security Conscious**: Built-in protections against timing attacks and data leakage
5. **Performance Optimized**: Minimal overhead, efficient memory usage, fast operations
6. **Extensible**: Easy to add new providers, serializers, or features

## Development Guidelines

### Code Style

1. **Interface-First Design**: All major functionality exposed through interfaces
2. **Error Handling**: Detailed, typed errors with proper context
3. **Concurrency**: All components must be thread-safe by design
4. **Testing**: Comprehensive unit, integration, and performance tests
5. **Documentation**: All public APIs thoroughly documented with examples

### Provider Implementation Requirements

**All providers must implement:**
- Complete `Cache` interface
- Thread-safe operations
- Proper error handling and propagation
- Metrics collection integration
- Lifecycle hook support
- Serialization abstraction
- Memory cleanup (no leaks)

**Provider-Specific Features:**
- **Memory Provider**: Fast local caching, automatic cleanup, secondary indexing
- **Redis Provider**: Distributed caching, cluster support, persistence, Lua scripts

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

### Core Cache Interface

```go
type Cache interface {
    // Basic operations
    Get(ctx context.Context, key string) (any, bool, error)
    Set(ctx context.Context, key string, value any, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    
    // Advanced operations
    GetByIndex(ctx context.Context, indexName string, indexKey string) ([]string, error)
    AddIndex(ctx context.Context, indexName string, keyPattern string, indexKey string) error
    UpdateMetadata(ctx context.Context, key string, updater MetadataUpdater) error
    
    // Lifecycle
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

### Basic Cache Operations

```go
// Initialize cache
cache, err := manager.GetCache("sessions", 
    WithProvider("memory"),
    WithTTL(24*time.Hour),
    WithCleanupInterval(5*time.Minute),
)

// Basic operations
err = cache.Set(ctx, "session:123", session, time.Hour)
value, found, err := cache.Get(ctx, "session:123")
err = cache.Delete(ctx, "session:123")
```

### Secondary Indexing

```go
// Add indexing for complex queries
err = cache.AddIndex(ctx, "sessions_by_user", "session:*", "user:456")

// Query by index
sessionKeys, err := cache.GetByIndex(ctx, "sessions_by_user", "user:456")

// Bulk operations using indexes
err = cache.DeleteByIndex(ctx, "sessions_by_user", "user:456")
```

### With Security Features

```go
cache, err := manager.GetCache("secure_sessions",
    WithSecurity(&SecurityConfig{
        EnableTimingProtection: true,
        MinProcessingTime:      5*time.Millisecond,
        SecureCleanup:         true,
    }),
    WithHooks(&CacheHooks{
        PreGet: func(ctx context.Context, key string) error {
            // Validate access permissions
            return validateAccess(ctx, key)
        },
    }),
)
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

**Expected Integration Pattern:**

```go
// go-auth session manager using go-cache
type UnifiedSessionManager struct {
    cache      cache.Cache
    // ... other fields
}

func (s *UnifiedSessionManager) Create(ctx context.Context, authResult *AuthResult, metadata map[string]any) (*Session, error) {
    session := &Session{...}
    
    // Store session with automatic TTL
    err := s.cache.Set(ctx, s.sessionKey(session.ID), session, s.defaultTTL)
    
    // Add to user index for bulk operations
    err = s.cache.AddIndex(ctx, "sessions_by_user", "session:*", s.userIndexKey(session.SubjectID))
    
    return session, err
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

### ✅ Recommended Patterns

```go
// Use context for cancellation
value, found, err := cache.Get(ctx, key)

// Check errors explicitly
if err != nil {
    return fmt.Errorf("cache operation failed: %w", err)
}

// Use TTL consistently
err = cache.Set(ctx, key, value, cache.DefaultTTL)

// Leverage indexes for complex queries
keys, err := cache.GetByIndex(ctx, "user_sessions", userID)
```

### ❌ Anti-Patterns to Avoid

```go
// Don't ignore errors
cache.Set(ctx, key, value, ttl) // Missing error check

// Don't use empty context
cache.Get(context.Background(), key) // Use request context

// Don't bypass TTL without reason
cache.Set(ctx, key, value, 0) // Use appropriate TTL

// Don't assume key existence
value := cache.Get(ctx, key).(Session) // Check found boolean
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

## Future Enhancements

### Planned Features

1. **Advanced Eviction Policies**: LRU, LFU, custom eviction strategies
2. **Distributed Locking**: Coordination primitives for distributed scenarios
3. **Cache Warming**: Proactive loading of frequently accessed data
4. **Multi-Level Caching**: L1 (memory) + L2 (Redis) hierarchical caching
5. **Compression Support**: Automatic compression for large values
6. **Partitioning**: Sharding support for very large datasets

### Integration Opportunities

- **Prometheus Metrics**: Native Prometheus metrics export
- **OpenTelemetry**: Distributed tracing support
- **Health Checks**: Kubernetes-style health endpoints
- **Configuration Management**: Hot-reload of cache configuration

---

This module is designed to be the foundational caching layer for complex Go applications, with particular focus on supporting authentication systems, session management, and other security-critical use cases that require both performance and reliability.
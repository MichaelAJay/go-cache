# CacheManager Design Plan

## Problem Statement

Based on session management use case requirements:
- **Sessions**: Store in Memory OR Redis (runtime configurable)
- **Indexes**: Always Memory for performance (subjectId→sessions, sessionId→subjectId)
- **Efficiency**: Should reuse provider instances, not create duplicates

## Core Design Questions

### 1. Provider Instance Management

**CRITICAL CONSTRAINT**: Serialization type conflicts prevent provider reuse

**Problem**: In-memory providers use gob serialization which requires knowing the target type at deserialization. One provider instance cannot serve multiple generic types:
```go
// This WILL NOT WORK - gob serialization conflict
memProvider := memory.NewProvider(opts)  
cache1 := memProvider.NewCache[*Session]("sessions")     // Expects *Session
cache2 := memProvider.NewCache[[]string]("subject-index") // Expects []string - CONFLICT!
```

**Revised Approach**: Each cache needs its own provider instance
```go
// Each cache gets its own type-specific provider
sessionsProvider := memory.NewProvider[*Session](opts)
indexProvider := memory.NewProvider[[]string](opts)
```

**Impact on Manager**: CacheManager cannot share provider instances across different types

### 2. Cache Identification & Naming

**Current Issue**: No way to distinguish caches from same provider

**Requirements**:
- Multiple caches per provider need unique identifiers
- Cache names should be meaningful for debugging/monitoring
- Type safety must be preserved

**Proposed Solution**:
```go
sessions := manager.NewCache[*Session]("sessions", "memory", opts...)
subjectIndex := manager.NewCache[[]string]("subject-index", "memory", opts...)
```

### 3. Configuration Management

**Challenge**: Different caches from same provider may need different configs
- Sessions: TTL=24h, MaxEntries=10000
- Indexes: TTL=forever, MaxEntries=100000

**Options**:
1. **Cache-level options**: Pass options to NewCache() 
2. **Provider-level defaults**: Provider handles cache-specific config
3. **Hybrid**: Provider defaults + cache overrides

### 4. Runtime Provider Selection

**Use Case**: Session provider determined by config at startup
```go
sessionProvider := config.GetSessionProvider() // "memory" or "redis"
sessions := manager.NewCache[*Session]("sessions", sessionProvider, opts...)
```

**Requirements**:
- Clean syntax for provider selection
- Fail fast if provider not registered
- Type-safe cache creation regardless of provider

### 5. Lifecycle Management

**Current Issue**: Close() method panics, unclear ownership model

**Requirements**:
- Manager should own provider lifecycles
- Closing manager should close all providers
- Individual cache closure should be possible
- Resource cleanup must be deterministic

### 6. Type Safety & Storage

**Current Issue**: `map[string]any` loses compile-time safety

**Challenge**: Manager needs to store caches with different generic types
- Cache[*Session]
- Cache[[]string]  
- Cache[string]

**Possible Solutions**:
1. **Type erasure with runtime checks**
2. **Interface-based storage** 
3. **Separate typed registries**

## Proposed API Design

### Manager Creation & Provider Registration
```go
manager := cache.NewManager()

// Register provider factories (not instances, due to type constraints)
manager.RegisterProviderFactory("memory", memory.NewProviderFactory())
manager.RegisterProviderFactory("redis", redis.NewProviderFactory())
```

### Cache Creation
```go
// Named caches with provider selection
sessions, err := manager.NewCache[*Session]("sessions", sessionProvider, 
    cache.WithTTL(24*time.Hour),
    cache.WithMaxEntries(10000),
)

subjectIndex, err := manager.NewCache[[]string]("subject-index", "memory",
    cache.WithTTL(0), // no expiration
    cache.WithMaxEntries(100000),
)

sessionToSubjectIndex, err := manager.NewCache[string]("session-to-subject", "memory",
    cache.WithTTL(0),
    cache.WithMaxEntries(100000),
)
```

### Resource Management
```go
// Close specific cache
err := sessions.Close()

// Close all caches and providers
err := manager.Close()
```

## Implementation Requirements

### 1. Manager Structure
```go
type CacheManager struct {
    mu               sync.RWMutex
    providerFactories map[string]ProviderFactory  // factories, not instances
    caches           map[string]cacheEntry        // cache metadata + closer interface
}

type ProviderFactory interface {
    CreateProvider[T any](opts ...Option) (CacheProvider[T], error)
}

type cacheEntry struct {
    name     string
    provider string
    closer   io.Closer // type-erased closer interface
    // metadata for monitoring/debugging
}
```

### 2. Provider Interface Requirements
- Providers must support creating multiple named caches
- Providers must handle cache-specific configuration
- Providers must support efficient resource sharing

### 3. Error Handling
- Clear errors for unregistered providers
- Validation of cache names (uniqueness)
- Proper error propagation from providers

### 4. Thread Safety
- Manager operations must be thread-safe
- Multiple goroutines can create caches concurrently
- Provider registration should be startup-time only

### 5. Monitoring & Debugging
- Cache enumeration for health checks
- Provider status reporting  
- Cache metadata exposure

## Benefits for Session Management Use Case

### Clean Runtime Selection
```go
// Configuration drives provider choice
sessions, err := manager.NewCache[*Session]("sessions", config.SessionProvider, sessionOpts)
if err != nil {
    return fmt.Errorf("failed to create session cache: %w", err)
}
```

### Revised Index Strategy (Due to Serialization Constraints)
```go
// Forward index uses cache (subjectId → []sessionIds)
subjectIndex, _ := manager.NewCache[[]string]("subject-index", "memory", indexOpts)

// Reverse index lives in session manager memory (sessionId → subjectId)
// Avoids serialization complexity and is more efficient for 1M sessions (~80MB)
type SessionManager struct {
    sessions      Cache[*Session]
    subjectIndex  Cache[[]string]  
    reverseIndex  map[string]string  // In-memory: sessionId → subjectId
    mu           sync.RWMutex       // Protect reverse index
}
```

### Centralized Lifecycle
```go
// Single call cleans up all caching resources
defer manager.Close()
```

### Type Safety Preserved
```go
// No interface{} or type assertions needed
session, found, err := sessions.Get(ctx, sessionID)  // Returns *Session
userSessions, err := subjectIndex.Get(ctx, userID)   // Returns []string
```

## Next Steps

1. **Finalize API design** based on this analysis
2. **Update interface definitions** to support named caches
3. **Implement core manager functionality**
4. **Add provider interface requirements**
5. **Create comprehensive tests** including the session management scenario
6. **Update provider implementations** to support manager pattern

## Open Questions

1. Should cache names be globally unique or unique per provider?
2. How should cache-specific metrics be exposed through the manager?
3. Should the manager support cache discovery/enumeration APIs?
4. How should configuration inheritance work (manager → provider → cache)?
5. Should there be a fluent builder pattern for complex cache configurations?
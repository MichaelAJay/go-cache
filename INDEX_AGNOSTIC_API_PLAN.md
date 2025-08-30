# Index-Agnostic Public API Plan

## Executive Summary

This plan restructures the go-cache public API to be completely **index-agnostic**, where consumers interact with a simple, unified interface while the cache provider handles indexing optimizations transparently. The core principle: **consumers specify WHAT they want, not HOW to retrieve it**.

## Current Problem Statement

The existing `Cache[T]` interface exposes indexing implementation details to consumers:

```go
// PROBLEMATIC: Consumer must choose implementation strategy
session, found, err := cache.Get(ctx, "session:123")           // Direct lookup
sessions, err := cache.GetByIndex(ctx, "sessions_by_user", userID) // Index lookup
cache.AddIndex(ctx, "sessions_by_user", "session:*", userID)     // Manual index management
```

This creates:
- ❌ **API Complexity**: Consumers must understand indexing mechanics
- ❌ **Implementation Coupling**: Consumer code tied to cache internal strategy
- ❌ **Race Conditions**: Manual index management allows inconsistent state
- ❌ **Performance Complexity**: Consumer must choose optimal retrieval method

## Target Architecture

### 1. Owner-Based Index-Agnostic API

```go
type Cache[T any] interface {
    // CORE OPERATIONS: Simple, consistent interface
    Get(ctx context.Context, key string) (value T, found bool, err error)
    Set(ctx context.Context, key string, value T, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Clear(ctx context.Context) error
    Has(ctx context.Context, key string) bool
    
    // OWNER-BASED QUERIES: Cache handles owner extraction internally
    GetByOwner(ctx context.Context, ownerID string) ([]T, error)
    DeleteByOwner(ctx context.Context, ownerID string) (deletedCount int, err error)
    
    // ATOMIC OPERATIONS: Thread-safe, index-aware internally
    GetOrSet(ctx context.Context, key string, loader func(ctx context.Context) (T, error), ttl time.Duration) (T, error)
    Update(ctx context.Context, key string, updater func(old T, exists bool) (T, error), ttl time.Duration) (T, error)
    
    // BATCH OPERATIONS: Optimized automatically
    GetMany(ctx context.Context, keys []string) (map[string]T, error)
    SetMany(ctx context.Context, items map[string]T, ttl time.Duration) error
    DeleteMany(ctx context.Context, keys []string) error
    
    // CONDITIONAL OPERATIONS: Atomic across data + indexes
    SetIfNotExists(ctx context.Context, key string, value T, ttl time.Duration) (wasSet bool, err error)
    SetIfExists(ctx context.Context, key string, value T, ttl time.Duration) (wasSet bool, err error)
    
    // PATTERN OPERATIONS: For advanced use cases
    GetKeysByPattern(ctx context.Context, pattern string) ([]string, error)
    DeleteByPattern(ctx context.Context, pattern string) (deletedCount int, err error)
    
    // METADATA: Read-only introspection
    GetMetadata(ctx context.Context, key string) (*CacheEntryMetadata, error)
    
    // LIFECYCLE: Resource management
    Close() error
}
```

### 2. Owner-Based Configuration

```go
// OwnerExtractor: Simple function to extract owner from data
// Configured once at cache creation, not per operation
type OwnerExtractor[T any] func(value T) string

// Example extractors for common patterns:
// sessionOwnerExtractor := func(session *Session) string { return session.UserID }
// documentOwnerExtractor := func(doc *Document) string { return doc.OrganizationID }
// tokenOwnerExtractor := func(token *APIToken) string { return token.ApplicationID }
```

### 3. Setup-Time Configuration

```go
// Simple owner-based configuration
type CacheOptions struct {
    // Basic configuration
    TTL              time.Duration
    MaxEntries       int
    CleanupInterval  time.Duration
    
    // OWNER-BASED INDEXING: Simple setup-time configuration
    OwnerExtractor   OwnerExtractor[T] // Optional - enables owner-based queries
    
    // Provider-specific optimizations
    ProviderOptions map[string]any
    
    // Observability
    Metrics CacheMetrics
    Hooks   *CacheHooks
}
```

## Implementation Strategy

### Phase 1: Public API Refactoring

**Objective**: Eliminate indexing methods from public interface

**Tasks**:
1. **Remove Index Management Methods**:
   - Delete `AddIndex()`, `RemoveIndex()` from `Cache[T]` interface
   - Replace with configuration-driven approach

2. **Consolidate Query Methods**:
   - Replace `GetByIndex()` with unified `Query()` method
   - Replace `DeleteByIndex()` with unified `DeleteBy()` method

3. **Add Configuration Types**:
   - Create `QueryCriteria` struct for declarative queries
   - Create `IndexExtractor[T]` function type for index configuration
   - Add indexing options to `CacheOptions`

4. **Update Interface Documentation**:
   - Emphasize index-agnostic consumer experience
   - Document internal optimization guarantees

### Phase 2: Internal Implementation Architecture

**Objective**: Smart internal indexing that's transparent to consumers

**Tasks**:
1. **Provider Interface Enhancement**:
   ```go
   // Internal provider interface - NOT public
   type CacheProvider[T any] interface {
       // Standard operations with automatic index management
       Set(ctx context.Context, key string, value T, ttl time.Duration, indexExtractions IndexExtractions) error
       Get(ctx context.Context, key string) (T, bool, error)
       Delete(ctx context.Context, key string, indexExtractions IndexExtractions) error
       
       // Query optimization - provider chooses best strategy
       Query(ctx context.Context, criteria QueryCriteria, fallbackToScan bool) ([]T, error)
       
       // Lifecycle with index cleanup
       Close() error
   }
   ```

2. **Lua Script Integration** (Redis Provider):
   - Atomic data + index operations via Lua scripts
   - Query optimization: index lookup vs. pattern scan
   - Batch operations with index consistency

3. **Memory Provider Optimization**:
   - In-memory index structures (concurrent maps)
   - Atomic operations with index maintenance
   - Pattern matching fallbacks

### Phase 3: Consumer Experience Enhancement

**Objective**: Seamless, performant consumer API

**Tasks**:
1. **Cache Manager Enhancement**:
   ```go
   // Simple owner-based cache creation
   manager := cache.NewManager()
   
   // Session cache with user-based indexing
   sessions, err := manager.NewCache[*Session]("redis",
       WithTTL(24*time.Hour),
       WithOwnerExtractor(func(session *Session) string {
           return session.UserID // Extract owner at setup time
       }),
   )
   
   // Document cache with organization-based indexing  
   documents, err := manager.NewCache[*Document]("redis",
       WithTTL(1*time.Hour),
       WithOwnerExtractor(func(doc *Document) string {
           return doc.OrganizationID // Different owner type
       }),
   )
   
   // Simple cache without indexing
   tokens, err := manager.NewCache[*APIToken]("memory",
       WithTTL(15*time.Minute),
       // No OwnerExtractor = no indexing, just fast key-value
   )
   ```

2. **Graceful Degradation**:
   - Same API works with or without indexing enabled
   - Automatic fallback to pattern matching when indexes unavailable
   - Performance hints via metrics/logging

3. **Error Handling Enhancement**:
   - Clear error messages that don't expose internal indexing details
   - Validation errors for malformed query criteria
   - Timeout handling for complex queries

## Consumer Usage Patterns

### Basic Operations (Index-Agnostic)

```go
// Consumer code is simple and consistent
session := &Session{ID: "sess123", UserID: "user456", DeviceType: "mobile"}

// SET: Cache extracts owner automatically using configured OwnerExtractor
err = sessions.Set(ctx, session.ID, session, time.Hour)
// Internal: cache calls ownerExtractor(session) -> "user456"
//          Lua script updates data + owner indexes atomically

// GET: Always the same call - direct key lookup
session, found, err := sessions.Get(ctx, "sess123")  
// Internal: Fast O(1) direct key lookup

// DELETE: Cache handles index cleanup automatically
err = sessions.Delete(ctx, "sess123")
// Internal: Removes data + cleans up owner indexes atomically
```

### Owner-Based Query Operations

```go
// Get all sessions for a user - cache handles indexing internally
userSessions, err := sessions.GetByOwner(ctx, "user456")
// Internal: Fast O(1) owner index lookup via Lua script
//          Returns []*Session directly (type-safe)

// Bulk delete all sessions for a user
deletedCount, err := sessions.DeleteByOwner(ctx, "user456") 
// Internal: Atomic bulk delete with complete index cleanup

// Standard operations work seamlessly with indexing
session, err := sessions.GetOrSet(ctx, "sess123", func(ctx context.Context) (*Session, error) {
    return loadSessionFromDB(ctx, "sess123")
}, time.Hour)
// Internal: If new session created, owner indexes updated automatically
```

### Multiple Access Patterns (Different Cache Instances)

```go
// Same data, different access patterns via separate caches
userSessionCache := manager.NewCache[*Session]("redis", WithOwnerExtractor(func(s *Session) string {
    return s.UserID  // Access sessions by user
}))

orgSessionCache := manager.NewCache[*Session]("redis", WithOwnerExtractor(func(s *Session) string {
    return s.OrgID   // Access sessions by organization  
}))

// Each cache optimized for its specific query pattern
userSessions, err := userSessionCache.GetByOwner(ctx, "user456")
orgSessions, err := orgSessionCache.GetByOwner(ctx, "org789")
```

## Internal Architecture Details

### Redis Provider with Lua Scripts

```lua
-- Example: Atomic set with owner index updates  
-- SET_WITH_OWNER.lua
local element_key = KEYS[1]
local value = ARGV[1] 
local ttl = tonumber(ARGV[2])
local owner_id = ARGV[3]

-- Set primary data
redis.call('SETEX', element_key, ttl, value)

if owner_id and owner_id ~= "" then
    -- Update owner -> elements index
    local owner_set_key = 'owner:' .. owner_id
    redis.call('SADD', owner_set_key, element_key)
    redis.call('EXPIRE', owner_set_key, ttl)
    
    -- Update element -> owner reverse index (for cleanup)
    local reverse_key = 'rev:' .. element_key  
    redis.call('SETEX', reverse_key, ttl, owner_id)
end

return "OK"
```

```lua
-- Example: Get all elements for owner
-- GET_BY_OWNER.lua
local owner_id = ARGV[1]
local owner_set_key = 'owner:' .. owner_id

-- Get all element keys for this owner
local element_keys = redis.call('SMEMBERS', owner_set_key)

-- Retrieve all values
local results = {}
for _, element_key in ipairs(element_keys) do
    local value = redis.call('GET', element_key)
    if value then
        table.insert(results, {element_key, value})
    else
        -- Clean up stale index entry
        redis.call('SREM', owner_set_key, element_key)
    end
end

return results
```

```lua
-- Example: Delete element with index cleanup
-- DELETE_WITH_CLEANUP.lua  
local element_key = KEYS[1]
local reverse_key = 'rev:' .. element_key

-- Get owner from reverse index
local owner_id = redis.call('GET', reverse_key)

-- Delete primary data
local deleted = redis.call('DEL', element_key)

-- Clean up indexes if element existed
if deleted > 0 and owner_id then
    local owner_set_key = 'owner:' .. owner_id
    redis.call('SREM', owner_set_key, element_key)
    redis.call('DEL', reverse_key)
end

return deleted
```

### Memory Provider with Owner Indexing

```go
// Internal memory provider implementation
type MemoryProvider[T any] struct {
    // Primary data storage
    data        sync.Map  // elementKey -> *CacheEntry[T]
    
    // Owner indexing (only if OwnerExtractor configured)
    ownerIndex   sync.Map  // ownerID -> *sync.Map[elementKey -> struct{}] 
    reverseIndex sync.Map  // elementKey -> ownerID
    
    // Configuration  
    ownerExtractor OwnerExtractor[T] // nil if no indexing
    
    // Cleanup
    cleanup        *time.Ticker
    closed         chan struct{}
}

func (m *MemoryProvider[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
    entry := &CacheEntry[T]{
        Value:     value,
        ExpiresAt: time.Now().Add(ttl),
    }
    
    // Store primary data
    m.data.Store(key, entry)
    
    // Update owner indexes if configured
    if m.ownerExtractor != nil {
        ownerID := m.ownerExtractor(value)
        m.updateOwnerIndex(key, ownerID)
    }
    
    return nil
}

func (m *MemoryProvider[T]) GetByOwner(ctx context.Context, ownerID string) ([]T, error) {
    if m.ownerExtractor == nil {
        return nil, fmt.Errorf("owner indexing not enabled for this cache")
    }
    
    // Get owner's element set
    elementsInterface, exists := m.ownerIndex.Load(ownerID)
    if !exists {
        return []T{}, nil // No elements for this owner
    }
    
    elements := elementsInterface.(*sync.Map)
    var results []T
    
    elements.Range(func(keyInterface, _ any) bool {
        key := keyInterface.(string)
        if entryInterface, found := m.data.Load(key); found {
            entry := entryInterface.(*CacheEntry[T])
            if !entry.IsExpired() {
                results = append(results, entry.Value)
            } else {
                // Clean up expired entry
                m.deleteWithCleanup(key)
            }
        }
        return true
    })
    
    return results, nil
}

func (m *MemoryProvider[T]) updateOwnerIndex(elementKey, ownerID string) {
    // Remove from old owner if exists
    if oldOwnerInterface, exists := m.reverseIndex.Load(elementKey); exists {
        oldOwner := oldOwnerInterface.(string)
        if oldElementsInterface, found := m.ownerIndex.Load(oldOwner); found {
            oldElements := oldElementsInterface.(*sync.Map)
            oldElements.Delete(elementKey)
        }
    }
    
    // Add to new owner
    elementsInterface, _ := m.ownerIndex.LoadOrStore(ownerID, &sync.Map{})
    elements := elementsInterface.(*sync.Map)
    elements.Store(elementKey, struct{}{})
    
    // Update reverse index
    m.reverseIndex.Store(elementKey, ownerID)
}
```

## Migration Strategy

### Backward Compatibility Approach

**BREAKING CHANGES ACCEPTED**: This is a strategic refactoring prioritizing clean design

**Migration Steps**:

1. **Phase 1**: Introduce new interface alongside old (temporary)
2. **Phase 2**: Update internal implementations to support both APIs  
3. **Phase 3**: Migrate consumers to new API
4. **Phase 4**: Remove deprecated methods
5. **Phase 5**: Performance optimization and refinement

### Consumer Migration Example

```go
// BEFORE: Manual index management (complex, error-prone)
cache.Set(ctx, "session:123", session, ttl)
cache.AddIndex(ctx, "sessions_by_user", "session:*", session.SubjectID)
userSessions, err := cache.GetByIndex(ctx, "sessions_by_user", userID)

// AFTER: Owner-based, setup-time configuration (simple, safe)
sessions, err := manager.NewCache[*Session]("redis",
    WithTTL(24*time.Hour),
    WithOwnerExtractor(func(session *Session) string {
        return session.UserID // Configured once at setup
    }),
)

sessions.Set(ctx, "session:123", session, ttl)  // Owner index updated automatically
userSessions, err := sessions.GetByOwner(ctx, userID)  // Type-safe, fast lookup
```

## Success Metrics

### API Simplicity Metrics
- ✅ **Reduced Method Count**: `Cache[T]` interface simplified to core operations + `GetByOwner()`/`DeleteByOwner()`
- ✅ **Zero Manual Index Management**: No `AddIndex()`, `RemoveIndex()` - all automatic
- ✅ **Setup-Time Configuration**: Owner extraction configured once, not per operation
- ✅ **Type-Safe Results**: `GetByOwner()` returns `[]T`, not `[]string` keys

### Performance Metrics
- ✅ **Index Query Performance**: <1ms for index-based queries (Redis)
- ✅ **Fallback Performance**: Graceful degradation to pattern matching
- ✅ **Atomic Operations**: Zero race conditions in concurrent index updates

### Maintainability Metrics  
- ✅ **Consumer Code Reduction**: 50%+ reduction in cache-related consumer code
- ✅ **Implementation Flexibility**: Easy to add new providers without API changes
- ✅ **Testing Simplicity**: Single API surface for comprehensive testing

## Risk Assessment & Mitigation

### High-Risk Areas

1. **Query Performance Without Indexes**
   - **Risk**: Fallback pattern matching may be slow for large datasets
   - **Mitigation**: Clear documentation, performance monitoring, automatic warnings

2. **Index Configuration Complexity**  
   - **Risk**: Complex IndexExtractor functions may be error-prone
   - **Mitigation**: Comprehensive examples, validation, testing utilities

3. **Provider Implementation Complexity**
   - **Risk**: Internal indexing logic increases provider complexity
   - **Mitigation**: Shared utilities, comprehensive test suites, clear interfaces

### Medium-Risk Areas

1. **Migration Complexity**
   - **Risk**: Consumers may resist API changes
   - **Mitigation**: Clear migration guides, performance benefits demonstration

2. **Debugging Difficulty**
   - **Risk**: Transparent optimizations may be hard to debug
   - **Mitigation**: Rich metrics, debug modes, query explanation features

## Conclusion

This plan transforms go-cache from an implementation-aware API to a **declarative, optimization-transparent interface**. Consumers specify their intent, and the cache handles all implementation complexity internally.

**Core Benefits**:
- 🎯 **Simplified Consumer Experience**: Always use `Get()`, `Set()`, `Delete()`, `Query()`
- ⚡ **Transparent Performance**: Cache chooses optimal retrieval strategy automatically  
- 🔒 **Atomic Consistency**: Lua scripts ensure data + index consistency
- 🔄 **Future-Proof**: Easy to add new optimization strategies without API changes
- 🧪 **Testable**: Single interface surface reduces testing complexity

The API becomes a **smart abstraction** that grows more powerful internally while remaining simple externally - exactly what a cache library should be.
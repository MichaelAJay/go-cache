# RedisCache Refactor Plan: Clean Abstraction & Atomic Operations

## Problem Statement

The current RedisCache implementation has fundamental design flaws that break clean abstraction and atomic guarantees:

1. **Broken Abstraction**: All consumers must provide `IndexExtractor` even for basic caching
2. **Non-Atomic Indexing**: Uses pipelining instead of Lua scripts, breaking atomicity
3. **Inconsistent Implementation**: Some operations use Lua scripts, others use pipelining
4. **Poor Separation of Concerns**: Indexing logic mixed into all basic operations

## Target Design Goals

### Clean Consumer Abstraction

```go
// Basic caching - consumer doesn't know/care about indexing
basicCache, err := NewCache[User](client)
err = basicCache.Set(ctx, user, ttl) // Just works, no indexing complexity

// Advanced caching with indexing - consumer opts in
indexedCache, err := NewCache[Session](client, &IndexExtractor[Session]{...})
err = indexedCache.Set(ctx, session, ttl) // Same API, indexing handled transparently
```

### Internal Implementation Principles

- **Indexing Decision Made Once**: At initialization, not per-operation
- **Atomic Operations**: All data + index operations use Lua scripts when indexing enabled
- **Branch-Based Implementation**: Operations branch cleanly based on indexing mode
- **Fail-Fast Validation**: Configuration errors caught at initialization

## Detailed Implementation Plan

### Phase 1: API Design Changes

#### 1.1 Constructor Signature Change

**Current (Broken)**:
```go
func NewCache[T any](client redis.Cmdable, opts ...Option[T]) (interfaces.Cache[T], error)
```

**New (Clean)**:
```go
func NewCache[T any](client redis.Cmdable, extractor *IndexExtractor[T], opts ...Option[T]) (interfaces.Cache[T], error)
```

**Rationale**: 
- `extractor *IndexExtractor[T]` parameter enables initialization-time validation
- `nil` extractor = basic caching mode
- Non-nil extractor = indexed caching mode
- Removes need for `WithIndexExtractor` functional option

#### 1.2 Remove IndexExtractor Functional Option

**Remove**:
```go
func WithIndexExtractor[T any](extractor IndexExtractor[T]) Option[T] // DELETE THIS
```

**Rationale**: Functional options are applied after struct creation, making initialization-time validation impossible.

#### 1.3 Add Internal Indexing Flag

```go
type RedisCache[T any] struct {
    // ... existing fields
    extractor    *IndexExtractor[T] // nil = no indexing
    indexingMode bool               // derived from extractor != nil
}
```

### Phase 2: Initialization Logic Changes

#### 2.1 Constructor Implementation

```go
func NewCache[T any](client redis.Cmdable, extractor *IndexExtractor[T], opts ...Option[T]) (interfaces.Cache[T], error) {
    if client == nil {
        return nil, fmt.Errorf("redis client cannot be nil")
    }

    cache := &RedisCache[T]{
        client:       client,
        extractor:    extractor,
        indexingMode: extractor != nil,
        options:      config.DefaultOptions(),
        instanceID:   generateInstanceID(),
    }

    // Apply remaining functional options
    for _, opt := range opts {
        opt(cache)
    }

    // CRITICAL: Validate indexing configuration at initialization
    if err := cache.validateIndexingConfig(); err != nil {
        return nil, fmt.Errorf("indexing configuration error: %w", err)
    }

    if err := cache.initialize(); err != nil {
        return nil, fmt.Errorf("failed to initialize cache: %w", err)
    }

    return cache, nil
}
```

#### 2.2 Indexing Configuration Validation

```go
func (c *RedisCache[T]) validateIndexingConfig() error {
    if !c.indexingMode {
        return nil // Basic mode - no validation needed
    }

    // Indexing mode - validate extractors
    if c.extractor.GetEntryKey == nil {
        return fmt.Errorf("indexing enabled but IndexExtractor.GetEntryKey is nil")
    }
    if c.extractor.GetOwnerKey == nil {
        return fmt.Errorf("indexing enabled but IndexExtractor.GetOwnerKey is nil") 
    }

    return nil
}
```

### Phase 3: Lua Script Architecture

#### 3.1 Basic Set Script (No Indexing)

```go
// setBasicScript - atomic set without indexing
c.setBasicScript = redis.NewScript(`
    local dataKey = KEYS[1]
    local metaKey = KEYS[2]
    local ttl     = tonumber(ARGV[1])
    local value   = ARGV[2]
    
    -- Set data
    if ttl and ttl > 0 then
        redis.call('SETEX', dataKey, ttl, value)
    else
        redis.call('SET', dataKey, value)
    end
    
    -- Set metadata
    local now = redis.call('TIME')
    local ts  = now[1]
    redis.call('HSET', metaKey,
        'created_at', ts,
        'last_accessed', ts,
        'access_count', '1',
        'ttl', tostring(ttl or 0),
        'size', tostring(string.len(value))
    )
    if ttl and ttl > 0 then
        redis.call('EXPIRE', metaKey, ttl)
    end
    
    return 'OK'
`)
```

#### 3.2 Indexed Set Script (With Indexing)

```go
// setIndexedScript - atomic set with indexing
c.setIndexedScript = redis.NewScript(`
    local dataKey  = KEYS[1]
    local metaKey  = KEYS[2]  
    local indexKey = KEYS[3]
    local ttl      = tonumber(ARGV[1])
    local value    = ARGV[2]
    local entryKey = ARGV[3]
    
    -- Set data
    if ttl and ttl > 0 then
        redis.call('SETEX', dataKey, ttl, value)
    else
        redis.call('SET', dataKey, value)
    end
    
    -- Set metadata  
    local now = redis.call('TIME')
    local ts  = now[1]
    redis.call('HSET', metaKey,
        'created_at', ts,
        'last_accessed', ts,
        'access_count', '1', 
        'ttl', tostring(ttl or 0),
        'size', tostring(string.len(value))
    )
    if ttl and ttl > 0 then
        redis.call('EXPIRE', metaKey, ttl)
    end
    
    -- Update index atomically
    redis.call('SADD', indexKey, entryKey)
    if ttl and ttl > 0 then
        redis.call('EXPIRE', indexKey, ttl)
    end
    
    return 'OK'
`)
```

### Phase 4: Operation Implementation Changes

#### 4.1 Set Method Refactor

```go
func (c *RedisCache[T]) Set(ctx context.Context, value T, ttl time.Duration) error {
    start := time.Now()

    if c.isCircuitBreakerOpen() {
        c.metrics.RecordError("redis", "set", "circuit_breaker", "availability", c.getMetricTags())
        return cacheErrors.ErrCircuitBreakerOpen
    }

    // Serialize value
    serializedValue, err := c.serializer.Serialize(value)
    if err != nil {
        c.metrics.RecordError("redis", "set", "serialization_error", "data", c.getMetricTags())
        return fmt.Errorf("serialization error: %w", err)
    }

    if c.indexingMode {
        return c.setWithIndexing(ctx, value, serializedValue, ttl, start)
    } else {
        return c.setBasic(ctx, value, serializedValue, ttl, start)
    }
}

func (c *RedisCache[T]) setBasic(ctx context.Context, value T, serializedValue []byte, ttl time.Duration, start time.Time) error {
    // Extract key - still needed for storage even without indexing
    key := c.extractor.GetEntryKey(value)
    dataKey := c.buildDataKey(key)
    metaKey := c.buildMetaKey(key)

    _, err := c.setBasicScript.Run(ctx, c.client, 
        []string{dataKey, metaKey}, 
        int64(ttl.Seconds()), string(serializedValue)).Result()

    if err != nil {
        c.handleError("set", err)
        c.metrics.RecordError("redis", "set", "redis_error", "infrastructure", c.getMetricTags())
        return fmt.Errorf("Redis set error: %w", err)
    }

    c.metrics.RecordOperation("redis", "set", "success", time.Since(start), c.getMetricTags())
    return nil
}

func (c *RedisCache[T]) setWithIndexing(ctx context.Context, value T, serializedValue []byte, ttl time.Duration, start time.Time) error {
    key := c.extractor.GetEntryKey(value)
    ownerKey := c.extractor.GetOwnerKey(value)
    
    dataKey := c.buildDataKey(key)
    metaKey := c.buildMetaKey(key)
    indexKey := c.buildIndexKey("owner", ownerKey)

    _, err := c.setIndexedScript.Run(ctx, c.client,
        []string{dataKey, metaKey, indexKey},
        int64(ttl.Seconds()), string(serializedValue), key).Result()

    if err != nil {
        c.handleError("set", err)
        c.metrics.RecordError("redis", "set", "redis_error", "infrastructure", c.getMetricTags())
        return fmt.Errorf("Redis set error: %w", err)
    }

    c.metrics.RecordOperation("redis", "set", "success", time.Since(start), c.getMetricTags())
    return nil
}
```

#### 4.2 Owner Operations Validation

```go
func (c *RedisCache[T]) GetByOwner(ctx context.Context, ownerKey string) ([]T, error) {
    if !c.indexingMode {
        return nil, fmt.Errorf("GetByOwner requires indexing to be enabled")
    }
    // ... existing implementation
}

func (c *RedisCache[T]) DeleteByOwner(ctx context.Context, ownerKey string) (int, error) {
    if !c.indexingMode {
        return 0, fmt.Errorf("DeleteByOwner requires indexing to be enabled")
    }
    // ... existing implementation
}
```

### Phase 5: Error Handling & Edge Cases

#### 5.1 Initialization Error Cases

```go
var (
    ErrIndexingMissingGetEntryKey = errors.New("indexing enabled but GetEntryKey is nil")
    ErrIndexingMissingGetOwnerKey = errors.New("indexing enabled but GetOwnerKey is nil")
    ErrOwnerOperationRequiresIndexing = errors.New("owner-based operation requires indexing to be enabled")
)
```

### Phase 6: Testing Strategy

#### 6.1 Unit Test Structure

```go
func TestRedisCache_BasicMode(t *testing.T) {
    // Test cache without indexing
    cache, err := NewCache[TestData](mockClient, nil)
    // ... test basic operations work
    // ... verify owner operations return proper errors
}

func TestRedisCache_IndexedMode(t *testing.T) {
    extractor := &IndexExtractor[TestData]{...}
    cache, err := NewCache[TestData](mockClient, extractor)
    // ... test all operations work including owner-based
}

func TestRedisCache_InitializationValidation(t *testing.T) {
    // Test various invalid extractor configurations
    // Test that errors are caught at initialization
}
```

#### 6.2 Atomicity Tests

```go
func TestRedisCache_AtomicIndexing(t *testing.T) {
    // High-concurrency test to verify Set operations are truly atomic
    // Verify no partial index updates occur
}
```

## Implementation Checklist

### API Changes
- [ ] Change NewCache signature to accept `*IndexExtractor[T]` parameter
- [ ] Remove `WithIndexExtractor` functional option
- [ ] Add `indexingMode bool` field to RedisCache struct
- [ ] Add initialization-time validation

### Script Development
- [ ] Create `setBasicScript` for non-indexed operations
- [ ] Create `setIndexedScript` for indexed operations
- [ ] Update existing scripts to handle both modes appropriately
- [ ] Remove pipeline-based Set implementation

### Operation Updates  
- [ ] Refactor Set method with mode branching
- [ ] Update owner-based operations to check indexing mode
- [ ] Ensure all operations respect indexing mode
- [ ] Add proper error messages for mode mismatches

### Testing & Validation
- [ ] Write comprehensive initialization validation tests
- [ ] Add atomicity tests for indexed operations
- [ ] Test both basic and indexed modes extensively
- [ ] Performance benchmarks for both modes

### Documentation
- [ ] Update API documentation with new patterns
- [ ] Add usage examples for both modes  
- [ ] Update README with new patterns

## Expected Outcomes

### For Consumers

**Basic Caching** (90% of use cases):
```go
cache, err := NewCache[User](client, nil)
err = cache.Set(ctx, user, ttl) // Clean, simple, fast
```

**Advanced Caching** (10% of use cases):
```go
extractor := &IndexExtractor[Session]{...}
cache, err := NewCache[Session](client, extractor)  
err = cache.Set(ctx, session, ttl) // Same API, indexing transparent
sessions, err := cache.GetByOwner(ctx, userID) // Advanced features available
```

### For Implementation

- **Atomic Guarantees**: All indexed operations use Lua scripts
- **Clean Separation**: Indexing complexity isolated from basic operations  
- **Fail-Fast Validation**: Configuration errors caught immediately
- **Performance Optimization**: Basic mode has zero indexing overhead

This refactor achieves the clean abstraction you described while fixing the fundamental atomicity and design issues in the current implementation.
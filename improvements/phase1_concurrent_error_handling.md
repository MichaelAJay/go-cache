# Phase 1: Concurrent Error Handling Patterns

## Error Handling Philosophy for Concurrent Operations

### Core Principles

**Principle 1: Fail Fast with Context**
- **Detect errors early** in the operation chain
- **Provide rich context** about the specific failure
- **Preserve original errors** while adding operation context
- **Enable easy debugging** with clear error traces

**Principle 2: Atomic Error Semantics**
- **Operations either succeed completely or fail completely**
- **No partial success states** visible to consumers
- **Rollback mechanisms** for multi-step operations
- **Consistent error behavior** across all providers

**Principle 3: Graceful Degradation**
- **Continue serving** what can be served during partial failures
- **Circuit breaker patterns** to prevent cascade failures
- **Retry with exponential backoff** for transient errors
- **Clear distinction** between permanent and temporary failures

**Principle 4: Thread-Safe Error Propagation**
- **Errors are immutable once created**
- **Safe to share across goroutines** without additional synchronization
- **Error aggregation** for batch operations
- **Timeout and cancellation** respect for all operations

## Error Type Hierarchy

### Base Error Types

```go
// Core cache errors - immutable and thread-safe
var (
    ErrKeyNotFound           = errors.New("key not found")
    ErrKeyExists            = errors.New("key already exists")
    ErrInvalidKey           = errors.New("invalid key format")
    ErrInvalidValue         = errors.New("invalid value")
    ErrInvalidTTL           = errors.New("invalid TTL value")
    ErrCacheClosed          = errors.New("cache is closed")
    ErrResourceExhausted    = errors.New("resource exhausted")
    ErrTimeout              = errors.New("operation timeout")
    ErrConcurrentModification = errors.New("concurrent modification detected")
    ErrSerializationFailed  = errors.New("serialization failed")
    ErrDeserializationFailed = errors.New("deserialization failed")
    ErrIndexNotFound        = errors.New("index not found")
    ErrPatternInvalid       = errors.New("invalid pattern")
)
```

### Structured Error Types

```go
// CacheError provides rich context for cache operations
type CacheError struct {
    Op       string                 // Operation name (Get, Set, Update, etc.)
    Key      string                // Key involved in operation  
    Provider string                // Provider name (memory, redis)
    Shard    int                   // Shard number (for memory provider)
    Err      error                 // Underlying error
    Context  map[string]interface{} // Additional context
    Time     time.Time             // When error occurred
}

func (e *CacheError) Error() string {
    return fmt.Sprintf("cache %s operation failed on key '%s' (provider: %s): %v", 
        e.Op, e.Key, e.Provider, e.Err)
}

func (e *CacheError) Unwrap() error {
    return e.Err
}

func (e *CacheError) Is(target error) bool {
    return errors.Is(e.Err, target)
}

// Thread-safe context addition
func (e *CacheError) WithContext(key string, value interface{}) *CacheError {
    // Create new error with additional context (immutable pattern)
    newContext := make(map[string]interface{}, len(e.Context)+1)
    for k, v := range e.Context {
        newContext[k] = v
    }
    newContext[key] = value
    
    return &CacheError{
        Op:       e.Op,
        Key:      e.Key,
        Provider: e.Provider,
        Shard:    e.Shard,
        Err:      e.Err,
        Context:  newContext,
        Time:     e.Time,
    }
}
```

### Batch Operation Errors

```go
// BatchError aggregates errors from batch operations
type BatchError struct {
    Op           string                    // Batch operation name
    Provider     string                   // Provider name
    TotalKeys    int                      // Total keys in batch
    SuccessCount int                      // Number of successful operations
    Errors       map[string]error         // Per-key errors
    Time         time.Time               // When batch started
}

func (be *BatchError) Error() string {
    return fmt.Sprintf("batch %s operation had %d errors out of %d keys (provider: %s)", 
        be.Op, len(be.Errors), be.TotalKeys, be.Provider)
}

// Check if batch had any errors
func (be *BatchError) HasErrors() bool {
    return len(be.Errors) > 0
}

// Get errors for specific keys
func (be *BatchError) ErrorsForKeys(keys []string) map[string]error {
    result := make(map[string]error)
    for _, key := range keys {
        if err, exists := be.Errors[key]; exists {
            result[key] = err
        }
    }
    return result
}

// Thread-safe error addition during batch processing
func (be *BatchError) addError(key string, err error) {
    // This would be called from synchronized context during batch processing
    be.Errors[key] = err
}
```

### Provider-Specific Errors

```go
// MemoryProviderError for memory-specific issues
type MemoryProviderError struct {
    Shard     int
    Operation string
    Err       error
    
    // Memory-specific context
    CurrentSize int64
    MaxSize     int64
    LoadFactor  float64
}

func (e *MemoryProviderError) Error() string {
    return fmt.Sprintf("memory provider error in shard %d during %s: %v (size: %d/%d, load: %.2f)", 
        e.Shard, e.Operation, e.Err, e.CurrentSize, e.MaxSize, e.LoadFactor)
}

// RedisProviderError for Redis-specific issues  
type RedisProviderError struct {
    Network     bool   // Network-related error
    Retryable   bool   // Whether operation can be retried
    Operation   string
    RedisCmd    string // Redis command that failed
    Err         error
    
    // Redis-specific context
    ConnectionCount int
    Pool            string
    Cluster         bool
}

func (e *RedisProviderError) Error() string {
    errorType := "redis"
    if e.Network {
        errorType = "redis-network"
    }
    return fmt.Sprintf("%s provider error during %s (%s): %v (connections: %d, pool: %s)", 
        errorType, e.Operation, e.RedisCmd, e.Err, e.ConnectionCount, e.Pool)
}

func (e *RedisProviderError) IsRetryable() bool {
    return e.Retryable && !e.Network // Don't retry network errors immediately
}
```

## Concurrent Error Handling Strategies

### Atomic Operation Error Handling

```go
// GetOrSet error handling with atomic semantics
func (c *cache) GetOrSet(ctx context.Context, key string, loader func(ctx context.Context) (T, error), ttl time.Duration) (T, error) {
    var zero T
    
    // Input validation - fail fast
    if key == "" {
        return zero, &CacheError{
            Op:       "GetOrSet",
            Key:      key,
            Provider: c.provider.Name(),
            Err:      ErrInvalidKey,
            Time:     time.Now(),
        }
    }
    
    // Context timeout check
    if ctx.Err() != nil {
        return zero, &CacheError{
            Op:       "GetOrSet",
            Key:      key,
            Provider: c.provider.Name(),
            Err:      ctx.Err(),
            Context:  map[string]interface{}{"timeout": "context already expired"},
            Time:     time.Now(),
        }
    }
    
    startTime := time.Now()
    
    // Attempt operation with timeout
    result, err := c.provider.GetOrSet(ctx, key, loader, ttl)
    if err != nil {
        // Enrich error with operation context
        if cacheErr, ok := err.(*CacheError); ok {
            return zero, cacheErr.WithContext("duration", time.Since(startTime))
        }
        
        return zero, &CacheError{
            Op:       "GetOrSet",
            Key:      key,
            Provider: c.provider.Name(),
            Err:      err,
            Context: map[string]interface{}{
                "duration": time.Since(startTime),
                "ttl":      ttl,
            },
            Time: startTime,
        }
    }
    
    return result, nil
}
```

### Update Operation Error Handling

```go
// Update with optimistic concurrency error handling
func (p *memoryProvider) Update(ctx context.Context, key string, updater func(old T, exists bool) (T, error), ttl time.Duration) (T, error) {
    var zero T
    const maxRetries = 3
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        // Get current version atomically
        entry, exists := p.getEntry(key)
        var currentValue T
        var version int64
        
        if exists {
            var valid bool
            currentValue, valid = entry.getValue()
            if !valid {
                exists = false // Entry expired
            } else {
                version = entry.getVersion()
            }
        }
        
        // Execute updater function - isolate user code errors
        newValue, err := func() (T, error) {
            defer func() {
                if r := recover(); r != nil {
                    // Convert panic to error for thread safety
                    err = fmt.Errorf("updater function panicked: %v", r)
                }
            }()
            return updater(currentValue, exists)
        }()
        
        if err != nil {
            return zero, &CacheError{
                Op:       "Update",
                Key:      key,
                Provider: "memory",
                Err:      err,
                Context: map[string]interface{}{
                    "attempt": attempt + 1,
                    "exists":  exists,
                },
                Time: time.Now(),
            }
        }
        
        // Attempt atomic update
        success, updateErr := p.atomicUpdate(key, newValue, version, exists, ttl)
        if updateErr != nil {
            return zero, &CacheError{
                Op:       "Update",
                Key:      key,
                Provider: "memory",
                Err:      updateErr,
                Context: map[string]interface{}{
                    "attempt": attempt + 1,
                    "version": version,
                },
                Time: time.Now(),
            }
        }
        
        if success {
            return newValue, nil
        }
        
        // Concurrent modification detected - retry with backoff
        if attempt < maxRetries-1 {
            backoff := time.Duration(1<<attempt) * time.Millisecond
            select {
            case <-ctx.Done():
                return zero, &CacheError{
                    Op:       "Update",
                    Key:      key,
                    Provider: "memory",
                    Err:      ctx.Err(),
                    Context: map[string]interface{}{
                        "attempt": attempt + 1,
                        "backoff": backoff,
                    },
                    Time: time.Now(),
                }
            case <-time.After(backoff):
                // Continue to next attempt
            }
        }
    }
    
    // All retries exhausted
    return zero, &CacheError{
        Op:       "Update",
        Key:      key,
        Provider: "memory",
        Err:      ErrConcurrentModification,
        Context: map[string]interface{}{
            "maxRetries": maxRetries,
            "reason":     "too much contention",
        },
        Time: time.Now(),
    }
}
```

### Batch Operation Error Aggregation

```go
// GetMany with partial success handling
func (c *cache) GetMany(ctx context.Context, keys []string) (map[string]T, error) {
    if len(keys) == 0 {
        return make(map[string]T), nil
    }
    
    results := make(map[string]T, len(keys))
    batchErr := &BatchError{
        Op:           "GetMany",
        Provider:     c.provider.Name(),
        TotalKeys:    len(keys),
        SuccessCount: 0,
        Errors:       make(map[string]error),
        Time:         time.Now(),
    }
    
    // Process in parallel chunks for better performance
    const chunkSize = 100
    chunks := chunkKeys(keys, chunkSize)
    
    var wg sync.WaitGroup
    var resultMu sync.Mutex
    
    for _, chunk := range chunks {
        wg.Add(1)
        go func(keyChunk []string) {
            defer wg.Done()
            
            chunkResults, chunkErrs := c.provider.GetMany(ctx, keyChunk)
            
            resultMu.Lock()
            defer resultMu.Unlock()
            
            // Aggregate successful results
            for key, value := range chunkResults {
                results[key] = value
                batchErr.SuccessCount++
            }
            
            // Aggregate errors
            for key, err := range chunkErrs {
                batchErr.Errors[key] = &CacheError{
                    Op:       "GetMany",
                    Key:      key,
                    Provider: c.provider.Name(),
                    Err:      err,
                    Time:     time.Now(),
                }
            }
        }(chunk)
    }
    
    wg.Wait()
    
    // Return results with aggregated error info
    if batchErr.HasErrors() {
        return results, batchErr
    }
    
    return results, nil
}
```

## Error Recovery and Circuit Breaker Patterns

### Circuit Breaker for Provider Failures

```go
type CircuitBreaker struct {
    name        string
    maxFailures int
    resetTime   time.Duration
    
    // Thread-safe state
    failures    int64     // atomic
    lastFailure int64     // atomic, Unix timestamp
    state       int32     // atomic: 0=closed, 1=open, 2=half-open
}

const (
    CircuitClosed   = 0
    CircuitOpen     = 1
    CircuitHalfOpen = 2
)

func (cb *CircuitBreaker) Execute(ctx context.Context, operation func(ctx context.Context) error) error {
    currentState := atomic.LoadInt32(&cb.state)
    
    switch currentState {
    case CircuitOpen:
        // Check if reset time has passed
        lastFailure := atomic.LoadInt64(&cb.lastFailure)
        if time.Since(time.Unix(lastFailure, 0)) > cb.resetTime {
            // Transition to half-open
            if atomic.CompareAndSwapInt32(&cb.state, CircuitOpen, CircuitHalfOpen) {
                return cb.tryOperation(ctx, operation)
            }
        }
        return &CacheError{
            Op:       "CircuitBreaker",
            Provider: cb.name,
            Err:      errors.New("circuit breaker is open"),
            Context: map[string]interface{}{
                "failures":    atomic.LoadInt64(&cb.failures),
                "lastFailure": time.Unix(lastFailure, 0),
            },
            Time: time.Now(),
        }
        
    case CircuitHalfOpen:
        return cb.tryOperation(ctx, operation)
        
    default: // CircuitClosed
        err := operation(ctx)
        if err != nil {
            cb.recordFailure()
        } else {
            cb.recordSuccess()
        }
        return err
    }
}

func (cb *CircuitBreaker) tryOperation(ctx context.Context, operation func(ctx context.Context) error) error {
    err := operation(ctx)
    if err != nil {
        cb.recordFailure()
        // Transition back to open on failure
        atomic.StoreInt32(&cb.state, CircuitOpen)
        return err
    }
    
    // Success - transition to closed
    cb.recordSuccess()
    atomic.StoreInt32(&cb.state, CircuitClosed)
    return nil
}

func (cb *CircuitBreaker) recordFailure() {
    failures := atomic.AddInt64(&cb.failures, 1)
    atomic.StoreInt64(&cb.lastFailure, time.Now().Unix())
    
    // Open circuit if threshold exceeded
    if failures >= int64(cb.maxFailures) {
        atomic.StoreInt32(&cb.state, CircuitOpen)
    }
}

func (cb *CircuitBreaker) recordSuccess() {
    atomic.StoreInt64(&cb.failures, 0)
}
```

### Retry Strategies with Backoff

```go
type RetryConfig struct {
    MaxAttempts  int
    InitialDelay time.Duration
    MaxDelay     time.Duration
    Multiplier   float64
    Jitter       bool
}

func RetryWithExponentialBackoff(ctx context.Context, config RetryConfig, operation func(ctx context.Context) error) error {
    var lastErr error
    delay := config.InitialDelay
    
    for attempt := 0; attempt < config.MaxAttempts; attempt++ {
        // Execute operation
        err := operation(ctx)
        if err == nil {
            return nil // Success
        }
        
        lastErr = err
        
        // Check if error is retryable
        if !isRetryableError(err) {
            return &CacheError{
                Op:       "Retry",
                Err:      err,
                Context: map[string]interface{}{
                    "attempt":     attempt + 1,
                    "retryable":   false,
                    "reason":      "non-retryable error",
                },
                Time: time.Now(),
            }
        }
        
        // Don't delay after last attempt
        if attempt == config.MaxAttempts-1 {
            break
        }
        
        // Calculate backoff with jitter
        actualDelay := delay
        if config.Jitter {
            jitterRange := float64(delay) * 0.1 // 10% jitter
            jitter := time.Duration((rand.Float64() * 2 - 1) * jitterRange)
            actualDelay = delay + jitter
        }
        
        // Wait for backoff period or context cancellation
        select {
        case <-ctx.Done():
            return &CacheError{
                Op:       "Retry",
                Err:      ctx.Err(),
                Context: map[string]interface{}{
                    "attempt":    attempt + 1,
                    "lastError":  lastErr.Error(),
                    "cancelled":  true,
                },
                Time: time.Now(),
            }
        case <-time.After(actualDelay):
            // Continue to next attempt
        }
        
        // Calculate next delay
        delay = time.Duration(float64(delay) * config.Multiplier)
        if delay > config.MaxDelay {
            delay = config.MaxDelay
        }
    }
    
    // All attempts exhausted
    return &CacheError{
        Op:       "Retry",
        Err:      lastErr,
        Context: map[string]interface{}{
            "attempts":   config.MaxAttempts,
            "exhausted":  true,
            "finalDelay": delay,
        },
        Time: time.Now(),
    }
}

func isRetryableError(err error) bool {
    // Network errors are generally retryable
    if errors.Is(err, context.DeadlineExceeded) {
        return true
    }
    
    // Resource exhaustion might be temporary
    if errors.Is(err, ErrResourceExhausted) {
        return true
    }
    
    // Provider-specific retryable errors
    if redisErr, ok := err.(*RedisProviderError); ok {
        return redisErr.IsRetryable()
    }
    
    // Most cache errors are not retryable
    return false
}
```

### Graceful Degradation Patterns

```go
// Cache with fallback provider for resilience
type ResilientCache struct {
    primary   Cache[T]
    fallback  Cache[T]  // Usually memory cache as fallback
    breaker   *CircuitBreaker
}

func (rc *ResilientCache) Get(ctx context.Context, key string) (T, bool, error) {
    var zero T
    
    // Try primary cache with circuit breaker
    var primaryResult T
    var primaryFound bool
    var primaryErr error
    
    primaryErr = rc.breaker.Execute(ctx, func(ctx context.Context) error {
        result, found, err := rc.primary.Get(ctx, key)
        primaryResult = result
        primaryFound = found
        return err
    })
    
    if primaryErr == nil {
        return primaryResult, primaryFound, nil
    }
    
    // Primary failed - try fallback
    fallbackResult, fallbackFound, fallbackErr := rc.fallback.Get(ctx, key)
    if fallbackErr == nil {
        // Log degraded mode for monitoring
        log.Warn("cache operating in degraded mode", 
            "key", key, 
            "primaryError", primaryErr.Error(),
            "fallbackUsed", true)
        
        return fallbackResult, fallbackFound, &CacheError{
            Op:       "Get",
            Key:      key,
            Provider: "resilient",
            Err:      primaryErr,
            Context: map[string]interface{}{
                "degraded":      true,
                "fallbackUsed":  true,
                "primaryError":  primaryErr.Error(),
            },
            Time: time.Now(),
        }
    }
    
    // Both primary and fallback failed
    return zero, false, &CacheError{
        Op:       "Get",
        Key:      key,
        Provider: "resilient",
        Err:      fmt.Errorf("both primary and fallback failed: primary=%v, fallback=%v", primaryErr, fallbackErr),
        Context: map[string]interface{}{
            "primaryError":  primaryErr.Error(),
            "fallbackError": fallbackErr.Error(),
            "totalFailure":  true,
        },
        Time: time.Now(),
    }
}
```

This comprehensive error handling framework ensures robust, predictable behavior under all concurrent scenarios while providing rich diagnostic information for debugging and monitoring.
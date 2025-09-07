# Metrics Usage Guidelines

## Overview

This document establishes standards and guidelines for metrics usage in the go-cache codebase to prevent regression to legacy allocation-heavy patterns and ensure consistent, high-performance metrics collection.

## Background

The go-cache project has completed a comprehensive **Legacy Metrics Elimination** initiative that achieved:
- **18-57% allocation reductions** across all major cache operations
- **100% elimination** of legacy `c.metrics.*` patterns (48 calls replaced)
- **Zero-allocation metrics** through precomputed metrics architecture

This document ensures these performance gains are maintained and extended to all future development.

## Core Principles

### 1. Always Use Precomputed Metrics
**NEVER** use legacy `c.metrics.*` patterns. Always use precomputed metrics for zero-allocation performance.

❌ **WRONG - Legacy Pattern:**
```go
c.metrics.RecordOperation("redis", "get", "success", duration, c.getMetricTags())
c.metrics.RecordError("redis", "get", "circuit_breaker", "availability", c.getMetricTags())
```

✅ **CORRECT - Precomputed Pattern:**
```go
c.precomputedMetrics.GetTimer().Record(duration)
c.precomputedMetrics.GetSuccessCounter().Inc()
c.precomputedMetrics.GetCircuitBreakerErrorCounter().Inc()
```

### 2. Zero-Allocation Requirement
All metrics operations must achieve **0 B/op, 0 allocs/op** in benchmarks.

### 3. Complete Coverage Principle
Every cache operation must have appropriate precomputed metrics for:
- **Success Timing**: `OperationTimer().Record(duration)`
- **Success Counting**: `OperationSuccessCounter().Inc()`
- **Error Counting**: `OperationErrorTypeErrorCounter().Inc()` for each error type

## Precomputed Metrics Architecture

### Pattern Categories

#### Timer + Success Counter Pattern (Most Common)
Used for successful operations that require both timing and counting:
```go
start := time.Now()
// ... perform operation ...
duration := time.Since(start)

c.precomputedMetrics.OperationTimer().Record(duration)
c.precomputedMetrics.OperationSuccessCounter().Inc()
```

#### Error Counter Pattern
Used for error conditions (no timing required):
```go
c.precomputedMetrics.OperationCircuitBreakerErrorCounter().Inc()
c.precomputedMetrics.OperationRedisErrorCounter().Inc()
c.precomputedMetrics.OperationTimeoutErrorCounter().Inc()
```

#### Gauge Pattern
Used for current state values:
```go
c.precomputedMetrics.MemoryUsageGauge().Set(float64(memoryBytes))
```

#### Batch Counter Pattern
Used for batch operations:
```go
c.precomputedMetrics.OperationTimer().Record(duration)
c.precomputedMetrics.OperationBatchCounter().Inc() // Tracks batch count, not individual items
```

### Available Precomputed Metrics

#### Core Operations
- `ClearTimer()`, `ClearSuccessCounter()`, `ClearEmptyCounter()`, `ClearCircuitBreakerErrorCounter()`

#### Owner Operations  
- `GetByOwnerTimer()`, `GetByOwnerSuccessCounter()`, `GetByOwnerHitCounter()`, `GetByOwnerMissCounter()`
- `GetByOwnerCircuitBreakerErrorCounter()`, `GetByOwnerRedisErrorCounter()`, `GetByOwnerSerializationErrorCounter()`
- `DeleteByOwnerTimer()`, `DeleteByOwnerSuccessCounter()`, `DeleteByOwnerCircuitBreakerErrorCounter()`, `DeleteByOwnerRedisErrorCounter()`

#### Pattern Operations
- `GetKeysByPatternTimer()`, `GetKeysByPatternSuccessCounter()`, `GetKeysByPatternCircuitBreakerErrorCounter()`, `GetKeysByPatternRedisErrorCounter()`

#### Counter Operations
- `IncrementTimer()`, `IncrementSuccessCounter()`, `IncrementCircuitBreakerErrorCounter()`, `IncrementRedisErrorCounter()`, `IncrementTimeoutErrorCounter()`
- `DecrementTimer()`, `DecrementSuccessCounter()`, `DecrementCircuitBreakerErrorCounter()`, `DecrementRedisErrorCounter()`, `DecrementTimeoutErrorCounter()`
- `IncrementFloatTimer()`, `IncrementFloatSuccessCounter()`, `IncrementFloatCircuitBreakerErrorCounter()`, `IncrementFloatRedisErrorCounter()`, `IncrementFloatTimeoutErrorCounter()`

#### Lifecycle Operations
- `ExtendTTLTimer()`, `ExtendTTLSuccessCounter()`, `ExtendTTLCircuitBreakerErrorCounter()`, `ExtendTTLRedisErrorCounter()`, `ExtendTTLKeyNotFoundErrorCounter()`
- `TouchTimer()`, `TouchSuccessCounter()`, `TouchCircuitBreakerErrorCounter()`, `TouchRedisErrorCounter()`, `TouchKeyNotFoundErrorCounter()`
- `AppendFieldTimer()`, `AppendFieldSuccessCounter()`, `AppendFieldCircuitBreakerErrorCounter()`, `AppendFieldRedisErrorCounter()`, `AppendFieldUnsupportedOperationErrorCounter()`

#### Batch Operations
- `DeleteManyTimer()`, `DeleteManyBatchCounter()`, `DeleteManyCircuitBreakerErrorCounter()`, `DeleteManyRedisErrorCounter()`

#### Metadata Operations
- `GetMetadataTimer()`, `GetMetadataSuccessCounter()`, `GetMetadataNotFoundCounter()`, `GetMetadataCircuitBreakerErrorCounter()`, `GetMetadataRedisErrorCounter()`
- `CleanupOrphanedMetadataSuccessCounter()`

#### System Operations
- `MemoryUsageGauge()`, `MemoryPressureCounter()`, `SecurityEventCounter()`

## Development Guidelines

### Adding New Cache Operations

When adding new cache operations, follow this process:

1. **Design Metrics First**: Identify what metrics the operation needs (timing, success/error counters, etc.)

2. **Check Existing Precomputed Metrics**: Review available methods to see if they cover your operation

3. **Add Missing Precomputed Metrics**: If needed, add new precomputed methods to `metrics/precomputed_metrics.go`:
   ```go
   func (pcm *PrecomputedCacheMetrics) NewOperationTimer() metric.Timer {
       return pcm.newOperationTimer
   }
   
   func (pcm *PrecomputedCacheMetrics) NewOperationSuccessCounter() metric.Counter {
       return pcm.newOperationSuccessCounter
   }
   ```

4. **Initialize in Constructor**: Add initialization in `NewPrecomputedCacheMetrics()`:
   ```go
   newOperationTimer: createOperationTimer(registry, baseTags),
   newOperationSuccessCounter: createOperationSuccessCounter(registry, baseTags),
   ```

5. **Add Helper Functions**: Create helper functions following existing patterns:
   ```go
   func createOperationTimer(registry metric.Registry, baseTags metric.Tags) metric.Timer {
       tags := baseTags.Copy()
       tags["operation"] = "new_operation"
       return registry.GetOrCreateTimer("cache.operation.duration", tags)
   }
   ```

6. **Implement with Precomputed Patterns**: Use the precomputed methods in your operation:
   ```go
   func (c *redisCache[T]) NewOperation(ctx context.Context, key string) error {
       start := time.Now()
       
       // Perform operation...
       
       if circuitBreakerError {
           c.precomputedMetrics.NewOperationCircuitBreakerErrorCounter().Inc()
           return err
       }
       
       duration := time.Since(start)
       c.precomputedMetrics.NewOperationTimer().Record(duration)
       c.precomputedMetrics.NewOperationSuccessCounter().Inc()
       return nil
   }
   ```

7. **Add Tests**: Include tests for new precomputed metrics following existing patterns in `metrics/precomputed_metrics_test.go`

8. **Benchmark Zero Allocations**: Verify new metrics achieve 0 B/op, 0 allocs/op

### Error Handling Patterns

Different error types require different precomputed metrics:

```go
switch {
case circuitBreakerError:
    c.precomputedMetrics.OperationCircuitBreakerErrorCounter().Inc()
case redisError:
    c.precomputedMetrics.OperationRedisErrorCounter().Inc()
case timeoutError:
    c.precomputedMetrics.OperationTimeoutErrorCounter().Inc()
case keyNotFoundError:
    c.precomputedMetrics.OperationKeyNotFoundErrorCounter().Inc()
case serializationError:
    c.precomputedMetrics.OperationSerializationErrorCounter().Inc()
case unsupportedOperationError:
    c.precomputedMetrics.OperationUnsupportedOperationErrorCounter().Inc()
}
```

## Code Review Checklist

### Pre-Review Validation
- [ ] All new cache operations use precomputed metrics
- [ ] No `c.metrics.*` patterns in new code
- [ ] No `c.getMetricTags()` calls in new code
- [ ] All metrics operations follow established patterns

### During Review
- [ ] Success paths use Timer + Success Counter pattern
- [ ] Error paths use appropriate Error Counter pattern
- [ ] System metrics use appropriate Gauge/Counter pattern
- [ ] Batch operations use Timer + Batch Counter pattern
- [ ] All error types have dedicated counters

### Post-Review Validation
- [ ] Benchmark tests show 0 B/op, 0 allocs/op for new metrics
- [ ] All tests pass with no functional regressions
- [ ] New metrics initialize properly in constructor
- [ ] Helper functions follow naming conventions

### Regression Prevention
- [ ] No legacy `c.metrics.RecordOperation()` calls
- [ ] No legacy `c.metrics.RecordError()` calls
- [ ] No legacy `c.metrics.RecordBatchOperation()` calls
- [ ] No legacy `c.metrics.RecordMemoryUsage()` calls
- [ ] No legacy `c.getMetricTags()` usage

## Future-Proofing Measures

### Linting and Static Analysis
Consider adding linting rules to catch legacy patterns:
```bash
# Example grep-based checks in CI
if grep -r "c\.metrics\." --include="*.go" .; then
  echo "ERROR: Legacy metrics patterns detected"
  exit 1
fi

if grep -r "getMetricTags()" --include="*.go" .; then
  echo "ERROR: Legacy getMetricTags() usage detected"
  exit 1
fi
```

### Architecture Enforcement
- **Required PrecomputedMetrics**: All cache instances must use precomputed metrics
- **No Fallback Behavior**: Legacy metrics support has been completely removed
- **Type Safety**: All metrics use strongly-typed interfaces

### Performance Monitoring
- Include allocation benchmarks in CI/CD pipeline
- Set up alerts for allocation regressions (>5% increase)
- Regular performance audits of metrics operations

## Training Materials

### Quick Reference Card

**✅ DO:**
- Use `c.precomputedMetrics.OperationTimer().Record(duration)`
- Use `c.precomputedMetrics.OperationSuccessCounter().Inc()`
- Use specific error counters for each error type
- Benchmark all new metrics for zero allocations

**❌ DON'T:**
- Use `c.metrics.RecordOperation()` (legacy)
- Use `c.metrics.RecordError()` (legacy)
- Use `c.getMetricTags()` (legacy)
- Create maps for metric tags

### Common Mistakes to Avoid

1. **Legacy Pattern Usage**: Never use `c.metrics.*` patterns
2. **Missing Timing**: Remember to add Timer for success operations
3. **Generic Error Counters**: Use specific error type counters
4. **Tag Allocation**: Never create `map[string]string` for tags
5. **Forgetting Initialization**: Add new metrics to constructor

### Performance Benchmarking

Always verify new metrics performance:
```bash
go test -bench=BenchmarkNewOperationMetrics -benchmem
```

Expected output for precomputed metrics:
```
BenchmarkNewOperationTimer-8         1000000000  0.32 ns/op   0 B/op   0 allocs/op
BenchmarkNewOperationCounter-8       1000000000  0.36 ns/op   0 B/op   0 allocs/op
```

## Migration from Legacy Patterns

If you encounter legacy metrics patterns in existing code:

1. **Identify the Pattern**: Find `c.metrics.*` or `c.getMetricTags()` calls
2. **Map to Precomputed**: Use the mapping table in `LEGACY_METRICS_ELIMINATION_PLAN.md`
3. **Replace Systematically**: Follow the documented replacement patterns
4. **Test Thoroughly**: Ensure no functional regressions
5. **Benchmark**: Verify allocation improvements

## Support and Resources

- **Reference Implementation**: See `redis_cache.go`, `batch_operations.go`, `metadata.go` for examples
- **Complete Mapping Guide**: `LEGACY_METRICS_ELIMINATION_PLAN.md` Section 2.2
- **Test Examples**: `metrics/precomputed_metrics_test.go`
- **Performance Results**: `LEGACY_METRICS_ELIMINATION_PLAN.md` Phase 3/4 results

## Conclusion

Following these guidelines ensures:
- **High Performance**: Zero-allocation metrics maintain excellent performance
- **Consistency**: Standardized patterns across all operations
- **Maintainability**: Clear, predictable metrics implementation
- **Future-Proofing**: Prevents regression to inefficient legacy patterns

The investment in precomputed metrics architecture has delivered significant performance improvements and should be preserved and extended in all future development.
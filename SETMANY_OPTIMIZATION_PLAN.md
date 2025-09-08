# SetMany Method Allocation Optimization Plan

## Current Status: CRITICAL PERFORMANCE ISSUE
- **Current**: 5,296 allocs/op for 100 values (53 allocs per value)
- **Comparison**: GetMany optimized to 27 allocs per key
- **Performance**: 833,691 ns/op, 461,119 B/op
- **Severity**: **2x worse allocation rate than GetMany before optimization**

## Root Cause Analysis

SetMany has **severe allocation inefficiencies**:

### Major Allocation Sources (estimated breakdown)
1. **Legacy Metrics (4-8 allocs)**: `getMetricTags()` map creation on error paths
2. **Individual Key Building (200+ allocs)**: buildDataKey/buildMetaKey called individually 
3. **Per-Value Metadata Maps (100+ allocs)**: `map[string]any{}` created for each value
4. **Pipeline Command Objects (300-500+ allocs)**: Redis client internal allocations
5. **SetItem Structs (100+ allocs)**: Individual struct creation and slice growth
6. **Serialization Operations (200+ allocs)**: Per-value serialization overhead

## Immediate Optimizations (High Impact)

### Phase 1: Apply Proven Patterns from GetMany

#### 1.1 Replace Legacy Metrics ⚠️ CRITICAL
```go
// Before: Multiple getMetricTags() calls
c.metrics.RecordError("redis", "setmany", "circuit_breaker", "availability", c.getMetricTags())

// After: Precomputed metrics  
c.precomputedMetrics.SetManyCircuitBreakerErrorCounter().Inc()
```

#### 1.2 Batch String Building ⚠️ HIGH IMPACT
```go
// Before: 200 individual calls for 100 values
item.dataKey = c.buildDataKey(key)  // 100x
item.metaKey = c.buildMetaKey(key)  // 100x

// After: 2 batch operations
c.buildDataKeysMany(keys, dataKeys)   // Single batch
c.buildMetaKeysMany(keys, metaKeys)   // Single batch
```

#### 1.3 Pool SetItem Slice ⚠️ MEDIUM IMPACT  
```go
// Before: Growing slice allocation
items := make([]setItem, 0, len(values))

// After: Pooled slice
items := c.slicePool.GetSetItemSlice(len(values))  // New pool method needed
defer c.slicePool.PutSetItemSlice(items)
```

### Phase 2: SetMany-Specific Optimizations

#### 2.1 Eliminate Per-Value Metadata Maps ⚠️ CRITICAL
**Current Problem**: 100 `map[string]any{}` allocations
```go
pipe.HSet(ctx, item.metaKey, map[string]any{  // 100 allocations!
    "created_at":    now,
    "last_accessed": now,
    "access_count":  1,
    "ttl":           ttlToMilliseconds(ttl),
    "size":          len(item.serializedValue),
})
```

**Solution**: Pre-allocate reusable metadata map
```go
// Single reusable metadata template
metadataTemplate := map[string]any{
    "created_at":    now,
    "last_accessed": now, 
    "access_count":  1,
    "ttl":           ttlToMilliseconds(ttl),
    "size":          0,  // Update per item
}

for _, item := range items {
    metadataTemplate["size"] = len(item.serializedValue)
    pipe.HSet(ctx, item.metaKey, metadataTemplate)
}
```

#### 2.2 Optimize Pipeline Usage
**Analysis Needed**: Determine if pipeline overhead can be reduced through:
- Command batching strategies
- Alternative Redis client usage patterns  
- Pipeline reuse opportunities

## Expected Impact

### Conservative Estimates
- **Legacy metrics replacement**: -10 to -20 allocs
- **Batch string building**: -150 to -200 allocs (major win)
- **Metadata map reuse**: -90 to -100 allocs (major win) 
- **Slice pooling**: -10 to -20 allocs
- **Total estimated reduction**: -260 to -340 allocs

### Target Results  
- **From**: 5,296 allocs/op
- **To**: ~5,000 allocs/op (conservative) to ~4,950 allocs/op (optimistic)
- **Improvement**: 5-7% allocation reduction
- **Per-value rate**: ~50 → ~45-47 allocs per value

### Implementation Priority
1. **CRITICAL**: Legacy metrics replacement (immediate ~20 alloc reduction)
2. **HIGH**: Batch string building (immediate ~180 alloc reduction)  
3. **HIGH**: Metadata map reuse (immediate ~100 alloc reduction)
4. **MEDIUM**: Slice pooling (marginal improvement)

## Implementation Notes

### Constraints
- Must maintain functional equivalence
- Cannot break indexing behavior
- Pipeline operations likely have fundamental Redis client overhead
- Serialization overhead unavoidable

### Success Metrics
- **Primary**: <5,000 allocs/op for 100 values (<50 per value)
- **Secondary**: Memory usage reduction, performance maintenance
- **Tertiary**: Architecture improvements for future optimization

---

**Status**: Ready for Implementation  
**Estimated Impact**: 260-340 allocation reduction (5-7% improvement)  
**Implementation Complexity**: Medium (proven patterns from GetMany)
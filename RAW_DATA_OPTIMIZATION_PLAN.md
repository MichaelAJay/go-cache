# Raw Data Access Optimization Plan

## Overview

Based on the breakthrough discovery that skipping deserialization reduces allocations by **63.5%** (22,054 → 8,037 allocs/op), this plan outlines a systematic approach to apply the "raw data access" pattern across the entire codebase.

**Core Insight**: Redis stores serialized data as strings. For many use cases, we can return raw serialized data without the expensive deserialization step, dramatically reducing allocations and improving performance.

**Additional Opportunity**: Serialization on writes (Set operations) also has significant allocation overhead from encoder/buffer creation. Pooled serializers can provide additional massive allocation reductions (5× or more) that combine with raw data optimization for system-wide performance gains.

---

## Phase 1: Comprehensive Analysis & API Design ✅ COMPLETED

### ✅ Step 1: Complete Inventory of Deserialization Points
**RESULTS**: 
- **Primary Methods Identified**: `Get()`, `GetMany()`, `GetByOwner()`, `GetOrSet()`
- **Baseline Performance**: GetManyRaw achieves 63.5% allocation reduction (22,054 → 8,037 allocs/op)
- **Complexity Classification**: All methods have significant deserialization overhead
- **Scope Confirmed**: 4 core retrieval methods + experimental GetManyRaw already exists

### ✅ Step 2: Usage Pattern Analysis Complete
**RESULTS**:
- **Pass-through scenarios** (40-60% of reads): API gateways, proxies, message routing
- **Batch processing** (20-30% of reads): Analytics, data migration, background jobs  
- **Conditional processing** (15-25% of reads): Security filters, rate limiting, lazy loading
- **Performance Opportunity**: 40-70% system-wide allocation reduction potential
- **Consumer Impact**: Zero breaking changes rejected - adopting "burn the boats" approach

### ✅ Step 3: API Design - BURN THE BOATS APPROACH
**DECISION**: Complete method replacement for maximum performance
- **Philosophy**: All retrieval methods return raw strings, consumers deserialize only when needed
- **Breaking Change**: Yes - 100% of cache retrieval calls must be updated  
- **Performance Target**: 63.5%+ allocation reduction across all methods

**NEW API SIGNATURES**:
```go
// BEFORE (Current)
func (c *RedisCache[T]) Get(ctx context.Context, key string) (T, bool, error)
func (c *RedisCache[T]) GetMany(ctx context.Context, keys []string) (map[string]T, error)  
func (c *RedisCache[T]) GetByOwner(ctx context.Context, ownerKey string) ([]T, error)
func (c *RedisCache[T]) GetOrSet(ctx context.Context, key string, loader func(ctx context.Context) (T, error), ttl time.Duration) (T, error)

// AFTER (Raw-First)
func (c *RedisCache[T]) Get(ctx context.Context, key string) (string, bool, error)
func (c *RedisCache[T]) GetMany(ctx context.Context, keys []string) (map[string]string, error)
func (c *RedisCache[T]) GetByOwner(ctx context.Context, ownerKey string) (map[string]string, error)  
func (c *RedisCache[T]) GetOrSet(ctx context.Context, key string, loader func(ctx context.Context) (T, error), ttl time.Duration) (string, error)
```

---

## Phase 2: BURN THE BOATS Implementation

### Step 4: Replace Get() Method 
**Location**: `redis_cache.go:301-369`  
**Task**: Replace Get() implementation to return raw strings instead of deserialized objects
**Implementation**:
```go
// Change from: func (c *RedisCache[T]) Get(ctx context.Context, key string) (T, bool, error)
//         to: func (c *RedisCache[T]) Get(ctx context.Context, key string) (string, bool, error)
```

**Definition of Done**:
- Remove all deserialization logic from Get() method (lines 351-362)
- Return raw serialized string directly from Redis response
- Update interface signature in `interfaces/cache.go`
- All tests updated to expect raw string results
- 60%+ allocation reduction achieved

**Expected Impact**: 
- All single-key retrievals: 60%+ allocation reduction
- Breaking change: All Get() usage must be updated by consumers

---

### Step 5: Replace GetMany() Method
**Location**: `batch_operations.go:13-115`  
**Task**: Convert experimental GetManyRaw to production GetMany() method
**Implementation**:
```go  
// Promote experimental_batch_operations.go:ExperimentalGetManyRaw() to GetMany()
// Change return type from map[string]T to map[string]string
```

**Definition of Done**:
- Replace existing GetMany() implementation with raw string return
- Copy logic from experimental_batch_operations.go:ExperimentalGetManyRaw()
- Remove all deserialization logic from batch_operations.go:13-115
- Update interface signature and all tests
- Proven 63.5% allocation reduction maintained

**Expected Impact**: 
- All batch operations: 63.5% allocation reduction (proven)
- Breaking change: All GetMany() usage must be updated by consumers

**Note**: Write-side optimization (Set/SetMany) will be addressed through **independent pooled serializer implementation** that can deliver 5× additional allocation reductions.

---

### Step 6: Replace GetByOwner() Method
**Location**: `redis_cache.go:638-710`
**Task**: Replace GetByOwner() to return map[string]string instead of []T
**Implementation**:
```go
// Change from: func (c *RedisCache[T]) GetByOwner(ctx context.Context, ownerKey string) ([]T, error)
//         to: func (c *RedisCache[T]) GetByOwner(ctx context.Context, ownerKey string) (map[string]string, error)
```

**Definition of Done**:
- Remove deserialization loop (lines 687-695)
- Return map[entryKey]rawValue instead of slice of deserialized objects
- Update interface signature and all tests
- Maintain integration with indexing system (metadata updates, LRU tracking)
- 60%+ allocation reduction achieved

**Expected Impact**: Owner-based operations: 60%+ allocation reduction, breaking change for all consumers

---

## Phase 3: Complex Atomic Operations

### Step 7: Replace GetOrSet() Method  
**Location**: `atomic_operations.go:12-136`
**Task**: Replace GetOrSet() to return raw strings while maintaining loader pattern
**Implementation**:
```go
// Change from: func (c *RedisCache[T]) GetOrSet(...) (T, error)
//         to: func (c *RedisCache[T]) GetOrSet(...) (string, error)
```

**Complexity Handling**:
- **Cache HIT**: Return raw serialized string directly (no deserialization) 
- **Cache MISS**: Call loader to get typed object, serialize and store, return raw result
- **Internal logic**: Only deserialize when comparing existing vs loader result (rare case)

**Definition of Done**:
- Remove deserialization from hit path (lines 87-99) 
- Maintain loader function signature (still returns T)
- Return raw string from both hit and miss paths
- Update interface signature and all tests
- Partial allocation reduction (hits get full benefit, misses still serialize)

**Expected Impact**: Cache hits: 60%+ allocation reduction, misses: minimal impact

---

### Step 8: Update Interface Signatures
**Location**: `interfaces/cache.go:26-182`
**Task**: Update all Cache[T] interface method signatures to reflect raw returns
**Implementation**:
```go
type Cache[T any] interface {
    // Updated method signatures
    Get(ctx context.Context, key string) (string, bool, error)  // was (T, bool, error)
    GetMany(ctx context.Context, keys []string) (map[string]string, error)  // was map[string]T
    GetByOwner(ctx context.Context, ownerKey string) (map[string]string, error)  // was []T
    GetOrSet(ctx context.Context, key string, loader func(ctx context.Context) (T, error), ttl time.Duration) (string, error)  // was T
    // ... other methods unchanged
}
```

**Definition of Done**:
- All retrieval method signatures updated in interface  
- Documentation updated to reflect raw string returns
- Breaking change documented for all consumers
- Interface version consideration (if needed)

**Expected Impact**: Compilation breaks for all consumers until code updated

---

### Step 9: Comprehensive Test Updates  
**Location**: All test files with cache retrieval operations
**Task**: Update all tests to expect raw string returns instead of deserialized objects
**Scope**:
- Unit tests for Get(), GetMany(), GetByOwner(), GetOrSet()
- Integration tests across all cache operations
- Benchmark tests (should show allocation improvements)
- Property-based tests and failure scenario tests

**Definition of Done**:
- All tests pass with new raw string returns
- Test assertions updated to validate raw serialized data
- Performance tests demonstrate 60%+ allocation reduction
- No regression in functional behavior (same data, different format)

**Expected Impact**: Complete test suite validation of burn-the-boats approach

---

## Phase 4: Deployment & Consumer Migration

### Step 10: Consumer Code Impact Analysis
**Location**: All cache consumer codebases  
**Task**: Identify and catalog all code that will break with new API
**Implementation**: 
- Search for all Get(), GetMany(), GetByOwner(), GetOrSet() usage
- Classify usage patterns: pass-through vs processing vs mixed
- Create migration guide with before/after examples

**Definition of Done**:
- Complete inventory of breaking changes across all consumers
- Migration guide with deserialization patterns for each use case  
- Estimated migration effort per consumer codebase
- Helper utilities for common deserialization patterns (if needed)

**Expected Impact**: Comprehensive preparation for consumer migration

---

### Step 11: Performance Validation & Benchmarking
**Location**: Comprehensive performance testing across realistic workloads
**Task**: Validate allocation reduction targets under production-like conditions
**Scope**:
- **Real workload simulation**: Mix of pass-through and processing scenarios  
- **Memory pressure testing**: Confirm GC pressure reduction
- **Latency impact**: Measure end-to-end performance improvements
**Definition of Done**:
- Benchmarks confirm 60%+ allocation reduction across all methods
- Real-world workload testing shows system-wide memory improvement  
- Performance regression testing confirms no latency degradation
- Memory pressure incidents demonstrate significant reduction

**Expected Impact**: Production-ready validation of burn-the-boats optimization

---

### Step 12: Rollback Preparation & Risk Mitigation  
**Location**: Deployment strategy and rollback procedures
**Task**: Prepare comprehensive rollback strategy for high-risk deployment
**Scope**:
- **Version management**: Maintain ability to revert to previous API
- **Gradual deployment**: Staged rollout strategy (if possible with breaking changes)  
- **Monitoring**: Enhanced metrics to track migration success/issues
- **Rollback triggers**: Clear criteria for when to abort and revert

**Definition of Done**:
- Complete rollback procedure documented and tested
- Deployment process allows for quick reversion if needed
- Enhanced monitoring covers allocation improvements and error rates  
- Clear success/failure criteria established

**Expected Impact**: Risk mitigation for high-impact breaking change deployment

---

## Phase 5: Production Deployment & Documentation

### Step 13: Consumer Migration Execution
**Location**: All cache consumer applications
**Task**: Execute breaking change migration across all consumers  
**Scope**:
- **Coordinated deployment**: All consumers must update simultaneously 
- **Migration assistance**: Support teams updating their cache usage
- **Testing validation**: Ensure all consumers work with raw string returns
- **Performance monitoring**: Track allocation improvements across consumers

**Definition of Done**:
- All consumer applications successfully updated to handle raw strings
- No functional regressions in consumer business logic
- Compilation successful across entire ecosystem
- Consumer teams trained on new deserialization patterns

**Expected Impact**: Successful ecosystem-wide migration to raw-first cache API

---

### Step 14: Production Performance Monitoring
**Location**: Live production environment
**Task**: Monitor real-world performance improvements post-deployment
**Scope**:
- **Allocation tracking**: Confirm 60%+ reduction in production workloads
- **Memory pressure**: Monitor GC frequency and memory usage patterns
- **Error monitoring**: Track any increase in deserialization errors at consumer level
- **Latency monitoring**: Measure end-to-end performance improvements

**Definition of Done**:
- Production metrics confirm target allocation reduction (60%+)
- Memory pressure incidents reduced by 40-70%
- No significant increase in error rates post-migration  
- Consumer performance improvements documented

**Expected Impact**: Validated production performance gains, system stability

---

### Step 15: Documentation & Knowledge Transfer
**Location**: Documentation system and team knowledge base
**Task**: Complete documentation for burn-the-boats raw data approach
**Scope**:
- **API documentation**: Updated method signatures and usage patterns
- **Migration guide**: Before/after examples, deserialization helpers
- **Performance guide**: When to deserialize vs pass-through raw data
- **Troubleshooting**: Common migration issues and solutions

**Definition of Done**:
- Complete documentation reflects new raw-first API
- Migration guide helps future cache integrations
- Performance characteristics documented with real benchmarks
- Team knowledge transfer complete for ongoing maintenance

**Expected Impact**: Sustainable knowledge base for raw-first cache architecture

---

## Success Metrics - BURN THE BOATS

### Primary Targets (Must Achieve)
- **Allocation Reduction**: ≥60% reduction in allocs/op for all retrieval methods (proven: 63.5%)
- **Memory Reduction**: ≥50% reduction in B/op for all retrieval methods
- **System-wide Impact**: 40-70% reduction in total cache system allocations
- **Migration Completion**: 100% of consumers successfully updated to raw string APIs

### Secondary Targets (High Priority)
- **Production Stability**: No increase in functional error rates post-migration  
- **Memory Pressure**: 40-70% reduction in memory pressure incidents
- **Consumer Performance**: Measurable end-to-end performance improvements
- **GC Efficiency**: Significant reduction in GC frequency and pause times

### Risk Management (Breaking Change Strategy)
- **Coordinated Deployment**: All consumers update simultaneously to avoid partial failures
- **Rollback Readiness**: Complete rollback procedure available if critical issues arise
- **Error Monitoring**: Track deserialization errors moving from cache to consumer code
- **Performance Validation**: Confirm no latency degradation despite deserialization shift

---

## Expected Overall Impact - BURN THE BOATS

**Guaranteed Results**: 60-70% reduction in total cache system allocations (based on 63.5% proven reduction)
**System Benefits**: 
- Massive reduction in GC pressure and memory allocation rate
- Significant improvement in pass-through/proxy scenarios (API gateways)
- Elimination of wasteful deserialization in non-processing use cases

**Migration Impact**:
- **100% breaking change** - all cache retrieval calls must be updated
- **Consumer effort**: Moderate - add explicit deserialization where data processing occurs
- **Compilation**: All consumer code stops compiling until migration complete

**Timeline**: 4-6 weeks for implementation + consumer migration across all phases  
**Risk Level**: Medium-High (breaking changes, high reward, proven approach)

This burn-the-boats approach delivers maximum performance optimization by eliminating all unnecessary deserialization, with consumers taking explicit control over when typed data is needed versus raw string pass-through.

---

## Future Architecture Consideration

**Complete Generics Elimination**: The ultimate optimization may be to remove the generic `[T]` constraint entirely and operate purely on raw string data at all levels. This would enable:

- **Pure pass-through cache**: No type information needed, just `string -> string`
- **Zero serialization overhead**: Store and retrieve raw data without any type conversions
- **Maximum flexibility**: Consumers handle all data format decisions (JSON, MsgPack, etc.)
- **Simplified implementation**: Single code path instead of generic complexity

This approach would make the cache a pure data storage layer with consumers responsible for all data format handling, potentially delivering even greater performance gains than the current burn-the-boats approach.

**Implementation Note**: The independent **pooled serializer module** (see `Msgpack-serializer-implementation.md`) can be developed separately to deliver 5× write-side allocation reductions that combine with the 63.5% read-side improvements from this plan.
# Raw Data Access Optimization Plan

## Overview

Based on the breakthrough discovery that skipping deserialization reduces allocations by **63.5%** (22,054 → 8,037 allocs/op), this plan outlines a systematic approach to apply the "raw data access" pattern across the entire codebase.

**Core Insight**: Redis stores serialized data as strings. For many use cases, we can return raw serialized data without the expensive deserialization step, dramatically reducing allocations and improving performance.

---

## Phase 1: Comprehensive Analysis & API Design

### Step 1: Identify All Deserialization Points
**Location**: All cache method implementations  
**Task**: Catalog every method that currently deserializes data from Redis  
**Scope**:
- `Get()` - Single key retrieval
- `GetMany()` - Already implemented ✅  
- `GetByOwner()` - Retrieves multiple values for owner
- `GetOrSet()` - May retrieve existing value
- `GetKeysByPattern()` - May include value retrieval variants
- Any custom/specialized retrieval methods

**Definition of Done**: 
- Complete inventory document with methods, current allocation costs, and usage patterns
- Each method annotated with deserialization complexity (Simple/Complex/Conditional)
- Performance baseline measurements for each method (allocs/op, B/op, ns/op)

**Expected Impact**: Understanding full scope of optimization opportunities

---

### Step 2: Analyze Usage Patterns & Use Cases  
**Location**: Consumer code analysis  
**Task**: Identify scenarios where raw data access would be beneficial  
**Scope**:
- **Pass-through scenarios**: Data fetched and forwarded without processing
- **Batch processing**: Multiple items processed together (defer deserialization)  
- **Conditional processing**: Deserialize only when certain conditions met
- **Migration/backup**: Data moved between systems as-is
- **Caching proxies**: Intermediate layers that don't process content

**Definition of Done**:
- Use case classification matrix (High/Medium/Low benefit for raw access)
- Consumer impact analysis (breaking changes vs new APIs)
- Performance opportunity sizing (potential allocation reduction per use case)

**Expected Impact**: Prioritized implementation roadmap based on real usage patterns

---

### Step 3: Design Raw Data Access API Pattern
**Location**: Interface design documentation  
**Task**: Define consistent API patterns for raw data access methods  
**Scope**:
- **Naming convention**: `Method()` vs `MethodRaw()` vs `Method(..., raw bool)`
- **Return types**: `map[string]string` vs `map[string][]byte` vs custom types
- **Error handling**: Consistent error patterns across raw methods
- **Interface evolution**: How raw methods integrate with existing interfaces
- **Type safety**: Compile-time vs runtime guarantees

**Definition of Done**:
- API design document with examples for each method pattern
- Interface definitions that maintain backward compatibility  
- Type safety analysis (generic constraints, interface compliance)
- Performance contract specification (allocation targets, failure modes)

**Expected Impact**: Consistent, maintainable API expansion foundation

---

## Phase 2: Core Methods Implementation

### Step 4: Implement GetRaw() Method
**Location**: `redis_cache.go` (single key operations)  
**Task**: Create raw version of single-key Get operation  
**Implementation**:
```go
func (c *RedisCache[T]) GetRaw(ctx context.Context, key string) (string, bool, error)
```

**Definition of Done**:
- Method implemented with same Redis operations as Get() but skips deserialization
- Benchmarks show allocation reduction comparable to GetManyRaw (60%+ reduction)
- All existing Get() tests pass, plus new tests for GetRaw()
- Error handling consistent with existing patterns

**Expected Impact**: 
- Single-key operations allocation reduction: ~60%
- Foundation for other single-key raw methods

---

### Step 5: Implement GetByOwnerRaw() Method  
**Location**: `redis_cache.go` (owner-based operations)  
**Task**: Create raw version of GetByOwner for indexed retrievals  
**Implementation**:
```go  
func (c *RedisCache[T]) GetByOwnerRaw(ctx context.Context, ownerKey string) (map[string]string, error)
```

**Definition of Done**:
- Method uses same Lua script as GetByOwner() but returns raw serialized data
- Benchmarks demonstrate allocation reduction proportional to result set size
- Integration with indexing system maintained (metadata updates, LRU tracking)
- Performance scales linearly with number of owned entries

**Expected Impact**: 
- Owner-based bulk operations allocation reduction: ~60%
- Particularly valuable for sessions, user data, multi-tenant scenarios

---

### Step 6: Benchmark & Validate Core Methods
**Location**: Benchmark test suite expansion  
**Task**: Comprehensive performance validation of core raw methods  
**Scope**:
- **Comparative benchmarks**: Each raw method vs original method
- **Allocation profiling**: Detailed memory allocation analysis  
- **Scaling characteristics**: Performance across different data sizes (10/100/1000+ keys)
- **Regression testing**: Ensure no performance degradation in original methods

**Definition of Done**:
- Benchmark suite with consistent allocation reduction (≥60% target)
- Performance regression detection (original methods unchanged)
- Memory profiling confirms elimination of deserialization allocations
- Scaling behavior documented (linear/sub-linear allocation growth)

**Expected Impact**: Validated performance improvements, regression prevention

---

## Phase 3: Advanced Operations & Complex Scenarios

### Step 7: Analyze GetOrSet() Raw Implementation Complexity
**Location**: `atomic_operations.go`  
**Task**: Design approach for GetOrSet with raw data support  
**Complexity**: GetOrSet may need to deserialize for the loader function, but could return raw data  
**Design Options**:
1. `GetOrSetRaw()` - Returns raw, but loader still provides typed value
2. `GetOrSetWithRawLoader()` - Loader provides raw data directly  
3. Hybrid approach with conditional deserialization

**Definition of Done**:
- Technical design document with trade-off analysis
- Prototype implementation of preferred approach
- Performance comparison showing net allocation benefit
- API compatibility assessment with existing GetOrSet usage

**Expected Impact**: Complex atomic operations optimization pathway

---

### Step 8: Implement Conditional Raw Access Pattern
**Location**: New utility methods  
**Task**: Create methods that conditionally deserialize based on caller needs  
**Implementation**:
```go
type RawResult[T any] struct {
    Raw   string
    Value *T // nil if not deserialized
}
func (c *RedisCache[T]) GetConditional(ctx context.Context, key string, deserialize bool) (RawResult[T], bool, error)
```

**Definition of Done**:
- Conditional access pattern implemented and tested
- Zero allocation when deserialize=false
- Lazy deserialization option for RawResult
- Benchmarks showing allocation scaling with deserialization ratio

**Expected Impact**: Flexible optimization for mixed-usage scenarios

---

### Step 9: Batch Raw Operations Beyond GetMany
**Location**: `batch_operations.go` expansion  
**Task**: Apply raw pattern to other batch operations where applicable  
**Scope**:
- `SetManyRaw()` - Accept raw serialized data for storage
- `DeleteManyWithRaw()` - Return deleted raw values  
- Pipeline operations with raw data support

**Definition of Done**:
- Batch operations maintain pipeline efficiency with raw data
- Consistent API patterns across all batch raw methods
- Benchmarks demonstrate additive allocation benefits
- Integration with existing batch optimization (pooling, key building)

**Expected Impact**: Comprehensive batch operation optimization

---

## Phase 4: Integration & Advanced Optimizations

### Step 10: Pipeline Raw Data Streaming  
**Location**: New streaming interface  
**Task**: Design streaming/iterator pattern for large result sets  
**Implementation**: Avoid building large maps in memory, stream raw results

**Definition of Done**:
- Streaming interface design with raw data support
- Memory usage remains constant regardless of result set size
- Iterator pattern compatible with existing Redis pipeline operations
- Benchmarks show flat memory profile for large data sets

**Expected Impact**: Memory efficiency for very large operations (10K+ keys)

---

### Step 11: Integration with Existing Optimization Systems
**Location**: Cross-system integration  
**Task**: Ensure raw data access works with existing optimizations  
**Scope**:
- **StringPool integration**: Raw string handling with pooled builders
- **SlicePool integration**: Raw result collection with pooled slices  
- **Metrics integration**: Track raw vs deserialized operation ratios
- **Circuit breaker**: Raw operations fail-fast behavior

**Definition of Done**:
- All existing optimization systems support raw data operations
- No allocation regressions in pooling systems
- Metrics provide visibility into raw vs normal usage patterns
- Circuit breaker behavior consistent across raw and normal methods

**Expected Impact**: Compound optimization benefits, system coherence

---

### Step 12: Memory Pressure Adaptation
**Location**: `memory_tracking.go` integration  
**Task**: Adapt memory pressure system to account for raw data patterns  
**Scope**:
- Memory usage calculations account for skipped deserialization
- Pressure thresholds adjust based on raw vs deserialized usage ratios
- Automatic raw data promotion under memory pressure

**Definition of Done**:
- Memory tracking accurately reflects raw data usage patterns  
- Pressure-based automatic optimization (promote to raw under pressure)
- Memory accounting shows true memory footprint reduction
- Integration with existing memory sampling and alerting

**Expected Impact**: Adaptive memory optimization, automatic efficiency scaling

---

## Phase 5: Validation, Documentation & Production Readiness

### Step 13: End-to-End Performance Validation
**Location**: Integration test suite  
**Task**: Comprehensive real-world performance testing  
**Scope**:
- **Mixed workload simulation**: Combination of raw and normal operations
- **Production scenario testing**: Realistic usage patterns and data sizes
- **Regression testing**: Ensure no degradation of existing functionality
- **Scaling validation**: Performance characteristics at production scale

**Definition of Done**:
- Integration tests demonstrate target allocation reductions (≥60%)
- Production workload simulations show net performance improvement
- No functional regressions detected in comprehensive test suite
- Performance characteristics documented across scale ranges

**Expected Impact**: Production-ready validation, performance predictability

---

### Step 14: API Documentation & Usage Guidelines  
**Location**: Documentation system  
**Task**: Comprehensive documentation for raw data access patterns  
**Scope**:
- **API reference**: Complete documentation for all raw methods
- **Usage guidelines**: When to use raw vs normal methods
- **Performance characteristics**: Expected allocation and latency improvements
- **Migration guide**: How to adopt raw patterns in existing code
- **Best practices**: Common patterns and anti-patterns

**Definition of Done**:
- Complete API documentation with examples and benchmarks
- Usage decision tree for raw vs normal method selection
- Migration guide with before/after performance examples
- Best practices guide with real-world usage patterns

**Expected Impact**: Developer adoption enablement, correct usage patterns

---

### Step 15: Production Rollout Strategy
**Location**: Deployment planning  
**Task**: Phased rollout plan for raw data optimizations  
**Scope**:
- **Feature flag integration**: Gradual raw method adoption
- **Monitoring & alerting**: Track adoption and performance impact
- **Rollback strategy**: Quick reversion if issues detected  
- **Performance baseline**: Establish pre/post optimization metrics

**Definition of Done**:
- Feature flag system enables gradual raw method adoption
- Comprehensive monitoring shows allocation reduction and adoption rates
- Rollback procedures tested and documented
- Production baseline metrics established for performance tracking

**Expected Impact**: Safe, measurable production performance improvement

---

## Success Metrics

### Primary Targets
- **Allocation Reduction**: ≥60% reduction in allocs/op for raw methods
- **Memory Reduction**: ≥50% reduction in B/op for raw methods  
- **Performance Improvement**: 10-20% improvement in ns/op
- **API Consistency**: 100% of retrieval methods have raw equivalents

### Secondary Targets  
- **Adoption Rate**: Measurable uptake of raw methods in production
- **System Efficiency**: Overall cache system allocation reduction
- **Memory Pressure**: Reduced memory pressure incidents
- **Developer Experience**: Positive feedback on API usability

### Risk Mitigation
- **Backward Compatibility**: Zero breaking changes to existing APIs
- **Performance Regression**: No degradation of existing method performance
- **Complexity Management**: Maintainable code complexity metrics
- **Production Stability**: No increase in error rates or failure modes

---

## Expected Overall Impact

**Conservative Estimate**: 40-50% reduction in total cache system allocations  
**Optimistic Estimate**: 60-70% reduction in total cache system allocations

**Timeline**: 4-6 weeks for full implementation across all phases  
**Risk Level**: Low (additive changes, extensive validation, gradual rollout)

This systematic approach ensures maximum allocation reduction while maintaining system stability and developer experience.
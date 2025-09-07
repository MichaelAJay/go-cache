# WRONGTYPE Error Fix Plan

**Issue Discovery**: During Task 4.1 performance validation, multiple benchmarks failed with:
```
redis set error: WRONGTYPE Operation against a key holding the wrong kind of value script: 959fe40041927ffa7d05f1e055e5e2e2e882854008, on @user_script:26
```

**How I Found The Issue**:
1. Ran allocation benchmarks: `go test -bench='BenchmarkRedisCache.*Allocations' -benchmem -run='^$' -tags=integration`
2. Multiple benchmarks failed with WRONGTYPE errors in Lua scripts
3. Affected operations: Set, GetOrSet, Delete (with existing data), and other write operations
4. Only cache miss operations (Has_Miss, Get_Miss, Delete_Missing) work correctly

---

## Root Cause Analysis Plan

### Phase 1: Error Investigation
**Objective**: Understand what's causing the WRONGTYPE errors

1. **Lua Script Analysis**
   - Find all Lua scripts in the codebase (`*.lua` files or embedded scripts)
   - Identify script hash `959fe40041927ffa7d05f1e055e5e2e882854008` 
   - Locate line 26 in the failing script (`@user_script:26`)
   - Review script logic for key type operations

2. **Redis Key Inspection**
   - Check what key types are being created vs expected
   - Identify if keys contain strings, hashes, sets, or other Redis types
   - Examine if previous test runs left conflicting key types in Redis

3. **Test Environment Analysis**
   - Review benchmark setup in `setupBenchmarkCache()`
   - Check if multiple test runs create key conflicts
   - Examine Redis container initialization and cleanup

### Phase 2: Key Type Conflict Detection
**Objective**: Identify specific keys causing type conflicts

1. **Benchmark Key Pattern Analysis**
   - Review key naming in failing benchmarks: `allocation:set:*`, `allocation:getorset:*`
   - Check for key overlap between different test operations
   - Identify if metadata keys conflict with data keys

2. **Redis State Investigation**
   - Check Redis state between benchmark runs
   - Identify if keys persist from previous operations with wrong types
   - Review key expiration and cleanup patterns

### Phase 3: Fix Implementation (UPDATED PRIORITY)
**Objective**: Fix the systematic Lua script errors causing WRONGTYPE failures

1. **CRITICAL FIXES** (Must fix core functionality)
   - **Find and fix Set Lua script error at line 26** (script hash: 959fe40041927ffa7d05f1e055e5e2e882854008)
   - **Find and fix Get Lua script error at line 16** (script hash: 69f17cb2380f4145134d6508bcd8f34dbe08221b)
   - **Test basic operations work** in fresh Redis containers

2. **SECONDARY FIXES** (After core functionality restored)
   - Add Redis FLUSHDB before each benchmark run to ensure clean state
   - Implement unique key prefixes per benchmark to prevent conflicts
   - Add key type validation in Lua scripts with better error handling

3. **Prevention Measures** (Long-term)
   - Add unit tests that validate key types before operations
   - Implement Redis key type debugging utilities
   - Add key conflict detection to benchmark setup

**PRIORITY CHANGE**: This is no longer a benchmark isolation issue - it's a **core functionality bug** affecting basic cache operations.

---

## Specific Investigation Tasks

### Task A: Find The Failing Lua Script
```bash
# Find all Lua scripts in codebase
find . -name "*.lua" -type f
grep -r "EVAL\|EVALSHA" . --include="*.go"
grep -r "user_script" . --include="*.go"
```

### Task B: Identify Key Type Conflicts
```bash
# Run single benchmark and inspect Redis state
GOCACHE_TEST_MODE=containers go test -bench='BenchmarkRedisCache_Set_Allocations' -run='^$' -tags=integration -count=1
# Then inspect Redis keys:
docker exec -it <redis_container> redis-cli
> KEYS allocation:*
> TYPE <each_key>
```

### Task C: Script Hash Resolution
```bash
# Find script with hash 959fe40041927ffa7d05f1e055e5e2e882854008
grep -r "959fe40041927ffa7d05f1e055e5e2e882854008" .
# Or search for scripts that might generate this hash
```

---

## Integration Test Findings (September 7, 2025)

**CRITICAL DISCOVERY**: WRONGTYPE errors are **NOT limited to benchmarks** - they occur in regular integration tests too.

### Failed Integration Test
- **Test**: `TestCircuitBreakerFailureRecovery` 
- **Same Error Scripts**: 
  - Set operations: `script: 959fe40041927ffa7d05f1e055e5e2e882854008, on @user_script:26`
  - Get operations: `script: 69f17cb2380f4145134d6508bcd8f34dbe08221b, on @user_script:16`
- **Impact**: Basic Set/Get operations fail in fresh Redis containers

### Error Pattern Confirmed
```
redis set error: WRONGTYPE Operation against a key holding the wrong kind of value script: 959fe40041927ffa7d05f1e055e5e2e882854008, on @user_script:26.
redis get error: WRONGTYPE Operation against a key holding the wrong kind of value script: 69f17cb2380f4145134d6508bcd8f34dbe08221b, on @user_script:16.
```

**This indicates a SYSTEMATIC ISSUE with the Lua scripts themselves, not just test pollution.**

## Root Cause Analysis (Updated)

### PRIMARY ROOT CAUSE: Lua Script Logic Error
1. **Set Script (959fe40...)**: Line 26 has key type conflict in set operation logic
2. **Get Script (69f17cb...)**: Line 16 has key type conflict in get operation logic  
3. **Fresh Container Failure**: Errors occur even in clean Redis instances
4. **Systematic Failure**: Core cache operations are fundamentally broken

### SECONDARY CAUSES (Original Assessment - Less Likely)
1. **Key Type Pollution**: Previous benchmark runs left keys with wrong Redis types
2. **Metadata Key Conflicts**: Cache metadata keys conflict with data keys
3. **Test Isolation Issues**: Benchmarks don't properly isolate Redis state

---

## Success Criteria

1. ✅ All allocation benchmarks pass without WRONGTYPE errors
2. ✅ Write operations (Set, GetOrSet, Delete) benchmark successfully  
3. ✅ Benchmarks can run repeatedly without Redis state conflicts
4. ✅ Complete Task 4.1 validation with full benchmark suite
5. ✅ Root cause documented and prevention measures implemented

---

## Implementation Priority

1. **HIGH**: Find and fix the immediate WRONGTYPE error source
2. **HIGH**: Implement Redis state cleanup between benchmarks
3. **MEDIUM**: Add key type validation to Lua scripts
4. **LOW**: Implement comprehensive Redis debugging utilities

This plan will restore benchmark functionality and complete the performance validation that was interrupted by these Redis key type conflicts.
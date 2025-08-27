# Cache Module Concurrency & Safety Plan

## Overview

- **Goal**: Ensure the cache module is **thread-safe**, **provider-agnostic**, and eliminates the need for consumer-side locking.
- **Core principle**: **All thread-safety is the responsibility of the cache layer**.
- **Providers**: In-memory & Redis (must meet identical concurrency semantics).

---

## Phase 1: Interface & Concurrency Semantics

### Tasks

- **Define a clear, goroutine-safe interface:**

  ```go
  type Cache[V any] interface {
      Get(ctx context.Context, key string) (V, bool, error)
      Set(ctx context.Context, key string, val V, ttl time.Duration) error
      Delete(ctx context.Context, key string) error

      GetOrSet(ctx context.Context, key string, ttl time.Duration, loader func(ctx context.Context) (V, error)) (V, error)
      Update(ctx context.Context, key string, ttl time.Duration, fn func(old V, ok bool) (V, error)) (V, error)
  }
  ```

- **Document guarantees:**
  - Safe for concurrent use
  - Per-key atomicity for Set, Update, GetOrSet
  - No shared mutable state crosses the boundary (deep copies)

- **Introduce Codec[V] for serialization/copying**

### Definition of Done

- [ ] Interface defined and documented
- [ ] Guarantees clearly specified in docstrings
- [ ] Codec[V] abstraction available for implementations
- [ ] Unit tests compile against interface stubs

---

## Phase 2: In-Memory Provider

### Tasks

**Implement with:**

- **Sharded map** (64/128 shards)
- **Per-shard sync.RWMutex**
- **Per-key locks** (striped or map of mutexes)
- **singleflight.Group** for GetOrSet
- **Store values as encoded bytes** via Codec
  - On Get: decode into a fresh instance
  - On Set: encode (deep copy semantics)
- **Add TTL handling** with jitter
- **Implement Update** with per-key lock

### Definition of Done

**In-memory provider passes:**

- [ ] Race detector (`go test -race`)
- [ ] Parallel GetOrSet tests (loader runs once)
- [ ] Parallel Update tests (no lost updates)
- [ ] Values returned are always fresh (no shared mutable state)
- [ ] Expiry works with jitter
- [ ] Benchmarks run without global lock contention

---

## Phase 3: Redis Provider

### Tasks

- **Use goroutine-safe Redis client** (e.g., go-redis)

**Implement:**

- **GetOrSet:**
  - Process-level singleflight
  - Cross-process atomicity via Redis lock key (`SET lock:<k> NX PX` + loader)

- **Update:**
  - `WATCH` + `MULTI`/`EXEC`, or Lua script for atomic update

- **Increment/Decrement:** atomic Redis commands
- **Store values via Codec** (same format as in-memory)
- **Add TTL jitter**
- **Optional:** stale-while-revalidate with lock key

### Definition of Done

- [ ] Redis provider passes integration tests with real Redis
- [ ] Parallel GetOrSet across multiple processes executes loader once
- [ ] Parallel Update preserves invariants with no race conditions
- [ ] TTL jitter observed in expiry behavior
- [ ] Fallback paths (lock timeout, retry) tested

---

## Phase 4: Unified Testing & Validation

### Tasks

**Write concurrency stress tests:**

- Hundreds of goroutines calling GetOrSet on same key
- Parallel Update loops

**Run fuzz tests** for key/value pairs (codec validation)

**Test cross-provider consistency:**

- In-memory & Redis behave identically under same tests

**Add benchmarks** (Get, Set, GetOrSet under load)

### Definition of Done

- [ ] All tests pass under `-race`
- [ ] Cross-provider consistency tests pass
- [ ] Benchmarks show no pathological contention
- [ ] Coverage >90% for cache logic

---

## Phase 5: Consumer Migration (Session Manager)

### Tasks

**Replace consumer-side mutexes with:**

- GetOrSet for session creation
- Update for session mutation
- Remove app-level locking assumptions

**Add integration tests** for session-manager using both providers

### Definition of Done

- [ ] Session-manager no longer uses its own locks for cache safety
- [ ] Session-manager tests pass with both Redis and in-memory cache
- [ ] Verified no regressions in session correctness under concurrency load

---

## Phase 6: Rollout & Observability

### Tasks

**Add metrics:**

- Loader executions (per key)
- Lock contention count
- TTL expirations & jitter distribution

**Deploy behind feature flag**

**Monitor for regressions**

### Definition of Done

- [ ] Metrics exported and observed
- [ ] Feature flag rollout successful
- [ ] No reported race conditions or regressions in production load tests

# MsgPack Serializer — Implementation Plan (Aggressive optimization: **Burn the boats**)

**Purpose**

This document defines an actionable, step-by-step plan for implementing a heavily optimized MessagePack serializer/deserializer for Go. The goal is to aggressively reduce allocations and copies in high-throughput paths (e.g., `SetMany`/`GetMany` to Redis pipelines), even if that requires breaking changes to the serializer API. We will prioritize raw performance and minimal heap churn over backwards compatibility.

Each task is written so an AI agent or engineer can implement it directly. Every step includes a **Definition of Done (DoD)** — objective, easily-verifiable completion criteria.

> **Philosophy:** Burn the boats. We are allowed to make breaking changes and to design for maximum performance. The code should be safe and correct, but _aggressively optimized_.

---

## High-level goals

1. Reduce per-value allocations from current \~20–40 allocations to single-digit allocations where possible.
2. Minimize copies between encoder output and Redis call argument buffers.
3. Provide both _safe_ (zero-risk) and _unsafe/high-performance_ APIs so callers can choose tradeoffs.
4. Maintain thread-safety and avoid data races.
5. Provide comprehensive tests and benchmarks that prove allocation and latency improvements.

---

## Terminology & primitives

- **PooledEncoder**: an encoder object (MessagePack encoder + a backing `bytes.Buffer`) retrieved from a `sync.Pool`. Reused across calls.
- **PooledDecoder**: decoder object (MessagePack decoder + backing `bytes.Reader` or `bytes.Buffer`) from a `sync.Pool`.
- **PooledBuf / PooledBytes**: a lightweight ownership wrapper around a pooled `bytes.Buffer` that allows callers to _own_ the encoded bytes temporarily and return them to the pool with a `Release()` call.
- **SafeSerialize**: encode to a newly-allocated `[]byte` (no shared pooled buffers returned). Lower complexity for callers; still benefits from pooled encoder internals.
- **UnsafeSerialize / SerializePooled**: return a `*PooledBuf` (zero-copy) that gives direct access to the encoder's buffer; caller must `Release()` after use.

---

## Step 0 — Preconditions and repository changes (small)

**Tasks**

- Add a new package (or folder) `serializer/msgpack` (or update existing) to hold the optimized implementation.
- Add tests, benchmark files, and a micro-benchmark harness (e.g. `bench/redis_setmany_bench_test.go`).
- Add `go.mod` if necessary and ensure the chosen msgpack library version (e.g. `github.com/vmihailenco/msgpack/v5`) is pinned.

**DoD**

- A new `serializer/msgpack` folder exists in repo with `go.mod` (if needed), empty test file, and benchmark harness placeholder.

---

## Step 1 — Implement pooled encoder infrastructure ✅ COMPLETED

**Objective**
Create an encoder pool that reuses MessagePack encoders and underlying buffers to avoid per-call allocations for encoder state and token tables.

**Implementation details**

- Create a struct `pooledEncoder` that contains a `*msgpack.Encoder` and a `*bytes.Buffer`.
- Implement a `sync.Pool` whose `New` function constructs `pooledEncoder{enc: msgpack.NewEncoder(&bytes.Buffer{}), buf: &bytes.Buffer{}}` (adjust to the actual msgpack library constructor). Ensure the encoder can be `Reset` to use the `buf` as its writer.
- Provide helper functions:

  - `getPooledEncoder() *pooledEncoder`
  - `putPooledEncoder(pe *pooledEncoder)` — this should inspect buffer capacity and if it is `> MAX_BUF_CAP` or `pe.buf.Cap()` is enormous, create a fresh buffer instead of returning the oversized buffer into the pool.

- Choose a conservative `MAX_BUF_CAP` (e.g. `1<<20` = 1MB) to avoid unbounded buffer growth.

**API notes**

- The pool must be safe for concurrent access.
- Always `buf.Reset()` before reuse; always `enc.Reset(buf)` (or equivalent) to bind the encoder to the buffer.

**DoD** ✅

- ✅ `pooledEncoder` and `sync.Pool` are implemented and unit tested.
- ✅ Unit test acquires an encoder from the pool, encodes a small structure, `put` it back, gets another encoder, encodes again, and verifies output correctness.
- ✅ A small stress test shows no races (`go test -race`) and buffers are capped.

**Implementation Notes**
- Added proper buffer capacity management that discards entire encoders (not just buffers) when they exceed MAX_BUF_CAP
- Comprehensive test suite includes concurrent stress testing with 100 goroutines × 1000 operations
- All tests pass under race detector

---

## Step 2 — Implement pooled decoder infrastructure

**Objective**
Reuse decoder state to minimize per-deserialize allocations and improve decode throughput.

**Implementation details**

- Create `pooledDecoder` with `*msgpack.Decoder` and an `io.Reader` backed by `bytes.Reader` / `bytes.Buffer`.
- Use a `sync.Pool` for decoders similar to encoders.
- Provide `getPooledDecoder(data []byte) *pooledDecoder` which attaches the decoder to a `bytes.Reader` wrapping `data` (or a recycled buffer that copies data — see safety notes below).

**Safety note:** If the decoder implementation mutates the underlying byte slice or expects persistent memory, ensure the data passed in is not from a shared pooled buffer that will be released early. Prefer to copy into a `bytes.Reader` or use the decoder’s `Reset` API which takes a `[]byte` reference.

**DoD**

- `pooledDecoder` and `sync.Pool` implemented and tested.
- Round-trip unit tests (encode then decode) pass under race detector.
- Microbenchmark for `Deserialize` shows reduced allocations vs `msgpack.Unmarshal` baseline.

---

## Step 3 — Implement `SerializeSafe` using pooled encoders

**Objective**
Provide a drop-in `Serialize(v any) ([]byte, error)` replacement that uses pooled encoder internals to reduce allocations but returns an owned `[]byte` (safe, no life-cycle constraints for callers).

**Implementation details**

- Acquire a `pooledEncoder` from the pool.
- `buf.Reset()` and attach encoder to buffer via `enc.Reset(buf)`.
- `enc.Encode(v)`.
- Allocate `out := make([]byte, buf.Len())` and `copy(out, buf.Bytes())` to return an owned slice.
- Put encoder back in pool, but first check buffer capacity and possibly discard the buffer if too large.

**Rationale**

- This avoids the encoder/bytes.Buffer allocation per call but still returns a fresh `[]byte` so callers have no ownership concerns.

**DoD**

- `SerializeSafe` implemented and used in existing call sites with minimal code changes.
- Benchmarks show encoder-internal allocation reduction (report the `B/op` and `allocs/op` improvements).
- Unit tests demonstrate correct encoding and zero data races.

---

## Step 4 — Implement `SerializePooled` (zero-copy, high-performance) and `PooledBuf`

**Objective**
Expose a zero-copy path that returns a pooled buffer whose bytes can be used by callers _without copying_. The caller is responsible for calling `Release()` when done. This is the aggressive path that lets `SetMany` avoid allocating per-value output slices.

**Implementation details**

- Introduce a type:

```go
// PooledBuf owns a pointer to an encoder's buffer. Caller must call Release()
// after the buffer is no longer needed.
type PooledBuf struct {
    buf *bytes.Buffer
    enc *msgpack.Encoder // optional, if you need to hold the whole pooledEncoder
    pool *sync.Pool      // reference to the encoder pool for release
}

func (p *PooledBuf) Bytes() []byte { return p.buf.Bytes() }
func (p *PooledBuf) Len() int { return p.buf.Len() }
func (p *PooledBuf) Release() { /* puts the underlying pooledEncoder back into pool */ }
```

- Implement `SerializePooled(v any) (*PooledBuf, error)` which:

  1. `pe := getPooledEncoder()`
  2. `pe.buf.Reset()` and `pe.enc.Reset(pe.buf)`
  3. `pe.enc.Encode(v)`
  4. Return `&PooledBuf{buf: pe.buf, enc: pe.enc, pool: &encoderPool}` but **do NOT** put the encoder back into the pool — ownership transferred to caller.

- Implement `(*PooledBuf).Release()` to return the `pooledEncoder` to the pool, performing buffer capacity trimming.

**Important usage contract (must be documented and enforced)**

- The caller MUST NOT call `Release()` until all uses of the returned `[]byte` are complete.
- If the caller intends to call `Release()` immediately (e.g., they pass the bytes into a library that makes its own copy), they may; otherwise they must `Release()` after the Redis `Exec` completes.

**Safety helpers**

- Provide convenience helpers to reduce misuse:

  - `CopyAndRelease(pb *PooledBuf) []byte` — copies the bytes to a fresh `[]byte`, releases the pooled buffer, returns copy.
  - `UseWithPipeline(ctx, pipe redis.Pipeliner, key string, pb *PooledBuf, ttl time.Duration)` — helper which enqueues the `Set` and retains pointer to `pb` until `Exec` returns, then releases. This helper will be opinionated but safe (handles release after Exec).

**DoD**

- `PooledBuf` implemented and covered by unit tests for `Bytes()`/`Release()` semantics.
- A `SetMany` test demonstrates using `SerializePooled` to build pipeline commands without copying and releasing buffers after `pipe.Exec()`.
- Integration benchmark shows dramatic reduction in `allocs/op` for batch set compared to baseline.

---

## Step 5 — Implement `Deserialize` variants that use pooled decoders

**Objective**
Minimize allocations on decode operations by reusing decoder state and by allowing decode into preallocated structs (when callers provide them).

**Implementation details**

- Implement `Deserialize(data []byte, out any) error` that uses `pooledDecoder` for decode. Use decoder's `Reset` to point it at `data` without copying if supported by the library.
- Implement `DeserializeFromPooled(pb *PooledBuf, out any) error` that decodes directly from a pooled buffer without copying the bytes.
- Provide `UnsafeDeserialize` option if a decoder mutates the underlying buffer — document constraints.

**DoD**

- `Deserialize` and `DeserializeFromPooled` implemented and unit-tested with multiple struct shapes.
- Benchmarks show allocations reduced on decode operations.

---

## Step 6 — Integrate with `SetMany` and `GetMany` (example changes)

**Objective**
Update `SetMany` and `GetMany` to use the new pooled serializer APIs and demonstrate safe patterns and extreme-performance patterns.

**Implementation details**

- **Safe path**: call `SerializeSafe` for each value; continue to pass `[]byte` to `pipe.SetEX()`; this reduces internal allocations while keeping ownership simple.
- **Aggressive path**: call `SerializePooled` for each value, pass `pb.Bytes()` into `pipe.SetEX()` and keep track of `[]*PooledBuf` returned. After `pipe.Exec()`, iterate the pooled buffers and call `Release()` on each.
- Provide `SetManyPooled(ctx, values []T, ttl time.Duration)` function as a high-performance variant.
- For `GetMany`, prefer `Get(...).Bytes()` and `DeserializeFromPooled` where possible. If go-redis copies the bytes internally, detect that and fallback to `Deserialize`.

**DoD**

- `SetManyPooled` added and unit-tested.
- Integration benchmark comparing `SetMany` baseline vs `SetManySafe` vs `SetManyPooled` shows expected allocation reductions.
- No data races with `-race`.

---

## Step 7 — Tests, Benchmarks, and CI

**Tests**

- Unit tests for each API surface, including correctness (round-trip tests) and lifecycle tests for `PooledBuf.Release()`.
- Concurrency tests: encode/decode concurrently with a high goroutine count to smoke out races.

**Benchmarks**

- Microbenchmarks for individual `SerializeSafe`, `SerializePooled`, `Deserialize`, `DeserializeFromPooled`.
- Integration benchmarks that run `SetMany`/`GetMany` using a local Redis or `redismock` to measure allocations and latency for small and large batch sizes (10, 100, 1000).
- Report baseline vs optimized numbers, including `ns/op`, `B/op`, and `allocs/op`.

**CI**

- Add a CI job (or GitHub Action) that runs the microbenchmarks and fails if `allocs/op` regress beyond a chosen threshold.

**DoD**

- Unit tests pass.
- Benchmarks show measurable alloc/latency improvement for the optimized path.
- CI includes at least one bench job and is green.

---

## Step 8 — Operational safety and memory considerations

**Limits and safety**

- Introduce a `MAX_BUF_CAP` constant (e.g. `1 << 20`) and do not return buffers with capacity > `MAX_BUF_CAP` back into the pool. This avoids buffer memory leak bloat across high variance items.
- Consider a second pool for large objects if you routinely need objects > `MAX_BUF_CAP`.
- Document that `PooledBuf` is not safe to hold across unknown third-party calls unless the caller guarantees it will not be reused concurrently.

**DoD**

- Buffer cap implemented and tested.
- Documentation added in README section of package describing safety constraints and ownership semantics.

---

## Step 9 — Migration & API hygiene (breaking-change checklist)

**Decisions**

- Because we are "burning the boats", choose one of the following API strategies and document it publicly in the change log:

  1. **Backwards-compatible minimal change**: Keep `Serialize(v) ([]byte, error)` but implement `SerializeSafe` under the hood; add new `SerializePooled`/`PooledBuf` methods as optional extension.
  2. **Breaking upgrade**: Replace `Serialize(v)` with the new pooled-first behavior (documented) and bump major version. This is the most aggressive option.

**Migration tasks**

- If making breaking changes, create a `v2` package path (e.g. `serializer/msgpack/v2`) and port callers.
- Provide a `compat` shim that forwards to `SerializeSafe` for old callers during a transitional period (optional).

**DoD**

- Public CHANGELOG entry describing the chosen strategy and migration steps.
- If breaking change chosen: a `v2` package exists and example call sites updated.

---

## Step 10 — Instrumentation & telemetry

**What to measure**

- Pool hit/miss counters.
- Number of pooled buffers currently in circulation.
- Average buffer size returned to pool (histogram).
- Per-call encode/decode latency distribution (histogram).

**DoD**

- Basic metrics emitted via existing metrics system (Prometheus, statsd, etc.) and visible on staging.

---

## Step 11 — Optional: zero-copy detection & go-redis compatibility check

**Objective**
Determine whether go-redis copies `[]byte` arguments at command enqueue time or defers copying until network write. Adjust design accordingly.

**Tasks**

- Add a micro-test that encodes with `SerializePooled`, uses the pooled buffer bytes in a `pipe.Set` call, and then attempts to `Release()` the buffer immediately; if the pipeline still obtains the bytes safely (no corruption) then go-redis copied the bytes; if not, we must keep buffers alive until `Exec()`.
- Based on test outcome, implement the safe helper `UseWithPipeline` described earlier.

**DoD**

- Test determines behavior and is documented.
- `SetManyPooled` behaves correctly given go-redis semantics (releases only after Exec if necessary).

---

## Step 12 — Documentation & example usage

**Deliverables**

- A clear README.md in `serializer/msgpack` explaining the API, usage patterns, and recommended call paths: `SerializeSafe`, `SerializePooled` + `Release`, and `SetManyPooled`.
- Examples for both safe and aggressive usage.

**DoD**

- README exists and includes code snippets for `SetManySafe` and `SetManyPooled`.

---

## Acceptance criteria (overall)

When all steps are complete, we expect:

- A working, well-tested `msgpack` serializer package with pooled encoders/decoders.
- The `SetManyPooled` integration shows a measurable reduction in `allocs/op` (target: **≥ 5× reduction** from the original baseline) and reduced `B/op`.
- No data races under stress tests.
- Clear documentation and a migration path for callers.

---

## Implementation hints & gotchas (for the implementer/AI)

- Always run `go test -race` during development — pooling bugs show up fast.
- Be careful with buffer ownership semantics — prefer safe-by-default but provide an explicit `PooledBuf` contract for advanced callers.
- Trim oversized buffers before returning them to the pool to avoid memory bloat.
- Benchmarks must include `-benchmem` to capture `B/op` and `allocs/op`.
- Consider API ergonomics: exposing a `CopyAndRelease(pb)` helper avoids frequent boilerplate for callers that want convenience.

---

## Go-Cache Integration Considerations

When porting this implementation to the go-cache module, consider these additional requirements and patterns:

### Cache-Specific API Extensions

**Cache Lifecycle Integration:**
```go
// CacheEntry wraps pooled serialization with TTL and metadata
type CacheEntry struct {
    Key    string
    Value  *PooledBuf
    TTL    time.Duration
    Expiry time.Time
}

// BatchCacheEntry for bulk operations
type BatchCacheEntry struct {
    entries []*CacheEntry
}

func (b *BatchCacheEntry) Release() {
    for _, entry := range b.entries {
        if entry.Value != nil {
            entry.Value.Release()
        }
    }
}
```

**Redis Pipeline Integration:**
```go
// RedisSetPooled uses pooled serialization with automatic lifecycle management
func (c *Cache) SetPooled(ctx context.Context, key string, value any, ttl time.Duration) error {
    pb, err := c.serializer.SerializePooled(value)
    if err != nil {
        return err
    }
    
    // Pass ownership to Redis operation
    return c.redis.SetWithPooledBuf(ctx, key, pb, ttl)
}

// SetManyPooled for batch operations
func (c *Cache) SetManyPooled(ctx context.Context, items map[string]any, ttl time.Duration) error {
    pipe := c.redis.Pipeline()
    pooledBufs := make([]*PooledBuf, 0, len(items))
    
    // Serialize all values first
    for key, value := range items {
        pb, err := c.serializer.SerializePooled(value)
        if err != nil {
            // Release any already-serialized buffers
            for _, buf := range pooledBufs {
                buf.Release()
            }
            return fmt.Errorf("serialize %s: %w", key, err)
        }
        
        pooledBufs = append(pooledBufs, pb)
        pipe.SetEX(ctx, key, pb.Bytes(), ttl)
    }
    
    // Execute pipeline
    _, err := pipe.Exec(ctx)
    
    // Release all pooled buffers after pipeline execution
    for _, pb := range pooledBufs {
        pb.Release()
    }
    
    return err
}
```

### Error Handling & Resource Cleanup

**Defer Pattern for Safety:**
```go
// SafeSerializeWithCleanup ensures pooled buffers are always released
func SafeSerializeWithCleanup(serializer PooledSerializer, value any, fn func([]byte) error) error {
    pb, err := serializer.SerializePooled(value)
    if err != nil {
        return err
    }
    defer pb.Release() // Always cleanup, even on panic
    
    return fn(pb.Bytes())
}
```

**Bulk Operation Error Recovery:**
```go
// BulkOperation manages multiple pooled buffers with partial failure recovery
type BulkOperation struct {
    serializer PooledSerializer
    buffers    []*PooledBuf
    completed  []bool
}

func (b *BulkOperation) AddItem(value any) error {
    pb, err := b.serializer.SerializePooled(value)
    if err != nil {
        return err
    }
    b.buffers = append(b.buffers, pb)
    b.completed = append(b.completed, false)
    return nil
}

func (b *BulkOperation) Execute(fn func(int, []byte) error) error {
    var firstErr error
    for i, pb := range b.buffers {
        if err := fn(i, pb.Bytes()); err != nil && firstErr == nil {
            firstErr = err
        } else {
            b.completed[i] = true
        }
    }
    return firstErr
}

func (b *BulkOperation) Cleanup() {
    for _, pb := range b.buffers {
        pb.Release()
    }
}
```

### Cache Configuration & Tuning

**Pool Configuration:**
```go
// CacheConfig allows tuning of pooled serialization
type CacheConfig struct {
    // Serialization pool settings
    MaxBufferCapacity   int           // MAX_BUF_CAP equivalent
    PoolInitialSize     int           // Pre-populate pool
    PoolMaxIdleTime     time.Duration // Cleanup idle encoders
    
    // Performance monitoring
    EnableMetrics       bool
    MetricsInterval     time.Duration
}

// PooledSerializerConfig for cache-specific tuning
type PooledSerializerConfig struct {
    MaxEncoderBufCap int
    MaxDecoderBufCap int
    PoolSize         int // Hint for initial pool size
}
```

### Performance Monitoring Integration

**Cache-Level Metrics:**
```go
// SerializationMetrics for cache performance monitoring
type SerializationMetrics struct {
    PoolHits        int64
    PoolMisses      int64
    BuffersInUse    int64
    AvgBufferSize   int64
    SerializeTime   time.Duration
    DeserializeTime time.Duration
    PooledOperations int64
    SafeOperations  int64
}

// Instrument serialization calls
func (c *Cache) instrumentedSerialize(value any) (*PooledBuf, error) {
    start := time.Now()
    pb, err := c.serializer.SerializePooled(value)
    
    // Update metrics
    atomic.AddInt64(&c.metrics.SerializeTime, int64(time.Since(start)))
    if err == nil {
        atomic.AddInt64(&c.metrics.PooledOperations, 1)
        atomic.AddInt64(&c.metrics.BuffersInUse, 1)
    }
    
    return pb, err
}
```

### Testing Helpers

**Cache Integration Testing:**
```go
// TestCachePool provides test utilities for pooled operations
type TestCachePool struct {
    cache       *Cache
    activeBuffers []*PooledBuf
}

func (t *TestCachePool) SerializeForTest(value any) *PooledBuf {
    pb, err := t.cache.serializer.SerializePooled(value)
    if err != nil {
        panic(err)
    }
    t.activeBuffers = append(t.activeBuffers, pb)
    return pb
}

func (t *TestCachePool) Cleanup() {
    for _, pb := range t.activeBuffers {
        pb.Release()
    }
    t.activeBuffers = nil
}

// Test helper for verifying no buffer leaks
func AssertNoBufferLeaks(t *testing.T, cache *Cache) {
    // Implementation would check pool metrics or internal counters
    if cache.serializer.ActiveBufferCount() > 0 {
        t.Errorf("Buffer leak detected: %d active buffers", cache.serializer.ActiveBufferCount())
    }
}
```

---

## Example sketches

> The code below are sketches to convey intent. Implementations must be adapted to the specific MessagePack library API in use.

```go
var encoderPool = sync.Pool{ New: func() any { return &pooledEncoder{enc: msgpack.NewEncoder(&bytes.Buffer{}), buf: &bytes.Buffer{}} } }

func getPooledEncoder() *pooledEncoder { return encoderPool.Get().(*pooledEncoder) }
func putPooledEncoder(pe *pooledEncoder) {
    if pe.buf.Cap() > MAX_BUF_CAP { pe.buf = &bytes.Buffer{} }
    encoderPool.Put(pe)
}

// Safe serialize (copy result)
func SerializeSafe(v any) ([]byte, error) {
    pe := getPooledEncoder()
    pe.buf.Reset()
    pe.enc.Reset(pe.buf)
    if err := pe.enc.Encode(v); err != nil { putPooledEncoder(pe); return nil, err }
    out := make([]byte, pe.buf.Len())
    copy(out, pe.buf.Bytes())
    putPooledEncoder(pe)
    return out, nil
}

// Pooled (zero-copy)
func SerializePooled(v any) (*PooledBuf, error) {
    pe := getPooledEncoder()
    pe.buf.Reset()
    pe.enc.Reset(pe.buf)
    if err := pe.enc.Encode(v); err != nil { putPooledEncoder(pe); return nil, err }
    return &PooledBuf{pe: pe}, nil
}

func (p *PooledBuf) Release() {
    // trimming logic ...
    putPooledEncoder(p.pe)
}
```

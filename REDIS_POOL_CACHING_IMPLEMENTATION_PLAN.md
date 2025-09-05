Redis Cache Pooling Implementation Plan

This document describes a phased plan for introducing pluggable object pooling into the Redis cache module.
The goal: enable efficient memory reuse inside the module, while allowing advanced consumers to inject their own pools if desired.

Phase 1: Define Pooling Interface

Goal: Introduce a minimal, extensible interface to abstract pooling.

Tasks

Create a BufferPool interface with methods to acquire and release argument buffers.

Document expected semantics (GetArgsBuf may honor a size hint, PutArgsBuf must accept slices to be reused).

Code Sketch
type BufferPool interface {
GetArgsBuf(sizeHint int) []interface{}
PutArgsBuf([]interface{})
}

Definition of Done

BufferPool interface exists in the cache package.

Unit test compiles a dummy struct implementing the interface.

Phase 2: Implement Default Pool

Goal: Provide a sensible default pooling strategy based on sync.Pool.

Tasks

Implement a defaultArgPool type that satisfies BufferPool.

Ensure GetArgsBuf reuses buffers if capacity allows, otherwise allocates a new one.

Ensure PutArgsBuf trims slice length and returns it to the pool.

Code Sketch

type defaultArgPool struct {
pool sync.Pool
}

func newDefaultArgPool() \*defaultArgPool {
return &defaultArgPool{
pool: sync.Pool{
New: func() any { return make([]interface{}, 0, 128) },
},
}
}

Definition of Done

defaultArgPool is implemented and compiles.

Unit test verifies GetArgsBuf returns an empty slice with requested capacity.

Unit test verifies PutArgsBuf and subsequent GetArgsBuf reuse the buffer.

Phase 3: Integrate Pool into Cache Struct

Goal: Wire the pool into the RedisCache so all command argument buffers are pooled.

Tasks

Add argPool BufferPool to RedisCache.

Update NewRedisCache to initialize with a defaultArgPool.

Add WithBufferPool to allow injection of a custom pool.

Update GetMany (and similar methods) to use the pool for command arguments.

Code Sketch
type RedisCache struct {
client RedisClient
argPool BufferPool
}

func NewRedisCache(client RedisClient) \*RedisCache {
return &RedisCache{
client: client,
argPool: newDefaultArgPool(),
}
}

func (c *RedisCache) WithBufferPool(pool BufferPool) *RedisCache {
c.argPool = pool
return c
}

Definition of Done

RedisCache defaults to using defaultArgPool.

GetMany compiles and uses c.argPool.GetArgsBuf / c.argPool.PutArgsBuf.

Unit test passes when creating cache with and without custom pool.

Phase 4: Benchmark & Validate Allocations

Goal: Confirm pooling reduces memory usage.

Tasks

Add BenchmarkGetMany with -benchmem.

Compare before/after B/op and allocs/op.

Ensure allocations drop from thousands to near O(n) (≈100 for 100 keys).

Definition of Done

Benchmarks run successfully.

allocs/op reduced significantly compared to pre-pooling implementation.

Results documented in comments or a benchstat diff.

Phase 5: Advanced Pool Injection Example

Goal: Demonstrate how consumers can provide their own pool.

Tasks

Write an example MySharedPool that implements BufferPool.

Add an example usage snippet in example_test.go.

Code Sketch
type MySharedPool struct {
pool sync.Pool
}
func (p _MySharedPool) GetArgsBuf(sizeHint int) []interface{} { /_ ... */ }
func (p *MySharedPool) PutArgsBuf(buf []interface{}) { /_ ... _/ }
Definition of Done

Example compiles and shows how to inject a custom pool.

Example appears in go doc generated documentation.

Final Outcome

Redis cache module uses pooling internally by default.

Consumers can inject their own pooling strategy if needed.

Allocations and memory usage are significantly reduced in benchmarks.

Library maintains clean API defaults, while preserving advanced extensibility

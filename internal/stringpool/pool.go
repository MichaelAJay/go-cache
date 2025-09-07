package stringpool

import (
	"strings"
	"sync"
)

var builderPool = sync.Pool{
	New: func() interface{} {
		return &strings.Builder{}
	},
}

// Get returns a strings.Builder from the pool.
// The builder is reset and ready for use.
func Get() *strings.Builder {
	builder := builderPool.Get().(*strings.Builder)
	builder.Reset()
	return builder
}

// Put returns a strings.Builder to the pool for reuse.
// The builder should not be used after calling Put.
func Put(builder *strings.Builder) {
	// Only return to pool if it's not too large to prevent memory bloat
	if builder.Cap() < 1024 {
		builderPool.Put(builder)
	}
}
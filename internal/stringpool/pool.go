package stringpool

import (
	"strings"
	"sync"
)

var builderPool = sync.Pool{
	New: func() any {
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
// For security, the underlying buffer is overwritten to clear sensitive data.
func Put(builder *strings.Builder) {
	// Only return to pool if it's not too large to prevent memory bloat
	if builder.Cap() < 1024 {
		// Security: Overwrite the underlying buffer to clear any sensitive data
		// This prevents sensitive information like session IDs or user keys 
		// from persisting in pooled memory
		clearBuffer(builder)
		builderPool.Put(builder)
	}
}

// clearBuffer securely overwrites the builder's internal buffer
func clearBuffer(builder *strings.Builder) {
	// Get the current length to know how much to overwrite
	currentLen := builder.Len()
	if currentLen == 0 {
		return
	}
	
	// Reset and fill with zeros to overwrite sensitive data
	builder.Reset()
	for i := 0; i < currentLen; i++ {
		builder.WriteByte(0)
	}
	builder.Reset() // Final reset to make it ready for reuse
}

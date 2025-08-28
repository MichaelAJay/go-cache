package cache

import (
	"github.com/MichaelAJay/go-cache/interfaces"
	memoryprovider "github.com/MichaelAJay/go-cache/internal/providers/memory"
	redisprovider "github.com/MichaelAJay/go-cache/internal/providers/redis"
)

// NewMemoryProvider creates a new memory cache provider.
// The memory provider offers fast local caching with features like:
// - Thread-safe operations using sync.RWMutex
// - Automatic cleanup of expired entries
// - Secondary indexing for complex queries
// - Comprehensive metrics and observability
// - Security features like timing attack protection
// - gob serialization
func NewMemoryProvider() interfaces.CacheProvider {
	return memoryprovider.NewProvider()
}

// NewRedisProvider creates a new Redis cache provider.
// The Redis provider offers distributed caching with features like:
// - Redis-backed persistence and scalability
// - Atomic operations using Redis commands and Lua scripts
// - Pipeline operations for improved performance
// - Connection pooling and health monitoring
// - Serialization with multiple format support (MessagePack, JSON, etc.)
func NewRedisProvider() interfaces.CacheProvider {
	return redisprovider.NewProvider()
}

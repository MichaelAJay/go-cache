package common

import (
	"time"

	"github.com/MichaelAJay/go-cache"
)

// ApplyTimingProtection applies timing protection by ensuring minimum processing time.
// This helps protect against timing attacks by making operation duration consistent.
//
// Algorithm Details:
// 1. Calculate actual elapsed time since startTime
// 2. If elapsed time is less than configured minimum, sleep for the difference
// 3. This ensures all operations take at least MinProcessingTime, regardless of:
//    - Whether the key exists or not
//    - Size of the cached value  
//    - Success or failure of the operation
//    - Internal processing complexity
// 
// This is critical for security-sensitive applications where response timing
// could leak information about cache contents or system state.
func ApplyTimingProtection(securityConfig *cache.SecurityConfig, startTime time.Time) {
	if securityConfig == nil || !securityConfig.EnableTimingProtection {
		return
	}

	elapsed := time.Since(startTime)
	if elapsed < securityConfig.MinProcessingTime {
		time.Sleep(securityConfig.MinProcessingTime - elapsed)
	}
}

// SecureWipeSlice securely wipes a byte slice by overwriting with zeros.
// This helps prevent sensitive data from remaining in memory.
//
// Security Implementation Details:
// 1. Checks for nil slice to avoid panic
// 2. Iterates through each byte position and sets to zero
// 3. Overwrites the original memory locations (not just slice header)
// 4. This prevents sensitive data from being recoverable via memory dumps
//    or garbage collection inspection
//
// Use this function when deallocating cache entries that contain sensitive
// data like session tokens, API keys, or personal information.
func SecureWipeSlice(data []byte) {
	if data == nil {
		return
	}
	for i := range data {
		data[i] = 0
	}
}
package common

import (
	"time"

	"github.com/MichaelAJay/go-cache"
)

// ApplyTimingProtection applies timing protection by ensuring minimum processing time
// This helps protect against timing attacks by making operation duration consistent
func ApplyTimingProtection(securityConfig *cache.SecurityConfig, startTime time.Time) {
	if securityConfig == nil || !securityConfig.EnableTimingProtection {
		return
	}

	elapsed := time.Since(startTime)
	if elapsed < securityConfig.MinProcessingTime {
		time.Sleep(securityConfig.MinProcessingTime - elapsed)
	}
}

// SecureWipeSlice securely wipes a byte slice by overwriting with zeros
// This helps prevent sensitive data from remaining in memory
func SecureWipeSlice(data []byte) {
	if data == nil {
		return
	}
	for i := range data {
		data[i] = 0
	}
}
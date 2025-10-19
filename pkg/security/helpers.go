package security

import (
	"crypto/subtle"
	"fmt"
	"math"
	"time"
)

// safeInt64ToUint64 safely converts int64 to uint64 with overflow checking
func safeInt64ToUint64(val int64) (uint64, error) {
	if val < 0 {
		return 0, fmt.Errorf("negative value cannot be converted to uint64: %d", val)
	}
	return uint64(val), nil
}

// safeInt64ToUint32 safely converts int64 to uint32 with overflow checking
func safeInt64ToUint32(val int64) (uint32, error) {
	if val < 0 {
		return 0, fmt.Errorf("negative value cannot be converted to uint32: %d", val)
	}
	if val > math.MaxUint32 {
		return 0, fmt.Errorf("value exceeds uint32 range: %d", val)
	}
	return uint32(val), nil
}

// safeIntToUint16 safely converts int to uint16 with overflow checking
func safeIntToUint16(val int) (uint16, error) {
	if val < 0 {
		return 0, fmt.Errorf("negative value cannot be converted to uint16: %d", val)
	}
	if val > math.MaxUint16 {
		return 0, fmt.Errorf("value exceeds uint16 range: %d", val)
	}
	return uint16(val), nil
}

// constantTimeCompare performs constant-time comparison of two byte slices
func constantTimeCompare(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare(a, b) == 1
}

// zeroSensitiveData securely zeros sensitive data in memory
func zeroSensitiveData(data []byte) {
	for i := range data {
		data[i] = 0
	}
}

// validateCellInput validates cell input data
func validateCellInput(data []byte) error {
	if data == nil {
		return fmt.Errorf("nil input data")
	}
	if len(data) < 5 {
		return fmt.Errorf("cell too short: %d bytes", len(data))
	}
	if len(data) > 65535 {
		return fmt.Errorf("cell too long: %d bytes", len(data))
	}
	return nil
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	tokens    int
	maxTokens int
	refillAt  time.Time
	interval  time.Duration
}

// newRateLimiter creates a new rate limiter
func newRateLimiter(maxTokens int, interval time.Duration) *RateLimiter {
	return &RateLimiter{
		tokens:    maxTokens,
		maxTokens: maxTokens,
		refillAt:  time.Now().Add(interval),
		interval:  interval,
	}
}

// Allow checks if an operation is allowed
func (rl *RateLimiter) Allow() bool {
	now := time.Now()
	if now.After(rl.refillAt) {
		rl.tokens = rl.maxTokens
		rl.refillAt = now.Add(rl.interval)
	}
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}
	return false
}

// ResourceManager manages resource allocation limits
type ResourceManager struct {
	limit   int
	current int
	// resource field removed as it was unused
}

// newResourceManager creates a new resource manager
func newResourceManager(limit int) *ResourceManager {
	return &ResourceManager{
		limit:   limit,
		current: 0,
	}
}

// Allocate attempts to allocate a resource
func (rm *ResourceManager) Allocate(resourceType string) error {
	if rm.current >= rm.limit {
		return fmt.Errorf("resource limit exceeded for %s: %d/%d", resourceType, rm.current, rm.limit)
	}
	rm.current++
	return nil
}

// Release releases a resource
func (rm *ResourceManager) Release() {
	if rm.current > 0 {
		rm.current--
	}
}

// getRecommendedTLSConfig returns a secure TLS configuration
func getRecommendedTLSConfig() *Config {
	return &Config{
		MinVersion: VersionTLS12,
		CipherSuites: []uint16{
			TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
		PreferServerCipherSuites: true,
	}
}

// Config represents a TLS configuration for testing
type Config struct {
	MinVersion               uint16
	CipherSuites             []uint16
	PreferServerCipherSuites bool
}

// TLS version constants
const (
	VersionTLS12 = 0x0303
)

// TLS cipher suite constants
const (
	TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256 = 0xc02b
	TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256   = 0xc02f
	TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384 = 0xc02c
	TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384   = 0xc030
	TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305  = 0xcca9
	TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305    = 0xcca8
)

// Package security provides security utilities for the Tor client implementation
package security

import (
	"fmt"
	"math"
	"time"
)

// SafeUnixToUint64 safely converts a Unix timestamp to uint64
// Returns error if the timestamp is negative or would overflow
func SafeUnixToUint64(t time.Time) (uint64, error) {
	unix := t.Unix()
	if unix < 0 {
		return 0, fmt.Errorf("negative timestamp: %d", unix)
	}
	// Check would overflow uint64 (though practically impossible with Unix timestamps)
	if unix < 0 {
		return 0, fmt.Errorf("timestamp overflow: %d", unix)
	}
	return uint64(unix), nil
}

// SafeUnixToUint32 safely converts a Unix timestamp to uint32
// Returns error if the timestamp is negative or would overflow uint32
// Note: Will overflow in year 2106 (max uint32 = 4294967295)
func SafeUnixToUint32(t time.Time) (uint32, error) {
	unix := t.Unix()
	if unix < 0 {
		return 0, fmt.Errorf("negative timestamp: %d", unix)
	}
	if unix > math.MaxUint32 {
		return 0, fmt.Errorf("timestamp exceeds uint32 range: %d (max: %d)", unix, uint32(math.MaxUint32))
	}
	return uint32(unix), nil
}

// SafeIntToUint64 safely converts an int to uint64
// Returns error if the value is negative
func SafeIntToUint64(val int) (uint64, error) {
	if val < 0 {
		return 0, fmt.Errorf("negative value: %d", val)
	}
	return uint64(val), nil
}

// SafeIntToUint16 safely converts an int to uint16
// Returns error if the value is negative or exceeds uint16 range
func SafeIntToUint16(val int) (uint16, error) {
	if val < 0 {
		return 0, fmt.Errorf("value out of uint16 range (negative): %d", val)
	}
	if val > math.MaxUint16 {
		return 0, fmt.Errorf("value out of uint16 range: %d (max: %d)", val, math.MaxUint16)
	}
	return uint16(val), nil
}

// SafeInt64ToUint64 safely converts an int64 to uint64
// Returns error if the value is negative
func SafeInt64ToUint64(val int64) (uint64, error) {
	if val < 0 {
		return 0, fmt.Errorf("negative int64 value: %d", val)
	}
	return uint64(val), nil
}

// SafeLenToUint16 is a convenience function to safely convert a slice length to uint16
// This is commonly needed for protocol length fields
func SafeLenToUint16(data []byte) (uint16, error) {
	return SafeIntToUint16(len(data))
}

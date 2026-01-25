// Package config provides bridge parsing and validation.
package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// BridgeInfo represents a parsed bridge line with optional pluggable transport.
type BridgeInfo struct {
	// Transport is the pluggable transport name (empty for vanilla bridges)
	Transport string

	// Address is the bridge IP address or hostname
	Address string

	// Port is the bridge OR port
	Port int

	// Fingerprint is the optional bridge identity fingerprint (40 hex chars)
	Fingerprint string

	// Parameters contains PT-specific parameters (key=value pairs)
	Parameters map[string]string

	// Raw is the original bridge line
	Raw string
}

// ParseBridge parses a bridge line from torrc format.
//
// Supported formats:
//   - Bridge IP:PORT [fingerprint]
//   - Bridge IP:PORT fingerprint
//   - Bridge transport IP:PORT [fingerprint] [key=value ...]
//   - Bridge transport IP:PORT fingerprint key=value ...
//
// Examples:
//   - Bridge 192.0.2.1:443
//   - Bridge 192.0.2.1:443 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
//   - Bridge obfs4 192.0.2.1:1234 cert=abcd iat-mode=0
//   - Bridge obfs4 192.0.2.1:1234 AAAA... cert=abcd iat-mode=0
func ParseBridge(line string) (*BridgeInfo, error) {
	// Remove leading "Bridge" keyword if present
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "Bridge ") {
		line = strings.TrimPrefix(line, "Bridge ")
		line = strings.TrimSpace(line)
	}

	if line == "" {
		return nil, fmt.Errorf("empty bridge line")
	}

	parts := strings.Fields(line)
	if len(parts) < 1 {
		return nil, fmt.Errorf("invalid bridge line: %s", line)
	}

	bridge := &BridgeInfo{
		Raw:        line,
		Parameters: make(map[string]string),
	}

	idx := 0

	// Check if first part is a transport name (not an address:port)
	// Transport names don't contain ':' character
	firstPart := parts[idx]
	if !strings.Contains(firstPart, ":") {
		// Could be a transport name or just missing port
		// Check if next part exists and looks like address:port
		if idx+1 < len(parts) && strings.Contains(parts[idx+1], ":") {
			// This is a transport name
			bridge.Transport = firstPart
			idx++
		}
	}

	// Next part must be address:port
	addrPort := parts[idx]
	idx++

	// Parse address:port
	host, portStr, err := net.SplitHostPort(addrPort)
	if err != nil {
		return nil, fmt.Errorf("invalid address:port '%s': %w", addrPort, err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port '%s': %w", portStr, err)
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("port %d out of range [1-65535]", port)
	}

	bridge.Address = host
	bridge.Port = port

	// Parse optional fingerprint and parameters
	for idx < len(parts) {
		part := parts[idx]
		idx++

		// Check if this is a key=value parameter
		if strings.Contains(part, "=") {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				bridge.Parameters[kv[0]] = kv[1]
			}
			continue
		}

		// Check if this is a fingerprint (40 hex chars)
		if len(part) == 40 && isHexString(part) {
			bridge.Fingerprint = strings.ToUpper(part)
			continue
		}

		// Unknown format - treat as parameter without value
		bridge.Parameters[part] = ""
	}

	return bridge, nil
}

// String returns a canonical string representation of the bridge.
func (b *BridgeInfo) String() string {
	var parts []string

	if b.Transport != "" {
		parts = append(parts, b.Transport)
	}

	parts = append(parts, fmt.Sprintf("%s:%d", b.Address, b.Port))

	if b.Fingerprint != "" {
		parts = append(parts, b.Fingerprint)
	}

	// Add parameters in deterministic order
	for k, v := range b.Parameters {
		if v != "" {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		} else {
			parts = append(parts, k)
		}
	}

	return strings.Join(parts, " ")
}

// IsPluggableTransport returns true if this bridge uses a pluggable transport.
func (b *BridgeInfo) IsPluggableTransport() bool {
	return b.Transport != ""
}

// GetTransportName returns the transport name or empty string for vanilla bridges.
func (b *BridgeInfo) GetTransportName() string {
	return b.Transport
}

// GetAddress returns the bridge address (host:port).
func (b *BridgeInfo) GetAddress() string {
	return fmt.Sprintf("%s:%d", b.Address, b.Port)
}

// isHexString returns true if s contains only hexadecimal characters
func isHexString(s string) bool {
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

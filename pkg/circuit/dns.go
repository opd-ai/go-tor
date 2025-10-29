// Package circuit provides DNS resolution through Tor circuits
package circuit

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
)

// DNS resolution types (tor-spec.txt section 6.4)
const (
	// DNS record types for RELAY_RESOLVE
	DNSTypeHostname = 0x00 // Hostname to IPv4/IPv6
	DNSTypeIPv4     = 0x04 // IPv4 address (PTR query)
	DNSTypeIPv6     = 0x06 // IPv6 address (PTR query)
	DNSTypeError    = 0xF0 // Error response
	DNSTypeErrorTTL = 0xF1 // Error with TTL
)

// DNS error codes (tor-spec.txt section 6.4)
const (
	DNSErrorNone              = 0x00 // No error
	DNSErrorFormat            = 0x01 // Format error
	DNSErrorServerFailure     = 0x02 // Server failure
	DNSErrorNotExist          = 0x03 // Name does not exist
	DNSErrorNotImplemented    = 0x04 // Not implemented
	DNSErrorRefused           = 0x05 // Query refused
	DNSErrorTransientFailure  = 0xF0 // Transient failure
	DNSErrorNonTransientFailure = 0xF1 // Non-transient failure
)

// DNSResult represents the result of a DNS query
type DNSResult struct {
	Type     byte          // DNS record type
	TTL      uint32        // Time to live in seconds
	Addresses []net.IP     // Resolved IP addresses
	Hostname string        // Resolved hostname (for PTR queries)
	Error    byte          // Error code (if Type is DNSTypeError)
}

// ResolveHostname resolves a hostname to IP addresses through the circuit
// This implements DNS leak prevention by routing DNS queries through Tor
func (c *Circuit) ResolveHostname(ctx context.Context, hostname string) (*DNSResult, error) {
	// Validate hostname
	if hostname == "" {
		return nil, fmt.Errorf("hostname cannot be empty")
	}

	// Create RELAY_RESOLVE payload
	// Format: hostname\x00 (null-terminated string)
	payload := append([]byte(hostname), 0x00)

	// Use stream ID 0 for DNS queries (they don't need a stream)
	resolveCell := cell.NewRelayCell(0, cell.RelayResolve, payload)

	// Send RELAY_RESOLVE cell
	if err := c.SendRelayCell(resolveCell); err != nil {
		return nil, fmt.Errorf("failed to send RELAY_RESOLVE: %w", err)
	}

	// Wait for RELAY_RESOLVED response with timeout
	resolveCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resolvedCell, err := c.ReceiveRelayCell(resolveCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to receive RELAY_RESOLVED: %w", err)
	}

	// Verify this is a RELAY_RESOLVED cell
	if resolvedCell.Command != cell.RelayResolved {
		return nil, fmt.Errorf("expected RELAY_RESOLVED, got %s", cell.RelayCmdString(resolvedCell.Command))
	}

	// Parse RELAY_RESOLVED response
	result, err := parseResolvedCell(resolvedCell.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RELAY_RESOLVED: %w", err)
	}

	// Check for errors
	if result.Type == DNSTypeError || result.Type == DNSTypeErrorTTL {
		return result, fmt.Errorf("DNS resolution failed: error code %d", result.Error)
	}

	return result, nil
}

// ResolveIP performs reverse DNS lookup (PTR query) through the circuit
func (c *Circuit) ResolveIP(ctx context.Context, ipAddr net.IP) (*DNSResult, error) {
	// Validate IP address
	if ipAddr == nil {
		return nil, fmt.Errorf("IP address cannot be nil")
	}

	// Create RELAY_RESOLVE payload for PTR query
	// Format: TYPE (0x04 for IPv4 or 0x06 for IPv6) | LENGTH | ADDRESS
	var payload []byte
	if ipv4 := ipAddr.To4(); ipv4 != nil {
		// IPv4 PTR query
		payload = make([]byte, 6)
		payload[0] = DNSTypeIPv4
		payload[1] = 4 // Length
		copy(payload[2:], ipv4)
	} else if ipv6 := ipAddr.To16(); ipv6 != nil {
		// IPv6 PTR query
		payload = make([]byte, 18)
		payload[0] = DNSTypeIPv6
		payload[1] = 16 // Length
		copy(payload[2:], ipv6)
	} else {
		return nil, fmt.Errorf("invalid IP address")
	}

	// Use stream ID 0 for DNS queries
	resolveCell := cell.NewRelayCell(0, cell.RelayResolve, payload)

	// Send RELAY_RESOLVE cell
	if err := c.SendRelayCell(resolveCell); err != nil {
		return nil, fmt.Errorf("failed to send RELAY_RESOLVE: %w", err)
	}

	// Wait for RELAY_RESOLVED response
	resolveCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resolvedCell, err := c.ReceiveRelayCell(resolveCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to receive RELAY_RESOLVED: %w", err)
	}

	// Verify this is a RELAY_RESOLVED cell
	if resolvedCell.Command != cell.RelayResolved {
		return nil, fmt.Errorf("expected RELAY_RESOLVED, got %s", cell.RelayCmdString(resolvedCell.Command))
	}

	// Parse RELAY_RESOLVED response
	result, err := parseResolvedCell(resolvedCell.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RELAY_RESOLVED: %w", err)
	}

	// Check for errors
	if result.Type == DNSTypeError || result.Type == DNSTypeErrorTTL {
		return result, fmt.Errorf("reverse DNS lookup failed: error code %d", result.Error)
	}

	return result, nil
}

// parseResolvedCell parses a RELAY_RESOLVED cell payload
// Format per tor-spec.txt section 6.4:
// - Multiple answers, each:
//   - TYPE (1 byte): 0x00 (hostname), 0x04 (IPv4), 0x06 (IPv6), 0xF0/0xF1 (error)
//   - LENGTH (1 byte): length of answer
//   - VALUE (variable): depends on type
//   - TTL (4 bytes): time to live in seconds
//
// Note: While the protocol supports multiple DNS records in a single response,
// this implementation currently returns only the first valid record found.
// This matches typical DNS resolver behavior where the first address is used.
// Applications requiring multiple addresses should make multiple RESOLVE requests
// or use the circuit API directly with custom parsing logic.
func parseResolvedCell(data []byte) (*DNSResult, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty RELAY_RESOLVED data")
	}

	result := &DNSResult{
		Addresses: make([]net.IP, 0),
	}

	offset := 0
	for offset < len(data) {
		// Need at least TYPE + LENGTH (2 bytes)
		if offset+2 > len(data) {
			break
		}

		recordType := data[offset]
		length := int(data[offset+1])
		offset += 2

		// Validate length
		if offset+length+4 > len(data) {
			return nil, fmt.Errorf("invalid RELAY_RESOLVED record: incomplete data")
		}

		value := data[offset : offset+length]
		offset += length

		// Read TTL (4 bytes)
		ttl := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		// Process based on record type
		switch recordType {
		case DNSTypeHostname:
			// Hostname (null-terminated string)
			hostname := string(value)
			if len(hostname) > 0 && hostname[len(hostname)-1] == 0 {
				hostname = hostname[:len(hostname)-1]
			}
			result.Type = DNSTypeHostname
			result.Hostname = hostname
			result.TTL = ttl
			return result, nil // Return immediately for hostname

		case DNSTypeIPv4:
			// IPv4 address (4 bytes)
			if length != 4 {
				return nil, fmt.Errorf("invalid IPv4 address length: %d", length)
			}
			ip := net.IPv4(value[0], value[1], value[2], value[3])
			result.Type = DNSTypeIPv4
			result.Addresses = append(result.Addresses, ip)
			result.TTL = ttl
			return result, nil // Return immediately for IPv4

		case DNSTypeIPv6:
			// IPv6 address (16 bytes)
			if length != 16 {
				return nil, fmt.Errorf("invalid IPv6 address length: %d", length)
			}
			ip := make(net.IP, 16)
			copy(ip, value)
			result.Type = DNSTypeIPv6
			result.Addresses = append(result.Addresses, ip)
			result.TTL = ttl
			return result, nil // Return immediately for IPv6

		case DNSTypeError, DNSTypeErrorTTL:
			// Error response (1 byte error code)
			if length < 1 {
				return nil, fmt.Errorf("invalid error record length: %d", length)
			}
			result.Type = recordType
			result.Error = value[0]
			result.TTL = ttl
			return result, nil // Return immediately on error

		default:
			// Unknown record type - skip it
			continue
		}
	}

	return nil, fmt.Errorf("no valid DNS records found in RELAY_RESOLVED")
}

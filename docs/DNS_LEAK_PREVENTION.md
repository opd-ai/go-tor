# DNS Leak Prevention

## Overview

DNS leak prevention is a critical security feature that ensures all DNS queries are routed through the Tor network, preventing information leakage to ISPs or local network observers.

## Implementation Status

**Phase**: ROADMAP Phase 1.2 - Partially Implemented  
**Status**: SOCKS5 Command Support Added, RELAY_RESOLVE Cells Pending

## Features

### SOCKS5 Command Support

The SOCKS5 proxy server now accepts and recognizes Tor-specific DNS resolution commands:

- **RESOLVE (0xF0)**: DNS hostname to IP address resolution
- **RESOLVE_PTR (0xF1)**: Reverse DNS (IP address to hostname) resolution

### Configuration

DNS resolution can be controlled via the SOCKS5 server configuration:

```go
config := &socks.Config{
    EnableDNSResolution: true,              // Enable/disable DNS commands
    DNSTimeout:          30 * time.Second,  // Timeout for DNS operations
}
```

**Default**: DNS resolution is **enabled by default** to prevent leaks.

### Command Processing

When a client sends a RESOLVE or RESOLVE_PTR command:

1. The SOCKS5 server validates the command based on configuration
2. If enabled, the command is accepted and parsed
3. A circuit is allocated from the circuit pool
4. The DNS query is queued for resolution through the Tor network

## Current Limitations

### RELAY_RESOLVE Cell Support

The current implementation accepts RESOLVE/RESOLVE_PTR commands but does not yet implement the full RELAY_RESOLVE cell protocol. This means:

- DNS commands are **accepted** to prevent rejection and potential DNS leaks
- Commands currently return an error indicating cells are not yet implemented
- Applications should handle this gracefully or use CONNECT to IP addresses

### Future Work

To complete DNS leak prevention (remaining ROADMAP Phase 1.2 tasks):

1. **Implement RELAY_RESOLVE cells** in `pkg/cell/relay.go`
   - Add RELAY_RESOLVE (type 11) and RELAY_RESOLVED (type 12) cell types
   - Implement cell encoding/decoding

2. **Add DNS resolution protocol** in `pkg/stream/`
   - Send RELAY_RESOLVE cells through circuits
   - Wait for and parse RELAY_RESOLVED responses
   - Handle multiple IP addresses in responses
   - Implement proper error handling for DNS failures

3. **Integrate with SOCKS5 handlers**
   - Update `handleResolve()` to send RELAY_RESOLVE cells
   - Update `handleResolvePTR()` for reverse DNS
   - Return resolved addresses to client via `sendDNSReply()`

4. **Add DNS caching** (optional optimization)
   - Cache resolved addresses with TTL
   - Reduce circuit load for repeated queries

## Security Benefits

When fully implemented, DNS leak prevention provides:

- **Privacy**: DNS queries don't reveal visited domains to ISP
- **Anonymity**: No correlation between DNS and network traffic
- **Tor Network Integration**: All resolution through Tor exit nodes
- **Attack Resistance**: Prevents DNS-based traffic analysis

## Testing

DNS resolution command acceptance is tested in:
- `pkg/socks/socks_test.go::TestDNSResolutionCommands`
- `pkg/socks/socks_test.go::TestDNSConfigDefaults`
- `pkg/socks/socks_test.go::TestRequestInfoStructure`

Integration tests for full RELAY_RESOLVE protocol should be added after cell implementation.

## Usage Example

### Client Application

```go
// Application should configure resolver to use SOCKS5 DNS
// This prevents system DNS lookups that would leak outside Tor

// Option 1: Use SOCKS5 RESOLVE command (when fully implemented)
// Most Tor-aware applications support this automatically

// Option 2: Resolve to IP first, then CONNECT
// Less ideal but prevents leaks if RESOLVE not supported
```

### Server Configuration

```go
import "github.com/opd-ai/go-tor/pkg/socks"

// Enable DNS resolution (default)
config := socks.DefaultConfig()
config.EnableDNSResolution = true
config.DNSTimeout = 30 * time.Second

server := socks.NewServerWithConfig(":9050", circuitMgr, logger, config)
```

## References

- Tor Specification: Section 6.4 - RELAY_RESOLVE cells
- RFC 1928: SOCKS Protocol Version 5
- ROADMAP.md: Phase 1.2 - Missing DNS Leak Prevention Mechanisms
- AUDIT.md: Medium Severity Issue #4 - Missing DNS leak prevention

## Related Issues

- ROADMAP Phase 1.2: DNS Leak Prevention (in progress)
- Cell protocol extensions needed for RELAY_RESOLVE support
- Stream management integration for DNS queries

## Changelog

### 2025-10-29
- Added RESOLVE (0xF0) and RESOLVE_PTR (0xF1) command support
- Implemented `EnableDNSResolution` configuration option
- Added `handleResolve()` and `handleResolvePTR()` handlers
- Created `sendDNSReply()` for DNS response formatting
- Added comprehensive test coverage for command acceptance
- Documented implementation status and future work

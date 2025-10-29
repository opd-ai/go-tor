# DNS Resolution Through Tor Circuits

## Overview

This document describes the implementation of DNS resolution through Tor circuits (RELAY_RESOLVE/RELAY_RESOLVED cells), which prevents DNS leaks and ensures all DNS queries are routed through the Tor network.

## Background

DNS leaks are a critical privacy concern when using Tor. If DNS queries are sent directly to the local DNS server or ISP, they can reveal which websites a user is trying to visit, defeating the purpose of using Tor for anonymity.

## Implementation

### RELAY_RESOLVE Cells

RELAY_RESOLVE cells (command type 11) are used to request DNS resolution through a Tor circuit. They support two types of queries:

1. **Hostname to IP** (forward DNS lookup)
2. **IP to Hostname** (reverse DNS / PTR query)

### RELAY_RESOLVED Cells

RELAY_RESOLVED cells (command type 12) contain the response to a RELAY_RESOLVE query. They can contain:

- IPv4 addresses (type 0x04)
- IPv6 addresses (type 0x06)
- Hostnames (type 0x00, for PTR responses)
- Error codes (types 0xF0/0xF1)

## API Usage

### Forward DNS Lookup (Hostname to IP)

```go
import (
    "context"
    "github.com/opd-ai/go-tor/pkg/circuit"
)

// Resolve a hostname through a Tor circuit
result, err := circuit.ResolveHostname(ctx, "example.com")
if err != nil {
    log.Fatal(err)
}

// Access the resolved IP addresses
for _, ip := range result.Addresses {
    fmt.Printf("Resolved to: %s (TTL: %d)\n", ip, result.TTL)
}
```

### Reverse DNS Lookup (IP to Hostname)

```go
import (
    "context"
    "net"
    "github.com/opd-ai/go-tor/pkg/circuit"
)

// Perform reverse DNS lookup through a Tor circuit
ip := net.ParseIP("192.0.2.1")
result, err := circuit.ResolveIP(ctx, ip)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("IP %s resolves to: %s (TTL: %d)\n", ip, result.Hostname, result.TTL)
```

## SOCKS5 Integration

The SOCKS5 server automatically handles DNS resolution commands:

### RESOLVE Command (0xF0)

Clients can send a RESOLVE command to resolve hostnames without establishing a connection:

```
Client → SOCKS5: [0x05][0xF0][0x00][0x03][len][hostname][port]
SOCKS5 → Client: [0x05][status][0x00][addr_type][address][TTL]
```

### RESOLVE_PTR Command (0xF1)

Clients can send a RESOLVE_PTR command for reverse DNS lookups:

```
Client → SOCKS5: [0x05][0xF1][0x00][addr_type][address][port]
SOCKS5 → Client: [0x05][status][0x00][0x03][len][hostname][TTL]
```

## Configuration

DNS resolution is enabled by default. You can configure it using the SOCKS5 server configuration:

```go
import "github.com/opd-ai/go-tor/pkg/socks"

config := socks.DefaultConfig()
config.EnableDNSResolution = true  // Enable DNS resolution (default)
config.DNSTimeout = 30 * time.Second  // DNS query timeout
```

## Protocol Details

### RELAY_RESOLVE Payload Format

**Hostname Query:**
```
hostname\x00  (null-terminated string)
```

**PTR Query (IPv4):**
```
[TYPE: 0x04][LENGTH: 4][IPv4 address: 4 bytes]
```

**PTR Query (IPv6):**
```
[TYPE: 0x06][LENGTH: 16][IPv6 address: 16 bytes]
```

### RELAY_RESOLVED Payload Format

Multiple answers may be included in a single response:

```
For each answer:
  [TYPE: 1 byte]
  [LENGTH: 1 byte]
  [VALUE: variable]
  [TTL: 4 bytes, big-endian]
```

**Record Types:**
- `0x00`: Hostname (null-terminated string)
- `0x04`: IPv4 address (4 bytes)
- `0x06`: IPv6 address (16 bytes)
- `0xF0`: Error (with error code)
- `0xF1`: Error with TTL

**Error Codes:**
- `0x00`: No error
- `0x01`: Format error
- `0x02`: Server failure
- `0x03`: Name does not exist (NXDOMAIN)
- `0x04`: Not implemented
- `0x05`: Query refused
- `0xF0`: Transient failure
- `0xF1`: Non-transient failure

## Security Considerations

1. **Stream ID 0**: DNS queries use stream ID 0 as they don't require a persistent stream.

2. **Timeout**: DNS queries have a 30-second timeout to prevent indefinite waiting.

3. **Privacy**: All DNS queries are routed through the exit node, preventing local DNS leaks.

4. **Error Handling**: DNS errors are properly categorized and reported to the application.

5. **TTL Handling**: DNS responses include TTL values, but caching is not yet implemented. Applications should handle their own DNS caching if needed.

## Testing

The implementation includes comprehensive unit tests:

```bash
# Run DNS-related tests
go test ./pkg/circuit -v -run "TestDNS|TestResolve|TestParse"

# Run all circuit tests
go test ./pkg/circuit -v
```

### Test Coverage

- ✅ RELAY_RESOLVE cell creation (hostname and PTR queries)
- ✅ RELAY_RESOLVED cell parsing (IPv4, IPv6, hostname, error responses)
- ✅ DNS error handling (NXDOMAIN, server failure, etc.)
- ✅ Invalid input validation
- ✅ Payload format verification

## Known Limitations

1. **No DNS Caching**: The implementation does not cache DNS responses. Applications should implement their own caching if needed.

2. **Single Address Response**: While the protocol supports multiple addresses, the SOCKS5 response format only returns the first address. Applications needing all addresses should use the circuit API directly.

3. **No DNSSEC**: DNSSEC validation is not implemented.

4. **Integration Tests**: Full integration tests with real circuits require additional mocking infrastructure and are not included in the current test suite.

## References

- [Tor Specification - Section 6.4: Remote hostname lookup](https://spec.torproject.org/tor-spec/redirecting-streams.html#remote-hostname-lookup)
- [SOCKS5 Extensions for DNS](https://gitweb.torproject.org/torspec.git/tree/socks-extensions.txt)
- [RFC 1928: SOCKS Protocol Version 5](https://tools.ietf.org/html/rfc1928)

## Implementation Files

- `pkg/circuit/dns.go`: Core DNS resolution implementation
- `pkg/circuit/dns_test.go`: Comprehensive unit tests
- `pkg/socks/socks.go`: SOCKS5 RESOLVE/RESOLVE_PTR command handling
- `pkg/cell/relay.go`: RELAY_RESOLVE/RELAY_RESOLVED cell definitions

## Future Enhancements

1. **DNS Caching**: Implement a DNS cache with TTL-based expiration
2. **DNSSEC**: Add DNSSEC validation support
3. **Multiple Addresses**: Enhance SOCKS5 response to return multiple addresses
4. **Integration Tests**: Add end-to-end tests with mock Tor relays
5. **Performance Metrics**: Track DNS resolution latency and success rates

# Circuit Extension Implementation

This document describes the implementation of server-side circuit extension handling in the go-tor relay package.

## Overview

Circuit extension is a fundamental operation in the Tor network that allows clients to build multi-hop circuits through relay servers. When a relay receives a `RELAY_EXTEND2` cell, it must:

1. Parse the link specifiers to determine the next hop relay
2. Establish a connection to the next hop
3. Forward the circuit creation request (CREATE2)
4. Relay the response back to the client (EXTENDED2)

## Implementation Status

**Task 10.2.1: Circuit Extension Handling** ✅ **COMPLETED**

### Components

#### Link Specifier Parsing

The `parseLinkSpecifiers()` function implements link specifier parsing per tor-spec.txt §5.3:

```
NSPEC      (1 byte)   - Number of link specifiers
NSPEC * [
  LSTYPE   (1 byte)   - Link specifier type
  LSLEN    (1 byte)   - Link specifier length
  LSPEC    (LSLEN)    - Link specifier data
]
```

Supported link specifier types:
- Type 0: IPv4 address + port (6 bytes: 4 IP + 2 port)
- Type 1: IPv6 address + port (18 bytes: 16 IP + 2 port)
- Type 2: Legacy identity fingerprint (20 bytes)
- Type 3: Ed25519 identity key (32 bytes)

#### Address Extraction

The `extractAddressFromLinkSpecs()` function converts link specifiers into a connectable address string (IP:port format). IPv4 addresses are preferred over IPv6 for compatibility.

#### Extension Handler

The `ExtensionHandler` manages circuit extension operations:

- **Connection Pooling**: Maintains a pool of outbound connections to next hop relays to avoid repeated connection establishment
- **Link Protocol Handshake**: Performs VERSIONS cell exchange with next hop relays
- **CREATE2 Forwarding**: Forwards circuit creation requests to the next hop
- **EXTENDED2 Relaying**: Relays handshake responses back to the client

### EXTEND2 Cell Format

Per tor-spec.txt §5.3, EXTEND2 cells contain:

```
NSPEC      (1 byte)     - Number of link specifiers
Link Specs (variable)   - Link specifiers (see above)
HTYPE      (2 bytes)    - Handshake type (0x0002 for ntor)
HLEN       (2 bytes)    - Handshake data length
HDATA      (HLEN bytes) - Handshake data
```

### EXTENDED2 Cell Format

Per tor-spec.txt §5.4, EXTENDED2 cells contain:

```
HLEN       (2 bytes)    - Handshake response length
HDATA      (HLEN bytes) - Handshake response data
```

## Security Considerations

### Handshake Type Validation

Only ntor handshakes (type 0x0002) are supported. TAP handshakes are rejected as they use deprecated RSA-1024 cryptography.

### Resource Management

The implementation includes:
- Connection pooling to limit resource consumption
- Timeouts on all network operations (10s for connections, 30s for handshakes)
- Proper cleanup of connections on handler shutdown

### Error Handling

All errors during extension are logged and propagated. Common failure modes:
- Invalid link specifiers (malformed data)
- No usable addresses in link specifiers
- Connection failures to next hop
- Handshake type mismatch
- Circuit ID conflicts

## Testing

Comprehensive unit tests cover:
- Link specifier parsing (>93% coverage)
- Address extraction from link specifiers (90% coverage)
- Error handling for malformed EXTEND2 cells
- Unsupported handshake type rejection
- Circuit registration
- EXTENDED2 cell construction

Test files:
- `pkg/relay/extension_test.go` - Unit tests for extension functionality

## Integration Points

### CircuitHandler Integration

The `ExtensionHandler` works with the `CircuitHandler` to:
- Look up circuits by ID
- Register extended circuit mappings
- Track next hop connections

### Future Work (Task 10.2.2)

Complete cell forwarding implementation will require:
- Bidirectional relay cell routing between circuits
- Proper encryption layer management at each hop
- RELAY_EARLY cell counting (max 8 per direction)
- DESTROY cell propagation

## Usage Example

```go
// Create extension handler
keys, _ := relay.GenerateRelayKeys()
circuits := relay.NewCircuitHandler(keys, logger)
extension := relay.NewExtensionHandler(keys, circuits, logger)

// Handle EXTEND2 cell from client
ctx := context.Background()
circuitID := uint32(123)
relayCell := &cell.RelayCell{
    Command:  cell.RelayExtend2,
    StreamID: 0,
    Data:     extend2Data,
}

err := extension.HandleExtend2(ctx, circuitID, relayCell)
if err != nil {
    // Handle error
}

// Cleanup
extension.Close()
```

## References

- **tor-spec.txt §5.3**: EXTEND2 cell specification
- **tor-spec.txt §5.4**: EXTENDED2 cell specification
- **tor-spec.txt §1-2**: Link protocol and TLS handshaking
- **tor-spec.txt §4**: Circuit creation and ntor handshake

## Changelog

- **2026-01-25**: Initial implementation of Task 10.2.1 (Circuit Extension Handling)
  - Link specifier parsing
  - Next hop connection management
  - CREATE2/CREATED2 forwarding
  - EXTENDED2 response construction
  - Comprehensive test coverage (>80% for core functions)

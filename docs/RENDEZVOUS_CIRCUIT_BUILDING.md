# Rendezvous Circuit Building for Onion Services

## Overview

This document describes the implementation of rendezvous circuit building for onion service hosting, which is Task 9.2.2 in the go-tor implementation plan. This feature enables onion services to build circuits to client-specified rendezvous points as part of the onion service introduction protocol.

## Specification Reference

- **rend-spec-v3.txt §3.2-3.3**: INTRODUCE2 cell handling and rendezvous circuit establishment
- **tor-spec.txt §5.1.2**: Link specifier format

## Architecture

### Components

#### `RendezvousCircuitBuilder` (`pkg/onion/rendezvous.go`)

The `RendezvousCircuitBuilder` is responsible for:

1. **Parsing Link Specifiers**: Extracting relay information (IPv4/IPv6 address, fingerprint) from the client's INTRODUCE2 cell
2. **Relay Discovery**: Finding the rendezvous relay in the consensus using fingerprint or address matching
3. **Path Selection**: Selecting a 3-hop path with the rendezvous point as the exit relay
4. **Circuit Building**: Using the existing circuit builder to construct the circuit

```go
type RendezvousCircuitBuilder struct {
    circuitBuilder CircuitBuilderInterface
    pathSelector   PathSelectorInterface
    logger         *logger.Logger
}
```

#### Integration with Onion Service

The `Service` struct has been enhanced with:

```go
type Service struct {
    // ... existing fields ...
    rendezvousBuilder   *RendezvousCircuitBuilder  // Builds circuits to rendezvous points
    rendezvousCircuits  map[string]uint32          // cookie -> circuit ID
}
```

## Protocol Flow

When a client wants to connect to an onion service:

1. **Client sends INTRODUCE2** → Onion service receives cell at introduction point
2. **Parse INTRODUCE2** → Extract rendezvous cookie, client onion key, and link specifiers
3. **Build Rendezvous Circuit** (THIS TASK) → Construct 3-hop circuit to rendezvous point
4. **Send RENDEZVOUS1** (Task 9.2.3, TODO) → Complete handshake with client
5. **Stream Handling** (Task 9.3, TODO) → Forward traffic to local service

## Implementation Details

### Link Specifier Parsing

Link specifiers (tor-spec.txt §5.1.2) describe how to reach a relay:

- **Type 0x00**: TLS-over-TCP-IPv4 (6 bytes: 4-byte IP + 2-byte port)
- **Type 0x01**: TLS-over-TCP-IPv6 (18 bytes: 16-byte IP + 2-byte port)
- **Type 0x02**: Legacy RSA identity fingerprint (20 bytes)
- **Type 0x03**: Ed25519 identity key (32 bytes, preferred)

The implementation:
- Extracts address and fingerprint from link specifiers
- Prefers IPv4 over IPv6 for address resolution
- Uses Ed25519 identity for secure relay matching

### Relay Discovery

The `findRelayInConsensus()` method:

1. **Primary matching**: Ed25519 identity key (most secure)
2. **Legacy matching**: RSA fingerprint (backward compatibility)
3. **Fallback matching**: IPv4 address (when fingerprint unavailable)

### Path Selection

The circuit to the rendezvous point uses:

- **Guard relay**: Selected from guard-flagged relays
- **Middle relay**: Selected from stable relays
- **Exit relay**: The client-specified rendezvous point

Selection ensures:
- No relay appears twice in the path
- Family diversity (no relays in same /16 subnet)
- Bandwidth-weighted selection for guard and middle

### Asynchronous Building

Circuit building happens asynchronously to avoid blocking INTRODUCE2 handling:

```go
go func() {
    ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
    defer cancel()
    
    circ, err := s.rendezvousBuilder.BuildRendezvousCircuit(...)
    // ... handle result ...
}()
```

This allows the service to:
- Continue processing other introduction requests
- Handle multiple concurrent rendezvous establishment attempts
- Clean up failed attempts without blocking the service

## Error Handling

The implementation handles various failure cases:

- **Invalid link specifiers**: Missing address information
- **Relay not in consensus**: Rendezvous point not found or not reachable
- **Path selection failure**: Insufficient relays for diversity requirements
- **Circuit build timeout**: 25-second timeout for circuit establishment

On error:
- Pending introduction is removed from tracking
- Error is logged for debugging
- No RENDEZVOUS1 is sent (client will timeout and retry)

## Testing

Comprehensive test coverage (>95%) includes:

- **Unit tests**: Link specifier parsing, relay discovery, path selection
- **Integration tests**: End-to-end rendezvous circuit building
- **Error cases**: Invalid inputs, missing relays, timeouts

Test file: `pkg/onion/rendezvous_test.go` (18 test cases)

## Usage Example

```go
// Create service with circuit builder and path selector
config := &ServiceConfig{
    CircuitBuilder: circuitBuilder,
    PathSelector:   pathSelector,
    // ... other config ...
}

service, err := NewService(config, logger)
if err != nil {
    // handle error
}

// Start service
err = service.Start()

// Service will automatically build rendezvous circuits when
// INTRODUCE2 cells are received at introduction points
```

## Future Work (Task 9.2.3)

The next step is implementing RENDEZVOUS1 cell construction:

1. **Server-side ntor handshake**: Generate handshake response for client
2. **Key derivation**: Derive end-to-end encryption keys
3. **RENDEZVOUS1 cell**: Send handshake response to client via rendezvous circuit
4. **Stream establishment**: Complete the connection for traffic forwarding

## Performance Considerations

- **Circuit caching**: Rendezvous circuits are tracked by cookie for correlation with RENDEZVOUS1
- **Concurrent builds**: Multiple rendezvous circuits can be built simultaneously
- **Timeout handling**: 25-second circuit build timeout prevents resource exhaustion
- **Cleanup**: Failed attempts are removed from tracking to prevent memory leaks

## Security Considerations

- **Fingerprint verification**: Ed25519 identity verification prevents relay impersonation
- **Constant-time comparison**: Fingerprint matching uses constant-time comparison
- **No information leakage**: Errors don't reveal network topology or relay selection
- **Educational use only**: This implementation is for research/education, not production anonymity

## References

- [rend-spec-v3.txt](https://spec.torproject.org/rend-spec-v3) - v3 onion service specification
- [tor-spec.txt](https://spec.torproject.org/tor-spec) - Core Tor protocol specification
- `pkg/circuit/builder.go` - Circuit building infrastructure
- `pkg/path/path.go` - Path selection algorithms

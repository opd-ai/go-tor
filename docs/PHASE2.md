# Phase 2: Core Protocol Implementation Guide

This document provides detailed information about the Phase 2 implementation of go-tor, which adds core protocol functionality including TLS connections, protocol handshake, and directory client.

## Overview

Phase 2 implements the foundational networking components required to communicate with the Tor network:

1. **TLS Connection Management** (`pkg/connection`)
2. **Protocol Handshake** (`pkg/protocol`)
3. **Directory Client** (`pkg/directory`)

These components enable the client to:
- Establish secure connections to Tor relays
- Negotiate protocol versions
- Fetch network consensus information
- Prepare for circuit building (Phase 3)

## Components

### 1. Connection Package (`pkg/connection`)

The connection package provides TLS connection management for communicating with Tor relays.

#### Key Features
- Asynchronous TLS connection establishment with timeout
- Connection state tracking (Connecting → Handshaking → Open → Closed/Failed)
- Thread-safe cell sending and receiving
- Proper error handling and connection cleanup
- Context-aware operations

#### Usage Example

```go
import (
    "context"
    "github.com/opd-ai/go-tor/pkg/connection"
    "github.com/opd-ai/go-tor/pkg/logger"
)

// Create connection configuration
cfg := connection.DefaultConfig("127.0.0.1:9001")
log := logger.NewDefault()

// Create and connect
conn := connection.New(cfg, log)
err := conn.Connect(context.Background(), cfg)
if err != nil {
    log.Error("Connection failed", "error", err)
    return
}
defer conn.Close()

// Send a cell
cell := cell.NewCell(0, cell.CmdVersions)
err = conn.SendCell(cell)

// Receive a cell
receivedCell, err := conn.ReceiveCell()
```

#### Connection States

- **StateConnecting**: Initial state, TCP connection in progress
- **StateHandshaking**: TLS handshake in progress
- **StateOpen**: Connection ready for sending/receiving cells
- **StateClosed**: Connection closed normally
- **StateFailed**: Connection failed to establish or had error

### 2. Protocol Package (`pkg/protocol`)

The protocol package implements Tor protocol handshake and version negotiation.

#### Key Features
- Link protocol version negotiation (supports v3-v5)
- VERSIONS cell exchange
- NETINFO cell exchange
- Automatic version selection (highest mutually supported)

#### Usage Example

```go
import (
    "context"
    "github.com/opd-ai/go-tor/pkg/protocol"
)

// Assuming conn is an established connection
handshake := protocol.NewHandshake(conn, log)

err := handshake.PerformHandshake(context.Background())
if err != nil {
    log.Error("Handshake failed", "error", err)
    return
}

version := handshake.NegotiatedVersion()
log.Info("Protocol negotiated", "version", version)
```

#### Protocol Flow

1. **Client → Relay**: VERSIONS cell with supported versions [3, 4, 5]
2. **Relay → Client**: VERSIONS cell with relay's supported versions
3. **Negotiate**: Select highest mutually supported version
4. **Client → Relay**: NETINFO cell with timestamp and address info
5. **Relay → Client**: NETINFO cell response
6. **Complete**: Handshake complete, connection ready for circuits

#### Supported Versions

- **Version 3**: Link protocol v3 (older relays)
- **Version 4**: Link protocol v4 with 4-byte circuit IDs (preferred)
- **Version 5**: Link protocol v5 (newest)

### 3. Directory Package (`pkg/directory`)

The directory package provides HTTP-based directory protocol client functionality for fetching network consensus.

#### Key Features
- Fetch consensus from multiple directory authorities
- Parse consensus documents
- Extract relay information (nickname, address, ports, flags)
- Filter relays by flags (Guard, Exit, Stable, etc.)
- Automatic fallback to other authorities on failure

#### Usage Example

```go
import (
    "context"
    "github.com/opd-ai/go-tor/pkg/directory"
)

// Create directory client
dirClient := directory.NewClient(log)

// Fetch network consensus
ctx := context.Background()
relays, err := dirClient.FetchConsensus(ctx)
if err != nil {
    log.Error("Failed to fetch consensus", "error", err)
    return
}

log.Info("Consensus fetched", "total_relays", len(relays))

// Find guard relays
for _, relay := range relays {
    if relay.IsGuard() && relay.IsRunning() {
        log.Info("Found guard", "nickname", relay.Nickname, "address", relay.Address)
    }
}
```

#### Relay Information

Each relay in the consensus includes:
- **Nickname**: Human-readable relay name
- **Fingerprint**: Relay's unique identifier
- **Address**: IP address
- **ORPort**: Onion Router port
- **DirPort**: Directory port (if directory mirror)
- **Flags**: List of flags (Guard, Exit, Stable, Fast, Running, Valid, etc.)

#### Relay Flags

- **Guard**: Suitable for use as entry guard
- **Exit**: Allows exit traffic
- **Fast**: Above bandwidth threshold
- **Stable**: High uptime
- **Running**: Currently operational
- **Valid**: Verified by directory authorities

## Testing

All Phase 2 components include comprehensive unit tests:

```bash
# Test all Phase 2 packages
go test ./pkg/connection ./pkg/protocol ./pkg/directory -v

# Run with race detector
go test -race ./pkg/connection ./pkg/protocol ./pkg/directory

# With coverage
go test -cover ./pkg/connection ./pkg/protocol ./pkg/directory
```

Current test coverage:
- `pkg/connection`: ~85%
- `pkg/protocol`: ~90%
- `pkg/directory`: ~90%

## Demo Program

A complete demo program is provided in `examples/phase2-demo/`:

```bash
# Build the demo
go build ./examples/phase2-demo

# Run the demo
./phase2-demo
```

The demo demonstrates:
1. Fetching network consensus
2. Finding suitable guard relays
3. Attempting connection to a relay
4. Performing protocol handshake

**Note**: Full connection may fail due to TLS certificate validation requirements, but the demo shows the complete API usage.

## Integration

Phase 2 components integrate with existing Phase 1 components:

- Uses `pkg/cell` for encoding/decoding cells
- Uses `pkg/logger` for structured logging
- Uses `pkg/config` for configuration
- Uses `pkg/crypto` for cryptographic operations
- Prepares for `pkg/circuit` circuit building (Phase 3)

## Security Considerations

1. **TLS Security**: Currently uses `InsecureSkipVerify` for testing. Production code must implement proper certificate validation per Tor specification.

2. **Timeout Management**: All operations have configurable timeouts to prevent hangs.

3. **State Management**: Thread-safe state transitions prevent race conditions.

4. **Error Handling**: Errors are properly wrapped and logged without leaking sensitive information.

## Known Limitations

1. **Certificate Validation**: Proper Tor relay certificate validation not yet implemented
2. **Consensus Parsing**: Basic parsing implemented; full microdescriptor support pending
3. **Multiplexing**: Cell multiplexing on single connection not yet implemented
4. **Consensus Caching**: No caching of consensus documents yet

## Next Steps (Phase 3)

The Phase 2 implementation prepares for Phase 3, which will add:

1. **Circuit Building**: CREATE2/CREATED2 cell handling
2. **Path Selection**: Guard/middle/exit node selection algorithms
3. **Circuit Extension**: EXTEND2/EXTENDED2 for multi-hop circuits
4. **Key Derivation**: KDF-TOR for layer encryption keys
5. **Circuit Multiplexing**: Multiple circuits on single connection

## References

- [tor-spec.txt](https://spec.torproject.org/tor-spec): Core Tor protocol specification
- [dir-spec.txt](https://spec.torproject.org/dir-spec): Directory protocol specification
- Link Protocol: tor-spec.txt section 2
- Handshake: tor-spec.txt section 3
- Directory: dir-spec.txt section 3

## Troubleshooting

### Connection Timeout
If connections timeout, check:
- Network connectivity
- Firewall rules allowing outbound connections
- Relay address and port are correct
- Timeout value is sufficient (default 30s)

### Handshake Failure
Common causes:
- Incompatible protocol versions
- Network issues during handshake
- Relay closed connection

### Consensus Fetch Failure
Try:
- Check internet connectivity
- Verify directory authority URLs are accessible
- Increase HTTP client timeout
- Check for proxy/firewall blocking HTTPS

## Contributing

When contributing to Phase 2 code:

1. Add comprehensive unit tests
2. Use existing logging patterns
3. Handle all error cases
4. Add timeout to all network operations
5. Document public APIs
6. Follow existing code style

# go-tor Architecture

## Overview

go-tor is a pure Go implementation of a Tor client designed for embedded systems. This document provides an architectural overview of the system.

## Design Principles

1. **Pure Go**: No CGo dependencies for maximum portability
2. **Client-Only**: No relay or exit node functionality 
3. **Embedded-Optimized**: Low memory footprint and efficient resource usage
4. **Modular**: Clean separation of concerns between packages
5. **Testable**: Comprehensive unit and integration tests

## System Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     Application Layer                    │
│                   (SOCKS5 Clients, etc.)                 │
└─────────────────────────────────────────────────────────┘
                            ▲
                            │
┌─────────────────────────────────────────────────────────┐
│                     SOCKS5 Proxy                         │
│              (pkg/socks - RFC 1928)                      │
└─────────────────────────────────────────────────────────┘
                            ▲
                            │
┌─────────────────────────────────────────────────────────┐
│                  Circuit Manager                         │
│         (pkg/circuit - Circuit lifecycle)                │
└─────────────────────────────────────────────────────────┘
                            ▲
                            │
┌─────────────────────────────────────────────────────────┐
│                   Cell Processing                        │
│        (pkg/cell - Encode/decode protocol cells)         │
└─────────────────────────────────────────────────────────┘
                            ▲
                            │
┌─────────────────────────────────────────────────────────┐
│                  Protocol Layer                          │
│      (pkg/protocol - TLS, handshake, versioning)         │
└─────────────────────────────────────────────────────────┘
                            ▲
                            │
┌─────────────────────────────────────────────────────────┐
│                  Network Layer                           │
│            (TCP/TLS connections to relays)               │
└─────────────────────────────────────────────────────────┘
```

### Supporting Systems

```
┌─────────────────┐  ┌──────────────────┐  ┌─────────────────┐
│  Path Selection │  │ Directory Client │  │ Crypto Primitives│
│   (pkg/path)    │  │  (pkg/directory) │  │   (pkg/crypto)   │
└─────────────────┘  └──────────────────┘  └─────────────────┘

┌─────────────────┐  ┌──────────────────┐  ┌─────────────────┐
│ Onion Services  │  │ Control Protocol │  │  Configuration   │
│   (pkg/onion)   │  │  (pkg/control)   │  │   (pkg/config)   │
└─────────────────┘  └──────────────────┘  └─────────────────┘
```

## Package Descriptions

### pkg/cell
Cell encoding and decoding. Implements fixed-size (514 bytes) and variable-size cell formats according to tor-spec.txt section 3.

**Key Types:**
- `Cell`: Represents a Tor protocol cell
- `RelayCell`: Represents a RELAY/RELAY_EARLY cell payload
- `Command`: Cell command type

### pkg/circuit
Circuit management and lifecycle. Handles creation, extension, and teardown of circuits.

**Key Types:**
- `Circuit`: Represents a path through the Tor network
- `Hop`: Represents a single relay in a circuit
- `Manager`: Manages multiple circuits
- `State`: Circuit state (Building, Open, Closed, Failed)

### pkg/crypto
Cryptographic primitives used by Tor protocol.

**Key Operations:**
- AES-128/256 in CTR mode (cell encryption)
- RSA with OAEP (hybrid encryption)
- SHA-1, SHA-256 (hashing and digests)
- Key derivation functions (KDF-TOR, HKDF)

### pkg/config
Configuration management with torrc-compatible options.

**Key Types:**
- `Config`: Main configuration structure
- `OnionServiceConfig`: Onion service configuration

### pkg/protocol (TODO)
Core Tor protocol implementation including:
- Version negotiation
- TLS connection setup
- Link protocol handshake
- Cell multiplexing

### pkg/directory (TODO)
Directory protocol client for:
- Consensus document fetching
- Router descriptor retrieval
- Microdescriptor support
- Directory authority interaction

### pkg/path (TODO)
Path selection algorithms:
- Guard node selection and persistence
- Middle node selection
- Exit node selection based on policies
- Path diversity enforcement

### pkg/socks (TODO)
SOCKS5 proxy server (RFC 1928):
- SOCKS5 protocol handling
- Stream routing through circuits
- DNS resolution over Tor
- Stream isolation

### pkg/onion (TODO)
Onion service functionality:
- Client: .onion address resolution
- Server: Service hosting and descriptor publishing
- Introduction point protocol
- Rendezvous protocol

### pkg/control (TODO)
Tor control protocol (client-relevant subset):
- Authentication
- Circuit/stream information
- Configuration management
- Event monitoring

## Data Flow

### Circuit Creation
```
1. Application → SOCKS5 → Circuit Manager: Request circuit
2. Circuit Manager → Path Selection: Select guards/middle/exit
3. Circuit Manager → Protocol Layer: CREATE2 cell to guard
4. Protocol Layer → Network: Send cell over TLS
5. Network → Protocol Layer: CREATED2 response
6. Circuit Manager: Mark circuit as ready
```

### Stream Routing
```
1. SOCKS5: Accept connection
2. SOCKS5 → Circuit Manager: Get available circuit
3. SOCKS5 → Circuit: Send RELAY_BEGIN cell
4. Circuit → Network: Route through circuit hops
5. Network → Circuit: RELAY_CONNECTED response
6. SOCKS5 ↔ Circuit: Proxy data as RELAY_DATA cells
```

## Security Considerations

1. **Constant-time Crypto**: All cryptographic operations use constant-time implementations to prevent timing attacks
2. **Memory Zeroing**: Sensitive data (keys, etc.) are explicitly zeroed after use
3. **Error Handling**: Errors do not leak timing or state information
4. **Circuit Padding**: Traffic analysis resistance through padding
5. **Guard Discovery**: Precautions against guard fingerprinting

## Performance Targets

- **Circuit Build Time**: < 5 seconds (95th percentile)
- **Memory Usage**: < 50MB RSS in steady state
- **Concurrent Streams**: 100+ on Raspberry Pi 3
- **Binary Size**: < 15MB static binary

## Testing Strategy

1. **Unit Tests**: Test individual components in isolation
2. **Integration Tests**: Test with real Tor network (using chutney)
3. **Fuzzing**: Fuzz cell parsers and protocol handlers
4. **Load Tests**: Stress test circuit management
5. **Compatibility Tests**: Verify compatibility with C Tor

## Development Phases

### Phase 1: Foundation ✅ (Complete)
✅ Project structure
✅ Cell encoding/decoding
✅ Circuit management types
✅ Crypto wrappers
✅ Configuration system
✅ Structured logging (log/slog)
✅ Graceful shutdown with context

### Phase 2: Core Protocol (In Progress)
- [ ] TLS connection handling
- [ ] Protocol handshake
- [ ] Cell I/O and multiplexing
- [ ] Directory client

### Phase 3: Client Functionality (Planned)
- [ ] SOCKS5 proxy
- [ ] Path selection
- [ ] Guard persistence
- [ ] Stream handling

### Phase 4: Onion Services (Planned)
- [ ] Client-side .onion support
- [ ] Server-side service hosting
- [ ] Descriptor management

### Phase 5: Production Ready (Planned)
- [ ] Control protocol
- [ ] Optimization for embedded
- [ ] Security hardening
- [ ] Comprehensive testing

## References

- [tor-spec.txt](https://spec.torproject.org/tor-spec) - Core protocol
- [dir-spec.txt](https://spec.torproject.org/dir-spec) - Directory protocol
- [rend-spec.txt](https://spec.torproject.org/rend-spec) - Rendezvous (onion services)
- [control-spec.txt](https://spec.torproject.org/control-spec) - Control protocol

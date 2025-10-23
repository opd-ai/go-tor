# go-tor Architecture

## ⚠️ Important Notice

This document describes an **unofficial, experimental Tor client implementation** developed for educational and research purposes. This software has been developed **without the supervision or endorsement of The Tor Project**.

**This software should NOT be used for any real anonymity or privacy needs.**

For actual Tor usage:
- **Users**: Use [Tor Browser](https://www.torproject.org/download/)
- **Developers**: Use [Arti](https://gitlab.torproject.org/tpo/core/arti) (official Rust implementation) or the [C reference implementation](https://github.com/torproject/tor)

---

## Overview

go-tor is a pure Go implementation of a Tor client designed for embedded systems **as an educational project**. This document provides an architectural overview of the system.

**Note**: While this implementation attempts to follow Tor specifications, it has not undergone the extensive security review and testing that official Tor software receives.

## Design Principles

1. **Pure Go**: No CGo dependencies for maximum portability
2. **Client-Only**: No relay or exit node functionality 
3. **Embedded-Optimized**: Low memory footprint and efficient resource usage
4. **Modular**: Clean separation of concerns between packages
5. **Testable**: Comprehensive unit and integration tests
6. **Educational**: Designed for learning about Tor protocols (NOT for real anonymity)

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

### pkg/connection ✅ (NEW in Phase 2)
TLS connection management for Tor relays.

**Key Types:**
- `Connection`: TLS connection to a relay with state management
- `Config`: Connection configuration
- `State`: Connection state (Connecting, Handshaking, Open, Closed, Failed)

**Key Operations:**
- Establish TLS connections with timeout
- Send/receive cells with proper synchronization
- Connection lifecycle management
- Error handling and state transitions

### pkg/protocol ✅ (NEW in Phase 2)
Core Tor protocol implementation including version negotiation and handshake.

**Key Types:**
- `Handshake`: Performs protocol handshake
- Protocol version constants (v3-v5 support)

**Key Operations:**
- Version negotiation (VERSIONS cell exchange)
- NETINFO cell exchange
- Protocol version selection

### pkg/directory ✅ (NEW in Phase 2)
Directory protocol client for fetching network information.

**Key Types:**
- `Client`: HTTP-based directory client
- `Relay`: Relay information from consensus

**Key Operations:**
- Fetch network consensus from directory authorities
- Parse consensus documents
- Extract relay information (address, ports, flags)
- Filter relays by flags (Guard, Exit, etc.)
- Version negotiation
- TLS connection setup
- Link protocol handshake
- Cell multiplexing

### pkg/path ✅
Path selection algorithms for building circuits.

**Key Features:**
- Guard node selection and persistence
- Middle node selection
- Exit node selection based on policies
- Path diversity enforcement
- Bandwidth-weighted selection

### pkg/socks ✅
SOCKS5 proxy server (RFC 1928).

**Key Features:**
- SOCKS5 protocol handling
- Stream routing through circuits
- DNS resolution over Tor
- .onion address support
- Connection multiplexing

### pkg/onion ✅
Onion service functionality for v3 onion services.

**Key Features:**
- Client: .onion address resolution and connection
- Server: Service hosting and descriptor publishing
- Introduction point protocol
- Rendezvous protocol
- Descriptor management and caching
- Ed25519 cryptographic operations

### pkg/control ✅
Tor control protocol implementation.

**Key Features:**
- Authentication (NULL method)
- Circuit and stream information
- Configuration management
- Event monitoring and notifications
- GETINFO, SETCONF, GETCONF commands
- Event types: CIRC, STREAM, BW, ORCONN, NEWDESC, GUARD, NS

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

### Phase 2: Core Protocol ✅ (Complete)
✅ TLS connection handling
✅ Protocol handshake and version negotiation
✅ Cell I/O with state management
✅ Directory client (consensus fetching)
✅ Connection lifecycle management
✅ Error handling and timeouts

### Phase 3: Client Functionality ✅ (Complete)
✅ SOCKS5 proxy server (RFC 1928)
✅ Path selection (guard, middle, exit)
✅ Guard persistence
✅ Stream handling and multiplexing

### Phase 4: Stream Management ✅ (Complete)
✅ Stream multiplexing over circuits
✅ Circuit extension (CREATE2/EXTENDED2)
✅ Key derivation (KDF-TOR)
✅ Stream isolation foundation

### Phase 5: Component Integration ✅ (Complete)
✅ Client orchestration package
✅ Circuit pool management
✅ Health monitoring
✅ Functional Tor client application
✅ End-to-end testing

### Phase 6: Production Hardening ✅ (Complete)
✅ Complete circuit extension cryptography
✅ Guard node persistence
✅ Performance optimization
✅ Security hardening and audit
✅ Comprehensive testing and benchmarking

### Phase 7: Control Protocol & Onion Services ✅ (Complete)
✅ Control protocol server with basic commands
✅ Event notification system (CIRC, STREAM, BW, ORCONN, NEWDESC, GUARD, NS)
✅ v3 onion address parsing and validation
✅ Descriptor management (caching, crypto)
✅ HSDir protocol and descriptor fetching
✅ Introduction protocol
✅ Rendezvous protocol
✅ Hidden service server (hosting)

### Phase 8: Advanced Features ✅ (Complete)
✅ Configuration file loading (torrc-compatible)
✅ Enhanced error handling and resilience
✅ Performance optimization and tuning
✅ Security hardening and audit
✅ Comprehensive testing and documentation
✅ Onion Service Infrastructure Completion

## References

- [tor-spec.txt](https://spec.torproject.org/tor-spec) - Core protocol
- [dir-spec.txt](https://spec.torproject.org/dir-spec) - Directory protocol
- [rend-spec.txt](https://spec.torproject.org/rend-spec) - Rendezvous (onion services)
- [control-spec.txt](https://spec.torproject.org/control-spec) - Control protocol

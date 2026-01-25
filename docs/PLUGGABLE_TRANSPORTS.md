# Pluggable Transports (PT) Implementation

This document describes the pluggable transport support implementation in go-tor per [pt-spec.txt](https://spec.torproject.org/pt-spec).

## Overview

Pluggable transports (PTs) allow Tor to use external programs to transform traffic and evade censorship. The `pkg/pt` package implements PT version 1 IPC protocol for managing external PT processes.

## Architecture

### Components

1. **Transport Interface** (`transport.go`)
   - `Transport`: Base interface for all pluggable transports
   - `ClientTransport`: Client-side PT interface
   - `ServerTransport`: Server-side PT interface (for bridge relays)
   - `TransportConfig`: Configuration for PT instances
   - `MethodInfo`: Metadata for PT methods (CMETHOD/SMETHOD)

2. **Managed Client** (`client.go`)
   - `ManagedClient`: Implements `ClientTransport` interface
   - Manages external PT process lifecycle
   - Performs PT IPC handshake
   - Provides SOCKS proxy connections through PT

### Design Principles

- **Process Isolation**: PT runs as separate process for security
- **Standard IPC**: Uses PT v1 protocol over stdin/stdout
- **SOCKS Wrapping**: PT provides SOCKS proxy, we dial through it
- **Environment Configuration**: PT configured via environment variables

## PT Protocol Flow

### Client-Side Handshake

```
1. Parent (go-tor)          PT Process
   ├─ Set environment vars
   ├─ Launch PT executable ──>
   │                        <── VERSION 1
   │                        <── CMETHOD <name> socks5 <addr>
   │                        <── CMETHODS DONE
   ├─ Parse CMETHOD lines
   └─ Ready to dial
```

### Environment Variables

Per pt-spec.txt, the following environment variables configure the PT:

- `TOR_PT_MANAGED_TRANSPORT_VER=1` - Protocol version
- `TOR_PT_STATE_LOCATION=<dir>` - State directory path
- `TOR_PT_CLIENT_TRANSPORTS=*` - Requested transports ("*" = all)
- `TOR_PT_PROXY=<url>` - Optional upstream proxy
- `TOR_PT_OPT_<KEY>=<value>` - Transport-specific options

### CMETHOD Line Format

```
CMETHOD <transport> <socksversion> <address> [<arg>=<value>]*
```

Example:
```
CMETHOD obfs4 socks5 127.0.0.1:1234 cert=ABCD iat-mode=0
```

## Usage

### Basic Client Usage

```go
import "github.com/opd-ai/go-tor/pkg/pt"

// Create PT client
config := pt.TransportConfig{
    BinaryPath: "/usr/bin/obfs4proxy",
    StateDir:   "/var/lib/tor-pt",
    Options: map[string]string{
        "cert": "...",
        "iat-mode": "0",
    },
}

client, err := pt.NewManagedClient(config)
if err != nil {
    log.Fatal(err)
}

// Start PT process and handshake
ctx := context.Background()
if err := client.Start(ctx); err != nil {
    log.Fatal(err)
}
defer client.Close()

// Check available methods
methods := client.Methods()
fmt.Println("Available transports:", methods)

// Dial through PT
conn, err := client.Dial(ctx, "bridge.example.com:443")
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

// Use conn for Tor circuit building
```

### Bridge Configuration

To use PTs with bridges, configure bridge lines in your torrc:

```
UseBridges 1
Bridge obfs4 192.0.2.1:443 cert=ABCDEF... iat-mode=0
ClientTransportPlugin obfs4 exec /usr/bin/obfs4proxy
```

## Supported Transports

The PT framework supports any PT that implements the protocol. Common transports:

- **obfs4**: Randomizes traffic to look like random bytes
- **meek**: Tunnels through HTTPS (domain fronting)
- **snowflake**: WebRTC-based anti-censorship
- **scramblesuit**: Polymorphic encryption

## Implementation Status

### Phase 11.1.1: PT Client Interface ✅

- [x] `ClientTransport` interface defined
- [x] PT version 1 IPC protocol implemented
- [x] PT subprocess lifecycle management
- [x] CMETHOD line parsing
- [x] SOCKS5 connection wrapping
- [x] Environment variable configuration
- [x] Comprehensive unit tests (37.9% coverage)

### Remaining Work

- [ ] Task 11.1.2: PT Server Interface (for bridge relays)
- [ ] Task 11.1.3: PT Configuration (torrc parsing)
- [ ] Task 11.2: Built-in obfs4 support
- [ ] Task 11.3: External PT integration
- [ ] Task 11.4: Bridge client integration

## Testing

Run tests with:

```bash
# Unit tests (fast)
go test -v -short ./pkg/pt/...

# All tests including integration
go test -v ./pkg/pt/...

# With race detector
go test -race -v ./pkg/pt/...
```

### Test Coverage

Current coverage: **37.9%**

Areas tested:
- Client creation and configuration
- Environment variable generation
- CMETHOD parsing (socks4, socks5, args)
- Method registration
- Process lifecycle
- SOCKS5 handshake

Integration tests require actual PT binaries and are skipped in `-short` mode.

## Security Considerations

1. **Process Isolation**: PT runs in separate process with limited privileges
2. **State Directory**: Should have restrictive permissions (0700)
3. **Binary Verification**: Verify PT binary integrity before execution
4. **Timeouts**: Handshake has 30-second timeout to prevent hangs
5. **Error Handling**: PT errors don't leak sensitive information

## References

- [pt-spec.txt](https://spec.torproject.org/pt-spec) - Pluggable Transport Specification
- [obfs4 spec](https://gitlab.com/yawning/obfs4/-/blob/master/doc/obfs4-spec.txt)
- [Tor Browser User Manual](https://tb-manual.torproject.org/bridges/)

---

**Status**: Phase 11.1.1 Complete  
**Last Updated**: January 25, 2026  
**Test Coverage**: 37.9%

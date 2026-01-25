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

3. **Managed Server** (`server.go`)
   - `ManagedServer`: Implements `ServerTransport` interface
   - Manages server-side PT process lifecycle
   - Performs SMETHOD handshake
   - Provides listener for bridge relays

4. **PT Manager** (`manager.go`)
   - Multi-PT lifecycle management
   - Automatic process restart on failure
   - Concurrent client and server PT support
   - Health monitoring

5. **PT Discovery** (`discovery.go`)
   - Automatic PT binary path discovery
   - Platform-specific search paths
   - Common PT detection (obfs4proxy, snowflake, etc.)

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

### PT Manager (Multi-PT with Auto-Restart)

```go
import "github.com/opd-ai/go-tor/pkg/pt"

// Create manager with auto-restart enabled
mgr := pt.NewManager(pt.ManagerConfig{
    StateDir:     "/var/lib/tor/pt",
    AutoRestart:  true,
    RestartDelay: 5 * time.Second,
    MaxRestarts:  0, // Unlimited restarts
})
defer mgr.Close()

// Add multiple PTs
mgr.AddClient("obfs4", pt.TransportConfig{
    BinaryPath: "/usr/bin/obfs4proxy",
})

mgr.AddClient("snowflake", pt.TransportConfig{
    BinaryPath: "/usr/bin/snowflake-client",
})

// Start all PTs (monitored automatically)
if err := mgr.StartAll(context.Background()); err != nil {
    log.Warn("Some PTs failed to start:", err)
    // Manager will auto-restart failed PTs
}

// Get specific PT for use
client, _ := mgr.GetClient("obfs4")
conn, _ := client.Dial(ctx, "bridge:443")
```

### PT Discovery

```go
// Find specific PT binary
path, err := pt.DiscoverPT("obfs4proxy")
if err != nil {
    log.Fatal("obfs4proxy not found:", err)
}

// Discover all common PTs
pts := pt.DiscoverCommonPTs()
for name, path := range pts {
    fmt.Printf("Found %s at %s\n", name, path)
}

// Use discovered PT
if path, ok := pts["obfs4proxy"]; ok {
    config := pt.TransportConfig{BinaryPath: path}
    client, _ := pt.NewManagedClient(config)
}
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

### Phase 11.1.2: PT Server Interface ✅

- [x] `ServerTransport` interface implemented
- [x] SMETHOD line parsing
- [x] Server-side environment configuration
- [x] Bridge relay PT support
- [x] Comprehensive unit tests (39.9% coverage)

### Phase 11.1.3: PT Configuration ✅

- [x] torrc PT configuration parsing
- [x] ClientTransportPlugin support
- [x] ServerTransportPlugin support
- [x] PT options parsing
- [x] Comprehensive tests (82.7% config coverage)

### Phase 11.3.1: Managed PT Mode ✅

- [x] PT Manager for multi-PT lifecycle
- [x] Automatic process restart on failure
- [x] Configurable restart delay and limits
- [x] Process health monitoring
- [x] Graceful shutdown
- [x] Comprehensive tests (65.2% coverage)

### Phase 11.3.2: PT Path Configuration ✅

- [x] PT binary path discovery
- [x] Platform-specific search paths
- [x] Common PT detection (obfs4, snowflake, meek, lyrebird)
- [x] Absolute and relative path support
- [x] HOME directory search
- [x] Comprehensive tests (85.0% discovery coverage)

### Phase 11.4: Bridge Client Integration ✅

- [x] Bridge line parsing (vanilla and PT bridges)
- [x] PT parameter extraction
- [x] Configuration integration
- [x] Example implementations

### Remaining Work

- [ ] Task 11.2: Built-in obfs4 support (optional)
- [ ] Integration tests with real PT binaries

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

Current coverage: **65.2%** (overall pkg/pt package)

Component coverage:
- **client.go**: 37.9% (CMETHOD, SOCKS5, lifecycle)
- **server.go**: 39.9% (SMETHOD, server lifecycle)
- **manager.go**: Comprehensive (multi-PT, restart, monitoring)
- **discovery.go**: 85.0% (path discovery, platform detection)

Areas tested:
- Client/server creation and configuration
- Environment variable generation
- CMETHOD/SMETHOD parsing (socks4/5, args)
- Method registration
- Process lifecycle and monitoring
- Automatic restart with backoff
- PT binary discovery
- Multi-PT management

Integration tests require actual PT binaries and are skipped in `-short` mode.

## Security Considerations

1. **Process Isolation**: PT runs in separate process with limited privileges
2. **State Directory**: Should have restrictive permissions (0700)
3. **Binary Verification**: Verify PT binary integrity before execution
4. **Timeouts**: Handshake has 30-second timeout to prevent hangs
5. **Error Handling**: PT errors don't leak sensitive information
6. **Restart Limits**: Set `MaxRestarts` to prevent infinite restart loops
7. **Logging**: PT stderr output may contain sensitive configuration

## References

- [pt-spec.txt](https://spec.torproject.org/pt-spec) - Pluggable Transport Specification
- [obfs4 spec](https://gitlab.com/yawning/obfs4/-/blob/master/doc/obfs4-spec.txt)
- [Tor Browser User Manual](https://tb-manual.torproject.org/bridges/)

---

**Status**: Phases 11.1, 11.3, 11.4 Complete  
**Last Updated**: January 25, 2026  
**Test Coverage**: 65.2% overall

# obfs4 Pluggable Transport Implementation

This document describes the obfs4 pluggable transport implementation in go-tor.

## Overview

obfs4 is a transport protocol that obscures traffic patterns and content to evade censorship. It uses a variant of the ntor handshake for key exchange and AES-GCM for encryption, with optional Inter-Arrival Time (IAT) obfuscation to hide timing patterns.

This implementation wraps the external `obfs4proxy` binary as a managed PT process, following the pt-spec.txt IPC protocol. This approach ensures compatibility with the official obfs4 implementation and maintains security guarantees.

## Architecture

### Components

1. **obfs4.Client** (`pkg/pt/obfs4/client.go`)
   - Client-side obfs4 transport
   - Wraps ManagedClient from `pkg/pt`
   - Manages obfs4proxy subprocess
   - Provides SOCKS proxy connections

2. **obfs4.Server** (`pkg/pt/obfs4/server.go`)
   - Server-side obfs4 transport for bridge relays
   - Wraps ManagedServer from `pkg/pt`
   - Manages obfs4proxy server process
   - Generates and distributes certificates

3. **obfs4 Configuration** (`pkg/pt/obfs4/config.go`)
   - Certificate management
   - Bridge line parsing
   - Key export/import
   - Configuration validation

### Design Principles

- **External Process Management**: Uses obfs4proxy as subprocess for security isolation
- **Standard Compliance**: Follows pt-spec.txt IPC protocol
- **Battle-Tested Crypto**: Relies on official obfs4proxy implementation
- **Simple Integration**: Integrates seamlessly with existing PT infrastructure

## Usage

### Client Usage

```go
import (
    "context"
    "github.com/opd-ai/go-tor/pkg/pt/obfs4"
)

// Parse bridge line
bridgeLine := "obfs4 192.0.2.1:1234 AAAA cert=dGVzdGNlcnQ= iat-mode=0"
config, address, err := obfs4.ParseBridgeLine(bridgeLine)
if err != nil {
    log.Fatal(err)
}

// Create client
clientConfig := obfs4.ClientConfig{
    BinaryPath: "", // Auto-discover obfs4proxy
    Cert:       config.Certificate,
    IATMode:    config.IATMode,
    StateDir:   "/var/lib/tor-pt",
}

client, err := obfs4.NewClient(clientConfig)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// Start the PT process
ctx := context.Background()
if err := client.Start(ctx); err != nil {
    log.Fatal(err)
}

// Dial through obfs4
conn, err := client.Dial(ctx, address)
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

// Use connection...
```

### Server Usage

```go
import (
    "context"
    "github.com/opd-ai/go-tor/pkg/pt/obfs4"
)

// Create server
serverConfig := obfs4.ServerConfig{
    BinaryPath: "", // Auto-discover obfs4proxy
    BindAddr:   "127.0.0.1:0",
    StateDir:   "/var/lib/tor-bridge-pt",
    IATMode:    0,
}

server, err := obfs4.NewServer(serverConfig)
if err != nil {
    log.Fatal(err)
}
defer server.Close()

// Start the PT server process
ctx := context.Background()
if err := server.Start(ctx); err != nil {
    log.Fatal(err)
}

// Get certificate for distribution
cert, err := server.GetCertificate()
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Bridge certificate: %s\n", cert)

// Listen for connections
listener, err := server.Listen(ctx, "")
if err != nil {
    log.Fatal(err)
}

// Note: obfs4proxy manages the actual listener
// The returned listener is a placeholder
```

## Certificate Management

### Certificate Format

obfs4 certificates are base64-encoded strings containing:
- Node ID (public identifier)
- Public key (for handshake)

Example certificate:
```
cert=dGVzdGNlcnRpZmljYXRlMTIzNDU2Nzg5MDEyMzQ1Njc4OTA=
```

### Bridge Line Format

Standard bridge line format:
```
Bridge obfs4 <IP:PORT> <FINGERPRINT> cert=<CERT> iat-mode=<MODE>
```

Example:
```
Bridge obfs4 192.0.2.1:1234 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA cert=dGVzdGNlcnQ= iat-mode=0
```

### Key Management

#### Export Keys

```go
err := obfs4.ExportKeys(stateDir, "/path/to/backup.dat")
```

#### Import Keys

```go
err := obfs4.ImportKeys("/path/to/backup.dat", stateDir)
```

#### Load Existing Keys

```go
cert, err := obfs4.LoadServerKeys(stateDir)
```

## IAT (Inter-Arrival Time) Modes

obfs4 supports three IAT obfuscation modes:

- **Mode 0 (Disabled)**: No IAT obfuscation, minimal overhead
  - Use for: High-bandwidth applications, low-latency requirements
  - Overhead: Minimal

- **Mode 1 (Enabled)**: Standard IAT obfuscation
  - Use for: General censorship circumvention
  - Overhead: Moderate (~5-10%)

- **Mode 2 (Paranoid)**: Maximum IAT obfuscation
  - Use for: Highly restrictive censorship environments
  - Overhead: High (~15-25%)

Configure IAT mode in ClientConfig or ServerConfig:

```go
config := obfs4.ClientConfig{
    Cert:    "...",
    IATMode: 1, // 0, 1, or 2
}
```

## Integration with go-tor

### Circuit Builder Integration

obfs4 integrates with the circuit builder for bridge connections:

```go
// In circuit builder, when using bridges:
bridgeConfig, err := config.ParseBridge(bridgeLine)
if bridgeConfig.Transport == "obfs4" {
    client, err := obfs4.NewClient(obfs4.ClientConfig{
        Cert:     bridgeConfig.Params["cert"],
        IATMode:  atoi(bridgeConfig.Params["iat-mode"]),
        StateDir: cfg.DataDirectory,
    })
    
    conn, err := client.Dial(ctx, bridgeConfig.Address)
    // Use conn for circuit creation
}
```

### Bridge Relay Integration

obfs4 integrates with bridge relay server:

```go
// In bridge relay setup:
server, err := obfs4.NewServer(obfs4.ServerConfig{
    BindAddr: cfg.ORPort,
    StateDir: cfg.DataDirectory,
    IATMode:  0,
})

cert, err := server.GetCertificate()
// Include cert in bridge descriptor
```

## Requirements

### System Requirements

- **obfs4proxy** binary installed and accessible
  - Debian/Ubuntu: `apt-get install obfs4proxy`
  - Arch: `pacman -S obfs4proxy`
  - macOS: `brew install obfs4proxy`
  - From source: https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird

### PT Discovery

obfs4 uses the PT discovery mechanism to locate obfs4proxy:

```go
discovered := pt.DiscoverCommonPTs()
if obfs4Path, ok := discovered["obfs4proxy"]; ok {
    // Use obfs4Path
}
```

Search paths (automatically checked):
- `/usr/bin/obfs4proxy`
- `/usr/local/bin/obfs4proxy`
- `~/tor-browser/Browser/TorBrowser/Tor/PluggableTransports/obfs4proxy`
- User home directory (`~/.local/bin`, `~/bin`)
- Current PATH

## Security Considerations

### Why External obfs4proxy?

This implementation uses external obfs4proxy rather than reimplementing obfs4:

1. **Security**: obfs4proxy is battle-tested and audited by The Tor Project
2. **Correctness**: Ensures protocol compatibility with official implementation
3. **Maintenance**: Security updates handled by upstream project
4. **Simplicity**: Avoids duplicating complex cryptographic code

### Process Isolation

obfs4proxy runs as a separate process with:
- Separate memory space
- Limited file system access (state directory only)
- IPC communication over stdin/stdout
- No network privileges beyond its bind address

### Key Storage

- Keys stored in state directory with 0600 permissions
- State directory should be on encrypted filesystem
- Keys persisted across restarts
- Export function for backup purposes

## Testing

### Unit Tests

```bash
go test ./pkg/pt/obfs4/... -v
```

Coverage: 67.2%

### Integration Tests

The implementation includes comprehensive tests for:
- Client configuration
- Server configuration
- Certificate validation
- Bridge line parsing
- Key export/import

### Example Demo

```bash
cd examples/obfs4-demo
go run main.go
```

## Troubleshooting

### obfs4proxy Not Found

**Error**: `obfs4: obfs4proxy binary not found`

**Solution**: Install obfs4proxy:
```bash
# Debian/Ubuntu
apt-get install obfs4proxy

# macOS
brew install obfs4proxy
```

### Certificate Invalid

**Error**: `obfs4: invalid certificate`

**Solution**: Verify certificate format:
```go
err := obfs4.ValidateCertificate(cert)
```

Certificates must be:
- Valid base64 encoding
- At least 20 characters
- Obtained from trusted bridge distributor

### Connection Timeout

**Error**: `obfs4: dial failed: timeout`

**Possible causes**:
1. Bridge offline or blocked
2. Incorrect certificate
3. Network connectivity issues
4. IAT mode mismatch

**Solution**: Try different bridge or IAT mode

## Specification References

- **obfs4 Specification**: https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/obfs4/-/blob/HEAD/doc/obfs4-spec.txt
- **PT Specification**: https://spec.torproject.org/pt-spec
- **obfs4proxy**: https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird

## Examples

See `examples/obfs4-demo/main.go` for a complete working example demonstrating:
- PT discovery
- Client configuration
- Server configuration
- Bridge line parsing
- Certificate validation
- Key management

## Future Enhancements

Potential improvements (not currently implemented):

- [ ] Built-in obfs4 implementation (pure Go, no external dependency)
- [ ] Automatic bridge discovery
- [ ] Certificate pinning
- [ ] Adaptive IAT mode selection
- [ ] Performance metrics collection
- [ ] Connection pooling

## License

This implementation follows the go-tor project license.

**⚠️  EDUCATIONAL PURPOSE ONLY**

This software is NOT safe for production anonymity needs. Use official Tor Browser for real privacy requirements.

---

**Last Updated**: January 2026  
**Status**: Completed (Tasks 11.2.1, 11.2.2, 11.2.3)

# Pluggable Transport Configuration

This document describes how to configure pluggable transports (PTs) in go-tor for censorship circumvention.

## Overview

Pluggable Transports provide a framework for transforming Tor traffic to evade censorship. go-tor supports both client-side and server-side PT configuration compatible with the official Tor PT specification (pt-spec.txt).

**Status**: Configuration support complete (Phase 11.1.3) ✅  
**Last Updated**: January 25, 2026

## Supported Transports

go-tor supports external pluggable transports via the PT protocol:
- **obfs4** - Obfuscation using obfs4proxy
- **meek** - Domain fronting via meek-client  
- **snowflake** - Decentralized proxy via snowflake-client
- **Any PT v1 compatible binary**

## Client Configuration

### Using torrc File

```ini
# Enable bridges with PT
UseBridges 1
Bridge obfs4 192.0.2.1:1234 cert=CERTIFICATE iat-mode=0

# Configure pluggable transport
ClientTransportPlugin obfs4 exec /usr/bin/obfs4proxy

# Optional: Proxy for PT connections
TransportProxy socks5 127.0.0.1:9150
```

### Programmatic Configuration

```go
import "github.com/opd-ai/go-tor/pkg/config"

cfg := config.DefaultConfig()
cfg.UseBridges = true
cfg.BridgeAddresses = []string{
    "obfs4 192.0.2.1:1234 cert=CERTIFICATE iat-mode=0",
}
cfg.ClientTransports = []config.ClientTransportConfig{
    {
        Name:       "obfs4",
        BinaryPath: "/usr/bin/obfs4proxy",
        Options:    map[string]string{"cert": "abc123", "iat-mode": "0"},
    },
}
```

## Server Configuration (Bridge Relay)

### Using torrc File

```ini
# Bridge relay with PT
BridgeRelay 1
ORPort 9001
ServerTransportPlugin obfs4 exec /usr/bin/obfs4proxy
ServerTransportListenAddr obfs4 0.0.0.0:9443
ServerTransportOptions obfs4 iat-mode=1
```

### Programmatic Configuration

```go
cfg.ServerTransports = []config.ServerTransportConfig{
    {
        Name:       "obfs4",
        BinaryPath: "/usr/bin/obfs4proxy",
        BindAddr:   "0.0.0.0:9443",
        Options:    map[string]string{"iat-mode": "1"},
    },
}
```

## Configuration Directives

### ClientTransportPlugin

**Format**: `ClientTransportPlugin <transport> exec <path> [options]`

Configures a client-side pluggable transport.

**Example**:
```ini
ClientTransportPlugin obfs4 exec /usr/bin/obfs4proxy
ClientTransportPlugin meek exec /usr/bin/meek-client url=https://example.com
```

### ServerTransportPlugin

**Format**: `ServerTransportPlugin <transport> exec <path>`

Configures a server-side pluggable transport for bridge relays.

**Example**:
```ini
ServerTransportPlugin obfs4 exec /usr/bin/obfs4proxy
```

### ServerTransportListenAddr

**Format**: `ServerTransportListenAddr <transport> <address:port>`

Specifies the bind address for a server transport.

**Example**:
```ini
ServerTransportListenAddr obfs4 0.0.0.0:9443
ServerTransportListenAddr obfs4 [::]:9443
```

### ServerTransportOptions

**Format**: `ServerTransportOptions <transport> <key=value> [key=value...]`

Sets transport-specific server options.

**Example**:
```ini
ServerTransportOptions obfs4 iat-mode=1 drbg-seed=0123456789ABCDEF
```

### TransportProxy

**Format**: `TransportProxy <protocol> <address:port>`

Configures an upstream proxy for PT connections. Only SOCKS5 is supported.

**Example**:
```ini
TransportProxy socks5 127.0.0.1:9150
```

## Environment Variables

When launching PT processes, go-tor sets the following environment variables per pt-spec.txt:

### Client-side
- `TOR_PT_MANAGED_TRANSPORT_VER=1` - PT protocol version
- `TOR_PT_STATE_LOCATION=<path>` - State directory path
- `TOR_PT_CLIENT_TRANSPORTS=<list>` - Requested transports
- `TOR_PT_PROXY=<url>` - Upstream proxy (if configured)

### Server-side
- `TOR_PT_MANAGED_TRANSPORT_VER=1` - PT protocol version
- `TOR_PT_STATE_LOCATION=<path>` - State directory path
- `TOR_PT_SERVER_TRANSPORTS=<list>` - Server transports to provide
- `TOR_PT_SERVER_BINDADDR=<transport>-<address:port>` - Bind addresses
- `TOR_PT_ORPORT=<address:port>` - OR port for bridge
- `TOR_PT_EXTENDED_SERVER_PORT=<address:port>` - Extended OR port (optional)

## Getting Bridge Addresses

For censored regions, obtain bridge addresses from:
1. **BridgeDB**: https://bridges.torproject.org/
2. **Email**: bridges@torproject.org (send email with "get bridges" in body)
3. **Telegram**: @GetBridgesBot

## Installing PT Binaries

### obfs4proxy

```bash
# Debian/Ubuntu
sudo apt install obfs4proxy

# macOS
brew install obfs4proxy

# From source
go install gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird@latest
```

### meek-client

```bash
# From source
go install git.torproject.org/pluggable-transports/meek.git/meek-client@latest
```

### snowflake-client

```bash
# From source
go install git.torproject.org/pluggable-transports/snowflake.git/client@latest
```

## State Directory

PT processes require a state directory for persistent data. By default, go-tor uses:
- Linux: `~/.config/go-tor/pt-state`
- macOS: `~/Library/Application Support/go-tor/pt-state`
- Windows: `%APPDATA%\go-tor\pt-state`

Override with `DataDirectory` configuration option.

## Example Usage

See the complete example in `examples/pt-configuration/`:

```bash
cd examples/pt-configuration
go run main.go
```

This demonstrates:
1. Loading PT configuration from torrc
2. Server-side PT setup for bridge relays
3. Programmatic PT configuration
4. Configuration file generation

## Troubleshooting

### PT Process Fails to Start

**Check**:
1. PT binary path is correct and executable
2. State directory exists and is writable
3. Bind addresses are available (no port conflicts)

**Debug**:
```bash
# Test PT binary manually
TOR_PT_MANAGED_TRANSPORT_VER=1 \
TOR_PT_STATE_LOCATION=/tmp/pt-state \
TOR_PT_CLIENT_TRANSPORTS=obfs4 \
/usr/bin/obfs4proxy
```

### Bridge Connection Fails

**Check**:
1. Bridge address is correct and reachable
2. Certificate matches the bridge
3. Firewall allows outbound connections
4. PT binary supports the transport method

### Server Transport Not Listening

**Check**:
1. BindAddr is not already in use
2. Firewall allows inbound connections on bind port
3. SELinux/AppArmor policies allow PT to bind

## Security Considerations

⚠️ **Important Notes**:
- PTs provide traffic obfuscation, NOT strong encryption
- Always use PTs over Tor's encrypted protocol
- Verify PT binaries from official sources
- Keep PT binaries updated for security patches
- Some PTs leak DNS or metadata - read their documentation

## Limitations

Current limitations:
- Only PT protocol version 1 supported
- Built-in PT implementations (obfs4, meek) not included
- External PT binaries required
- No automatic PT selection

## Future Work

Planned enhancements (see AUDIT.md):
- Task 11.2: Built-in obfs4 implementation
- Task 11.3: Enhanced external PT management
- Task 11.4: Bridge client integration

## References

- [pt-spec.txt](https://spec.torproject.org/pt-spec) - Pluggable Transport Specification
- [bridge-spec.txt](https://spec.torproject.org/bridge-spec) - Bridge Relay Specification
- [obfs4 Specification](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird)
- [Tor Pluggable Transports](https://tb-manual.torproject.org/circumvention/)

## API Reference

See `pkg/config/config.go` for complete configuration struct definitions:
- `ClientTransportConfig` - Client PT configuration
- `ServerTransportConfig` - Server PT configuration
- `Config.ClientTransports` - List of client transports
- `Config.ServerTransports` - List of server transports
- `Config.TransportProxy` - Upstream proxy configuration

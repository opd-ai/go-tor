# Bridge Relay Setup and Operation

This document provides a comprehensive guide to setting up and operating a bridge relay using go-tor.

## ⚠️ Important Notice

This is an **unofficial, experimental implementation** developed for educational and research purposes. This software has been developed **without the supervision or endorsement of The Tor Project**.

**This software should NOT be used as a production bridge relay.**

For running actual Tor bridges:
- Use the official [tor](https://gitlab.torproject.org/tpo/core/tor) implementation (C)
- Or use [Arti](https://gitlab.torproject.org/tpo/core/arti) (official Rust implementation)

## Overview

A **bridge relay** (or simply "bridge") is a Tor relay that is not listed in the public Tor directory. Bridges help censored users connect to the Tor network when direct access to public Tor relays is blocked.

### What is a Bridge?

Bridges serve two main purposes:

1. **Censorship Circumvention**: Provide unlisted entry points to the Tor network
2. **Network Entry**: Act as the first hop in a circuit for users in censored regions

### What Bridges DO NOT Do

⚠️ **Important Limitations**:

- **NOT an exit relay**: Bridges never forward traffic to the public internet (reject-all exit policy)
- **NOT publicly listed**: Bridges are not published in the main Tor consensus
- **NOT high-bandwidth relays**: Bridges typically serve fewer users than public relays
- **NOT a full relay**: Bridges only relay traffic, they don't participate in directory operations

## Implementation Status

The go-tor bridge relay implementation includes:

### ✅ Completed Features

- **OR Protocol Server** (Phase 10.1)
  - TLS server setup with proper cipher suites
  - Link protocol server (VERSIONS, CERTS, NETINFO)
  - Server-side circuit handling (CREATE2, CREATED2, DESTROY)
  
- **Non-Exit Relay Functionality** (Phase 10.2)
  - Circuit extension handling (RELAY_EXTEND2, RELAY_EXTENDED2)
  - Cell forwarding between circuits
  - Reject-all exit policy enforcement
  
- **Bridge Descriptor Publishing** (Phase 10.3)
  - Server descriptor generation per dir-spec.txt §2.1
  - Extra-info descriptor with statistics
  - Bridge authority communication via HTTP POST
  - Automatic descriptor refresh (18h default)
  
- **Security Hardening** (Phase 10.4)
  - Rate limiting (circuits, connections, cells)
  - DoS protection (connection/circuit limits)
  - Comprehensive metrics and monitoring

### ❌ Not Implemented

- **BridgeDB Integration**: Bridge distribution mechanisms (optional, Phase 10.3.3)
- **Pluggable Transports**: obfs4, Snowflake integration (Phase 11)

## Quick Start

### 1. Generate Relay Keys

```go
package main

import (
	"log"
	"github.com/opd-ai/go-tor/pkg/relay"
)

func main() {
	// Generate relay identity keys
	keys, err := relay.GenerateRelayKeys()
	if err != nil {
		log.Fatal(err)
	}
	defer keys.Destroy() // Securely zero keys when done
	
	// Save keys to disk for persistence
	err = keys.SaveKeys("/var/lib/tor/keys")
	if err != nil {
		log.Fatal(err)
	}
	
	log.Printf("RSA Fingerprint: %s", keys.Fingerprint())
	log.Printf("Ed25519 Fingerprint: %s", keys.Ed25519Fingerprint())
}
```

### 2. Create and Publish Bridge Descriptor

```go
package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"github.com/opd-ai/go-tor/pkg/relay"
	"github.com/opd-ai/go-tor/pkg/logger"
)

func main() {
	// Load relay keys
	keys, err := relay.LoadKeys("/var/lib/tor/keys")
	if err != nil {
		log.Fatal(err)
	}
	defer keys.Destroy()
	
	// Create descriptor configuration
	config := &relay.DescriptorConfig{
		Nickname:       "MyBridge",
		Address:        "203.0.113.42", // Your public IP
		ORPort:         9001,
		DirPort:        0, // Always 0 for bridges
		Contact:        "operator@example.com",
		BandwidthAvg:   1024 * 1024,     // 1 MB/s
		BandwidthBurst: 2 * 1024 * 1024, // 2 MB/s
		IsBridge:       true,
	}
	
	// Generate server descriptor
	desc, err := relay.GenerateServerDescriptor(keys, config)
	if err != nil {
		log.Fatal(err)
	}
	
	// Create publisher
	log := logger.New(slog.LevelInfo, os.Stdout)
	publisher := relay.NewDescriptorPublisher(relay.DefaultPublisherConfig(), log)
	
	// Publish descriptor to bridge authority
	ctx := context.Background()
	successCount, err := publisher.PublishDescriptor(ctx, desc)
	if err != nil {
		log.Fatal(err)
	}
	
	log.Printf("Descriptor published to %d authorities", successCount)
}
```

### 3. Start Bridge Relay Server

```go
package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
	
	"github.com/opd-ai/go-tor/pkg/relay"
	"github.com/opd-ai/go-tor/pkg/logger"
)

func main() {
	// Load relay keys
	keys, err := relay.LoadKeys("/var/lib/tor/keys")
	if err != nil {
		log.Fatal(err)
	}
	defer keys.Destroy()
	
	// Create logger
	log := logger.New(slog.LevelInfo, os.Stdout)
	
	// Create OR listener
	cfg := relay.DefaultORListenerConfig(":9001", keys)
	cfg.MaxConnections = 1000
	cfg.ReadTimeout = 60 * time.Second
	cfg.WriteTimeout = 60 * time.Second
	
	listener, err := relay.NewORListener(cfg, log)
	if err != nil {
		log.Fatal(err)
	}
	
	// Start listener
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	if err := listener.Start(ctx); err != nil {
		log.Fatal(err)
	}
	
	log.Println("Bridge relay started on :9001")
	
	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	
	log.Println("Shutting down...")
	listener.Stop()
}
```

## Configuration

### Relay Keys (`RelayKeys`)

Bridge relays require three cryptographic keys:

| Key Type | Size | Purpose |
|----------|------|---------|
| **Ed25519 Identity** | 32 bytes (public), 64 bytes (private) | Primary relay identity |
| **RSA Identity** | 1024 bits | Legacy compatibility, fingerprints |
| **Ntor Onion Key** | 32 bytes (Curve25519) | Circuit creation handshakes |

**Key Persistence**:
- Keys are stored in separate files with secure permissions (0600)
- Use `SaveKeys()` and `LoadKeys()` for atomic file operations
- Always call `Destroy()` to securely zero keys from memory

### OR Listener Configuration

```go
type ORListenerConfig struct {
	Address        string           // Listen address (e.g., ":9001")
	Keys           *RelayKeys       // Relay cryptographic keys
	MaxConnections int              // Max concurrent connections (default: 100)
	ReadTimeout    time.Duration    // Connection read timeout (default: 60s)
	WriteTimeout   time.Duration    // Connection write timeout (default: 60s)
}
```

**Recommended Settings**:
- `MaxConnections`: 100-1000 depending on available resources
- `ReadTimeout`/`WriteTimeout`: 60s (Tor default)
- `Address`: Use `:9001` or `:443` (port 443 helps circumvent some firewalls)

### Descriptor Configuration

```go
type DescriptorConfig struct {
	Nickname       string   // Relay nickname (1-19 alphanumeric, required)
	Address        string   // IPv4 address (required)
	ORPort         uint16   // OR port (required)
	DirPort        uint16   // Directory port (0 for bridges)
	Contact        string   // Contact email (optional but recommended)
	Family         []string // Relay family members (optional)
	BandwidthAvg   uint64   // Average bandwidth in bytes/sec
	BandwidthBurst uint64   // Burst bandwidth in bytes/sec
	IPv6Addr       string   // IPv6 address:port (optional, e.g., "[2001:db8::1]:9001")
	IsBridge       bool     // Must be true for bridges
}
```

**Bridge-Specific Requirements**:
- `DirPort` must be **0** (bridges don't serve directory info)
- `IsBridge` must be **true**
- `Nickname` should be unique and memorable
- `Contact` helps bridge authority operators reach you

### Publisher Configuration

```go
type PublisherConfig struct {
	Authorities     []BridgeAuthority // Bridge authorities to publish to
	PublishInterval time.Duration     // Interval between publishes (default: 18h)
	HTTPTimeout     time.Duration     // HTTP request timeout (default: 30s)
	RetryAttempts   int               // Retry attempts per authority (default: 3)
	RetryDelay      time.Duration     // Initial retry delay (default: 5s)
	MaxRetryDelay   time.Duration     // Maximum retry delay (default: 60s)
}
```

**Default Bridge Authority**:
```go
var DefaultBridgeAuthorities = []BridgeAuthority{
	{
		Address: "86.59.21.38:80",
		URL:     "http://86.59.21.38/tor/",
	},
}
```

## Security Features

### Rate Limiting

The bridge relay implements token bucket rate limiting for DoS protection:

```go
cfg := &relay.RateLimiterConfig{
	CircuitRate:     10.0,  // circuits per second
	CircuitBurst:    20,    // max burst size
	ConnectionRate:  5.0,   // connections per second per IP
	ConnectionBurst: 10,
	CellRate:        100.0, // cells per second per circuit
	CellBurst:       200,
	CleanupInterval: 5 * time.Minute,
}

rateLimiter := relay.NewRateLimiter(cfg)
```

**Features**:
- Circuit creation rate limiting (prevents rapid circuit creation attacks)
- Per-IP connection rate limiting (prevents connection flooding)
- Per-circuit cell processing rate limiting (prevents cell flooding)
- Automatic cleanup of stale limiters

### DoS Protection

```go
cfg := &relay.ProtectionConfig{
	MaxConnectionsPerIP:   10,   // Max connections from single IP
	MaxCircuitsPerConn:    1000, // Max circuits per connection
	MaxTotalConnections:   5000, // Global connection limit
	CleanupInterval:       5 * time.Minute,
}

protection := relay.NewProtectionManager(cfg)
```

**Features**:
- Per-IP connection counting
- Per-connection circuit counting
- Global connection limits
- Automatic cleanup and metrics tracking

### Exit Policy

All bridge relays in go-tor enforce a **reject-all exit policy**:

```go
log := logger.New(slog.LevelInfo, os.Stdout)
policy := relay.NewExitPolicy(log)
// policy.String() returns "reject *:*"

// Any RELAY_BEGIN attempts are rejected with EXITPOLICY reason
err := policy.CheckExit(relay.RelayCommandBegin)
// Returns: ExitPolicyViolation error
```

**Why Reject-All?**:
- Bridges are **not exit relays** by design
- Reduces legal liability for bridge operators
- Focuses resources on censorship circumvention (entry) not exit traffic

## Monitoring and Metrics

### Relay Metrics

The relay package provides comprehensive metrics:

```go
metrics := relay.NewRelayMetrics()

// Circuit metrics
metrics.CircuitsCreated.Inc()
metrics.CircuitsExtended.Inc()
metrics.ActiveCircuits.Set(42)

// Connection metrics
metrics.ConnectionsAccepted.Inc()
metrics.ActiveConnections.Set(15)

// Bandwidth metrics
metrics.BytesReceived.Add(1024)
metrics.BytesTransmitted.Add(2048)

// Error metrics
metrics.HandshakeErrors.Inc()
metrics.ProtocolErrors.Inc()

// Get snapshot
snapshot := metrics.Snapshot()
log.Printf("Active circuits: %d", snapshot.ActiveCircuits)
log.Printf("Uptime: %s", snapshot.Uptime)
```

### Available Metrics

| Metric Category | Metrics |
|----------------|---------|
| **Circuits** | `CircuitsCreated`, `CircuitsExtended`, `CircuitsDestroyed`, `ActiveCircuits` |
| **Connections** | `ConnectionsAccepted`, `ConnectionsRejected`, `ActiveConnections`, `ConnectionDuration` |
| **Cells** | `CellsReceived`, `CellsForwarded`, `CellsDropped` |
| **Bandwidth** | `BytesReceived`, `BytesTransmitted` |
| **Rate Limiting** | `RateLimitedCircuits`, `RateLimitedConnections`, `RateLimitedCells` |
| **DoS Protection** | `DoSConnectionsBlocked`, `DoSCircuitsBlocked` |
| **Errors** | `HandshakeErrors`, `ProtocolErrors`, `ExtensionErrors` |

## Operational Considerations

### Bandwidth Requirements

**Minimum Recommended**:
- **Bandwidth**: 1 Mbps sustained (128 KB/s)
- **Monthly Transfer**: ~330 GB/month
- **Memory**: 256 MB RAM
- **CPU**: 1 core @ 1 GHz

**Optimal**:
- **Bandwidth**: 10 Mbps sustained (1.25 MB/s)
- **Monthly Transfer**: 3+ TB/month
- **Memory**: 1 GB RAM
- **CPU**: 2 cores @ 2 GHz

### Network Configuration

**Firewall Rules**:
```bash
# Allow incoming connections on OR port
iptables -A INPUT -p tcp --dport 9001 -j ACCEPT

# For port 443 (helps with some firewalls)
iptables -A INPUT -p tcp --dport 443 -j ACCEPT
```

**Port Forwarding** (if behind NAT):
- Forward external port to internal OR port
- Use static internal IP or DHCP reservation
- Ensure router doesn't reset connections

**Recommended Ports**:
- **9001**: Standard OR port
- **443**: HTTPS port (helps circumvent port-based blocking)
- **9030**: Alternative (if 9001 blocked)

### Uptime and Reliability

**Best Practices**:
- Maintain **95%+ uptime** (bridges are more useful when reliable)
- Use systemd or supervisor for automatic restart
- Monitor logs for errors and warnings
- Keep system and dependencies updated

**Systemd Service Example**:
```ini
[Unit]
Description=Go-Tor Bridge Relay
After=network.target

[Service]
Type=simple
User=tor
Group=tor
ExecStart=/usr/local/bin/go-tor-bridge
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

### Security Best Practices

1. **Isolate the Relay**:
   - Run on dedicated VM or container
   - Use separate user account (`tor` user)
   - Limit file system access

2. **Keep Software Updated**:
   - Monitor go-tor releases
   - Update Go runtime regularly
   - Patch operating system

3. **Log Management**:
   - Use `slog.LevelWarn` or `slog.LevelInfo` in production
   - Rotate logs to prevent disk exhaustion
   - Monitor for suspicious activity

4. **Key Protection**:
   - Store keys in `/var/lib/tor/keys` with 0700 permissions
   - Use encrypted disk for key storage
   - Backup keys securely (offline, encrypted)

## Troubleshooting

### Common Issues

#### Descriptor Publishing Fails

**Symptom**: `Publish()` returns error

**Solutions**:
1. Check network connectivity to bridge authority
2. Verify descriptor is valid (`ValidateDescriptor()`)
3. Ensure IP address matches actual public IP
4. Check firewall/NAT configuration

#### No Incoming Connections

**Symptom**: `ConnectionsAccepted` metric stays at 0

**Solutions**:
1. Verify port forwarding (if behind NAT)
2. Check firewall rules allow incoming connections
3. Ensure descriptor was published successfully
4. Wait 1-2 hours for bridge to be distributed

#### High Memory Usage

**Symptom**: Memory usage grows over time

**Solutions**:
1. Enable rate limiting and DoS protection
2. Reduce `MaxConnections` and `MaxCircuitsPerConn`
3. Check for connection leaks (monitor `ActiveConnections`)
4. Ensure cleanup intervals are appropriate (5 minutes default)

#### Protocol Errors

**Symptom**: `ProtocolErrors` metric increasing

**Solutions**:
1. Check logs for specific error messages
2. Verify TLS certificate is valid
3. Ensure relay keys match descriptor
4. Update to latest go-tor version

### Debug Logging

Enable verbose logging for troubleshooting:

```go
log := logger.New(slog.LevelDebug, os.Stdout)
```

**Key Log Messages**:
- `"OR listener started"` - Server listening successfully
- `"Connection accepted"` - Incoming connection established
- `"Circuit created"` - New circuit created
- `"Descriptor published"` - Descriptor uploaded successfully
- `"Rate limit exceeded"` - Rate limiting active (may need adjustment)

## Complete Example

See `examples/bridge-descriptor/` for a complete working example:

```bash
cd examples/bridge-descriptor
go run main.go
```

This example demonstrates:
- Relay key generation and persistence
- Server descriptor creation
- Bridge authority publishing
- OR listener setup
- Graceful shutdown handling

## Additional Resources

### Documentation
- [RELAY_IMPLEMENTATION.md](RELAY_IMPLEMENTATION.md) - Technical implementation details
- [RELAY_SECURITY.md](RELAY_SECURITY.md) - Security hardening features
- [CIRCUIT_EXTENSION.md](CIRCUIT_EXTENSION.md) - Circuit extension protocol
- [LINK_PROTOCOL_SERVER.md](LINK_PROTOCOL_SERVER.md) - Server-side link protocol

### Tor Specifications
- [tor-spec.txt](https://spec.torproject.org/tor-spec) - Core Tor protocol
- [dir-spec.txt](https://spec.torproject.org/dir-spec) - Directory protocol and descriptors
- [bridge-spec.txt](https://spec.torproject.org/bridge-spec) - Bridge relay specification

### Official Tor Resources
- [Tor Project Bridges](https://community.torproject.org/relay/types-of-relays/#bridge) - Bridge relay overview
- [Bridge Operator Guide](https://community.torproject.org/relay/setup/bridge/) - Official setup guide
- [Tor Metrics](https://metrics.torproject.org/) - Network statistics

## License

This implementation is provided for **educational and research purposes only**. See LICENSE file for details.

---

**Last Updated**: January 25, 2026  
**Status**: Complete (Phase 10.1-10.4)

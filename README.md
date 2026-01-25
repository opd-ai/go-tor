# go-tor

A pure Go implementation of the Tor protocol providing client functionality and bridge relay capabilities for educational and research purposes.

## ⚠️ IMPORTANT SAFETY NOTICE

**THIS IS UNOFFICIAL SOFTWARE** that has been developed without the supervision or endorsement of [The Tor Project](https://www.torproject.org/). This software should **NOT** be considered safe or production-ready.

**For actual privacy and anonymity needs, please use official Tor software:**

- **For users**: Use [Tor Browser](https://www.torproject.org/download/) - the only recommended way to safely browse anonymously
- **For developers**: Use [Arti](https://gitlab.torproject.org/tpo/core/arti) - the official Tor implementation in Rust, or the [reference C implementation](https://github.com/torproject/tor)

**Do not rely on this software for:**
- Personal safety or anonymity
- Protection from surveillance
- Accessing sensitive information
- Any production use case
- Any situation where your safety depends on anonymity

This project is an **experimental implementation** for learning and research purposes only.

---

## Description

go-tor is a cross-platform Tor client written entirely in Go without CGo dependencies. The package implements the core Tor protocol specifications including circuit management, cryptographic operations, SOCKS5 proxy serving, and v3 onion service support. This implementation prioritizes portability and embedded optimization while maintaining client feature parity with the reference C implementation.

---

## Installation

### Prerequisites

- Go 1.24 or later
- Git

### Build from Source

```bash
git clone https://github.com/opd-ai/go-tor.git
cd go-tor
make build
```

The compiled binary will be available at `bin/tor-client`.

### Docker Installation

```bash
# Build Docker image
docker build -t go-tor:latest .

# Run with SOCKS proxy on port 9050
docker run -d --name tor-client -p 9050:9050 go-tor:latest
```

---

## Usage

### Command-Line Client

Zero-configuration mode automatically detects settings and starts the Tor client:

```bash
# Run with default settings (SOCKS on port 9050)
./bin/tor-client

# Specify custom SOCKS port
./bin/tor-client -socks-port 9150

# Use configuration file
./bin/tor-client -config /etc/tor/torrc

# Enable HTTP metrics endpoint
./bin/tor-client -metrics-port 9052
```

First connection requires 60-90 seconds for consensus download and circuit building. Subsequent starts are faster.

### Library Integration

Simplest integration using zero-configuration API:

```go
package main

import (
    "time"
    "github.com/opd-ai/go-tor/pkg/client"
)

func main() {
    // Connect to Tor network (auto-configured)
    torClient, err := client.Connect()
    if err != nil {
        panic(err)
    }
    defer torClient.Close()
    
    // Wait for circuit establishment
    if err := torClient.WaitUntilReady(90 * time.Second); err != nil {
        panic(err)
    }
    
    // Get SOCKS5 proxy URL
    proxyURL := torClient.ProxyURL()  // "socks5://127.0.0.1:9050"
    _ = proxyURL // avoid unused variable error in this minimal example
    
    // Configure your HTTP client to use proxyURL
}
```

### Advanced Configuration

Custom configuration with control protocol and metrics:

```go
package main

import (
    "context"
    "time"
    "github.com/opd-ai/go-tor/pkg/client"
    "github.com/opd-ai/go-tor/pkg/config"
    "github.com/opd-ai/go-tor/pkg/logger"
)

func main() {
    cfg := config.DefaultConfig()
    cfg.SocksPort = 9150
    cfg.ControlPort = 9151
    cfg.CircuitBuildTimeout = 90 * time.Second
    cfg.EnableMetrics = true
    
    log := logger.NewDefault()
    torClient, err := client.New(cfg, log)
    if err != nil {
        panic(err)
    }
    
    err = torClient.Start(context.Background())
    if err != nil {
        panic(err)
    }
    defer torClient.Close()
}
```

### HTTP Client Helper

Streamlined HTTP requests through Tor:

```go
package main

import (
    "time"
    "github.com/opd-ai/go-tor/pkg/client"
    "github.com/opd-ai/go-tor/pkg/helpers"
)

func main() {
    torClient, err := client.Connect()
    if err != nil {
        panic(err)
    }
    defer torClient.Close()
    
    if err := torClient.WaitUntilReady(90 * time.Second); err != nil {
        panic(err)
    }
    
    // Create HTTP client configured for Tor
    httpClient, err := helpers.NewHTTPClient(torClient, nil)
    if err != nil {
        panic(err)
    }
    
    // Make requests through Tor network
    resp, err := httpClient.Get("https://check.torproject.org")
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()
}
```

---

## Features

- **Cell Protocol** - Fixed and variable-size cell encoding/decoding with relay cell support
- **Circuit Management** - Complete circuit lifecycle including creation, extension, and teardown
- **Cryptography** - AES-CTR, RSA-1024, SHA-1/256, Ed25519, and KDF-TOR key derivation
- **Directory Protocol** - Consensus fetching and relay descriptor parsing
- **Path Selection** - Guard, middle, and exit node selection with guard persistence
- **SOCKS5 Proxy** - RFC 1928 compliant proxy server for application integration
- **Stream Multiplexing** - Multiple data streams over single circuits
- **Control Protocol** - Password-authenticated control interface with event notifications
- **Onion Services** - v3 onion address support including client connections and service hosting
- **Metrics & Observability** - Prometheus, JSON, and HTML dashboard endpoints
- **Health Monitoring** - Component-level health checks and status reporting
- **Resource Pooling** - Circuit and connection pooling for performance optimization

---

## Requirements

### System Requirements

- Linux, macOS, Windows, or BSD operating system
- Network connectivity to Tor directory authorities
- 50MB+ available RAM

### Go Dependencies

Key dependencies from `go.mod`:

- `go.opentelemetry.io/otel` - Distributed tracing and observability
- `golang.org/x/crypto` - Cryptographic primitives
- `golang.org/x/net` - Network protocol utilities
- `github.com/gofrs/flock` - File locking for guard persistence
- `github.com/cretz/bine` - Tor controller library

---

## License

BSD 3-Clause License

Copyright (c) 2024, OPD AI. See [LICENSE](LICENSE) for full license text.

---

## References

- [Tor Protocol Specifications](https://spec.torproject.org/)
- [The Tor Project](https://www.torproject.org/)
- [Architecture Documentation](docs/ARCHITECTURE.md)
- [API Reference](docs/API.md)
- [Examples Directory](examples/)

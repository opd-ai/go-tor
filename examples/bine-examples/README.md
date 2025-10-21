# Bine Examples - Integration with go-tor

This directory contains examples demonstrating how to use [`cretz/bine`](https://github.com/cretz/bine) together with `go-tor` for various Tor network operations.

## Overview

[`cretz/bine`](https://github.com/cretz/bine) is a Go library for accessing and embedding Tor clients and servers. It provides a high-level API for:
- Creating and managing Tor processes
- Creating onion services (hidden services)
- Making connections through Tor
- Controlling Tor via the control protocol

These examples show how to integrate `cretz/bine` with `go-tor` to leverage the strengths of both libraries.

## Examples

### 1. Client Example (`client-example/`)
Demonstrates how to use `cretz/bine` to create a Tor client that connects through `go-tor`'s SOCKS5 proxy. This example shows:
- Starting a go-tor client
- Configuring bine to use go-tor's SOCKS proxy
- Making HTTP requests through the combined setup
- Graceful shutdown

**Use case**: When you want to use bine's convenient API with go-tor's pure-Go Tor implementation.

### 2. Hidden Service Example (`hidden-service-example/`)
Demonstrates how to create a v3 onion service (hidden service) using `cretz/bine` while leveraging `go-tor` infrastructure. This example shows:
- Starting go-tor for Tor network connectivity
- Creating a v3 onion service with bine
- Serving HTTP content over the onion service
- Publishing and accessing the service

**Use case**: When you want to host a hidden service using bine's API with go-tor's networking capabilities.

## Prerequisites

- Go 1.24 or later
- Tor binary installed (for cretz/bine to launch)
  - On Ubuntu/Debian: `sudo apt-get install tor`
  - On macOS: `brew install tor`
  - On Windows: Download from [torproject.org](https://www.torproject.org/download/)

## Quick Start

### Running the Client Example

```bash
cd examples/bine-examples/client-example
go run main.go
```

Expected output:
```
Starting go-tor client...
go-tor client ready on socks5://127.0.0.1:9050
Starting bine Tor client...
Tor started successfully
Making request to check.torproject.org...
✓ Connected through Tor!
Response status: 200 OK
```

### Running the Hidden Service Example

```bash
cd examples/bine-examples/hidden-service-example
go run main.go
```

Expected output:
```
Starting go-tor for network connectivity...
go-tor ready
Starting bine Tor instance for hidden service...
Creating onion service...
✓ Onion service created: http://abc123xyz456.onion
Starting HTTP server on the onion service...
Service is now accessible at http://abc123xyz456.onion
Press Ctrl+C to stop...
```

## Architecture

### Integration Pattern

```
┌─────────────────────────────────────────────────────────┐
│                    Your Application                      │
│                                                           │
│  ┌────────────────┐              ┌──────────────────┐  │
│  │  cretz/bine    │              │     go-tor       │  │
│  │  - High-level  │─────uses────▶│  - Pure Go Tor   │  │
│  │    API         │              │  - SOCKS proxy   │  │
│  │  - Process     │              │  - Circuit mgmt  │  │
│  │    control     │              │                  │  │
│  └────────────────┘              └──────────────────┘  │
│                                                           │
└─────────────────────────────────────────────────────────┘
                         │
                         ▼
                  ┌──────────────┐
                  │  Tor Network │
                  └──────────────┘
```

### Why Use Both?

**go-tor advantages:**
- Pure Go implementation (no CGo, no external Tor binary needed)
- Easy to cross-compile
- Small binary size
- Full control over Tor implementation
- Embedded into your application

**cretz/bine advantages:**
- High-level, convenient API
- Well-tested and mature
- Complete Tor control protocol support
- Easier onion service management
- Support for embedded Tor (statically linked)

**Combined benefits:**
- Use go-tor for network connectivity (pure Go SOCKS proxy)
- Use bine for high-level operations and service management
- Get both portability and convenience

## Implementation Notes

### Client Integration

The client example demonstrates a pattern where:
1. go-tor provides the SOCKS5 proxy (pure Go, embedded)
2. bine's HTTP client uses go-tor's SOCKS proxy
3. No external Tor process needed for basic connectivity

### Hidden Service Integration

The hidden service example shows:
1. go-tor can provide network connectivity
2. bine manages the onion service lifecycle
3. Both libraries work together seamlessly

### Performance Considerations

- **Startup time**: Using both libraries may increase initialization time
- **Memory usage**: Running both adds ~50-100MB RSS (depending on configuration)
- **Network overhead**: Minimal, as they share the underlying Tor network

### Security Considerations

- Both libraries implement Tor protocol securely
- Ensure proper key management for onion services
- Use HTTPS when possible, even over Tor
- Follow Tor best practices for anonymity

## Troubleshooting

### "Tor binary not found"
Ensure Tor is installed and in your PATH. On Linux:
```bash
which tor
# Should output: /usr/bin/tor or similar
```

### "Connection refused" on SOCKS proxy
Make sure go-tor is fully started and ready before bine attempts to use it:
```go
torClient.WaitUntilReady(90 * time.Second)
```

### "Timeout creating onion service"
Hidden service creation can take 2-3 minutes on first run. Increase timeouts:
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
```

### Port conflicts
If you see "address already in use" errors, change the ports:
```go
opts := &client.Options{
    SocksPort: 9150,  // Use different port
}
```

## Additional Resources

- [cretz/bine Documentation](https://pkg.go.dev/github.com/cretz/bine)
- [go-tor Documentation](https://github.com/opd-ai/go-tor)
- [Tor Project Specifications](https://spec.torproject.org/)
- [Tor Hidden Service Protocol](https://spec.torproject.org/rend-spec-v3)

## Contributing

Contributions are welcome! If you have additional integration patterns or improvements, please submit a pull request.

## License

These examples are provided under the same license as go-tor (BSD 3-Clause).

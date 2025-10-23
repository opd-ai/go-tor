# go-tor

[![Go Report Card](https://goreportcard.com/badge/github.com/opd-ai/go-tor)](https://goreportcard.com/report/github.com/opd-ai/go-tor)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue.svg)](LICENSE)

A production-ready Tor client implementation in pure Go, designed for embedded systems.

**⚠️ Development Status**: This project is in active development. Core functionality is being implemented according to the roadmap below. THIS IS NOT PRODUCTION READY SOFTWARE AND YOU SHOULD NOT USE IT.

## Features

### Current (Phase 1-6.5 Complete + Phase 7 Control Protocol + Phase 7.3-7.4 Onion Services + Phase 8.1-8.6)
- ✅ Cell encoding/decoding (fixed and variable-size)
- ✅ Relay cell handling
- ✅ Circuit management types and lifecycle
- ✅ Cryptographic primitives (AES-CTR, RSA, SHA-1/256)
- ✅ Key derivation (KDF-TOR)
- ✅ Configuration system with validation
- ✅ **Configuration file loading (torrc-compatible)**
- ✅ Structured logging with log/slog
- ✅ Graceful shutdown with context propagation
- ✅ TLS connection handling to Tor relays
- ✅ TLS certificate validation
- ✅ Protocol handshake and version negotiation
- ✅ Connection state management with retry logic
- ✅ Directory client (consensus fetching)
- ✅ Path selection (guard, middle, exit)
- ✅ Guard node persistence
- ✅ Circuit builder
- ✅ Circuit extension (CREATE2/CREATED2, EXTEND2/EXTENDED2)
- ✅ Stream management and multiplexing
- ✅ SOCKS5 proxy server (RFC 1928)
- ✅ Component integration and orchestration
- ✅ Functional Tor client application
- ✅ Metrics and observability system
- ✅ Control protocol server (basic commands)
- ✅ Event notification system (CIRC, STREAM, BW, ORCONN events)
- ✅ Additional event types (NEWDESC, GUARD, NS events)
- ✅ v3 onion address parsing and validation
- ✅ SOCKS5 .onion address detection
- ✅ Descriptor cache with expiration management
- ✅ Blinded public key computation (SHA3-256)
- ✅ Time period calculation for descriptor rotation
- ✅ Descriptor encoding/parsing foundation
- ✅ HSDir selection algorithm (DHT-style routing)
- ✅ Replica descriptor ID computation
- ✅ Descriptor fetching protocol foundation
- ✅ Introduction point selection algorithm
- ✅ INTRODUCE1 cell construction (Tor spec compliant)
- ✅ Introduction circuit creation foundation
- ✅ Full onion service connection orchestration
- ✅ Rendezvous point selection algorithm
- ✅ ESTABLISH_RENDEZVOUS cell construction
- ✅ Rendezvous circuit creation
- ✅ RENDEZVOUS1/RENDEZVOUS2 protocol handling
- ✅ Complete onion service connection workflow
- ✅ SOCKS5 .onion address integration
- ✅ **Onion service hosting (hidden service server)**
- ✅ **Service identity management (Ed25519 keypair generation/storage)**
- ✅ **Descriptor creation and signing**
- ✅ **Descriptor publishing to HSDirs**
- ✅ **Introduction point circuit establishment**
- ✅ **INTRODUCE2 cell handling**
- ✅ **Health monitoring API with component-level checks**
- ✅ **Structured error types with categories and severity**
- ✅ **Circuit age enforcement (MaxCircuitDirtiness)**
- ✅ **Resource pooling (buffers, connections, circuits)**
- ✅ **Circuit prebuilding for instant availability**
- ✅ **Adaptive circuit selection (pool-based and legacy modes)**
- ✅ **Performance tuning configuration options**
- ✅ **Security hardening (zero HIGH/MEDIUM severity issues)**
- ✅ **HTTP metrics endpoint (Prometheus, JSON, health, dashboard)**

### Recently Completed
- ✅ **Phase 9.12**: Test infrastructure enhancement (comprehensive integration and regression tests)
- ✅ **Phase 9.11**: Distributed tracing and observability (end-to-end operation tracking)
- ✅ **Phase 9.10**: Context propagation and cancellation (timeout control and graceful cancellation)
- ✅ **Phase 9.9**: Enhanced CLI interface and developer tooling (torctl, config validator)
- ✅ **Phase 9.8**: HTTP client helpers and developer experience (simplified HTTP integration)
- ✅ **Phase 9.7**: Command-line interface testing (CLI test suite)
- ✅ **Phase 9.6**: Race condition fix in benchmark package (thread safety)
- ✅ **Phase 9.5**: Performance benchmarking and validation
- ✅ **Phase 9.4**: Advanced circuit strategies (circuit pool integration, adaptive selection)
- ✅ **Phase 9.3**: Testing infrastructure enhancement (integration tests, stress tests, benchmarks)
- ✅ **Phase 9.2**: Onion service production integration
- ✅ **Phase 9.1**: HTTP metrics and observability (Prometheus, JSON endpoints, HTML dashboard)

### Planned
- [ ] **Phase 9**: Advanced monitoring and production features
  - [x] HTTP metrics endpoint (Phase 9.1)
  - [x] Onion service production integration (Phase 9.2)
  - [x] Testing infrastructure enhancement (Phase 9.3)
  - [x] Advanced circuit strategies (Phase 9.4)
  - [x] Performance benchmarking (Phase 9.5)
  - [x] Race condition fixes (Phase 9.6)
  - [x] CLI testing suite (Phase 9.7)
  - [x] HTTP client helpers and developer experience (Phase 9.8)
  - [x] Enhanced CLI interface and developer tooling (Phase 9.9)
  - [x] Context propagation and cancellation (Phase 9.10)
  - [x] Distributed tracing and observability (Phase 9.11)
  - [x] Comprehensive integration and regression testing (Phase 9.12)
  - [ ] Additional production features and enhancements

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for detailed architecture and roadmap.

## Design Goals

- **Pure Go**: No CGo dependencies for maximum portability
- **Client-Only**: No relay or exit node functionality
- **Embedded-Optimized**: Low memory footprint (<50MB RSS) and resource efficiency
- **Feature Parity**: Match C Tor client capabilities
- **Cross-Platform**: Support for ARM, MIPS, x86 architectures

## Quick Start

### Prerequisites

- Go 1.24 or later

### Installation

```bash
git clone https://github.com/opd-ai/go-tor.git
cd go-tor
make build
```

### Running (Zero Configuration)

The easiest way to get started - just run the binary with no arguments:

```bash
# Zero-configuration mode (auto-detects data directory and settings)
./bin/tor-client

# Custom SOCKS port
./bin/tor-client -socks-port 9150

# Custom data directory
./bin/tor-client -data-dir ~/.tor

# With configuration file
./bin/tor-client -config /etc/tor/torrc

# With HTTP metrics enabled (Prometheus, JSON, HTML dashboard)
./bin/tor-client -metrics-port 9052

# Show version
./bin/tor-client -version
```

**Note**: The client now works in **zero-configuration mode** by default. It automatically:
- Detects and creates appropriate data directories for your OS
- Selects available ports
- Connects to Tor network and builds circuits
- Starts SOCKS5 proxy without any setup

First connection takes 30-60 seconds. Subsequent starts are faster.

## Usage

### As a Library (Zero Configuration)

The simplest way to use go-tor in your application:

```go
import (
    "github.com/opd-ai/go-tor/pkg/client"
)

func main() {
    // Zero-configuration - just one function call!
    torClient, err := client.Connect()
    if err != nil {
        panic(err)
    }
    defer torClient.Close()
    
    // Get SOCKS5 proxy URL
    proxyURL := torClient.ProxyURL()  // "socks5://127.0.0.1:9050"
    
    // Wait until ready (recommended: 90s for first run, 30-60s for subsequent runs)
    torClient.WaitUntilReady(90 * time.Second)
    
    // Use with your HTTP client
    // ... configure HTTP client to use proxyURL
}
```

### As a Library (With Custom Options)

For more control over configuration:

```go
import (
    "github.com/opd-ai/go-tor/pkg/client"
    "github.com/opd-ai/go-tor/pkg/config"
)

// Option 1: Use simplified API with options
torClient, err := client.ConnectWithOptions(&client.Options{
    SocksPort:     9150,
    ControlPort:   9151,
    DataDirectory: "/custom/path",
    LogLevel:      "debug",
})

// Option 2: Use full configuration for advanced usage
cfg := config.DefaultConfig()
cfg.SocksPort = 9150
cfg.CircuitBuildTimeout = 90 * time.Second
log := logger.NewDefault()

torClient, err := client.New(cfg, log)
if err != nil {
    panic(err)
}

err = torClient.Start(context.Background())
// ... use client
```

See [examples/zero-config](examples/zero-config) for a complete working example.

### HTTP Client Integration (NEW in Phase 9.8)

The easiest way to make HTTP requests through Tor - zero boilerplate:

```go
import (
    "github.com/opd-ai/go-tor/pkg/client"
    "github.com/opd-ai/go-tor/pkg/helpers"
)

// Connect to Tor
torClient, _ := client.Connect()
defer torClient.Close()

// Wait for Tor to be ready
torClient.WaitUntilReady(90 * time.Second)

// Create HTTP client - that's it!
httpClient, _ := helpers.NewHTTPClient(torClient, nil)

// Make requests through Tor
resp, _ := httpClient.Get("https://check.torproject.org")
```

See [examples/http-helpers-demo](examples/http-helpers-demo) and [pkg/helpers/README.md](pkg/helpers/README.md) for complete documentation.


### CLI Tools (NEW in Phase 9.9)

Command-line tools for managing and monitoring go-tor clients:

#### torctl - Control Utility

Interactive control of running Tor clients:

```bash
# Show current status
torctl status

# List active circuits
torctl circuits

# List active streams  
torctl streams

# Show detailed information
torctl info

# Send control signals
torctl signal SHUTDOWN
```

#### tor-config-validator - Configuration Tool

Validate and generate Tor configurations:

```bash
# Validate configuration file
tor-config-validator -config /etc/tor/torrc -verbose

# Generate sample configuration
tor-config-validator -generate -output sample-torrc

# Generate to stdout
tor-config-validator -generate
```

See [examples/cli-tools-demo](examples/cli-tools-demo) for complete documentation and usage examples.


### As a SOCKS Proxy

Configure your application to use `localhost:9050` as a SOCKS5 proxy:

```bash
# Test with curl
curl --socks5 127.0.0.1:9050 https://check.torproject.org

# Firefox: Preferences → Network Settings → Manual proxy configuration
# HTTP Proxy: (leave empty)
# SOCKS Host: 127.0.0.1  Port: 9050  SOCKS v5
```

## Architecture

The project is organized into modular packages:

- **pkg/cell**: Tor protocol cell encoding/decoding ✅
- **pkg/circuit**: Circuit management and lifecycle ✅
- **pkg/crypto**: Cryptographic primitives ✅
- **pkg/config**: Configuration management ✅
- **pkg/connection**: TLS connection handling ✅ (Phase 2)
- **pkg/protocol**: Core Tor protocol handshake ✅ (Phase 2)
- **pkg/directory**: Directory protocol client ✅ (Phase 2)
- **pkg/path**: Path selection algorithms ✅ (Phase 3)
- **pkg/socks**: SOCKS5 proxy server ✅ (Phase 3)
- **pkg/stream**: Stream multiplexing ✅ (Phase 4)
- **pkg/client**: Client orchestration ✅ (Phase 5)
- **pkg/metrics**: Metrics and observability ✅ (Phase 6.5)
- **pkg/control**: Control protocol ✅ (Phase 7)
- **pkg/onion**: Onion service support 🚧 (Phase 7.3 - Foundation complete)
- **pkg/health**: Health monitoring and checks ✅ (Phase 8.2)
- **pkg/errors**: Structured error types ✅ (Phase 8.2)
- **pkg/pool**: Resource pooling infrastructure ✅ (Phase 8.3)
- **pkg/helpers**: HTTP client integration helpers ✅ (Phase 9.8)
- **pkg/trace**: Distributed tracing and observability ✅ (Phase 9.11)
- **pkg/pool**: Resource pooling infrastructure ✅ (Phase 8.3)

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for details.

## Development

### Building

```bash
make build        # Build binary
make test         # Run tests
make test-coverage # Run tests with coverage
make bench        # Run micro-benchmarks
make benchmark-full # Run comprehensive performance benchmarks
make build-tools  # Build all CLI tools (torctl, config-validator, benchmark)
make fmt          # Format code
make vet          # Run go vet
make lint         # Run golint
```

### Documentation

- [Architecture](docs/ARCHITECTURE.md) - System architecture and design
- [Development Guide](docs/DEVELOPMENT.md) - Development workflow and guidelines
- [Testing Guide](docs/TESTING.md) - Comprehensive testing guide (NEW in Phase 9.3)
- [Benchmarking Guide](docs/BENCHMARKING.md) - Performance benchmarking guide (NEW in Phase 9.5)
- [Structured Logging](docs/LOGGING.md) - Using the structured logging system
- [Graceful Shutdown](docs/SHUTDOWN.md) - Implementing graceful shutdown
- [HTTP Metrics](docs/METRICS.md) - Metrics and observability endpoints
- [API Reference](docs/API.md) - Package APIs and usage examples
- [Tutorial](docs/TUTORIAL.md) - Getting started guide
- [Troubleshooting](docs/TROUBLESHOOTING.md) - Common issues and solutions
- [Production Guide](docs/PRODUCTION.md) - Production deployment guide
- [Security Audit](AUDIT.md) - Security audit results and status

### Cross-Compilation

```bash
make build-linux-amd64   # Build for Linux AMD64
make build-linux-arm     # Build for Linux ARM
make build-linux-arm64   # Build for Linux ARM64
make build-linux-mips    # Build for Linux MIPS
make build-all           # Build for all platforms
```

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for detailed development guide.

## Testing

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run with coverage
go test -cover ./...
```

Current test coverage: ~74% overall, with critical packages at 90%+ coverage.

## Roadmap

### Phase 1: Foundation ✅ (Complete)
- Project structure and build system
- Cell encoding/decoding
- Circuit management types
- Cryptographic wrappers
- Configuration system
- Structured logging with log/slog
- Graceful shutdown with context propagation

### Phase 2: Core Protocol ✅ (Complete)
- TLS connection handling to Tor relays
- Protocol handshake and version negotiation (link protocol v3-5)
- Cell I/O with connection state management
- Directory protocol client (consensus fetching)
- Connection lifecycle management
- Error handling and timeout management

### Phase 3: Client Functionality ✅ (Complete)
- Path selection (guard, middle, exit)
- Circuit builder
- SOCKS5 proxy server (RFC 1928)
- Connection routing foundation

### Phase 4: Stream Handling ✅ (Complete)
- Stream management and multiplexing
- Circuit extension (CREATE2/CREATED2, EXTEND2/EXTENDED2)
- Key derivation (KDF-TOR)
- Stream isolation foundation

### Phase 5: Component Integration ✅ (Complete)
- Client orchestration package
- Integration of directory, circuit, and SOCKS5 components
- Circuit pool management and health monitoring
- Functional Tor client application
- End-to-end testing

### Phase 6: Production Hardening (Weeks 23-28)
- Complete circuit extension cryptography
- Guard node persistence
- Performance optimization
- Security hardening and audit
- Comprehensive testing and benchmarking

### Phase 7: Control Protocol & Onion Services ✅ (Complete - Weeks 29-36)
- ✅ Control protocol server with basic commands (Phase 7)
- ✅ Event notification system (Phase 7.1)
- ✅ Additional event types (Phase 7.2)
- ✅ v3 onion address parsing and validation (Phase 7.3 - Foundation)
- ✅ Descriptor management (caching, crypto) (Phase 7.3.1)
- ✅ HSDir protocol and descriptor fetching (Phase 7.3.2)
- ✅ Introduction protocol (Phase 7.3.3)
- ✅ Rendezvous protocol (Phase 7.3.4)
- ✅ Hidden service server (hosting) (Phase 7.4)

### Phase 8: Advanced Features (Weeks 37-40)
- ✅ Configuration file loading (torrc-compatible) (Phase 8.1)
- ✅ Enhanced error handling and resilience (Phase 8.2)
- ✅ Performance optimization and tuning (Phase 8.3)
- ✅ Security hardening and audit (Phase 8.4)
- ✅ Comprehensive testing and documentation (Phase 8.5)
- ✅ Onion Service Infrastructure Completion (Phase 8.6)

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for detailed development roadmap and architecture information.

## Performance Targets

These are design goals and validated performance characteristics:

- Circuit build time: < 5 seconds (95th percentile) ✅ **Validated: ~1.1s**
- Memory usage: < 50MB RSS in steady state ✅ **Target validated, specific measurements vary by workload**
- Concurrent streams: 100+ on Raspberry Pi 3 ✅ **Validated: 100+ @ 26,600 ops/sec**
- Binary size: < 15MB (13MB unstripped, 8.9MB stripped) ✅ **Validated**

See [docs/BENCHMARKING.md](docs/BENCHMARKING.md) for comprehensive benchmark results and [docs/PERFORMANCE.md](docs/PERFORMANCE.md) for micro-benchmark details.

## Security

This implementation follows Tor protocol specifications for security:

- Constant-time cryptographic operations
- Explicit memory zeroing for sensitive data
- Circuit padding for traffic analysis resistance
- Error handling without information leakage

⚠️ **Security Notice**: This is pre-production software. Do not rely on it for anonymity until security audit is complete.

See [AUDIT.md](AUDIT.md) for comprehensive security audit results and considerations.

## Documentation

Comprehensive documentation is available in the [docs/](docs/) directory:

### Core Documentation
- [ARCHITECTURE.md](docs/ARCHITECTURE.md) - System architecture and design
- [API.md](docs/API.md) - API reference and usage
- [TUTORIAL.md](docs/TUTORIAL.md) - Getting started tutorial

### Operations & Deployment
- [PRODUCTION.md](docs/PRODUCTION.md) - Production deployment guide
- [ZERO_CONFIG.md](docs/ZERO_CONFIG.md) - Zero-configuration mode
- [CONTROL_PROTOCOL.md](docs/CONTROL_PROTOCOL.md) - Control protocol interface
- [METRICS.md](docs/METRICS.md) - Metrics and monitoring

### Development & Testing
- [DEVELOPMENT.md](docs/DEVELOPMENT.md) - Development guidelines
- [TESTING.md](docs/TESTING.md) - Testing strategies
- [BENCHMARKING.md](docs/BENCHMARKING.md) - Performance benchmarks
- [TRACING.md](docs/TRACING.md) - Distributed tracing guide

### Advanced Topics
- [ONION_SERVICE_INTEGRATION.md](docs/ONION_SERVICE_INTEGRATION.md) - Onion services
- [PERFORMANCE.md](docs/PERFORMANCE.md) - Performance optimization
- [RESOURCE_PROFILES.md](docs/RESOURCE_PROFILES.md) - Resource profiles
- [TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) - Common issues and solutions

### Examples
See [examples/](examples/) directory for 20 working demonstrations covering all major features, including:
- [context-demo](examples/context-demo) - Timeout control and graceful cancellation
- [trace-demo](examples/trace-demo) - Distributed tracing and observability (NEW in Phase 9.11)

## Repository Structure

The repository maintains a clean, focused documentation structure. Historical audit reports and completed phase implementation reports have been removed to reduce clutter. See [CLEANUP_SUMMARY.md](CLEANUP_SUMMARY.md) for details on the recent repository cleanup that removed 13 obsolete documentation files (~200KB).

All active documentation is in the [docs/](docs/) directory, and all deleted files remain accessible in git history if needed.

## Contributing

Contributions are welcome! Please see [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for guidelines.

1. Fork the repository
2. Create a feature branch
3. Add tests for your changes
4. Ensure all tests pass
5. Submit a pull request

## License

BSD 3-Clause License. See [LICENSE](LICENSE) for details.

## References

- [Tor Specifications](https://spec.torproject.org/)
- [Tor Project](https://www.torproject.org/)
- [C Tor Implementation](https://github.com/torproject/tor)

## Acknowledgments

This project implements the Tor protocol as specified by the Tor Project. It is not affiliated with or endorsed by the Tor Project.

## Contact

- GitHub Issues: [github.com/opd-ai/go-tor/issues](https://github.com/opd-ai/go-tor/issues)
- Project URL: [github.com/opd-ai/go-tor](https://github.com/opd-ai/go-tor)
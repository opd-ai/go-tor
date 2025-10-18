# go-tor

[![Go Report Card](https://goreportcard.com/badge/github.com/opd-ai/go-tor)](https://goreportcard.com/report/github.com/opd-ai/go-tor)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue.svg)](LICENSE)

A production-ready Tor client implementation in pure Go, designed for embedded systems.

**⚠️ Development Status**: This project is in active development. Core functionality is being implemented according to the roadmap below.

## Features

### Current (Phase 1 & 2 - Foundation & Core Protocol)
- ✅ Cell encoding/decoding (fixed and variable-size)
- ✅ Relay cell handling
- ✅ Circuit management types and lifecycle
- ✅ Cryptographic primitives (AES-CTR, RSA, SHA-1/256)
- ✅ Configuration system with validation
- ✅ Structured logging with log/slog
- ✅ Graceful shutdown with context propagation
- ✅ TLS connection handling to Tor relays
- ✅ Protocol handshake and version negotiation
- ✅ Connection state management
- ✅ Directory client (consensus fetching)

### Planned
- [ ] **Phase 3**: SOCKS5 proxy, path selection, and circuit building
- [ ] **Phase 4**: Onion services (client and server)
- [ ] **Phase 5**: Control protocol and production hardening

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for detailed architecture and roadmap.

## Design Goals

- **Pure Go**: No CGo dependencies for maximum portability
- **Client-Only**: No relay or exit node functionality
- **Embedded-Optimized**: Low memory footprint (<50MB RSS) and resource efficiency
- **Feature Parity**: Match C Tor client capabilities
- **Cross-Platform**: Support for ARM, MIPS, x86 architectures

## Quick Start

### Prerequisites

- Go 1.21 or later

### Installation

```bash
git clone https://github.com/opd-ai/go-tor.git
cd go-tor
make build
```

### Running

```bash
# Run with default settings (SOCKS on 9050, Control on 9051)
./bin/tor-client

# Run with custom ports
./bin/tor-client -socks-port 9150 -control-port 9151

# Show version
./bin/tor-client -version
```

**Note**: The client is not yet functional. Core protocol implementation is in progress.

## Usage

### As a Library

```go
import (
    "github.com/opd-ai/go-tor/pkg/circuit"
    "github.com/opd-ai/go-tor/pkg/config"
)

// Create configuration
cfg := config.DefaultConfig()
cfg.SocksPort = 9050

// Create circuit manager
manager := circuit.NewManager()
circuit, err := manager.CreateCircuit()
// ... build and use circuit
```

### As a SOCKS Proxy (Coming Soon)

Configure your application to use `localhost:9050` as a SOCKS5 proxy.

## Architecture

The project is organized into modular packages:

- **pkg/cell**: Tor protocol cell encoding/decoding ✅
- **pkg/circuit**: Circuit management and lifecycle ✅
- **pkg/crypto**: Cryptographic primitives ✅
- **pkg/config**: Configuration management ✅
- **pkg/connection**: TLS connection handling ✅ (NEW in Phase 2)
- **pkg/protocol**: Core Tor protocol handshake ✅ (NEW in Phase 2)
- **pkg/directory**: Directory protocol client ✅ (NEW in Phase 2)
- **pkg/socks**: SOCKS5 proxy server (TODO)
- **pkg/onion**: Onion service support (TODO)
- **pkg/control**: Control protocol (TODO)
- **pkg/path**: Path selection algorithms (TODO)

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for details.

## Development

### Building

```bash
make build        # Build binary
make test         # Run tests
make test-coverage # Run tests with coverage
make fmt          # Format code
make vet          # Run go vet
make lint         # Run golint
```

### Documentation

- [Architecture](docs/ARCHITECTURE.md) - System architecture and design
- [Development Guide](docs/DEVELOPMENT.md) - Development workflow and guidelines
- [Structured Logging](docs/LOGGING.md) - Using the structured logging system
- [Graceful Shutdown](docs/SHUTDOWN.md) - Implementing graceful shutdown

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

Current test coverage: ~90% for implemented packages.

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

### Phase 3: Client Functionality (Weeks 11-16)
- SOCKS5 proxy server
- Path selection and guard persistence
- Stream handling and isolation
- DNS over Tor

### Phase 4: Onion Services (Weeks 17-22)
- Hidden service client (.onion resolution)
- Hidden service server (hosting)
- Descriptor management
- Introduction/rendezvous protocol

### Phase 5: Production Ready (Weeks 23-30)
- Control protocol implementation
- Embedded system optimization
- Security hardening and audit
- Comprehensive testing and documentation

See [problem statement](docs/ROADMAP.md) for full 30-week roadmap.

## Performance Targets

- Circuit build time: < 5 seconds (95th percentile)
- Memory usage: < 50MB RSS in steady state
- Concurrent streams: 100+ on Raspberry Pi 3
- Binary size: < 15MB static binary

## Security

This implementation follows Tor protocol specifications for security:

- Constant-time cryptographic operations
- Explicit memory zeroing for sensitive data
- Circuit padding for traffic analysis resistance
- Error handling without information leakage

⚠️ **Security Notice**: This is pre-production software. Do not rely on it for anonymity until security audit is complete.

See [docs/SECURITY.md](docs/SECURITY.md) for security considerations.

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
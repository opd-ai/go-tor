# Project Overview

go-tor is a pure Go implementation of the Tor protocol providing client functionality and bridge relay capabilities for educational and research purposes. This is **experimental software** developed without the supervision or endorsement of The Tor Project. The project implements core Tor protocol specifications including circuit management, cryptographic operations, SOCKS5 proxy serving, and v3 onion service support.

The primary audience includes researchers studying anonymity network protocols, developers learning about Tor internals, and educators teaching network security concepts. The codebase prioritizes portability, embeddability, and code clarity over production use. All development follows Tor protocol specifications (tor-spec.txt, dir-spec.txt, rend-spec.txt, control-spec.txt).

Key technologies: Go 1.24+, OpenTelemetry for observability, x/crypto for cryptographic primitives, SOCKS5 for proxy protocol, and the Tor cell protocol for network communication. The architecture follows clean separation of concerns with modular packages handling distinct protocol layers.

## Technical Stack

- **Primary Language**: Go 1.24+ (pure Go, no CGo dependencies)
- **Frameworks/Libraries**:
  - `go.opentelemetry.io/otel` v1.39.0 - Distributed tracing and observability
  - `golang.org/x/crypto` v0.44.0 - Cryptographic primitives (AES-CTR, RSA, SHA, Ed25519)
  - `golang.org/x/net` v0.47.0 - Network protocol utilities
  - `github.com/gofrs/flock` v0.13.0 - File locking for guard persistence
  - `github.com/cretz/bine` v0.2.0 - Tor controller library integration
- **Testing**: Go built-in testing with table-driven tests, race detector, 74%+ coverage target
- **Build/Deploy**: Makefile-based builds, Docker support, cross-compilation for Linux (AMD64, ARM, ARM64, MIPS)

## Code Assistance Guidelines

1. **Follow Tor protocol specifications**: All protocol implementations must adhere to the official Tor specs at spec.torproject.org. Reference tor-spec.txt for cell encoding, dir-spec.txt for directory operations, and rend-spec.txt for onion services.

2. **Use structured logging with log/slog**: Create component-specific loggers using `logger.New(slog.LevelInfo, os.Stdout)` and include contextual fields like circuit_id, relay address, and timing information in all log statements.

3. **Implement proper error handling**: Use the custom `pkg/errors` package with categories (CategoryNetwork, CategoryProtocol, CategoryCircuit, etc.) and severity levels. Always wrap errors with context using `errors.Wrap()` or `fmt.Errorf("context: %w", err)`.

4. **Write table-driven tests**: Follow Go testing conventions with descriptive test names, comprehensive edge case coverage, and use `t.TempDir()` for test data. Target 80%+ coverage for new code, 90%+ for security-critical packages.

5. **Use resource pooling for performance**: Leverage `pkg/pool` for buffer pools (CellBufferPool, PayloadBufferPool, CryptoBufferPool) and connection pools. Always return pooled resources using `defer pool.Put(buf)`.

6. **Implement constant-time cryptographic operations**: All crypto operations in `pkg/crypto` must prevent timing attacks. Zero sensitive data after use and ensure errors don't leak timing or state information.

7. **Support graceful shutdown with context**: All long-running operations must accept `context.Context` and respect cancellation. Use `context.WithTimeout` for operations with known time bounds.

## Project Context

- **Domain**: Anonymity network protocols implementing the Tor specification for circuit-based anonymous communication. Key concepts include guard/middle/exit relays, onion routing, directory authorities, consensus documents, and v3 onion services.

- **Architecture**: Layered design with Application Layer (SOCKS5 clients) → SOCKS5 Proxy (`pkg/socks`) → Circuit Manager (`pkg/circuit`) → Cell Processing (`pkg/cell`) → Protocol Layer (`pkg/protocol`) → Network Layer (TLS connections). Supporting systems include path selection, directory client, crypto primitives, and control protocol.

- **Key Directories**:
  - `cmd/tor-client/` - Main executable entry point
  - `pkg/client/` - High-level client orchestration
  - `pkg/circuit/` - Circuit management and lifecycle
  - `pkg/cell/` - Cell encoding/decoding (514-byte fixed cells)
  - `pkg/crypto/` - Cryptographic primitives (AES, RSA, SHA, KDF)
  - `pkg/socks/` - SOCKS5 proxy server (RFC 1928)
  - `pkg/onion/` - v3 onion service support
  - `pkg/control/` - Tor control protocol server
  - `examples/` - Usage examples and demos

- **Configuration**: Use `config.DefaultConfig()` for sensible defaults (SocksPort 9050, ControlPort 9051). Load torrc-compatible files with `config.LoadFromFile()`. Validate with `cfg.Validate()`.

## Quality Standards

- **Testing Requirements**: Maintain >74% overall coverage, >90% for critical packages (config, control, errors, health, logger, metrics, security). Run `go test -race ./...` to detect data races. Use `-short` flag to skip slow integration tests.

- **Code Review Criteria**: All exported types, functions, and constants must have GoDoc comments. Follow Effective Go and Go Code Review Comments guidelines. Use `gofmt`, `go vet`, and `staticcheck` before commits.

- **Documentation Standards**: Update relevant docs in `/docs` directory when modifying public APIs. Keep examples in `/examples` synchronized with API changes. Follow existing markdown structure.

- **Performance Targets**: Circuit build time <5s (95th percentile), memory <50MB RSS steady state, 100+ concurrent streams on resource-constrained devices, <15MB static binary.

## Networking Best Practices

When declaring network variables, always use interface types:
- Never use `net.UDPAddr`, `net.IPAddr`, or `net.TCPAddr`. Use `net.Addr` only instead.
- Never use `net.UDPConn`, use `net.PacketConn` instead
- Never use `net.TCPConn`, use `net.Conn` instead
- Never use `net.UDPListener` or `net.TCPListener`, use `net.Listener` instead
- Never use a type switch or type assertion to convert from an interface type to a concrete type. Use the interface methods instead.

This approach enhances testability and flexibility when working with different network implementations or mocks.

## Security Considerations

- **Educational Purpose Only**: This software is NOT safe for production anonymity needs. Always direct users to official Tor software (Tor Browser, Arti) for actual privacy requirements.
- **Constant-Time Crypto**: All cryptographic operations must use constant-time implementations
- **Memory Zeroing**: Explicitly zero sensitive data (keys, secrets) after use
- **Error Handling**: Errors must not leak timing or state information
- **Exit Relay Prohibition**: Exit node functionality is explicitly out of scope and must not be implemented

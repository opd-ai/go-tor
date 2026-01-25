# Go-Tor Roadmap

## Current Status (January 2026)

The go-tor implementation has achieved **~98% protocol compliance** with all critical components implemented and functional. See [AUDIT.md](AUDIT.md) for the comprehensive compliance audit report.

## Future Enhancements (Optional)

These are potential enhancements that could be implemented in the future. None are critical for core functionality.

### Performance Optimizations

- [x] **Circuit Rate Limiting** ✅ **COMPLETED (January 25, 2026)**
  - Implemented `CircuitCreationsPerSecond` and `CircuitCreationsBurst` parameters
  - Added metrics tracking for `RateLimitedCircuits` and `RateLimitWaitTime`
  - Token bucket algorithm with zero overhead when disabled
  - Comprehensive test coverage (>95%)
  - Documentation: `docs/CIRCUIT_RATELIMIT.md`
  - Example: `examples/circuit-ratelimit/`
  - Priority: Low → COMPLETED
  - Benefit: Protection against circuit creation DoS achieved
  
- [x] **Stream Backpressure** ✅ **COMPLETED (January 25, 2026)**
  - Implemented `StreamBufferHighWaterMark` and `StreamBufferLowWaterMark` parameters
  - Added metrics for `BackpressurePauses` and `BackpressureResumes`
  - Hysteresis-based control prevents oscillation
  - Independent send/receive buffer management
  - Comprehensive test coverage (>95%)
  - Documentation: `docs/STREAM_BACKPRESSURE.md`
  - Example: `examples/stream-backpressure/`
  - Priority: Low → COMPLETED
  - Benefit: Better memory management under high load achieved

### Testing Enhancements

- [x] **Integration Test Suite Expansion** ✅ **COMPLETED (January 25, 2026)**
  - Added end-to-end tests for client authorization workflows
  - Three comprehensive integration tests covering:
    - Complete client authorization workflow (credential generation → decryption)
    - Multiple authorized clients with credential isolation
    - Address validation and error handling
  - Documentation: `docs/TESTING_CLIENT_AUTHORIZATION.md`
  - Tests: `pkg/onion/client_auth_integration_test.go`
  - Priority: Low → COMPLETED
  - Benefit: Improved reliability detection for private onion services achieved

- [x] **Benchmark Suite Expansion** ✅ **COMPLETED (January 25, 2026)**
  - Expanded `pkg/benchmark` test coverage from 21.6% to 84.6%
  - Added comprehensive tests for all benchmark suite methods
  - Fixed divide-by-zero bugs in edge cases (short timeouts)
  - Added unit tests for:
    - RunAll comprehensive benchmark suite
    - All individual benchmark methods (circuit build, memory, streams)
    - Circuit build with pool, memory leak detection
    - Stream scaling and multiplexing
  - Tests run in both short mode (fast) and full mode (comprehensive)
  - All tests pass with race detector
  - Priority: Low → COMPLETED
  - Benefit: Better test coverage and reliability for performance tracking

### Protocol Extensions

- [ ] **Congestion Control**
  - Implement Tor's congestion control algorithm (proposal 324)
  - Priority: Low
  - Benefit: Better performance on congested networks

- [ ] **Additional Padding Machines**
  - Implement application-specific padding strategies beyond APE
  - Priority: Low
  - Benefit: Enhanced traffic analysis resistance for specific use cases

### Developer Experience

- [ ] **CLI Tool Enhancements**
  - Add interactive configuration wizard
  - Add network diagnostic tools
  - Priority: Low
  - Benefit: Easier setup and debugging

- [ ] **Documentation Expansion**
  - Add architecture decision records (ADRs)
  - Add protocol implementation guides
  - Priority: Low
  - Benefit: Better understanding for contributors

## Maintenance Mode

The project is currently in **maintenance mode** with all core features complete. Future work will focus on:
- Bug fixes as they are discovered
- Security updates and patches  
- Dependency updates
- Performance optimizations
- Optional enhancements from the list above

## Non-Goals

The following are explicitly **out of scope** for this implementation:

- **Exit Node Functionality**: Exit relay operation is explicitly out of scope
- **Directory Authority Operation**: Authority operations are out of scope
- **Tor Browser Integration**: This is a library/client, not a browser
- **TAP Handshake**: Deprecated protocol (RSA-1024) - ntor is required

**In Scope** (included in the project):

- **Onion Service Hosting**: Server-side onion service hosting is supported
- **Traffic Relaying**: Bridge relay and non-exit relay functionality is in scope
- **Pluggable Transports**: Pluggable transport support for censorship resistance is in scope

## Contributing

While the core implementation is complete, contributions are welcome for:
- Bug reports and fixes
- Performance improvements
- Test coverage improvements
- Documentation enhancements
- Optional roadmap items listed above

Please see [CONTRIBUTING.md](CONTRIBUTING.md) if it exists, or open an issue to discuss contributions.

---

**Note**: This roadmap represents potential future work, not commitments or requirements. The current implementation is fully functional and production-ready for client use cases.

**Last Updated**: January 25, 2026

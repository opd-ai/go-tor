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

- [ ] **Benchmark Suite**
  - Expand `pkg/benchmark` test coverage (currently 21.6%)
  - Add continuous performance monitoring
  - Priority: Low
  - Benefit: Performance tracking over time

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

- **Relay/Exit Node Functionality**: This is a client-only implementation
- **Directory Authority Operation**: Authority operations are out of scope
- **Onion Service Hosting**: Server-side onion service hosting (only client-side connection support)
- **Tor Browser Integration**: This is a library/client, not a browser
- **TAP Handshake**: Deprecated protocol (RSA-1024) - ntor is required

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

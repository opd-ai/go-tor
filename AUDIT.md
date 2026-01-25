# Implementation Plan: Advanced Tor Features

This document outlines the implementation plan for completing three major features in go-tor:

1. **Onion Service Hosting**: Server-side onion service hosting
2. **Traffic Relaying**: Bridge relay and non-exit relay functionality
3. **Pluggable Transports**: Pluggable transport support for censorship resistance

## ⚠️ Important Notice

This is an **unofficial, experimental implementation** developed for educational and research purposes. This software has been developed **without the supervision or endorsement of The Tor Project**.

**This software should NOT be used for any real anonymity or privacy needs.**

For actual Tor usage:
- **Users**: Use [Tor Browser](https://www.torproject.org/download/)
- **Developers**: Use [Arti](https://gitlab.torproject.org/tpo/core/arti) (official Rust implementation)

---

## Current Status Summary

Based on the comprehensive audit (see [AUDIT.md](AUDIT.md)), the implementation has achieved **~98% protocol compliance** with all critical client components implemented:

- ✅ Cell Protocol (tor-spec §0-3) - 100% compliant
- ✅ TLS and Link Protocol (tor-spec §1-2) - 100% compliant
- ✅ Circuit Creation/Extension (tor-spec §4-5) - 100% compliant
- ✅ v3 Onion Services Client (rend-spec-v3) - 100% compliant
- ✅ Client Authorization (rend-spec-v3 §2.5) - 100% compliant
- ✅ Circuit Padding (padding-spec) - 100% compliant
- ✅ Path Bias Detection (path-spec §5.3) - 100% compliant

### Existing Onion Service Server Implementation

The `pkg/onion/service.go` already contains foundational server-side functionality:
- Ed25519 identity key generation and management
- v3 onion address derivation from public key
- Introduction point establishment (placeholder)
- Descriptor creation and signing (certificate-based per cert-spec.txt)
- Descriptor publishing to HSDirs
- INTRODUCE2 cell handling (partial)
- Maintenance loop for descriptor refresh

---

## Phase 9: Onion Service Hosting (Enhanced)

### 9.1 Complete Introduction Point Protocol

**Specification Reference**: rend-spec-v3.txt §3.1

**Current State**: Basic structure exists, needs production integration

**Tasks**:

- [x] **9.1.1 Real Circuit Building for Introduction Points** ✅ **COMPLETED** (January 25, 2026)
  - Integrated with `pkg/circuit/Builder` for building 3-hop circuits
  - Integrated with `pkg/path/Selector` for path selection
  - Implemented circuit retry logic with exponential backoff
  - Added circuit health monitoring for intro point circuits
  - Implementation: `pkg/onion/intro_protocol.go` (`BuildIntroCircuitWithRetry`)

- [x] **9.1.2 ESTABLISH_INTRO Cell Protocol** ✅ **COMPLETED** (Already implemented)
  - Complete `sendEstablishIntro()` implementation with proper cell format
  - Implement MAC computation over ESTABLISH_INTRO cell
  - Add DoS extension support (optional)
  - Handle INTRO_ESTABLISHED response validation
  - Implementation: `pkg/onion/service.go` (`sendEstablishIntro`, `waitForIntroEstablished`)

- [x] **9.1.3 Introduction Point Rotation** ✅ **COMPLETED** (January 25, 2026)
  - Implemented intro point health checking
  - Added automatic replacement of failed intro points
  - Support configurable rotation intervals (24h default)
  - Maintain minimum required intro points (configurable, default: 3)
  - Implementation: `pkg/onion/intro_protocol.go` (`IntroPointManager`), `pkg/onion/service.go` (`rotateUnhealthyIntroPoints`)

**Files Modified/Created**:
- `pkg/onion/intro_protocol.go` (new) - Introduction point protocol handling
- `pkg/onion/intro_protocol_test.go` (new) - Comprehensive test coverage (>95%)
- `pkg/onion/service.go` - Enhanced `establishIntroductionPoint()`, integrated `IntroPointManager`
- `docs/INTRO_POINT_PROTOCOL.md` (new) - Complete documentation

### 9.2 Complete INTRODUCE2 Handling

**Specification Reference**: rend-spec-v3.txt §3.2-3.3

**Current State**: INTRODUCE2 parsing complete

**Tasks**:

- [x] **9.2.1 INTRODUCE2 Cell Parsing** ✅ **COMPLETED** (January 25, 2026)
  - Parse complete INTRODUCE2 cell format (encrypted portion)
  - Decrypt client data using introduction point keys
  - Extract rendezvous cookie, client onion key, link specifiers
  - Validate cell MAC
  - Implementation: `pkg/onion/introduce2.go` with comprehensive test coverage (>70%)
  - Tests: `pkg/onion/introduce2_test.go` (9 tests, all passing)
  - Added crypto helpers: `DecryptAES256CTR`, `EncryptAES256CTR`, `ConstantTimeCompare`
  - Tests: `pkg/crypto/crypto_test.go` (comprehensive coverage >85%)

- [ ] **9.2.2 Rendezvous Circuit Building**
  - Build circuit to rendezvous point specified by client
  - Parse link specifiers to determine rendezvous relay
  - Use existing circuit builder infrastructure

- [ ] **9.2.3 RENDEZVOUS1 Cell Construction**
  - Implement ntor handshake server-side
  - Construct RENDEZVOUS1 cell with handshake response
  - Send RENDEZVOUS1 on rendezvous circuit
  - Complete key derivation for end-to-end encryption

**Files Modified/Created**:
- `pkg/onion/introduce2.go` (new) - INTRODUCE2 parsing and decryption
- `pkg/onion/introduce2_test.go` (new) - Comprehensive test coverage
- `pkg/onion/service.go` - Updated `HandleIntroduce2()` to use new parser
- `pkg/onion/service_test.go` - Updated tests for new parsing logic
- `pkg/crypto/crypto.go` - Added `DecryptAES256CTR`, `EncryptAES256CTR`, `ConstantTimeCompare`
- `pkg/crypto/crypto_test.go` - Added comprehensive tests for new functions

### 9.3 Stream Handling for Services

**Specification Reference**: tor-spec.txt §6

**Tasks**:

- [ ] **9.3.1 Incoming Stream Management**
  - Accept RELAY_BEGIN cells from clients on rendezvous circuits
  - Map virtual ports to local service endpoints
  - Forward traffic to local service

- [ ] **9.3.2 Service Backend Connection**
  - Connect to local service ports defined in `ServiceConfig.Ports`
  - Implement bidirectional data forwarding
  - Handle connection errors and cleanup

- [ ] **9.3.3 Service Metrics**
  - Track active connections per service
  - Monitor descriptor publication success
  - Report introduction success/failure rates

**Files to Modify**:
- `pkg/onion/service.go` - Add stream handling
- `pkg/onion/service_stream.go` (new) - Stream management for services
- `pkg/metrics/onion_service.go` (new) - Service-specific metrics

### 9.4 Service Persistence

**Tasks**:

- [ ] **9.4.1 Key Persistence**
  - Save/load identity keys from `DataDirectory`
  - Implement secure key storage (encrypted on disk)
  - Support key import/export for backup

- [ ] **9.4.2 State Persistence**
  - Persist service state across restarts
  - Cache introduction point assignments
  - Store descriptor publication timestamps

**Files to Modify**:
- `pkg/onion/persistence.go` (new) - Service state persistence
- `pkg/config/config.go` - Add service persistence configuration

---

## Phase 10: Bridge Relay Implementation

### 10.1 OR Protocol Server

**Specification Reference**: tor-spec.txt §1-5 (server-side)

**Tasks**:

- [ ] **10.1.1 TLS Server Setup**
  - Generate/load relay identity keys (Ed25519 + RSA)
  - Configure TLS server with proper cipher suites
  - Accept incoming OR connections
  - Implement TLS certificate generation per tor-spec.txt §1.1

- [ ] **10.1.2 Link Protocol Server**
  - Handle incoming VERSIONS cells
  - Send CERTS, AUTH_CHALLENGE, NETINFO cells
  - Implement in-protocol link authentication
  - Support link protocol versions 3-5

- [ ] **10.1.3 Circuit Handling (Server-Side)**
  - Accept CREATE2 cells from clients
  - Perform ntor handshake server-side
  - Send CREATED2 responses
  - Manage server-side circuit state

**Files to Create**:
- `pkg/relay/relay.go` - Main relay server
- `pkg/relay/or_listener.go` - OR connection listener
- `pkg/relay/or_handler.go` - Connection handler
- `pkg/relay/circuit_handler.go` - Server-side circuit handling

### 10.2 Non-Exit Relay Functionality

**Specification Reference**: tor-spec.txt §5.3-5.6

**Tasks**:

- [ ] **10.2.1 Circuit Extension Handling**
  - Handle RELAY_EXTEND2 cells
  - Connect to next hop relay
  - Forward RELAY_EXTENDED2 responses
  - Implement proper encryption layer management

- [ ] **10.2.2 Cell Forwarding**
  - Forward relay cells between circuits
  - Implement proper cell routing
  - Handle RELAY_EARLY cell counting (8 max per circuit direction)
  - Process DESTROY cells correctly

- [ ] **10.2.3 Exit Policy (Reject All)**
  - Implement reject-all exit policy
  - Respond with RELAY_END (EXITPOLICY) for any exit attempts
  - Ensure no exit traffic is relayed

**Files to Create**:
- `pkg/relay/extension.go` - Circuit extension handling
- `pkg/relay/forwarding.go` - Cell forwarding logic
- `pkg/relay/policy.go` - Exit policy enforcement

### 10.3 Bridge Descriptor Publishing

**Specification Reference**: dir-spec.txt §4, bridge-spec.txt

**Tasks**:

- [ ] **10.3.1 Server Descriptor Generation**
  - Generate signed server descriptor
  - Include bridge-specific fields
  - Support extra-info descriptor

- [ ] **10.3.2 Bridge Authority Communication**
  - Publish descriptors to bridge authority
  - Handle descriptor upload responses
  - Implement descriptor refresh schedule

- [ ] **10.3.3 BridgeDB Integration** (Optional)
  - Support bridge distribution mechanisms
  - Implement bridge email responder integration (research/educational only)

**Files to Create**:
- `pkg/relay/descriptor.go` - Server descriptor generation
- `pkg/relay/publisher.go` - Descriptor publishing
- `pkg/relay/bridge_config.go` - Bridge-specific configuration

### 10.4 Relay Security Hardening

**Tasks**:

- [ ] **10.4.1 Rate Limiting**
  - Limit circuit creation rate
  - Limit connections per IP
  - Implement token bucket for cell processing

- [ ] **10.4.2 DoS Protection**
  - Connection count limits
  - Circuit count limits per connection
  - Cell rate limiting per circuit

- [ ] **10.4.3 Logging and Monitoring**
  - Relay-specific metrics (circuits/second, bandwidth)
  - Connection statistics
  - Attack detection logging

**Files to Create**:
- `pkg/relay/ratelimit.go` - Relay rate limiting
- `pkg/relay/protection.go` - DoS protection
- `pkg/relay/metrics.go` - Relay metrics

---

## Phase 11: Pluggable Transports

### 11.1 PT Framework

**Specification Reference**: pt-spec.txt

**Tasks**:

- [ ] **11.1.1 PT Client Interface**
  - Implement `ClientTransport` interface
  - Support PT version 2 IPC protocol
  - Manage PT subprocess lifecycle

- [ ] **11.1.2 PT Server Interface**
  - Implement `ServerTransport` interface  
  - Support PT server configuration
  - Handle SMETHOD/SMETHODS protocol

- [ ] **11.1.3 PT Configuration**
  - Parse PT configuration from torrc
  - Support environment variable passing
  - Handle state directory management

**Files to Create**:
- `pkg/pt/transport.go` - Transport interface definitions
- `pkg/pt/client.go` - PT client implementation
- `pkg/pt/server.go` - PT server implementation
- `pkg/pt/ipc.go` - IPC protocol with PT processes
- `pkg/pt/manager.go` - PT process lifecycle management

### 11.2 Built-in Transport: obfs4

**Specification Reference**: obfs4 specification

**Tasks**:

- [ ] **11.2.1 obfs4 Client**
  - Implement obfs4 client handshake
  - ntor-aes256-gcm-sha256 key exchange
  - Packet framing and encryption
  - Integrate with lyrebird library if available

- [ ] **11.2.2 obfs4 Server**
  - Implement obfs4 server handshake
  - Key material management
  - Bridge line generation

- [ ] **11.2.3 obfs4 Configuration**
  - Certificate generation
  - Key persistence
  - IAT mode configuration

**Files to Create**:
- `pkg/pt/obfs4/client.go` - obfs4 client
- `pkg/pt/obfs4/server.go` - obfs4 server
- `pkg/pt/obfs4/handshake.go` - obfs4 handshake implementation
- `pkg/pt/obfs4/framing.go` - Packet framing

### 11.3 External PT Integration

**Tasks**:

- [ ] **11.3.1 Managed PT Mode**
  - Launch external PT binaries (obfs4proxy, snowflake-client)
  - Parse CMETHOD/SMETHOD lines
  - Handle PT process restarts

- [ ] **11.3.2 PT Path Configuration**
  - Configurable PT binary paths
  - Support for multiple PTs
  - PT state directory management

**Files to Create**:
- `pkg/pt/external.go` - External PT integration
- `pkg/pt/protocol.go` - PT IPC protocol implementation

### 11.4 Bridge Client Integration

**Tasks**:

- [ ] **11.4.1 Bridge Address Parsing**
  - Parse bridge lines with PT specifications
  - Support multiple bridge address formats
  - Handle PT-specific bridge parameters

- [ ] **11.4.2 PT Connection Flow**
  - Connect via PT for circuit building
  - Transparent PT usage in circuit builder
  - Fallback to direct connections if PT fails

- [ ] **11.4.3 Configuration Integration**
  - Add PT configuration to `Config` struct
  - Support torrc `Bridge` and `ClientTransport*` directives
  - PT process supervision

**Files to Modify**:
- `pkg/config/config.go` - Add PT configuration
- `pkg/circuit/builder.go` - PT-aware circuit building
- `pkg/connection/connection.go` - PT connection wrapping

---

## Testing Plan

### Unit Tests

Each new package should have comprehensive unit tests:

- `pkg/relay/*_test.go` - Relay functionality tests
- `pkg/pt/*_test.go` - Pluggable transport tests
- `pkg/onion/service_*_test.go` - Enhanced service tests

### Integration Tests

- [ ] End-to-end onion service hosting test
- [ ] Bridge relay connectivity test (with local client)
- [ ] Pluggable transport connectivity test
- [ ] Mixed scenario tests

### Compatibility Tests

- [ ] Test against reference Tor implementation
- [ ] Verify interoperability with tor (C) client
- [ ] Test with official PT implementations (obfs4proxy)

---

## Documentation Updates

### New Documentation

- [ ] `docs/ONION_SERVICE_HOSTING.md` - Complete service hosting guide
- [ ] `docs/BRIDGE_RELAY.md` - Bridge relay setup and operation
- [ ] `docs/PLUGGABLE_TRANSPORTS.md` - PT configuration and usage

### Updates to Existing Docs

- [ ] `docs/ARCHITECTURE.md` - Add relay and PT architecture
- [ ] `docs/CONFIGURATION.md` - Add relay and PT configuration
- [ ] `docs/API.md` - Add new public APIs
- [ ] `ROADMAP.md` - Update with completion status

---

## Dependencies

### New Dependencies (Potential)

```go
// go.mod additions (if needed)
require (
    // obfs4 library (optional - can use external process)
    gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird v0.x.x
)
```

### Internal Dependencies

All new packages will depend on existing infrastructure:
- `pkg/cell` - Cell encoding/decoding
- `pkg/crypto` - Cryptographic operations
- `pkg/circuit` - Circuit management
- `pkg/logger` - Logging
- `pkg/config` - Configuration

---

## Timeline Estimates

| Phase | Feature | Estimated Duration | Priority |
|-------|---------|-------------------|----------|
| 9.1 | Introduction Point Protocol | 2-3 weeks | High |
| 9.2 | INTRODUCE2 Handling | 2-3 weeks | High |
| 9.3 | Stream Handling for Services | 1-2 weeks | High |
| 9.4 | Service Persistence | 1 week | Medium |
| 10.1 | OR Protocol Server | 3-4 weeks | High |
| 10.2 | Non-Exit Relay | 2-3 weeks | High |
| 10.3 | Descriptor Publishing | 2 weeks | Medium |
| 10.4 | Relay Security | 2 weeks | High |
| 11.1 | PT Framework | 2-3 weeks | Medium |
| 11.2 | obfs4 Implementation | 3-4 weeks | Medium |
| 11.3 | External PT Integration | 1-2 weeks | Low |
| 11.4 | Bridge Client Integration | 2 weeks | Medium |

**Total Estimated Duration**: 20-30 weeks for complete implementation

---

## Non-Goals (Explicit Exclusions)

The following are **explicitly out of scope** and will NOT be implemented:

- ❌ **Exit Node Functionality**: Exit relay operation that forwards traffic to the public internet
- ❌ **Directory Authority Operation**: Running directory authorities
- ❌ **Guard Node Advertising**: Operating as a guard relay for the general network
- ❌ **Bandwidth Authority**: Participating in bandwidth measurement
- ❌ **Production Anonymity**: Guarantees of privacy or safety

---

## References

### Tor Specifications

- [tor-spec.txt](https://spec.torproject.org/tor-spec) - Core protocol
- [dir-spec.txt](https://spec.torproject.org/dir-spec) - Directory protocol
- [rend-spec-v3.txt](https://spec.torproject.org/rend-spec-v3) - v3 onion services
- [pt-spec.txt](https://spec.torproject.org/pt-spec) - Pluggable transports
- [bridge-spec.txt](https://spec.torproject.org/bridge-spec) - Bridge specification
- [cert-spec.txt](https://spec.torproject.org/cert-spec) - Certificate format

### Related Projects

- [Arti](https://gitlab.torproject.org/tpo/core/arti) - Official Tor Rust implementation
- [Lyrebird](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird) - PT implementations
- [tor](https://gitlab.torproject.org/tpo/core/tor) - Reference C implementation

---

**Last Updated**: January 2026  
**Status**: Planning Phase

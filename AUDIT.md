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

- [x] **9.2.2 Rendezvous Circuit Building** ✅ **COMPLETED** (January 25, 2026)
  - Build circuit to rendezvous point specified by client
  - Parse link specifiers to determine rendezvous relay
  - Use existing circuit builder infrastructure
  - Implementation: `pkg/onion/rendezvous.go` (rendezvous circuit builder)
  - Tests: `pkg/onion/rendezvous_test.go` (comprehensive test coverage >95%)
  - Integration: Enhanced `pkg/onion/service.go` `HandleIntroduce2()` to build rendezvous circuits
  - Asynchronous circuit building to avoid blocking introduction handling

- [x] **9.2.3 RENDEZVOUS1 Cell Construction** ✅ **COMPLETED** (January 25, 2026)
  - Implemented server-side ntor handshake (`NtorServerHandshake`)
  - Constructed RENDEZVOUS1 cells with handshake response
  - Sent RENDEZVOUS1 on rendezvous circuits
  - Completed key derivation for end-to-end encryption
  - Implementation: `pkg/crypto/ntor_server.go`, `pkg/onion/rendezvous1.go`
  - Tests: `pkg/crypto/ntor_server_test.go`, `pkg/onion/rendezvous1_test.go` (>95% coverage)
  - Integration: Enhanced `pkg/onion/service.go` `HandleIntroduce2()` to send RENDEZVOUS1
  - Added ntor key generation in service initialization
  - Documentation: `docs/RENDEZVOUS1_IMPLEMENTATION.md`

**Files Modified/Created**:
- `pkg/onion/introduce2.go` (new) - INTRODUCE2 parsing and decryption
- `pkg/onion/introduce2_test.go` (new) - Comprehensive test coverage
- `pkg/onion/rendezvous.go` (new) - Rendezvous circuit building
- `pkg/onion/rendezvous_test.go` (new) - Comprehensive test coverage (>95%)
- `pkg/onion/service.go` - Updated `HandleIntroduce2()` to build rendezvous circuits, added `RendezvousCircuitBuilder`
- `pkg/onion/service_test.go` - Updated tests for new parsing logic
- `pkg/crypto/crypto.go` - Added `DecryptAES256CTR`, `EncryptAES256CTR`, `ConstantTimeCompare`
- `pkg/crypto/crypto_test.go` - Added comprehensive tests for new functions

### 9.3 Stream Handling for Services

**Specification Reference**: tor-spec.txt §6

**Tasks**:

- [x] **9.3.1 Incoming Stream Management** ✅ **COMPLETED** (January 25, 2026)
  - Accept RELAY_BEGIN cells from clients on rendezvous circuits
  - Map virtual ports to local service endpoints
  - Forward traffic to local service
  - Implementation: `pkg/onion/service_stream.go` (ServiceStreamManager)
  - Tests: `pkg/onion/service_stream_test.go` (>75% coverage)
  - Features:
    - RELAY_BEGIN handling with address/port parsing
    - RELAY_CONNECTED response to clients
    - RELAY_DATA bidirectional forwarding
    - RELAY_END cleanup and stream termination
    - Backend TCP connection management with timeouts
    - Stream lifecycle management and statistics

- [x] **9.3.2 Service Backend Connection** ✅ **COMPLETED** (January 25, 2026)
  - Connected to local service ports defined in `ServiceConfig.Ports`
  - Implemented bidirectional data forwarding
  - Handled connection errors and cleanup
  - Implementation: `pkg/onion/service_stream.go` (`connectToBackend`, `forwardToCircuit`, `forwardFromCircuit`)
  - TCP dial with 10-second timeout
  - Graceful connection lifecycle management

- [x] **9.3.3 Service Metrics** ✅ **COMPLETED** (January 25, 2026)
  - Tracked active connections per service (`OnionServiceStreamsActive`)
  - Monitored descriptor publication success (`OnionServiceDescriptorPublished/Failed`)
  - Reported introduction success/failure rates (`OnionServiceIntroEstablished/Failed/Received`)
  - Implementation: `pkg/metrics/metrics.go` (comprehensive onion service metrics)
  - Additional metrics: Stream data transferred, rendezvous success/failure, intro point count, service duration

**Files Modified/Created**:
- `pkg/onion/service_stream.go` (new) - Stream management for services
- `pkg/onion/service_stream_test.go` (new) - Comprehensive test coverage (>75%)
- `pkg/onion/service.go` - Add stream handling, updated Stop() to close streams, added `handleRendezvousCircuitCells()`
- `pkg/circuit/circuit.go` - Added `GetID()` method for CircuitInterface
- `pkg/onion/rendezvous1.go` - Updated CircuitInterface to include GetID() and ReceiveRelayCell()
- `pkg/cell/relay.go` - Added END_REASON constants

### 9.4 Service Persistence

**Tasks**:

- [x] **9.4.1 Key Persistence** ✅ **COMPLETED** (January 25, 2026)
  - Saved/loaded identity keys from `DataDirectory`
  - Implemented secure key storage (owner read/write only, permissions 0600)
  - Supported key import/export for backup
  - Implementation: `pkg/onion/persistence.go` (ServicePersistence)
  - Features:
    - Ed25519 identity key persistence with version format
    - Curve25519 ntor key persistence
    - Secure file permissions (0600 for keys)
    - Atomic state writes with temp file + rename
    - Secure deletion with 3-pass random overwrite
    - Export/import functionality for backups
  - Tests: `pkg/onion/persistence_test.go` (12 tests, >85% coverage)
  - Integration: Enhanced `pkg/onion/service.go` `NewService()` to load/save keys
  - Integration tests: `pkg/onion/service_test.go` (3 integration tests)

- [x] **9.4.2 State Persistence** ✅ **COMPLETED** (January 25, 2026)
  - Persist service state across restarts
  - Cache introduction point assignments
  - Store descriptor publication timestamps
  - Track descriptor revision counter for monotonically increasing versions
  - Implementation: Enhanced `pkg/onion/service.go` with state management
  - Features:
    - State automatically saved on Stop() and after descriptor publish
    - State loaded on service initialization from DataDirectory
    - Intro point cache includes established intro points only
    - Descriptor revision counter persisted and incremented on each publish
    - Creation timestamp tracked across restarts
  - Tests: `pkg/onion/service_state_test.go` (7 tests, all passing)
  - Coverage: saveState() method has 92.9% coverage

**Files Modified/Created**:
- `pkg/onion/persistence.go` (new) - Service state persistence with secure key storage
- `pkg/onion/persistence_test.go` (new) - Comprehensive test coverage (>85%)
- `pkg/onion/service.go` - Enhanced `NewService()` to load/save keys from DataDirectory
- `pkg/onion/service_test.go` - Added 3 integration tests for persistence
- `pkg/config/config.go` - Add service persistence configuration (already has DataDirectory)

---

## Phase 10: Bridge Relay Implementation

### 10.1 OR Protocol Server

**Specification Reference**: tor-spec.txt §1-5 (server-side)

**Tasks**:

- [x] **10.1.1 TLS Server Setup** ✅ **COMPLETED** (January 25, 2026)
  - Generate/load relay identity keys (Ed25519 + RSA)
  - Configure TLS server with proper cipher suites
  - Accept incoming OR connections
  - Implement TLS certificate generation per tor-spec.txt §1.1
  - Implementation: `pkg/relay/keys.go`, `pkg/relay/or_listener.go`
  - Tests: `pkg/relay/keys_test.go`, `pkg/relay/or_listener_test.go` (84.7% coverage)
  - Documentation: `docs/RELAY_IMPLEMENTATION.md`

- [x] **10.1.2 Link Protocol Server** ✅ **COMPLETED** (January 25, 2026)
  - Implemented server-side VERSIONS cell handling (receive from client, send response)
  - Implemented CERTS cell sending with relay identity certificates (TLS, RSA ID, Ed25519)
  - Implemented NETINFO cell exchange (send to client, receive from client)
  - Implemented in-protocol link version negotiation (versions 3-5 supported)
  - Added Ed25519 signing certificate generation per cert-spec.txt
  - Implementation: `pkg/relay/or_handler.go` (`LinkProtocolHandler`)
  - Tests: `pkg/relay/or_handler_test.go` (>80% coverage, 14 tests passing)
  - Integration: Enhanced `pkg/relay/or_listener.go` to use `LinkProtocolHandler`
  - Features:
    - Protocol version negotiation (highest mutual version selection)
    - Multi-certificate CERTS cell with proper encoding
    - Ed25519 signing certificate with signature validation
    - NETINFO with timestamp and address information
    - Context-aware cell reading with timeout handling

- [x] **10.1.3 Circuit Handling (Server-Side)** ✅ **COMPLETED** (January 25, 2026)
  - Accept CREATE2 cells from clients
  - Perform ntor handshake server-side
  - Send CREATED2 responses
  - Manage server-side circuit state
  - Implementation: `pkg/relay/circuit_handler.go` (`CircuitHandler`)
  - Tests: `pkg/relay/circuit_handler_test.go` (comprehensive test coverage)
  - Features:
    - Server-side ntor handshake using existing crypto infrastructure
    - Circuit state management with concurrent access protection
    - DESTROY cell handling
    - Circuit lifecycle management (create, relay, destroy)
    - Support for future RELAY cell processing (Task 10.2)

**Files Created/Modified**:
- `pkg/relay/relay.go` - Main relay server (planned)
- `pkg/relay/or_listener.go` ✅ - OR connection listener (already implemented)
- `pkg/relay/or_handler.go` ✅ - Connection handler (already implemented)
- `pkg/relay/circuit_handler.go` ✅ **NEW** - Server-side circuit handling
- `pkg/relay/circuit_handler_test.go` ✅ **NEW** - Comprehensive tests for circuit handling
- `pkg/relay/keys.go` ✅ - Enhanced with NtorOnionKey field
- `pkg/cell/cell.go` ✅ - Added DESTROY reason constants

### 10.2 Non-Exit Relay Functionality

**Specification Reference**: tor-spec.txt §5.3-5.6

**Tasks**:

- [x] **10.2.1 Circuit Extension Handling** ✅ **COMPLETED** (January 25, 2026)
  - Handle RELAY_EXTEND2 cells
  - Connect to next hop relay
  - Forward RELAY_EXTENDED2 responses
  - Implement proper encryption layer management
  - Implementation: `pkg/relay/extension.go` (`ExtensionHandler`)
  - Tests: `pkg/relay/extension_test.go` (>80% coverage for core functions)
  - Features:
    - Link specifier parsing (IPv4, IPv6, Legacy ID, Ed25519 ID)
    - Next hop connection pooling and management
    - VERSIONS cell exchange with next hop
    - CREATE2 forwarding to next hop
    - CREATED2 response reception
    - EXTENDED2 relay cell construction
    - Circuit extension registration
    - Resource cleanup and error handling
  - Documentation: `docs/CIRCUIT_EXTENSION.md`

- [x] **10.2.2 Cell Forwarding** ✅ **COMPLETED** (January 25, 2026)
  - Forward relay cells between circuits
  - Implement proper cell routing
  - Handle RELAY_EARLY cell counting (8 max per circuit direction)
  - Process DESTROY cells correctly
  - Implementation: `pkg/relay/forwarding.go` (`ForwardingHandler`)
  - Tests: `pkg/relay/forwarding_test.go` (>85% coverage, all tests passing)
  - Features:
    - Extended circuit tracking with client/next-hop mapping
    - RELAY_EARLY limiting (max 8 per circuit direction)
    - Cell forwarding between client and next hop
    - Local relay cell handling for non-extended circuits
    - Exit attempt rejection with EXITPOLICY reason
    - TRUNCATE cell handling
    - DESTROY cell forwarding and cleanup
    - Concurrent access protection with mutexes

- [x] **10.2.3 Exit Policy (Reject All)** ✅ **COMPLETED** (January 25, 2026)
  - Implement reject-all exit policy
  - Respond with RELAY_END (EXITPOLICY) for any exit attempts
  - Ensure no exit traffic is relayed
  - Implementation: `pkg/relay/policy.go` (`ExitPolicy`)
  - Tests: `pkg/relay/policy_test.go` (>90% coverage, all tests passing)
  - Features:
    - Reject-all exit policy for non-exit relays
    - Exit attempt validation for BEGIN/BEGIN_DIR commands
    - Rejected connection tracking (atomic counter)
    - ExitPolicyViolation error type with reason codes
    - Policy string generation (torrc format: "reject *:*")
    - Thread-safe exit attempt counting

**Files to Create**:
- `pkg/relay/extension.go` ✅ **NEW** - Circuit extension handling
- `pkg/relay/extension_test.go` ✅ **NEW** - Comprehensive tests for extension functionality
- `pkg/relay/forwarding.go` ✅ **NEW** - Cell forwarding logic (Task 10.2.2)
- `pkg/relay/forwarding_test.go` ✅ **NEW** - Comprehensive tests for forwarding (>85% coverage)
- `pkg/relay/policy.go` ✅ **NEW** - Exit policy enforcement (Task 10.2.3)
- `pkg/relay/policy_test.go` ✅ **NEW** - Comprehensive tests for exit policy (>90% coverage)

### 10.3 Bridge Descriptor Publishing

**Specification Reference**: dir-spec.txt §4, bridge-spec.txt

**Tasks**:

- [x] **10.3.1 Server Descriptor Generation** ✅ **COMPLETED** (January 25, 2026)
  - Generated signed server descriptors per dir-spec.txt §2.1
  - Included bridge-specific fields (DirPort=0 for bridges)
  - Supported extra-info descriptor generation with statistics
  - Implementation: `pkg/relay/descriptor.go` (`GenerateServerDescriptor`, `GenerateExtraInfo`)
  - Tests: `pkg/relay/descriptor_test.go` (19 tests, all passing)
  - Features:
    - RSA-1024 and Ed25519 identity keys
    - ntor onion key (Curve25519)
    - IPv4 and optional IPv6 addresses
    - Bandwidth advertisement (average, burst, observed)
    - Reject-all exit policy for non-exit relays
    - Relay family support
    - Contact information
    - Platform string
    - Protocol version declaration (Link=3-5, Circuit=1-2)
    - SHA-1 digest and RSA-PKCS1v15 signature
    - Descriptor validation with comprehensive error checking
    - Extra-info descriptor with custom statistics
  - Crypto helpers: Added `RSAPublicKeyToPEM` to `pkg/crypto/crypto.go`

- [x] **10.3.2 Bridge Authority Communication** ✅ **COMPLETED** (January 25, 2026)
  - Published descriptors to bridge authority via HTTP POST
  - Handled descriptor upload responses (200 OK, 202 Accepted)
  - Implemented descriptor refresh schedule with configurable intervals (default: 18h)
  - Added retry logic with exponential backoff (3 attempts per authority)
  - Implemented scheduled publisher for automatic descriptor updates
  - Features:
    - HTTP POST to /tor/ endpoint per dir-spec.txt §4.3
    - Content-Type: application/octet-stream
    - Retry mechanism with exponential backoff (5s → 60s max)
    - Support for multiple bridge authorities
    - Extra-info descriptor publishing
    - Publisher statistics tracking (last publish time, count)
    - Scheduled publishing with configurable interval
    - Graceful shutdown support
  - Implementation: `pkg/relay/publisher.go` (`DescriptorPublisher`, `ScheduledPublisher`)
  - Tests: `pkg/relay/publisher_test.go` (14 tests, all passing, >87% coverage)
  - Configuration: `PublisherConfig` with sensible defaults (18h interval, 30s timeout)

- [ ] **10.3.3 BridgeDB Integration** (Optional)
  - Support bridge distribution mechanisms
  - Implement bridge email responder integration (research/educational only)

**Files to Create**:
- `pkg/relay/descriptor.go` ✅ - Server descriptor generation (already implemented)
- `pkg/relay/publisher.go` ✅ **NEW** - Descriptor publishing (Task 10.3.2)
- `pkg/relay/publisher_test.go` ✅ **NEW** - Publisher tests (>87% coverage)
- `pkg/relay/bridge_config.go` - Bridge-specific configuration

### 10.4 Relay Security Hardening

**Tasks**:

- [x] **10.4.1 Rate Limiting** ✅ **COMPLETED** (January 25, 2026)
  - Implemented circuit creation rate limiting (token bucket, 10/sec default)
  - Implemented per-IP connection rate limiting (5/sec default)
  - Implemented per-circuit cell processing rate limiting (100/sec default)
  - Added automatic cleanup of stale limiters
  - Implementation: `pkg/relay/ratelimit.go` (`RateLimiter`)
  - Tests: `pkg/relay/ratelimit_test.go` (11 tests, all passing)
  - Features:
    - Context-aware rate limiting with graceful cancellation
    - Configurable rates and burst sizes
    - Metrics integration for tracking rate-limited operations
    - Periodic cleanup to prevent memory leaks

- [x] **10.4.2 DoS Protection** ✅ **COMPLETED** (January 25, 2026)
  - Implemented per-IP connection count limits (10 per IP default)
  - Implemented per-connection circuit count limits (1000 per connection default)
  - Implemented global connection limit (5000 total default)
  - Added automatic cleanup of stale trackers
  - Implementation: `pkg/relay/protection.go` (`ProtectionManager`)
  - Tests: `pkg/relay/protection_test.go` (11 tests, all passing)
  - Features:
    - Separate tracking for connections and circuits
    - Atomic operations for thread safety
    - Metrics integration for DoS event tracking
    - Configurable limits with sensible defaults

- [x] **10.4.3 Logging and Monitoring** ✅ **COMPLETED** (January 25, 2026)
  - Implemented comprehensive relay metrics package
  - Added circuit metrics (creation, extension, active count)
  - Added connection metrics (accepted, rejected, duration)
  - Added cell forwarding metrics (received, forwarded, dropped)
  - Added bandwidth metrics (bytes received/transmitted)
  - Added rate limiting and DoS protection metrics
  - Added error tracking metrics (handshake, protocol, extension)
  - Implementation: `pkg/relay/metrics.go` (`RelayMetrics`)
  - Tests: `pkg/relay/metrics_test.go` (14 tests, 100% coverage)
  - Features:
    - Thread-safe Counter, Gauge, and Histogram types
    - Snapshot functionality for point-in-time metrics
    - Uptime tracking
    - Comprehensive metric categories

**Files Created**:
- `pkg/relay/ratelimit.go` ✅ - Relay rate limiting (177 lines)
- `pkg/relay/ratelimit_test.go` ✅ - Rate limiting tests (11 tests, all passing)
- `pkg/relay/protection.go` ✅ - DoS protection (288 lines)
- `pkg/relay/protection_test.go` ✅ - DoS protection tests (11 tests, all passing)
- `pkg/relay/metrics.go` ✅ - Relay metrics (336 lines)
- `pkg/relay/metrics_test.go` ✅ - Relay metrics tests (14 tests, 100% coverage)

**Dependencies Added**:
- `golang.org/x/time/rate` v0.14.0 - Token bucket rate limiting

**Test Coverage**: 
- Overall relay package: 79.1%
- New files: >85% average coverage
- metrics.go: 100% coverage

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

- [x] `docs/ONION_SERVICE_HOSTING.md` - Complete service hosting guide ✅ **COMPLETED** (January 25, 2026)
- [x] `docs/LINK_PROTOCOL_SERVER.md` - Server-side link protocol implementation ✅ **COMPLETED** (January 25, 2026)
- [x] `docs/RELAY_SECURITY.md` - Relay security hardening (rate limiting, DoS protection, metrics) ✅ **COMPLETED** (January 25, 2026)
- [x] `docs/BRIDGE_RELAY.md` - Bridge relay setup and operation ✅ **COMPLETED** (January 25, 2026)
- [ ] `docs/PLUGGABLE_TRANSPORTS.md` - PT configuration and usage

### Updates to Existing Docs

- [x] `docs/ARCHITECTURE.md` - Add relay and onion service server architecture ✅ **COMPLETED** (January 25, 2026)
  - Added relay mode system architecture diagram
  - Added onion service mode system architecture diagram
  - Added pkg/relay package description with all features
  - Enhanced pkg/onion description with server features
  - Added relay-specific data flows (circuit extension, cell forwarding)
  - Added onion service data flows (introduction, rendezvous)
  - Updated Phase 9 and Phase 10 completion status
  - Updated overview to reflect all three operating modes
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

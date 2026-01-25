# Onion Service Hosting Guide

## Overview

This guide provides comprehensive documentation for hosting v3 onion services using go-tor. An onion service (formerly "hidden service") allows you to host services on the Tor network with location privacy and end-to-end encryption.

**Status**: Phase 9 Complete (January 2026)
- ✅ Introduction Point Protocol
- ✅ INTRODUCE2 Handling
- ✅ Rendezvous Circuit Building
- ✅ RENDEZVOUS1 Construction
- ✅ Stream Handling
- ✅ Service Persistence

## Table of Contents

1. [Architecture](#architecture)
2. [Quick Start](#quick-start)
3. [Configuration](#configuration)
4. [Service Lifecycle](#service-lifecycle)
5. [Introduction Points](#introduction-points)
6. [Client Connections](#client-connections)
7. [Stream Management](#stream-management)
8. [Persistence](#persistence)
9. [Monitoring](#monitoring)
10. [Security Considerations](#security-considerations)
11. [Troubleshooting](#troubleshooting)
12. [API Reference](#api-reference)

## Architecture

### v3 Onion Service Protocol

The implementation follows [rend-spec-v3.txt](https://spec.torproject.org/rend-spec-v3) with complete support for:

```
Client                    Service                 Introduction Point      Rendezvous Point
  |                         |                            |                         |
  |-- Fetch Descriptor ---->|                            |                         |
  |<---- Descriptor --------|                            |                         |
  |                         |                            |                         |
  |---- INTRODUCE1 -------->|---- INTRODUCE2 ----------->|                         |
  |                         |<---- INTRO_ACK ------------|                         |
  |                         |                                                      |
  |                         |------------ RENDEZVOUS1 ------------------------->   |
  |<--------------------- RENDEZVOUS2 --------------------------------------------|
  |                         |                                                      |
  |<==================== END-TO-END ENCRYPTED STREAM ==========================> |
```

### Components

1. **Identity Management** (`pkg/onion/service.go`)
   - Ed25519 identity key (permanent service identity)
   - Curve25519 ntor onion key (ephemeral for handshakes)
   - Onion address derivation (base32-encoded public key + checksum)

2. **Introduction Point Protocol** (`pkg/onion/intro_protocol.go`)
   - Circuit building with retry and backoff
   - ESTABLISH_INTRO cell construction and MAC computation
   - Health checking and automatic rotation
   - Configurable intro point count (default: 3, range: 1-10)

3. **Descriptor Management** (`pkg/onion/service.go`)
   - v3 descriptor creation with Ed25519 signatures
   - Certificate generation per cert-spec.txt
   - Publishing to responsible HSDirs (6 replicas)
   - Automatic refresh before expiration (default: 3h lifetime)

4. **Connection Handling** (`pkg/onion/introduce2.go`, `rendezvous.go`, `rendezvous1.go`)
   - INTRODUCE2 cell parsing and decryption
   - Rendezvous circuit building to client-specified relay
   - Server-side ntor handshake (`pkg/crypto/ntor_server.go`)
   - RENDEZVOUS1 construction with handshake response
   - End-to-end key derivation

5. **Stream Management** (`pkg/onion/service_stream.go`)
   - RELAY_BEGIN handling
   - Backend connection pooling
   - Bidirectional data forwarding
   - RELAY_END cleanup
   - Connection statistics

6. **Persistence** (`pkg/onion/persistence.go`)
   - Secure key storage (0600 permissions)
   - State persistence across restarts
   - Descriptor revision tracking
   - Export/import for backups

## Quick Start

### Basic Example

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/opd-ai/go-tor/pkg/logger"
    "github.com/opd-ai/go-tor/pkg/onion"
)

func main() {
    // Create logger
    lg := logger.NewDefault()

    // Configure service
    config := &onion.ServiceConfig{
        Ports: map[int]string{
            80: "localhost:8080", // Map virtual port 80 to local HTTP server
        },
        NumIntroPoints:     3,
        DescriptorLifetime: 3 * time.Hour,
        DataDirectory:      "/var/lib/myservice",
    }

    // Create service (loads existing keys if present)
    service, err := onion.NewService(config, lg)
    if err != nil {
        log.Fatal(err)
    }

    // Start hosting
    ctx := context.Background()
    hsdirs := getHSDirsFromConsensus() // Your directory client
    if err := service.Start(ctx, hsdirs); err != nil {
        log.Fatal(err)
    }

    log.Printf("Service online: %s", service.GetAddress())
    
    // Keep running
    select {}
}
```

## Configuration

### ServiceConfig Structure

```go
type ServiceConfig struct {
    // Service identity (if nil, generates new or loads from DataDirectory)
    PrivateKey ed25519.PrivateKey

    // Port mappings: virtual_port -> local_address
    // Virtual ports are advertised to clients
    // Local addresses are where traffic is forwarded
    Ports map[int]string

    // Number of introduction points (1-10, default: 3)
    // More points = better availability, but more overhead
    NumIntroPoints int

    // Descriptor validity period (default: 3h, min: 30m, max: 12h)
    DescriptorLifetime time.Duration

    // Directory for persistent state
    // Stores: identity keys, ntor keys, service state
    DataDirectory string

    // Client authorization keys (optional, for private services)
    // Map of client public keys to descriptive names
    AuthorizedClients map[[32]byte]string

    // Maximum concurrent streams (default: 1000)
    MaxStreams int

    // Stream idle timeout (default: 5m)
    StreamTimeout time.Duration
}
```

### Port Mapping Examples

```go
// Single HTTP service
Ports: map[int]string{
    80: "localhost:8080",
}

// HTTP + HTTPS
Ports: map[int]string{
    80:  "localhost:8080",
    443: "localhost:8443",
}

// Custom application ports
Ports: map[int]string{
    9000: "127.0.0.1:9000",
    9001: "127.0.0.1:9001",
}

// Forward to remote host (use with caution - defeats location privacy)
Ports: map[int]string{
    80: "192.168.1.100:80",
}
```

## Service Lifecycle

### 1. Creation

```go
service, err := onion.NewService(config, logger)
```

**Actions**:
- Loads existing identity key from `DataDirectory/identity.key` if present
- Generates new Ed25519 identity if no key found
- Generates new Curve25519 ntor onion key
- Derives onion address from public key
- Initializes service state
- Creates stream manager

**Persistence**: Keys are automatically saved to DataDirectory with 0600 permissions.

### 2. Starting

```go
err := service.Start(ctx, hsdirs)
```

**Actions**:
1. **Introduction Point Establishment** (parallel, ~5s per point)
   - Selects relays with HSDir flag
   - Builds 3-hop circuits to each intro point
   - Sends ESTABLISH_INTRO cells
   - Waits for INTRO_ESTABLISHED confirmation
   - Retries failed points with exponential backoff

2. **Descriptor Creation**
   - Constructs v3 descriptor with intro point info
   - Signs descriptor with Ed25519 identity key
   - Generates certificates per cert-spec.txt
   - Increments revision counter

3. **Descriptor Publishing**
   - Computes responsible HSDirs from consensus
   - Uploads descriptor to 6 HSDir replicas
   - Tracks publication success/failure

4. **Maintenance Loop**
   - Monitors intro point health
   - Rotates unhealthy intro points
   - Refreshes descriptor before expiration
   - Handles INTRODUCE2 cells

### 3. Running

While running, the service:

- **Accepts Introduction Requests**
  - Receives INTRODUCE2 cells from clients
  - Decrypts client rendezvous info
  - Builds circuits to rendezvous points

- **Completes Rendezvous**
  - Performs server-side ntor handshake
  - Sends RENDEZVOUS1 to client
  - Derives shared keys for encryption

- **Handles Streams**
  - Processes RELAY_BEGIN from clients
  - Connects to local backend services
  - Forwards data bidirectionally
  - Manages stream lifecycle

### 4. Monitoring

```go
stats := service.GetStats()
// Returns: status, intro point count, descriptor age, pending intros
```

### 5. Stopping

```go
err := service.Stop()
```

**Actions**:
- Stops maintenance loop
- Closes all active streams
- Tears down introduction circuits
- Saves service state to DataDirectory
- Cleans up resources

**Persistence**: State is saved including intro point cache, descriptor revision, and timestamp.

## Introduction Points

### Selection Criteria

Introduction points are selected from relays with:
- HSDir flag (can store descriptors)
- Stable flag (reliable uptime)
- Fast flag (adequate bandwidth)
- Not already selected for this service

### Circuit Building

```go
// From pkg/onion/intro_protocol.go
func BuildIntroCircuitWithRetry(
    ctx context.Context,
    builder CircuitBuilderInterface,
    selector PathSelectorInterface,
    relay *Relay,
    maxRetries int,
) (CircuitInterface, error)
```

**Process**:
1. Select 3-hop path (guard → middle → intro point)
2. Build circuit using existing circuit builder
3. Retry with exponential backoff on failure (1s, 2s, 4s, 8s, 16s)
4. Monitor circuit health after establishment

### ESTABLISH_INTRO Protocol

```go
// Cell format per rend-spec-v3.txt §3.1.1
ESTABLISH_INTRO {
    AUTH_KEY_TYPE   [1 byte]    = 0x02 (Ed25519)
    AUTH_KEY_LEN    [2 bytes]   = 0x0020
    AUTH_KEY        [32 bytes]  = intro auth public key
    N_EXTENSIONS    [1 byte]    = 0 or 1
    EXTENSIONS      [variable]  = DoS parameters (optional)
    HANDSHAKE_AUTH  [MAC_LEN]   = MAC(cell contents)
    SIG_LEN         [2 bytes]
    SIG             [SIG_LEN]   = Ed25519 signature
}
```

**MAC Computation**:
```
MAC = HMAC-SHA256(
    key = KDF(ntor_key_seed, "intro-mac"),
    msg = "Tor establish-intro cell" || cell_contents
)
```

### Health Monitoring

The `IntroPointManager` tracks:
- Circuit liveness (periodic PADDING cells)
- INTRODUCE2 delivery success rate
- Last activity timestamp
- Failure counters

**Rotation Triggers**:
- Circuit failure or closure
- Excessive INTRODUCE2 failures (>10)
- Scheduled rotation (default: 24h)
- Below minimum intro point count

## Client Connections

### INTRODUCE2 Handling

When a client sends INTRODUCE1 → intro point forwards INTRODUCE2:

```go
// From pkg/onion/introduce2.go
type INTRODUCE2Data struct {
    RendezvousCookie [20]byte     // Client's cookie for RP
    OnionKey         []byte        // Client's ntor public key
    LinkSpecifiers   []LinkSpec    // How to reach rendezvous point
    ClientAuthKey    []byte        // Optional client auth key
}
```

**Decryption** (AES-256-CTR):
```go
plaintext := DecryptAES256CTR(
    ciphertext,
    key = KDF(intro_auth_secret, "intro-decrypt"),
    iv = zeroes
)
```

### Rendezvous Circuit Building

```go
// From pkg/onion/rendezvous.go
func BuildRendezvousCircuit(
    ctx context.Context,
    builder CircuitBuilderInterface,
    selector PathSelectorInterface,
    linkSpecs []LinkSpecifier,
    timeout time.Duration,
) (CircuitInterface, error)
```

**Process**:
1. Parse link specifiers (IPv4, IPv6, Ed25519 ID, legacy ID)
2. Select 2-hop path (guard → middle)
3. Extend to rendezvous point using link specs
4. Total: 3-hop circuit to rendezvous

### RENDEZVOUS1 Construction

```go
// From pkg/onion/rendezvous1.go
func SendRendezvous1(
    circuit CircuitInterface,
    cookie [20]byte,
    clientNtorKey *[32]byte,
    serverNtorKey *[32]byte,
) (sharedSecret []byte, err error)
```

**Server-Side ntor Handshake**:
```go
// From pkg/crypto/ntor_server.go
response, sharedSecret := NtorServerHandshake(
    clientPublicKey,
    serverPrivateKey,
    serverPublicKey,
    identityKey,
)
```

**RENDEZVOUS1 Cell** (per rend-spec-v3.txt §3.2.2):
```
RENDEZVOUS1 {
    COOKIE          [20 bytes]  = Client's rendezvous cookie
    HANDSHAKE_INFO  [variable]  = ntor handshake response
}
```

**Key Derivation**:
```
K = HKDF-SHA256(
    sharedSecret,
    info = "Tor onion service ntor key expand",
    length = 92
)

Split K into:
- Kf (forward digest) [20 bytes]
- Kb (backward digest) [20 bytes]
- Kenc_f (forward encryption) [16 bytes]
- Kenc_b (backward encryption) [16 bytes]
- Additional key material [20 bytes]
```

## Stream Management

### RELAY_BEGIN Handling

```go
// From pkg/onion/service_stream.go
type ServiceStreamManager struct {
    service      *Service
    streams      map[uint16]*ServiceStream
    maxStreams   int
    streamTimeout time.Duration
}
```

**Process**:
1. Receive RELAY_BEGIN on rendezvous circuit
2. Parse target address (should be "localhost:port" or virtual port)
3. Map virtual port to backend address using ServiceConfig.Ports
4. Connect to backend with timeout (default: 10s)
5. Send RELAY_CONNECTED to client
6. Start bidirectional forwarding

### Backend Connection

```go
func (s *ServiceStream) connectToBackend(address string) error {
    // TCP dial with timeout
    conn, err := net.DialTimeout("tcp", address, 10*time.Second)
    if err != nil {
        return err
    }
    s.backend = conn
    return nil
}
```

### Data Forwarding

**Circuit → Backend**:
```go
func (s *ServiceStream) forwardFromCircuit(data []byte) error {
    _, err := s.backend.Write(data)
    return err
}
```

**Backend → Circuit**:
```go
func (s *ServiceStream) forwardToCircuit() {
    buffer := make([]byte, 498) // RELAY_DATA payload size
    for {
        n, err := s.backend.Read(buffer)
        if n > 0 {
            s.circuit.SendRelayData(s.streamID, buffer[:n])
        }
        if err != nil {
            break
        }
    }
}
```

### Stream Cleanup

```go
func (s *ServiceStream) Close() error {
    // Send RELAY_END to client
    s.circuit.SendRelayEnd(s.streamID, END_REASON_DONE)
    
    // Close backend connection
    if s.backend != nil {
        s.backend.Close()
    }
    
    // Remove from manager
    delete(s.manager.streams, s.streamID)
    
    return nil
}
```

## Persistence

### File Structure

```
DataDirectory/
├── identity.key        # Ed25519 private key (64 bytes + version)
├── ntor.key           # Curve25519 private key (32 bytes + version)
└── service.state      # JSON state file
```

### Identity Key Format

```
Version: 1
Key Format: Raw Ed25519 seed (64 bytes)
File Permissions: 0600 (owner read/write only)
```

```go
// From pkg/onion/persistence.go
func SaveIdentityKey(path string, key ed25519.PrivateKey) error {
    data := append([]byte{0x01}, key...) // Version byte + key
    return os.WriteFile(path, data, 0600)
}
```

### Service State Format

```json
{
  "version": 1,
  "address": "abcdef...xyz.onion",
  "created_at": "2026-01-25T10:00:00Z",
  "descriptor_revision": 42,
  "last_descriptor_publish": "2026-01-25T13:00:00Z",
  "intro_points": [
    {
      "identity": "0123456789ABCDEF...",
      "address": "203.0.113.1:9001",
      "established_at": "2026-01-25T10:05:00Z"
    }
  ]
}
```

### State Management

```go
// Automatically saved on:
// 1. Service Stop()
// 2. After descriptor publish
// 3. After intro point rotation

func (s *Service) saveState() error {
    state := &ServiceState{
        Version:              1,
        Address:              s.GetAddress(),
        CreatedAt:            s.createdAt,
        DescriptorRevision:   s.descriptorRevision,
        LastDescriptorPublish: s.lastDescriptorPublish,
        IntroPoints:          s.getIntroPointCache(),
    }
    return s.persistence.SaveState(state)
}
```

### Secure Deletion

```go
// From pkg/onion/persistence.go
func SecureDelete(path string) error {
    // 3-pass overwrite:
    // 1. Random data
    // 2. Complement of random data
    // 3. Random data again
    // Then delete file
}
```

## Monitoring

### Service Statistics

```go
type ServiceStats struct {
    Status              string    // "running" or "stopped"
    Address             string    // .onion address
    NumIntroPoints      int       // Currently established intro points
    DescriptorAge       time.Duration
    PendingIntros       int       // Queued INTRODUCE2 cells
    TotalIntrosReceived int64
    TotalRendezvous     int64
    ActiveStreams       int
    TotalBytesReceived  int64
    TotalBytesSent      int64
}

stats := service.GetStats()
```

### Metrics

The implementation tracks OpenTelemetry metrics:

```go
// From pkg/metrics/metrics.go

// Counters
OnionServiceDescriptorPublished  // Total descriptors published
OnionServiceDescriptorFailed     // Publication failures
OnionServiceIntroEstablished     // Intro points established
OnionServiceIntroFailed          // Intro establishment failures
OnionServiceIntroReceived        // INTRODUCE2 cells received
OnionServiceRendezvousSuccess    // Successful rendezvous
OnionServiceRendezvousFailed     // Failed rendezvous
OnionServiceStreamBytesReceived  // Bytes received from clients
OnionServiceStreamBytesSent      // Bytes sent to clients

// Gauges
OnionServiceStreamsActive        // Current active streams
OnionServiceIntroPointCount      // Current intro point count
OnionServiceUptime               // Service uptime in seconds
```

### Health Checks

```go
// Check if service is healthy
func (s *Service) IsHealthy() bool {
    return s.running &&
           s.NumIntroPoints() >= s.config.NumIntroPoints &&
           time.Since(s.lastDescriptorPublish) < s.config.DescriptorLifetime
}
```

## Security Considerations

### ⚠️ Important Warnings

**This is experimental software for educational purposes.**

**DO NOT use for production anonymity needs.** Use official Tor software:
- **Users**: [Tor Browser](https://www.torproject.org/download/)
- **Developers**: [Arti](https://gitlab.torproject.org/tpo/core/arti)

### Key Security

- **Private Key Protection**: Store identity keys with 0600 permissions
- **Key Backup**: Export and securely backup identity keys offline
- **Key Generation**: Use crypto/rand for all key generation (already implemented)
- **Key Rotation**: Do NOT rotate identity keys (changes .onion address)

### Network Security

- **Location Privacy**: Service IP is hidden, but ensure backend services don't leak it
- **End-to-End Encryption**: All connections use Tor's encryption, but configure TLS for backend
- **Client Authentication**: For private services, use authorized_clients configuration
- **Rate Limiting**: Currently not implemented - limit at backend service level

### DoS Protection

**Current Limitations**:
- No introduction rate limiting
- No circuit creation limits
- No stream creation limits

**Mitigations**:
- Configure MaxStreams in ServiceConfig
- Monitor metrics for abnormal activity
- Implement rate limiting in backend service
- Use client authorization for private services

### Best Practices

1. **Separation of Concerns**
   - Run service in isolated container/VM
   - Limit backend service exposure (bind to localhost only)
   - Use firewall rules to prevent leaks

2. **Monitoring**
   - Log all INTRODUCE2 cells (without PII)
   - Alert on abnormal intro point failures
   - Track descriptor publication success rate
   - Monitor stream count and bandwidth

3. **Updates**
   - Keep go-tor updated for security patches
   - Monitor security advisories for Tor protocol
   - Test updates in staging environment

4. **Incident Response**
   - Have procedure for key compromise
   - Plan for service migration
   - Document disaster recovery steps

## Troubleshooting

### Service Won't Start

**Symptom**: `Start()` returns error

**Common Causes**:
1. Cannot establish introduction points
   - Check network connectivity to Tor relays
   - Verify directory client is working
   - Check logs for circuit build failures

2. Descriptor publishing fails
   - Verify HSDirs are reachable
   - Check clock synchronization (±30min tolerance)
   - Ensure descriptor signature is valid

**Solutions**:
```go
// Enable debug logging
lg := logger.New(slog.LevelDebug, os.Stdout)

// Increase timeouts
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

// Retry Start() with exponential backoff
```

### No Client Connections

**Symptom**: Service starts but clients cannot connect

**Common Causes**:
1. Descriptor not propagated
   - Wait 5-10 minutes for propagation
   - Check descriptor publication metrics
   - Verify HSDirs have HSDir flag

2. Introduction points failed
   - Check intro point count (should be ≥ 1)
   - Verify circuits to intro points are alive
   - Check intro point rotation logs

3. Backend service not running
   - Verify local service is listening
   - Test backend connection locally
   - Check port mapping configuration

**Debug**:
```go
stats := service.GetStats()
log.Printf("Intro Points: %d", stats.NumIntroPoints)
log.Printf("Descriptor Age: %s", stats.DescriptorAge)
log.Printf("Pending Intros: %d", stats.PendingIntros)
```

### High CPU/Memory Usage

**Symptom**: Service consumes excessive resources

**Common Causes**:
1. Too many concurrent streams
   - Check `ActiveStreams` metric
   - Reduce `MaxStreams` configuration
   - Implement backend rate limiting

2. Introduction point rotation thrashing
   - Check intro point failure rate
   - Increase rotation interval
   - Verify network stability

3. Memory leaks in stream handling
   - Monitor stream cleanup (RELAY_END sent)
   - Check for blocked goroutines
   - Review backend connection pooling

### Slow Connections

**Symptom**: High latency for client connections

**Common Causes**:
1. Long rendezvous circuit paths
   - Normal for Tor (6+ hops total)
   - Consider using vanguards for stability
   - Optimize backend service performance

2. Introduction point congestion
   - Rotate introduction points more frequently
   - Increase `NumIntroPoints` (more options for clients)

3. Backend service bottleneck
   - Profile backend service
   - Add caching layer
   - Scale backend horizontally

## API Reference

### Service Creation

```go
func NewService(
    config *ServiceConfig,
    logger *slog.Logger,
) (*Service, error)
```

Creates a new onion service instance. Loads existing keys from DataDirectory if present, otherwise generates new identity.

### Service Control

```go
func (s *Service) Start(
    ctx context.Context,
    hsdirs []*HSDirectory,
) error
```

Starts the service: establishes intro points, publishes descriptor, starts maintenance loop.

```go
func (s *Service) Stop() error
```

Gracefully stops the service: closes streams, tears down circuits, saves state.

### Information

```go
func (s *Service) GetAddress() string
```

Returns the .onion address (56 characters + ".onion" = 62 total).

```go
func (s *Service) GetStats() *ServiceStats
```

Returns current service statistics.

### Persistence

```go
func (s *Service) ExportKeys() (identity, ntor []byte, err error)
```

Exports private keys for backup.

```go
func (s *Service) ImportKeys(identity, ntor []byte) error
```

Imports private keys from backup.

### Testing Helpers

```go
func (s *Service) SetCircuitBuilder(builder CircuitBuilderInterface)
func (s *Service) SetPathSelector(selector PathSelectorInterface)
func (s *Service) SetRendezvousCircuitBuilder(builder RendezvousCircuitBuilderInterface)
```

Inject dependencies for testing.

## Examples

See `examples/onion-service-demo/` for basic usage and `examples/onion-service-persistence/` for persistence examples.

## Related Documentation

- [INTRO_POINT_PROTOCOL.md](INTRO_POINT_PROTOCOL.md) - Introduction point details
- [INTRODUCE2_PARSING.md](INTRODUCE2_PARSING.md) - INTRODUCE2 cell format
- [RENDEZVOUS_CIRCUIT_BUILDING.md](RENDEZVOUS_CIRCUIT_BUILDING.md) - Rendezvous circuits
- [RENDEZVOUS1_IMPLEMENTATION.md](RENDEZVOUS1_IMPLEMENTATION.md) - RENDEZVOUS1 cells
- [STREAM_HANDLING.md](STREAM_HANDLING.md) - Stream management
- [ONION_SERVICE_PERSISTENCE.md](ONION_SERVICE_PERSISTENCE.md) - State persistence
- [ONION_SERVICE_INTEGRATION.md](ONION_SERVICE_INTEGRATION.md) - Integration guide

## References

- [rend-spec-v3.txt](https://spec.torproject.org/rend-spec-v3) - v3 Onion Service Specification
- [tor-spec.txt](https://spec.torproject.org/tor-spec) - Tor Protocol Specification
- [cert-spec.txt](https://spec.torproject.org/cert-spec) - Certificate Format Specification
- [Onion Services Best Practices](https://community.torproject.org/onion-services/)

---

**Document Version**: 1.0  
**Last Updated**: January 2026  
**Status**: Phase 9 Complete

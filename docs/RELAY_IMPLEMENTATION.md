# Tor Relay Implementation

This document describes the Tor relay (bridge/non-exit) implementation in go-tor.

## ⚠️ Important Notice

This is an **unofficial, experimental implementation** developed for educational and research purposes. This software has been developed **without the supervision or endorsement of The Tor Project**.

**This software should NOT be used as a production relay.**

For running actual Tor relays:
- Use the official [tor](https://gitlab.torproject.org/tpo/core/tor) implementation (C)
- Or use [Arti](https://gitlab.torproject.org/tpo/core/arti) (official Rust implementation)

## Overview

The relay package (`pkg/relay`) provides server-side OR (Onion Router) protocol support. This allows the software to accept incoming Tor connections and relay traffic (non-exit only).

### Current Implementation Status

**Phase 10.1.1: TLS Server Setup** ✅ **COMPLETED** (January 25, 2026)

- ✅ Relay identity key generation (Ed25519 + RSA 1024-bit)
- ✅ Self-signed TLS certificate generation per tor-spec.txt §1.1
- ✅ Secure key persistence with atomic file writes
- ✅ TLS server listener with proper cipher suites
- ✅ Connection management and rate limiting
- ✅ Test coverage: 84.7%

## Components

### 1. Relay Keys (`pkg/relay/keys.go`)

Manages cryptographic keys for relay operation:

```go
type RelayKeys struct {
    Ed25519Public  ed25519.PublicKey  // 32-byte identity key (public)
    Ed25519Private ed25519.PrivateKey // 64-byte identity key (private)
    RSAPrivate     *rsa.PrivateKey    // 1024-bit RSA identity key
    TLSCert        []byte             // DER-encoded X.509 certificate
}
```

**Key Generation:**
```go
keys, err := relay.GenerateRelayKeys()
if err != nil {
    log.Fatal(err)
}
defer keys.Destroy() // Securely zero keys when done
```

**Key Persistence:**
```go
// Save keys to disk
err = keys.SaveKeys("/var/lib/tor/keys")

// Load keys from disk
keys, err = relay.LoadKeys("/var/lib/tor/keys")
```

**Fingerprints:**
```go
rsaFP := keys.Fingerprint()          // SHA-1 of RSA public key (40 hex chars)
ed25519FP := keys.Ed25519Fingerprint() // Hex-encoded Ed25519 key (64 hex chars)
```

### 2. OR Listener (`pkg/relay/or_listener.go`)

Accepts incoming OR protocol connections over TLS:

```go
// Create listener configuration
cfg := relay.DefaultORListenerConfig(":9001", keys)
cfg.MaxConnections = 1000
cfg.ReadTimeout = 60 * time.Second
cfg.WriteTimeout = 60 * time.Second

// Create listener
listener, err := relay.NewORListener(cfg, logger)
if err != nil {
    log.Fatal(err)
}

// Start accepting connections
ctx := context.Background()
err = listener.Start(ctx)
if err != nil {
    log.Fatal(err)
}

// Stop listener (graceful shutdown)
listener.Stop()
```

**Features:**
- TLS 1.2+ with secure cipher suites (ECDHE + AEAD)
- Connection counting and limits
- Context-aware shutdown
- Per-connection read/write timeouts

## Security Considerations

### Key Management

1. **Key Generation**: Uses `crypto/rand` for cryptographically secure random number generation
2. **File Permissions**: 
   - Private keys: `0600` (owner read/write only)
   - Certificates: `0644` (world-readable)
3. **Secure Deletion**: `Destroy()` method zeros memory using `security.SecureZeroMemory()`
4. **Atomic Writes**: Keys are written to temp files then renamed atomically

### TLS Configuration

Per tor-spec.txt §1.1, relays use:
- Self-signed certificates with 1-year validity
- Organization: "Tor"
- CommonName: "www.torproject.org"
- KeyUsage: KeyEncipherment | DigitalSignature
- ExtKeyUsage: ServerAuth

**Cipher Suites** (forward secrecy required):
```
TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305
TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305
```

No CBC-mode ciphers (vulnerable to Lucky13, POODLE).

### Rate Limiting

The OR listener supports:
- Maximum concurrent connections (`MaxConnections`)
- Per-connection read/write timeouts
- Graceful connection rejection when limit reached

## File Structure

Relay keys are stored in the data directory with the following structure:

```
<DataDirectory>/
  ├── ed25519_identity_secret_key  # Raw 64-byte Ed25519 private key (0600)
  ├── rsa_identity_secret_key      # PEM-encoded RSA private key (0600)
  └── tls_certificate.pem          # PEM-encoded X.509 certificate (0644)
```

## Testing

Run relay tests:
```bash
go test ./pkg/relay/... -cover
```

Current coverage: **84.7%** (keys, listener), **87%+** (publisher)

## Descriptor Publishing

### Overview

Bridge relays publish server descriptors to bridge authorities via HTTP POST. The publisher handles automatic descriptor refresh and retry logic.

### Basic Usage

```go
// Create publisher configuration
pubConfig := relay.PublisherConfig{
    Authorities: relay.DefaultBridgeAuthorities,
    PublishInterval: 18 * time.Hour,  // Refresh every 18 hours
    HTTPTimeout: 30 * time.Second,
    RetryAttempts: 3,
}

// Create publisher
publisher := relay.NewDescriptorPublisher(pubConfig, logger)

// Generate descriptor
descriptor, err := relay.GenerateServerDescriptor(keys, &relay.DescriptorConfig{
    Nickname: "MyBridge",
    Address:  "192.0.2.1",
    ORPort:   9001,
    DirPort:  0,  // Bridges don't run directory servers
    IsBridge: true,
})

// Publish once
ctx := context.Background()
successCount, err := publisher.PublishDescriptor(ctx, descriptor)
if err != nil {
    log.Printf("Failed to publish: %v", err)
}
log.Printf("Published to %d authorities", successCount)
```

### Scheduled Publishing

For automatic descriptor refresh:

```go
// Generator function that creates fresh descriptors
generateFunc := func() (*relay.ServerDescriptor, *relay.ExtraInfoDescriptor, error) {
    desc, err := relay.GenerateServerDescriptor(keys, config)
    if err != nil {
        return nil, nil, err
    }
    
    // Optional extra-info with statistics
    extraInfo, _ := relay.GenerateExtraInfo(keys, desc, map[string]string{
        "bridge-stats-end": time.Now().Format("2006-01-02 15:04:05"),
        "bridge-ips": "42",
    })
    
    return desc, extraInfo, nil
}

// Create scheduled publisher
scheduler := relay.NewScheduledPublisher(
    publisher,
    18 * time.Hour,  // Publish interval
    generateFunc,
    logger,
)

// Start scheduled publishing
err := scheduler.Start(ctx)
if err != nil {
    log.Fatal(err)
}

// Stop when done
defer scheduler.Stop()
```

### Features

- **Automatic Retry**: Exponential backoff (5s → 60s max) with configurable attempts
- **Multiple Authorities**: Publishes to all configured bridge authorities
- **Success Tracking**: Returns count of successful publications
- **Extra-Info Support**: Optional extra-info descriptor with statistics
- **Graceful Shutdown**: Context-aware cancellation
- **Statistics**: Track last publish time and total publish count

### Configuration

```go
type PublisherConfig struct {
    Authorities      []BridgeAuthority // Bridge authorities to publish to
    PublishInterval  time.Duration     // Interval between publishes (default: 18h)
    HTTPTimeout      time.Duration     // HTTP request timeout (default: 30s)
    RetryAttempts    int               // Retry attempts per authority (default: 3)
    RetryDelay       time.Duration     // Initial retry delay (default: 5s)
    MaxRetryDelay    time.Duration     // Maximum retry delay (default: 60s)
}

type BridgeAuthority struct {
    Address string // IP:Port
    URL     string // HTTP URL for uploads (e.g., "http://authority/tor/")
}
```

### HTTP Protocol

Per dir-spec.txt §4.3, descriptors are POSTed to `/tor/` endpoint:

```
POST /tor/ HTTP/1.1
Host: 86.59.21.38
Content-Type: application/octet-stream
Content-Length: <descriptor-length>

<server-descriptor-text>
```

**Success Responses**: `200 OK` or `202 Accepted`

## Next Steps

Phase 10.3.3: BridgeDB Integration (Optional)
- Support bridge distribution mechanisms
- Implement bridge email responder integration (research/educational only)

Phase 10.4: Relay Security Hardening (Pending)
- Rate limiting for circuit creation and connections
- DoS protection mechanisms
- Relay-specific monitoring and metrics

## References

- [tor-spec.txt](https://spec.torproject.org/tor-spec) - Tor protocol specification
- [dir-spec.txt](https://spec.torproject.org/dir-spec) - Directory protocol
- [bridge-spec.txt](https://spec.torproject.org/bridge-spec) - Bridge specification

## License

See LICENSE file for details.

---

**Last Updated**: January 25, 2026  
**Implementation Status**: Phase 10.3.2 Complete (Bridge Descriptor Publishing)

# Certificate Pinning Enhancement

## Overview

This document describes the certificate pinning enhancement implemented to improve MITM attack resistance per tor-spec.txt §2. The enhancement automatically validates relay identities against the directory consensus during circuit construction.

## Implementation

### What Changed

**Before**: Connections to relays used basic TLS validation without verifying relay identity from the consensus.

**After**: The circuit builder automatically configures certificate pinning for each relay using identity information from the directory consensus:
- Ed25519 identity key (32 bytes)
- RSA fingerprint (40 hex characters)
- Strict CERTS validation mode enabled

### Key Components

#### 1. Enhanced Connection Configuration (`pkg/circuit/builder.go`)

The `connectToRelay` method now accepts relay information and configures certificate pinning:

```go
func (b *Builder) connectToRelay(ctx context.Context, address string, relay *directory.Relay) (*connection.Connection, error) {
    cfg := connection.DefaultConfig(address)
    
    if relay != nil {
        // Set expected Ed25519 identity from consensus
        if len(relay.IdentityKey) == 32 {
            cfg.ExpectedIdentity = relay.IdentityKey
        }
        
        // Set expected RSA fingerprint from consensus
        if relay.Fingerprint != "" {
            cfg.ExpectedFingerprint = relay.Fingerprint
        }
        
        // Enable strict CERTS validation
        cfg.RequireCERTS = true
    }
    
    // ... connection establishment
}
```

#### 2. CERTS Cell Validation (`pkg/protocol/certs.go`)

The `ValidateRelayIdentity` method verifies the relay's certificates match consensus:

```go
func (c *CERTSCell) ValidateRelayIdentity(expectedRSAFingerprint string, expectedEd25519Identity []byte) error {
    // Verify RSA fingerprint matches consensus
    // Verify Ed25519 identity matches consensus
    // Return error if mismatch detected
}
```

#### 3. Link Protocol Handshake (`pkg/protocol/protocol.go`)

The handshake validates CERTS cells in strict mode:

```go
func (h *Handshake) receiveCERTS(ctx context.Context) error {
    // Parse CERTS cell
    // Validate expiration
    // Validate signatures
    // Validate relay identity against expected values
    // Fail handshake if RequireCERTS=true and validation fails
}
```

## Security Benefits

### 1. MITM Attack Prevention

An adversary cannot present a valid self-signed certificate for a different relay's identity. The implementation verifies:
- The relay's Ed25519 identity key matches the consensus
- The relay's RSA fingerprint matches the consensus
- Certificate signatures are cryptographically valid

### 2. Defense in Depth

Certificate pinning provides multiple layers of validation:
1. **TLS Layer**: Basic certificate structure validation
2. **Link Protocol Layer**: CERTS cell signature verification
3. **Identity Layer**: Consensus-based identity verification

### 3. Automated Protection

No manual configuration required - certificate pinning is automatically enabled for all circuit builds using consensus data.

## Testing

### Unit Tests

The implementation includes comprehensive tests in `pkg/circuit/pinning_test.go`:

1. **TestConnectToRelayWithPinning**: Verifies pinning configuration with full relay info
2. **TestConnectToRelayWithoutRelay**: Ensures backward compatibility when relay is nil
3. **TestConnectToRelayWithPartialIdentity**: Tests partial identity information handling
4. **TestCertificatePinningIntegrity**: Validates integrity of pinning configuration

### Running Tests

```bash
# Run certificate pinning tests
go test ./pkg/circuit -run TestConnectToRelay -v

# Run all circuit tests
go test ./pkg/circuit -v

# Run connection and protocol tests
go test ./pkg/connection ./pkg/protocol -v
```

## Configuration

### Default Behavior

Certificate pinning is **automatically enabled** when building circuits:
- Enabled for all guard, middle, and exit relays
- Uses identity information from directory consensus
- Strict validation mode (fails on mismatch)

### Disabling (Not Recommended)

To disable certificate pinning (for testing only):

```go
cfg := connection.DefaultConfig(address)
cfg.RequireCERTS = false
cfg.ExpectedIdentity = nil
cfg.ExpectedFingerprint = ""
```

**Warning**: Disabling certificate pinning reduces security and should only be done in controlled testing environments.

## Performance Impact

**Minimal**: The enhancement adds:
- ~100 bytes of configuration data per connection
- ~1ms for CERTS validation during handshake
- No impact on steady-state circuit operation

## Compatibility

### Tor Network

The implementation is fully compatible with the production Tor network:
- Uses standard CERTS cell format (tor-spec.txt §4.2)
- Validates against directory consensus (dir-spec.txt)
- Supports all link protocol versions (v3-v5)

### Existing Code

The enhancement is backward compatible:
- Does not break existing tests
- Maintains same API for connection establishment
- Gracefully handles missing identity information

## Implementation Notes

### Consensus Data Requirements

Certificate pinning requires relay information from consensus:
- `Fingerprint`: 40-character hex string (RSA fingerprint)
- `IdentityKey`: 32-byte Ed25519 public key

These fields are populated when parsing consensus documents in `pkg/directory/directory.go`.

### Error Handling

Certificate validation failures result in:
1. Handshake failure with descriptive error
2. Circuit build failure
3. Automatic retry with different path (by circuit builder)

### Logging

Certificate pinning operations are logged at appropriate levels:
- **DEBUG**: Pinning configuration details
- **INFO**: Successful identity verification
- **WARN**: Validation failures in non-strict mode
- **ERROR**: Validation failures in strict mode

## Future Enhancements

Potential improvements for consideration:

1. **Connection-level Padding**: Add padding to TLS connections (padding-spec.txt)
2. **Certificate Caching**: Cache validated certificates to reduce handshake overhead
3. **Metrics**: Track pinning validation success/failure rates
4. **Configurable Strictness**: Allow per-relay or per-circuit pinning policies

## References

- **tor-spec.txt §2**: TLS and Link Protocol
- **tor-spec.txt §4.2**: CERTS cell format
- **cert-spec.txt**: Ed25519 certificate specification
- **dir-spec.txt**: Directory consensus format
- **AUDIT.md**: Certificate pinning audit finding (P3)

## Related Files

- `pkg/circuit/builder.go`: Circuit builder with pinning
- `pkg/circuit/pinning_test.go`: Certificate pinning tests
- `pkg/connection/connection.go`: Connection configuration
- `pkg/protocol/certs.go`: CERTS cell parsing and validation
- `pkg/protocol/protocol.go`: Link protocol handshake
- `docs/CERTIFICATE_PINNING.md`: This document

# CERTS Cell Strict Enforcement Demo

This example demonstrates the RequireCERTS flag for strict enforcement of CERTS cell validation during Tor relay handshakes.

## Overview

The RequireCERTS feature provides two modes for CERTS cell validation:

1. **Non-enforcing mode** (default): Validation failures are logged as warnings, connection continues
2. **Strict enforcing mode**: Validation failures terminate the handshake with an error

## Usage

### Non-Enforcing Mode (Default)

```go
cfg := connection.DefaultConfig("127.0.0.1:9001")
// RequireCERTS is false by default
conn := connection.New(cfg, logger)
// Validation failures → warnings logged, connection continues
```

### Strict Enforcing Mode

```go
cfg := &connection.Config{
    Address:             "127.0.0.1:9001",
    ExpectedIdentity:    relayIdentityFromConsensus,
    ExpectedFingerprint: relayFingerprintFromConsensus,
    RequireCERTS:        true, // Enable strict enforcement
}
conn := connection.New(cfg, logger)
// Validation failures → handshake terminates with error
```

## Validation Types

When RequireCERTS is enabled, the following validations are enforced:

1. **Certificate Expiration**: Rejects expired certificates
2. **Signature Validation**: Rejects certificates with invalid Ed25519 signatures
3. **Identity Validation**: Rejects relays with mismatched identities

## Running the Demo

```bash
go run main.go
```

## Production Recommendation

For production deployments:
- Set `RequireCERTS = true` when connecting to specific relays
- Always provide `ExpectedIdentity` and `ExpectedFingerprint` from directory consensus
- Use strict mode to prevent man-in-the-middle attacks

## Backward Compatibility

The default behavior (RequireCERTS=false) maintains backward compatibility with existing code. Strict enforcement is opt-in.

## See Also

- [PLAN.md](../../PLAN.md) - Section 7 (Protocol Handshake) for implementation details
- [pkg/connection](../../pkg/connection) - Connection configuration
- [pkg/protocol](../../pkg/protocol) - Protocol handshake implementation

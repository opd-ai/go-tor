# Microdescriptor Fetching (SPEC-001)

## Overview

This document describes the implementation of relay key extraction from Tor directory microdescriptors, completed in January 2026 as part of SPEC-001.

## Background

Tor relays have cryptographic keys that are essential for building circuits:
- **Ed25519 Identity Key**: 32-byte identity key for relay authentication
- **Ntor Onion Key**: 32-byte Curve25519 public key for ntor handshake

These keys are not included in the consensus document itself. Instead, the consensus contains microdescriptor digests, and the actual keys must be fetched from microdescriptors.

## Implementation

### Consensus Parsing

The consensus document contains "a" lines with microdescriptor digests:

```
r TestRelay AAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBB 2024-01-01 00:00:00 192.168.1.1 9001 0
a sha256=dGVzdGRpZ2VzdA==
s Fast Guard Running Stable Valid
```

The parser now extracts the `sha256=` digest and stores it in `Relay.MicrodescDigest`.

### Microdescriptor Fetching

Per dir-spec.txt §3.3, microdescriptors are fetched via HTTP:

```
GET /tor/micro/d/digest1-digest2-digest3
```

The implementation:
1. Collects unique microdescriptor digests from all relays
2. Batches them into groups of 90 (per spec recommendation)
3. Fetches from directory authorities with fallback
4. Handles gzip/deflate compression

### Microdescriptor Parsing

Microdescriptor format per dir-spec.txt:

```
onion-key
-----BEGIN RSA PUBLIC KEY-----
...
-----END RSA PUBLIC KEY-----
ntor-onion-key base64(curve25519_public_key)
id ed25519
base64(32-byte_identity_key)
```

The parser:
1. Extracts `ntor-onion-key` (32 bytes after base64 decode)
2. Extracts `id ed25519` (next line, 32 bytes after base64 decode)
3. Calculates SHA256 digest of the microdescriptor
4. Matches digest to relays and populates keys

### Automatic Integration

The `FetchConsensus()` method now automatically calls `FetchMicrodescriptors()` after fetching the consensus. This ensures that all relays have their cryptographic keys populated before being used for circuit building.

## Usage

```go
import (
    "context"
    "github.com/opd-ai/go-tor/pkg/directory"
    "github.com/opd-ai/go-tor/pkg/logger"
)

// Create directory client
client := directory.NewClient(logger.NewDefault())

// Fetch consensus (automatically fetches microdescriptors)
relays, err := client.FetchConsensus(context.Background())
if err != nil {
    log.Fatal(err)
}

// Check if relays have valid keys
for _, relay := range relays {
    if relay.HasValidKeys() {
        // Ready for circuit building
        identityKey := relay.GetIdentityKey()  // 32 bytes
        ntorKey := relay.GetNtorOnionKey()     // 32 bytes
        
        // Use keys for circuit extension
    }
}
```

## Testing

Comprehensive unit tests validate:
- Microdescriptor digest parsing from consensus
- Batch fetching with proper URL construction
- Key extraction and base64 decoding
- Digest matching and relay population
- Key validation (length checks)

Run tests:
```bash
go test -v ./pkg/directory/... -run "Microdescriptor"
```

## Performance

- **Batch Size**: 90 microdescriptors per request (per spec)
- **Compression**: Automatic gzip/deflate support
- **Fallback**: Tries multiple directory authorities
- **Memory**: Efficient digest map for O(1) relay lookup

## Integration with Circuit Building

The circuit extension code (`pkg/circuit/extension.go`) uses the extracted keys:

```go
type RelayWithKeys interface {
    GetIdentityKey() []byte
    GetNtorOnionKey() []byte
}

// Extract keys for ntor handshake
identityKey, ntorKey, err := getRelayKeys(relay)
```

This enables proper ntor handshake execution during circuit extension.

## Specification Compliance

✅ **dir-spec.txt §3.3**: Microdescriptor fetching protocol  
✅ **dir-spec.txt §3.3.1**: URL format `/tor/micro/d/digest-list`  
✅ **dir-spec.txt §3.3.2**: Microdescriptor document format  
✅ **tor-spec.txt §5.1.4**: Ntor key format (32-byte Curve25519)  
✅ **tor-spec.txt §0.3**: Ed25519 identity keys

## Future Enhancements

- [ ] Cache microdescriptors to reduce network requests
- [ ] Validate microdescriptor signatures
- [ ] Support descriptor fetching for legacy TAP handshake
- [ ] Add metrics for microdescriptor fetch success rate

## References

- [Tor Directory Protocol Specification](https://spec.torproject.org/dir-spec)
- [Tor Protocol Specification](https://spec.torproject.org/tor-spec)
- AUDIT.md §4: Directory Protocol compliance
- SPEC-001: Relay key extraction task

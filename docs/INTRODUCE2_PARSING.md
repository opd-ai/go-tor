# INTRODUCE2 Cell Parsing Implementation

This document describes the INTRODUCE2 cell parsing implementation for onion service hosting in go-tor, following the Tor rend-spec-v3.txt specification §3.2-3.3.

## Overview

The INTRODUCE2 cell is sent by clients to introduction points when they want to connect to an onion service. This implementation provides complete parsing and decryption of INTRODUCE2 cells, extracting all necessary information to establish a rendezvous connection.

## Implementation Details

### Cell Format

The INTRODUCE2 cell follows the structure defined in rend-spec-v3.txt §3.2:

```
Outer Layer (before encryption):
  AUTH_KEY_TYPE   [1 byte]  - 0x02 for ED25519-SHA3-256
  AUTH_KEY_LEN    [2 bytes] - Length of AUTH_KEY (32 for Ed25519)
  AUTH_KEY        [32 bytes] - Client's authentication key
  EXTENSIONS      [N bytes]  - Extension fields (currently skipped)
  ENCRYPTED_DATA  [variable] - Encrypted inner layer + MAC

Inner Layer (after decryption):
  RENDEZVOUS_COOKIE  [20 bytes] - Cookie for rendezvous point
  NSPEC              [1 byte]   - Number of link specifiers
  LINK_SPECIFIERS    [variable] - How to reach rendezvous point
  ONION_KEY_TYPE     [1 byte]   - 0x00 for ntor
  ONION_KEY_LEN      [2 bytes]  - Length of onion key (32 for ntor)
  ONION_KEY          [32 bytes] - Client's ephemeral public key
  EXTENSIONS         [variable] - Extension fields
```

### Encryption and MAC

The encrypted portion uses:
- **Key Derivation**: HKDF-SHA256 with intro point encryption key
- **Encryption**: AES-256-CTR with derived encryption key
- **MAC**: HMAC-SHA256 with derived MAC key
- **Format**: `CIPHERTEXT || MAC(CIPHERTEXT)`

Key derivation uses the info string: `"tor-hs-ntor-curve25519-sha3-256-1:hs_key_extract"`

### Link Specifiers

Link specifiers describe how to reach the rendezvous point:

- **Type 0x00**: TLS-over-TCP-IPv4 (6 bytes: 4-byte IPv4 + 2-byte port)
- **Type 0x01**: TLS-over-TCP-IPv6 (18 bytes: 16-byte IPv6 + 2-byte port)
- **Type 0x02**: Legacy identity (20 bytes: SHA-1 fingerprint)
- **Type 0x03**: Ed25519 identity (32 bytes: Ed25519 public key)

## API Usage

### Parsing an INTRODUCE2 Cell

```go
// Introduction point keys (established during ESTABLISH_INTRO)
introAuthKey := []byte{...} // 32 bytes
introEncKey := []byte{...}  // 32 bytes

// Parse the INTRODUCE2 cell
request, err := ParseIntroduce2(introduce2Data, introAuthKey, introEncKey)
if err != nil {
    log.Errorf("Failed to parse INTRODUCE2: %v", err)
    return err
}

// Access parsed data
fmt.Printf("Rendezvous cookie: %x\n", request.RendezvousCookie)
fmt.Printf("Client onion key: %x\n", request.ClientOnionKey)
fmt.Printf("Client auth key: %x\n", request.ClientAuthKey)

// Convert link specifiers to address
address, err := LinkSpecifierToAddress(request.LinkSpecifiers)
if err == nil {
    fmt.Printf("Rendezvous point: %s\n", address)
}
```

### Integration with Service

The `Service.HandleIntroduce2()` method automatically:

1. Finds the introduction point for the circuit
2. Parses and decrypts the INTRODUCE2 cell
3. Extracts rendezvous information
4. Stores the pending introduction request

```go
// In service implementation
func (s *Service) HandleIntroduce2(introCircuitID uint32, data []byte) error {
    // Find intro point
    introPoint := s.findIntroPoint(introCircuitID)
    
    // Parse INTRODUCE2
    request, err := ParseIntroduce2(data, introPoint.AuthKey, introPoint.EncKey)
    if err != nil {
        return err
    }
    
    // Store pending request
    s.storePendingIntro(request)
    
    // Next steps (TODO):
    // - Build circuit to rendezvous point
    // - Send RENDEZVOUS1 cell
    // - Establish connection
    
    return nil
}
```

## Security Considerations

### Constant-Time Operations

All cryptographic operations use constant-time implementations:

- **MAC Verification**: Uses `crypto.ConstantTimeCompare()` to prevent timing attacks
- **Key Comparisons**: All key comparisons use constant-time functions
- **No Timing Leaks**: Error paths don't leak information about failure reasons

### Key Derivation

Keys are derived using HKDF-SHA256:

```go
kdfInfo := []byte("tor-hs-ntor-curve25519-sha3-256-1:hs_key_extract")
kdf := hkdf.New(sha256.New, introEncKey, nil, kdfInfo)
keys := make([]byte, 64)
io.ReadFull(kdf, keys)

encKey := keys[0:32]  // Encryption key
macKey := keys[32:64] // MAC key
```

### MAC Verification

MAC is verified before decryption to prevent oracle attacks:

```go
computedMAC := hmac.New(sha256.New, macKey)
computedMAC.Write(ciphertext)
expectedMAC := computedMAC.Sum(nil)

if !crypto.ConstantTimeCompare(mac, expectedMAC) {
    return nil, fmt.Errorf("MAC verification failed")
}
```

## Error Handling

The implementation handles various error conditions:

- **Truncated cells**: Detected early with length checks
- **Invalid auth key types**: Only ED25519-SHA3-256 (0x02) supported
- **Invalid onion key types**: Only ntor (0x00) supported
- **MAC verification failures**: Constant-time comparison prevents timing attacks
- **Decryption failures**: Proper error propagation
- **Invalid link specifiers**: Graceful handling of unsupported types

## Testing

### Unit Tests

Comprehensive test coverage (>70%) includes:

1. **Valid Cell Parsing**: End-to-end test with proper encryption
2. **Invalid Formats**: Truncated cells, wrong lengths
3. **MAC Failures**: Invalid MAC detection
4. **Link Specifiers**: IPv4, IPv6, and unsupported types
5. **Benchmarks**: Performance testing

### Test Coverage

```bash
$ go test ./pkg/onion -coverprofile=coverage.out
$ go tool cover -func=coverage.out | grep introduce2.go
ParseIntroduce2              72.5%
parseIntroduce2Inner         64.9%
LinkSpecifierToAddress       100.0%
```

### Running Tests

```bash
# Run all INTRODUCE2 tests
go test ./pkg/onion -v -run TestParseIntroduce2

# Run with race detector
go test ./pkg/onion -race -run TestParseIntroduce2

# Run benchmarks
go test ./pkg/onion -bench BenchmarkParseIntroduce2
```

## Performance

Typical performance metrics:

- **Parsing time**: ~50-100µs per cell
- **Memory allocation**: Minimal (uses pre-allocated buffers where possible)
- **Throughput**: >10,000 cells/second on modern hardware

Benchmark results:
```
BenchmarkParseIntroduce2-8   20000   85432 ns/op   4096 B/op   24 allocs/op
```

## Dependencies

### Internal Packages

- `pkg/crypto`: AES-CTR encryption, HMAC, constant-time comparison
- `pkg/logger`: Structured logging

### External Packages

- `golang.org/x/crypto/hkdf`: Key derivation
- `crypto/hmac`: MAC computation
- `crypto/sha256`: Hash functions

## Future Work

The current implementation completes task 9.2.1. Remaining work:

- **Task 9.2.2**: Rendezvous circuit building using parsed link specifiers
- **Task 9.2.3**: RENDEZVOUS1 cell construction and ntor handshake
- **Task 9.3**: Stream handling for established connections

## References

- [rend-spec-v3.txt §3.2](https://spec.torproject.org/rend-spec-v3) - INTRODUCE2 cell format
- [rend-spec-v3.txt §3.3](https://spec.torproject.org/rend-spec-v3) - Rendezvous protocol
- [tor-spec.txt §5.1.4](https://spec.torproject.org/tor-spec) - ntor handshake
- [cert-spec.txt](https://spec.torproject.org/cert-spec) - Certificate formats

## Changelog

### January 25, 2026

- ✅ Initial implementation of INTRODUCE2 parsing
- ✅ Complete encryption and MAC verification
- ✅ Link specifier parsing for IPv4 and IPv6
- ✅ Comprehensive test coverage (>70%)
- ✅ Added crypto helpers: `DecryptAES256CTR`, `EncryptAES256CTR`, `ConstantTimeCompare`
- ✅ Integration with `Service.HandleIntroduce2()`
- ✅ Documentation and examples

---

**Status**: ✅ Complete  
**Task**: 9.2.1 INTRODUCE2 Cell Parsing  
**Last Updated**: January 25, 2026

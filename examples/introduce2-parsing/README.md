# INTRODUCE2 Cell Parsing Example

This example demonstrates how to parse and decrypt INTRODUCE2 cells in an onion service implementation.

## What It Does

1. **Generates Introduction Point Keys**: Creates the keys that would be established during `ESTABLISH_INTRO`
2. **Creates Mock INTRODUCE2 Cell**: Simulates what a client would send when connecting
3. **Parses the Cell**: Demonstrates the complete parsing and decryption process
4. **Extracts Information**: Shows how to access rendezvous cookie, client keys, and link specifiers

## Running the Example

```bash
go run main.go
```

## Expected Output

```
=== INTRODUCE2 Cell Parsing Example ===

1. Generating introduction point keys...
   Auth Key: a436c9175c87e8ae...
   Enc Key:  fcb66a6acb27648c...

2. Creating mock INTRODUCE2 cell...
   Cell size: 133 bytes

3. Parsing INTRODUCE2 cell...
   ✓ Successfully parsed and decrypted

4. Parsed Information:
   Rendezvous Cookie: f22b7a584cb11a6cdc407d1d0f9b6952aa2afa48
   Client Onion Key:  c8fc6cd1152b65ce04d48044c6a553cc...
   Client Auth Key:   efc9fa76c0b949dbd1a24a86c5918996...
   Link Specifiers:   1

5. Extracting Rendezvous Point:
   Rendezvous Address: 192.0.2.1:9001

=== Next Steps ===
In a complete implementation, the service would now:
  1. Build a circuit to the rendezvous point
  2. Perform ntor handshake with the client
  3. Send RENDEZVOUS1 cell with handshake response
  4. Establish end-to-end encrypted connection
```

## What This Demonstrates

### Cell Format

The INTRODUCE2 cell follows the Tor specification (rend-spec-v3.txt §3.2):

- **Outer Layer**: Auth key type, auth key, extensions, encrypted data
- **Encryption**: AES-256-CTR with HMAC-SHA256 MAC
- **Inner Layer**: Rendezvous cookie, link specifiers, client onion key

### Security Features

- **MAC Verification**: Uses constant-time comparison to prevent timing attacks
- **HKDF Key Derivation**: Derives encryption and MAC keys from intro point key
- **Proper Encryption**: AES-256-CTR mode for authenticated encryption

### Link Specifiers

The example includes parsing link specifiers that tell the service how to reach the rendezvous point:

- **IPv4**: 4-byte address + 2-byte port
- **IPv6**: 16-byte address + 2-byte port  
- **Legacy ID**: 20-byte SHA-1 fingerprint
- **Ed25519 ID**: 32-byte public key

## Integration with Real Service

In a complete onion service implementation, this parsing would be part of the `HandleIntroduce2` method:

```go
func (s *Service) HandleIntroduce2(circuitID uint32, data []byte) error {
    // Find intro point
    introPoint := s.findIntroPoint(circuitID)
    
    // Parse INTRODUCE2
    request, err := ParseIntroduce2(data, introPoint.AuthKey, introPoint.EncKey)
    if err != nil {
        return err
    }
    
    // Next: Build circuit to rendezvous point
    // ...
}
```

## Further Reading

- [INTRODUCE2_PARSING.md](../../docs/INTRODUCE2_PARSING.md) - Complete documentation
- [rend-spec-v3.txt](https://spec.torproject.org/rend-spec-v3) - Tor specification
- [INTRO_POINT_PROTOCOL.md](../../docs/INTRO_POINT_PROTOCOL.md) - Introduction point setup

## Related Examples

- [intro-point-management/](../intro-point-management/) - Introduction point lifecycle
- [onion-service-demo/](../onion-service-demo/) - Complete service example

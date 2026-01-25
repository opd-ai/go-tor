# Link Protocol Server Implementation

This document describes the server-side link protocol implementation for OR (Onion Router) connections.

## Overview

The link protocol server (`pkg/relay/or_handler.go`) implements the server-side Tor protocol handshake per tor-spec.txt §1-2. This allows the relay to accept incoming connections from Tor clients and other relays.

## Specification Reference

- **tor-spec.txt §2**: Link Protocol (server-side)
- **tor-spec.txt §4.2**: CERTS cell format
- **tor-spec.txt §4.5**: NETINFO cell format
- **cert-spec.txt §2.1**: Ed25519 certificate format

## Architecture

### LinkProtocolHandler

The `LinkProtocolHandler` manages the server-side link protocol handshake:

```go
type LinkProtocolHandler struct {
    keys   *RelayKeys  // Relay identity keys
    logger *logger.Logger
}
```

### Handshake Flow

The server performs the following steps when a client connects:

1. **Receive VERSIONS** - Client sends supported protocol versions
2. **Send VERSIONS** - Server responds with mutual versions (3, 4, 5)
3. **Send CERTS** - Server sends identity certificates
4. **Send NETINFO** - Server sends network info (timestamp, addresses)
5. **Receive NETINFO** - Server receives client's network info

```
Client                                Server
  |                                     |
  |-------- VERSIONS (3,4,5) --------->|
  |                                     |
  |<------- VERSIONS (3,4,5) ----------|
  |<--------- CERTS --------------------|
  |<--------- NETINFO ------------------|
  |                                     |
  |--------- NETINFO ------------------>|
  |                                     |
  [Link established, version negotiated]
```

## Version Negotiation

The server supports link protocol versions **3, 4, and 5**:

- **Version 3**: Basic OR protocol
- **Version 4**: 4-byte circuit IDs (preferred)
- **Version 5**: Extended features

Version selection uses highest mutual version:

```go
func (h *LinkProtocolHandler) selectVersion(clientVersions []int) int {
    supportedVersions := []int{5, 4, 3}
    for _, supported := range supportedVersions {
        for _, client := range clientVersions {
            if client == supported {
                return supported
            }
        }
    }
    return 0 // No compatible version
}
```

## CERTS Cell

The server sends a CERTS cell containing multiple certificates:

### Certificate Types

| Type | Description | Required |
|------|-------------|----------|
| 0x01 | TLS link certificate | Yes |
| 0x02 | RSA identity certificate | Yes |
| 0x04 | Ed25519 signing key certificate | Yes |

### CERTS Cell Format

```
N             [1 octet]   Number of certificates
N times:
  CertType    [1 octet]   Certificate type
  CLEN        [2 octets]  Certificate length (big-endian)
  Certificate [CLEN bytes] Certificate body
```

### Ed25519 Signing Certificate

The server generates an Ed25519 signing certificate per cert-spec.txt §2.1:

```
Version (1) || CertType (1) || ExpirationDate (4) || CertKeyType (1) ||
CertifiedKey (32) || NumExtensions (1) || Signature (64)
```

- **Version**: 0x01
- **CertType**: 0x04 (Ed25519 signing key)
- **ExpirationDate**: Hours since epoch (4 bytes, big-endian)
- **CertKeyType**: 0x01 (Ed25519 key)
- **CertifiedKey**: 32-byte Ed25519 public key
- **NumExtensions**: 0x00 (no extensions)
- **Signature**: 64-byte Ed25519 signature over certificate body

The certificate is signed with the relay's Ed25519 identity key.

## NETINFO Cell

The NETINFO cell contains timestamp and address information:

### Format

```
Timestamp       [4 octets]  Current time in seconds since epoch
OtherAddress    [varies]    Client's address as seen by server
  Type          [1 octet]   Address type (0x04 = IPv4)
  Length        [1 octet]   Address length
  Address       [Length]    IP address bytes
NumAddresses    [1 octet]   Number of server addresses (usually 0)
```

The server:
1. Sends current timestamp (using `security.SafeUnixToUint32`)
2. Includes the client's remote address (IPv4 or 0.0.0.0)
3. Sends 0 for number of server addresses (simplified)

## Cell Reading/Writing

### Context-Aware Reading

The handler uses context-aware cell reading with timeouts:

```go
func (h *LinkProtocolHandler) readCellWithContext(ctx context.Context, conn net.Conn) (*cell.Cell, error) {
    // Read header (CircID + Command)
    header := make([]byte, 5)
    
    // Use goroutine with result channel for cancellation
    resultCh := make(chan readResult, 1)
    go func() {
        n, err := conn.Read(header)
        resultCh <- readResult{n, err}
    }()
    
    select {
    case <-ctx.Done():
        return nil, fmt.Errorf("read cancelled: %w", ctx.Err())
    case result := <-resultCh:
        // Process result...
    }
}
```

### Cell Encoding

Cells are encoded using the standard cell encoder:

```go
func (h *LinkProtocolHandler) writeCell(conn net.Conn, c *cell.Cell) error {
    var buf bytes.Buffer
    if err := c.Encode(&buf); err != nil {
        return fmt.Errorf("failed to encode cell: %w", err)
    }
    _, err := conn.Write(buf.Bytes())
    return err
}
```

## Integration with OR Listener

The `ORListener` uses the `LinkProtocolHandler` to perform handshakes:

```go
// In or_listener.go handleConnection():
linkHandler := NewLinkProtocolHandler(l.keys, l.logger)
serverConn, err := linkHandler.HandleConnection(ctx, rawConn)
if err != nil {
    l.logger.Warn("Link protocol handshake failed", "error", err)
    return
}

l.logger.Info("Link protocol complete", "version", serverConn.negotiatedVersion)
```

## Error Handling

The handler returns descriptive errors for common failure cases:

- **No compatible version**: Client and server have no mutual protocol version
- **Invalid cell type**: Received unexpected cell type (e.g., NETINFO instead of VERSIONS)
- **Timeout**: Cell not received within configured timeout (10 seconds default)
- **EOF**: Client disconnected during handshake

## Testing

Comprehensive tests cover:

1. **Version negotiation**: All version combinations (3, 4, 5, incompatible)
2. **Cell sending/receiving**: VERSIONS, CERTS, NETINFO
3. **Ed25519 certificate generation**: Structure, signature validation
4. **Error cases**: Invalid cells, timeouts, wrong cell types

Coverage: **>80%** for or_handler.go

### Running Tests

```bash
# Run link protocol tests
go test -v ./pkg/relay/... -run "TestLinkProtocol|TestSend|TestReceive"

# Run with coverage
go test ./pkg/relay/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Future Enhancements

Tasks not yet implemented (planned for Task 10.1.3):

1. **AUTH_CHALLENGE cell**: Optional authentication challenge (for client authentication)
2. **AUTHENTICATE cell**: Client authentication response
3. **CREATE2/CREATED2 handling**: Circuit creation on the server side (Task 10.1.3)
4. **Circuit management**: Server-side circuit state tracking (Task 10.1.3)

## Security Considerations

1. **Certificate validation**: The server sends certificates but does NOT validate client certificates (client auth not implemented)
2. **Timeout handling**: All cell reads use 10-second timeout to prevent DoS
3. **Context cancellation**: All blocking operations respect context cancellation
4. **Ed25519 signatures**: All certificates are properly signed and verifiable

## References

- [tor-spec.txt](https://spec.torproject.org/tor-spec) - Tor Protocol Specification
- [cert-spec.txt](https://spec.torproject.org/cert-spec) - Certificate Format Specification
- [Tor Protocol](https://github.com/torproject/torspec) - Official Tor Specifications Repository

## Related Files

- `pkg/relay/or_handler.go` - Link protocol server implementation
- `pkg/relay/or_handler_test.go` - Comprehensive test suite
- `pkg/relay/or_listener.go` - OR connection listener
- `pkg/relay/keys.go` - Relay key generation and management
- `pkg/protocol/protocol.go` - Client-side link protocol (for reference)
- `pkg/protocol/certs.go` - CERTS cell parsing utilities

# Client Authorization Integration Example

This example demonstrates the complete workflow for accessing private onion services using client authorization.

## Overview

Client authorization allows onion service operators to restrict access to their services. Only clients with valid x25519 credentials can decrypt the service descriptor and connect.

## Running the Example

```bash
cd examples/client-auth-integration
go run main.go
```

## What This Example Shows

1. **Service Creation**: Creates a private v3 onion service
2. **Credential Generation**: Generates x25519 keypair for authorized client
3. **Credential Sharing**: Demonstrates how service operator shares credentials (simulated)
4. **Credential Storage**: Client stores authorization credentials
5. **Credential Management**: Shows credential lookup and verification
6. **Connection Readiness**: Prepares client for private service access

## Expected Output

```
=======================================================================
Client Authorization Integration Example
=======================================================================

[1/6] Creating private onion service...
✓ Private service created: [56-character-address].onion

[2/6] Service operator generates client credentials...
✓ Client keypair generated
  Public key (first 8 bytes): [hex]

[3/6] Service operator shares credentials with client...
  Service address: [56-character-address].onion
  Client private key: (32 bytes - keep secret!)

[4/6] Client adds authorization credentials...
✓ Credential stored successfully

[5/6] Demonstrating credential management...
  ✓ Credential lookup works
  ✓ Credentials are persisted in memory

[6/6] Client ready to connect to private service...
  ✓ Client authorized successfully!

Summary:
  ✓ Private onion service created
  ✓ Client credentials generated (x25519 keypair)
  ✓ Credentials securely stored in auth store
  ✓ Client ready to access private service
```

## Real-World Usage

### Service Operator Workflow

```go
// 1. Create private onion service
service, err := onion.NewService(config, logger)

// 2. Generate client keypair
var clientPrivate [32]byte
rand.Read(clientPrivate[:])

var clientPublic [32]byte
curve25519.ScalarBaseMult(&clientPublic, &clientPrivate)

// 3. Share clientPrivate with authorized user via secure channel
// (Signal, PGP-encrypted email, in-person, etc.)
```

### Client Workflow

```go
// 1. Receive private key from service operator
var clientPrivate [32]byte
// ... load from secure storage ...

// 2. Create onion client
client := onion.NewClient(logger)

// 3. Add authorization credential
client.AddClientAuth(serviceAddress, clientPrivate)

// 4. Connect to service (automatic descriptor decryption)
// Client will automatically use stored credentials when needed
```

## Security Considerations

### Key Management

- **Private keys**: Keep secret! Never share or transmit insecurely
- **Public keys**: Can be safely shared with service operator
- **Storage**: Store private keys in secure key store (not plaintext files)

### Key Distribution

Service operators should use secure channels to share credentials:
- ✅ In-person key exchange
- ✅ PGP-encrypted email
- ✅ Signal/encrypted messaging
- ✅ Hardware security modules (HSM)
- ❌ Plaintext email
- ❌ SMS/text messages
- ❌ Unencrypted chat

### Revocation

To revoke a client's access:
1. Service operator removes client's public key from authorized list
2. Service republishes descriptor without that client
3. Client can no longer decrypt new descriptors

## API Reference

### Client Methods

```go
// Add client authorization credential
func (c *Client) AddClientAuth(onionAddress string, privateKey [32]byte) error

// Check if client has authorization for an address
func (c *Client) HasClientAuth(onionAddress string) bool

// Remove client authorization credential
func (c *Client) RemoveClientAuth(onionAddress string)

// Automatically called when fetching descriptors
func (c *Client) TryClientAuth(descriptor *Descriptor, address *Address) (*Descriptor, error)
```

### Service Methods

```go
// Create private onion service
func NewService(config *ServiceConfig, logger *logger.Logger) (*Service, error)

// Get service address
func (s *Service) GetAddress() string

// Start service (publishes descriptor with auth layer)
func (s *Service) Start(ctx context.Context) error
```

## Related Documentation

- [CLIENT_AUTHORIZATION.md](../../docs/CLIENT_AUTHORIZATION.md) - User guide
- [TESTING_CLIENT_AUTHORIZATION.md](../../docs/TESTING_CLIENT_AUTHORIZATION.md) - Integration tests
- [rend-spec-v3.txt §2.5](https://spec.torproject.org/rend-spec/v3.html) - Protocol specification

## Troubleshooting

### "descriptor requires client authorization but no credential available"

**Cause**: Trying to access private service without credentials

**Solution**: Add credentials using `client.AddClientAuth(address, privateKey)`

### "client authorization failed: could not decrypt descriptor"

**Cause**: Wrong private key or corrupted descriptor

**Solutions**:
- Verify you have the correct private key from service operator
- Check that service address matches
- Ensure descriptor hasn't been tampered with

### Build Errors

```bash
# Update dependencies
go mod tidy

# Verify Go version (requires 1.21+)
go version
```

## License

This example is part of the go-tor project and uses the same license.

## Disclaimer

This is an experimental implementation for educational purposes. For real privacy needs, use the official Tor Browser.

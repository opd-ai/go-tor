# Bridge Relay Server Descriptor Example

This example demonstrates how to generate and validate Tor relay server descriptors for bridge relays.

## Overview

Server descriptors are documents that relays publish to advertise their capabilities and connection information. This implementation follows the Tor directory specification (dir-spec.txt §2.1).

## Features Demonstrated

1. **Relay Key Generation**
   - Ed25519 identity key (32 bytes)
   - RSA-1024 identity key  
   - ntor onion key (Curve25519)
   - Self-signed TLS certificate

2. **Descriptor Configuration**
   - Nickname (1-19 alphanumeric characters)
   - IPv4 address and OR port
   - Optional IPv6 address
   - Bandwidth limits (average/burst/observed)
   - Contact information
   - Relay family membership
   - Bridge-specific settings (DirPort=0)

3. **Descriptor Generation**
   - Proper formatting per dir-spec.txt
   - SHA-1 digest computation
   - RSA-PKCS1v15 signature
   - Protocol version declaration
   - Exit policy (reject all for non-exit relays)

4. **Extra-Info Descriptor**
   - Bandwidth statistics
   - Read/write history
   - Uptime tracking
   - Custom statistics

## Running the Example

```bash
go run main.go
```

## Output

The example will:
1. Generate relay cryptographic keys
2. Create a bridge descriptor configuration
3. Generate and sign the server descriptor
4. Validate descriptor integrity
5. Display the complete descriptor text
6. Generate an extra-info descriptor with statistics
7. Optionally save keys to disk

## Example Descriptor Format

```
router MyBridge 192.0.2.100 443 0 0
or-address [2001:db8::1]:443
platform go-tor 0.1.0 on Go
proto Link=3-5 Circuit=1-2
published 2024-01-25 12:00:00
identity-ed25519
-----BEGIN ED25519 CERT-----
<base64-encoded Ed25519 public key>
-----END ED25519 CERT-----
master-key-ed25519 <base64>
bandwidth 5242880 10485760 5242880
uptime 0
ntor-onion-key <base64>
contact bridge@example.com
reject *:*
router-signature
-----BEGIN SIGNATURE-----
<base64-encoded RSA signature>
-----END SIGNATURE-----
```

## Security Considerations

### Educational Use Only

This implementation is for educational and research purposes:
- **DO NOT** use for production bridge operation
- **DO NOT** use for real anonymity needs
- Use official Tor software for actual deployment

### Key Management

- Private keys are sensitive and must be protected
- Keys are saved with 0600 permissions (owner read/write only)
- Clean up test keys after use
- Never expose private keys in logs or error messages

### Descriptor Publishing

This example only generates descriptors. Actual bridge operation requires:
- Publishing descriptors to bridge authority
- Implementing bridge distribution mechanisms
- Handling descriptor refresh schedules
- Managing bridge reachability tests

## Next Steps

To implement a complete bridge relay, you would need:

1. **Descriptor Publisher** (`pkg/relay/publisher.go`)
   - Upload descriptors to bridge authority via HTTP POST
   - Handle upload responses and errors
   - Implement automatic refresh schedule

2. **Bridge Authority Communication**
   - Connect to bridge authority directory
   - Submit descriptors every 18 hours
   - Process reachability test results

3. **Bridge Distribution**
   - Integration with BridgeDB (optional)
   - Support for different distribution mechanisms
   - Bridge line generation for users

## References

- [dir-spec.txt](https://spec.torproject.org/dir-spec) - Directory protocol specification
- [bridge-spec.txt](https://spec.torproject.org/bridge-spec) - Bridge specification
- [tor-spec.txt](https://spec.torproject.org/tor-spec) - Core Tor protocol

## Related Examples

- `examples/relay-server/` - Complete relay server implementation (when available)
- `examples/bridge-client/` - Bridge client connection (when available)

## License

This code is provided for educational purposes as part of the go-tor project.

# Onion Service Hosting Demo

This example demonstrates how to host a Tor onion service (hidden service) using go-tor.

## Overview

An onion service allows you to host services (websites, APIs, etc.) on the Tor network with the following benefits:

- **Location Privacy**: Your server's IP address is hidden
- **End-to-End Encryption**: All connections are encrypted
- **Censorship Resistance**: Services are harder to block
- **No DNS Required**: Access via .onion addresses

## Features Demonstrated

This demo shows:

1. **Service Identity Management**: Creating and managing Ed25519 keypairs
2. **Descriptor Creation**: Building and signing v3 onion service descriptors
3. **Introduction Points**: Establishing circuits to introduction points
4. **Descriptor Publishing**: Publishing descriptors to HSDirs
5. **Service Monitoring**: Tracking service status and statistics

## Running the Demo

```bash
# Build and run
cd examples/onion-service-demo
go build
./onion-service-demo
```

## Expected Output

```
=== Onion Service Hosting Demo ===

Creating onion service...
✓ Service created
✓ Onion address: abcdefghijklmnopqrstuvwxyz234567abcdefghijklmnopqrst.onion

Using 6 HSDirs from consensus

Starting onion service...
  1. Establishing introduction points...
  2. Creating and signing descriptor...
  3. Publishing descriptor to HSDirs...

✓ Onion service is now ONLINE

─────────────────────────────────────────
Address:         abcdef...xyz.onion
Status:          ONLINE ✓
Intro Points:    3
Descriptor Age:  0s
Pending Intros:  0
HSDirs:          6
─────────────────────────────────────────

CONNECTION INFORMATION:
─────────────────────────────────────────
Your service is accessible at:
  http://abcdef...xyz.onion
  https://abcdef...xyz.onion

Users can connect via Tor Browser or any Tor client

BEHIND THE SCENES:
─────────────────────────────────────────
• Your service descriptor has been published to the Tor network
• Introduction points are ready to relay connection requests
• Descriptor will be automatically refreshed before expiration
• All connections use end-to-end encryption

Press Ctrl+C to stop the service...
```

## Configuration Options

The `ServiceConfig` struct provides several configuration options:

```go
config := &onion.ServiceConfig{
    // Service identity (if nil, generates new)
    PrivateKey: nil,

    // Port mappings: virtual_port -> local_address
    Ports: map[int]string{
        80:  "localhost:8080",  // HTTP
        443: "localhost:8443",  // HTTPS
    },

    // Number of introduction points (1-10, default: 3)
    NumIntroPoints: 3,

    // Descriptor validity period (default: 3h)
    DescriptorLifetime: 3 * time.Hour,

    // Directory for persistent state
    DataDirectory: "/var/lib/onion-service",
}
```

## Service Lifecycle

### 1. Creation

```go
service, err := onion.NewService(config, logger)
```

Creates a new onion service with the specified configuration. If no private key is provided, a new Ed25519 identity is generated.

### 2. Starting

```go
err := service.Start(ctx, hsdirs)
```

Starts the service by:
- Establishing introduction point circuits
- Creating and signing the descriptor
- Publishing to responsible HSDirs
- Starting background maintenance tasks

### 3. Monitoring

```go
stats := service.GetStats()
```

Retrieve service statistics including:
- Current status (running/stopped)
- Number of introduction points
- Descriptor age
- Pending introduction requests

### 4. Stopping

```go
err := service.Stop()
```

Gracefully shuts down the service by:
- Stopping maintenance tasks
- Closing introduction circuits
- Cleaning up resources

## Key Concepts

### Identity Key

Each onion service has a unique Ed25519 identity key pair. The public key is used to derive the .onion address. For persistent services, save and reuse the private key.

### Introduction Points

Relays that accept introduction requests on behalf of the service. The service establishes circuits to these points and advertises them in the descriptor.

### Descriptor

A signed document containing:
- Service public key information
- Introduction point details
- Validity period
- Protocol version

### HSDirs (Hidden Service Directories)

Special relays that store and distribute onion service descriptors. The descriptor is published to multiple HSDirs for redundancy.

## Production Considerations

For production deployments:

1. **Persistent Identity**: Save and load the private key to maintain the same .onion address
2. **Key Security**: Protect the private key with appropriate file permissions
3. **Local Services**: Ensure your local services (e.g., web server) are running before starting the onion service
4. **Monitoring**: Implement health checks and alerting
5. **Logging**: Use structured logging for troubleshooting
6. **Updates**: Regularly update go-tor for security patches

## Security Notes

⚠️ **Important Security Considerations:**

- This is a Phase 7.4 implementation demonstrating core functionality
- In production, additional security measures are required:
  - Rate limiting on introduction points
  - Client authorization for private services
  - DoS protection mechanisms
  - Regular security audits

## Related Examples

- `onion-address-demo/`: Parsing and validating .onion addresses
- `descriptor-demo/`: Working with onion service descriptors
- `intro-demo/`: Introduction protocol details
- `rendezvous-demo/`: Rendezvous protocol details

## References

- [Tor rend-spec-v3.txt](https://spec.torproject.org/rend-spec-v3): v3 Onion Service Specification
- [Tor Protocol Specifications](https://spec.torproject.org/)
- [Onion Services Best Practices](https://community.torproject.org/onion-services/setup/)

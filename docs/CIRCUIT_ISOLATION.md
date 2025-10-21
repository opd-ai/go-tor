# Circuit Isolation

Circuit isolation is a security feature that prevents different applications, users, or activities from sharing Tor circuits. This helps protect against correlation attacks where an adversary might try to link different activities based on circuit sharing.

## Table of Contents

- [Overview](#overview)
- [Isolation Levels](#isolation-levels)
- [Configuration](#configuration)
- [API Usage](#api-usage)
- [SOCKS5 Integration](#socks5-integration)
- [Performance Considerations](#performance-considerations)
- [Security Model](#security-model)
- [Examples](#examples)

## Overview

### What is Circuit Isolation?

In Tor, a circuit is a path through the network consisting of three relays: a guard, a middle relay, and an exit. By default, multiple connections can share the same circuit for efficiency. However, this sharing can potentially enable correlation attacks.

Circuit isolation ensures that different activities use separate circuits, preventing an adversary from correlating them based on circuit usage patterns.

### When to Use Circuit Isolation

Circuit isolation is recommended for:

- **Multi-user systems**: Prevent different users from sharing circuits
- **Multi-application scenarios**: Isolate different applications (browser, email, etc.)
- **Privacy-sensitive applications**: Separate different types of activities (banking, shopping, browsing)
- **High-security environments**: Minimize correlation risks

### Backward Compatibility

Circuit isolation is **disabled by default** to maintain backward compatibility. Existing applications will continue to work without any changes. Isolation must be explicitly enabled through configuration.

## Isolation Levels

### 1. No Isolation (Default)

**Level:** `IsolationNone`  
**Config:** `IsolationLevel = "none"`

All connections share circuits from a common pool. This is the default behavior.

```go
// Default behavior - no isolation
circ, err := pool.Get(ctx)
```

### 2. Destination Isolation

**Level:** `IsolationDestination`  
**Config:** `IsolationLevel = "destination"` or `IsolateDestinations = true`

Each unique destination (host:port) gets its own circuit. Connections to `example.com:443` will not share a circuit with connections to `wikipedia.org:443`.

**Use cases:**
- Prevent correlation between different websites
- Isolate sensitive destinations from general browsing
- Per-site circuit policies

```go
key := circuit.NewIsolationKey(circuit.IsolationDestination).
    WithDestination("example.com:443")
circ, err := pool.GetWithIsolation(ctx, key)
```

### 3. Credential Isolation

**Level:** `IsolationCredential`  
**Config:** `IsolationLevel = "credential"` or `IsolateSOCKSAuth = true`

Each SOCKS5 username gets its own circuit. This is automatically extracted from SOCKS5 username/password authentication (RFC 1929).

**Use cases:**
- Multi-user proxy servers
- Per-application isolation (each app uses different credentials)
- Session-based isolation at the SOCKS5 level

```go
key := circuit.NewIsolationKey(circuit.IsolationCredential).
    WithCredentials("alice")
circ, err := pool.GetWithIsolation(ctx, key)
```

**SOCKS5 Usage:**
```bash
# Different users get different circuits
curl --socks5 alice:password@localhost:9050 https://example.com
curl --socks5 bob:password@localhost:9050 https://example.com
```

### 4. Port Isolation

**Level:** `IsolationPort`  
**Config:** `IsolationLevel = "port"` or `IsolateClientPort = true`

Each client source port gets its own circuit. This automatically isolates different applications connecting from different ports.

**Use cases:**
- Automatic application isolation
- No configuration needed by applications
- Operating system assigns different ports to different processes

```go
key := circuit.NewIsolationKey(circuit.IsolationPort).
    WithSourcePort(12345)
circ, err := pool.GetWithIsolation(ctx, key)
```

### 5. Session Isolation

**Level:** `IsolationSession`  
**Config:** `IsolationLevel = "session"`

Custom session tokens allow application-level control over isolation. Applications can create arbitrary isolation boundaries.

**Use cases:**
- Fine-grained control over circuit sharing
- Application-specific isolation logic
- Temporary isolation for specific tasks

```go
key := circuit.NewIsolationKey(circuit.IsolationSession).
    WithSessionToken("shopping-session-abc")
circ, err := pool.GetWithIsolation(ctx, key)
```

## Configuration

### Configuration File (torrc)

```
# Disable isolation (default)
IsolationLevel none

# Enable destination-based isolation
IsolationLevel destination

# Enable credential-based isolation
IsolationLevel credential

# Enable port-based isolation
IsolationLevel port

# Enable session-based isolation
IsolationLevel session

# Enable specific isolation types (can be combined)
IsolateDestinations 1
IsolateSOCKSAuth 1
IsolateClientPort 1
```

### Go Configuration

```go
package main

import (
    "github.com/opd-ai/go-tor/pkg/config"
)

func main() {
    cfg := config.DefaultConfig()
    
    // Set isolation level
    cfg.IsolationLevel = "destination"
    
    // Or enable specific isolation types
    cfg.IsolateDestinations = true
    cfg.IsolateSOCKSAuth = true
    cfg.IsolateClientPort = true
    
    // Adjust circuit pool size for isolation
    cfg.CircuitPoolMaxSize = 20  // More circuits for isolation
    
    // Validate configuration
    if err := cfg.Validate(); err != nil {
        log.Fatal(err)
    }
}
```

## API Usage

### Basic Usage

```go
import (
    "context"
    "github.com/opd-ai/go-tor/pkg/circuit"
    "github.com/opd-ai/go-tor/pkg/pool"
)

// Get circuit without isolation
circ, err := circuitPool.Get(ctx)

// Get circuit with isolation
key := circuit.NewIsolationKey(circuit.IsolationDestination).
    WithDestination("example.com:443")
circ, err := circuitPool.GetWithIsolation(ctx, key)
```

### Creating Isolation Keys

```go
// Destination isolation
key := circuit.NewIsolationKey(circuit.IsolationDestination).
    WithDestination("example.com:443")

// Credential isolation
key := circuit.NewIsolationKey(circuit.IsolationCredential).
    WithCredentials("username")

// Port isolation
key := circuit.NewIsolationKey(circuit.IsolationPort).
    WithSourcePort(12345)

// Session isolation
key := circuit.NewIsolationKey(circuit.IsolationSession).
    WithSessionToken("custom-session-token")
```

### Validation

```go
key := circuit.NewIsolationKey(circuit.IsolationDestination).
    WithDestination("example.com:443")

if err := key.Validate(); err != nil {
    log.Fatalf("Invalid isolation key: %v", err)
}
```

### Working with Streams

```go
import "github.com/opd-ai/go-tor/pkg/stream"

// Create stream with isolation key
stream := stream.NewStream(streamID, circuitID, target, port, logger)
stream.SetIsolationKey(key)

// Retrieve isolation key
isolationKey := stream.GetIsolationKey()
```

### Circuit Pool Statistics

```go
stats := circuitPool.Stats()
fmt.Printf("Total circuits: %d\n", stats.Total)
fmt.Printf("Isolated pools: %d\n", stats.IsolatedPools)
fmt.Printf("Isolated circuits: %d\n", stats.IsolatedCircuits)
```

## SOCKS5 Integration

### Automatic Isolation

The SOCKS5 server automatically applies circuit isolation based on the configured isolation policy. No changes are required to SOCKS5 clients - the server extracts isolation metadata and selects appropriate circuits transparently.

### Configuration

Circuit isolation for SOCKS5 is configured via the Tor client config:

```go
import (
    "github.com/opd-ai/go-tor/pkg/config"
    "github.com/opd-ai/go-tor/pkg/client"
)

cfg := config.DefaultConfig()

// Enable destination-based isolation
cfg.IsolationLevel = "destination"
cfg.IsolateDestinations = true

// Or enable credential-based isolation
cfg.IsolationLevel = "credential"
cfg.IsolateSOCKSAuth = true

// Or enable port-based isolation
cfg.IsolationLevel = "port"
cfg.IsolateClientPort = true

// Create client - SOCKS server will use isolation automatically
client, err := client.New(cfg, logger)
```

### Username/Password Authentication

The SOCKS5 server supports RFC 1929 username/password authentication for credential-based isolation:

```go
import "github.com/opd-ai/go-tor/pkg/socks"

// SOCKS5 server automatically extracts username and applies isolation
server := socks.NewServer(":9050", circuitManager, logger)

// Or configure isolation explicitly
socksConfig := &socks.Config{
    MaxConnections:      1000,
    IsolationLevel:      circuit.IsolationCredential,
    IsolateSOCKSAuth:    true,
}
server := socks.NewServerWithConfig(":9050", circuitManager, logger, socksConfig)
```

### Client Usage

```bash
# curl with SOCKS5 authentication - each user gets isolated circuit
curl --socks5 alice:password@localhost:9050 https://example.com
curl --socks5 bob:password@localhost:9050 https://example.com

# Environment variable
export ALL_PROXY=socks5://bob:password@localhost:9050
curl https://example.com

# Python with requests
proxies = {
    'http': 'socks5://alice:password@localhost:9050',
    'https': 'socks5://alice:password@localhost:9050',
}
response = requests.get('https://example.com', proxies=proxies)
```

### How It Works

1. **SOCKS5 Handshake**: Client connects and optionally authenticates
2. **Metadata Extraction**: Server extracts isolation parameters:
   - Target destination (for destination isolation)
   - Username (for credential isolation)
   - Source port (for port isolation)
3. **Isolation Key Creation**: Server creates isolation key based on config
4. **Circuit Selection**: Server requests circuit from pool with isolation key
5. **Circuit Reuse**: Subsequent requests with same isolation parameters reuse the circuit
6. **Transparent Operation**: No client-side changes required

### Source Port Detection

The SOCKS5 server automatically detects the client's source port for port-based isolation:

```go
// Automatically extracted from connection
remoteAddr := conn.RemoteAddr().(*net.TCPAddr)
sourcePort := remoteAddr.Port
```

### Example: Multi-User Proxy

```go
cfg := config.DefaultConfig()
cfg.IsolationLevel = "credential"
cfg.IsolateSOCKSAuth = true
cfg.EnableCircuitPrebuilding = true

client, err := client.New(cfg, logger)
if err != nil {
    log.Fatal(err)
}

// Start client - SOCKS5 server will isolate users automatically
if err := client.Start(context.Background()); err != nil {
    log.Fatal(err)
}

// Users alice and bob will get separate circuits:
// curl --socks5 alice:pass@localhost:9050 https://example.com
// curl --socks5 bob:pass@localhost:9050 https://example.com
```

## Performance Considerations

### Memory Usage

Each isolated pool maintains separate circuits:

- **No isolation**: Single shared pool (minimal memory)
- **With isolation**: Multiple pools (increased memory)
- **Impact**: ~1KB per circuit + pool overhead

**Recommendation**: Set appropriate `CircuitPoolMaxSize` based on expected isolation keys.

### Circuit Build Time

- **Pool hits**: Instant (reuse from isolated pool)
- **Pool misses**: 1-5 seconds (build new circuit)
- **Mitigation**: Enable circuit prebuilding

### Monitoring

Track isolation performance with metrics:

```go
metrics := torClient.Metrics()
hitRate := float64(metrics.IsolationHits) / 
           float64(metrics.IsolationHits + metrics.IsolationMisses)
fmt.Printf("Isolation pool hit rate: %.2f%%\n", hitRate * 100)
```

### Tuning

```go
cfg := config.DefaultConfig()

// Increase pool size for more isolation keys
cfg.CircuitPoolMaxSize = 30

// Prebuild circuits for common isolation keys
cfg.EnableCircuitPrebuilding = true
cfg.CircuitPoolMinSize = 5

// Adjust circuit lifetime
cfg.MaxCircuitDirtiness = 10 * time.Minute
```

## Security Model

### Privacy Protection

1. **Credential Hashing**: Usernames and session tokens are hashed (SHA-256) before storage
2. **No PII Leakage**: Isolation keys don't expose sensitive data in logs
3. **Constant-Time Comparison**: Hash comparisons use constant-time operations

```go
// Credentials are automatically hashed
key := circuit.NewIsolationKey(circuit.IsolationCredential).
    WithCredentials("alice")  // SHA-256 hashed internally

// Session tokens are hashed
key := circuit.NewIsolationKey(circuit.IsolationSession).
    WithSessionToken("secret-token")  // SHA-256 hashed internally
```

### Correlation Resistance

Isolation helps prevent:
- **Website fingerprinting**: Different destinations use different circuits
- **User correlation**: Different users can't be linked via circuit sharing
- **Application correlation**: Different apps use different circuits

### Limitations

Circuit isolation does **not** protect against:
- **Timing attacks**: Circuit-level timing is still observable
- **Traffic analysis**: Volume and timing patterns remain visible
- **Exit node surveillance**: Exit node still sees plaintext traffic
- **Guard node correlation**: All circuits share the same guard node

### Best Practices

1. **Combine with other protections**: Use HTTPS, application-level encryption
2. **Regular circuit rotation**: Set appropriate `MaxCircuitDirtiness`
3. **Monitor for anomalies**: Track isolation metrics for unexpected patterns
4. **Test configuration**: Verify isolation is working as expected

## Examples

### Example 1: Multi-User Proxy

```go
// Server automatically isolates by SOCKS5 username
server := socks.NewServer(":9050", circuitManager, logger)

// Users connect with different credentials
// Alice: socks5://alice:pass@localhost:9050
// Bob: socks5://bob:pass@localhost:9050
// Circuits are automatically isolated
```

### Example 2: Application-Level Isolation

```go
// Shopping activity
shoppingKey := circuit.NewIsolationKey(circuit.IsolationSession).
    WithSessionToken("shopping-" + sessionID)
shoppingCirc, _ := pool.GetWithIsolation(ctx, shoppingKey)

// Banking activity
bankingKey := circuit.NewIsolationKey(circuit.IsolationSession).
    WithSessionToken("banking-" + sessionID)
bankingCirc, _ := pool.GetWithIsolation(ctx, bankingKey)

// Different activities use different circuits
```

### Example 3: Per-Destination Isolation

```go
// Configure destination isolation
cfg.IsolateDestinations = true

// Connections are automatically isolated by destination
// www.google.com:443 -> Circuit A
// en.wikipedia.org:443 -> Circuit B
// github.com:443 -> Circuit C
```

See [examples/circuit-isolation](../examples/circuit-isolation) for a complete working example.

## References

- [Tor Project: Stream Isolation](https://www.torproject.org/docs/tor-manual.html.en)
- [SOCKS Extensions for Tor](https://spec.torproject.org/socks-extensions.html)
- [RFC 1928: SOCKS Protocol Version 5](https://tools.ietf.org/html/rfc1928)
- [RFC 1929: Username/Password Authentication for SOCKS V5](https://tools.ietf.org/html/rfc1929)

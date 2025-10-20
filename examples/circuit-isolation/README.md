# Circuit Isolation Example

This example demonstrates how to use circuit isolation in the go-tor client to prevent different applications or users from sharing Tor circuits.

## What is Circuit Isolation?

Circuit isolation ensures that different activities use separate circuits through the Tor network. This helps protect against correlation attacks where an adversary might try to link different activities based on circuit sharing.

## Isolation Levels

The go-tor client supports five isolation levels:

### 1. No Isolation (Default)
**Level:** `IsolationNone`

All connections share circuits from a common pool. This is the default behavior for backward compatibility.

```go
// No isolation key needed
circ, _ := pool.Get(ctx)
```

### 2. Destination-Based Isolation
**Level:** `IsolationDestination`

Each unique destination (host:port) gets its own circuit. Useful for preventing correlation between connections to different sites.

```go
key := circuit.NewIsolationKey(circuit.IsolationDestination).
    WithDestination("example.com:443")
circ, _ := pool.GetWithIsolation(ctx, key)
```

### 3. Credential-Based Isolation
**Level:** `IsolationCredential`

Each SOCKS5 username gets its own circuit. Ideal for multi-user scenarios or per-application isolation.

```go
key := circuit.NewIsolationKey(circuit.IsolationCredential).
    WithCredentials("alice")
circ, _ := pool.GetWithIsolation(ctx, key)
```

**SOCKS5 Usage:**
```bash
# Alice's traffic uses one circuit
curl --socks5 user:alice@localhost:9050 https://example.com

# Bob's traffic uses a different circuit
curl --socks5 user:bob@localhost:9050 https://example.com
```

### 4. Port-Based Isolation
**Level:** `IsolationPort`

Each client source port gets its own circuit. Automatically isolates different applications connecting from different ports.

```go
key := circuit.NewIsolationKey(circuit.IsolationPort).
    WithSourcePort(12345)
circ, _ := pool.GetWithIsolation(ctx, key)
```

### 5. Session-Based Isolation
**Level:** `IsolationSession`

Custom session tokens allow application-level control over isolation. You can create arbitrary isolation boundaries.

```go
key := circuit.NewIsolationKey(circuit.IsolationSession).
    WithSessionToken("shopping-session-abc")
circ, _ := pool.GetWithIsolation(ctx, key)
```

## Running the Example

```bash
cd examples/circuit-isolation
go run main.go
```

## Expected Output

```
=== Circuit Isolation Example ===

Example 1: No Isolation (Backward Compatible)
-----------------------------------------------
Built circuit 1
Got circuit 1 for first request (no isolation)
Got circuit 1 for second request (no isolation)
✓ Circuits are shared (same circuit ID)

Example 2: Destination-Based Isolation
---------------------------------------
Built circuit 2
Got circuit 2 for www.google.com:443
Built circuit 3
Got circuit 3 for en.wikipedia.org:443
✓ Different destinations use different circuits
Got circuit 2 for www.google.com:443 (second request)
✓ Same destination reuses circuit from isolated pool

...
```

## Configuration

Enable isolation in your torrc or configuration file:

```
# torrc example
IsolationLevel destination

# Or use specific flags
IsolateDestinations 1
IsolateSOCKSAuth 1
IsolateClientPort 1
```

Go configuration:

```go
cfg := config.DefaultConfig()

// Enable destination isolation
cfg.IsolationLevel = "destination"
cfg.IsolateDestinations = true

// Enable credential isolation
cfg.IsolateSOCKSAuth = true

// Enable port isolation
cfg.IsolateClientPort = true
```

## Security Considerations

1. **Privacy Protection**: Credentials and session tokens are hashed (SHA-256) before storage to protect user privacy
2. **Performance Impact**: Each isolated pool maintains separate circuits, increasing memory usage
3. **Circuit Lifetime**: Isolated circuits follow the same lifecycle as regular circuits (MaxCircuitDirtiness)
4. **Backward Compatibility**: Default configuration maintains existing behavior (no isolation)

## Use Cases

### Multi-User Proxy
Different users connect with different SOCKS5 credentials, each getting isolated circuits:
```bash
# User Alice
export http_proxy=socks5://alice:password@localhost:9050
curl https://example.com

# User Bob
export http_proxy=socks5://bob:password@localhost:9050
curl https://example.com
```

### Application Separation
Different applications automatically isolated by source port:
```
Browser (port 45678) → Circuit A
Email client (port 45679) → Circuit B
Chat app (port 45680) → Circuit C
```

### Session-Based Privacy
Web application creates isolated sessions for different activities:
```go
// Banking session
bankingKey := circuit.NewIsolationKey(circuit.IsolationSession).
    WithSessionToken("banking-" + sessionID)

// Shopping session
shoppingKey := circuit.NewIsolationKey(circuit.IsolationSession).
    WithSessionToken("shopping-" + sessionID)
```

## Performance Tuning

Adjust circuit pool size based on expected isolation keys:

```go
cfg.CircuitPoolMaxSize = 20  // Allow more isolated circuits
cfg.CircuitPoolMinSize = 5   // Prebuild more circuits
```

## Monitoring

Check isolation metrics:
```go
metrics := torClient.Metrics()
fmt.Printf("Isolated circuits: %d\n", metrics.IsolatedCircuits)
fmt.Printf("Isolation keys: %d\n", metrics.IsolationKeys)
fmt.Printf("Isolation hits: %d\n", metrics.IsolationHits)
fmt.Printf("Isolation misses: %d\n", metrics.IsolationMisses)
```

## References

- [Tor Project: Stream Isolation](https://www.torproject.org/docs/tor-manual.html.en#IsolateDestAddr)
- [SOCKS Extensions for Tor](https://spec.torproject.org/socks-extensions.html)
- [go-tor API Documentation](../../docs/API.md)

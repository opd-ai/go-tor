# Relay Security Hardening

This document describes the security hardening features implemented for the go-tor relay functionality.

## Overview

The relay security hardening implementation provides three key components:

1. **Rate Limiting** - Token bucket-based rate limiting for circuits, connections, and cells
2. **DoS Protection** - Connection and circuit count limits to prevent resource exhaustion
3. **Metrics** - Comprehensive metrics for monitoring relay operations and detecting attacks

## Rate Limiting

**File**: `pkg/relay/ratelimit.go`

The `RateLimiter` implements token bucket rate limiting using `golang.org/x/time/rate`:

### Features

- **Circuit Creation Rate Limiting**: Limits the rate at which new circuits can be created (default: 10/sec with burst of 20)
- **Per-IP Connection Rate Limiting**: Limits connection rate per IP address (default: 5/sec with burst of 10)
- **Per-Circuit Cell Processing**: Limits cell processing rate per circuit (default: 100/sec with burst of 200)
- **Automatic Cleanup**: Periodically removes stale limiters to prevent memory leaks
- **Context-Aware**: Respects context cancellation for graceful shutdown

### Configuration

```go
cfg := &relay.RateLimiterConfig{
    CircuitRate:     10.0,  // circuits per second
    CircuitBurst:    20,    // max burst size
    ConnectionRate:  5.0,   // connections per second per IP
    ConnectionBurst: 10,
    CellRate:        100.0, // cells per second per circuit
    CellBurst:       200,
    CleanupInterval: 5 * time.Minute,
    Metrics:         relayMetrics, // optional
}

rateLimiter := relay.NewRateLimiter(cfg)
```

### Usage

```go
// Check if circuit creation is allowed
if err := rateLimiter.AllowCircuit(ctx); err != nil {
    return err // rate limit exceeded
}

// Check if connection from IP is allowed
if err := rateLimiter.AllowConnection(ctx, clientIP); err != nil {
    conn.Close()
    return err
}

// Check if cell processing is allowed
if err := rateLimiter.AllowCell(ctx, circuitID); err != nil {
    // Drop cell or queue for later
    return err
}

// Clean up when circuit closes
rateLimiter.RemoveCircuit(circuitID)
```

### Metrics Integration

When configured with metrics, the rate limiter automatically tracks:
- `RateLimitedCircuits`: Number of circuits rejected due to rate limits
- `RateLimitedConnections`: Number of connections rejected due to rate limits
- `RateLimitedCells`: Number of cells delayed or rejected due to rate limits

## DoS Protection

**File**: `pkg/relay/protection.go`

The `ProtectionManager` implements resource limits to prevent DoS attacks:

### Features

- **Per-IP Connection Limits**: Maximum connections per IP address (default: 10)
- **Per-Connection Circuit Limits**: Maximum circuits per connection (default: 1000)
- **Global Connection Limit**: Total connection limit across all IPs (default: 5000)
- **Thread-Safe**: Uses atomic operations and mutexes for concurrent access
- **Automatic Cleanup**: Removes stale trackers after inactivity

### Configuration

```go
cfg := &relay.ProtectionConfig{
    MaxConnectionsPerIP:    10,
    MaxCircuitsPerConn:     1000,
    MaxTotalConnections:    5000,
    CleanupInterval:        5 * time.Minute,
    Metrics:                relayMetrics, // optional
}

protection := relay.NewProtectionManager(cfg)
```

### Usage

```go
// Check if connection should be accepted
if err := protection.AllowConnection(remoteAddr); err != nil {
    // Reject connection - DoS limit exceeded
    conn.Close()
    return err
}
defer protection.ReleaseConnection(remoteAddr)

// Check if circuit creation is allowed
if err := protection.AllowCircuit(remoteAddr); err != nil {
    // Reject circuit - DoS limit exceeded
    return err
}
defer protection.ReleaseCircuit(remoteAddr)

// Get current statistics
stats := protection.Stats()
log.Printf("Active connections: %d/%d", stats.TotalConnections, stats.MaxTotalConnections)
```

### Metrics Integration

When configured with metrics, the protection manager tracks:
- `DoSConnectionsRejected`: Connections rejected due to DoS limits
- `DoSCircuitsRejected`: Circuits rejected due to DoS limits
- `DoSEventsDetected`: Potential DoS events detected
- `ActiveConnections`: Current active connection count

## Relay Metrics

**File**: `pkg/relay/metrics.go`

The `RelayMetrics` package provides comprehensive monitoring for relay operations:

### Metric Categories

#### Circuit Metrics
- `CircuitsCreated`: Total circuits created (server-side)
- `CircuitsDestroyed`: Total circuits destroyed
- `CircuitsExtended`: Total circuit extensions processed
- `ActiveCircuits`: Currently active circuits (gauge)
- `CircuitCreationTime`: Histogram of circuit creation times

#### Connection Metrics
- `ConnectionsAccepted`: Total connections accepted
- `ConnectionsRejected`: Total connections rejected
- `ConnectionsClosed`: Total connections closed
- `ActiveConnections`: Currently active connections (gauge)
- `ConnectionDuration`: Histogram of connection lifetimes

#### Cell Forwarding Metrics
- `CellsReceived`: Total cells received from clients
- `CellsForwarded`: Total cells forwarded to next hop
- `CellsDropped`: Total cells dropped (errors, rate limits)
- `RelayEarlyViolations`: RELAY_EARLY limit violations
- `CellForwardingTime`: Histogram of cell forwarding times

#### Bandwidth Metrics
- `BytesReceived`: Total bytes received
- `BytesTransmitted`: Total bytes transmitted
- `BandwidthUsage`: Current bandwidth usage (bytes/sec)

#### Security Metrics
- Rate limiting metrics (see Rate Limiting section)
- DoS protection metrics (see DoS Protection section)
- Exit policy violation metrics

#### Error Metrics
- `HandshakeErrors`: TLS/link protocol handshake errors
- `CellDecodingErrors`: Cell decoding errors
- `ProtocolErrors`: Protocol violations
- `ExtensionErrors`: Circuit extension errors

### Usage

```go
// Create metrics instance
metrics := relay.NewRelayMetrics()

// Record events
metrics.CircuitsCreated.Inc()
metrics.CellsReceived.Add(100)
metrics.CircuitCreationTime.Observe(500) // 500ms
metrics.ActiveCircuits.Inc()

// Update uptime periodically
ticker := time.NewTicker(1 * time.Minute)
go func() {
    for range ticker.C {
        metrics.UpdateUptime()
    }
}()

// Get snapshot for reporting
snapshot := metrics.Snapshot()
fmt.Printf("Circuits created: %d\n", snapshot.CircuitsCreated)
fmt.Printf("Active circuits: %d\n", snapshot.ActiveCircuits)
fmt.Printf("Bytes received: %d\n", snapshot.BytesReceived)
```

## Integration Example

Complete example integrating all three components:

```go
// Create metrics
metrics := relay.NewRelayMetrics()

// Configure rate limiting
rateLimitCfg := &relay.RateLimiterConfig{
    CircuitRate:     10.0,
    CircuitBurst:    20,
    ConnectionRate:  5.0,
    ConnectionBurst: 10,
    CellRate:        100.0,
    CellBurst:       200,
    Metrics:         metrics,
}
rateLimiter := relay.NewRateLimiter(rateLimitCfg)

// Configure DoS protection
protectionCfg := &relay.ProtectionConfig{
    MaxConnectionsPerIP: 10,
    MaxCircuitsPerConn:  1000,
    MaxTotalConnections: 5000,
    Metrics:             metrics,
}
protection := relay.NewProtectionManager(protectionCfg)

// Handle incoming connection
func handleConnection(ctx context.Context, conn net.Conn) error {
    remoteAddr := conn.RemoteAddr().String()
    
    // 1. Check DoS protection
    if err := protection.AllowConnection(remoteAddr); err != nil {
        metrics.ConnectionsRejected.Inc()
        return err
    }
    defer protection.ReleaseConnection(remoteAddr)
    
    // 2. Check rate limit
    if err := rateLimiter.AllowConnection(ctx, remoteAddr); err != nil {
        metrics.ConnectionsRejected.Inc()
        return err
    }
    
    metrics.ConnectionsAccepted.Inc()
    metrics.ActiveConnections.Inc()
    defer metrics.ActiveConnections.Dec()
    
    // Process connection...
    return nil
}

// Handle circuit creation
func handleCreateCircuit(ctx context.Context, remoteAddr string, circuitID uint32) error {
    // 1. Check DoS protection
    if err := protection.AllowCircuit(remoteAddr); err != nil {
        metrics.DoSCircuitsRejected.Inc()
        return err
    }
    defer protection.ReleaseCircuit(remoteAddr)
    
    // 2. Check rate limit
    if err := rateLimiter.AllowCircuit(ctx); err != nil {
        return err
    }
    
    start := time.Now()
    // Create circuit...
    metrics.CircuitCreationTime.Observe(time.Since(start).Milliseconds())
    metrics.CircuitsCreated.Inc()
    metrics.ActiveCircuits.Inc()
    
    return nil
}
```

## Performance Considerations

### Rate Limiting
- Token bucket algorithm has O(1) time complexity
- Memory usage scales with number of active IPs and circuits
- Periodic cleanup prevents unbounded memory growth
- Default cleanup interval: 5 minutes

### DoS Protection
- Connection tracking uses atomic operations for speed
- Per-IP and per-connection maps require memory
- Cleanup removes stale entries after 10 minutes of inactivity
- Thread-safe for concurrent access

### Metrics
- All metric operations use atomic primitives
- No locks required for metric updates
- Snapshot operation is O(1) and thread-safe
- Minimal performance overhead

## Security Notes

### Educational Purpose
This implementation is for **educational and research purposes only**. Do not use in production for actual anonymity needs.

### Rate Limiting Best Practices
- Configure rates based on expected legitimate traffic
- Monitor rate-limited events to detect attacks
- Adjust burst sizes for legitimate usage spikes
- Use context cancellation for graceful shutdown

### DoS Protection Best Practices
- Set limits based on available system resources
- Monitor rejected connections/circuits
- Implement gradual increase under attack
- Log potential DoS events for analysis

### Metrics Best Practices
- Export metrics to monitoring system (Prometheus, etc.)
- Set up alerts for anomalies
- Track trends over time
- Use metrics to tune rate limits and protection

## Testing

All components have comprehensive test coverage:

- `pkg/relay/ratelimit_test.go` - 11 tests, >85% coverage
- `pkg/relay/protection_test.go` - 11 tests, >85% coverage  
- `pkg/relay/metrics_test.go` - 14 tests, 100% coverage

Run tests:
```bash
go test -v ./pkg/relay/... -run "TestRateLimiter|TestProtection|TestRelayMetrics"
```

## References

- [Tor Specification §5.3](https://spec.torproject.org/tor-spec) - Cell rate limiting
- [golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate) - Token bucket implementation
- Token Bucket Algorithm: [Wikipedia](https://en.wikipedia.org/wiki/Token_bucket)

---

**Implementation Date**: January 25, 2026  
**Status**: Complete

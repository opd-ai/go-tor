# Circuit Rate Limiting

## Overview

Circuit rate limiting protects the Tor client from resource exhaustion by controlling the rate at which circuits are created. This prevents denial-of-service (DoS) attacks and ensures fair resource allocation.

## Features

- **Token Bucket Algorithm**: Implements industry-standard rate limiting
- **Configurable Rate and Burst**: Fine-tune limits for your use case
- **Metrics Tracking**: Monitor rate limiting activity
- **Zero Overhead When Disabled**: No performance impact when not configured

## Configuration

Circuit rate limiting is controlled by two configuration parameters:

```go
// Allow 10 circuit creations per second
cfg.CircuitCreationsPerSecond = 10.0

// Allow burst of 5 circuits
cfg.CircuitCreationsBurst = 5
```

### Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `CircuitCreationsPerSecond` | `float64` | `10.0` | Maximum sustained circuit creation rate (tokens per second) |
| `CircuitCreationsBurst` | `int` | `5` | Burst capacity (maximum accumulated tokens) |

### Recommended Values

| Use Case | Rate | Burst | Notes |
|----------|------|-------|-------|
| **Default** | 10.0 | 5 | Balanced protection and performance |
| **High Traffic** | 50.0 | 20 | For servers or high-load scenarios |
| **Conservative** | 5.0 | 2 | Strong DoS protection |
| **Disabled** | 0.0 | 0 | No rate limiting (not recommended) |

## How It Works

### Token Bucket Algorithm

1. **Bucket Capacity**: Set to `CircuitCreationsBurst`
2. **Refill Rate**: `CircuitCreationsPerSecond` tokens per second
3. **Circuit Creation**: Consumes one token
4. **No Tokens**: Request waits until token is available

### Example Timeline

With `CircuitCreationsPerSecond=10.0` and `CircuitCreationsBurst=5`:

```
Circuit  1: 0ms      (burst - immediate)
Circuit  2: 0ms      (burst - immediate)
Circuit  3: 0ms      (burst - immediate)
Circuit  4: 0ms      (burst - immediate)
Circuit  5: 0ms      (burst - immediate)
Circuit  6: 100ms    (rate limited - wait for token)
Circuit  7: 200ms    (rate limited - wait for token)
Circuit  8: 300ms    (rate limited - wait for token)
Circuit  9: 400ms    (rate limited - wait for token)
Circuit 10: 500ms    (rate limited - wait for token)
```

## Usage Example

```go
package main

import (
    "context"
    "log"

    "github.com/opd-ai/go-tor/pkg/client"
    "github.com/opd-ai/go-tor/pkg/config"
    "github.com/opd-ai/go-tor/pkg/logger"
)

func main() {
    // Create configuration with rate limiting
    cfg := config.DefaultConfig()
    cfg.CircuitCreationsPerSecond = 10.0
    cfg.CircuitCreationsBurst = 5
    
    // Create and start client
    logger := logger.NewDefault()
    client, err := client.New(cfg, logger)
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }
    defer client.Close()
    
    err = client.Start(context.Background())
    if err != nil {
        log.Fatalf("Failed to start client: %v", err)
    }
    
    // Client will now rate limit circuit creation automatically
    // First 5 circuits: immediate (burst)
    // Subsequent: max 10 per second
}
```

## Metrics

Circuit rate limiting records the following metrics:

### `RateLimitedCircuits`
- **Type**: Counter
- **Description**: Number of circuit creation requests that were rate limited
- **Access**: `metrics.RateLimitedCircuits`

### `RateLimitWaitTime`
- **Type**: Histogram
- **Description**: Time spent waiting for rate limiter tokens
- **Access**: `metrics.RateLimitWaitTimeAvg` (average)

### Monitoring Example

```go
// Get metrics snapshot
snapshot := client.GetMetrics()

// Check rate limiting activity
if snapshot.RateLimitedCircuits > 0 {
    log.Printf("Rate limited %d circuits", snapshot.RateLimitedCircuits)
    log.Printf("Average wait time: %v", snapshot.RateLimitWaitTimeAvg)
}
```

## Behavior

### Normal Operation
- First `CircuitCreationsBurst` circuits: Created immediately
- Subsequent circuits: Rate limited to `CircuitCreationsPerSecond`
- Tokens accumulate up to burst capacity when idle

### Under Load
- Circuit creation requests wait for available tokens
- Fair queuing ensures requests are served in order
- Context cancellation respected (requests can timeout)

### Error Handling
- Rate-limited requests that timeout return error with context
- Metrics track both successful waits and rate-limited rejections

## Performance Impact

### When Enabled
- **Minimal CPU overhead**: O(1) token bucket operations
- **Small memory footprint**: Single rate limiter per client (~100 bytes)
- **No goroutines**: No background workers

### When Disabled (default 0.0)
- **Zero overhead**: No rate checking performed
- **No allocations**: Limiter not created

## Best Practices

### 1. Set Appropriate Limits
```go
// Bad: Too restrictive for normal use
cfg.CircuitCreationsPerSecond = 0.1  // Only 1 per 10 seconds
cfg.CircuitCreationsBurst = 1

// Good: Balanced protection
cfg.CircuitCreationsPerSecond = 10.0
cfg.CircuitCreationsBurst = 5
```

### 2. Monitor Metrics
```go
// Periodically check rate limiting activity
ticker := time.NewTicker(1 * time.Minute)
defer ticker.Stop()

for range ticker.C {
    snapshot := client.GetMetrics()
    if snapshot.RateLimitedCircuits > 100 {
        log.Warn("High rate limiting activity detected")
    }
}
```

### 3. Adjust Based on Use Case
- **Interactive applications**: Higher burst (10-20) for responsiveness
- **Background services**: Lower rate (5/sec) for steady operation
- **High-security**: Conservative limits with monitoring

## Architecture

### Components

1. **Builder (`pkg/circuit/builder.go`)**
   - Integrates rate limiter into circuit building
   - Validates paths before acquiring tokens
   - Records metrics on wait times

2. **Rate Limiter (`pkg/ratelimit/limiter.go`)**
   - Token bucket implementation
   - Thread-safe operations
   - Context-aware waiting

3. **Metrics (`pkg/metrics/metrics.go`)**
   - Tracks rate limiting statistics
   - Aggregates wait times
   - Exposes via HTTP/control protocol

### Flow Diagram

```
Circuit Request
     |
     v
[Rate Limiter Check]
     |
     ├─> Tokens Available ──> Build Circuit
     |
     └─> No Tokens ──> Wait for Token
              |              (or timeout)
              v
         [Retry Check]
```

## Troubleshooting

### Problem: All Circuits Rate Limited

**Symptoms**: `RateLimitedCircuits` continuously increasing

**Solutions**:
- Increase `CircuitCreationsPerSecond`
- Increase `CircuitCreationsBurst`
- Check if circuit churn is excessive

### Problem: High Wait Times

**Symptoms**: `RateLimitWaitTimeAvg` > 100ms

**Solutions**:
- Increase rate limit
- Optimize circuit reuse
- Use circuit pooling

### Problem: Unexpected Rate Limiting

**Symptoms**: Rate limiting active with low traffic

**Solutions**:
- Check configuration values are correct
- Verify multiple clients aren't sharing resources
- Review circuit creation patterns

## Testing

### Unit Tests
```bash
go test -v ./pkg/circuit -run TestBuilderRateLimit
```

### Load Testing
```bash
# Run example with monitoring
go run examples/circuit-ratelimit/main.go
```

### Integration Tests
Circuit rate limiting is tested in:
- `pkg/circuit/ratelimit_test.go`: Unit tests
- Integration tests: Real network scenarios

## Related Documentation

- [ROADMAP.md](../ROADMAP.md): Future enhancements
- [AUDIT.md](../AUDIT.md): Compliance status
- [Circuit Padding](CIRCUIT_PADDING.md): Traffic analysis resistance

## References

- **Token Bucket Algorithm**: [Wikipedia](https://en.wikipedia.org/wiki/Token_bucket)
- **Rate Limiting Patterns**: [NGINX Blog](https://www.nginx.com/blog/rate-limiting-nginx/)

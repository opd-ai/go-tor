# Enhanced Error Recovery

## Overview

Phase 9.13.2 adds production-grade error recovery mechanisms:
- **Exponential Backoff Retry** with jitter for transient failures
- **Circuit Breaker Pattern** for fast failure and service protection
- **Comprehensive Error Statistics** for monitoring and debugging

## Retry Logic

### Quick Start

```go
import (
    "context"
    "github.com/opd-ai/go-tor/pkg/errors"
)

// Simple retry with defaults (3 attempts, 1s initial delay)
err := errors.Retry(ctx, func() error {
    // Your operation here
    return connectToRelay()
})
```

### Custom Retry Policy

```go
policy := &errors.RetryPolicy{
    MaxAttempts:  5,                    // Max retry attempts
    InitialDelay: 500 * time.Millisecond, // First retry delay
    MaxDelay:     30 * time.Second,     // Cap on delay
    Multiplier:   2.0,                  // Exponential backoff factor
    Jitter:       0.1,                  // 10% randomness to prevent thundering herd
    RetryableErrors: map[errors.ErrorCategory]bool{
        errors.CategoryConnection: true,
        errors.CategoryNetwork:    true,
        errors.CategoryTimeout:    true,
    },
}

err := errors.RetryWithPolicy(ctx, policy, func() error {
    return buildCircuit()
})
```

### Predefined Policies

```go
// Aggressive retry for critical operations
policy := errors.AggressiveRetryPolicy() // 5 attempts, 500ms initial delay

// Conservative retry for expensive operations  
policy := errors.ConservativeRetryPolicy() // 2 attempts, 2s initial delay

// Default balanced policy
policy := errors.DefaultRetryPolicy() // 3 attempts, 1s initial delay
```

### Retry with Statistics

```go
stats, err := errors.RetryWithStats(ctx, policy, func() error {
    return fetchDescriptor()
})

log.Printf("Attempts: %d, Duration: %v, Success: %v",
    stats.TotalAttempts,
    stats.TotalDuration,
    stats.SuccessfulRetry)
```

### Retry with Callbacks

```go
err := errors.RetryWithCallbackFunc(ctx, policy, func() error {
    return createCircuit()
}, func(attempt int, err error, willRetry bool) {
    log.Printf("Attempt %d failed: %v (will retry: %v)", attempt, err, willRetry)
    metrics.RecordRetryAttempt(attempt)
})
```

## Circuit Breaker

### Quick Start

```go
import "github.com/opd-ai/go-tor/pkg/errors"

cb := errors.NewCircuitBreaker(nil) // Use defaults

err := cb.Execute(ctx, func() error {
    return connectToHSDir()
})
```

### Custom Configuration

```go
config := &errors.CircuitBreakerConfig{
    MaxFailures:         5,              // Open after 5 consecutive failures
    Timeout:             30 * time.Second, // Stay open for 30s
    HalfOpenMaxRequests: 1,              // Test with 1 request in half-open
    FailureThreshold:    0.5,            // Or open at 50% error rate
    MinRequests:         10,             // Need 10 requests before checking rate
    OnStateChange: func(from, to errors.CircuitState) {
        log.Printf("Circuit %s -> %s", from, to)
        metrics.RecordCircuitStateChange(to)
    },
}

cb := errors.NewCircuitBreaker(config)
```

### Circuit States

The circuit breaker has three states:

1. **Closed** (Normal Operation)
   - All requests are attempted
   - Failures are counted
   - Opens if failure threshold exceeded

2. **Open** (Fast Failure)
   - All requests fail immediately
   - No backend calls made
   - After timeout, transitions to half-open

3. **Half-Open** (Testing)
   - Limited requests allowed
   - Success closes the circuit
   - Failure reopens the circuit

### Monitoring Circuit State

```go
// Get current state
state := cb.State()
log.Printf("Circuit is %s", state)

// Get detailed statistics
stats := cb.Stats()
log.Printf("State: %s, Failures: %d, Error Rate: %.2f%%",
    stats.State,
    stats.Failures,
    stats.ErrorRate*100)
```

### Manual Control

```go
// Manually reset circuit to closed
cb.Reset()

// Force circuit open (for maintenance)
cb.ForceOpen()
```

### Combining Retry and Circuit Breaker

```go
cb := errors.NewCircuitBreaker(nil)
retryPolicy := errors.DefaultRetryPolicy()

err := cb.ExecuteWithRetry(ctx, retryPolicy, func() error {
    return fetchConsensus()
})
```

## Best Practices

### 1. Choose Appropriate Retry Policies

```go
// Critical path: be aggressive
buildCircuit := errors.AggressiveRetryPolicy()

// Background tasks: be conservative
refreshGuards := errors.ConservativeRetryPolicy()

// Most operations: use defaults
fetchData := errors.DefaultRetryPolicy()
```

### 2. Use Jitter to Prevent Thundering Herd

```go
policy := errors.DefaultRetryPolicy()
policy.Jitter = 0.2 // 20% randomness
// This prevents all clients retrying at exactly the same time
```

### 3. Respect Context Cancellation

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Retry will respect context timeout
err := errors.Retry(ctx, func() error {
    return longRunningOperation()
})
```

### 4. Monitor Circuit Breaker State

```go
config := errors.DefaultCircuitBreakerConfig()
config.OnStateChange = func(from, to errors.CircuitState) {
    // Log state changes
    log.Printf("Circuit breaker: %s -> %s", from, to)
    
    // Update metrics
    circuitStateMetric.Set(float64(to))
    
    // Alert on open
    if to == errors.StateOpen {
        alerts.SendCircuitOpen("hsdir-fetch")
    }
}
```

### 5. Set Appropriate Thresholds

```go
// High-traffic service: use error rate threshold
config := &errors.CircuitBreakerConfig{
    FailureThreshold: 0.5,  // 50% error rate
    MinRequests:      100,  // Need 100 requests to be significant
}

// Low-traffic service: use max failures
config := &errors.CircuitBreakerConfig{
    MaxFailures: 3,  // Just 3 consecutive failures
}
```

### 6. Use Different Circuit Breakers for Different Services

```go
// One circuit breaker per external dependency
var (
    hsdirCircuit    = errors.NewCircuitBreaker(nil)
    directoryCircuit = errors.NewCircuitBreaker(nil)
    guardCircuit     = errors.NewCircuitBreaker(nil)
)

// This prevents one failing service from affecting others
```

## Integration Examples

### Circuit Manager with Retry

```go
type CircuitManager struct {
    breaker *errors.CircuitBreaker
    policy  *errors.RetryPolicy
}

func (cm *CircuitManager) BuildCircuit(ctx context.Context) (*Circuit, error) {
    var circuit *Circuit
    
    err := cm.breaker.ExecuteWithRetry(ctx, cm.policy, func() error {
        var err error
        circuit, err = createAndExtendCircuit(ctx)
        return err
    })
    
    return circuit, err
}
```

### HSDir Fetcher with Error Recovery

```go
type HSDir struct {
    breaker *errors.CircuitBreaker
}

func (h *HSDir) FetchDescriptor(ctx context.Context, addr string) (*Descriptor, error) {
    policy := &errors.RetryPolicy{
        MaxAttempts:  3,
        InitialDelay: 1 * time.Second,
        MaxDelay:     10 * time.Second,
        Multiplier:   2.0,
        Jitter:       0.1,
    }
    
    var desc *Descriptor
    err := h.breaker.ExecuteWithRetry(ctx, policy, func() error {
        var err error
        desc, err = h.fetchFromHSDir(ctx, addr)
        return err
    })
    
    return desc, err
}
```

### Connection Pool with Circuit Protection

```go
type ConnectionPool struct {
    breaker *errors.CircuitBreaker
}

func (cp *ConnectionPool) GetConnection(ctx context.Context) (*Connection, error) {
    return cp.breaker.Execute(ctx, func() error {
        return cp.createConnection(ctx)
    })
}
```

## Error Categories and Retryability

By default, these error categories are retryable:

- `CategoryConnection` - Connection failures
- `CategoryCircuit` - Circuit building failures  
- `CategoryDirectory` - Directory fetch failures
- `CategoryTimeout` - Timeout errors
- `CategoryNetwork` - Network errors

These are NOT retried by default (permanent errors):

- `CategoryProtocol` - Protocol violations
- `CategoryCrypto` - Cryptographic failures
- `CategoryConfiguration` - Configuration errors

### Customizing Retryable Categories

```go
policy := errors.DefaultRetryPolicy()

// Add protocol errors as retryable (not recommended)
policy.RetryableErrors[errors.CategoryProtocol] = true

// Remove timeout errors from retry (if they're terminal)
delete(policy.RetryableErrors, errors.CategoryTimeout)

// Or set to nil to only respect Retryable flag on errors
policy.RetryableErrors = nil
```

## Performance Considerations

### Retry Overhead

- Initial delay: Configurable (default 1s)
- Exponential backoff: Prevents rapid retry storms
- Jitter: Adds ~10% randomness to delays
- Max delay cap: Prevents unbounded waiting

### Circuit Breaker Overhead

- State check: O(1) with read lock
- Request recording: O(1) with write lock
- Memory: Minimal (counters only)
- Thread-safe: Yes (mutex protected)

### Recommendations

```go
// For high-frequency operations
policy := &errors.RetryPolicy{
    MaxAttempts:  2,              // Quick retry
    InitialDelay: 100 * time.Millisecond,
    MaxDelay:     1 * time.Second,
}

// For low-frequency, critical operations
policy := &errors.RetryPolicy{
    MaxAttempts:  5,              // More attempts
    InitialDelay: 1 * time.Second,
    MaxDelay:     60 * time.Second,
}
```

## Testing

### Testing with Retry

```go
func TestRetryableOperation(t *testing.T) {
    attempts := 0
    policy := &errors.RetryPolicy{
        MaxAttempts:  3,
        InitialDelay: 10 * time.Millisecond, // Fast for tests
        MaxDelay:     50 * time.Millisecond,
    }
    
    err := errors.RetryWithPolicy(context.Background(), policy, func() error {
        attempts++
        if attempts < 2 {
            return errors.NetworkError("temp failure", fmt.Errorf("network"))
        }
        return nil
    })
    
    assert.NoError(t, err)
    assert.Equal(t, 2, attempts)
}
```

### Testing with Circuit Breaker

```go
func TestCircuitBreakerOpens(t *testing.T) {
    config := &errors.CircuitBreakerConfig{
        MaxFailures: 3,
        Timeout:     50 * time.Millisecond, // Fast for tests
    }
    cb := errors.NewCircuitBreaker(config)
    
    // Generate failures
    for i := 0; i < 3; i++ {
        cb.Execute(context.Background(), func() error {
            return fmt.Errorf("failure")
        })
    }
    
    assert.Equal(t, errors.StateOpen, cb.State())
}
```

## Migration Guide

### From Basic Error Handling

**Before:**
```go
err := connectToRelay()
if err != nil {
    return err
}
```

**After:**
```go
err := errors.Retry(ctx, func() error {
    return connectToRelay()
})
```

### Adding Circuit Breaker

**Before:**
```go
func (c *Client) FetchData(ctx context.Context) error {
    return c.fetch(ctx)
}
```

**After:**
```go
type Client struct {
    breaker *errors.CircuitBreaker
}

func (c *Client) FetchData(ctx context.Context) error {
    return c.breaker.Execute(ctx, func() error {
        return c.fetch(ctx)
    })
}
```

## Troubleshooting

### Retries Taking Too Long

**Problem:** Operations timing out due to retry delays

**Solution:**
```go
// Reduce delays or max attempts
policy := &errors.RetryPolicy{
    MaxAttempts:  2,  // Fewer attempts
    InitialDelay: 500 * time.Millisecond,
    MaxDelay:     5 * time.Second,
}
```

### Circuit Opening Too Frequently

**Problem:** Circuit breaker too sensitive

**Solution:**
```go
config := &errors.CircuitBreakerConfig{
    MaxFailures:      10,  // Increase threshold
    FailureThreshold: 0.7, // Higher error rate needed
    MinRequests:      50,  // More requests before checking
}
```

### Circuit Not Opening

**Problem:** Service degraded but circuit stays closed

**Solution:**
```go
config := &errors.CircuitBreakerConfig{
    MaxFailures:      3,   // Lower threshold
    FailureThreshold: 0.3, // Lower error rate
    MinRequests:      5,   // Fewer requests before checking
}
```

## Metrics Integration

```go
// Track retry attempts
err := errors.RetryWithCallbackFunc(ctx, policy, fn, func(attempt int, err error, willRetry bool) {
    metrics.RetryAttempts.WithLabelValues("circuit_build").Inc()
    if !willRetry && err != nil {
        metrics.RetryFailures.WithLabelValues("circuit_build").Inc()
    }
})

// Track circuit breaker state
config := &errors.CircuitBreakerConfig{
    OnStateChange: func(from, to errors.CircuitState) {
        metrics.CircuitState.WithLabelValues("hsdir").Set(float64(to))
        if to == errors.StateOpen {
            metrics.CircuitOpens.WithLabelValues("hsdir").Inc()
        }
    },
}
```

## See Also

- [Error Handling](errors.md) - Structured error types
- [Observability](METRICS.md) - Metrics and monitoring
- [Configuration](HOT_RELOAD.md) - Hot reload configuration

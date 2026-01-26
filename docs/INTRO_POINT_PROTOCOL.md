# Introduction Point Protocol Implementation

This document describes the introduction point protocol implementation for onion service hosting in go-tor, following the Tor rend-spec-v3.txt specification §3.1.

## Overview

The introduction point protocol allows onion services to establish and maintain circuits to introduction points, which act as meeting points for clients wishing to connect to the service. This implementation provides:

- **Circuit Retry Logic**: Exponential backoff retry for circuit building
- **Health Monitoring**: Continuous monitoring of introduction point health
- **Automatic Rotation**: Replacement of unhealthy or stale introduction points

## Architecture

### Components

1. **IntroPointManager**: Manages the lifecycle of introduction points
2. **IntroPointHealth**: Tracks health metrics for each introduction point
3. **Service Integration**: Seamless integration with the onion service

### Key Features

#### 1. Circuit Retry with Exponential Backoff

When building circuits to introduction points, the system automatically retries failed attempts with exponential backoff:

- **Initial delay**: 2 seconds
- **Max delay**: 30 seconds
- **Max retries**: 3 attempts
- **Backoff formula**: delay = base_delay * 2^attempt (capped at max_delay)

```go
// Example: Building a circuit with retry
manager := NewIntroPointManager(service, logger)
circuit, err := manager.BuildIntroCircuitWithRetry(ctx, relay)
if err != nil {
    // Circuit build failed after all retry attempts
    log.Error("Failed to establish introduction point", "error", err)
}
```

#### 2. Health Monitoring

Each introduction point is continuously monitored for:

- **Liveness**: Regular health checks every 30 seconds
- **Failure tracking**: Records both total and consecutive failures
- **Health status**: Automatically marked unhealthy after 3 consecutive failures
- **Staleness detection**: Identifies intro points older than 24 hours

```go
// Health metrics tracked for each intro point
type IntroPointHealth struct {
    CircuitID        uint32        // Circuit identifier
    LastChecked      time.Time     // Last health check time
    LastSuccess      time.Time     // Last successful interaction
    FailureCount     int           // Total failure count
    ConsecutiveFails int           // Consecutive failures
    Healthy          bool          // Current health status
}
```

#### 3. Automatic Rotation

The service automatically rotates introduction points based on:

- **Health status**: Unhealthy intro points (≥3 consecutive failures)
- **Age**: Stale intro points (>24 hours old)
- **Minimum count**: Maintains configured number of intro points

Rotation happens during the maintenance loop (runs every hour or 2/3 of descriptor lifetime):

1. Identify unhealthy or stale introduction points
2. Remove them from the active set
3. Establish new introduction points as replacements
4. Update and re-publish service descriptor

## Usage

### Basic Service Setup

```go
import (
    "github.com/opd-ai/go-tor/pkg/onion"
    "github.com/opd-ai/go-tor/pkg/circuit"
    "github.com/opd-ai/go-tor/pkg/path"
)

// Configure the service
config := &onion.ServiceConfig{
    NumIntroPoints:     3,                  // Number of introduction points
    DescriptorLifetime: 3 * time.Hour,      // Descriptor validity
    CircuitBuilder:     circuitBuilder,     // Circuit builder instance
    PathSelector:       pathSelector,       // Path selection instance
}

// Create the service
service, err := onion.NewService(config, logger)
if err != nil {
    log.Fatal(err)
}

// Start the service (automatically establishes intro points)
err = service.Start(ctx, hsdirs)
if err != nil {
    log.Fatal(err)
}
```

### Manual Health Monitoring

```go
// Check if a specific intro point is healthy
if manager.IsHealthy(circuitID) {
    log.Info("Introduction point is healthy")
}

// Get all unhealthy intro points
unhealthy := manager.GetUnhealthyIntroPoints()
for _, circuitID := range unhealthy {
    log.Warn("Unhealthy intro point", "circuit", circuitID)
}

// Get stale intro points that need rotation
stale := manager.GetStaleIntroPoints()
for _, circuitID := range stale {
    log.Info("Stale intro point needs rotation", "circuit", circuitID)
}
```

### Recording Success/Failure

The manager automatically tracks success and failure, but you can also record them manually:

```go
// Record successful interaction
manager.RecordSuccess(circuitID)

// Record failure
manager.RecordFailure(circuitID)
```

## Configuration Parameters

### Retry Parameters

```go
const (
    defaultMaxRetries     = 3                    // Maximum retry attempts
    defaultBaseRetryDelay = 2 * time.Second      // Initial retry delay
    defaultMaxRetryDelay  = 30 * time.Second     // Maximum retry delay
)
```

### Health Check Parameters

```go
const (
    defaultHealthCheckInterval = 30 * time.Second  // Health check frequency
    defaultRotationInterval    = 24 * time.Hour    // Max intro point age
    defaultCircuitTimeout      = 30 * time.Second  // Circuit build timeout
)
```

## Implementation Details

### Circuit Building Flow

1. **Path Selection**: Select a path to the introduction point (3-hop circuit)
2. **Circuit Creation**: Build the circuit with specified timeout
3. **ESTABLISH_INTRO**: Send ESTABLISH_INTRO cell with authentication keys
4. **Wait for ACK**: Wait for INTRO_ESTABLISHED acknowledgment
5. **Register**: Register circuit for health monitoring

### Health Check Flow

1. **Periodic Check**: Health checker runs every 30 seconds
2. **Update Timestamp**: Update LastChecked for all intro points
3. **Identify Issues**: Find unhealthy or stale intro points
4. **Log Warnings**: Report issues via structured logging

### Rotation Flow

1. **Identify**: Find unhealthy/stale intro points during maintenance
2. **Remove**: Unregister and remove from active set
3. **Replace**: Establish new intro points to maintain count
4. **Update Descriptor**: Create and publish new descriptor with updated intro points

## Error Handling

### Circuit Build Failures

- Retries with exponential backoff
- Falls back to placeholder for testing if circuit builder unavailable
- Logs warnings for each retry attempt
- Returns error after all retry attempts exhausted

### Health Check Failures

- Tracks consecutive failures separately from total failures
- Marks unhealthy after 3 consecutive failures
- Single success resets consecutive failure count
- Unhealthy intro points scheduled for rotation

## Testing

Comprehensive test coverage includes:

- Registration and unregistration
- Success and failure recording
- Health status transitions
- Backoff calculation
- Stale intro point detection
- Circuit building with retry
- Context cancellation handling

Run tests:

```bash
# Run all intro point tests
go test -v ./pkg/onion/... -run TestIntroPoint

# Run with race detector
go test -race ./pkg/onion/... -run TestIntroPoint

# Short mode (skips long-running tests)
go test -short ./pkg/onion/... -run TestIntroPoint
```

## Specification Compliance

This implementation follows:

- **rend-spec-v3.txt §3.1**: Introduction point protocol
- **rend-spec-v3.txt §3.1.1**: ESTABLISH_INTRO cell format
- **tor-spec.txt §5.1**: Circuit creation and extension

### Differences from C Tor

- **Simplified rotation**: Rotates based on health and age only (C Tor has more complex heuristics)
- **Health check interval**: 30 seconds (configurable vs C Tor's adaptive intervals)
- **Failure threshold**: 3 consecutive failures (matches C Tor default)

## Future Enhancements

Potential improvements for production use:

1. **Adaptive health checks**: Vary frequency based on observed stability
2. **Geographic diversity**: Ensure intro points from different regions
3. **Circuit state verification**: Active keepalive messages to verify circuit liveness
4. **Graceful teardown**: Send proper teardown messages when rotating intro points
5. **Metrics integration**: Export health metrics to observability systems

## References

- [rend-spec-v3.txt](https://spec.torproject.org/rend-spec-v3) - v3 Onion Service Specification
- [tor-spec.txt](https://spec.torproject.org/tor-spec) - Core Tor Protocol
- [AUDIT.md](../../AUDIT.md) - Implementation plan for onion service features

---

**Status**: Implemented (January 2026)  
**Compliance**: rend-spec-v3.txt §3.1 - Introduction Point Protocol

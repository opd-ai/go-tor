# Stream Backpressure Implementation

## Overview

Stream backpressure is a memory management mechanism that prevents buffer exhaustion by pausing reads/writes when buffer utilization exceeds configured thresholds. This implementation provides protection against memory overflow under high load conditions.

## Features

- **Hysteresis-based control**: Uses separate high and low water marks to prevent oscillation
- **Independent send/receive**: Send and receive buffers are managed independently
- **Metrics tracking**: Records `BackpressurePauses` and `BackpressureResumes` events
- **Zero overhead when disabled**: No performance impact if backpressure is not configured
- **Thread-safe**: Concurrent operations are safely handled with atomic operations

## Configuration

Stream backpressure is configured through two parameters in `Config`:

```go
type Config struct {
    // High water mark threshold (bytes) - backpressure applied when exceeded
    StreamBufferHighWaterMark int // Default: 65536 (64KB)
    
    // Low water mark threshold (bytes) - backpressure released when below this
    StreamBufferLowWaterMark  int // Default: 16384 (16KB)
}
```

### Defaults

- **High Water Mark**: 65536 bytes (64KB)
- **Low Water Mark**: 16384 bytes (16KB)

These defaults provide a 4:1 hysteresis ratio to prevent rapid pause/resume cycles.

## Behavior

### Backpressure Lifecycle

1. **Normal State**: Buffer size is below high water mark
   - Data flows freely
   - No backpressure applied

2. **Pause Triggered**: Buffer size reaches or exceeds high water mark
   - Backpressure is applied
   - New writes/reads are rejected with error
   - Metrics: `BackpressurePauses` counter incremented

3. **Paused State**: Buffer size is between low and high water marks
   - Backpressure remains active (hysteresis)
   - Buffer continues draining through consumption

4. **Resume Triggered**: Buffer size drops to or below low water mark
   - Backpressure is released
   - Normal operation resumes
   - Metrics: `BackpressureResumes` counter incremented

### Error Messages

When backpressure is active, operations return specific errors:

- Send operations: `"send buffer full (backpressure applied)"`
- Receive operations: `"receive buffer full (backpressure applied)"`

## Usage Example

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "github.com/opd-ai/go-tor/pkg/config"
    "github.com/opd-ai/go-tor/pkg/metrics"
    "github.com/opd-ai/go-tor/pkg/stream"
)

func main() {
    // Configure backpressure thresholds
    cfg := config.DefaultConfig()
    cfg.StreamBufferHighWaterMark = 100000  // 100KB
    cfg.StreamBufferLowWaterMark = 25000    // 25KB
    
    // Create metrics tracker
    m := metrics.New()
    
    // Create backpressure controller
    bp := stream.NewBackpressureState(cfg, m)
    
    // Create stream and attach backpressure
    s := stream.NewStream(1, 100, "example.com", 443, nil)
    s.SetBackpressure(bp)
    s.SetState(stream.StateConnected)
    
    // Send data - will apply backpressure if buffer fills
    data := make([]byte, 50000)
    err := s.Send(data)
    if err != nil {
        fmt.Printf("Send failed: %v\n", err)
    }
    
    // Check buffer status
    fmt.Printf("Send buffer size: %d bytes\n", s.GetSendBufferSize())
    fmt.Printf("Backpressure paused: %v\n", bp.IsSendPaused())
    
    // Consume data to release backpressure
    ctx := context.Background()
    consumed, _ := s.SendData(ctx)
    fmt.Printf("Consumed: %d bytes\n", len(consumed))
    
    // Check metrics
    snapshot := m.Snapshot()
    fmt.Printf("Backpressure pauses: %d\n", snapshot.BackpressurePauses)
    fmt.Printf("Backpressure resumes: %d\n", snapshot.BackpressureResumes)
}
```

### Multiple Streams

Each stream can have its own backpressure controller, or streams can share a controller:

```go
// Independent backpressure per stream
bp1 := stream.NewBackpressureState(cfg, m)
stream1.SetBackpressure(bp1)

bp2 := stream.NewBackpressureState(cfg, m)
stream2.SetBackpressure(bp2)

// Shared backpressure across streams
sharedBP := stream.NewBackpressureState(cfg, m)
stream1.SetBackpressure(sharedBP)
stream2.SetBackpressure(sharedBP)
```

## Tuning Guidelines

### High Load Scenarios

For high-throughput applications, increase thresholds:

```go
cfg.StreamBufferHighWaterMark = 1048576  // 1MB
cfg.StreamBufferLowWaterMark = 262144    // 256KB
```

### Memory-Constrained Environments

For low-memory systems, decrease thresholds:

```go
cfg.StreamBufferHighWaterMark = 32768   // 32KB
cfg.StreamBufferLowWaterMark = 8192     // 8KB
```

### Hysteresis Ratio

Maintain a 4:1 ratio (high:low) to prevent oscillation:

```go
// Good: 4:1 ratio
cfg.StreamBufferHighWaterMark = 100000
cfg.StreamBufferLowWaterMark = 25000

// Bad: Too close, may oscillate
cfg.StreamBufferHighWaterMark = 100000
cfg.StreamBufferLowWaterMark = 90000
```

## Monitoring

### Metrics

Track backpressure events through metrics:

```go
snapshot := m.Snapshot()

// Number of times backpressure was applied
fmt.Printf("Pauses: %d\n", snapshot.BackpressurePauses)

// Number of times backpressure was released
fmt.Printf("Resumes: %d\n", snapshot.BackpressureResumes)

// Calculate pause/resume ratio
ratio := float64(snapshot.BackpressurePauses) / float64(snapshot.BackpressureResumes)
if ratio > 1.5 {
    // Excessive backpressure - consider tuning thresholds
}
```

### Buffer Status

Monitor buffer sizes in real-time:

```go
// Check current buffer sizes
sendSize := stream.GetSendBufferSize()
recvSize := stream.GetRecvBufferSize()

// Check backpressure state
bp := stream.GetBackpressure()
sendPaused := bp.IsSendPaused()
recvPaused := bp.IsRecvPaused()

fmt.Printf("Send: %d bytes (paused: %v)\n", sendSize, sendPaused)
fmt.Printf("Recv: %d bytes (paused: %v)\n", recvSize, recvPaused)
```

## Performance Impact

- **Disabled**: Zero overhead - no checks performed
- **Enabled (normal)**: Minimal overhead - atomic boolean checks
- **Enabled (paused)**: Small overhead - buffer size tracking and hysteresis logic

Typical overhead: <1% CPU when backpressure is active

## Best Practices

1. **Enable for production**: Always enable backpressure in production environments
2. **Monitor metrics**: Track pause/resume ratios to detect issues
3. **Tune thresholds**: Adjust based on your application's memory profile
4. **Test under load**: Verify backpressure behavior under realistic load
5. **Log events**: Consider logging backpressure events for debugging

## Integration with Flow Control

Stream backpressure works alongside Tor's protocol-level flow control:

- **Protocol flow control** (tor-spec.txt §7.4): SENDME cells for network-level control
- **Stream backpressure**: Memory-level protection for buffer management

Both mechanisms work together to prevent resource exhaustion at different layers.

## Troubleshooting

### Frequent Pauses

**Symptom**: High `BackpressurePauses` count

**Solutions**:
- Increase `StreamBufferHighWaterMark`
- Improve data consumption rate
- Check for slow consumers blocking the pipeline

### Memory Growth

**Symptom**: Memory usage grows despite backpressure

**Solutions**:
- Decrease `StreamBufferHighWaterMark`
- Check for leak in stream lifecycle management
- Verify streams are being properly closed

### Oscillation

**Symptom**: Rapid pause/resume cycles

**Solutions**:
- Increase hysteresis ratio (widen gap between high/low marks)
- Recommended minimum: 4:1 ratio

## API Reference

### BackpressureState

```go
// Create a new backpressure controller
func NewBackpressureState(cfg *config.Config, m *metrics.Metrics) *BackpressureState

// Check and update send buffer state
func (b *BackpressureState) CheckSendBuffer(bufferSize int) bool

// Check and update receive buffer state
func (b *BackpressureState) CheckRecvBuffer(bufferSize int) bool

// Query current state
func (b *BackpressureState) IsSendPaused() bool
func (b *BackpressureState) IsRecvPaused() bool

// Reset backpressure state
func (b *BackpressureState) Reset()

// Get configuration
func (b *BackpressureState) GetHighWaterMark() int
func (b *BackpressureState) GetLowWaterMark() int
```

### Stream Methods

```go
// Attach backpressure controller
func (s *Stream) SetBackpressure(bp *BackpressureState)

// Get backpressure controller
func (s *Stream) GetBackpressure() *BackpressureState

// Get buffer sizes
func (s *Stream) GetSendBufferSize() int
func (s *Stream) GetRecvBufferSize() int
```

## Testing

Comprehensive test coverage (>95%) validates:

- Threshold-based pause/resume behavior
- Hysteresis mechanism
- Independent send/receive control
- Metrics accuracy
- Concurrent operations
- Buffer size tracking
- Edge cases and error conditions

Run tests:

```bash
go test ./pkg/stream -run Backpressure -v
```

## References

- [ROADMAP.md](../ROADMAP.md) - Stream Backpressure feature description
- [pkg/config/config.go](../pkg/config/config.go) - Configuration parameters
- [pkg/metrics/metrics.go](../pkg/metrics/metrics.go) - Metrics definitions
- [pkg/stream/stream.go](../pkg/stream/stream.go) - Stream implementation
- [pkg/stream/backpressure.go](../pkg/stream/backpressure.go) - Backpressure logic

---

**Last Updated**: January 25, 2026

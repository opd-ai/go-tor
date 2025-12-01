# Replay Protection

## Overview

This document describes the replay protection mechanism implemented in go-tor to prevent attackers from capturing and replaying valid cells. Replay attacks can compromise anonymity, leak information, or cause unauthorized actions.

## Why Replay Protection Matters

In the Tor protocol, cells are encrypted and transmitted through circuits. Without replay protection, an attacker who captures encrypted cells could:

1. **Replay cells**: Send captured cells again to cause duplicate actions
2. **Traffic correlation**: Use replay timing to link activities across circuits
3. **Resource exhaustion**: Replay cells to exhaust circuit windows or cause denial of service
4. **Information leakage**: Observe how the system responds to replayed cells

## Implementation

The replay protection is implemented in `pkg/cell/replay.go` and integrated into the circuit layer at `pkg/circuit/circuit.go`.

### Key Components

#### ReplayProtection Structure

```go
type ReplayProtection struct {
    // Forward direction (client → exit)
    forwardSeq     uint64              // Next expected sequence number
    forwardWindow  map[uint64]struct{} // Seen sequence numbers in window
    forwardDigests map[[16]byte]uint64 // Cell digest → sequence number

    // Backward direction (exit → client)
    backwardSeq     uint64
    backwardWindow  map[uint64]struct{}
    backwardDigests map[[16]byte]uint64

    // Configuration
    windowSize uint64 // Sliding window size
}
```

#### Detection Methods

1. **Sequence Number Tracking**: Each cell in a direction is assigned a monotonically increasing sequence number. Replayed cells will have already-seen sequence numbers.

2. **Digest Tracking**: A truncated SHA-256 hash (first 16 bytes) of each cell is tracked. Cells with identical content are rejected even if they have different sequence numbers.

3. **Sliding Window**: A configurable window size (default: 32) allows for slightly out-of-order cells while still detecting replays. Cells too old (before the window) are rejected.

### Validation Flow

When a cell is received:

1. Compute truncated SHA-256 digest of cell data
2. Check if digest was already seen → **REPLAY**
3. Check if sequence number was already seen → **REPLAY**
4. Check if sequence number is too old (before window) → **REPLAY**
5. Check if sequence number is too far ahead → **SUSPICIOUS**
6. Track the cell for future detection
7. Cleanup old entries outside the window

### Circuit Integration

Each circuit maintains its own `ReplayProtection` instance for independent tracking:

```go
type Circuit struct {
    // ... other fields ...
    replayProtection *cell.ReplayProtection // Replay protection for cells
}
```

The `DeliverRelayCell` method validates incoming cells:

```go
func (c *Circuit) DeliverRelayCell(cellData *cell.Cell) error {
    // ... decrypt cell ...

    // SECURITY-001: Validate against replay attacks
    if c.replayProtection != nil {
        seqNum := c.replayProtection.GetNextSequence(cell.ReplayBackward)
        err := c.replayProtection.ValidateAndTrack(cell.ReplayBackward, seqNum, decryptedPayload)
        if err != nil {
            return fmt.Errorf("replay protection: %w", err)
        }
    }

    // ... continue processing ...
}
```

## Metrics

Replay protection events are tracked in the metrics system:

| Metric | Description |
|--------|-------------|
| `ReplayAttemptsDetected` | Total number of detected replay attempts |
| `ReplayForwardAttempts` | Replay attempts in client→exit direction |
| `ReplayBackwardAttempts` | Replay attempts in exit→client direction |
| `OutOfOrderCells` | Cells received out of order (not replays) |

Access replay stats per circuit:

```go
stats := circuit.GetReplayStats()
fmt.Printf("Replay attempts: %d\n", stats.ReplayAttemptsForward + stats.ReplayAttemptsBackward)
fmt.Printf("Out of order: %d\n", stats.OutOfOrderForward + stats.OutOfOrderBackward)
```

## Configuration

The sliding window size can be configured when creating replay protection:

```go
// Default window size (32 cells)
rp := cell.NewReplayProtection()

// Custom window size (64 cells)
rp := cell.NewReplayProtectionWithWindow(64)
```

Larger windows allow more out-of-order tolerance but use more memory.

## Testing

Comprehensive tests are provided in:
- `pkg/cell/replay_test.go` - Core replay protection tests
- `pkg/circuit/circuit_test.go` - Circuit integration tests

Key test scenarios:
- Normal flow (sequential cells)
- Duplicate cell detection
- Duplicate sequence number detection
- Duplicate digest detection
- Out-of-order acceptance (within window)
- Too-old cell rejection
- Too-far-ahead cell rejection
- Bidirectional independence
- Concurrent access safety
- Reset and cleanup

## Performance

Benchmarks show efficient operation:
- `ValidateAndTrack`: ~500ns per cell
- `GetNextSequence`: ~50ns per call
- `Stats`: ~100ns per call

Memory usage scales with window size:
- Per-direction overhead: O(windowSize) for sequence tracking
- Digest tracking: O(windowSize) for content hashing

## Security Considerations

1. **Window Size Trade-offs**: Smaller windows use less memory but may reject legitimate out-of-order cells. Larger windows provide more tolerance but use more memory and allow older replays.

2. **Digest Truncation**: Using 16 bytes (128 bits) of SHA-256 provides strong collision resistance while reducing memory overhead. Birthday attack resistance is ~2^64 operations.

3. **Constant-Time Considerations**: The implementation uses map lookups which may not be strictly constant-time. For maximum security, consider additional timing attack mitigations.

4. **Circuit State Reset**: When a circuit is torn down, replay protection should be reset to prevent state leakage between circuits.

## References

- tor-spec.txt Section 6.1: Relay cell format
- tor-spec.txt Section 7: Flow control and replay prevention
- [Tor Protocol Specification](https://spec.torproject.org/tor-spec)

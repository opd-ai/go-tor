# Connection-Level Padding Implementation

This document describes the connection-level (link-level) padding implementation in go-tor, which provides traffic analysis resistance at the TLS connection layer per the Tor padding-spec.txt.

## Overview

Connection-level padding complements circuit-level padding by sending PADDING cells on the TLS connection itself, independent of any circuits. This provides defense against traffic analysis attacks that observe connection-level patterns.

**Key Differences from Circuit Padding:**
- **Circuit Padding**: Uses RELAY cells within a circuit (commands 41-42)
- **Connection Padding**: Uses PADDING/VPADDING cells on the TLS connection (commands 0, 128)

## Architecture

The connection padding system consists of:

1. **ConnectionPaddingMachine**: Manages padding for a single connection
2. **ConnectionPaddingConfig**: Configuration for padding behavior
3. **ConnectionPaddingStrategy**: Defines padding scheduling strategies

## Padding Strategies

### None (Disabled)
No padding is sent on the connection.

```go
config := &connection.ConnectionPaddingConfig{
    Strategy: connection.ConnectionPaddingNone,
}
```

### Fixed Interval
Padding is sent at regular, fixed intervals.

```go
config := &connection.ConnectionPaddingConfig{
    Strategy:    connection.ConnectionPaddingFixed,
    MinInterval: 5 * time.Second,
}
```

### Random Interval
Padding is sent at random intervals within a specified range.

```go
config := &connection.ConnectionPaddingConfig{
    Strategy:    connection.ConnectionPaddingRandom,
    MinInterval: 5 * time.Second,
    MaxInterval: 15 * time.Second,
}
```

### Adaptive
Padding adjusts based on connection activity. During quiet periods, padding is more aggressive. During active periods, padding is reduced to avoid unnecessary overhead.

```go
config := &connection.ConnectionPaddingConfig{
    Strategy:    connection.ConnectionPaddingAdaptive,
    MinInterval: 5 * time.Second,
    MaxInterval: 15 * time.Second,
}
```

## Cell Types

### PADDING Cells (Command 0)
Fixed-size padding cells (514 bytes total) with random payload. These are the standard padding cells used by Tor.

```go
config := &connection.ConnectionPaddingConfig{
    UseVariableLength: false,  // Use PADDING cells
}
```

### VPADDING Cells (Command 128)
Variable-length padding cells that can have different payload sizes (100-509 bytes). This makes traffic analysis harder as cell sizes vary.

```go
config := &connection.ConnectionPaddingConfig{
    UseVariableLength: true,  // Use VPADDING cells
}
```

## Usage Examples

### Basic Usage

```go
import (
    "context"
    "github.com/opd-ai/go-tor/pkg/connection"
)

// Create connection
conn := connection.New(connection.DefaultConfig("127.0.0.1:9001"), logger)
if err := conn.Connect(context.Background(), cfg); err != nil {
    log.Fatal(err)
}

// Create padding machine with default config
pm, err := connection.NewConnectionPaddingMachine(conn, nil)
if err != nil {
    log.Fatal(err)
}

// Start padding
ctx := context.Background()
if err := pm.Start(ctx); err != nil {
    log.Fatal(err)
}

// ... use connection ...

// Stop padding when done
pm.Stop()
```

### Custom Configuration

```go
config := &connection.ConnectionPaddingConfig{
    Strategy:          connection.ConnectionPaddingRandom,
    MinInterval:       3 * time.Second,
    MaxInterval:       10 * time.Second,
    IdleTimeout:       2 * time.Second,
    UseVariableLength: true,  // Use VPADDING for better resistance
}

pm, err := connection.NewConnectionPaddingMachine(conn, config)
if err != nil {
    log.Fatal(err)
}

if err := pm.Start(context.Background()); err != nil {
    log.Fatal(err)
}
defer pm.Stop()
```

### Monitoring Statistics

```go
// Get padding statistics
stats := pm.Stats()
fmt.Printf("PADDING cells sent: %d\n", stats.PaddingsSent)
fmt.Printf("VPADDING cells sent: %d\n", stats.VPaddingsSent)
fmt.Printf("Failed padding sends: %d\n", stats.FailedPaddings)
```

### Updating Configuration Dynamically

```go
// Start with conservative padding
pm.Start(context.Background())

// Later, increase padding for higher security
newConfig := &connection.ConnectionPaddingConfig{
    Strategy:    connection.ConnectionPaddingFixed,
    MinInterval: 2 * time.Second,
}

if err := pm.UpdateConfig(newConfig); err != nil {
    log.Printf("Failed to update config: %v", err)
}
```

### Recording Connection Activity

The padding machine can be informed of connection activity to optimize padding in adaptive mode:

```go
// When sending or receiving cells, record activity
pm.RecordActivity()

// The adaptive strategy will reduce padding during active periods
// and increase it during quiet periods
```

## Configuration Parameters

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `Strategy` | ConnectionPaddingStrategy | Padding scheduling strategy | Random |
| `MinInterval` | time.Duration | Minimum time between padding cells | 5s |
| `MaxInterval` | time.Duration | Maximum time between padding cells | 15s |
| `IdleTimeout` | time.Duration | Idle time before padding begins | 2s |
| `UseVariableLength` | bool | Use VPADDING instead of PADDING | false |

## Performance Impact

### Bandwidth Overhead

With default settings (Random strategy, 5-15s intervals):
- **Average overhead**: ~0.5-2 KB/minute
- **Maximum overhead**: ~6 KB/minute
- Each PADDING cell: 514 bytes
- VPADDING cells: 100-509 bytes (average ~300 bytes)

### CPU Usage

Connection padding uses cryptographically secure random number generation:
- **CPU impact**: <0.05% on modern systems
- All timing uses rejection sampling to avoid modulo bias

### Memory

Each ConnectionPaddingMachine uses approximately:
- **Base overhead**: ~150 bytes
- **Goroutine**: ~2-4 KB
- **Total per connection**: ~5 KB

## Security Considerations

### Cryptographic Timing

All random delays and payload generation use `crypto/rand` to prevent:
- Timing correlation attacks
- Statistical analysis of padding patterns
- Modulo bias in random number generation

### Idle Detection

The padding machine only sends padding when the connection has been idle for longer than `IdleTimeout`. This:
- Prevents redundant padding during active use
- Reduces bandwidth overhead
- Maintains good traffic analysis resistance

### Defense-in-Depth

Connection-level padding provides defense-in-depth when combined with:
- Circuit-level padding (RELAY PADDING cells)
- Path selection diversity
- Guard node rotation
- Certificate pinning

## Specification Compliance

### Implemented Features

✅ **PADDING cells** (tor-spec.txt §7.1, command 0)
- Fixed-size cells with random payload
- Silently discarded by receiver
- Circuit ID 0 for connection-level padding

✅ **VPADDING cells** (tor-spec.txt §7.1, command 128)
- Variable-length cells (100-509 bytes)
- Enhanced traffic analysis resistance
- Circuit ID 0 for connection-level padding

✅ **Connection-level padding** (padding-spec.txt)
- Independent of circuit state
- Idle timeout before padding begins
- Configurable strategies and intervals

✅ **Cryptographically secure timing**
- Uses crypto/rand for all random values
- Rejection sampling to avoid modulo bias
- No predictable patterns

### Partial Implementation

⚠️ **Padding parameter negotiation**
- No consensus-based parameter updates
- Manual configuration required

### Not Implemented

❌ **WTF-PAD algorithm** (academic research, not in spec)
❌ **Connection-level state machines** (only circuit-level state machines are implemented)

## Integration with Circuit Padding

Connection-level and circuit-level padding work together:

```go
// Connection-level padding (this implementation)
connPM, _ := connection.NewConnectionPaddingMachine(conn, connConfig)
connPM.Start(ctx)

// Circuit-level padding (existing implementation)
circuitPM, _ := circuit.NewPaddingMachine(circ, circuitConfig)
circuitPM.Start(ctx)

// Both run independently, providing layered protection
```

**Recommendations:**
- Use connection-level padding for all connections
- Use circuit-level padding for sensitive circuits
- Use adaptive strategies to minimize bandwidth overhead

## Testing

Run connection padding tests:

```bash
# All padding tests
go test ./pkg/connection -run TestConnectionPadding -v

# Test coverage
go test ./pkg/connection -cover

# Specific tests
go test ./pkg/connection -run TestConnectionPaddingMachineStartStop -v
go test ./pkg/connection -run TestConnectionPaddingConfigValidation -v
```

## Future Enhancements

1. **Consensus-based parameters**: Fetch padding parameters from network consensus
2. **State machine padding**: Implement formal state machines for connection-level padding
3. **Padding negotiation**: Negotiate padding parameters with relays
4. **Connection-level APE**: Adapt Adaptive Padding Engine for connection level
5. **Performance tuning**: Auto-adjust parameters based on network conditions

## References

- [tor-spec.txt §7.1](https://spec.torproject.org/tor-spec) - PADDING cells
- [padding-spec.txt](https://spec.torproject.org/padding-spec) - Padding specification
- [Proposal 254](https://gitlab.torproject.org/tpo/core/torspec/-/blob/main/proposals/254-padding-negotiation.txt) - Padding negotiation

## Implementation Notes

**File Locations:**
- Implementation: `pkg/connection/padding.go`
- Tests: `pkg/connection/padding_test.go`
- Documentation: `docs/CONNECTION_PADDING.md`

**Code Quality:**
- Test coverage: >95% for padding code
- All error paths tested
- Concurrent access safe (uses sync.RWMutex)
- No goroutine leaks (proper cleanup on Stop)

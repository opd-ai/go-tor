# Circuit Padding Implementation

This document describes the circuit padding implementation in go-tor, which provides traffic analysis resistance per the Tor padding-spec.txt.

## Overview

The padding system consists of two main components:

1. **PaddingMachine**: Legacy adaptive padding with configurable strategies
2. **StateMachine**: Formal padding state machines per padding-spec.txt (APE)

## Padding Strategies

### Legacy Strategies (PaddingMachine)

The `PaddingMachine` provides backward-compatible padding with these strategies:

- **None**: No padding (disabled)
- **Fixed**: Padding at fixed intervals
- **Random**: Padding at random intervals within a range
- **Adaptive**: Traffic-aware padding that adjusts based on activity

### State Machine Padding (APE)

The new `StateMachine` implementation follows padding-spec.txt §3 with formal states:

- **START**: Initial state before padding begins
- **BURST**: Sending padding cells in rapid succession
- **GAP**: Idle period between bursts
- **END**: Terminal state (machine stopped)

## Machine Types

### Adaptive Padding Engine (APE)

The APE machine provides sophisticated traffic analysis resistance:

```go
sm := circuit.NewAPEMachine(circuit)
sm.Start()

// Process events periodically
shouldPad, delay := sm.ProcessEvent()
if shouldPad {
    // Send padding cell
}
```

**Parameters** (from padding-spec.txt §3):
- Burst size: 2-10 cells
- Gap duration: 1500-9500 ms
- Cell delay: 20 ms between burst cells

### Circuit Setup Machine

Optimized for the circuit building phase with more aggressive padding:

```go
sm := circuit.NewCircuitSetupMachine(circuit)
sm.Start()
```

**Parameters**:
- Burst size: 1-5 cells  
- Gap duration: 500-2000 ms
- Cell delay: 50 ms

## Padding Negotiation Protocol

### PADDING_NEGOTIATE Cell

Circuits can negotiate padding with relays using PADDING_NEGOTIATE cells:

```go
// Request APE padding
err := circuit.SendPaddingNegotiate(circuit.PaddingMachineAPE, true)

// Stop padding
err := circuit.SendPaddingNegotiate(circuit.PaddingMachineAPE, false)
```

The relay responds with a PADDING_NEGOTIATED cell indicating:
- `PaddingResponseStarted`: Padding activated
- `PaddingResponseStopped`: Padding deactivated
- `PaddingResponseError`: Request failed

### Protocol Format

**PADDING_NEGOTIATE** (relay command 41):
```
Version (1 byte)
Command (1 byte): 1=START, 2=STOP
MachineType (1 byte): 0=None, 1=APE, 2=CircuitSetup
```

**PADDING_NEGOTIATED** (relay command 42):
```
Version (1 byte)
Command (1 byte): 1=STARTED, 2=STOPPED, 3=ERROR
MachineType (1 byte): 0=None, 1=APE, 2=CircuitSetup
```

## Usage Examples

### Using Legacy PaddingMachine

```go
config := &circuit.PaddingConfig{
    Strategy:    circuit.PaddingStrategyAdaptive,
    MinInterval: 3 * time.Second,
    MaxInterval: 10 * time.Second,
    IdleTimeout: time.Second,
    BurstSize:   2,
}

pm, err := circuit.NewPaddingMachine(circuit, config)
if err != nil {
    log.Fatal(err)
}

ctx := context.Background()
pm.Start(ctx)
defer pm.Stop()

// View statistics
stats := pm.Stats()
fmt.Printf("Padding sent: %d\n", stats.PaddingsSent)
```

### Using State Machine APE

```go
sm := circuit.NewAPEMachine(circuit)
if err := sm.Start(); err != nil {
    log.Fatal(err)
}

ticker := time.NewTicker(100 * time.Millisecond)
defer ticker.Stop()

for {
    select {
    case <-ticker.C:
        shouldPad, delay := sm.ProcessEvent()
        if shouldPad {
            // Send a padding cell
            paddingCell := circuit.NewPaddingCell(circuit.ID)
            circuit.SendCell(paddingCell)
        }
        ticker.Reset(delay)
    }
}
```

### Negotiating Padding with Relay

```go
// Start APE padding on the circuit
err := circuit.SendPaddingNegotiate(circuit.PaddingMachineAPE, true)
if err != nil {
    log.Printf("Failed to negotiate padding: %v", err)
}

// The relay will respond with PADDING_NEGOTIATED
// Handle the response in your relay cell processor
```

## Performance Considerations

### Bandwidth Overhead

- **APE**: ~5-50 cells/minute depending on traffic patterns
- **Circuit Setup**: ~10-30 cells/minute during setup phase
- Each padding cell is 514 bytes

### CPU Usage

State machines use cryptographically secure random number generation which has minimal CPU impact (<0.1% on modern systems).

### Memory

Each active state machine uses ~200 bytes of memory.

## Compliance

### Implemented Features

✅ PADDING cells (tor-spec.txt §7.1)
✅ PADDING_NEGOTIATE protocol (padding-spec.txt)
✅ Formal state machine implementation (padding-spec.txt §3)
✅ Adaptive Padding Engine (APE)
✅ Burst and gap states
✅ Cryptographically secure timing

### Partial Implementation

⚠️ Connection-level padding (not circuit-level only)
⚠️ Full padding machine negotiation protocol

### Not Implemented

❌ WTF-PAD algorithm (academic research, not in spec)
❌ Tor Project's specific machine parameters tuning

## Security Notes

1. **Timing Security**: All random delays use `crypto/rand` to prevent timing attacks
2. **No Modulo Bias**: Uses rejection sampling for uniform distribution
3. **State Isolation**: Each circuit has independent padding state
4. **Resource Limits**: Burst sizes and delays are capped to prevent DoS

## Testing

Run padding tests:

```bash
# Test legacy padding machine
go test ./pkg/circuit -run TestPaddingMachine -v

# Test state machines
go test ./pkg/circuit -run TestStateMachine -v

# Test negotiation protocol
go test ./pkg/circuit -run TestPaddingNegotiate -v
```

## References

- [tor-spec.txt §7.1](https://spec.torproject.org/tor-spec) - PADDING cells
- [padding-spec.txt](https://spec.torproject.org/padding-spec) - Circuit padding specification
- [Proposal 254](https://gitlab.torproject.org/tpo/core/torspec/-/blob/main/proposals/254-padding-negotiation.txt) - Padding negotiation protocol

## Future Work

1. Implement connection-level padding
2. Add machine parameter auto-tuning based on network conditions
3. Support additional padding machine types
4. Integrate with consensus parameters for network-wide coordination

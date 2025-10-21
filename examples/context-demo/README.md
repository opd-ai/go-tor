# Context-Aware Operations Demo

This example demonstrates the new context-aware operations for improved timeout and cancellation control in the go-tor library.

## Features Demonstrated

### 1. Circuit Context Operations
- Creating circuits with context support
- Waiting for circuit state changes with timeout
- Checking circuit age
- Closing circuits with context

### 2. Stream Context Operations
- Waiting for stream connection with timeout
- Sending data with timeout
- Checking stream status (active/closed)
- Closing streams gracefully with context

### 3. Manager Context Operations
- Waiting for specific circuit counts
- Getting circuits by state
- Counting circuits by state
- Closing manager with deadline

### 4. Cancellation Handling
- Graceful cancellation of long-running operations
- Timeout handling
- Context error handling

## Running the Demo

```bash
cd examples/context-demo
go run main.go
```

## Expected Output

```
=== Context-Aware Operations Demo ===

--- Demo 1: Circuit State Management with Context ---
✓ Circuit created with ID: 1
  Waiting for circuit to become ready...
  Circuit state changed to OPEN
✓ Circuit is ready!
✓ Circuit is older than 50ms (age: ~100ms)
✓ Circuit closed gracefully

--- Demo 2: Stream Operations with Context ---
✓ Stream created (ID: 1, Circuit: 100)
  Waiting for stream to connect...
  Stream state changed to CONNECTED
✓ Stream connected!
✓ Sent 12 bytes with timeout
✓ Stream is active
✓ Stream closed gracefully
✓ Stream is closed

--- Demo 3: Manager Operations with Context ---
Creating circuits...
  Created circuit 2
  Created circuit 3
  Created circuit 4
  Waiting for at least 2 circuits to be ready...
✓ Required circuits are ready!
✓ Found 3 open circuits
✓ Circuit count by state (OPEN): 3
✓ Manager closed gracefully

--- Demo 4: Graceful Cancellation ---
Starting long-running operation...
  Cancelling operation...
✓ Operation cancelled gracefully
  Waiting for circuit (will timeout)...
✓ Timeout handled gracefully

=== Demo Complete ===
```

## Key Concepts

### Context Propagation
Context is propagated through operations to enable:
- Timeout control
- Cancellation propagation
- Deadline enforcement

### Timeout Management
Operations support timeouts at multiple levels:
- Circuit creation and state changes
- Stream connection and data transfer
- Manager operations

### Graceful Cancellation
When a context is cancelled:
- Operations stop cleanly
- Resources are properly released
- Error is properly reported

## API Reference

### Circuit Operations
```go
// Wait for circuit to be ready
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
err := circuit.WaitUntilReady(ctx)

// Create circuit with context
circuit, err := manager.CreateCircuitWithContext(ctx)

// Close circuit with context
err = manager.CloseCircuitWithContext(ctx, circuitID)
```

### Stream Operations
```go
// Send with timeout
err := stream.SendWithTimeout(5*time.Second, data)

// Wait for state
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
err := stream.WaitForState(ctx, StateConnected)

// Close with context
err = stream.CloseWithContext(ctx)
```

### Manager Operations
```go
// Wait for circuit count
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
err := manager.WaitForCircuitCount(ctx, StateOpen, 3)

// Get circuits by state
circuits := manager.GetCircuitsByState(StateOpen)

// Count by state
count := manager.CountByState(StateOpen)
```

## Benefits

1. **Better Resource Control**: Timeout enforcement prevents operations from hanging indefinitely
2. **Graceful Shutdown**: Context cancellation enables clean shutdown of long-running operations
3. **Production Ready**: Following Go best practices for context handling
4. **Backward Compatible**: Existing APIs remain unchanged

## Related Documentation

- [docs/API.md](../../docs/API.md) - Complete API reference
- [docs/TUTORIAL.md](../../docs/TUTORIAL.md) - Getting started guide
- [pkg/stream/stream_context.go](../../pkg/stream/stream_context.go) - Stream context implementation
- [pkg/circuit/circuit_context.go](../../pkg/circuit/circuit_context.go) - Circuit context implementation

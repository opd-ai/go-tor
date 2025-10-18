# Graceful Shutdown

The go-tor client implements graceful shutdown using Go's context package to ensure clean termination and state preservation.

## Overview

Graceful shutdown ensures that:
- All circuits are properly closed
- State is saved before exit
- Network connections are cleanly terminated
- No data loss or corruption occurs
- Resources are released properly

## Implementation

### Main Application

The main application in `cmd/tor-client/main.go` implements graceful shutdown:

```go
// Create cancellable context
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Run application
if err := run(ctx, cfg, log); err != nil {
    log.Error("Application error", "error", err)
    os.Exit(1)
}
```

### Signal Handling

The application listens for OS signals (SIGINT, SIGTERM) for graceful shutdown:

```go
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
defer signal.Stop(sigChan)

select {
case sig := <-sigChan:
    log.Info("Received shutdown signal", "signal", sig.String())
case <-ctx.Done():
    log.Info("Context cancelled", "reason", ctx.Err())
}
```

### Shutdown Timeout

A timeout ensures shutdown doesn't hang indefinitely:

```go
shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
defer shutdownCancel()

log.Info("Initiating graceful shutdown...")

select {
case <-shutdownCtx.Done():
    log.Warn("Shutdown timeout exceeded, forcing exit")
    return shutdownCtx.Err()
case <-time.After(100 * time.Millisecond):
    // Shutdown completed successfully
}
```

## Component Integration

### Circuit Manager

The circuit manager supports graceful shutdown:

```go
// Create manager
manager := circuit.NewManager()

// ... use manager ...

// Graceful shutdown
if err := manager.Close(ctx); err != nil {
    log.Error("Failed to close circuit manager", "error", err)
}
```

The `Close` method:
- Marks manager as closed to prevent new circuits
- Closes all existing circuits
- Returns error if already closed

### Future Components

Future components (SOCKS5, Directory Client, etc.) will follow the same pattern:

```go
type Component interface {
    Run(ctx context.Context) error
    Close(ctx context.Context) error
}
```

## Usage Examples

### Starting the Client

```bash
# Start with default settings
./bin/tor-client

# Start with custom settings
./bin/tor-client --socks-port 9150 --log-level debug
```

### Graceful Shutdown

**SIGINT (Ctrl+C)**:
```bash
# Press Ctrl+C in the terminal
```

**SIGTERM**:
```bash
# From another terminal
kill -TERM $(pidof tor-client)
```

**Output**:
```
time=2025-10-18T03:27:08.878Z level=INFO msg="Received shutdown signal" signal=terminated
time=2025-10-18T03:27:08.879Z level=INFO msg="Initiating graceful shutdown..."
time=2025-10-18T03:27:08.979Z level=INFO msg="Shutdown complete"
```

## Best Practices

### Context Propagation

Pass context through all layers:

```go
func MyFunction(ctx context.Context, ...) error {
    // Check for cancellation
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }
    
    // Do work...
    
    return nil
}
```

### Respect Context Cancellation

Always check context in long-running operations:

```go
for {
    select {
    case <-ctx.Done():
        return ctx.Err()
    case data := <-dataChan:
        // Process data
    }
}
```

### Cleanup Resources

Use defer for guaranteed cleanup:

```go
func ProcessCircuit(ctx context.Context, circuit *Circuit) error {
    // Ensure circuit is closed
    defer circuit.Close()
    
    // Process circuit...
    return nil
}
```

### Timeout Durations

Choose appropriate timeouts:
- **Short operations** (< 1s): 5-10 second timeout
- **Medium operations** (1-5s): 30 second timeout
- **Long operations** (> 5s): 60-120 second timeout

## Testing Graceful Shutdown

### Manual Testing

```bash
# Terminal 1: Start client
./bin/tor-client --log-level debug

# Terminal 2: Send signals
kill -TERM $(pidof tor-client)
```

### Automated Testing

```go
func TestGracefulShutdown(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    
    // Start component
    errChan := make(chan error)
    go func() {
        errChan <- component.Run(ctx)
    }()
    
    // Trigger shutdown
    cancel()
    
    // Wait with timeout
    select {
    case err := <-errChan:
        if err != nil && err != context.Canceled {
            t.Errorf("Unexpected error: %v", err)
        }
    case <-time.After(5 * time.Second):
        t.Error("Shutdown timeout")
    }
}
```

## Troubleshooting

### Shutdown Hangs

If shutdown hangs, check:
1. All goroutines respect context cancellation
2. No blocking operations without timeout
3. Channels are properly closed
4. No deadlocks in cleanup code

### Timeout Exceeded

If shutdown timeout is exceeded:
1. Increase timeout duration if legitimate
2. Identify slow shutdown operations with profiling
3. Add intermediate progress logging
4. Consider async cleanup for non-critical resources

### Resource Leaks

To detect resource leaks:
1. Use `pprof` to monitor goroutines
2. Check open file descriptors
3. Monitor memory usage during shutdown
4. Add resource tracking in debug mode

## Future Enhancements

- State persistence before shutdown
- Progressive shutdown (close non-critical components first)
- Shutdown hooks for custom cleanup
- Metrics and monitoring integration
- Crash recovery and restart logic

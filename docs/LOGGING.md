# Structured Logging

The `pkg/logger` package provides structured logging capabilities for the go-tor project using Go's standard `log/slog` package.

## Features

- **Structured Logging**: Key-value pairs for easy parsing and filtering
- **Context Propagation**: Logger can be attached to and retrieved from context
- **Component Tagging**: Built-in helpers for tagging logs by component, circuit, stream
- **Standard Log Levels**: Debug, Info, Warn, Error
- **Zero Dependencies**: Uses only Go standard library

## Usage

### Basic Usage

```go
import "github.com/opd-ai/go-tor/pkg/logger"

// Create a logger
log := logger.NewDefault()

// Log with structured attributes
log.Info("Starting service", "port", 9050, "protocol", "socks5")
log.Error("Connection failed", "error", err, "retries", 3)
```

### With Context

```go
// Attach logger to context
ctx := logger.WithContext(context.Background(), log)

// Retrieve logger from context
log := logger.FromContext(ctx)
log.Info("Processing request")
```

### Component Logging

```go
// Create component-specific logger
circuitLog := log.Component("circuit")
circuitLog.Info("Building circuit", "hops", 3)

// Add circuit/stream context
log.Circuit(12345).Info("Circuit established")
log.Stream(42).Debug("Stream data received", "bytes", 1024)
```

### Log Levels

```go
// Parse log level from string
level, err := logger.ParseLevel("debug")
log := logger.New(level, os.Stdout)

log.Debug("Detailed information")
log.Info("General information")
log.Warn("Warning message")
log.Error("Error occurred")
```

### Grouped Attributes

```go
// Create logger with grouped attributes
networkLog := log.WithGroup("network")
networkLog.Info("Bytes transferred", "sent", 1024, "received", 2048)
// Outputs: network.sent=1024 network.received=2048
```

## Integration with Main Application

The main application in `cmd/tor-client/main.go` demonstrates integration:

```go
// Initialize logger from configuration
level, _ := logger.ParseLevel(cfg.LogLevel)
log := logger.New(level, os.Stdout)

// Attach to context
ctx := logger.WithContext(context.Background(), log)

// Use throughout application
log.Info("Starting go-tor", "version", version)
```

## Best Practices

1. **Pass loggers through context**: Use `WithContext` and `FromContext` for clean propagation
2. **Use structured attributes**: Prefer key-value pairs over formatted strings
3. **Component tagging**: Use `Component()` to identify log sources
4. **Appropriate levels**: 
   - Debug: Detailed diagnostic information
   - Info: General operational information
   - Warn: Warning conditions that don't prevent operation
   - Error: Error conditions that prevent specific operations

## Testing

The logger package includes comprehensive tests in `pkg/logger/logger_test.go`:

```bash
go test ./pkg/logger/...
```

## Performance

- Uses Go's standard `log/slog` for optimal performance
- Minimal allocation overhead
- Efficient text formatting
- Thread-safe operations

## Future Enhancements

- JSON output format option
- Log rotation support
- Remote logging backends (syslog, etc.)
- Per-component log level configuration

# Configuration Hot Reload

## Overview

Configuration hot reload allows you to modify certain configuration parameters without restarting the go-tor client. This feature is essential for production deployments where downtime must be minimized.

## Quick Start

```go
import (
    "context"
    "time"
    "github.com/opd-ai/go-tor/pkg/config"
)

// Create a reloadable configuration
cfg := config.DefaultConfig()
reloadableConfig := config.NewReloadableConfig(cfg, "/etc/tor/torrc", logger)

// Register callback to be notified of configuration changes
reloadableConfig.OnReload(func(oldConfig, newConfig *config.Config) error {
    log.Printf("Configuration changed: LogLevel %s -> %s", 
        oldConfig.LogLevel, newConfig.LogLevel)
    return nil
})

// Start watching for file changes (checks every 30 seconds)
ctx := context.Background()
go reloadableConfig.StartWatcher(ctx, 30*time.Second)

// Access current configuration
currentConfig := reloadableConfig.Get()
```

## Reloadable Fields

The following configuration fields can be changed without restarting:

### Logging
- **LogLevel**: debug, info, warn, error

### Circuit Configuration
- **MaxCircuitDirtiness**: Maximum time to use a circuit
- **NewCircuitPeriod**: How often to rotate circuits
- **CircuitBuildTimeout**: Maximum time to build a circuit
- **CircuitPoolMinSize**: Minimum circuits to prebuild
- **CircuitPoolMaxSize**: Maximum circuits in pool
- **EnableCircuitPrebuilding**: Enable/disable circuit prebuilding

### Connection Pooling
- **EnableConnectionPooling**: Enable/disable connection pooling
- **ConnectionPoolMaxIdle**: Maximum idle connections per relay
- **ConnectionPoolMaxLife**: Maximum lifetime for pooled connections
- **EnableBufferPooling**: Enable/disable buffer pooling

### Circuit Isolation
- **IsolateDestinations**: Isolate circuits by destination
- **IsolateSOCKSAuth**: Isolate circuits by SOCKS5 username
- **IsolateClientPort**: Isolate circuits by client port
- **IsolateClientProtocol**: Isolate circuits by protocol

## Non-Reloadable Fields

These fields require a service restart to take effect:

- **SocksPort**: SOCKS5 proxy port
- **ControlPort**: Control protocol port
- **MetricsPort**: HTTP metrics port
- **DataDirectory**: Data directory location
- **UseEntryGuards**: Whether to use entry guards
- **UseBridges**: Whether to use bridges
- **BridgeAddresses**: Bridge addresses
- **OnionServices**: Onion service configurations

## Manual Reload

You can manually trigger a configuration reload:

```go
if err := reloadableConfig.Reload(); err != nil {
    log.Fatalf("Failed to reload configuration: %v", err)
}
```

## Automatic File Watching

The watcher periodically checks for file modifications:

```go
// Watch for changes every 30 seconds
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go reloadableConfig.StartWatcher(ctx, 30*time.Second)

// Stop the watcher gracefully
reloadableConfig.Stop()
```

## Reload Callbacks

Callbacks allow you to:
- Validate configuration changes before they're applied
- Update components that depend on configuration
- Reject invalid configuration changes

```go
reloadableConfig.OnReload(func(old, new *config.Config) error {
    // Validate the change
    if new.CircuitPoolMaxSize < new.CircuitPoolMinSize {
        return fmt.Errorf("CircuitPoolMaxSize must be >= CircuitPoolMinSize")
    }
    
    // Update dependent components
    circuitPool.UpdatePoolSize(new.CircuitPoolMinSize, new.CircuitPoolMaxSize)
    
    return nil
})
```

If a callback returns an error, the reload is rejected and the old configuration is preserved.

## Error Handling

Configuration reload can fail for several reasons:

### Invalid Configuration
```go
if err := reloadableConfig.Reload(); err != nil {
    // Configuration file contains invalid values
    log.Printf("Invalid configuration: %v", err)
    // Original configuration is preserved
}
```

### Callback Rejection
```go
reloadableConfig.OnReload(func(old, new *config.Config) error {
    if new.CircuitBuildTimeout < 10*time.Second {
        return fmt.Errorf("CircuitBuildTimeout too low")
    }
    return nil
})

// Reload will fail if callback returns error
if err := reloadableConfig.Reload(); err != nil {
    log.Printf("Configuration rejected by callback: %v", err)
}
```

### File System Errors
- File not found: Warning logged, no reload
- Permission denied: Error logged, no reload
- File disappeared: Warning logged, no reload

## Best Practices

### 1. Use Reasonable Check Intervals
```go
// Good: 30-60 seconds for production
reloadableConfig.StartWatcher(ctx, 30*time.Second)

// Too frequent: wastes CPU
reloadableConfig.StartWatcher(ctx, 1*time.Second)

// Too infrequent: delays config changes
reloadableConfig.StartWatcher(ctx, 10*time.Minute)
```

### 2. Validate in Callbacks
```go
reloadableConfig.OnReload(func(old, new *config.Config) error {
    // Validate business logic constraints
    if new.CircuitPoolMaxSize > 100 {
        return fmt.Errorf("CircuitPoolMaxSize too high for this deployment")
    }
    return nil
})
```

### 3. Use Context for Clean Shutdown
```go
ctx, cancel := context.WithCancel(context.Background())

go reloadableConfig.StartWatcher(ctx, 30*time.Second)

// On shutdown
cancel() // Stops watcher gracefully
```

### 4. Log Configuration Changes
The reload system automatically logs:
- When files are detected as changed
- Which fields were modified
- Success or failure of reload operations

Example log output:
```
INFO Configuration file changed, reloading path=/etc/tor/torrc
INFO Configuration fields updated changes="[LogLevel: info -> debug MaxCircuitDirtiness: 10m0s -> 15m0s]" count=2
INFO Configuration reloaded successfully path=/etc/tor/torrc
```

## Example: Production Setup

```go
package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "github.com/opd-ai/go-tor/pkg/config"
    "github.com/opd-ai/go-tor/pkg/client"
)

func main() {
    // Set up logging
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    
    // Load initial configuration
    cfg := config.DefaultConfig()
    if err := config.LoadFromFile("/etc/tor/torrc", cfg); err != nil {
        logger.Error("Failed to load config", "error", err)
        os.Exit(1)
    }
    
    // Create reloadable wrapper
    reloadableConfig := config.NewReloadableConfig(cfg, "/etc/tor/torrc", logger)
    
    // Register reload callback to update logger level
    reloadableConfig.OnReload(func(old, new *config.Config) error {
        if old.LogLevel != new.LogLevel {
            // Update logger level dynamically
            updateLogLevel(logger, new.LogLevel)
        }
        return nil
    })
    
    // Start configuration watcher
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    go reloadableConfig.StartWatcher(ctx, 30*time.Second)
    
    // Start Tor client with current config
    torClient, err := client.New(reloadableConfig.Get(), logger)
    if err != nil {
        logger.Error("Failed to create client", "error", err)
        os.Exit(1)
    }
    defer torClient.Close()
    
    if err := torClient.Start(ctx); err != nil {
        logger.Error("Failed to start client", "error", err)
        os.Exit(1)
    }
    
    // Wait for shutdown signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan
    
    logger.Info("Shutting down gracefully")
}

func updateLogLevel(logger *slog.Logger, level string) {
    var logLevel slog.Level
    switch level {
    case "debug":
        logLevel = slog.LevelDebug
    case "info":
        logLevel = slog.LevelInfo
    case "warn":
        logLevel = slog.LevelWarn
    case "error":
        logLevel = slog.LevelError
    }
    // Update handler with new level
    // Implementation depends on your logging setup
}
```

## Limitations

1. **Not all fields are reloadable** - See list above for what can/cannot be changed
2. **Callback errors prevent reload** - All callbacks must succeed for reload to complete
3. **File system dependent** - Reload only works when configuration is loaded from a file
4. **Polling-based** - Uses periodic checks rather than inotify/fsnotify for maximum portability
5. **No atomic rollback** - If service restart is needed for non-reloadable fields, plan for downtime

## Troubleshooting

### Configuration Not Reloading

Check:
1. Is the config file path correct?
2. Is the watcher started? `StartWatcher()` must be called
3. Is the check interval too long? Try shorter interval for testing
4. Are you modifying reloadable fields? Check the reloadable fields list

### Reload Fails with Validation Error

Check:
1. Configuration file syntax is valid
2. All required fields are present
3. Values are within valid ranges
4. Callbacks are not rejecting the change

### Changes Not Taking Effect

Some fields require component restart even if reloadable:
- Circuit pool changes require existing circuits to expire
- Connection pool changes require new connections
- Isolation changes only affect new circuits

## Security Considerations

1. **File permissions**: Ensure config file is not world-writable
2. **Path validation**: Config loader validates paths to prevent traversal attacks
3. **Validation**: All reloaded configs go through full validation
4. **Rollback**: Failed reloads preserve the original configuration
5. **Audit logging**: All reload attempts are logged for security audit

## Performance Impact

- **Minimal overhead**: Checks are infrequent (30s default)
- **No blocking**: File checks happen in background goroutine
- **Read-lock overhead**: Config reads take a read lock (negligible)
- **Write-lock overhead**: Reloads take a write lock briefly

Recommended for all production deployments.

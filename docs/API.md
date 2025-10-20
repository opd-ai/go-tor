# go-tor API Reference

This document provides comprehensive API documentation for the go-tor library. It covers all major packages and their public interfaces with examples.

## Table of Contents

- [Client API](#client-api)
- [Circuit Management](#circuit-management)
- [SOCKS5 Proxy](#socks5-proxy)
- [Configuration](#configuration)
- [Control Protocol](#control-protocol)
- [Metrics & Observability](#metrics--observability)
- [Error Handling](#error-handling)
- [Resource Pooling](#resource-pooling)

---

## Client API

The client package provides the high-level orchestration for the Tor client.

### Creating a Client

```go
import (
    "log/slog"
    "os"
    
    "github.com/opd-ai/go-tor/pkg/client"
    "github.com/opd-ai/go-tor/pkg/config"
    "github.com/opd-ai/go-tor/pkg/logger"
)

// Create configuration
cfg := config.DefaultConfig()
cfg.SocksPort = 9050
cfg.ControlPort = 9051
cfg.DataDirectory = "/var/lib/tor"

// Create logger
log := logger.New(slog.LevelInfo, os.Stdout)
// Or use the default logger
// log := logger.NewDefault()

// Create client
torClient, err := client.New(cfg, log)
if err != nil {
    log.Fatal("Failed to create client", "error", err)
}
```

### Starting and Stopping

```go
// Start the client
ctx := context.Background()
if err := torClient.Start(ctx); err != nil {
    log.Fatal("Failed to start client", "error", err)
}

// Get client statistics
stats := torClient.GetStats()
fmt.Printf("Active circuits: %d\n", stats.ActiveCircuits)
fmt.Printf("SOCKS port: %d\n", stats.SocksPort)

// Graceful shutdown
if err := torClient.Stop(); err != nil {
    log.Warn("Error during shutdown", "error", err)
}
```

### Client Statistics

```go
type Stats struct {
    ActiveCircuits int
    SocksPort      int
    ControlPort    int
    Uptime         time.Duration
}

stats := torClient.GetStats()
```

---

## Circuit Management

The circuit package handles circuit creation, extension, and lifecycle management.

### Circuit States

```go
const (
    StateBuilding   State = iota  // Circuit is being built
    StateOpen                      // Circuit is ready for use
    StateClosed                    // Circuit has been closed
    StateFailed                    // Circuit build failed
)
```

### Creating Circuits

```go
import "github.com/opd-ai/go-tor/pkg/circuit"

// Create a circuit manager
manager := circuit.NewManager()

// Circuit building happens automatically through the client
// or via the circuit pool for better performance
```

### Circuit Information

```go
type Circuit struct {
    ID        uint32
    CreatedAt time.Time
    // ... (internal fields)
}

// Get circuit state
state := circuit.GetState()

// Set circuit state (internal use)
circuit.SetState(circuit.StateOpen)
```

---

## SOCKS5 Proxy

The socks package implements a RFC 1928 compliant SOCKS5 proxy server with .onion support.

### Creating a SOCKS Server

```go
import "github.com/opd-ai/go-tor/pkg/socks"

// Create SOCKS5 server
addr := "127.0.0.1:9050"
socksServer := socks.NewServer(addr, circuitMgr, log)

// Start serving
ctx := context.Background()
if err := socksServer.Start(ctx); err != nil {
    log.Fatal("Failed to start SOCKS server", "error", err)
}

// Stop serving
if err := socksServer.Stop(); err != nil {
    log.Warn("Error stopping SOCKS server", "error", err)
}
```

### SOCKS5 Usage

Configure your application to use the SOCKS5 proxy:

```bash
# Using curl
curl --socks5 127.0.0.1:9050 https://check.torproject.org

# Using Firefox
# Settings → Network Settings → Manual proxy configuration
# SOCKS Host: 127.0.0.1  Port: 9050  SOCKS v5
```

### Onion Services

The SOCKS5 server automatically handles .onion addresses:

```go
// Connect to onion service (v3)
// http://example.onion will be automatically routed through Tor
```

---

## Configuration

The config package manages application configuration with torrc compatibility.

### Default Configuration

```go
import "github.com/opd-ai/go-tor/pkg/config"

cfg := config.DefaultConfig()
// Returns configuration with sensible defaults:
// - SocksPort: 9050
// - ControlPort: 9051
// - DataDirectory: /var/lib/tor
// - LogLevel: info
```

### Loading from File

```go
// Create base config
cfg := config.DefaultConfig()

// Load torrc-compatible file
if err := config.LoadFromFile("/etc/tor/torrc", cfg); err != nil {
    log.Fatal("Failed to load config", "error", err)
}

// Command-line flags override file settings
cfg.SocksPort = 9150  // Override
```

### Configuration Options

```go
type Config struct {
    SocksPort         int
    ControlPort       int
    DataDirectory     string
    LogLevel          string
    
    // Circuit options
    MaxCircuitDirtiness time.Duration
    CircuitBuildTimeout time.Duration
    
    // Performance tuning
    PrebuiltCircuits    int
    MaxIdleCircuits     int
    ConnectionPoolSize  int
}
```

### Validation

```go
if err := cfg.Validate(); err != nil {
    log.Fatal("Invalid configuration", "error", err)
}
```

---

## Control Protocol

The control package implements a subset of the Tor control protocol for monitoring and management.

### Creating a Control Server

```go
import "github.com/opd-ai/go-tor/pkg/control"

// Create control server
addr := "127.0.0.1:9051"
server := control.NewServer(addr, statsProvider, log)

// Start serving
ctx := context.Background()
if err := server.Start(ctx); err != nil {
    log.Fatal("Failed to start control server", "error", err)
}
```

### Control Commands

Supported commands:
- `GETINFO` - Get information about the Tor client
- `SETEVENTS` - Subscribe to events
- `SIGNAL` - Send signals (SHUTDOWN, RELOAD, etc.)

### Event Types

```go
const (
    EventCirc    EventType = "CIRC"     // Circuit status changes
    EventStream  EventType = "STREAM"   // Stream status changes
    EventBW      EventType = "BW"       // Bandwidth usage
    EventORConn  EventType = "ORCONN"   // OR connection status
    EventNewDesc EventType = "NEWDESC"  // New descriptor available
    EventGuard   EventType = "GUARD"    // Guard node changes
    EventNS      EventType = "NS"       // Network status changes
)
```

### Connecting to Control Port

```bash
# Using telnet
telnet 127.0.0.1 9051

# Example commands
GETINFO version
SETEVENTS CIRC STREAM BW
SIGNAL SHUTDOWN
```

---

## Metrics & Observability

The metrics package provides comprehensive metrics collection and reporting.

### Creating Metrics

```go
import "github.com/opd-ai/go-tor/pkg/metrics"

m := metrics.New()
```

### Recording Metrics

```go
// Record circuit build
m.RecordCircuitBuild(duration, success)

// Record stream operation
m.RecordStream(opened, closed)

// Record bandwidth usage
m.RecordBandwidth(bytesRead, bytesWritten)
```

### Getting Metrics Snapshot

```go
snapshot := m.Snapshot()

fmt.Printf("Total circuits: %d\n", snapshot.TotalCircuits)
fmt.Printf("Active circuits: %d\n", snapshot.ActiveCircuits)
fmt.Printf("Failed circuits: %d\n", snapshot.FailedCircuits)
fmt.Printf("Avg build time: %v\n", snapshot.AvgCircuitBuildTime)
fmt.Printf("Total bandwidth: %d bytes\n", snapshot.TotalBytesRead + snapshot.TotalBytesWritten)
```

### Health Checks

```go
import "github.com/opd-ai/go-tor/pkg/health"

// Create health checker
checker := health.NewChecker()

// Register component checks
checker.RegisterCheck("circuits", circuitHealthCheck)
checker.RegisterCheck("socks", socksHealthCheck)

// Check health
status := checker.Check()
if status.Healthy {
    fmt.Println("All systems operational")
} else {
    fmt.Printf("Health issues: %v\n", status.Failures)
}
```

---

## Error Handling

The errors package provides structured error types with categories and severity levels.

### Error Types

```go
import "github.com/opd-ai/go-tor/pkg/errors"

// Error categories
const (
    CategoryNetwork     Category = "network"
    CategoryProtocol    Category = "protocol"
    CategoryCrypto      Category = "crypto"
    CategoryDirectory   Category = "directory"
    CategoryCircuit     Category = "circuit"
    CategoryStream      Category = "stream"
)

// Error severity
const (
    SeverityLow      Severity = "low"
    SeverityMedium   Severity = "medium"
    SeverityHigh     Severity = "high"
    SeverityCritical Severity = "critical"
)
```

### Creating Errors

```go
// Create a new error
err := errors.New(
    errors.CategoryNetwork,
    errors.SeverityHigh,
    "connection timeout",
    fmt.Errorf("failed to connect to relay"),
)

// Wrap an existing error
err = errors.Wrap(
    baseErr,
    errors.CategoryCircuit,
    errors.SeverityMedium,
    "circuit build failed",
)
```

### Error Handling Pattern

```go
if err != nil {
    if torErr, ok := err.(*errors.TorError); ok {
        log.Error("Tor error",
            "category", torErr.Category,
            "severity", torErr.Severity,
            "message", torErr.Message,
        )
        
        if torErr.Severity == errors.SeverityCritical {
            // Take immediate action
            panic(torErr)
        }
    }
    return err
}
```

---

## Resource Pooling

The pool package provides resource pooling for performance optimization.

### Buffer Pools

```go
import "github.com/opd-ai/go-tor/pkg/pool"

// Pre-configured pools
buf := pool.CellBufferPool.Get()        // 514 bytes
defer pool.CellBufferPool.Put(buf)

payloadBuf := pool.PayloadBufferPool.Get()  // 509 bytes
defer pool.PayloadBufferPool.Put(payloadBuf)

cryptoBuf := pool.CryptoBufferPool.Get()    // 1KB
defer pool.CryptoBufferPool.Put(cryptoBuf)

// Custom buffer pool
customPool := pool.NewBufferPool(2048)
buf := customPool.Get()
defer customPool.Put(buf)
```

### Circuit Pool

```go
// Create circuit pool with prebuilding
cfg := &pool.CircuitPoolConfig{
    MinCircuits:     2,
    MaxCircuits:     10,
    PrebuildEnabled: true,
    RebuildInterval: 30 * time.Second,
}

builder := func(ctx context.Context) (*circuit.Circuit, error) {
    return circuitManager.BuildCircuit(ctx)
}

pool := pool.NewCircuitPool(cfg, builder, log)
defer pool.Close()

// Get a circuit (fast - from pool)
ctx := context.Background()
circ, err := pool.Get(ctx)
if err != nil {
    return err
}

// Return circuit to pool when done
pool.Put(circ)

// Get stats
stats := pool.Stats()
fmt.Printf("Total circuits: %d\n", stats.Total)
fmt.Printf("Open circuits: %d\n", stats.Open)
```

### Connection Pool

```go
// Create connection pool
poolCfg := pool.DefaultConnectionPoolConfig()
connPool := pool.NewConnectionPool(poolCfg, log)
defer connPool.Close()

// Get connection (reuses if available)
ctx := context.Background()
connCfg := connection.DefaultConfig("relay-address:9001")
conn, err := connPool.Get(ctx, "relay-address:9001", connCfg)
if err != nil {
    return err
}

// Connection is automatically returned to pool when released

// Clean up expired connections
connPool.CleanupExpired()

// Get stats
stats := connPool.Stats()
fmt.Printf("Total: %d, In Use: %d, Idle: %d\n", 
    stats.Total, stats.InUse, stats.Idle)
```

---

## Logger API

The logger package provides structured logging with log/slog.

### Creating a Logger

```go
import "github.com/opd-ai/go-tor/pkg/logger"

// Create logger with level
log := logger.New(logger.LevelInfo, os.Stdout)

// Create component-specific logger
clientLog := log.Component("client")
```

### Log Levels

```go
const (
    LevelDebug Level = iota
    LevelInfo
    LevelWarn
    LevelError
)
```

### Logging

```go
// Structured logging
log.Info("Circuit opened", 
    "circuit_id", 123,
    "path_length", 3,
    "build_time_ms", 2500,
)

log.Error("Connection failed",
    "relay", "127.0.0.1:9001",
    "error", err,
)

// With context
ctx = logger.WithContext(ctx, log)
logFromCtx := logger.FromContext(ctx)
```

---

## Examples

### Complete Example: Basic Tor Client

```go
package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"

    "github.com/opd-ai/go-tor/pkg/client"
    "github.com/opd-ai/go-tor/pkg/config"
    "github.com/opd-ai/go-tor/pkg/logger"
)

func main() {
    // Create configuration
    cfg := config.DefaultConfig()
    cfg.SocksPort = 9050
    cfg.ControlPort = 9051
    
    // Create logger
    log := logger.New(logger.LevelInfo, os.Stdout)
    
    // Create and start client
    torClient, err := client.New(cfg, log)
    if err != nil {
        log.Error("Failed to create client", "error", err)
        os.Exit(1)
    }
    
    ctx := context.Background()
    if err := torClient.Start(ctx); err != nil {
        log.Error("Failed to start client", "error", err)
        os.Exit(1)
    }
    
    // Display status
    stats := torClient.GetStats()
    fmt.Printf("Tor client running on port %d\n", stats.SocksPort)
    
    // Wait for interrupt signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan
    
    // Graceful shutdown
    fmt.Println("Shutting down...")
    if err := torClient.Stop(); err != nil {
        log.Warn("Error during shutdown", "error", err)
    }
}
```

### Example: Using SOCKS5 Proxy

```go
package main

import (
    "fmt"
    "io"
    "net/http"
    "net/url"
    "os"
)

func main() {
    // Configure HTTP client to use SOCKS5 proxy
    proxyURL, _ := url.Parse("socks5://127.0.0.1:9050")
    transport := &http.Transport{
        Proxy: http.ProxyURL(proxyURL),
    }
    
    client := &http.Client{Transport: transport}
    
    // Make request through Tor
    resp, err := client.Get("https://check.torproject.org")
    if err != nil {
        fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
        os.Exit(1)
    }
    defer resp.Body.Close()
    
    body, _ := io.ReadAll(resp.Body)
    fmt.Println(string(body))
}
```

---

## Best Practices

### 1. Resource Management

Always close resources properly:

```go
defer torClient.Stop()
defer pool.Close()
defer server.Stop()
```

### 2. Error Handling

Use structured errors for better diagnostics:

```go
if err != nil {
    if torErr, ok := err.(*errors.TorError); ok {
        log.Error("Operation failed",
            "category", torErr.Category,
            "severity", torErr.Severity,
            "error", err,
        )
    }
    return err
}
```

### 3. Performance Optimization

Enable circuit prebuilding for better performance:

```go
cfg := config.DefaultConfig()
cfg.PrebuiltCircuits = 3
cfg.MaxIdleCircuits = 10
```

### 4. Monitoring

Use metrics and health checks:

```go
// Periodic health check
ticker := time.NewTicker(30 * time.Second)
for range ticker.C {
    status := healthChecker.Check()
    if !status.Healthy {
        log.Warn("Health check failed", "issues", status.Failures)
    }
}
```

### 5. Graceful Shutdown

Implement proper shutdown handling:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := torClient.Stop(); err != nil {
    log.Warn("Shutdown error", "error", err)
}

select {
case <-ctx.Done():
    log.Error("Shutdown timeout exceeded")
default:
    log.Info("Shutdown complete")
}
```

---

## Support

- GitHub Issues: [github.com/opd-ai/go-tor/issues](https://github.com/opd-ai/go-tor/issues)
- Documentation: [github.com/opd-ai/go-tor/docs](https://github.com/opd-ai/go-tor/tree/main/docs)
- Tor Specifications: [spec.torproject.org](https://spec.torproject.org/)

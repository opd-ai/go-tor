# go-tor API Reference
## ⚠️ CRITICAL WARNING
**THIS IS UNOFFICIAL, EXPERIMENTAL SOFTWARE** developed without the supervision or endorsement of [The Tor Project](https://www.torproject.org/).
**DO NOT USE THIS API FOR:**
- Real anonymity or privacy needs
- Personal safety or security
- Production applications
- Any situation where privacy matters
**For actual Tor integration in your applications:**
- **Users**: Use [Tor Browser](https://www.torproject.org/download/)
- **Developers**: Use [Arti](https://gitlab.torproject.org/tpo/core/arti) - the official Tor implementation in Rust with a proper API
- **All developers**: See [official Tor documentation](https://www.torproject.org/download/) for proper integration methods
This API documentation is **for educational and research purposes only**. Code written using this API should never be deployed in situations where anonymity or security is required.

**Note**: The project scope includes Tor client functionality, onion service hosting, traffic relaying (bridge/non-exit relay), and pluggable transport support. Only exit node functionality is explicitly out of scope and will not be implemented.
---
## Table of Contents
- [Client API](#client-api)
- [Circuit Management](#circuit-management)
- [SOCKS5 Proxy](#socks5-proxy)
- [Onion Service Hosting](#onion-service-hosting)
- [Relay Mode (Bridge/Non-Exit)](#relay-mode-bridgenon-exit)
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
```bash
# Using curl
curl --socks5 127.0.0.1:9050 https://check.torproject.org

# Using Firefox
# Settings → Network Settings → Manual proxy configuration
# SOCKS Host: 127.0.0.1  Port: 9050  SOCKS v5
```
### Onion Services (Client)
The SOCKS5 server automatically handles .onion addresses:
```go
// Connect to onion service (v3)
// http://example.onion will be automatically routed through Tor
```
---
## Onion Service Hosting
The onion package provides server-side functionality for hosting v3 onion services.

⚠️ **Educational/Research Only**: Not for production anonymity needs.

### Creating an Onion Service
```go
import (
    "github.com/opd-ai/go-tor/pkg/onion"
    "github.com/opd-ai/go-tor/pkg/logger"
    "github.com/opd-ai/go-tor/pkg/circuit"
    "github.com/opd-ai/go-tor/pkg/path"
)

// Configure the service
cfg := &onion.ServiceConfig{
    // Service ports: map virtual port -> local backend
    Ports: map[int]string{
        80:  "localhost:8080",  // HTTP
        443: "localhost:8443",  // HTTPS
    },
    
    // Number of introduction points (default: 3)
    NumIntroPoints: 3,
    
    // Descriptor lifetime (default: 3 hours)
    DescriptorLifetime: 3 * time.Hour,
    
    // Data directory for persistent keys and state
    DataDirectory: "/var/lib/go-tor/onion-service",
    
    // Circuit builder (required for production)
    CircuitBuilder: circuitBuilder,
    
    // Path selector (required for production)
    PathSelector: pathSelector,
}

// Create the service
log := logger.NewDefault()
service, err := onion.NewService(cfg, log)
if err != nil {
    log.Fatal("Failed to create onion service", "error", err)
}

// Get the .onion address
address := service.Address()
log.Info("Onion service address", "address", address)
// Example: "abcdefghijklmnop.onion"

// Start the service
ctx := context.Background()
if err := service.Start(ctx); err != nil {
    log.Fatal("Failed to start service", "error", err)
}

// Service is now accepting connections...

// Graceful shutdown
if err := service.Stop(); err != nil {
    log.Warn("Error during shutdown", "error", err)
}
```

### Service Configuration Options
```go
type ServiceConfig struct {
    // Optional: Existing private key (otherwise auto-generated)
    PrivateKey ed25519.PrivateKey
    
    // Service ports: virtual port -> local target
    Ports map[int]string
    
    // Number of introduction points (1-10, default: 3)
    NumIntroPoints int
    
    // Descriptor lifetime (default: 3h)
    DescriptorLifetime time.Duration
    
    // Data directory for key/state persistence
    DataDirectory string
    
    // Circuit builder for introduction points
    CircuitBuilder *circuit.Builder
    
    // Path selector for choosing relay paths
    PathSelector *path.Selector
    
    // Optional metrics collector
    Metrics MetricsCollector
}
```

### Service Persistence
Keys and state are automatically persisted to `DataDirectory`:
```
DataDirectory/
├── keys/
│   ├── identity_key      # Ed25519 identity (permissions 0600)
│   └── ntor_key          # Curve25519 ntor key
└── state/
    └── service_state.json # Descriptor revisions, intro points
```

### Service Metrics
```go
// Get service statistics
stats := service.GetStats()
fmt.Printf("Active streams: %d\n", stats.ActiveStreams)
fmt.Printf("Total connections: %d\n", stats.TotalConnections)
fmt.Printf("Introduction points: %d\n", stats.IntroductionPoints)
```

---
## Relay Mode (Bridge/Non-Exit)
The relay package implements server-side OR protocol for bridge and non-exit relays.

⚠️ **Educational/Research Only**: Not intended for production relay operation.

### Creating a Bridge Relay
```go
import (
    "github.com/opd-ai/go-tor/pkg/relay"
    "github.com/opd-ai/go-tor/pkg/logger"
)

// Generate or load relay keys
keys, err := relay.GenerateRelayKeys()
if err != nil {
    log.Fatal("Failed to generate keys", "error", err)
}

// Or load existing keys
// keys, err := relay.LoadRelayKeys("/var/lib/go-tor/keys")

// Configure OR listener
orConfig := &relay.ORListenerConfig{
    Address:        ":9001",  // OR port
    Keys:           keys,
    MaxConnections: 1000,
    ReadTimeout:    60 * time.Second,
    WriteTimeout:   60 * time.Second,
}

log := logger.NewDefault()
listener, err := relay.NewORListener(orConfig, log)
if err != nil {
    log.Fatal("Failed to create OR listener", "error", err)
}

// Start accepting connections
ctx := context.Background()
if err := listener.Start(ctx); err != nil {
    log.Fatal("Failed to start listener", "error", err)
}

log.Info("Bridge relay started",
    "address", orConfig.Address,
    "fingerprint", keys.Fingerprint())

// Relay is now accepting connections...

// Graceful shutdown
listener.Stop()
```

### Relay Descriptor Publishing
Bridges publish descriptors to bridge authorities:
```go
// Configure descriptor
descConfig := &relay.DescriptorConfig{
    Nickname:  "MyBridge",
    Contact:   "operator@example.com",
    Platform:  "go-tor/0.1.0",
    Address:   "1.2.3.4",
    ORPort:    9001,
    
    // Bandwidth in bytes/sec
    BandwidthAvg:   1024 * 1024,      // 1 MB/s
    BandwidthBurst: 2 * 1024 * 1024,  // 2 MB/s
    BandwidthObs:   1024 * 1024,
    
    // Bridge-specific: no DirPort
    DirPort: 0,
    
    // Non-exit relay
    ExitPolicy: []string{"reject *:*"},
}

// Generate descriptor
descriptor, err := relay.GenerateServerDescriptor(keys, descConfig)
if err != nil {
    log.Fatal("Failed to generate descriptor", "error", err)
}

// Publish to bridge authority
publisher := relay.NewDescriptorPublisher(&relay.PublisherConfig{
    Authorities: []string{"https://bridge-authority.torproject.org"},
    Interval:    18 * time.Hour,  // Refresh every 18h
    Timeout:     30 * time.Second,
}, log)

if err := publisher.Publish(descriptor, nil); err != nil {
    log.Warn("Failed to publish descriptor", "error", err)
}

// Or use scheduled publishing
scheduledPub := relay.NewScheduledPublisher(publisher, descriptor, log)
go scheduledPub.Run(ctx)
```

### Relay Security Features
```go
// Rate limiting
rateLimiter := relay.NewRateLimiter(&relay.RateLimitConfig{
    CircuitCreationRate: 10.0,    // circuits/sec
    CircuitCreationBurst: 20,
    ConnectionRate: 5.0,           // connections/sec per IP
    ConnectionBurst: 10,
    CellProcessingRate: 100.0,    // cells/sec per circuit
    CellProcessingBurst: 200,
})

// DoS protection
protection := relay.NewProtectionManager(&relay.ProtectionConfig{
    MaxConnectionsPerIP:     10,
    MaxCircuitsPerConnection: 1000,
    MaxTotalConnections:     5000,
})

// Check before accepting connection
if !protection.AllowConnection(remoteIP) {
    // Reject connection
    log.Warn("Connection rejected by DoS protection", "ip", remoteIP)
}
```

### Relay Metrics
```go
// Initialize metrics
metrics := relay.NewRelayMetrics()

// Record operations
metrics.CircuitsCreated.Inc()
metrics.CellsForwarded.Inc()
metrics.BytesReceived.Add(1024)

// Get snapshot
snapshot := metrics.Snapshot()
fmt.Printf("Circuits: %d\n", snapshot.CircuitsCreated)
fmt.Printf("Bandwidth: %d bytes\n", snapshot.BytesReceived)
fmt.Printf("Uptime: %v\n", snapshot.Uptime)
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
    CircuitPoolMinSize       int
    CircuitPoolMaxSize       int
    ConnectionPoolMaxIdle    int
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
import (
    "context"
    "github.com/opd-ai/go-tor/pkg/health"
)

// Create health monitor
monitor := health.NewMonitor()

// Create and register component health checkers
// Using built-in circuit health checker:
circuitChecker := health.NewCircuitHealthChecker(func() health.CircuitStats {
    return health.CircuitStats{
        ActiveCircuits: 3,
        MinRequired:    2,
        FailedBuilds:   0,
    }
})
monitor.RegisterChecker(circuitChecker)

// Using built-in connection health checker:
connChecker := health.NewConnectionHealthChecker(func() health.ConnectionStats {
    return health.ConnectionStats{
        OpenConnections:    5,
        FailedConnections:  0,
        ConnectionAttempts: 10,
    }
})
monitor.RegisterChecker(connChecker)

// Check health of all components
ctx := context.Background()
overallHealth := monitor.Check(ctx)

if overallHealth.Status == health.StatusHealthy {
    fmt.Println("All systems operational")
} else {
    for name, component := range overallHealth.Components {
        if component.Status != health.StatusHealthy {
            fmt.Printf("Component %s: %s - %s\n", name, component.Status, component.Message)
        }
    }
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
import (
    "log/slog"
    "os"

    "github.com/opd-ai/go-tor/pkg/logger"
)

// Create logger with level (uses slog.Level)
log := logger.New(slog.LevelInfo, os.Stdout)

// Or use the default logger (info level, stdout)
log = logger.NewDefault()

// Create component-specific logger
clientLog := log.Component("client")

// Parse log level from string
level, err := logger.ParseLevel("debug") // Returns slog.LevelDebug
if err != nil {
    // ParseLevel returns slog.LevelInfo as default on error
    level = slog.LevelInfo
}
log = logger.New(level, os.Stdout)
```
### Log Levels
Log levels use the standard `log/slog` levels:
```go
import "log/slog"

// Available levels from slog package
slog.LevelDebug  // -4
slog.LevelInfo   // 0
slog.LevelWarn   // 4
slog.LevelError  // 8

// Parse from string with error handling
level, err := logger.ParseLevel("debug")
if err != nil {
    // ParseLevel returns slog.LevelInfo as default on error
    level = slog.LevelInfo
}
// Valid strings: "debug", "info", "warn", "error"
// Returns corresponding slog.Level values shown above
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
    log := logger.NewDefault()

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
```go
cfg := config.DefaultConfig()
cfg.CircuitPoolMinSize = 3
cfg.CircuitPoolMaxSize = 10
```
### 4. Monitoring
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
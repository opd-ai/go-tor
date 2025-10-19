# go-tor Tutorial: Getting Started

This tutorial will guide you through setting up and using go-tor, from installation to creating your first Tor-enabled application.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Installation](#installation)
3. [Quick Start](#quick-start)
4. [Building Your First Tor Client](#building-your-first-tor-client)
5. [Using the SOCKS5 Proxy](#using-the-socks5-proxy)
6. [Monitoring and Control](#monitoring-and-control)
7. [Performance Tuning](#performance-tuning)
8. [Troubleshooting](#troubleshooting)

---

## Prerequisites

Before you begin, ensure you have:

- **Go 1.21 or later** installed
- Basic understanding of Go programming
- Terminal/command line access
- Internet connection (for downloading dependencies and connecting to Tor network)

Check your Go version:

```bash
go version
# Should output: go version go1.21+ ...
```

---

## Installation

### Step 1: Clone the Repository

```bash
git clone https://github.com/opd-ai/go-tor.git
cd go-tor
```

### Step 2: Download Dependencies

```bash
go mod download
go mod verify
```

### Step 3: Build the Client

```bash
make build
```

Or build manually:

```bash
go build -o bin/tor-client ./cmd/tor-client
```

### Step 4: Verify Installation

```bash
./bin/tor-client -version
```

You should see output like:

```
go-tor version v0.1.0-dev (built 2025-10-19_15:30:00)
Pure Go Tor client implementation
```

---

## Quick Start

### Run with Default Settings

The easiest way to get started:

```bash
./bin/tor-client
```

This will:
- Start a SOCKS5 proxy on port 9050
- Start a control server on port 9051
- Create a data directory at `/var/lib/tor` (or current directory if not writable)

You should see output like:

```
time=2025-10-19T15:30:00Z level=INFO msg="Starting go-tor" version=dev
time=2025-10-19T15:30:00Z level=INFO msg="SOCKS5 proxy available at" address=127.0.0.1:9050
time=2025-10-19T15:30:00Z level=INFO msg="Press Ctrl+C to exit"
```

### Test the Connection

In another terminal, test the SOCKS5 proxy:

```bash
curl --socks5 127.0.0.1:9050 https://check.torproject.org/api/ip
```

If successful, you'll see JSON output indicating you're using Tor:

```json
{"IsTor":true,"IP":"<tor-exit-ip>"}
```

### Stop the Client

Press `Ctrl+C` to gracefully shut down:

```
^C
time=2025-10-19T15:35:00Z level=INFO msg="Received shutdown signal" signal=interrupt
time=2025-10-19T15:35:00Z level=INFO msg="Initiating graceful shutdown..."
time=2025-10-19T15:35:01Z level=INFO msg="Shutdown complete"
```

---

## Building Your First Tor Client

Let's create a simple Go application that uses go-tor as a library.

### Step 1: Create a New Project

```bash
mkdir my-tor-app
cd my-tor-app
go mod init my-tor-app
```

### Step 2: Add go-tor Dependency

```bash
go get github.com/opd-ai/go-tor
```

### Step 3: Write the Application

Create `main.go`:

```go
package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/opd-ai/go-tor/pkg/client"
    "github.com/opd-ai/go-tor/pkg/config"
    "github.com/opd-ai/go-tor/pkg/logger"
)

func main() {
    // Step 1: Create configuration
    cfg := config.DefaultConfig()
    cfg.SocksPort = 9050
    cfg.ControlPort = 9051
    cfg.DataDirectory = "./tor-data"
    cfg.LogLevel = "info"
    
    // Step 2: Validate configuration
    if err := cfg.Validate(); err != nil {
        fmt.Fprintf(os.Stderr, "Invalid configuration: %v\n", err)
        os.Exit(1)
    }
    
    // Step 3: Create logger
    level, _ := logger.ParseLevel(cfg.LogLevel)
    log := logger.New(level, os.Stdout)
    
    log.Info("Starting Tor client application")
    
    // Step 4: Create Tor client
    torClient, err := client.New(cfg, log)
    if err != nil {
        log.Error("Failed to create Tor client", "error", err)
        os.Exit(1)
    }
    
    // Step 5: Start the client
    ctx := context.Background()
    if err := torClient.Start(ctx); err != nil {
        log.Error("Failed to start Tor client", "error", err)
        os.Exit(1)
    }
    
    // Step 6: Display status
    stats := torClient.GetStats()
    log.Info("Tor client running",
        "socks_port", stats.SocksPort,
        "control_port", stats.ControlPort,
        "active_circuits", stats.ActiveCircuits,
    )
    
    fmt.Printf("\n✓ SOCKS5 proxy running on 127.0.0.1:%d\n", stats.SocksPort)
    fmt.Printf("✓ Configure your applications to use this proxy\n")
    fmt.Printf("✓ Press Ctrl+C to exit\n\n")
    
    // Step 7: Wait for interrupt signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    <-sigChan
    
    // Step 8: Graceful shutdown
    log.Info("Shutting down gracefully...")
    
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if err := torClient.Stop(); err != nil {
        log.Warn("Error during shutdown", "error", err)
    }
    
    select {
    case <-shutdownCtx.Done():
        log.Error("Shutdown timeout exceeded")
        os.Exit(1)
    default:
        log.Info("Shutdown complete")
    }
}
```

### Step 4: Run Your Application

```bash
go run main.go
```

---

## Using the SOCKS5 Proxy

Once your Tor client is running, you can route traffic through it using the SOCKS5 proxy.

### Using curl

```bash
# Test Tor connection
curl --socks5 127.0.0.1:9050 https://check.torproject.org

# Access a website through Tor
curl --socks5 127.0.0.1:9050 https://www.example.com

# Access an onion service
curl --socks5 127.0.0.1:9050 http://example.onion
```

### Using Go HTTP Client

Create `http-client.go`:

```go
package main

import (
    "fmt"
    "io"
    "net/http"
    "net/url"
)

func main() {
    // Configure SOCKS5 proxy
    proxyURL, err := url.Parse("socks5://127.0.0.1:9050")
    if err != nil {
        panic(err)
    }
    
    // Create HTTP client with proxy
    transport := &http.Transport{
        Proxy: http.ProxyURL(proxyURL),
    }
    
    client := &http.Client{
        Transport: transport,
        Timeout:   30 * time.Second,
    }
    
    // Make request through Tor
    resp, err := client.Get("https://check.torproject.org/api/ip")
    if err != nil {
        fmt.Printf("Request failed: %v\n", err)
        return
    }
    defer resp.Body.Close()
    
    // Read response
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        fmt.Printf("Failed to read response: %v\n", err)
        return
    }
    
    fmt.Printf("Response: %s\n", body)
}
```

Run it:

```bash
go run http-client.go
```

### Configuring Applications

Many applications support SOCKS5 proxies:

#### Firefox

1. Open Settings → Network Settings
2. Select "Manual proxy configuration"
3. Set SOCKS Host: `127.0.0.1`
4. Set Port: `9050`
5. Select "SOCKS v5"
6. Check "Proxy DNS when using SOCKS v5"

#### SSH

```bash
ssh -o ProxyCommand="nc -X 5 -x 127.0.0.1:9050 %h %p" user@example.com
```

#### Git

```bash
git config --global http.proxy socks5://127.0.0.1:9050
```

---

## Monitoring and Control

### Using the Control Protocol

The control protocol allows you to monitor and manage your Tor client.

#### Connect via Telnet

```bash
telnet 127.0.0.1 9051
```

#### Get Information

```
GETINFO version
250-version=go-tor dev
250 OK

GETINFO status/circuit-established
250-status/circuit-established=1
250 OK
```

#### Subscribe to Events

```
SETEVENTS CIRC STREAM BW
250 OK
```

Now you'll receive real-time events:

```
650 CIRC 1 LAUNCHED
650 CIRC 1 EXTENDED
650 CIRC 1 BUILT
650 BW 1024 2048
650 STREAM 1 NEW
650 STREAM 1 SENTCONNECT
```

### Programmatic Control

Create `control-example.go`:

```go
package main

import (
    "bufio"
    "fmt"
    "net"
    "strings"
)

func main() {
    // Connect to control port
    conn, err := net.Dial("tcp", "127.0.0.1:9051")
    if err != nil {
        panic(err)
    }
    defer conn.Close()
    
    reader := bufio.NewReader(conn)
    
    // Send GETINFO command
    fmt.Fprintf(conn, "GETINFO version\r\n")
    
    // Read response
    for {
        line, err := reader.ReadString('\n')
        if err != nil {
            break
        }
        
        fmt.Print(line)
        
        // Check for end of response
        if strings.HasPrefix(line, "250 ") {
            break
        }
    }
}
```

---

## Performance Tuning

### Enable Circuit Prebuilding

Prebuilding circuits significantly reduces connection time.

Create a custom configuration:

```go
cfg := config.DefaultConfig()

// Circuit prebuilding
cfg.PrebuiltCircuits = 3        // Keep 3 circuits ready
cfg.MaxIdleCircuits = 10        // Allow up to 10 idle circuits
cfg.CircuitBuildTimeout = 60 * time.Second

// Connection pooling
cfg.ConnectionPoolSize = 20     // Pool 20 connections
cfg.MaxCircuitDirtiness = 10 * time.Minute  // Rotate circuits every 10 min
```

### Monitor Performance

Use metrics to track performance:

```go
// Get client statistics
stats := torClient.GetStats()

fmt.Printf("Active Circuits: %d\n", stats.ActiveCircuits)
fmt.Printf("Uptime: %v\n", stats.Uptime)

// If you have access to metrics:
// snapshot := metrics.Snapshot()
// fmt.Printf("Avg Circuit Build Time: %v\n", snapshot.AvgCircuitBuildTime)
```

### Configuration File

Create `torrc`:

```
# Network settings
SocksPort 9050
ControlPort 9051
DataDirectory ./tor-data

# Logging
Log info

# Performance
PrebuiltCircuits 3
MaxIdleCircuits 10
CircuitBuildTimeout 60
MaxCircuitDirtiness 600

# Connection pooling
ConnectionPoolSize 20
```

Run with config file:

```bash
./bin/tor-client -config torrc
```

---

## Troubleshooting

### Common Issues

#### Issue 1: Port Already in Use

**Error**: `bind: address already in use`

**Solution**: Change the port in configuration:

```go
cfg.SocksPort = 9150  // Use different port
cfg.ControlPort = 9151
```

Or stop the conflicting service:

```bash
# Find process using port 9050
lsof -i :9050

# Kill the process
kill <PID>
```

#### Issue 2: Permission Denied for Data Directory

**Error**: `permission denied: /var/lib/tor`

**Solution**: Use a writable directory:

```go
cfg.DataDirectory = "./tor-data"  // Current directory
// or
cfg.DataDirectory = os.TempDir() + "/tor-data"  // Temp directory
```

#### Issue 3: Circuit Build Timeout

**Error**: `circuit build timeout`

**Solution**: Increase timeout:

```go
cfg.CircuitBuildTimeout = 120 * time.Second  // 2 minutes
```

#### Issue 4: Cannot Connect to Tor Network

**Symptoms**: No circuits being built

**Solutions**:
1. Check your internet connection
2. Verify firewall settings (allow outbound connections to port 443 and 9001)
3. Try running with debug logging:

```go
cfg.LogLevel = "debug"
```

### Debug Logging

Enable detailed logging to diagnose issues:

```go
// Create debug logger
log := logger.New(logger.LevelDebug, os.Stdout)

// Or set via config
cfg.LogLevel = "debug"
```

Debug output will show:
- Circuit building steps
- Connection attempts
- Protocol handshake details
- Error details

### Getting Help

If you encounter issues:

1. Check the [Troubleshooting Guide](TROUBLESHOOTING.md)
2. Review [Architecture Documentation](ARCHITECTURE.md)
3. Search [GitHub Issues](https://github.com/opd-ai/go-tor/issues)
4. Create a new issue with:
   - go-tor version
   - Go version
   - Operating system
   - Configuration used
   - Complete error message
   - Debug logs (if applicable)

---

## Next Steps

Now that you have go-tor running, you can:

1. **Explore the API**: Read the [API Reference](API.md) for detailed documentation
2. **Configure Advanced Features**: Check [Configuration Guide](PRODUCTION.md)
3. **Monitor Performance**: Learn about [Metrics & Observability](PERFORMANCE.md)
4. **Secure Your Application**: Review [Security Best Practices](SECURITY.md)
5. **Contribute**: See [Development Guide](DEVELOPMENT.md)

---

## Additional Resources

- [Official Tor Specifications](https://spec.torproject.org/)
- [Tor Project](https://www.torproject.org/)
- [go-tor GitHub Repository](https://github.com/opd-ai/go-tor)
- [Go Documentation](https://golang.org/doc/)

---

## Quick Reference

### Common Commands

```bash
# Build
make build

# Run with defaults
./bin/tor-client

# Run with custom ports
./bin/tor-client -socks-port 9150 -control-port 9151

# Run with config file
./bin/tor-client -config /etc/tor/torrc

# Run with debug logging
./bin/tor-client -log-level debug

# Show version
./bin/tor-client -version
```

### Common Configuration

```go
// Minimal configuration
cfg := config.DefaultConfig()
cfg.SocksPort = 9050

// Performance-optimized
cfg := config.DefaultConfig()
cfg.PrebuiltCircuits = 3
cfg.MaxIdleCircuits = 10
cfg.ConnectionPoolSize = 20

// Debug configuration
cfg := config.DefaultConfig()
cfg.LogLevel = "debug"
cfg.DataDirectory = "./debug-data"
```

### Test Commands

```bash
# Test SOCKS5
curl --socks5 127.0.0.1:9050 https://check.torproject.org/api/ip

# Test control port
echo "GETINFO version" | nc 127.0.0.1 9051

# Monitor circuits
echo "SETEVENTS CIRC" | nc 127.0.0.1 9051
```

---

**Congratulations!** You've completed the go-tor tutorial. You now know how to:
- ✓ Install and run go-tor
- ✓ Use it as a library in your applications
- ✓ Route traffic through the SOCKS5 proxy
- ✓ Monitor and control the client
- ✓ Optimize performance
- ✓ Troubleshoot common issues

Happy coding with go-tor! 🎉

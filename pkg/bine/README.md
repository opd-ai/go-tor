# Package bine - Zero-Configuration Wrapper for cretz/bine + go-tor

The `pkg/bine` package provides a zero-configuration wrapper that seamlessly integrates `cretz/bine` with `go-tor`, giving you the best of both libraries with minimal setup.

## Overview

This wrapper automatically manages:
- **go-tor client** for pure-Go Tor connectivity (no external binary needed)
- **cretz/bine** for hidden service management (optional, requires Tor binary)
- SOCKS5 proxy configuration
- Lifecycle management and graceful shutdown

## Quick Start

### Simple Client Connection

```go
package main

import (
    "log"
    "github.com/opd-ai/go-tor/pkg/bine"
)

func main() {
    // Zero-configuration connection
    client, err := bine.Connect()
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Make HTTP requests through Tor
    httpClient, _ := client.HTTPClient()
    resp, _ := httpClient.Get("https://check.torproject.org")
    defer resp.Body.Close()
    
    // Or get the SOCKS proxy address
    proxyAddr := client.ProxyAddr()  // "127.0.0.1:9050"
}
```

### Creating a Hidden Service

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "github.com/opd-ai/go-tor/pkg/bine"
)

func main() {
    // Enable bine for hidden services
    client, err := bine.ConnectWithOptions(&bine.Options{
        EnableBine: true,  // Required for hidden services
    })
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Create v3 onion service on port 80
    ctx := context.Background()
    service, err := client.CreateHiddenService(ctx, 80)
    if err != nil {
        log.Fatal(err)
    }
    defer service.Close()

    fmt.Printf("Service available at: http://%s\n", service.OnionAddress())

    // Serve HTTP on the onion service
    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello from hidden service!")
    })
    
    http.Serve(service, mux)
}
```

## Features

### Automatic Configuration

No manual setup required. The wrapper automatically:
- Starts go-tor with sensible defaults
- Waits for circuits to be ready
- Configures SOCKS5 proxy
- Manages lifecycle and cleanup

### Pure Go Client

The underlying go-tor client is pure Go:
- No external Tor binary needed for client operations
- Easy cross-compilation
- Small binary size
- Embedded in your application

### Optional Hidden Services

Hidden services use bine (requires Tor binary):
- Easy v3 onion service creation
- Automatic descriptor publishing
- Simple net.Listener interface
- Production-ready

## API Reference

### Client Creation

#### `Connect() (*Client, error)`

Zero-configuration connection. Uses defaults for everything.

```go
client, err := bine.Connect()
defer client.Close()
```

#### `ConnectWithOptions(opts *Options) (*Client, error)`

Connect with custom options:

```go
client, err := bine.ConnectWithOptions(&bine.Options{
    LogLevel:       "debug",
    EnableBine:     true,
    StartupTimeout: 120 * time.Second,
})
```

### Options

```go
type Options struct {
    // SocksPort specifies the SOCKS5 proxy port (default: auto-selected)
    SocksPort int

    // ControlPort specifies the control protocol port (default: auto-selected)
    ControlPort int

    // DataDirectory specifies the data directory (default: platform-specific)
    DataDirectory string

    // LogLevel: "debug", "info", "warn", "error" (default: "info")
    LogLevel string

    // EnableBine enables hidden services (requires Tor binary installed)
    EnableBine bool

    // StartupTimeout is max time to wait for ready (default: 90s)
    StartupTimeout time.Duration
}
```

### Client Methods

#### `Close() error`

Gracefully shuts down all Tor instances.

```go
defer client.Close()
```

#### `ProxyAddr() string`

Returns SOCKS5 proxy address (e.g., "127.0.0.1:9050").

```go
addr := client.ProxyAddr()
```

#### `ProxyURL() string`

Returns SOCKS5 proxy URL (e.g., "socks5://127.0.0.1:9050").

```go
url := client.ProxyURL()
```

#### `HTTPClient() (*http.Client, error)`

Returns an HTTP client configured to use Tor.

```go
httpClient, err := client.HTTPClient()
resp, _ := httpClient.Get("https://example.com")
```

#### `Dialer() proxy.Dialer`

Returns a SOCKS5 dialer for custom connections.

```go
dialer := client.Dialer()
conn, _ := dialer.Dial("tcp", "example.com:80")
```

#### `IsReady() bool`

Checks if the client has active circuits.

```go
if client.IsReady() {
    // Make requests
}
```

### Hidden Services

#### `CreateHiddenService(ctx context.Context, remotePorts ...int) (*HiddenService, error)`

Creates a v3 onion service. Requires `EnableBine: true`.

```go
service, err := client.CreateHiddenService(ctx, 80, 443)
fmt.Printf("Address: http://%s\n", service.OnionAddress())
http.Serve(service, handler)
```

#### `CreateHiddenServiceWithConfig(ctx context.Context, config *HiddenServiceConfig) (*HiddenService, error)`

Creates a hidden service with advanced configuration.

```go
config := &bine.HiddenServiceConfig{
    RemotePorts: []int{80},
    PrivateKey:  savedKey,  // For persistent address
}
service, err := client.CreateHiddenServiceWithConfig(ctx, config)
```

### HiddenService Methods

#### `OnionAddress() string`

Returns the .onion address (without "http://").

```go
addr := service.OnionAddress()
fmt.Printf("http://%s\n", addr)
```

#### `Accept() (net.Conn, error)`

Accepts incoming connections (implements net.Listener).

```go
conn, err := service.Accept()
```

#### `Close() error`

Shuts down the hidden service.

```go
defer service.Close()
```

## Examples

### Making HTTP Requests

```go
client, _ := bine.Connect()
defer client.Close()

httpClient, _ := client.HTTPClient()
resp, _ := httpClient.Get("https://check.torproject.org/api/ip")
defer resp.Body.Close()

body, _ := io.ReadAll(resp.Body)
fmt.Println(string(body))
```

### Custom Dialer

```go
client, _ := bine.Connect()
defer client.Close()

dialer := client.Dialer()
conn, _ := dialer.Dial("tcp", "example.onion:80")
defer conn.Close()

// Use the connection
conn.Write([]byte("GET / HTTP/1.1\r\nHost: example.onion\r\n\r\n"))
```

### Multiple Hidden Services

```go
client, _ := bine.ConnectWithOptions(&bine.Options{EnableBine: true})
defer client.Close()

// Create multiple services
service1, _ := client.CreateHiddenService(ctx, 80)
service2, _ := client.CreateHiddenService(ctx, 443)

fmt.Printf("HTTP:  http://%s\n", service1.OnionAddress())
fmt.Printf("HTTPS: http://%s\n", service2.OnionAddress())
```

### Persistent Hidden Service

```go
// First run - save the key
client, _ := bine.ConnectWithOptions(&bine.Options{EnableBine: true})
service, _ := client.CreateHiddenService(ctx, 80)

// Save the key (implementation depends on bine API)
// savedKey := service.Key()

// Later runs - reuse the key
config := &bine.HiddenServiceConfig{
    RemotePorts: []int{80},
    PrivateKey:  savedKey,  // Same .onion address
}
service, _ := client.CreateHiddenServiceWithConfig(ctx, config)
```

## Error Handling

### Common Errors

**"bine not enabled"**
- You're trying to create a hidden service without `EnableBine: true`
- Solution: Use `ConnectWithOptions(&bine.Options{EnableBine: true})`

**"failed to start bine (Tor binary required)"**
- Tor binary not installed
- Solution: Install Tor (`apt-get install tor`, `brew install tor`, etc.)

**"timeout waiting for go-tor"**
- Network issues or very slow first connection
- Solution: Increase `StartupTimeout` in options

**"failed to create hidden service"**
- bine/Tor process issue
- Solution: Check Tor binary is working, check logs

## Performance

- **Client startup**: 30-90 seconds (first run), faster on subsequent runs
- **Hidden service creation**: 2-3 minutes (descriptor publication)
- **Memory usage**: ~50MB (go-tor) + ~30-50MB (bine, if enabled)
- **Binary size**: Minimal overhead, ~100KB for wrapper

## Security Considerations

- Always use HTTPS even over Tor (defense in depth)
- Validate all inputs to hidden services
- Implement rate limiting
- Keep dependencies updated
- Monitor for suspicious activity
- Protect hidden service keys

## Comparison with Direct Usage

### Using Wrapper (Recommended)

```go
// Zero configuration
client, _ := bine.Connect()
defer client.Close()

httpClient, _ := client.HTTPClient()
resp, _ := httpClient.Get("https://example.com")
```

### Without Wrapper (More Code)

```go
// Manual setup
torClient, _ := client.Connect()
defer torClient.Close()
torClient.WaitUntilReady(90 * time.Second)

dialer, _ := proxy.SOCKS5("tcp", torClient.ProxyAddr(), nil, proxy.Direct)
httpClient := &http.Client{
    Transport: &http.Transport{Dial: dialer.Dial},
}
resp, _ := httpClient.Get("https://example.com")
```

The wrapper reduces boilerplate and handles edge cases automatically.

## Requirements

### For Client Operations (Always Available)
- Go 1.24+
- go-tor (included)

### For Hidden Services (Optional)
- Tor binary installed on system
- `EnableBine: true` in options

## See Also

- [go-tor Documentation](https://github.com/opd-ai/go-tor)
- [cretz/bine Documentation](https://pkg.go.dev/github.com/cretz/bine)
- [Examples Directory](../../examples/bine-examples/)

## Contributing

Contributions welcome! Please ensure:
- All tests pass
- Code is properly documented
- Examples are updated if API changes

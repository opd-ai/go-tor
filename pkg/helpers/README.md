# HTTP Client Helpers

The `helpers` package provides convenience functions for integrating go-tor with common Go patterns, particularly HTTP clients.

## Overview

This package simplifies the process of using the Tor client with standard library HTTP clients and custom network applications. It eliminates boilerplate code and provides idiomatic Go interfaces.

## Features

- **Zero-boilerplate HTTP client creation**: One function call to get a Tor-enabled HTTP client
- **Custom configuration support**: Fine-tune connection parameters for your use case
- **Existing client wrapping**: Route existing HTTP clients through Tor without recreation
- **Context-aware dialing**: Custom dial functions for advanced network applications
- **Type-safe interfaces**: Testable design with interface-based API

## Quick Start

### Basic HTTP Client

```go
import (
    "github.com/opd-ai/go-tor/pkg/client"
    "github.com/opd-ai/go-tor/pkg/helpers"
)

// Connect to Tor
torClient, _ := client.Connect()
defer torClient.Close()

// Wait for Tor to be ready
torClient.WaitUntilReady(90 * time.Second)

// Create HTTP client
httpClient, _ := helpers.NewHTTPClient(torClient, nil)

// Make requests through Tor
resp, _ := httpClient.Get("https://check.torproject.org")
```

### Custom Configuration

```go
config := &helpers.HTTPClientConfig{
    Timeout:             60 * time.Second,
    MaxIdleConns:        20,
    DisableKeepAlives:   false,
    IdleConnTimeout:     120 * time.Second,
    TLSHandshakeTimeout: 15 * time.Second,
}

httpClient, _ := helpers.NewHTTPClient(torClient, config)
```

### Wrap Existing Client

```go
existingClient := &http.Client{
    Timeout: 60 * time.Second,
}

// Wrap it to use Tor
helpers.WrapHTTPClient(existingClient, torClient, nil)

// Now existingClient routes through Tor
resp, _ := existingClient.Get("https://example.com")
```

### Custom Network Applications

```go
// Get a context-aware dial function
dialFunc := helpers.DialContext(torClient)

// Use it with custom transports or network applications
transport := &http.Transport{
    DialContext: dialFunc,
}
```

## API Reference

### NewHTTPClient

```go
func NewHTTPClient(torClient TorClient, config *HTTPClientConfig) (*http.Client, error)
```

Creates an `http.Client` configured to use the Tor SOCKS5 proxy. This is the most common and convenient way to use Tor with HTTP requests.

**Parameters:**
- `torClient`: A Tor client instance (typically `*client.SimpleClient`)
- `config`: Optional configuration (uses defaults if `nil`)

**Returns:**
- `*http.Client`: Configured HTTP client
- `error`: Error if configuration fails

**Example:**
```go
httpClient, err := helpers.NewHTTPClient(torClient, nil)
if err != nil {
    log.Fatal(err)
}
resp, _ := httpClient.Get("https://example.com")
```

### NewHTTPTransport

```go
func NewHTTPTransport(torClient TorClient, config *HTTPClientConfig) (*http.Transport, error)
```

Creates an `http.Transport` configured for Tor. Use this when you need to customize the transport before creating a client.

**Parameters:**
- `torClient`: A Tor client instance
- `config`: Optional configuration (uses defaults if `nil`)

**Returns:**
- `*http.Transport`: Configured transport
- `error`: Error if configuration fails

**Example:**
```go
transport, err := helpers.NewHTTPTransport(torClient, nil)
if err != nil {
    log.Fatal(err)
}

// Customize transport
transport.DisableCompression = true

// Create client with custom transport
httpClient := &http.Client{Transport: transport}
```

### DialContext

```go
func DialContext(torClient TorClient) func(ctx context.Context, network, addr string) (net.Conn, error)
```

Returns a `DialContext` function that uses the Tor SOCKS5 proxy. Useful for custom network applications that need context-aware dialing.

**Parameters:**
- `torClient`: A Tor client instance

**Returns:**
- Function with signature `func(context.Context, string, string) (net.Conn, error)`

**Example:**
```go
dialFunc := helpers.DialContext(torClient)

// Use with custom transport
transport := &http.Transport{
    DialContext: dialFunc,
}

// Or dial directly
conn, err := dialFunc(context.Background(), "tcp", "example.onion:80")
```

### WrapHTTPClient

```go
func WrapHTTPClient(httpClient *http.Client, torClient TorClient, config *HTTPClientConfig) error
```

Wraps an existing `http.Client` to use the Tor proxy. This modifies the client in-place by replacing its transport.

**Parameters:**
- `httpClient`: The HTTP client to wrap
- `torClient`: A Tor client instance
- `config`: Optional configuration (uses defaults if `nil`)

**Returns:**
- `error`: Error if wrapping fails

**Example:**
```go
existingClient := &http.Client{
    Timeout: 60 * time.Second,
}

err := helpers.WrapHTTPClient(existingClient, torClient, nil)
if err != nil {
    log.Fatal(err)
}

// existingClient now routes through Tor
```

### HTTPClientConfig

```go
type HTTPClientConfig struct {
    Timeout             time.Duration
    DialTimeout         time.Duration
    TLSHandshakeTimeout time.Duration
    MaxIdleConns        int
    IdleConnTimeout     time.Duration
    DisableKeepAlives   bool
}
```

Configuration options for HTTP clients.

**Fields:**
- `Timeout`: HTTP request timeout (default: 30s)
- `DialTimeout`: Connection establishment timeout (default: 10s)
- `TLSHandshakeTimeout`: TLS handshake timeout (default: 10s)
- `MaxIdleConns`: Maximum idle connections (default: 10)
- `IdleConnTimeout`: Idle connection timeout (default: 90s)
- `DisableKeepAlives`: Disable HTTP keep-alives (default: false)

**Example:**
```go
config := &helpers.HTTPClientConfig{
    Timeout:      60 * time.Second,
    MaxIdleConns: 20,
}
```

### DefaultHTTPClientConfig

```go
func DefaultHTTPClientConfig() *HTTPClientConfig
```

Returns a configuration with sensible defaults for Tor HTTP clients.

**Returns:**
- `*HTTPClientConfig`: Default configuration

**Example:**
```go
config := helpers.DefaultHTTPClientConfig()
config.Timeout = 60 * time.Second // Override specific values
```

## Complete Examples

See [examples/http-helpers-demo](../../examples/http-helpers-demo) for a working demonstration of all features.

## Best Practices

### 1. Wait for Tor to be Ready

Always wait for Tor to bootstrap before making requests:

```go
torClient, _ := client.Connect()
defer torClient.Close()

// Wait for circuits to be established
if err := torClient.WaitUntilReady(90 * time.Second); err != nil {
    log.Fatal("Tor not ready:", err)
}

// Now safe to create HTTP client and make requests
httpClient, _ := helpers.NewHTTPClient(torClient, nil)
```

### 2. Use Appropriate Timeouts

Tor connections are slower than direct connections. Configure timeouts accordingly:

```go
config := &helpers.HTTPClientConfig{
    Timeout:             45 * time.Second,  // Longer than typical HTTP
    TLSHandshakeTimeout: 15 * time.Second,  // Tor adds latency
}

httpClient, _ := helpers.NewHTTPClient(torClient, config)
```

### 3. Handle Context Cancellation

For long-running operations, use contexts:

```go
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()

req, _ := http.NewRequestWithContext(ctx, "GET", "https://example.onion", nil)
resp, err := httpClient.Do(req)
```

### 4. Reuse HTTP Clients

Create one HTTP client and reuse it for multiple requests:

```go
// Good: Create once, reuse
httpClient, _ := helpers.NewHTTPClient(torClient, nil)
resp1, _ := httpClient.Get("https://example1.com")
resp2, _ := httpClient.Get("https://example2.com")

// Bad: Creating new client for each request
// (wastes resources and circuits)
```

### 5. Test with Mock Clients

The `TorClient` interface makes testing easy:

```go
type mockTorClient struct{}

func (m *mockTorClient) ProxyURL() string {
    return "socks5://127.0.0.1:9050"
}

func TestMyFunction(t *testing.T) {
    mock := &mockTorClient{}
    httpClient, _ := helpers.NewHTTPClient(mock, nil)
    // Test your code with the mock client
}
```

## Performance Considerations

- **Connection Pooling**: The helpers configure connection pooling by default. Adjust `MaxIdleConns` based on your workload.
- **Keep-Alives**: Enabled by default for better performance. Disable only if needed.
- **Circuit Reuse**: Multiple HTTP requests reuse the same Tor circuit unless circuit isolation is configured.

## Comparison: Before vs After

### Before (Manual Configuration)

```go
// 20+ lines of boilerplate
proxyURL, _ := url.Parse(torClient.ProxyURL())
dialer, _ := proxy.FromURL(proxyURL, proxy.Direct)

transport := &http.Transport{
    DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
        return dialer.Dial(network, addr)
    },
    MaxIdleConns:          10,
    IdleConnTimeout:       90 * time.Second,
    TLSHandshakeTimeout:   10 * time.Second,
    ResponseHeaderTimeout: 30 * time.Second,
}

httpClient := &http.Client{
    Transport: transport,
    Timeout:   30 * time.Second,
}

// Make request
resp, _ := httpClient.Get("https://example.com")
```

### After (Using Helpers)

```go
// 2 lines
httpClient, _ := helpers.NewHTTPClient(torClient, nil)
resp, _ := httpClient.Get("https://example.com")
```

**Result**: 90% reduction in boilerplate code while maintaining full functionality.

## Testing

The helpers package includes comprehensive unit tests. Run them with:

```bash
go test ./pkg/helpers -v
```

Coverage: 73.6% of statements (includes context-aware dialing implementation)

## License

BSD 3-Clause License - See LICENSE file for details

# Client Example - Bine + go-tor Integration

This example demonstrates how to use `cretz/bine` together with `go-tor` for Tor client operations.

## What This Example Shows

1. **Starting go-tor**: Initialize a pure-Go Tor client
2. **SOCKS5 Proxy**: Use go-tor's SOCKS proxy for network connections
3. **HTTP Requests**: Make HTTP requests through the Tor network
4. **Bine Integration**: Optional integration with bine for advanced features

## Running the Example

```bash
# Install dependencies
go mod download

# Run the example
go run main.go
```

## Expected Output

```
=== Bine + go-tor Client Integration Example ===

Step 1: Starting go-tor client...
  Waiting for Tor circuits to be ready (this may take 30-90 seconds)...
✓ go-tor client ready on socks5://127.0.0.1:9050

Step 2: Making HTTP request through go-tor SOCKS proxy...
  Making request to check.torproject.org...
  ✓ Request successful! Status: 200 OK
  Response: {"IsTor":true,"IP":"xxx.xxx.xxx.xxx"}

Step 3: Demonstrating bine integration pattern...
  Bine Integration Pattern:

  Option 1: Use go-tor as primary SOCKS proxy (shown above)
    - Pure Go implementation
    - No external Tor binary needed
    - Configure your app to use: 127.0.0.1:9050

  Option 2: Use bine to start a separate Tor instance
    - Requires Tor binary to be installed
    - Full control protocol support
    - Example shown below...

All examples completed successfully!
Press Ctrl+C to exit...
```

## Code Walkthrough

### 1. Starting go-tor Client

```go
torClient, err := client.Connect()
if err != nil {
    log.Fatalf("Failed to start go-tor: %v", err)
}
defer torClient.Close()

// Wait for circuits to be ready
torClient.WaitUntilReady(90 * time.Second)
```

### 2. Making HTTP Requests Through SOCKS5

```go
// Get SOCKS proxy address
proxyAddr := torClient.ProxyAddr() // e.g., "127.0.0.1:9050"

// Create SOCKS5 dialer
dialer, _ := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)

// Create HTTP client
httpClient := &http.Client{
    Transport: &http.Transport{
        Dial: dialer.Dial,
    },
}

// Make requests through Tor
resp, _ := httpClient.Get("https://check.torproject.org/api/ip")
```

### 3. Using with Bine (Optional)

If you have the Tor binary installed, you can also use bine:

```go
// Start bine Tor instance
ctx := context.Background()
t, err := tor.Start(ctx, nil)
if err != nil {
    log.Fatal(err)
}
defer t.Close()

// Now you can use bine's features while go-tor provides connectivity
```

## Integration Patterns

### Pattern 1: go-tor as Primary (Recommended for Pure Go)

**Advantages:**
- No external Tor binary required
- Pure Go implementation
- Easy cross-compilation
- Smaller deployment footprint

**Use when:**
- You want a pure-Go solution
- You're deploying to embedded systems
- You want minimal dependencies

```go
// Just use go-tor
torClient, _ := client.Connect()
defer torClient.Close()
torClient.WaitUntilReady(90 * time.Second)

// Use the SOCKS proxy
proxyURL, _ := url.Parse(torClient.ProxyURL())
httpClient := &http.Client{
    Transport: &http.Transport{
        Proxy: http.ProxyURL(proxyURL),
    },
}
```

### Pattern 2: Bine for Advanced Features

**Advantages:**
- Full Tor control protocol support
- Mature and well-tested
- Easier onion service management

**Use when:**
- You need advanced control protocol features
- You're creating hidden services
- You need mature, battle-tested Tor integration

```go
// Use bine's features while potentially using go-tor for connectivity
ctx := context.Background()
t, _ := tor.Start(ctx, nil)
defer t.Close()

// Create hidden service
onion, _ := t.Listen(ctx, &tor.ListenConf{RemotePorts: []int{80}})
```

### Pattern 3: Hybrid Approach

**Advantages:**
- Best of both worlds
- go-tor for connectivity
- bine for service management

**Use when:**
- You want pure-Go connectivity
- You also need advanced Tor features
- You're willing to manage two libraries

```go
// Start go-tor for connectivity
torClient, _ := client.Connect()
defer torClient.Close()

// Use bine for hidden services
t, _ := tor.Start(ctx, nil)
onion, _ := t.Listen(ctx, &tor.ListenConf{RemotePorts: []int{80}})
```

## Dependencies

This example requires:
- `github.com/opd-ai/go-tor` - Pure Go Tor implementation
- `github.com/cretz/bine` - Tor control and management library
- `golang.org/x/net/proxy` - SOCKS5 proxy support

Optional (for bine Tor instance):
- Tor binary installed on your system

## Troubleshooting

### "Connection refused"
Make sure go-tor is fully started:
```go
torClient.WaitUntilReady(90 * time.Second)
```

### "Tor binary not found"
This is expected if Tor isn't installed. The example works fine with just go-tor for basic connectivity.

### Slow startup
First time connecting to Tor can take 30-90 seconds. Subsequent starts are faster.

## Next Steps

- See `../hidden-service-example/` for creating onion services
- Check `../../http-helpers-demo/` for easier HTTP integration
- Read the main README in `../` for more integration patterns

# All-in-One Example - Complete Bine + go-tor Integration

This example demonstrates the complete integration of `cretz/bine` with `go-tor`, showcasing both client and hidden service functionality working together.

## What This Example Shows

This is a comprehensive demonstration that combines:

1. **go-tor Client**: Pure-Go Tor connectivity providing a SOCKS5 proxy
2. **Bine Hidden Service**: v3 onion service created with bine
3. **Service Access**: Accessing the hidden service through go-tor's proxy
4. **Complete Lifecycle**: Full startup, operation, and shutdown flow

## Why This Pattern?

This integration gives you the best of both worlds:

- **go-tor** for client connectivity (pure Go, no external Tor binary needed for client)
- **bine** for hidden service management (convenient API, battle-tested)
- Both libraries work together seamlessly

## Prerequisites

- Go 1.24 or later
- Tor binary installed (for bine's hidden service):
  - Ubuntu/Debian: `sudo apt-get install tor`
  - macOS: `brew install tor`
  - Windows: Download from [torproject.org](https://www.torproject.org/download/)

## Running the Example

```bash
# Install dependencies
go mod download

# Run the example
go run main.go
```

## Expected Output

```
=== All-in-One: Bine + go-tor Integration ===

This example demonstrates the complete integration:
  1. go-tor for client connectivity (pure Go)
  2. bine for hidden service management
  3. Accessing the service through go-tor

Step 1: Starting go-tor client...
  Waiting for Tor circuits (30-90 seconds)...
✓ go-tor client ready on 127.0.0.1:9050

Step 2: Creating hidden service with bine...
  (This requires Tor binary to be installed)
  Starting bine Tor instance...
  Creating onion service (2-3 minutes)...
✓ Hidden service created: http://abc123xyz456.onion

Step 3: Accessing hidden service through go-tor...
  ✓ Successfully accessed service!
  Response: {"status":"healthy","integration":"active"}

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
ALL SERVICES RUNNING
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✓ go-tor Client (Pure Go)
  SOCKS Proxy: 127.0.0.1:9050

✓ Bine Hidden Service
  Onion Address: http://abc123xyz456.onion

🌐 Access the service:
  1. Via Tor Browser:
     http://abc123xyz456.onion

  2. Via curl (using go-tor's proxy):
     curl --socks5 127.0.0.1:9050 http://abc123xyz456.onion

  3. Via any app configured to use the SOCKS proxy

📋 API Endpoints:
  http://abc123xyz456.onion/         - Home page
  http://abc123xyz456.onion/api      - JSON API
  http://abc123xyz456.onion/health   - Health check

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Press Ctrl+C to exit...
```

## What's Happening

### Architecture

```
┌─────────────────────────────────────────────┐
│           Your Application                   │
│                                               │
│  ┌──────────────┐      ┌─────────────────┐  │
│  │   go-tor     │      │  bine (Tor)     │  │
│  │  (Client)    │      │ (Hidden Service)│  │
│  │              │      │                 │  │
│  │ SOCKS Proxy  │◄────▶│ Onion Service   │  │
│  │ Pure Go      │      │ HTTP Server     │  │
│  └──────────────┘      └─────────────────┘  │
│         │                      │             │
└─────────┼──────────────────────┼─────────────┘
          │                      │
          ▼                      ▼
    ┌─────────────────────────────────┐
    │        Tor Network               │
    │  - Directory servers             │
    │  - HSDir nodes                   │
    │  - Rendezvous points            │
    └─────────────────────────────────┘
```

### Step-by-Step Flow

1. **go-tor starts**: Bootstraps to Tor network, creates circuits, starts SOCKS proxy
2. **bine starts**: Launches separate Tor process for hidden service management
3. **Hidden service created**: bine creates v3 onion service, publishes descriptor
4. **Self-test**: Example accesses its own hidden service through go-tor's SOCKS proxy
5. **Ready**: Both services running and integrated

## Accessing the Hidden Service

### Method 1: Tor Browser

1. Download Tor Browser: https://www.torproject.org/download/
2. Open Tor Browser
3. Navigate to the `.onion` address shown in the output
4. You'll see the welcome page!

### Method 2: Command Line (via go-tor's proxy)

```bash
# Health check
curl --socks5 127.0.0.1:9050 http://your-address.onion/health

# Get API response
curl --socks5 127.0.0.1:9050 http://your-address.onion/api

# View home page
curl --socks5 127.0.0.1:9050 http://your-address.onion/
```

### Method 3: Custom Go Application

```go
package main

import (
    "net/http"
    "golang.org/x/net/proxy"
)

func main() {
    // Connect through go-tor's SOCKS proxy
    dialer, _ := proxy.SOCKS5("tcp", "127.0.0.1:9050", nil, proxy.Direct)
    
    client := &http.Client{
        Transport: &http.Transport{
            Dial: dialer.Dial,
        },
    }
    
    // Access the hidden service
    resp, _ := client.Get("http://your-address.onion/api")
    // ... process response
}
```

## Code Walkthrough

### Starting go-tor

```go
torClient, err := client.Connect()
if err != nil {
    log.Fatal(err)
}
defer torClient.Close()

// Wait for circuits
torClient.WaitUntilReady(90 * time.Second)

// Get proxy address
proxyAddr := torClient.ProxyAddr() // "127.0.0.1:9050"
```

### Creating Hidden Service with Bine

```go
// Start bine Tor instance
t, err := tor.Start(ctx, nil)
if err != nil {
    log.Fatal(err)
}
defer t.Close()

// Create v3 onion service
conf := &tor.ListenConf{
    RemotePorts: []int{80},
    Version3:    true,
}

onion, err := t.Listen(ctx, conf)
if err != nil {
    log.Fatal(err)
}

// Get the onion address
onionAddr := fmt.Sprintf("%v.onion", onion.ID)
```

### Serving HTTP on the Onion Service

```go
mux := http.NewServeMux()
mux.HandleFunc("/", handleRoot)
mux.HandleFunc("/api", handleAPI)

srv := &http.Server{Handler: mux}
go srv.Serve(onion)
```

### Accessing Through go-tor

```go
// Create SOCKS5 dialer using go-tor
dialer, _ := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)

// Create HTTP client
httpClient := &http.Client{
    Transport: &http.Transport{
        Dial: dialer.Dial,
    },
}

// Access the hidden service
resp, _ := httpClient.Get("http://" + onionAddr + "/health")
```

## Integration Benefits

### Pure Go Client + Managed Service

**Client Side (go-tor)**:
- ✓ Pure Go implementation
- ✓ No external Tor binary needed
- ✓ Easy cross-compilation
- ✓ Low resource usage
- ✓ Embedded in your app

**Service Side (bine)**:
- ✓ Convenient API
- ✓ Battle-tested
- ✓ Easy service management
- ✓ Full control protocol support
- ✓ Mature hidden service implementation

### When to Use This Pattern

**Perfect for**:
- Applications that need both client and service functionality
- Wanting pure-Go client but proven service management
- Hybrid deployments (some nodes client, some service)
- Development and testing scenarios

**Consider alternatives if**:
- You only need client OR service (use just one library)
- You want 100% pure Go (use go-tor for both)
- You want bine for everything (use only bine)

## Performance Characteristics

- **Startup time**: 
  - go-tor client: 30-90 seconds
  - bine hidden service: 2-3 minutes
  - Total: ~3-4 minutes first run
- **Memory usage**: 
  - go-tor: ~50MB
  - bine Tor process: ~30-50MB
  - Total: ~80-100MB
- **CPU usage**: Low after initialization

## Troubleshooting

### "Tor binary not found"
Install Tor:
```bash
sudo apt-get install tor  # Ubuntu/Debian
brew install tor          # macOS
```

### Service not accessible
- Wait 2-3 minutes for service to be published
- Check both services are running
- Verify SOCKS proxy is working: `curl --socks5 127.0.0.1:9050 https://check.torproject.org`

### Port conflicts
If ports are in use, kill other Tor instances:
```bash
pkill -9 tor
```

### Slow performance
This is normal for Tor:
- Hidden services have higher latency
- 6-8 hops total (client → service)
- Use for anonymity, not speed

## Production Considerations

### Service Keys
Save and reuse hidden service keys:
```go
// Save key from first run
savedKey := onion.Key

// Reuse in future runs
conf := &tor.ListenConf{
    RemotePorts: []int{80},
    Version3:    true,
    Key:         savedKey, // Same address every time
}
```

### Monitoring
Monitor both components:
- go-tor circuit count
- bine service health
- Connection metrics
- Error rates

### Scaling
For multiple services:
- One go-tor client can serve multiple apps
- Create multiple hidden services with bine
- Load balance at application layer

### Security
- Use HTTPS even over Tor (defense in depth)
- Implement rate limiting
- Validate all inputs
- Keep both libraries updated
- Monitor for suspicious activity

## Next Steps

- Explore `../client-example/` for client-only patterns
- Check `../hidden-service-example/` for service-only patterns
- Read go-tor's `onion-service-demo` for native go-tor services
- Review bine documentation for advanced features

## References

- [go-tor Documentation](https://github.com/opd-ai/go-tor)
- [cretz/bine Documentation](https://pkg.go.dev/github.com/cretz/bine)
- [Tor Hidden Service Protocol](https://spec.torproject.org/rend-spec-v3)
- [Tor Project](https://www.torproject.org/)

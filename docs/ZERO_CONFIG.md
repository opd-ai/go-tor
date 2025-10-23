# Zero-Configuration Quick Start

This guide shows you how to use go-tor with absolutely no configuration needed.

## CLI Usage

### Installation

```bash
git clone https://github.com/opd-ai/go-tor.git
cd go-tor
make build
```

### Run (No Configuration Needed!)

Just run the binary:

```bash
./bin/tor-client
```

That's it! The client will:
1. Auto-detect the appropriate data directory for your OS
2. Create necessary directories with secure permissions
3. Connect to Tor network
4. Build circuits
5. Start SOCKS5 proxy on port 9050

**Output:**
```
[INFO] Using zero-configuration mode
[INFO] Data directory: /home/user/.config/go-tor
time=2025-10-19T... level=INFO msg="Starting go-tor" version=... 
time=2025-10-19T... level=INFO msg="Initializing Tor client..."
time=2025-10-19T... level=INFO msg="Bootstrapping Tor network connection..."
time=2025-10-19T... level=INFO msg="This may take up to 90 seconds on first run (consensus download + circuits)"
time=2025-10-19T... level=INFO msg="✓ Connected to Tor network" bootstrap_time=45s active_circuits=3
time=2025-10-19T... level=INFO msg="✓ SOCKS proxy available" address="127.0.0.1:9050"

Example: Test with curl
  curl --socks5 127.0.0.1:9050 https://check.torproject.org

Press Ctrl+C to exit
```

### Custom Options (Optional)

You can still customize if needed:

```bash
# Custom SOCKS port
./bin/tor-client -socks-port 9150

# Custom data directory
./bin/tor-client -data-dir ~/.tor

# Verbose logging
./bin/tor-client -log-level debug
```

## Library Usage

### Minimal Example (3 Lines!)

```go
package main

import (
    "log"
    "github.com/opd-ai/go-tor/pkg/client"
)

func main() {
    // One line to connect!
    tor, err := client.Connect()
    if err != nil {
        log.Fatal(err)
    }
    defer tor.Close()
    
    // Use the proxy
    proxyURL := tor.ProxyURL() // "socks5://127.0.0.1:9050"
    
    // Your code here...
}
```

### With HTTP Client

```go
package main

import (
    "fmt"
    "io"
    "log"
    "net/http"
    "net/url"
    "time"
    
    "github.com/opd-ai/go-tor/pkg/client"
)

func main() {
    // Connect to Tor
    tor, err := client.Connect()
    if err != nil {
        log.Fatal(err)
    }
    defer tor.Close()
    
    // Wait for circuits
    tor.WaitUntilReady(60 * time.Second)
    
    // Configure HTTP client to use Tor
    proxyURL, _ := url.Parse(tor.ProxyURL())
    httpClient := &http.Client{
        Transport: &http.Transport{
            Proxy: http.ProxyURL(proxyURL),
        },
        Timeout: 30 * time.Second,
    }
    
    // Make request through Tor
    resp, err := httpClient.Get("https://check.torproject.org")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()
    
    body, _ := io.ReadAll(resp.Body)
    fmt.Println(string(body))
}
```

### With Custom Options

```go
package main

import (
    "log"
    "github.com/opd-ai/go-tor/pkg/client"
)

func main() {
    // Custom options
    tor, err := client.ConnectWithOptions(&client.Options{
        SocksPort:     9150,
        ControlPort:   9151,
        DataDirectory: "/custom/path",
        LogLevel:      "debug",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer tor.Close()
    
    // Use the proxy
    proxyURL := tor.ProxyURL()
    // ...
}
```

## Platform-Specific Data Directories

go-tor automatically detects the appropriate directory for your OS:

- **Linux**: `~/.config/go-tor`
- **macOS**: `~/Library/Application Support/go-tor`
- **Windows**: `%APPDATA%\go-tor`

All directories are created with secure permissions (700 on Unix systems).

## What Happens on First Run?

1. **Directory Creation**: Auto-detects and creates data directory
2. **Network Bootstrap**: Fetches consensus documents (~5-15 seconds)
3. **Circuit Building**: Establishes 3 circuits (~30-60 seconds)
4. **Ready**: SOCKS5 proxy available for use

Total time: Up to 90 seconds on first run (consensus download + circuit building), faster on subsequent runs.

## Checking Connection

Test your Tor connection:

```bash
# With curl
curl --socks5 127.0.0.1:9050 https://check.torproject.org

# Check your IP
curl --socks5 127.0.0.1:9050 https://api.ipify.org
```

## Examples

See working examples in:
- [`examples/zero-config/`](../examples/zero-config/) - Minimal usage
- [`examples/zero-config-custom/`](../examples/zero-config-custom/) - With options

Build and run:
```bash
cd examples/zero-config
go build
./zero-config
```

## FAQ

**Q: Do I need to install Tor separately?**  
A: No! This is a pure Go implementation. No external dependencies.

**Q: Where are my circuits/keys stored?**  
A: In your platform-specific data directory (see above).

**Q: Can I use a config file?**  
A: Yes! Use `-config` flag: `./tor-client -config torrc`

**Q: How do I change the SOCKS port?**  
A: Use `-socks-port` flag or `Options.SocksPort` in code.

**Q: Is this production-ready?**  
A: The project is in active development. See [security notice](../README.md#security) in main README.

## Troubleshooting

**Port already in use:**
```bash
# Use different port
./bin/tor-client -socks-port 9150
```

**Permission denied (data directory):**
```bash
# Specify writable directory
./bin/tor-client -data-dir ./tor-data
```

**Connection timeout:**
- Check network connectivity
- Try again (Tor network may be slow)
- Increase timeout in code: `ConnectWithContext(ctx)` with longer context

## Next Steps

- Read the [full documentation](../README.md)
- Explore [examples](../examples/)
- Check the [API reference](../docs/API.md)
- Learn about [security considerations](../AUDIT_SUMMARY.md)

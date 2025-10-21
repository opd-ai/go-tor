# Hidden Service Example - Bine Onion Service

This example demonstrates how to create and manage a v3 onion service (hidden service) using `cretz/bine`.

## What This Example Shows

1. **v3 Onion Service**: Create a modern Tor hidden service
2. **HTTP Server**: Serve web content over the onion network
3. **Service Lifecycle**: Manage the complete lifecycle from creation to shutdown
4. **Security Features**: End-to-end encryption, location hiding, NAT traversal

## Prerequisites

⚠️ **IMPORTANT**: This example requires the Tor binary to be installed on your system.

### Installing Tor

**Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install tor
```

**macOS:**
```bash
brew install tor
```

**Windows:**
Download and install from [https://www.torproject.org/download/](https://www.torproject.org/download/)

### Verify Installation

```bash
which tor
# Should output: /usr/bin/tor or similar

tor --version
# Should output: Tor version x.x.x
```

## Running the Example

```bash
# Install dependencies
go mod download

# Run the example
go run main.go
```

## Expected Output

```
=== Bine Hidden Service Example ===

This example demonstrates creating a v3 onion service using cretz/bine.
The onion service will host a simple HTTP server.

Checking for Tor binary...
✓ Tor binary found

Step 1: Starting Tor (this may take 30-60 seconds)...
✓ Tor started successfully

Step 2: Creating v3 onion service...
  This may take 2-3 minutes as the service is published to the network...

✓ Onion service created successfully!
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Onion Address: http://abc123xyz456def789.onion
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Step 3: Starting HTTP server on the onion service...
✓ HTTP server started

SERVICE INFORMATION:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Your onion service is now online and accessible!

🌐 Access the service:
   http://abc123xyz456def789.onion

📝 Available endpoints:
   http://abc123xyz456def789.onion/          - Home page
   http://abc123xyz456def789.onion/api       - JSON API
   http://abc123xyz456def789.onion/health    - Health check

🔐 Security features:
   ✓ End-to-end encryption
   ✓ Hidden server location
   ✓ Self-authenticating address
   ✓ NAT traversal (no port forwarding)

📱 How to access:
   1. Use Tor Browser: https://www.torproject.org/download/
   2. Or any application configured to use Tor SOCKS proxy

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Press Ctrl+C to stop the service...
```

## Code Walkthrough

### 1. Starting Tor

```go
ctx := context.Background()
t, err := tor.Start(ctx, nil)
if err != nil {
    log.Fatal(err)
}
defer t.Close()
```

### 2. Creating the Onion Service

```go
// Configure the service
conf := &tor.ListenConf{
    RemotePorts: []int{80},  // Port clients connect to
    Version3:    true,        // Use v3 (recommended)
}

// Create the service
onion, err := t.Listen(ctx, conf)
if err != nil {
    log.Fatal(err)
}
defer onion.Close()

// Get the onion address
fmt.Printf("Address: http://%v.onion\n", onion.ID)
```

### 3. Serving HTTP Content

```go
// Create HTTP server
mux := http.NewServeMux()
mux.HandleFunc("/", handler)

srv := &http.Server{Handler: mux}

// Serve on the onion service
srv.Serve(onion)
```

## Accessing Your Onion Service

### Option 1: Tor Browser (Easiest)

1. Download Tor Browser: https://www.torproject.org/download/
2. Open Tor Browser
3. Navigate to your `.onion` address
4. You should see the welcome page!

### Option 2: Command Line with cURL

```bash
# Using Tor's SOCKS proxy (default port 9050)
curl --socks5 127.0.0.1:9050 http://your-onion-address.onion
```

### Option 3: Using go-tor Client

See the `client-example` directory for how to access your service using go-tor.

## Features Demonstrated

### HTTP Endpoints

The example creates three endpoints:

**1. Home Page (`/`)**
- HTML welcome page
- Service information
- Feature overview

**2. API Endpoint (`/api`)**
```bash
curl --socks5 127.0.0.1:9050 http://your-address.onion/api
```
Returns JSON:
```json
{
  "service": "Bine Hidden Service",
  "onion_address": "your-address.onion",
  "status": "online",
  "timestamp": "2024-01-01T00:00:00Z",
  "features": ["hidden", "encrypted", "authenticated"]
}
```

**3. Health Check (`/health`)**
```bash
curl --socks5 127.0.0.1:9050 http://your-address.onion/health
```
Returns:
```json
{"status":"healthy","service":"online"}
```

## Understanding v3 Onion Services

### What is an Onion Service?

An onion service (formerly "hidden service") allows you to host a service on the Tor network that:
- **Hides your location**: No one knows where your server is
- **Encrypts traffic**: End-to-end encryption by default
- **Bypasses NAT**: No port forwarding required
- **Self-authenticating**: The `.onion` address proves identity

### v3 vs v2 Onion Services

| Feature | v2 (Deprecated) | v3 (Current) |
|---------|----------------|--------------|
| Address length | 16 characters | 56 characters |
| Cryptography | RSA-1024 | Ed25519 |
| Encryption | Optional | Always-on |
| Security | Weak | Strong |
| Status | Deprecated | Recommended |

**Always use v3** (Version3: true in the code).

### How It Works

```
1. Your Server
   └─> Creates onion service with bine
   └─> Publishes descriptor to HSDirs

2. Tor Network
   └─> Stores service descriptor
   └─> Facilitates connections via rendezvous points

3. Client
   └─> Looks up service descriptor
   └─> Connects through rendezvous point
   └─> End-to-end encrypted connection established
```

## Security Considerations

### ✅ What Onion Services Provide

- **Location hiding**: Your server's IP address is hidden
- **End-to-end encryption**: All traffic is encrypted
- **NAT traversal**: Works behind firewalls without configuration
- **Self-authentication**: Address cryptographically proves identity

### ⚠️ What They Don't Provide

- **Content security**: Still need HTTPS for application-layer security
- **DDoS protection**: Can still be targeted
- **Anonymity guarantee**: Behavior/traffic analysis can de-anonymize
- **Performance**: Slower than direct connections

### Best Practices

1. **Use HTTPS**: Even over Tor, use HTTPS for defense in depth
2. **Rate limiting**: Implement rate limiting to prevent abuse
3. **Input validation**: Sanitize all user inputs
4. **Key management**: Protect your onion service private key
5. **Monitor logs**: Watch for suspicious activity
6. **Keep updated**: Update Tor regularly for security patches

## Production Deployment

For production use, consider:

### 1. Persistent Keys

Save and reuse your onion service keys:
```go
conf := &tor.ListenConf{
    RemotePorts: []int{80},
    Version3:    true,
    Key:         savedKey, // Reuse existing key
}
```

### 2. Multiple Introduction Points

For reliability:
```go
conf := &tor.ListenConf{
    RemotePorts:       []int{80, 443},
    Version3:          true,
    NumIntroPoints:    3, // Default, can be 1-10
}
```

### 3. Service Monitoring

Monitor service health:
- Check descriptor publication
- Monitor connection counts
- Track error rates
- Alert on downtime

### 4. Backup and Recovery

- Backup private keys securely
- Document recovery procedures
- Test restore process regularly

## Troubleshooting

### Service Creation Takes Too Long

- First-time creation: 2-5 minutes is normal
- Check Tor logs for issues
- Ensure good network connectivity
- Try increasing timeout

### Service Not Accessible

- Verify service is running: check logs
- Test with Tor Browser first
- Ensure client is using Tor correctly
- Check firewall settings (though usually not needed)

### "Tor binary not found"

```bash
# Verify Tor is installed
which tor

# Install if missing
sudo apt-get install tor  # Ubuntu/Debian
brew install tor          # macOS
```

### Port Already in Use

Bine automatically selects a local port. If you see errors:
- Check for other Tor instances
- Restart the example
- Check system logs

## Performance Notes

- **Startup time**: 30-60 seconds for Tor, 2-3 minutes for service
- **Latency**: Higher than clearnet (6-8 hops total)
- **Bandwidth**: Slower than direct connections
- **Reliability**: Depends on Tor network health

## Advanced Configuration

### Custom Data Directory

```go
conf := &tor.StartConf{
    DataDir: "/var/lib/tor-myservice",
}
t, _ := tor.Start(ctx, conf)
```

### Multiple Ports

```go
conf := &tor.ListenConf{
    RemotePorts: []int{80, 443, 8080},
    Version3:    true,
}
```

### Service Authentication (Client Auth)

For restricted access, see bine documentation on client authorization.

## Integration with go-tor

To use go-tor's pure-Go implementation alongside bine:

```go
// Start go-tor for connectivity
torClient, _ := client.Connect()
defer torClient.Close()

// Use bine for hidden service management
t, _ := tor.Start(ctx, nil)
onion, _ := t.Listen(ctx, conf)

// Both work together!
```

## Next Steps

- See `../client-example/` for accessing onion services
- Check go-tor's `onion-service-demo` for native go-tor hidden services
- Read Tor Project documentation: https://community.torproject.org/onion-services/

## References

- [Tor v3 Onion Services Spec](https://spec.torproject.org/rend-spec-v3)
- [cretz/bine Documentation](https://pkg.go.dev/github.com/cretz/bine)
- [Tor Project - Onion Services](https://community.torproject.org/onion-services/)

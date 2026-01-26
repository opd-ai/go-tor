# BridgeDB Integration (Educational)

**⚠️ WARNING: Educational Implementation Only**

This document describes the BridgeDB integration in go-tor, implemented for educational and research purposes only. **This should NOT be used for production bridge distribution or real anonymity needs.**

## Overview

BridgeDB is Tor's bridge distribution system that helps users obtain bridge addresses while preventing enumeration by censors. This implementation demonstrates the core concepts:

- Bridge database management
- Multi-channel distribution (HTTP, email simulation)
- Rate limiting and anti-enumeration measures
- Transport type filtering

## Architecture

### Components

1. **BridgeDistributor** (`pkg/relay/bridgedb.go`)
   - Core bridge database and distribution logic
   - Rate limiting per IP address
   - Deterministic bridge selection
   - Transport filtering

2. **BridgeDistributorServer** (`pkg/relay/bridgedb.go`)
   - HTTP API for bridge distribution
   - RESTful endpoints for bridge requests
   - Statistics reporting

3. **EmailResponder** (`pkg/relay/bridgedb.go`)
   - Simulated email-based bridge distribution
   - Bridge line formatting
   - Rate limiting per email address

## Usage

### Creating a Bridge Distributor

```go
import (
    "github.com/opd-ai/go-tor/pkg/logger"
    "github.com/opd-ai/go-tor/pkg/relay"
)

// Create logger
log := logger.New(slog.LevelInfo, os.Stdout)

// Create distributor with default config
config := relay.DefaultDistributorConfig()
distributor := relay.NewBridgeDistributor(config, log)
```

### Adding Bridges

```go
bridge := &relay.BridgeInfo{
    Fingerprint: "A1B2C3D4E5F6A7B8C9D0E1F2A3B4C5D6E7F8A9B0",
    Address:     "192.0.2.1:9001",
    Transport:   "obfs4",
    Params:      "cert=abcd1234;iat-mode=0",
}

err := distributor.AddBridge(bridge)
if err != nil {
    log.Error("Failed to add bridge", "error", err)
}
```

### HTTP Distribution

```go
// Create HTTP server
server := relay.NewBridgeDistributorServer(distributor, log)

// Start server
http.ListenAndServe(":8080", server)
```

Available endpoints:
- `GET /bridges` - Get bridges (query params: `transport`, `count`)
- `GET /stats` - Get distribution statistics

### Email Responder

```go
// Create email responder
responder := relay.NewEmailResponder(distributor, log)

// Generate response for a user
response, err := responder.GenerateEmailResponse("user@example.com", "obfs4")
if err != nil {
    log.Error("Failed to generate response", "error", err)
}

fmt.Println(response)
```

## Configuration

```go
type DistributorConfig struct {
    RateLimitInterval time.Duration // Minimum time between requests (default: 1h)
}
```

Default configuration:
```go
config := relay.DefaultDistributorConfig()
// RateLimitInterval: 1 hour
```

## Rate Limiting

The distributor implements rate limiting to prevent bridge enumeration:

- **HTTP Distribution**: Rate limited per IP address
- **Email Distribution**: Rate limited per email address (via hash)
- **Default Interval**: 1 hour between requests

If a requestor exceeds the rate limit, they receive an error:
```
rate limit exceeded, try again later
```

## Bridge Selection

Bridge selection is deterministic based on requestor identity (IP or email hash):

1. Hash requestor identifier (IP address or email)
2. Use hash to select offset in bridge list
3. Return bridges starting from that offset

This ensures:
- Same requestor gets same bridges (within rate limit window)
- Different requestors get distributed across bridge pool
- No single bridge gets overloaded

## Transport Filtering

Supported transport types:
- `vanilla` - Standard Tor bridge (no pluggable transport)
- `obfs4` - obfs4 obfuscation
- `meek_lite` - Meek domain fronting
- `snowflake` - Snowflake WebRTC transport
- Any other custom transport

Filter by transport:
```go
bridges, err := distributor.GetBridges("198.51.100.1", "obfs4", 3)
```

## Statistics

Get distribution statistics:
```go
stats := distributor.GetStats()
// Returns: map[string]interface{}{
//   "total_bridges": 10,
//   "by_transport": map[string]int{
//     "vanilla": 2,
//     "obfs4": 6,
//     "meek_lite": 2,
//   }
// }
```

## Security Considerations

### Educational Implementation

This implementation is **NOT suitable for production use** because:

1. **No Persistent Storage**: Bridges are only stored in memory
2. **Simple Rate Limiting**: Rate limiter uses in-memory map (no persistence)
3. **No Captcha**: No protection against automated enumeration beyond rate limiting
4. **No Geographic Distribution**: No consideration of user location
5. **Simplified Security**: No advanced anti-enumeration measures

### For Production Use

Real BridgeDB implementations require:

- Persistent bridge database
- Multiple distribution strategies (HTTP, Email, Moat)
- Captcha integration
- Geographic and transport-aware distribution
- Bridge reachability testing
- Integration with bridge authorities
- Advanced anti-enumeration heuristics

## API Reference

### BridgeInfo

```go
type BridgeInfo struct {
    Fingerprint string    // Bridge identity fingerprint (40 hex chars)
    Address     string    // IP:Port
    Transport   string    // Transport type (vanilla, obfs4, etc.)
    Params      string    // Transport-specific parameters
    AddedAt     time.Time // When bridge was added
}
```

### BridgeDistributor

```go
// Create distributor
func NewBridgeDistributor(config DistributorConfig, log *logger.Logger) *BridgeDistributor

// Manage bridges
func (bd *BridgeDistributor) AddBridge(info *BridgeInfo) error
func (bd *BridgeDistributor) RemoveBridge(fingerprint string)

// Distribute bridges
func (bd *BridgeDistributor) GetBridges(requestorIP string, transport string, count int) ([]*BridgeInfo, error)

// Statistics
func (bd *BridgeDistributor) GetStats() map[string]interface{}
```

### BridgeDistributorServer

```go
// Create HTTP server
func NewBridgeDistributorServer(distributor *BridgeDistributor, log *logger.Logger) *BridgeDistributorServer

// Implements http.Handler
func (s *BridgeDistributorServer) ServeHTTP(w http.ResponseWriter, r *http.Request)
```

### EmailResponder

```go
// Create email responder
func NewEmailResponder(distributor *BridgeDistributor, log *logger.Logger) *EmailResponder

// Generate email response
func (er *EmailResponder) GenerateEmailResponse(senderEmail string, transport string) (string, error)
```

## Example

See `examples/bridgedb-demo/` for a complete working example.

## Testing

Comprehensive test coverage in `pkg/relay/bridgedb_test.go`:

```bash
go test ./pkg/relay -run TestBridge -v
```

Tests cover:
- Bridge addition and removal
- Rate limiting behavior
- Transport filtering
- HTTP API endpoints
- Email response generation
- Statistics tracking

## References

- [BridgeDB Specification](https://gitlab.torproject.org/tpo/anti-censorship/bridgedb)
- [Bridge Distribution Documentation](https://tb-manual.torproject.org/bridges/)
- [Tor Bridge Specification](https://spec.torproject.org/bridge-spec)

## Disclaimer

**This is an educational implementation only.** For real bridge distribution:

- **Production Use**: Deploy official BridgeDB from The Tor Project
- **User Access**: Direct users to https://bridges.torproject.org/
- **Research**: Study official implementation at https://gitlab.torproject.org/tpo/anti-censorship/bridgedb

This implementation is provided solely for learning about bridge distribution mechanisms and should never be used for actual censorship circumvention.

# Stream Isolation

**Note:** Stream isolation and circuit isolation refer to the same feature in go-tor. Both terms are used interchangeably in the Tor ecosystem.

For complete documentation on stream/circuit isolation, please see:

## [→ Circuit Isolation Documentation](./CIRCUIT_ISOLATION.md)

This document covers:

- **Isolation Levels**: None, Destination, Credential, Port, Session
- **Configuration**: How to enable and configure isolation
- **API Usage**: Direct API usage for custom implementations
- **SOCKS5 Integration**: Automatic isolation via SOCKS5 proxy
- **Performance Considerations**: Memory and latency impact
- **Security Model**: Protections and limitations
- **Examples**: Complete working examples

## Quick Start

Enable stream isolation via configuration:

```go
import (
    "github.com/opd-ai/go-tor/pkg/config"
    "github.com/opd-ai/go-tor/pkg/client"
)

cfg := config.DefaultConfig()
cfg.IsolationLevel = "credential"  // or "destination", "port", "session"
cfg.IsolateSOCKSAuth = true

client, err := client.New(cfg, logger)
```

Or use it directly via SOCKS5:

```bash
# Each username gets isolated circuit
curl --socks5 alice:password@localhost:9050 https://example.com
curl --socks5 bob:password@localhost:9050 https://example.com
```

## Terminology

- **Stream Isolation**: Tor ecosystem term for preventing streams from sharing circuits
- **Circuit Isolation**: Implementation term for keeping circuits separate based on isolation keys
- Both achieve the same privacy goal: preventing correlation of different activities

For detailed documentation, examples, and API reference, see [CIRCUIT_ISOLATION.md](./CIRCUIT_ISOLATION.md).

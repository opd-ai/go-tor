# Configuration Guide

Comprehensive guide to configuring go-tor for various use cases.

## Table of Contents

- [Quick Start](#quick-start)
- [Configuration Files](#configuration-files)
- [Configuration Templates](#configuration-templates)
- [Configuration Options](#configuration-options)
- [Validation](#validation)
- [JSON Schema](#json-schema)
- [Advanced Topics](#advanced-topics)

## Quick Start

### Zero Configuration

The simplest way to get started - no configuration needed:

```bash
./bin/tor-client
```

go-tor will automatically:
- Detect and create appropriate data directories
- Select available ports
- Configure sensible defaults
- Connect to the Tor network

### Minimal Configuration

Create a minimal configuration file:

```bash
tor-config-validator -template minimal -output torrc
./bin/tor-client -config torrc
```

Or manually create `torrc`:

```ini
SocksPort 9050
ControlPort 9051
```

## Configuration Files

### File Format

go-tor uses a torrc-compatible configuration file format with key-value pairs:

```ini
# Comments start with #
SocksPort 9050
ControlPort 9051
LogLevel info

# Lists can be comma-separated
ExcludeNodes $BADNODE1, $BADNODE2

# Or multiple lines
ExcludeNodes $BADNODE3
ExcludeNodes $BADNODE4
```

### Loading Configuration

Three ways to load configuration:

**1. Command-line argument:**
```bash
tor-client -config /path/to/torrc
```

**2. Programmatically (Go API):**
```go
cfg := config.DefaultConfig()
err := config.LoadFromFile("torrc", cfg)
```

**3. Zero-config mode (uses defaults):**
```bash
tor-client
```

## Configuration Templates

Use the config-validator tool to generate template configurations:

### Available Templates

| Template | Use Case | Description |
|----------|----------|-------------|
| **minimal** | Getting started | Simplest working configuration |
| **production** | Production deployments | Performance tuning, monitoring, best practices |
| **development** | Local development | Debug logging, metrics, relaxed timeouts |
| **high-security** | Privacy-focused | Strict isolation, conservative settings |

### Generating Templates

```bash
# List available templates
tor-config-validator -list-templates

# Generate a specific template
tor-config-validator -template production -output torrc

# View template on stdout
tor-config-validator -template minimal
```

### Template Descriptions

#### Minimal Template
```bash
tor-config-validator -template minimal -output torrc
```
- Bare minimum settings
- Uses defaults for everything else
- Best for: Quick testing, simple use cases

#### Production Template
```bash
tor-config-validator -template production -output torrc.prod
```
- Performance-optimized settings
- Connection and circuit pooling enabled
- Monitoring endpoints configured
- Best for: Production deployments, high-traffic scenarios

#### Development Template
```bash
tor-config-validator -template development -output torrc.dev
```
- Debug logging enabled
- Metrics endpoint active
- Alternative ports (9150/9151/9152)
- Best for: Local development, debugging

#### High-Security Template
```bash
tor-config-validator -template high-security -output torrc.secure
```
- Strict circuit isolation
- Short circuit lifetimes
- Privacy over performance
- Best for: Maximum privacy (but see security notice below)

**⚠️ IMPORTANT SECURITY NOTICE:**
go-tor is an unofficial, experimental implementation. For real privacy and anonymity needs, use [Tor Browser](https://www.torproject.org/download/) or [official Tor software](https://www.torproject.org/).

## Configuration Options

### Network Settings

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `SocksPort` | integer | 9050 | SOCKS5 proxy port (0 to disable) |
| `ControlPort` | integer | 9051 | Control protocol port (0 to disable) |
| `DataDirectory` | string | (platform-specific) | Directory for persistent state |

Example:
```ini
SocksPort 9050
ControlPort 9051
DataDirectory /var/lib/go-tor
```

### Circuit Settings

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `CircuitBuildTimeout` | duration | 60s | Maximum time to build a circuit |
| `MaxCircuitDirtiness` | duration | 10m | Maximum circuit lifetime |
| `NewCircuitPeriod` | duration | 30s | Circuit rotation interval |
| `NumEntryGuards` | integer | 3 | Number of entry guards to use |

Example:
```ini
CircuitBuildTimeout 90s
MaxCircuitDirtiness 30m
NewCircuitPeriod 1m
NumEntryGuards 3
```

**Duration formats:**
- `30s` - 30 seconds
- `5m` - 5 minutes
- `2h` - 2 hours
- `1d` - 1 day (non-standard but supported)

### Path Selection

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `UseEntryGuards` | boolean | true | Use entry guards (recommended) |
| `UseBridges` | boolean | false | Use bridge relays |
| `BridgeAddresses` | list | [] | Bridge addresses (if UseBridges=true) |
| `ExcludeNodes` | list | [] | Nodes to exclude from paths |
| `ExcludeExitNodes` | list | [] | Exit nodes to exclude |

Example:
```ini
UseEntryGuards true
UseBridges false

# Exclude specific relays (by fingerprint or nickname)
ExcludeNodes $BADRELAY1, $BADRELAY2
ExcludeExitNodes $BADEXIT1

# Bridge configuration (for censored networks)
# UseBridges true
# BridgeAddresses obfs4 192.0.2.1:443, obfs4 192.0.2.2:443
```

### Connection Settings

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `ConnLimit` | integer | 1000 | Maximum concurrent connections |
| `DormantTimeout` | duration | 24h | Dormant mode timeout |

Example:
```ini
ConnLimit 1000
DormantTimeout 24h
```

### Performance Tuning

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `EnableConnectionPooling` | boolean | true | Connection pooling |
| `ConnectionPoolMaxIdle` | integer | 5 | Max idle pooled connections |
| `ConnectionPoolMaxLife` | duration | 10m | Max pooled connection lifetime |
| `EnableCircuitPrebuilding` | boolean | true | Prebuild circuits |
| `CircuitPoolMinSize` | integer | 2 | Minimum prebuilt circuits |
| `CircuitPoolMaxSize` | integer | 10 | Maximum circuits in pool |
| `EnableBufferPooling` | boolean | true | Buffer pooling |

Example:
```ini
EnableConnectionPooling true
ConnectionPoolMaxIdle 5
ConnectionPoolMaxLife 10m

EnableCircuitPrebuilding true
CircuitPoolMinSize 3
CircuitPoolMaxSize 15

EnableBufferPooling true
```

### Circuit Isolation

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `IsolationLevel` | enum | none | Isolation level (none, destination, credential, port, session) |
| `IsolateDestinations` | boolean | false | Isolate by destination |
| `IsolateSOCKSAuth` | boolean | false | Isolate by SOCKS username |
| `IsolateClientPort` | boolean | false | Isolate by client port |
| `IsolateClientProtocol` | boolean | false | Isolate by protocol |

Example:
```ini
# No isolation (better performance)
IsolationLevel none

# Destination-based isolation (better privacy)
# IsolationLevel destination
# IsolateDestinations true

# App-level isolation (via SOCKS username)
# IsolationLevel credential
# IsolateSOCKSAuth true
```

**Isolation Levels:**
- `none` - Share circuits (faster, less private)
- `destination` - Separate circuit per destination
- `credential` - Separate circuit per SOCKS username
- `port` - Separate circuit per client port
- `session` - Separate circuit per session ID

### Logging

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `LogLevel` | enum | info | Log verbosity (debug, info, warn, error) |

Example:
```ini
# Production: info or warn
LogLevel info

# Development: debug
# LogLevel debug

# Minimal: error
# LogLevel error
```

### Monitoring

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `EnableMetrics` | boolean | false | Enable HTTP metrics endpoint |
| `MetricsPort` | integer | 0 | Metrics server port (0=disabled) |

Example:
```ini
# Enable metrics for monitoring
EnableMetrics true
MetricsPort 9052
```

**Metrics Endpoints:**
- `http://localhost:9052/metrics` - Prometheus format
- `http://localhost:9052/metrics/json` - JSON format
- `http://localhost:9052/health` - Health check
- `http://localhost:9052/dashboard` - HTML dashboard

### Onion Services

Configure hidden services:

```ini
# First onion service
[[OnionServices]]
ServiceDir /var/lib/go-tor/service1
VirtualPort 80
TargetAddr localhost:8080
MaxStreams 0

# Second onion service
[[OnionServices]]
ServiceDir /var/lib/go-tor/service2
VirtualPort 443
TargetAddr localhost:8443
MaxStreams 100
```

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `ServiceDir` | string | yes | Service keys/state directory |
| `VirtualPort` | integer | yes | Advertised port |
| `TargetAddr` | string | yes | Local service address |
| `MaxStreams` | integer | no | Max concurrent streams (0=unlimited) |
| `ClientAuth` | map | no | Client authorization keys |

## Validation

### Using the Config Validator

```bash
# Validate a configuration file
tor-config-validator -config torrc

# Verbose validation with detailed feedback
tor-config-validator -config torrc -verbose
```

### Validation Output

**Valid configuration:**
```
✓ Configuration is valid
```

**Invalid configuration:**
```
Validation failed: validation error: invalid SocksPort: 99999

Errors:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✗ invalid port number: 99999
  → use a port between 0 and 65535 (0 to disable, 1024-65535 recommended for non-root)
```

**Configuration with warnings:**
```
✓ Configuration is valid

Warnings:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
⚠  using privileged port (< 1024)
  → consider using port >= 1024 to avoid requiring root privileges
```

### Programmatic Validation

```go
cfg := config.DefaultConfig()
// ... configure ...

// Simple validation
if err := cfg.Validate(); err != nil {
    log.Fatal(err)
}

// Detailed validation
result := cfg.ValidateDetailed()
if !result.Valid {
    for _, err := range result.Errors {
        fmt.Printf("Error: %s\n", err.Message)
        if err.Suggestion != "" {
            fmt.Printf("  → %s\n", err.Suggestion)
        }
    }
}

// Check warnings
for _, warn := range result.Warnings {
    fmt.Printf("Warning: %s\n", warn.Message)
}
```

## JSON Schema

### Generating the Schema

```bash
# Generate JSON schema
tor-config-validator -schema -output config-schema.json

# View schema on stdout
tor-config-validator -schema
```

### Using the Schema

**With VS Code:**

Add to `.vscode/settings.json`:
```json
{
  "json.schemas": [{
    "fileMatch": ["torrc", "*.torrc"],
    "url": "./config-schema.json"
  }]
}
```

**With JetBrains IDEs (IntelliJ, PyCharm, etc.):**

1. Settings → Languages & Frameworks → Schemas and DTDs → JSON Schema Mappings
2. Add new mapping
3. Schema file: `config-schema.json`
4. File pattern: `*.torrc`

**Benefits:**
- IDE autocomplete for configuration options
- Real-time validation
- Inline documentation
- Type checking

## Advanced Topics

### Port Selection

**Default ports:**
- SOCKS: 9050
- Control: 9051
- Metrics: 0 (disabled)

**Alternative ports (avoid conflicts with system Tor):**
```ini
SocksPort 9150
ControlPort 9151
MetricsPort 9152
```

**Disable a port:**
```ini
# Disable control protocol
ControlPort 0
```

**Privileged ports:**
Ports < 1024 require root privileges. Use ports ≥ 1024 instead:
```ini
# Bad: requires root
# SocksPort 80

# Good: no root required
SocksPort 8080
```

### Data Directory

**Platform defaults:**
- Linux: `~/.local/share/go-tor`
- macOS: `~/Library/Application Support/go-tor`
- Windows: `%APPDATA%\go-tor`

**Custom directory:**
```ini
# Relative path
DataDirectory ./tor-data

# Absolute path
DataDirectory /var/lib/go-tor

# User home directory
DataDirectory ~/.tor
```

**Permissions:**
```bash
# Create with restrictive permissions
mkdir -p /var/lib/go-tor
chmod 0700 /var/lib/go-tor
chown tor:tor /var/lib/go-tor
```

### Environment-Specific Configurations

#### Development

```ini
# Use alternative ports
SocksPort 9150
ControlPort 9151

# Enable metrics
EnableMetrics true
MetricsPort 9152

# Debug logging
LogLevel debug

# Faster circuit builds
CircuitBuildTimeout 45s
CircuitPoolMinSize 1
```

#### Production

```ini
# Standard ports
SocksPort 9050
ControlPort 9051

# Production logging
LogLevel info

# Monitoring
EnableMetrics true
MetricsPort 9052

# Performance tuning
EnableConnectionPooling true
EnableCircuitPrebuilding true
CircuitPoolMinSize 3
CircuitPoolMaxSize 15

# Reliability
CircuitBuildTimeout 90s
NumEntryGuards 5
```

#### High Security

```ini
# Strict isolation
IsolationLevel destination
IsolateDestinations true
IsolateSOCKSAuth true

# Short circuit lifetimes
MaxCircuitDirtiness 5m
NewCircuitPeriod 15s

# Conservative settings
NumEntryGuards 5
CircuitBuildTimeout 120s

# Minimal logging
LogLevel warn

# Disable metrics (no info leak)
EnableMetrics false
```

### Configuration Best Practices

1. **Start with a template** - Use `tor-config-validator -template` to generate a starting point
2. **Validate before use** - Always run `tor-config-validator -config` before deployment
3. **Use comments** - Document why you chose specific settings
4. **Version control** - Keep configuration files in version control
5. **Secure permissions** - Set DataDirectory to mode 0700
6. **Monitor metrics** - Enable metrics in production for observability
7. **Test thoroughly** - Test configuration changes in development first
8. **Document changes** - Keep a changelog for configuration updates

### Common Pitfalls

**Port conflicts:**
```ini
# BAD: All ports the same
SocksPort 9050
ControlPort 9050  # ← Conflict!
MetricsPort 9050  # ← Conflict!

# GOOD: Different ports
SocksPort 9050
ControlPort 9051
MetricsPort 9052
```

**Invalid durations:**
```ini
# BAD: Invalid format
CircuitBuildTimeout 60  # ← Missing unit

# GOOD: Valid duration
CircuitBuildTimeout 60s
```

**Pool size mismatch:**
```ini
# BAD: Max < Min
CircuitPoolMinSize 10
CircuitPoolMaxSize 5  # ← Must be >= MinSize

# GOOD: Max >= Min
CircuitPoolMinSize 2
CircuitPoolMaxSize 10
```

### Migration Guide

#### From Official Tor

Most torrc options are compatible:

| Official Tor | go-tor | Notes |
|--------------|--------|-------|
| `SocksPort` | `SocksPort` | ✅ Compatible |
| `ControlPort` | `ControlPort` | ✅ Compatible |
| `DataDirectory` | `DataDirectory` | ✅ Compatible |
| `Log` | `LogLevel` | ⚠️ Simplified (debug/info/warn/error) |
| `NumEntryGuards` | `NumEntryGuards` | ✅ Compatible |
| `CircuitBuildTimeout` | `CircuitBuildTimeout` | ✅ Compatible |
| `MaxCircuitDirtiness` | `MaxCircuitDirtiness` | ✅ Compatible |
| `NewCircuitPeriod` | `NewCircuitPeriod` | ✅ Compatible |
| `HiddenServiceDir` | `[[OnionServices]] ServiceDir` | ⚠️ Different syntax |
| `HiddenServicePort` | `[[OnionServices]] VirtualPort` | ⚠️ Different syntax |

## Examples

### Example 1: Basic Client

```ini
# Simple Tor client
SocksPort 9050
ControlPort 9051
LogLevel info
```

### Example 2: Development Setup

```ini
# Development configuration
SocksPort 9150
ControlPort 9151

# Enable metrics dashboard
EnableMetrics true
MetricsPort 9152

# Debug logging
LogLevel debug

# Local data directory
DataDirectory ./dev-tor-data

# Faster circuit rotation for testing
CircuitBuildTimeout 45s
MaxCircuitDirtiness 5m
NewCircuitPeriod 15s

# Small circuit pool
CircuitPoolMinSize 1
CircuitPoolMaxSize 3
```

### Example 3: Production with Monitoring

```ini
# Production Tor client with full monitoring

# Standard ports
SocksPort 9050
ControlPort 9051

# Monitoring
EnableMetrics true
MetricsPort 9052

# Production logging
LogLevel info

# Data directory
DataDirectory /var/lib/go-tor

# Optimized circuit settings
CircuitBuildTimeout 90s
MaxCircuitDirtiness 30m
NewCircuitPeriod 1m
NumEntryGuards 5

# Connection limits
ConnLimit 1000
DormantTimeout 24h

# Performance tuning
EnableConnectionPooling true
ConnectionPoolMaxIdle 10
ConnectionPoolMaxLife 30m

EnableCircuitPrebuilding true
CircuitPoolMinSize 5
CircuitPoolMaxSize 20

EnableBufferPooling true
```

### Example 4: Onion Service Host

```ini
# Host an onion service

# Standard settings
SocksPort 9050
ControlPort 9051
LogLevel info
DataDirectory /var/lib/go-tor

# Onion service configuration
[[OnionServices]]
ServiceDir /var/lib/go-tor/my-service
VirtualPort 80
TargetAddr localhost:8080
MaxStreams 100

# Optional: Second service
[[OnionServices]]
ServiceDir /var/lib/go-tor/another-service
VirtualPort 443
TargetAddr localhost:8443
MaxStreams 50
```

### Example 5: High Security

```ini
# Privacy-focused configuration

# Standard ports
SocksPort 9050
ControlPort 9051

# Minimal logging
LogLevel warn

# Secure data directory
DataDirectory ~/.tor-secure

# Strict circuit isolation
IsolationLevel destination
IsolateDestinations true
IsolateSOCKSAuth true
IsolateClientPort true
IsolateClientProtocol true

# Short circuit lifetimes
MaxCircuitDirtiness 5m
NewCircuitPeriod 15s
CircuitBuildTimeout 120s

# More entry guards
NumEntryGuards 5

# Small circuit pool (reduce fingerprinting)
CircuitPoolMinSize 1
CircuitPoolMaxSize 3

# Conservative connection pooling
ConnectionPoolMaxIdle 2
ConnectionPoolMaxLife 5m

# Disable metrics (no information leak)
EnableMetrics false
```

## See Also

- [Getting Started](TUTORIAL.md)
- [Architecture](ARCHITECTURE.md)
- [Production Guide](PRODUCTION.md)
- [Security](../AUDIT.md)
- [Troubleshooting](TROUBLESHOOTING.md)

## Support

- GitHub Issues: [github.com/opd-ai/go-tor/issues](https://github.com/opd-ai/go-tor/issues)
- Documentation: [github.com/opd-ai/go-tor/docs](https://github.com/opd-ai/go-tor/tree/main/docs)

**For actual Tor support**: Please contact [The Tor Project](https://www.torproject.org/contact/) directly.

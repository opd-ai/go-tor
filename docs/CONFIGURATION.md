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
```
**3. Zero-config mode (uses defaults):**
```bash
# List available templates
tor-config-validator -list-templates

# Generate a specific template
tor-config-validator -template production -output torrc

# View template on stdout
tor-config-validator -template minimal
```bash
tor-config-validator -template minimal -output torrc
```bash
tor-config-validator -template production -output torrc.prod
```bash
tor-config-validator -template development -output torrc.dev
```bash
tor-config-validator -template high-security -output torrc.secure
```ini
SocksPort 9050
ControlPort 9051
DataDirectory /var/lib/go-tor
```ini
CircuitBuildTimeout 90s
MaxCircuitDirtiness 30m
NewCircuitPeriod 1m
NumEntryGuards 3
```ini
UseEntryGuards true
UseBridges false

# Exclude specific relays (by fingerprint or nickname)
ExcludeNodes $BADRELAY1, $BADRELAY2
ExcludeExitNodes $BADEXIT1

# Bridge configuration (for censored networks)
# UseBridges true
# BridgeAddresses obfs4 192.0.2.1:443, obfs4 192.0.2.2:443
```ini
ConnLimit 1000
DormantTimeout 24h
```ini
EnableConnectionPooling true
ConnectionPoolMaxIdle 5
ConnectionPoolMaxLife 10m

EnableCircuitPrebuilding true
CircuitPoolMinSize 3
CircuitPoolMaxSize 15

EnableBufferPooling true
```ini
# No isolation (better performance)
IsolationLevel none

# Destination-based isolation (better privacy)
# IsolationLevel destination
# IsolateDestinations true

# App-level isolation (via SOCKS username)
# IsolationLevel credential
# IsolateSOCKSAuth true
```ini
# Production: info or warn
LogLevel info

# Development: debug
# LogLevel debug

# Minimal: error
# LogLevel error
```ini
# Enable metrics for monitoring
EnableMetrics true
MetricsPort 9052
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
```bash
# Validate a configuration file
tor-config-validator -config torrc

# Verbose validation with detailed feedback
tor-config-validator -config torrc -verbose
```
✓ Configuration is valid
```
Validation failed: validation error: invalid SocksPort: 99999

Errors:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✗ invalid port number: 99999
  → use a port between 0 and 65535 (0 to disable, 1024-65535 recommended for non-root)
```
✓ Configuration is valid

Warnings:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
⚠  using privileged port (< 1024)
  → consider using port >= 1024 to avoid requiring root privileges
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
# Relative path
DataDirectory ./tor-data

# Absolute path
DataDirectory /var/lib/go-tor

# User home directory
DataDirectory ~/.tor
```bash
# Create with restrictive permissions
mkdir -p /var/lib/go-tor
chmod 0700 /var/lib/go-tor
chown tor:tor /var/lib/go-tor
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
```ini
# BAD: All ports the same
SocksPort 9050
ControlPort 9050  # ← Conflict!
MetricsPort 9050  # ← Conflict!

# GOOD: Different ports
SocksPort 9050
ControlPort 9051
MetricsPort 9052
```ini
# BAD: Invalid format
CircuitBuildTimeout 60  # ← Missing unit

# GOOD: Valid duration
CircuitBuildTimeout 60s
```ini
# BAD: Max < Min
CircuitPoolMinSize 10
CircuitPoolMaxSize 5  # ← Must be >= MinSize

# GOOD: Max >= Min
CircuitPoolMinSize 2
CircuitPoolMaxSize 10
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
### Example 6: Bridge Relay
```ini
# Bridge relay configuration (non-exit relay for censorship resistance)
# ⚠️ EXPERIMENTAL - For research/educational purposes only

# Standard client ports (optional for bridges)
SocksPort 9050
ControlPort 9051

# OR port for accepting relay connections
ORPort 9001

# Bridge mode (do not publish to public directories)
BridgeRelay true

# Data directory for relay state
DataDirectory /var/lib/go-tor-bridge

# Relay identity (auto-generated on first run)
# Keys stored in: DataDirectory/keys/

# Contact information (optional but recommended)
ContactInfo your-email@example.com

# Nickname (auto-generated if not specified)
Nickname MyBridge

# Exit policy (bridges are non-exit relays)
ExitPolicy reject *:*

# Optional: Bridge authority for descriptor publishing
# BridgeAuthority 1.2.3.4:9030

# Bandwidth limits (optional)
# RelayBandwidthRate 1 MB
# RelayBandwidthBurst 2 MB

# Logging
LogLevel info
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
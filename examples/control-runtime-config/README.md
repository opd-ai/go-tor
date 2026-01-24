# Control Protocol Runtime Configuration Demo

This example demonstrates runtime configuration updates via the Tor control protocol's SETCONF command.

## Features

- **Runtime-Updateable Options**: Update circuit timing parameters without restarting the client
- **Configuration Validation**: Demonstrates proper validation of parameter values
- **Read-Only Protection**: Shows that critical options (ports, directories) correctly reject updates
- **Error Handling**: Demonstrates comprehensive error handling for invalid configurations

## Runtime-Updateable Options

### Circuit Timing Parameters

1. **MaxCircuitDirtiness** (≥30s)
   - How long circuits can be reused before rebuilding
   - Examples: "10m", "1h", "30m"
   - Default: Varies by client configuration

2. **NewCircuitPeriod** (≥10s)
   - How often to build new circuits preemptively
   - Examples: "30s", "1m", "5m"
   - Default: Varies by client configuration

3. **CircuitBuildTimeout** (10s-5m)
   - Maximum time allowed for circuit construction
   - Examples: "60s", "2m", "90s"
   - Default: Varies by client configuration

### Logging

4. **LogLevel** (debug/info/warn/error)
   - Logging verbosity level
   - Note: Configuration is updated, but logger requires restart
   - Default: info

## Read-Only Options

The following options require a client restart to change:
- **SocksPort**: SOCKS5 proxy port
- **ControlPort**: Control protocol port
- **MetricsPort**: Metrics HTTP server port
- **DataDirectory**: Data storage directory
- **NumEntryGuards**: Number of entry guards
- **UseEntryGuards**: Enable/disable entry guards
- **UseBridges**: Enable/disable bridge mode
- **EnableMetrics**: Enable/disable metrics server

## Running the Example

### Prerequisites

1. Start the go-tor client with control port enabled:
```bash
# Using default configuration (auto-detects available ports)
go run cmd/client/main.go

# Or specify control port explicitly
go run cmd/client/main.go -control-port 9051
```

2. Verify the control port is listening:
```bash
netstat -an | grep 9051
```

### Run the Demo

```bash
# From the repository root
go run examples/control-runtime-config/main.go
```

### Expected Output

```
=== Control Protocol: Runtime Configuration Updates ===

✓ Authenticated

Setting Circuit Dirtiness to 15m
  Description: How long circuits can be reused before rebuilding
  Current value: 10m0s
  New value: 15m0s
  ✓ Updated successfully

Setting New Circuit Period to 45s
  Description: How often to build new circuits preemptively
  Current value: 30s
  New value: 45s
  ✓ Updated successfully

Setting Circuit Build Timeout to 90s
  Description: Maximum time allowed for circuit construction
  Current value: 1m0s
  New value: 1m30s
  ✓ Updated successfully

Setting Log Level to debug
  Description: Logging verbosity (requires restart to affect logger)
  Current value: info
  New value: debug
  ✓ Updated successfully

=== Attempting to Update Read-Only Options ===

Attempting to set SocksPort...
  Expected error: 552 configuration option SocksPort requires restart

Attempting to set ControlPort...
  Expected error: 552 configuration option ControlPort requires restart

Attempting to set DataDirectory...
  Expected error: 552 configuration option DataDirectory requires restart

=== Testing Configuration Validation ===

Setting MaxCircuitDirtiness=10s (expecting: too short (minimum 30s))
  Expected error: 552 MaxCircuitDirtiness must be at least 30 seconds

Setting NewCircuitPeriod=5s (expecting: too short (minimum 10s))
  Expected error: 552 NewCircuitPeriod must be at least 10 seconds

Setting CircuitBuildTimeout=10m (expecting: too long (maximum 5m))
  Expected error: 552 CircuitBuildTimeout must not exceed 5 minutes

Setting LogLevel=trace (expecting: invalid log level)
  Expected error: 552 invalid log level: trace

=== Demo Complete ===

Runtime-updateable options:
  • MaxCircuitDirtiness (≥30s)
  • NewCircuitPeriod (≥10s)
  • CircuitBuildTimeout (10s-5m)
  • LogLevel (debug/info/warn/error)

These options can be updated without restarting the client.
```

## Using with torctl

You can also use the `torctl` command-line tool to update configuration:

```bash
# Connect to control port
torctl connect 127.0.0.1:9051

# Update circuit timing parameters
torctl setconf MaxCircuitDirtiness=15m
torctl setconf NewCircuitPeriod=45s
torctl setconf CircuitBuildTimeout=90s
torctl setconf LogLevel=debug

# Get current values
torctl getconf MaxCircuitDirtiness
torctl getconf NewCircuitPeriod
torctl getconf CircuitBuildTimeout
torctl getconf LogLevel
```

## Manual Control Protocol Commands

You can also interact directly with the control protocol using telnet:

```bash
# Connect to control port
telnet 127.0.0.1 9051

# Authenticate (if no password)
AUTHENTICATE

# Get configuration value
GETCONF MaxCircuitDirtiness

# Set configuration value
SETCONF MaxCircuitDirtiness=15m

# Verify update
GETCONF MaxCircuitDirtiness
```

## Implementation Details

### Duration Format

All timing parameters accept Go duration strings:
- Seconds: "30s", "60s", "90s"
- Minutes: "1m", "5m", "15m"
- Hours: "1h", "2h"
- Combined: "1h30m", "2h15m30s"

### Validation Rules

1. **MaxCircuitDirtiness**: Must be ≥30 seconds
2. **NewCircuitPeriod**: Must be ≥10 seconds
3. **CircuitBuildTimeout**: Must be 10 seconds to 5 minutes
4. **LogLevel**: Must be one of: debug, info, warn, error

### Error Codes

- **250**: Success
- **552**: Invalid argument (validation failure)
- **553**: Unrecognized option or read-only option

## Performance Tuning

### Optimizing for Latency
```bash
# Reduce circuit build timeout for faster failover
SETCONF CircuitBuildTimeout=30s

# Build new circuits more frequently
SETCONF NewCircuitPeriod=20s

# Use circuits for shorter periods
SETCONF MaxCircuitDirtiness=5m
```

### Optimizing for Throughput
```bash
# Allow longer circuit build times
SETCONF CircuitBuildTimeout=2m

# Reuse circuits for longer periods
SETCONF MaxCircuitDirtiness=30m

# Build new circuits less frequently
SETCONF NewCircuitPeriod=2m
```

### Optimizing for Privacy
```bash
# Build circuits more frequently
SETCONF NewCircuitPeriod=30s

# Rotate circuits more frequently
SETCONF MaxCircuitDirtiness=10m
```

## See Also

- [Control Protocol Specification](https://github.com/torproject/torspec/blob/main/control-spec.txt)
- [Control Authentication Example](../control-auth/)
- [Control Configuration Example](../control-config/)
- [AUDIT.md Section 10](../../AUDIT.md#10-control-protocol-control-spectxt) - Control Protocol compliance

# Control Protocol Configuration Management Example

This example demonstrates the enhanced GETCONF and SETCONF commands in the go-tor control protocol implementation.

## Overview

The go-tor client now supports comprehensive configuration management via the control protocol, allowing you to:

- Query 70+ configuration parameters via GETCONF
- Modify 20+ runtime-configurable parameters via SETCONF
- Use flexible boolean value formats (1/0, true/false, yes/no)
- Validate parameter values with automatic constraint enforcement
- Distinguish between runtime-configurable and restart-required settings

## Building

```bash
go build -o control_config_example ./examples/control_config
```

## Running

```bash
./control_config_example
```

## What This Example Demonstrates

### GETCONF Operations

1. **Single parameter query** - Query individual configuration values
2. **Multiple parameter query** - Query several related settings at once
3. **Circuit settings** - Inspect timeout, dirtiness, and rotation periods
4. **Padding configuration** - Check traffic analysis resistance settings
5. **Rate limiting** - View connection rate limits and burst capacity
6. **Tracing configuration** - Inspect distributed tracing settings

### SETCONF Operations

7. **Timeout modification** - Adjust circuit build timeout
8. **Padding strategy change** - Switch between padding algorithms
9. **Rate limit adjustment** - Modify SOCKS connection rate limits
10. **Multiple settings** - Configure several parameters simultaneously
11. **Memory monitoring** - Enable runtime monitoring features
12. **Restart-required error** - Attempt to modify immutable settings
13. **Boolean formats** - Test various boolean value representations

## Sample Output

```
=== Enhanced GETCONF/SETCONF Example ===

Starting Tor client with control port on :9051...
Connecting to control port...
Authenticating...
  250 OK

=== GETCONF Examples ===

1. Query SocksPort:
  250 SocksPort=9050

2. Query multiple circuit settings:
  250-CircuitBuildTimeout=1m0s
  250-MaxCircuitDirtiness=10m0s
  250 NewCircuitPeriod=30s

...

=== SETCONF Examples ===

6. Increase circuit build timeout to 90 seconds:
  250 OK
  250 CircuitBuildTimeout=1m30s

7. Change padding strategy to adaptive:
  250 OK
  250 PaddingStrategy=adaptive

...

11. Attempt to modify SocksPort (should fail - requires restart):
  552 configuration option SocksPort requires restart

...
```

## Key Features Demonstrated

### Comprehensive Parameter Coverage

The implementation supports querying and modifying parameters across all major configuration categories:

- Network settings (ports, directories, limits)
- Circuit settings (timeouts, rotation, guards)
- Path selection (excluded nodes, bridges)
- Circuit padding (strategy, intervals, burst size)
- Rate limiting (connections per second, burst capacity)
- Performance tuning (pooling, prebuilding, buffers)
- Monitoring (metrics, tracing, memory, profiling)

### Flexible Boolean Values

Boolean parameters accept multiple value formats for convenience:

- Numeric: `0`, `1`
- Lowercase: `false`, `true`, `no`, `yes`
- Capitalized: `False`, `True`, `No`, `Yes`
- Uppercase: `FALSE`, `TRUE`, `NO`, `YES`

### Parameter Validation

SETCONF enforces constraints on parameter values:

- Duration minimums (e.g., CircuitBuildTimeout >= 10s)
- Duration maximums (e.g., CircuitBuildTimeout <= 5m)
- Numeric ranges (e.g., PaddingBurstSize between 1 and 100)
- Valid enumerations (e.g., PaddingStrategy in [none, fixed, random, adaptive])
- Positive values for rates and limits

### Runtime vs. Restart-Required

The implementation clearly distinguishes between:

- **Runtime-configurable**: Parameters that can be modified while the client is running (timeouts, padding settings, rate limits, etc.)
- **Restart-required**: Parameters that require a client restart to take effect (ports, pooling settings, isolation levels, etc.)

Attempting to modify restart-required parameters via SETCONF returns an informative error message.

## Related Documentation

- [Control Protocol Configuration Reference](../../docs/CONTROL_PROTOCOL_CONFIG.md) - Complete parameter list and usage guide
- [Control Protocol Overview](../../docs/CONTROL_PROTOCOL.md) - General control protocol documentation
- [Configuration Reference](../../pkg/config/config.go) - Complete configuration structure

## Control-Spec Compliance

This implementation follows the Tor control protocol specification (control-spec.txt §3.1) for GETCONF and SETCONF commands, including:

- Proper authentication requirements
- Standard response formatting (250-/250 delimiters)
- Error code handling (552, 514, 551)
- Multi-value parameter support
- Key=Value syntax for SETCONF

## Testing

The implementation includes comprehensive test coverage (>95%) for both GetConfigValue and SetConfigValue operations:

```bash
go test -v ./pkg/client -run TestClientConfigProvider
```

Tests verify:
- Correct value retrieval for all 70+ parameters
- Proper boolean value parsing
- Parameter validation and constraint enforcement
- Error handling for invalid values
- Nil config handling
- Restart-required parameter detection

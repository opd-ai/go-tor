# Control Protocol Configuration Management

This document describes the GETCONF and SETCONF commands in the go-tor control protocol implementation, which allow runtime querying and modification of configuration parameters.

## Overview

The go-tor control protocol implements a subset of the Tor control protocol (control-spec.txt §3.1) for configuration management. These commands enable runtime inspection and modification of client configuration without requiring a restart.

## GETCONF Command

The GETCONF command retrieves the current value of one or more configuration options.

### Syntax

```
GETCONF <KeywordLine>
```

Where `<KeywordLine>` is one or more configuration keys separated by spaces.

### Response Format

```
250-Key1=Value1
250-Key2=Value2
250 Key3=Value3
```

The last response line uses `250 ` (with space) to indicate end of response.

### Supported Configuration Keys

The following configuration parameters can be queried via GETCONF:

#### Network Settings
- `SocksPort` - SOCKS5 proxy port
- `ControlPort` - Control protocol port
- `DataDirectory` - Directory for persistent state
- `ConnLimit` - Max concurrent connections
- `DormantTimeout` - Time before entering dormant mode

#### Circuit Settings
- `CircuitBuildTimeout` - Max time to build a circuit
- `MaxCircuitDirtiness` - Max time to use a circuit
- `NewCircuitPeriod` - How often to rotate circuits
- `NumEntryGuards` - Number of entry guards to use

#### Path Selection
- `UseEntryGuards` - Whether to use entry guards (0/1)
- `UseBridges` - Whether to use bridges (0/1)
- `ExcludeNodes` - Comma-separated list of nodes to exclude
- `ExcludeExitNodes` - Comma-separated list of exit nodes to exclude

#### Logging
- `LogLevel` - Log level: debug, info, warn, error

#### Monitoring
- `MetricsPort` - HTTP metrics server port
- `EnableMetrics` - Enable HTTP metrics endpoint (0/1)

#### Performance Tuning
- `EnableConnectionPooling` - Enable connection pooling (0/1)
- `ConnectionPoolMaxIdle` - Max idle connections per relay
- `ConnectionPoolMaxLife` - Max lifetime for pooled connections
- `EnableCircuitPrebuilding` - Enable circuit prebuilding (0/1)
- `CircuitPoolMinSize` - Minimum circuits to prebuild
- `CircuitPoolMaxSize` - Maximum circuits in pool
- `EnableBufferPooling` - Enable buffer pooling (0/1)

#### Circuit Isolation
- `IsolationLevel` - Isolation level: none, destination, credential, port, session
- `IsolateDestinations` - Isolate by destination (0/1)
- `IsolateSOCKSAuth` - Isolate by SOCKS5 username (0/1)
- `IsolateClientPort` - Isolate by client source port (0/1)
- `IsolateClientProtocol` - Isolate by protocol (0/1)

#### Circuit Padding
- `EnableCircuitPadding` - Enable circuit padding (0/1)
- `PaddingStrategy` - Padding strategy: none, fixed, random, adaptive
- `PaddingMinInterval` - Minimum interval between padding cells
- `PaddingMaxInterval` - Maximum interval between padding cells
- `PaddingIdleTimeout` - Time circuit must be idle before padding
- `PaddingDummyTraffic` - Use dummy RELAY_DATA instead of PADDING cells (0/1)
- `PaddingBurstSize` - Number of padding cells per burst

#### Rate Limiting
- `EnableRateLimiting` - Enable rate limiting (0/1)
- `SOCKSConnectionsPerSecond` - Max SOCKS connections per second
- `SOCKSConnectionsBurst` - Burst capacity for SOCKS connections
- `MaxConcurrentConnections` - Max concurrent SOCKS connections
- `EnablePerClientRateLimiting` - Enable per-client rate limiting (0/1)
- `PerClientConnectionsPerSecond` - Per-client connection rate
- `PerClientConnectionsBurst` - Per-client burst capacity
- `RateLimitCleanupInterval` - Cleanup interval for per-client limiters (seconds)

#### Guard Persistence
- `GuardStateBackupCount` - Number of guard state backup files to retain
- `GuardStateSnapshotInterval` - Interval between automatic guard state snapshots (seconds)
- `GuardStateLockTimeout` - Timeout for acquiring guard state file lock (seconds)

#### Distributed Tracing
- `EnableTracing` - Enable distributed tracing (0/1)
- `TracingEndpoint` - Collector endpoint for OTLP
- `TracingSampleRate` - Sampling rate 0.0 to 1.0
- `TracingExporter` - Exporter type: otlp, stdout, noop
- `TracingInsecure` - Disable TLS for OTLP exporter (0/1)
- `TracingTimeout` - Export timeout duration

#### Memory Monitoring
- `EnableMemoryMonitoring` - Enable memory pressure monitoring (0/1)
- `MemoryHighWaterMark` - Heap allocation threshold for degraded status (bytes)
- `MemoryCriticalMark` - Heap allocation threshold for unhealthy status (bytes)
- `MemoryMaxGoroutines` - Maximum goroutine count threshold
- `MemoryCheckInterval` - Interval between memory checks (seconds)
- `MemoryTriggerGCOnCritical` - Trigger GC when critical memory pressure detected (0/1)

#### Crash Recovery
- `EnableCrashRecovery` - Enable crash recovery checkpointing (0/1)
- `CrashRecoveryCheckpointPath` - Path to checkpoint file
- `CrashRecoveryInterval` - Interval between checkpoints (seconds)
- `CrashRecoveryBackupCount` - Number of checkpoint backup files to retain

#### Profiling
- `EnableProfiling` - Enable pprof HTTP endpoints (0/1)
- `ProfilingPort` - Port for pprof endpoints
- `ProfilingPath` - Path prefix for pprof endpoints
- `EnableCPUProfiling` - Enable CPU profiling capability (0/1)
- `EnableHeapProfiling` - Enable heap profiling capability (0/1)
- `EnableMutexProfile` - Enable mutex contention profiling (0/1)
- `EnableBlockProfile` - Enable blocking profiling (0/1)

### Example

```
C: GETCONF SocksPort ControlPort LogLevel
S: 250-SocksPort=9050
S: 250-ControlPort=9051
S: 250 LogLevel=info
```

## SETCONF Command

The SETCONF command sets the value of one or more configuration options at runtime.

### Syntax

```
SETCONF <KeywordLine>
```

Where `<KeywordLine>` is one or more `Key=Value` pairs separated by spaces.

### Response Format

```
250 OK
```

Or on error:

```
552 Unrecognized option
```

### Runtime-Configurable Parameters

The following parameters can be modified at runtime via SETCONF:

#### Circuit Settings
- `MaxCircuitDirtiness` - Must be at least 30 seconds
- `NewCircuitPeriod` - Must be at least 10 seconds
- `CircuitBuildTimeout` - Must be between 10 seconds and 5 minutes
- `DormantTimeout` - Must be at least 1 minute

#### Path Selection
- `ExcludeNodes` - Comma-separated list (empty string clears list)
- `ExcludeExitNodes` - Comma-separated list (empty string clears list)

#### Logging
- `LogLevel` - Valid values: debug, info, warn, error (requires restart to take effect)

#### Circuit Padding
- `EnableCircuitPadding` - Boolean (0/1, true/false, yes/no)
- `PaddingStrategy` - Valid values: none, fixed, random, adaptive
- `PaddingMinInterval` - Must be at least 100ms
- `PaddingMaxInterval` - Must be at least 1 second
- `PaddingIdleTimeout` - Must be at least 100ms
- `PaddingBurstSize` - Must be between 1 and 100

#### Rate Limiting
- `EnableRateLimiting` - Boolean (0/1, true/false, yes/no)
- `SOCKSConnectionsPerSecond` - Must be positive
- `SOCKSConnectionsBurst` - Must be at least 1
- `MaxConcurrentConnections` - Must be at least 10
- `EnablePerClientRateLimiting` - Boolean (0/1, true/false, yes/no)
- `PerClientConnectionsPerSecond` - Must be positive
- `PerClientConnectionsBurst` - Must be at least 1

#### Distributed Tracing
- `EnableTracing` - Boolean (0/1, true/false, yes/no)
- `TracingSampleRate` - Must be between 0.0 and 1.0

#### Memory Monitoring
- `EnableMemoryMonitoring` - Boolean (0/1, true/false, yes/no)
- `MemoryTriggerGCOnCritical` - Boolean (0/1, true/false, yes/no)

### Parameters Requiring Restart

The following parameters cannot be modified at runtime and will return an error indicating a restart is required:

- Network settings: `SocksPort`, `ControlPort`, `DataDirectory`, `ConnLimit`
- Path selection: `NumEntryGuards`, `UseEntryGuards`, `UseBridges`
- Monitoring: `MetricsPort`, `EnableMetrics`
- Performance tuning: All connection pooling and circuit prebuilding options
- Circuit isolation: All isolation settings
- Padding: `PaddingDummyTraffic`
- Rate limiting: `RateLimitCleanupInterval`
- Guard persistence: All guard state settings
- Tracing: `TracingEndpoint`, `TracingExporter`, `TracingInsecure`, `TracingTimeout`
- Memory monitoring: `MemoryHighWaterMark`, `MemoryCriticalMark`, `MemoryMaxGoroutines`, `MemoryCheckInterval`
- Crash recovery: All crash recovery settings
- Profiling: All profiling settings

### Boolean Value Formats

Boolean values can be specified using any of the following formats:

**True values:** `1`, `true`, `True`, `TRUE`, `yes`, `Yes`, `YES`  
**False values:** `0`, `false`, `False`, `FALSE`, `no`, `No`, `NO`

### Example

```
C: SETCONF MaxCircuitDirtiness=5m EnableCircuitPadding=1
S: 250 OK

C: SETCONF SocksPort=9999
S: 552 configuration option SocksPort requires restart
```

## Integration with torctl

The `torctl` command-line utility provides a user-friendly interface to the control protocol:

```bash
# Query configuration
torctl info

# The getConfig function is now available for direct configuration queries
```

## Implementation Details

### Location

- **Interface definition:** `pkg/control/control.go`
- **Implementation:** `pkg/client/client.go` (clientConfigProvider)
- **Tests:** `pkg/client/config_provider_test.go`, `pkg/control/config_test.go`

### Coverage

Over 70 configuration parameters are now accessible via GETCONF, with 20+ parameters runtime-configurable via SETCONF. This provides comprehensive configuration management capabilities without requiring client restarts for common adjustments.

### Compliance

This implementation follows the Tor control protocol specification (control-spec.txt §3.1) for GETCONF and SETCONF commands, including proper authentication requirements, response formatting, and error handling.

## See Also

- [Control Protocol Overview](CONTROL_PROTOCOL.md) - General control protocol documentation
- [Configuration Reference](../pkg/config/config.go) - Complete configuration structure
- control-spec.txt §3.1 - Official Tor control protocol specification

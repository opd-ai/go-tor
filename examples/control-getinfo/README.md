# Control Protocol GETINFO Demo

This example demonstrates the enhanced GETINFO command coverage in the go-tor control protocol.

## Features Demonstrated

### Enhanced GETINFO Keys

The control protocol now supports comprehensive monitoring through these GETINFO keys:

**Circuit Statistics:**
- `status/circuits` - Number of active circuits
- `status/circuit-builds` - Total circuit build attempts
- `status/circuit-build-success` - Successful circuit builds
- `status/circuit-build-failure` - Failed circuit builds

**Guard Statistics:**
- `status/guards/active` - Number of active guards
- `status/guards/confirmed` - Number of confirmed guards

**Network Information:**
- `net/listeners/socks` - SOCKS proxy listener address
- `net/listeners/control` - Control protocol listener address
- `status/connection-attempts` - Total connection attempts

**System Information:**
- `status/uptime` - Client uptime in seconds
- `config-file` - Data directory path

**Discovery:**
- `info/names` - List all available GETINFO keys

**Legacy Keys (still supported):**
- `version` - Client version
- `traffic/read` - Bytes read
- `traffic/written` - Bytes written
- `status/circuit-established` - Whether circuits are established
- `status/enough-dir-info` - Whether directory info is sufficient

## Running the Example

```bash
cd examples/control-getinfo
go run main.go
```

## Example Output

```
=== Basic Information ===
  version:                            go-tor 0.1.0
  status/circuit-established:         1
  status/enough-dir-info:             1

=== Circuit Statistics ===
  status/circuits:                    3
  status/circuit-builds:              100
  status/circuit-build-success:       95
  status/circuit-build-failure:       5

=== Guard Statistics ===
  status/guards/active:               3
  status/guards/confirmed:            2

=== Network Information ===
  net/listeners/socks:                127.0.0.1:19050
  net/listeners/control:              127.0.0.1:19051
  status/connection-attempts:         200

=== System Information ===
  status/uptime:                      3600
  config-file:                        /tmp/go-tor
```

## Use Cases

1. **Monitoring Tools**: Build dashboards showing circuit health, guard status, and system metrics
2. **Debugging**: Query detailed statistics to diagnose connection issues
3. **Automation**: Script circuit management based on build success rates
4. **Integration**: Connect monitoring systems like Prometheus to track Tor client health

## Comparison with Official Tor

This implementation provides a focused subset of GETINFO keys most relevant for client monitoring. The official Tor implementation supports 100+ keys including:
- Relay-specific information (not applicable for pure clients)
- Consensus/descriptor queries (available via directory client API)
- Stream/circuit details (available via control events)

The go-tor implementation prioritizes:
- Client health metrics
- Circuit/guard statistics
- Network configuration
- System resource usage

## See Also

- `examples/control-auth` - Control protocol authentication
- `examples/control-config` - Configuration management (GETCONF/SETCONF)
- `examples/metrics-demo` - Metrics HTTP endpoint

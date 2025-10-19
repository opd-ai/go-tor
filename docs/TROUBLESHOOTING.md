# go-tor Troubleshooting Guide

This guide helps you diagnose and resolve common issues when using go-tor.

## Table of Contents

- [Connection Issues](#connection-issues)
- [Circuit Build Problems](#circuit-build-problems)
- [SOCKS5 Proxy Issues](#socks5-proxy-issues)
- [Performance Problems](#performance-problems)
- [Configuration Errors](#configuration-errors)
- [Control Protocol Issues](#control-protocol-issues)
- [Resource Usage](#resource-usage)
- [Logging and Debugging](#logging-and-debugging)
- [Common Error Messages](#common-error-messages)

---

## Connection Issues

### Cannot Connect to Tor Network

**Symptoms:**
- Client starts but no circuits are built
- "Connection refused" or "timeout" errors
- No network traffic

**Possible Causes & Solutions:**

1. **Firewall Blocking Outbound Connections**
   
   Tor needs to connect to directory servers and relays on ports 443 and 9001.
   
   ```bash
   # Check if ports are accessible
   nc -zv 66.111.2.131 9001
   nc -zv 154.35.175.225 443
   ```
   
   Solution: Configure firewall to allow outbound TCP on ports 443 and 9001.

2. **Network Proxy Required**
   
   If you're behind a corporate proxy, Tor can't connect directly.
   
   Solution: Currently, go-tor doesn't support HTTPS/HTTP proxies for bootstrap. Use network-level proxy configuration or VPN.

3. **DNS Resolution Issues**
   
   ```bash
   # Test DNS resolution
   nslookup www.torproject.org
   ```
   
   Solution: Ensure DNS is working. You can use public DNS (8.8.8.8 or 1.1.1.1).

4. **No Internet Connection**
   
   ```bash
   # Test basic connectivity
   ping -c 3 8.8.8.8
   curl https://www.google.com
   ```
   
   Solution: Fix your internet connection first.

### Connection Timeout

**Error:** `context deadline exceeded` or `connection timeout`

**Solutions:**

1. **Increase Timeouts**
   
   ```go
   cfg := config.DefaultConfig()
   cfg.CircuitBuildTimeout = 120 * time.Second  // Increase from default
   ```

2. **Check Network Latency**
   
   ```bash
   # Test latency to a Tor directory server
   ping -c 5 131.188.40.189
   ```
   
   If latency is high (>500ms), increase timeouts proportionally.

3. **Try Different Directory Servers**
   
   Enable debug logging to see which servers are being contacted:
   
   ```go
   cfg.LogLevel = "debug"
   ```

---

## Circuit Build Problems

### Circuit Build Failures

**Symptoms:**
- "circuit build failed" errors
- Circuits stuck in "BUILDING" state
- Frequent circuit timeouts

**Diagnostic Steps:**

1. **Check Circuit Build Logs**
   
   ```go
   log := logger.New(logger.LevelDebug, os.Stdout)
   ```
   
   Look for:
   - Which hop fails (guard, middle, exit)
   - Specific error messages
   - TLS handshake failures

2. **Verify Relay Connectivity**
   
   ```bash
   # Test if you can reach Tor relays
   nc -zv <relay-ip> 9001
   nc -zv <relay-ip> 443
   ```

**Common Causes:**

1. **Slow Network**
   
   Solution: Increase circuit build timeout
   
   ```go
   cfg.CircuitBuildTimeout = 180 * time.Second
   ```

2. **Relay Selection Issues**
   
   Some relays may be unreliable. The client will automatically try others.
   
   Monitor with:
   ```bash
   # Via control port
   echo "SETEVENTS CIRC" | nc 127.0.0.1 9051
   ```

3. **TLS Certificate Validation**
   
   If relays have invalid certificates:
   
   ```
   ERROR: TLS verification failed for relay X
   ```
   
   This is expected behavior - the client should move to another relay.

### No Guard Nodes Available

**Error:** `no suitable guard nodes found`

**Solutions:**

1. **Wait for Directory Download**
   
   The client needs to download the consensus first (usually takes 10-30 seconds on first run).

2. **Check Data Directory**
   
   ```bash
   ls -la /var/lib/tor/  # or your configured data directory
   ```
   
   Ensure the directory is writable:
   
   ```bash
   chmod 755 /var/lib/tor
   ```

3. **Delete Corrupted Cache**
   
   ```bash
   rm -rf /var/lib/tor/*
   # Restart client to re-download
   ```

---

## SOCKS5 Proxy Issues

### Cannot Connect to SOCKS5 Port

**Error:** `connection refused to 127.0.0.1:9050`

**Solutions:**

1. **Verify Port Configuration**
   
   ```bash
   # Check if go-tor is listening
   netstat -tln | grep 9050
   # or
   lsof -i :9050
   ```

2. **Port Already in Use**
   
   ```bash
   # Find what's using the port
   lsof -i :9050
   ```
   
   Solution: Change port or stop conflicting process
   
   ```go
   cfg.SocksPort = 9150  // Use different port
   ```

3. **Binding to Wrong Interface**
   
   By default, go-tor binds to 127.0.0.1 (localhost only).
   
   To accept remote connections (be careful - security risk):
   
   ```go
   // This would require modifying the SOCKS server code
   // Currently not supported for security reasons
   ```

### SOCKS5 Authentication Errors

**Error:** `SOCKS5 authentication failed`

**Note:** go-tor currently uses no authentication (method 0x00). If your client requires authentication, it's incompatible.

**Solution:** Configure your client to use "no authentication".

### Slow SOCKS5 Performance

**Symptoms:**
- High latency through proxy
- Slow page loads
- Connection timeouts

**Solutions:**

1. **Enable Circuit Prebuilding**
   
   ```go
   cfg.PrebuiltCircuits = 3
   cfg.MaxIdleCircuits = 10
   ```

2. **Increase Circuit Pool**
   
   More circuits = better parallelism
   
   ```go
   cfg.MaxIdleCircuits = 20
   ```

3. **Optimize Circuit Lifetime**
   
   ```go
   cfg.MaxCircuitDirtiness = 10 * time.Minute
   ```

4. **Check Resource Limits**
   
   ```bash
   # Check open files limit
   ulimit -n
   ```
   
   Increase if low (recommended: 4096+):
   
   ```bash
   ulimit -n 8192
   ```

---

## Performance Problems

### High Memory Usage

**Symptoms:**
- RSS memory > 100MB
- Increasing memory over time
- Out of memory errors

**Diagnostic Steps:**

1. **Check Current Usage**
   
   ```bash
   ps aux | grep tor-client
   ```

2. **Profile Memory**
   
   ```go
   import _ "net/http/pprof"
   
   go func() {
       log.Println(http.ListenAndServe("localhost:6060", nil))
   }()
   ```
   
   Then:
   ```bash
   go tool pprof http://localhost:6060/debug/pprof/heap
   ```

**Solutions:**

1. **Reduce Circuit Pool**
   
   ```go
   cfg.PrebuiltCircuits = 1
   cfg.MaxIdleCircuits = 5
   ```

2. **More Aggressive Circuit Rotation**
   
   ```go
   cfg.MaxCircuitDirtiness = 5 * time.Minute
   ```

3. **Connection Pool Cleanup**
   
   Ensure resources are being cleaned up:
   
   ```go
   // Should happen automatically, but verify in logs
   log.SetLevel(logger.LevelDebug)
   ```

### High CPU Usage

**Symptoms:**
- CPU usage > 50% consistently
- System slowdown
- Thermal throttling

**Solutions:**

1. **Check for Busy Loop**
   
   ```bash
   # CPU profile
   curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof
   go tool pprof cpu.prof
   ```

2. **Reduce Prebuilding Aggressiveness**
   
   ```go
   cfg.PrebuiltCircuits = 0  // Disable prebuilding
   ```

3. **Increase Rebuild Interval**
   
   ```go
   // Reduce frequency of circuit checks
   cfg.CircuitBuildTimeout = 60 * time.Second
   ```

### Slow Circuit Builds

**Symptoms:**
- Circuit build time > 10 seconds
- Timeouts during build
- Poor user experience

**Solutions:**

1. **Network Issue**
   
   Check latency:
   ```bash
   # Test round-trip to common relay
   ping -c 10 <relay-ip>
   ```

2. **CPU Bottleneck**
   
   Cryptographic operations are CPU-intensive.
   
   Solution: Ensure adequate CPU resources.

3. **Relay Selection**
   
   The client might be selecting slow relays.
   
   Monitor circuit paths:
   ```bash
   echo "SETEVENTS CIRC" | nc 127.0.0.1 9051
   ```

---

## Configuration Errors

### Invalid Configuration

**Error:** `invalid configuration: <reason>`

**Common Mistakes:**

1. **Invalid Port Numbers**
   
   ```go
   // Wrong - port out of range
   cfg.SocksPort = 70000  // Max is 65535
   
   // Wrong - reserved port without permissions
   cfg.SocksPort = 80     // Requires root
   
   // Correct
   cfg.SocksPort = 9050   // Valid user port
   ```

2. **Invalid Directory Path**
   
   ```go
   // Wrong - non-existent parent
   cfg.DataDirectory = "/nonexistent/tor-data"
   
   // Correct - create parent first or use writable location
   cfg.DataDirectory = "./tor-data"
   ```

3. **Invalid Log Level**
   
   ```go
   // Wrong
   cfg.LogLevel = "verbose"  // Not a valid level
   
   // Correct
   cfg.LogLevel = "debug"    // Valid: debug, info, warn, error
   ```

### Configuration File Parse Errors

**Error:** `failed to parse config file`

**Solutions:**

1. **Check File Format**
   
   ```bash
   # Validate torrc syntax
   cat /path/to/torrc
   ```
   
   Correct format:
   ```
   # Comments start with #
   SocksPort 9050
   ControlPort 9051
   DataDirectory /var/lib/tor
   Log info
   ```

2. **Check File Permissions**
   
   ```bash
   chmod 644 /path/to/torrc
   ```

3. **Verify File Path**
   
   ```bash
   # Check if file exists
   ls -la /path/to/torrc
   ```

---

## Control Protocol Issues

### Cannot Connect to Control Port

**Error:** `connection refused to 127.0.0.1:9051`

**Solutions:**

1. **Verify Control Port is Enabled**
   
   ```go
   cfg.ControlPort = 9051  // Ensure it's set
   ```

2. **Check if Port is Listening**
   
   ```bash
   netstat -tln | grep 9051
   ```

3. **Port Conflict**
   
   ```bash
   lsof -i :9051
   ```
   
   Solution: Use different port or stop conflicting service.

### Invalid Control Commands

**Error:** `512 Unrecognized command`

**Solution:** Check command syntax

Supported commands:
```
GETINFO version
GETINFO status/circuit-established
SETEVENTS CIRC STREAM BW
SIGNAL SHUTDOWN
SIGNAL RELOAD
```

Unsupported commands will return 512 error.

### Event Subscription Issues

**Problem:** Not receiving events after `SETEVENTS`

**Solutions:**

1. **Verify Subscription**
   
   ```bash
   echo "SETEVENTS CIRC STREAM" | nc -i 30 127.0.0.1 9051
   ```
   
   You should see `250 OK` response.

2. **Keep Connection Open**
   
   Events are delivered over the same connection. Don't close it.
   
   ```bash
   # Wrong - connection closes immediately
   echo "SETEVENTS CIRC" | nc 127.0.0.1 9051
   
   # Right - connection stays open
   nc 127.0.0.1 9051
   > SETEVENTS CIRC
   < 250 OK
   # Wait for events...
   ```

---

## Resource Usage

### Too Many Open Files

**Error:** `too many open files`

**Solutions:**

1. **Check Current Limit**
   
   ```bash
   ulimit -n
   ```

2. **Increase Limit**
   
   ```bash
   # Temporary
   ulimit -n 8192
   
   # Permanent (add to /etc/security/limits.conf)
   * soft nofile 8192
   * hard nofile 16384
   ```

3. **Reduce Connection Pool**
   
   ```go
   cfg.ConnectionPoolSize = 10  // Reduce from default
   cfg.MaxIdleCircuits = 5
   ```

### Disk Space Issues

**Error:** `no space left on device`

**Solutions:**

1. **Check Disk Usage**
   
   ```bash
   df -h /var/lib/tor  # or your data directory
   du -sh /var/lib/tor
   ```

2. **Clean Up Old Data**
   
   ```bash
   # Remove all cached data (will re-download)
   rm -rf /var/lib/tor/*
   ```

3. **Use Different Directory**
   
   ```go
   cfg.DataDirectory = "/path/with/more/space"
   ```

---

## Logging and Debugging

### Enable Debug Logging

```go
// Via configuration
cfg.LogLevel = "debug"

// Or create debug logger directly
log := logger.New(logger.LevelDebug, os.Stdout)
```

### Structured Logging Output

Debug logs include:
- Component name
- Timestamps
- Structured key-value pairs

Example output:
```
time=2025-10-19T15:30:00Z level=DEBUG msg="Building circuit" component=builder guard=Node1 middle=Node2 exit=Node3
time=2025-10-19T15:30:02Z level=DEBUG msg="Circuit extended" component=circuit circuit_id=42 hop=1
```

### Log to File

```go
logFile, err := os.OpenFile("tor-client.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
if err != nil {
    panic(err)
}
defer logFile.Close()

log := logger.New(logger.LevelDebug, logFile)
```

### Selective Component Logging

```go
// Get component-specific logger
circuitLog := log.Component("circuit")
socksLog := log.Component("socks")

// Only logs from these components will show component name
circuitLog.Debug("Building circuit")  // Shows: component=circuit
```

---

## Common Error Messages

### "circuit build timeout"

**Meaning:** Circuit didn't complete building within timeout period.

**Solutions:**
- Increase `CircuitBuildTimeout`
- Check network connectivity
- Enable debug logging to see which hop fails

### "no suitable relays found"

**Meaning:** Can't find relays matching requirements (usually exit policy).

**Solutions:**
- Wait for consensus download to complete
- Verify internet connectivity
- Check if requesting unusual port (may need different exit policy)

### "TLS handshake failed"

**Meaning:** Couldn't establish TLS connection with relay.

**Solutions:**
- Normal - client will try other relays
- If persistent, check network/firewall
- Ensure outbound TLS (port 443/9001) allowed

### "consensus download failed"

**Meaning:** Couldn't download network consensus from directory servers.

**Solutions:**
- Check internet connectivity
- Verify DNS resolution
- Check if directory server ports (80/443) are accessible

### "guard node persistence failed"

**Meaning:** Couldn't save guard nodes to disk.

**Solutions:**
- Check data directory permissions
- Verify disk space
- Ensure parent directory exists

---

## Getting Help

If you can't resolve your issue:

1. **Collect Information:**
   - go-tor version (`./bin/tor-client -version`)
   - Go version (`go version`)
   - Operating system and version
   - Configuration used
   - Complete error message
   - Debug logs (relevant sections)

2. **Search Issues:**
   - GitHub Issues: https://github.com/opd-ai/go-tor/issues
   - Look for similar problems

3. **Create New Issue:**
   - Include all information from step 1
   - Describe steps to reproduce
   - What you expected vs. what happened
   - Any troubleshooting steps you tried

4. **Community Resources:**
   - Project documentation: https://github.com/opd-ai/go-tor/docs
   - Tor specifications: https://spec.torproject.org/
   - General Tor help: https://support.torproject.org/

---

## Prevention Tips

1. **Always Validate Configuration**
   ```go
   if err := cfg.Validate(); err != nil {
       // Handle before starting client
   }
   ```

2. **Implement Graceful Shutdown**
   ```go
   defer torClient.Stop()
   ```

3. **Monitor Resource Usage**
   ```go
   stats := torClient.GetStats()
   // Log periodically
   ```

4. **Use Appropriate Log Levels**
   - Production: `info` or `warn`
   - Development: `debug`
   - Testing: `debug`

5. **Handle Errors Properly**
   ```go
   if err != nil {
       log.Error("Operation failed", "error", err)
       // Take appropriate action
   }
   ```

---

## Quick Diagnostic Checklist

When encountering issues, check:

- [ ] Internet connectivity working
- [ ] Firewall allows outbound on ports 443, 9001
- [ ] DNS resolution working
- [ ] Ports 9050/9051 not in use
- [ ] Data directory writable
- [ ] Sufficient disk space
- [ ] Adequate file descriptor limit
- [ ] Valid configuration
- [ ] Using supported Go version (1.21+)
- [ ] Latest go-tor version

Run basic tests:

```bash
# Network connectivity
ping -c 3 8.8.8.8

# Tor relay reachability
nc -zv 131.188.40.189 9001

# Port availability
netstat -tln | grep -E "9050|9051"

# Disk space
df -h

# File descriptors
ulimit -n
```

---

This troubleshooting guide covers the most common issues. For additional help, refer to the [API Documentation](API.md), [Tutorial](TUTORIAL.md), or create an issue on GitHub.

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
2. **Network Proxy Required**
3. **DNS Resolution Issues**
4. **No Internet Connection**
### Connection Timeout
**Error:** `context deadline exceeded` or `connection timeout`
**Solutions:**
1. **Increase Timeouts**
   ```go
   cfg := config.DefaultConfig()
   cfg.CircuitBuildTimeout = 120 * time.Second  // Increase from default
2. **Check Network Latency**
3. **Try Different Directory Servers**
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
**Common Causes:**
1. **Slow Network**
2. **Relay Selection Issues**
   echo "SETEVENTS CIRC" | nc 127.0.0.1 9051
3. **TLS Certificate Validation**
### No Guard Nodes Available
**Error:** `no suitable guard nodes found`
**Solutions:**
1. **Wait for Directory Download**
   The client needs to download the consensus first (usually takes 10-30 seconds on first run).
2. **Check Data Directory**
3. **Delete Corrupted Cache**
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
3. **Binding to Wrong Interface**
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
2. **Increase Circuit Pool**
3. **Optimize Circuit Lifetime**
4. **Check Resource Limits**
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
**Solutions:**
1. **Reduce Circuit Pool**
2. **More Aggressive Circuit Rotation**
3. **Connection Pool Cleanup**
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
2. **Reduce Prebuilding Aggressiveness**
3. **Increase Rebuild Interval**
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
2. **CPU Bottleneck**
3. **Relay Selection**
   echo "SETEVENTS CIRC" | nc 127.0.0.1 9051
---
## Configuration Errors
### Invalid Configuration
**Error:** `invalid configuration: <reason>`
**Common Mistakes:**
1. **Invalid Port Numbers**
   ```go
   // Wrong - port out of range
   cfg.SocksPort = 70000  // Max is 65535
2. **Invalid Directory Path**
3. **Invalid Log Level**
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
2. **Check File Permissions**
3. **Verify File Path**
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
   netstat -tln | grep 9051
3. **Port Conflict**
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
### Event Subscription Issues
**Problem:** Not receiving events after `SETEVENTS`
**Solutions:**
1. **Verify Subscription**
   ```bash
   echo "SETEVENTS CIRC STREAM" | nc -i 30 127.0.0.1 9051
   ```
   You should see `250 OK` response.
2. **Keep Connection Open**
   echo "SETEVENTS CIRC" | nc 127.0.0.1 9051
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
   * soft nofile 8192
   * hard nofile 16384
   ```
3. **Reduce Connection Pool**
   ```go
   cfg.ConnectionPoolSize = 10  // Reduce from default
   cfg.MaxIdleCircuits = 5
### Disk Space Issues
**Error:** `no space left on device`
**Solutions:**
1. **Check Disk Usage**
   ```bash
   df -h /var/lib/tor  # or your data directory
   du -sh /var/lib/tor
2. **Clean Up Old Data**
3. **Use Different Directory**
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
2. **Implement Graceful Shutdown**
3. **Monitor Resource Usage**
4. **Use Appropriate Log Levels**
   - Production: `info` or `warn`
   - Development: `debug`
   - Testing: `debug`
5. **Handle Errors Properly**
   ```go
   if err != nil {
       log.Error("Operation failed", "error", err)
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
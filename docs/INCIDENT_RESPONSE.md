# go-tor Incident Response Playbook

## ⚠️ IMPORTANT DISCLAIMER

**THIS IS UNOFFICIAL SOFTWARE** developed without the supervision or endorsement of [The Tor Project](https://www.torproject.org/). This software should **NOT** be used in production or for any real anonymity needs.

**For actual privacy and security:**
- **Users**: Use [Tor Browser](https://www.torproject.org/download/)
- **Developers**: Use [Arti](https://gitlab.torproject.org/tpo/core/arti)

This playbook is for **testing and development environments only**.

---

## Table of Contents

- [Incident Classification](#incident-classification)
- [Initial Response](#initial-response)
- [Incident Playbooks](#incident-playbooks)
  - [No Active Circuits](#incident-no-active-circuits)
  - [High Circuit Build Failure Rate](#incident-high-circuit-build-failure-rate)
  - [SOCKS Proxy Unresponsive](#incident-socks-proxy-unresponsive)
  - [High Memory Usage](#incident-high-memory-usage)
  - [Service Not Starting](#incident-service-not-starting)
  - [Connection Exhaustion](#incident-connection-exhaustion)
  - [Stale Directory Consensus](#incident-stale-directory-consensus)
  - [Guard Node Issues](#incident-guard-node-issues)
- [Post-Incident Review](#post-incident-review)
- [Escalation Procedures](#escalation-procedures)

---

## Incident Classification

### Severity Levels

| Severity | Impact | Response Time | Examples |
|----------|--------|---------------|----------|
| **P1 - Critical** | Complete service outage | Immediate | No circuits, service down |
| **P2 - High** | Significant degradation | < 30 min | High failure rate, slow connections |
| **P3 - Medium** | Partial impact | < 4 hours | Degraded performance, warnings |
| **P4 - Low** | Minimal impact | < 24 hours | Minor issues, cosmetic |

### Incident Detection Sources

- Health check alerts (`/health` returning unhealthy)
- Prometheus alerting rules
- User reports
- Log monitoring (errors/warnings)
- Metrics anomalies

---

## Initial Response

### 1. Acknowledge and Assess

```bash
# Step 1: Check current health status
curl -s http://localhost:9052/health | jq '.'

# Step 2: Get current metrics snapshot
curl -s http://localhost:9052/metrics/json | jq '.'

# Step 3: Check service status
sudo systemctl status tor-client

# Step 4: Review recent logs
sudo journalctl -u tor-client -n 100 --no-pager | tail -50
```

### 2. Initial Categorization

Ask these questions:
1. Is the service running? (Process exists)
2. Are circuits being built? (ActiveCircuits > 0)
3. Is SOCKS proxy responding? (Port 9050 accepting connections)
4. Are there error patterns in logs?

### 3. Communication Template

```
INCIDENT OPENED
Time: [timestamp]
Severity: [P1/P2/P3/P4]
Impact: [description]
Status: Investigating
Initial findings: [brief summary]
Next update: [time]
```

---

## Incident Playbooks

### INCIDENT: No Active Circuits

**Severity**: P1 - Critical  
**Symptoms**: `ActiveCircuits = 0`, SOCKS requests timing out

#### Diagnosis

```bash
# 1. Confirm no active circuits
curl -s http://localhost:9052/metrics/json | jq '.ActiveCircuits'

# 2. Check circuit build attempts
curl -s http://localhost:9052/metrics/json | jq '{builds: .CircuitBuilds, success: .CircuitBuildSuccess, failure: .CircuitBuildFailure}'

# 3. Check connection status
curl -s http://localhost:9052/metrics/json | jq '{attempts: .ConnectionAttempts, success: .ConnectionSuccess, failures: .ConnectionFailures}'

# 4. Review circuit build errors in logs
sudo journalctl -u tor-client -n 200 | grep -i "circuit\|build"
```

#### Resolution Steps

**Step 1: Check network connectivity**
```bash
# Test if Tor relays are reachable
nc -zv 131.188.40.189 9001 -w 5
curl -I https://www.torproject.org/ --max-time 10
```

If network is unreachable → Check firewall, DNS, network configuration

**Step 2: Check guard nodes**
```bash
# Inspect guard state
cat /var/lib/tor-client/guard_state.json | jq '.guards | length'

# If guards are failing, reset them
sudo systemctl stop tor-client
rm /var/lib/tor-client/guard_state.json
sudo systemctl start tor-client
```

**Step 3: Restart service**
```bash
sudo systemctl restart tor-client

# Wait for circuits (up to 2 minutes)
for i in {1..24}; do
  circuits=$(curl -s http://localhost:9052/metrics/json | jq '.ActiveCircuits' 2>/dev/null)
  if [ "$circuits" -gt 0 ] 2>/dev/null; then
    echo "Circuits restored: $circuits"
    break
  fi
  echo "Waiting for circuits... ($i/24)"
  sleep 5
done
```

**Step 4: If still failing, enable debug logging**
```bash
# Restart with debug logging
sudo systemctl stop tor-client
./bin/tor-client -config /etc/tor/torrc -log-level debug 2>&1 | tee /tmp/tor-debug.log &
DEBUG_PID=$!

# Analyze debug output after waiting for data
sleep 30
grep -E "ERROR|WARN|circuit|connect" /tmp/tor-debug.log

# When done debugging, clean up the background process
kill $DEBUG_PID 2>/dev/null
```

#### Escalation Trigger

Escalate if circuits not restored after:
- 5 minutes of troubleshooting
- Network connectivity confirmed but builds still failing

---

### INCIDENT: High Circuit Build Failure Rate

**Severity**: P2 - High  
**Symptoms**: Circuit build success rate < 50%, increased latency

#### Diagnosis

```bash
# 1. Calculate failure rate
curl -s http://localhost:9052/metrics/json | jq '{
  success: .CircuitBuildSuccess,
  failure: .CircuitBuildFailure,
  rate: ((.CircuitBuildSuccess / (.CircuitBuildSuccess + .CircuitBuildFailure + 0.001)) * 100 | floor)
}'

# 2. Check which stage is failing
sudo journalctl -u tor-client -n 500 | grep "circuit" | grep -i "fail\|error\|timeout"

# 3. Check connection success rate
curl -s http://localhost:9052/metrics/json | jq '{
  conn_success: .ConnectionSuccess,
  conn_failure: .ConnectionFailures,
  conn_rate: ((.ConnectionSuccess / (.ConnectionAttempts + 0.001)) * 100 | floor)
}'
```

#### Resolution Steps

**Step 1: Check for TLS handshake failures**
```bash
# Look for TLS errors
sudo journalctl -u tor-client -n 200 | grep -i "tls\|certificate\|handshake"

# If TLS issues, check system time
timedatectl status
```

**Step 2: Check guard node health**
```bash
# Verify guards are responding
cat /var/lib/tor-client/guard_state.json | jq '.guards[] | {address, confirmed, last_used}'

# If guards are stale (last_used > 24h ago), consider reset
```

**Step 3: Increase circuit build timeout (if network is slow)**
```bash
# Temporarily increase timeout
# In /etc/tor/torrc:
# CircuitBuildTimeout 120s

sudo systemctl restart tor-client
```

**Step 4: Check for relay issues**
```bash
# If specific relays are failing, check if they're in consensus
# Use control port to get circuit info
echo "GETINFO circuit-status" | nc 127.0.0.1 9051
```

#### Recovery Criteria

- Build success rate > 80% for 5 minutes
- No new TLS/connection errors in logs

---

### INCIDENT: SOCKS Proxy Unresponsive

**Severity**: P1 - Critical  
**Symptoms**: Port 9050 not responding, connection refused/timeout

#### Diagnosis

```bash
# 1. Check if port is listening
ss -tuln | grep 9050

# 2. Check if process is running
pgrep -a tor-client

# 3. Test connection
nc -zv 127.0.0.1 9050

# 4. Check for resource exhaustion
ulimit -n
ls /proc/$(pgrep tor-client)/fd | wc -l
```

#### Resolution Steps

**Step 1: If port not listening but process running**
```bash
# Check logs for SOCKS server errors
sudo journalctl -u tor-client -n 100 | grep -i "socks\|listen\|bind"

# Restart service
sudo systemctl restart tor-client
```

**Step 2: If process not running**
```bash
# Check why it stopped
sudo journalctl -u tor-client -n 50 --no-pager

# Check for crash dumps
ls /var/crash/*tor* 2>/dev/null

# Restart service
sudo systemctl start tor-client
```

**Step 3: If port conflict**
```bash
# Check what's using the port
sudo lsof -i :9050

# Kill conflicting process or change port
```

**Step 4: If file descriptor exhaustion**
```bash
# Increase limits
ulimit -n 8192

# Or permanently in /etc/security/limits.conf:
# tor-client soft nofile 8192
# tor-client hard nofile 16384

sudo systemctl restart tor-client
```

---

### INCIDENT: High Memory Usage

**Severity**: P2 - High  
**Symptoms**: Memory usage > 150MB or growing unbounded

#### Diagnosis

```bash
# 1. Check current memory usage
ps aux | grep tor-client | awk '{print "PID: " $2 ", RSS: " $6/1024 " MB"}'

# 2. Check active resources
curl -s http://localhost:9052/metrics/json | jq '{
  circuits: .ActiveCircuits,
  connections: .ActiveConnections,
  streams: .ActiveStreams
}'

# 3. Check for memory trend (if monitoring is available)
# Review metrics over time

# 4. Check for goroutine leak (if pprof enabled)
# curl http://localhost:6060/debug/pprof/goroutine?debug=1 | head -50
```

#### Resolution Steps

**Step 1: Reduce circuit pool size**
```bash
# Edit /etc/tor/torrc:
# CircuitPoolMinSize 2
# CircuitPoolMaxSize 5

sudo systemctl restart tor-client
```

**Step 2: Force circuit rotation**
```bash
# Signal to use new circuits (drops old ones)
echo "SIGNAL NEWNYM" | nc 127.0.0.1 9051
```

**Step 3: Restart service (if memory doesn't decrease)**
```bash
sudo systemctl restart tor-client
```

**Step 4: Monitor memory after restart**
```bash
# Watch memory usage
watch -n 5 'ps aux | grep tor-client | awk "{print \$6/1024 \" MB\"}"'
```

---

### INCIDENT: Service Not Starting

**Severity**: P1 - Critical  
**Symptoms**: Service fails to start, immediate exit

#### Diagnosis

```bash
# 1. Check service status
sudo systemctl status tor-client

# 2. Check startup logs
sudo journalctl -u tor-client -n 50 --no-pager

# 3. Try manual start
./bin/tor-client -config /etc/tor/torrc 2>&1
```

#### Resolution Steps

**Step 1: Configuration error**
```bash
# Validate configuration
./bin/tor-config-validator -config /etc/tor/torrc -verbose

# If validation fails, fix configuration
```

**Step 2: Port already in use**
```bash
# Check ports
sudo lsof -i :9050
sudo lsof -i :9051
sudo lsof -i :9052

# Kill conflicting process or change port in config
```

**Step 3: Permission denied**
```bash
# Check data directory permissions
ls -la /var/lib/tor-client
ls -la /etc/tor/torrc

# Fix permissions
sudo chown -R tor-client:tor-client /var/lib/tor-client
sudo chmod 700 /var/lib/tor-client
sudo chmod 600 /etc/tor/torrc
```

**Step 4: Binary or dependency issues**
```bash
# Verify binary
./bin/tor-client -version

# Rebuild if necessary
make clean && make build
```

---

### INCIDENT: Connection Exhaustion

**Severity**: P2 - High  
**Symptoms**: "too many open files" errors, connection failures

#### Diagnosis

```bash
# 1. Check file descriptor usage
ls /proc/$(pgrep tor-client)/fd | wc -l
ulimit -n

# 2. Check connection metrics
curl -s http://localhost:9052/metrics/json | jq '{
  connections: .ActiveConnections,
  socks: .SocksConnections
}'

# 3. Look for connection leak
sudo journalctl -u tor-client -n 200 | grep -i "too many\|file descriptor\|connection"
```

#### Resolution Steps

**Step 1: Increase file descriptor limits**
```bash
# Temporarily
ulimit -n 8192

# Permanently in /etc/security/limits.conf:
# tor-client soft nofile 8192
# tor-client hard nofile 16384

# Or in systemd unit:
# [Service]
# LimitNOFILE=8192

sudo systemctl restart tor-client
```

**Step 2: Reduce connection pool**
```bash
# Edit /etc/tor/torrc:
# ConnectionPoolMaxIdle 3
# CircuitPoolMaxSize 5
# ConnLimit 500

sudo systemctl restart tor-client
```

**Step 3: Force connection cleanup**
```bash
# Signal new identity to reset connections
echo "SIGNAL NEWNYM" | nc 127.0.0.1 9051
```

---

### INCIDENT: Stale Directory Consensus

**Severity**: P2 - High  
**Symptoms**: Health check shows "Directory consensus is stale"

#### Diagnosis

```bash
# 1. Check directory health
curl -s http://localhost:9052/health | jq '.components.directory'

# 2. Check logs for directory errors
sudo journalctl -u tor-client -n 200 | grep -i "consensus\|directory\|fetch"

# 3. Test directory server connectivity
curl -I http://128.31.0.34:9131 --max-time 10
```

#### Resolution Steps

**Step 1: Force consensus refresh**
```bash
# Restart to trigger fresh download
sudo systemctl restart tor-client

# Wait for consensus download (30-60 seconds)
sleep 60
curl -s http://localhost:9052/health | jq '.components.directory'
```

**Step 2: Check network connectivity to directory servers**
```bash
# Test Tor directory authorities
nc -zv 128.31.0.34 9131 -w 5
nc -zv 86.59.21.38 80 -w 5
```

**Step 3: Clear cached consensus**
```bash
sudo systemctl stop tor-client
rm -rf /var/lib/tor-client/consensus/
sudo systemctl start tor-client
```

---

### INCIDENT: Guard Node Issues

**Severity**: P3 - Medium  
**Symptoms**: Guard-related warnings in logs, circuit build delays

#### Diagnosis

```bash
# 1. Check guard status
cat /var/lib/tor-client/guard_state.json | jq '.guards[] | {address, confirmed, last_used}'

# 2. Check metrics
curl -s http://localhost:9052/metrics/json | jq '{active: .GuardsActive, confirmed: .GuardsConfirmed}'

# 3. Look for guard errors
sudo journalctl -u tor-client -n 200 | grep -i "guard"
```

#### Resolution Steps

**Step 1: If guards not confirming**
```bash
# Guards take 1-4 weeks to confirm normally
# Check if at least some circuits are being built

curl -s http://localhost:9052/metrics/json | jq '.CircuitBuildSuccess'
```

**Step 2: If guards are unreachable**
```bash
# Reset guard state
sudo systemctl stop tor-client
rm /var/lib/tor-client/guard_state.json
sudo systemctl start tor-client
```

**Step 3: Verify new guards are selected**
```bash
# Wait 60 seconds for new guards
sleep 60
cat /var/lib/tor-client/guard_state.json | jq '.guards | length'
```

---

## Post-Incident Review

### Review Checklist

After resolving any P1 or P2 incident:

- [ ] Root cause identified
- [ ] Timeline documented
- [ ] Resolution steps documented
- [ ] Monitoring/alerting gaps identified
- [ ] Prevention measures proposed
- [ ] Runbook updates made (if applicable)

### Post-Incident Report Template

```markdown
## Incident Report

**Incident ID**: [ID]
**Date/Time**: [Start] - [End]
**Duration**: [minutes/hours]
**Severity**: [P1/P2/P3/P4]
**Responders**: [names]

### Summary
[Brief description of what happened]

### Timeline
- HH:MM - [Event]
- HH:MM - [Event]

### Root Cause
[What caused the incident]

### Resolution
[How it was fixed]

### Impact
- [Number of affected users/requests]
- [Duration of outage/degradation]

### Lessons Learned
- [What went well]
- [What could be improved]

### Action Items
- [ ] [Action item with owner and due date]
```

---

## Escalation Procedures

### Escalation Matrix

| Severity | Primary | Secondary | Executive |
|----------|---------|-----------|-----------|
| P1 | Immediate | After 15 min | After 30 min |
| P2 | 15 min | After 1 hour | After 4 hours |
| P3 | 1 hour | After 4 hours | N/A |
| P4 | Next business day | N/A | N/A |

### When to Escalate

Escalate immediately if:
- Root cause cannot be determined within expected timeframe
- Resolution requires access/knowledge you don't have
- Multiple systems are affected
- Data integrity may be compromised
- The issue is spreading or worsening

### Escalation Template

```
ESCALATION REQUEST
Incident: [description]
Severity: [P1/P2]
Started: [time]
Current Status: [status]
Actions Taken: [summary]
Blocking Issue: [why escalation is needed]
Request: [what you need]
```

---

## See Also

- [RUNBOOK.md](RUNBOOK.md) - Operational procedures
- [MONITORING_GUIDE.md](MONITORING_GUIDE.md) - Monitoring setup
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Common issues
- [PRODUCTION.md](PRODUCTION.md) - Production deployment

# go-tor Operational Runbook

## ⚠️ IMPORTANT DISCLAIMER

**THIS IS UNOFFICIAL SOFTWARE** developed without the supervision or endorsement of [The Tor Project](https://www.torproject.org/). This software should **NOT** be used in production or for any real anonymity needs.

**For actual privacy and security:**
- **Users**: Use [Tor Browser](https://www.torproject.org/download/)
- **Developers**: Use [Arti](https://gitlab.torproject.org/tpo/core/arti)

This runbook is for **testing and development environments only**.

---

## Table of Contents

- [Daily Operations](#daily-operations)
- [Startup and Shutdown Procedures](#startup-and-shutdown-procedures)
- [Health Monitoring](#health-monitoring)
- [Configuration Management](#configuration-management)
- [Data and State Management](#data-and-state-management)
- [Performance Tuning](#performance-tuning)
- [Backup and Recovery](#backup-and-recovery)
- [Capacity Planning](#capacity-planning)

---

## Daily Operations

### 1. Health Check Routine

Perform these checks daily to ensure system health:

```bash
# 1. Verify service is running
curl -s http://localhost:9052/health | jq '.status'
# Expected output: "healthy"

# 2. Check active circuits
curl -s http://localhost:9052/metrics/json | jq '.ActiveCircuits'
# Expected: 2-10 circuits (depends on configuration)

# 3. Verify SOCKS proxy is responding
curl --socks5 127.0.0.1:9050 -m 30 https://check.torproject.org/api/ip 2>/dev/null | jq '.IsTor'
# Expected output: true

# 4. Check circuit build success rate
curl -s http://localhost:9052/metrics/json | jq '{
  success: .CircuitBuildSuccess, 
  failure: .CircuitBuildFailure
} | . + {rate: ((.success / (.success + .failure + 0.001)) * 100 | floor)}'
# Expected: rate > 80%

# 5. Verify guard node status
curl -s http://localhost:9052/metrics/json | jq '{active: .GuardsActive, confirmed: .GuardsConfirmed}'
# Expected: 3 active guards, at least 2 confirmed
```

### 2. Log Review

Check logs for errors and warnings:

```bash
# Review recent errors (last 100 lines)
tail -100 /var/log/tor-client/tor-client.log | grep -E "ERROR|WARN"

# Count errors by type in last 24 hours
grep "ERROR" /var/log/tor-client/tor-client.log | tail -1000 | awk '{print $4}' | sort | uniq -c | sort -rn

# Check for circuit build failures
grep "circuit build failed" /var/log/tor-client/tor-client.log | tail -20
```

### 3. Resource Monitoring

Monitor system resources:

```bash
# Check memory usage
ps aux | grep tor-client | awk '{print "RSS: " $6/1024 " MB, VSZ: " $5/1024 " MB"}'
# Expected: RSS < 100MB for typical workloads

# Check file descriptors
ls -l /proc/$(pgrep tor-client)/fd | wc -l
# Expected: < 500 for normal operation

# Check network connections
ss -tuln | grep -E "9050|9051|9052"
```

---

## Startup and Shutdown Procedures

### Standard Startup

```bash
# 1. Verify configuration
./bin/tor-config-validator -config /etc/tor/torrc -verbose

# 2. Start the service
./bin/tor-client -config /etc/tor/torrc -data-dir /var/lib/tor-client

# 3. Wait for bootstrap (up to 90 seconds for first run)
for i in {1..18}; do
  status=$(curl -s http://localhost:9052/health | jq -r '.status' 2>/dev/null)
  if [ "$status" = "healthy" ]; then
    echo "Bootstrap complete"
    break
  fi
  echo "Waiting for bootstrap... ($i/18)"
  sleep 5
done

# 4. Verify circuits are built
curl -s http://localhost:9052/metrics/json | jq '.ActiveCircuits'
```

### Graceful Shutdown

```bash
# 1. Check for active connections
curl -s http://localhost:9052/metrics/json | jq '.ActiveStreams'

# 2. Send shutdown signal via control port
echo "SIGNAL SHUTDOWN" | nc 127.0.0.1 9051

# 3. Wait for graceful shutdown (default: 10 seconds)
sleep 12

# 4. Verify process has stopped
pgrep tor-client && echo "WARNING: Process still running" || echo "Shutdown complete"

# 5. If process still running, force kill (last resort)
# pkill -9 tor-client
```

### Systemd Service Management

If using systemd:

```bash
# Start service
sudo systemctl start tor-client

# Stop service (graceful)
sudo systemctl stop tor-client

# Restart service
sudo systemctl restart tor-client

# Check status
sudo systemctl status tor-client

# View recent logs
sudo journalctl -u tor-client -n 50 --no-pager
```

### Docker Container Management

```bash
# Start container
docker-compose up -d

# Stop container (graceful)
docker-compose stop

# Restart container
docker-compose restart

# View logs
docker-compose logs -f --tail=100

# Check container health
docker inspect --format='{{.State.Health.Status}}' tor-client
```

---

## Health Monitoring

### Health Check Endpoints

| Endpoint | Description | Expected Response |
|----------|-------------|-------------------|
| `GET /health` | Overall health status | 200 OK with JSON |
| `GET /metrics` | Prometheus metrics | 200 OK with text |
| `GET /metrics/json` | JSON metrics | 200 OK with JSON |
| `GET /debug/metrics` | HTML dashboard | 200 OK with HTML |

### Health Status Interpretation

| Status | HTTP Code | Meaning | Action Required |
|--------|-----------|---------|-----------------|
| `healthy` | 200 | All components functioning | None |
| `degraded` | 200 | Reduced capacity | Monitor closely |
| `unhealthy` | 503 | Critical failure | Immediate action |

### Component Health Checks

```bash
# Check individual component health
curl -s http://localhost:9052/health | jq '.components'

# Expected components:
# - circuits: Circuit pool health
# - connections: Network connection health
# - directory: Consensus status
```

### Circuit Health Thresholds

| Metric | Healthy | Degraded | Unhealthy |
|--------|---------|----------|-----------|
| Active Circuits | ≥ 3 | 1-2 | 0 |
| Build Success Rate | ≥ 80% | 50-80% | < 50% |
| Avg Build Time | < 5s | 5-10s | > 10s |

### Connection Health Thresholds

| Metric | Healthy | Degraded | Unhealthy |
|--------|---------|----------|-----------|
| Open Connections | ≥ 3 | 1-2 | 0 |
| Failure Rate | < 10% | 10-30% | > 30% |
| Avg Latency | < 500ms | 500-2000ms | > 2000ms |

---

## Configuration Management

### Configuration File Location

Default locations:
- **Linux**: `/etc/tor/torrc` or `~/.tor/torrc`
- **macOS**: `~/Library/Application Support/go-tor/torrc`
- **Docker**: `/app/config/torrc` (mounted volume)

### Configuration Validation

Always validate configuration before applying:

```bash
# Validate configuration file
./bin/tor-config-validator -config /etc/tor/torrc -verbose

# Generate sample configuration
./bin/tor-config-validator -generate -output /etc/tor/torrc.sample

# Compare configurations
diff /etc/tor/torrc /etc/tor/torrc.sample
```

### Common Configuration Changes

#### 1. Change SOCKS Port

```
# In torrc:
SocksPort 9150

# Verify:
ss -tuln | grep 9150
```

#### 2. Enable Metrics

```
# In torrc:
MetricsPort 9052
EnableMetrics 1

# Verify:
curl http://localhost:9052/health
```

#### 3. Adjust Circuit Pool

```
# In torrc:
CircuitPoolMinSize 3
CircuitPoolMaxSize 15
EnableCircuitPrebuilding 1
```

#### 4. Configure Circuit Isolation

```
# In torrc:
IsolationLevel destination
IsolateDestinations 1
```

### Configuration Reload

**Note**: Hot reload is not currently supported. Restart required for configuration changes.

```bash
# Apply configuration changes
sudo systemctl restart tor-client
```

---

## Data and State Management

### Data Directory Structure

```
/var/lib/tor-client/
├── guard_state.json      # Persistent guard nodes
├── consensus/            # Directory consensus cache
├── descriptors/          # Relay descriptors cache
└── onion_services/       # Onion service keys and state
```

### Guard State Management

Guard nodes are persisted for improved security and performance:

```bash
# View current guard state
cat /var/lib/tor-client/guard_state.json | jq '.'

# Expected structure:
# {
#   "guards": [...],
#   "last_updated": "2025-..."
# }

# Guard state health check
guard_count=$(cat /var/lib/tor-client/guard_state.json | jq '.guards | length')
confirmed=$(cat /var/lib/tor-client/guard_state.json | jq '[.guards[] | select(.confirmed == true)] | length')
echo "Guards: $guard_count, Confirmed: $confirmed"
```

### Reset Guard State

**Warning**: Only do this if guards are causing connection issues.

```bash
# 1. Stop the service
sudo systemctl stop tor-client

# 2. Remove guard state
rm /var/lib/tor-client/guard_state.json

# 3. Start service (new guards will be selected)
sudo systemctl start tor-client
```

### Clear All State

**Warning**: This resets all persistent state.

```bash
# 1. Stop the service
sudo systemctl stop tor-client

# 2. Backup data directory (recommended)
cp -r /var/lib/tor-client /var/lib/tor-client.backup.$(date +%Y%m%d)

# 3. Clear state
rm -rf /var/lib/tor-client/*

# 4. Start service
sudo systemctl start tor-client
```

---

## Performance Tuning

### Resource Profiles

| Profile | Memory | Circuits | Use Case |
|---------|--------|----------|----------|
| Minimal | 30MB | 2-3 | Embedded/IoT |
| Standard | 50MB | 5-10 | General use |
| High | 100MB+ | 10-20 | High throughput |

### Minimal Profile Configuration

```
# For embedded systems or low-memory environments
CircuitPoolMinSize 1
CircuitPoolMaxSize 3
ConnectionPoolMaxIdle 2
EnableCircuitPrebuilding 0
```

### High Throughput Configuration

```
# For servers handling many concurrent connections
CircuitPoolMinSize 5
CircuitPoolMaxSize 20
ConnectionPoolMaxIdle 10
EnableCircuitPrebuilding 1
EnableConnectionPooling 1
```

### Circuit Build Optimization

```bash
# Monitor circuit build times
watch -n 5 'curl -s http://localhost:9052/metrics/json | jq "{avg: .CircuitBuildTimeAvg, p95: .CircuitBuildTimeP95}"'

# Reduce build time by prebuilding circuits
# In torrc:
EnableCircuitPrebuilding 1
CircuitPoolMinSize 3
```

### Connection Tuning

```bash
# Check connection pool status
curl -s http://localhost:9052/metrics/json | jq '{active: .ActiveConnections, attempts: .ConnectionAttempts, failures: .ConnectionFailures}'

# Tune connection pool in torrc:
EnableConnectionPooling 1
ConnectionPoolMaxIdle 5
ConnectionPoolMaxLife 10m
```

---

## Backup and Recovery

### What to Backup

| Data | Location | Frequency | Priority |
|------|----------|-----------|----------|
| Configuration | `/etc/tor/torrc` | On change | Critical |
| Guard state | `/var/lib/tor-client/guard_state.json` | Daily | High |
| Onion service keys | `/var/lib/tor-client/onion_services/` | On change | Critical |

### Backup Procedure

```bash
#!/bin/bash
# Backup script for go-tor

BACKUP_DIR="/var/backups/tor-client"
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/tor-backup-$DATE.tar.gz"

mkdir -p "$BACKUP_DIR"

# Create backup
tar -czvf "$BACKUP_FILE" \
    /etc/tor/torrc \
    /var/lib/tor-client/guard_state.json \
    /var/lib/tor-client/onion_services/ \
    2>/dev/null

# Verify backup
tar -tzf "$BACKUP_FILE" > /dev/null && echo "Backup created: $BACKUP_FILE"

# Cleanup old backups (keep 30 days)
find "$BACKUP_DIR" -name "tor-backup-*.tar.gz" -mtime +30 -delete
```

### Recovery Procedure

```bash
#!/bin/bash
# Recovery script for go-tor

BACKUP_FILE="$1"

if [ -z "$BACKUP_FILE" ]; then
    echo "Usage: $0 <backup-file>"
    exit 1
fi

# Stop service
sudo systemctl stop tor-client

# Extract backup
tar -xzvf "$BACKUP_FILE" -C /

# Fix permissions
chmod 700 /var/lib/tor-client
chmod 600 /var/lib/tor-client/guard_state.json
chown -R tor-client:tor-client /var/lib/tor-client

# Start service
sudo systemctl start tor-client
```

---

## Capacity Planning

### Resource Requirements per Load Level

| Concurrent Streams | Memory | CPU | Circuits |
|--------------------|--------|-----|----------|
| 10 | 50MB | 5% | 3 |
| 50 | 80MB | 15% | 5-8 |
| 100 | 120MB | 25% | 10-15 |
| 200+ | 200MB+ | 50%+ | 15-20 |

### Scaling Guidelines

1. **Memory**: Plan for ~1MB per active stream
2. **Circuits**: Plan for ~1 circuit per 10-20 concurrent streams
3. **Connections**: Each circuit uses 3 connections (guard, middle, exit)

### Monitoring for Capacity

```bash
# Current utilization
curl -s http://localhost:9052/metrics/json | jq '{
  streams: .ActiveStreams,
  circuits: .ActiveCircuits,
  connections: .ActiveConnections,
  memory_estimate_mb: (.ActiveStreams * 1 + .ActiveCircuits * 5)
}'

# Trend analysis (collect over time)
while true; do
  echo "$(date -Iseconds),$(curl -s http://localhost:9052/metrics/json | jq -c '.')" >> /var/log/tor-metrics.csv
  sleep 60
done
```

### Scaling Thresholds

| Metric | Scale Up When | Scale Down When |
|--------|--------------|-----------------|
| Memory Usage | > 80% limit | < 30% for 1 hour |
| Circuit Utilization | Avg streams/circuit > 15 | < 5 for 1 hour |
| Circuit Build Failures | > 20% rate | N/A |

---

## Quick Reference Commands

```bash
# Health check
curl -s http://localhost:9052/health | jq '.status'

# Metrics summary
curl -s http://localhost:9052/metrics/json | jq '{circuits: .ActiveCircuits, streams: .ActiveStreams, uptime: .UptimeSeconds}'

# Circuit status via control port
echo "GETINFO circuit-status" | nc 127.0.0.1 9051

# Force new identity (new circuits)
echo "SIGNAL NEWNYM" | nc 127.0.0.1 9051

# Check if connected to Tor network
curl --socks5 127.0.0.1:9050 -m 30 https://check.torproject.org/api/ip

# View configuration
./bin/tor-config-validator -config /etc/tor/torrc -verbose

# Debug logging (temporary)
./bin/tor-client -log-level debug 2>&1 | tee /tmp/tor-debug.log
```

---

## See Also

- [INCIDENT_RESPONSE.md](INCIDENT_RESPONSE.md) - Incident response procedures
- [MONITORING_GUIDE.md](MONITORING_GUIDE.md) - Detailed monitoring setup
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Common issues and solutions
- [PRODUCTION.md](PRODUCTION.md) - Production deployment guide
- [METRICS.md](METRICS.md) - Metrics reference

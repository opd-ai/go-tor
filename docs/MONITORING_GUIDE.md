# go-tor Monitoring Guide

## ⚠️ IMPORTANT DISCLAIMER

**THIS IS UNOFFICIAL SOFTWARE** developed without the supervision or endorsement of [The Tor Project](https://www.torproject.org/). This software should **NOT** be used in production or for any real anonymity needs.

**For actual privacy and security:**
- **Users**: Use [Tor Browser](https://www.torproject.org/download/)
- **Developers**: Use [Arti](https://gitlab.torproject.org/tpo/core/arti)

This guide is for **testing and development environments only**.

---

## Table of Contents

- [Overview](#overview)
- [Enabling Metrics](#enabling-metrics)
- [Metrics Reference](#metrics-reference)
- [Prometheus Integration](#prometheus-integration)
- [Alerting Configuration](#alerting-configuration)
- [Dashboard Setup](#dashboard-setup)
- [Health Checks](#health-checks)
- [Log Monitoring](#log-monitoring)
- [Best Practices](#best-practices)

---

## Overview

go-tor provides comprehensive monitoring through:

1. **HTTP Metrics Server** - Prometheus and JSON endpoints
2. **Health Check API** - Component-level health status
3. **Structured Logging** - slog-based logging with levels
4. **Control Protocol** - Real-time event notifications

### Monitoring Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     go-tor Client                       │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  │
│  │ SOCKS Proxy │  │   Control   │  │  HTTP Metrics   │  │
│  │   :9050     │  │   :9051     │  │     :9052       │  │
│  └─────────────┘  └─────────────┘  └─────────────────┘  │
│                                           │             │
│  ┌────────────────────────────────────────┘             │
│  │                                                      │
│  ▼                                                      │
│  ┌──────────────────────────────────────────────────┐   │
│  │              Metrics Endpoints                    │   │
│  │  /metrics      - Prometheus format               │   │
│  │  /metrics/json - JSON format                     │   │
│  │  /health       - Health check                    │   │
│  │  /debug/metrics - HTML dashboard                 │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
           │                    │
           ▼                    ▼
    ┌─────────────┐      ┌─────────────┐
    │ Prometheus  │      │   Grafana   │
    │  Scraper    │─────▶│  Dashboard  │
    └─────────────┘      └─────────────┘
           │
           ▼
    ┌─────────────┐
    │ AlertManager│
    └─────────────┘
```

---

## Enabling Metrics

### Via Command Line

```bash
# Enable metrics on port 9052
./bin/tor-client --metrics-port 9052
```

### Via Configuration File

```
# In torrc
MetricsPort 9052
EnableMetrics 1
```

### Via Code

```go
cfg := config.DefaultConfig()
cfg.MetricsPort = 9052
cfg.EnableMetrics = true

torClient, err := client.New(cfg, logger)
```

### Verify Metrics Are Enabled

```bash
# Check if metrics server is responding
curl -s http://localhost:9052/ | head -5

# Test health endpoint
curl -s http://localhost:9052/health | jq '.status'

# View Prometheus metrics
curl -s http://localhost:9052/metrics | head -20
```

---

## Metrics Reference

### Circuit Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `tor_circuit_builds_total` | Counter | Total circuit build attempts |
| `tor_circuit_build_success_total` | Counter | Successful circuit builds |
| `tor_circuit_build_failures_total` | Counter | Failed circuit builds |
| `tor_circuit_build_duration_seconds_avg` | Gauge | Average build duration |
| `tor_circuit_build_duration_seconds_p95` | Gauge | 95th percentile build duration |
| `tor_active_circuits` | Gauge | Currently active circuits |

**Key Indicators:**
- Build success rate: `tor_circuit_build_success_total / tor_circuit_builds_total`
- Circuit availability: `tor_active_circuits >= 2`

### Connection Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `tor_connection_attempts_total` | Counter | Total connection attempts |
| `tor_connection_success_total` | Counter | Successful connections |
| `tor_connection_failures_total` | Counter | Failed connections |
| `tor_connection_retries_total` | Counter | Connection retry attempts |
| `tor_active_connections` | Gauge | Currently active connections |
| `tor_tls_handshake_seconds_avg` | Gauge | Average TLS handshake duration |

**Key Indicators:**
- Connection success rate: `tor_connection_success_total / tor_connection_attempts_total`
- Retry ratio: `tor_connection_retries_total / tor_connection_attempts_total`

### Stream Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `tor_streams_created_total` | Counter | Total streams created |
| `tor_streams_closed_total` | Counter | Total streams closed |
| `tor_stream_failures_total` | Counter | Total stream failures |
| `tor_active_streams` | Gauge | Currently active streams |
| `tor_stream_data_bytes_total` | Counter | Total bytes transferred |

**Key Indicators:**
- Stream churn: `rate(tor_streams_created_total[5m])`
- Active load: `tor_active_streams`

### Guard Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `tor_guards_active` | Gauge | Active guard nodes |
| `tor_guards_confirmed` | Gauge | Confirmed guard nodes |

**Key Indicators:**
- Guard health: `tor_guards_confirmed >= 2`

### SOCKS Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `tor_socks_connections_total` | Counter | Total SOCKS connections |
| `tor_socks_requests_total` | Counter | Total SOCKS requests |
| `tor_socks_errors_total` | Counter | Total SOCKS errors |

**Key Indicators:**
- SOCKS error rate: `rate(tor_socks_errors_total[5m]) / rate(tor_socks_requests_total[5m])`

### System Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `tor_uptime_seconds` | Gauge | Client uptime in seconds |

---

## Prometheus Integration

### Prometheus Configuration

Add to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'go-tor'
    scrape_interval: 15s
    scrape_timeout: 10s
    static_configs:
      - targets: ['localhost:9052']
        labels:
          instance: 'tor-client-1'
          environment: 'development'
```

### Service Discovery (Kubernetes)

```yaml
scrape_configs:
  - job_name: 'go-tor-kubernetes'
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        regex: go-tor
        action: keep
      - source_labels: [__meta_kubernetes_pod_container_port_name]
        regex: metrics
        action: keep
```

### Useful Prometheus Queries

```promql
# Circuit build success rate (5 minute window)
sum(rate(tor_circuit_build_success_total[5m])) / sum(rate(tor_circuit_builds_total[5m])) * 100

# Active circuits trend
avg_over_time(tor_active_circuits[1h])

# Connection failure rate
sum(rate(tor_connection_failures_total[5m])) / sum(rate(tor_connection_attempts_total[5m])) * 100

# Average circuit build time
tor_circuit_build_duration_seconds_avg

# Streams per circuit
tor_active_streams / (tor_active_circuits + 0.001)

# Data throughput (bytes/sec)
rate(tor_stream_data_bytes_total[5m])

# SOCKS request rate
rate(tor_socks_requests_total[5m])

# Guard confirmation status
tor_guards_confirmed / tor_guards_active * 100
```

---

## Alerting Configuration

### Prometheus Alerting Rules

Create `tor-alerts.yml`:

```yaml
groups:
  - name: go-tor-alerts
    rules:
      # Critical: No active circuits
      - alert: TorNoActiveCircuits
        expr: tor_active_circuits == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "No active Tor circuits"
          description: "go-tor has no active circuits for 2 minutes. SOCKS proxy is non-functional."
          runbook_url: "https://example.com/docs/INCIDENT_RESPONSE.md#incident-no-active-circuits"

      # Critical: Service down
      - alert: TorServiceDown
        expr: up{job="go-tor"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "go-tor service is down"
          description: "go-tor metrics endpoint is not responding."

      # High: High circuit build failure rate
      - alert: TorHighCircuitFailureRate
        expr: |
          (sum(rate(tor_circuit_build_failures_total[5m])) / 
           sum(rate(tor_circuit_builds_total[5m]))) > 0.3
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High circuit build failure rate"
          description: "Circuit build failure rate is {{ $value | humanizePercentage }}."
          runbook_url: "https://example.com/docs/INCIDENT_RESPONSE.md#incident-high-circuit-build-failure-rate"

      # High: Low circuit count
      - alert: TorLowCircuitCount
        expr: tor_active_circuits < 2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Low active circuit count"
          description: "Only {{ $value }} active circuits. May impact performance."

      # High: High connection failure rate
      - alert: TorHighConnectionFailureRate
        expr: |
          (sum(rate(tor_connection_failures_total[5m])) / 
           sum(rate(tor_connection_attempts_total[5m]))) > 0.2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High connection failure rate"
          description: "Connection failure rate is {{ $value | humanizePercentage }}."

      # Medium: Slow circuit builds
      - alert: TorSlowCircuitBuilds
        expr: tor_circuit_build_duration_seconds_avg > 10
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Slow circuit build times"
          description: "Average circuit build time is {{ $value }}s."

      # Medium: Guard issues
      - alert: TorNoConfirmedGuards
        expr: tor_guards_confirmed == 0
        for: 30m
        labels:
          severity: warning
        annotations:
          summary: "No confirmed guard nodes"
          description: "No guard nodes have been confirmed. This may impact security."

      # Low: High SOCKS error rate
      - alert: TorHighSocksErrorRate
        expr: |
          (rate(tor_socks_errors_total[5m]) / 
           (rate(tor_socks_requests_total[5m]) + 0.001)) > 0.1
        for: 10m
        labels:
          severity: info
        annotations:
          summary: "Elevated SOCKS error rate"
          description: "SOCKS error rate is {{ $value | humanizePercentage }}."
```

### AlertManager Configuration

Example `alertmanager.yml`:

```yaml
route:
  receiver: 'default-receiver'
  group_by: ['alertname', 'severity']
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
  routes:
    - match:
        severity: critical
      receiver: 'pagerduty-critical'
      continue: true
    - match:
        severity: warning
      receiver: 'slack-warnings'

receivers:
  - name: 'default-receiver'
    email_configs:
      - to: 'team@example.com'

  - name: 'pagerduty-critical'
    pagerduty_configs:
      - service_key: '<service-key>'

  - name: 'slack-warnings'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/...'
        channel: '#tor-alerts'
        send_resolved: true
```

---

## Dashboard Setup

### Grafana Dashboard (JSON Export)

A minimal Grafana dashboard configuration:

```json
{
  "title": "go-tor Overview",
  "panels": [
    {
      "title": "Active Circuits",
      "type": "stat",
      "targets": [{"expr": "tor_active_circuits"}],
      "gridPos": {"x": 0, "y": 0, "w": 6, "h": 4}
    },
    {
      "title": "Circuit Build Success Rate",
      "type": "gauge",
      "targets": [{
        "expr": "sum(tor_circuit_build_success_total) / sum(tor_circuit_builds_total) * 100"
      }],
      "gridPos": {"x": 6, "y": 0, "w": 6, "h": 4}
    },
    {
      "title": "Active Streams",
      "type": "stat",
      "targets": [{"expr": "tor_active_streams"}],
      "gridPos": {"x": 12, "y": 0, "w": 6, "h": 4}
    },
    {
      "title": "Uptime",
      "type": "stat",
      "targets": [{"expr": "tor_uptime_seconds / 3600"}],
      "gridPos": {"x": 18, "y": 0, "w": 6, "h": 4}
    },
    {
      "title": "Circuit Build Rate",
      "type": "graph",
      "targets": [
        {"expr": "rate(tor_circuit_build_success_total[5m])", "legendFormat": "Success"},
        {"expr": "rate(tor_circuit_build_failures_total[5m])", "legendFormat": "Failure"}
      ],
      "gridPos": {"x": 0, "y": 4, "w": 12, "h": 8}
    },
    {
      "title": "Connection Metrics",
      "type": "graph",
      "targets": [
        {"expr": "tor_active_connections", "legendFormat": "Active"},
        {"expr": "rate(tor_connection_failures_total[5m])", "legendFormat": "Failures/s"}
      ],
      "gridPos": {"x": 12, "y": 4, "w": 12, "h": 8}
    }
  ]
}
```

### Key Dashboard Panels

| Panel | Query | Thresholds |
|-------|-------|------------|
| Active Circuits | `tor_active_circuits` | Green: ≥3, Yellow: 1-2, Red: 0 |
| Build Success Rate | `(success/total)*100` | Green: ≥80%, Yellow: 50-80%, Red: <50% |
| Active Connections | `tor_active_connections` | Informational |
| Circuit Build Time | `tor_circuit_build_duration_seconds_avg` | Green: <5s, Yellow: 5-10s, Red: >10s |
| Guard Status | `tor_guards_confirmed` | Green: ≥2, Yellow: 1, Red: 0 |

### Built-in Dashboard

Access the built-in HTML dashboard:

```bash
# Open in browser
open http://localhost:9052/debug/metrics
```

Features:
- Auto-refresh every 5 seconds
- Color-coded metrics
- No external dependencies

---

## Health Checks

### Health Endpoint

```bash
# Full health check
curl -s http://localhost:9052/health | jq '.'
```

Response structure:

```json
{
  "status": "healthy",
  "components": {
    "circuits": {
      "name": "circuits",
      "status": "healthy",
      "message": "Circuits functioning normally",
      "last_checked": "2025-10-20T12:00:00Z",
      "response_time_ms": 1,
      "details": {
        "active_circuits": 5,
        "min_required": 2
      }
    },
    "connections": {
      "name": "connections",
      "status": "healthy",
      "message": "Connections functioning normally"
    },
    "directory": {
      "name": "directory",
      "status": "healthy",
      "message": "Directory consensus is current"
    }
  },
  "timestamp": "2025-10-20T12:00:00Z",
  "uptime": 3600000000000
}
```

### Health Check Integration

#### Kubernetes Probes

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 9052
  initialDelaySeconds: 30
  periodSeconds: 10
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /health
    port: 9052
  initialDelaySeconds: 10
  periodSeconds: 5
```

#### Docker Health Check

```dockerfile
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
  CMD curl -f http://localhost:9052/health || exit 1
```

#### Load Balancer Health Check

Configure your load balancer to probe:
- **URL**: `http://<host>:9052/health`
- **Expected**: HTTP 200
- **Interval**: 10-30 seconds
- **Timeout**: 5 seconds
- **Unhealthy threshold**: 3 consecutive failures

### Scripted Health Check

```bash
#!/bin/bash
# health-check.sh

HEALTH_URL="${1:-http://localhost:9052/health}"
TIMEOUT=5

response=$(curl -s -w "\n%{http_code}" --max-time $TIMEOUT "$HEALTH_URL" 2>/dev/null)
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | head -n-1)

if [ "$http_code" = "200" ]; then
    status=$(echo "$body" | jq -r '.status')
    if [ "$status" = "healthy" ]; then
        echo "OK: Service is healthy"
        exit 0
    elif [ "$status" = "degraded" ]; then
        echo "WARNING: Service is degraded"
        echo "$body" | jq '.components'
        exit 1
    else
        echo "CRITICAL: Service is unhealthy"
        echo "$body" | jq '.components'
        exit 2
    fi
elif [ "$http_code" = "503" ]; then
    echo "CRITICAL: Service is unhealthy (HTTP 503)"
    exit 2
else
    echo "CRITICAL: Health check failed (HTTP $http_code)"
    exit 2
fi
```

---

## Log Monitoring

### Log Levels

| Level | Description | When to Use |
|-------|-------------|-------------|
| `debug` | Verbose debugging | Development, troubleshooting |
| `info` | Normal operations | Production default |
| `warn` | Warning conditions | Important but not critical |
| `error` | Error conditions | Requires attention |

### Log Format

go-tor uses structured logging (slog):

```
time=2025-10-20T12:00:00Z level=INFO msg="Circuit built successfully" component=circuit circuit_id=42 hops=3 duration=3.5s
time=2025-10-20T12:00:01Z level=WARN msg="Connection retry" component=connection address=1.2.3.4:9001 attempt=2
time=2025-10-20T12:00:02Z level=ERROR msg="Circuit build failed" component=circuit error="timeout" hop=2
```

### Log Aggregation Patterns

#### Grep for Errors

```bash
# All errors
grep "level=ERROR" /var/log/tor-client.log

# Circuit-specific errors
grep "level=ERROR.*circuit" /var/log/tor-client.log

# Connection failures
grep -E "level=(ERROR|WARN).*connection" /var/log/tor-client.log
```

#### Log Analysis with jq (JSON logs)

If logging in JSON format:

```bash
# Count errors by component
cat /var/log/tor-client.json | jq -s 'group_by(.component) | map({component: .[0].component, count: length})'

# Find slow circuit builds
cat /var/log/tor-client.json | jq 'select(.msg == "Circuit built" and .duration > 5)'
```

### Important Log Patterns to Monitor

| Pattern | Meaning | Action |
|---------|---------|--------|
| `circuit build failed` | Circuit construction failed | Check if > 30% |
| `connection timeout` | Relay unreachable | Check network |
| `TLS handshake failed` | TLS negotiation error | Check time sync |
| `consensus download failed` | Directory fetch error | Check connectivity |
| `guard persistence failed` | Can't save guard state | Check disk permissions |
| `too many open files` | File descriptor exhaustion | Increase limits |

---

## Best Practices

### 1. Monitoring Checklist

Before deployment, ensure:

- [ ] Metrics endpoint enabled and accessible
- [ ] Prometheus scraping configured
- [ ] Critical alerts defined (no circuits, service down)
- [ ] Warning alerts defined (high failure rate, slow builds)
- [ ] Dashboard created with key metrics
- [ ] Health check integrated with load balancer
- [ ] Log aggregation configured
- [ ] Runbook links added to alert annotations

### 2. Scrape Interval Recommendations

| Environment | Interval | Rationale |
|-------------|----------|-----------|
| Development | 5s | Rapid iteration |
| Testing | 15s | Balance detail/overhead |
| Production | 30s | Reduce overhead |

### 3. Metric Retention

| Metric Type | Retention | Downsampling |
|-------------|-----------|--------------|
| Raw metrics | 2 weeks | None |
| 5m averages | 3 months | After 2 weeks |
| 1h averages | 1 year | After 3 months |

### 4. Alert Tuning

Start conservative, then tighten:

1. **Week 1**: Alert only on critical (no circuits, service down)
2. **Week 2**: Add warning alerts, tune thresholds based on baseline
3. **Week 3+**: Refine based on false positives/negatives

### 5. Security Considerations

- Bind metrics server to localhost only
- Use reverse proxy with authentication for external access
- No sensitive data in metrics (no addresses, keys, etc.)
- Limit Prometheus scrape access via firewall

```nginx
# Secure metrics with nginx
server {
    listen 443 ssl;
    server_name metrics.example.com;
    
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    location /metrics {
        proxy_pass http://127.0.0.1:9052;
        auth_basic "Metrics";
        auth_basic_user_file /etc/nginx/.htpasswd;
    }
}
```

---

## Quick Reference

### Key Endpoints

| Endpoint | Purpose | Format |
|----------|---------|--------|
| `/metrics` | Prometheus scraping | text |
| `/metrics/json` | Programmatic access | JSON |
| `/health` | Health status | JSON |
| `/debug/metrics` | Human dashboard | HTML |

### Critical Metrics

```promql
# Must be > 0
tor_active_circuits

# Should be > 80%
sum(tor_circuit_build_success_total) / sum(tor_circuit_builds_total) * 100

# Should be > 0
up{job="go-tor"}
```

### Quick Health Check

```bash
curl -sf http://localhost:9052/health | jq -e '.status == "healthy"' > /dev/null && echo "OK" || echo "FAIL"
```

---

## See Also

- [RUNBOOK.md](RUNBOOK.md) - Operational procedures
- [INCIDENT_RESPONSE.md](INCIDENT_RESPONSE.md) - Incident response
- [METRICS.md](METRICS.md) - Detailed metrics reference
- [PRODUCTION.md](PRODUCTION.md) - Production deployment
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Common issues

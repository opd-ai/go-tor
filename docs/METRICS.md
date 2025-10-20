# HTTP Metrics and Observability

**Phase 9.1: Advanced Monitoring and Observability**

## Overview

The go-tor client includes a built-in HTTP metrics server that exposes operational metrics in multiple formats for integration with monitoring and alerting systems. This guide covers enabling, accessing, and using the metrics endpoints.

## Features

- **Prometheus-compatible metrics** endpoint for scraping
- **JSON metrics** for programmatic access
- **HTML dashboard** with auto-refresh for real-time monitoring
- **Health check** endpoint with HTTP status codes
- **Zero-configuration** - enable with a single flag
- **Production-ready** - follows industry best practices

## Quick Start

### Enable Metrics via CLI

The simplest way to enable metrics is using the `--metrics-port` flag:

```bash
# Start with metrics on port 9052
./tor-client --metrics-port 9052

# Access the dashboard
open http://localhost:9052/debug/metrics
```

### Enable Metrics via Configuration

Add to your `torrc` configuration:

```
MetricsPort 9052
EnableMetrics 1
```

### Enable Metrics via Code

```go
import (
    "github.com/opd-ai/go-tor/pkg/client"
    "github.com/opd-ai/go-tor/pkg/config"
)

cfg := config.DefaultConfig()
cfg.MetricsPort = 9052
cfg.EnableMetrics = true

torClient, err := client.New(cfg, logger)
// ... start client
```

## Available Endpoints

### 1. Root Index (`/`)

Lists all available endpoints with descriptions.

```bash
curl http://localhost:9052/
```

**Response**: HTML page with links to all endpoints

---

### 2. Prometheus Metrics (`/metrics`)

Exposes metrics in Prometheus text format for scraping.

```bash
curl http://localhost:9052/metrics
```

**Response Format**: Prometheus text format (Content-Type: `text/plain; version=0.0.4`)

**Example Output**:
```
# HELP tor_circuit_builds_total Total number of circuit build attempts
# TYPE tor_circuit_builds_total counter
tor_circuit_builds_total 150

# HELP tor_active_circuits Current number of active circuits
# TYPE tor_active_circuits gauge
tor_active_circuits 3

# HELP tor_circuit_build_duration_seconds_avg Average circuit build duration in seconds
# TYPE tor_circuit_build_duration_seconds_avg gauge
tor_circuit_build_duration_seconds_avg 3.456
```

**Prometheus Configuration**:
```yaml
scrape_configs:
  - job_name: 'go-tor'
    static_configs:
      - targets: ['localhost:9052']
    scrape_interval: 15s
```

---

### 3. JSON Metrics (`/metrics/json`)

Exposes metrics in JSON format for programmatic access.

```bash
curl http://localhost:9052/metrics/json
```

**Response Format**: JSON (Content-Type: `application/json`)

**Example Output**:
```json
{
  "CircuitBuilds": 150,
  "CircuitBuildSuccess": 145,
  "CircuitBuildFailure": 5,
  "CircuitBuildTimeAvg": 3456000000,
  "CircuitBuildTimeP95": 5000000000,
  "ActiveCircuits": 3,
  "ConnectionAttempts": 75,
  "ConnectionSuccess": 72,
  "ConnectionFailures": 3,
  "ActiveConnections": 3,
  "StreamsCreated": 500,
  "StreamsClosed": 490,
  "ActiveStreams": 10,
  "GuardsActive": 3,
  "GuardsConfirmed": 2,
  "SocksConnections": 250,
  "SocksRequests": 245,
  "UptimeSeconds": 3600
}
```

---

### 4. Health Check (`/health`)

Returns health status with appropriate HTTP status codes.

```bash
curl http://localhost:9052/health
```

**Response Format**: JSON (Content-Type: `application/json`)

**HTTP Status Codes**:
- `200 OK` - System is healthy
- `200 OK` - System is degraded (check response body)
- `503 Service Unavailable` - System is unhealthy

**Example Output**:
```json
{
  "status": "healthy",
  "components": {
    "circuits": {
      "name": "circuits",
      "status": "healthy",
      "message": "3 circuits active",
      "last_checked": "2025-10-20T02:45:00Z",
      "response_time_ms": 5
    }
  },
  "timestamp": "2025-10-20T02:45:00Z",
  "uptime": 3600000000000
}
```

---

### 5. HTML Dashboard (`/debug/metrics`)

Real-time dashboard with auto-refresh every 5 seconds.

```bash
# Access in browser
open http://localhost:9052/debug/metrics
```

**Features**:
- Auto-refreshes every 5 seconds
- Color-coded metrics (green for success, red for failures)
- Organized into logical sections
- Responsive design
- No external dependencies

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

### Connection Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `tor_connection_attempts_total` | Counter | Total connection attempts |
| `tor_connection_success_total` | Counter | Successful connections |
| `tor_connection_failures_total` | Counter | Failed connections |
| `tor_active_connections` | Gauge | Currently active connections |

### Stream Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `tor_streams_created_total` | Counter | Total streams created |
| `tor_streams_closed_total` | Counter | Total streams closed |
| `tor_active_streams` | Gauge | Currently active streams |
| `tor_stream_data_bytes_total` | Counter | Total bytes transferred |

### Guard Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `tor_guards_active` | Gauge | Active guard nodes |
| `tor_guards_confirmed` | Gauge | Confirmed guard nodes |

### SOCKS Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `tor_socks_connections_total` | Counter | Total SOCKS connections |
| `tor_socks_requests_total` | Counter | Total SOCKS requests |

### System Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `tor_uptime_seconds` | Gauge | Client uptime in seconds |

---

## Integration Examples

### Prometheus + Grafana

1. **Configure Prometheus** (`prometheus.yml`):
```yaml
scrape_configs:
  - job_name: 'go-tor'
    static_configs:
      - targets: ['localhost:9052']
    scrape_interval: 15s
```

2. **Start Prometheus**:
```bash
prometheus --config.file=prometheus.yml
```

3. **Add Grafana Dashboard**:
   - Create custom dashboard using available metrics
   - Example queries provided below
   - (Pre-built dashboard JSON coming in a future release)

### Automated Health Checks

```bash
#!/bin/bash
# health-check.sh - Monitor go-tor health

HEALTH_URL="http://localhost:9052/health"

response=$(curl -s -w "\n%{http_code}" "$HEALTH_URL")
status_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | head -n-1)

if [ "$status_code" -eq 200 ]; then
    echo "✓ go-tor is healthy"
    exit 0
elif [ "$status_code" -eq 503 ]; then
    echo "✗ go-tor is unhealthy: $body"
    exit 1
else
    echo "? Unexpected status: $status_code"
    exit 2
fi
```

### Alerting with AlertManager

```yaml
# alertmanager.yml
groups:
  - name: go-tor
    interval: 30s
    rules:
      - alert: TorCircuitBuildFailureRate
        expr: rate(tor_circuit_build_failures_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High circuit build failure rate"
          description: "Circuit build failure rate is {{ $value }} per second"
      
      - alert: TorNoActiveCircuits
        expr: tor_active_circuits < 1
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "No active circuits"
          description: "go-tor has no active circuits for 2 minutes"
```

---

## Performance Considerations

### Resource Usage

The HTTP metrics server is lightweight and adds minimal overhead:

- **Memory**: ~2MB additional RSS
- **CPU**: <1% for typical scrape rates (1/min)
- **Network**: ~5KB per scrape (Prometheus format)

### Best Practices

1. **Scrape Interval**: Recommended 15-30 seconds for Prometheus
2. **Dashboard Refresh**: Default 5 seconds is suitable for development; increase for production
3. **Port Selection**: Use dedicated port (9052) separate from SOCKS (9050) and Control (9051)
4. **Access Control**: Bind to localhost only; use reverse proxy for external access
5. **TLS**: Not implemented; use reverse proxy (nginx, caddy) for HTTPS

---

## Security Considerations

### Access Control

The metrics server binds to `127.0.0.1` by default, restricting access to localhost only.

**For external access**, use a reverse proxy with authentication:

```nginx
# nginx.conf
server {
    listen 443 ssl;
    server_name metrics.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:9052;
        auth_basic "Metrics";
        auth_basic_user_file /etc/nginx/.htpasswd;
    }
}
```

### Information Disclosure

Metrics endpoints expose operational data but no sensitive information:

- ✅ No circuit paths or relay identities
- ✅ No stream destinations
- ✅ No cryptographic material
- ✅ Only aggregate statistics

However, metrics can reveal:
- Network usage patterns
- Activity levels
- Performance characteristics

**Recommendation**: Limit access to trusted monitoring systems only.

---

## Troubleshooting

### Metrics Server Not Starting

**Symptom**: No metrics endpoint available

**Possible Causes**:
1. Metrics not enabled in configuration
2. Port already in use
3. Permission denied (port < 1024)

**Solutions**:
```bash
# Check if enabled
grep MetricsPort /path/to/torrc

# Check port availability
netstat -an | grep 9052

# Use unprivileged port (recommended)
tor-client --metrics-port 9052  # Good
tor-client --metrics-port 80    # Requires root (not recommended)
```

### Missing Metrics

**Symptom**: Some metrics show zero or are missing

**Possible Causes**:
1. Client just started (metrics accumulate over time)
2. No circuits built yet
3. No SOCKS connections yet

**Solution**: Wait for client to bootstrap and handle traffic

### High Memory Usage

**Symptom**: Metrics server using more memory than expected

**Possible Cause**: High scrape rate with large number of metrics

**Solution**: Increase scrape interval in Prometheus configuration

---

## API Examples

### Python

```python
import requests
import json

# Fetch JSON metrics
response = requests.get('http://localhost:9052/metrics/json')
metrics = response.json()

print(f"Active Circuits: {metrics['ActiveCircuits']}")
print(f"Circuit Build Success Rate: {metrics['CircuitBuildSuccess'] / metrics['CircuitBuilds'] * 100:.1f}%")

# Check health
health = requests.get('http://localhost:9052/health')
if health.status_code == 200:
    print("✓ System is healthy")
else:
    print(f"✗ System is unhealthy: {health.status_code}")
```

### Go

```go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    
    "github.com/opd-ai/go-tor/pkg/metrics"
)

func fetchMetrics() (*metrics.Snapshot, error) {
    resp, err := http.Get("http://localhost:9052/metrics/json")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var snapshot metrics.Snapshot
    if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
        return nil, err
    }
    
    return &snapshot, nil
}

func main() {
    snapshot, err := fetchMetrics()
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Active Circuits: %d\n", snapshot.ActiveCircuits)
    fmt.Printf("Uptime: %d seconds\n", snapshot.UptimeSeconds)
}
```

### curl + jq

```bash
# Get specific metric
curl -s http://localhost:9052/metrics/json | jq '.ActiveCircuits'

# Calculate success rate
curl -s http://localhost:9052/metrics/json | \
  jq '(.CircuitBuildSuccess / .CircuitBuilds * 100) | floor'

# Monitor in real-time
watch -n 5 'curl -s http://localhost:9052/metrics/json | jq .'
```

---

## See Also

- [ARCHITECTURE.md](ARCHITECTURE.md) - System architecture
- [PRODUCTION.md](PRODUCTION.md) - Production deployment guide
- [PERFORMANCE.md](PERFORMANCE.md) - Performance tuning
- [../examples/metrics-demo](../examples/metrics-demo) - Full working example

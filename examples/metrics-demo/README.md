# HTTP Metrics Demo

This example demonstrates how to use go-tor with the HTTP metrics endpoint enabled for monitoring and observability.

## Features

- HTTP metrics server on port 9052
- Real-time metrics display every 10 seconds
- Demonstrates all metrics endpoints:
  - `/metrics` - Prometheus format
  - `/metrics/json` - JSON format
  - `/health` - Health check
  - `/debug/metrics` - HTML dashboard

## Prerequisites

- Go 1.24 or later

## Running the Example

```bash
cd examples/metrics-demo
go run main.go
```

## What It Does

1. Starts go-tor client with metrics enabled on port 9052
2. Connects to the Tor network
3. Displays available metrics endpoints
4. Periodically fetches and displays current metrics (every 10 seconds)
5. Continues running until you press Ctrl+C

## Example Output

```
go-tor HTTP Metrics Demo
========================

Starting Tor client with metrics enabled...
✅ Tor client started successfully!
   SOCKS5 Proxy:  socks5://127.0.0.1:9050
   Control Port:  127.0.0.1:9051
   Metrics HTTP:  http://127.0.0.1:9052/

Available metrics endpoints:
  - Dashboard:   http://127.0.0.1:9052/debug/metrics
  - Prometheus:  http://127.0.0.1:9052/metrics
  - JSON:        http://127.0.0.1:9052/metrics/json
  - Health:      http://127.0.0.1:9052/health

Press Ctrl+C to exit

─────────────────────────────────────────────
Current Metrics (Uptime: 15s)
─────────────────────────────────────────────
Circuits:     3 active, 3 built (3 success, 0 failed)
Connections:  3 active, 3 total (3 success, 0 failed)
Streams:      0 active, 0 created, 0 closed
Guards:       3 active, 2 confirmed
SOCKS:        0 connections, 0 requests
Build Time:   avg 4.23s, p95 4.85s
─────────────────────────────────────────────
```

## Accessing the Dashboard

While the example is running, open your browser to:

```
http://localhost:9052/debug/metrics
```

This provides a real-time dashboard that auto-refreshes every 5 seconds.

## Using with Prometheus

To scrape metrics with Prometheus, add this to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'go-tor-demo'
    static_configs:
      - targets: ['localhost:9052']
    scrape_interval: 15s
```

Then query metrics in Prometheus:

```
# Active circuits
tor_active_circuits

# Circuit build success rate
rate(tor_circuit_build_success_total[5m]) / rate(tor_circuit_builds_total[5m])

# Average circuit build time
tor_circuit_build_duration_seconds_avg
```

## Programmatic Access

You can fetch metrics programmatically:

```bash
# Get JSON metrics
curl http://localhost:9052/metrics/json | jq .

# Get Prometheus metrics
curl http://localhost:9052/metrics

# Check health
curl http://localhost:9052/health
```

## See Also

- [../../docs/METRICS.md](../../docs/METRICS.md) - Complete metrics documentation
- [../../docs/PRODUCTION.md](../../docs/PRODUCTION.md) - Production deployment guide
- [../../README.md](../../README.md) - Main project README

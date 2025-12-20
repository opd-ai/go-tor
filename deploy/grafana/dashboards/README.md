# Grafana Dashboards

This directory contains Grafana dashboard templates for monitoring go-tor.

## ⚠️ IMPORTANT DISCLAIMER

**THIS IS UNOFFICIAL SOFTWARE** developed without the supervision or endorsement of [The Tor Project](https://www.torproject.org/). This software should **NOT** be used in production or for any real anonymity needs.

---

## Available Dashboards

| Dashboard | File | Description |
|-----------|------|-------------|
| go-tor Overview | `go-tor-overview.json` | Main monitoring dashboard with all key metrics |

## Dashboard Features

### go-tor Overview

The overview dashboard includes:

- **Service Overview Row**
  - Service status (UP/DOWN)
  - Active circuits count
  - Circuit success rate gauge
  - Average circuit build time
  - Active streams count
  - Uptime

- **Circuits Row**
  - Circuit build rate (success vs failures over time)
  - Circuit build duration (average and P95)

- **Connections Row**
  - Connection rate (success, failures, retries)
  - Active connections over time

- **SOCKS Proxy Row**
  - SOCKS request rate (requests vs errors)
  - Active streams over time

- **Guards & Security Row**
  - Active guards count
  - Confirmed guards count
  - Replay attempts detected
  - Isolated circuits count

- **Data Transfer Row**
  - Stream data throughput

## Installation

### Method 1: Import via Grafana UI

1. Open Grafana (typically http://localhost:3000)
2. Go to **Dashboards** → **Import**
3. Click **Upload JSON file**
4. Select `go-tor-overview.json`
5. Select your Prometheus data source
6. Click **Import**

### Method 2: Grafana Provisioning

1. Copy dashboard to Grafana provisioning directory:

```bash
cp dashboards/*.json /etc/grafana/provisioning/dashboards/
```

2. Create dashboard provider configuration:

```yaml
# /etc/grafana/provisioning/dashboards/go-tor.yaml
apiVersion: 1
providers:
  - name: 'go-tor'
    orgId: 1
    folder: 'Tor'
    type: file
    disableDeletion: false
    editable: true
    options:
      path: /etc/grafana/provisioning/dashboards
```

3. Restart Grafana:

```bash
sudo systemctl restart grafana-server
```

### Method 3: Grafana API

```bash
# Import dashboard via API
curl -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <api-key>" \
  -d @go-tor-overview.json \
  http://localhost:3000/api/dashboards/db
```

## Configuration

### Variables

The dashboards support the following template variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `datasource` | Prometheus data source | Prometheus |
| `instance` | Filter by instance | All |

### Customization

#### Change Refresh Interval

Edit the JSON file and modify:

```json
"refresh": "30s"
```

#### Modify Thresholds

Find the panel and update thresholds:

```json
"thresholds": {
  "mode": "absolute",
  "steps": [
    {"color": "red", "value": null},
    {"color": "yellow", "value": 1},
    {"color": "green", "value": 2}
  ]
}
```

#### Add New Panel

1. Edit dashboard in Grafana UI
2. Add panel and configure
3. Export updated JSON via **Share** → **Export** → **Save to file**

## Panel Quick Reference

| Panel | Metric | Good Value |
|-------|--------|------------|
| Service Status | `up{job="go-tor"}` | 1 (UP) |
| Active Circuits | `tor_active_circuits` | ≥ 2 |
| Circuit Success Rate | `success / total * 100` | ≥ 70% |
| Avg Build Time | `tor_circuit_build_duration_seconds_avg` | < 5s |
| Active Guards | `tor_guards_active` | ≥ 2 |
| Confirmed Guards | `tor_guards_confirmed` | ≥ 2 |

## Alerts Integration

The dashboards reference Prometheus alerts defined in `../prometheus/alerts.yml`. Ensure alerts are configured for visual indicators on panels.

## Troubleshooting

### No Data Displayed

1. Verify Prometheus data source is configured correctly
2. Check that go-tor metrics are being scraped:
   ```bash
   curl http://localhost:9090/api/v1/query?query=up{job="go-tor"}
   ```
3. Verify instance variable matches your setup

### Dashboard Not Loading

1. Check Grafana logs for errors
2. Validate JSON syntax:
   ```bash
   jq . go-tor-overview.json > /dev/null
   ```
3. Ensure dashboard UID is unique

## See Also

- [Prometheus Alerts](../prometheus/) - Alerting rules
- [Monitoring Guide](../../docs/MONITORING_GUIDE.md) - Full monitoring setup
- [Alerts Guide](../../docs/ALERTS.md) - Alert response procedures

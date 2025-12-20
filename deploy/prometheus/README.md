# Prometheus Alerting Configuration

This directory contains Prometheus alerting rules for monitoring go-tor.

## ⚠️ IMPORTANT DISCLAIMER

**THIS IS UNOFFICIAL SOFTWARE** developed without the supervision or endorsement of [The Tor Project](https://www.torproject.org/). This software should **NOT** be used in production or for any real anonymity needs.

---

## Files

- `alerts.yml` - Prometheus alerting rules with SLI-based alerts

## Installation

### 1. Copy to Prometheus Rules Directory

```bash
cp alerts.yml /etc/prometheus/rules/go-tor-alerts.yml
```

### 2. Add to Prometheus Configuration

Edit `prometheus.yml`:

```yaml
rule_files:
  - "rules/go-tor-alerts.yml"
```

### 3. Configure AlertManager (Optional)

Create or update `alertmanager.yml`:

```yaml
route:
  receiver: 'default-receiver'
  group_by: ['alertname', 'severity']
  routes:
    - match:
        severity: critical
      receiver: 'pagerduty-critical'
    - match:
        severity: warning
      receiver: 'slack-warnings'

receivers:
  - name: 'default-receiver'
    email_configs:
      - to: 'team@example.com'

  - name: 'pagerduty-critical'
    pagerduty_configs:
      - service_key: '<your-service-key>'

  - name: 'slack-warnings'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/...'
        channel: '#tor-alerts'
```

### 4. Reload Prometheus

```bash
# Using systemctl
sudo systemctl reload prometheus

# Or via HTTP API
curl -X POST http://localhost:9090/-/reload
```

## Alert Severity Levels

| Severity | Action | Response Time |
|----------|--------|---------------|
| **critical** | Page immediately | < 5 min |
| **warning** | Create ticket | < 4 hours |
| **info** | Review | Next business day |

## SLIs (Service Level Indicators)

The alerts are designed around these SLIs:

| SLI | Target | Alert Threshold |
|-----|--------|-----------------|
| Availability | 99.9% | Service down > 1 min |
| Circuit Success | ≥ 70% | Failure rate > 30% |
| Connection Success | ≥ 80% | Failure rate > 20% |
| Latency (P95) | ≤ 10s | Build time > 10s |

## Testing Alerts

### Verify Rules Syntax

```bash
promtool check rules alerts.yml
```

### Test Alert Evaluation

```bash
# Dry-run against Prometheus
promtool test rules test-alerts.yml
```

## Customization

### Adjusting Thresholds

Edit `alerts.yml` and modify the `expr` field:

```yaml
- alert: TorHighCircuitFailureRate
  expr: |
    (sum(rate(tor_circuit_build_failures_total[5m])) /
     sum(rate(tor_circuit_builds_total[5m]))) > 0.3  # Change this threshold
  for: 5m
```

### Adding Custom Labels

Add labels for routing:

```yaml
labels:
  severity: warning
  team: infrastructure  # Add team label
  service: tor-proxy    # Add service label
```

## Alert Documentation

See [docs/ALERTS.md](../../docs/ALERTS.md) for detailed alert response procedures.

## See Also

- [Grafana Dashboards](../grafana/dashboards/) - Pre-built dashboards
- [Kubernetes Deployment](../kubernetes/) - Kubernetes manifests
- [Monitoring Guide](../../docs/MONITORING_GUIDE.md) - Full monitoring setup

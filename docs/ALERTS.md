# go-tor Alert Response Guide

## ⚠️ IMPORTANT DISCLAIMER

**THIS IS UNOFFICIAL SOFTWARE** developed without the supervision or endorsement of [The Tor Project](https://www.torproject.org/). This software should **NOT** be used in production or for any real anonymity needs.

**For actual privacy and security:**
- **Users**: Use [Tor Browser](https://www.torproject.org/download/)
- **Developers**: Use [Arti](https://gitlab.torproject.org/tpo/core/arti)

This guide is for **testing and development environments only**.

---

## Table of Contents

- [Alert Severity Levels](#alert-severity-levels)
- [Service Level Indicators (SLIs)](#service-level-indicators-slis)
- [Critical Alerts](#critical-alerts)
- [Warning Alerts](#warning-alerts)
- [Info Alerts](#info-alerts)
- [SLO Burn Rate Alerts](#slo-burn-rate-alerts)
- [Alert Response Workflow](#alert-response-workflow)
- [Escalation Policy](#escalation-policy)

---

## Alert Severity Levels

| Severity | Action | Response Time | Notification |
|----------|--------|---------------|--------------|
| **Critical** | Page on-call immediately | < 5 minutes | PagerDuty, Phone |
| **Warning** | Create ticket, investigate | < 4 hours | Slack, Email |
| **Info** | Review during business hours | Next business day | Dashboard only |

---

## Service Level Indicators (SLIs)

go-tor defines the following SLIs for monitoring service health:

| SLI | Target | Measurement | Rationale |
|-----|--------|-------------|-----------|
| **Availability** | 99.9% | `up{job="go-tor"} == 1` | Service must be running |
| **Circuit Success Rate** | ≥ 70% | `success / total builds` | Most circuits should succeed |
| **Connection Success Rate** | ≥ 80% | `success / total attempts` | Connections should be reliable |
| **Latency (P95)** | ≤ 10s | `circuit_build_duration_p95` | Circuits should build quickly |
| **Error Rate** | ≤ 10% | `errors / requests` | Low error rate for users |

### SLO Targets

| SLO | Monthly Error Budget | Alert Threshold |
|-----|---------------------|-----------------|
| Circuit Success 70% | 30% failures allowed | Fast burn: >2%/hour |
| Connection Success 80% | 20% failures allowed | Fast burn: >1%/hour |
| Latency P95 ≤ 10s | 0.1% violations allowed | >10s for 10 minutes |

---

## Critical Alerts

### TorServiceDown

**Summary**: go-tor service is unreachable.

**Impact**: Complete service outage. No Tor connectivity available.

**Symptoms**:
- Prometheus cannot scrape metrics endpoint
- Health check endpoint not responding
- SOCKS proxy not accepting connections

**Investigation Steps**:

1. **Check if process is running**:
   ```bash
   # Check process status
   ps aux | grep tor-client
   systemctl status go-tor
   
   # Check if port is listening
   ss -tlnp | grep -E '905[0-2]'
   ```

2. **Review system logs**:
   ```bash
   # Check service logs
   journalctl -u go-tor -n 100 --no-pager
   
   # Check for OOM kills
   dmesg | grep -i "out of memory"
   ```

3. **Check resource usage**:
   ```bash
   # Check disk space
   df -h /var/lib/go-tor
   
   # Check memory
   free -m
   ```

**Resolution**:

1. **Restart the service**:
   ```bash
   systemctl restart go-tor
   ```

2. **If restart fails**, check configuration:
   ```bash
   /path/to/tor-config-validator -config /etc/go-tor/config.yaml
   ```

3. **If persistent failures**, check for:
   - Corrupted state files
   - Network connectivity issues
   - Resource exhaustion

**Escalation**: If not resolved within 5 minutes, escalate to secondary on-call.

---

### TorNoActiveCircuits

**Summary**: No active Tor circuits available.

**Impact**: SOCKS proxy is non-functional. All traffic routing will fail.

**Symptoms**:
- `tor_active_circuits == 0`
- SOCKS connections time out or fail
- Applications report proxy errors

**Investigation Steps**:

1. **Check circuit build status**:
   ```bash
   # Check metrics
   curl -s http://localhost:9052/metrics/json | jq '.circuit'
   
   # Check recent build failures
   grep -i "circuit.*fail" /var/log/go-tor/tor.log | tail -20
   ```

2. **Verify network connectivity**:
   ```bash
   # Check if we can reach directory authorities
   curl -I https://www.torproject.org
   
   # Check DNS resolution
   dig +short check.torproject.org
   ```

3. **Check directory consensus**:
   ```bash
   # Look for consensus-related errors
   grep -i "consensus" /var/log/go-tor/tor.log | tail -20
   ```

**Resolution**:

1. **Check network path**:
   ```bash
   # Verify outbound connectivity on port 9001
   nc -zv [relay-ip] 9001
   ```

2. **Force consensus refresh**:
   - Restart the service to trigger fresh consensus download
   - Check if consensus file is corrupted and remove if needed

3. **Check guard nodes**:
   - If guards are unavailable, circuits cannot be built
   - Review guard persistence file for issues

**Escalation**: If no circuits for > 10 minutes after investigation, escalate.

---

### TorNoConfirmedGuardsUrgent

**Summary**: No guard nodes available (neither active nor confirmed).

**Impact**: Cannot build circuits. Security guarantees not maintained.

**Symptoms**:
- `tor_guards_confirmed == 0 AND tor_guards_active == 0`
- All circuit builds fail at first hop
- No connections to Tor network

**Investigation Steps**:

1. **Check guard state file**:
   ```bash
   # Check if guard state exists
   ls -la /var/lib/go-tor/guards.json
   
   # Check file content (if exists)
   cat /var/lib/go-tor/guards.json | jq '.guards | length'
   ```

2. **Verify directory consensus**:
   ```bash
   # Check consensus freshness
   ls -la /var/lib/go-tor/consensus
   
   # Check consensus age
   stat /var/lib/go-tor/consensus --printf="%Y"
   ```

3. **Check guard selection logs**:
   ```bash
   grep -i "guard" /var/log/go-tor/tor.log | tail -50
   ```

**Resolution**:

1. **If guard state is corrupted**:
   ```bash
   # Backup and remove corrupted state
   mv /var/lib/go-tor/guards.json /var/lib/go-tor/guards.json.bak
   
   # Restart to rebuild guards
   systemctl restart go-tor
   ```

2. **If consensus is stale**:
   ```bash
   # Remove stale consensus
   rm /var/lib/go-tor/consensus
   
   # Restart to fetch fresh consensus
   systemctl restart go-tor
   ```

3. **Verify network connectivity to directory authorities**

---

## Warning Alerts

### TorLowCircuitCount

**Summary**: Only 1 active circuit. Service is degraded.

**Impact**: Limited circuit multiplexing. Performance degraded.

**Investigation Steps**:

1. Check circuit build rate and failure rate
2. Review circuit build duration (may be slow)
3. Check if guards are healthy

**Resolution**:

1. Review prebuilt circuit configuration
2. Check for network issues affecting circuit builds
3. Consider increasing circuit timeout values

---

### TorHighCircuitFailureRate

**Summary**: >30% of circuit builds are failing.

**Impact**: Reduced circuit availability. Increased latency.

**Investigation Steps**:

1. **Identify failure patterns**:
   ```bash
   grep -i "circuit.*fail" /var/log/go-tor/tor.log | \
     awk '{print $NF}' | sort | uniq -c | sort -rn
   ```

2. Check which hop is failing (guard, middle, exit)
3. Review network connectivity

**Resolution**:

1. Check relay health (may be blacklisted relays)
2. Review consensus freshness
3. Consider adjusting circuit timeout

---

### TorHighConnectionFailureRate

**Summary**: >20% of connections to relays are failing.

**Impact**: Difficulty establishing circuits. Increased build times.

**Investigation Steps**:

1. **Check connection errors**:
   ```bash
   grep -i "connection.*fail\|connect.*error" /var/log/go-tor/tor.log | tail -50
   ```

2. Check firewall rules for outbound port 9001
3. Verify TLS configuration

**Resolution**:

1. Check network path to Tor relays
2. Verify firewall allows outbound 9001/tcp
3. Check for TLS handshake errors

---

### TorSlowCircuitBuilds

**Summary**: P95 circuit build time >10 seconds.

**Impact**: User experience degraded. Slow connection establishment.

**Investigation Steps**:

1. Check average vs P95 build times
2. Identify which circuits are slow
3. Check network latency

**Resolution**:

1. Consider geographic relay preferences
2. Check for network congestion
3. Review relay selection policy

---

### TorHighConnectionRetryRate

**Summary**: >50% of connections require retries.

**Impact**: Increased resource usage. Longer connection times.

**Investigation Steps**:

1. Review retry logs
2. Check for intermittent connectivity
3. Identify failing relays

**Resolution**:

1. Check network stability
2. Consider adjusting retry policy
3. Review relay blacklisting

---

### TorNoConfirmedGuards

**Summary**: Active guards but none confirmed after 30 minutes.

**Impact**: Guard rotation may be excessive.

**Investigation Steps**:

1. Check guard selection activity
2. Review guard persistence
3. Check if guards are reachable

**Resolution**:

1. Guards will confirm over time
2. Check guard persistence file permissions
3. Verify guards are stable and reachable

---

### TorHighSocksErrorRate

**Summary**: >10% of SOCKS requests are failing.

**Impact**: Client applications experiencing failures.

**Investigation Steps**:

1. **Review SOCKS errors**:
   ```bash
   grep -i "socks.*error" /var/log/go-tor/tor.log | tail -50
   ```

2. Check circuit availability
3. Review client request patterns

**Resolution**:

1. Ensure circuits are available
2. Check for malformed client requests
3. Review stream isolation settings

---

### TorReplayAttacksDetected

**Summary**: Replay attacks are being detected.

**Impact**: Potential security issue. Possible attack in progress.

**Investigation Steps**:

1. **Review attack patterns**:
   ```bash
   grep -i "replay" /var/log/go-tor/tor.log | tail -100
   ```

2. Check forward vs backward replay attempts
3. Identify source circuits

**Resolution**:

1. Rotate affected circuits
2. Check for malicious relays in path
3. Review replay protection logs
4. Consider reporting to Tor Project if persistent

---

## Info Alerts

### TorHighStreamActivity

**Summary**: >100 active streams.

**Impact**: High load. Monitor for resource exhaustion.

**Investigation**: Review stream distribution. Consider scaling.

---

### TorLowGuardCount

**Summary**: Only 1 active guard.

**Impact**: Reduced resilience if guard fails.

**Investigation**: Guards are added automatically over time.

---

### TorHighIsolationMisses

**Summary**: >50% of isolated circuit requests miss cache.

**Impact**: Building many isolated circuits.

**Investigation**: Review isolation requirements. Consider prebuilding.

---

### TorOutOfOrderCells

**Summary**: Out-of-order cells detected.

**Impact**: May indicate network issues.

**Investigation**: Review network path. Check for packet loss.

---

### TorExtendedUptime

**Summary**: Running for >7 days.

**Impact**: May be running outdated code.

**Investigation**: Schedule maintenance window for update.

---

## SLO Burn Rate Alerts

### TorCircuitSuccessSLOFastBurn

**Summary**: Consuming error budget at >2% per hour.

**Impact**: Will exhaust monthly budget in days if sustained.

**Immediate Actions**:
1. Investigate circuit build failures immediately
2. Check relay connectivity
3. Review consensus freshness

### TorCircuitSuccessSLOSlowBurn

**Summary**: Slowly consuming error budget.

**Impact**: Will impact monthly SLO if sustained for days.

**Actions**:
1. Review circuit build trends
2. Plan investigation during business hours
3. Check for gradual degradation

---

## Alert Response Workflow

```
┌─────────────────────────────────────────────────────────────┐
│                    ALERT RECEIVED                           │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  1. ACKNOWLEDGE                                             │
│     - Note alert time and details                           │
│     - Check for related alerts                              │
│     - Review recent changes (deploys, config)               │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  2. INVESTIGATE                                             │
│     - Check dashboards for patterns                         │
│     - Review logs for errors                                │
│     - Verify basic connectivity                             │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  3. MITIGATE                                                │
│     - Apply immediate fix if known                          │
│     - Restart service if safe                               │
│     - Rollback recent changes if applicable                 │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  4. ESCALATE (if needed)                                    │
│     - Critical: After 5 minutes without resolution          │
│     - Warning: After 1 hour without resolution              │
│     - Include: Alert details, investigation, attempted fixes│
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  5. RESOLVE & DOCUMENT                                      │
│     - Verify alert is resolved                              │
│     - Document root cause                                   │
│     - Create follow-up tickets if needed                    │
│     - Update runbook if new failure mode                    │
└─────────────────────────────────────────────────────────────┘
```

---

## Escalation Policy

### Critical Alerts

1. **0-5 minutes**: Primary on-call investigates
2. **5-15 minutes**: Escalate to secondary on-call
3. **15-30 minutes**: Escalate to team lead
4. **30+ minutes**: Incident commander engaged

### Warning Alerts

1. **0-1 hour**: Assigned engineer investigates
2. **1-4 hours**: Escalate to team lead
3. **4+ hours**: Schedule postmortem if pattern continues

### Contact Information

| Role | Contact Method | Response Time |
|------|----------------|---------------|
| Primary On-Call | PagerDuty | < 5 min |
| Secondary On-Call | PagerDuty | < 15 min |
| Team Lead | Slack/Phone | < 30 min |

---

## See Also

- [MONITORING_GUIDE.md](MONITORING_GUIDE.md) - Monitoring setup and configuration
- [INCIDENT_RESPONSE.md](INCIDENT_RESPONSE.md) - Incident response procedures
- [RUNBOOK.md](RUNBOOK.md) - Operational procedures
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Common issues and solutions

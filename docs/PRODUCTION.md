# Phase 6: Production Hardening - Deployment Guide

## Overview

This guide covers deploying and operating the go-tor client in production environments. Phase 6 introduced critical production hardening features including TLS certificate validation, guard node persistence, and connection retry logic.

## Production Features (Phase 6)

### 1. TLS Certificate Validation

**What Changed:**
- Replaced `InsecureSkipVerify: true` with Tor-specific certificate validation
- Validates X.509 certificate structure, expiration, and signatures
- Accepts self-signed certificates per Tor protocol specification
- Enforces TLS 1.2+ with secure cipher suites

**Security Implications:**
- Prevents man-in-the-middle attacks on relay connections
- Ensures certificate structural integrity
- Full relay identity verification happens via directory consensus

### 2. Guard Node Persistence

**What It Does:**
- Maintains a persistent set of guard nodes across restarts
- Stores guard state in `{data-directory}/guard_state.json`
- Automatically cleans up expired guards (90-day retention)
- Confirms guards after successful circuit builds
- Limits to 3 guard nodes (configurable)

**Benefits:**
- Improved anonymity by using consistent entry points
- Faster circuit build times after initial selection
- Reduces fingerprinting opportunities
- Follows Tor specification for guard behavior

**Configuration:**
```bash
# Guard state is automatically persisted to data directory
./tor-client -data-dir /path/to/data
```

**Guard State File Structure:**
```json
{
  "guards": [
    {
      "fingerprint": "AAAA...",
      "nickname": "GuardRelay1",
      "address": "1.2.3.4:9001",
      "first_used": "2025-10-18T12:00:00Z",
      "last_used": "2025-10-18T18:00:00Z",
      "confirmed": true
    }
  ],
  "last_updated": "2025-10-18T18:00:00Z"
}
```

### 3. Connection Retry Logic

**Features:**
- Exponential backoff with configurable parameters
- Context-aware retry (respects cancellation)
- Optional jitter to prevent thundering herd
- Connection pooling for reusability

**Default Configuration:**
- Max attempts: 3
- Initial backoff: 1 second
- Max backoff: 30 seconds
- Backoff multiplier: 2x
- Jitter: enabled (±25%)

**Usage Example:**
```go
import "github.com/opd-ai/go-tor/pkg/connection"

// Create custom retry config
retryCfg := &connection.RetryConfig{
    MaxAttempts:       5,
    InitialBackoff:    2 * time.Second,
    MaxBackoff:        60 * time.Second,
    BackoffMultiplier: 2.0,
    Jitter:            true,
}

// Connect with retry
cfg := connection.DefaultConfig("1.2.3.4:9001")
conn := connection.New(cfg, logger)
err := conn.ConnectWithRetry(ctx, cfg, retryCfg)
```

## Deployment Considerations

### System Requirements

**Minimum:**
- Go 1.21+
- 50MB RAM
- 100MB disk space
- Network connectivity to Tor directory authorities

**Recommended:**
- 100MB+ RAM for multiple circuits
- 500MB+ disk space for logs and state
- Stable network connection
- Low-latency network path

### File System Structure

```
/path/to/data/
├── guard_state.json      # Persistent guard nodes
└── (future: other state files)
```

**Permissions:**
- Data directory: 0700 (owner only)
- Guard state file: 0600 (owner read/write only)

### Configuration

**Command Line Flags:**
```bash
./tor-client \
  -socks-port 9050 \
  -control-port 9051 \
  -data-dir /var/lib/tor-client \
  -log-level info
```

**Environment Considerations:**
- Ensure data directory is writable
- Data directory should be on persistent storage (not /tmp)
- For containers, mount data directory as volume

### Monitoring

**Health Checks:**

Check if SOCKS5 proxy is responding:
```bash
curl --socks5 127.0.0.1:9050 https://check.torproject.org
```

**Logs to Monitor:**
- Guard persistence operations
- Connection retry attempts
- TLS certificate validation failures
- Circuit build successes/failures

**Key Metrics:**
- Active circuits count
- Guard node confirmations
- Connection retry rates
- TLS handshake failures

### Resource Limits

**Connections:**
- Each circuit requires 3 relay connections (guard, middle, exit)
- Connection pooling reuses connections when possible
- Typical: 3-10 active circuits = 9-30 connections

**Memory:**
- Base: ~20MB
- Per circuit: ~2-5MB
- Per connection: ~1-2MB
- Typical 50MB total with 3-5 active circuits

**Disk I/O:**
- Guard state writes: infrequent (only on changes)
- Minimal disk usage for state persistence

## Security Best Practices

### 1. Data Directory Protection

```bash
# Set restrictive permissions
chmod 700 /var/lib/tor-client
chmod 600 /var/lib/tor-client/guard_state.json
```

### 2. Process Isolation

**Using systemd:**
```ini
[Service]
User=tor-client
Group=tor-client
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/tor-client
```

### 3. Network Security

- Firewall: Allow outbound connections to Tor relays (443, 9001, 9030)
- No inbound connections required (client-only)
- Consider using separate network namespace for isolation

### 4. Secrets Management

- No secrets stored currently
- Future: control port authentication tokens should use secure storage

## Troubleshooting

### Common Issues

**1. Permission Denied on Data Directory**
```
Error: failed to create guard manager: failed to create data directory: permission denied
```
Solution: Ensure data directory is writable by the process user.

**2. Connection Retry Exhausted**
```
Error: connection failed after 3 attempts
```
Solutions:
- Check network connectivity
- Verify firewall allows outbound Tor connections
- Increase retry attempts if network is unstable

**3. TLS Certificate Validation Failed**
```
Error: invalid certificate signature
```
Solutions:
- Verify system time is correct
- Check for network intercepting proxies
- Ensure relay is legitimate (check directory consensus)

### Debug Logging

Enable debug logging for detailed information:
```bash
./tor-client -log-level debug
```

### State Recovery

If guard state becomes corrupted:
```bash
# Remove guard state to start fresh
rm /var/lib/tor-client/guard_state.json
# Client will select new guards on next start
```

## Performance Tuning

### Circuit Build Optimization

**Guard Selection:**
- Persistent guards reduce circuit build time after initial selection
- First circuit build: ~5-10 seconds
- Subsequent builds: ~3-5 seconds (using persistent guards)

**Connection Pooling:**
- Reuse connections when possible
- Pool size recommendation: 5-10 connections
- Tune based on circuit usage patterns

### Retry Tuning

**High-Latency Networks:**
```go
retryCfg := &connection.RetryConfig{
    MaxAttempts:       5,      // More attempts
    InitialBackoff:    3 * time.Second,  // Longer initial wait
    MaxBackoff:        120 * time.Second, // Higher ceiling
    BackoffMultiplier: 2.0,
    Jitter:            true,
}
```

**Low-Latency Networks:**
```go
retryCfg := &connection.RetryConfig{
    MaxAttempts:       2,      // Fewer attempts
    InitialBackoff:    500 * time.Millisecond, // Shorter initial wait
    MaxBackoff:        10 * time.Second,
    BackoffMultiplier: 2.0,
    Jitter:            true,
}
```

## Container Deployment

### Docker Example

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o tor-client ./cmd/tor-client

FROM alpine:latest
RUN adduser -D -s /bin/sh tor-client
USER tor-client
COPY --from=builder /app/tor-client /usr/local/bin/
VOLUME ["/data"]
EXPOSE 9050 9051
ENTRYPOINT ["/usr/local/bin/tor-client", "-data-dir", "/data"]
```

**Run:**
```bash
docker run -d \
  -v tor-data:/data \
  -p 127.0.0.1:9050:9050 \
  -p 127.0.0.1:9051:9051 \
  go-tor:latest
```

### Kubernetes Example

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: tor-client-data
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tor-client
spec:
  replicas: 1
  selector:
    matchLabels:
      app: tor-client
  template:
    metadata:
      labels:
        app: tor-client
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000
      containers:
      - name: tor-client
        image: go-tor:latest
        ports:
        - containerPort: 9050
          name: socks
        - containerPort: 9051
          name: control
        volumeMounts:
        - name: data
          mountPath: /data
        resources:
          limits:
            memory: "128Mi"
            cpu: "500m"
          requests:
            memory: "64Mi"
            cpu: "100m"
        livenessProbe:
          tcpSocket:
            port: 9050
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          tcpSocket:
            port: 9050
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: tor-client-data
```

## Upgrade Path

### From Phase 5 to Phase 6

**Changes:**
1. TLS certificate validation is now enabled (was disabled)
2. Guard state is now persisted (was ephemeral)
3. Connection retry logic is available (was not present)

**Migration:**
- No breaking changes
- Existing deployments will automatically create guard state on first run
- Data directory must be specified (defaults to `/var/lib/tor`)

**Recommendations:**
1. Update data directory location if using default
2. Review TLS certificate validation logs
3. Monitor guard persistence operations
4. Tune retry configuration based on network conditions

## Future Enhancements (Phase 7+)

Planned production features:
- Onion service support (.onion addresses)
- Control protocol for runtime configuration
- Metrics and observability hooks
- Rate limiting for resource protection
- Enhanced circuit health monitoring
- Bandwidth usage tracking

## Support and Resources

- Project repository: https://github.com/opd-ai/go-tor
- Tor specifications: https://spec.torproject.org/
- Issue tracker: https://github.com/opd-ai/go-tor/issues

## License

BSD 3-Clause License - See LICENSE file for details.

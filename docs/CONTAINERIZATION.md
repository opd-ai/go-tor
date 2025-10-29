# Container Deployment Guide

This guide covers running go-tor in containers using Docker and container orchestration platforms.

## ⚠️ IMPORTANT NOTICE

**This software is for educational and research purposes only.** For actual privacy and anonymity needs, use official Tor software:
- [Tor Browser](https://www.torproject.org/download/) - for safe anonymous browsing
- [Arti](https://gitlab.torproject.org/tpo/core/arti) - official Tor implementation in Rust

**Do NOT use this software for any situation where your safety or anonymity is at stake.**

---

## Quick Start with Docker

### Building the Image

```bash
# Build with default version
docker build -t go-tor:latest .

# Build with specific version
docker build -t go-tor:v0.1.0 \
  --build-arg VERSION=v0.1.0 \
  --build-arg BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S') \
  .
```

### Running the Container

```bash
# Basic run with SOCKS proxy on port 9050
docker run -d --name tor-client \
  -p 9050:9050 \
  go-tor:latest

# Run with all ports and metrics enabled
docker run -d --name tor-client \
  -p 9050:9050 \
  -p 9051:9051 \
  -p 9052:9052 \
  go-tor:latest \
  -data-dir /home/nonroot/.tor \
  -socks-port 9050 \
  -control-port 9051 \
  -metrics-port 9052 \
  -log-level info

# Run with persistent data volume
docker run -d --name tor-client \
  -p 9050:9050 \
  -v tor-data:/home/nonroot/.tor \
  go-tor:latest
```

### Testing the Container

```bash
# Check if container is running
docker ps

# View logs
docker logs tor-client

# Follow logs
docker logs -f tor-client

# Test SOCKS5 proxy
curl --socks5 127.0.0.1:9050 https://check.torproject.org

# Check metrics (if enabled)
curl http://localhost:9052/metrics
curl http://localhost:9052/health

# Access dashboard
open http://localhost:9052/debug/metrics
```

## Docker Compose

Docker Compose provides a convenient way to manage container configuration:

```bash
# Start services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down

# Stop and remove volumes
docker-compose down -v

# Rebuild and start
docker-compose up -d --build
```

### Custom Configuration with Compose

Create a `docker-compose.override.yml` file:

```yaml
version: '3.8'

services:
  tor-client:
    environment:
      - TZ=America/New_York
    command:
      - "-socks-port"
      - "9050"
      - "-log-level"
      - "debug"
```

## Image Details

### Image Characteristics

- **Base Image**: `gcr.io/distroless/static-debian12:nonroot`
- **Architecture**: linux/amd64 (additional architectures planned)
- **Image Size**: ~15-20MB (compressed)
- **User**: non-root (uid/gid 65532)
- **Security**: Read-only root filesystem capable

### What's Inside

The image contains:
- Statically compiled `tor-client` binary (no external dependencies)
- CA certificates for TLS connections (`/etc/ssl/certs/`)
- Timezone data (`/usr/share/zoneinfo`)
- Non-root user setup for security

### Exposed Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 9050 | TCP | SOCKS5 proxy server |
| 9051 | TCP | Control protocol (optional) |
| 9052 | TCP | HTTP metrics endpoint (optional) |

### Volumes

| Path | Purpose |
|------|---------|
| `/home/nonroot/.tor` | Persistent Tor data (consensus, guard state, etc.) |

## Security Best Practices

### 1. Use Non-Root User

The image runs as the `nonroot` user (uid 65532) by default. Never run as root:

```bash
# Good - uses default non-root user
docker run go-tor:latest

# Bad - don't do this
docker run --user root go-tor:latest
```

### 2. Read-Only Root Filesystem

Enable read-only root filesystem for additional security:

```bash
docker run -d --name tor-client \
  --read-only \
  --tmpfs /tmp:size=10M,mode=1777 \
  -v tor-data:/home/nonroot/.tor \
  -p 9050:9050 \
  go-tor:latest
```

### 3. Resource Limits

Always set resource limits to prevent resource exhaustion:

```bash
docker run -d --name tor-client \
  --memory="128m" \
  --memory-swap="128m" \
  --cpus="0.5" \
  -p 9050:9050 \
  go-tor:latest
```

### 4. Network Isolation

Use custom networks for better isolation:

```bash
# Create isolated network
docker network create tor-network

# Run container in isolated network
docker run -d --name tor-client \
  --network tor-network \
  -p 9050:9050 \
  go-tor:latest
```

### 5. Security Options

Enable additional security options:

```bash
docker run -d --name tor-client \
  --security-opt=no-new-privileges:true \
  --cap-drop=ALL \
  -p 9050:9050 \
  go-tor:latest
```

## Health Checks

### Docker Health Check

The Dockerfile includes a commented health check. For production use with metrics enabled:

```dockerfile
HEALTHCHECK --interval=30s --timeout=10s --start-period=90s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9052/health || exit 1
```

**Note**: Distroless images don't include shell utilities. For health checks:

1. **Enable metrics** with `-metrics-port 9052`
2. **Use orchestration health checks** (Kubernetes liveness/readiness probes)
3. **External monitoring** of SOCKS port availability

### Kubernetes Health Check

Example Kubernetes health probes:

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 9052
  initialDelaySeconds: 90
  periodSeconds: 30
  timeoutSeconds: 10
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /health
    port: 9052
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3
```

## Kubernetes Deployment

### Basic Deployment

Create `deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: go-tor
  labels:
    app: go-tor
spec:
  replicas: 1
  selector:
    matchLabels:
      app: go-tor
  template:
    metadata:
      labels:
        app: go-tor
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
        fsGroup: 65532
        seccompProfile:
          type: RuntimeDefault
      containers:
      - name: tor-client
        image: go-tor:latest
        imagePullPolicy: IfNotPresent
        args:
        - "-data-dir"
        - "/home/nonroot/.tor"
        - "-socks-port"
        - "9050"
        - "-control-port"
        - "9051"
        - "-metrics-port"
        - "9052"
        - "-log-level"
        - "info"
        ports:
        - containerPort: 9050
          name: socks
          protocol: TCP
        - containerPort: 9051
          name: control
          protocol: TCP
        - containerPort: 9052
          name: metrics
          protocol: TCP
        livenessProbe:
          httpGet:
            path: /health
            port: metrics
          initialDelaySeconds: 90
          periodSeconds: 30
          timeoutSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: metrics
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
        resources:
          requests:
            memory: "64Mi"
            cpu: "250m"
          limits:
            memory: "128Mi"
            cpu: "500m"
        volumeMounts:
        - name: tor-data
          mountPath: /home/nonroot/.tor
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities:
            drop:
            - ALL
      volumes:
      - name: tor-data
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: go-tor
spec:
  selector:
    app: go-tor
  ports:
  - name: socks
    port: 9050
    targetPort: 9050
  - name: control
    port: 9051
    targetPort: 9051
  - name: metrics
    port: 9052
    targetPort: 9052
```

Deploy to Kubernetes:

```bash
kubectl apply -f deployment.yaml
kubectl get pods -l app=go-tor
kubectl logs -f -l app=go-tor
```

### Persistent Storage in Kubernetes

For production deployments with persistent guard state:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: tor-data
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
---
# In Deployment, replace emptyDir with:
      volumes:
      - name: tor-data
        persistentVolumeClaim:
          claimName: tor-data
```

## Advanced Configuration

### Custom Configuration File

Mount a custom `torrc` configuration:

```bash
# Create custom torrc
cat > torrc <<EOF
SocksPort 9050
ControlPort 9051
DataDirectory /home/nonroot/.tor
Log notice stdout
EOF

# Mount as volume
docker run -d --name tor-client \
  -v $(pwd)/torrc:/home/nonroot/.tor/torrc:ro \
  -p 9050:9050 \
  go-tor:latest -config /home/nonroot/.tor/torrc
```

### Environment-Based Configuration

Create wrapper script for environment-based config:

```bash
#!/bin/sh
exec /usr/local/bin/tor-client \
  -socks-port ${SOCKS_PORT:-9050} \
  -control-port ${CONTROL_PORT:-9051} \
  -metrics-port ${METRICS_PORT:-9052} \
  -log-level ${LOG_LEVEL:-info} \
  -data-dir ${DATA_DIR:-/home/nonroot/.tor}
```

### Multi-Architecture Builds

Build for multiple architectures using buildx:

```bash
# Create buildx builder
docker buildx create --name multiarch --use

# Build for multiple platforms
docker buildx build \
  --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t go-tor:latest \
  --push \
  .
```

## Monitoring and Observability

### Prometheus Metrics

The metrics endpoint provides Prometheus-compatible metrics:

```bash
# Scrape metrics
curl http://localhost:9052/metrics

# View JSON format
curl http://localhost:9052/metrics/json

# Access dashboard
open http://localhost:9052/debug/metrics
```

### Prometheus Configuration

Add to `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'go-tor'
    static_configs:
      - targets: ['tor-client:9052']
```

### Kubernetes ServiceMonitor

For Prometheus Operator:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: go-tor
  labels:
    app: go-tor
spec:
  selector:
    matchLabels:
      app: go-tor
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

## Troubleshooting

### Container Won't Start

```bash
# Check logs
docker logs tor-client

# Common issues:
# 1. Port already in use - change port mapping
# 2. Permission denied - ensure volume permissions correct
# 3. Out of memory - increase resource limits
```

### No Network Connectivity

```bash
# Verify SOCKS proxy is listening
docker exec tor-client netstat -tlnp | grep 9050

# Test from inside container
docker exec tor-client curl --socks5 127.0.0.1:9050 https://check.torproject.org
```

### Slow Circuit Building

Circuit building takes time on first run:
- First consensus download: ~30-60 seconds
- First circuit build: ~60-90 seconds
- Subsequent starts: ~30-60 seconds (cached consensus)

Enable debug logging to diagnose:

```bash
docker run -d --name tor-client \
  -p 9050:9050 \
  go-tor:latest -log-level debug
```

### Health Check Failures

Ensure metrics are enabled:

```bash
# Health checks require metrics server
docker run -d --name tor-client \
  -p 9050:9050 \
  -p 9052:9052 \
  go-tor:latest -metrics-port 9052

# Test health endpoint
curl http://localhost:9052/health
```

## Performance Tuning

### Resource Requirements

Recommended resources by workload:

| Workload | Memory | CPU | Notes |
|----------|--------|-----|-------|
| Light (< 10 circuits) | 64 MB | 0.25 cores | Basic usage |
| Medium (10-50 circuits) | 128 MB | 0.5 cores | Typical usage |
| Heavy (> 50 circuits) | 256 MB | 1 core | High throughput |

### Optimization Tips

1. **Persistent volumes**: Use persistent storage for faster startup
2. **Guard persistence**: Maintains stable guard nodes across restarts
3. **Circuit pooling**: Enabled by default in Phase 9.4
4. **Resource limits**: Set appropriate limits to prevent resource exhaustion

## Security Considerations

### Container Security Scanning

Scan the image for vulnerabilities:

```bash
# Using Trivy
trivy image go-tor:latest

# Using Docker scan
docker scan go-tor:latest

# Using Grype
grype go-tor:latest
```

### Hardening Checklist

- [ ] Run as non-root user (default)
- [ ] Enable read-only root filesystem
- [ ] Drop all capabilities
- [ ] Set resource limits
- [ ] Use private networks
- [ ] Enable security options (no-new-privileges)
- [ ] Regular image updates
- [ ] Scan for vulnerabilities
- [ ] Monitor security advisories

## Production Deployment Checklist

Before deploying to production:

- [ ] Review security best practices
- [ ] Configure persistent storage
- [ ] Set up monitoring and alerting
- [ ] Configure resource limits
- [ ] Test health checks
- [ ] Document deployment procedures
- [ ] Set up backup procedures
- [ ] Configure log aggregation
- [ ] Test disaster recovery
- [ ] Review and update documentation

## Additional Resources

- [Docker Security Best Practices](https://docs.docker.com/develop/security-best-practices/)
- [Kubernetes Security Best Practices](https://kubernetes.io/docs/concepts/security/overview/)
- [Distroless Container Images](https://github.com/GoogleContainerTools/distroless)
- [go-tor Documentation](https://github.com/opd-ai/go-tor/tree/main/docs)

## Support

For issues and questions:
- GitHub Issues: https://github.com/opd-ai/go-tor/issues
- Documentation: https://github.com/opd-ai/go-tor/tree/main/docs

**Remember**: This is educational software. For real anonymity needs, use official Tor Browser or Tor software from torproject.org.

# Kubernetes Deployment

This directory contains Kubernetes manifests for deploying go-tor.

## Quick Start

### Using Kustomize

```bash
# Apply all resources (creates the `go-tor` namespace automatically)
kubectl apply -k deploy/kubernetes/
```

### Using kubectl directly

```bash
# Create namespace
kubectl apply -f deploy/kubernetes/namespace.yaml

# Apply remaining resources
kubectl apply -f deploy/kubernetes/configmap.yaml
kubectl apply -f deploy/kubernetes/deployment.yaml
kubectl apply -f deploy/kubernetes/service.yaml
kubectl apply -f deploy/kubernetes/pdb.yaml
```

## Resources

| File | Description |
|------|-------------|
| `namespace.yaml` | Creates the `go-tor` namespace |
| `configmap.yaml` | Production torrc configuration |
| `deployment.yaml` | Deployment with health checks, security context, and resource limits |
| `service.yaml` | ClusterIP service exposing SOCKS, control, and metrics ports |
| `pdb.yaml` | PodDisruptionBudget for high availability |
| `kustomization.yaml` | Kustomize configuration for easy deployment |

## Configuration

### Ports

| Port | Name | Description |
|------|------|-------------|
| 9050 | socks | SOCKS5 proxy for routing traffic through Tor |
| 9051 | control | Tor control protocol |
| 9052 | metrics | Prometheus metrics and health endpoints |

### Resource Limits

Default resource configuration:
- Requests: 64Mi memory, 100m CPU
- Limits: 256Mi memory, 500m CPU

Adjust in `deployment.yaml` based on your workload.

### Health Checks

The deployment includes three types of probes:
- **Startup probe**: Allows up to 3 minutes for initial bootstrap
- **Liveness probe**: Checks `/health` endpoint every 30s
- **Readiness probe**: Checks `/health` endpoint every 10s

## Accessing the Service

### Port Forward (Development)

```bash
# Forward SOCKS proxy and metrics
kubectl -n go-tor port-forward svc/go-tor 9050:9050 9052:9052

# Test with curl
curl --socks5 127.0.0.1:9050 https://check.torproject.org

# View metrics
curl http://127.0.0.1:9052/metrics

# Check health
curl http://127.0.0.1:9052/health
```

### From Other Pods

Applications in the same cluster can use:
- SOCKS proxy: `socks5://go-tor.go-tor.svc.cluster.local:9050`

## Security Considerations

The deployment follows Kubernetes security best practices:

- Runs as non-root user (UID 65532)
- Read-only root filesystem
- No privilege escalation
- All capabilities dropped
- Seccomp profile enabled
- Resource limits enforced

## Customization

### Using a Custom Image

Edit `deployment.yaml`:
```yaml
containers:
  - name: go-tor
    image: your-registry/go-tor:v1.0.0
```

### Persistent Storage

To persist Tor data across restarts, modify the volume:
```yaml
volumes:
  - name: tor-data
    persistentVolumeClaim:
      claimName: go-tor-data
```

## Monitoring

The deployment includes Prometheus annotations for automatic scraping:
```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "9052"
  prometheus.io/path: "/metrics"
```

## Troubleshooting

### Check pod status
```bash
kubectl -n go-tor get pods
kubectl -n go-tor describe pod <pod-name>
```

### View logs
```bash
kubectl -n go-tor logs -f deployment/go-tor
```

### Check events
```bash
kubectl -n go-tor get events --sort-by='.lastTimestamp'
```

## Warning

**This is an educational implementation.** Do NOT use for real anonymity or privacy-critical applications. Use the official Tor Browser or Arti for genuine anonymity needs.

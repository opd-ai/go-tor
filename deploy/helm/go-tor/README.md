# go-tor Helm Chart

A Helm chart for deploying go-tor, a pure Go Tor client implementation.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.2+

## Installing the Chart

```bash
# Add the chart repository (if hosted)
# helm repo add go-tor https://example.com/charts

# Install from local directory
helm install my-tor ./deploy/helm/go-tor

# Install with custom values
helm install my-tor ./deploy/helm/go-tor -f my-values.yaml

# Install in specific namespace
helm install my-tor ./deploy/helm/go-tor --namespace tor-proxy --create-namespace
```

## Uninstalling the Chart

```bash
helm uninstall my-tor
```

## Configuration

See [values.yaml](values.yaml) for the full list of configurable parameters.

### Common Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas | `1` |
| `image.repository` | Container image repository | `go-tor` |
| `image.tag` | Container image tag | Chart appVersion |
| `service.type` | Kubernetes service type | `ClusterIP` |
| `service.socksPort` | SOCKS5 proxy port | `9050` |
| `service.controlPort` | Control protocol port | `9051` |
| `service.metricsPort` | Metrics HTTP port | `9052` |

### Tor Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.logLevel` | Log level (debug, info, warn, error) | `info` |
| `config.numEntryGuards` | Number of entry guards | `3` |
| `config.useEntryGuards` | Enable entry guards | `true` |
| `config.circuitBuildTimeout` | Circuit build timeout | `60s` |
| `config.maxCircuitDirtiness` | Max circuit age before rotation | `10m` |
| `config.newCircuitPeriod` | New circuit creation period | `30s` |
| `config.connLimit` | Maximum connections | `1000` |
| `config.dormantTimeout` | Dormant timeout | `24h` |

> **Note:** Advanced options like connection pooling, circuit prebuilding, buffer pooling, and isolation levels are configured programmatically by go-tor and are not available via torrc configuration.

### Resource Management

| Parameter | Description | Default |
|-----------|-------------|---------|
| `resources.requests.memory` | Memory request | `64Mi` |
| `resources.requests.cpu` | CPU request | `100m` |
| `resources.limits.memory` | Memory limit | `256Mi` |
| `resources.limits.cpu` | CPU limit | `500m` |

### Persistence

| Parameter | Description | Default |
|-----------|-------------|---------|
| `persistence.enabled` | Enable persistent storage | `false` |
| `persistence.size` | PVC size | `1Gi` |
| `persistence.storageClass` | Storage class name | `""` |

### High Availability

| Parameter | Description | Default |
|-----------|-------------|---------|
| `podDisruptionBudget.enabled` | Enable PDB | `true` |
| `podDisruptionBudget.minAvailable` | Min available pods | `0` |

## Examples

### High Availability Deployment

```yaml
# ha-values.yaml
replicaCount: 3

podDisruptionBudget:
  enabled: true
  minAvailable: 1

resources:
  requests:
    memory: "128Mi"
    cpu: "200m"
  limits:
    memory: "512Mi"
    cpu: "1000m"

persistence:
  enabled: true
  size: 2Gi
```

```bash
helm install tor-ha ./deploy/helm/go-tor -f ha-values.yaml
```

### Privacy-Focused Deployment

```yaml
# privacy-values.yaml
config:
  numEntryGuards: 4
  circuitBuildTimeout: "90s"
```

```bash
helm install tor-private ./deploy/helm/go-tor -f privacy-values.yaml
```

## Accessing the Service

After installation, follow the notes printed by Helm, or:

```bash
# Port forward
kubectl port-forward svc/my-tor-go-tor 9050:9050 9052:9052

# Test
curl --socks5 127.0.0.1:9050 https://check.torproject.org

# View metrics
curl http://127.0.0.1:9052/metrics
```

## Security

This chart follows security best practices:

- Non-root container (UID 65532)
- Read-only root filesystem
- No privilege escalation
- Dropped capabilities
- Seccomp profile enabled
- Resource limits enforced
- ServiceAccount with minimal permissions

## Warning

**This is an educational implementation.** Do NOT use for real anonymity or privacy-critical applications. Use the official Tor Browser or Arti for genuine anonymity needs.

# Runtime Profiling Guide

This guide explains how to use go-tor's integrated runtime profiling capabilities for performance analysis and debugging.

## Overview

The profiling package provides comprehensive runtime profiling through HTTP endpoints, including:

- **CPU Profiling**: Identify CPU-intensive code paths
- **Heap Profiling**: Analyze memory allocation patterns
- **Goroutine Profiling**: Debug goroutine leaks and concurrency issues
- **Mutex/Block Profiling**: Identify lock contention and blocking operations
- **Custom Statistics**: Track goroutine counts, memory usage, and GC activity

## Security Warning

⚠️ **IMPORTANT**: Profiling endpoints expose detailed runtime information about your application. Only enable profiling in controlled environments. **Never** expose profiling endpoints on public networks without proper authentication and access controls.

## Configuration

### Enabling Profiling

Add these configuration options to enable profiling:

```go
cfg := &config.Config{
    EnableProfiling:     true,           // Enable profiling endpoints
    ProfilingPort:       0,              // 0 = use metrics port
    ProfilingPath:       "/debug/pprof", // URL path prefix
    EnableCPUProfiling:  true,           // Enable CPU profiling
    EnableHeapProfiling: true,           // Enable heap profiling
    EnableMutexProfile:  false,          // Disabled (high overhead)
    EnableBlockProfile:  false,          // Disabled (high overhead)
    MutexProfileRate:    0,              // 0 = disabled
    BlockProfileRate:    0,              // 0 = disabled (nanoseconds)
}
```

### Command-Line Flags

Enable profiling when starting tor-client:

```bash
# Enable profiling on metrics port (9052)
./bin/tor-client -metrics-port 9052 -enable-profiling

# Enable with mutex and block profiling (high overhead)
./bin/tor-client -metrics-port 9052 -enable-profiling \
    -enable-mutex-profile -mutex-profile-rate 1 \
    -enable-block-profile -block-profile-rate 100000
```

## Available Endpoints

When profiling is enabled, the following endpoints are available (default prefix: `/debug/pprof`):

### Core pprof Endpoints

| Endpoint | Description | Usage |
|----------|-------------|-------|
| `/debug/pprof/` | Index page listing all profiles | View in browser |
| `/debug/pprof/profile` | 30-second CPU profile | `go tool pprof` |
| `/debug/pprof/heap` | Heap allocation profile | `go tool pprof` |
| `/debug/pprof/allocs` | All past memory allocations | `go tool pprof` |
| `/debug/pprof/goroutine` | Stack traces of all goroutines | `go tool pprof` |
| `/debug/pprof/threadcreate` | Thread creation profile | `go tool pprof` |
| `/debug/pprof/mutex` | Mutex contention profile (if enabled) | `go tool pprof` |
| `/debug/pprof/block` | Blocking profile (if enabled) | `go tool pprof` |
| `/debug/pprof/cmdline` | Command-line arguments | View in browser |
| `/debug/pprof/symbol` | Symbol lookup | Internal use |

### Custom Statistics Endpoints

| Endpoint | Description | Returns |
|----------|-------------|---------|
| `/debug/pprof/stats` | Runtime statistics | JSON |
| `/debug/pprof/memory` | Detailed memory statistics | JSON |
| `/debug/pprof/goroutine-leak` | Goroutine leak detection | JSON |
| `/debug/pprof/gc` | Trigger manual GC (POST only) | JSON |

## Usage Examples

### CPU Profiling

Capture a 30-second CPU profile:

```bash
# Capture profile
curl http://localhost:9052/debug/pprof/profile?seconds=30 > cpu.prof

# Analyze with pprof
go tool pprof cpu.prof

# Interactive mode commands:
# top10    - Show top 10 CPU consumers
# list main.functionName - Show source code with annotations
# web      - Generate callgraph visualization (requires graphviz)
```

### Heap Profiling

Analyze memory allocation:

```bash
# Capture heap profile
curl http://localhost:9052/debug/pprof/heap > heap.prof

# Analyze allocations
go tool pprof heap.prof

# Common pprof commands:
# top10    - Show functions with most allocations
# list     - Show source with allocation annotations
# inuse_space  - Sort by memory in use
# alloc_objects - Sort by allocation count
```

### Goroutine Analysis

Debug goroutine leaks:

```bash
# Check for potential leaks
curl http://localhost:9052/debug/pprof/goroutine-leak | jq .

# Example output:
# {
#   "current_goroutines": 42,
#   "peak_goroutines": 50,
#   "leak_rate_per_second": 0.1,
#   "likely_leaking": false,
#   "recommendation": "No goroutine leak detected. Goroutine count is stable."
# }

# Get full goroutine stack traces
curl http://localhost:9052/debug/pprof/goroutine?debug=2 > goroutines.txt

# Analyze with pprof
curl http://localhost:9052/debug/pprof/goroutine > goroutine.prof
go tool pprof goroutine.prof
```

### Memory Statistics

Get detailed memory stats:

```bash
curl http://localhost:9052/debug/pprof/memory | jq .

# Example output:
# {
#   "heap_alloc_bytes": 5242880,
#   "heap_sys_bytes": 67108864,
#   "heap_idle_bytes": 61865984,
#   "heap_inuse_bytes": 5242880,
#   "num_gc": 42,
#   ...
# }
```

### Runtime Statistics

Monitor overall runtime health:

```bash
curl http://localhost:9052/debug/pprof/stats | jq .

# Example output:
# {
#   "num_goroutines": 42,
#   "peak_goroutines": 50,
#   "heap_alloc_bytes": 5242880,
#   "peak_heap_bytes": 10485760,
#   "total_alloc_bytes": 104857600,
#   "num_gc": 42,
#   "last_gc_time": "2026-01-23T19:00:00Z",
#   "goroutine_leak_rate": 0.1
# }
```

### Manual Garbage Collection

Trigger GC for debugging:

```bash
# Trigger GC (POST request required)
curl -X POST http://localhost:9052/debug/pprof/gc | jq .

# Example output:
# {
#   "gc_triggered": true,
#   "heap_before_bytes": 10485760,
#   "heap_after_bytes": 5242880,
#   "freed_bytes": 5242880,
#   "num_gc": 43
# }
```

## Analyzing Performance Issues

### Identifying CPU Hotspots

1. Capture a CPU profile during high load:
   ```bash
   curl http://localhost:9052/debug/pprof/profile?seconds=60 > cpu.prof
   ```

2. Analyze with pprof:
   ```bash
   go tool pprof -http=:8080 cpu.prof
   ```

3. Look for:
   - Functions with high `flat%` (time spent in function itself)
   - Functions with high `cum%` (time spent in function + callees)
   - Unexpected loops or allocations

### Finding Memory Leaks

1. Capture heap profile:
   ```bash
   curl http://localhost:9052/debug/pprof/heap > heap.prof
   ```

2. Compare with baseline:
   ```bash
   # Baseline
   curl http://localhost:9052/debug/pprof/heap > heap1.prof
   # Wait some time
   sleep 300
   # New snapshot
   curl http://localhost:9052/debug/pprof/heap > heap2.prof
   
   # Compare
   go tool pprof -base heap1.prof heap2.prof
   ```

3. Look for:
   - Functions with increasing allocations over time
   - Objects that aren't being released

### Debugging Goroutine Leaks

1. Check leak detection:
   ```bash
   curl http://localhost:9052/debug/pprof/goroutine-leak
   ```

2. If leak detected, capture goroutine profile:
   ```bash
   curl http://localhost:9052/debug/pprof/goroutine?debug=2 > goroutines.txt
   ```

3. Look for:
   - Goroutines waiting on channels that never close
   - Goroutines stuck in infinite loops
   - Goroutines waiting on mutexes

## Best Practices

### 1. Security

- **Never** expose profiling endpoints on public networks
- Use firewall rules to restrict access to profiling port
- Consider adding authentication middleware if needed
- Disable profiling in production unless actively debugging

### 2. Performance Impact

- CPU profiling has minimal overhead (~5%)
- Heap profiling has low overhead
- **Mutex profiling** can have significant overhead (10-30%)
- **Block profiling** can have significant overhead (10-30%)
- Only enable mutex/block profiling when actively debugging

### 3. Profiling Workflow

1. **Baseline**: Capture profile before changes
2. **Reproduce**: Run workload that exhibits the issue
3. **Capture**: Collect profile during the issue
4. **Compare**: Use `-base` flag to compare profiles
5. **Fix**: Address identified hotspots
6. **Verify**: Capture new profile to confirm improvement

### 4. Integration with Monitoring

```go
// Check goroutine leak rate periodically
profiler := profiling.NewProfiler(cfg, logger)
defer profiler.Close()

ticker := time.NewTicker(1 * time.Minute)
for range ticker.C {
    if profiler.DetectGoroutineLeaks(ctx, 1.0) {
        logger.Warn("Potential goroutine leak detected")
        // Trigger alert or capture profile
    }
}
```

## Continuous Profiling

For production monitoring, consider:

1. **Periodic Snapshots**: Capture profiles on schedule
2. **Triggered Captures**: Capture on high resource usage
3. **Profile Storage**: Store profiles for historical analysis
4. **Automated Analysis**: Run pprof analysis in CI/CD

Example periodic capture script:

```bash
#!/bin/bash
# capture-profiles.sh

PROFILE_DIR="/var/log/go-tor/profiles"
METRICS_URL="http://localhost:9052/debug/pprof"

mkdir -p "$PROFILE_DIR"

while true; do
    TIMESTAMP=$(date +%Y%m%d_%H%M%S)
    
    # CPU profile (30s)
    curl "$METRICS_URL/profile?seconds=30" > "$PROFILE_DIR/cpu_$TIMESTAMP.prof"
    
    # Heap profile
    curl "$METRICS_URL/heap" > "$PROFILE_DIR/heap_$TIMESTAMP.prof"
    
    # Goroutine profile
    curl "$METRICS_URL/goroutine" > "$PROFILE_DIR/goroutine_$TIMESTAMP.prof"
    
    # Statistics
    curl "$METRICS_URL/stats" > "$PROFILE_DIR/stats_$TIMESTAMP.json"
    
    # Wait 5 minutes
    sleep 300
done
```

## Troubleshooting

### Profiling endpoints return 404

- Verify `EnableProfiling` is set to `true`
- Check profiling port matches metrics port
- Verify path prefix configuration

### High overhead from mutex/block profiling

- Reduce `MutexProfileRate` (higher value = less sampling)
- Reduce `BlockProfileRate` (higher value = less sampling)
- Only enable when actively debugging

### pprof command not found

```bash
# Install pprof
go install github.com/google/pprof@latest

# Or use built-in tool
go tool pprof
```

### Profiles show no data

- Ensure application is under load when capturing
- Check profile capture duration (increase `?seconds=` parameter)
- Verify profiling types are enabled in configuration

## References

- [Go pprof Documentation](https://pkg.go.dev/runtime/pprof)
- [Profiling Go Programs](https://go.dev/blog/pprof)
- [Diagnostics](https://go.dev/doc/diagnostics)
- [High Performance Go Workshop](https://dave.cheney.net/high-performance-go-workshop/gopherchina-2019.html)

## See Also

- [METRICS.md](METRICS.md) - Metrics and monitoring
- [PRODUCTION.md](PRODUCTION.md) - Production deployment guide
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Common issues

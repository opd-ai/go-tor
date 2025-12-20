# Security Remediation Plan

This document provides step-by-step resolution guidance for all security issues identified in [ROADMAP.md](ROADMAP.md), [AUDIT.md](AUDIT.md), and additional security concerns found in the codebase.

**Document Status**: Active Planning Document  
**Last Updated**: 2025-12-20  
**Target Completion**: Production readiness

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Priority 1: Critical Security Issues (Resolved)](#priority-1-critical-security-issues-resolved)
3. [Priority 2: High Severity Issues (Remaining)](#priority-2-high-severity-issues-remaining)
4. [Priority 3: Medium Severity Issues](#priority-3-medium-severity-issues)
5. [Priority 4: Low Severity Issues](#priority-4-low-severity-issues)
6. [Codebase Security Concerns](#codebase-security-concerns)
7. [Implementation Timeline](#implementation-timeline)
8. [Verification Procedures](#verification-procedures)

---

## Executive Summary

### Current Security Posture

| Category | Status | Count |
|----------|--------|-------|
| **Critical Issues** | ✅ All Resolved | 8/8 |
| **High Severity** | ⚠️ Partially Complete | 8/12 remaining |
| **Medium Severity** | ⚠️ Partially Complete | 6/7 remaining |
| **Low Severity** | 📋 Planned | 7/8 remaining |

### Key Findings

The go-tor implementation demonstrates solid engineering practices:
- ✅ No use of `unsafe` package
- ✅ Proper use of `crypto/rand` for security-sensitive randomness
- ✅ No `log.Fatal()` or `os.Exit()` in library code (pkg/)
- ✅ Race detector clean
- ✅ Comprehensive constant-time operations
- ✅ CI/CD security scanning (gosec, govulncheck, CodeQL, Trivy)

**Remaining Priority Work**:
- Error context in logs (correlation IDs)
- Rate limiting and backpressure
- Guard persistence improvements
- Test coverage improvements for critical packages

---

## Priority 1: Critical Security Issues (Resolved)

All Phase 1 critical issues from ROADMAP.md have been resolved:

### 1.1 ✅ DNS Leak Prevention (ROADMAP 1.2)

**Status**: COMPLETE

**Resolution**: Full RELAY_RESOLVE/RELAY_RESOLVED implementation prevents DNS leaks to local resolvers.

**Verification**:
```bash
# Run DNS resolution tests
go test -v ./pkg/socks/... -run TestResolve
```

**Files**: `pkg/socks/socks.go`

---

### 1.2 ✅ Ntor Handshake Implementation (ROADMAP 1.3, AUDIT MED-001)

**Status**: COMPLETE

**Resolution**: Full ntor handshake with AUTH MAC validation and key derivation per tor-spec.txt §5.1.4.

**Verification**:
```bash
go test -v ./pkg/circuit/... -run TestNtor
go test -v ./pkg/crypto/... -run TestNtor
```

**Files**: `pkg/crypto/crypto.go`, `pkg/circuit/extension.go`

---

### 1.3 ✅ Descriptor Signature Verification (ROADMAP 1.4, AUDIT MED-002)

**Status**: COMPLETE

**Resolution**: Added `VerifyDescriptorSignature` call in `FetchDescriptor` after parsing. Failed verification causes retry with next HSDir.

**Verification**:
```bash
go test -v ./pkg/onion/... -run TestDescriptor
```

**Files**: `pkg/onion/onion.go`

---

### 1.4 ✅ Replay Protection (ROADMAP 1.5, AUDIT MED-003)

**Status**: COMPLETE

**Resolution**: `ReplayProtection` in `pkg/cell/replay.go` with sliding window sequence tracking and digest-based duplicate detection.

**Verification**:
```bash
go test -v ./pkg/cell/... -run TestReplay
go test -v ./pkg/circuit/... -run TestReplay
```

**Files**: `pkg/cell/replay.go`, `pkg/circuit/circuit.go`

---

### 1.5 ✅ Graceful Shutdown (ROADMAP 1.6)

**Status**: COMPLETE

**Resolution**: Context-aware shutdown with configurable timeouts and drain period.

**Verification**:
```bash
go test -v ./pkg/httpmetrics/... -run TestGraceful
```

**Files**: `pkg/httpmetrics/server.go`

---

### 1.6 ✅ Circuit Padding (ROADMAP 2.1, AUDIT MED-005)

**Status**: COMPLETE

**Resolution**: PaddingMachine with configurable strategies (none, fixed, random, adaptive).

**Verification**:
```bash
go test -v ./pkg/circuit/... -run TestPadding
```

**Files**: `pkg/circuit/padding.go`, `pkg/config/config.go`

---

### 1.7 ✅ Stream Isolation Enforcement (ROADMAP 2.2, AUDIT MED-006)

**Status**: COMPLETE

**Resolution**: `IsolationEnforcer` with configurable modes (off, warn, strict).

**Verification**:
```bash
go test -v ./pkg/stream/... -run TestIsolation
```

**Files**: `pkg/stream/isolation.go`, `pkg/circuit/isolation.go`

---

### 1.8 ✅ Mock Cleanup (ROADMAP 2.3)

**Status**: COMPLETE

**Resolution**: Mock fallbacks removed; proper error returns for nil dependencies.

**Verification**:
```bash
grep -r "createMockDescriptor" pkg/  # Should return no results
go test -v ./pkg/onion/...
```

**Files**: `pkg/onion/onion.go`

---

## Priority 2: High Severity Issues (Remaining)

### 2.1 ✅ CI/CD Security Scanning (ROADMAP 2.9)

**Status**: COMPLETE

**Priority**: HIGH  
**Effort**: 2 days

**Solution**:
- [x] Create security workflow file (`.github/workflows/security.yml`)
- [x] Add gosec SAST scanning with SHA-pinned action
- [x] Add govulncheck for dependency vulnerabilities
- [x] Add Trivy container image scanning
- [x] Add CodeQL analysis
- [x] Create Dependabot configuration (`.github/dependabot.yml`)

**Implementation Details**:
- Created `.github/workflows/security.yml` with four security scanning jobs:
  - **gosec**: SAST scanning with SARIF output uploaded to GitHub Security
  - **govulncheck**: Go vulnerability database scanning for dependencies
  - **trivy**: Container image scanning for the Docker image
  - **codeql**: Semantic code analysis for Go
- Created `.github/dependabot.yml` with automated updates for:
  - Go modules (weekly on Mondays)
  - GitHub Actions (weekly on Mondays)
  - Docker base images (weekly on Mondays)
- All GitHub Actions are pinned to immutable SHAs for supply chain security

**Files Created**:
- `.github/workflows/security.yml`
- `.github/dependabot.yml`

---

### 2.2 🔴 Error Context in Logs (ROADMAP 2.10)

**Status**: NOT IMPLEMENTED

**Priority**: HIGH  
**Effort**: 2 days

**Problem**: Many error logs lack sufficient context (request IDs, circuit IDs, correlation IDs).

**Step-by-Step Resolution**:

1. **Add correlation ID to logger package**:
   ```go
   // pkg/logger/context.go
   type contextKey string
   const CorrelationIDKey contextKey = "correlation_id"
   
   func WithCorrelationID(ctx context.Context, id string) context.Context
   func GetCorrelationID(ctx context.Context) string
   func (l *Logger) WithContext(ctx context.Context) *Logger
   ```

2. **Add request ID generation**:
   ```go
   // pkg/logger/request_id.go
   import (
       "crypto/rand"
       "encoding/hex"
       "fmt"
   )
   
   // GenerateRequestID creates a cryptographically random 16-character request ID.
   // Returns the ID and an error if random generation fails.
   func GenerateRequestID() (string, error) {
       b := make([]byte, 8)
       if _, err := rand.Read(b); err != nil {
           return "", fmt.Errorf("failed to generate request ID: %w", err)
       }
       return hex.EncodeToString(b), nil
   }
   ```

3. **Propagate context through circuit operations**:
   ```go
   // Update Circuit methods to accept context
   func (c *Circuit) DeliverRelayCell(ctx context.Context, cell *cell.Cell) error
   ```

4. **Update all logging call sites** to include IDs:
   ```go
   c.logger.Error("Circuit operation failed",
       "circuit_id", c.ID,
       "correlation_id", logger.GetCorrelationID(ctx),
       "error", err)
   ```

5. **Add middleware for SOCKS connections**:
   ```go
   func (s *Server) handleConnection(conn net.Conn) {
       ctx := logger.WithCorrelationID(context.Background(), logger.GenerateRequestID())
       // ... use ctx throughout
   }
   ```

**Files to Modify**:
- `pkg/logger/logger.go` - Add context support
- `pkg/logger/context.go` (new) - Context utilities
- `pkg/circuit/circuit.go` - Add circuit ID to logs
- `pkg/socks/socks.go` - Add correlation IDs
- `pkg/stream/stream.go` - Add stream IDs
- `pkg/connection/connection.go` - Add connection IDs

**Verification**:
```bash
go test -v ./pkg/logger/...
# Check logs contain expected IDs
go run ./cmd/tor-client/... 2>&1 | grep -E "(circuit_id|correlation_id)"
```

---

### 2.3 🔴 Rate Limiting and Backpressure (ROADMAP 2.11)

**Status**: NOT IMPLEMENTED

**Priority**: HIGH  
**Effort**: 3 days

**Problem**: No rate limiting for SOCKS connections, circuit creation, or resource consumption.

**Step-by-Step Resolution**:

1. **Create rate limiter package**:
   ```go
   // pkg/ratelimit/limiter.go
   type RateLimiter struct {
       rate       float64
       burst      int
       tokens     float64
       lastUpdate time.Time
       mu         sync.Mutex
   }
   
   func NewRateLimiter(rate float64, burst int) *RateLimiter
   func (r *RateLimiter) Allow() bool
   func (r *RateLimiter) Wait(ctx context.Context) error
   ```

2. **Add SOCKS connection rate limiting**:
   ```go
   // pkg/socks/ratelimit.go
   type RateLimitedServer struct {
       server    *Server
       limiter   *ratelimit.RateLimiter
       maxConns  int
       connCount int64
   }
   ```

3. **Add circuit creation rate limiting**:
   ```go
   // pkg/circuit/ratelimit.go
   type RateLimitedBuilder struct {
       builder *CircuitBuilder
       limiter *ratelimit.RateLimiter
   }
   ```

4. **Implement backpressure for streams**:
   ```go
   // pkg/stream/backpressure.go
   type BackpressureController struct {
       highWaterMark int
       lowWaterMark  int
       currentBuffer int
       paused        bool
   }
   ```

5. **Add configuration options**:
   ```go
   // pkg/config/config.go
   type RateLimitConfig struct {
       SOCKSConnectionsPerSecond float64
       CircuitCreationsPerSecond float64
       MaxConcurrentConnections  int
       StreamBufferHighWaterMark int
       StreamBufferLowWaterMark  int
   }
   ```

6. **Integrate with SOCKS server**:
   ```go
   const rateLimitBackoffDelay = 10 * time.Millisecond
   
   // Server must have a ctx field initialized during NewServer():
   //   type Server struct { ctx context.Context; ... }
   func (s *Server) acceptLoop() {
       for {
           // Use Wait() instead of Allow() to avoid tight loop when rate limited
           if err := s.rateLimiter.Wait(s.ctx); err != nil {
               // Context cancelled, shutdown gracefully
               return
           }
           conn, err := s.listener.Accept()
           if err != nil {
               if s.rateLimiter.Allow() {
                   s.logger.Error("Accept error", "error", err)
               } else {
                   s.metrics.IncrRateLimited()
                   time.Sleep(rateLimitBackoffDelay)
               }
               continue
           }
           // ... handle connection
       }
   }
   ```

**Files to Create**:
- `pkg/ratelimit/limiter.go`
- `pkg/ratelimit/limiter_test.go`
- `pkg/socks/ratelimit.go`
- `pkg/circuit/ratelimit.go`
- `pkg/stream/backpressure.go`

**Files to Modify**:
- `pkg/config/config.go` - Add rate limit config
- `pkg/socks/socks.go` - Integrate rate limiting
- `pkg/metrics/metrics.go` - Add rate limit metrics

**Verification**:
```bash
go test -v ./pkg/ratelimit/...
# Load test to verify rate limiting
go test -v ./pkg/socks/... -run TestRateLimit
```

---

### 2.4 🔴 Guard Persistence with Flatfile Storage (ROADMAP 2.12)

**Status**: PARTIALLY IMPLEMENTED

**Priority**: HIGH  
**Effort**: 3 days

**Problem**: Guard node persistence may need improvements for atomic writes, file locking, and backup.

**Step-by-Step Resolution**:

1. **Implement atomic file writes**:
   ```go
   // pkg/path/persistence.go
   // Recommended: Use github.com/natefinch/atomic for cross-platform atomicity
   import (
       "bytes"
       "github.com/natefinch/atomic"
   )
   
   // atomicWrite writes data to path atomically across platforms.
   // Uses github.com/natefinch/atomic to ensure atomicity on all platforms
   // including Windows, where os.Rename may fail if destination exists.
   func (p *Persistence) atomicWrite(path string, data []byte) error {
       return atomic.WriteFile(path, bytes.NewReader(data))
   }
   ```

2. **Add file locking**:
   ```go
   func (p *Persistence) acquireLock(path string) (*flock.Flock, error) {
       lock := flock.New(path + ".lock")
       if err := lock.Lock(); err != nil {
           return nil, err
       }
       return lock, nil
   }
   ```

3. **Add schema versioning**:
   ```go
   type GuardStateV2 struct {
       Version   int           `json:"version"`
       Guards    []GuardEntry  `json:"guards"`
       UpdatedAt time.Time     `json:"updated_at"`
       Checksum  string        `json:"checksum"`
   }
   ```

4. **Add integrity checks**:
   ```go
   import (
       "crypto/sha256"
       "encoding/hex"
       "encoding/json"
   )
   
   // calculateChecksum returns the hex-encoded SHA-256 checksum of the given data.
   func (p *Persistence) calculateChecksum(data []byte) string {
       h := sha256.Sum256(data)
       return hex.EncodeToString(h[:])
   }
   
   // verifyChecksum recomputes the checksum for the given state (excluding the
   // Checksum field itself) and compares it with the stored Checksum value.
   // Returns true only if the checksum matches.
   func (p *Persistence) verifyChecksum(state *GuardStateV2) bool {
       if state == nil || state.Checksum == "" {
           return false
       }
       // Make a copy and clear Checksum so it's not included in calculation
       copyState := *state
       copyState.Checksum = ""
       
       data, err := json.Marshal(copyState)
       if err != nil {
           return false
       }
       
       expected := p.calculateChecksum(data)
       return expected == state.Checksum
   }
   ```

5. **Implement backup rotation**:
   ```go
   import (
       "fmt"
       "io"
       "os"
   )
   
   // copyFile copies src to dst with proper resource cleanup.
   // Preserves source file permissions (0600) for security.
   func copyFile(src, dst string) error {
       source, err := os.Open(src)
       if err != nil {
           return fmt.Errorf("open source: %w", err)
       }
       defer source.Close()
       
       // Get source file info for permissions
       info, err := source.Stat()
       if err != nil {
           return fmt.Errorf("stat source: %w", err)
       }
       
       dest, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
       if err != nil {
           return fmt.Errorf("create destination: %w", err)
       }
       
       n, err := io.Copy(dest, source)
       if closeErr := dest.Close(); closeErr != nil && err == nil {
           err = fmt.Errorf("close destination: %w", closeErr)
       }
       if err != nil {
           os.Remove(dst)  // Clean up partial copy on error
           return fmt.Errorf("copy failed: %w", err)
       }
       
       // Verify copy was complete
       if n != info.Size() {
           os.Remove(dst)
           return fmt.Errorf("incomplete copy: wrote %d of %d bytes", n, info.Size())
       }
       
       return nil
   }
   
   func (p *Persistence) rotateBackups(path string, keepN int) error {
       // Keep last N backups - rotate from oldest to newest
       for i := keepN - 1; i >= 1; i-- {
           oldPath := fmt.Sprintf("%s.backup.%d", path, i)
           newPath := fmt.Sprintf("%s.backup.%d", path, i+1)
           if err := os.Rename(oldPath, newPath); err != nil {
               // Ignore missing older backups, but fail on other errors
               if !os.IsNotExist(err) {
                   return fmt.Errorf("failed to rotate backup %s to %s: %w", oldPath, newPath, err)
               }
           }
       }
       
       if err := copyFile(path, path+".backup.1"); err != nil {
           return fmt.Errorf("failed to create primary backup for %s: %w", path, err)
       }
       
       return nil
   }
   ```

6. **Add periodic state snapshots**:
   ```go
   import (
       "context"
       "time"
   )
   
   // startSnapshotLoop periodically saves guard state. Accepts context for graceful shutdown.
   func (gm *GuardManager) startSnapshotLoop(ctx context.Context, interval time.Duration) {
       ticker := time.NewTicker(interval)
       defer ticker.Stop()
       
       for {
           select {
           case <-ctx.Done():
               gm.logger.Info("Stopping snapshot loop", "reason", ctx.Err())
               return
           case <-ticker.C:
               if err := gm.SaveState(); err != nil {
                   gm.logger.Error("Failed to save guard state", "error", err)
               }
           }
       }
   }
   ```

**Files to Create**:
- `pkg/path/persistence.go`
- `pkg/path/persistence_test.go`

**Files to Modify**:
- `pkg/path/guards.go` - Use new persistence layer
- `pkg/config/config.go` - Add persistence config

**Verification**:
```bash
go test -v ./pkg/path/... -run TestPersistence
# Test concurrent access
go test -race ./pkg/path/...
```

---

## Priority 3: Medium Severity Issues

### 3.1 🟡 Test Coverage Improvement (ROADMAP 3.1, AUDIT MED-007)

**Status**: PARTIALLY COMPLETE

**Priority**: MEDIUM  
**Effort**: 5 days

**Problem**: Several critical packages have test coverage below 70%.

| Package | Current Coverage | Target |
|---------|-----------------|--------|
| pkg/protocol | 27.6% | 70%+ |
| pkg/client | 35.1% | 70%+ |
| pkg/socks | 43.1% | 70%+ |
| pkg/circuit | 58.4% | 75%+ |
| pkg/crypto | 64.8% | 80%+ |

**Step-by-Step Resolution**:

1. **Protocol package (pkg/protocol)**:
   - Add tests for VERSIONS cell handling
   - Add tests for NETINFO cell handling
   - Add tests for error conditions
   - Add tests for timeout scenarios

2. **Client package (pkg/client)**:
   - Add integration tests with mock Tor network
   - Add tests for circuit pool management
   - Add tests for configuration loading
   - Add tests for graceful shutdown

3. **SOCKS package (pkg/socks)**:
   - Add tests for all SOCKS5 commands
   - Add tests for authentication methods
   - Add tests for connection handling
   - Add tests for rate limiting (when implemented)

4. **Circuit package (pkg/circuit)**:
   - Add tests for circuit extension
   - Add tests for relay cell encryption
   - Add tests for circuit timeout handling
   - Add tests for replay protection edge cases

5. **Crypto package (pkg/crypto)**:
   - Add tests for ntor edge cases
   - Add tests for key derivation
   - Add tests for constant-time operations
   - Add fuzzing tests for parsers

**Files to Create/Modify**:
- All `*_test.go` files in affected packages

**Verification**:
```bash
go test -cover ./pkg/protocol/...
go test -cover ./pkg/client/...
go test -cover ./pkg/socks/...
go test -cover ./pkg/circuit/...
go test -cover ./pkg/crypto/...
```

---

### 3.2 🟡 Configuration Validation Enhancement (ROADMAP 3.3)

**Status**: PARTIALLY IMPLEMENTED

**Priority**: MEDIUM  
**Effort**: 3 days

**Step-by-Step Resolution**:

1. **Add comprehensive field validators**:
   ```go
   // pkg/config/validation.go
   func (v *Validator) validatePort(port int, fieldName string) error
   func (v *Validator) validateAddress(addr string) error
   func (v *Validator) validatePath(path string, mustExist bool) error
   func (v *Validator) validateDuration(d time.Duration, min, max time.Duration) error
   ```

2. **Add validation rules with clear messages**:
   ```go
   type ValidationRule struct {
       Field      string
       Validator  func(interface{}) error
       Message    string
       Suggestion string
   }
   ```

3. **Add configuration templates**:
   - `configs/embedded.yaml` - Minimal resource profile
   - `configs/desktop.yaml` - Standard desktop usage
   - `configs/server.yaml` - High-performance server

4. **Enhance config validation CLI**:
   ```bash
   tor-config-validator --config /path/to/config --strict
   ```

**Files to Create/Modify**:
- `pkg/config/validation.go`
- `configs/templates/` (new directory)
- `cmd/tor-config-validator/main.go`

---

### 3.3 🟡 Connection Pooling (ROADMAP 3.5)

**Status**: PARTIALLY IMPLEMENTED

**Priority**: MEDIUM  
**Effort**: 4 days

**Step-by-Step Resolution**:

1. **Enhance connection pool with health checks**:
   ```go
   // pkg/pool/connection_pool.go
   // 
   // The Connection type MUST implement:
   //     func (c *Connection) Ping() bool
   // which sends a lightweight padding/keepalive cell and returns true
   // if the underlying connection is still alive and healthy, or false otherwise.
   //
   // healthCheck validates that a pooled Connection is still usable.
   func (p *ConnectionPool) healthCheck(conn *Connection) bool {
       if time.Since(conn.lastUsed) > p.maxIdleTime {
           return false
       }
       // Use Connection.Ping() to verify the connection is alive before reuse.
       // Ping() should send a PADDING cell and verify the connection responds.
       return conn.Ping()
   }
   ```

2. **Add connection reuse metrics**:
   ```go
   ConnectionReused  prometheus.Counter
   ConnectionCreated prometheus.Counter
   ConnectionClosed  prometheus.Counter
   PoolSize          prometheus.Gauge
   ```

3. **Implement lifecycle management**:
   ```go
   type ConnectionLifecycle struct {
       onCreate  func(*Connection)
       onBorrow  func(*Connection)
       onReturn  func(*Connection)
       onDestroy func(*Connection)
   }
   ```

**Files to Modify**:
- `pkg/pool/connection_pool.go`
- `pkg/metrics/metrics.go`

---

### 3.4 🟡 Distributed Tracing Integration (ROADMAP 3.6)

**Status**: PARTIALLY IMPLEMENTED

**Priority**: MEDIUM  
**Effort**: 3 days

**Step-by-Step Resolution**:

1. **Add OpenTelemetry SDK integration**:
   ```go
   // pkg/trace/otel.go
   import "go.opentelemetry.io/otel"
   
   func InitOTelTracer(serviceName string, endpoint string) (*sdktrace.TracerProvider, error)
   ```

2. **Add Jaeger exporter**:
   ```go
   // pkg/trace/exporters.go
   func NewJaegerExporter(endpoint string) (sdktrace.SpanExporter, error)
   ```

3. **Add configuration**:
   ```go
   type TracingConfig struct {
       Enabled   bool
       Endpoint  string   // Jaeger/Zipkin endpoint
       SampleRate float64 // 0.0 to 1.0
       Exporter  string   // "jaeger", "zipkin", "otlp"
   }
   ```

**Files to Create**:
- `pkg/trace/exporters.go`
- `pkg/trace/otel.go`

**Files to Modify**:
- `pkg/config/config.go`

---

## Priority 4: Low Severity Issues

### 4.1 📋 RSA-1024 Support (AUDIT LOW-001)

**Status**: DOCUMENTED (Low Priority)

**Problem**: Legacy TAP handshake uses RSA-1024 (deprecated).

**Resolution**: Document as deprecated, prefer ntor handshake. RSA-1024 support maintained for backward compatibility with older relays.

**Action**: Add deprecation warning in logs when TAP handshake is used.

---

### 4.2 📋 Deferred Resource Cleanup (AUDIT LOW-002)

**Status**: NEEDS REVIEW

**Action**: Audit all resource acquisition functions for proper defer statements.

```bash
# Find functions that acquire resources
grep -rn "Open\|Dial\|Accept\|Lock" pkg/ --include="*.go" | grep -v "_test.go"
```

---

### 4.3 📋 Goroutine Leak in acceptLoop (AUDIT LOW-003)

**Status**: NEEDS REVIEW

**Location**: `pkg/socks/socks.go:280-320`

**Action**: Ensure all goroutines check for shutdown signal and exit cleanly.

---

### 4.4 📋 Guard Fingerprinting Resistance (AUDIT LOW-004)

**Status**: DOCUMENTATION ONLY

**Problem**: Path selection patterns may be distinguishable.

**Action**: Document limitations in security considerations section.

---

### 4.5 📋 Lenient SOCKS5 Version Handling (AUDIT LOW-005)

**Status**: NEEDS REVIEW

**Location**: `pkg/socks/socks.go:250-260`

**Action**: Verify SOCKS5 version byte is strictly validated.

---

### 4.6 📋 Memory Pressure Monitoring (AUDIT LOW-007)

**Status**: NOT IMPLEMENTED

**Action**: Add optional memory pressure monitoring for embedded deployments.

---

### 4.7 📋 Crash Recovery State (AUDIT LOW-008)

**Status**: NOT IMPLEMENTED

**Action**: Implement state checkpointing for crash recovery.

---

## Codebase Security Concerns

### TODOs in Security-Critical Code

The following TODOs exist in security-relevant code paths:

| Location | Function/Context | Issue | Priority |
|----------|-----------------|-------|----------|
| `pkg/socks/socks.go` | `handleOnionConnection` | Relay data through rendezvous circuit | HIGH |
| `pkg/control/control.go` | `handleGetConf` | GETCONF returns empty values | LOW |
| `pkg/onion/service.go` | `createIntroductionPointCircuit` | Full circuit establishment | MEDIUM |
| `pkg/circuit/circuit.go` | `handleRelayData` | Deliver to correct stream via stream manager | MEDIUM |
| `pkg/onion/onion.go` | `ConnectToOnion` | Store session keys for stream encryption | HIGH |

### Math/Rand Usage Review

All `math/rand` usage has been reviewed and is acceptable for non-security-critical purposes:

| Location | Usage | Justification |
|----------|-------|---------------|
| `pkg/testing/chaos/chaos.go` | Chaos testing simulation | Non-security-critical; annotated with `//nolint:gosec` |
| `pkg/trace/sampler.go` | Trace sampling probability | Non-security-critical; sampling decisions don't affect security |
| `pkg/errors/retry.go` | Retry jitter calculation | Non-security-critical; jitter for performance only (documented in code comments) |

**Note**: Security-sensitive randomness (e.g., cryptographic keys, nonces) uses `crypto/rand` throughout the codebase. The `math/rand` usage above is intentional for performance-sensitive, non-security-critical paths.

---

## Implementation Timeline

### Week 1-2: CI/CD Security (2.1) + Error Context (2.2)

| Day | Task |
|-----|------|
| 1 | Create `.github/workflows/security.yml` |
| 2 | Add gosec and govulncheck integration |
| 3 | Add Trivy container scanning |
| 4 | Create `.github/dependabot.yml` |
| 5 | Design correlation ID propagation |
| 6 | Implement `pkg/logger/context.go` |
| 7 | Update logging call sites in pkg/circuit |
| 8 | Update logging call sites in pkg/socks |
| 9 | Update logging call sites in pkg/stream |
| 10 | Testing and verification |

### Week 3-4: Rate Limiting (2.3) + Guard Persistence (2.4)

| Day | Task |
|-----|------|
| 1 | Design rate limiter interface |
| 2 | Implement `pkg/ratelimit/limiter.go` |
| 3 | Integrate with SOCKS server |
| 4 | Add backpressure for streams |
| 5 | Design persistence layer |
| 6 | Implement atomic writes and file locking |
| 7 | Add schema versioning |
| 8 | Implement backup rotation |
| 9 | Integration testing |
| 10 | Documentation |

### Week 5-6: Test Coverage + Medium Priority Items

| Day | Task |
|-----|------|
| 1-3 | Protocol package tests |
| 4-5 | Client package tests |
| 6-7 | SOCKS package tests |
| 8 | Circuit package tests |
| 9 | Configuration validation enhancements |
| 10 | Connection pooling improvements |

---

## Verification Procedures

### Security Scanning Verification

```bash
# Run gosec locally
gosec -fmt json -out gosec-results.json ./...

# Run govulncheck
govulncheck ./...

# Run race detector
go test -race ./...

# Check for unsafe usage
grep -rn "unsafe" pkg/ --include="*.go" | grep -v "_test.go"

# Check for log.Fatal in library code
grep -rn "log.Fatal\|os.Exit" pkg/ --include="*.go" | grep -v "_test.go" | grep -v "README"
```

### Test Coverage Verification

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Check critical package coverage
go test -cover ./pkg/crypto/... | grep coverage
go test -cover ./pkg/circuit/... | grep coverage
go test -cover ./pkg/socks/... | grep coverage
```

### Integration Testing

```bash
# Run integration tests
go test -tags=integration ./pkg/testing/integration/...

# Run chaos tests
go test -tags=integration ./pkg/testing/chaos/...
```

### Production Readiness Checklist

- [ ] All Critical issues resolved
- [ ] All High priority security issues resolved
- [ ] Test coverage ≥ 75% for critical packages
- [ ] Security scanning passing (no HIGH/CRITICAL findings)
- [ ] Race detector clean
- [ ] Documentation updated
- [ ] Runbooks verified
- [ ] Monitoring and alerting operational

---

## References

- [ROADMAP.md](ROADMAP.md) - Full production readiness roadmap
- [AUDIT.md](AUDIT.md) - Security audit findings
- [docs/RUNBOOK.md](docs/RUNBOOK.md) - Operational procedures
- [docs/INCIDENT_RESPONSE.md](docs/INCIDENT_RESPONSE.md) - Incident handling
- [docs/MONITORING_GUIDE.md](docs/MONITORING_GUIDE.md) - Monitoring setup

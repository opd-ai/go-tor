# Remediation Quick Reference Guide

**Purpose**: Quick reference for developers working on security audit remediation  
**Last Updated**: October 19, 2025

---

## 🔴 Critical Priorities

### Must-Fix for Production

1. **Circuit Padding** (SPEC-001)
   - Status: NOT IMPLEMENTED
   - File: New pkg/padding/
   - Spec: padding-spec.txt
   - Effort: 3 weeks
   - **Impact**: Critical anonymity gap

2. **Input Validation** (SEC-001)
   - Status: PLANNED
   - File: pkg/cell/cell.go, pkg/cell/relay.go
   - Add: Bounds checking, enum validation
   - Effort: 1 week

3. **Descriptor Signatures** (SEC-010)
   - Status: PLANNED
   - File: pkg/onion/onion.go
   - Add: Full signature verification
   - Effort: 1-2 weeks

---

## 🟡 High Priorities

### Security Fixes

**SEC-002: Race Conditions**
- Run: `go test -race ./...`
- Review: pkg/control/events.go, pkg/circuit/manager.go
- Fix: Add proper locking

**SEC-003: Rate Limiting**
- Use: RateLimiter in pkg/security/helpers.go
- Apply to: Circuit creation, stream creation, directory requests
- Config: Add rate limit settings

**SEC-006: Memory Zeroing**
- Use: `security.SecureZeroMemory(data)`
- Apply to: All key material, sensitive buffers
- Pattern:
```go
defer security.SecureZeroMemory(sensitiveData)
```

**SEC-011: Circuit Timeouts**
- Add: Timeout enforcement in circuit manager
- Monitor: Hanging circuits
- Cleanup: Implement reaping

---

## 🟢 Specification Compliance

### SPEC-002: Bandwidth-Weighted Selection

**Files**: pkg/path/path.go, pkg/directory/directory.go

**Tasks**:
1. Parse bandwidth-weights from consensus
2. Implement weighted random selection
3. Apply to guard/middle/exit selection

**Algorithm**:
```go
// Pseudocode
weights := parseWeights(consensus)
relay := weightedRandom(candidates, weights)
```

**Spec**: dir-spec.txt Section 3.8.3

---

### SPEC-003: Family Exclusion

**Files**: pkg/path/path.go

**Tasks**:
1. Parse MyFamily declarations
2. Build family graph
3. Exclude families in path selection

**Pattern**:
```go
if inSameFamily(relay1, relay2) {
    exclude(relay2)
}
```

**Spec**: tor-spec.txt Section 5.3.4

---

## 🔧 Development Tools

### Running Tests
```bash
# All tests with race detector
go test -race ./...

# With coverage
go test -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Specific package
go test -v -race ./pkg/security/...
```

### Static Analysis
```bash
# Go vet
go vet ./...

# Staticcheck (if installed)
staticcheck ./...

# Gosec (if installed)
gosec -fmt=json -out=gosec-report.json ./...
```

### Validation
```bash
# Run comprehensive validation
bash scripts/validate-remediation.sh
```

### Building
```bash
# Standard build
make build

# Cross-compile
make build-all

# With optimization
go build -ldflags="-s -w" -o bin/tor-client ./cmd/tor-client
```

---

## 📝 Code Patterns

### Safe Integer Conversions

**Before (UNSAFE)**:
```go
length := uint16(len(data))
timestamp := uint32(time.Now().Unix())
revision := uint64(time.Now().Unix())
```

**After (SAFE)**:
```go
import "github.com/opd-ai/go-tor/pkg/security"

length, err := security.SafeLenToUint16(data)
if err != nil {
    return nil, fmt.Errorf("data too large: %w", err)
}

timestamp, err := security.SafeUnixToUint32(time.Now())
if err != nil {
    // Handle error (log and use fallback)
    timestamp = 0
}

revision, err := security.SafeUnixToUint64(time.Now())
if err != nil {
    revision = 0  // Fallback
}
```

---

### Constant-Time Comparisons

**Before (UNSAFE)**:
```go
if bytes.Equal(key1, key2) {
    // Timing attack vulnerable
}
```

**After (SAFE)**:
```go
import "github.com/opd-ai/go-tor/pkg/security"

if security.ConstantTimeCompare(key1, key2) {
    // Timing-safe comparison
}
```

---

### Memory Zeroing

**Pattern**:
```go
import "github.com/opd-ai/go-tor/pkg/security"

func handleKey(key []byte) error {
    // Ensure key is zeroed on function exit
    defer security.SecureZeroMemory(key)
    
    // Use key...
    
    return nil
}
```

---

### Rate Limiting

**Pattern**:
```go
import "github.com/opd-ai/go-tor/pkg/security"

// In struct
type Manager struct {
    circuitLimiter *security.RateLimiter
}

// Initialize
func NewManager() *Manager {
    return &Manager{
        circuitLimiter: security.NewRateLimiter(
            10,                    // 10 circuits
            time.Minute,           // per minute
        ),
    }
}

// Use
func (m *Manager) CreateCircuit() error {
    if !m.circuitLimiter.Allow() {
        return fmt.Errorf("rate limit exceeded")
    }
    // Create circuit...
}
```

---

### Input Validation

**Pattern**:
```go
func decodeCell(data []byte) (*Cell, error) {
    // Validate length
    if len(data) < MinCellSize {
        return nil, fmt.Errorf("cell too short: %d bytes", len(data))
    }
    if len(data) > MaxCellSize {
        return nil, fmt.Errorf("cell too large: %d bytes", len(data))
    }
    
    // Validate command
    cmd := Command(data[0])
    if !cmd.IsValid() {
        return nil, fmt.Errorf("invalid command: %d", cmd)
    }
    
    // Continue decoding...
}
```

---

### Error Handling with Cleanup

**Pattern**:
```go
func buildCircuit() error {
    circuit := allocateCircuit()
    
    // Ensure cleanup on error
    success := false
    defer func() {
        if !success {
            circuit.Destroy()
        }
    }()
    
    // Build circuit (may fail)
    if err := extendCircuit(circuit); err != nil {
        return err  // cleanup happens via defer
    }
    
    success = true
    return nil
}
```

---

## 📊 Testing Patterns

### Security Test Example

```go
func TestIntegerOverflowPrevention(t *testing.T) {
    tests := []struct {
        name    string
        input   int
        wantErr bool
    }{
        {"valid", 100, false},
        {"max uint16", 65535, false},
        {"overflow", 65536, true},
        {"negative", -1, true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := security.SafeIntToUint16(tt.input)
            if tt.wantErr {
                if err == nil {
                    t.Error("expected error, got nil")
                }
            } else {
                if err != nil {
                    t.Errorf("unexpected error: %v", err)
                }
                if result != uint16(tt.input) {
                    t.Errorf("got %d, want %d", result, tt.input)
                }
            }
        })
    }
}
```

---

### Race Condition Test

```go
func TestConcurrentAccess(t *testing.T) {
    manager := NewManager()
    
    // Launch multiple goroutines
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < 100; j++ {
                manager.CreateCircuit()
                manager.DestroyCircuit()
            }
        }()
    }
    
    wg.Wait()
    
    // Run with: go test -race
}
```

---

### Fuzzing Test

```go
// +build gofuzz

func Fuzz(data []byte) int {
    _, err := DecodeCell(data)
    if err != nil {
        return 0  // Invalid input
    }
    return 1  // Valid input
}

// Run: go-fuzz -bin=./cell-fuzz.zip -workdir=fuzz
```

---

## 🐛 Debugging Tips

### Finding Integer Overflows
```bash
# Use gosec
gosec -include=G115 ./...

# Look for patterns
grep -r "uint64(time.Now().Unix())" pkg/
grep -r "uint16(len(" pkg/
```

### Finding Memory Leaks
```bash
# Profile memory
go test -memprofile=mem.prof ./...
go tool pprof mem.prof

# Check for defer cleanup
grep -r "defer.*Close\|defer.*Destroy" pkg/
```

### Finding Race Conditions
```bash
# Always run with -race during development
go test -race ./...

# For specific packages
go test -race -run=TestConcurrent ./pkg/circuit/
```

### Finding Unsafe Comparisons
```bash
# Check for timing-unsafe comparisons
grep -r "bytes.Equal.*key\|==.*password" pkg/
```

---

## 📚 Specification References

### Quick Links

- **Main Protocol**: https://spec.torproject.org/tor-spec
- **Directory**: https://spec.torproject.org/dir-spec
- **Onion Services**: https://spec.torproject.org/rend-spec-v3
- **Circuit Padding**: https://spec.torproject.org/padding-spec
- **Control Protocol**: https://spec.torproject.org/control-spec

### Key Sections

**Circuit Building**:
- tor-spec.txt Section 5: Circuit creation
- tor-spec.txt Section 5.5: KDF-TOR

**Path Selection**:
- dir-spec.txt Section 3.8.3: Bandwidth weights
- tor-spec.txt Section 5.3.4: Family exclusion

**Onion Services**:
- rend-spec-v3.txt Section 4: Descriptor format
- rend-spec-v3.txt Section 5: HSDir selection

**Circuit Padding**:
- padding-spec.txt: Full specification

---

## 🎯 Coverage Targets

| Package | Current | Target | Priority |
|---------|---------|--------|----------|
| security | 95.9% | 100% | LOW |
| crypto | 88.4% | 95% | HIGH |
| circuit | 82.1% | 90% | HIGH |
| onion | 92.4% | 95% | MEDIUM |
| protocol | 10.2% | 85% | **CRITICAL** |
| client | 22.2% | 85% | **CRITICAL** |

**Focus Areas**:
1. protocol package (10% → 85%)
2. client package (22% → 85%)
3. Error paths in all packages
4. Concurrent access scenarios

---

## ✅ Review Checklist

Before submitting remediation code:

**Security**:
- [ ] All integer conversions use safe functions
- [ ] Constant-time comparisons for sensitive data
- [ ] Memory zeroing for key material
- [ ] Input validation on all parsers
- [ ] Rate limiting on resource allocation
- [ ] Proper error handling with cleanup

**Testing**:
- [ ] Unit tests added for new code
- [ ] Tests pass with `-race` flag
- [ ] Coverage increased (not decreased)
- [ ] Edge cases covered
- [ ] Error paths tested

**Documentation**:
- [ ] Code comments explain security considerations
- [ ] Spec references in comments
- [ ] godoc comments for exported functions
- [ ] README updated if needed

**Static Analysis**:
- [ ] `go vet ./...` passes
- [ ] `staticcheck ./...` passes (if available)
- [ ] No new gosec warnings
- [ ] No unchecked errors

---

## 🆘 Getting Help

**Reports**:
- Main: TOR_CLIENT_REMEDIATION_REPORT.md
- Phase 1: REMEDIATION_PHASE1_REPORT.md
- Audit: SECURITY_AUDIT_REPORT.md
- Compliance: COMPLIANCE_MATRIX_UPDATED.md

**Validation**:
```bash
bash scripts/validate-remediation.sh
```

**Questions**:
- Check specification first
- Review existing tests for patterns
- Look at pkg/security/ for utilities

---

**Last Updated**: October 19, 2025  
**Maintained By**: Security Remediation Team

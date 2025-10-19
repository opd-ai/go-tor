# Comprehensive Testing Protocol - Security Audit

**Date**: 2025-10-19  
**Project**: go-tor Security Audit  
**Purpose**: Testing methodology and validation procedures

---

## 1. Test Environment Setup

### 1.1 Development Environment

```bash
# Install required Go version
go version  # Should be 1.21+

# Install audit tools
go install honnef.co/go/tools/cmd/staticcheck@latest
go install github.com/securego/gosec/v2/cmd/gosec@latest
go install golang.org/x/vuln/cmd/govulncheck@latest

# Clone and build
git clone https://github.com/opd-ai/go-tor.git
cd go-tor
make build

# Verify build
./bin/tor-client -version
```

### 1.2 Test Data Preparation

```bash
# Create test directory
mkdir -p /tmp/tor-test/{config,data,logs}

# Generate test configuration
cat > /tmp/tor-test/config/torrc <<EOF
SocksPort 9050
ControlPort 9051
DataDirectory /tmp/tor-test/data
Log debug file /tmp/tor-test/logs/debug.log
EOF
```

---

## 2. Functional Testing

### 2.1 Basic Functionality Tests

**Test Case: TC-FUNC-001 - Application Startup**
```bash
# Test: Application starts successfully
./bin/tor-client -config /tmp/tor-test/config/torrc

# Expected: No errors, SOCKS/Control ports listening
# Validation:
netstat -an | grep LISTEN | grep -E "(9050|9051)"
```

**Test Case: TC-FUNC-002 - SOCKS5 Proxy**
```bash
# Test: SOCKS5 proxy accepts connections
curl --socks5 127.0.0.1:9050 https://check.torproject.org

# Expected: Connection through Tor network successful
# Validation: Response indicates Tor IP, not local IP
```

**Test Case: TC-FUNC-003 - Onion Service Connection**
```bash
# Test: Connect to v3 onion service
curl --socks5 127.0.0.1:9050 https://www.facebookwkhpilnemxj7asaniu7vnjjbiltxjqhye3mhbshg7kx5tfyd.onion

# Expected: Successful connection to onion service
# Validation: HTTP 200 response
```

**Test Case: TC-FUNC-004 - Control Protocol**
```bash
# Test: Control protocol commands
echo -e "GETINFO version\r\nQUIT\r\n" | nc 127.0.0.1 9051

# Expected: Version information returned
# Validation: 250 OK response
```

**Test Case: TC-FUNC-005 - Graceful Shutdown**
```bash
# Test: Application shuts down cleanly
pkill -TERM tor-client
# Wait 5 seconds
sleep 5

# Expected: No hung processes, clean shutdown logs
# Validation: No tor-client processes remain
ps aux | grep tor-client
```

### 2.2 Circuit Management Tests

**Test Case: TC-CIRC-001 - Circuit Creation**
```go
// Test in pkg/circuit/circuit_test.go
func TestCircuitCreation(t *testing.T) {
    manager := circuit.NewManager()
    c, err := manager.CreateCircuit()
    if err != nil {
        t.Fatalf("circuit creation failed: %v", err)
    }
    if c.State() != circuit.StateBuilding {
        t.Errorf("wrong initial state: %v", c.State())
    }
}
```

**Test Case: TC-CIRC-002 - Circuit Extension**
```go
func TestCircuitExtension(t *testing.T) {
    // Test 3-hop circuit extension
    c := createTestCircuit(t)
    for i := 0; i < 3; i++ {
        relay := selectTestRelay(t)
        if err := c.Extend(relay); err != nil {
            t.Fatalf("extend hop %d failed: %v", i, err)
        }
    }
}
```

### 2.3 Stream Management Tests

**Test Case: TC-STREAM-001 - Stream Multiplexing**
```go
func TestStreamMultiplexing(t *testing.T) {
    c := createEstablishedCircuit(t)
    
    // Create multiple concurrent streams
    streams := make([]*stream.Stream, 10)
    for i := 0; i < 10; i++ {
        s, err := c.CreateStream()
        if err != nil {
            t.Fatalf("stream %d creation failed: %v", i, err)
        }
        streams[i] = s
    }
    
    // Verify all streams are active
    for i, s := range streams {
        if s.State() != stream.StateConnected {
            t.Errorf("stream %d not connected", i)
        }
    }
}
```

---

## 3. Security Testing

### 3.1 Static Analysis

**Test Case: TC-SEC-001 - Go Vet**
```bash
# Run go vet on all packages
go vet ./...

# Expected: No issues
# Validation: Exit code 0
echo $?
```

**Test Case: TC-SEC-002 - Staticcheck**
```bash
# Run staticcheck
staticcheck ./...

# Expected: No issues
# Validation: No output, exit code 0
```

**Test Case: TC-SEC-003 - Security Scanner (gosec)**
```bash
# Run gosec
gosec -fmt=json -out=gosec-report.json ./...

# Expected: Known issues only (SHA1 usage for Tor spec)
# Validation: Review findings, ensure no new critical issues
jq '.Issues | length' gosec-report.json
```

**Test Case: TC-SEC-004 - Vulnerability Check**
```bash
# Check for known vulnerabilities
govulncheck ./...

# Expected: No vulnerable dependencies
# Validation: Exit code 0 or only known false positives
```

### 3.2 Race Condition Detection

**Test Case: TC-SEC-005 - Race Detector**
```bash
# Run all tests with race detector
go test -race ./... -timeout 10m

# Expected: No race conditions (or only documented test races)
# Validation: Tests pass, no "WARNING: DATA RACE" for production code
```

**Test Case: TC-SEC-006 - Concurrent Operations**
```go
func TestConcurrentCircuitCreation(t *testing.T) {
    manager := circuit.NewManager()
    
    // Create circuits concurrently
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            _, err := manager.CreateCircuit()
            if err != nil {
                t.Errorf("concurrent circuit creation failed: %v", err)
            }
        }()
    }
    wg.Wait()
}
```

### 3.3 Cryptographic Validation

**Test Case: TC-SEC-007 - Key Derivation**
```go
func TestKDFCompliance(t *testing.T) {
    // Test KDF-TOR compliance with known test vectors
    // From tor-spec.txt section 5.2.1
    secret := []byte{/* test secret */}
    derived := crypto.KDF(secret, 72)
    
    expected := []byte{/* expected from spec */}
    if !bytes.Equal(derived, expected) {
        t.Error("KDF output doesn't match specification")
    }
}
```

**Test Case: TC-SEC-008 - Constant-Time Operations**
```go
func TestConstantTimeComparison(t *testing.T) {
    // Verify constant-time comparison
    a := make([]byte, 32)
    b := make([]byte, 32)
    
    // Time equal comparison
    start := time.Now()
    for i := 0; i < 1000000; i++ {
        security.ConstantTimeCompare(a, b)
    }
    equalTime := time.Since(start)
    
    // Time unequal comparison (first byte different)
    b[0] = 1
    start = time.Now()
    for i := 0; i < 1000000; i++ {
        security.ConstantTimeCompare(a, b)
    }
    unequalTime := time.Since(start)
    
    // Times should be similar (within 10%)
    ratio := float64(unequalTime) / float64(equalTime)
    if ratio < 0.9 || ratio > 1.1 {
        t.Errorf("non-constant-time: ratio %.2f", ratio)
    }
}
```

### 3.4 Input Validation

**Test Case: TC-SEC-009 - Cell Input Validation**
```go
func TestCellInputValidation(t *testing.T) {
    tests := []struct {
        name    string
        input   []byte
        wantErr bool
    }{
        {"valid fixed cell", make([]byte, 514), false},
        {"too short", make([]byte, 4), true},
        {"too long", make([]byte, 70000), true},
        {"nil input", nil, true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := cell.Decode(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("got error %v, want error: %v", err, tt.wantErr)
            }
        })
    }
}
```

---

## 4. Performance Testing

### 4.1 Circuit Build Time

**Test Case: TC-PERF-001 - Circuit Build Latency**
```go
func TestCircuitBuildTime(t *testing.T) {
    manager := circuit.NewManager()
    
    var times []time.Duration
    for i := 0; i < 100; i++ {
        start := time.Now()
        c, err := manager.CreateCircuit()
        if err != nil {
            t.Fatalf("circuit creation failed: %v", err)
        }
        err = c.WaitForReady(context.Background())
        if err != nil {
            t.Fatalf("circuit build failed: %v", err)
        }
        times = append(times, time.Since(start))
    }
    
    // Calculate percentiles
    sort.Slice(times, func(i, j int) bool {
        return times[i] < times[j]
    })
    p50 := times[50]
    p95 := times[95]
    
    t.Logf("Circuit build times - P50: %v, P95: %v", p50, p95)
    
    // Target: P95 < 5 seconds
    if p95 > 5*time.Second {
        t.Logf("WARNING: P95 build time %v exceeds target of 5s", p95)
    }
}
```

### 4.2 Memory Profiling

**Test Case: TC-PERF-002 - Memory Leak Detection**
```bash
# Run with memory profiling
go test -memprofile=mem.prof -bench=. ./pkg/circuit

# Analyze profile
go tool pprof -alloc_space mem.prof
# In pprof: top, list, web

# Expected: No unbounded growth
# Validation: Memory usage stabilizes
```

### 4.3 Throughput Testing

**Test Case: TC-PERF-003 - Stream Throughput**
```go
func BenchmarkStreamThroughput(b *testing.B) {
    c := createEstablishedCircuit(b)
    s, _ := c.CreateStream()
    
    data := make([]byte, 1024*1024) // 1 MB
    rand.Read(data)
    
    b.SetBytes(int64(len(data)))
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        _, err := s.Write(data)
        if err != nil {
            b.Fatalf("write failed: %v", err)
        }
    }
}
```

---

## 5. Compliance Testing

### 5.1 Specification Compliance

**Test Case: TC-COMP-001 - Cell Format Compliance**
```go
func TestCellFormatCompliance(t *testing.T) {
    // Verify cell format matches tor-spec.txt section 0.2
    
    tests := []struct {
        name     string
        cmd      cell.Command
        circID   uint32
        payload  []byte
        wantSize int
    }{
        {"fixed padding", cell.PADDING, 1, make([]byte, 509), 514},
        {"fixed create", cell.CREATE2, 1, make([]byte, 509), 514},
        {"variable versions", cell.VERSIONS, 0, []byte{0, 5}, 7},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            c := cell.New(tt.cmd, tt.circID, tt.payload)
            encoded := c.Encode()
            if len(encoded) != tt.wantSize {
                t.Errorf("cell size %d, want %d", len(encoded), tt.wantSize)
            }
        })
    }
}
```

**Test Case: TC-COMP-002 - Protocol Version Negotiation**
```go
func TestProtocolVersionNegotiation(t *testing.T) {
    // Test link protocol version negotiation per tor-spec 0.1
    versions := []uint16{3, 4, 5}
    
    conn := createTestConnection(t)
    err := conn.SendVersions(versions)
    if err != nil {
        t.Fatalf("send versions failed: %v", err)
    }
    
    chosen, err := conn.ReceiveVersions()
    if err != nil {
        t.Fatalf("receive versions failed: %v", err)
    }
    
    // Should choose highest mutually supported
    if chosen < 3 || chosen > 5 {
        t.Errorf("invalid version chosen: %d", chosen)
    }
}
```

### 5.2 SOCKS5 RFC Compliance

**Test Case: TC-COMP-003 - SOCKS5 RFC 1928**
```go
func TestSOCKS5RFC1928Compliance(t *testing.T) {
    server := socks.NewServer(socks.Config{
        Addr: "127.0.0.1:0",
    })
    go server.ListenAndServe()
    defer server.Shutdown(context.Background())
    
    // Test SOCKS5 handshake per RFC 1928
    conn, err := net.Dial("tcp", server.Addr())
    if err != nil {
        t.Fatalf("dial failed: %v", err)
    }
    defer conn.Close()
    
    // Send version/methods
    _, err = conn.Write([]byte{0x05, 0x01, 0x00})
    if err != nil {
        t.Fatalf("write failed: %v", err)
    }
    
    // Read version/method selection
    response := make([]byte, 2)
    _, err = io.ReadFull(conn, response)
    if err != nil {
        t.Fatalf("read failed: %v", err)
    }
    
    if response[0] != 0x05 {
        t.Errorf("wrong version: %d", response[0])
    }
    if response[1] != 0x00 {
        t.Errorf("wrong method: %d", response[1])
    }
}
```

---

## 6. Embedded Platform Testing

### 6.1 Cross-Compilation

**Test Case: TC-EMB-001 - Cross-Platform Builds**
```bash
# Test all target platforms
for platform in linux/amd64 linux/arm linux/arm64 linux/mips; do
    echo "Building for $platform..."
    GOOS=$(echo $platform | cut -d/ -f1)
    GOARCH=$(echo $platform | cut -d/ -f2)
    GOOS=$GOOS GOARCH=$GOARCH go build -o bin/tor-client-$GOARCH ./cmd/tor-client
    
    # Check binary was created and is correct format
    file bin/tor-client-$GOARCH
    ls -lh bin/tor-client-$GOARCH
done

# Expected: All builds successful, correct architectures
```

### 6.2 Resource Constraints

**Test Case: TC-EMB-002 - Low Memory Operation**
```go
func TestLowMemoryOperation(t *testing.T) {
    // Configure for low-memory environment
    cfg := config.Config{
        MaxCircuits:          5,
        MaxStreamsPerCircuit: 10,
        CircuitMaxDirtiness:  5 * time.Minute,
    }
    
    client := client.New(cfg)
    err := client.Start()
    if err != nil {
        t.Fatalf("start failed: %v", err)
    }
    defer client.Stop()
    
    // Monitor memory usage
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    initialAlloc := m.Alloc
    t.Logf("Initial memory: %d MB", initialAlloc/1024/1024)
    
    // Create circuits up to limit
    for i := 0; i < cfg.MaxCircuits; i++ {
        _, err := client.CreateCircuit()
        if err != nil {
            t.Fatalf("circuit %d failed: %v", i, err)
        }
    }
    
    runtime.ReadMemStats(&m)
    finalAlloc := m.Alloc
    t.Logf("Final memory: %d MB", finalAlloc/1024/1024)
    
    // Should stay under 50 MB
    if finalAlloc > 50*1024*1024 {
        t.Errorf("memory usage %d MB exceeds 50 MB limit", finalAlloc/1024/1024)
    }
}
```

---

## 7. Integration Testing

### 7.1 End-to-End Scenarios

**Test Case: TC-INT-001 - Complete Connection Flow**
```bash
#!/bin/bash
# End-to-end test: Start client, make connection, verify

# Start client
./bin/tor-client -config test-config.conf &
CLIENT_PID=$!

# Wait for ready
sleep 10

# Test connection through Tor
response=$(curl --socks5 127.0.0.1:9050 -s https://check.torproject.org/api/ip)

# Verify response indicates Tor
if echo "$response" | grep -q '"IsTor":true'; then
    echo "✓ End-to-end test PASSED"
    result=0
else
    echo "✗ End-to-end test FAILED"
    result=1
fi

# Cleanup
kill $CLIENT_PID
exit $result
```

### 7.2 Stress Testing

**Test Case: TC-INT-002 - High Load Stress**
```go
func TestHighLoadStress(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping stress test in short mode")
    }
    
    client := setupTestClient(t)
    
    // Create maximum circuits
    circuits := make([]*circuit.Circuit, 20)
    for i := 0; i < 20; i++ {
        c, err := client.CreateCircuit()
        if err != nil {
            t.Fatalf("circuit %d failed: %v", i, err)
        }
        circuits[i] = c
    }
    
    // Create streams on each circuit
    var wg sync.WaitGroup
    for _, c := range circuits {
        for i := 0; i < 10; i++ {
            wg.Add(1)
            go func(circ *circuit.Circuit) {
                defer wg.Done()
                s, err := circ.CreateStream()
                if err != nil {
                    t.Errorf("stream creation failed: %v", err)
                    return
                }
                // Use stream
                time.Sleep(5 * time.Second)
                s.Close()
            }(c)
        }
    }
    
    wg.Wait()
    
    // Verify no crashes or errors
    t.Log("Stress test completed successfully")
}
```

---

## 8. Regression Testing

### 8.1 Known Issue Verification

**Test Case: TC-REG-001 - FINDING H-001 Fixed**
```go
func TestFindingH001_RaceConditionFixed(t *testing.T) {
    // Verify race condition in SOCKS shutdown is fixed
    // Run with: go test -race
    
    server := socks.NewServer(socks.Config{
        Addr: "127.0.0.1:0",
    })
    
    done := make(chan struct{})
    go func() {
        server.ListenAndServe()
        close(done)
    }()
    
    time.Sleep(100 * time.Millisecond)
    
    // Capture address before shutdown (should not race)
    addr := server.Addr()
    t.Logf("Server address: %s", addr)
    
    // Shutdown
    server.Shutdown(context.Background())
    <-done
    
    // Test passes if no race condition detected
}
```

**Test Case: TC-REG-002 - FINDING H-002 Fixed**
```go
func TestFindingH002_IntegerOverflowFixed(t *testing.T) {
    // Verify integer overflow in timestamp conversion is fixed
    
    tests := []struct {
        name      string
        timestamp int64
        wantError bool
    }{
        {"negative", -1, true},
        {"zero", 0, false},
        {"max int64", math.MaxInt64, false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := security.SafeInt64ToUint64(tt.timestamp)
            if (err != nil) != tt.wantError {
                t.Errorf("got error %v, want error: %v", err, tt.wantError)
            }
        })
    }
}
```

---

## 9. Test Execution Plan

### 9.1 Continuous Integration

```yaml
# .github/workflows/tests.yml
name: Comprehensive Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with:
          go-version: '1.21'
      
      - name: Static Analysis
        run: |
          go vet ./...
          go install honnef.co/go/tools/cmd/staticcheck@latest
          staticcheck ./...
      
      - name: Security Scan
        run: |
          go install github.com/securego/gosec/v2/cmd/gosec@latest
          gosec ./...
      
      - name: Unit Tests
        run: go test ./... -v -cover
      
      - name: Race Detection
        run: go test ./... -race -timeout 10m
      
      - name: Benchmarks
        run: go test ./... -bench=. -benchmem
```

### 9.2 Pre-Release Checklist

```markdown
## Pre-Release Testing Checklist

### Static Analysis
- [ ] go vet passes
- [ ] staticcheck passes
- [ ] gosec findings reviewed
- [ ] govulncheck clean

### Unit Tests
- [ ] All tests pass
- [ ] Coverage >70% overall
- [ ] Critical packages >90%

### Security Tests
- [ ] Race detector clean
- [ ] Security test suite passes
- [ ] Cryptographic tests pass
- [ ] Input validation tests pass

### Integration Tests
- [ ] End-to-end tests pass
- [ ] Stress tests pass
- [ ] SOCKS5 compliance tests pass
- [ ] Control protocol tests pass

### Performance Tests
- [ ] Circuit build time acceptable
- [ ] Memory usage within limits
- [ ] No memory leaks detected
- [ ] CPU usage reasonable

### Platform Tests
- [ ] Cross-compilation succeeds
- [ ] Binary sizes acceptable
- [ ] All architectures tested

### Regression Tests
- [ ] Known issues verified fixed
- [ ] No new regressions
- [ ] Backward compatibility maintained
```

---

## 10. Test Metrics and Reporting

### 10.1 Coverage Requirements

```
Target Coverage by Package:
- pkg/security:   >95%  (Critical security code)
- pkg/crypto:     >90%  (Cryptographic operations)
- pkg/cell:       >85%  (Protocol core)
- pkg/circuit:    >85%  (Circuit management)
- pkg/socks:      >80%  (External interface)
- Overall:        >75%  (Project-wide)
```

### 10.2 Test Execution Time

```
Fast Tests (<1s):        Unit tests, static analysis
Medium Tests (1-10s):    Integration tests, simple scenarios
Slow Tests (>10s):       Stress tests, full end-to-end
Very Slow (>1m):         Fuzzing, extended stress tests

CI Pipeline:             Fast + Medium (~5 minutes)
Pre-merge:               Fast + Medium + Slow (~15 minutes)
Nightly:                 All tests including fuzzing (~60 minutes)
```

### 10.3 Success Criteria

```
✓ All unit tests passing
✓ Static analysis clean
✓ Security tests passing
✓ No critical race conditions
✓ Coverage targets met
✓ Performance within targets
✓ All platforms building
✓ Integration tests passing
✓ Known issues fixed
✓ No new regressions
```

---

## Conclusion

This comprehensive testing protocol ensures thorough validation of the go-tor implementation across:
- ✅ Functionality (SOCKS5, circuits, streams, control protocol)
- ✅ Security (static analysis, race detection, crypto validation)
- ✅ Performance (latency, throughput, resource usage)
- ✅ Compliance (Tor specs, RFC 1928, protocol standards)
- ✅ Embedded suitability (cross-compilation, resource constraints)
- ✅ Regression prevention (known issue verification)

**Recommended Testing Cycle**:
1. **Every Commit**: Fast tests (5 min)
2. **Pre-Merge**: Fast + Medium + Slow (15 min)
3. **Daily**: Full suite including stress tests (60 min)
4. **Pre-Release**: Complete validation + manual testing (4 hours)

---

**Document Version**: 1.0  
**Last Updated**: 2025-10-19  
**Maintainer**: Security Audit Team

# Testing Guide for go-tor

This document provides comprehensive guidance on testing the go-tor Tor client implementation.

## Table of Contents

- [Overview](#overview)
- [Test Organization](#test-organization)
- [Running Tests](#running-tests)
- [Test Coverage](#test-coverage)
- [Integration Tests](#integration-tests)
- [Stress Tests](#stress-tests)
- [Benchmark Tests](#benchmark-tests)
- [Writing New Tests](#writing-new-tests)
- [Best Practices](#best-practices)

## Overview

The go-tor project maintains comprehensive test coverage across all packages. As of Phase 9.3, the project includes:

- **Unit Tests**: Testing individual functions and methods in isolation
- **Integration Tests**: Testing component interactions (e.g., HTTP metrics endpoint)
- **Stress Tests**: Testing behavior under concurrent load
- **Benchmark Tests**: Measuring performance characteristics

### Current Test Statistics

- **Overall Coverage**: ~74%
- **Critical Packages**: 90%+ coverage
  - config: 90.1%
  - control: 91.6%
  - errors: 100%
  - health: 96.5%
  - logger: 100%
  - metrics: 100%
  - security: 95.8%

## Test Organization

### Test File Naming

Tests follow Go conventions:

```
pkg/example/
├── example.go              # Production code
├── example_test.go         # Unit tests
├── example_integration_test.go  # Integration tests
└── stress_test.go          # Stress and benchmark tests
```

### Test Function Naming

- **Unit Tests**: `TestFunctionName`
- **Integration Tests**: `TestComponentIntegration`
- **Stress Tests**: `TestScenarioStress`
- **Benchmarks**: `BenchmarkOperation`

## Running Tests

### Basic Test Commands

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run specific package tests
go test ./pkg/client

# Run with race detector (recommended)
go test -race ./...

# Run with verbose output
go test -v ./...

# Run short tests only (skip slow integration/stress tests)
go test -short ./...
```

### Advanced Test Commands

```bash
# Run tests with timeout
go test -timeout 5m ./...

# Run specific test function
go test ./pkg/client -run TestNew

# Run tests matching pattern
go test ./... -run ".*Integration.*"

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Run benchmarks
go test -bench=. ./...

# Run benchmarks with memory stats
go test -bench=. -benchmem ./...
```

## Test Coverage

### Viewing Coverage

```bash
# Generate and view coverage report
make test-coverage

# View coverage for specific package
go test -cover ./pkg/client

# Detailed coverage by function
go test -coverprofile=coverage.out ./pkg/client
go tool cover -func=coverage.out
```

### Coverage by Package

| Package | Coverage | Notes |
|---------|----------|-------|
| autoconfig | 61.7% | Auto-configuration utilities |
| cell | 76.1% | Cell encoding/decoding |
| circuit | 79.2% | Circuit management |
| client | 33.0% | Orchestration layer (improving) |
| config | 90.1% | Configuration management |
| connection | 61.5% | TLS connections |
| control | 91.6% | Control protocol |
| crypto | 65.3% | Cryptographic operations |
| directory | 72.5% | Directory protocol |
| errors | 100% | Error types |
| health | 96.5% | Health monitoring |
| httpmetrics | 88.2% | HTTP metrics server |
| logger | 100% | Logging infrastructure |
| metrics | 100% | Metrics collection |
| onion | 77.9% | Onion service support |
| path | 64.8% | Path selection |
| pool | 67.8% | Resource pooling |
| protocol | 27.6% | Protocol handshake (needs improvement) |
| security | 95.8% | Security utilities |
| socks | 74.7% | SOCKS5 proxy |
| stream | 86.7% | Stream handling |

### Coverage Goals

- **Critical Packages**: 90%+ (security, errors, config, control)
- **Core Packages**: 70%+ (circuit, cell, crypto, directory)
- **Integration Packages**: 60%+ (client, connection, socks)

## Integration Tests

### HTTP Metrics Integration Tests

Location: `pkg/client/metrics_integration_test.go`

Tests the HTTP metrics endpoint integration:

```go
// Test metrics endpoint availability
TestMetricsServerIntegration
TestMetricsServerDisabled

// Test endpoint responses
TestMetricsEndpointJSON
TestMetricsEndpointPrometheus
TestMetricsEndpointHealth

// Test lifecycle
TestMetricsServerLifecycle
TestMetricsWithClientStart

// Test metrics recording
TestMetricsRecording
```

### Running Integration Tests

```bash
# Run all integration tests
go test ./pkg/client -run ".*Integration.*"

# Run specific integration test
go test ./pkg/client -run TestMetricsServerIntegration

# Run with verbose output
go test -v ./pkg/client -run ".*Integration.*"
```

## Stress Tests

### Overview

Stress tests validate behavior under concurrent load and race conditions.

Location: `pkg/client/stress_test.go`

### Available Stress Tests

1. **TestConcurrentBandwidthRecording**
   - Tests concurrent bandwidth recording
   - 100 goroutines × 1000 iterations
   - Validates thread-safety

2. **TestMultipleStartStop**
   - Tests rapid start/stop cycles
   - Validates shutdown idempotency

3. **TestStatsUnderLoad**
   - Tests GetStats under concurrent access
   - 50 goroutines for 2 seconds
   - Validates read safety

4. **TestClientLifecycleStress**
   - Tests rapid client creation/destruction
   - 10 clients created and destroyed
   - Validates resource cleanup

5. **TestContextCancellationRace**
   - Tests context cancellation races
   - Validates graceful cancellation

6. **TestMetricsUnderLoad**
   - Tests metrics recording under load
   - 100 goroutines × 500 iterations
   - Validates metric consistency

### Running Stress Tests

```bash
# Run all stress tests (skip with -short)
go test ./pkg/client -run ".*Stress.*"

# Run with race detector (recommended)
go test -race ./pkg/client -run ".*Stress.*"

# Run all tests including stress tests
go test ./pkg/client

# Skip stress tests (faster)
go test -short ./pkg/client
```

### Interpreting Results

Stress tests are designed to:
- Detect race conditions (use `-race` flag)
- Validate concurrent access patterns
- Ensure no deadlocks occur
- Verify graceful shutdown

**Success Criteria:**
- All tests pass without panics
- No race detector warnings
- Reasonable execution time (<30s per test)

## Benchmark Tests

### Available Benchmarks

Location: `pkg/client/stress_test.go`

1. **BenchmarkBandwidthRecording**
   - Measures bandwidth recording performance
   - Target: <100ns per operation

2. **BenchmarkGetStats**
   - Measures GetStats performance
   - Target: <1µs per operation

3. **BenchmarkClientCreation**
   - Measures client creation overhead
   - Target: <100ms per client

### Running Benchmarks

```bash
# Run all benchmarks
go test -bench=. ./...

# Run with memory statistics
go test -bench=. -benchmem ./...

# Run specific benchmark
go test -bench=BenchmarkBandwidthRecording ./pkg/client

# Compare benchmarks (save baseline first)
go test -bench=. ./pkg/client > old.txt
# Make changes...
go test -bench=. ./pkg/client > new.txt
benchcmp old.txt new.txt
```

### Sample Benchmark Output

```
BenchmarkBandwidthRecording-8    50000000    25.3 ns/op    0 B/op    0 allocs/op
BenchmarkGetStats-8               5000000   302 ns/op     0 B/op    0 allocs/op
BenchmarkClientCreation-8              50  35.2 ms/op  12345 B/op  234 allocs/op
```

## Writing New Tests

### Unit Test Template

```go
func TestFunctionName(t *testing.T) {
    // Setup
    cfg := config.DefaultConfig()
    cfg.DataDirectory = t.TempDir() // Use temp dir for tests
    
    // Execute
    result, err := SomeFunction(cfg)
    
    // Assert
    if err != nil {
        t.Fatalf("Expected no error, got: %v", err)
    }
    if result != expected {
        t.Errorf("Expected %v, got %v", expected, result)
    }
}
```

### Table-Driven Test Template

```go
func TestFunctionVariants(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected int
        wantErr  bool
    }{
        {"valid_input", "test", 42, false},
        {"empty_input", "", 0, true},
        {"invalid_input", "bad", 0, true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := Process(tt.input)
            
            if tt.wantErr && err == nil {
                t.Error("Expected error, got nil")
            }
            if !tt.wantErr && err != nil {
                t.Errorf("Unexpected error: %v", err)
            }
            if result != tt.expected {
                t.Errorf("Expected %v, got %v", tt.expected, result)
            }
        })
    }
}
```

### Integration Test Template

```go
func TestComponentIntegration(t *testing.T) {
    // Create components
    cfg := config.DefaultConfig()
    cfg.DataDirectory = t.TempDir()
    client, err := New(cfg, logger.NewDefault())
    if err != nil {
        t.Fatalf("Setup failed: %v", err)
    }
    defer client.Stop()
    
    // Test integration
    // ... component interactions ...
    
    // Verify results
    if !expectedCondition {
        t.Error("Integration test failed")
    }
}
```

### Stress Test Template

```go
func TestOperationStress(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping stress test in short mode")
    }
    
    // Setup
    const numGoroutines = 100
    var wg sync.WaitGroup
    wg.Add(numGoroutines)
    
    // Execute concurrently
    for i := 0; i < numGoroutines; i++ {
        go func() {
            defer wg.Done()
            // ... test operations ...
        }()
    }
    
    // Wait and verify
    wg.Wait()
    t.Log("Stress test completed successfully")
}
```

### Benchmark Template

```go
func BenchmarkOperation(b *testing.B) {
    // Setup (not timed)
    cfg := config.DefaultConfig()
    cfg.DataDirectory = b.TempDir()
    
    b.ResetTimer() // Start timing
    
    for i := 0; i < b.N; i++ {
        // Operation to benchmark
        _ = ExpensiveOperation()
    }
}
```

## Best Practices

### General Guidelines

1. **Use Temp Directories**: Always use `t.TempDir()` for test data
2. **Clean Up Resources**: Use `defer` for cleanup or explicit cleanup at end
3. **Avoid Hardcoded Ports**: Use auto-assigned ports or high port numbers
4. **Test Error Paths**: Don't just test happy paths
5. **Use Table-Driven Tests**: For testing multiple scenarios
6. **Run with Race Detector**: Use `-race` flag regularly

### Test Independence

```go
// Good: Independent test
func TestOperation(t *testing.T) {
    cfg := config.DefaultConfig()
    cfg.DataDirectory = t.TempDir() // Isolated
    // ... test code ...
}

// Bad: Shared state
var sharedClient *Client // DON'T DO THIS

func TestOperation(t *testing.T) {
    // Tests using sharedClient will interfere with each other
}
```

### Timeout Handling

```go
func TestWithTimeout(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    // Operation with timeout
    err := LongOperation(ctx)
    if err == context.DeadlineExceeded {
        t.Error("Operation timed out")
    }
}
```

### Parallel Tests

```go
func TestParallel(t *testing.T) {
    t.Parallel() // Mark as safe to run in parallel
    
    // Test code...
}
```

### Skip Conditions

```go
func TestExpensive(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping expensive test in short mode")
    }
    // ... expensive test ...
}
```

### Assertion Helpers

```go
// Helper for common assertions
func assertEqual(t *testing.T, got, want interface{}) {
    t.Helper() // Mark as helper for better error messages
    if got != want {
        t.Errorf("Got %v, want %v", got, want)
    }
}

func TestWithHelper(t *testing.T) {
    assertEqual(t, result, expected)
}
```

## Continuous Integration

### Pre-commit Checks

```bash
# Run before committing
make fmt      # Format code
make vet      # Run go vet
make test     # Run tests
```

### CI Pipeline

Tests run automatically on:
- Pull requests
- Commits to main branch
- Nightly builds

Pipeline runs:
1. `go fmt` check
2. `go vet`
3. `go test -race ./...` (with race detector)
4. `go test -cover ./...` (with coverage reporting)

## Troubleshooting

### Common Issues

**Issue**: Tests fail with port conflicts
```bash
# Solution: Use higher port numbers or auto-assign
cfg.SocksPort = 19000 + rand.Intn(1000)
```

**Issue**: Race conditions detected
```bash
# Solution: Protect shared state with mutexes
var mu sync.Mutex
mu.Lock()
sharedState = newValue
mu.Unlock()
```

**Issue**: Tests hang indefinitely
```bash
# Solution: Add timeouts
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
```

**Issue**: Flaky tests
```go
// Solution: Use proper synchronization instead of sleep
// Bad: time.Sleep(100 * time.Millisecond)

// Good: Use channels for synchronization
// chan struct{} is idiomatic for signaling (zero memory overhead)
ready := make(chan struct{})
go func() {
    // Setup...
    close(ready) // Signal ready
}()
<-ready // Wait for ready

// Good: Use sync.WaitGroup
var wg sync.WaitGroup
wg.Add(1)
go func() {
    defer wg.Done()
    // Work...
}()
wg.Wait()

// Good: Use context with timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
```

## Contributing

### Adding New Tests

1. Identify untested code paths
2. Write comprehensive tests
3. Verify coverage increases
4. Run with race detector
5. Submit PR with test results

### Test Coverage Goals

When adding features:
- Add unit tests for new functions
- Add integration tests for new components
- Maintain or improve overall coverage
- Document complex test scenarios

## References

- [Go Testing Documentation](https://golang.org/pkg/testing/)
- [Go Test Coverage](https://go.dev/blog/cover)
- [Table Driven Tests](https://dave.cheney.net/2013/06/09/writing-table-driven-tests-in-go)
- [go-tor Architecture](ARCHITECTURE.md)
- [Development Guide](DEVELOPMENT.md)

## Support

For testing questions or issues:
- Check [Troubleshooting Guide](TROUBLESHOOTING.md)
- Open an issue on GitHub
- Consult existing test files for examples

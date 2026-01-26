# Compatibility Testing Documentation

This document describes the compatibility test suite for go-tor, which validates interoperability with the reference Tor implementation (C implementation) and official pluggable transport binaries.

## Overview

The compatibility tests verify that go-tor correctly implements the Tor protocol specifications by testing against:

1. **Reference Tor Implementation** - The official C implementation of Tor
2. **Official Tor Client** - Testing go-tor relay with official Tor as client
3. **Official Pluggable Transports** - Testing with obfs4proxy binary

## Prerequisites

To run compatibility tests, you need the following installed:

```bash
# Install Tor (reference implementation)
# Debian/Ubuntu
sudo apt-get install tor

# macOS
brew install tor

# Verify installation
tor --version
```

Optional (for PT tests):
```bash
# Install obfs4proxy
# From source
go install gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/cmd/obfs4proxy@latest

# Verify installation
obfs4proxy --version
```

## Running Tests

### Run All Compatibility Tests

```bash
go test -v ./pkg/testing/integration/... -run Compatibility -timeout 5m
```

### Run Specific Test Suites

**Test against reference Tor implementation:**
```bash
go test -v ./pkg/testing/integration/... \
  -run TestCompatibilityWithReferenceTor \
  -timeout 3m
```

**Test with Tor client connecting to go-tor relay:**
```bash
go test -v ./pkg/testing/integration/... \
  -run TestCompatibilityWithTorClient \
  -timeout 3m
```

**Test with official obfs4proxy:**
```bash
go test -v ./pkg/testing/integration/... \
  -run TestCompatibilityWithObfs4proxy \
  -timeout 3m
```

**Test HTTP over Tor (end-to-end):**
```bash
go test -v ./pkg/testing/integration/... \
  -run TestCompatibilityHTTPOverTor \
  -timeout 3m
```

### Skip Tests Without Prerequisites

If `tor` or `obfs4proxy` binaries are not available, the tests will automatically skip with an informative message:

```
--- SKIP: TestCompatibilityWithReferenceTor (0.00s)
    compatibility_test.go:58: tor binary not found in PATH, skipping compatibility test
```

## Test Descriptions

### TestCompatibilityWithReferenceTor

**Purpose**: Validate that go-tor client can interoperate with the reference Tor implementation.

**What it tests**:
1. **SOCKS5 Protocol Compatibility**
   - Starts a local Tor instance with SOCKS proxy
   - Connects go-tor client through Tor SOCKS5 proxy
   - Validates SOCKS5 handshake completion

2. **OR Protocol Handshake**
   - Connects to Tor's ORPort using TLS
   - Exchanges VERSIONS cells with reference Tor relay
   - Validates protocol version negotiation (versions 3-5)

**Expected Behavior**:
- Tor bootstraps successfully (may take 30-60 seconds)
- SOCKS5 handshake completes with version 5, no-auth method
- VERSIONS cell exchange succeeds
- Protocol version is negotiated correctly

**Example Output**:
```
=== RUN   TestCompatibilityWithReferenceTor/ConnectThroughTorSOCKS
    Successfully completed SOCKS5 handshake with reference Tor
--- PASS: TestCompatibilityWithReferenceTor/ConnectThroughTorSOCKS (2.34s)
=== RUN   TestCompatibilityWithReferenceTor/ORProtocolHandshake
    Successfully exchanged VERSIONS with reference Tor (negotiated versions: [3 4 5])
--- PASS: TestCompatibilityWithReferenceTor/ORProtocolHandshake (0.05s)
```

### TestCompatibilityWithTorClient

**Purpose**: Validate that official Tor clients can connect to go-tor relay implementation.

**What it tests**:
1. **Server-Side OR Protocol**
   - Starts go-tor OR listener with generated relay keys
   - Configures Tor client to use go-tor relay as bridge
   - Monitors connection establishment

2. **Link Protocol Server-Side**
   - Tor client initiates TLS connection to go-tor relay
   - go-tor relay performs server-side link handshake
   - Connection statistics verified

**Expected Behavior**:
- go-tor OR listener starts successfully
- Tor client connects to go-tor relay
- Link protocol handshake completes
- Connection appears in relay statistics

**Example Output**:
```
=== RUN   TestCompatibilityWithTorClient
    Tor client successfully connected to go-tor relay
    Relay stats: Total=1, Active=1
--- PASS: TestCompatibilityWithTorClient (15.23s)
```

### TestCompatibilityWithObfs4proxy

**Purpose**: Validate that go-tor PT implementation works with official obfs4proxy binary.

**What it tests**:
1. **obfs4 Client to Server**
   - Starts obfs4proxy in server mode
   - Creates go-tor obfs4 client
   - Attempts connection through obfs4 transport

2. **obfs4 Server to Client**
   - Starts go-tor obfs4 server
   - Creates obfs4proxy in client mode
   - Validates certificate exchange and PT protocol

**Expected Behavior**:
- obfs4proxy server starts and provides certificate
- go-tor obfs4 client initializes with certificate
- Transport protocol handshake succeeds
- Certificate validation passes

**Example Output**:
```
=== RUN   TestCompatibilityWithObfs4proxy/Obfs4ClientToServer
    Got obfs4 certificate: AIq0qLB4TQdgJg...
    Successfully established obfs4 connection to reference obfs4proxy
--- PASS: TestCompatibilityWithObfs4proxy/Obfs4ClientToServer (3.12s)
=== RUN   TestCompatibilityWithObfs4proxy/Obfs4ServerToClient
    Go-tor obfs4 server certificate: AIq0qLB4TQdgJg...
    Successfully configured obfs4proxy client to connect to go-tor obfs4 server
--- PASS: TestCompatibilityWithObfs4proxy/Obfs4ServerToClient (2.87s)
```

### TestCompatibilityHTTPOverTor

**Purpose**: End-to-end integration test demonstrating HTTP requests through Tor network.

**What it tests**:
1. **Complete Circuit Building**
   - Starts local HTTP test server
   - Starts Tor with exit policy allowing localhost
   - Validates SOCKS5 connection to test server

2. **End-to-End Connectivity**
   - go-tor client connects through Tor SOCKS proxy
   - SOCKS5 handshake completes
   - Validates full protocol stack

**Expected Behavior**:
- Tor fully bootstraps (up to 60 seconds)
- Test HTTP server starts successfully
- SOCKS5 connection established
- End-to-end connectivity validated

## Test Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                  Compatibility Test Suite                     │
├──────────────────────────────────────────────────────────────┤
│                                                                │
│  ┌────────────────┐         ┌────────────────┐               │
│  │  Reference Tor │◄────────┤  go-tor Client │               │
│  │   (C impl)     │  SOCKS5 │                │               │
│  └────────────────┘         └────────────────┘               │
│         │                                                      │
│         │ OR Protocol                                         │
│         ▼                                                      │
│  ┌────────────────┐         ┌────────────────┐               │
│  │  Reference Tor │────────►│  go-tor Relay  │               │
│  │   Client       │   TLS   │   (OR Server)  │               │
│  └────────────────┘         └────────────────┘               │
│                                                                │
│  ┌────────────────┐         ┌────────────────┐               │
│  │  obfs4proxy    │◄───────►│  go-tor obfs4  │               │
│  │  (official)    │  PT IPC │   Client/Srv   │               │
│  └────────────────┘         └────────────────┘               │
│                                                                │
└──────────────────────────────────────────────────────────────┘
```

## Troubleshooting

### Tor Bootstrap Timeout

If tests timeout during Tor bootstrap:

1. **Check network connectivity** - Tor needs to connect to directory authorities
2. **Increase timeout** - Use `-timeout 5m` for slower networks
3. **Check firewall** - Ensure outbound connections are allowed
4. **Use local Tor** - Set `UseBridges 0` if connecting to public network

### obfs4proxy Not Found

If obfs4proxy tests skip:

```bash
# Install obfs4proxy
go install gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/cmd/obfs4proxy@latest

# Add to PATH
export PATH=$PATH:$(go env GOPATH)/bin

# Verify
which obfs4proxy
```

### TLS Certificate Errors

If TLS handshake fails:

- Tests use `InsecureSkipVerify: true` for test relays
- This is intentional for compatibility testing
- Production code should validate certificates properly

### Port Conflicts

Tests use random free ports via `findFreePort()` helper, but if conflicts occur:

- Stop other local Tor instances
- Close applications using ports 9001-9200
- Tests will retry with different ports automatically

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Compatibility Tests

on: [push, pull_request]

jobs:
  compatibility:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.24'
      
      - name: Install Tor
        run: |
          sudo apt-get update
          sudo apt-get install -y tor
      
      - name: Install obfs4proxy
        run: |
          go install gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/cmd/obfs4proxy@latest
          echo "$(go env GOPATH)/bin" >> $GITHUB_PATH
      
      - name: Run Compatibility Tests
        run: |
          go test -v ./pkg/testing/integration/... \
            -run Compatibility \
            -timeout 5m
```

### GitLab CI Example

```yaml
compatibility_tests:
  image: golang:1.24
  before_script:
    - apt-get update && apt-get install -y tor
    - go install gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/cmd/obfs4proxy@latest
  script:
    - go test -v ./pkg/testing/integration/... -run Compatibility -timeout 5m
  allow_failure: false
```

## Best Practices

1. **Run regularly** - Compatibility tests should run on every PR and commit to main
2. **Monitor timeouts** - Network conditions affect Tor bootstrap time
3. **Version tracking** - Log Tor and obfs4proxy versions in test output
4. **Isolation** - Tests use temporary directories for all state
5. **Cleanup** - All processes are properly terminated via defer statements

## Performance Expectations

| Test | Typical Duration | Timeout |
|------|-----------------|---------|
| ORProtocolHandshake | 0.1-0.5s | 30s |
| ConnectThroughTorSOCKS | 30-60s | 2m |
| TorClientConnection | 10-20s | 3m |
| Obfs4ClientToServer | 2-5s | 1m |
| Obfs4ServerToClient | 2-5s | 1m |
| HTTPOverTor | 60-90s | 3m |

## Future Enhancements

Potential additions to the compatibility test suite:

- [ ] Test with multiple Tor versions (0.4.7.x, 0.4.8.x, 0.4.9.x)
- [ ] Test other PT implementations (snowflake, meek)
- [ ] Load testing with concurrent connections
- [ ] Protocol fuzzing for edge cases
- [ ] Bandwidth measurement validation
- [ ] Directory protocol compatibility

## References

- [Tor Specification](https://spec.torproject.org/tor-spec)
- [Pluggable Transport Specification](https://spec.torproject.org/pt-spec)
- [obfs4 Specification](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/obfs4/-/blob/HEAD/doc/obfs4-spec.txt)
- [Tor Project Testing](https://gitlab.torproject.org/tpo/core/tor/-/wikis/doc/HACKING/WritingTests)

## Reporting Issues

If compatibility tests fail:

1. **Capture full test output** - Use `-v` flag for verbose logs
2. **Include versions** - Report Tor, obfs4proxy, and Go versions
3. **Provide system info** - OS, network setup, firewall configuration
4. **Check known issues** - Review GitHub issues for similar problems
5. **Create minimal reproduction** - Isolate to specific test case

---

**Last Updated**: January 2026  
**Maintainer**: go-tor team  
**Status**: Active - Compatibility tests run on all PRs

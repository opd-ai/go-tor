# Circuit Integration Tests

This directory contains integration tests that validate circuit building functionality against the actual Tor network.

## Overview

The integration tests in `circuit_relay_integration_test.go` verify:

1. **Circuit Building with Real Relays** - End-to-end circuit construction using live Tor network relays from consensus
2. **CREATE2/CREATED2 Handshake** - First-hop cryptographic handshake validation with real guard relays  
3. **Flow Control Infrastructure** - Verification of circuit-level flow control window initialization

## Running Integration Tests

Integration tests are disabled by default (require `-tags=integration` build tag) to prevent accidental network connections during normal testing.

### Run All Integration Tests

```bash
go test -tags=integration -v -timeout=5m ./pkg/circuit -run TestIntegration
```

### Run Individual Tests

```bash
# Test full circuit building
go test -tags=integration -v -timeout=5m ./pkg/circuit -run TestIntegrationCircuitBuildingWithRealRelays

# Test first-hop handshake only
go test -tags=integration -v -timeout=5m ./pkg/circuit -run TestIntegrationFirstHopHandshake

# Test flow control initialization
go test -tags=integration -v -timeout=5m ./pkg/circuit -run TestIntegrationFlowControlWithRealCircuit
```

## Requirements

- **Network Access**: Tests connect to real Tor directory authorities and relay nodes
- **Time**: Full test suite takes 2-5 minutes depending on network conditions
- **Consensus Availability**: Tests fetch current consensus from directory authorities

## Test Coverage

### TestIntegrationCircuitBuildingWithRealRelays

- Fetches real Tor consensus from directory authorities
- Selects a 3-hop path (guard, middle, exit) using path selection algorithms
- Builds circuit using `Builder.BuildCircuit()`
- Validates:
  - Circuit state transitions (Building → Open)
  - First hop has full cryptographic state (CREATE2 handshake)
  - Circuit has 3 hop structures
  - Connection is established

**Note**: Currently only the first hop uses real CREATE2/CREATED2 handshake. Middle and exit hops are simulated pending full EXTEND2 implementation in `builder.go`.

### TestIntegrationFirstHopHandshake

- Fetches consensus and selects a guard relay with valid ntor keys
- Establishes TLS connection to guard relay
- Performs CREATE2/CREATED2 handshake with ntor key exchange
- Validates:
  - Cryptographic state derivation (forward/backward ciphers and digests)
  - Hop addition to circuit
  - Key material extraction from relay descriptor

This test validates the core CREATE2 protocol implementation against real Tor relays.

### TestIntegrationFlowControlWithRealCircuit

- Builds a circuit using real consensus and path selection
- Validates flow control infrastructure initialization
- Verifies circuit reaches Open state

This test confirms flow control windows are properly initialized per tor-spec.txt §7.4.

## Current Limitations

1. **EXTEND2 Not Fully Integrated**: Builder currently only performs CREATE2 for first hop. Middle and exit hops are simulated (see AUDIT.md section 3).
2. **No Multi-Hop Extension Tests**: Integration tests for EXTEND2/EXTENDED2 would require full multi-hop implementation in builder.
3. **No Data Relay Tests**: Tests validate circuit building but don't send RELAY_DATA cells.

## Future Enhancements

As documented in AUDIT.md, future integration test additions include:

- Multi-hop EXTEND2 validation when builder integrates full extension
- RELAY_DATA send/receive through established circuits
- Stream creation and management over circuits
- High-throughput flow control validation
- Circuit failure and recovery scenarios

## Troubleshooting

### Tests Timeout

- Increase timeout: `-timeout=10m`
- Network latency may affect consensus fetch or relay connections
- Some relays may be slow or unreachable

### Connection Failures

- Directory authorities may be temporarily unavailable
- Selected guard relays may reject connections
- Tests will fail if consensus cannot be fetched

### Build Errors

- Ensure `-tags=integration` flag is present
- Verify all dependencies are up to date: `go mod download`

## Implementation Reference

See AUDIT.md section 3 ("Circuit Creation and Extension") for detailed specification compliance and progress tracking.

**Related Files:**
- `pkg/circuit/builder.go` - Circuit building logic
- `pkg/circuit/extension.go` - CREATE2/EXTEND2 implementation
- `pkg/directory/directory.go` - Consensus fetching
- `pkg/path/path.go` - Path selection algorithms

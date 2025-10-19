# Introduction Point Protocol Demo

This example demonstrates the **Phase 7.3.3 Introduction Point Protocol** implementation for v3 onion services in go-tor.

## Overview

The introduction point protocol is a critical component of Tor's onion service (hidden service) architecture. It allows clients to initiate connections to onion services through introduction points listed in the service's descriptor.

## What This Demo Shows

1. **Descriptor with Introduction Points** - Creating a descriptor containing multiple introduction points
2. **Introduction Point Selection** - Selecting an appropriate introduction point from the descriptor
3. **INTRODUCE1 Cell Construction** - Building the INTRODUCE1 cell per Tor specification (rend-spec-v3.txt)
4. **Introduction Circuit Creation** - Creating a circuit to the introduction point
5. **Sending INTRODUCE1** - Sending the introduction cell over the circuit
6. **Full Connection Orchestration** - Complete workflow for connecting to an onion service

## Running the Demo

```bash
cd examples/intro-demo
go run main.go
```

## Expected Output

```
=== Introduction Point Protocol Demo ===

✓ Created onion service client
✓ Onion address: thgtoa7imksbg7rit4grgijl2ef6kc7b56bp56pmtta4g354lydlzkqd.onion

--- Creating Descriptor with Introduction Points ---
✓ Descriptor created with 3 introduction points
✓ Descriptor cached

--- Selecting Introduction Point ---
✓ Introduction point selected:
  - OnionKey: 1a2b3c4d...
  - AuthKey: 5e6f7a8b...
  - Link Specifiers: 2

--- Building INTRODUCE1 Cell ---
✓ INTRODUCE1 cell built (108 bytes)
  Cell structure:
    - LEGACY_KEY_ID: 20 bytes (zeros for v3)
    - AUTH_KEY_TYPE: 1 byte (0x02 for ed25519)
    - AUTH_KEY_LEN: 2 bytes
    - AUTH_KEY: 32 bytes
    - EXTENSIONS: N bytes
    - ENCRYPTED_DATA: remaining bytes

--- Creating Introduction Circuit ---
✓ Introduction circuit created (ID: 1000)
  In a full implementation, this circuit would:
    1. Use the circuit builder to create a 3-hop circuit
    2. Extend to the introduction point
    3. Wait for circuit establishment

--- Sending INTRODUCE1 Cell ---
✓ INTRODUCE1 cell sent
  In a full implementation, this would:
    1. Wrap data in RELAY cell with INTRODUCE1 command
    2. Send over the circuit
    3. Wait for acknowledgment

--- Full Connection Orchestration ---
✓ Successfully orchestrated connection to onion service
  Circuit ID: 1000
  Address: 7raifnx6ofgmvzphgy3pxewn47ff4wqevpc6wz6j2qinq4w5eksbe4ad.onion

=== Demo Complete ===
```

## INTRODUCE1 Cell Structure

Per Tor specification (rend-spec-v3.txt section 3.2):

```
INTRODUCE1 {
  LEGACY_KEY_ID     [20 bytes]  - Set to zeros for v3
  AUTH_KEY_TYPE     [1 byte]    - 0x02 for ed25519
  AUTH_KEY_LEN      [2 bytes]   - Length of AUTH_KEY (32 for ed25519)
  AUTH_KEY          [32 bytes]  - Introduction point's auth key
  EXTENSIONS        [N bytes]   - Extension data (N_EXTENSIONS || extensions)
  ENCRYPTED_DATA    [remaining] - Encrypted payload containing:
                                  - RENDEZVOUS_COOKIE (20 bytes)
                                  - ONION_KEY (32 bytes for x25519)
                                  - LINK_SPECIFIERS (for rendezvous point)
}
```

## Implementation Status

### Phase 7.3.3 - Complete ✅

- ✅ Introduction point selection algorithm
- ✅ INTRODUCE1 cell construction per spec
- ✅ Introduction circuit creation (mock)
- ✅ Cell sending protocol (mock)
- ✅ Full connection orchestration
- ✅ Comprehensive unit tests
- ✅ Performance benchmarks

### Phase 7.3.4 - Next Steps

- [ ] Rendezvous point selection
- [ ] RENDEZVOUS1 cell construction
- [ ] RENDEZVOUS2 protocol
- [ ] End-to-end circuit completion
- [ ] Stream establishment

### Phase 8 - Production Hardening

- [ ] Real circuit-based communication
- [ ] Encryption of INTRODUCE1 data with intro point's key
- [ ] INTRODUCE_ACK handling
- [ ] Retry and timeout logic
- [ ] Error handling and recovery
- [ ] Circuit failure handling

## Protocol Flow

The complete onion service connection flow:

1. **Descriptor Fetch** (Phase 7.3.2) - Fetch descriptor from HSDirs
2. **Introduction** (Phase 7.3.3 - Current) - Connect to introduction point
3. **Rendezvous** (Phase 7.3.4 - Next) - Establish rendezvous point
4. **Complete Circuit** - Service connects to rendezvous, circuit complete
5. **Stream Data** - Normal Tor stream over the circuit

## Key Components

### IntroductionProtocol

Main handler for introduction point operations:

```go
intro := onion.NewIntroductionProtocol(log)

// Select an introduction point
introPoint, err := intro.SelectIntroductionPoint(descriptor)

// Build INTRODUCE1 cell
req := &onion.IntroduceRequest{
    IntroPoint:       introPoint,
    RendezvousCookie: cookie,
    RendezvousPoint:  rpFingerprint,
    OnionKey:         ephemeralKey,
}
introduce1Data, err := intro.BuildIntroduce1Cell(req)

// Create circuit and send
circuitID, err := intro.CreateIntroductionCircuit(ctx, introPoint)
err = intro.SendIntroduce1(ctx, circuitID, introduce1Data)
```

### Client.ConnectToOnionService

High-level orchestration:

```go
client := onion.NewClient(log)
circuitID, err := client.ConnectToOnionService(ctx, address)
```

## Technical Details

### Introduction Point Selection

Currently selects the first available introduction point. In a full implementation:
- Filter out previously failed introduction points
- Randomly select from remaining points
- Consider network conditions and relay performance
- Implement retry logic with different introduction points

### Circuit Creation

Mock implementation returns a circuit ID. In production (Phase 8):
- Use circuit builder to create a 3-hop circuit
- Extend circuit to the introduction point
- Wait for circuit to be fully established
- Handle circuit build failures and timeouts

### Cell Encryption

Current implementation uses plaintext for ENCRYPTED_DATA. In production:
- Encrypt with introduction point's public key (curve25519)
- Use proper key derivation and HKDF
- Include MAC for authentication
- Follow Tor specification exactly

## References

- [Tor Rendezvous Specification v3](https://spec.torproject.org/rend-spec-v3.html) - Section 3.2 (INTRODUCE1)
- [Tor Proposal 224: Next-Generation Hidden Services](https://spec.torproject.org/proposals/224-rend-spec-ng.html)
- [go-tor Architecture Documentation](../../docs/ARCHITECTURE.md)

## Testing

Run tests for the introduction protocol:

```bash
cd ../../
go test ./pkg/onion/... -v -run TestIntroduction
go test ./pkg/onion/... -v -run TestConnectToOnionService
```

Run benchmarks:

```bash
go test ./pkg/onion/... -bench=Introduce -benchmem
```

## License

BSD 3-Clause License. See [LICENSE](../../LICENSE) for details.

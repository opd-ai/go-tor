# Rendezvous Protocol Demo

This demo showcases the **Phase 7.3.4: Rendezvous Protocol** implementation for the go-tor project.

## Overview

The rendezvous protocol is the final step in establishing a connection to an onion service. After the introduction protocol (Phase 7.3.3), this phase demonstrates:

1. **Rendezvous Point Selection** - Choosing a relay to act as the meeting point
2. **ESTABLISH_RENDEZVOUS** - Setting up the rendezvous point
3. **RENDEZVOUS1** - Service-side rendezvous cell (for completeness)
4. **RENDEZVOUS2** - Client-side rendezvous completion
5. **Full Connection Orchestration** - Complete end-to-end onion service connection

## What This Demo Shows

### Rendezvous Point Selection
```go
rendezvous := onion.NewRendezvousProtocol(logger)
rendezvousPoint, err := rendezvous.SelectRendezvousPoint(relays)
```
Selects an appropriate relay from the consensus to act as the rendezvous point.

### ESTABLISH_RENDEZVOUS Cell Construction
```go
req := &onion.EstablishRendezvousRequest{
    RendezvousCookie: rendezvousCookie,
}
data, err := rendezvous.BuildEstablishRendezvousCell(req)
```
Constructs an ESTABLISH_RENDEZVOUS cell per Tor specification (rend-spec-v3.txt section 3.3).

### Rendezvous Circuit Creation
```go
circuitID, err := rendezvous.CreateRendezvousCircuit(ctx, rendezvousPoint)
```
Creates a circuit to the selected rendezvous point.

### RENDEZVOUS1 Cell (Service Side)
```go
req := &onion.Rendezvous1Request{
    RendezvousCookie: cookie,
    HandshakeData:    handshake,
}
data, err := rendezvous.BuildRendezvous1Cell(req)
```
Demonstrates the service-side RENDEZVOUS1 cell construction (included for completeness in client implementation).

### RENDEZVOUS2 Cell Parsing (Client Side)
```go
handshakeData, err := rendezvous.ParseRendezvous2Cell(data)
```
Parses the RENDEZVOUS2 cell received from the hidden service.

### Full Connection Orchestration
```go
circuitID, err := client.ConnectToOnionService(ctx, address)
```
Demonstrates the complete connection flow:
1. Fetch descriptor
2. Establish rendezvous point
3. Select introduction point
4. Send INTRODUCE1
5. Wait for RENDEZVOUS2
6. Complete connection

## Running the Demo

```bash
cd examples/rendezvous-demo
go run main.go
```

## Expected Output

```
=== Rendezvous Protocol Demo ===

✓ Created onion service client
✓ Onion address: [56-char v3 address].onion
✓ Updated client with 3 available relays

--- Selecting Rendezvous Point ---
✓ Rendezvous point selected:
  - Fingerprint: AAAA...
  - Address: 1.1.1.1:9001

--- Building ESTABLISH_RENDEZVOUS Cell ---
✓ ESTABLISH_RENDEZVOUS cell built (20 bytes)

--- Creating Rendezvous Circuit ---
✓ Rendezvous circuit created
  - Circuit ID: 2000

--- Sending ESTABLISH_RENDEZVOUS ---
✓ ESTABLISH_RENDEZVOUS sent successfully

--- Building RENDEZVOUS1 Cell (Service Side) ---
✓ RENDEZVOUS1 cell built (52 bytes)

--- Parsing RENDEZVOUS2 Cell (Client Side) ---
✓ RENDEZVOUS2 cell parsed successfully

--- Full Rendezvous Point Establishment ---
✓ Rendezvous point fully established

--- Completing Rendezvous Protocol ---
✓ Rendezvous protocol completed successfully

--- Full Onion Service Connection Orchestration ---
✓ Successfully orchestrated full connection to onion service

=== Demo Complete ===
```

## Implementation Details

### Phase 7.3.4 Components

1. **RendezvousProtocol** - Handler for rendezvous operations
2. **SelectRendezvousPoint()** - Relay selection (currently first-available)
3. **BuildEstablishRendezvousCell()** - ESTABLISH_RENDEZVOUS cell construction
4. **CreateRendezvousCircuit()** - Circuit creation to rendezvous point
5. **SendEstablishRendezvous()** - Send ESTABLISH_RENDEZVOUS cell
6. **BuildRendezvous1Cell()** - RENDEZVOUS1 cell construction (service-side)
7. **ParseRendezvous2Cell()** - RENDEZVOUS2 cell parsing (client-side)
8. **WaitForRendezvous2()** - Wait for RENDEZVOUS2 from service
9. **EstablishRendezvousPoint()** - High-level establishment API
10. **CompleteRendezvous()** - Complete rendezvous protocol

### Protocol Flow

```
Client                  Rendezvous Point              Hidden Service
  |                            |                             |
  |--- ESTABLISH_RENDEZVOUS -->|                             |
  |<-- RENDEZVOUS_ESTABLISHED -|                             |
  |                            |                             |
  |                            |<-- RENDEZVOUS1 -------------|
  |<-- RENDEZVOUS2 ------------|                             |
  |                            |                             |
  |<-------------- Connection Established ------------------>|
```

### Tor Specification Compliance

This implementation follows:
- **rend-spec-v3.txt section 3.3** - ESTABLISH_RENDEZVOUS
- **rend-spec-v3.txt section 3.4** - RENDEZVOUS1/RENDEZVOUS2

### Cell Structures

**ESTABLISH_RENDEZVOUS**:
```
RENDEZVOUS_COOKIE [20 bytes]
```

**RENDEZVOUS1** (sent by service):
```
RENDEZVOUS_COOKIE [20 bytes]
HANDSHAKE_DATA    [remaining bytes]
```

**RENDEZVOUS2** (sent to client):
```
HANDSHAKE_DATA [remaining bytes]
```

## Current Implementation Status

### Phase 7.3.4 (Rendezvous Protocol) ✅ Complete
- ✅ Rendezvous point selection
- ✅ ESTABLISH_RENDEZVOUS cell construction
- ✅ Rendezvous circuit creation
- ✅ RENDEZVOUS1 cell construction
- ✅ RENDEZVOUS2 cell parsing
- ✅ Full protocol orchestration
- ✅ SOCKS5 integration

### Mock vs Production

**Current (Phase 7.3.4)**:
- Circuit creation returns mock circuit IDs
- Cell sending is logged but not sent over real circuits
- Handshake data is not encrypted
- No actual network communication

**Future (Phase 8)**:
- Real circuit creation using circuit builder
- Actual cell transmission over circuits
- Full cryptographic handshake
- End-to-end encrypted communication
- Relay selection with performance metrics
- Retry logic and error handling

## Integration with Existing Code

The rendezvous protocol seamlessly integrates with:
- **Phase 7.3.1** - Descriptor management (caching, blinded keys)
- **Phase 7.3.2** - HSDir protocol (descriptor fetching)
- **Phase 7.3.3** - Introduction protocol (INTRODUCE1)
- **SOCKS5 proxy** - Automatic .onion address handling

## Testing

The implementation includes comprehensive tests:
- 15 unit tests for rendezvous protocol
- 3 benchmarks for performance
- Full integration test with ConnectToOnionService
- All existing tests continue to pass (271+ total)

Run tests:
```bash
cd ../../
go test ./pkg/onion/... -v
```

## Next Steps

### Phase 8 - Production Hardening
1. Real circuit-based communication
2. Cryptographic handshake implementation
3. Error handling and retry logic
4. Performance optimization
5. Security audit

### Usage in Production

Once Phase 8 is complete, connecting to an onion service will be as simple as:

```go
client := onion.NewClient(logger)
circuitID, err := client.ConnectToOnionService(ctx, onionAddress)
// Use circuitID to send/receive data
```

## References

- [Tor Rendezvous Specification v3](https://spec.torproject.org/rend-spec-v3.html)
- [Tor Proposal 224: Next-Generation Hidden Services](https://spec.torproject.org/proposals/224-rend-spec-ng.html)
- [C Tor Implementation](https://github.com/torproject/tor)

## License

BSD 3-Clause License. See [LICENSE](../../LICENSE) for details.

# HSDir Protocol Demo

This example demonstrates the Hidden Service Directory (HSDir) protocol implementation for Tor onion services (Phase 7.3.2).

## What This Demo Shows

- **HSDir Selection**: DHT-style routing to find responsible HSDirs for a descriptor
- **Replica Management**: Computing descriptor IDs for multiple replicas (redundancy)
- **Blinded Key Derivation**: Time-based public key blinding for privacy
- **Time Period Calculation**: 24-hour descriptor rotation periods
- **Descriptor Fetching**: Retrieving descriptors from HSDirs with fallback
- **Caching**: Efficient descriptor caching to minimize network requests

## Running the Demo

```bash
cd examples/hsdir-demo
go run main.go
```

## Expected Output

The demo will:
1. Parse a v3 onion address
2. Compute the current time period
3. Derive the blinded public key
4. Create mock HSDirs (simulating consensus)
5. Demonstrate HSDir selection for both replicas
6. Fetch a descriptor from HSDirs
7. Demonstrate descriptor caching

## Phase 7.3.2 Features

### HSDir Selection Algorithm

The HSDir selection algorithm implements DHT-style routing:
- Computes XOR distance between HSDir fingerprints and descriptor ID
- Selects the 3 closest HSDirs for each replica
- Supports replica-based redundancy (2 replicas)

### Descriptor Fetching

The descriptor fetching process:
1. Computes time period and blinded public key
2. Derives descriptor ID from blinded key
3. Selects responsible HSDirs for each replica
4. Attempts to fetch from each HSDir in order
5. Caches successfully retrieved descriptors

### Technical Details

**Time Period**: `(unix_time + 12h) / 24h`
- Descriptors rotate every 24 hours
- 12-hour offset prevents synchronization issues

**Blinded Public Key**: `SHA3-256("Derive temporary signing key" || pubkey || time_period)`
- Privacy: Different blinded key each time period
- Unlinkability: Can't correlate descriptors across time periods

**Descriptor ID**: `SHA3-256(blinded_pubkey)`
- Unique identifier for descriptor location
- Used for HSDir selection

**Replica ID**: `SHA3-256(descriptor_id || replica_number)`
- Multiple replicas for redundancy
- Different HSDirs store each replica

## Next Steps

Phase 7.3.3 will implement:
- Introduction point protocol
- INTRODUCE1 cell construction  
- Circuit creation to introduction points
- INTRODUCE_ACK handling

## References

- [Tor Rendezvous Specification v3](https://spec.torproject.org/rend-spec-v3.html)
- [Proposal 224: Next-Generation Hidden Services](https://spec.torproject.org/proposals/224-rend-spec-ng.html)

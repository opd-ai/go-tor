# Descriptor Management Demo

This example demonstrates the onion service descriptor management functionality implemented in Phase 7.3.1.

## Features Demonstrated

- **Descriptor Caching**: Efficient caching of onion service descriptors with expiration
- **Time Period Calculation**: Computing the current time period for descriptor rotation
- **Descriptor Fetching**: Mock descriptor fetching with cache integration
- **Descriptor Encoding/Parsing**: Converting descriptors to/from wire format
- **Cache Operations**: Put, Get, Remove, Clear, and CleanExpired operations

## Running the Demo

```bash
cd examples/descriptor-demo
go run main.go
```

## Expected Output

The demo will:
1. Create an onion service client
2. Parse a v3 onion address
3. Calculate the current time period
4. Fetch a descriptor (mock implementation)
5. Demonstrate cache hit on second fetch
6. Show cache statistics with multiple descriptors
7. Encode and parse a descriptor

## Implementation Status

✅ **Complete (Phase 7.3.1)**:
- Descriptor cache with expiration
- Time period calculation per Tor spec
- Blinded public key computation
- Descriptor encoding/parsing foundation
- Cache management operations

🚧 **Future Phases**:
- Phase 7.3.2: Full HSDir protocol implementation
- Phase 7.3.3: Introduction point protocol
- Phase 7.3.4: Rendezvous protocol

## Notes

This demo uses a mock descriptor fetching implementation. The full HSDir protocol for fetching real descriptors from the Tor network will be implemented in future phases.

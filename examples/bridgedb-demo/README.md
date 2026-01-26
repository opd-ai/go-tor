# BridgeDB Integration Demo

**⚠️ WARNING: This is an educational implementation ONLY!**

This example demonstrates a simplified bridge distribution system inspired by Tor's BridgeDB. **Do NOT use this for real anonymity or privacy needs.** For actual Tor bridges, visit [bridges.torproject.org](https://bridges.torproject.org/).

## What is BridgeDB?

BridgeDB is Tor's bridge distribution system that:
- Receives bridge descriptors from bridge authorities
- Distributes bridges to users through various channels (HTTPS, email, Moat)
- Implements rate limiting and anti-enumeration measures
- Categorizes bridges by transport type (obfs4, meek, etc.)

## This Educational Implementation

This demo implements core BridgeDB concepts for research and learning:

### Features

1. **Bridge Database Management**
   - Add/remove bridges
   - Filter by transport type
   - Track statistics

2. **HTTP Distribution API**
   - `GET /bridges` - Get bridges (with optional transport filter and count)
   - `GET /stats` - Get distribution statistics

3. **Email Responder Simulation**
   - Generate bridge lines in email response format
   - Rate limiting per email address

4. **Rate Limiting**
   - Default: 1 hour between requests from same IP/email
   - Prevents bridge enumeration

5. **Deterministic Selection**
   - Same requestor gets same bridges (within rate limit window)
   - Distribution across available bridge pool

## Running the Demo

```bash
cd examples/bridgedb-demo
go run main.go
```

The server will start on port 8080.

## Example Requests

### Get all bridges
```bash
curl http://localhost:8080/bridges
```

Response:
```json
{
  "bridges": [
    "Bridge 192.0.2.1:9001 A1B2C3D4E5F6A7B8C9D0E1F2A3B4C5D6E7F8A9B0",
    "Bridge obfs4 192.0.2.2:9002 B2C3D4E5F6A7B8C9D0E1F2A3B4C5D6E7F8A9B0C1 cert=abcd1234;iat-mode=0"
  ],
  "count": 2
}
```

### Get obfs4 bridges only
```bash
curl "http://localhost:8080/bridges?transport=obfs4&count=2"
```

### Get statistics
```bash
curl http://localhost:8080/stats
```

Response:
```json
{
  "total_bridges": 4,
  "by_transport": {
    "vanilla": 1,
    "obfs4": 2,
    "meek_lite": 1
  }
}
```

## Code Structure

- `BridgeDistributor` - Core bridge management and distribution logic
- `BridgeDistributorServer` - HTTP API server
- `EmailResponder` - Email response generation (simulated)

## Educational Purposes Only

This implementation:
- ✅ Demonstrates BridgeDB concepts
- ✅ Teaches bridge distribution mechanisms
- ✅ Shows rate limiting implementation
- ❌ Is NOT suitable for production use
- ❌ Does NOT provide real anonymity
- ❌ Should NOT be used for actual bridge distribution

## For Real Tor Bridges

If you need real Tor bridges:
- **Users**: Get bridges from [bridges.torproject.org](https://bridges.torproject.org/)
- **Email**: Send an email to bridges@torproject.org
- **Tor Browser**: Use built-in bridge configuration

## License

Part of the go-tor educational implementation project. See main repository README for license details.

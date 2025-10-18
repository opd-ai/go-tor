# Onion Address Demo

This example demonstrates how to use the `pkg/onion` package to work with v3 .onion addresses.

## Features Demonstrated

1. **Parsing and validation** - Parse v3 onion addresses and validate checksums
2. **Address detection** - Check if a string is an onion address
3. **Address generation** - Create new valid v3 onion addresses
4. **Encoding** - Encode addresses back to string format
5. **Round-trip** - Parse and encode addresses without data loss
6. **Error handling** - Handle invalid addresses gracefully

## Running the Example

```bash
cd examples/onion-address-demo
go run main.go
```

## Example Output

```
=== Onion Address Demo ===

Example 1: Parsing a v3 onion address
---------------------------------------
Address: vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion
✓ Valid v3 address!
  Version: 3
  Public key length: 32 bytes
  Public key (hex): adadec040be047f9...

Example 2: Checking if strings are onion addresses
---------------------------------------------------
✗ example.com is NOT an onion address
✓ vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion IS an onion address
✗ 192.168.1.1 is NOT an onion address
✓ invalid.onion IS an onion address

Example 3: Generating a new v3 onion address
---------------------------------------------
Generated address: jnvhht6xdw3sjf26dxucczeenc2fn3ewxxosbkn45gzpt7nxhbqul6qd.onion
✓ Successfully generated and validated new address
  Length: 56 characters (excluding .onion)

...
```

## v3 Onion Address Format

v3 onion addresses are 56 characters long (plus ".onion") and are based on ed25519 public keys:

```
<base32(pubkey || checksum || version)>.onion

- pubkey: 32 bytes (ed25519 public key)
- checksum: 2 bytes (SHA3-256 derived)
- version: 1 byte (0x03 for v3)
```

Example: `vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion`

## API Usage

### Parse an Address

```go
addr, err := onion.ParseAddress("vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion")
if err != nil {
    // Handle error
}
fmt.Printf("Version: %d\n", addr.Version)
fmt.Printf("Pubkey: %x\n", addr.Pubkey)
```

### Check if String is Onion Address

```go
if onion.IsOnionAddress("example.onion") {
    fmt.Println("This is an onion address")
}
```

### Generate a New Address

```go
pubkey, _, _ := ed25519.GenerateKey(rand.Reader)
addr := &onion.Address{
    Version: onion.V3,
    Pubkey:  pubkey,
}
encoded := addr.Encode()
fmt.Println(encoded) // prints: xxxxxxx...xxx.onion
```

### Encode an Address

```go
addr, _ := onion.ParseAddress("vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion")
encoded := addr.Encode()
// encoded is the same as the original address
```

## Performance

- **Parsing**: ~1μs per address
- **Encoding**: ~0.86μs per address
- **Detection**: <10ns (simple string suffix check)

## Notes

- Only v3 addresses are supported (v2 is deprecated by Tor Project)
- Checksums are cryptographically validated using SHA3-256
- All operations are case-insensitive
- Invalid addresses return clear error messages

## See Also

- [PHASE73_ONION_SERVICES_REPORT.md](../../PHASE73_ONION_SERVICES_REPORT.md) - Complete implementation details
- [pkg/onion/onion.go](../../pkg/onion/onion.go) - Package source code
- [pkg/onion/onion_test.go](../../pkg/onion/onion_test.go) - Test suite

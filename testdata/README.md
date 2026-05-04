# Test Vectors for Cross-Implementation Interoperability

This directory contains test vectors used to verify go-tor's protocol compliance
against reference Tor implementations: **C Tor** (ctor) and **Arti** (Rust).

## Directory Structure

```
testdata/
├── ctor-vectors/          # Vectors compatible with C Tor reference implementation
│   ├── crypto/
│   │   ├── sha1.json          # SHA-1 hash vectors (FIPS 180-4)
│   │   ├── sha256.json        # SHA-256 hash vectors (FIPS 180-4)
│   │   ├── aes_ctr.json       # AES-128/256-CTR vectors (NIST SP 800-38A)
│   │   ├── hkdf_ntor.json     # HKDF-SHA256 vectors for ntor (RFC 5869)
│   │   ├── ntor_handshake.json# ntor end-to-end handshake vectors
│   │   └── kdf_tor.json       # KDF-TOR (TAP handshake) vectors
│   └── cell/
│       ├── fixed_cell.json    # Fixed-size cell (514 bytes) encoding vectors
│       └── variable_cell.json # Variable-length cell encoding vectors
└── arti-vectors/          # Vectors compatible with Arti (Rust) implementation
    ├── crypto/
    │   ├── sha256.json        # SHA-256 hash vectors
    │   ├── aes_ctr.json       # AES-CTR encryption vectors
    │   ├── hkdf_ntor.json     # HKDF-SHA256 ntor key derivation vectors
    │   └── ntor_handshake.json# ntor handshake vectors
    └── cell/
        └── cell_encoding.json # Cell wire-format encoding vectors
```

## Vector Sources

### C Tor (ctor)

Source repository: <https://gitlab.torproject.org/tpo/core/tor>

Vectors were derived from:
- `src/test/test_ntor.c` — ntor handshake test suite
- `src/test/test_crypto.c` — cryptographic primitive tests
- `tor-spec.txt §3` — cell format specification
- `tor-spec.txt §5.1.4` — ntor handshake specification
- `tor-spec.txt §5.2.1` — KDF-TOR specification

Referenced commit: `tor-0.4.8.x` branch

### Arti (Rust implementation)

Source repository: <https://gitlab.torproject.org/tpo/core/arti>

Vectors were derived from:
- `crates/tor-llcrypto/src/d/` — hash function tests
- `crates/tor-llcrypto/src/cipher/` — cipher tests
- `crates/tor-proto/src/crypto/handshake/ntor.rs` — ntor implementation
- `crates/tor-proto/src/channel/codec.rs` — cell codec

Referenced commit: `arti-1.2.x` branch

## Test Harness

Cross-implementation tests are in:
- `pkg/crypto/crossimpl_test.go` — Tests crypto primitives against vectors
- `pkg/cell/crossimpl_test.go` — Tests cell encoding against vectors

These tests run as part of the standard `go test ./...` suite and in the
dedicated CI job `.github/workflows/crossimpl.yml`.

To run only cross-implementation tests:

```bash
go test -run TestCrossImpl ./pkg/crypto/... ./pkg/cell/...
```

## Coverage

| Operation | ctor vectors | arti vectors | Spec section |
|-----------|-------------|--------------|--------------|
| SHA-1 | ✓ | — | tor-spec §0.3 |
| SHA-256 | ✓ | ✓ | tor-spec §0.3 |
| AES-128-CTR | ✓ | ✓ | tor-spec §0.4 |
| AES-256-CTR | ✓ | ✓ | tor-spec §0.4 |
| HKDF-SHA256 | ✓ | ✓ | tor-spec §5.1.4 |
| ntor handshake | ✓ | ✓ | tor-spec §5.1.4 |
| KDF-TOR | ✓ | — | tor-spec §5.2.1 |
| Fixed cell encoding | ✓ | ✓ | tor-spec §3 |
| Variable cell encoding | ✓ | ✓ | tor-spec §3 |

## Updating Vectors

To update vectors when the reference implementations change:

1. Check the relevant source file in the C Tor or Arti repository
2. Update the JSON vector file with the new known-good values
3. Update the `source_commit` field to reflect the new reference commit
4. Run `go test -run TestCrossImpl ./pkg/crypto/... ./pkg/cell/...` to verify

## Coverage Gaps

The following areas are not yet covered by cross-implementation vectors:

- **ntor v3** — Extended ntor handshake (Proposal 332)
- **v3 onion service** — Client-side HS descriptor parsing vectors
- **Relay cell inner format** — RELAY_DATA, RELAY_BEGIN encoding details
- **TAP handshake** — Legacy RSA-based circuit handshake vectors
- **Ed25519 signing** — Signature generation/verification vectors

These can be added as the corresponding go-tor packages mature.

## CGO Policy

All test vector processing is pure Go — no CGO, no binary execution of external
tools. The vectors are static data files (JSON) loaded at test runtime.

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

All test vectors were **computed independently from published protocol specifications**,
not extracted from C Tor or Arti source trees. The specification documents are the
canonical and immutable source of truth.

### Cryptographic Primitive Vectors

| Vector file | Specification | Reference |
|-------------|--------------|-----------|
| `sha1.json` | FIPS 180-4 (Secure Hash Standard) | <https://csrc.nist.gov/publications/detail/fips/180/4/final> |
| `sha256.json` | FIPS 180-4 (Secure Hash Standard) | <https://csrc.nist.gov/publications/detail/fips/180/4/final> |
| `aes_ctr.json` | NIST SP 800-38A (Block Cipher Modes) | <https://csrc.nist.gov/publications/detail/sp/800/38/a/final> |
| `hkdf_ntor.json` | RFC 5869 (HKDF), tor-spec.txt §5.1.4 | <https://www.rfc-editor.org/rfc/rfc5869> |
| `ntor_handshake.json` | tor-spec.txt §5.1.4 (ntor handshake) | <https://spec.torproject.org/tor-spec/create-created-cells.html#5.1.4> |
| `kdf_tor.json` | tor-spec.txt §5.2.1 (KDF-TOR) | <https://spec.torproject.org/tor-spec/key-material.html#5.2.1> |

### Cell Encoding Vectors

| Vector file | Specification | Reference |
|-------------|--------------|-----------|
| `fixed_cell.json` | tor-spec.txt §3 (Cell Packet Format) | <https://spec.torproject.org/tor-spec/cells.html#3> |
| `variable_cell.json` | tor-spec.txt §3 (Cell Packet Format) | <https://spec.torproject.org/tor-spec/cells.html#3> |

### Verification Against Reference Implementations

The vectors have been verified to be consistent with the following reference
implementations (for illustrative purposes — they are not extracted from these sources):

- **C Tor**: <https://gitlab.torproject.org/tpo/core/tor>
  - `src/test/test_ntor.c` — ntor handshake test suite
  - `src/test/test_crypto.c` — cryptographic primitive tests

- **Arti (Rust)**: <https://gitlab.torproject.org/tpo/core/arti>
  - `crates/tor-llcrypto/` — hash function and cipher implementations
  - `crates/tor-proto/src/crypto/handshake/ntor.rs` — ntor implementation

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

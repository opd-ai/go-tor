# go-tor: README / GoDoc vs Implementation Gaps

**Repository:** opd-ai/go-tor  
**Scope:** Discrepancies between documented claims (README, GoDoc, ROADMAP, code comments) and actual implementation behavior.

---

## Gap Legend

| Type | Meaning |
|------|---------|
| MISSING | Feature documented as present/working; not implemented |
| PARTIAL | Feature partially implemented; key parts missing |
| MISLEADING | Documentation/comment accurately implies X, implementation does Y |
| UNSAFE | Documentation omits a security-critical limitation |

---

## MISSING Implementations

### GAP-M-1 — `NtorClientHandshake` GoDoc does not disclose broken return value

**Type:** UNSAFE  
**File:** `pkg/crypto/crypto.go`

The exported function `NtorClientHandshake` is documented as performing the ntor handshake and returning a shared secret. The actual implementation returns `ephemeral.Private[:]` — the raw ephemeral private key — with a `// TODO: Complete implementation - this is a placeholder` comment. Neither the GoDoc nor the function signature communicates this. Any library consumer calling this function will unknowingly use the private key as key material.

**Expected:** GoDoc must explicitly state that the return value is a placeholder / not a valid shared secret, and callers must use `NtorProcessResponse` instead.

---

### GAP-M-2 — `VerifyDigest` exported API is broken but undocumented

**Type:** MISSING  
**File:** `pkg/circuit/circuit.go:485–516`

`VerifyDigest` is exported as a functional digest verification API. Its GoDoc says:
> "VerifyDigest verifies the digest of an incoming relay cell (CRYPTO-001). This prevents cell injection and replay attacks per tor-spec.txt §6.1. Returns error if digest verification fails."

In practice, `VerifyDigest` computes `digest.Sum(nil)` on the **pre-cell** hash state and compares against the received digest (which per Tor spec is the **post-cell** hash). The comparison never matches for cells from a spec-compliant peer. The function always returns an error for valid incoming cells. The GoDoc contains no mention of this limitation.

---

### GAP-M-3 — `verifyRelayIdentityPinning` is a stub but is invoked as a security check

**Type:** MISSING  
**File:** `pkg/connection/connection.go:153–210`

The TLS `VerifyPeerCertificate` callback is set to `verifyRelayIdentityPinning` when `ExpectedIdentity` or `ExpectedFingerprint` is non-nil. The GoDoc-level comment on `Config.ExpectedIdentity` says:
> "Expected relay Ed25519 identity key (32 bytes) - for certificate pinning (AUDIT-004)"

The function body parses the certificate for structural validity but silently accepts all certificates regardless of identity. There is no comparison against `expectedIdentity` or `expectedFingerprint`. The TLS-level pinning described in comments does not exist.

---

### GAP-M-4 — README claims "v3 onion service support"; actual routing is unimplemented

**Type:** PARTIAL  
**File:** `pkg/onion/`, README.md

README.md advertises:
> "v3 onion service support"

The `pkg/onion` package provides:
- ✅ Address parsing and checksum validation  
- ✅ Descriptor struct definitions and parsing skeleton  
- ✅ HSDir fetch over clearnet HTTP (with `InsecureSkipVerify: true`)  
- ✅ `RendezvousState` management  
- ❌ No actual circuit-based descriptor fetching (uses clearnet HTTP to HSDirs)  
- ❌ No INTRODUCE1 cell construction  
- ❌ No rendezvous completion (RENDEZVOUS1/RENDEZVOUS2 exchange)  
- ❌ No end-to-end onion service circuit establishment

Connecting to a `.onion` address through this client does not work. The `CellSender` and `CircuitBuilder` interfaces are defined but never wired in the main client path (`pkg/client/`).

---

### GAP-M-5 — README implies working SOCKS5 proxy; circuit communication is broken

**Type:** MISLEADING  
**File:** README.md, `pkg/socks/`

README.md describes:
> "SOCKS5 proxy server"
> Usage example: `curl --proxy socks5h://127.0.0.1:9050 https://check.torproject.org`

The SOCKS5 server accepts connections and obtains a circuit, but due to AUDIT-HIGH-1 (relay cell digest verification always fails), all relay cells from real Tor relays are silently dropped. `RELAY_CONNECTED` cells are never delivered to `OpenStream`, causing every SOCKS `CONNECT` to time out. The proxy advertised in the README does not function for real Tor network traffic.

---

### GAP-M-6 — GoDoc for `ValidateConsensusMetadata` omits that failures are silently ignored by caller

**Type:** UNSAFE  
**File:** `pkg/directory/directory.go:694–739`

`ValidateConsensusMetadata` is documented as validating consensus metadata per dir-spec.txt §3.4. Its GoDoc does not mention that the sole caller (`FetchConsensus`) ignores validation errors with a `logger.Warn`. Consumers expecting this validation to gate consensus use will find it has no enforcement effect.

---

### GAP-M-7 — Bridge relay (`pkg/relay/`) documented in ROADMAP but non-functional

**Type:** PARTIAL  
**File:** `pkg/relay/`, `ROADMAP.md`

ROADMAP.md Phase 3 describes a "Bridge relay with pluggable transport support." The `pkg/relay/` package (14 files) and `pkg/pt/` (8 files) exist with interfaces defined, but no complete bridge relay accepts inbound connections or forwards cells. The relay loop, cell forwarding, and PT integration are skeleton code.

---

### GAP-M-8 — SOCKS5 UDP ASSOCIATE not implemented

**Type:** MISSING  
**File:** `pkg/socks/socks.go`

RFC 1928 defines three SOCKS5 commands: CONNECT, BIND, and UDP ASSOCIATE. The `pkg/socks` package only handles `CONNECT`. `UDP ASSOCIATE` requests are rejected with "command not supported". The README and GoDoc do not call this out.

---

## PARTIAL Implementations

### GAP-P-1 — CERTS cell validation is partial: cert chain not rooted to identity key

**Type:** PARTIAL  
**File:** `pkg/protocol/certs.go:450–495`

`ValidateSignatures` is documented as verifying Ed25519 certificate signatures per cert-spec.txt. It verifies:
- Type 5 cert signed by type 4 cert ✅  
- Type 6 cert signed by type 4 cert ✅  
- Type 4 cert as **self-signed** ❌ (should be signed by identity key from type 7 cert)

The cert chain per cert-spec.txt should be: type-7 RSA cross-cert anchors type-4, type-4 signs type-5 and type-6. The implementation skips the anchor step, making the chain unrooted.

---

### GAP-P-2 — `ValidateRelayIdentity` computes non-standard RSA fingerprint

**Type:** PARTIAL  
**File:** `pkg/protocol/certs.go:305–317`

```go
// Comment says "For Tor, we use SHA-256 of the DER encoding"
fingerprint := sha256.Sum256(derBytes)
fingerprintHex := fmt.Sprintf("%X", fingerprint[:20]) // Use first 20 bytes
```

Tor's relay fingerprint is SHA-1 of the DER-encoded RSA public key (per dir-spec.txt), not SHA-256 truncated to 20 bytes. The GoDoc doesn't document which algorithm is used. Fingerprints in consensus data (SHA-1 hex) will never match against this implementation.

---

### GAP-P-3 — Consensus `minSignatureThreshold` value and parser not documented

**Type:** PARTIAL  
**File:** `pkg/directory/directory.go`

`ValidateConsensusMetadata` enforces a `minSignatureThreshold` for authority signatures. The actual value of this constant and whether the signature-counting code path in the consensus parser is exercised is not documented. If the consensus parser does not populate `SignatureCount` for microdesc-format consensus documents (the default), the threshold check trivially passes with `0 < 0 = false`.

---

### GAP-P-4 — Circuit padding parameters fetched from consensus but never applied

**Type:** PARTIAL  
**File:** `pkg/directory/directory.go:764–830`, `pkg/circuit/padding.go`

`GetPaddingParams` correctly extracts `circpad_*` parameters from consensus metadata into a `PaddingParams` struct. However no code in `pkg/circuit/` or `pkg/client/` calls `GetPaddingParams` and applies the result to circuit padding machines. The padding machines in `pkg/circuit/padding_machine.go` use hard-coded defaults.

---

### GAP-P-5 — Relay descriptor `IdentityKey` and `NtorOnionKey` fields not populated

**Type:** PARTIAL  
**File:** `pkg/directory/directory.go`, `pkg/circuit/extension.go:288–363`

`extension.go` attempts to extract `IdentityKey` and `NtorOnionKey` from a relay descriptor via two interface patterns. The `directory.Relay` struct does not expose `GetIdentityKey()` or `GetNtorOnionKey()` methods, and the `IdentityKey`/`NtorOnionKey` fields are not defined on `Relay`. The fallback (lines 183–186) uses 32 zero bytes as placeholder keys:
```go
relayIdentity = make([]byte, 32)  // all zeros
relayNtorKey  = make([]byte, 32)  // all zeros
```

Any circuit created against a real relay will use zero keys, causing the ntor handshake to fail.

---

### GAP-P-6 — NETINFO cell does not include actual local address

**Type:** PARTIAL  
**File:** `pkg/protocol/protocol.go:177–209`

The NETINFO cell is sent with `0.0.0.0` as the "other address" (relay's observed address) and 0 "this addresses". Per tor-spec.txt §4.5, this cell should include the relay's observed IP address of our connection and our own external address. Relays may reject or de-prioritize connections with malformed NETINFO cells.

---

## Documentation / Comment Mismatches

### GAP-D-1 — `encryptForward` comment says "guard → middle → exit"; loop does opposite

**File:** `pkg/circuit/circuit.go:569`

```go
// Comment: "forward order (guard -> middle -> exit)"
// Actual loop: for i := len(hops) - 1; i >= 0; i-- // exit → middle → guard
```

The code is correct for onion encryption (guard's layer must be outermost, so applied last). The comment is wrong and could cause a future developer to "fix" the loop to the incorrect order.

---

### GAP-D-2 — `verifyRelayIdentityPinning` comment describes unimplemented future behavior

**File:** `pkg/connection/connection.go:153`

The function comment says:
> "identity verification happens post-TLS in link protocol"

This implies the pinning is intentionally deferred, but the link-protocol CERTS validation also has gaps (GAP-P-1, GAP-P-2). Together, neither layer provides complete identity verification.

---

### GAP-D-3 — `NtorProcessResponse` not referenced from `NtorClientHandshake` GoDoc

**File:** `pkg/crypto/crypto.go`

The GoDoc for `NtorClientHandshake` does not reference `NtorProcessResponse` as the required second phase. Library consumers reading only the exported API will not discover the two-phase design.

---

## Summary Table

| ID | Type | Package | Description |
|----|------|---------|-------------|
| GAP-M-1 | UNSAFE | pkg/crypto | NtorClientHandshake returns private key; GoDoc silent |
| GAP-M-2 | MISSING | pkg/circuit | VerifyDigest is broken; GoDoc implies it works |
| GAP-M-3 | MISSING | pkg/connection | verifyRelayIdentityPinning is a stub; Config docs imply pinning works |
| GAP-M-4 | PARTIAL | pkg/onion | v3 onion services: routing and protocol unimplemented |
| GAP-M-5 | MISLEADING | pkg/socks | README implies working proxy; SOCKS CONNECT is non-functional end-to-end |
| GAP-M-6 | UNSAFE | pkg/directory | ValidateConsensusMetadata ignored by caller; GoDoc silent |
| GAP-M-7 | PARTIAL | pkg/relay | Bridge relay in ROADMAP but not functional |
| GAP-M-8 | MISSING | pkg/socks | UDP ASSOCIATE not implemented; not documented |
| GAP-P-1 | PARTIAL | pkg/protocol | CERTS chain not rooted to identity key |
| GAP-P-2 | PARTIAL | pkg/protocol | RSA fingerprint uses SHA-256 not SHA-1 (wrong algorithm) |
| GAP-P-3 | PARTIAL | pkg/directory | Signature threshold check may always trivially pass |
| GAP-P-4 | PARTIAL | pkg/circuit | Padding params fetched but never applied |
| GAP-P-5 | PARTIAL | pkg/directory | Relay IdentityKey/NtorOnionKey not populated; zero keys used |
| GAP-P-6 | PARTIAL | pkg/protocol | NETINFO cell uses 0.0.0.0; omits actual address |
| GAP-D-1 | MISLEADING | pkg/circuit | encryptForward comment contradicts correct loop direction |
| GAP-D-2 | MISLEADING | pkg/connection | verifyRelayIdentityPinning comment implies deferred, not missing |
| GAP-D-3 | MISLEADING | pkg/crypto | NtorClientHandshake GoDoc doesn't reference NtorProcessResponse |

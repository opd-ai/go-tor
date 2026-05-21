# go-tor Security & Bug Audit Report

**Repository:** opd-ai/go-tor  
**Audited commit:** HEAD (branch at time of audit)  
**Audit scope:** All source files in `pkg/`, `cmd/`, `examples/`  
**Method:** Static analysis, manual code review, `go vet`, `go build`  
**Baseline:** `go vet ./...` — clean; `go build ./...` — clean

---

## Severity Legend

| Level | Meaning |
|-------|---------|
| CRITICAL | Cryptographic failure or key leakage; service unusable or actively insecure |
| HIGH | Security boundary bypassed or core protocol broken |
| MEDIUM | Incorrect behavior under common conditions; reliability risk |
| LOW | Code quality or minor protocol deviation |

---

## CRITICAL Findings

### AUDIT-CRIT-1 — `NtorClientHandshake` returns the ephemeral private key as the "shared secret"

**File:** `pkg/crypto/crypto.go:368–370`

```go
// TODO: Complete implementation - this is a placeholder
return NtorResult{
    SharedSecret: ephemeral.Private[:], // BUG: returns private key
}, nil
```

**Impact:** Any caller that uses the return value of `NtorClientHandshake` as key material receives the raw ephemeral private key instead of the true ECDH shared secret. This completely breaks forward secrecy and key confidentiality for any code path that calls this function directly.

**Note on mitigation:** `pkg/circuit/extension.go` avoids this function and calls `NtorProcessResponse` directly after building the handshake bytes manually. However the exported public API (`NtorClientHandshake`) remains broken and is a trap for any future caller or consumer of the library.

**Remediation:** Implement the full ntor handshake ECDH: `sharedSecret = ECDH(ephemeral.Private, serverNtorKey) || ECDH(ephemeral.Private, serverIdentity)` per tor-spec.txt §5.1.4, then apply the HMAC-SHA256 KDF. Zero `ephemeral.Private` after use.

---

## HIGH Findings

### AUDIT-HIGH-1 — Relay cell digest verification always fails for real Tor relay traffic

**File:** `pkg/circuit/circuit.go:686–693` (`verifyRelayCellDigest`)  
**Related:** `pkg/circuit/circuit.go:1054–1064` (`SendRelayCell`)

**Root cause — sender (`SendRelayCell`):**
```go
// 1. Write cell (with digest=0) into hash → hash state becomes H(prev+cell)
exitHop.ForwardDigest.Write(cellCopy)
// 2. Read post-cell hash state → H(prev+cell)
digestSum := exitHop.ForwardDigest.Sum(nil)
// 3. Put H(prev+cell)[:4] into cell's digest field
payload[5] = digestSum[0]
```

**Root cause — receiver (`verifyRelayCellDigest`):**
```go
// Gets H(prev) — pre-cell state — BEFORE writing the current cell
expectedSum := hop.BackwardDigest.Sum(nil)
expected := [4]byte{expectedSum[0], expectedSum[1], expectedSum[2], expectedSum[3]}
// Compares H(prev)[:4] against H(prev+cell)[:4] from sender → NEVER MATCHES
if subtle.ConstantTimeCompare(expected[:], cellDigest[:]) == 1 && recognized == 0 {
```

The sender computes the digest as SHA1(prev\_cells + current\_cell), which is the correct per-spec behavior (tor-spec.txt §6.1). The receiver checks SHA1(prev\_cells) — without including the current cell — and compares against the cell's digest field. These never match.

**Effect:** `verifyRelayCellDigest` always returns `hopIdx = -1`. `DeliverRelayCell` silently discards all incoming relay cells (line 1147: "Silently drop unrecognized cells"). All circuits appear to establish (no errors) but no data is ever delivered. Every SOCKS connection times out waiting for `RELAY_CONNECTED`.

**Remediation:** Accumulate the cell into a cloned hash state before comparing:
```go
// Clone hash state (sha1 supports encoding/decoding for cloning)
hashCopy := cloneHash(hop.BackwardDigest)
hashCopy.Write(cellCopy)
sum := hashCopy.Sum(nil)
// Compare sum[:4] with cellDigest
// Only if match: hop.BackwardDigest.Write(cellCopy) to advance real state
```

---

### AUDIT-HIGH-2 — Certificate pinning at TLS level is a complete no-op

**File:** `pkg/connection/connection.go:153–210`

The function `verifyRelayIdentityPinning` is invoked as a TLS `VerifyPeerCertificate` callback. It parses the certificate for structural validity but never compares `expectedIdentity` or `expectedFingerprint` against any field of the certificate.

```go
// From verifyRelayIdentityPinning — the function ends without any comparison:
// Function returns nil always (accepts every certificate regardless of identity)
```

The `Config` struct (lines 76–78) carries `ExpectedIdentity`, `ExpectedFingerprint`, and `RequireCERTS`, and `builder.go` populates these fields — but the TLS-level verification ignores them entirely.

**Note:** `pkg/protocol/protocol.go` (`receiveCERTS`) does check `ExpectedIdentity`/`ExpectedFingerprint` against CERTS cell contents (lines 291–311) and can fail hard when `RequireCERTS = true`. However, this operates at the application protocol layer, not the TLS layer, and only fires if the relay actually sends a CERTS cell.

**Effect:** A TLS man-in-the-middle that presents any valid X.509 certificate (self-signed or otherwise) will be accepted at the TLS level.

**Remediation:** Implement `verifyRelayIdentityPinning` to compute the SHA-256 fingerprint of the presented DER certificate and compare against `expectedFingerprint`; verify the public key matches `expectedIdentity`.

---

### AUDIT-HIGH-3 — CERTS cell chain validation does not verify type-4 cert against identity key

**File:** `pkg/protocol/certs.go:462–467`

Per cert-spec.txt, a relay's type-4 (Ed25519 signing key) certificate must be signed by the relay's long-term Ed25519 identity key, which is cross-certified in the type-7 certificate. The implementation verifies type-4 as self-signed:

```go
case CertTypeEd25519Signing:
    // Verifies cert is signed by its OWN certified key (self-signed check)
    if err := cert.Ed25519Cert.VerifySignature(cert.Ed25519Cert.CertifiedKey); err != nil {
```

This allows any attacker to present a forged, self-signed type-4 certificate that passes the signature check without possessing the relay's true identity key.

**Remediation:** Locate the type-7 (cross-certification) certificate in the CERTS cell, extract the identity key from it, and verify the type-4 cert's signature against that identity key. Verify the type-7 cert using the RSA identity key from the type-2 cert.

---

### AUDIT-HIGH-4 — Consensus validation failure silently swallowed

**File:** `pkg/directory/directory.go:272–277`

```go
if err := ValidateConsensusMetadata(meta); err != nil {
    d.logger.Warn("Consensus metadata validation failed", "error", err)
    // TODO: This should be a hard error for security in production
}
```

`ValidateConsensusMetadata` checks timestamp validity, signature count threshold, and authority count. When it fails, the client continues with the potentially invalid/expired/insufficient consensus. An adversary serving a crafted consensus (wrong timestamps, too few authority signatures) would not be rejected.

**Remediation:** Return an error from the caller (`FetchConsensus`) when `ValidateConsensusMetadata` fails, preventing the use of invalid consensus documents.

---

### AUDIT-HIGH-5 — SOCKS handler ignores parent context for circuit acquisition

**File:** `pkg/socks/socks.go:532`

```go
// BUG: context.Background() used instead of the passed `ctx` parameter
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
```

The `handleConnection(ctx, conn)` function receives a server lifecycle context but creates a detached `context.Background()` for circuit pool acquisition. Server shutdown or graceful stop will not interrupt in-flight circuit acquisitions, causing goroutine leaks until the 10-second timeout expires.

**Remediation:** Replace `context.Background()` with the passed `ctx`:
```go
timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
```

---

## MEDIUM Findings

### AUDIT-MED-1 — `VerifyDigest` public API uses pre-cell hash state (dead code but misleading)

**File:** `pkg/circuit/circuit.go:504–511`

```go
// Gets pre-cell state — same bug as verifyRelayCellDigest
expectedSum := digest.Sum(nil)
expected := [4]byte{expectedSum[0], expectedSum[1], expectedSum[2], expectedSum[3]}
```

`VerifyDigest` is not called anywhere in the codebase (internal verification is done by `verifyRelayCellDigest`). However, it is an exported API with the same pre-cell digest state bug. Any external consumer would get a function that always returns an error for valid cells.

**Remediation:** Fix to clone hash state, write cell, then compare — or remove if intended to remain internal.

---

### AUDIT-MED-2 — `OpenStream` uses `context.Background()`, ignores caller's context

**File:** `pkg/circuit/circuit.go:1234`

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
```

`OpenStream` is called from the SOCKS proxy with a connection-scoped context. If the SOCKS client disconnects, the circuit-level `RELAY_BEGIN`/`RELAY_CONNECTED` exchange continues for up to 30 seconds, holding resources.

**Remediation:** Accept `ctx context.Context` as a parameter (or use the circuit's existing context if one is stored) instead of `context.Background()`.

---

### AUDIT-MED-3 — Circuit connection readiness uses a fixed 100ms timing guess

**File:** `pkg/circuit/builder.go:207–216`

```go
case <-time.After(100 * time.Millisecond):
    // Connection should be ready by now
```

The builder waits 100ms for a TLS connection to become "ready" rather than waiting for an actual state transition (e.g., polling `conn.State()` or using a ready channel). On slow links or under load this can race with TLS handshake completion; on fast loopback it wastes 100ms.

**Remediation:** Add a `Ready() <-chan struct{}` channel to `Connection` that is closed when the state transitions to `StateOpen`, and block on that channel.

---

### AUDIT-MED-4 — `DeliverRelayCell` channel-send timeout uses `time.After` (minor goroutine pressure)

**File:** `pkg/circuit/circuit.go:1213–1215`

```go
case <-time.After(100 * time.Millisecond):
    return fmt.Errorf("relay receive channel full or blocked")
```

Each `time.After` call allocates a timer and channel that is not collected until the timer fires. In a busy circuit processing many cells per second, this creates steady goroutine/GC pressure. Additionally, when the channel is full, cells are silently dropped instead of applying back-pressure.

**Remediation:** Store a `*time.Timer` on the `Circuit` struct and `Reset()` it, or use `context.WithTimeout` on the circuit's own context.

---

### AUDIT-MED-5 — `encryptForward` comment contradicts actual (correct) encryption order

**File:** `pkg/circuit/circuit.go:569–579`

```go
// Comment says: "Encrypt with each hop's cipher in forward order (guard -> middle -> exit)"
// Actual loop: for i := len(hops) - 1; i >= 0; i-- // exit → middle → guard
```

The **code is correct** for onion encryption (innermost layer applied first, guard's layer applied last). The comment is wrong, which could mislead future maintainers into "fixing" it to the wrong order.

**Remediation:** Update comment to: "Apply layers innermost-first (exit → middle → guard) so guard's layer is outermost."

---

## LOW Findings

### AUDIT-LOW-1 — Replay protection uses 16-byte SHA-256 truncation

**File:** `pkg/cell/replay.go:98–100`

```go
// Uses first 16 bytes of SHA-256 (128-bit security for collision resistance)
digest := sha256.Sum256(payload)
return digest[:16]
```

128-bit collision resistance is adequate under current threat models but is weaker than the full 32-byte SHA-256 digest. Birthday attacks on the replay cache cost 2^64 operations rather than 2^128.

**Remediation:** Consider using the full 32-byte digest, or document the deliberate 16-byte choice.

---

### AUDIT-LOW-2 — EXTEND2 hard-codes `127.0.0.1:0` as IPv4 link specifier fallback

**File:** `pkg/circuit/extension.go:257–258`

```go
data = append(data, 127, 0, 0, 1) // IPv4 (placeholder)
data = append(data, 0, 0)          // Port (placeholder)
```

When relay keys aren't provided, the EXTEND2 cell is built with loopback as the target address and port 0. A relay receiving this EXTEND2 will attempt to connect to localhost, which will fail and return DESTROY. This is logged as a warning but not an error.

**Remediation:** Parse the target address string into proper IPv4/IPv6 link specifiers, or return an error when target address is unavailable.

---

### AUDIT-LOW-3 — `CERTS` cell failure is non-fatal in `PerformHandshake`

**File:** `pkg/protocol/protocol.go:80–84`

```go
if err := h.receiveCERTS(ctx); err != nil {
    // Log warning but don't fail - CERTS authentication is optional for now
    h.logger.Warn("CERTS cell handling failed", "error", err)
}
```

Per tor-spec.txt §4.2, CERTS exchange is mandatory for link protocol v3+. The implementation accepts connections where CERTS fails even when `RequireCERTS = true` is set on the connection (the RequireCERTS flag is checked inside `receiveCERTS`, but the error it returns is then discarded here).

**Remediation:** Propagate the error returned by `receiveCERTS` when the negotiated link protocol version requires it (≥3).

---

### AUDIT-LOW-4 — TAP handshake uses random bytes instead of RSA-encrypted data

**File:** `pkg/circuit/extension.go:225–228`

```go
// TAP handshake: PK_ID (16 bytes) || Symmetric key material (128 bytes)
// This is legacy and simplified
data := make([]byte, 144)
rand.Read(data)
```

The TAP handshake per tor-spec.txt §5.1.3 requires RSA-encrypted key material. The current implementation sends random bytes, which any relay will reject with DESTROY. Since TAP is deprecated, this is low impact, but a deprecation warning is already logged.

---

### AUDIT-LOW-5 — `ValidateConsensusMetadata` signature count check uses wrong threshold

**File:** `pkg/directory/directory.go:718–720`

```go
if meta.SignatureCount < minSignatureThreshold {
    return fmt.Errorf("insufficient signatures: %d < %d", meta.SignatureCount, minSignatureThreshold)
}
```

The constant `minSignatureThreshold` is defined (check value) but there's no verification that the `SignatureCount` field is actually populated from the parsed consensus. The consensus parser (`parseMicrodescConsensus`) may not populate `SignatureCount` or `Signatures` for microdesc consensus documents, causing the check to trivially pass at 0 signatures vs. 0 threshold.

---

## Summary Table

| ID | Severity | Package | File | Description |
|----|----------|---------|------|-------------|
| AUDIT-CRIT-1 | CRITICAL | pkg/crypto | crypto.go:368 | NtorClientHandshake returns private key as shared secret |
| AUDIT-HIGH-1 | HIGH | pkg/circuit | circuit.go:687 | Relay cell digest verification always fails (pre-cell vs post-cell state) |
| AUDIT-HIGH-2 | HIGH | pkg/connection | connection.go:153 | Certificate pinning (TLS level) is a complete no-op |
| AUDIT-HIGH-3 | HIGH | pkg/protocol | certs.go:466 | Type-4 CERTS chain not verified against type-7 identity key |
| AUDIT-HIGH-4 | HIGH | pkg/directory | directory.go:272 | Consensus validation failure silently swallowed |
| AUDIT-HIGH-5 | HIGH | pkg/socks | socks.go:532 | Parent context ignored; context.Background() used for circuit acquisition |
| AUDIT-MED-1 | MEDIUM | pkg/circuit | circuit.go:505 | VerifyDigest public API has same pre-cell hash bug (dead code) |
| AUDIT-MED-2 | MEDIUM | pkg/circuit | circuit.go:1234 | OpenStream uses context.Background(); ignores caller context |
| AUDIT-MED-3 | MEDIUM | pkg/circuit | builder.go:207 | Connection readiness uses 100ms timing hack, not state check |
| AUDIT-MED-4 | MEDIUM | pkg/circuit | circuit.go:1213 | DeliverRelayCell time.After per-call creates GC pressure |
| AUDIT-MED-5 | MEDIUM | pkg/circuit | circuit.go:569 | encryptForward comment contradicts correct loop direction |
| AUDIT-LOW-1 | LOW | pkg/cell | replay.go:98 | Replay digest truncated to 16 bytes (128-bit security) |
| AUDIT-LOW-2 | LOW | pkg/circuit | extension.go:257 | EXTEND2 hardcodes 127.0.0.1:0 as link specifier placeholder |
| AUDIT-LOW-3 | LOW | pkg/protocol | protocol.go:80 | CERTS cell failure non-fatal even when RequireCERTS=true |
| AUDIT-LOW-4 | LOW | pkg/circuit | extension.go:225 | TAP handshake sends random bytes instead of RSA-encrypted data |
| AUDIT-LOW-5 | LOW | pkg/directory | directory.go:718 | SignatureCount may not be populated in microdesc consensus |

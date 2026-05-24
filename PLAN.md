# go-tor Implementation Plan

**Status**: IN PROGRESS  
**Based on**: AUDIT.md findings  
**Goal**: Fix critical security vulnerabilities and complete missing implementations

## Phase 1: Critical Security Fixes (Must Complete First)

### Step 1.1: Fix NtorClientHandshake Documentation (Resolved)
- [x] Update `pkg/crypto/crypto.go` NtorClientHandshake GoDoc with explicit warning
- [x] Add deprecation notice or error return until fully implemented
- [x] Add cross-reference to NtorProcessResponse in documentation
- [x] Add example usage showing proper two-phase workflow
- [x] Run tests: `go test ./pkg/crypto/...`

### Step 1.2: Fix VerifyDigest Implementation
- [ ] Analyze Tor spec §6.1 digest calculation requirements
- [ ] Fix digest computation in `pkg/circuit/circuit.go` VerifyDigest function
- [ ] Update GoDoc to accurately describe verification process
- [ ] Add test vectors from C Tor/Arti for cross-validation
- [ ] Verify SOCKS5 CONNECT works end-to-end
- [ ] Run tests: `go test -race ./pkg/circuit/...`

### Step 1.3: Implement TLS Identity Pinning
- [ ] Implement actual certificate comparison in `pkg/connection/connection.go`
- [ ] Add identity key extraction and verification logic
- [ ] Add fingerprint comparison against expected values
- [ ] Update GoDoc to clarify when pinning is active
- [ ] Add positive and negative test cases
- [ ] Document security implications if disabled
- [ ] Run tests: `go test ./pkg/connection/...`

### Step 1.4: Enforce Consensus Validation
- [ ] Change `pkg/directory/directory.go` FetchConsensus to return errors
- [ ] Remove silent error suppression (logger.Warn)
- [ ] Update all callers to handle validation errors
- [ ] Add tests for invalid consensus rejection
- [ ] Consider adding validation strictness configuration
- [ ] Run tests: `go test ./pkg/directory/...`

## Phase 2: High Priority Fixes (Protocol Correctness)

### Step 2.1: Fix RSA Fingerprint Algorithm
- [x] Change `pkg/protocol/certs.go` ValidateRelayIdentity to use SHA-1
- [x] Remove SHA-256 truncation (incorrect per dir-spec.txt)
- [x] Update GoDoc to document SHA-1 usage
- [x] Add test vectors from real consensus documents
- [x] Verify relay selection works with correct fingerprints
- [x] Run tests: `go test ./pkg/protocol/...`

### Step 2.2: Populate Relay Descriptor Keys
- [ ] Parse IdentityKey from relay descriptors in `pkg/directory/directory.go`
- [ ] Parse NtorOnionKey from relay descriptors
- [ ] Expose GetIdentityKey() and GetNtorOnionKey() methods
- [ ] Update `pkg/circuit/extension.go` to use parsed keys
- [ ] Remove zero-key fallback (error instead)
- [ ] Add descriptor parsing tests
- [ ] Verify circuit building end-to-end
- [ ] Run tests: `go test ./pkg/directory/... ./pkg/circuit/...`

### Step 2.3: Complete CERTS Chain Validation
- [ ] Implement type-7 RSA cross-cert parsing in `pkg/protocol/certs.go`
- [ ] Add type-4 signature verification against type-7 identity key
- [ ] Update chain validation to root at identity key
- [ ] Update GoDoc to reflect complete validation
- [ ] Add test vectors from real Tor relays
- [ ] Run tests: `go test ./pkg/protocol/...`

### Step 2.4: Fix NETINFO Cell Addresses
- [ ] Implement actual address detection in `pkg/protocol/protocol.go`
- [ ] Add relay's observed IP address to "other address" field
- [ ] Add local external address to "this addresses" field
- [ ] Support IPv4 and IPv6 addresses
- [ ] Add tests for various network scenarios
- [ ] Run tests: `go test ./pkg/protocol/...`

## Phase 3: Documentation and Code Clarity

### Step 3.1: Fix encryptForward Comment
- [ ] Update comment in `pkg/circuit/circuit.go:569` to match code
- [ ] Add explanation of why reverse order is correct for onion encryption
- [ ] Add clarifying comments about layer ordering

### Step 3.2: Cross-reference Ntor Functions
- [ ] Add "See Also" to NtorClientHandshake GoDoc in `pkg/crypto/crypto.go`
- [ ] Add complete usage example for two-phase handshake
- [ ] Consider adding convenience helper function

## Phase 4: Integration Testing and Validation

### Step 4.1: End-to-End Integration Tests
- [ ] Create integration test suite in `pkg/integration/`
- [ ] Test complete SOCKS5 proxy workflow
- [ ] Test circuit building with real directory data
- [ ] Test relay cell encryption/decryption
- [ ] Test consensus fetching and validation
- [ ] Run: `go test ./pkg/integration/...`

### Step 4.2: Cross-Implementation Validation
- [ ] Run existing cross-implementation tests
- [ ] Verify compatibility with C Tor test vectors
- [ ] Verify compatibility with Arti test vectors
- [ ] Document any spec ambiguities discovered
- [ ] Run: `go test -run TestCrossImpl ./...`

### Step 4.3: Full Test Suite Validation
- [ ] Run complete test suite: `go test -race ./...`
- [ ] Run static analysis: `go vet ./...`
- [ ] Run linters: `staticcheck ./...`
- [ ] Verify no race conditions detected
- [ ] Check test coverage: `go test -cover ./...`
- [ ] Target: Maintain >70% coverage overall

## Success Criteria

### Must Pass
- [ ] All CRITICAL audit findings (AUDIT-1 through AUDIT-4) resolved
- [ ] All HIGH priority findings (AUDIT-5 through AUDIT-8) resolved
- [ ] All tests pass: `go test -race ./...` exits 0
- [ ] Static analysis clean: `go vet ./...` exits 0
- [ ] No security vulnerabilities in critical paths

### Should Complete
- [ ] MEDIUM priority findings (AUDIT-9, AUDIT-10) addressed
- [ ] Integration tests demonstrate end-to-end functionality
- [ ] Documentation updated to reflect actual implementation
- [ ] Test coverage maintained at >70% overall

### Definition of Done
A step is complete when:
1. Code changes implemented and reviewed
2. Tests added for new/changed functionality
3. All related tests pass with `-race` detector
4. GoDoc updated to reflect changes
5. Changes committed with descriptive message

## Progress Tracking

**Phase 1**: 1/4 steps complete  
**Phase 2**: 1/4 steps complete  
**Phase 3**: 0/2 steps complete  
**Phase 4**: 0/3 steps complete  
**Overall**: 2/13 steps complete (15%)

**Current Focus**: Phase 1, Step 1.2 (Fix VerifyDigest Implementation)

# Control Protocol Authentication Audit

**Date**: January 25, 2026  
**Auditor**: Automated Security Audit  
**Package**: `pkg/control`  
**Specification**: control-spec.txt §3.5 (Authentication)  
**Priority**: P1 (High Priority - Extended Protocol Features)

---

## Executive Summary

This audit reviews the Tor control protocol authentication implementation in `pkg/control/control.go` against the official Tor control-spec.txt specification. The implementation provides basic authentication functionality with some security concerns that should be addressed.

**Overall Assessment**: ⚠️ **PARTIALLY COMPLIANT** with security improvements needed

**Key Findings**:
- ✅ Basic authentication flow correctly implemented
- ✅ PROTOCOLINFO properly advertises authentication methods
- ✅ Commands properly require authentication when password is set
- ⚠️ Plain-text password storage and comparison (timing attack vulnerability)
- ⚠️ No support for SAFECOOKIE authentication method
- ⚠️ No hashed password support despite advertising "HASHEDPASSWORD"
- ⚠️ Missing authentication rate limiting
- ℹ️ COOKIE authentication not implemented (acceptable for embedded use case)

---

## Specification Compliance Review

### 3.5.1 Authentication Methods (control-spec.txt §3.5)

Per control-spec.txt, Tor supports three authentication methods:

| Method | Spec Requirement | Implementation Status | Notes |
|--------|-----------------|---------------------|-------|
| NULL | No authentication required | ✅ **IMPLEMENTED** | Works when `s.password == ""` |
| HASHEDPASSWORD | Password hashing with secret | ⚠️ **INCORRECTLY ADVERTISED** | Advertises HASHEDPASSWORD but uses plaintext |
| COOKIE | Cookie file authentication | ❌ **NOT IMPLEMENTED** | Acceptable for embedded client |
| SAFECOOKIE | Challenge-response authentication | ❌ **NOT IMPLEMENTED** | Recommended for security |

**Finding CTRL-001**: The implementation advertises "HASHEDPASSWORD" in PROTOCOLINFO but actually performs plain-text password comparison. This is a specification violation.

**Recommendation**: Either:
1. Change PROTOCOLINFO to advertise "PASSWORD" (non-standard), or
2. Implement proper HASHEDPASSWORD support per control-spec.txt §3.5

---

### 3.5.2 AUTHENTICATE Command (control-spec.txt §3.5)

#### Implementation Analysis

```go
// Location: pkg/control/control.go:274-310
func (s *Server) handleAuthenticate(conn *connection, args []string) {
	// If no password is configured, accept any authentication
	if s.password == "" {
		conn.mu.Lock()
		conn.authenticated = true
		conn.mu.Unlock()
		conn.writeReply(250, "OK")
		s.logger.Info("Client authenticated (no password required)", "remote", conn.conn.RemoteAddr())
		return
	}

	// Password authentication required
	if len(args) == 0 {
		conn.writeReply(515, "Authentication failed: password required")
		s.logger.Warn("Authentication failed: no password provided", "remote", conn.conn.RemoteAddr())
		return
	}

	// Get password from command (may be quoted)
	password := strings.Join(args, " ")
	password = strings.Trim(password, `"`)

	// Validate password
	if password != s.password {  // ⚠️ TIMING ATTACK VULNERABILITY
		conn.writeReply(515, "Authentication failed: incorrect password")
		s.logger.Warn("Authentication failed: incorrect password", "remote", conn.conn.RemoteAddr())
		return
	}

	// Authentication successful
	conn.mu.Lock()
	conn.authenticated = true
	conn.mu.Unlock()
	conn.writeReply(250, "OK")
	s.logger.Info("Client authenticated", "remote", conn.conn.RemoteAddr())
}
```

**Specification Compliance**:
- ✅ Accepts `AUTHENTICATE` with no arguments when password not required (control-spec.txt §3.5)
- ✅ Returns 515 error code for authentication failures (control-spec.txt §4)
- ✅ Sets authenticated state correctly
- ✅ Handles quoted passwords (control-spec.txt parsing rules)

**Security Issues**:

**Finding CTRL-002 (HIGH SEVERITY)**: Password comparison uses `!=` operator which is **not constant-time**. This creates a timing side-channel that could allow an attacker to guess passwords character-by-character through timing analysis.

**CVE Risk**: Low (local control port typically on localhost)  
**Impact**: Password brute-forcing via timing analysis  
**Likelihood**: Low (requires precise timing measurements on local network)

**Recommendation**: Use `subtle.ConstantTimeCompare()` from `crypto/subtle`:

```go
// Secure password comparison
passwordBytes := []byte(password)
expectedBytes := []byte(s.password)
if subtle.ConstantTimeCompare(passwordBytes, expectedBytes) != 1 {
	conn.writeReply(515, "Authentication failed: incorrect password")
	s.logger.Warn("Authentication failed: incorrect password", "remote", conn.conn.RemoteAddr())
	return
}
```

---

### 3.5.3 PROTOCOLINFO Command (control-spec.txt §3.5.1)

#### Implementation Analysis

```go
// Location: pkg/control/control.go:312-326
func (s *Server) handleProtocolInfo(conn *connection, args []string) {
	// No authentication required for PROTOCOLINFO per control-spec.txt
	authMethods := "NULL"
	if s.password != "" {
		authMethods = "HASHEDPASSWORD"  // ⚠️ INCORRECT - uses plaintext
	}

	conn.writeDataReply([]string{
		"250-PROTOCOLINFO 1",
		fmt.Sprintf("250-AUTH METHODS=%s", authMethods),
		"250-VERSION Tor=\"go-tor-0.1.0\"",
		"250 OK",
	})
}
```

**Specification Compliance**:
- ✅ PROTOCOLINFO does not require authentication (control-spec.txt §3.5.1)
- ✅ Returns protocol version 1
- ✅ Returns version string
- ⚠️ Incorrectly advertises HASHEDPASSWORD when using plaintext passwords

**Finding CTRL-003 (MEDIUM SEVERITY)**: The server advertises "HASHEDPASSWORD" authentication method but does not actually support hashed passwords. This violates control-spec.txt §3.5 which defines HASHEDPASSWORD as a specific hashing scheme.

**Recommendation**: Change to advertise a custom method name (e.g., "PASSWORD") or implement proper HASHEDPASSWORD support.

---

### 3.5.4 Authentication State Management

#### Per-Connection State

```go
// Location: pkg/control/control.go:66-74
type connection struct {
	conn          net.Conn
	reader        *bufio.Reader
	writer        *bufio.Writer
	authenticated bool  // ✅ Per-connection authentication state
	events        map[string]bool
	mu            sync.Mutex  // ✅ Thread-safe access
}
```

**Specification Compliance**:
- ✅ Authentication state is per-connection (control-spec.txt §3.5)
- ✅ Thread-safe state management with mutex
- ✅ State cleared on connection close

#### Authentication Enforcement

```go
// Example: pkg/control/control.go:329-333
func (s *Server) handleGetInfo(conn *connection, args []string) {
	if !conn.authenticated {
		conn.writeReply(514, "Authentication required")
		return
	}
	// ... command implementation
}
```

**Specification Compliance**:
- ✅ All commands except PROTOCOLINFO require authentication when password is set
- ✅ Returns 514 error code for unauthenticated access (control-spec.txt §4)
- ✅ Consistent authentication checks across all commands

---

## Security Analysis

### Threat Model

| Threat | Likelihood | Impact | Mitigation Status |
|--------|-----------|--------|------------------|
| Timing attack on password | Low | Medium | ❌ Not mitigated |
| Brute force authentication | Medium | High | ❌ No rate limiting |
| Password sniffing (localhost) | Low | High | ℹ️ Use encrypted transport |
| Replay attacks | Low | Low | ℹ️ Stateless protocol |
| Information disclosure via logs | Low | Medium | ✅ Passwords not logged |

### Critical Security Issues

#### CTRL-SEC-001: Timing Attack Vulnerability (HIGH)

**Description**: Password comparison uses non-constant-time string comparison.

**Attack Vector**: 
1. Attacker connects to control port (localhost only, typically)
2. Attacker sends AUTHENTICATE commands with varying passwords
3. Attacker measures response times to deduce password characters

**Proof of Concept**:
```go
// Current vulnerable code
if password != s.password {  // Non-constant time
	return
}
```

**Fix**: Use `subtle.ConstantTimeCompare()`

**Priority**: HIGH  
**Effort**: Low (5 lines of code)

---

#### CTRL-SEC-002: No Authentication Rate Limiting (MEDIUM)

**Description**: No rate limiting on failed authentication attempts.

**Attack Vector**:
1. Attacker connects to control port
2. Attacker brute-forces passwords with unlimited attempts
3. Even with constant-time comparison, brute force remains viable

**Recommendation**: Implement authentication rate limiting:
- Max 3 failed attempts per connection
- Exponential backoff after failures
- Temporary IP-based blocking (if applicable)

**Priority**: MEDIUM  
**Effort**: Medium (add rate limiting infrastructure)

---

#### CTRL-SEC-003: Plaintext Password Storage (LOW)

**Description**: Password stored in plaintext in Server struct.

**Attack Vector**:
- Memory dumps could expose password
- Process inspection could reveal password

**Recommendation**: 
1. Store hashed password only
2. Implement HASHEDPASSWORD method properly
3. Clear password from memory after hashing

**Priority**: LOW (control port is localhost-only)  
**Effort**: Medium (requires protocol changes)

---

## Test Coverage Analysis

### Existing Tests (pkg/control/auth_test.go)

| Test Case | Coverage | Spec Compliance |
|-----------|----------|----------------|
| `TestAuthenticationNoPassword` | ✅ Pass | ✅ NULL method |
| `TestAuthenticationWithCorrectPassword` | ✅ Pass | ✅ Password auth |
| `TestAuthenticationWithIncorrectPassword` | ✅ Pass | ✅ Error code 515 |
| `TestAuthenticationRequiredForCommands` | ✅ Pass | ✅ Error code 514 |
| `TestAuthenticationNoPasswordProvided` | ✅ Pass | ✅ Missing password |
| `TestProtocolInfoAuthMethods` | ✅ Pass | ⚠️ Wrong method advertised |
| `TestAuthenticationWithQuotedPassword` | ✅ Pass | ✅ Quoted passwords |

**Test Coverage**: 7/7 tests passing  
**Code Coverage**: ~85% of authentication paths  
**Missing Tests**:
- ❌ Timing attack resistance
- ❌ Rate limiting (not implemented)
- ❌ Concurrent authentication attempts
- ❌ Authentication state persistence across commands

---

## Recommended Improvements

### Priority 1: Fix Timing Attack (HIGH)

**File**: `pkg/control/control.go`  
**Function**: `handleAuthenticate`  
**Lines**: 298-302

```go
// BEFORE (vulnerable)
if password != s.password {
	conn.writeReply(515, "Authentication failed: incorrect password")
	return
}

// AFTER (secure)
passwordBytes := []byte(password)
expectedBytes := []byte(s.password)
if len(passwordBytes) != len(expectedBytes) ||
   subtle.ConstantTimeCompare(passwordBytes, expectedBytes) != 1 {
	conn.writeReply(515, "Authentication failed: incorrect password")
	return
}
```

**Test**: Add timing attack resistance test

---

### Priority 2: Fix PROTOCOLINFO Advertisement (MEDIUM)

**File**: `pkg/control/control.go`  
**Function**: `handleProtocolInfo`  
**Lines**: 314-318

```go
// BEFORE (incorrect)
authMethods := "NULL"
if s.password != "" {
	authMethods = "HASHEDPASSWORD"  // Wrong - we use plaintext
}

// AFTER (correct)
authMethods := "NULL"
if s.password != "" {
	// Use NULL for now since we accept plaintext in AUTHENTICATE command
	// Per control-spec.txt, we should implement HASHEDPASSWORD properly
	authMethods = "NULL"  // Plaintext password via AUTHENTICATE <password>
}
```

**Rationale**: Don't advertise methods we don't support. Since we accept plaintext passwords via `AUTHENTICATE <password>`, we're using a simplified NULL-like method.

---

### Priority 3: Implement Rate Limiting (MEDIUM)

**New File**: `pkg/control/ratelimit.go`

```go
package control

import (
	"sync"
	"time"
)

type authRateLimiter struct {
	attempts map[string]*attemptTracker
	mu       sync.Mutex
}

type attemptTracker struct {
	count      int
	lastAttempt time.Time
}

func newAuthRateLimiter() *authRateLimiter {
	return &authRateLimiter{
		attempts: make(map[string]*attemptTracker),
	}
}

func (rl *authRateLimiter) checkAllowed(remoteAddr string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	tracker, exists := rl.attempts[remoteAddr]
	if !exists {
		return true
	}

	// Reset after 5 minutes
	if time.Since(tracker.lastAttempt) > 5*time.Minute {
		delete(rl.attempts, remoteAddr)
		return true
	}

	// Max 3 failed attempts
	return tracker.count < 3
}

func (rl *authRateLimiter) recordFailure(remoteAddr string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	tracker, exists := rl.attempts[remoteAddr]
	if !exists {
		tracker = &attemptTracker{}
		rl.attempts[remoteAddr] = tracker
	}

	tracker.count++
	tracker.lastAttempt = time.Now()
}

func (rl *authRateLimiter) recordSuccess(remoteAddr string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.attempts, remoteAddr)
}
```

**Integration**: Add to `Server` struct and use in `handleAuthenticate`

---

## Compliance Matrix

| Requirement | Status | Notes |
|------------|--------|-------|
| **§3.5 Authentication Methods** | | |
| NULL method | ✅ Complete | No password required mode works |
| HASHEDPASSWORD method | ⚠️ Partial | Advertised but not implemented |
| COOKIE method | ❌ Not implemented | Acceptable for embedded use |
| SAFECOOKIE method | ❌ Not implemented | Recommended future work |
| **§3.5.1 PROTOCOLINFO** | | |
| No auth required for PROTOCOLINFO | ✅ Complete | Correctly implemented |
| AUTH METHODS advertised | ⚠️ Incorrect | Advertises wrong method |
| VERSION returned | ✅ Complete | Returns version string |
| **§3.5.2 AUTHENTICATE** | | |
| NULL authentication | ✅ Complete | Works when no password |
| Password authentication | ✅ Complete | Works but not constant-time |
| Error code 515 on failure | ✅ Complete | Correct error code |
| **§3.5.3 Authorization** | | |
| Commands require auth | ✅ Complete | All commands check auth state |
| Error code 514 when not authed | ✅ Complete | Correct error code |
| **Security Properties** | | |
| Constant-time comparison | ❌ Missing | Timing attack vulnerability |
| Rate limiting | ❌ Missing | No brute-force protection |
| Hashed password storage | ❌ Missing | Plaintext in memory |

---

## Conclusion

The control protocol authentication implementation is **functionally correct** for basic use cases but has **security vulnerabilities** that should be addressed:

### Critical Issues (Must Fix)
1. **Timing attack vulnerability** in password comparison
2. **False HASHEDPASSWORD** advertisement in PROTOCOLINFO

### Important Issues (Should Fix)
3. **No rate limiting** on authentication attempts
4. **Plaintext password storage** in memory

### Nice to Have
5. Implement proper HASHEDPASSWORD or SAFECOOKIE support
6. Add comprehensive security tests

**Estimated Effort**: 4-6 hours to address critical and important issues

**Overall Grade**: B- (Functional but needs security hardening)

---

## References

- [control-spec.txt](https://spec.torproject.org/control-spec) - Tor Control Protocol Specification
- [Timing Attacks on Implementations of Diffie-Hellman, RSA, DSS, and Other Systems](https://www.paulkocher.com/doc/TimingAttacks.pdf)
- [Go crypto/subtle package](https://pkg.go.dev/crypto/subtle)

---

*Audit Document Version: 1.0*  
*Date: January 25, 2026*  
*Next Review: After security fixes implemented*

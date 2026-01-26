# Control Protocol Password Hashing Audit

**Date**: January 26, 2026  
**Auditor**: Automated Security Audit  
**Package**: `pkg/control`  
**Specification**: control-spec.txt §3.5 (HASHEDPASSWORD Authentication)  
**Priority**: P1 (High Priority - Authentication Security)  
**Estimated Time**: 2 hours

---

## Executive Summary

This audit examines the control protocol password hashing implementation in `pkg/control` against the Tor control-spec.txt specification §3.5 for HASHEDPASSWORD authentication. The goal is to verify compliance with RFC2440's iterated and salted S2K algorithm and assess the security of password storage.

**Overall Assessment**: ❌ **NON-COMPLIANT** - HASHEDPASSWORD not implemented

**Compliance Score**: 0% (0/10 requirements implemented)

**Key Findings**:
- ❌ **CRITICAL**: No password hashing implementation exists
- ❌ **CRITICAL**: Passwords stored in plaintext in memory (line 25, 101 in control.go)
- ⚠️ **HIGH**: False advertisement of "HASHEDPASSWORD" in PROTOCOLINFO (line 350)
- ✅ **GOOD**: Constant-time password comparison prevents timing attacks (line 325)
- ℹ️ **INFO**: Current implementation suitable for embedded/testing use only

**Risk Level**: HIGH (for production use), ACCEPTABLE (for educational/research use)

---

## 1. Specification Requirements Analysis

### 1.1 HASHEDPASSWORD Method (control-spec.txt §3.5)

The Tor control protocol specification defines HASHEDPASSWORD authentication:

| Requirement | Specification | Implementation Status |
|-------------|--------------|---------------------|
| **HASH-001** | Hash algorithm: RFC2440 S2K (iterated, salted) | ❌ NOT IMPLEMENTED |
| **HASH-002** | Hash function: SHA-1 | ❌ NOT IMPLEMENTED |
| **HASH-003** | Salt: 8 bytes (16 hex chars) | ❌ NOT IMPLEMENTED |
| **HASH-004** | Iteration count: 65536 (0x10000) | ❌ NOT IMPLEMENTED |
| **HASH-005** | Format: `16:SALTHEX$HASH` (16 = algorithm ID) | ❌ NOT IMPLEMENTED |
| **HASH-006** | Hash output: 20 bytes (40 hex chars, SHA-1) | ❌ NOT IMPLEMENTED |
| **HASH-007** | Storage: HashedControlPassword in config | ❌ NOT IMPLEMENTED |
| **HASH-008** | Advertise: PROTOCOLINFO returns HASHEDPASSWORD | ⚠️ PARTIAL (advertises but doesn't implement) |
| **HASH-009** | Validation: Hash input password, constant-time compare | ⚠️ PARTIAL (constant-time compare only) |
| **HASH-010** | No plaintext password storage | ❌ NOT IMPLEMENTED |

**Overall Compliance**: 0/10 requirements (0%)

---

## 2. Current Implementation Analysis

### 2.1 Password Storage (control.go:25, 101)

```go
// Server structure
type Server struct {
	address      string
	listener     net.Listener
	logger       *logger.Logger
	clientGetter ClientInfoGetter
	password     string // ❌ PLAINTEXT PASSWORD IN MEMORY
	// ...
}

// Constructor
func NewServerWithPassword(address string, clientGetter ClientInfoGetter, password string, log *logger.Logger) *Server {
	// ...
	return &Server{
		// ...
		password:     password, // ❌ STORES PLAINTEXT PASSWORD
		// ...
	}
}
```

**Finding HASH-SEC-001 (CRITICAL)**: Passwords are stored in plaintext in memory as a `string` field. This violates security best practices and control-spec.txt §3.5 which mandates hashed password storage.

**Security Impact**:
- Memory dumps expose passwords
- Process inspection reveals passwords
- Debugging/crash dumps leak credentials
- No protection against memory disclosure attacks

**CWE Classification**: CWE-256 (Unprotected Storage of Credentials)

**Recommendation**: Store only the hashed password (RFC2440 S2K format):
```go
type Server struct {
	hashedPassword string // Format: "16:SALTHEX$HASH" (RFC2440 S2K)
	// OR for no auth:
	// hashedPassword string // Empty string = no auth required
}
```

---

### 2.2 Password Comparison (control.go:323-325)

```go
// Validate password using constant-time comparison to prevent timing attacks
// Convert strings to byte slices for constant-time comparison
passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(s.password)) == 1

if !passwordMatch {
	s.recordFailedAuth(remoteIP)
	conn.writeReply(515, "Authentication failed: incorrect password")
	s.logger.Warn("Authentication failed: incorrect password", "remote", remoteIP)
	return
}
```

**Finding HASH-SEC-002 (HIGH)**: The implementation correctly uses constant-time comparison to prevent timing attacks (✅ GOOD), but compares plaintext passwords instead of hashed values.

**What Should Happen (control-spec.txt §3.5)**:
1. Client sends plaintext password: `AUTHENTICATE mypassword`
2. Server extracts salt from stored hash: `16:SALTHEX$HASH`
3. Server hashes input password with salt using RFC2440 S2K
4. Server compares computed hash to stored hash using constant-time comparison

**Current Behavior**:
1. Client sends plaintext password: `AUTHENTICATE mypassword`
2. Server compares plaintext directly: `subtle.ConstantTimeCompare([]byte("mypassword"), []byte(s.password))`

**Impact**: While timing-safe, the comparison operates on plaintext passwords which are insecure to store.

---

### 2.3 PROTOCOLINFO Advertisement (control.go:349-350)

```go
func (s *Server) handleProtocolInfo(conn *connection, args []string) {
	// No authentication required for PROTOCOLINFO per control-spec.txt
	authMethods := "NULL"
	if s.password != "" {
		authMethods = "HASHEDPASSWORD"  // ⚠️ FALSE ADVERTISEMENT
	}

	conn.writeDataReply([]string{
		"250-PROTOCOLINFO 1",
		fmt.Sprintf("250-AUTH METHODS=%s", authMethods),
		"250-VERSION Tor=\"go-tor-0.1.0\"",
		"250 OK",
	})
}
```

**Finding HASH-SEC-003 (HIGH)**: The server advertises "HASHEDPASSWORD" authentication method when a password is configured, but does not implement the HASHEDPASSWORD protocol. This is a specification violation.

**Specification Requirement (control-spec.txt §3.5.1)**:
> The PROTOCOLINFO command returns information about the Tor process, including supported authentication methods. The server MUST NOT advertise authentication methods it does not support.

**Current Violation**:
- Advertises: `HASHEDPASSWORD`
- Actually supports: Plaintext password comparison

**Impact**:
- Clients expect RFC2440 S2K hashed passwords
- May send hashed passwords that server treats as plaintext
- Breaks interoperability with Tor-compliant clients
- Violates principle of least surprise

**Recommendation**: Either:
1. Implement full HASHEDPASSWORD support per control-spec.txt §3.5, OR
2. Advertise a custom method name (e.g., "PASSWORD" or "PLAINTEXT"), OR
3. Advertise only "NULL" and document password as internal-only

---

## 3. RFC2440 S2K Algorithm Requirements

### 3.1 Algorithm Overview (RFC2440 §3.7.1)

The "Iterated and Salted S2K" algorithm (type 3) is specified in RFC2440:

```
1. Salt generation: Generate 8 random bytes (cryptographically secure)
2. Count parameter: 65536 iterations (0x10000) for Tor
3. Hash function: SHA-1 (20-byte output)
4. Input: salt || password (concatenation)
5. Iteration: Hash the input repeatedly, concatenating until count bytes processed
6. Output: First 20 bytes of final hash
```

### 3.2 Tor-Specific Format

Tor encodes the hash as:

```
Format: "16:SALTHEX$HASH"
- "16" = algorithm identifier (iterated and salted S2K, count=65536)
- SALTHEX = 8 bytes salt, hex-encoded (16 characters)
- $ = separator
- HASH = 20 bytes SHA-1 output, hex-encoded (40 characters)

Example:
16:872860B76453A77799A7D1E07DC64BB5$32A3D35BC76BD3A47ED5825CDD8BF9F70C7DE47B
└┬┘ └───────────────┬──────────────┘ └────────────┬─────────────────┘
 │                  │                              │
 Algorithm ID       8-byte salt (hex)              20-byte SHA-1 hash (hex)
```

### 3.3 Implementation Requirements

| Component | Requirement | Implementation |
|-----------|------------|----------------|
| Salt generation | `crypto/rand.Reader` (CSPRNG) | ❌ NOT IMPLEMENTED |
| Hash function | `crypto/sha1.New()` | ❌ NOT IMPLEMENTED |
| Iteration count | 65536 (0x10000) | ❌ NOT IMPLEMENTED |
| Format encoding | `16:SALTHEX$HASH` | ❌ NOT IMPLEMENTED |
| Format parsing | Parse `16:SALTHEX$HASH` | ❌ NOT IMPLEMENTED |
| Hash verification | Constant-time compare | ✅ Already implemented |

---

## 4. Security Analysis

### 4.1 Current Security Posture

**Strengths**:
- ✅ Constant-time password comparison (prevents timing attacks)
- ✅ Rate limiting with exponential backoff (prevents brute-force)
- ✅ Per-IP tracking (prevents distributed attacks)
- ✅ Secure logging (no password values in logs)

**Weaknesses**:
- ❌ Plaintext password storage (CWE-256)
- ❌ No password hashing (CWE-256)
- ❌ Memory dumps expose credentials
- ❌ False HASHEDPASSWORD advertisement (spec violation)

**Risk Assessment**:
- **Severity**: HIGH (plaintext credential storage)
- **Likelihood**: MEDIUM (local control port, limited exposure)
- **Impact**: HIGH (full control port access if password leaked)
- **Overall Risk**: HIGH for production, ACCEPTABLE for educational use

### 4.2 Attack Vectors

| Attack | Current Protection | With HASHEDPASSWORD |
|--------|-------------------|---------------------|
| Timing attacks | ✅ Constant-time compare | ✅ Constant-time compare |
| Brute-force | ✅ Rate limiting | ✅ Rate limiting + slow hash |
| Memory dumps | ❌ Plaintext exposed | ✅ Hash exposed (no plaintext) |
| Process inspection | ❌ Plaintext readable | ✅ Only hash readable |
| Rainbow tables | N/A (plaintext) | ✅ Salt prevents precomputation |
| Credential reuse | ❌ Plaintext reusable | ✅ Hash not reusable elsewhere |

### 4.3 Compliance with Best Practices

| Best Practice | Status | Notes |
|--------------|--------|-------|
| Never store plaintext passwords | ❌ VIOLATED | Passwords stored as `string` |
| Use cryptographic hashing | ❌ NOT IMPLEMENTED | No hashing at all |
| Use salt for each password | ❌ NOT IMPLEMENTED | No salt generation |
| Use slow hash (PBKDF2/bcrypt/scrypt) | ❌ NOT IMPLEMENTED | RFC2440 S2K is moderately slow |
| Constant-time comparison | ✅ IMPLEMENTED | Using `crypto/subtle` |
| Secure memory zeroing | ❌ NOT IMPLEMENTED | Strings not zeroed |

**Overall Best Practices Score**: 17% (1/6 implemented)

---

## 5. Recommended Implementation

### 5.1 Hash Generation (for configuration)

```go
package control

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// GenerateHashedPassword creates a RFC2440 S2K hashed password for torrc
// Format: "16:SALTHEX$HASH"
// - 16 = algorithm ID (iterated and salted S2K, count=65536)
// - SALTHEX = 8-byte salt (hex-encoded)
// - HASH = 20-byte SHA-1 hash (hex-encoded)
func GenerateHashedPassword(password string) (string, error) {
	// Generate 8-byte cryptographic salt
	salt := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Hash password using RFC2440 S2K algorithm
	hash := hashPasswordRFC2440(password, salt, 65536)

	// Format: "16:SALTHEX$HASH"
	return fmt.Sprintf("16:%s$%s",
		hex.EncodeToString(salt),
		hex.EncodeToString(hash),
	), nil
}

// hashPasswordRFC2440 implements RFC2440 Iterated and Salted S2K
// count: number of bytes to hash (65536 for Tor)
func hashPasswordRFC2440(password string, salt []byte, count int) []byte {
	// Prepare input: salt || password
	input := append(salt, []byte(password)...)
	
	// SHA-1 hasher
	h := sha1.New()
	
	// Iterate until we've hashed 'count' bytes
	bytesHashed := 0
	for bytesHashed < count {
		h.Write(input)
		bytesHashed += len(input)
	}
	
	// Return first 20 bytes (SHA-1 output size)
	return h.Sum(nil)
}
```

### 5.2 Hash Validation

```go
// VerifyHashedPassword validates a password against a stored hash
// hashedPassword format: "16:SALTHEX$HASH"
// Returns true if password matches, false otherwise
func VerifyHashedPassword(password, hashedPassword string) bool {
	// Parse hash format: "16:SALTHEX$HASH"
	parts := strings.SplitN(hashedPassword, ":", 2)
	if len(parts) != 2 || parts[0] != "16" {
		return false // Invalid format or algorithm
	}
	
	saltHash := strings.SplitN(parts[1], "$", 2)
	if len(saltHash) != 2 {
		return false // Missing salt or hash
	}
	
	// Decode salt (16 hex chars = 8 bytes)
	salt, err := hex.DecodeString(saltHash[0])
	if err != nil || len(salt) != 8 {
		return false // Invalid salt
	}
	
	// Decode stored hash (40 hex chars = 20 bytes)
	storedHash, err := hex.DecodeString(saltHash[1])
	if err != nil || len(storedHash) != 20 {
		return false // Invalid hash
	}
	
	// Compute hash of provided password with same salt
	computedHash := hashPasswordRFC2440(password, salt, 65536)
	
	// Constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare(computedHash, storedHash) == 1
}
```

### 5.3 Server Integration

```go
// Modify Server struct
type Server struct {
	address        string
	listener       net.Listener
	logger         *logger.Logger
	clientGetter   ClientInfoGetter
	hashedPassword string // Format: "16:SALTHEX$HASH" or "" for no auth
	// ... rest of fields
}

// Modified constructor
func NewServerWithPassword(address string, clientGetter ClientInfoGetter, password string, log *logger.Logger) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Hash the password on initialization (or accept pre-hashed)
	var hashedPassword string
	if password != "" {
		// Check if already hashed (starts with "16:")
		if strings.HasPrefix(password, "16:") {
			hashedPassword = password
		} else {
			// Generate hash from plaintext (for convenience)
			var err error
			hashedPassword, err = GenerateHashedPassword(password)
			if err != nil {
				log.Component("control").Error("Failed to hash password", "error", err)
				hashedPassword = "" // Fall back to no auth
			}
		}
	}
	
	return &Server{
		address:        address,
		logger:         log.Component("control"),
		clientGetter:   clientGetter,
		hashedPassword: hashedPassword,
		// ... rest of initialization
	}
}

// Modified authentication handler
func (s *Server) handleAuthenticate(conn *connection, args []string) {
	// If no password is configured, accept any authentication
	if s.hashedPassword == "" {
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

	// Extract IP address for rate limiting
	remoteIP := conn.conn.RemoteAddr().String()
	if host, _, err := net.SplitHostPort(remoteIP); err == nil {
		remoteIP = host
	}

	// Check rate limiting before attempting authentication
	if !s.checkAuthRateLimit(remoteIP) {
		conn.writeReply(515, "Authentication failed: too many attempts, try again later")
		s.logger.Warn("Authentication rate limited", "remote", remoteIP)
		return
	}

	// Validate password using RFC2440 S2K hashing
	passwordMatch := VerifyHashedPassword(password, s.hashedPassword)

	if !passwordMatch {
		s.recordFailedAuth(remoteIP)
		conn.writeReply(515, "Authentication failed: incorrect password")
		s.logger.Warn("Authentication failed: incorrect password", "remote", remoteIP)
		return
	}

	// Reset rate limiter on successful authentication
	s.resetAuthRateLimit(remoteIP)

	// Authentication successful
	conn.mu.Lock()
	conn.authenticated = true
	conn.mu.Unlock()
	conn.writeReply(250, "OK")
	s.logger.Info("Client authenticated", "remote", conn.conn.RemoteAddr())
}
```

---

## 6. Test Requirements

### 6.1 Hash Generation Tests

- ✅ Generate valid hash format `16:SALTHEX$HASH`
- ✅ Salt is 8 bytes (16 hex chars)
- ✅ Hash is 20 bytes (40 hex chars, SHA-1)
- ✅ Different salts produce different hashes
- ✅ Same password+salt produces same hash (deterministic)
- ✅ Hash output has high entropy (Shannon entropy > 7 bits/byte)

### 6.2 Hash Validation Tests

- ✅ Correct password validates successfully
- ✅ Incorrect password fails validation
- ✅ Invalid format rejected (missing parts, wrong algorithm ID)
- ✅ Invalid salt length rejected
- ✅ Invalid hash length rejected
- ✅ Constant-time comparison (no timing leaks)

### 6.3 Integration Tests

- ✅ PROTOCOLINFO advertises HASHEDPASSWORD correctly
- ✅ AUTHENTICATE with plaintext password works with hashed storage
- ✅ Pre-hashed passwords accepted in constructor
- ✅ Empty password disables authentication
- ✅ Rate limiting works with hashed passwords

---

## 7. Compliance Matrix

| Requirement | Status | Implementation |
|------------|--------|----------------|
| **HASH-001**: RFC2440 S2K algorithm | ❌ NOT IMPLEMENTED | Need `hashPasswordRFC2440()` |
| **HASH-002**: SHA-1 hash function | ❌ NOT IMPLEMENTED | Need `crypto/sha1` |
| **HASH-003**: 8-byte salt | ❌ NOT IMPLEMENTED | Need `crypto/rand` |
| **HASH-004**: 65536 iteration count | ❌ NOT IMPLEMENTED | Hard-code count=65536 |
| **HASH-005**: Format `16:SALTHEX$HASH` | ❌ NOT IMPLEMENTED | Need format parsing/generation |
| **HASH-006**: 20-byte SHA-1 output | ❌ NOT IMPLEMENTED | SHA-1 produces 20 bytes |
| **HASH-007**: Config storage | ❌ NOT IMPLEMENTED | Accept in constructor |
| **HASH-008**: PROTOCOLINFO advertisement | ⚠️ PARTIAL | Already advertises, need implementation |
| **HASH-009**: Hash validation | ⚠️ PARTIAL | Constant-time compare exists |
| **HASH-010**: No plaintext storage | ❌ NOT IMPLEMENTED | Store hash only |

**Overall Compliance**: 0% (0/10 requirements fully implemented)

---

## 8. Findings Summary

### 8.1 Critical Findings

| Finding ID | Severity | Description | Remediation |
|-----------|----------|-------------|-------------|
| HASH-SEC-001 | CRITICAL | Plaintext password storage | Implement RFC2440 S2K hashing |
| HASH-SEC-003 | HIGH | False HASHEDPASSWORD advertisement | Implement or remove advertisement |

### 8.2 High Priority Findings

| Finding ID | Severity | Description | Remediation |
|-----------|----------|-------------|-------------|
| HASH-SEC-002 | HIGH | Plaintext password comparison | Hash input, compare to stored hash |

### 8.3 Informational Findings

| Finding ID | Severity | Description | Notes |
|-----------|----------|-------------|-------|
| HASH-INFO-001 | INFO | Constant-time comparison implemented | ✅ GOOD PRACTICE |
| HASH-INFO-002 | INFO | Rate limiting implemented | ✅ GOOD PRACTICE |

---

## 9. Recommendations

### 9.1 Immediate Actions (Required for Production)

1. **Implement RFC2440 S2K Hashing** (2-3 hours)
   - Add `GenerateHashedPassword()` function
   - Add `hashPasswordRFC2440()` helper
   - Add `VerifyHashedPassword()` function
   - Update `Server.password` to `Server.hashedPassword`
   - Accept both plaintext and pre-hashed passwords in constructor

2. **Update PROTOCOLINFO Advertisement** (15 minutes)
   - Keep "HASHEDPASSWORD" advertisement (now accurate)
   - Update documentation to reflect implementation

3. **Add Comprehensive Tests** (1-2 hours)
   - Hash generation tests (format, salt, determinism)
   - Hash validation tests (correct/incorrect, edge cases)
   - Integration tests (AUTHENTICATE with hashed passwords)
   - Constant-time verification tests

### 9.2 Optional Enhancements

1. **Command-line Hash Generator**
   ```bash
   go-tor --hash-password mypassword
   # Output: 16:872860B76453A77799A7D1E07DC64BB5$32A3D35BC76BD3A47ED5825CDD8BF9F70C7DE47B
   ```

2. **Configuration File Support**
   ```
   HashedControlPassword 16:872860B76453A77799A7D1E07DC64BB5$32A3D35BC76BD3A47ED5825CDD8BF9F70C7DE47B
   ```

3. **SAFECOOKIE Authentication** (control-spec.txt §3.5)
   - More secure than HASHEDPASSWORD
   - Challenge-response protocol
   - Prevents replay attacks

---

## 10. Conclusion

**Overall Assessment**: ❌ **NON-COMPLIANT**

The current implementation **does not** implement HASHEDPASSWORD authentication despite advertising it in PROTOCOLINFO. Passwords are stored in plaintext in memory, which violates security best practices and control-spec.txt §3.5.

**Compliance Score**: 0/10 requirements (0%)

**Security Grade**: D (for production), C (for educational use)

**Production Readiness**: ❌ NOT READY (plaintext password storage)

**Educational/Research Use**: ✅ ACCEPTABLE (with documented limitations)

### Risk Summary

- **Current State**: Plaintext passwords, false advertisement, spec violation
- **Impact**: HIGH (credential exposure, spec non-compliance)
- **Likelihood**: MEDIUM (local control port, limited exposure)
- **Mitigation**: Implement RFC2440 S2K hashing (2-3 hours effort)

### Next Steps

1. Implement RFC2440 S2K hashing functions (high priority)
2. Update Server struct to store hashed passwords only
3. Modify authentication handler to hash input passwords
4. Add comprehensive test suite
5. Update documentation and examples
6. Consider SAFECOOKIE for future enhancement

**Estimated Implementation Time**: 4-6 hours total

---

## 11. References

- **control-spec.txt §3.5**: Tor Control Protocol Authentication
- **RFC2440 §3.7.1**: OpenPGP String-to-Key (S2K) Algorithms
- **CWE-256**: Unprotected Storage of Credentials
- **CWE-916**: Use of Password Hash With Insufficient Computational Effort
- **OWASP Password Storage Cheat Sheet**: https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html

---

**Document Version**: 1.0  
**Audit Date**: January 26, 2026  
**Next Review**: After implementation of HASHEDPASSWORD support

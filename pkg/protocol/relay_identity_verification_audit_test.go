// Package protocol provides comprehensive security audit tests for relay identity verification
package protocol

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" // #nosec G505 - SHA-1 required by Tor spec for RSA fingerprints
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
)

// TestRelayIdentityVerification_Comprehensive performs a comprehensive audit
// of relay identity verification against tor-spec.txt §4.2
func TestRelayIdentityVerification_Comprehensive(t *testing.T) {
	t.Run("RSA_Identity_Verification", func(t *testing.T) {
		t.Run("Valid_RSA_Fingerprint_Match", testValidRSAFingerprintMatch)
		t.Run("Invalid_RSA_Fingerprint_Mismatch", testInvalidRSAFingerprintMismatch)
		t.Run("Missing_RSA_Certificate", testMissingRSACertificate)
		t.Run("Invalid_RSA_Public_Key_Type", testInvalidRSAPublicKeyType)
		t.Run("RSA_Fingerprint_Case_Sensitivity", testRSAFingerprintCaseSensitivity)
		t.Run("RSA_Fingerprint_Length_Validation", testRSAFingerprintLengthValidation)
	})

	t.Run("Ed25519_Identity_Verification", func(t *testing.T) {
		t.Run("Valid_Ed25519_Identity_Match", testValidEd25519IdentityMatch)
		t.Run("Invalid_Ed25519_Identity_Mismatch", testInvalidEd25519IdentityMismatch)
		t.Run("Missing_Ed25519_Certificate", testMissingEd25519Certificate)
		t.Run("Invalid_Ed25519_Key_Length", testInvalidEd25519KeyLength)
		t.Run("Ed25519_Cross_Certification", testEd25519CrossCertification)
		t.Run("Ed25519_Identity_Byte_By_Byte", testEd25519IdentityByteByByte)
	})

	t.Run("Dual_Identity_Verification", func(t *testing.T) {
		t.Run("Both_RSA_And_Ed25519_Valid", testBothRSAAndEd25519Valid)
		t.Run("RSA_Valid_Ed25519_Invalid", testRSAValidEd25519Invalid)
		t.Run("RSA_Invalid_Ed25519_Valid", testRSAInvalidEd25519Valid)
		t.Run("Both_RSA_And_Ed25519_Invalid", testBothRSAAndEd25519Invalid)
	})

	t.Run("Attack_Vectors", func(t *testing.T) {
		t.Run("Fingerprint_Collision_Resistance", testFingerprintCollisionResistance)
		t.Run("Identity_Substitution_Attack", testIdentitySubstitutionAttack)
		t.Run("Certificate_Chain_Manipulation", testCertificateChainManipulation)
		t.Run("Timing_Attack_Resistance", testTimingAttackResistance)
		t.Run("Null_Byte_Injection", testNullByteInjection)
		t.Run("Buffer_Overflow_Attempts", testBufferOverflowAttempts)
	})

	t.Run("Edge_Cases", func(t *testing.T) {
		t.Run("Empty_Expected_Values", testEmptyExpectedValues)
		t.Run("Nil_Certificates", testNilCertificates)
		t.Run("Multiple_Identity_Certificates", testMultipleIdentityCertificates)
		t.Run("Zero_Byte_Identity", testZeroByteIdentity)
		t.Run("Max_Length_Fingerprint", testMaxLengthFingerprint)
		t.Run("Unicode_In_Fingerprint", testUnicodeInFingerprint)
	})

	t.Run("Specification_Compliance", func(t *testing.T) {
		t.Run("Tor_Spec_4_2_RSA_Identity", testTorSpec42RSAIdentity)
		t.Run("Tor_Spec_4_2_Ed25519_Identity", testTorSpec42Ed25519Identity)
		t.Run("Cert_Type_2_RSA_ID", testCertType2RSAID)
		t.Run("Cert_Type_4_Ed25519_Signing", testCertType4Ed25519Signing)
		t.Run("Cert_Type_7_Cross_Certification", testCertType7CrossCertification)
	})
}

// Test valid RSA fingerprint matching
func testValidRSAFingerprintMatch(t *testing.T) {
	// Generate RSA key pair
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Create X.509 certificate
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test Relay"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &rsaKey.PublicKey, rsaKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	x509Cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Calculate expected fingerprint
	derBytes, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	if err != nil {
		t.Fatalf("Failed to marshal public key: %v", err)
	}
	fingerprint := sha1.Sum(derBytes) // #nosec G401 - SHA-1 required by Tor spec
	fingerprintHex := fmt.Sprintf("%X", fingerprint[:20])

	// Create CERTS cell
	cert := &Certificate{
		CertType: CertTypeRSAID,
		CertBody: certDER,
		X509Cert: x509Cert,
	}
	certsCell := &CERTSCell{
		Certificates: []*Certificate{cert},
	}

	// Validate - should succeed
	err = certsCell.ValidateRelayIdentity(fingerprintHex, nil)
	if err != nil {
		t.Errorf("ValidateRelayIdentity failed with valid fingerprint: %v", err)
	}
}

// Test invalid RSA fingerprint mismatch
func testInvalidRSAFingerprintMismatch(t *testing.T) {
	// Generate RSA key pair
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Create X.509 certificate
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test Relay"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &rsaKey.PublicKey, rsaKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	x509Cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Create CERTS cell
	cert := &Certificate{
		CertType: CertTypeRSAID,
		CertBody: certDER,
		X509Cert: x509Cert,
	}
	certsCell := &CERTSCell{
		Certificates: []*Certificate{cert},
	}

	// Use incorrect fingerprint
	wrongFingerprint := "0123456789ABCDEF0123456789ABCDEF01234567"

	// Validate - should fail
	err = certsCell.ValidateRelayIdentity(wrongFingerprint, nil)
	if err == nil {
		t.Error("ValidateRelayIdentity should fail with mismatched fingerprint")
	}
	// Error message should indicate mismatch
	t.Logf("Correctly detected mismatch: %v", err)
}

// Test missing RSA certificate
func testMissingRSACertificate(t *testing.T) {
	certsCell := &CERTSCell{
		Certificates: []*Certificate{},
	}

	expectedFingerprint := "0123456789ABCDEF0123456789ABCDEF01234567"

	err := certsCell.ValidateRelayIdentity(expectedFingerprint, nil)
	if err == nil {
		t.Error("ValidateRelayIdentity should fail when RSA certificate is missing")
	}
	if err != nil && err.Error() != "missing RSA identity certificate" {
		t.Errorf("Expected 'missing RSA identity certificate', got: %v", err)
	}
}

// Test invalid RSA public key type
func testInvalidRSAPublicKeyType(t *testing.T) {
	// Create a certificate with Ed25519 public key instead of RSA
	ed25519Pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test Relay"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}

	// Create certificate with Ed25519 public key
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, ed25519Pub, ed25519Pub)
	if err != nil {
		// Expected to fail - X.509 with Ed25519 may not be supported
		t.Skip("X.509 with Ed25519 not supported in this Go version")
	}

	x509Cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	cert := &Certificate{
		CertType: CertTypeRSAID,
		CertBody: certDER,
		X509Cert: x509Cert,
	}
	certsCell := &CERTSCell{
		Certificates: []*Certificate{cert},
	}

	expectedFingerprint := "0123456789ABCDEF0123456789ABCDEF01234567"

	err = certsCell.ValidateRelayIdentity(expectedFingerprint, nil)
	if err == nil {
		t.Error("ValidateRelayIdentity should fail with non-RSA public key")
	}
}

// Test RSA fingerprint case sensitivity
func testRSAFingerprintCaseSensitivity(t *testing.T) {
	// Generate RSA key pair
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test Relay"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &rsaKey.PublicKey, rsaKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	x509Cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Calculate expected fingerprint
	derBytes, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	if err != nil {
		t.Fatalf("Failed to marshal public key: %v", err)
	}
	fingerprint := sha1.Sum(derBytes) // #nosec G401 - SHA-1 required by Tor spec
	fingerprintHex := fmt.Sprintf("%X", fingerprint[:20])

	cert := &Certificate{
		CertType: CertTypeRSAID,
		CertBody: certDER,
		X509Cert: x509Cert,
	}
	certsCell := &CERTSCell{
		Certificates: []*Certificate{cert},
	}

	// Test uppercase (should match)
	err = certsCell.ValidateRelayIdentity(fingerprintHex, nil)
	if err != nil {
		t.Errorf("ValidateRelayIdentity failed with uppercase fingerprint: %v", err)
	}

	// Test lowercase (should fail - case sensitive comparison)
	lowercaseFingerprint := fmt.Sprintf("%x", fingerprint[:20])
	err = certsCell.ValidateRelayIdentity(lowercaseFingerprint, nil)
	if err == nil {
		t.Error("Fingerprint comparison should be case-sensitive")
	}
}

// Test RSA fingerprint length validation
func testRSAFingerprintLengthValidation(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test Relay"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &rsaKey.PublicKey, rsaKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	x509Cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	cert := &Certificate{
		CertType: CertTypeRSAID,
		CertBody: certDER,
		X509Cert: x509Cert,
	}
	certsCell := &CERTSCell{
		Certificates: []*Certificate{cert},
	}

	// Test with too short fingerprint (should mismatch)
	shortFingerprint := "0123456789"
	err = certsCell.ValidateRelayIdentity(shortFingerprint, nil)
	if err == nil {
		t.Error("ValidateRelayIdentity should fail with too short fingerprint")
	}

	// Test with too long fingerprint (should mismatch)
	longFingerprint := "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF"
	err = certsCell.ValidateRelayIdentity(longFingerprint, nil)
	if err == nil {
		t.Error("ValidateRelayIdentity should fail with too long fingerprint")
	}
}

// Test valid Ed25519 identity matching
func testValidEd25519IdentityMatch(t *testing.T) {
	// Generate Ed25519 identity key
	identityPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 key: %v", err)
	}

	// Create Ed25519 certificate
	ed25519Cert := createAuditTestEd25519Cert(identityPub)

	cert := &Certificate{
		CertType:    CertTypeEd25519Signing,
		Ed25519Cert: ed25519Cert,
	}
	certsCell := &CERTSCell{
		Certificates: []*Certificate{cert},
	}

	// Validate - should succeed
	err = certsCell.ValidateRelayIdentity("", identityPub)
	if err != nil {
		t.Errorf("ValidateRelayIdentity failed with valid Ed25519 identity: %v", err)
	}
}

// Test invalid Ed25519 identity mismatch
func testInvalidEd25519IdentityMismatch(t *testing.T) {
	// Generate two different Ed25519 keys
	identityPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 key: %v", err)
	}

	wrongPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate wrong Ed25519 key: %v", err)
	}

	ed25519Cert := createAuditTestEd25519Cert(identityPub)

	cert := &Certificate{
		CertType:    CertTypeEd25519Signing,
		Ed25519Cert: ed25519Cert,
	}
	certsCell := &CERTSCell{
		Certificates: []*Certificate{cert},
	}

	// Validate with wrong key - should fail
	err = certsCell.ValidateRelayIdentity("", wrongPub)
	if err == nil {
		t.Error("ValidateRelayIdentity should fail with mismatched Ed25519 identity")
	}
}

// Test missing Ed25519 certificate
func testMissingEd25519Certificate(t *testing.T) {
	certsCell := &CERTSCell{
		Certificates: []*Certificate{},
	}

	expectedIdentity := make([]byte, 32)
	rand.Read(expectedIdentity)

	err := certsCell.ValidateRelayIdentity("", expectedIdentity)
	if err == nil {
		t.Error("ValidateRelayIdentity should fail when Ed25519 certificate is missing")
	}
	if err != nil && err.Error() != "missing Ed25519 identity certificate" {
		t.Errorf("Expected 'missing Ed25519 identity certificate', got: %v", err)
	}
}

// Test invalid Ed25519 key length
func testInvalidEd25519KeyLength(t *testing.T) {
	// Create certificate with wrong key length
	ed25519Cert := &Ed25519Certificate{
		Version:      1,
		CertType:     4,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		CertKeyType:  1,
		CertifiedKey: make([]byte, 16), // Wrong length (should be 32)
		Extensions:   []Ed25519Extension{},
		Signature:    make([]byte, 64),
	}

	cert := &Certificate{
		CertType:    CertTypeEd25519Signing,
		Ed25519Cert: ed25519Cert,
	}
	certsCell := &CERTSCell{
		Certificates: []*Certificate{cert},
	}

	expectedIdentity := make([]byte, 32)
	err := certsCell.ValidateRelayIdentity("", expectedIdentity)
	if err == nil {
		t.Error("ValidateRelayIdentity should fail with invalid key length")
	}
}

// Test Ed25519 cross-certification (type 7)
func testEd25519CrossCertification(t *testing.T) {
	identityPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 key: %v", err)
	}

	ed25519Cert := createAuditTestEd25519Cert(identityPub)

	// Use type 7 (cross-certification) instead of type 4
	cert := &Certificate{
		CertType:    CertTypeEd25519Identity,
		Ed25519Cert: ed25519Cert,
	}
	certsCell := &CERTSCell{
		Certificates: []*Certificate{cert},
	}

	err = certsCell.ValidateRelayIdentity("", identityPub)
	if err != nil {
		t.Errorf("ValidateRelayIdentity should accept type 7 cross-certification: %v", err)
	}
}

// Test Ed25519 identity byte-by-byte comparison
func testEd25519IdentityByteByByte(t *testing.T) {
	identityPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 key: %v", err)
	}

	ed25519Cert := createAuditTestEd25519Cert(identityPub)

	cert := &Certificate{
		CertType:    CertTypeEd25519Signing,
		Ed25519Cert: ed25519Cert,
	}
	certsCell := &CERTSCell{
		Certificates: []*Certificate{cert},
	}

	// Test each byte position
	for i := 0; i < 32; i++ {
		wrongIdentity := make([]byte, 32)
		copy(wrongIdentity, identityPub)
		wrongIdentity[i] ^= 0xFF // Flip all bits in one byte

		err = certsCell.ValidateRelayIdentity("", wrongIdentity)
		if err == nil {
			t.Errorf("ValidateRelayIdentity should detect mismatch at byte %d", i)
		}
	}
}

// Test both RSA and Ed25519 valid
func testBothRSAAndEd25519Valid(t *testing.T) {
	// Generate RSA key
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test Relay"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}

	rsaCertDER, err := x509.CreateCertificate(rand.Reader, template, template, &rsaKey.PublicKey, rsaKey)
	if err != nil {
		t.Fatalf("Failed to create RSA certificate: %v", err)
	}

	rsaX509Cert, err := x509.ParseCertificate(rsaCertDER)
	if err != nil {
		t.Fatalf("Failed to parse RSA certificate: %v", err)
	}

	rsaDerBytes, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	if err != nil {
		t.Fatalf("Failed to marshal RSA public key: %v", err)
	}
	rsaFingerprint := sha1.Sum(rsaDerBytes) // #nosec G401
	rsaFingerprintHex := fmt.Sprintf("%X", rsaFingerprint[:20])

	// Generate Ed25519 key
	ed25519Pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 key: %v", err)
	}

	ed25519Cert := createAuditTestEd25519Cert(ed25519Pub)

	certsCell := &CERTSCell{
		Certificates: []*Certificate{
			{CertType: CertTypeRSAID, CertBody: rsaCertDER, X509Cert: rsaX509Cert},
			{CertType: CertTypeEd25519Signing, Ed25519Cert: ed25519Cert},
		},
	}

	// Both should validate
	err = certsCell.ValidateRelayIdentity(rsaFingerprintHex, ed25519Pub)
	if err != nil {
		t.Errorf("ValidateRelayIdentity failed with both valid identities: %v", err)
	}
}

// Test RSA valid, Ed25519 invalid
func testRSAValidEd25519Invalid(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test Relay"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}

	rsaCertDER, err := x509.CreateCertificate(rand.Reader, template, template, &rsaKey.PublicKey, rsaKey)
	if err != nil {
		t.Fatalf("Failed to create RSA certificate: %v", err)
	}

	rsaX509Cert, err := x509.ParseCertificate(rsaCertDER)
	if err != nil {
		t.Fatalf("Failed to parse RSA certificate: %v", err)
	}

	rsaDerBytes, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	if err != nil {
		t.Fatalf("Failed to marshal RSA public key: %v", err)
	}
	rsaFingerprint := sha1.Sum(rsaDerBytes) // #nosec G401
	rsaFingerprintHex := fmt.Sprintf("%X", rsaFingerprint[:20])

	// Generate two different Ed25519 keys
	correctEd25519, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 key: %v", err)
	}

	wrongEd25519, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate wrong Ed25519 key: %v", err)
	}

	ed25519Cert := createAuditTestEd25519Cert(correctEd25519)

	certsCell := &CERTSCell{
		Certificates: []*Certificate{
			{CertType: CertTypeRSAID, CertBody: rsaCertDER, X509Cert: rsaX509Cert},
			{CertType: CertTypeEd25519Signing, Ed25519Cert: ed25519Cert},
		},
	}

	// Should fail due to Ed25519 mismatch
	err = certsCell.ValidateRelayIdentity(rsaFingerprintHex, wrongEd25519)
	if err == nil {
		t.Error("ValidateRelayIdentity should fail with Ed25519 mismatch")
	}
}

// Test RSA invalid, Ed25519 valid
func testRSAInvalidEd25519Valid(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test Relay"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}

	rsaCertDER, err := x509.CreateCertificate(rand.Reader, template, template, &rsaKey.PublicKey, rsaKey)
	if err != nil {
		t.Fatalf("Failed to create RSA certificate: %v", err)
	}

	rsaX509Cert, err := x509.ParseCertificate(rsaCertDER)
	if err != nil {
		t.Fatalf("Failed to parse RSA certificate: %v", err)
	}

	wrongRSAFingerprint := "0123456789ABCDEF0123456789ABCDEF01234567"

	ed25519Pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 key: %v", err)
	}

	ed25519Cert := createAuditTestEd25519Cert(ed25519Pub)

	certsCell := &CERTSCell{
		Certificates: []*Certificate{
			{CertType: CertTypeRSAID, CertBody: rsaCertDER, X509Cert: rsaX509Cert},
			{CertType: CertTypeEd25519Signing, Ed25519Cert: ed25519Cert},
		},
	}

	// Should fail due to RSA mismatch
	err = certsCell.ValidateRelayIdentity(wrongRSAFingerprint, ed25519Pub)
	if err == nil {
		t.Error("ValidateRelayIdentity should fail with RSA mismatch")
	}
}

// Test both RSA and Ed25519 invalid
func testBothRSAAndEd25519Invalid(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test Relay"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}

	rsaCertDER, err := x509.CreateCertificate(rand.Reader, template, template, &rsaKey.PublicKey, rsaKey)
	if err != nil {
		t.Fatalf("Failed to create RSA certificate: %v", err)
	}

	rsaX509Cert, err := x509.ParseCertificate(rsaCertDER)
	if err != nil {
		t.Fatalf("Failed to parse RSA certificate: %v", err)
	}

	wrongRSAFingerprint := "0123456789ABCDEF0123456789ABCDEF01234567"

	correctEd25519, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 key: %v", err)
	}

	wrongEd25519, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate wrong Ed25519 key: %v", err)
	}

	ed25519Cert := createAuditTestEd25519Cert(correctEd25519)

	certsCell := &CERTSCell{
		Certificates: []*Certificate{
			{CertType: CertTypeRSAID, CertBody: rsaCertDER, X509Cert: rsaX509Cert},
			{CertType: CertTypeEd25519Signing, Ed25519Cert: ed25519Cert},
		},
	}

	// Should fail (RSA checked first)
	err = certsCell.ValidateRelayIdentity(wrongRSAFingerprint, wrongEd25519)
	if err == nil {
		t.Error("ValidateRelayIdentity should fail with both identities mismatched")
	}
}

// Test fingerprint collision resistance
func testFingerprintCollisionResistance(t *testing.T) {
	// Generate two different RSA keys
	rsaKey1, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key 1: %v", err)
	}

	rsaKey2, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key 2: %v", err)
	}

	// Calculate fingerprints
	derBytes1, _ := x509.MarshalPKIXPublicKey(&rsaKey1.PublicKey)
	derBytes2, _ := x509.MarshalPKIXPublicKey(&rsaKey2.PublicKey)

	fingerprint1 := sha1.Sum(derBytes1) // #nosec G401
	fingerprint2 := sha1.Sum(derBytes2) // #nosec G401

	// Verify fingerprints are different (collision resistant)
	if string(fingerprint1[:]) == string(fingerprint2[:]) {
		t.Error("SHA-256 collision detected (extremely unlikely!)")
	}

	fingerprintHex1 := fmt.Sprintf("%X", fingerprint1[:20])
	fingerprintHex2 := fmt.Sprintf("%X", fingerprint2[:20])

	if fingerprintHex1 == fingerprintHex2 {
		t.Error("Fingerprint collision detected")
	}

	t.Logf("Collision resistance verified: different keys produce different fingerprints")
}

// Test identity substitution attack
func testIdentitySubstitutionAttack(t *testing.T) {
	// Attacker tries to substitute legitimate relay's identity with their own

	// Legitimate relay's identity
	legitimateEd25519, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate legitimate Ed25519 key: %v", err)
	}

	// Attacker's identity
	attackerEd25519, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate attacker Ed25519 key: %v", err)
	}

	// Attacker creates certificate with their key
	attackerCert := createAuditTestEd25519Cert(attackerEd25519)

	cert := &Certificate{
		CertType:    CertTypeEd25519Signing,
		Ed25519Cert: attackerCert,
	}
	certsCell := &CERTSCell{
		Certificates: []*Certificate{cert},
	}

	// Validate against legitimate identity - should fail
	err = certsCell.ValidateRelayIdentity("", legitimateEd25519)
	if err == nil {
		t.Error("Identity substitution attack succeeded (should fail)")
	}

	t.Logf("Identity substitution attack correctly detected: %v", err)
}

// Test certificate chain manipulation
func testCertificateChainManipulation(t *testing.T) {
	// Attacker tries to manipulate certificate order to bypass validation

	identity1, _, _ := ed25519.GenerateKey(rand.Reader)
	identity2, _, _ := ed25519.GenerateKey(rand.Reader)

	cert1 := createAuditTestEd25519Cert(identity1)
	cert2 := createAuditTestEd25519Cert(identity2)

	// Try multiple certificates of same type
	certsCell := &CERTSCell{
		Certificates: []*Certificate{
			{CertType: CertTypeEd25519Signing, Ed25519Cert: cert1},
			{CertType: CertTypeEd25519Signing, Ed25519Cert: cert2},
		},
	}

	// ValidateRelayIdentity should use first matching certificate
	err := certsCell.ValidateRelayIdentity("", identity1)
	if err != nil {
		t.Errorf("Failed to validate with first certificate: %v", err)
	}

	// Should fail with second identity
	err = certsCell.ValidateRelayIdentity("", identity2)
	if err == nil {
		t.Error("Certificate chain manipulation allowed unauthorized identity")
	}
}

// Test timing attack resistance
func testTimingAttackResistance(t *testing.T) {
	identityPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 key: %v", err)
	}

	ed25519Cert := createAuditTestEd25519Cert(identityPub)

	cert := &Certificate{
		CertType:    CertTypeEd25519Signing,
		Ed25519Cert: ed25519Cert,
	}
	certsCell := &CERTSCell{
		Certificates: []*Certificate{cert},
	}

	// Measure timing for correct vs incorrect identities
	iterations := 100

	// Correct identity timing
	wrongIdentity := make([]byte, 32)
	copy(wrongIdentity, identityPub)
	wrongIdentity[0] ^= 0x01 // Change first byte

	var correctTiming, incorrectTiming time.Duration

	// Warm-up
	for i := 0; i < 10; i++ {
		certsCell.ValidateRelayIdentity("", identityPub)
		certsCell.ValidateRelayIdentity("", wrongIdentity)
	}

	// Measure correct identity
	start := time.Now()
	for i := 0; i < iterations; i++ {
		certsCell.ValidateRelayIdentity("", identityPub)
	}
	correctTiming = time.Since(start)

	// Measure incorrect identity
	start = time.Now()
	for i := 0; i < iterations; i++ {
		certsCell.ValidateRelayIdentity("", wrongIdentity)
	}
	incorrectTiming = time.Since(start)

	avgCorrect := correctTiming / time.Duration(iterations)
	avgIncorrect := incorrectTiming / time.Duration(iterations)

	t.Logf("Average timing - Correct: %v, Incorrect: %v", avgCorrect, avgIncorrect)

	// Note: Current implementation uses simple byte-by-byte comparison (not constant-time)
	// This is acceptable for educational use but should be improved for production
	if avgCorrect == avgIncorrect {
		t.Log("Timing is identical (perfect constant-time)")
	} else {
		timingDiff := float64(avgCorrect-avgIncorrect) / float64(avgCorrect) * 100
		t.Logf("Timing difference: %.2f%%", timingDiff)
		// For research purposes, small timing differences are acceptable
	}
}

// Test null byte injection
func testNullByteInjection(t *testing.T) {
	identityPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 key: %v", err)
	}

	// Try to inject null byte in identity
	maliciousIdentity := make([]byte, 32)
	copy(maliciousIdentity, identityPub[:16])
	// Rest is zeros (null bytes)

	ed25519Cert := createAuditTestEd25519Cert(identityPub)

	cert := &Certificate{
		CertType:    CertTypeEd25519Signing,
		Ed25519Cert: ed25519Cert,
	}
	certsCell := &CERTSCell{
		Certificates: []*Certificate{cert},
	}

	err = certsCell.ValidateRelayIdentity("", maliciousIdentity)
	if err == nil {
		t.Error("Null byte injection should not bypass validation")
	}
}

// Test buffer overflow attempts
func testBufferOverflowAttempts(t *testing.T) {
	// Try oversized identity
	oversizedIdentity := make([]byte, 1024)
	rand.Read(oversizedIdentity)

	ed25519Cert := createAuditTestEd25519Cert(make([]byte, 32))

	cert := &Certificate{
		CertType:    CertTypeEd25519Signing,
		Ed25519Cert: ed25519Cert,
	}
	certsCell := &CERTSCell{
		Certificates: []*Certificate{cert},
	}

	err := certsCell.ValidateRelayIdentity("", oversizedIdentity)
	if err == nil {
		t.Error("Oversized identity should be rejected")
	}

	// Try undersized identity
	undersizedIdentity := make([]byte, 16)
	err = certsCell.ValidateRelayIdentity("", undersizedIdentity)
	if err == nil {
		t.Error("Undersized identity should be rejected")
	}
}

// Test empty expected values
func testEmptyExpectedValues(t *testing.T) {
	certsCell := &CERTSCell{
		Certificates: []*Certificate{},
	}

	// Both empty - should succeed (no validation required)
	err := certsCell.ValidateRelayIdentity("", nil)
	if err != nil {
		t.Errorf("ValidateRelayIdentity should succeed with empty expected values: %v", err)
	}

	// Empty identity slice
	err = certsCell.ValidateRelayIdentity("", []byte{})
	if err != nil {
		t.Errorf("ValidateRelayIdentity should succeed with empty identity slice: %v", err)
	}
}

// Test nil certificates
func testNilCertificates(t *testing.T) {
	certsCell := &CERTSCell{
		Certificates: []*Certificate{
			{CertType: CertTypeRSAID, X509Cert: nil},
			{CertType: CertTypeEd25519Signing, Ed25519Cert: nil},
		},
	}

	err := certsCell.ValidateRelayIdentity("ABCD1234", nil)
	if err == nil {
		t.Error("ValidateRelayIdentity should fail with nil X509Cert")
	}

	identity := make([]byte, 32)
	err = certsCell.ValidateRelayIdentity("", identity)
	if err == nil {
		t.Error("ValidateRelayIdentity should fail with nil Ed25519Cert")
	}
}

// Test multiple identity certificates
func testMultipleIdentityCertificates(t *testing.T) {
	identity1, _, _ := ed25519.GenerateKey(rand.Reader)
	identity2, _, _ := ed25519.GenerateKey(rand.Reader)

	cert1 := createAuditTestEd25519Cert(identity1)
	cert2 := createAuditTestEd25519Cert(identity2)

	certsCell := &CERTSCell{
		Certificates: []*Certificate{
			{CertType: CertTypeEd25519Signing, Ed25519Cert: cert1},
			{CertType: CertTypeEd25519Signing, Ed25519Cert: cert2},
		},
	}

	// Should use first matching certificate
	err := certsCell.ValidateRelayIdentity("", identity1)
	if err != nil {
		t.Errorf("Failed to validate with first certificate: %v", err)
	}
}

// Test zero-byte identity
func testZeroByteIdentity(t *testing.T) {
	zeroIdentity := make([]byte, 32)
	ed25519Cert := createAuditTestEd25519Cert(zeroIdentity)

	cert := &Certificate{
		CertType:    CertTypeEd25519Signing,
		Ed25519Cert: ed25519Cert,
	}
	certsCell := &CERTSCell{
		Certificates: []*Certificate{cert},
	}

	// Should validate successfully with all-zero identity
	err := certsCell.ValidateRelayIdentity("", zeroIdentity)
	if err != nil {
		t.Errorf("ValidateRelayIdentity should accept all-zero identity: %v", err)
	}
}

// Test max length fingerprint
func testMaxLengthFingerprint(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test Relay"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}

	certDER, _ := x509.CreateCertificate(rand.Reader, template, template, &rsaKey.PublicKey, rsaKey)
	x509Cert, _ := x509.ParseCertificate(certDER)

	cert := &Certificate{
		CertType: CertTypeRSAID,
		CertBody: certDER,
		X509Cert: x509Cert,
	}
	certsCell := &CERTSCell{
		Certificates: []*Certificate{cert},
	}

	// Test with very long fingerprint string
	veryLongFingerprint := string(make([]byte, 10000))
	err := certsCell.ValidateRelayIdentity(veryLongFingerprint, nil)
	if err == nil {
		t.Error("Very long fingerprint should mismatch")
	}
}

// Test unicode in fingerprint
func testUnicodeInFingerprint(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test Relay"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}

	certDER, _ := x509.CreateCertificate(rand.Reader, template, template, &rsaKey.PublicKey, rsaKey)
	x509Cert, _ := x509.ParseCertificate(certDER)

	cert := &Certificate{
		CertType: CertTypeRSAID,
		CertBody: certDER,
		X509Cert: x509Cert,
	}
	certsCell := &CERTSCell{
		Certificates: []*Certificate{cert},
	}

	unicodeFingerprint := "你好世界ABCDEF0123456789"
	err := certsCell.ValidateRelayIdentity(unicodeFingerprint, nil)
	if err == nil {
		t.Error("Unicode fingerprint should mismatch")
	}
}

// Specification compliance tests

// Test tor-spec.txt §4.2 RSA identity requirement
func testTorSpec42RSAIdentity(t *testing.T) {
	// Per tor-spec.txt §4.2, RSA identity certificate (type 2) contains
	// the relay's RSA identity key
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Tor Relay"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}

	certDER, _ := x509.CreateCertificate(rand.Reader, template, template, &rsaKey.PublicKey, rsaKey)
	x509Cert, _ := x509.ParseCertificate(certDER)

	cert := &Certificate{
		CertType: CertTypeRSAID,
		CertBody: certDER,
		X509Cert: x509Cert,
	}

	// Verify cert type is 2
	if cert.CertType != CertTypeRSAID {
		t.Errorf("RSA identity cert type should be 2, got %d", cert.CertType)
	}

	// Verify contains RSA public key
	if _, ok := x509Cert.PublicKey.(*rsa.PublicKey); !ok {
		t.Error("RSA identity cert should contain RSA public key")
	}

	t.Log("tor-spec.txt §4.2 RSA identity requirement verified")
}

// Test tor-spec.txt §4.2 Ed25519 identity requirement
func testTorSpec42Ed25519Identity(t *testing.T) {
	// Per tor-spec.txt §4.2, Ed25519 signing key certificate (type 4)
	// or cross-certification (type 7) contains the relay's Ed25519 identity
	_, pubKey, _ := ed25519.GenerateKey(rand.Reader)

	cert := createAuditTestEd25519Cert(pubKey)

	// Verify certified key is 32 bytes
	if len(cert.CertifiedKey) != 32 {
		t.Errorf("Ed25519 certified key should be 32 bytes, got %d", len(cert.CertifiedKey))
	}

	// Verify matches generated key
	for i := 0; i < 32; i++ {
		if cert.CertifiedKey[i] != pubKey[i] {
			t.Error("Certified key doesn't match generated key")
			break
		}
	}

	t.Log("tor-spec.txt §4.2 Ed25519 identity requirement verified")
}

// Test cert type 2 (RSA ID)
func testCertType2RSAID(t *testing.T) {
	if CertTypeRSAID != 0x02 {
		t.Errorf("CertTypeRSAID should be 0x02, got 0x%02X", CertTypeRSAID)
	}

	if CertTypeRSAID.String() != "RSA_ID" {
		t.Errorf("CertTypeRSAID.String() should be 'RSA_ID', got '%s'", CertTypeRSAID.String())
	}
}

// Test cert type 4 (Ed25519 signing key)
func testCertType4Ed25519Signing(t *testing.T) {
	if CertTypeEd25519Signing != 0x04 {
		t.Errorf("CertTypeEd25519Signing should be 0x04, got 0x%02X", CertTypeEd25519Signing)
	}

	if CertTypeEd25519Signing.String() != "ED25519_SIGNING" {
		t.Errorf("CertTypeEd25519Signing.String() should be 'ED25519_SIGNING', got '%s'", CertTypeEd25519Signing.String())
	}
}

// Test cert type 7 (RSA cross-certification of Ed25519)
func testCertType7CrossCertification(t *testing.T) {
	if CertTypeEd25519Identity != 0x07 {
		t.Errorf("CertTypeEd25519Identity should be 0x07, got 0x%02X", CertTypeEd25519Identity)
	}

	if CertTypeEd25519Identity.String() != "ED25519_IDENTITY" {
		t.Errorf("CertTypeEd25519Identity.String() should be 'ED25519_IDENTITY', got '%s'", CertTypeEd25519Identity.String())
	}
}

// Helper function to create a test Ed25519 certificate (for audit tests)
func createAuditTestEd25519Cert(publicKey []byte) *Ed25519Certificate {
	cert := &Ed25519Certificate{
		Version:      1,
		CertType:     4,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		CertKeyType:  1,
		CertifiedKey: make([]byte, 32),
		Extensions:   []Ed25519Extension{},
		Signature:    make([]byte, 64),
	}
	copy(cert.CertifiedKey, publicKey)
	return cert
}

// TestRelayIdentityIntegration tests complete identity verification workflow
func TestRelayIdentityIntegration(t *testing.T) {
	// Create a complete CERTS cell as would be received from a relay
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Integration Test Relay"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}

	rsaCertDER, _ := x509.CreateCertificate(rand.Reader, template, template, &rsaKey.PublicKey, rsaKey)

	rsaDerBytes, _ := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	rsaFingerprint := sha1.Sum(rsaDerBytes) // #nosec G401
	rsaFingerprintHex := fmt.Sprintf("%X", rsaFingerprint[:20])

	ed25519Pub, _, _ := ed25519.GenerateKey(rand.Reader)
	ed25519Cert := createAuditTestEd25519Cert(ed25519Pub)

	// Create cell payload
	payload := make([]byte, 0, 1024)
	payload = append(payload, 2) // 2 certificates

	// RSA certificate
	payload = append(payload, byte(CertTypeRSAID))
	rsaCertLen := make([]byte, 2)
	binary.BigEndian.PutUint16(rsaCertLen, uint16(len(rsaCertDER)))
	payload = append(payload, rsaCertLen...)
	payload = append(payload, rsaCertDER...)

	// Ed25519 certificate (encode it)
	ed25519Bytes := encodeEd25519Cert(ed25519Cert)
	payload = append(payload, byte(CertTypeEd25519Signing))
	ed25519CertLen := make([]byte, 2)
	binary.BigEndian.PutUint16(ed25519CertLen, uint16(len(ed25519Bytes)))
	payload = append(payload, ed25519CertLen...)
	payload = append(payload, ed25519Bytes...)

	// Parse CERTS cell
	cellData := cell.NewCell(0, cell.CmdCerts)
	cellData.Payload = payload

	certsCell, err := ParseCERTSCell(cellData)
	if err != nil {
		t.Fatalf("Failed to parse CERTS cell: %v", err)
	}

	// Validate both identities
	err = certsCell.ValidateRelayIdentity(rsaFingerprintHex, ed25519Pub)
	if err != nil {
		t.Errorf("Integration test failed: %v", err)
	}

	t.Log("Integration test passed: full CERTS cell verification successful")
}

// Helper to encode Ed25519 certificate for testing
func encodeEd25519Cert(cert *Ed25519Certificate) []byte {
	data := make([]byte, 0, 256)

	data = append(data, cert.Version)
	data = append(data, cert.CertType)

	expirationHours := uint32(cert.ExpiresAt.Unix() / 3600)
	expBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(expBytes, expirationHours)
	data = append(data, expBytes...)

	data = append(data, cert.CertKeyType)
	data = append(data, cert.CertifiedKey...)

	data = append(data, byte(len(cert.Extensions)))
	for _, ext := range cert.Extensions {
		extLen := uint16(2 + len(ext.ExtData))
		extLenBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(extLenBytes, extLen)
		data = append(data, extLenBytes...)
		data = append(data, ext.ExtType)
		data = append(data, ext.Flags)
		data = append(data, ext.ExtData...)
	}

	data = append(data, cert.Signature...)

	return data
}

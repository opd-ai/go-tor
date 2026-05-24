package protocol

import (
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
)

// TestValidateRelayIdentity_RSA_Success tests successful RSA fingerprint validation
func TestValidateRelayIdentity_RSA_Success(t *testing.T) {
	// Generate RSA key pair
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Create X.509 certificate
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Tor RSA Identity"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &rsaKey.PublicKey, rsaKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	x509Cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Calculate expected fingerprint using SHA-1 (per dir-spec.txt)
	derBytes := x509.MarshalPKCS1PublicKey(&rsaKey.PublicKey)
	fingerprint := sha1.Sum(derBytes) // #nosec G401 - SHA-1 required by Tor spec
	expectedFingerprint := fmt.Sprintf("%X", fingerprint[:20])

	// Create CERTSCell with RSA identity cert
	certs := &CERTSCell{
		Certificates: []*Certificate{
			{
				CertType: CertTypeRSAID,
				X509Cert: x509Cert,
			},
		},
	}

	// Should pass with matching fingerprint
	err = certs.ValidateRelayIdentity(expectedFingerprint, nil)
	if err != nil {
		t.Errorf("ValidateRelayIdentity failed with matching fingerprint: %v", err)
	}
}

// TestValidateRelayIdentity_RSA_Mismatch tests RSA fingerprint mismatch detection
func TestValidateRelayIdentity_RSA_Mismatch(t *testing.T) {
	// Generate RSA key pair
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Create X.509 certificate
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Tor RSA Identity"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &rsaKey.PublicKey, rsaKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	x509Cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Create CERTSCell with RSA identity cert
	certs := &CERTSCell{
		Certificates: []*Certificate{
			{
				CertType: CertTypeRSAID,
				X509Cert: x509Cert,
			},
		},
	}

	// Use wrong fingerprint
	wrongFingerprint := "0000000000000000000000000000000000000000"

	// Should fail with mismatched fingerprint
	err = certs.ValidateRelayIdentity(wrongFingerprint, nil)
	if err == nil {
		t.Error("Expected error for mismatched RSA fingerprint, got nil")
	}
	if err != nil && err.Error()[:24] != "RSA identity mismatch: e" {
		t.Errorf("Expected RSA identity mismatch error, got: %v", err)
	}
}

// TestValidateRelayIdentity_RSA_MissingCert tests error when RSA cert is missing
func TestValidateRelayIdentity_RSA_MissingCert(t *testing.T) {
	// Create CERTSCell without RSA identity cert
	certs := &CERTSCell{
		Certificates: []*Certificate{},
	}

	// Should fail when fingerprint expected but cert missing
	err := certs.ValidateRelayIdentity("AAAA", nil)
	if err == nil {
		t.Error("Expected error for missing RSA certificate, got nil")
	}
	if err != nil && err.Error() != "missing RSA identity certificate" {
		t.Errorf("Expected 'missing RSA identity certificate', got: %v", err)
	}
}

// TestValidateRelayIdentity_RSA_InvalidPublicKey tests error with non-RSA public key
func TestValidateRelayIdentity_RSA_InvalidPublicKey(t *testing.T) {
	// Create X.509 certificate with ECDSA key (not RSA)
	// This is a bit tricky - we'll create a cert with nil X509Cert.PublicKey that's not RSA
	certs := &CERTSCell{
		Certificates: []*Certificate{
			{
				CertType: CertTypeRSAID,
				X509Cert: &x509.Certificate{
					PublicKey: "not-an-rsa-key", // Invalid type
				},
			},
		},
	}

	// Should fail with invalid public key type
	err := certs.ValidateRelayIdentity("AAAA", nil)
	if err == nil {
		t.Error("Expected error for invalid public key type, got nil")
	}
	if err != nil && err.Error() != "RSA identity cert does not contain RSA public key" {
		t.Errorf("Expected 'RSA identity cert does not contain RSA public key', got: %v", err)
	}
}

// TestValidateRelayIdentity_Ed25519_MissingCert tests error when Ed25519 cert is missing
func TestValidateRelayIdentity_Ed25519_MissingCert(t *testing.T) {
	expectedIdentity := make([]byte, 32)
	for i := 0; i < 32; i++ {
		expectedIdentity[i] = byte(i)
	}

	// Create CERTSCell without Ed25519 cert
	certs := &CERTSCell{
		Certificates: []*Certificate{},
	}

	// Should fail when identity expected but cert missing
	err := certs.ValidateRelayIdentity("", expectedIdentity)
	if err == nil {
		t.Error("Expected error for missing Ed25519 certificate, got nil")
	}
	if err != nil && err.Error() != "missing Ed25519 identity certificate" {
		t.Errorf("Expected 'missing Ed25519 identity certificate', got: %v", err)
	}
}

// TestValidateRelayIdentity_Ed25519_InvalidKeyLength tests error with wrong key length
func TestValidateRelayIdentity_Ed25519_InvalidKeyLength(t *testing.T) {
	expectedIdentity := make([]byte, 32)

	// Create Ed25519 cert with invalid key length
	data := make([]byte, 40+64)
	offset := 0
	data[offset] = 1 // Version
	offset++
	data[offset] = 4 // CertType
	offset++

	expirationHours := uint32(time.Now().Add(365*24*time.Hour).Unix() / 3600)
	binary.BigEndian.PutUint32(data[offset:offset+4], expirationHours)
	offset += 4

	data[offset] = 1 // CertKeyType
	offset++

	// Only 16 bytes instead of 32
	shortKey := make([]byte, 16)
	copy(data[offset:], shortKey)
	offset += 16

	data[offset] = 0 // No extensions
	offset++

	// Create certificate with truncated certified key
	ed25519Cert := &Ed25519Certificate{
		Version:      1,
		CertType:     4,
		CertifiedKey: shortKey, // Invalid length
	}

	certs := &CERTSCell{
		Certificates: []*Certificate{
			{
				CertType:    CertTypeEd25519Signing,
				Ed25519Cert: ed25519Cert,
			},
		},
	}

	// Should fail with invalid key length
	err := certs.ValidateRelayIdentity("", expectedIdentity)
	if err == nil {
		t.Error("Expected error for invalid Ed25519 key length, got nil")
	}
	if err != nil && err.Error() != "invalid Ed25519 certified key length: 16" {
		t.Errorf("Expected 'invalid Ed25519 certified key length: 16', got: %v", err)
	}
}

// TestValidateRelayIdentity_Ed25519_InvalidExpectedLength tests error with wrong expected identity length
func TestValidateRelayIdentity_Ed25519_InvalidExpectedLength(t *testing.T) {
	// Create valid Ed25519 cert
	data := make([]byte, 40+64)
	offset := 0
	data[offset] = 1 // Version
	offset++
	data[offset] = 4 // CertType
	offset++

	expirationHours := uint32(time.Now().Add(365*24*time.Hour).Unix() / 3600)
	binary.BigEndian.PutUint32(data[offset:offset+4], expirationHours)
	offset += 4

	data[offset] = 1 // CertKeyType
	offset++

	validKey := make([]byte, 32)
	copy(data[offset:offset+32], validKey)
	offset += 32

	data[offset] = 0 // No extensions
	offset++

	ed25519Cert := &Ed25519Certificate{
		Version:      1,
		CertType:     4,
		CertifiedKey: validKey,
	}

	certs := &CERTSCell{
		Certificates: []*Certificate{
			{
				CertType:    CertTypeEd25519Signing,
				Ed25519Cert: ed25519Cert,
			},
		},
	}

	// Use wrong expected identity length (16 instead of 32)
	invalidExpected := make([]byte, 16)

	// Should fail with invalid expected length
	err := certs.ValidateRelayIdentity("", invalidExpected)
	if err == nil {
		t.Error("Expected error for invalid expected Ed25519 identity length, got nil")
	}
	if err != nil && err.Error() != "invalid expected Ed25519 identity length: 16" {
		t.Errorf("Expected 'invalid expected Ed25519 identity length: 16', got: %v", err)
	}
}

// TestValidateRelayIdentity_Ed25519_CrossCert tests using type 7 certificate
func TestValidateRelayIdentity_Ed25519_CrossCert(t *testing.T) {
	expectedIdentity := make([]byte, 32)
	for i := 0; i < 32; i++ {
		expectedIdentity[i] = byte(i)
	}

	// Create Ed25519 cert with type 7 (cross-certification)
	data := make([]byte, 40+64)
	offset := 0
	data[offset] = 1 // Version
	offset++
	data[offset] = 7 // CertType (cross-cert)
	offset++

	expirationHours := uint32(time.Now().Add(365*24*time.Hour).Unix() / 3600)
	binary.BigEndian.PutUint32(data[offset:offset+4], expirationHours)
	offset += 4

	data[offset] = 1 // CertKeyType
	offset++

	copy(data[offset:offset+32], expectedIdentity)
	offset += 32

	data[offset] = 0 // No extensions

	ed25519Cert := &Ed25519Certificate{
		Version:      1,
		CertType:     7,
		CertifiedKey: expectedIdentity,
	}

	// Create cell with type 7 cert (not type 4)
	certs := &CERTSCell{
		Certificates: []*Certificate{
			{
				CertType:    CertTypeEd25519Identity,
				Ed25519Cert: ed25519Cert,
			},
		},
	}

	// Should pass - validates using type 7 as fallback
	err := certs.ValidateRelayIdentity("", expectedIdentity)
	if err != nil {
		t.Errorf("ValidateRelayIdentity failed with type 7 cert: %v", err)
	}
}

// TestValidateRelayIdentity_BothRSAAndEd25519 tests validation with both fingerprints
func TestValidateRelayIdentity_BothRSAAndEd25519(t *testing.T) {
	// Generate RSA key pair
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Create X.509 certificate
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Tor RSA Identity"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &rsaKey.PublicKey, rsaKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	x509Cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Calculate RSA fingerprint
	derBytes := x509.MarshalPKCS1PublicKey(&rsaKey.PublicKey)
	fingerprint := sha1.Sum(derBytes) // #nosec G401 - SHA-1 required by Tor spec
	expectedFingerprint := fmt.Sprintf("%X", fingerprint[:20])

	// Create Ed25519 identity
	expectedIdentity := make([]byte, 32)
	for i := 0; i < 32; i++ {
		expectedIdentity[i] = byte(i)
	}

	ed25519Cert := &Ed25519Certificate{
		Version:      1,
		CertType:     4,
		CertifiedKey: expectedIdentity,
	}

	// Create CERTSCell with both certs
	certs := &CERTSCell{
		Certificates: []*Certificate{
			{
				CertType: CertTypeRSAID,
				X509Cert: x509Cert,
			},
			{
				CertType:    CertTypeEd25519Signing,
				Ed25519Cert: ed25519Cert,
			},
		},
	}

	// Should pass when both match
	err = certs.ValidateRelayIdentity(expectedFingerprint, expectedIdentity)
	if err != nil {
		t.Errorf("ValidateRelayIdentity failed with both valid fingerprints: %v", err)
	}

	// Should fail when RSA matches but Ed25519 doesn't
	wrongIdentity := make([]byte, 32)
	for i := 0; i < 32; i++ {
		wrongIdentity[i] = byte(i + 100)
	}

	err = certs.ValidateRelayIdentity(expectedFingerprint, wrongIdentity)
	if err == nil {
		t.Error("Expected error when Ed25519 identity mismatches")
	}
}

// TestValidateRelayIdentity_EmptyInputs tests validation with empty inputs
func TestValidateRelayIdentity_EmptyInputs(t *testing.T) {
	certs := &CERTSCell{
		Certificates: []*Certificate{},
	}

	// Should pass when no validation requested (both empty)
	err := certs.ValidateRelayIdentity("", nil)
	if err != nil {
		t.Errorf("Expected no error with empty inputs, got: %v", err)
	}
}

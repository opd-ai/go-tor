package protocol

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"testing"
	"time"
)

// TestCERTSValidation_StrictModeExpiration tests that strict mode enforces certificate expiration
func TestCERTSValidation_StrictModeExpiration(t *testing.T) {
	// Create an expired Ed25519 certificate
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 key: %v", err)
	}

	// Create certificate that expired 48 hours ago
	// Note: Ed25519 cert expiration is in hours since Unix epoch, not seconds
	expiredTimeHours := uint32(time.Now().Add(-48*time.Hour).Unix() / 3600)
	certData := createTestEd25519Cert(t, pubKey, privKey, expiredTimeHours)

	// Parse the certificate using internal parser (via CERTS cell)
	ed25519Cert, err := parseEd25519Certificate(certData)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Create CERTS cell with expired certificate
	certsCell := &CERTSCell{
		Certificates: []*Certificate{
			{
				CertType:    CertTypeEd25519Signing,
				Ed25519Cert: ed25519Cert,
			},
		},
	}

	// Test ValidateExpiration() behavior
	err = certsCell.ValidateExpiration()
	if err == nil {
		t.Error("ValidateExpiration() should return error for expired certificate")
	}
}

// TestCERTSValidation_StrictModeSignature tests that strict mode enforces signature validation
func TestCERTSValidation_StrictModeSignature(t *testing.T) {
	// Create certificate with INVALID signature
	pubKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 key: %v", err)
	}

	// Create future expiration (in hours since epoch)
	expiryTimeHours := uint32(time.Now().Add(24*time.Hour).Unix() / 3600)

	// Build certificate with all-zero signature (invalid)
	certData := make([]byte, 104)
	certData[0] = 1 // Version
	certData[1] = 4 // CertType: Ed25519 signing key
	binary.BigEndian.PutUint32(certData[2:6], expiryTimeHours)
	certData[6] = 1 // CertKeyType: Ed25519
	copy(certData[7:39], pubKey)
	certData[39] = 0 // N_Extensions
	// Signature bytes 40-103 are all zeros (invalid)

	// Parse the certificate
	ed25519Cert, err := parseEd25519Certificate(certData)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Create CERTS cell
	certsCell := &CERTSCell{
		Certificates: []*Certificate{
			{
				CertType:    CertTypeEd25519Signing,
				Ed25519Cert: ed25519Cert,
			},
		},
	}

	// Test ValidateSignatures() behavior
	err = certsCell.ValidateSignatures()
	if err == nil {
		t.Error("ValidateSignatures() should return error for invalid signature")
	}
}

// TestCERTSValidation_StrictModeIdentity tests that strict mode enforces identity validation
func TestCERTSValidation_StrictModeIdentity(t *testing.T) {
	// Create valid certificate
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 key: %v", err)
	}

	expiryTimeHours := uint32(time.Now().Add(24*time.Hour).Unix() / 3600)
	certData := createTestEd25519Cert(t, pubKey, privKey, expiryTimeHours)

	// Parse the certificate
	ed25519Cert, err := parseEd25519Certificate(certData)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Create different expected identity (mismatch)
	wrongPubKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate wrong Ed25519 key: %v", err)
	}

	// Create CERTS cell
	certsCell := &CERTSCell{
		Certificates: []*Certificate{
			{
				CertType:    CertTypeEd25519Signing,
				Ed25519Cert: ed25519Cert,
			},
		},
	}

	// Test ValidateRelayIdentity() with wrong expected identity
	err = certsCell.ValidateRelayIdentity("", wrongPubKey)
	if err == nil {
		t.Error("ValidateRelayIdentity() should return error for identity mismatch")
	}

	// Test ValidateRelayIdentity() with correct expected identity
	err = certsCell.ValidateRelayIdentity("", pubKey)
	if err != nil {
		t.Errorf("ValidateRelayIdentity() should succeed with matching identity, got error: %v", err)
	}
}

// Helper function to create a signed Ed25519 certificate for testing
func createTestEd25519Cert(t *testing.T, pubKey ed25519.PublicKey, privKey ed25519.PrivateKey, expiryTimeHours uint32) []byte {
	certData := make([]byte, 104) // Version(1) + CertType(1) + Expiry(4) + CertKeyType(1) + CertifiedKey(32) + N_Extensions(1) + Signature(64)
	certData[0] = 1                // Version
	certData[1] = 4                // CertType: Ed25519 signing key (self-signed)
	binary.BigEndian.PutUint32(certData[2:6], expiryTimeHours)
	certData[6] = 1 // CertKeyType: Ed25519
	copy(certData[7:39], pubKey)
	certData[39] = 0 // N_Extensions

	// Sign the certificate data
	signedData := append([]byte("Tor node signing key certificate v1"), certData[0:40]...)
	signature := ed25519.Sign(privKey, signedData)
	copy(certData[40:104], signature)

	return certData
}

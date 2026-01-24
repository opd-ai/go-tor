package protocol

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
)

// TestEd25519CertificateVerifySignature tests Ed25519 signature verification
func TestEd25519CertificateVerifySignature(t *testing.T) {
	// Generate a real Ed25519 keypair for signing
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 keypair: %v", err)
	}

	// Create a certificate structure
	cert := &Ed25519Certificate{
		Version:      1,
		CertType:     4, // Ed25519 signing key
		ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
		CertKeyType:  1, // Ed25519 key type
		CertifiedKey: make([]byte, 32),
		Extensions:   []Ed25519Extension{},
	}
	copy(cert.CertifiedKey, publicKey)

	// Build the signed data (all fields before signature)
	signedData := make([]byte, 0, 256)
	signedData = append(signedData, cert.Version)
	signedData = append(signedData, cert.CertType)
	
	expirationHours := uint32(cert.ExpiresAt.Unix() / 3600)
	expBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(expBytes, expirationHours)
	signedData = append(signedData, expBytes...)
	
	signedData = append(signedData, cert.CertKeyType)
	signedData = append(signedData, cert.CertifiedKey...)
	signedData = append(signedData, byte(len(cert.Extensions)))

	// Sign the data
	signature := ed25519.Sign(privateKey, signedData)
	cert.Signature = signature

	// Test verification with correct key
	err = cert.VerifySignature(publicKey)
	if err != nil {
		t.Errorf("Signature verification failed with correct key: %v", err)
	}

	// Test verification with wrong key
	wrongKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate wrong keypair: %v", err)
	}
	
	err = cert.VerifySignature(wrongKey)
	if err == nil {
		t.Error("Signature verification should fail with wrong key")
	}
}

// TestEd25519CertificateVerifySignature_WithExtensions tests signature verification with extensions
func TestEd25519CertificateVerifySignature_WithExtensions(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 keypair: %v", err)
	}

	// Create certificate with extensions
	cert := &Ed25519Certificate{
		Version:      1,
		CertType:     5, // Ed25519 TLS link
		ExpiresAt:    time.Now().Add(30 * 24 * time.Hour),
		CertKeyType:  1,
		CertifiedKey: make([]byte, 32),
		Extensions: []Ed25519Extension{
			{
				ExtType: 1,
				Flags:   0,
				ExtData: []byte{0x01, 0x02, 0x03, 0x04},
			},
			{
				ExtType: 2,
				Flags:   1,
				ExtData: []byte{0xAA, 0xBB},
			},
		},
	}
	copy(cert.CertifiedKey, publicKey)

	// Build signed data with extensions
	signedData := make([]byte, 0, 256)
	signedData = append(signedData, cert.Version)
	signedData = append(signedData, cert.CertType)
	
	expirationHours := uint32(cert.ExpiresAt.Unix() / 3600)
	expBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(expBytes, expirationHours)
	signedData = append(signedData, expBytes...)
	
	signedData = append(signedData, cert.CertKeyType)
	signedData = append(signedData, cert.CertifiedKey...)
	signedData = append(signedData, byte(len(cert.Extensions)))
	
	for _, ext := range cert.Extensions {
		extLen := uint16(2 + len(ext.ExtData))
		extLenBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(extLenBytes, extLen)
		signedData = append(signedData, extLenBytes...)
		signedData = append(signedData, ext.ExtType)
		signedData = append(signedData, ext.Flags)
		signedData = append(signedData, ext.ExtData...)
	}

	signature := ed25519.Sign(privateKey, signedData)
	cert.Signature = signature

	err = cert.VerifySignature(publicKey)
	if err != nil {
		t.Errorf("Signature verification failed with extensions: %v", err)
	}
}

// TestEd25519CertificateVerifySignature_InvalidSignatureLength tests error handling
func TestEd25519CertificateVerifySignature_InvalidSignatureLength(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 keypair: %v", err)
	}

	cert := &Ed25519Certificate{
		Version:      1,
		CertType:     4,
		ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
		CertKeyType:  1,
		CertifiedKey: make([]byte, 32),
		Extensions:   []Ed25519Extension{},
		Signature:    make([]byte, 32), // Invalid: should be 64 bytes
	}

	err = cert.VerifySignature(publicKey)
	if err == nil {
		t.Error("Expected error for invalid signature length")
	}
}

// TestEd25519CertificateVerifySignature_InvalidKeyLength tests error handling
func TestEd25519CertificateVerifySignature_InvalidKeyLength(t *testing.T) {
	cert := &Ed25519Certificate{
		Version:      1,
		CertType:     4,
		ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
		CertKeyType:  1,
		CertifiedKey: make([]byte, 32),
		Extensions:   []Ed25519Extension{},
		Signature:    make([]byte, 64),
	}

	invalidKey := make([]byte, 16) // Invalid: should be 32 bytes
	err := cert.VerifySignature(invalidKey)
	if err == nil {
		t.Error("Expected error for invalid key length")
	}
}

// TestValidateSignatures tests the CERTSCell signature validation
func TestValidateSignatures(t *testing.T) {
	// Generate keypair for signing
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 keypair: %v", err)
	}

	// Create a self-signed Ed25519 signing key certificate (type 4)
	signingKeyCert := createSignedEd25519Cert(t, 4, publicKey, privateKey, publicKey)
	
	// Create CERTS cell with the signing key certificate
	certsCell := &CERTSCell{
		Certificates: []*Certificate{
			{
				CertType:    CertTypeEd25519Signing,
				Ed25519Cert: signingKeyCert,
			},
		},
	}

	// Validate signatures
	err = certsCell.ValidateSignatures()
	if err != nil {
		t.Errorf("Signature validation failed for valid self-signed cert: %v", err)
	}
}

// TestValidateSignatures_WithTLSLink tests signature validation with signing key and TLS link
func TestValidateSignatures_WithTLSLink(t *testing.T) {
	// Generate keypairs
	signingPubKey, signingPrivKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate signing keypair: %v", err)
	}

	linkPubKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate link keypair: %v", err)
	}

	// Create self-signed signing key cert (type 4)
	signingKeyCert := createSignedEd25519Cert(t, 4, signingPubKey, signingPrivKey, signingPubKey)
	
	// Create TLS link cert (type 5) signed by signing key
	tlsLinkCert := createSignedEd25519Cert(t, 5, linkPubKey, signingPrivKey, signingPubKey)

	// Create CERTS cell
	certsCell := &CERTSCell{
		Certificates: []*Certificate{
			{
				CertType:    CertTypeEd25519Signing,
				Ed25519Cert: signingKeyCert,
			},
			{
				CertType:    CertTypeEd25519TLSLink,
				Ed25519Cert: tlsLinkCert,
			},
		},
	}

	// Validate signatures
	err = certsCell.ValidateSignatures()
	if err != nil {
		t.Errorf("Signature validation failed for valid cert chain: %v", err)
	}
}

// TestValidateSignatures_MissingSigningKey tests error when TLS link has no signing key
func TestValidateSignatures_MissingSigningKey(t *testing.T) {
	linkPubKey, linkPrivKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate link keypair: %v", err)
	}

	// Create TLS link cert without a signing key cert in the cell
	tlsLinkCert := createSignedEd25519Cert(t, 5, linkPubKey, linkPrivKey, linkPubKey)

	certsCell := &CERTSCell{
		Certificates: []*Certificate{
			{
				CertType:    CertTypeEd25519TLSLink,
				Ed25519Cert: tlsLinkCert,
			},
		},
	}

	err = certsCell.ValidateSignatures()
	if err == nil {
		t.Error("Expected error when TLS link cert is present without signing key cert")
	}
}

// TestValidateSignatures_InvalidSignature tests rejection of invalid signatures
func TestValidateSignatures_InvalidSignature(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create a certificate with an invalid signature
	cert := &Ed25519Certificate{
		Version:      1,
		CertType:     4,
		ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
		CertKeyType:  1,
		CertifiedKey: make([]byte, 32),
		Extensions:   []Ed25519Extension{},
		Signature:    make([]byte, 64), // Random bytes, not a valid signature
	}
	copy(cert.CertifiedKey, publicKey)

	certsCell := &CERTSCell{
		Certificates: []*Certificate{
			{
				CertType:    CertTypeEd25519Signing,
				Ed25519Cert: cert,
			},
		},
	}

	err = certsCell.ValidateSignatures()
	if err == nil {
		t.Error("Expected error for invalid signature")
	}
}

// Helper function to create a properly signed Ed25519 certificate
func createSignedEd25519Cert(t *testing.T, certType uint8, certifiedKey, privateKey, publicKey []byte) *Ed25519Certificate {
	cert := &Ed25519Certificate{
		Version:      1,
		CertType:     certType,
		ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
		CertKeyType:  1,
		CertifiedKey: make([]byte, 32),
		Extensions:   []Ed25519Extension{},
	}
	copy(cert.CertifiedKey, certifiedKey)

	// Build signed data
	signedData := make([]byte, 0, 256)
	signedData = append(signedData, cert.Version)
	signedData = append(signedData, cert.CertType)
	
	expirationHours := uint32(cert.ExpiresAt.Unix() / 3600)
	expBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(expBytes, expirationHours)
	signedData = append(signedData, expBytes...)
	
	signedData = append(signedData, cert.CertKeyType)
	signedData = append(signedData, cert.CertifiedKey...)
	signedData = append(signedData, byte(len(cert.Extensions)))

	// Sign with private key
	signature := ed25519.Sign(ed25519.PrivateKey(privateKey), signedData)
	cert.Signature = signature

	return cert
}

// TestValidateSignatures_Integration tests full CERTS cell parsing and validation
func TestValidateSignatures_Integration(t *testing.T) {
	// Generate keypair
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Build a complete Ed25519 certificate in wire format
	certData := make([]byte, 0, 256)
	certData = append(certData, 1) // Version
	certData = append(certData, 4) // CertType (signing key)
	
	expirationHours := uint32(time.Now().Add(365*24*time.Hour).Unix() / 3600)
	expBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(expBytes, expirationHours)
	certData = append(certData, expBytes...)
	
	certData = append(certData, 1) // CertKeyType
	certData = append(certData, publicKey...) // CertifiedKey (32 bytes)
	certData = append(certData, 0) // No extensions

	// Sign the certificate data
	signature := ed25519.Sign(privateKey, certData)
	certData = append(certData, signature...)

	// Build CERTS cell payload
	payload := make([]byte, 0, 256)
	payload = append(payload, 1) // Number of certificates
	payload = append(payload, byte(CertTypeEd25519Signing)) // Cert type
	
	certLen := make([]byte, 2)
	binary.BigEndian.PutUint16(certLen, uint16(len(certData)))
	payload = append(payload, certLen...)
	payload = append(payload, certData...)

	// Create cell and parse
	cellData := cell.NewCell(0, cell.CmdCerts)
	cellData.Payload = payload

	certsCell, err := ParseCERTSCell(cellData)
	if err != nil {
		t.Fatalf("Failed to parse CERTS cell: %v", err)
	}

	// Validate signatures
	err = certsCell.ValidateSignatures()
	if err != nil {
		t.Errorf("Signature validation failed for integrated test: %v", err)
	}
}

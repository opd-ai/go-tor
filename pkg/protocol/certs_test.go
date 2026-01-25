package protocol

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"math/big"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
)

// TestParseCERTSCell tests basic CERTS cell parsing
func TestParseCERTSCell(t *testing.T) {
	// Create a minimal CERTS cell with 1 certificate
	payload := make([]byte, 100)
	payload[0] = 1                               // Number of certificates
	payload[1] = byte(CertTypeEd25519Signing)    // Cert type
	binary.BigEndian.PutUint16(payload[2:4], 40) // Cert length (minimal Ed25519 cert)

	// Minimal Ed25519 cert: version(1) + certType(1) + expiration(4) + keyType(1) + key(32) + numExt(1) = 40 bytes
	offset := 4
	payload[offset] = 1 // Version
	offset++
	payload[offset] = 4 // CertType (signing key)
	offset++
	// Expiration (1 year from now in hours)
	expirationHours := uint32(time.Now().Add(365*24*time.Hour).Unix() / 3600)
	binary.BigEndian.PutUint32(payload[offset:offset+4], expirationHours)
	offset += 4
	payload[offset] = 1 // CertKeyType (Ed25519)
	offset++
	// 32 bytes of key data
	for i := 0; i < 32; i++ {
		payload[offset+i] = byte(i)
	}
	offset += 32
	payload[offset] = 0 // No extensions

	cellData := cell.NewCell(0, cell.CmdCerts)
	cellData.Payload = payload[:offset+1]

	certs, err := ParseCERTSCell(cellData)
	if err != nil {
		t.Fatalf("Failed to parse CERTS cell: %v", err)
	}

	if len(certs.Certificates) != 1 {
		t.Errorf("Expected 1 certificate, got %d", len(certs.Certificates))
	}

	if certs.Certificates[0].CertType != CertTypeEd25519Signing {
		t.Errorf("Expected cert type %s, got %s", CertTypeEd25519Signing, certs.Certificates[0].CertType)
	}
}

// TestParseCERTSCell_WrongCommand tests error handling for wrong cell type
func TestParseCERTSCell_WrongCommand(t *testing.T) {
	cellData := cell.NewCell(0, cell.CmdNetinfo)
	cellData.Payload = []byte{1, 2, 3}

	_, err := ParseCERTSCell(cellData)
	if err == nil {
		t.Fatal("Expected error for wrong cell type")
	}
}

// TestParseCERTSCell_EmptyPayload tests error handling for empty payload
func TestParseCERTSCell_EmptyPayload(t *testing.T) {
	cellData := cell.NewCell(0, cell.CmdCerts)
	cellData.Payload = []byte{}

	_, err := ParseCERTSCell(cellData)
	if err == nil {
		t.Fatal("Expected error for empty payload")
	}
}

// TestParseCERTSCell_TruncatedHeader tests error handling for truncated cert header
func TestParseCERTSCell_TruncatedHeader(t *testing.T) {
	cellData := cell.NewCell(0, cell.CmdCerts)
	cellData.Payload = []byte{1, 2} // Num certs = 1, but only 1 byte of header

	_, err := ParseCERTSCell(cellData)
	if err == nil {
		t.Fatal("Expected error for truncated header")
	}
}

// TestParseCERTSCell_TruncatedBody tests error handling for truncated cert body
func TestParseCERTSCell_TruncatedBody(t *testing.T) {
	cellData := cell.NewCell(0, cell.CmdCerts)
	payload := make([]byte, 10)
	payload[0] = 1 // Num certs = 1
	payload[1] = byte(CertTypeEd25519Signing)
	binary.BigEndian.PutUint16(payload[2:4], 100) // Claim 100 bytes but don't provide them
	cellData.Payload = payload

	_, err := ParseCERTSCell(cellData)
	if err == nil {
		t.Fatal("Expected error for truncated body")
	}
}

// TestParseEd25519Certificate tests Ed25519 certificate parsing
func TestParseEd25519Certificate(t *testing.T) {
	// Create a minimal Ed25519 certificate
	data := make([]byte, 40+64) // Minimal cert + signature
	offset := 0

	// Version
	data[offset] = 1
	offset++

	// CertType
	data[offset] = 4
	offset++

	// Expiration (1 year from now)
	expirationHours := uint32(time.Now().Add(365*24*time.Hour).Unix() / 3600)
	binary.BigEndian.PutUint32(data[offset:offset+4], expirationHours)
	offset += 4

	// CertKeyType
	data[offset] = 1
	offset++

	// Certified key (32 bytes)
	for i := 0; i < 32; i++ {
		data[offset+i] = byte(i)
	}
	offset += 32

	// No extensions
	data[offset] = 0
	offset++

	// Signature (64 bytes)
	for i := 0; i < 64; i++ {
		data[offset+i] = byte(i + 100)
	}

	cert, err := parseEd25519Certificate(data)
	if err != nil {
		t.Fatalf("Failed to parse Ed25519 certificate: %v", err)
	}

	if cert.Version != 1 {
		t.Errorf("Expected version 1, got %d", cert.Version)
	}

	if cert.CertType != 4 {
		t.Errorf("Expected cert type 4, got %d", cert.CertType)
	}

	if len(cert.CertifiedKey) != 32 {
		t.Errorf("Expected 32-byte certified key, got %d bytes", len(cert.CertifiedKey))
	}

	if len(cert.Signature) != 64 {
		t.Errorf("Expected 64-byte signature, got %d bytes", len(cert.Signature))
	}
}

// TestParseEd25519Certificate_WithExtensions tests parsing with extensions
func TestParseEd25519Certificate_WithExtensions(t *testing.T) {
	data := make([]byte, 200)
	offset := 0

	// Version
	data[offset] = 1
	offset++

	// CertType
	data[offset] = 4
	offset++

	// Expiration
	expirationHours := uint32(time.Now().Add(365*24*time.Hour).Unix() / 3600)
	binary.BigEndian.PutUint32(data[offset:offset+4], expirationHours)
	offset += 4

	// CertKeyType
	data[offset] = 1
	offset++

	// Certified key (32 bytes)
	for i := 0; i < 32; i++ {
		data[offset+i] = byte(i)
	}
	offset += 32

	// 2 extensions
	data[offset] = 2
	offset++

	// Extension 1: 10 bytes total (2 for type+flags + 8 data)
	binary.BigEndian.PutUint16(data[offset:offset+2], 10)
	offset += 2
	data[offset] = 1 // ExtType
	offset++
	data[offset] = 0 // Flags
	offset++
	for i := 0; i < 8; i++ { // 10 total - 2 (type+flags) = 8 bytes data
		data[offset+i] = byte(i)
	}
	offset += 8

	// Extension 2: 5 bytes total (2 for type+flags + 3 data)
	binary.BigEndian.PutUint16(data[offset:offset+2], 5)
	offset += 2
	data[offset] = 2 // ExtType
	offset++
	data[offset] = 1 // Flags
	offset++
	for i := 0; i < 3; i++ { // 5 total - 2 (type+flags) = 3 bytes data
		data[offset+i] = byte(i + 10)
	}
	offset += 3

	// Signature (64 bytes)
	for i := 0; i < 64; i++ {
		data[offset+i] = byte(i + 100)
	}
	offset += 64

	cert, err := parseEd25519Certificate(data[:offset])
	if err != nil {
		t.Fatalf("Failed to parse Ed25519 certificate with extensions: %v", err)
	}

	if len(cert.Extensions) != 2 {
		t.Errorf("Expected 2 extensions, got %d", len(cert.Extensions))
	}

	if cert.Extensions[0].ExtType != 1 {
		t.Errorf("Expected extension 0 type 1, got %d", cert.Extensions[0].ExtType)
	}

	if len(cert.Extensions[0].ExtData) != 8 {
		t.Errorf("Expected extension 0 data length 8, got %d", len(cert.Extensions[0].ExtData))
	}

	if cert.Extensions[1].ExtType != 2 {
		t.Errorf("Expected extension 1 type 2, got %d", cert.Extensions[1].ExtType)
	}

	if len(cert.Extensions[1].ExtData) != 3 {
		t.Errorf("Expected extension 1 data length 3, got %d", len(cert.Extensions[1].ExtData))
	}
}

// TestParseEd25519Certificate_InvalidVersion tests error handling
func TestParseEd25519Certificate_InvalidVersion(t *testing.T) {
	data := make([]byte, 40)
	data[0] = 2 // Invalid version

	_, err := parseEd25519Certificate(data)
	if err == nil {
		t.Fatal("Expected error for invalid version")
	}
}

// TestFindCertificate tests finding certificates by type
func TestFindCertificate(t *testing.T) {
	certs := &CERTSCell{
		Certificates: []*Certificate{
			{CertType: CertTypeTLSLink},
			{CertType: CertTypeEd25519Signing},
			{CertType: CertTypeRSAID},
		},
	}

	cert := certs.FindCertificate(CertTypeEd25519Signing)
	if cert == nil {
		t.Fatal("Failed to find Ed25519 signing certificate")
	}

	if cert.CertType != CertTypeEd25519Signing {
		t.Errorf("Expected type %s, got %s", CertTypeEd25519Signing, cert.CertType)
	}

	// Try to find non-existent type
	cert = certs.FindCertificate(CertTypeEd25519Auth)
	if cert != nil {
		t.Error("Expected nil for non-existent certificate type")
	}
}

// TestValidateExpiration tests certificate expiration validation
func TestValidateExpiration(t *testing.T) {
	// Create a valid Ed25519 cert
	data := make([]byte, 40+64)
	offset := 0
	data[offset] = 1 // Version
	offset++
	data[offset] = 4 // CertType
	offset++

	// Expiration in future
	expirationHours := uint32(time.Now().Add(365*24*time.Hour).Unix() / 3600)
	binary.BigEndian.PutUint32(data[offset:offset+4], expirationHours)
	offset += 4

	data[offset] = 1 // CertKeyType
	offset++
	offset += 32     // Certified key
	data[offset] = 0 // No extensions
	offset++
	offset += 64 // Signature

	ed25519Cert, err := parseEd25519Certificate(data)
	if err != nil {
		t.Fatalf("Failed to parse Ed25519 cert: %v", err)
	}

	certs := &CERTSCell{
		Certificates: []*Certificate{
			{
				CertType:    CertTypeEd25519Signing,
				Ed25519Cert: ed25519Cert,
			},
		},
	}

	// Should pass - cert is valid
	if err := certs.ValidateExpiration(); err != nil {
		t.Errorf("Unexpected error for valid certificate: %v", err)
	}

	// Create an expired cert
	expiredData := make([]byte, 40+64)
	copy(expiredData, data)
	expiredHours := uint32(time.Now().Add(-365*24*time.Hour).Unix() / 3600)
	binary.BigEndian.PutUint32(expiredData[2:6], expiredHours)

	expiredCert, err := parseEd25519Certificate(expiredData)
	if err != nil {
		t.Fatalf("Failed to parse expired cert: %v", err)
	}

	expiredCerts := &CERTSCell{
		Certificates: []*Certificate{
			{
				CertType:    CertTypeEd25519Signing,
				Ed25519Cert: expiredCert,
			},
		},
	}

	// Should fail - cert is expired
	if err := expiredCerts.ValidateExpiration(); err == nil {
		t.Error("Expected error for expired certificate")
	}
}

// TestValidateRelayIdentity_Ed25519 tests Ed25519 identity validation
func TestValidateRelayIdentity_Ed25519(t *testing.T) {
	// Generate expected identity
	expectedIdentity := make([]byte, 32)
	for i := 0; i < 32; i++ {
		expectedIdentity[i] = byte(i)
	}

	// Create Ed25519 cert with matching identity
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

	// Copy expected identity as certified key
	copy(data[offset:offset+32], expectedIdentity)
	offset += 32

	data[offset] = 0 // No extensions
	offset++
	offset += 64 // Signature

	ed25519Cert, err := parseEd25519Certificate(data)
	if err != nil {
		t.Fatalf("Failed to parse Ed25519 cert: %v", err)
	}

	certs := &CERTSCell{
		Certificates: []*Certificate{
			{
				CertType:    CertTypeEd25519Signing,
				CertBody:    data,
				Ed25519Cert: ed25519Cert,
			},
		},
	}

	// Should pass - identity matches
	if err := certs.ValidateRelayIdentity("", expectedIdentity); err != nil {
		t.Errorf("Unexpected error for matching identity: %v", err)
	}

	// Should fail - wrong identity
	wrongIdentity := make([]byte, 32)
	for i := 0; i < 32; i++ {
		wrongIdentity[i] = byte(i + 100)
	}

	if err := certs.ValidateRelayIdentity("", wrongIdentity); err == nil {
		t.Error("Expected error for mismatched identity")
	}
}

// TestCertTypeString tests CertType string representation
func TestCertTypeString(t *testing.T) {
	tests := []struct {
		certType CertType
		expected string
	}{
		{CertTypeTLSLink, "TLS_LINK"},
		{CertTypeRSAID, "RSA_ID"},
		{CertTypeEd25519Signing, "ED25519_SIGNING"},
		{CertType(99), "UNKNOWN(99)"},
	}

	for _, tt := range tests {
		if got := tt.certType.String(); got != tt.expected {
			t.Errorf("CertType(%d).String() = %s, want %s", tt.certType, got, tt.expected)
		}
	}
}

// TestParseX509Certificate tests X.509 certificate parsing
func TestParseX509Certificate(t *testing.T) {
	// Generate a test RSA key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Create a self-signed certificate
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test-relay",
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	// Create CERTS cell with X.509 cert
	payload := make([]byte, 3+len(certDER)+1)
	payload[0] = 1 // Num certs
	payload[1] = byte(CertTypeTLSLink)
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(certDER)))
	copy(payload[4:], certDER)

	cellData := cell.NewCell(0, cell.CmdCerts)
	cellData.Payload = payload

	certs, err := ParseCERTSCell(cellData)
	if err != nil {
		t.Fatalf("Failed to parse CERTS cell with X.509: %v", err)
	}

	if len(certs.Certificates) != 1 {
		t.Errorf("Expected 1 certificate, got %d", len(certs.Certificates))
	}

	cert := certs.Certificates[0]
	if cert.X509Cert == nil {
		t.Fatal("Expected X.509 certificate to be parsed")
	}

	if cert.X509Cert.Subject.CommonName != "test-relay" {
		t.Errorf("Expected CN=test-relay, got %s", cert.X509Cert.Subject.CommonName)
	}
}

// TestMultipleCertificates tests parsing multiple certificates in one CERTS cell
func TestMultipleCertificates(t *testing.T) {
	// Create minimal Ed25519 cert data
	makeCert := func(certType CertType, keyByte byte) []byte {
		data := make([]byte, 40+64)
		offset := 0
		data[offset] = 1 // Version
		offset++
		data[offset] = byte(certType)
		offset++

		expirationHours := uint32(time.Now().Add(365*24*time.Hour).Unix() / 3600)
		binary.BigEndian.PutUint32(data[offset:offset+4], expirationHours)
		offset += 4

		data[offset] = 1 // CertKeyType
		offset++

		for i := 0; i < 32; i++ {
			data[offset+i] = keyByte
		}
		offset += 32

		data[offset] = 0 // No extensions

		return data
	}

	cert1 := makeCert(CertTypeEd25519Signing, 0x11)
	cert2 := makeCert(CertTypeEd25519TLSLink, 0x22)

	payload := make([]byte, 1+3+len(cert1)+3+len(cert2))
	offset := 0

	payload[offset] = 2 // Num certs
	offset++

	// Cert 1
	payload[offset] = byte(CertTypeEd25519Signing)
	offset++
	binary.BigEndian.PutUint16(payload[offset:offset+2], uint16(len(cert1)))
	offset += 2
	copy(payload[offset:], cert1)
	offset += len(cert1)

	// Cert 2
	payload[offset] = byte(CertTypeEd25519TLSLink)
	offset++
	binary.BigEndian.PutUint16(payload[offset:offset+2], uint16(len(cert2)))
	offset += 2
	copy(payload[offset:], cert2)

	cellData := cell.NewCell(0, cell.CmdCerts)
	cellData.Payload = payload

	certs, err := ParseCERTSCell(cellData)
	if err != nil {
		t.Fatalf("Failed to parse CERTS cell with multiple certs: %v", err)
	}

	if len(certs.Certificates) != 2 {
		t.Errorf("Expected 2 certificates, got %d", len(certs.Certificates))
	}

	if certs.Certificates[0].CertType != CertTypeEd25519Signing {
		t.Errorf("Expected first cert type %s, got %s", CertTypeEd25519Signing, certs.Certificates[0].CertType)
	}

	if certs.Certificates[1].CertType != CertTypeEd25519TLSLink {
		t.Errorf("Expected second cert type %s, got %s", CertTypeEd25519TLSLink, certs.Certificates[1].CertType)
	}
}

// TestEd25519RealKeypair tests parsing with actual Ed25519 keys
func TestEd25519RealKeypair(t *testing.T) {
	// Generate a real Ed25519 keypair
	pubKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 key: %v", err)
	}

	// Create cert with real public key
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

	copy(data[offset:offset+32], pubKey)
	offset += 32

	data[offset] = 0 // No extensions
	offset++
	offset += 64 // Signature

	cert, err := parseEd25519Certificate(data)
	if err != nil {
		t.Fatalf("Failed to parse Ed25519 cert with real key: %v", err)
	}

	if len(cert.CertifiedKey) != 32 {
		t.Errorf("Expected 32-byte certified key, got %d", len(cert.CertifiedKey))
	}

	// Verify key matches
	for i := 0; i < 32; i++ {
		if cert.CertifiedKey[i] != pubKey[i] {
			t.Errorf("Certified key mismatch at byte %d", i)
			break
		}
	}
}

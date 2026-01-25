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

// TestCertTypeStringComplete tests all CertType.String() values
func TestCertTypeStringComplete(t *testing.T) {
	if testing.Short() {
		tests := []struct {
			certType CertType
			expected string
		}{
			{CertTypeTLSLink, "TLS_LINK"},
			{CertTypeRSAID, "RSA_ID"},
			{CertTypeRSAAuth, "RSA_AUTH"},
			{CertTypeEd25519Signing, "ED25519_SIGNING"},
			{CertTypeEd25519TLSLink, "ED25519_TLS_LINK"},
			{CertTypeEd25519Auth, "ED25519_AUTH"},
			{CertTypeEd25519Identity, "ED25519_IDENTITY"},
			{CertType(99), "UNKNOWN(99)"},
			{CertType(255), "UNKNOWN(255)"},
		}

		for _, tt := range tests {
			t.Run(tt.expected, func(t *testing.T) {
				if got := tt.certType.String(); got != tt.expected {
					t.Errorf("CertType(%d).String() = %s, want %s", tt.certType, got, tt.expected)
				}
			})
		}
	}
}

// TestValidateExpirationX509 tests X.509 certificate expiration validation
func TestValidateExpirationX509(t *testing.T) {
	if testing.Short() {
		// Create expired X.509 cert
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("Failed to generate RSA key: %v", err)
		}

		// Test expired certificate
		expiredTemplate := x509.Certificate{
			SerialNumber: big.NewInt(1),
			Subject:      pkix.Name{CommonName: "expired-relay"},
			NotBefore:    time.Now().Add(-48 * time.Hour),
			NotAfter:     time.Now().Add(-24 * time.Hour), // Expired yesterday
			KeyUsage:     x509.KeyUsageDigitalSignature,
		}

		expiredDER, err := x509.CreateCertificate(rand.Reader, &expiredTemplate, &expiredTemplate, &privateKey.PublicKey, privateKey)
		if err != nil {
			t.Fatalf("Failed to create expired certificate: %v", err)
		}

		expiredCert, err := x509.ParseCertificate(expiredDER)
		if err != nil {
			t.Fatalf("Failed to parse expired certificate: %v", err)
		}

		certsCell := &CERTSCell{
			Certificates: []*Certificate{
				{
					CertType: CertTypeTLSLink,
					X509Cert: expiredCert,
				},
			},
		}

		err = certsCell.ValidateExpiration()
		if err == nil {
			t.Error("Expected error for expired X.509 certificate")
		}

		// Test not-yet-valid certificate
		futureTemplate := x509.Certificate{
			SerialNumber: big.NewInt(2),
			Subject:      pkix.Name{CommonName: "future-relay"},
			NotBefore:    time.Now().Add(24 * time.Hour), // Valid tomorrow
			NotAfter:     time.Now().Add(48 * time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature,
		}

		futureDER, err := x509.CreateCertificate(rand.Reader, &futureTemplate, &futureTemplate, &privateKey.PublicKey, privateKey)
		if err != nil {
			t.Fatalf("Failed to create future certificate: %v", err)
		}

		futureCert, err := x509.ParseCertificate(futureDER)
		if err != nil {
			t.Fatalf("Failed to parse future certificate: %v", err)
		}

		certsCell.Certificates[0].X509Cert = futureCert

		err = certsCell.ValidateExpiration()
		if err == nil {
			t.Error("Expected error for not-yet-valid X.509 certificate")
		}

		// Test valid certificate
		validTemplate := x509.Certificate{
			SerialNumber: big.NewInt(3),
			Subject:      pkix.Name{CommonName: "valid-relay"},
			NotBefore:    time.Now().Add(-1 * time.Hour),
			NotAfter:     time.Now().Add(24 * time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature,
		}

		validDER, err := x509.CreateCertificate(rand.Reader, &validTemplate, &validTemplate, &privateKey.PublicKey, privateKey)
		if err != nil {
			t.Fatalf("Failed to create valid certificate: %v", err)
		}

		validCert, err := x509.ParseCertificate(validDER)
		if err != nil {
			t.Fatalf("Failed to parse valid certificate: %v", err)
		}

		certsCell.Certificates[0].X509Cert = validCert

		err = certsCell.ValidateExpiration()
		if err != nil {
			t.Errorf("Expected no error for valid X.509 certificate, got: %v", err)
		}
	}
}

// TestValidateExpirationEd25519 tests Ed25519 certificate expiration validation
func TestValidateExpirationEd25519(t *testing.T) {
	if testing.Short() {
		// Create expired Ed25519 cert
		certsCell := &CERTSCell{
			Certificates: []*Certificate{
				{
					CertType: CertTypeEd25519Signing,
					Ed25519Cert: &Ed25519Certificate{
						Version:      1,
						CertType:     4,
						ExpiresAt:    time.Now().Add(-24 * time.Hour), // Expired yesterday
						CertKeyType:  1,
						CertifiedKey: make([]byte, 32),
					},
				},
			},
		}

		err := certsCell.ValidateExpiration()
		if err == nil {
			t.Error("Expected error for expired Ed25519 certificate")
		}

		// Test valid Ed25519 cert
		certsCell.Certificates[0].Ed25519Cert.ExpiresAt = time.Now().Add(24 * time.Hour)

		err = certsCell.ValidateExpiration()
		if err != nil {
			t.Errorf("Expected no error for valid Ed25519 certificate, got: %v", err)
		}

		// Test empty CERTS cell
		emptyCerts := &CERTSCell{Certificates: []*Certificate{}}
		err = emptyCerts.ValidateExpiration()
		if err != nil {
			t.Errorf("Expected no error for empty CERTS cell, got: %v", err)
		}

		// Test cert with neither X509 nor Ed25519
		mixedCerts := &CERTSCell{
			Certificates: []*Certificate{
				{CertType: CertTypeTLSLink},
			},
		}
		err = mixedCerts.ValidateExpiration()
		if err != nil {
			t.Errorf("Expected no error for cert without parsed data, got: %v", err)
		}
	}
}

// TestValidateSignaturesUnit tests ValidateSignatures function
func TestValidateSignaturesUnit(t *testing.T) {
	if testing.Short() {
		// Generate Ed25519 key pair for testing
		pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatalf("Failed to generate Ed25519 key: %v", err)
		}

		// Helper to create signed Ed25519 certificate
		createSignedCert := func(certType uint8, certifiedKey []byte, signingKey ed25519.PrivateKey) *Ed25519Certificate {
			expiresAt := time.Now().Add(24 * time.Hour)
			expirationHours := uint32(expiresAt.Unix() / 3600)

			signedData := make([]byte, 0, 256)
			signedData = append(signedData, 1)        // Version
			signedData = append(signedData, certType) // CertType

			expBytes := make([]byte, 4)
			binary.BigEndian.PutUint32(expBytes, expirationHours)
			signedData = append(signedData, expBytes...)

			signedData = append(signedData, 1)               // CertKeyType
			signedData = append(signedData, certifiedKey...) // CertifiedKey (32 bytes)
			signedData = append(signedData, 0)               // NumExtensions

			signature := ed25519.Sign(signingKey, signedData)

			return &Ed25519Certificate{
				Version:      1,
				CertType:     certType,
				ExpiresAt:    expiresAt,
				CertKeyType:  1,
				CertifiedKey: certifiedKey,
				Extensions:   []Ed25519Extension{},
				Signature:    signature,
			}
		}

		// Test Ed25519 signing key (self-signed)
		signingCert := createSignedCert(4, pubKey, privKey)
		certsCell := &CERTSCell{
			Certificates: []*Certificate{
				{CertType: CertTypeEd25519Signing, Ed25519Cert: signingCert},
			},
		}
		err = certsCell.ValidateSignatures()
		if err != nil {
			t.Errorf("Expected valid self-signed signing cert, got: %v", err)
		}

		// Test Ed25519 TLS link cert (signed by signing key)
		tlsPubKey, tlsPrivKey, _ := ed25519.GenerateKey(rand.Reader)
		tlsCert := createSignedCert(5, tlsPubKey, privKey) // Signed by signing key

		certsCell.Certificates = append(certsCell.Certificates, &Certificate{
			CertType:    CertTypeEd25519TLSLink,
			Ed25519Cert: tlsCert,
		})

		err = certsCell.ValidateSignatures()
		if err != nil {
			t.Errorf("Expected valid TLS link cert, got: %v", err)
		}

		// Test Ed25519 auth cert (signed by signing key)
		authPubKey, _, _ := ed25519.GenerateKey(rand.Reader)
		authCert := createSignedCert(6, authPubKey, privKey) // Signed by signing key

		certsCell.Certificates = append(certsCell.Certificates, &Certificate{
			CertType:    CertTypeEd25519Auth,
			Ed25519Cert: authCert,
		})

		err = certsCell.ValidateSignatures()
		if err != nil {
			t.Errorf("Expected valid auth cert, got: %v", err)
		}

		// Test TLS link cert without signing cert (should fail)
		tlsOnly := &CERTSCell{
			Certificates: []*Certificate{
				{CertType: CertTypeEd25519TLSLink, Ed25519Cert: tlsCert},
			},
		}
		err = tlsOnly.ValidateSignatures()
		if err == nil {
			t.Error("Expected error for TLS link cert without signing cert")
		}

		// Test auth cert without signing cert (should fail)
		authOnly := &CERTSCell{
			Certificates: []*Certificate{
				{CertType: CertTypeEd25519Auth, Ed25519Cert: authCert},
			},
		}
		err = authOnly.ValidateSignatures()
		if err == nil {
			t.Error("Expected error for auth cert without signing cert")
		}

		// Test TLS link cert with wrong signature
		wrongTLSCert := createSignedCert(5, tlsPubKey, tlsPrivKey) // Self-signed instead
		wrongTLSCell := &CERTSCell{
			Certificates: []*Certificate{
				{CertType: CertTypeEd25519Signing, Ed25519Cert: signingCert},
				{CertType: CertTypeEd25519TLSLink, Ed25519Cert: wrongTLSCert},
			},
		}
		err = wrongTLSCell.ValidateSignatures()
		if err == nil {
			t.Error("Expected error for TLS cert with wrong signature")
		}

		// Test CERTSCell.ValidateSignatures with nil Ed25519Cert (should skip)
		nilCell := &CERTSCell{
			Certificates: []*Certificate{
				{CertType: CertTypeTLSLink, Ed25519Cert: nil},
			},
		}
		err = nilCell.ValidateSignatures()
		if err != nil {
			t.Errorf("Expected no error for nil Ed25519Cert, got: %v", err)
		}

		// Test empty certificates
		emptyCerts := &CERTSCell{Certificates: []*Certificate{}}
		err = emptyCerts.ValidateSignatures()
		if err != nil {
			t.Errorf("Expected no error for empty certificates, got: %v", err)
		}
	}
}

// TestEd25519VerifySignatureErrors tests error conditions in VerifySignature
func TestEd25519VerifySignatureErrors(t *testing.T) {
	if testing.Short() {
		ed25519Cert := &Ed25519Certificate{
			Version:      1,
			CertType:     4,
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			CertKeyType:  1,
			CertifiedKey: make([]byte, 32),
			Extensions:   []Ed25519Extension{},
			Signature:    make([]byte, 64),
		}

		// Test invalid signing key length
		err := ed25519Cert.VerifySignature(make([]byte, 16))
		if err == nil {
			t.Error("Expected error for invalid signing key length")
		}

		// Test invalid signature length
		ed25519Cert.Signature = make([]byte, 32) // Wrong length
		err = ed25519Cert.VerifySignature(make([]byte, 32))
		if err == nil {
			t.Error("Expected error for invalid signature length")
		}

		// Test valid lengths but invalid signature (use actual data that won't verify)
		ed25519Cert.Signature = make([]byte, 64)
		for i := range ed25519Cert.Signature {
			ed25519Cert.Signature[i] = byte(i) // Non-zero pattern
		}
		validKey := make([]byte, 32)
		for i := range validKey {
			validKey[i] = byte(i + 1)
		}
		err = ed25519Cert.VerifySignature(validKey)
		if err == nil {
			t.Error("Expected error for invalid signature")
		}
	}
}

// TestEd25519CertWithExtensions tests signature verification with extensions
func TestEd25519CertWithExtensions(t *testing.T) {
	if testing.Short() {
		pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatalf("Failed to generate Ed25519 key: %v", err)
		}

		expiresAt := time.Now().Add(24 * time.Hour)
		expirationHours := uint32(expiresAt.Unix() / 3600)

		// Create extension
		ext := Ed25519Extension{
			ExtType: 1,
			Flags:   0,
			ExtData: []byte{0x01, 0x02, 0x03},
		}

		// Build signed data with extension
		signedData := make([]byte, 0, 256)
		signedData = append(signedData, 1) // Version
		signedData = append(signedData, 4) // CertType

		expBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(expBytes, expirationHours)
		signedData = append(signedData, expBytes...)

		signedData = append(signedData, 1)         // CertKeyType
		signedData = append(signedData, pubKey...) // CertifiedKey
		signedData = append(signedData, 1)         // NumExtensions

		// Extension length (2 + len(ExtData))
		extLen := uint16(2 + len(ext.ExtData))
		extLenBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(extLenBytes, extLen)
		signedData = append(signedData, extLenBytes...)

		signedData = append(signedData, ext.ExtType)
		signedData = append(signedData, ext.Flags)
		signedData = append(signedData, ext.ExtData...)

		// Sign the data
		signature := ed25519.Sign(privKey, signedData)

		ed25519Cert := &Ed25519Certificate{
			Version:      1,
			CertType:     4,
			ExpiresAt:    expiresAt,
			CertKeyType:  1,
			CertifiedKey: pubKey,
			Extensions:   []Ed25519Extension{ext},
			Signature:    signature,
		}

		// Verify signature
		err = ed25519Cert.VerifySignature(pubKey)
		if err != nil {
			t.Errorf("Expected valid signature with extensions, got error: %v", err)
		}
	}
}

// TestParseCERTSCellErrors tests error handling in ParseCERTSCell
func TestParseCERTSCellErrors(t *testing.T) {
	if testing.Short() {
		// Test empty payload
		emptyCell := cell.NewCell(0, cell.CmdCerts)
		emptyCell.Payload = []byte{}

		_, err := ParseCERTSCell(emptyCell)
		if err == nil {
			t.Error("Expected error for empty payload")
		}

		// Test truncated payload (missing certificate data)
		truncatedCell := cell.NewCell(0, cell.CmdCerts)
		truncatedCell.Payload = []byte{1, 1, 0, 10} // Says cert is 10 bytes but no data

		_, err = ParseCERTSCell(truncatedCell)
		if err == nil {
			t.Error("Expected error for truncated payload")
		}

		// Test valid empty CERTS cell (0 certificates)
		validEmpty := cell.NewCell(0, cell.CmdCerts)
		validEmpty.Payload = []byte{0}

		certs, err := ParseCERTSCell(validEmpty)
		if err != nil {
			t.Errorf("Expected no error for 0 certificates, got: %v", err)
		}
		if len(certs.Certificates) != 0 {
			t.Errorf("Expected 0 certificates, got %d", len(certs.Certificates))
		}
	}
}

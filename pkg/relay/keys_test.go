package relay

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateRelayKeys(t *testing.T) {
	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate relay keys: %v", err)
	}
	defer keys.Destroy()

	// Verify Ed25519 key
	if len(keys.Ed25519Public) != ed25519.PublicKeySize {
		t.Errorf("Invalid Ed25519 public key size: got %d, want %d", len(keys.Ed25519Public), ed25519.PublicKeySize)
	}
	if len(keys.Ed25519Private) != ed25519.PrivateKeySize {
		t.Errorf("Invalid Ed25519 private key size: got %d, want %d", len(keys.Ed25519Private), ed25519.PrivateKeySize)
	}

	// Verify Ed25519 key consistency
	derivedPub := keys.Ed25519Private.Public().(ed25519.PublicKey)
	if string(derivedPub) != string(keys.Ed25519Public) {
		t.Error("Ed25519 public key doesn't match private key")
	}

	// Verify RSA key
	if keys.RSAPrivate == nil {
		t.Fatal("RSA private key is nil")
	}
	if keys.RSAPrivate.N.BitLen() != 1024 {
		t.Errorf("Invalid RSA key size: got %d bits, want 1024", keys.RSAPrivate.N.BitLen())
	}

	// Verify TLS certificate
	if len(keys.TLSCert) == 0 {
		t.Fatal("TLS certificate is empty")
	}

	// Parse certificate to verify it's valid
	cert, err := x509.ParseCertificate(keys.TLSCert)
	if err != nil {
		t.Fatalf("Failed to parse TLS certificate: %v", err)
	}

	// Verify certificate fields
	if len(cert.Subject.Organization) == 0 || cert.Subject.Organization[0] != "Tor" {
		t.Errorf("Invalid certificate organization: got %v, want ['Tor']", cert.Subject.Organization)
	}
	if cert.Subject.CommonName != "www.torproject.org" {
		t.Errorf("Invalid certificate CN: got %s, want www.torproject.org", cert.Subject.CommonName)
	}
}

func TestRelayKeysFingerprint(t *testing.T) {
	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate relay keys: %v", err)
	}
	defer keys.Destroy()

	// Test RSA fingerprint
	fp := keys.Fingerprint()
	if len(fp) != 40 {
		t.Errorf("Invalid fingerprint length: got %d, want 40", len(fp))
	}

	// Fingerprint should be hex string
	for _, c := range fp {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Invalid hex character in fingerprint: %c", c)
		}
	}

	// Test Ed25519 fingerprint
	ed25519FP := keys.Ed25519Fingerprint()
	if len(ed25519FP) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("Invalid Ed25519 fingerprint length: got %d, want 64", len(ed25519FP))
	}
}

func TestSaveAndLoadKeys(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Generate keys
	originalKeys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate relay keys: %v", err)
	}

	// Save keys
	if err := originalKeys.SaveKeys(tmpDir); err != nil {
		t.Fatalf("Failed to save keys: %v", err)
	}

	// Verify files exist with correct permissions
	checkFile := func(filename string, expectedPerm os.FileMode) {
		path := filepath.Join(tmpDir, filename)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("File %s doesn't exist: %v", filename, err)
			return
		}
		perm := info.Mode().Perm()
		if perm != expectedPerm {
			t.Errorf("File %s has wrong permissions: got %o, want %o", filename, perm, expectedPerm)
		}
	}

	checkFile("ed25519_identity_secret_key", 0o600)
	checkFile("rsa_identity_secret_key", 0o600)
	checkFile("tls_certificate.pem", 0o644)

	// Load keys
	loadedKeys, err := LoadKeys(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load keys: %v", err)
	}
	defer loadedKeys.Destroy()

	// Verify Ed25519 keys match
	if string(originalKeys.Ed25519Public) != string(loadedKeys.Ed25519Public) {
		t.Error("Ed25519 public keys don't match")
	}
	if string(originalKeys.Ed25519Private) != string(loadedKeys.Ed25519Private) {
		t.Error("Ed25519 private keys don't match")
	}

	// Verify RSA keys match
	if originalKeys.RSAPrivate.N.Cmp(loadedKeys.RSAPrivate.N) != 0 {
		t.Error("RSA keys don't match")
	}

	// Verify TLS certificates match
	if string(originalKeys.TLSCert) != string(loadedKeys.TLSCert) {
		t.Error("TLS certificates don't match")
	}

	// Verify fingerprints match
	if originalKeys.Fingerprint() != loadedKeys.Fingerprint() {
		t.Error("Fingerprints don't match")
	}

	originalKeys.Destroy()
}

func TestLoadKeysErrors(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(string) error
		wantErr string
	}{
		{
			name:    "missing directory",
			setup:   func(dir string) error { return nil },
			wantErr: "failed to read Ed25519 key",
		},
		{
			name: "invalid ed25519 key size",
			setup: func(dir string) error {
				os.MkdirAll(dir, 0o700)
				return os.WriteFile(filepath.Join(dir, "ed25519_identity_secret_key"), []byte("short"), 0o600)
			},
			wantErr: "invalid Ed25519 key size",
		},
		{
			name: "invalid rsa pem",
			setup: func(dir string) error {
				os.MkdirAll(dir, 0o700)
				// Write valid ed25519 key
				keys, _ := GenerateRelayKeys()
				os.WriteFile(filepath.Join(dir, "ed25519_identity_secret_key"), keys.Ed25519Private, 0o600)
				// Write invalid RSA PEM
				return os.WriteFile(filepath.Join(dir, "rsa_identity_secret_key"), []byte("invalid"), 0o600)
			},
			wantErr: "invalid RSA key PEM format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if err := tt.setup(tmpDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			_, err := LoadKeys(tmpDir)
			if err == nil {
				t.Error("Expected error, got nil")
			} else if tt.wantErr != "" && !contains(err.Error(), tt.wantErr) {
				t.Errorf("Error doesn't contain %q: got %v", tt.wantErr, err)
			}
		})
	}
}

func TestKeysDestroy(t *testing.T) {
	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate relay keys: %v", err)
	}

	keys.Destroy()

	// Verify keys are zeroed
	if keys.Ed25519Private != nil {
		t.Error("Ed25519 private key not nil after Destroy")
	}
	if keys.Ed25519Public != nil {
		t.Error("Ed25519 public key not nil after Destroy")
	}
	if keys.RSAPrivate != nil {
		t.Error("RSA private key not nil after Destroy")
	}
	if keys.TLSCert != nil {
		t.Error("TLS cert not nil after Destroy")
	}
}

func TestFingerprintWithNilKeys(t *testing.T) {
	keys := &RelayKeys{}

	fp := keys.Fingerprint()
	if fp != "" {
		t.Errorf("Expected empty fingerprint for nil RSA key, got %s", fp)
	}

	ed25519FP := keys.Ed25519Fingerprint()
	if ed25519FP != "" {
		t.Errorf("Expected empty Ed25519 fingerprint for nil key, got %s", ed25519FP)
	}
}

func TestTLSCertificatePEMFormat(t *testing.T) {
	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate relay keys: %v", err)
	}
	defer keys.Destroy()

	tmpDir := t.TempDir()
	if err := keys.SaveKeys(tmpDir); err != nil {
		t.Fatalf("Failed to save keys: %v", err)
	}

	// Read and verify TLS certificate PEM
	certPath := filepath.Join(tmpDir, "tls_certificate.pem")
	pemData, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("Failed to read certificate file: %v", err)
	}

	block, _ := pem.Decode(pemData)
	if block == nil {
		t.Fatal("Failed to decode PEM block")
	}
	if block.Type != "CERTIFICATE" {
		t.Errorf("Invalid PEM block type: got %s, want CERTIFICATE", block.Type)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Verify self-signed (issuer == subject)
	if cert.Issuer.String() != cert.Subject.String() {
		t.Error("Certificate is not self-signed")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && s[0:] != s[0:0] && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

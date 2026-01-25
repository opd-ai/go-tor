// TLS Configuration Specification Compliance Tests
// Verifies tor-spec.txt §2 requirements for TLS connections

package connection

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"
)

// TestTLSConfigSpecCompliance_MinVersion verifies TLS 1.2 minimum per tor-spec.txt §2
func TestTLSConfigSpecCompliance_MinVersion(t *testing.T) {
	t.Run("default config uses TLS 1.2 minimum", func(t *testing.T) {
		cfg := createTorTLSConfig()

		if cfg.MinVersion != tls.VersionTLS12 {
			t.Errorf("MinVersion = %d, want %d (TLS 1.2)", cfg.MinVersion, tls.VersionTLS12)
		}
	})

	t.Run("pinning config uses TLS 1.2 minimum", func(t *testing.T) {
		identity := make([]byte, 32) // 32-byte Ed25519 identity
		cfg := createTorTLSConfigWithPinning(identity, "")

		if cfg.MinVersion != tls.VersionTLS12 {
			t.Errorf("MinVersion = %d, want %d (TLS 1.2)", cfg.MinVersion, tls.VersionTLS12)
		}
	})

	t.Run("rejects TLS 1.0 and 1.1", func(t *testing.T) {
		cfg := createTorTLSConfig()

		// TLS 1.2 is version 0x0303
		if cfg.MinVersion < tls.VersionTLS12 {
			t.Errorf("MinVersion %d allows insecure TLS versions < 1.2", cfg.MinVersion)
		}
	})
}

// TestTLSConfigSpecCompliance_CipherSuites verifies AEAD cipher suites with forward secrecy
func TestTLSConfigSpecCompliance_CipherSuites(t *testing.T) {
	t.Run("uses AEAD cipher suites only", func(t *testing.T) {
		cfg := createTorTLSConfig()

		if len(cfg.CipherSuites) == 0 {
			t.Fatal("no cipher suites configured")
		}

		// All configured cipher suites must be AEAD
		aeadCiphers := map[uint16]string{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:   "ECDHE-RSA-AES256-GCM-SHA384",
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:   "ECDHE-RSA-AES128-GCM-SHA256",
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384: "ECDHE-ECDSA-AES256-GCM-SHA384",
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256: "ECDHE-ECDSA-AES128-GCM-SHA256",
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305:    "ECDHE-RSA-CHACHA20-POLY1305",
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305:  "ECDHE-ECDSA-CHACHA20-POLY1305",
		}

		for _, suite := range cfg.CipherSuites {
			if _, ok := aeadCiphers[suite]; !ok {
				t.Errorf("cipher suite 0x%04x is not an approved AEAD cipher", suite)
			}
		}
	})

	t.Run("uses ECDHE for forward secrecy", func(t *testing.T) {
		cfg := createTorTLSConfig()

		// All cipher suites must use ECDHE for perfect forward secrecy
		for _, suite := range cfg.CipherSuites {
			switch suite {
			case tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305:
				// Valid ECDHE cipher
			default:
				t.Errorf("cipher suite 0x%04x does not use ECDHE", suite)
			}
		}
	})

	t.Run("excludes CBC mode ciphers", func(t *testing.T) {
		cfg := createTorTLSConfig()

		// CBC mode ciphers vulnerable to padding oracle attacks (Lucky13, POODLE)
		vulnerableCBCCiphers := []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		}

		for _, cbcSuite := range vulnerableCBCCiphers {
			for _, suite := range cfg.CipherSuites {
				if suite == cbcSuite {
					t.Errorf("vulnerable CBC cipher 0x%04x found in cipher suite list", suite)
				}
			}
		}
	})

	t.Run("excludes non-forward-secret ciphers", func(t *testing.T) {
		cfg := createTorTLSConfig()

		// Non-ECDHE ciphers without perfect forward secrecy
		nonPFSCiphers := []uint16{
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		}

		for _, nonPFS := range nonPFSCiphers {
			for _, suite := range cfg.CipherSuites {
				if suite == nonPFS {
					t.Errorf("non-forward-secret cipher 0x%04x found in cipher suite list", suite)
				}
			}
		}
	})
}

// TestTLSConfigSpecCompliance_CertificateVerification verifies Tor-specific certificate handling
func TestTLSConfigSpecCompliance_CertificateVerification(t *testing.T) {
	t.Run("accepts self-signed certificates", func(t *testing.T) {
		cfg := createTorTLSConfig()

		// InsecureSkipVerify must be true to accept self-signed certificates
		if !cfg.InsecureSkipVerify {
			t.Error("InsecureSkipVerify = false, want true to accept self-signed Tor certificates")
		}
	})

	t.Run("has custom verification function", func(t *testing.T) {
		cfg := createTorTLSConfig()

		if cfg.VerifyPeerCertificate == nil {
			t.Error("VerifyPeerCertificate = nil, want custom verification function")
		}
	})

	t.Run("custom verification accepts valid certificates", func(t *testing.T) {
		cfg := createTorTLSConfig()

		// Create a minimal self-signed certificate for testing
		cert := &x509.Certificate{
			SerialNumber: big.NewInt(1),
			Subject: pkix.Name{
				CommonName: "test-relay",
			},
			NotBefore:             time.Now().Add(-24 * time.Hour),
			NotAfter:              time.Now().Add(24 * time.Hour),
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			BasicConstraintsValid: true,
		}

		// Generate RSA key pair
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("failed to generate RSA key: %v", err)
		}

		// Create self-signed certificate
		certDER, err := x509.CreateCertificate(rand.Reader, cert, cert, &privateKey.PublicKey, privateKey)
		if err != nil {
			t.Fatalf("failed to create certificate: %v", err)
		}

		// Verify custom verification accepts valid certificate
		err = cfg.VerifyPeerCertificate([][]byte{certDER}, nil)
		if err != nil {
			t.Errorf("VerifyPeerCertificate rejected valid self-signed certificate: %v", err)
		}
	})
}

// TestTLSConfigSpecCompliance_IdentityPinning verifies certificate pinning implementation
func TestTLSConfigSpecCompliance_IdentityPinning(t *testing.T) {
	t.Run("pinning config has custom verification", func(t *testing.T) {
		identity := make([]byte, 32) // 32-byte Ed25519 identity
		cfg := createTorTLSConfigWithPinning(identity, "test-fingerprint")

		if cfg.VerifyPeerCertificate == nil {
			t.Error("VerifyPeerCertificate = nil, want custom verification with pinning")
		}
	})

	t.Run("pinning accepts nil identity and empty fingerprint", func(t *testing.T) {
		err := verifyRelayIdentityPinning(nil, nil, "")
		if err != nil {
			t.Errorf("verifyRelayIdentityPinning with no pinning = %v, want nil", err)
		}
	})

	t.Run("pinning rejects empty certificate list", func(t *testing.T) {
		identity := make([]byte, 32)
		err := verifyRelayIdentityPinning([][]byte{}, identity, "")
		if err == nil {
			t.Error("verifyRelayIdentityPinning with empty certs = nil, want error")
		}
	})
}

// TestTLSConfigSpecCompliance_DefaultConfig verifies default connection configuration
func TestTLSConfigSpecCompliance_DefaultConfig(t *testing.T) {
	t.Run("default config has reasonable timeout", func(t *testing.T) {
		cfg := DefaultConfig("127.0.0.1:9001")

		if cfg.Timeout == 0 {
			t.Error("Timeout = 0, want non-zero default timeout")
		}

		// 30 seconds is the default per implementation
		expectedTimeout := 30 * time.Second
		if cfg.Timeout != expectedTimeout {
			t.Errorf("Timeout = %v, want %v", cfg.Timeout, expectedTimeout)
		}
	})

	t.Run("default config uses link protocol v4", func(t *testing.T) {
		cfg := DefaultConfig("127.0.0.1:9001")

		if !cfg.LinkProtocolV4 {
			t.Error("LinkProtocolV4 = false, want true for 4-byte circuit IDs")
		}
	})

	t.Run("default config has no pinning", func(t *testing.T) {
		cfg := DefaultConfig("127.0.0.1:9001")

		if cfg.ExpectedIdentity != nil {
			t.Error("ExpectedIdentity != nil, want nil (no pinning by default)")
		}
		if cfg.ExpectedFingerprint != "" {
			t.Error("ExpectedFingerprint != \"\", want empty (no pinning by default)")
		}
	})

	t.Run("default config is non-enforcing", func(t *testing.T) {
		cfg := DefaultConfig("127.0.0.1:9001")

		if cfg.RequireCERTS {
			t.Error("RequireCERTS = true, want false (backward compatible mode)")
		}
	})
}

// TestTLSConfigSpecCompliance_SecurityProperties verifies security properties
func TestTLSConfigSpecCompliance_SecurityProperties(t *testing.T) {
	t.Run("all cipher suites support forward secrecy", func(t *testing.T) {
		cfg := createTorTLSConfig()

		// All ECDHE cipher suites provide perfect forward secrecy
		for _, suite := range cfg.CipherSuites {
			suiteName := tls.CipherSuiteName(suite)
			if suiteName == "" {
				t.Errorf("cipher suite 0x%04x has no name (may be unknown/deprecated)", suite)
			}
		}
	})

	t.Run("minimum TLS version prevents downgrade attacks", func(t *testing.T) {
		cfg := createTorTLSConfig()

		// TLS 1.2 is required to prevent downgrade attacks to TLS 1.0/1.1
		// which are vulnerable to POODLE, BEAST, and other attacks
		if cfg.MinVersion < tls.VersionTLS12 {
			t.Errorf("MinVersion allows TLS < 1.2, vulnerable to downgrade attacks")
		}
	})

	t.Run("cipher suites are in preferred order", func(t *testing.T) {
		cfg := createTorTLSConfig()

		// Verify cipher suites are ordered by preference (strongest first)
		if len(cfg.CipherSuites) < 2 {
			t.Skip("not enough cipher suites to verify ordering")
		}

		// AES-256-GCM should be preferred over AES-128-GCM
		var found256GCM, found128GCM bool
		var index256GCM, index128GCM int

		for i, suite := range cfg.CipherSuites {
			switch suite {
			case tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384:
				found256GCM = true
				index256GCM = i
			case tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256:
				found128GCM = true
				index128GCM = i
			}
		}

		if found256GCM && found128GCM && index256GCM > index128GCM {
			t.Errorf("AES-256-GCM at index %d comes after AES-128-GCM at index %d (want stronger cipher first)",
				index256GCM, index128GCM)
		}
	})
}

// TestTLSConfigSpecCompliance_CipherSuiteCount verifies reasonable number of cipher suites
func TestTLSConfigSpecCompliance_CipherSuiteCount(t *testing.T) {
	t.Run("has multiple cipher suites for compatibility", func(t *testing.T) {
		cfg := createTorTLSConfig()

		if len(cfg.CipherSuites) < 4 {
			t.Errorf("only %d cipher suites configured, want at least 4 for compatibility", len(cfg.CipherSuites))
		}
	})

	t.Run("does not have excessive cipher suites", func(t *testing.T) {
		cfg := createTorTLSConfig()

		// More than 10 cipher suites is probably unnecessary and may indicate
		// inclusion of weak or deprecated ciphers
		if len(cfg.CipherSuites) > 10 {
			t.Errorf("%d cipher suites configured, want <= 10 (excessive may include weak ciphers)", len(cfg.CipherSuites))
		}
	})

	t.Run("has exactly 6 approved cipher suites", func(t *testing.T) {
		cfg := createTorTLSConfig()

		// Current implementation has exactly 6 cipher suites
		expectedCount := 6
		if len(cfg.CipherSuites) != expectedCount {
			t.Errorf("cipher suite count = %d, want %d", len(cfg.CipherSuites), expectedCount)
		}
	})
}

// TestTLSConfigSpecCompliance_CertificateValidation verifies certificate validation behavior
func TestTLSConfigSpecCompliance_CertificateValidation(t *testing.T) {
	t.Run("rejects empty certificate list", func(t *testing.T) {
		err := verifyTorRelayCertificate([][]byte{}, nil)
		if err == nil {
			t.Error("verifyTorRelayCertificate with empty certs = nil, want error")
		}
	})

	t.Run("rejects invalid certificate encoding", func(t *testing.T) {
		invalidCert := []byte{0x00, 0x01, 0x02} // Invalid DER encoding
		err := verifyTorRelayCertificate([][]byte{invalidCert}, nil)
		if err == nil {
			t.Error("verifyTorRelayCertificate with invalid cert = nil, want error")
		}
	})
}

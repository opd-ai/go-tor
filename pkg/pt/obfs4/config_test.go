package obfs4

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if config.IATMode != 0 {
		t.Errorf("DefaultConfig().IATMode = %d, want 0", config.IATMode)
	}
}

func TestValidateCertificate(t *testing.T) {
	tests := []struct {
		name    string
		cert    string
		wantErr bool
	}{
		{
			name:    "valid base64 cert",
			cert:    "dGVzdGNlcnRpZmljYXRlMTIzNDU2Nzg5MDEyMzQ1Njc4OTA=", // Valid base64, 30+ chars
			wantErr: false,
		},
		{
			name:    "empty cert",
			cert:    "",
			wantErr: true,
		},
		{
			name:    "invalid base64",
			cert:    "!!!invalid!!!",
			wantErr: true,
		},
		{
			name:    "too short",
			cert:    "YWJj", // "abc" in base64, only 4 chars
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCertificate(tt.cert)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCertificate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseBridgeLine(t *testing.T) {
	tests := []struct {
		name        string
		bridgeLine  string
		wantCert    string
		wantIATMode int
		wantAddr    string
		wantErr     bool
	}{
		{
			name:        "full bridge line",
			bridgeLine:  "obfs4 192.0.2.1:1234 AAAA cert=dGVzdA== iat-mode=0",
			wantCert:    "dGVzdA==",
			wantIATMode: 0,
			wantAddr:    "192.0.2.1:1234",
			wantErr:     false,
		},
		{
			name:        "with Bridge prefix",
			bridgeLine:  "Bridge obfs4 192.0.2.2:5678 AAAA cert=eHl6MTIz iat-mode=1",
			wantCert:    "eHl6MTIz",
			wantIATMode: 1,
			wantAddr:    "192.0.2.2:5678",
			wantErr:     false,
		},
		{
			name:        "iat-mode=2",
			bridgeLine:  "obfs4 192.0.2.3:9001 cert=YWJjZGVm iat-mode=2",
			wantCert:    "YWJjZGVm",
			wantIATMode: 2,
			wantAddr:    "192.0.2.3:9001",
			wantErr:     false,
		},
		{
			name:       "missing cert",
			bridgeLine: "obfs4 192.0.2.4:1234",
			wantErr:    true,
		},
		{
			name:       "invalid iat-mode",
			bridgeLine: "obfs4 192.0.2.5:1234 cert=dGVzdA== iat-mode=99",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, addr, err := ParseBridgeLine(tt.bridgeLine)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseBridgeLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if config.Certificate != tt.wantCert {
				t.Errorf("Certificate = %v, want %v", config.Certificate, tt.wantCert)
			}
			if config.IATMode != tt.wantIATMode {
				t.Errorf("IATMode = %v, want %v", config.IATMode, tt.wantIATMode)
			}
			if addr != tt.wantAddr {
				t.Errorf("Address = %v, want %v", addr, tt.wantAddr)
			}
		})
	}
}

func TestExtractParam(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		param   string
		want    string
		wantErr bool
	}{
		{
			name:    "cert parameter",
			line:    "obfs4 192.0.2.1:1234 cert=dGVzdA== iat-mode=0",
			param:   "cert=",
			want:    "dGVzdA==",
			wantErr: false,
		},
		{
			name:    "iat-mode parameter",
			line:    "obfs4 192.0.2.1:1234 cert=dGVzdA== iat-mode=1",
			param:   "iat-mode=",
			want:    "1",
			wantErr: false,
		},
		{
			name:    "parameter not found",
			line:    "obfs4 192.0.2.1:1234",
			param:   "cert=",
			wantErr: true,
		},
		{
			name:    "parameter at end of line",
			line:    "obfs4 192.0.2.1:1234 cert=finalvalue",
			param:   "cert=",
			want:    "finalvalue",
			wantErr: false,
		},
		{
			name:    "parameter with newline",
			line:    "obfs4 192.0.2.1:1234 cert=value\n",
			param:   "cert=",
			want:    "value",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractParam(tt.line, tt.param)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractParam() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("extractParam() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractAddress(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    string
		wantErr bool
	}{
		{
			name:    "simple bridge line",
			line:    "obfs4 192.0.2.1:1234 AAAA",
			want:    "192.0.2.1:1234",
			wantErr: false,
		},
		{
			name:    "with Bridge prefix",
			line:    "Bridge obfs4 192.0.2.2:5678 AAAA cert=xyz",
			want:    "192.0.2.2:5678",
			wantErr: false,
		},
		{
			name:    "IPv6 address",
			line:    "obfs4 [2001:db8::1]:9001 AAAA",
			want:    "[2001:db8::1]:9001",
			wantErr: false,
		},
		{
			name:    "invalid format - no address",
			line:    "obfs4",
			wantErr: true,
		},
		{
			name:    "invalid format - no colon",
			line:    "obfs4 192.0.2.1 AAAA",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractAddress(tt.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractAddress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("extractAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadServerKeys(t *testing.T) {
	tempDir := t.TempDir()

	// Create a mock bridge line file
	bridgeLine := "Bridge obfs4 192.0.2.1:1234 AAAA cert=dGVzdGNlcnQ= iat-mode=0\n"
	bridgeFile := filepath.Join(tempDir, "obfs4_bridgeline.txt")
	if err := os.WriteFile(bridgeFile, []byte(bridgeLine), 0o600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cert, err := LoadServerKeys(tempDir)
	if err != nil {
		t.Errorf("LoadServerKeys() error = %v", err)
		return
	}

	expectedCert := "dGVzdGNlcnQ="
	if cert != expectedCert {
		t.Errorf("LoadServerKeys() = %v, want %v", cert, expectedCert)
	}
}

func TestLoadServerKeys_NotFound(t *testing.T) {
	tempDir := t.TempDir()

	_, err := LoadServerKeys(tempDir)
	if err == nil {
		t.Error("LoadServerKeys() should fail when file doesn't exist")
	}
}

func TestLoadServerKeys_EmptyStateDir(t *testing.T) {
	_, err := LoadServerKeys("")
	if err == nil {
		t.Error("LoadServerKeys() should fail with empty state directory")
	}
}

func TestExportImportKeys(t *testing.T) {
	tempDir := t.TempDir()

	// Create a mock state file
	stateFile := filepath.Join(tempDir, "obfs4_state.json")
	testData := []byte(`{"test": "data"}`)
	if err := os.WriteFile(stateFile, testData, 0o600); err != nil {
		t.Fatalf("Failed to create test state: %v", err)
	}

	// Export keys
	exportPath := filepath.Join(tempDir, "export.dat")
	if err := ExportKeys(tempDir, exportPath); err != nil {
		t.Errorf("ExportKeys() error = %v", err)
	}

	// Verify export file exists
	if _, err := os.Stat(exportPath); os.IsNotExist(err) {
		t.Error("Export file was not created")
	}

	// Import to new directory
	importDir := filepath.Join(tempDir, "import")
	if err := os.MkdirAll(importDir, 0o700); err != nil {
		t.Fatalf("Failed to create import dir: %v", err)
	}

	if err := ImportKeys(exportPath, importDir); err != nil {
		t.Errorf("ImportKeys() error = %v", err)
	}

	// Verify imported state
	importedState := filepath.Join(importDir, "obfs4_state.json")
	data, err := os.ReadFile(importedState)
	if err != nil {
		t.Errorf("Failed to read imported state: %v", err)
	}

	if string(data) != string(testData) {
		t.Errorf("Imported data = %v, want %v", string(data), string(testData))
	}
}

func TestGetStateFilePath(t *testing.T) {
	stateDir := "/tmp/test-obfs4"
	expected := filepath.Join(stateDir, "obfs4_state.json")

	got := GetStateFilePath(stateDir)
	if got != expected {
		t.Errorf("GetStateFilePath() = %v, want %v", got, expected)
	}
}

func TestGetBridgeLineExample(t *testing.T) {
	address := "192.0.2.1:1234"
	cert := "dGVzdGNlcnQ="
	iatMode := 1

	bridgeLine := GetBridgeLineExample(address, cert, iatMode)

	// Verify it contains expected components
	if !strings.Contains(bridgeLine, address) {
		t.Errorf("Bridge line doesn't contain address: %s", bridgeLine)
	}
	if !strings.Contains(bridgeLine, cert) {
		t.Errorf("Bridge line doesn't contain cert: %s", bridgeLine)
	}
	if !strings.Contains(bridgeLine, "iat-mode=1") {
		t.Errorf("Bridge line doesn't contain iat-mode: %s", bridgeLine)
	}
	if !strings.HasPrefix(bridgeLine, "Bridge obfs4") {
		t.Errorf("Bridge line doesn't start with 'Bridge obfs4': %s", bridgeLine)
	}
}

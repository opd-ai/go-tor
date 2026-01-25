package relay

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestGenerateServerDescriptor(t *testing.T) {
	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}
	
	tests := []struct {
		name      string
		config    *DescriptorConfig
		wantErr   bool
		errString string
	}{
		{
			name: "valid descriptor",
			config: &DescriptorConfig{
				Nickname: "TestRelay",
				Address:  "192.168.1.100",
				ORPort:   9001,
				DirPort:  9030,
				Contact:  "admin@example.com",
			},
			wantErr: false,
		},
		{
			name: "bridge descriptor (no DirPort)",
			config: &DescriptorConfig{
				Nickname: "BridgeRelay",
				Address:  "10.0.0.1",
				ORPort:   443,
				DirPort:  0,
				IsBridge: true,
			},
			wantErr: false,
		},
		{
			name: "descriptor with IPv6",
			config: &DescriptorConfig{
				Nickname: "IPv6Relay",
				Address:  "203.0.113.1",
				ORPort:   9001,
				IPv6Addr: "[2001:db8::1]:9001",
			},
			wantErr: false,
		},
		{
			name: "descriptor with family",
			config: &DescriptorConfig{
				Nickname: "FamilyRelay",
				Address:  "192.0.2.1",
				ORPort:   9001,
				Family:   []string{"$AAAA", "$BBBB"},
			},
			wantErr: false,
		},
		{
			name:      "nil config",
			config:    nil,
			wantErr:   true,
			errString: "descriptor config cannot be nil",
		},
		{
			name: "missing address",
			config: &DescriptorConfig{
				Nickname: "NoAddress",
				ORPort:   9001,
			},
			wantErr:   true,
			errString: "relay address is required",
		},
		{
			name: "missing OR port",
			config: &DescriptorConfig{
				Nickname: "NoPort",
				Address:  "192.168.1.1",
			},
			wantErr:   true,
			errString: "OR port is required",
		},
		{
			name: "invalid address",
			config: &DescriptorConfig{
				Nickname: "BadAddr",
				Address:  "not-an-ip",
				ORPort:   9001,
			},
			wantErr:   true,
			errString: "invalid IPv4 address",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc, err := GenerateServerDescriptor(keys, tt.config)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				} else if tt.errString != "" && !strings.Contains(err.Error(), tt.errString) {
					t.Errorf("Expected error containing %q, got %q", tt.errString, err.Error())
				}
				return
			}
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			
			// Validate descriptor
			if err := desc.Validate(); err != nil {
				t.Errorf("Descriptor validation failed: %v", err)
			}
			
			// Check basic fields
			if tt.config.Nickname != "" && desc.Nickname != tt.config.Nickname {
				t.Errorf("Nickname mismatch: got %s, want %s", desc.Nickname, tt.config.Nickname)
			}
			if desc.Address != tt.config.Address {
				t.Errorf("Address mismatch: got %s, want %s", desc.Address, tt.config.Address)
			}
			if desc.ORPort != tt.config.ORPort {
				t.Errorf("ORPort mismatch: got %d, want %d", desc.ORPort, tt.config.ORPort)
			}
			if desc.DirPort != tt.config.DirPort {
				t.Errorf("DirPort mismatch: got %d, want %d", desc.DirPort, tt.config.DirPort)
			}
			
			// Check cryptographic fields
			if desc.RSAIdentity == nil {
				t.Error("RSA identity key is nil")
			}
			if len(desc.Ed25519Identity) != ed25519.PublicKeySize {
				t.Errorf("Invalid Ed25519 key size: %d", len(desc.Ed25519Identity))
			}
			if len(desc.NtorOnionKey) != 32 {
				t.Errorf("Invalid ntor key size: %d", len(desc.NtorOnionKey))
			}
			
			// Check signature
			if len(desc.Signature) == 0 {
				t.Error("Descriptor signature is empty")
			}
			if len(desc.Digest) == 0 {
				t.Error("Descriptor digest is empty")
			}
			
			// Check raw descriptor format
			if len(desc.RawDescriptor) == 0 {
				t.Error("Raw descriptor is empty")
			}
			
			rawStr := string(desc.RawDescriptor)
			
			// Verify required fields in descriptor
			requiredFields := []string{
				"router " + desc.Nickname,
				desc.Address,
				"platform",
				"proto Link=",
				"published",
				"bandwidth",
				"ntor-onion-key",
				"reject *:*",
				"router-signature",
			}
			
			for _, field := range requiredFields {
				if !strings.Contains(rawStr, field) {
					t.Errorf("Descriptor missing required field: %s", field)
				}
			}
			
			// Check optional fields
			if tt.config.Contact != "" && !strings.Contains(rawStr, "contact "+tt.config.Contact) {
				t.Error("Contact field not found in descriptor")
			}
			if tt.config.IPv6Addr != "" && !strings.Contains(rawStr, "or-address "+tt.config.IPv6Addr) {
				t.Error("IPv6 address not found in descriptor")
			}
			if len(tt.config.Family) > 0 && !strings.Contains(rawStr, "family") {
				t.Error("Family field not found in descriptor")
			}
		})
	}
}

func TestGenerateServerDescriptorNilKeys(t *testing.T) {
	config := &DescriptorConfig{
		Nickname: "Test",
		Address:  "192.168.1.1",
		ORPort:   9001,
	}
	
	_, err := GenerateServerDescriptor(nil, config)
	if err == nil {
		t.Error("Expected error for nil keys, got nil")
	}
	if !strings.Contains(err.Error(), "relay keys cannot be nil") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestDescriptorFingerprint(t *testing.T) {
	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}
	
	config := &DescriptorConfig{
		Nickname: "FingerprintTest",
		Address:  "192.168.1.1",
		ORPort:   9001,
	}
	
	desc, err := GenerateServerDescriptor(keys, config)
	if err != nil {
		t.Fatalf("Failed to generate descriptor: %v", err)
	}
	
	fingerprint := desc.Fingerprint()
	if len(fingerprint) != 40 {
		t.Errorf("Fingerprint length: got %d, want 40", len(fingerprint))
	}
	
	// Verify it's hex
	for _, c := range fingerprint {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')) {
			t.Errorf("Non-hex character in fingerprint: %c", c)
		}
	}
	
	// Ed25519 fingerprint
	ed25519Fp := desc.Ed25519Fingerprint()
	decoded, err := base64.StdEncoding.DecodeString(ed25519Fp)
	if err != nil {
		t.Errorf("Failed to decode Ed25519 fingerprint: %v", err)
	}
	if len(decoded) != ed25519.PublicKeySize {
		t.Errorf("Ed25519 fingerprint size: got %d, want %d", len(decoded), ed25519.PublicKeySize)
	}
}

func TestDescriptorValidation(t *testing.T) {
	keys, _ := GenerateRelayKeys()
	
	tests := []struct {
		name    string
		modify  func(*ServerDescriptor)
		wantErr string
	}{
		{
			name:    "valid descriptor",
			modify:  func(d *ServerDescriptor) {},
			wantErr: "",
		},
		{
			name: "empty nickname",
			modify: func(d *ServerDescriptor) {
				d.Nickname = ""
			},
			wantErr: "nickname is required",
		},
		{
			name: "nickname too long",
			modify: func(d *ServerDescriptor) {
				d.Nickname = "ThisNicknameIsWayTooLongForTor"
			},
			wantErr: "nickname too long",
		},
		{
			name: "empty address",
			modify: func(d *ServerDescriptor) {
				d.Address = ""
			},
			wantErr: "address is required",
		},
		{
			name: "zero OR port",
			modify: func(d *ServerDescriptor) {
				d.ORPort = 0
			},
			wantErr: "OR port is required",
		},
		{
			name: "nil RSA key",
			modify: func(d *ServerDescriptor) {
				d.RSAIdentity = nil
			},
			wantErr: "RSA identity key is required",
		},
		{
			name: "invalid Ed25519 key",
			modify: func(d *ServerDescriptor) {
				d.Ed25519Identity = make([]byte, 10)
			},
			wantErr: "invalid Ed25519 key size",
		},
		{
			name: "invalid ntor key",
			modify: func(d *ServerDescriptor) {
				d.NtorOnionKey = make([]byte, 10)
			},
			wantErr: "invalid ntor key size",
		},
		{
			name: "missing signature",
			modify: func(d *ServerDescriptor) {
				d.Signature = nil
			},
			wantErr: "descriptor signature is required",
		},
		{
			name: "empty raw descriptor",
			modify: func(d *ServerDescriptor) {
				d.RawDescriptor = nil
			},
			wantErr: "raw descriptor is empty",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &DescriptorConfig{
				Nickname: "ValidRelay",
				Address:  "192.168.1.1",
				ORPort:   9001,
			}
			
			desc, err := GenerateServerDescriptor(keys, config)
			if err != nil {
				t.Fatalf("Failed to generate descriptor: %v", err)
			}
			
			tt.modify(desc)
			
			err = desc.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Unexpected validation error: %v", err)
				}
			} else {
				if err == nil {
					t.Error("Expected validation error, got nil")
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

func TestDescriptorDefaults(t *testing.T) {
	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}
	
	config := &DescriptorConfig{
		Address: "192.168.1.1",
		ORPort:  9001,
		// No nickname, bandwidth specified - should use defaults
	}
	
	desc, err := GenerateServerDescriptor(keys, config)
	if err != nil {
		t.Fatalf("Failed to generate descriptor: %v", err)
	}
	
	// Check default nickname
	if desc.Nickname == "" {
		t.Error("Default nickname is empty")
	}
	if !strings.HasPrefix(desc.Nickname, "Unnamed") {
		t.Errorf("Default nickname should start with 'Unnamed', got %s", desc.Nickname)
	}
	
	// Check default bandwidth
	if desc.BandwidthAvg == 0 {
		t.Error("Default bandwidth average is 0")
	}
	if desc.BandwidthBurst == 0 {
		t.Error("Default bandwidth burst is 0")
	}
	if desc.BandwidthBurst <= desc.BandwidthAvg {
		t.Error("Bandwidth burst should be > average")
	}
	
	// Check exit policy
	if desc.ExitPolicy != "reject *:*" {
		t.Errorf("Exit policy should be 'reject *:*', got %s", desc.ExitPolicy)
	}
}

func TestDescriptorTimestamp(t *testing.T) {
	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}
	
	config := &DescriptorConfig{
		Nickname: "TimeTest",
		Address:  "192.168.1.1",
		ORPort:   9001,
	}
	
	before := time.Now().UTC()
	desc, err := GenerateServerDescriptor(keys, config)
	if err != nil {
		t.Fatalf("Failed to generate descriptor: %v", err)
	}
	after := time.Now().UTC()
	
	if desc.PublishedTime.Before(before) || desc.PublishedTime.After(after) {
		t.Errorf("Published time %v not within expected range [%v, %v]",
			desc.PublishedTime, before, after)
	}
	
	// Verify timestamp in raw descriptor
	rawStr := string(desc.RawDescriptor)
	expectedFormat := desc.PublishedTime.Format("2006-01-02 15:04:05")
	if !strings.Contains(rawStr, expectedFormat) {
		t.Errorf("Raw descriptor doesn't contain expected timestamp format: %s", expectedFormat)
	}
}

func TestGenerateExtraInfo(t *testing.T) {
	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}
	
	config := &DescriptorConfig{
		Nickname: "ExtraInfoTest",
		Address:  "192.168.1.1",
		ORPort:   9001,
	}
	
	desc, err := GenerateServerDescriptor(keys, config)
	if err != nil {
		t.Fatalf("Failed to generate descriptor: %v", err)
	}
	
	stats := map[string]string{
		"read-history":  "2024-01-01 00:00:00 (900 s) 1000,2000,3000",
		"write-history": "2024-01-01 00:00:00 (900 s) 500,1500,2500",
	}
	
	extraInfo, err := GenerateExtraInfo(keys, desc, stats)
	if err != nil {
		t.Fatalf("Failed to generate extra-info: %v", err)
	}
	
	if extraInfo.Nickname != desc.Nickname {
		t.Errorf("Nickname mismatch: got %s, want %s", extraInfo.Nickname, desc.Nickname)
	}
	
	if extraInfo.Fingerprint != desc.Fingerprint() {
		t.Errorf("Fingerprint mismatch: got %s, want %s", extraInfo.Fingerprint, desc.Fingerprint())
	}
	
	if len(extraInfo.Digest) == 0 {
		t.Error("Extra-info digest is empty")
	}
	
	if len(extraInfo.Signature) == 0 {
		t.Error("Extra-info signature is empty")
	}
	
	if extraInfo.Statistics == nil {
		t.Error("Statistics map is nil")
	}
	
	for key, expectedVal := range stats {
		if actualVal, ok := extraInfo.Statistics[key]; !ok {
			t.Errorf("Missing statistic: %s", key)
		} else if actualVal != expectedVal {
			t.Errorf("Statistic %s mismatch: got %s, want %s", key, actualVal, expectedVal)
		}
	}
}

func TestGenerateExtraInfoNilInputs(t *testing.T) {
	keys, _ := GenerateRelayKeys()
	config := &DescriptorConfig{
		Nickname: "Test",
		Address:  "192.168.1.1",
		ORPort:   9001,
	}
	desc, _ := GenerateServerDescriptor(keys, config)
	
	tests := []struct {
		name      string
		keys      *RelayKeys
		desc      *ServerDescriptor
		wantErr   string
	}{
		{
			name:    "nil keys",
			keys:    nil,
			desc:    desc,
			wantErr: "relay keys cannot be nil",
		},
		{
			name:    "nil descriptor",
			keys:    keys,
			desc:    nil,
			wantErr: "server descriptor cannot be nil",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GenerateExtraInfo(tt.keys, tt.desc, nil)
			if err == nil {
				t.Error("Expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestDescriptorString(t *testing.T) {
	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}
	
	config := &DescriptorConfig{
		Nickname: "StringTest",
		Address:  "192.168.1.1",
		ORPort:   9001,
	}
	
	desc, err := GenerateServerDescriptor(keys, config)
	if err != nil {
		t.Fatalf("Failed to generate descriptor: %v", err)
	}
	
	str := desc.String()
	
	expectedParts := []string{
		"ServerDescriptor",
		"StringTest",
		"192.168.1.1:9001",
		"fingerprint=",
	}
	
	for _, part := range expectedParts {
		if !strings.Contains(str, part) {
			t.Errorf("String representation missing %q: %s", part, str)
		}
	}
}

func TestGenerateNickname(t *testing.T) {
	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}
	
	nickname := generateNickname(keys)
	
	if nickname == "" {
		t.Error("Generated nickname is empty")
	}
	
	if !strings.HasPrefix(nickname, "Unnamed") {
		t.Errorf("Expected nickname to start with 'Unnamed', got %s", nickname)
	}
	
	// Nickname should be deterministic for same keys
	nickname2 := generateNickname(keys)
	if nickname != nickname2 {
		t.Errorf("Nickname not deterministic: got %s and %s", nickname, nickname2)
	}
}

func TestDescriptorBandwidthCustom(t *testing.T) {
	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}
	
	config := &DescriptorConfig{
		Nickname:       "BandwidthTest",
		Address:        "192.168.1.1",
		ORPort:         9001,
		BandwidthAvg:   5 * 1024 * 1024, // 5 MB/s
		BandwidthBurst: 10 * 1024 * 1024, // 10 MB/s
	}
	
	desc, err := GenerateServerDescriptor(keys, config)
	if err != nil {
		t.Fatalf("Failed to generate descriptor: %v", err)
	}
	
	if desc.BandwidthAvg != config.BandwidthAvg {
		t.Errorf("BandwidthAvg: got %d, want %d", desc.BandwidthAvg, config.BandwidthAvg)
	}
	
	if desc.BandwidthBurst != config.BandwidthBurst {
		t.Errorf("BandwidthBurst: got %d, want %d", desc.BandwidthBurst, config.BandwidthBurst)
	}
	
	// Observed bandwidth should start at average
	if desc.BandwidthObs != config.BandwidthAvg {
		t.Errorf("BandwidthObs: got %d, want %d", desc.BandwidthObs, config.BandwidthAvg)
	}
}

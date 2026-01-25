package config

import (
	"testing"
)

func TestParseBridge(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		wantErr     bool
		want        *BridgeInfo
		checkFields func(*testing.T, *BridgeInfo)
	}{
		{
			name: "vanilla bridge with IP",
			line: "192.0.2.1:443",
			want: &BridgeInfo{
				Transport:   "",
				Address:     "192.0.2.1",
				Port:        443,
				Fingerprint: "",
				Parameters:  map[string]string{},
			},
		},
		{
			name: "vanilla bridge with prefix",
			line: "Bridge 192.0.2.1:9001",
			want: &BridgeInfo{
				Transport:   "",
				Address:     "192.0.2.1",
				Port:        9001,
				Fingerprint: "",
				Parameters:  map[string]string{},
			},
		},
		{
			name: "vanilla bridge with fingerprint",
			line: "192.0.2.1:443 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			want: &BridgeInfo{
				Transport:   "",
				Address:     "192.0.2.1",
				Port:        443,
				Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
				Parameters:  map[string]string{},
			},
		},
		{
			name: "obfs4 bridge without fingerprint",
			line: "obfs4 192.0.2.1:1234",
			want: &BridgeInfo{
				Transport:   "obfs4",
				Address:     "192.0.2.1",
				Port:        1234,
				Fingerprint: "",
				Parameters:  map[string]string{},
			},
		},
		{
			name: "obfs4 bridge with parameters",
			line: "obfs4 192.0.2.1:1234 cert=abcd1234 iat-mode=0",
			want: &BridgeInfo{
				Transport:   "obfs4",
				Address:     "192.0.2.1",
				Port:        1234,
				Fingerprint: "",
				Parameters: map[string]string{
					"cert":     "abcd1234",
					"iat-mode": "0",
				},
			},
		},
		{
			name: "obfs4 bridge with fingerprint and parameters",
			line: "Bridge obfs4 192.0.2.1:1234 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA cert=xyz iat-mode=1",
			want: &BridgeInfo{
				Transport:   "obfs4",
				Address:     "192.0.2.1",
				Port:        1234,
				Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
				Parameters: map[string]string{
					"cert":     "xyz",
					"iat-mode": "1",
				},
			},
		},
		{
			name: "bridge with hostname",
			line: "bridge.example.com:9001",
			want: &BridgeInfo{
				Transport:   "",
				Address:     "bridge.example.com",
				Port:        9001,
				Fingerprint: "",
				Parameters:  map[string]string{},
			},
		},
		{
			name: "meek_lite bridge with parameters",
			line: "meek_lite 192.0.2.1:443 url=https://example.com front=www.google.com",
			want: &BridgeInfo{
				Transport:   "meek_lite",
				Address:     "192.0.2.1",
				Port:        443,
				Fingerprint: "",
				Parameters: map[string]string{
					"url":   "https://example.com",
					"front": "www.google.com",
				},
			},
		},
		{
			name: "snowflake bridge",
			line: "snowflake 192.0.2.1:1234 fingerprint=ABC ice=stun:stun.l.google.com:19302",
			checkFields: func(t *testing.T, b *BridgeInfo) {
				if b.Transport != "snowflake" {
					t.Errorf("Transport = %s, want snowflake", b.Transport)
				}
				if b.Parameters["ice"] != "stun:stun.l.google.com:19302" {
					t.Errorf("ice parameter = %s, want stun:stun.l.google.com:19302", b.Parameters["ice"])
				}
			},
		},
		{
			name:    "empty line",
			line:    "",
			wantErr: true,
		},
		{
			name:    "invalid port",
			line:    "192.0.2.1:99999",
			wantErr: true,
		},
		{
			name:    "missing port",
			line:    "192.0.2.1",
			wantErr: true,
		},
		{
			name:    "transport only",
			line:    "obfs4",
			wantErr: true,
		},
		{
			name:    "negative port",
			line:    "192.0.2.1:-1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBridge(tt.line)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseBridge() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseBridge() unexpected error: %v", err)
			}

			if tt.checkFields != nil {
				tt.checkFields(t, got)
				return
			}

			if tt.want != nil {
				if got.Transport != tt.want.Transport {
					t.Errorf("Transport = %s, want %s", got.Transport, tt.want.Transport)
				}
				if got.Address != tt.want.Address {
					t.Errorf("Address = %s, want %s", got.Address, tt.want.Address)
				}
				if got.Port != tt.want.Port {
					t.Errorf("Port = %d, want %d", got.Port, tt.want.Port)
				}
				if got.Fingerprint != tt.want.Fingerprint {
					t.Errorf("Fingerprint = %s, want %s", got.Fingerprint, tt.want.Fingerprint)
				}
				if len(got.Parameters) != len(tt.want.Parameters) {
					t.Errorf("Parameters count = %d, want %d", len(got.Parameters), len(tt.want.Parameters))
				}
				for k, v := range tt.want.Parameters {
					if got.Parameters[k] != v {
						t.Errorf("Parameter %s = %s, want %s", k, got.Parameters[k], v)
					}
				}
			}
		})
	}
}

func TestBridgeInfo_String(t *testing.T) {
	tests := []struct {
		name   string
		bridge *BridgeInfo
		// We check that parsing the string returns equivalent bridge
	}{
		{
			name: "vanilla bridge",
			bridge: &BridgeInfo{
				Address:    "192.0.2.1",
				Port:       443,
				Parameters: make(map[string]string),
			},
		},
		{
			name: "obfs4 with params",
			bridge: &BridgeInfo{
				Transport:   "obfs4",
				Address:     "192.0.2.1",
				Port:        1234,
				Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
				Parameters: map[string]string{
					"cert": "xyz",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			str := tt.bridge.String()
			if str == "" {
				t.Error("String() returned empty string")
			}

			// Parse the string and verify it represents the same bridge
			parsed, err := ParseBridge(str)
			if err != nil {
				t.Errorf("ParseBridge(String()) error: %v", err)
				return
			}

			if parsed.Transport != tt.bridge.Transport {
				t.Errorf("Transport = %s, want %s", parsed.Transport, tt.bridge.Transport)
			}
			if parsed.Address != tt.bridge.Address {
				t.Errorf("Address = %s, want %s", parsed.Address, tt.bridge.Address)
			}
			if parsed.Port != tt.bridge.Port {
				t.Errorf("Port = %d, want %d", parsed.Port, tt.bridge.Port)
			}
		})
	}
}

func TestBridgeInfo_IsPluggableTransport(t *testing.T) {
	tests := []struct {
		name   string
		bridge *BridgeInfo
		want   bool
	}{
		{
			name:   "vanilla bridge",
			bridge: &BridgeInfo{Transport: ""},
			want:   false,
		},
		{
			name:   "obfs4 bridge",
			bridge: &BridgeInfo{Transport: "obfs4"},
			want:   true,
		},
		{
			name:   "meek bridge",
			bridge: &BridgeInfo{Transport: "meek_lite"},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bridge.IsPluggableTransport(); got != tt.want {
				t.Errorf("IsPluggableTransport() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBridgeInfo_GetTransportName(t *testing.T) {
	tests := []struct {
		name   string
		bridge *BridgeInfo
		want   string
	}{
		{
			name:   "vanilla bridge",
			bridge: &BridgeInfo{Transport: ""},
			want:   "",
		},
		{
			name:   "obfs4 bridge",
			bridge: &BridgeInfo{Transport: "obfs4"},
			want:   "obfs4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bridge.GetTransportName(); got != tt.want {
				t.Errorf("GetTransportName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBridgeInfo_GetAddress(t *testing.T) {
	bridge := &BridgeInfo{
		Address: "192.0.2.1",
		Port:    443,
	}

	want := "192.0.2.1:443"
	if got := bridge.GetAddress(); got != want {
		t.Errorf("GetAddress() = %v, want %v", got, want)
	}
}

func Test_isHexString(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", true},
		{"0123456789ABCDEF", true},
		{"0123456789abcdef", true},
		{"xyz", false},
		{"AAAA BBBB", false},
		{"", true}, // empty string is valid hex (no invalid chars)
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isHexString(tt.input); got != tt.want {
				t.Errorf("isHexString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// Benchmark for parsing different bridge types
func BenchmarkParseBridge(b *testing.B) {
	benchmarks := []struct {
		name string
		line string
	}{
		{"vanilla", "192.0.2.1:443"},
		{"vanilla_fingerprint", "192.0.2.1:443 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		{"obfs4_simple", "obfs4 192.0.2.1:1234"},
		{"obfs4_full", "obfs4 192.0.2.1:1234 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA cert=xyz iat-mode=1"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = ParseBridge(bm.line)
			}
		})
	}
}

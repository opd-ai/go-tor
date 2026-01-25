package directory

import (
	"strings"
	"testing"
)

// TestRelayDescriptorSpecCompliance_Format verifies relay descriptor ("r" line) format per dir-spec.txt §3.4.1
func TestRelayDescriptorSpecCompliance_Format(t *testing.T) {
	tests := []struct {
		name       string
		consensus  string
		wantRelays int
		validateFn func(*testing.T, []*Relay)
	}{
		{
			name: "9-field regular consensus format",
			consensus: `network-status-version 3
r TestRelay AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 9001 9030
s Fast Guard Running Stable Valid`,
			wantRelays: 1,
			validateFn: func(t *testing.T, relays []*Relay) {
				if relays[0].Nickname != "TestRelay" {
					t.Errorf("Nickname = %s, want TestRelay", relays[0].Nickname)
				}
				if relays[0].Fingerprint != "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" {
					t.Errorf("Fingerprint = %s, want AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", relays[0].Fingerprint)
				}
				if relays[0].Address != "192.0.2.1" {
					t.Errorf("Address = %s, want 192.0.2.1", relays[0].Address)
				}
				if relays[0].ORPort != 9001 {
					t.Errorf("ORPort = %d, want 9001", relays[0].ORPort)
				}
				if relays[0].DirPort != 9030 {
					t.Errorf("DirPort = %d, want 9030", relays[0].DirPort)
				}
			},
		},
		{
			name: "8-field microdescriptor consensus format",
			consensus: `network-status-version 3
r TestRelay AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA 2024-01-01 00:00:00 192.0.2.1 9001 9030
s Fast Running Stable Valid`,
			wantRelays: 1,
			validateFn: func(t *testing.T, relays []*Relay) {
				if relays[0].Nickname != "TestRelay" {
					t.Errorf("Nickname = %s, want TestRelay", relays[0].Nickname)
				}
				if relays[0].Address != "192.0.2.1" {
					t.Errorf("Address = %s, want 192.0.2.1", relays[0].Address)
				}
				if relays[0].ORPort != 9001 {
					t.Errorf("ORPort = %d, want 9001", relays[0].ORPort)
				}
			},
		},
		{
			name: "relay with no DirPort (0)",
			consensus: `network-status-version 3
r NoDirPort AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 9001 0
s Fast Running Stable Valid`,
			wantRelays: 1,
			validateFn: func(t *testing.T, relays []*Relay) {
				if relays[0].DirPort != 0 {
					t.Errorf("DirPort = %d, want 0 (no DirPort)", relays[0].DirPort)
				}
			},
		},
	}

	client := NewClient(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.consensus)
			relays, err := client.parseConsensus(reader)
			if err != nil {
				t.Fatalf("parseConsensus() error = %v", err)
			}
			if len(relays) != tt.wantRelays {
				t.Fatalf("parseConsensus() returned %d relays, want %d", len(relays), tt.wantRelays)
			}
			if tt.validateFn != nil {
				tt.validateFn(t, relays)
			}
		})
	}
}

// TestRelayDescriptorSpecCompliance_Validation verifies descriptor validation per dir-spec.txt §3.4.1
func TestRelayDescriptorSpecCompliance_Validation(t *testing.T) {
	tests := []struct {
		name          string
		consensus     string
		wantRelays    int
		expectError   bool
		validateError func(*testing.T, error)
	}{
		{
			name: "excessive malformed entries (>10%)",
			consensus: `network-status-version 3
r Good1 AAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 9001 0
r Bad1 CCCCCCCCCCCCCCCCCCCCCCCCCCCCCC
r Good2 DDDDDDDDDDDDDDDDDDDDDDDDDDDDDD EEEEEEEEEEEEEEEEEEEEEEEEEEEEEE 2024-01-01 00:00:00 192.0.2.2 9002 0
r Bad2 FFFFFFFFFFFFFFFFFFFFFFFFFFFF
r Good3 00000000000000000000000000000 11111111111111111111111111111 2024-01-01 00:00:00 192.0.2.3 9003 0
r Bad3 222222222222222222222222222222
r Bad4 333333333333333333333333333333
r Bad5 444444444444444444444444444444
r Bad6 555555555555555555555555555555
r Good4 666666666666666666666666666666 777777777777777777777777777777 2024-01-01 00:00:00 192.0.2.4 9004 0`,
			wantRelays:  0,
			expectError: true, // >10% malformed (6/10 = 60%)
			validateError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "excessive malformed entries") {
					t.Errorf("error message should mention 'excessive malformed entries', got: %v", err)
				}
			},
		},
		{
			name: "acceptable malformed entries (<10%)",
			consensus: `network-status-version 3
r Bad1 MALFORMED
r Good1 AAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 9001 0
s Fast Running
r Good2 DDDDDDDDDDDDDDDDDDDDDDDDDDDDDD EEEEEEEEEEEEEEEEEEEEEEEEEEEEEE 2024-01-01 00:00:00 192.0.2.2 9002 0
s Fast Running
r Good3 00000000000000000000000000000 11111111111111111111111111111 2024-01-01 00:00:00 192.0.2.3 9003 0
s Fast Running
r Good4 666666666666666666666666666666 777777777777777777777777777777 2024-01-01 00:00:00 192.0.2.4 9004 0
s Fast Running
r Good5 888888888888888888888888888888 999999999999999999999999999999 2024-01-01 00:00:00 192.0.2.5 9005 0
s Fast Running
r Good6 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.6 9006 0
s Fast Running
r Good7 CCCCCCCCCCCCCCCCCCCCCCCCCCCCCC DDDDDDDDDDDDDDDDDDDDDDDDDDDDDD 2024-01-01 00:00:00 192.0.2.7 9007 0
s Fast Running
r Good8 EEEEEEEEEEEEEEEEEEEEEEEEEEEEEE FFFFFFFFFFFFFFFFFFFFFFFFFFFF 2024-01-01 00:00:00 192.0.2.8 9008 0
s Fast Running
r Good9 00000000000000000000000000000 11111111111111111111111111111 2024-01-01 00:00:00 192.0.2.9 9009 0
s Fast Running
r Good10 222222222222222222222222222222 333333333333333333333333333333 2024-01-01 00:00:00 192.0.2.10 9010 0
s Fast Running`,
			wantRelays:  10, // 1/11 = 9.1% malformed (acceptable)
			expectError: false,
		},
	}

	client := NewClient(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.consensus)
			relays, err := client.parseConsensus(reader)

			if tt.expectError {
				if err == nil {
					t.Fatal("parseConsensus() expected error, got nil")
				}
				if tt.validateError != nil {
					tt.validateError(t, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseConsensus() unexpected error = %v", err)
			}
			if len(relays) != tt.wantRelays {
				t.Errorf("parseConsensus() returned %d relays, want %d", len(relays), tt.wantRelays)
			}
		})
	}
}

// TestRelayDescriptorSpecCompliance_Nickname verifies nickname validation per dir-spec.txt §2.1.1
func TestRelayDescriptorSpecCompliance_Nickname(t *testing.T) {
	tests := []struct {
		name       string
		nickname   string
		consensus  string
		wantRelays int
	}{
		{
			name:     "valid alphanumeric nickname",
			nickname: "ValidRelay123",
			consensus: `network-status-version 3
r ValidRelay123 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 9001 0
s Fast Running`,
			wantRelays: 1,
		},
		{
			name:     "short nickname (1 char)",
			nickname: "A",
			consensus: `network-status-version 3
r A AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 9001 0
s Fast Running`,
			wantRelays: 1,
		},
		{
			name:     "maximum length nickname (19 chars per spec)",
			nickname: "MaxLengthNickname19",
			consensus: `network-status-version 3
r MaxLengthNickname19 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 9001 0
s Fast Running`,
			wantRelays: 1,
		},
		{
			name:     "nickname with numbers",
			nickname: "Relay2024v2",
			consensus: `network-status-version 3
r Relay2024v2 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 9001 0
s Fast Running`,
			wantRelays: 1,
		},
	}

	client := NewClient(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.consensus)
			relays, err := client.parseConsensus(reader)
			if err != nil {
				t.Fatalf("parseConsensus() error = %v", err)
			}
			if len(relays) != tt.wantRelays {
				t.Fatalf("parseConsensus() returned %d relays, want %d", len(relays), tt.wantRelays)
			}
			if relays[0].Nickname != tt.nickname {
				t.Errorf("Nickname = %s, want %s", relays[0].Nickname, tt.nickname)
			}
		})
	}
}

// TestRelayDescriptorSpecCompliance_Fingerprint verifies fingerprint format per dir-spec.txt §2.1.3
func TestRelayDescriptorSpecCompliance_Fingerprint(t *testing.T) {
	tests := []struct {
		name        string
		fingerprint string
		consensus   string
		wantRelays  int
	}{
		{
			name:        "base64 fingerprint",
			fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAA+",
			consensus: `network-status-version 3
r Test AAAAAAAAAAAAAAAAAAAAAAAAAAA+ BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 9001 0
s Fast Running`,
			wantRelays: 1,
		},
		{
			name:        "fingerprint with slash",
			fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAA/w==",
			consensus: `network-status-version 3
r Test AAAAAAAAAAAAAAAAAAAAAAAAAAAA/w== BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 9001 0
s Fast Running`,
			wantRelays: 1,
		},
		{
			name:        "fingerprint with plus",
			fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAA+Q==",
			consensus: `network-status-version 3
r Test AAAAAAAAAAAAAAAAAAAAAAAAAAAA+Q== BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 9001 0
s Fast Running`,
			wantRelays: 1,
		},
	}

	client := NewClient(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.consensus)
			relays, err := client.parseConsensus(reader)
			if err != nil {
				t.Fatalf("parseConsensus() error = %v", err)
			}
			if len(relays) != tt.wantRelays {
				t.Fatalf("parseConsensus() returned %d relays, want %d", len(relays), tt.wantRelays)
			}
			if relays[0].Fingerprint != tt.fingerprint {
				t.Errorf("Fingerprint = %s, want %s", relays[0].Fingerprint, tt.fingerprint)
			}
		})
	}
}

// TestRelayDescriptorSpecCompliance_Address verifies IPv4 address format per dir-spec.txt §2.1.3
func TestRelayDescriptorSpecCompliance_Address(t *testing.T) {
	tests := []struct {
		name       string
		address    string
		consensus  string
		wantRelays int
	}{
		{
			name:    "private IPv4 address (RFC 1918)",
			address: "10.0.0.1",
			consensus: `network-status-version 3
r Test AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 10.0.0.1 9001 0
s Fast Running`,
			wantRelays: 1,
		},
		{
			name:    "public IPv4 address",
			address: "203.0.113.1",
			consensus: `network-status-version 3
r Test AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 203.0.113.1 9001 0
s Fast Running`,
			wantRelays: 1,
		},
		{
			name:    "localhost address",
			address: "127.0.0.1",
			consensus: `network-status-version 3
r Test AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 127.0.0.1 9001 0
s Fast Running`,
			wantRelays: 1,
		},
		{
			name:    "edge address (255.255.255.254)",
			address: "255.255.255.254",
			consensus: `network-status-version 3
r Test AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 255.255.255.254 9001 0
s Fast Running`,
			wantRelays: 1,
		},
	}

	client := NewClient(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.consensus)
			relays, err := client.parseConsensus(reader)
			if err != nil {
				t.Fatalf("parseConsensus() error = %v", err)
			}
			if len(relays) != tt.wantRelays {
				t.Fatalf("parseConsensus() returned %d relays, want %d", len(relays), tt.wantRelays)
			}
			if relays[0].Address != tt.address {
				t.Errorf("Address = %s, want %s", relays[0].Address, tt.address)
			}
		})
	}
}

// TestRelayDescriptorSpecCompliance_Ports verifies ORPort and DirPort parsing per dir-spec.txt §2.1.3
func TestRelayDescriptorSpecCompliance_Ports(t *testing.T) {
	tests := []struct {
		name       string
		orPort     int
		dirPort    int
		consensus  string
		wantRelays int
	}{
		{
			name:    "standard ports (9001/9030)",
			orPort:  9001,
			dirPort: 9030,
			consensus: `network-status-version 3
r Test AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 9001 9030
s Fast Running`,
			wantRelays: 1,
		},
		{
			name:    "alternative ORPort (443)",
			orPort:  443,
			dirPort: 80,
			consensus: `network-status-version 3
r Test AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 443 80
s Fast Running`,
			wantRelays: 1,
		},
		{
			name:    "no DirPort (0)",
			orPort:  9001,
			dirPort: 0,
			consensus: `network-status-version 3
r Test AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 9001 0
s Fast Running`,
			wantRelays: 1,
		},
		{
			name:    "high port numbers",
			orPort:  65535,
			dirPort: 65534,
			consensus: `network-status-version 3
r Test AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 65535 65534
s Fast Running`,
			wantRelays: 1,
		},
		{
			name:    "low port numbers",
			orPort:  1,
			dirPort: 1,
			consensus: `network-status-version 3
r Test AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 1 1
s Fast Running`,
			wantRelays: 1,
		},
	}

	client := NewClient(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.consensus)
			relays, err := client.parseConsensus(reader)
			if err != nil {
				t.Fatalf("parseConsensus() error = %v", err)
			}
			if len(relays) != tt.wantRelays {
				t.Fatalf("parseConsensus() returned %d relays, want %d", len(relays), tt.wantRelays)
			}
			if relays[0].ORPort != tt.orPort {
				t.Errorf("ORPort = %d, want %d", relays[0].ORPort, tt.orPort)
			}
			if relays[0].DirPort != tt.dirPort {
				t.Errorf("DirPort = %d, want %d", relays[0].DirPort, tt.dirPort)
			}
		})
	}
}

// TestRelayDescriptorSpecCompliance_InvalidPorts verifies port parsing error handling
func TestRelayDescriptorSpecCompliance_InvalidPorts(t *testing.T) {
	tests := []struct {
		name       string
		consensus  string
		wantRelays int
		// Port parsing errors are logged but don't fail parsing (ports default to 0)
	}{
		{
			name: "non-numeric ORPort",
			consensus: `network-status-version 3
r Test AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 ABC 9030
s Fast Running`,
			wantRelays: 1, // Entry is parsed, ORPort defaults to 0 on parse error
		},
		{
			name: "non-numeric DirPort",
			consensus: `network-status-version 3
r Test AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 9001 XYZ
s Fast Running`,
			wantRelays: 1, // Entry is parsed, DirPort defaults to 0 on parse error
		},
		{
			name: "negative port (invalid)",
			consensus: `network-status-version 3
r Test AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 -1 9030
s Fast Running`,
			wantRelays: 1, // Entry is parsed, port parsing handles invalid values
		},
	}

	client := NewClient(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.consensus)
			relays, err := client.parseConsensus(reader)
			if err != nil {
				t.Fatalf("parseConsensus() error = %v", err)
			}
			if len(relays) != tt.wantRelays {
				t.Errorf("parseConsensus() returned %d relays, want %d", len(relays), tt.wantRelays)
			}
			// Port parsing errors result in 0 values, which is acceptable
			// The implementation logs these errors but doesn't fail parsing
		})
	}
}

// TestRelayDescriptorSpecCompliance_MultipleRelays verifies parsing of multiple relay entries
func TestRelayDescriptorSpecCompliance_MultipleRelays(t *testing.T) {
	consensus := `network-status-version 3
vote-status consensus
r Relay1 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 9001 9030
s Fast Guard Running Stable Valid
w Bandwidth=1000000
r Relay2 CCCCCCCCCCCCCCCCCCCCCCCCCCCCCC DDDDDDDDDDDDDDDDDDDDDDDDDDDDDD 2024-01-01 01:00:00 192.0.2.2 9002 0
s Exit Fast Running Stable Valid
w Bandwidth=2000000
r Relay3 EEEEEEEEEEEEEEEEEEEEEEEEEEEEEE FFFFFFFFFFFFFFFFFFFFFFFFFFFF 2024-01-01 02:00:00 192.0.2.3 443 80
s Fast Running Valid
w Bandwidth=500000
r Relay4 00000000000000000000000000000 11111111111111111111111111111 2024-01-01 03:00:00 192.0.2.4 9004 0
s Running Valid
r Relay5 222222222222222222222222222222 333333333333333333333333333333 2024-01-01 04:00:00 192.0.2.5 9005 9035
s Fast Guard Running Stable Valid HSDir
w Bandwidth=3000000`

	client := NewClient(nil)
	reader := strings.NewReader(consensus)
	relays, err := client.parseConsensus(reader)
	if err != nil {
		t.Fatalf("parseConsensus() error = %v", err)
	}

	if len(relays) != 5 {
		t.Fatalf("parseConsensus() returned %d relays, want 5", len(relays))
	}

	// Verify first relay
	if relays[0].Nickname != "Relay1" {
		t.Errorf("relay[0].Nickname = %s, want Relay1", relays[0].Nickname)
	}
	if relays[0].ORPort != 9001 {
		t.Errorf("relay[0].ORPort = %d, want 9001", relays[0].ORPort)
	}
	if relays[0].DirPort != 9030 {
		t.Errorf("relay[0].DirPort = %d, want 9030", relays[0].DirPort)
	}
	if !relays[0].HasFlag("Guard") {
		t.Error("relay[0] should have Guard flag")
	}
	if relays[0].Bandwidth != 1000000 {
		t.Errorf("relay[0].Bandwidth = %d, want 1000000", relays[0].Bandwidth)
	}

	// Verify second relay (Exit)
	if relays[1].Nickname != "Relay2" {
		t.Errorf("relay[1].Nickname = %s, want Relay2", relays[1].Nickname)
	}
	if !relays[1].HasFlag("Exit") {
		t.Error("relay[1] should have Exit flag")
	}
	if relays[1].Bandwidth != 2000000 {
		t.Errorf("relay[1].Bandwidth = %d, want 2000000", relays[1].Bandwidth)
	}

	// Verify third relay (alternative ports)
	if relays[2].Nickname != "Relay3" {
		t.Errorf("relay[2].Nickname = %s, want Relay3", relays[2].Nickname)
	}
	if relays[2].ORPort != 443 {
		t.Errorf("relay[2].ORPort = %d, want 443", relays[2].ORPort)
	}
	if relays[2].DirPort != 80 {
		t.Errorf("relay[2].DirPort = %d, want 80", relays[2].DirPort)
	}

	// Verify fourth relay (minimal flags)
	if relays[3].Nickname != "Relay4" {
		t.Errorf("relay[3].Nickname = %s, want Relay4", relays[3].Nickname)
	}
	if len(relays[3].Flags) != 2 { // "Running Valid"
		t.Errorf("relay[3] has %d flags, want 2 (Running, Valid)", len(relays[3].Flags))
	}

	// Verify fifth relay (HSDir flag)
	if relays[4].Nickname != "Relay5" {
		t.Errorf("relay[4].Nickname = %s, want Relay5", relays[4].Nickname)
	}
	if !relays[4].HasFlag("HSDir") {
		t.Error("relay[4] should have HSDir flag")
	}
	if relays[4].Bandwidth != 3000000 {
		t.Errorf("relay[4].Bandwidth = %d, want 3000000", relays[4].Bandwidth)
	}
}

// TestRelayDescriptorSpecCompliance_Published verifies published timestamp field parsing
func TestRelayDescriptorSpecCompliance_Published(t *testing.T) {
	tests := []struct {
		name       string
		published  string // "YYYY-MM-DD HH:MM:SS" format
		consensus  string
		wantRelays int
	}{
		{
			name:      "valid published timestamp",
			published: "2024-01-15 12:30:45",
			consensus: `network-status-version 3
r Test AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-15 12:30:45 192.0.2.1 9001 0
s Fast Running`,
			wantRelays: 1,
		},
		{
			name:      "midnight timestamp",
			published: "2024-12-31 00:00:00",
			consensus: `network-status-version 3
r Test AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-12-31 00:00:00 192.0.2.1 9001 0
s Fast Running`,
			wantRelays: 1,
		},
		{
			name:      "end of day timestamp",
			published: "2024-06-15 23:59:59",
			consensus: `network-status-version 3
r Test AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-06-15 23:59:59 192.0.2.1 9001 0
s Fast Running`,
			wantRelays: 1,
		},
	}

	client := NewClient(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.consensus)
			relays, err := client.parseConsensus(reader)
			if err != nil {
				t.Fatalf("parseConsensus() error = %v", err)
			}
			if len(relays) != tt.wantRelays {
				t.Errorf("parseConsensus() returned %d relays, want %d", len(relays), tt.wantRelays)
			}
			// Published timestamp is parsed but not stored in Relay struct
			// This test verifies that parsing doesn't fail with valid timestamps
		})
	}
}

// TestRelayDescriptorSpecCompliance_EdgeCases verifies edge case handling
func TestRelayDescriptorSpecCompliance_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		consensus  string
		wantRelays int
		wantError  bool
	}{
		{
			name: "empty consensus",
			consensus: `network-status-version 3
vote-status consensus`,
			wantRelays: 0,
			wantError:  false,
		},
		{
			name: "consensus with only metadata",
			consensus: `network-status-version 3
vote-status consensus
valid-after 2024-01-01 00:00:00
fresh-until 2024-01-01 01:00:00
valid-until 2024-01-01 03:00:00`,
			wantRelays: 0,
			wantError:  false,
		},
		{
			name: "relay entry without flags",
			consensus: `network-status-version 3
r NoFlags AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 9001 0`,
			wantRelays: 1,
			wantError:  false,
		},
		{
			name: "relay entry without bandwidth",
			consensus: `network-status-version 3
r NoBW AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 9001 0
s Fast Running`,
			wantRelays: 1,
			wantError:  false,
		},
		{
			name: "relay with empty flags line",
			consensus: `network-status-version 3
r EmptyFlags AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 2024-01-01 00:00:00 192.0.2.1 9001 0
s `,
			wantRelays: 1,
			wantError:  false,
		},
	}

	client := NewClient(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.consensus)
			relays, err := client.parseConsensus(reader)

			if tt.wantError {
				if err == nil {
					t.Fatal("parseConsensus() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("parseConsensus() unexpected error = %v", err)
			}
			if len(relays) != tt.wantRelays {
				t.Errorf("parseConsensus() returned %d relays, want %d", len(relays), tt.wantRelays)
			}
		})
	}
}

package directory

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestNewClient(t *testing.T) {
	client := NewClient(nil)
	
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	if client.logger == nil {
		t.Error("logger should be initialized")
	}
	if client.httpClient == nil {
		t.Error("httpClient should be initialized")
	}
	if len(client.authorities) == 0 {
		t.Error("authorities should be initialized")
	}
}

func TestNewClientWithLogger(t *testing.T) {
	log := logger.NewDefault()
	client := NewClient(log)
	
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	if client.logger == nil {
		t.Error("logger should be initialized")
	}
}

func TestParseConsensus(t *testing.T) {
	// Sample consensus document fragment (matching actual format)
	consensusData := `network-status-version 3
vote-status consensus
r Test1 AAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBB 2024-01-01 00:00:00 192.168.1.1 9001 0
s Fast Guard Running Stable Valid
r Test2 CCCCCCCCCCCCCCCCCCCCCC DDDDDDDDDDDDD 2024-01-01 00:00:00 192.168.1.2 9002 9030
s Exit Fast Running Stable Valid
r Test3 EEEEEEEEEEEEEEEEEEEEEE FFFFFFFFFFFFF 2024-01-01 00:00:00 192.168.1.3 9003 0
s Running Valid
`

	client := NewClient(nil)
	reader := strings.NewReader(consensusData)
	
	relays, err := client.parseConsensus(reader)
	if err != nil {
		t.Fatalf("parseConsensus() error = %v", err)
	}
	
	if len(relays) != 3 {
		t.Errorf("parseConsensus() returned %d relays, want 3", len(relays))
		return
	}
	
	// Check first relay
	if relays[0].Nickname != "Test1" {
		t.Errorf("relay[0].Nickname = %s, want Test1", relays[0].Nickname)
	}
	if relays[0].Address != "192.168.1.1" {
		t.Errorf("relay[0].Address = %s, want 192.168.1.1", relays[0].Address)
	}
	if relays[0].ORPort != 9001 {
		t.Errorf("relay[0].ORPort = %d, want 9001", relays[0].ORPort)
	}
	if !relays[0].HasFlag("Guard") {
		t.Error("relay[0] should have Guard flag")
	}
	
	// Check second relay
	if relays[1].Nickname != "Test2" {
		t.Errorf("relay[1].Nickname = %s, want Test2", relays[1].Nickname)
	}
	if relays[1].DirPort != 9030 {
		t.Errorf("relay[1].DirPort = %d, want 9030", relays[1].DirPort)
	}
	if !relays[1].HasFlag("Exit") {
		t.Error("relay[1] should have Exit flag")
	}
}

func TestParseConsensusEmpty(t *testing.T) {
	client := NewClient(nil)
	reader := strings.NewReader("")
	
	relays, err := client.parseConsensus(reader)
	if err != nil {
		t.Fatalf("parseConsensus() error = %v", err)
	}
	
	if len(relays) != 0 {
		t.Errorf("parseConsensus() returned %d relays, want 0", len(relays))
	}
}

func TestRelayHasFlag(t *testing.T) {
	relay := &Relay{
		Nickname: "Test",
		Flags:    []string{"Fast", "Guard", "Running", "Stable", "Valid"},
	}
	
	tests := []struct {
		flag     string
		expected bool
	}{
		{"Fast", true},
		{"Guard", true},
		{"Running", true},
		{"Exit", false},
		{"NotAFlag", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			got := relay.HasFlag(tt.flag)
			if got != tt.expected {
				t.Errorf("HasFlag(%s) = %v, want %v", tt.flag, got, tt.expected)
			}
		})
	}
}

func TestRelayIsGuard(t *testing.T) {
	tests := []struct {
		name     string
		flags    []string
		expected bool
	}{
		{"with_guard_flag", []string{"Fast", "Guard", "Running"}, true},
		{"without_guard_flag", []string{"Fast", "Running"}, false},
		{"empty_flags", []string{}, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			relay := &Relay{Flags: tt.flags}
			got := relay.IsGuard()
			if got != tt.expected {
				t.Errorf("IsGuard() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRelayIsExit(t *testing.T) {
	tests := []struct {
		name     string
		flags    []string
		expected bool
	}{
		{"with_exit_flag", []string{"Exit", "Fast", "Running"}, true},
		{"without_exit_flag", []string{"Fast", "Running"}, false},
		{"empty_flags", []string{}, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			relay := &Relay{Flags: tt.flags}
			got := relay.IsExit()
			if got != tt.expected {
				t.Errorf("IsExit() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRelayIsStable(t *testing.T) {
	relay := &Relay{Flags: []string{"Fast", "Stable", "Running"}}
	if !relay.IsStable() {
		t.Error("IsStable() = false, want true")
	}
	
	relay2 := &Relay{Flags: []string{"Fast", "Running"}}
	if relay2.IsStable() {
		t.Error("IsStable() = true, want false")
	}
}

func TestRelayIsRunning(t *testing.T) {
	relay := &Relay{Flags: []string{"Running"}}
	if !relay.IsRunning() {
		t.Error("IsRunning() = false, want true")
	}
	
	relay2 := &Relay{Flags: []string{"Fast"}}
	if relay2.IsRunning() {
		t.Error("IsRunning() = true, want false")
	}
}

func TestRelayIsValid(t *testing.T) {
	relay := &Relay{Flags: []string{"Valid", "Running"}}
	if !relay.IsValid() {
		t.Error("IsValid() = false, want true")
	}
	
	relay2 := &Relay{Flags: []string{"Running"}}
	if relay2.IsValid() {
		t.Error("IsValid() = true, want false")
	}
}

func TestRelayString(t *testing.T) {
	relay := &Relay{
		Nickname: "TestRelay",
		Address:  "192.168.1.1",
		ORPort:   9001,
	}
	
	expected := "TestRelay (192.168.1.1:9001)"
	got := relay.String()
	
	if got != expected {
		t.Errorf("String() = %s, want %s", got, expected)
	}
}

func TestFetchConsensusTimeout(t *testing.T) {
	client := NewClient(nil)
	// Use invalid authorities to test timeout
	client.authorities = []string{"http://192.0.2.1:9999/consensus"}
	client.httpClient.Timeout = 100 * time.Millisecond
	
	ctx := context.Background()
	_, err := client.FetchConsensus(ctx)
	
	if err == nil {
		t.Error("FetchConsensus() should fail with invalid authority")
	}
}

func TestFetchConsensusContextCanceled(t *testing.T) {
	client := NewClient(nil)
	client.authorities = []string{"http://192.0.2.1:9999/consensus"}
	
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	
	_, err := client.FetchConsensus(ctx)
	
	if err == nil {
		t.Error("FetchConsensus() should fail with canceled context")
	}
}

func TestDefaultAuthorities(t *testing.T) {
	if len(DefaultAuthorities) == 0 {
		t.Error("DefaultAuthorities should not be empty")
	}
	
	for i, auth := range DefaultAuthorities {
		if !strings.HasPrefix(auth, "https://") && !strings.HasPrefix(auth, "http://") {
			t.Errorf("DefaultAuthorities[%d] = %s, should start with http:// or https://", i, auth)
		}
	}
}

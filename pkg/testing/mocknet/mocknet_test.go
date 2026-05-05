package mocknet_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/opd-ai/go-tor/pkg/testing/mocknet"
)

func TestNew_EmptyNetwork(t *testing.T) {
	n := mocknet.New()
	if n == nil {
		t.Fatal("New returned nil")
	}
	if len(n.Relays()) != 0 {
		t.Errorf("expected 0 relays, got %d", len(n.Relays()))
	}
}

func TestAddRelay_Defaults(t *testing.T) {
	n := mocknet.New()
	r := n.AddRelay(mocknet.RelayConfig{})

	if r.Nickname == "" {
		t.Error("Nickname should be set to a default")
	}
	if r.Fingerprint == "" {
		t.Error("Fingerprint should be generated automatically")
	}
	if r.NtorKey == "" {
		t.Error("NtorKey should be generated automatically")
	}
	if r.Bandwidth == 0 {
		t.Error("Bandwidth should default to non-zero")
	}
	if r.Address == "" {
		t.Error("Address should default to non-empty")
	}
}

func TestAddRelay_CustomConfig(t *testing.T) {
	n := mocknet.New()
	r := n.AddRelay(mocknet.RelayConfig{
		Nickname:  "TestGuard",
		Flags:     []string{"Guard", "Stable", "Running", "Valid"},
		Bandwidth: 5000,
		Address:   "127.0.0.1:9001",
	})

	if r.Nickname != "TestGuard" {
		t.Errorf("Nickname: got %q, want TestGuard", r.Nickname)
	}
	if r.Bandwidth != 5000 {
		t.Errorf("Bandwidth: got %d, want 5000", r.Bandwidth)
	}
	if r.Address != "127.0.0.1:9001" {
		t.Errorf("Address: got %q, want 127.0.0.1:9001", r.Address)
	}
}

func TestRelays_ReturnsSnapshot(t *testing.T) {
	n := mocknet.New()
	n.AddRelay(mocknet.RelayConfig{Nickname: "A"})
	n.AddRelay(mocknet.RelayConfig{Nickname: "B"})
	n.AddRelay(mocknet.RelayConfig{Nickname: "C"})

	relays := n.Relays()
	if len(relays) != 3 {
		t.Errorf("expected 3 relays, got %d", len(relays))
	}
}

func TestStartDirectory_ServesConsensus(t *testing.T) {
	n := mocknet.New()
	n.AddRelay(mocknet.RelayConfig{
		Nickname:  "Guard1",
		Flags:     []string{"Guard", "Stable", "Running", "Valid"},
		Bandwidth: 2000,
	})
	n.AddRelay(mocknet.RelayConfig{
		Nickname:  "Middle1",
		Flags:     []string{"Stable", "Running", "Valid"},
		Bandwidth: 1000,
	})

	dir, err := n.StartDirectory()
	if err != nil {
		t.Fatalf("StartDirectory: %v", err)
	}
	defer dir.Stop()

	url := dir.ConsensusURL()
	if url == "" {
		t.Fatal("ConsensusURL is empty")
	}

	resp, err := http.Get(url) // #nosec G107 - test code fetching from local mock server
	if err != nil {
		t.Fatalf("GET consensus: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	text := string(body)
	if !strings.Contains(text, "network-status-version 3") {
		t.Error("consensus missing 'network-status-version 3'")
	}
	if !strings.Contains(text, "Guard1") {
		t.Error("consensus missing relay Guard1")
	}
	if !strings.Contains(text, "Middle1") {
		t.Error("consensus missing relay Middle1")
	}
}

func TestStartDirectory_Addr(t *testing.T) {
	n := mocknet.New()
	dir, err := n.StartDirectory()
	if err != nil {
		t.Fatalf("StartDirectory: %v", err)
	}
	defer dir.Stop()

	if dir.Addr() == "" {
		t.Error("Addr should not be empty")
	}
	if !strings.Contains(dir.Addr(), ":") {
		t.Errorf("Addr should contain port: %s", dir.Addr())
	}
}

func TestDirectory_StopIdempotent(t *testing.T) {
	n := mocknet.New()
	dir, err := n.StartDirectory()
	if err != nil {
		t.Fatalf("StartDirectory: %v", err)
	}
	if err := dir.Stop(); err != nil {
		t.Fatalf("first Stop: %v", err)
	}
	// Second stop should not panic; may return error (acceptable).
	_ = dir.Stop()
}

func TestStartDirectory_EmptyNetwork(t *testing.T) {
	n := mocknet.New()
	dir, err := n.StartDirectory()
	if err != nil {
		t.Fatalf("StartDirectory: %v", err)
	}
	defer dir.Stop()

	resp, err := http.Get(dir.ConsensusURL()) // #nosec G107
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
}

func TestStartDirectory_FingerprintsUnique(t *testing.T) {
	n := mocknet.New()
	seen := map[string]bool{}
	for i := 0; i < 10; i++ {
		r := n.AddRelay(mocknet.RelayConfig{})
		if seen[r.Fingerprint] {
			t.Errorf("duplicate fingerprint: %s", r.Fingerprint)
		}
		seen[r.Fingerprint] = true
	}
}

func TestNetwork_MultipleDirectories(t *testing.T) {
	n := mocknet.New()
	n.AddRelay(mocknet.RelayConfig{Nickname: "R1"})

	d1, err := n.StartDirectory()
	if err != nil {
		t.Fatalf("first StartDirectory: %v", err)
	}
	defer d1.Stop()

	d2, err := n.StartDirectory()
	if err != nil {
		t.Fatalf("second StartDirectory: %v", err)
	}
	defer d2.Stop()

	// Both should serve the same consensus.
	for _, url := range []string{d1.ConsensusURL(), d2.ConsensusURL()} {
		resp, err := http.Get(url) // #nosec G107
		if err != nil {
			t.Fatalf("GET %s: %v", url, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status from %s: got %d, want 200", url, resp.StatusCode)
		}
	}
}

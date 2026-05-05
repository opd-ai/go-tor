// Package mocknet provides a lightweight mock Tor network environment for
// integration testing. It enables tests to exercise path selection, directory
// fetching, and circuit-level logic without connecting to the live Tor network.
//
// # Architecture
//
// A [Network] holds a set of [MockRelay] descriptors and an HTTP-based
// [MockDirectory] that serves a synthesised consensus document.  Tests can
// configure the relay topology, start the directory server on a local port,
// and inject the server URL into the directory client under test.
//
// # Example
//
//	net := mocknet.New()
//	net.AddRelay(mocknet.RelayConfig{
//	    Nickname: "Guard1",
//	    Flags:    []string{"Guard", "Stable", "Running", "Valid"},
//	    Bandwidth: 1000,
//	})
//	net.AddRelay(mocknet.RelayConfig{
//	    Nickname: "Middle1",
//	    Flags:    []string{"Stable", "Running", "Valid"},
//	    Bandwidth: 500,
//	})
//
//	dir, err := net.StartDirectory()
//	if err != nil {
//	    t.Fatal(err)
//	}
//	defer dir.Stop()
//
//	// URL to feed to a directory client under test:
//	consensusURL := dir.ConsensusURL()
package mocknet

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RelayConfig describes a single relay in the mock network.
type RelayConfig struct {
	// Nickname is the relay's human-readable name.
	Nickname string
	// Flags are the relay's consensus flags (e.g., "Guard", "Stable", "Running", "Valid").
	Flags []string
	// Bandwidth is the relay's bandwidth weight (KB/s).
	Bandwidth int
	// Address is the relay's IP:ORport; defaults to "127.0.0.1:9001".
	Address string
}

// MockRelay holds synthesised relay state used to build the consensus.
type MockRelay struct {
	RelayConfig
	// Fingerprint is the relay's 20-byte identity digest (hex).
	Fingerprint string
	// NtorKey is a random 32-byte ntor onion key (base64).
	NtorKey string
}

// Network is a mock Tor network.  Add relays with AddRelay, then call
// StartDirectory to expose a consensus document over HTTP.
type Network struct {
	mu     sync.Mutex
	relays []*MockRelay
}

// New creates an empty mock network.
func New() *Network {
	return &Network{}
}

// AddRelay adds a relay to the mock network. A fingerprint and ntor key
// are generated automatically if not already set.
func (n *Network) AddRelay(cfg RelayConfig) *MockRelay {
	n.mu.Lock()
	defer n.mu.Unlock()

	r := &MockRelay{RelayConfig: cfg}
	r.Fingerprint = randomHex(20)
	r.NtorKey = randomBase64(32)
	if cfg.Nickname == "" {
		r.Nickname = fmt.Sprintf("Relay%d", len(n.relays)+1)
	}
	if cfg.Bandwidth == 0 {
		r.Bandwidth = 1000
	}
	if cfg.Address == "" {
		r.Address = "127.0.0.1:9001"
	}
	n.relays = append(n.relays, r)
	return r
}

// Relays returns a snapshot of all relays in the network.
func (n *Network) Relays() []*MockRelay {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make([]*MockRelay, len(n.relays))
	copy(out, n.relays)
	return out
}

// consensus renders a minimal Tor consensus document containing all relays.
// The format is a stripped-down version of the microdesc consensus that the
// directory client can parse; only the fields strictly needed for relay
// selection are populated.
func (n *Network) consensus() string {
	n.mu.Lock()
	relays := make([]*MockRelay, len(n.relays))
	copy(relays, n.relays)
	n.mu.Unlock()

	now := time.Now().UTC()
	fresh := now.Add(3 * time.Hour)

	var sb strings.Builder

	// Preamble
	fmt.Fprintf(&sb, "network-status-version 3 microdesc\n")
	fmt.Fprintf(&sb, "vote-status consensus\n")
	fmt.Fprintf(&sb, "consensus-method 28\n")
	fmt.Fprintf(&sb, "valid-after %s\n", now.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&sb, "fresh-until %s\n", fresh.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&sb, "valid-until %s\n", fresh.Add(3*time.Hour).Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&sb, "voting-delay 300 300\n")
	fmt.Fprintf(&sb, "client-versions 0.4.7.0\n")
	fmt.Fprintf(&sb, "server-versions 0.4.7.0\n")
	fmt.Fprintf(&sb, "known-flags Authority Exit Fast Guard HSDir Running Stable V2Dir Valid\n")
	fmt.Fprintf(&sb, "params CircuitPriorityHalflifeMsec=30000 NumDirectoryGuards=3 NumEntryGuards=1\n")
	fmt.Fprintf(&sb, "shared-rand-current-value 1 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n")
	fmt.Fprintf(&sb, "shared-rand-previous-value 1 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n")
	fmt.Fprintf(&sb, "bandwidth-weights Wbd=0 Wbe=0 Wbg=4096 Wbm=10000 Wdb=10000 Web=10000 Wed=10000 Wee=10000 Weg=10000 Wem=10000 Wgb=10000 Wgd=0 Wgg=5904 Wgm=5904 Wmb=10000 Wmd=0 Wme=0 Wmg=4096 Wmm=10000\n")

	// Router entries
	for _, r := range relays {
		host, port, _ := net.SplitHostPort(r.Address)
		if host == "" {
			host = "127.0.0.1"
		}
		if port == "" {
			port = "9001"
		}

		flags := strings.Join(r.Flags, " ")
		fmt.Fprintf(&sb, "r %s %s %s 2026-01-01 00:00:00 %s %s 0\n",
			r.Nickname,
			base64.StdEncoding.EncodeToString(hexToBytes(r.Fingerprint)),
			base64.StdEncoding.EncodeToString([]byte(r.Fingerprint)),
			host, port)
		fmt.Fprintf(&sb, "s %s\n", flags)
		fmt.Fprintf(&sb, "v Tor 0.4.7.0\n")
		fmt.Fprintf(&sb, "pr Cons=1-2 Desc=1-2 DirCache=1-2 HSDir=1-2 HSIntro=4-5 HSRend=1-2 Link=1-5 Microdesc=1-2 Relay=2\n")
		fmt.Fprintf(&sb, "w Bandwidth=%d\n", r.Bandwidth)
		fmt.Fprintf(&sb, "m %s\n", r.NtorKey)
	}

	fmt.Fprintf(&sb, "directory-footer\n")
	fmt.Fprintf(&sb, "bandwidth-weights Wbd=0 Wbe=0 Wbg=4096 Wbm=10000 Wdb=10000 Web=10000 Wed=10000 Wee=10000 Weg=10000 Wem=10000 Wgb=10000 Wgd=0 Wgg=5904 Wgm=5904 Wmb=10000 Wmd=0 Wme=0 Wmg=4096 Wmm=10000\n")
	return sb.String()
}

// Directory is a running mock HTTP directory server.
type Directory struct {
	network  *Network
	listener net.Listener
	server   *http.Server
	addr     string
}

// StartDirectory starts an HTTP server that serves the mock consensus.
// Call Stop when done.
func (n *Network) StartDirectory() (*Directory, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	d := &Directory{
		network:  n,
		listener: ln,
		addr:     ln.Addr().String(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/tor/status-vote/current/consensus-microdesc", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(n.consensus()))
	})
	mux.HandleFunc("/tor/status-vote/current/consensus", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(n.consensus()))
	})

	d.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() { _ = d.server.Serve(ln) }()
	return d, nil
}

// ConsensusURL returns the full URL for the microdesc consensus endpoint.
func (d *Directory) ConsensusURL() string {
	return fmt.Sprintf("http://%s/tor/status-vote/current/consensus-microdesc", d.addr)
}

// Addr returns the listening address (e.g., "127.0.0.1:54321").
func (d *Directory) Addr() string { return d.addr }

// Stop shuts down the directory server.
func (d *Directory) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return d.server.Shutdown(ctx)
}

// --- helpers ---

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("mocknet: failed to read random bytes: %v", err))
	}
	return fmt.Sprintf("%X", b)
}

func randomBase64(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("mocknet: failed to read random bytes: %v", err))
	}
	return base64.StdEncoding.EncodeToString(b)
}

func hexToBytes(h string) []byte {
	b := make([]byte, len(h)/2)
	for i := range b {
		fmt.Sscanf(h[i*2:i*2+2], "%02X", &b[i])
	}
	return b
}

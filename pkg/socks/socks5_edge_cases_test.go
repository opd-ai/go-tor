package socks

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/metrics"
)

// helperServer creates a Server using net.Pipe for direct testing.
func helperServer(t *testing.T) *Server {
	t.Helper()
	log := logger.NewDefault()
	mgr := circuit.NewManager()
	return NewServer("127.0.0.1:0", mgr, log)
}

// helperHandshake writes a handshake and reads the server response via pipe.
func helperHandshake(t *testing.T, srv *Server, input []byte) ([]byte, error) {
	t.Helper()
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	errCh := make(chan error, 1)
	go func() {
		_, err := srv.handshake(server)
		errCh <- err
	}()

	if _, err := client.Write(input); err != nil {
		return nil, err
	}
	client.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 64)
	n, _ := client.Read(buf)
	// Wait for handshake to complete
	select {
	case err := <-errCh:
		return buf[:n], err
	case <-time.After(time.Second):
		return buf[:n], nil
	}
}

func TestEdgeHandshakeZeroMethods(t *testing.T) {
	srv := helperServer(t)
	// nmethods=0: no authentication methods offered
	input := []byte{socks5Version, 0x00}
	resp, err := helperHandshake(t, srv, input)

	// Server should reject with 0xFF (no acceptable methods)
	if err == nil {
		t.Fatal("expected error for zero methods")
	}
	if len(resp) >= 2 && resp[1] != authNoAccept {
		t.Errorf("expected authNoAccept (0xFF), got 0x%02X", resp[1])
	}
}

func TestEdgeHandshakeMaxMethods(t *testing.T) {
	srv := helperServer(t)
	// nmethods=255, all set to authNone
	input := make([]byte, 2+255)
	input[0] = socks5Version
	input[1] = 255
	for i := 2; i < len(input); i++ {
		input[i] = authNone
	}

	resp, err := helperHandshake(t, srv, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp) < 2 {
		t.Fatal("response too short")
	}
	if resp[0] != socks5Version || resp[1] != authNone {
		t.Errorf("expected [0x05,0x00], got [0x%02X,0x%02X]", resp[0], resp[1])
	}
}

func TestEdgeHandshakeClosedMidway(t *testing.T) {
	srv := helperServer(t)
	client, server := net.Pipe()
	defer server.Close()

	errCh := make(chan error, 1)
	go func() {
		_, err := srv.handshake(server)
		errCh <- err
	}()

	// Send only version byte then close
	client.Write([]byte{socks5Version})
	client.Close()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error when connection closed mid-handshake")
		}
	case <-time.After(time.Second):
		t.Fatal("handshake did not return")
	}
}

func TestEdgePasswordAuthBoundary(t *testing.T) {
	srv := helperServer(t)

	tests := []struct {
		name     string
		username []byte
		password []byte
	}{
		{"zero-length username", []byte{}, []byte("pass")},
		{"zero-length password", []byte("user"), []byte{}},
		{"max-length username (255)", bytes.Repeat([]byte("A"), 255), []byte("p")},
		{"max-length password (255)", []byte("u"), bytes.Repeat([]byte("B"), 255)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := net.Pipe()
			defer client.Close()
			defer server.Close()

			type result struct {
				username string
				err      error
			}
			resCh := make(chan result, 1)
			go func() {
				u, err := srv.handshake(server)
				resCh <- result{u, err}
			}()

			// Handshake: offer password auth
			handshakeMsg := []byte{socks5Version, 0x01, authPassword}
			client.Write(handshakeMsg)

			// Read method selection response
			resp := make([]byte, 2)
			io.ReadFull(client, resp)

			// Send auth sub-negotiation (RFC 1929)
			var authMsg bytes.Buffer
			authMsg.WriteByte(0x01) // version
			authMsg.WriteByte(byte(len(tt.username)))
			authMsg.Write(tt.username)
			authMsg.WriteByte(byte(len(tt.password)))
			authMsg.Write(tt.password)
			client.Write(authMsg.Bytes())

			// Read auth response
			authResp := make([]byte, 2)
			io.ReadFull(client, authResp)

			select {
			case r := <-resCh:
				if r.err != nil {
					t.Fatalf("unexpected error: %v", r.err)
				}
				if r.username != string(tt.username) {
					t.Errorf("username = %q, want %q", r.username, string(tt.username))
				}
			case <-time.After(time.Second):
				t.Fatal("timeout")
			}
		})
	}
}

func TestEdgePasswordAuthVersionMismatch(t *testing.T) {
	srv := helperServer(t)
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	errCh := make(chan error, 1)
	go func() {
		_, err := srv.handshake(server)
		errCh <- err
	}()

	// Offer password auth
	client.Write([]byte{socks5Version, 0x01, authPassword})
	resp := make([]byte, 2)
	io.ReadFull(client, resp)

	// Send auth with wrong version (0x02 instead of 0x01) in a goroutine
	// because the server will return error after reading version byte,
	// and net.Pipe is synchronous.
	go func() {
		client.Write([]byte{0x02, 0x04, 'u', 's', 'e', 'r', 0x04, 'p', 'a', 's', 's'})
	}()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error for auth version mismatch")
		}
		if !strings.Contains(err.Error(), "unsupported auth version") {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestEdgeRequestParsingAddresses(t *testing.T) {
	srv := helperServer(t)

	tests := []struct {
		name    string
		request []byte
		wantErr bool
	}{
		{
			name: "domain zero length",
			request: func() []byte {
				var buf bytes.Buffer
				buf.Write([]byte{socks5Version, cmdConnect, 0x00, addrDomain})
				buf.WriteByte(0) // domain length = 0
				binary.Write(&buf, binary.BigEndian, uint16(80))
				return buf.Bytes()
			}(),
			wantErr: false, // empty domain is accepted
		},
		{
			name: "domain max length 255",
			request: func() []byte {
				var buf bytes.Buffer
				buf.Write([]byte{socks5Version, cmdConnect, 0x00, addrDomain})
				domain := strings.Repeat("a", 255)
				buf.WriteByte(255)
				buf.WriteString(domain)
				binary.Write(&buf, binary.BigEndian, uint16(443))
				return buf.Bytes()
			}(),
			wantErr: false,
		},
		{
			name: "port 0",
			request: func() []byte {
				var buf bytes.Buffer
				buf.Write([]byte{socks5Version, cmdConnect, 0x00, addrIPv4})
				buf.Write(net.ParseIP("10.0.0.1").To4())
				binary.Write(&buf, binary.BigEndian, uint16(0))
				return buf.Bytes()
			}(),
			wantErr: false,
		},
		{
			name: "port 65535",
			request: func() []byte {
				var buf bytes.Buffer
				buf.Write([]byte{socks5Version, cmdConnect, 0x00, addrIPv4})
				buf.Write(net.ParseIP("10.0.0.1").To4())
				binary.Write(&buf, binary.BigEndian, uint16(65535))
				return buf.Bytes()
			}(),
			wantErr: false,
		},
		{
			name: "IPv4 0.0.0.0",
			request: func() []byte {
				var buf bytes.Buffer
				buf.Write([]byte{socks5Version, cmdConnect, 0x00, addrIPv4})
				buf.Write([]byte{0, 0, 0, 0})
				binary.Write(&buf, binary.BigEndian, uint16(80))
				return buf.Bytes()
			}(),
			wantErr: false,
		},
		{
			name: "IPv4 255.255.255.255",
			request: func() []byte {
				var buf bytes.Buffer
				buf.Write([]byte{socks5Version, cmdConnect, 0x00, addrIPv4})
				buf.Write([]byte{255, 255, 255, 255})
				binary.Write(&buf, binary.BigEndian, uint16(80))
				return buf.Bytes()
			}(),
			wantErr: false,
		},
		{
			name: "IPv6 all zeros",
			request: func() []byte {
				var buf bytes.Buffer
				buf.Write([]byte{socks5Version, cmdConnect, 0x00, addrIPv6})
				buf.Write(make([]byte, 16)) // ::
				binary.Write(&buf, binary.BigEndian, uint16(80))
				return buf.Bytes()
			}(),
			wantErr: false,
		},
		{
			name: "IPv6 max values",
			request: func() []byte {
				var buf bytes.Buffer
				buf.Write([]byte{socks5Version, cmdConnect, 0x00, addrIPv6})
				ip := make([]byte, 16)
				for i := range ip {
					ip[i] = 0xFF
				}
				buf.Write(ip)
				binary.Write(&buf, binary.BigEndian, uint16(80))
				return buf.Bytes()
			}(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := net.Pipe()
			defer client.Close()
			defer server.Close()

			type result struct {
				req *requestInfo
				err error
			}
			resCh := make(chan result, 1)
			go func() {
				req, err := srv.readRequest(server)
				resCh <- result{req, err}
			}()

			client.Write(tt.request)

			select {
			case r := <-resCh:
				if tt.wantErr && r.err == nil {
					t.Error("expected error")
				}
				if !tt.wantErr && r.err != nil {
					t.Errorf("unexpected error: %v", r.err)
				}
			case <-time.After(time.Second):
				t.Fatal("timeout")
			}
		})
	}
}

func TestEdgeServerDoubleShutdown(t *testing.T) {
	srv := helperServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go srv.ListenAndServe(ctx)
	srv.ListenerAddr() // wait for server ready

	// First shutdown
	err1 := srv.Shutdown(context.Background())
	if err1 != nil {
		t.Fatalf("first shutdown failed: %v", err1)
	}

	// Second shutdown should not panic
	err2 := srv.Shutdown(context.Background())
	if err2 != nil {
		t.Fatalf("second shutdown failed: %v", err2)
	}
}

func TestEdgeSetCircuitPoolNil(t *testing.T) {
	srv := helperServer(t)
	// Should not panic
	srv.SetCircuitPool(nil)
	if srv.circuitPool != nil {
		t.Error("expected circuitPool to be nil")
	}
}

func TestEdgeSetMetricsNil(t *testing.T) {
	srv := helperServer(t)
	// Set non-nil first
	srv.SetMetrics(&metrics.Metrics{})
	// Then set nil
	srv.SetMetrics(nil)
	if srv.metrics != nil {
		t.Error("expected metrics to be nil after SetMetrics(nil)")
	}
}

func TestEdgeSendReplyNilBindAddr(t *testing.T) {
	srv := helperServer(t)
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	done := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 32)
		n, _ := client.Read(buf)
		done <- buf[:n]
	}()

	err := srv.sendReply(server, replySuccess, nil)
	if err != nil {
		t.Fatalf("sendReply error: %v", err)
	}

	select {
	case resp := <-done:
		// With nil bindAddr: version + reply + reserved + addrIPv4 + 4 zeros + 2 port zeros = 10
		if len(resp) != 10 {
			t.Fatalf("expected 10 bytes, got %d", len(resp))
		}
		if resp[3] != addrIPv4 {
			t.Errorf("expected addrIPv4, got 0x%02X", resp[3])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestEdgeSendReplyIPv6BindAddr(t *testing.T) {
	srv := helperServer(t)
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	done := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 64)
		n, _ := client.Read(buf)
		done <- buf[:n]
	}()

	bindAddr := &net.TCPAddr{IP: net.ParseIP("fe80::1"), Port: 9050}
	err := srv.sendReply(server, replySuccess, bindAddr)
	if err != nil {
		t.Fatalf("sendReply error: %v", err)
	}

	select {
	case resp := <-done:
		// version(1) + reply(1) + reserved(1) + addrType(1) + IPv6(16) + port(2) = 22
		if len(resp) != 22 {
			t.Fatalf("expected 22 bytes, got %d", len(resp))
		}
		if resp[3] != addrIPv6 {
			t.Errorf("expected addrIPv6, got 0x%02X", resp[3])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestEdgeSendDNSReplyEmptyAddresses(t *testing.T) {
	srv := helperServer(t)
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	done := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 32)
		n, _ := client.Read(buf)
		done <- buf[:n]
	}()

	err := srv.sendDNSReply(server, replySuccess, nil, 0)
	if err != nil {
		t.Fatalf("sendDNSReply error: %v", err)
	}

	select {
	case resp := <-done:
		// With empty addresses even on success: header(4) + null IPv4(4) + TTL(4) = 12
		if len(resp) != 12 {
			t.Fatalf("expected 12 bytes, got %d", len(resp))
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestEdgeSendDNSReplyIPv6(t *testing.T) {
	srv := helperServer(t)
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	done := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 64)
		n, _ := client.Read(buf)
		done <- buf[:n]
	}()

	ipv6 := net.ParseIP("2001:db8::1")
	err := srv.sendDNSReply(server, replySuccess, []net.IP{ipv6}, 3600)
	if err != nil {
		t.Fatalf("sendDNSReply error: %v", err)
	}

	select {
	case resp := <-done:
		// header(4) + IPv6(16) + TTL(4) = 24
		if len(resp) != 24 {
			t.Fatalf("expected 24 bytes, got %d", len(resp))
		}
		if resp[3] != addrIPv6 {
			t.Errorf("expected addrIPv6, got 0x%02X", resp[3])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestEdgeSendDNSReplyHostnameEdgeCases(t *testing.T) {
	srv := helperServer(t)

	tests := []struct {
		name        string
		hostname    string
		wantErr     bool
		expectEmpty bool
	}{
		{"empty hostname", "", false, true},
		{"exactly 255 bytes", strings.Repeat("x", 255), false, false},
		{"exceeds 255 bytes", strings.Repeat("y", 256), true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := net.Pipe()
			defer client.Close()
			defer server.Close()

			done := make(chan []byte, 1)
			go func() {
				buf := make([]byte, 512)
				n, _ := client.Read(buf)
				done <- buf[:n]
			}()

			err := srv.sendDNSReplyHostname(server, replySuccess, tt.hostname, 300)

			if tt.wantErr && err == nil {
				t.Error("expected error")
			}

			select {
			case resp := <-done:
				if len(resp) < 4 {
					t.Fatal("response too short")
				}
				if tt.expectEmpty && resp[4] != 0 {
					t.Errorf("expected empty hostname length, got %d", resp[4])
				}
			case <-time.After(time.Second):
				t.Fatal("timeout")
			}
		})
	}
}

func TestEdgeExtractClientIPEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"full IPv6 with port", "[2001:db8::1]:443", "2001:db8::1"},
		{"empty string", "", ""},
		{"just IP no port", "10.0.0.1", "10.0.0.1"},
		{"IPv6 without brackets", "::1", "::1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractClientIP(tt.input)
			if got != tt.want {
				t.Errorf("extractClientIP(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEdgeConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxConnections != defaultMaxConnections {
		t.Errorf("MaxConnections = %d, want %d", cfg.MaxConnections, defaultMaxConnections)
	}
	if !cfg.EnableDNSResolution {
		t.Error("EnableDNSResolution should be true by default")
	}
	if cfg.DNSTimeout != 30*time.Second {
		t.Errorf("DNSTimeout = %v, want 30s", cfg.DNSTimeout)
	}
	if cfg.ConnectionsPerSecond != 100.0 {
		t.Errorf("ConnectionsPerSecond = %f, want 100.0", cfg.ConnectionsPerSecond)
	}
	if cfg.ConnectionsBurst != 50 {
		t.Errorf("ConnectionsBurst = %d, want 50", cfg.ConnectionsBurst)
	}
}

func TestEdgeConfigZeroMaxConnections(t *testing.T) {
	log := logger.NewDefault()
	mgr := circuit.NewManager()

	cfg := &Config{
		MaxConnections:  0, // unlimited
		IsolationMode:   "off",
		EnableDNSResolution: true,
		DNSTimeout:      30 * time.Second,
	}

	srv := NewServerWithConfig("127.0.0.1:0", mgr, log, cfg)
	if srv.config.MaxConnections != 0 {
		t.Errorf("MaxConnections = %d, want 0", srv.config.MaxConnections)
	}
}

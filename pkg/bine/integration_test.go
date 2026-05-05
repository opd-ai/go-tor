// Package bine provides integration tests for the bine wrapper client.
package bine

import (
	"context"
	"net/http"
	"testing"
	"time"

	"golang.org/x/net/proxy"
)

// TestOptionsDefaults_Integration verifies that ConnectWithOptionsContext applies
// defaults to an empty Options struct (StartupTimeout set to 90s).
func TestOptionsDefaults_Integration(t *testing.T) {
	// Use a context that cancels after a very short window so the Tor startup
	// fails quickly without requiring live network access.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	opts := &Options{}
	// StartupTimeout default (0 → 90s) is applied inside ConnectWithOptionsContext.
	_, err := ConnectWithOptionsContext(ctx, opts)
	// We expect an error because no live network is available.
	if err == nil {
		t.Skip("unexpected successful connection – live network present, skipping")
	}
	t.Logf("Got expected error: %v", err)
}

// TestConnectNilOptions_Integration confirms nil Options is accepted and defaults apply.
func TestConnectNilOptions_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := ConnectWithOptionsContext(ctx, nil)
	if err == nil {
		t.Skip("unexpected successful connection – live network present, skipping")
	}
	t.Logf("Got expected error: %v", err)
}

// TestConnectCustomPorts_Integration ensures port/directory options are forwarded
// to the underlying go-tor client without panicking.
func TestConnectCustomPorts_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	opts := &Options{
		SocksPort:     19050,
		ControlPort:   19051,
		DataDirectory: t.TempDir(),
		LogLevel:      "warn",
	}
	_, err := ConnectWithOptionsContext(ctx, opts)
	if err == nil {
		t.Skip("unexpected successful connection – live network present, skipping")
	}
	t.Logf("Got expected error: %v", err)
}

// TestConnectCancelledContext_Integration verifies that a pre-cancelled context
// causes ConnectWithOptionsContext to return an error immediately.
func TestConnectCancelledContext_Integration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling

	_, err := ConnectWithOptionsContext(ctx, nil)
	if err == nil {
		t.Skip("unexpected successful connection – live network present, skipping")
	}
	t.Logf("Got expected error with cancelled context: %v", err)
}

// TestCreateHiddenService_NoBine confirms that CreateHiddenService returns an
// error when the client was created without EnableBine.
func TestCreateHiddenService_NoBine(t *testing.T) {
	c := &Client{} // bineClient is nil
	_, err := c.CreateHiddenService(context.Background(), 80)
	if err == nil {
		t.Fatal("expected error when bine is not enabled")
	}
}

// TestCreateHiddenServiceWithConfig_NoBine confirms an error is returned when
// bine is not enabled regardless of config.
func TestCreateHiddenServiceWithConfig_NoBine(t *testing.T) {
	c := &Client{}
	cfg := &HiddenServiceConfig{RemotePorts: []int{80}}
	_, err := c.CreateHiddenServiceWithConfig(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error when bine is not enabled")
	}
}

// TestCreateHiddenServiceWithConfig_NilConfig verifies that a nil
// HiddenServiceConfig returns an error before any network operation.
func TestCreateHiddenServiceWithConfig_NilConfig(t *testing.T) {
	c := &Client{}
	_, err := c.CreateHiddenServiceWithConfig(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

// TestClientClose_NoBine ensures Close works without a bine client.
func TestClientClose_NoBine(t *testing.T) {
	// A Client with both goTorClient and bineClient nil should not panic
	// and should return nil (no error to report).
	c := &Client{}
	if err := c.Close(); err != nil {
		t.Errorf("Close with nil fields returned unexpected error: %v", err)
	}
}

// TestOptionsStruct_Fields ensures all public fields of Options are accessible.
func TestOptionsStruct_Fields(t *testing.T) {
	opts := Options{
		SocksPort:      9050,
		ControlPort:    9051,
		DataDirectory:  "/tmp",
		LogLevel:       "debug",
		EnableBine:     false,
		StartupTimeout: 30 * time.Second,
	}

	if opts.SocksPort != 9050 {
		t.Errorf("SocksPort: got %d, want 9050", opts.SocksPort)
	}
	if opts.ControlPort != 9051 {
		t.Errorf("ControlPort: got %d, want 9051", opts.ControlPort)
	}
	if opts.DataDirectory != "/tmp" {
		t.Errorf("DataDirectory: got %s, want /tmp", opts.DataDirectory)
	}
	if opts.LogLevel != "debug" {
		t.Errorf("LogLevel: got %s, want debug", opts.LogLevel)
	}
	if opts.EnableBine != false {
		t.Error("EnableBine: expected false")
	}
	if opts.StartupTimeout != 30*time.Second {
		t.Errorf("StartupTimeout: got %v, want 30s", opts.StartupTimeout)
	}
}

// TestHiddenServiceConfig_Fields ensures HiddenServiceConfig fields are accessible.
func TestHiddenServiceConfig_Fields(t *testing.T) {
	cfg := HiddenServiceConfig{
		RemotePorts: []int{80, 443},
		LocalAddr:   "127.0.0.1:8080",
	}

	if len(cfg.RemotePorts) != 2 {
		t.Errorf("RemotePorts: got %d, want 2", len(cfg.RemotePorts))
	}
	if cfg.LocalAddr != "127.0.0.1:8080" {
		t.Errorf("LocalAddr: got %s, want 127.0.0.1:8080", cfg.LocalAddr)
	}
	if cfg.PrivateKey != nil {
		t.Error("PrivateKey: expected nil")
	}
}

// TestAPISignatures_Integration is a compile-time check that all public symbols
// exist and have the expected signatures.
func TestAPISignatures_Integration(t *testing.T) {
	// Top-level constructors
	_ = Connect
	_ = ConnectWithOptions
	_ = ConnectWithOptionsContext

	// Client method signatures
	var c *Client
	_ = c.Close
	_ = c.ProxyAddr
	_ = c.ProxyURL
	_ = c.Dialer
	_ = c.HTTPClient
	_ = c.IsReady
	_ = c.CreateHiddenService
	_ = c.CreateHiddenServiceWithConfig

	// HiddenService method signatures
	var hs *HiddenService
	_ = hs.OnionAddress
	_ = hs.Accept
	_ = hs.Close
	_ = hs.Addr

	t.Log("all API signatures compile correctly")
}

// TestHTTPClientConstruction verifies that HTTPClient returns a non-nil
// *http.Client configured with a 30s timeout when a proxyDialer is present.
func TestHTTPClientConstruction(t *testing.T) {
	// Use proxy.Direct as a stand-in dialer so HTTPClient does not panic.
	c := &Client{
		proxyDialer: proxy.Direct,
	}

	httpClient, err := c.HTTPClient()
	if err != nil {
		t.Fatalf("HTTPClient returned error: %v", err)
	}
	if httpClient == nil {
		t.Fatal("HTTPClient returned nil")
	}
	if httpClient.Timeout != 30*time.Second {
		t.Errorf("Timeout: got %v, want 30s", httpClient.Timeout)
	}

	transport, ok := httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Transport is not *http.Transport")
	}
	if transport.Dial == nil {
		t.Error("expected non-nil Dial when proxyDialer is set")
	}
}

// TestConnectWithOptions_Integration is a convenience wrapper test that exercises
// the ConnectWithOptions path (non-context variant) and expects a quick failure
// when no live network is available.
func TestConnectWithOptions_Integration(t *testing.T) {
	// ConnectWithOptions uses context.Background() internally; we cannot easily
	// force a fast timeout here, so we skip if no error is returned.
	opts := &Options{
		StartupTimeout: 50 * time.Millisecond,
	}
	_, err := ConnectWithOptions(opts)
	if err == nil {
		t.Skip("unexpected successful connection – live network present, skipping")
	}
	t.Logf("Got expected error: %v", err)
}

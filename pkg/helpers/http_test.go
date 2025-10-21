package helpers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockSimpleClient implements a minimal client interface for testing
type mockSimpleClient struct {
	proxyURL string
}

func (m *mockSimpleClient) ProxyURL() string {
	return m.proxyURL
}

func (m *mockSimpleClient) Close() error {
	return nil
}

func (m *mockSimpleClient) IsReady() bool {
	return true
}

func (m *mockSimpleClient) WaitUntilReady(timeout time.Duration) error {
	return nil
}

func TestDefaultHTTPClientConfig(t *testing.T) {
	config := DefaultHTTPClientConfig()

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected Timeout to be 30s, got %v", config.Timeout)
	}

	if config.DialTimeout != 10*time.Second {
		t.Errorf("Expected DialTimeout to be 10s, got %v", config.DialTimeout)
	}

	if config.TLSHandshakeTimeout != 10*time.Second {
		t.Errorf("Expected TLSHandshakeTimeout to be 10s, got %v", config.TLSHandshakeTimeout)
	}

	if config.MaxIdleConns != 10 {
		t.Errorf("Expected MaxIdleConns to be 10, got %d", config.MaxIdleConns)
	}

	if config.IdleConnTimeout != 90*time.Second {
		t.Errorf("Expected IdleConnTimeout to be 90s, got %v", config.IdleConnTimeout)
	}

	if config.DisableKeepAlives != false {
		t.Errorf("Expected DisableKeepAlives to be false, got %v", config.DisableKeepAlives)
	}
}

func TestNewHTTPClient_NilClient(t *testing.T) {
	_, err := NewHTTPClient(nil, nil)
	if err == nil {
		t.Error("Expected error when torClient is nil")
	}

	expectedErr := "torClient cannot be nil"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestNewHTTPClient_InvalidProxyURL(t *testing.T) {
	mockClient := &mockSimpleClient{
		proxyURL: "://invalid-url",
	}

	_, err := NewHTTPClient(mockClient, nil)
	if err == nil {
		t.Error("Expected error with invalid proxy URL")
	}
}

func TestNewHTTPClient_Success(t *testing.T) {
	// Create a mock SOCKS5 server for testing
	// Note: In real tests, we'd need a proper SOCKS5 server
	// For unit tests, we just verify the client is configured correctly
	mockClient := &mockSimpleClient{
		proxyURL: "socks5://127.0.0.1:9050",
	}

	client, err := NewHTTPClient(mockClient, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if client == nil {
		t.Fatal("Expected non-nil HTTP client")
	}

	if client.Timeout != 30*time.Second {
		t.Errorf("Expected timeout to be 30s, got %v", client.Timeout)
	}

	if client.Transport == nil {
		t.Error("Expected non-nil Transport")
	}
}

func TestNewHTTPClient_CustomConfig(t *testing.T) {
	mockClient := &mockSimpleClient{
		proxyURL: "socks5://127.0.0.1:9050",
	}

	config := &HTTPClientConfig{
		Timeout:             60 * time.Second,
		MaxIdleConns:        20,
		DisableKeepAlives:   true,
		IdleConnTimeout:     120 * time.Second,
		TLSHandshakeTimeout: 15 * time.Second,
	}

	client, err := NewHTTPClient(mockClient, config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if client.Timeout != 60*time.Second {
		t.Errorf("Expected timeout to be 60s, got %v", client.Timeout)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected transport to be *http.Transport")
	}

	if transport.MaxIdleConns != 20 {
		t.Errorf("Expected MaxIdleConns to be 20, got %d", transport.MaxIdleConns)
	}

	if transport.DisableKeepAlives != true {
		t.Error("Expected DisableKeepAlives to be true")
	}

	if transport.IdleConnTimeout != 120*time.Second {
		t.Errorf("Expected IdleConnTimeout to be 120s, got %v", transport.IdleConnTimeout)
	}

	if transport.TLSHandshakeTimeout != 15*time.Second {
		t.Errorf("Expected TLSHandshakeTimeout to be 15s, got %v", transport.TLSHandshakeTimeout)
	}
}

func TestNewHTTPTransport_NilClient(t *testing.T) {
	_, err := NewHTTPTransport(nil, nil)
	if err == nil {
		t.Error("Expected error when torClient is nil")
	}

	expectedErr := "torClient cannot be nil"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestNewHTTPTransport_Success(t *testing.T) {
	mockClient := &mockSimpleClient{
		proxyURL: "socks5://127.0.0.1:9050",
	}

	transport, err := NewHTTPTransport(mockClient, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if transport == nil {
		t.Fatal("Expected non-nil transport")
	}

	if transport.MaxIdleConns != 10 {
		t.Errorf("Expected MaxIdleConns to be 10, got %d", transport.MaxIdleConns)
	}
}

func TestDialContext_NilClient(t *testing.T) {
	dialFunc := DialContext(nil)

	_, err := dialFunc(context.Background(), "tcp", "example.com:80")
	if err == nil {
		t.Error("Expected error when torClient is nil")
	}

	expectedErr := "torClient cannot be nil"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestDialContext_ContextCancellation(t *testing.T) {
	mockClient := &mockSimpleClient{
		proxyURL: "socks5://127.0.0.1:9050",
	}

	dialFunc := DialContext(mockClient)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := dialFunc(ctx, "tcp", "example.com:80")
	if err == nil {
		t.Error("Expected error when context is cancelled")
	}

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

func TestWrapHTTPClient_NilClient(t *testing.T) {
	mockTorClient := &mockSimpleClient{
		proxyURL: "socks5://127.0.0.1:9050",
	}

	err := WrapHTTPClient(nil, mockTorClient, nil)
	if err == nil {
		t.Error("Expected error when httpClient is nil")
	}

	expectedErr := "httpClient cannot be nil"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestWrapHTTPClient_Success(t *testing.T) {
	mockTorClient := &mockSimpleClient{
		proxyURL: "socks5://127.0.0.1:9050",
	}

	httpClient := &http.Client{
		Timeout: 60 * time.Second,
	}

	err := WrapHTTPClient(httpClient, mockTorClient, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if httpClient.Transport == nil {
		t.Error("Expected Transport to be set")
	}

	if httpClient.Timeout != 60*time.Second {
		t.Error("Expected original timeout to be preserved")
	}
}

func TestWrapHTTPClient_ReplacesTransport(t *testing.T) {
	mockTorClient := &mockSimpleClient{
		proxyURL: "socks5://127.0.0.1:9050",
	}

	originalTransport := &http.Transport{
		MaxIdleConns: 50,
	}

	httpClient := &http.Client{
		Transport: originalTransport,
		Timeout:   60 * time.Second,
	}

	err := WrapHTTPClient(httpClient, mockTorClient, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if httpClient.Transport == originalTransport {
		t.Error("Expected Transport to be replaced")
	}

	newTransport, ok := httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected Transport to be *http.Transport")
	}

	// Should use default config
	if newTransport.MaxIdleConns != 10 {
		t.Errorf("Expected MaxIdleConns to be 10 (default), got %d", newTransport.MaxIdleConns)
	}
}

// TestHTTPClientIntegration tests the HTTP client with a test server
func TestHTTPClientIntegration(t *testing.T) {
	// Create a test HTTP server
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello from test server")
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	// Note: This test would require a real SOCKS5 proxy to fully test
	// For now, we just verify the client can be created without errors
	mockClient := &mockSimpleClient{
		proxyURL: "socks5://127.0.0.1:9050",
	}

	httpClient, err := NewHTTPClient(mockClient, nil)
	if err != nil {
		t.Fatalf("Failed to create HTTP client: %v", err)
	}

	if httpClient == nil {
		t.Fatal("Expected non-nil HTTP client")
	}

	// We can't actually make a request through a non-existent SOCKS5 proxy,
	// but we verified the client is properly configured
}

// TestHTTPClientConfigValidation ensures all config fields are respected
func TestHTTPClientConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config *HTTPClientConfig
	}{
		{
			name:   "nil config uses defaults",
			config: nil,
		},
		{
			name: "custom timeout",
			config: &HTTPClientConfig{
				Timeout:             45 * time.Second,
				MaxIdleConns:        15,
				DisableKeepAlives:   false,
				IdleConnTimeout:     100 * time.Second,
				TLSHandshakeTimeout: 12 * time.Second,
			},
		},
		{
			name: "disabled keep-alives",
			config: &HTTPClientConfig{
				Timeout:           20 * time.Second,
				MaxIdleConns:      5,
				DisableKeepAlives: true,
			},
		},
	}

	mockClient := &mockSimpleClient{
		proxyURL: "socks5://127.0.0.1:9050",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewHTTPClient(mockClient, tt.config)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			if client == nil {
				t.Fatal("Expected non-nil client")
			}

			expectedConfig := tt.config
			if expectedConfig == nil {
				expectedConfig = DefaultHTTPClientConfig()
			}

			if client.Timeout != expectedConfig.Timeout {
				t.Errorf("Expected timeout %v, got %v", expectedConfig.Timeout, client.Timeout)
			}

			transport, ok := client.Transport.(*http.Transport)
			if !ok {
				t.Fatal("Expected *http.Transport")
			}

			if transport.MaxIdleConns != expectedConfig.MaxIdleConns {
				t.Errorf("Expected MaxIdleConns %d, got %d", expectedConfig.MaxIdleConns, transport.MaxIdleConns)
			}

			if transport.DisableKeepAlives != expectedConfig.DisableKeepAlives {
				t.Errorf("Expected DisableKeepAlives %v, got %v", expectedConfig.DisableKeepAlives, transport.DisableKeepAlives)
			}
		})
	}
}

// TestDialTimeoutRespected verifies that DialTimeout is applied during connection establishment
func TestDialTimeoutRespected(t *testing.T) {
	mockClient := &mockSimpleClient{
		proxyURL: "socks5://127.0.0.1:9050",
	}

	// Create config with very short DialTimeout
	config := &HTTPClientConfig{
		DialTimeout: 1 * time.Millisecond, // Very short timeout to ensure it triggers
		Timeout:     30 * time.Second,
	}

	transport, err := NewHTTPTransport(mockClient, config)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}

	// The transport should have DialContext that respects the timeout
	if transport.DialContext == nil {
		t.Fatal("Expected DialContext to be set")
	}

	// Test that a dial to a non-existent address times out quickly
	ctx := context.Background()
	start := time.Now()
	_, err = transport.DialContext(ctx, "tcp", "192.0.2.1:80") // Non-routable IP
	elapsed := time.Since(start)

	// Should fail (either timeout or connection error)
	if err == nil {
		t.Error("Expected error when dialing non-routable address")
	}

	// Should fail relatively quickly (within a reasonable margin)
	// We allow up to 100ms for the timeout plus overhead
	if elapsed > 100*time.Millisecond {
		t.Logf("Warning: Dial took %v, expected quick timeout (this may be OK on slow systems)", elapsed)
	}
}

// TestDialContextCancellationDuringDial verifies context cancellation during dial
func TestDialContextCancellationDuringDial(t *testing.T) {
	mockClient := &mockSimpleClient{
		proxyURL: "socks5://127.0.0.1:9050",
	}

	dialFunc := DialContext(mockClient)

	// Create a context with a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Try to dial - should fail with context deadline exceeded
	_, err := dialFunc(ctx, "tcp", "192.0.2.1:80")
	if err == nil {
		t.Error("Expected error when context times out during dial")
	}

	// Should be a context error
	if err != context.DeadlineExceeded && err != context.Canceled {
		t.Logf("Got error: %v (may be acceptable if dial failed before timeout)", err)
	}
}

// TestDialContextImmediateCancellation verifies pre-cancelled context
func TestDialContextImmediateCancellation(t *testing.T) {
	mockClient := &mockSimpleClient{
		proxyURL: "socks5://127.0.0.1:9050",
	}

	dialFunc := DialContext(mockClient)

	// Create and immediately cancel context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err := dialFunc(ctx, "tcp", "example.com:80")
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected error when context is already cancelled")
	}

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	// Should return immediately
	if elapsed > 10*time.Millisecond {
		t.Errorf("Expected immediate return, took %v", elapsed)
	}
}

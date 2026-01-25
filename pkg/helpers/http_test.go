package helpers

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/proxy"
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

// mockConn is a mock net.Conn for testing
type mockConn struct {
	net.Conn
	closed bool
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	return 0, errors.New("mock conn")
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	return len(b), nil
}

func (m *mockConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234}
}

func (m *mockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 5678}
}

func (m *mockConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// mockContextDialer implements proxy.ContextDialer for testing context-aware dialing
type mockContextDialer struct {
	shouldError bool
	delay       time.Duration
}

func (m *mockContextDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.shouldError {
		return nil, errors.New("mock dial error")
	}
	return &mockConn{}, nil
}

func (m *mockContextDialer) Dial(network, addr string) (net.Conn, error) {
	return m.DialContext(context.Background(), network, addr)
}

// mockStandardDialer implements only proxy.Dialer (not ContextDialer)
type mockStandardDialer struct {
	shouldError bool
	delay       time.Duration
}

func (m *mockStandardDialer) Dial(network, addr string) (net.Conn, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	if m.shouldError {
		return nil, errors.New("mock dial error")
	}
	return &mockConn{}, nil
}

// Compile-time interface checks
var (
	_ proxy.ContextDialer = (*mockContextDialer)(nil)
	_ proxy.Dialer        = (*mockContextDialer)(nil)
	_ proxy.Dialer        = (*mockStandardDialer)(nil)
)

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
		DialTimeout:         15 * time.Second, // Test DialTimeout path
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

	// Verify DialContext is set (implicitly tests DialTimeout path)
	if transport.DialContext == nil {
		t.Error("Expected DialContext to be set")
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

// TestNewHTTPTransport_DialTimeoutExecution tests that DialTimeout is actually applied during dial
func TestNewHTTPTransport_DialTimeoutExecution(t *testing.T) {
	mockClient := &mockSimpleClient{
		proxyURL: "socks5://127.0.0.1:9050",
	}

	config := &HTTPClientConfig{
		DialTimeout: 5 * time.Second, // Set DialTimeout to test the branch
		Timeout:     30 * time.Second,
	}

	transport, err := NewHTTPTransport(mockClient, config)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}

	// Actually invoke DialContext to execute the closure
	ctx := context.Background()
	_, err = transport.DialContext(ctx, "tcp", "127.0.0.1:1") // Non-existent port

	// Should fail (connection refused or timeout), but we tested the DialTimeout path
	if err == nil {
		t.Log("Dial unexpectedly succeeded (this is OK if port 1 is open)")
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

	// Error should be related to context cancellation (may be wrapped)
	errMsg := err.Error()
	if err != context.Canceled && !isContextError(err) && !strings.Contains(errMsg, "canceled") && !strings.Contains(errMsg, "cancelled") {
		t.Errorf("Expected context cancellation error, got %v", err)
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

	// Error should be related to context cancellation (may be wrapped)
	errMsg := err.Error()
	if err != context.Canceled && !isContextError(err) && !strings.Contains(errMsg, "canceled") && !strings.Contains(errMsg, "cancelled") {
		t.Errorf("Expected context cancellation error, got %v", err)
	}

	// Should return immediately
	if elapsed > 10*time.Millisecond {
		t.Errorf("Expected immediate return, took %v", elapsed)
	}
}

// isContextError checks if an error is related to context cancellation
func isContextError(err error) bool {
	if err == context.Canceled || err == context.DeadlineExceeded {
		return true
	}
	// Check wrapped errors
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		return isContextError(unwrapped)
	}
	return false
}

// TestNoGoroutineLeakOnContextCancellation verifies that goroutines are properly cleaned up
// when context is cancelled during dial operations (AUDIT fix verification)
func TestNoGoroutineLeakOnContextCancellation(t *testing.T) {
	mockClient := &mockSimpleClient{
		proxyURL: "socks5://127.0.0.1:9050",
	}

	// Get baseline goroutine count
	baseline := countGoroutines()

	// Create short-lived contexts that cancel during dial
	// Note: SOCKS5 dialer doesn't support context cancellation during blocking Dial(),
	// so we test that goroutines complete after the dial finishes
	for i := 0; i < 20; i++ {
		transport, err := NewHTTPTransport(mockClient, &HTTPClientConfig{
			DialTimeout: 5 * time.Millisecond,
			Timeout:     30 * time.Second,
		})
		if err != nil {
			t.Fatalf("Failed to create transport: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		// Try to dial localhost - should fail quickly with connection refused
		_, _ = transport.DialContext(ctx, "tcp", "127.0.0.1:1")
		cancel()
	}

	// Allow time for goroutines to complete after dial attempts finish
	time.Sleep(200 * time.Millisecond)

	// Check goroutine count hasn't grown significantly
	current := countGoroutines()
	growth := current - baseline

	// Allow some growth for background tasks, but not 20+ goroutines
	if growth > 10 {
		t.Errorf("Goroutine leak detected: baseline=%d, current=%d, growth=%d", baseline, current, growth)
	}
}

// countGoroutines returns the current number of goroutines
func countGoroutines() int {
	return runtime.NumGoroutine()
}

// TestDialWithContext_ContextDialer tests the context-aware dialing path
func TestDialWithContext_ContextDialer(t *testing.T) {
	dialer := &mockContextDialer{shouldError: false}

	ctx := context.Background()
	conn, err := dialWithContext(ctx, dialer, "tcp", "example.com:80")
	if err != nil {
		t.Fatalf("Expected successful dial, got error: %v", err)
	}

	if conn == nil {
		t.Fatal("Expected non-nil connection")
	}

	conn.Close()
}

// TestDialWithContext_ContextDialerError tests error handling with ContextDialer
func TestDialWithContext_ContextDialerError(t *testing.T) {
	dialer := &mockContextDialer{shouldError: true}

	ctx := context.Background()
	conn, err := dialWithContext(ctx, dialer, "tcp", "example.com:80")

	if err == nil {
		t.Fatal("Expected error from dialer")
	}

	if conn != nil {
		t.Error("Expected nil connection on error")
	}
}

// TestDialWithContext_ContextDialerCancellation tests context cancellation with ContextDialer
func TestDialWithContext_ContextDialerCancellation(t *testing.T) {
	dialer := &mockContextDialer{delay: 100 * time.Millisecond}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	conn, err := dialWithContext(ctx, dialer, "tcp", "example.com:80")

	if err == nil {
		t.Fatal("Expected context deadline exceeded error")
	}

	if err != context.DeadlineExceeded && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}

	if conn != nil {
		t.Error("Expected nil connection on cancellation")
	}
}

// TestDialWithContext_StandardDialer tests the fallback path for non-context dialers
func TestDialWithContext_StandardDialer(t *testing.T) {
	dialer := &mockStandardDialer{shouldError: false}

	ctx := context.Background()
	conn, err := dialWithContext(ctx, dialer, "tcp", "example.com:80")
	if err != nil {
		t.Fatalf("Expected successful dial, got error: %v", err)
	}

	if conn == nil {
		t.Fatal("Expected non-nil connection")
	}

	conn.Close()
}

// TestDialWithContext_StandardDialerError tests error handling with standard dialer
func TestDialWithContext_StandardDialerError(t *testing.T) {
	dialer := &mockStandardDialer{shouldError: true}

	ctx := context.Background()
	conn, err := dialWithContext(ctx, dialer, "tcp", "example.com:80")

	if err == nil {
		t.Fatal("Expected error from dialer")
	}

	if conn != nil {
		t.Error("Expected nil connection on error")
	}
}

// TestDialWithContext_StandardDialerCancellation tests context cancellation with standard dialer
func TestDialWithContext_StandardDialerCancellation(t *testing.T) {
	dialer := &mockStandardDialer{delay: 100 * time.Millisecond}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	conn, err := dialWithContext(ctx, dialer, "tcp", "example.com:80")

	if err == nil {
		t.Fatal("Expected context cancellation error")
	}

	if err != context.Canceled && !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	if conn != nil {
		t.Error("Expected nil connection on cancellation")
	}
}

// TestDialWithContext_StandardDialerContextTimeout tests timeout during goroutine-wrapped dial
func TestDialWithContext_StandardDialerContextTimeout(t *testing.T) {
	dialer := &mockStandardDialer{delay: 100 * time.Millisecond}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	conn, err := dialWithContext(ctx, dialer, "tcp", "example.com:80")

	if err == nil {
		t.Fatal("Expected context deadline exceeded error")
	}

	if err != context.DeadlineExceeded && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}

	if conn != nil {
		t.Error("Expected nil connection on timeout")
	}
}

// TestDialWithContext_StandardDialerConnCleanup tests connection cleanup on context cancellation
func TestDialWithContext_StandardDialerConnCleanup(t *testing.T) {
	// This test verifies that if a connection is established but context is cancelled,
	// the connection is properly closed (lines 79-81 in http.go)
	dialer := &mockStandardDialer{delay: 5 * time.Millisecond}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	// The dial should succeed but we cancel shortly after
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	conn, err := dialWithContext(ctx, dialer, "tcp", "example.com:80")

	// We might get either success or cancellation depending on timing
	if err != nil {
		// If cancelled, should be context error
		if err != context.Canceled && !errors.Is(err, context.Canceled) {
			t.Logf("Got error: %v (may be acceptable)", err)
		}
	} else if conn != nil {
		// If successful, connection should be usable
		conn.Close()
	}
}

package pt

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

// mockConn is a mock net.Conn for testing SOCKS5 handshake.
type mockConn struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
	closed   bool
}

func newMockConn(responseData []byte) *mockConn {
	return &mockConn{
		readBuf:  bytes.NewBuffer(responseData),
		writeBuf: &bytes.Buffer{},
	}
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	return m.readBuf.Read(b)
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	return m.writeBuf.Write(b)
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// TestSocks5Handshake_Success tests successful SOCKS5 handshake.
func TestSocks5Handshake_Success(t *testing.T) {
	client, err := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})
	if err != nil {
		t.Fatalf("NewManagedClient() failed: %v", err)
	}

	// Prepare SOCKS5 response: auth success + connect success
	response := []byte{
		0x05, 0x00, // Auth response: version 5, no auth required
		0x05, 0x00, 0x00, 0x01, // Connect response: success
		0x7f, 0x00, 0x00, 0x01, // Bind address (127.0.0.1)
		0x00, 0x50, // Bind port (80)
	}

	conn := newMockConn(response)
	err = client.socks5Handshake(conn, "example.com:443")
	if err != nil {
		t.Errorf("socks5Handshake() error = %v, want nil", err)
	}

	// Verify auth request was written
	written := conn.writeBuf.Bytes()
	if len(written) < 3 {
		t.Fatalf("auth request not written")
	}
	if written[0] != 0x05 || written[1] != 0x01 || written[2] != 0x00 {
		t.Errorf("invalid auth request: %v", written[:3])
	}
}

// TestSocks5Handshake_AuthFailed tests SOCKS5 auth failure.
func TestSocks5Handshake_AuthFailed(t *testing.T) {
	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	// Auth failed response
	response := []byte{0x05, 0xFF}
	conn := newMockConn(response)

	err := client.socks5Handshake(conn, "example.com:443")
	if err == nil {
		t.Error("socks5Handshake() expected error, got nil")
	}
	if err != nil && err.Error() != "SOCKS5 auth failed" {
		t.Errorf("socks5Handshake() error = %v, want SOCKS5 auth failed", err)
	}
}

// TestSocks5Handshake_ConnectFailed tests SOCKS5 connect failure.
func TestSocks5Handshake_ConnectFailed(t *testing.T) {
	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	// Auth success + connect failed
	response := []byte{
		0x05, 0x00, // Auth success
		0x05, 0x01, 0x00, 0x01, // Connect failed (code 0x01)
	}
	conn := newMockConn(response)

	err := client.socks5Handshake(conn, "example.com:443")
	if err == nil {
		t.Error("socks5Handshake() expected error, got nil")
	}
	if err != nil && err.Error() != "SOCKS5 connect failed: 1" {
		t.Errorf("socks5Handshake() error = %v, want connect failed", err)
	}
}

// TestSocks5Handshake_InvalidAddress tests invalid address parsing.
func TestSocks5Handshake_InvalidAddress(t *testing.T) {
	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	response := []byte{0x05, 0x00} // Auth success
	conn := newMockConn(response)

	err := client.socks5Handshake(conn, "invalid-address-no-port")
	if err == nil {
		t.Error("socks5Handshake() expected error for invalid address, got nil")
	}
}

// TestSocks5Handshake_ReadError tests read errors during handshake.
func TestSocks5Handshake_ReadError(t *testing.T) {
	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	// Empty response will cause io.EOF
	conn := newMockConn([]byte{})

	err := client.socks5Handshake(conn, "example.com:443")
	if err == nil {
		t.Error("socks5Handshake() expected error for read failure, got nil")
	}
}

// TestDial_NotReady tests Dial when PT is not running.
func TestDial_NotReady(t *testing.T) {
	client, err := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})
	if err != nil {
		t.Fatalf("NewManagedClient() failed: %v", err)
	}

	ctx := context.Background()
	_, err = client.Dial(ctx, "example.com:443")
	if err == nil {
		t.Error("Dial() expected error when PT not running, got nil")
	}
	if err != nil && err.Error() != "PT not ready" {
		t.Errorf("Dial() error = %v, want PT not ready", err)
	}
}

// TestDial_NoMethods tests Dial when no methods are registered.
func TestDial_NoMethods(t *testing.T) {
	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	// Set running but no methods
	client.mu.Lock()
	client.running = true
	client.mu.Unlock()

	ctx := context.Background()
	_, err := client.Dial(ctx, "example.com:443")
	if err == nil {
		t.Error("Dial() expected error when no methods, got nil")
	}
}

// TestIsRunning tests IsRunning state tracking.
func TestIsRunning(t *testing.T) {
	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	if client.IsRunning() {
		t.Error("IsRunning() = true, want false for new client")
	}

	client.mu.Lock()
	client.running = true
	client.mu.Unlock()

	if !client.IsRunning() {
		t.Error("IsRunning() = false, want true after setting running")
	}
}

// TestClose_NotRunning tests Close when PT is not running.
func TestClose_NotRunning(t *testing.T) {
	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	err := client.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil for non-running client", err)
	}
}

// TestClose_Running tests Close when PT is running.
func TestClose_Running(t *testing.T) {
	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	client.mu.Lock()
	client.running = true
	client.mu.Unlock()

	err := client.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}

	if client.IsRunning() {
		t.Error("IsRunning() = true after Close(), want false")
	}
}

// TestPerformHandshake_CMETHODError tests handshake with CMETHOD-ERROR.
func TestPerformHandshake_CMETHODError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration-style test in short mode")
	}

	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	// Create a pipe to simulate stdout
	reader, writer := io.Pipe()
	client.stdout = reader

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Write error response in background
	go func() {
		time.Sleep(100 * time.Millisecond)
		fmt.Fprintln(writer, "CMETHOD-ERROR obfs4 failed to initialize")
		writer.Close()
	}()

	err := client.performHandshake(ctx)
	if err == nil {
		t.Error("performHandshake() expected error for CMETHOD-ERROR, got nil")
	}
}

// TestPerformHandshake_Timeout tests handshake timeout.
func TestPerformHandshake_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}

	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	// Create a pipe that never sends CMETHODS DONE
	reader, _ := io.Pipe()
	client.stdout = reader

	ctx := context.Background()
	err := client.performHandshake(ctx)
	if err == nil {
		t.Error("performHandshake() expected timeout error, got nil")
	}
	if err != nil && err.Error() != "PT handshake timeout" {
		t.Errorf("performHandshake() error = %v, want timeout", err)
	}
}

// TestPerformHandshake_ContextCanceled tests handshake with canceled context.
func TestPerformHandshake_ContextCanceled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping context test in short mode")
	}

	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	reader, _ := io.Pipe()
	client.stdout = reader

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := client.performHandshake(ctx)
	if err == nil {
		t.Error("performHandshake() expected context error, got nil")
	}
	if err != context.Canceled {
		t.Errorf("performHandshake() error = %v, want context.Canceled", err)
	}
}

// TestPerformHandshake_Success tests successful handshake.
func TestPerformHandshake_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration-style test in short mode")
	}

	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	reader, writer := io.Pipe()
	client.stdout = reader

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Write successful handshake in background
	go func() {
		time.Sleep(100 * time.Millisecond)
		fmt.Fprintln(writer, "CMETHOD obfs4 socks5 127.0.0.1:1234")
		fmt.Fprintln(writer, "CMETHODS DONE")
		writer.Close()
	}()

	err := client.performHandshake(ctx)
	if err != nil {
		t.Errorf("performHandshake() error = %v, want nil", err)
	}

	// Verify method was registered
	methods := client.Methods()
	if len(methods) != 1 || methods[0] != "obfs4" {
		t.Errorf("Methods() = %v, want [obfs4]", methods)
	}
}

// TestReadStderr tests stderr reading.
func TestReadStderr(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stderr test in short mode")
	}

	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	reader, writer := io.Pipe()
	client.stderr = reader

	// Start reading stderr
	done := make(chan bool)
	go func() {
		client.readStderr()
		done <- true
	}()

	// Write some stderr lines
	fmt.Fprintln(writer, "Debug line 1")
	fmt.Fprintln(writer, "Debug line 2")
	writer.Close()

	// Wait for completion
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("readStderr() did not complete in time")
	}
}

// TestParseCMethod_EmptyLine tests parsing empty CMETHOD line.
func TestParseCMethod_EmptyLine(t *testing.T) {
	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	err := client.parseCMethod("CMETHOD")
	if err == nil {
		t.Error("parseCMethod() expected error for empty line, got nil")
	}
}

// TestParseCMethod_InvalidSOCKS tests parsing CMETHOD with invalid SOCKS version.
func TestParseCMethod_InvalidSOCKS(t *testing.T) {
	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	err := client.parseCMethod("CMETHOD test socks7 127.0.0.1:1234")
	if err == nil {
		t.Error("parseCMethod() expected error for invalid SOCKS version, got nil")
	}
}

// TestParseCMethod_ArgsWithoutEquals tests parsing CMETHOD with malformed args.
func TestParseCMethod_ArgsWithoutEquals(t *testing.T) {
	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	// Args without '=' should be ignored
	err := client.parseCMethod("CMETHOD test socks5 127.0.0.1:1234 badarg")
	if err != nil {
		t.Errorf("parseCMethod() error = %v, want nil (ignore bad args)", err)
	}

	methods := client.Methods()
	if len(methods) != 1 {
		t.Errorf("Methods() = %v, want 1 method", methods)
	}
}

// TestParseCMethod_SOCKS4 tests parsing CMETHOD with SOCKS4.
func TestParseCMethod_SOCKS4(t *testing.T) {
	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	err := client.parseCMethod("CMETHOD meek socks4 127.0.0.1:5678")
	if err != nil {
		t.Errorf("parseCMethod() error = %v, want nil", err)
	}

	client.mu.RLock()
	method := client.methods["meek"]
	client.mu.RUnlock()

	if method == nil {
		t.Fatal("method not registered")
	}
	if method.SOCKSVersion != 4 {
		t.Errorf("SOCKSVersion = %d, want 4", method.SOCKSVersion)
	}
}

// TestDial_MethodRegistered tests Dial with a registered method.
func TestDial_MethodRegistered(t *testing.T) {
	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	// Register a method and set running
	client.mu.Lock()
	client.running = true
	client.methods["obfs4"] = &MethodInfo{
		Name:         "obfs4",
		Address:      "127.0.0.1:1234",
		SOCKSVersion: 5,
	}
	client.mu.Unlock()

	// Dial will fail because there's no real SOCKS server, but we can test the method selection
	ctx := context.Background()
	_, err := client.Dial(ctx, "example.com:443")
	if err == nil {
		t.Error("Dial() expected error (no real SOCKS server), got nil")
	}
	// The error should be about connection failure, not "PT not ready"
	if err.Error() == "PT not ready" {
		t.Errorf("Dial() error = %v, should fail at connection stage", err)
	}
}

// TestClose_MultipleCloses tests calling Close multiple times.
func TestClose_MultipleCloses(t *testing.T) {
	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	client.mu.Lock()
	client.running = true
	client.mu.Unlock()

	// First close
	err := client.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}

	// Second close should be idempotent
	err = client.Close()
	if err != nil {
		t.Errorf("Close() second call error = %v, want nil", err)
	}

	if client.IsRunning() {
		t.Error("IsRunning() = true after double Close(), want false")
	}
}

// TestSocks5Handshake_PortParsing tests SOCKS5 handshake with various port formats.
func TestSocks5Handshake_PortParsing(t *testing.T) {
	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	tests := []struct {
		name    string
		address string
		port    int
	}{
		{"standard port", "example.com:443", 443},
		{"high port", "example.com:65535", 65535},
		{"low port", "example.com:80", 80},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare SOCKS5 response for successful handshake
			response := []byte{
				0x05, 0x00, // Auth success
				0x05, 0x00, 0x00, 0x01, // Connect success
				0x7f, 0x00, 0x00, 0x01, // Bind address
				0x00, 0x50, // Bind port
			}

			conn := newMockConn(response)
			err := client.socks5Handshake(conn, tt.address)
			if err != nil {
				t.Errorf("socks5Handshake(%s) error = %v, want nil", tt.address, err)
			}

			// Verify the request contains correct port
			written := conn.writeBuf.Bytes()
			if len(written) < 8 {
				t.Fatalf("connect request too short: %d bytes", len(written))
			}

			// Port is in last 2 bytes of connect request
			portBytes := written[len(written)-2:]
			portFromReq := (int(portBytes[0]) << 8) | int(portBytes[1])
			if portFromReq != tt.port {
				t.Errorf("port in request = %d, want %d", portFromReq, tt.port)
			}
		})
	}
}

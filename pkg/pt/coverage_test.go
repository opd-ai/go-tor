package pt

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

// TestServer_Close_WithListeners tests server Close with active listeners.
func TestServer_Close_WithListeners(t *testing.T) {
	server, _ := NewManagedServer(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	// Set running state and add mock listeners
	server.mu.Lock()
	server.running = true
	server.listeners["obfs4"] = &mockListener{}
	server.mu.Unlock()

	err := server.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}

	// Verify listeners were cleared
	if len(server.listeners) != 0 {
		t.Errorf("listeners not cleared after Close(), got %d", len(server.listeners))
	}
}

// TestServer_Close_MultipleCloses tests calling Close multiple times.
func TestServer_Close_MultipleCloses(t *testing.T) {
	server, _ := NewManagedServer(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	server.mu.Lock()
	server.running = true
	server.mu.Unlock()

	// First close
	err := server.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}

	// Second close should be idempotent
	err = server.Close()
	if err != nil {
		t.Errorf("Close() second call error = %v, want nil", err)
	}
}

// TestServer_Dial_NotSupported tests that Dial is not supported for servers.
func TestServer_Dial_NotSupported(t *testing.T) {
	server, _ := NewManagedServer(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	ctx := context.Background()
	_, err := server.Dial(ctx, "example.com:443")
	if err == nil {
		t.Error("Dial() expected error, got nil")
	}
	if err.Error() != "Dial not supported for server transports" {
		t.Errorf("Dial() error = %v, want 'Dial not supported'", err)
	}
}

// TestServer_Listen_NotReady tests Listen when server is not ready.
func TestServer_Listen_NotReady(t *testing.T) {
	server, _ := NewManagedServer(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	ctx := context.Background()
	_, err := server.Listen(ctx, "127.0.0.1:0")
	if err == nil {
		t.Error("Listen() expected error when not running, got nil")
	}
	if err.Error() != "PT server not ready" {
		t.Errorf("Listen() error = %v, want 'PT server not ready'", err)
	}
}

// TestServer_Listen_Success tests successful Listen.
func TestServer_Listen_Success(t *testing.T) {
	server, _ := NewManagedServer(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	// Set running and add a method
	server.mu.Lock()
	server.running = true
	server.methods["obfs4"] = &ServerMethodInfo{
		Name:    "obfs4",
		Address: "127.0.0.1:1234",
	}
	server.mu.Unlock()

	ctx := context.Background()
	listener, err := server.Listen(ctx, "127.0.0.1:0")
	if err != nil {
		t.Errorf("Listen() error = %v, want nil", err)
	}
	if listener == nil {
		t.Error("Listen() returned nil listener")
	}

	// Verify listener was registered
	server.mu.RLock()
	_, exists := server.listeners["obfs4"]
	server.mu.RUnlock()
	if !exists {
		t.Error("Listener not registered in server.listeners")
	}
}

// TestServer_PerformHandshake_SMETHODError tests handshake with SMETHOD-ERROR.
func TestServer_PerformHandshake_SMETHODError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration-style test in short mode")
	}

	server, _ := NewManagedServer(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	reader, writer := io.Pipe()
	server.stdout = reader

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Write error response in background
	go func() {
		time.Sleep(100 * time.Millisecond)
		fmt.Fprintln(writer, "SMETHOD-ERROR obfs4 failed to bind")
		writer.Close()
	}()

	err := server.performHandshake(ctx)
	if err == nil {
		t.Error("performHandshake() expected error for SMETHOD-ERROR, got nil")
	}
}

// TestServer_PerformHandshake_Timeout tests handshake timeout.
func TestServer_PerformHandshake_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}

	server, _ := NewManagedServer(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	reader, _ := io.Pipe()
	server.stdout = reader

	ctx := context.Background()
	err := server.performHandshake(ctx)
	if err == nil {
		t.Error("performHandshake() expected timeout error, got nil")
	}
	if err.Error() != "PT server handshake timeout" {
		t.Errorf("performHandshake() error = %v, want timeout", err)
	}
}

// TestServer_PerformHandshake_ContextCanceled tests handshake with canceled context.
func TestServer_PerformHandshake_ContextCanceled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping context test in short mode")
	}

	server, _ := NewManagedServer(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	reader, _ := io.Pipe()
	server.stdout = reader

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := server.performHandshake(ctx)
	if err == nil {
		t.Error("performHandshake() expected context error, got nil")
	}
	if err != context.Canceled {
		t.Errorf("performHandshake() error = %v, want context.Canceled", err)
	}
}

// TestServer_PerformHandshake_Success tests successful handshake.
func TestServer_PerformHandshake_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration-style test in short mode")
	}

	server, _ := NewManagedServer(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	reader, writer := io.Pipe()
	server.stdout = reader

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Write successful handshake in background
	go func() {
		time.Sleep(100 * time.Millisecond)
		fmt.Fprintln(writer, "SMETHOD obfs4 127.0.0.1:1234")
		fmt.Fprintln(writer, "SMETHODS DONE")
		writer.Close()
	}()

	err := server.performHandshake(ctx)
	if err != nil {
		t.Errorf("performHandshake() error = %v, want nil", err)
	}

	// Verify method was registered
	methods := server.Methods()
	if len(methods) != 1 || methods[0] != "obfs4" {
		t.Errorf("Methods() = %v, want [obfs4]", methods)
	}
}

// TestServer_ReadStderr tests stderr reading.
func TestServer_ReadStderr(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stderr test in short mode")
	}

	server, _ := NewManagedServer(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	reader, writer := io.Pipe()
	server.stderr = reader

	// Start reading stderr
	done := make(chan bool)
	go func() {
		server.readStderr()
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

// mockListener implements net.Listener for testing.
type mockListener struct {
	closed bool
}

func (m *mockListener) Accept() (net.Conn, error) {
	if m.closed {
		return nil, fmt.Errorf("listener closed")
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockListener) Close() error {
	m.closed = true
	return nil
}

func (m *mockListener) Addr() net.Addr {
	return nil
}

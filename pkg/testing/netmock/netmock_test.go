package netmock_test

import (
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/testing/netmock"
)

// --- Addr tests ---

func TestAddr_Network(t *testing.T) {
	a := netmock.NewAddr("tcp", "127.0.0.1:9001")
	if a.Network() != "tcp" {
		t.Errorf("Network: got %q, want %q", a.Network(), "tcp")
	}
}

func TestAddr_String(t *testing.T) {
	a := netmock.NewAddr("tcp", "127.0.0.1:9001")
	if a.String() != "127.0.0.1:9001" {
		t.Errorf("String: got %q, want %q", a.String(), "127.0.0.1:9001")
	}
}

func TestAddr_ImplementsNetAddr(t *testing.T) {
	var _ net.Addr = netmock.NewAddr("tcp", "127.0.0.1:0")
}

// --- Conn tests ---

func TestConn_ImplementsNetConn(t *testing.T) {
	var _ net.Conn = netmock.NewConn()
}

func TestConn_ReadWrite(t *testing.T) {
	c := netmock.NewConn()
	c.ReadBuf.WriteString("hello")

	buf := make([]byte, 5)
	n, err := c.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("Read error: %v", err)
	}
	if string(buf[:n]) != "hello" {
		t.Errorf("Read: got %q, want %q", string(buf[:n]), "hello")
	}
}

func TestConn_Write(t *testing.T) {
	c := netmock.NewConn()
	n, err := c.Write([]byte("world"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 5 {
		t.Errorf("Write n: got %d, want 5", n)
	}
	if c.WriteBuf.String() != "world" {
		t.Errorf("WriteBuf: got %q, want %q", c.WriteBuf.String(), "world")
	}
}

func TestConn_Close(t *testing.T) {
	c := netmock.NewConn()
	if c.IsClosed() {
		t.Fatal("new conn should not be closed")
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	if !c.IsClosed() {
		t.Fatal("conn should be closed after Close")
	}
	// Idempotent close
	if err := c.Close(); err != nil {
		t.Fatalf("second Close error: %v", err)
	}
}

func TestConn_ReadAfterClose(t *testing.T) {
	c := netmock.NewConn()
	_ = c.Close()
	_, err := c.Read(make([]byte, 4))
	if !errors.Is(err, netmock.ErrConnClosed) {
		t.Errorf("Read after close: got %v, want ErrConnClosed", err)
	}
}

func TestConn_WriteAfterClose(t *testing.T) {
	c := netmock.NewConn()
	_ = c.Close()
	_, err := c.Write([]byte("x"))
	if !errors.Is(err, netmock.ErrConnClosed) {
		t.Errorf("Write after close: got %v, want ErrConnClosed", err)
	}
}

func TestConn_ReadErr(t *testing.T) {
	c := netmock.NewConn()
	want := errors.New("read failure")
	c.ReadErr = want
	_, err := c.Read(make([]byte, 4))
	if err != want {
		t.Errorf("Read error: got %v, want %v", err, want)
	}
}

func TestConn_WriteErr(t *testing.T) {
	c := netmock.NewConn()
	want := errors.New("write failure")
	c.WriteErr = want
	_, err := c.Write([]byte("x"))
	if err != want {
		t.Errorf("Write error: got %v, want %v", err, want)
	}
}

func TestConn_CloseErr(t *testing.T) {
	c := netmock.NewConn()
	want := errors.New("close failure")
	c.CloseErr = want
	if err := c.Close(); err != want {
		t.Errorf("Close error: got %v, want %v", err, want)
	}
}

func TestConn_Addresses(t *testing.T) {
	local := netmock.NewAddr("tcp", "127.0.0.1:1111")
	remote := netmock.NewAddr("tcp", "10.0.0.1:2222")
	c := netmock.NewConnWithAddrs(local, remote)
	if c.LocalAddr().String() != "127.0.0.1:1111" {
		t.Errorf("LocalAddr: got %v", c.LocalAddr())
	}
	if c.RemoteAddr().String() != "10.0.0.1:2222" {
		t.Errorf("RemoteAddr: got %v", c.RemoteAddr())
	}
}

func TestConn_Deadlines(t *testing.T) {
	c := netmock.NewConn()
	dl := time.Now().Add(time.Second)
	if err := c.SetDeadline(dl); err != nil {
		t.Errorf("SetDeadline: %v", err)
	}
	if err := c.SetReadDeadline(dl); err != nil {
		t.Errorf("SetReadDeadline: %v", err)
	}
	if err := c.SetWriteDeadline(dl); err != nil {
		t.Errorf("SetWriteDeadline: %v", err)
	}
}

func TestConn_ConcurrentReadWrite(t *testing.T) {
	c := netmock.NewConn()
	c.ReadBuf.Write(make([]byte, 1024))

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_, _ = c.Write([]byte("x"))
		}
	}()
	go func() {
		defer wg.Done()
		buf := make([]byte, 4)
		for i := 0; i < 50; i++ {
			_, _ = c.Read(buf)
		}
	}()
	wg.Wait()
}

// --- Listener tests ---

func TestListener_ImplementsNetListener(t *testing.T) {
	var _ net.Listener = netmock.NewListener("127.0.0.1:9001")
}

func TestListener_AcceptEnqueued(t *testing.T) {
	ln := netmock.NewListener("127.0.0.1:9001")
	conn := netmock.NewConn()
	ln.Enqueue(conn)

	got, err := ln.Accept()
	if err != nil {
		t.Fatalf("Accept error: %v", err)
	}
	if got != conn {
		t.Error("Accept returned unexpected connection")
	}
}

func TestListener_AcceptAfterClose(t *testing.T) {
	ln := netmock.NewListener("127.0.0.1:9001")
	_ = ln.Close()

	_, err := ln.Accept()
	if !errors.Is(err, netmock.ErrListenerClosed) {
		t.Errorf("Accept after close: got %v, want ErrListenerClosed", err)
	}
}

func TestListener_CloseIdempotent(t *testing.T) {
	ln := netmock.NewListener("127.0.0.1:9001")
	if err := ln.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := ln.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestListener_Addr(t *testing.T) {
	ln := netmock.NewListener("127.0.0.1:9001")
	if ln.Addr().String() != "127.0.0.1:9001" {
		t.Errorf("Addr: got %v", ln.Addr())
	}
}

func TestListener_AcceptMultiple(t *testing.T) {
	ln := netmock.NewListener("127.0.0.1:9001")
	for i := 0; i < 5; i++ {
		ln.Enqueue(netmock.NewConn())
	}
	for i := 0; i < 5; i++ {
		conn, err := ln.Accept()
		if err != nil {
			t.Fatalf("Accept %d: %v", i, err)
		}
		if conn == nil {
			t.Fatalf("Accept %d returned nil conn", i)
		}
	}
}

// --- Pipe tests ---

func TestPipe_ReadWrite(t *testing.T) {
	client, server := netmock.Pipe()

	// Write from client, read on server.
	data := []byte("ping")
	if _, err := client.Write(data); err != nil {
		t.Fatalf("client Write: %v", err)
	}
	buf := make([]byte, 4)
	n, err := server.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("server Read: %v", err)
	}
	if string(buf[:n]) != "ping" {
		t.Errorf("server Read: got %q, want ping", string(buf[:n]))
	}
}

func TestPipe_BidirectionalFlow(t *testing.T) {
	client, server := netmock.Pipe()

	// Client → Server
	_, _ = client.Write([]byte("request"))
	buf := make([]byte, 7)
	n, _ := server.Read(buf)
	if string(buf[:n]) != "request" {
		t.Errorf("c→s: got %q", string(buf[:n]))
	}

	// Server → Client
	_, _ = server.Write([]byte("response"))
	buf2 := make([]byte, 8)
	n2, _ := client.Read(buf2)
	if string(buf2[:n2]) != "response" {
		t.Errorf("s→c: got %q", string(buf2[:n2]))
	}
}

func TestPipe_ImplementsNetConn(t *testing.T) {
	client, server := netmock.Pipe()
	var _ net.Conn = client
	var _ net.Conn = server
}

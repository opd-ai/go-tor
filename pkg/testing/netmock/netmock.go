// Package netmock provides reusable mock types for net.Conn, net.Listener,
// and net.Addr to support unit testing of network-facing code without
// requiring live network connections.
//
// All types implement the standard library interfaces and are safe for
// concurrent use. They are intended as drop-in test doubles for any
// code accepting the net.Conn or net.Listener interfaces.
//
// Example – test a read/write loop against a Conn:
//
//	conn := netmock.NewConn()
//	conn.ReadBuf.WriteString("hello\n")
//
//	buf := make([]byte, 6)
//	n, _ := conn.Read(buf)
//	// buf[:n] == "hello\n"
//
//	conn.Write([]byte("world"))
//	// conn.WriteBuf.String() == "world"
//
// Example – test a server's Accept loop:
//
//	ln := netmock.NewListener("127.0.0.1:9001")
//	conn := netmock.NewConn()
//	ln.Enqueue(conn) // will be returned by the next Accept call
//	ln.Close()       // subsequent Accept returns error
package netmock

import (
	"bytes"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

// ErrConnClosed is returned when a closed Conn is used.
var ErrConnClosed = errors.New("use of closed connection")

// ErrListenerClosed is returned when a closed Listener is used.
var ErrListenerClosed = errors.New("use of closed listener")

// Addr implements net.Addr for testing.
type Addr struct {
	NetworkStr string
	AddrStr    string
}

// Network returns the network name.
func (a *Addr) Network() string { return a.NetworkStr }

// String returns the address string.
func (a *Addr) String() string { return a.AddrStr }

// NewAddr creates a new Addr with the given network and address.
func NewAddr(network, addr string) *Addr {
	return &Addr{NetworkStr: network, AddrStr: addr}
}

// Conn is a mock implementation of net.Conn backed by in-memory buffers.
// ReadBuf is pre-populated with data to be returned by Read.
// WriteBuf accumulates all data passed to Write.
type Conn struct {
	ReadBuf  *bytes.Buffer
	WriteBuf *bytes.Buffer

	// Controllable errors for Read, Write, and Close.
	ReadErr  error
	WriteErr error
	CloseErr error

	localAddr  net.Addr
	remoteAddr net.Addr

	mu       sync.Mutex
	closed   bool
	deadline time.Time
}

// NewConn creates a new Conn with empty buffers and default addresses.
func NewConn() *Conn {
	return &Conn{
		ReadBuf:    &bytes.Buffer{},
		WriteBuf:   &bytes.Buffer{},
		localAddr:  NewAddr("tcp", "127.0.0.1:0"),
		remoteAddr: NewAddr("tcp", "127.0.0.1:0"),
	}
}

// NewConnWithAddrs creates a Conn with specific local/remote addresses.
func NewConnWithAddrs(local, remote net.Addr) *Conn {
	c := NewConn()
	c.localAddr = local
	c.remoteAddr = remote
	return c
}

// Read reads from the pre-populated ReadBuf. Returns ReadErr if set.
func (c *Conn) Read(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, ErrConnClosed
	}
	if c.ReadErr != nil {
		return 0, c.ReadErr
	}
	n, err := c.ReadBuf.Read(b)
	if err == io.EOF && n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

// Write appends to WriteBuf. Returns WriteErr if set.
func (c *Conn) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, ErrConnClosed
	}
	if c.WriteErr != nil {
		return 0, c.WriteErr
	}
	return c.WriteBuf.Write(b)
}

// Close marks the connection as closed. Returns CloseErr if set.
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.CloseErr
}

// IsClosed reports whether Close has been called.
func (c *Conn) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// LocalAddr returns the local network address.
func (c *Conn) LocalAddr() net.Addr { return c.localAddr }

// RemoteAddr returns the remote network address.
func (c *Conn) RemoteAddr() net.Addr { return c.remoteAddr }

// SetDeadline is a no-op for the mock connection.
func (c *Conn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deadline = t
	return nil
}

// SetReadDeadline is a no-op for the mock connection.
func (c *Conn) SetReadDeadline(t time.Time) error { return c.SetDeadline(t) }

// SetWriteDeadline is a no-op for the mock connection.
func (c *Conn) SetWriteDeadline(t time.Time) error { return c.SetDeadline(t) }

// Listener is a mock implementation of net.Listener.
// Connections to be returned by Accept are enqueued with Enqueue.
// Calling Close causes subsequent Accept calls to return ErrListenerClosed.
type Listener struct {
	addr   net.Addr
	conns  chan net.Conn
	closed chan struct{}
	once   sync.Once
}

// NewListener creates a new Listener bound to the given address string.
func NewListener(addr string) *Listener {
	return &Listener{
		addr:   NewAddr("tcp", addr),
		conns:  make(chan net.Conn, 16),
		closed: make(chan struct{}),
	}
}

// Enqueue adds a connection to be returned by the next Accept call.
func (l *Listener) Enqueue(conn net.Conn) {
	select {
	case l.conns <- conn:
	case <-l.closed:
	}
}

// Accept waits for and returns the next enqueued connection.
// Returns ErrListenerClosed when the listener has been closed.
func (l *Listener) Accept() (net.Conn, error) {
	select {
	case conn, ok := <-l.conns:
		if !ok {
			return nil, ErrListenerClosed
		}
		return conn, nil
	case <-l.closed:
		return nil, ErrListenerClosed
	}
}

// Close shuts down the listener.
func (l *Listener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

// Addr returns the listener's network address.
func (l *Listener) Addr() net.Addr { return l.addr }

// Pipe creates a pair of connected Conn objects whose reads and writes
// are synchronised through in-memory pipes. Writes to one end can be
// read from the other, mirroring the behaviour of net.Pipe.
func Pipe() (client, server *Conn) {
	cToS := &bytes.Buffer{}
	sToC := &bytes.Buffer{}

	client = &Conn{
		ReadBuf:    sToC,
		WriteBuf:   cToS,
		localAddr:  NewAddr("tcp", "127.0.0.1:1"),
		remoteAddr: NewAddr("tcp", "127.0.0.1:2"),
	}
	server = &Conn{
		ReadBuf:    cToS,
		WriteBuf:   sToC,
		localAddr:  NewAddr("tcp", "127.0.0.1:2"),
		remoteAddr: NewAddr("tcp", "127.0.0.1:1"),
	}
	return client, server
}

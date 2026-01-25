// Package relay - Test helpers shared across relay tests
package relay

import (
	"io"
	"net"
	"time"
)

// testMockConn is a mock connection for testing
// This is a separate implementation from mockConn in or_handler_test.go
// to avoid conflicts when running individual test files
type testMockConn struct {
	readData  []byte
	readPos   int
	writeData []byte
	closed    bool
}

func newTestMockConn() *testMockConn {
	return &testMockConn{
		readData: make([]byte, 0),
	}
}

func (m *testMockConn) Read(b []byte) (n int, err error) {
	if m.readPos >= len(m.readData) {
		return 0, io.EOF
	}
	n = copy(b, m.readData[m.readPos:])
	m.readPos += n
	return n, nil
}

func (m *testMockConn) Write(b []byte) (n int, err error) {
	m.writeData = append(m.writeData, b...)
	return len(b), nil
}

func (m *testMockConn) Close() error {
	m.closed = true
	return nil
}

func (m *testMockConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9001}
}

func (m *testMockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 54321}
}

func (m *testMockConn) SetDeadline(t time.Time) error      { return nil }
func (m *testMockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *testMockConn) SetWriteDeadline(t time.Time) error { return nil }

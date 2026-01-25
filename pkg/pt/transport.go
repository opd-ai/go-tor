// Package pt implements pluggable transport support for Tor per pt-spec.txt.
//
// This package provides interfaces and implementations for PT version 2
// IPC protocol, allowing go-tor to use external pluggable transports like
// obfs4, meek, and snowflake for censorship circumvention.
//
// Specification: https://spec.torproject.org/pt-spec
package pt

import (
	"context"
	"net"
)

// Transport represents a pluggable transport that can wrap Tor connections.
type Transport interface {
	// Name returns the transport method name (e.g., "obfs4", "meek").
	Name() string

	// Dial establishes a connection through the pluggable transport.
	// The address format depends on the transport implementation.
	Dial(ctx context.Context, address string) (net.Conn, error)

	// Close shuts down the transport and releases resources.
	Close() error
}

// ClientTransport represents a client-side pluggable transport.
// It manages the lifecycle of PT processes and provides connection capabilities.
type ClientTransport interface {
	Transport

	// Start initializes the pluggable transport client.
	// This typically launches the PT subprocess and performs IPC handshake.
	Start(ctx context.Context) error

	// Methods returns the available transport methods advertised by the PT.
	// Each method corresponds to a CMETHOD line in the PT protocol.
	Methods() []string

	// IsRunning reports whether the PT process is currently running.
	IsRunning() bool
}

// ServerTransport represents a server-side pluggable transport.
// This is used for bridge relays to accept incoming PT connections.
type ServerTransport interface {
	Transport

	// Listen starts accepting connections on the PT server.
	Listen(ctx context.Context, bindAddr string) (net.Listener, error)

	// Methods returns the server transport methods (SMETHOD lines).
	Methods() []string
}

// TransportConfig holds configuration for a pluggable transport.
type TransportConfig struct {
	// BinaryPath is the path to the PT executable.
	BinaryPath string

	// StateDir is the directory for PT state files.
	StateDir string

	// Options contains PT-specific configuration options.
	// For example, obfs4 might need "cert" and "iat-mode".
	Options map[string]string

	// TorSOCKSPort is the SOCKS port for the PT to connect to Tor (server-side).
	TorSOCKSPort int

	// ProxyURL is an optional upstream proxy (e.g., "socks5://127.0.0.1:9050").
	ProxyURL string
}

// MethodInfo describes a single transport method provided by a PT.
type MethodInfo struct {
	// Name is the transport method name.
	Name string

	// SOCKSVersion is the SOCKS protocol version (4, 4a, or 5).
	SOCKSVersion int

	// Address is the host:port where the PT accepts SOCKS connections.
	Address string

	// Args contains method-specific arguments.
	Args map[string]string

	// Options contains extended options.
	Options map[string]string
}

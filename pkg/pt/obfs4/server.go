package obfs4

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/opd-ai/go-tor/pkg/pt"
)

// Server implements an obfs4 server transport for bridge relays using external obfs4proxy.
type Server struct {
	config        ServerConfig
	managedServer *pt.ManagedServer
	started       bool
}

// ServerConfig holds configuration for obfs4 server.
type ServerConfig struct {
	// BinaryPath is the path to obfs4proxy executable.
	// If empty, uses discovery to find obfs4proxy.
	BinaryPath string

	// BindAddr is the address to listen on (e.g., "127.0.0.1:0").
	BindAddr string

	// StateDir is the directory for storing obfs4 state and keys.
	StateDir string

	// IATMode controls inter-arrival time obfuscation.
	// 0: disabled, 1: enabled, 2: paranoid.
	IATMode int

	// ExtORPort is the Extended ORPort address for the bridge relay.
	ExtORPort string
}

// NewServer creates a new obfs4 server transport.
func NewServer(config ServerConfig) (*Server, error) {
	if config.StateDir == "" {
		return nil, fmt.Errorf("obfs4: state directory is required")
	}

	// Discover obfs4proxy if not specified
	if config.BinaryPath == "" {
		discovered := pt.DiscoverCommonPTs()
		obfs4Path, ok := discovered["obfs4proxy"]
		if !ok {
			return nil, fmt.Errorf("obfs4: obfs4proxy binary not found (install obfs4proxy)")
		}
		config.BinaryPath = obfs4Path
	}

	if config.BindAddr == "" {
		config.BindAddr = "127.0.0.1:0"
	}

	// Build PT options
	options := make(map[string]string)
	options["iat-mode"] = fmt.Sprintf("%d", config.IATMode)

	// Create managed PT server
	ptConfig := pt.TransportConfig{
		BinaryPath: config.BinaryPath,
		StateDir:   config.StateDir,
		Options:    options,
	}

	managedServer, err := pt.NewManagedServer(ptConfig)
	if err != nil {
		return nil, fmt.Errorf("obfs4: failed to create managed server: %w", err)
	}

	return &Server{
		config:        config,
		managedServer: managedServer,
	}, nil
}

// Name returns the transport name.
func (s *Server) Name() string {
	return "obfs4"
}

// Start initializes the obfs4 server by launching obfs4proxy.
func (s *Server) Start(ctx context.Context) error {
	if s.started {
		return nil
	}

	if err := s.managedServer.Start(ctx); err != nil {
		return fmt.Errorf("obfs4: failed to start PT server: %w", err)
	}

	s.started = true
	return nil
}

// Listen starts the obfs4 server and returns a listener.
// The server must be started first via Start().
func (s *Server) Listen(ctx context.Context, bindAddr string) (net.Listener, error) {
	if !s.started {
		if err := s.Start(ctx); err != nil {
			return nil, err
		}
	}

	// The managed server handles listening
	// For obfs4, we return the listener from the managed server
	// Note: obfs4proxy manages its own listener via SMETHOD protocol
	return &obfs4ServerListener{
		server: s,
	}, nil
}

// Dial is not supported for server transports.
func (s *Server) Dial(ctx context.Context, address string) (net.Conn, error) {
	return nil, fmt.Errorf("obfs4: Dial not supported on server transport")
}

// Close closes the server transport.
func (s *Server) Close() error {
	s.started = false
	return s.managedServer.Close()
}

// Methods returns the server methods.
func (s *Server) Methods() []string {
	if s.managedServer == nil {
		return []string{"obfs4"}
	}
	return s.managedServer.Methods()
}

// GetCertificate returns the bridge's obfs4 certificate for distribution.
// This certificate should be included in bridge lines.
func (s *Server) GetCertificate() (string, error) {
	if !s.started {
		return "", fmt.Errorf("obfs4: server not started")
	}

	// Read certificate from state directory
	// obfs4proxy stores the certificate in obfs4_bridgeline.txt
	certFile := filepath.Join(s.config.StateDir, "obfs4_bridgeline.txt")
	data, err := os.ReadFile(certFile)
	if err != nil {
		return "", fmt.Errorf("obfs4: failed to read certificate: %w", err)
	}

	// Extract cert parameter from bridge line
	// Format: "Bridge obfs4 <IP:PORT> <FINGERPRINT> cert=<CERT> iat-mode=<MODE>"
	cert, err := extractParam(string(data), "cert=")
	if err != nil {
		return "", fmt.Errorf("obfs4: failed to extract certificate: %w", err)
	}

	return cert, nil
}

// GetBindAddress returns the address where obfs4proxy is listening.
func (s *Server) GetBindAddress() (string, error) {
	if !s.started {
		return "", fmt.Errorf("obfs4: server not started")
	}

	// Read from SMETHOD lines
	methods := s.managedServer.Methods()
	if len(methods) == 0 {
		return "", fmt.Errorf("obfs4: no methods available")
	}

	// The bind address is reported in SMETHOD lines
	// For now, return the configured bind address
	return s.config.BindAddr, nil
}

// obfs4ServerListener implements net.Listener for obfs4 server.
// Note: obfs4proxy manages the actual listening, this is a placeholder.
type obfs4ServerListener struct {
	server *Server
}

// Accept is not directly supported as obfs4proxy manages connections.
func (l *obfs4ServerListener) Accept() (net.Conn, error) {
	return nil, fmt.Errorf("obfs4: direct Accept not supported (obfs4proxy manages connections)")
}

// Close closes the listener.
func (l *obfs4ServerListener) Close() error {
	return l.server.Close()
}

// Addr returns the listener's network address.
func (l *obfs4ServerListener) Addr() net.Addr {
	addr, err := net.ResolveTCPAddr("tcp", l.server.config.BindAddr)
	if err != nil {
		return nil
	}
	return addr
}

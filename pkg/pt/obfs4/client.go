// Package obfs4 implements the obfs4 pluggable transport client.
//
// obfs4 is a transport protocol that obscures traffic patterns and content
// to evade censorship. It uses the ntor handshake variant for key exchange
// and AES-GCM for encryption with IAT (inter-arrival time) obfuscation.
//
// This implementation wraps external obfs4proxy binary as a managed PT process,
// following the pt-spec.txt IPC protocol. It uses the existing PT infrastructure
// in pkg/pt to manage the obfs4proxy subprocess.
//
// Specification: https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/obfs4/-/blob/HEAD/doc/obfs4-spec.txt
package obfs4

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/opd-ai/go-tor/pkg/pt"
)

// Client implements an obfs4 client transport using external obfs4proxy binary.
type Client struct {
	config        ClientConfig
	managedClient *pt.ManagedClient
	started       bool
}

// ClientConfig holds configuration for obfs4 client.
type ClientConfig struct {
	// BinaryPath is the path to obfs4proxy executable.
	// If empty, uses discovery to find obfs4proxy.
	BinaryPath string

	// Cert is the bridge's obfs4 certificate (node-id and public-key).
	// This is a base64-encoded string provided in the bridge line.
	Cert string

	// IATMode controls inter-arrival time obfuscation.
	// 0: disabled, 1: enabled, 2: paranoid (more padding).
	IATMode int

	// StateDir is the directory for PT state files.
	StateDir string

	// DialTimeout is the timeout for establishing connections.
	DialTimeout time.Duration
}

// NewClient creates a new obfs4 client transport.
func NewClient(config ClientConfig) (*Client, error) {
	if config.Cert == "" {
		return nil, fmt.Errorf("obfs4: certificate is required")
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

	if config.StateDir == "" {
		config.StateDir = "/tmp/obfs4-state"
	}

	if config.DialTimeout == 0 {
		config.DialTimeout = 30 * time.Second
	}

	// Build PT options
	options := make(map[string]string)
	options["cert"] = config.Cert
	options["iat-mode"] = fmt.Sprintf("%d", config.IATMode)

	// Create managed PT client
	ptConfig := pt.TransportConfig{
		BinaryPath: config.BinaryPath,
		StateDir:   config.StateDir,
		Options:    options,
	}

	managedClient, err := pt.NewManagedClient(ptConfig)
	if err != nil {
		return nil, fmt.Errorf("obfs4: failed to create managed client: %w", err)
	}

	return &Client{
		config:        config,
		managedClient: managedClient,
	}, nil
}

// Name returns the transport name.
func (c *Client) Name() string {
	return "obfs4"
}

// Start initializes the obfs4 transport by launching obfs4proxy.
func (c *Client) Start(ctx context.Context) error {
	if c.started {
		return nil
	}

	if err := c.managedClient.Start(ctx); err != nil {
		return fmt.Errorf("obfs4: failed to start PT process: %w", err)
	}

	c.started = true
	return nil
}

// Dial establishes an obfs4 connection to the specified address.
// The obfs4proxy process must be started first via Start().
func (c *Client) Dial(ctx context.Context, address string) (net.Conn, error) {
	if !c.started {
		if err := c.Start(ctx); err != nil {
			return nil, err
		}
	}

	// Dial through the managed PT client
	// The managed client provides a SOCKS proxy that handles obfs4 protocol
	conn, err := c.managedClient.Dial(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("obfs4: dial failed: %w", err)
	}

	return conn, nil
}

// Close closes the client transport and terminates obfs4proxy process.
func (c *Client) Close() error {
	c.started = false
	return c.managedClient.Close()
}

// IsRunning reports whether the obfs4proxy process is running.
func (c *Client) IsRunning() bool {
	return c.managedClient.IsRunning()
}

// Methods returns the available transport methods.
func (c *Client) Methods() []string {
	return c.managedClient.Methods()
}

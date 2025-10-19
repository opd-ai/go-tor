// Package client provides simplified API for zero-configuration Tor client usage.
package client

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/logger"
)

const (
	// ReadinessCheckInterval is the polling interval for WaitUntilReady
	ReadinessCheckInterval = 100 * time.Millisecond
)

// SimpleClient provides a zero-configuration Tor client interface.
type SimpleClient struct {
	client *Client
	logger *logger.Logger
}

// Connect creates and starts a new Tor client with sensible defaults.
// This is the main entry point for zero-configuration usage.
// It automatically:
// - Detects and creates appropriate data directories
// - Selects available ports
// - Bootstraps Tor network connection
// - Establishes initial circuits
//
// Example usage:
//
//	client, err := Connect()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//	
//	proxyURL := client.ProxyURL()
//	// Use proxyURL with your HTTP client
func Connect() (*SimpleClient, error) {
	return ConnectWithContext(context.Background())
}

// ConnectWithContext creates and starts a Tor client with the given context.
func ConnectWithContext(ctx context.Context) (*SimpleClient, error) {
	// Create default configuration
	cfg := config.DefaultConfig()
	
	// Create logger with info level by default
	logLevel, err := logger.ParseLevel("info")
	if err != nil {
		return nil, fmt.Errorf("failed to parse log level: %w", err)
	}
	logr := logger.New(logLevel, os.Stdout)

	// Create client
	client, err := New(cfg, logr)
	if err != nil {
		return nil, fmt.Errorf("failed to create Tor client: %w", err)
	}

	// Start the client
	if err := client.Start(ctx); err != nil {
		// Clean up on error
		_ = client.Stop()
		return nil, fmt.Errorf("failed to start Tor client: %w", err)
	}

	return &SimpleClient{
		client: client,
		logger: logr,
	}, nil
}

// ConnectWithOptions creates and starts a Tor client with custom options.
func ConnectWithOptions(opts *Options) (*SimpleClient, error) {
	return ConnectWithOptionsContext(context.Background(), opts)
}

// ConnectWithOptionsContext creates and starts a Tor client with custom options and context.
func ConnectWithOptionsContext(ctx context.Context, opts *Options) (*SimpleClient, error) {
	if opts == nil {
		opts = &Options{}
	}

	// Create configuration with defaults
	cfg := config.DefaultConfig()

	// Apply custom options
	if opts.SocksPort > 0 {
		cfg.SocksPort = opts.SocksPort
	}
	if opts.ControlPort > 0 {
		cfg.ControlPort = opts.ControlPort
	}
	if opts.DataDirectory != "" {
		cfg.DataDirectory = opts.DataDirectory
	}
	if opts.LogLevel != "" {
		cfg.LogLevel = opts.LogLevel
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create logger
	logLevel, err := logger.ParseLevel(cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}
	logr := logger.New(logLevel, os.Stdout)

	// Create client
	client, err := New(cfg, logr)
	if err != nil {
		return nil, fmt.Errorf("failed to create Tor client: %w", err)
	}

	// Start the client
	if err := client.Start(ctx); err != nil {
		// Clean up on error
		_ = client.Stop()
		return nil, fmt.Errorf("failed to start Tor client: %w", err)
	}

	return &SimpleClient{
		client: client,
		logger: logr,
	}, nil
}

// Options allows customization of the Tor client.
type Options struct {
	// SocksPort specifies the SOCKS5 proxy port (default: 9050)
	SocksPort int
	
	// ControlPort specifies the control protocol port (default: 9051)
	ControlPort int
	
	// DataDirectory specifies the data directory (default: platform-specific)
	DataDirectory string
	
	// LogLevel specifies the log level: debug, info, warn, error (default: info)
	LogLevel string
}

// Close gracefully shuts down the Tor client.
func (c *SimpleClient) Close() error {
	return c.client.Stop()
}

// ProxyURL returns the SOCKS5 proxy URL that applications can use.
func (c *SimpleClient) ProxyURL() string {
	stats := c.client.GetStats()
	return fmt.Sprintf("socks5://127.0.0.1:%d", stats.SocksPort)
}

// ProxyAddr returns the SOCKS5 proxy address (host:port).
func (c *SimpleClient) ProxyAddr() string {
	stats := c.client.GetStats()
	return fmt.Sprintf("127.0.0.1:%d", stats.SocksPort)
}

// IsReady returns true if the client has active circuits and is ready to proxy traffic.
func (c *SimpleClient) IsReady() bool {
	stats := c.client.GetStats()
	return stats.ActiveCircuits > 0
}

// WaitUntilReady blocks until the client has active circuits or the timeout expires.
func (c *SimpleClient) WaitUntilReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(ReadinessCheckInterval)
	defer ticker.Stop()

	for {
		if c.IsReady() {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for Tor client to be ready")
		}

		<-ticker.C
	}
}

// Stats returns current client statistics.
func (c *SimpleClient) Stats() Stats {
	return c.client.GetStats()
}

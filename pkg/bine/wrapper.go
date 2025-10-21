// Package bine provides a zero-configuration wrapper for using cretz/bine with go-tor.
//
// This package simplifies the integration of cretz/bine (Tor control library) with
// go-tor's pure-Go Tor implementation. It automatically manages:
// - go-tor client for network connectivity (pure Go, no external binary needed)
// - bine for hidden service management (when Tor binary is available)
// - SOCKS5 proxy configuration
// - Lifecycle management and graceful shutdown
//
// Example usage for client operations:
//
//	client, err := bine.Connect()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Use the SOCKS proxy
//	httpClient, err := client.HTTPClient()
//	resp, err := httpClient.Get("https://check.torproject.org")
//
// Example usage for hidden services:
//
//	client, err := bine.Connect()
//	defer client.Close()
//
//	service, err := client.CreateHiddenService(ctx, 80)
//	fmt.Printf("Service: http://%s\n", service.OnionAddress())
//	http.Serve(service, handler)
package bine

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/cretz/bine/tor"
	"github.com/opd-ai/go-tor/pkg/client"
	"golang.org/x/net/proxy"
)

// Client provides a unified interface for go-tor and bine integration.
type Client struct {
	goTorClient *client.SimpleClient
	bineClient  *tor.Tor
	proxyDialer proxy.Dialer
}

// Options configures the bine wrapper client.
type Options struct {
	// SocksPort specifies the SOCKS5 proxy port (default: auto-selected)
	SocksPort int

	// ControlPort specifies the control protocol port (default: auto-selected)
	ControlPort int

	// DataDirectory specifies the data directory (default: platform-specific)
	DataDirectory string

	// LogLevel specifies the log level: debug, info, warn, error (default: info)
	LogLevel string

	// EnableBine enables bine Tor instance for hidden services (requires Tor binary)
	// If false, only go-tor client is started (default: false)
	EnableBine bool

	// StartupTimeout is the maximum time to wait for Tor to be ready (default: 90s)
	StartupTimeout time.Duration
}

// Connect creates and starts a new integrated Tor client with zero configuration.
// This automatically:
// - Starts go-tor for pure-Go Tor connectivity
// - Waits for circuits to be ready
// - Optionally starts bine if enabled in options
//
// Example:
//
//	client, err := bine.Connect()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
func Connect() (*Client, error) {
	return ConnectWithOptions(nil)
}

// ConnectWithOptions creates a Tor client with custom configuration.
func ConnectWithOptions(opts *Options) (*Client, error) {
	return ConnectWithOptionsContext(context.Background(), opts)
}

// ConnectWithOptionsContext creates a Tor client with custom configuration and context.
func ConnectWithOptionsContext(ctx context.Context, opts *Options) (*Client, error) {
	if opts == nil {
		opts = &Options{}
	}

	// Set defaults
	if opts.StartupTimeout == 0 {
		opts.StartupTimeout = 90 * time.Second
	}

	// Start go-tor client
	var goTorClient *client.SimpleClient
	var err error

	if opts.SocksPort > 0 || opts.ControlPort > 0 || opts.DataDirectory != "" || opts.LogLevel != "" {
		clientOpts := &client.Options{
			SocksPort:     opts.SocksPort,
			ControlPort:   opts.ControlPort,
			DataDirectory: opts.DataDirectory,
			LogLevel:      opts.LogLevel,
		}
		goTorClient, err = client.ConnectWithOptionsContext(ctx, clientOpts)
	} else {
		goTorClient, err = client.ConnectWithContext(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to start go-tor client: %w", err)
	}

	// Wait for go-tor to be ready
	if err := goTorClient.WaitUntilReady(opts.StartupTimeout); err != nil {
		goTorClient.Close()
		return nil, fmt.Errorf("timeout waiting for go-tor: %w", err)
	}

	// Create SOCKS5 dialer
	dialer, err := proxy.SOCKS5("tcp", goTorClient.ProxyAddr(), nil, proxy.Direct)
	if err != nil {
		goTorClient.Close()
		return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	c := &Client{
		goTorClient: goTorClient,
		proxyDialer: dialer,
	}

	// Optionally start bine for hidden services
	if opts.EnableBine {
		bineClient, err := tor.Start(ctx, nil)
		if err != nil {
			goTorClient.Close()
			return nil, fmt.Errorf("failed to start bine (Tor binary required): %w", err)
		}
		c.bineClient = bineClient
	}

	return c, nil
}

// Close gracefully shuts down all Tor instances.
func (c *Client) Close() error {
	var firstErr error

	if c.bineClient != nil {
		if err := c.bineClient.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if c.goTorClient != nil {
		if err := c.goTorClient.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// ProxyAddr returns the SOCKS5 proxy address (e.g., "127.0.0.1:9050").
func (c *Client) ProxyAddr() string {
	return c.goTorClient.ProxyAddr()
}

// ProxyURL returns the SOCKS5 proxy URL (e.g., "socks5://127.0.0.1:9050").
func (c *Client) ProxyURL() string {
	return c.goTorClient.ProxyURL()
}

// Dialer returns a SOCKS5 dialer for making connections through Tor.
func (c *Client) Dialer() proxy.Dialer {
	return c.proxyDialer
}

// HTTPClient returns an HTTP client configured to use the Tor SOCKS proxy.
// This is a convenience method for making HTTP requests through Tor.
//
// Example:
//
//	httpClient, err := client.HTTPClient()
//	resp, err := httpClient.Get("https://check.torproject.org")
func (c *Client) HTTPClient() (*http.Client, error) {
	return &http.Client{
		Transport: &http.Transport{
			Dial: c.proxyDialer.Dial,
		},
		Timeout: 30 * time.Second,
	}, nil
}

// HiddenServiceConfig configures a hidden service.
type HiddenServiceConfig struct {
	// RemotePorts are the ports that clients connect to (e.g., 80 for HTTP)
	RemotePorts []int

	// LocalAddr is the local address to forward to (default: random port)
	LocalAddr string

	// PrivateKey is an optional private key for persistent .onion address
	// If nil, a new key is generated
	PrivateKey interface{}
}

// HiddenService represents an active hidden service.
type HiddenService struct {
	onion   *tor.OnionService
	service net.Listener
}

// OnionAddress returns the .onion address (without http://).
func (hs *HiddenService) OnionAddress() string {
	return fmt.Sprintf("%v.onion", hs.onion.ID)
}

// Accept waits for and returns the next connection to the service.
func (hs *HiddenService) Accept() (net.Conn, error) {
	return hs.onion.Accept()
}

// Close shuts down the hidden service.
func (hs *HiddenService) Close() error {
	return hs.onion.Close()
}

// Addr returns the listener's network address.
func (hs *HiddenService) Addr() net.Addr {
	return hs.onion.Addr()
}

// CreateHiddenService creates a v3 onion service.
// This requires bine to be enabled (EnableBine: true in options).
//
// Example:
//
//	client, _ := bine.ConnectWithOptions(&bine.Options{EnableBine: true})
//	defer client.Close()
//
//	service, _ := client.CreateHiddenService(ctx, 80)
//	fmt.Printf("Service: http://%s\n", service.OnionAddress())
//
//	http.Serve(service, handler)
func (c *Client) CreateHiddenService(ctx context.Context, remotePorts ...int) (*HiddenService, error) {
	if c.bineClient == nil {
		return nil, fmt.Errorf("bine not enabled; use EnableBine: true in options")
	}

	conf := &tor.ListenConf{
		RemotePorts: remotePorts,
		Version3:    true,
	}

	onion, err := c.bineClient.Listen(ctx, conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create hidden service: %w", err)
	}

	return &HiddenService{
		onion: onion,
	}, nil
}

// CreateHiddenServiceWithConfig creates a v3 onion service with custom configuration.
func (c *Client) CreateHiddenServiceWithConfig(ctx context.Context, config *HiddenServiceConfig) (*HiddenService, error) {
	if c.bineClient == nil {
		return nil, fmt.Errorf("bine not enabled; use EnableBine: true in options")
	}

	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	conf := &tor.ListenConf{
		RemotePorts: config.RemotePorts,
		Version3:    true,
	}

	if config.PrivateKey != nil {
		conf.Key = config.PrivateKey
	}

	onion, err := c.bineClient.Listen(ctx, conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create hidden service: %w", err)
	}

	return &HiddenService{
		onion: onion,
	}, nil
}

// IsReady returns true if the client is ready to handle requests.
func (c *Client) IsReady() bool {
	return c.goTorClient.IsReady()
}

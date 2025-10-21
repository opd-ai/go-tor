// Package helpers provides convenience functions for integrating go-tor with common Go patterns.
// This package simplifies the process of using the Tor client with standard library and popular third-party packages.
package helpers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/opd-ai/go-tor/pkg/client"
	"golang.org/x/net/proxy"
)

// TorClient is an interface that allows testing without a full Tor client.
// The client.SimpleClient satisfies this interface.
type TorClient interface {
	ProxyURL() string
}

// Ensure client.SimpleClient implements TorClient
var _ TorClient = (*client.SimpleClient)(nil)

// HTTPClientConfig configures the HTTP client with Tor proxy settings.
type HTTPClientConfig struct {
	// Timeout for HTTP requests (default: 30s)
	Timeout time.Duration

	// DialTimeout for establishing connections (default: 10s)
	DialTimeout time.Duration

	// TLSHandshakeTimeout for TLS handshake (default: 10s)
	TLSHandshakeTimeout time.Duration

	// MaxIdleConns controls the maximum number of idle connections (default: 10)
	MaxIdleConns int

	// IdleConnTimeout controls how long idle connections are kept (default: 90s)
	IdleConnTimeout time.Duration

	// DisableKeepAlives disables HTTP keep-alives (default: false)
	DisableKeepAlives bool
}

// DefaultHTTPClientConfig returns sensible defaults for Tor HTTP clients.
func DefaultHTTPClientConfig() *HTTPClientConfig {
	return &HTTPClientConfig{
		Timeout:             30 * time.Second,
		DialTimeout:         10 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		MaxIdleConns:        10,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
	}
}

// NewHTTPClient creates an http.Client configured to use the Tor SOCKS5 proxy.
// This is a convenience function that handles all the boilerplate configuration.
//
// Example:
//
//	torClient, _ := client.Connect()
//	defer torClient.Close()
//
//	httpClient, _ := helpers.NewHTTPClient(torClient, nil)
//	resp, _ := httpClient.Get("https://check.torproject.org")
func NewHTTPClient(torClient TorClient, config *HTTPClientConfig) (*http.Client, error) {
	if torClient == nil {
		return nil, fmt.Errorf("torClient cannot be nil")
	}

	if config == nil {
		config = DefaultHTTPClientConfig()
	}

	// Parse the SOCKS5 proxy URL
	proxyURL, err := url.Parse(torClient.ProxyURL())
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy URL: %w", err)
	}

	// Create SOCKS5 dialer
	dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	// Create custom transport with Tor proxy
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Apply DialTimeout if configured
			if config.DialTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, config.DialTimeout)
				defer cancel()
			}
			
			// Use the SOCKS5 dialer with context-aware dialing
			type result struct {
				conn net.Conn
				err  error
			}
			
			ch := make(chan result, 1)
			go func() {
				conn, err := dialer.Dial(network, addr)
				ch <- result{conn, err}
			}()
			
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case res := <-ch:
				return res.conn, res.err
			}
		},
		MaxIdleConns:          config.MaxIdleConns,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		DisableKeepAlives:     config.DisableKeepAlives,
		ResponseHeaderTimeout: config.Timeout,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}, nil
}

// NewHTTPTransport creates an http.Transport configured for Tor.
// This allows you to further customize the transport before creating the client.
//
// Example:
//
//	torClient, _ := client.Connect()
//	transport, _ := helpers.NewHTTPTransport(torClient, nil)
//	transport.DisableCompression = true // Custom configuration
//	httpClient := &http.Client{Transport: transport}
func NewHTTPTransport(torClient TorClient, config *HTTPClientConfig) (*http.Transport, error) {
	if torClient == nil {
		return nil, fmt.Errorf("torClient cannot be nil")
	}

	if config == nil {
		config = DefaultHTTPClientConfig()
	}

	// Parse the SOCKS5 proxy URL
	proxyURL, err := url.Parse(torClient.ProxyURL())
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy URL: %w", err)
	}

	// Create SOCKS5 dialer
	dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Apply DialTimeout if configured
			if config.DialTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, config.DialTimeout)
				defer cancel()
			}
			
			// Use the SOCKS5 dialer with context-aware dialing
			type result struct {
				conn net.Conn
				err  error
			}
			
			ch := make(chan result, 1)
			go func() {
				conn, err := dialer.Dial(network, addr)
				ch <- result{conn, err}
			}()
			
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case res := <-ch:
				return res.conn, res.err
			}
		},
		MaxIdleConns:          config.MaxIdleConns,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		DisableKeepAlives:     config.DisableKeepAlives,
		ResponseHeaderTimeout: config.Timeout,
	}, nil
}

// DialContext returns a DialContext function that uses the Tor SOCKS5 proxy.
// This is useful for custom network applications that need context-aware dialing.
//
// Example:
//
//	torClient, _ := client.Connect()
//	dialCtx := helpers.DialContext(torClient)
//	conn, err := dialCtx(context.Background(), "tcp", "example.onion:80")
func DialContext(torClient TorClient) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		if torClient == nil {
			return nil, fmt.Errorf("torClient cannot be nil")
		}

		// Parse the SOCKS5 proxy URL
		proxyURL, err := url.Parse(torClient.ProxyURL())
		if err != nil {
			return nil, fmt.Errorf("failed to parse proxy URL: %w", err)
		}

		// Create SOCKS5 dialer
		dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
		}

		// Use context-aware dialing
		type result struct {
			conn net.Conn
			err  error
		}
		
		ch := make(chan result, 1)
		go func() {
			conn, err := dialer.Dial(network, addr)
			ch <- result{conn, err}
		}()
		
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case res := <-ch:
			return res.conn, res.err
		}
	}
}

// WrapHTTPClient wraps an existing http.Client to use the Tor proxy.
// This is useful when you have an existing client with custom settings
// that you want to route through Tor.
//
// Note: This replaces the client's Transport. If you need to preserve
// custom transport settings, use NewHTTPTransport() instead.
//
// Example:
//
//	existingClient := &http.Client{Timeout: 60 * time.Second}
//	torClient, _ := client.Connect()
//	helpers.WrapHTTPClient(existingClient, torClient, nil)
//	// Now existingClient routes through Tor
func WrapHTTPClient(httpClient *http.Client, torClient TorClient, config *HTTPClientConfig) error {
	if httpClient == nil {
		return fmt.Errorf("httpClient cannot be nil")
	}

	transport, err := NewHTTPTransport(torClient, config)
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	httpClient.Transport = transport
	return nil
}

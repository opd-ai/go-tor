// Package config provides configuration management for the Tor client.
package config

import (
	"fmt"
	"time"

	"github.com/opd-ai/go-tor/pkg/autoconfig"
)

// Config represents the Tor client configuration
type Config struct {
	// Network settings
	SocksPort     int    // SOCKS5 proxy port (default: 9050)
	ControlPort   int    // Control protocol port (default: 9051)
	DataDirectory string // Directory for persistent state

	// Circuit settings
	CircuitBuildTimeout time.Duration // Max time to build a circuit (default: 60s)
	MaxCircuitDirtiness time.Duration // Max time to use a circuit (default: 10m)
	NewCircuitPeriod    time.Duration // How often to rotate circuits (default: 30s)
	NumEntryGuards      int           // Number of entry guards to use (default: 3)

	// Path selection
	UseEntryGuards   bool     // Whether to use entry guards (default: true)
	UseBridges       bool     // Whether to use bridges (default: false)
	BridgeAddresses  []string // Bridge addresses if UseBridges is true
	ExcludeNodes     []string // Nodes to exclude from path selection
	ExcludeExitNodes []string // Exit nodes to exclude

	// Network behavior
	ConnLimit      int           // Max concurrent connections (default: 1000)
	DormantTimeout time.Duration // Time before entering dormant mode (default: 24h)

	// Onion service settings
	OnionServices []OnionServiceConfig

	// Logging
	LogLevel string // Log level: debug, info, warn, error (default: info)

	// Monitoring and observability (Phase 9.1)
	MetricsPort   int  // HTTP metrics server port (default: 0 = disabled)
	EnableMetrics bool // Enable HTTP metrics endpoint (default: false)

	// Performance tuning (Phase 8.3)
	EnableConnectionPooling  bool          // Enable connection pooling for relay connections
	ConnectionPoolMaxIdle    int           // Max idle connections per relay (default: 5)
	ConnectionPoolMaxLife    time.Duration // Max lifetime for pooled connections (default: 10m)
	EnableCircuitPrebuilding bool          // Enable circuit prebuilding
	CircuitPoolMinSize       int           // Minimum circuits to prebuild (default: 2)
	CircuitPoolMaxSize       int           // Maximum circuits in pool (default: 10)
	EnableBufferPooling      bool          // Enable buffer pooling for cell operations (default: true)

	// Circuit isolation (backward compatible - disabled by default)
	IsolationLevel        string // Isolation level: "none", "destination", "credential", "port", "session" (default: "none")
	IsolateDestinations   bool   // Isolate circuits by destination host:port (default: false)
	IsolateSOCKSAuth      bool   // Isolate circuits by SOCKS5 username (default: false)
	IsolateClientPort     bool   // Isolate circuits by client source port (default: false)
	IsolateClientProtocol bool   // Isolate circuits by protocol (default: false)
}

// OnionServiceConfig represents configuration for a single onion service
type OnionServiceConfig struct {
	ServiceDir  string            // Directory for service keys and state
	VirtualPort int               // Virtual port for the onion service
	TargetAddr  string            // Target address (localhost:port)
	MaxStreams  int               // Max concurrent streams (default: 0 = unlimited)
	ClientAuth  map[string]string // Client authorization keys
}

// DefaultConfig returns a configuration with sensible defaults.
// It automatically detects the appropriate data directory for the current platform
// and uses ports that work without special privileges.
func DefaultConfig() *Config {
	// Auto-detect data directory for current platform
	dataDir, err := autoconfig.GetDefaultDataDir()
	if err != nil {
		// Fallback to current directory if auto-detection fails
		dataDir = "./go-tor-data"
	}

	return &Config{
		SocksPort:           autoconfig.FindAvailablePort(9050),
		ControlPort:         autoconfig.FindAvailablePort(9051),
		DataDirectory:       dataDir,
		CircuitBuildTimeout: 60 * time.Second,
		MaxCircuitDirtiness: 10 * time.Minute,
		NewCircuitPeriod:    30 * time.Second,
		NumEntryGuards:      3,
		UseEntryGuards:      true,
		UseBridges:          false,
		BridgeAddresses:     []string{},
		ExcludeNodes:        []string{},
		ExcludeExitNodes:    []string{},
		ConnLimit:           1000,
		DormantTimeout:      24 * time.Hour,
		OnionServices:       []OnionServiceConfig{},
		LogLevel:            "info",
		// Monitoring defaults (Phase 9.1)
		MetricsPort:   0,     // Disabled by default
		EnableMetrics: false, // Disabled by default
		// Performance tuning defaults (Phase 8.3)
		EnableConnectionPooling:  true,
		ConnectionPoolMaxIdle:    5,
		ConnectionPoolMaxLife:    10 * time.Minute,
		EnableCircuitPrebuilding: true,
		CircuitPoolMinSize:       2,
		CircuitPoolMaxSize:       10,
		EnableBufferPooling:      true,
		// Circuit isolation defaults (backward compatible - disabled by default)
		IsolationLevel:        "none",
		IsolateDestinations:   false,
		IsolateSOCKSAuth:      false,
		IsolateClientPort:     false,
		IsolateClientProtocol: false,
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.SocksPort < 0 || c.SocksPort > 65535 {
		return fmt.Errorf("invalid SocksPort: %d", c.SocksPort)
	}
	if c.ControlPort < 0 || c.ControlPort > 65535 {
		return fmt.Errorf("invalid ControlPort: %d", c.ControlPort)
	}
	if c.MetricsPort < 0 || c.MetricsPort > 65535 {
		return fmt.Errorf("invalid MetricsPort: %d", c.MetricsPort)
	}
	
	// Check for port conflicts between enabled services
	// Build a map of used ports to detect conflicts
	usedPorts := make(map[int]string)
	
	// SocksPort is always enabled if non-zero
	if c.SocksPort > 0 {
		usedPorts[c.SocksPort] = "SocksPort"
	}
	
	// ControlPort is always enabled if non-zero
	if c.ControlPort > 0 {
		if existing, exists := usedPorts[c.ControlPort]; exists {
			return fmt.Errorf("port conflict: ControlPort (%d) conflicts with %s", c.ControlPort, existing)
		}
		usedPorts[c.ControlPort] = "ControlPort"
	}
	
	// MetricsPort is enabled when non-zero or when EnableMetrics is true
	if c.MetricsPort > 0 || c.EnableMetrics {
		if c.MetricsPort > 0 {
			if existing, exists := usedPorts[c.MetricsPort]; exists {
				return fmt.Errorf("port conflict: MetricsPort (%d) conflicts with %s", c.MetricsPort, existing)
			}
			usedPorts[c.MetricsPort] = "MetricsPort"
		}
	}
	if c.CircuitBuildTimeout <= 0 {
		return fmt.Errorf("CircuitBuildTimeout must be positive")
	}
	if c.MaxCircuitDirtiness <= 0 {
		return fmt.Errorf("MaxCircuitDirtiness must be positive")
	}
	if c.NumEntryGuards < 1 {
		return fmt.Errorf("NumEntryGuards must be at least 1")
	}
	if c.ConnLimit < 1 {
		return fmt.Errorf("ConnLimit must be at least 1")
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid LogLevel: %s (must be debug, info, warn, or error)", c.LogLevel)
	}

	// Validate onion service configs
	for i, os := range c.OnionServices {
		if os.VirtualPort < 1 || os.VirtualPort > 65535 {
			return fmt.Errorf("onion service %d: invalid VirtualPort: %d", i, os.VirtualPort)
		}
		if os.TargetAddr == "" {
			return fmt.Errorf("onion service %d: TargetAddr is required", i)
		}
		if os.ServiceDir == "" {
			return fmt.Errorf("onion service %d: ServiceDir is required", i)
		}
	}

	// Validate performance tuning settings
	if c.ConnectionPoolMaxIdle < 0 {
		return fmt.Errorf("ConnectionPoolMaxIdle must be non-negative")
	}
	if c.ConnectionPoolMaxLife < 0 {
		return fmt.Errorf("ConnectionPoolMaxLife must be non-negative")
	}
	if c.CircuitPoolMinSize < 0 {
		return fmt.Errorf("CircuitPoolMinSize must be non-negative")
	}
	if c.CircuitPoolMaxSize < c.CircuitPoolMinSize {
		return fmt.Errorf("CircuitPoolMaxSize must be >= CircuitPoolMinSize")
	}

	// Validate circuit isolation settings
	validIsolationLevels := map[string]bool{
		"none":        true,
		"destination": true,
		"credential":  true,
		"port":        true,
		"session":     true,
	}
	if !validIsolationLevels[c.IsolationLevel] {
		return fmt.Errorf("invalid IsolationLevel: %s (must be none, destination, credential, port, or session)", c.IsolationLevel)
	}

	return nil
}

// Clone creates a deep copy of the configuration
func (c *Config) Clone() *Config {
	clone := *c
	clone.BridgeAddresses = append([]string{}, c.BridgeAddresses...)
	clone.ExcludeNodes = append([]string{}, c.ExcludeNodes...)
	clone.ExcludeExitNodes = append([]string{}, c.ExcludeExitNodes...)
	clone.OnionServices = make([]OnionServiceConfig, len(c.OnionServices))
	copy(clone.OnionServices, c.OnionServices)
	return &clone
}

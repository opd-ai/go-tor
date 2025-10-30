// Package config provides configuration management for the Tor client.
package config

import (
	"encoding/json"
	"fmt"
	"time"
)

// JSONSchema represents the JSON Schema v7 for the Tor configuration.
// This enables IDE autocomplete, validation, and documentation.
type JSONSchema struct {
	Schema      string                      `json:"$schema"`
	Title       string                      `json:"title"`
	Description string                      `json:"description"`
	Type        string                      `json:"type"`
	Properties  map[string]PropertySchema   `json:"properties"`
	Required    []string                    `json:"required,omitempty"`
	Definitions map[string]DefinitionSchema `json:"definitions,omitempty"`
}

// PropertySchema represents a property in the JSON schema
type PropertySchema struct {
	Type        string                    `json:"type,omitempty"`
	Description string                    `json:"description,omitempty"`
	Default     interface{}               `json:"default,omitempty"`
	Minimum     *int                      `json:"minimum,omitempty"`
	Maximum     *int                      `json:"maximum,omitempty"`
	Enum        []string                  `json:"enum,omitempty"`
	Items       *PropertySchema           `json:"items,omitempty"`
	Properties  map[string]PropertySchema `json:"properties,omitempty"`
	Ref         string                    `json:"$ref,omitempty"`
	Format      string                    `json:"format,omitempty"`
	Pattern     string                    `json:"pattern,omitempty"`
	MinLength   *int                      `json:"minLength,omitempty"`
	Examples    []interface{}             `json:"examples,omitempty"`
}

// DefinitionSchema represents a reusable definition in the JSON schema
type DefinitionSchema struct {
	Type        string                    `json:"type"`
	Description string                    `json:"description,omitempty"`
	Properties  map[string]PropertySchema `json:"properties,omitempty"`
	Required    []string                  `json:"required,omitempty"`
}

// GenerateJSONSchema creates a JSON Schema v7 for the Config structure.
// This schema can be used for IDE autocomplete, validation, and documentation.
func GenerateJSONSchema() (*JSONSchema, error) {
	minPort := 0
	maxPort := 65535
	minGuards := 1
	minConnLimit := 1
	minPoolSize := 0
	minPortPositive := 1 // For ports that must be > 0
	minStreamCount := 0  // For stream/connection counts

	schema := &JSONSchema{
		Schema:      "http://json-schema.org/draft-07/schema#",
		Title:       "go-tor Configuration",
		Description: "Configuration schema for go-tor Tor client implementation",
		Type:        "object",
		Properties: map[string]PropertySchema{
			"SocksPort": {
				Type:        "integer",
				Description: "SOCKS5 proxy port (0 to disable)",
				Default:     9050,
				Minimum:     &minPort,
				Maximum:     &maxPort,
				Examples:    []interface{}{9050, 9150},
			},
			"ControlPort": {
				Type:        "integer",
				Description: "Control protocol port (0 to disable)",
				Default:     9051,
				Minimum:     &minPort,
				Maximum:     &maxPort,
				Examples:    []interface{}{9051, 9151},
			},
			"DataDirectory": {
				Type:        "string",
				Description: "Directory for persistent state (guards, descriptors, keys)",
				Examples:    []interface{}{"./go-tor-data", "~/.tor", "/var/lib/tor"},
			},
			"CircuitBuildTimeout": {
				Type:        "string",
				Description: "Maximum time to build a circuit (duration string, e.g., '60s', '2m')",
				Default:     "60s",
				Pattern:     "^[0-9]+(ns|us|µs|ms|s|m|h)$",
				Examples:    []interface{}{"60s", "90s", "2m"},
			},
			"MaxCircuitDirtiness": {
				Type:        "string",
				Description: "Maximum time to use a circuit before rotation (duration string)",
				Default:     "10m",
				Pattern:     "^[0-9]+(ns|us|µs|ms|s|m|h)$",
				Examples:    []interface{}{"10m", "30m", "1h"},
			},
			"NewCircuitPeriod": {
				Type:        "string",
				Description: "How often to rotate circuits (duration string)",
				Default:     "30s",
				Pattern:     "^[0-9]+(ns|us|µs|ms|s|m|h)$",
				Examples:    []interface{}{"30s", "1m", "5m"},
			},
			"NumEntryGuards": {
				Type:        "integer",
				Description: "Number of entry guards to use (recommended: 3)",
				Default:     3,
				Minimum:     &minGuards,
				Examples:    []interface{}{3, 5},
			},
			"UseEntryGuards": {
				Type:        "boolean",
				Description: "Whether to use entry guards (recommended: true for anonymity)",
				Default:     true,
			},
			"UseBridges": {
				Type:        "boolean",
				Description: "Whether to use bridge relays (for censored networks)",
				Default:     false,
			},
			"BridgeAddresses": {
				Type:        "array",
				Description: "Bridge addresses if UseBridges is true (format: IP:PORT or transport IP:PORT)",
				Items: &PropertySchema{
					Type:    "string",
					Pattern: "^([a-zA-Z0-9]+\\s+)?([0-9]{1,3}\\.){3}[0-9]{1,3}:[0-9]{1,5}$",
				},
				Examples: []interface{}{
					[]string{"obfs4 192.0.2.1:443"},
					[]string{"192.0.2.2:9001", "192.0.2.3:9001"},
				},
			},
			"ExcludeNodes": {
				Type:        "array",
				Description: "Nodes to exclude from path selection (by fingerprint or nickname)",
				Items: &PropertySchema{
					Type: "string",
				},
				Examples: []interface{}{
					[]string{"$FINGERPRINT", "NickName"},
				},
			},
			"ExcludeExitNodes": {
				Type:        "array",
				Description: "Exit nodes to exclude (by fingerprint or nickname)",
				Items: &PropertySchema{
					Type: "string",
				},
			},
			"ConnLimit": {
				Type:        "integer",
				Description: "Maximum concurrent connections to Tor relays",
				Default:     1000,
				Minimum:     &minConnLimit,
				Examples:    []interface{}{1000, 500, 2000},
			},
			"DormantTimeout": {
				Type:        "string",
				Description: "Time before entering dormant mode (duration string)",
				Default:     "24h",
				Pattern:     "^[0-9]+(ns|us|µs|ms|s|m|h)$",
				Examples:    []interface{}{"24h", "12h", "48h"},
			},
			"OnionServices": {
				Type:        "array",
				Description: "Onion service configurations (hidden services)",
				Items: &PropertySchema{
					Ref: "#/definitions/OnionServiceConfig",
				},
			},
			"LogLevel": {
				Type:        "string",
				Description: "Logging verbosity level",
				Default:     "info",
				Enum:        []string{"debug", "info", "warn", "error"},
			},
			"MetricsPort": {
				Type:        "integer",
				Description: "HTTP metrics server port (0 to disable, non-zero to enable)",
				Default:     0,
				Minimum:     &minPort,
				Maximum:     &maxPort,
				Examples:    []interface{}{9052, 0},
			},
			"EnableMetrics": {
				Type:        "boolean",
				Description: "Enable HTTP metrics endpoint (Prometheus, JSON, HTML dashboard)",
				Default:     false,
			},
			"EnableConnectionPooling": {
				Type:        "boolean",
				Description: "Enable connection pooling for relay connections",
				Default:     true,
			},
			"ConnectionPoolMaxIdle": {
				Type:        "integer",
				Description: "Maximum idle connections per relay in pool",
				Default:     5,
				Minimum:     &minPoolSize,
				Examples:    []interface{}{5, 10, 20},
			},
			"ConnectionPoolMaxLife": {
				Type:        "string",
				Description: "Maximum lifetime for pooled connections (duration string)",
				Default:     "10m",
				Pattern:     "^[0-9]+(ns|us|µs|ms|s|m|h)$",
				Examples:    []interface{}{"10m", "30m", "1h"},
			},
			"EnableCircuitPrebuilding": {
				Type:        "boolean",
				Description: "Enable circuit prebuilding for instant availability",
				Default:     true,
			},
			"CircuitPoolMinSize": {
				Type:        "integer",
				Description: "Minimum circuits to prebuild and maintain",
				Default:     2,
				Minimum:     &minPoolSize,
				Examples:    []interface{}{2, 5, 10},
			},
			"CircuitPoolMaxSize": {
				Type:        "integer",
				Description: "Maximum circuits in pool",
				Default:     10,
				Minimum:     &minPoolSize,
				Examples:    []interface{}{10, 20, 50},
			},
			"EnableBufferPooling": {
				Type:        "boolean",
				Description: "Enable buffer pooling for cell operations (reduces GC pressure)",
				Default:     true,
			},
			"IsolationLevel": {
				Type:        "string",
				Description: "Circuit isolation level (none=shared circuits, destination=per-dest, etc.)",
				Default:     "none",
				Enum:        []string{"none", "destination", "credential", "port", "session"},
			},
			"IsolateDestinations": {
				Type:        "boolean",
				Description: "Isolate circuits by destination host:port",
				Default:     false,
			},
			"IsolateSOCKSAuth": {
				Type:        "boolean",
				Description: "Isolate circuits by SOCKS5 username",
				Default:     false,
			},
			"IsolateClientPort": {
				Type:        "boolean",
				Description: "Isolate circuits by client source port",
				Default:     false,
			},
			"IsolateClientProtocol": {
				Type:        "boolean",
				Description: "Isolate circuits by protocol",
				Default:     false,
			},
		},
		Definitions: map[string]DefinitionSchema{
			"OnionServiceConfig": {
				Type:        "object",
				Description: "Configuration for a single onion service (hidden service)",
				Properties: map[string]PropertySchema{
					"ServiceDir": {
						Type:        "string",
						Description: "Directory for service keys and state",
						Examples:    []interface{}{"./hidden_service", "/var/lib/tor/hidden_service"},
					},
					"VirtualPort": {
						Type:        "integer",
						Description: "Virtual port for the onion service (advertised port)",
						Minimum:     &minPortPositive, // Port minimum is 1 (must be valid port)
						Maximum:     &maxPort,
						Examples:    []interface{}{80, 443, 8080},
					},
					"TargetAddr": {
						Type:        "string",
						Description: "Target address (localhost:port where service is running)",
						Pattern:     "^[a-zA-Z0-9.-]+:[0-9]{1,5}$",
						Examples:    []interface{}{"localhost:8080", "127.0.0.1:80"},
					},
					"MaxStreams": {
						Type:        "integer",
						Description: "Maximum concurrent streams (0 = unlimited)",
						Default:     0,
						Minimum:     &minStreamCount, // Zero or positive stream count
					},
					"ClientAuth": {
						Type:        "object",
						Description: "Client authorization keys (client_name: public_key)",
					},
				},
				Required: []string{"ServiceDir", "VirtualPort", "TargetAddr"},
			},
		},
	}

	return schema, nil
}

// ToJSON converts the schema to JSON format
func (s *JSONSchema) ToJSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

// ValidationError represents a configuration validation error with context
type ValidationError struct {
	Field      string      // Field name that failed validation
	Value      interface{} // Actual value provided
	Message    string      // Human-readable error message
	Suggestion string      // Suggested fix
	Severity   string      // "error", "warning", "info"
}

// Error implements the error interface
func (v *ValidationError) Error() string {
	if v.Suggestion != "" {
		return fmt.Sprintf("%s: %s (suggestion: %s)", v.Field, v.Message, v.Suggestion)
	}
	return fmt.Sprintf("%s: %s", v.Field, v.Message)
}

// ValidationResult contains the results of configuration validation
type ValidationResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []ValidationError
}

// ValidateDetailed performs comprehensive validation with detailed feedback
func (c *Config) ValidateDetailed() *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationError{},
	}

	// Port validation with detailed messages
	if c.SocksPort < 0 || c.SocksPort > 65535 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:      "SocksPort",
			Value:      c.SocksPort,
			Message:    fmt.Sprintf("invalid port number: %d", c.SocksPort),
			Suggestion: "use a port between 0 and 65535 (0 to disable, 1024-65535 recommended for non-root)",
			Severity:   "error",
		})
	} else if c.SocksPort > 0 && c.SocksPort < 1024 {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:      "SocksPort",
			Value:      c.SocksPort,
			Message:    "using privileged port (< 1024)",
			Suggestion: "consider using port >= 1024 to avoid requiring root privileges",
			Severity:   "warning",
		})
	}

	if c.ControlPort < 0 || c.ControlPort > 65535 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:      "ControlPort",
			Value:      c.ControlPort,
			Message:    fmt.Sprintf("invalid port number: %d", c.ControlPort),
			Suggestion: "use a port between 0 and 65535 (0 to disable)",
			Severity:   "error",
		})
	}

	if c.MetricsPort < 0 || c.MetricsPort > 65535 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:      "MetricsPort",
			Value:      c.MetricsPort,
			Message:    fmt.Sprintf("invalid port number: %d", c.MetricsPort),
			Suggestion: "use a port between 0 and 65535 (0 to disable metrics)",
			Severity:   "error",
		})
	}

	// Check for port conflicts
	ports := make(map[int]string)
	if c.SocksPort > 0 {
		ports[c.SocksPort] = "SocksPort"
	}
	if c.ControlPort > 0 {
		if existing, exists := ports[c.ControlPort]; exists {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:      "ControlPort",
				Value:      c.ControlPort,
				Message:    fmt.Sprintf("port conflict with %s", existing),
				Suggestion: fmt.Sprintf("choose a different port (currently conflicts with %s on port %d)", existing, c.ControlPort),
				Severity:   "error",
			})
		}
		ports[c.ControlPort] = "ControlPort"
	}
	if c.MetricsPort > 0 || c.EnableMetrics {
		if c.MetricsPort > 0 {
			if existing, exists := ports[c.MetricsPort]; exists {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:      "MetricsPort",
					Value:      c.MetricsPort,
					Message:    fmt.Sprintf("port conflict with %s", existing),
					Suggestion: fmt.Sprintf("choose a different port (currently conflicts with %s on port %d)", existing, c.MetricsPort),
					Severity:   "error",
				})
			}
		}
	}

	// Timeout validation
	if c.CircuitBuildTimeout <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:      "CircuitBuildTimeout",
			Value:      c.CircuitBuildTimeout,
			Message:    "must be positive",
			Suggestion: "recommended: 60s to 120s for normal networks, 180s for slow networks",
			Severity:   "error",
		})
	} else if c.CircuitBuildTimeout < 30*time.Second {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:      "CircuitBuildTimeout",
			Value:      c.CircuitBuildTimeout,
			Message:    "unusually short timeout may cause circuit build failures",
			Suggestion: "recommended minimum: 30s",
			Severity:   "warning",
		})
	}

	if c.MaxCircuitDirtiness <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:      "MaxCircuitDirtiness",
			Value:      c.MaxCircuitDirtiness,
			Message:    "must be positive",
			Suggestion: "recommended: 10m to 30m for privacy/performance balance",
			Severity:   "error",
		})
	}

	// Guard validation
	if c.NumEntryGuards < 1 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:      "NumEntryGuards",
			Value:      c.NumEntryGuards,
			Message:    "must be at least 1",
			Suggestion: "recommended: 3 guards for security/availability balance",
			Severity:   "error",
		})
	} else if c.NumEntryGuards > 5 {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:      "NumEntryGuards",
			Value:      c.NumEntryGuards,
			Message:    "large number of guards may reduce anonymity",
			Suggestion: "recommended: 3-5 guards",
			Severity:   "warning",
		})
	}

	// Connection limit validation
	if c.ConnLimit < 1 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:      "ConnLimit",
			Value:      c.ConnLimit,
			Message:    "must be at least 1",
			Suggestion: "recommended: 1000 for normal usage, adjust based on available file descriptors",
			Severity:   "error",
		})
	}

	// Log level validation
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.LogLevel] {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:      "LogLevel",
			Value:      c.LogLevel,
			Message:    "invalid log level",
			Suggestion: "must be one of: debug, info, warn, error",
			Severity:   "error",
		})
	}

	// Onion service validation
	for i, os := range c.OnionServices {
		if os.VirtualPort < 1 || os.VirtualPort > 65535 {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:      fmt.Sprintf("OnionServices[%d].VirtualPort", i),
				Value:      os.VirtualPort,
				Message:    fmt.Sprintf("invalid port: %d", os.VirtualPort),
				Suggestion: "use a port between 1 and 65535",
				Severity:   "error",
			})
		}
		if os.TargetAddr == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:      fmt.Sprintf("OnionServices[%d].TargetAddr", i),
				Value:      os.TargetAddr,
				Message:    "target address is required",
				Suggestion: "specify target as 'host:port' (e.g., 'localhost:8080')",
				Severity:   "error",
			})
		}
		if os.ServiceDir == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:      fmt.Sprintf("OnionServices[%d].ServiceDir", i),
				Value:      os.ServiceDir,
				Message:    "service directory is required",
				Suggestion: "specify a directory to store service keys and state",
				Severity:   "error",
			})
		}
	}

	// Performance tuning validation
	if c.ConnectionPoolMaxIdle < 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:      "ConnectionPoolMaxIdle",
			Value:      c.ConnectionPoolMaxIdle,
			Message:    "must be non-negative",
			Suggestion: "recommended: 5-20 depending on traffic patterns",
			Severity:   "error",
		})
	}

	if c.ConnectionPoolMaxLife < 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:      "ConnectionPoolMaxLife",
			Value:      c.ConnectionPoolMaxLife,
			Message:    "must be non-negative",
			Suggestion: "recommended: 10m to 1h",
			Severity:   "error",
		})
	}

	if c.CircuitPoolMinSize < 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:      "CircuitPoolMinSize",
			Value:      c.CircuitPoolMinSize,
			Message:    "must be non-negative",
			Suggestion: "recommended: 2-5 for instant circuit availability",
			Severity:   "error",
		})
	}

	if c.CircuitPoolMaxSize < c.CircuitPoolMinSize {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:      "CircuitPoolMaxSize",
			Value:      c.CircuitPoolMaxSize,
			Message:    "must be >= CircuitPoolMinSize",
			Suggestion: fmt.Sprintf("set to at least %d (current CircuitPoolMinSize)", c.CircuitPoolMinSize),
			Severity:   "error",
		})
	}

	// Isolation level validation
	validIsolationLevels := map[string]bool{
		"none":        true,
		"destination": true,
		"credential":  true,
		"port":        true,
		"session":     true,
	}
	if !validIsolationLevels[c.IsolationLevel] {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:      "IsolationLevel",
			Value:      c.IsolationLevel,
			Message:    "invalid isolation level",
			Suggestion: "must be one of: none, destination, credential, port, session",
			Severity:   "error",
		})
	}

	return result
}

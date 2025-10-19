// Package config provides configuration file loading for torrc-compatible files.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// LoadFromFile loads configuration from a torrc-compatible file.
// It parses the file line by line and updates the provided config.
// Lines starting with # are treated as comments and ignored.
// Empty lines are ignored.
// Each configuration line follows the format: Key Value
func LoadFromFile(path string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Validate path to prevent directory traversal attacks
	if err := validatePath(path); err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}

	file, err := os.Open(path) // #nosec G304 - path is validated by validatePath
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse key-value pair
		parts := strings.Fields(line)
		if len(parts) < 1 {
			continue
		}

		key := parts[0]
		value := ""
		if len(parts) > 1 {
			value = strings.Join(parts[1:], " ")
		}

		// Process configuration option
		if err := processConfigOption(cfg, key, value); err != nil {
			return fmt.Errorf("line %d: %w", lineNum, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	// Validate the loaded configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	return nil
}

// processConfigOption processes a single configuration option
func processConfigOption(cfg *Config, key, value string) error {
	switch key {
	case "SocksPort":
		port, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid SocksPort value: %s", value)
		}
		cfg.SocksPort = port

	case "ControlPort":
		port, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid ControlPort value: %s", value)
		}
		cfg.ControlPort = port

	case "DataDirectory":
		cfg.DataDirectory = value

	case "CircuitBuildTimeout":
		timeout, err := parseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid CircuitBuildTimeout: %w", err)
		}
		cfg.CircuitBuildTimeout = timeout

	case "MaxCircuitDirtiness":
		duration, err := parseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid MaxCircuitDirtiness: %w", err)
		}
		cfg.MaxCircuitDirtiness = duration

	case "NewCircuitPeriod":
		period, err := parseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid NewCircuitPeriod: %w", err)
		}
		cfg.NewCircuitPeriod = period

	case "NumEntryGuards":
		num, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid NumEntryGuards value: %s", value)
		}
		cfg.NumEntryGuards = num

	case "UseEntryGuards":
		cfg.UseEntryGuards = parseBool(value)

	case "UseBridges":
		cfg.UseBridges = parseBool(value)

	case "Bridge":
		cfg.BridgeAddresses = append(cfg.BridgeAddresses, value)

	case "ExcludeNodes":
		cfg.ExcludeNodes = append(cfg.ExcludeNodes, value)

	case "ExcludeExitNodes":
		cfg.ExcludeExitNodes = append(cfg.ExcludeExitNodes, value)

	case "ConnLimit":
		limit, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid ConnLimit value: %s", value)
		}
		cfg.ConnLimit = limit

	case "DormantTimeout":
		timeout, err := parseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid DormantTimeout: %w", err)
		}
		cfg.DormantTimeout = timeout

	case "LogLevel":
		cfg.LogLevel = strings.ToLower(value)

	// Ignore unknown options for compatibility with standard torrc files
	default:
		// Silently ignore unknown options for forward compatibility
	}

	return nil
}

// parseDuration parses a duration string with support for common time units.
// Supports: seconds (s), minutes (m), hours (h), days (d)
// Examples: "60s", "5m", "2h", "1d"
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	// Try parsing as Go duration first
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	// Check if it ends with a known suffix
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration format: %s", s)
	}

	suffix := s[len(s)-1:]
	valueStr := s[:len(s)-1]

	value, err := strconv.ParseInt(valueStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration value: %s", s)
	}

	switch suffix {
	case "s", "S":
		return time.Duration(value) * time.Second, nil
	case "m", "M":
		return time.Duration(value) * time.Minute, nil
	case "h", "H":
		return time.Duration(value) * time.Hour, nil
	case "d", "D":
		return time.Duration(value) * 24 * time.Hour, nil
	default:
		// Try parsing as seconds without suffix
		val, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration format: %s", s)
		}
		return time.Duration(val) * time.Second, nil
	}
}

// parseBool parses a boolean value from various string formats.
// Accepts: 1/0, true/false, yes/no, on/off (case-insensitive)
func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return false
	}
}

// validatePath validates a file path to prevent directory traversal attacks.
// It ensures the path doesn't contain ".." components and is an absolute or safe relative path.
func validatePath(path string) error {
	// Clean the path to normalize it
	cleanPath := filepath.Clean(path)

	// Check for directory traversal attempts
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("invalid path: directory traversal detected")
	}

	// Additional check: ensure the clean path doesn't escape the intended directory
	// by checking if it becomes absolute when it shouldn't be
	if !filepath.IsAbs(path) && filepath.IsAbs(cleanPath) {
		return fmt.Errorf("invalid path: attempts to escape working directory")
	}

	return nil
}

// SaveToFile saves the configuration to a torrc-compatible file.
// This creates a human-readable configuration file that can be loaded later.
func SaveToFile(path string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Validate path to prevent directory traversal attacks
	if err := validatePath(path); err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}

	file, err := os.Create(path) // #nosec G304 - path is validated by validatePath
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// Write header comment
	fmt.Fprintf(writer, "# go-tor configuration file\n")
	fmt.Fprintf(writer, "# Generated automatically - edit with care\n\n")

	// Network settings
	fmt.Fprintf(writer, "# Network Settings\n")
	fmt.Fprintf(writer, "SocksPort %d\n", cfg.SocksPort)
	fmt.Fprintf(writer, "ControlPort %d\n", cfg.ControlPort)
	fmt.Fprintf(writer, "DataDirectory %s\n\n", cfg.DataDirectory)

	// Circuit settings
	fmt.Fprintf(writer, "# Circuit Settings\n")
	fmt.Fprintf(writer, "CircuitBuildTimeout %s\n", formatDuration(cfg.CircuitBuildTimeout))
	fmt.Fprintf(writer, "MaxCircuitDirtiness %s\n", formatDuration(cfg.MaxCircuitDirtiness))
	fmt.Fprintf(writer, "NewCircuitPeriod %s\n", formatDuration(cfg.NewCircuitPeriod))
	fmt.Fprintf(writer, "NumEntryGuards %d\n\n", cfg.NumEntryGuards)

	// Path selection
	fmt.Fprintf(writer, "# Path Selection\n")
	fmt.Fprintf(writer, "UseEntryGuards %s\n", formatBool(cfg.UseEntryGuards))
	fmt.Fprintf(writer, "UseBridges %s\n", formatBool(cfg.UseBridges))
	for _, bridge := range cfg.BridgeAddresses {
		fmt.Fprintf(writer, "Bridge %s\n", bridge)
	}
	for _, node := range cfg.ExcludeNodes {
		fmt.Fprintf(writer, "ExcludeNodes %s\n", node)
	}
	for _, node := range cfg.ExcludeExitNodes {
		fmt.Fprintf(writer, "ExcludeExitNodes %s\n", node)
	}
	fmt.Fprintf(writer, "\n")

	// Network behavior
	fmt.Fprintf(writer, "# Network Behavior\n")
	fmt.Fprintf(writer, "ConnLimit %d\n", cfg.ConnLimit)
	fmt.Fprintf(writer, "DormantTimeout %s\n\n", formatDuration(cfg.DormantTimeout))

	// Logging
	fmt.Fprintf(writer, "# Logging\n")
	fmt.Fprintf(writer, "LogLevel %s\n", cfg.LogLevel)

	return writer.Flush()
}

// formatDuration formats a duration for writing to config file
func formatDuration(d time.Duration) string {
	if d%(24*time.Hour) == 0 && d >= 24*time.Hour {
		return fmt.Sprintf("%dd", d/(24*time.Hour))
	}
	if d%time.Hour == 0 && d >= time.Hour {
		return fmt.Sprintf("%dh", d/time.Hour)
	}
	if d%time.Minute == 0 && d >= time.Minute {
		return fmt.Sprintf("%dm", d/time.Minute)
	}
	return fmt.Sprintf("%ds", d/time.Second)
}

// formatBool formats a boolean for writing to config file
func formatBool(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

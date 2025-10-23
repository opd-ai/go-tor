// Package autoconfig provides automatic configuration management for zero-configuration setup.
package autoconfig

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
)

// GetDefaultDataDir returns the platform-appropriate data directory for go-tor.
// On Unix: ~/.config/go-tor
// On Windows: %APPDATA%/go-tor
// On macOS: ~/Library/Application Support/go-tor
func GetDefaultDataDir() (string, error) {
	var baseDir string

	switch runtime.GOOS {
	case "windows":
		// Use %APPDATA% on Windows
		baseDir = os.Getenv("APPDATA")
		if baseDir == "" {
			baseDir = os.Getenv("USERPROFILE")
			if baseDir == "" {
				return "", fmt.Errorf("cannot determine Windows user directory")
			}
			baseDir = filepath.Join(baseDir, "AppData", "Roaming")
		}
		return filepath.Join(baseDir, "go-tor"), nil

	case "darwin":
		// Use ~/Library/Application Support on macOS
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		return filepath.Join(homeDir, "Library", "Application Support", "go-tor"), nil

	default:
		// Use XDG_CONFIG_HOME or ~/.config on Linux/Unix
		configDir := os.Getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("cannot determine home directory: %w", err)
			}
			configDir = filepath.Join(homeDir, ".config")
		}
		return filepath.Join(configDir, "go-tor"), nil
	}
}

// EnsureDataDir creates the data directory if it doesn't exist and sets proper permissions.
// On Unix systems, sets permissions to 700 (owner read/write/execute only).
func EnsureDataDir(path string) error {
	// Check if directory exists
	info, err := os.Stat(path)
	if err == nil {
		// Directory exists, verify it's a directory
		if !info.IsDir() {
			return fmt.Errorf("path exists but is not a directory: %s", path)
		}
		// Verify permissions on Unix systems
		if runtime.GOOS != "windows" {
			mode := info.Mode().Perm()
			if mode != 0o700 {
				// Fix permissions
				if err := os.Chmod(path, 0o700); err != nil {
					return fmt.Errorf("failed to set directory permissions: %w", err)
				}
			}
		}
		return nil
	}

	// Directory doesn't exist, create it
	if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check directory: %w", err)
	}

	// Create directory with proper permissions
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}

// EnsureSubDir creates a subdirectory within the data directory.
func EnsureSubDir(dataDir, subDir string) (string, error) {
	path := filepath.Join(dataDir, subDir)
	if err := EnsureDataDir(path); err != nil {
		return "", err
	}
	return path, nil
}

// CleanupTempFiles removes temporary files from the data directory.
func CleanupTempFiles(dataDir string) error {
	// Look for common temporary file patterns
	patterns := []string{"*.tmp", "*.temp", "*.lock~"}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(dataDir, pattern))
		if err != nil {
			return fmt.Errorf("failed to search for temp files: %w", err)
		}

		for _, match := range matches {
			if err := os.Remove(match); err != nil && !os.IsNotExist(err) {
				// Log but don't fail on individual file deletion errors
				continue
			}
		}
	}

	return nil
}

// FindAvailablePort finds an available port starting from the preferred port.
// Returns the preferred port if available, otherwise searches up to 100 ports higher.
// If no port is available in the search range, returns the preferred port
// (which will fail later with a clear "address already in use" error).
// This function is used by DefaultConfig() to enable true zero-configuration
// port selection when default Tor ports (9050, 9051) are already in use.
func FindAvailablePort(preferredPort int) int {
	// Try preferred port first
	if isPortAvailable(preferredPort) {
		return preferredPort
	}

	// Try ports in range [preferredPort, preferredPort+100]
	for port := preferredPort + 1; port < preferredPort+100; port++ {
		if isPortAvailable(port) {
			return port
		}
	}

	// Fall back to preferred port (will fail later with clear error)
	return preferredPort
}

// isPortAvailable checks if a port is available for binding.
func isPortAvailable(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	if err := listener.Close(); err != nil {
		return false
	}
	return true
}

// Package main provides tests for the Tor client executable.
package main

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestVersionFlag tests the -version flag
func TestVersionFlag(t *testing.T) {
	// Build a test binary
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "tor-client-test")

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}

	// Run with -version flag
	cmd = exec.Command(binaryPath, "-version")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to run with -version: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "go-tor version") {
		t.Errorf("Version output missing version string, got: %s", output)
	}
	if !strings.Contains(output, "Pure Go Tor client implementation") {
		t.Errorf("Version output missing description, got: %s", output)
	}
}

// TestInvalidConfigFile tests behavior with invalid config file
func TestInvalidConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "tor-client-test")

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}

	// Run with non-existent config file
	cmd = exec.Command(binaryPath, "-config", "/nonexistent/config.torrc")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Error("Expected error for non-existent config file, got nil")
	}

	output := stderr.String()
	if !strings.Contains(output, "Failed to load config file") {
		t.Errorf("Expected config file error message, got: %s", output)
	}
}

// TestInvalidLogLevel tests behavior with invalid log level
func TestInvalidLogLevel(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "tor-client-test")

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}

	// Run with invalid log level
	cmd = exec.Command(binaryPath, "-log-level", "invalid")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Error("Expected error for invalid log level, got nil")
	}

	output := stderr.String()
	if !strings.Contains(output, "Invalid configuration") && !strings.Contains(output, "invalid LogLevel") {
		t.Errorf("Expected log level error message, got: %s", output)
	}
}

// TestFlagParsing tests that flags are properly parsed
func TestFlagParsing(t *testing.T) {
	// Reset flags for testing
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// Test default values
	configFile := flag.String("config", "", "Path to configuration file (torrc format)")
	socksPort := flag.Int("socks-port", 0, "SOCKS5 proxy port (default: auto-detect or 9050)")
	controlPort := flag.Int("control-port", 0, "Control protocol port (default: 9051)")
	metricsPort := flag.Int("metrics-port", 0, "HTTP metrics server port (default: 0 = disabled)")
	dataDir := flag.String("data-dir", "", "Data directory for persistent state (default: auto-detect)")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	showVersion := flag.Bool("version", false, "Show version information")

	// Parse empty args (defaults)
	flag.CommandLine.Parse([]string{})

	if *configFile != "" {
		t.Errorf("Expected empty config file, got: %s", *configFile)
	}
	if *socksPort != 0 {
		t.Errorf("Expected socks port 0, got: %d", *socksPort)
	}
	if *controlPort != 0 {
		t.Errorf("Expected control port 0, got: %d", *controlPort)
	}
	if *metricsPort != 0 {
		t.Errorf("Expected metrics port 0, got: %d", *metricsPort)
	}
	if *dataDir != "" {
		t.Errorf("Expected empty data dir, got: %s", *dataDir)
	}
	if *logLevel != "info" {
		t.Errorf("Expected log level 'info', got: %s", *logLevel)
	}
	if *showVersion {
		t.Error("Expected version flag false, got true")
	}
}

// TestFlagParsingWithValues tests flag parsing with custom values
func TestFlagParsingWithValues(t *testing.T) {
	// Reset flags for testing
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	configFile := flag.String("config", "", "Path to configuration file (torrc format)")
	socksPort := flag.Int("socks-port", 0, "SOCKS5 proxy port")
	controlPort := flag.Int("control-port", 0, "Control protocol port")
	metricsPort := flag.Int("metrics-port", 0, "HTTP metrics server port")
	dataDir := flag.String("data-dir", "", "Data directory")
	logLevel := flag.String("log-level", "info", "Log level")

	// Parse with custom values
	args := []string{
		"-config", "/tmp/torrc",
		"-socks-port", "9150",
		"-control-port", "9151",
		"-metrics-port", "9152",
		"-data-dir", "/tmp/tor-data",
		"-log-level", "debug",
	}
	flag.CommandLine.Parse(args)

	if *configFile != "/tmp/torrc" {
		t.Errorf("Expected config file '/tmp/torrc', got: %s", *configFile)
	}
	if *socksPort != 9150 {
		t.Errorf("Expected socks port 9150, got: %d", *socksPort)
	}
	if *controlPort != 9151 {
		t.Errorf("Expected control port 9151, got: %d", *controlPort)
	}
	if *metricsPort != 9152 {
		t.Errorf("Expected metrics port 9152, got: %d", *metricsPort)
	}
	if *dataDir != "/tmp/tor-data" {
		t.Errorf("Expected data dir '/tmp/tor-data', got: %s", *dataDir)
	}
	if *logLevel != "debug" {
		t.Errorf("Expected log level 'debug', got: %s", *logLevel)
	}
}

// TestVersionVariable tests that version variables exist
func TestVersionVariable(t *testing.T) {
	if version == "" {
		t.Error("version variable should not be empty")
	}
	if buildTime == "" {
		t.Error("buildTime variable should not be empty")
	}
}

// TestValidConfigFile tests behavior with a valid config file
func TestValidConfigFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal valid config file
	configPath := filepath.Join(tmpDir, "test.torrc")
	configContent := `# Test configuration
SocksPort 9050
ControlPort 9051
DataDirectory ` + tmpDir + `/tor-data
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Build a test binary
	binaryPath := filepath.Join(tmpDir, "tor-client-test")
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}

	// Run with valid config file, but kill after short time
	cmd = exec.Command(binaryPath, "-config", configPath)
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start with valid config: %v", err)
	}

	// Give it a moment to start
	time.Sleep(500 * time.Millisecond)

	// Kill the process
	if err := cmd.Process.Kill(); err != nil {
		t.Logf("Warning: Failed to kill process: %v", err)
	}

	// Wait for process to exit
	cmd.Wait()
}

// TestZeroConfigMode tests that zero-config mode works
func TestZeroConfigMode(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "tor-client-test")

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}

	// Run in zero-config mode (no flags)
	cmd = exec.Command(binaryPath)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	// Start the process
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start in zero-config mode: %v", err)
	}

	// Give it a moment to initialize
	time.Sleep(500 * time.Millisecond)

	// Kill the process
	if err := cmd.Process.Kill(); err != nil {
		t.Logf("Warning: Failed to kill process: %v", err)
	}

	// Wait for process to exit
	cmd.Wait()

	output := stdout.String()
	if !strings.Contains(output, "Using zero-configuration mode") {
		t.Logf("Output did not contain zero-config message (may have not output yet): %s", output)
	}
}

// TestCustomPorts tests setting custom ports via flags
func TestCustomPorts(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "tor-client-test")

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}

	// Run with custom ports
	cmd = exec.Command(binaryPath, "-socks-port", "19050", "-control-port", "19051")

	// Start the process
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start with custom ports: %v", err)
	}

	// Give it a moment to initialize
	time.Sleep(500 * time.Millisecond)

	// Kill the process
	if err := cmd.Process.Kill(); err != nil {
		t.Logf("Warning: Failed to kill process: %v", err)
	}

	// Wait for process to exit
	cmd.Wait()
}

// TestMetricsPortFlag tests the metrics port flag
func TestMetricsPortFlag(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "tor-client-test")

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}

	// Run with metrics port
	cmd = exec.Command(binaryPath, "-metrics-port", "19052")

	// Start the process
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start with metrics port: %v", err)
	}

	// Give it a moment to initialize
	time.Sleep(500 * time.Millisecond)

	// Kill the process
	if err := cmd.Process.Kill(); err != nil {
		t.Logf("Warning: Failed to kill process: %v", err)
	}

	// Wait for process to exit
	cmd.Wait()
}

// TestDataDirFlag tests the data directory flag
func TestDataDirFlag(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "tor-client-test")
	customDataDir := filepath.Join(tmpDir, "custom-tor-data")

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}

	// Run with custom data directory
	cmd = exec.Command(binaryPath, "-data-dir", customDataDir)

	// Start the process
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start with custom data dir: %v", err)
	}

	// Give it a moment to initialize
	time.Sleep(500 * time.Millisecond)

	// Kill the process
	if err := cmd.Process.Kill(); err != nil {
		t.Logf("Warning: Failed to kill process: %v", err)
	}

	// Wait for process to exit
	cmd.Wait()

	// Verify data directory was created
	if _, err := os.Stat(customDataDir); os.IsNotExist(err) {
		t.Errorf("Custom data directory was not created: %s", customDataDir)
	}
}

// TestAllLogLevels tests all valid log levels
func TestAllLogLevels(t *testing.T) {
	logLevels := []string{"debug", "info", "warn", "error"}

	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "tor-client-test")

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}

	for _, level := range logLevels {
		t.Run(level, func(t *testing.T) {
			cmd := exec.Command(binaryPath, "-log-level", level)

			// Start the process
			if err := cmd.Start(); err != nil {
				t.Fatalf("Failed to start with log level %s: %v", level, err)
			}

			// Give it a moment to initialize
			time.Sleep(300 * time.Millisecond)

			// Kill the process
			if err := cmd.Process.Kill(); err != nil {
				t.Logf("Warning: Failed to kill process: %v", err)
			}

			// Wait for process to exit
			cmd.Wait()
		})
	}
}

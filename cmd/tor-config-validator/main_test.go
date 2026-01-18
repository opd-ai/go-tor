// Package main provides tests for the tor-config-validator executable.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildTestBinary builds the validator binary for testing
func buildTestBinary(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "tor-config-validator-test")

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	return binaryPath
}

// createTempConfig creates a temporary config file with the given content
func createTempConfig(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.torrc")
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create temp config: %v", err)
	}
	return configPath
}

// TestVersionFlag tests the -version flag
func TestVersionFlag(t *testing.T) {
	binaryPath := buildTestBinary(t)

	cmd := exec.Command(binaryPath, "-version")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to run with -version: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "tor-config-validator") {
		t.Errorf("Version output missing validator name, got: %s", output)
	}
}

// TestStrictModeWithWarnings tests that -strict flag fails validation when warnings exist
func TestStrictModeWithWarnings(t *testing.T) {
	binaryPath := buildTestBinary(t)

	// Create a config with a privileged port (< 1024) which generates a warning
	configContent := `# Test config with privileged port
SocksPort 80
ControlPort 9051
LogLevel info
`
	configPath := createTempConfig(t, configContent)

	// Run in strict mode - should fail due to warning
	cmd := exec.Command(binaryPath, "-config", configPath, "-strict")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Error("Expected strict mode to fail with warnings, but it passed")
	}

	// Check stderr for the expected error message
	stderrOutput := stderr.String()
	if !strings.Contains(stderrOutput, "strict mode") {
		t.Errorf("Expected 'strict mode' in error output, got: %s", stderrOutput)
	}
}

// TestNonStrictModeWithWarnings tests that warnings are allowed without -strict flag
func TestNonStrictModeWithWarnings(t *testing.T) {
	binaryPath := buildTestBinary(t)

	// Create a config with a privileged port (< 1024) which generates a warning
	configContent := `# Test config with privileged port
SocksPort 80
ControlPort 9051
LogLevel info
`
	configPath := createTempConfig(t, configContent)

	// Run without strict mode - should pass despite warning
	cmd := exec.Command(binaryPath, "-config", configPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Errorf("Expected non-strict mode to pass with warnings, got error: %v (stderr: %s)", err, stderr.String())
	}

	// Check stdout for success message
	stdoutOutput := stdout.String()
	if !strings.Contains(stdoutOutput, "Configuration is valid") {
		t.Errorf("Expected 'Configuration is valid' in output, got: %s", stdoutOutput)
	}
}

// TestStrictModeWithValidConfig tests that -strict flag passes for configs without warnings
func TestStrictModeWithValidConfig(t *testing.T) {
	binaryPath := buildTestBinary(t)

	// Create a valid config with no warnings
	configContent := `# Valid config with non-privileged ports
SocksPort 9050
ControlPort 9051
LogLevel info
`
	configPath := createTempConfig(t, configContent)

	// Run in strict mode - should pass
	cmd := exec.Command(binaryPath, "-config", configPath, "-strict")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Errorf("Expected strict mode to pass for valid config, got error: %v (stderr: %s)", err, stderr.String())
	}

	// Check stdout for success message with strict mode indication
	stdoutOutput := stdout.String()
	if !strings.Contains(stdoutOutput, "strict mode") {
		t.Errorf("Expected 'strict mode' in success output, got: %s", stdoutOutput)
	}
}

// TestStrictModeVerboseOutput tests that -strict -verbose shows warnings before failing
func TestStrictModeVerboseOutput(t *testing.T) {
	binaryPath := buildTestBinary(t)

	// Create a config with a privileged port
	configContent := `# Test config with privileged port
SocksPort 80
ControlPort 9051
LogLevel info
`
	configPath := createTempConfig(t, configContent)

	// Run in strict mode with verbose - should show warnings and fail
	cmd := exec.Command(binaryPath, "-config", configPath, "-strict", "-verbose")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Error("Expected strict mode to fail with warnings, but it passed")
	}

	// Check stdout for warning details (verbose mode prints to stdout)
	stdoutOutput := stdout.String()
	if !strings.Contains(stdoutOutput, "privileged port") {
		t.Errorf("Expected 'privileged port' warning in verbose output, got: %s", stdoutOutput)
	}
}

// TestNonExistentConfigFile tests behavior with non-existent config file
func TestNonExistentConfigFile(t *testing.T) {
	binaryPath := buildTestBinary(t)

	cmd := exec.Command(binaryPath, "-config", "/nonexistent/config.torrc")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Error("Expected error for non-existent config file, got nil")
	}

	output := stderr.String()
	if !strings.Contains(output, "does not exist") {
		t.Errorf("Expected 'does not exist' error message, got: %s", output)
	}
}

// TestInvalidConfig tests behavior with invalid configuration
func TestInvalidConfig(t *testing.T) {
	binaryPath := buildTestBinary(t)

	// Create a config with invalid port
	configContent := `# Invalid config with bad port
SocksPort 99999
LogLevel info
`
	configPath := createTempConfig(t, configContent)

	cmd := exec.Command(binaryPath, "-config", configPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Error("Expected error for invalid config, got nil")
	}
}

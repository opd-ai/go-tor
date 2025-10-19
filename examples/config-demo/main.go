// Package main demonstrates the configuration file loading functionality.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/opd-ai/go-tor/pkg/config"
)

func main() {
	fmt.Println("=== Configuration File Loading Demo ===")
	fmt.Println()

	// Create a temporary directory for demonstration
	tmpDir, err := os.MkdirTemp("", "go-tor-config-demo-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp directory: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	configFile := filepath.Join(tmpDir, "torrc")

	// Demo 1: Create and save a configuration
	fmt.Println("--- Demo 1: Creating and Saving Configuration ---")
	cfg := config.DefaultConfig()
	cfg.SocksPort = 9150
	cfg.ControlPort = 9151
	cfg.DataDirectory = "/custom/tor/data"
	cfg.LogLevel = "debug"
	cfg.NumEntryGuards = 5

	fmt.Printf("Created configuration:\n")
	fmt.Printf("  SocksPort: %d\n", cfg.SocksPort)
	fmt.Printf("  ControlPort: %d\n", cfg.ControlPort)
	fmt.Printf("  DataDirectory: %s\n", cfg.DataDirectory)
	fmt.Printf("  LogLevel: %s\n", cfg.LogLevel)
	fmt.Printf("  NumEntryGuards: %d\n\n", cfg.NumEntryGuards)

	// Save to file
	if err := config.SaveToFile(configFile, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Configuration saved to: %s\n\n", configFile)

	// Demo 2: Load configuration from file
	fmt.Println("--- Demo 2: Loading Configuration from File ---")
	loadedCfg := config.DefaultConfig()
	if err := config.LoadFromFile(configFile, loadedCfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Loaded configuration:\n")
	fmt.Printf("  SocksPort: %d\n", loadedCfg.SocksPort)
	fmt.Printf("  ControlPort: %d\n", loadedCfg.ControlPort)
	fmt.Printf("  DataDirectory: %s\n", loadedCfg.DataDirectory)
	fmt.Printf("  LogLevel: %s\n", loadedCfg.LogLevel)
	fmt.Printf("  NumEntryGuards: %d\n", loadedCfg.NumEntryGuards)
	fmt.Printf("  CircuitBuildTimeout: %v\n", loadedCfg.CircuitBuildTimeout)
	fmt.Printf("  MaxCircuitDirtiness: %v\n\n", loadedCfg.MaxCircuitDirtiness)

	// Verify values match
	if loadedCfg.SocksPort != cfg.SocksPort {
		fmt.Fprintf(os.Stderr, "Configuration mismatch: SocksPort\n")
		os.Exit(1)
	}
	fmt.Println("✓ Configuration loaded successfully and values match")
	fmt.Println()

	// Demo 3: Display the actual file content
	fmt.Println("--- Demo 3: Configuration File Content ---")
	content, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read config file: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(content))

	// Demo 4: Load with custom configuration
	fmt.Println("--- Demo 4: Custom Configuration File ---")
	customConfigFile := filepath.Join(tmpDir, "custom.conf")
	customContent := `# Custom go-tor configuration
SocksPort 9999
ControlPort 9998
DataDirectory /opt/tor
LogLevel warn
CircuitBuildTimeout 90s
MaxCircuitDirtiness 20m
NumEntryGuards 7
UseEntryGuards yes
UseBridges no
Bridge 10.0.0.1:9001
Bridge 10.0.0.2:9001
ExcludeNodes badnode1
ExcludeExitNodes badexit1
ConnLimit 2000
`
	if err := os.WriteFile(customConfigFile, []byte(customContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create custom config: %v\n", err)
		os.Exit(1)
	}

	customCfg := config.DefaultConfig()
	if err := config.LoadFromFile(customConfigFile, customCfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load custom config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Custom configuration loaded:\n")
	fmt.Printf("  SocksPort: %d\n", customCfg.SocksPort)
	fmt.Printf("  ControlPort: %d\n", customCfg.ControlPort)
	fmt.Printf("  DataDirectory: %s\n", customCfg.DataDirectory)
	fmt.Printf("  LogLevel: %s\n", customCfg.LogLevel)
	fmt.Printf("  CircuitBuildTimeout: %v\n", customCfg.CircuitBuildTimeout)
	fmt.Printf("  MaxCircuitDirtiness: %v\n", customCfg.MaxCircuitDirtiness)
	fmt.Printf("  NumEntryGuards: %d\n", customCfg.NumEntryGuards)
	fmt.Printf("  UseEntryGuards: %v\n", customCfg.UseEntryGuards)
	fmt.Printf("  UseBridges: %v\n", customCfg.UseBridges)
	fmt.Printf("  BridgeAddresses: %v\n", customCfg.BridgeAddresses)
	fmt.Printf("  ExcludeNodes: %v\n", customCfg.ExcludeNodes)
	fmt.Printf("  ExcludeExitNodes: %v\n", customCfg.ExcludeExitNodes)
	fmt.Printf("  ConnLimit: %d\n\n", customCfg.ConnLimit)

	// Verify custom values
	if customCfg.SocksPort != 9999 {
		fmt.Fprintf(os.Stderr, "Custom config mismatch: SocksPort\n")
		os.Exit(1)
	}
	if len(customCfg.BridgeAddresses) != 2 {
		fmt.Fprintf(os.Stderr, "Custom config mismatch: BridgeAddresses count\n")
		os.Exit(1)
	}
	fmt.Println("✓ Custom configuration loaded successfully")
	fmt.Println()

	// Demo 5: Validation
	fmt.Println("--- Demo 5: Configuration Validation ---")
	invalidConfigFile := filepath.Join(tmpDir, "invalid.conf")
	invalidContent := `SocksPort 70000  # Invalid - port too high
ControlPort 9051
`
	if err := os.WriteFile(invalidConfigFile, []byte(invalidContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create invalid config: %v\n", err)
		os.Exit(1)
	}

	invalidCfg := config.DefaultConfig()
	err = config.LoadFromFile(invalidConfigFile, invalidCfg)
	if err == nil {
		fmt.Fprintf(os.Stderr, "Expected validation error for invalid config\n")
		os.Exit(1)
	}
	fmt.Printf("✓ Invalid configuration correctly rejected: %v\n\n", err)

	fmt.Println("=== Demo Complete ===")
	fmt.Println("\nConfiguration file loading is now fully functional!")
	fmt.Println("You can use it with the tor-client binary like this:")
	fmt.Println("  ./bin/tor-client -config /path/to/torrc")
}

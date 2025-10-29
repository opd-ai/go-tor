// Package main demonstrates configuration validation and schema generation features.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/opd-ai/go-tor/pkg/config"
)

func main() {
	fmt.Println("=== go-tor Configuration Features Demo ===")
	fmt.Println()

	// Demo 1: Generate JSON Schema
	fmt.Println("1. Generating JSON Schema...")
	fmt.Println("   Run: tor-config-validator -schema -output config-schema.json")
	schema, err := config.GenerateJSONSchema()
	if err != nil {
		log.Fatalf("Failed to generate schema: %v", err)
	}
	fmt.Printf("   ✓ Schema generated with %d properties\n", len(schema.Properties))
	fmt.Println()

	// Demo 2: Default Configuration
	fmt.Println("2. Creating Default Configuration...")
	cfg := config.DefaultConfig()
	fmt.Printf("   ✓ SocksPort: %d\n", cfg.SocksPort)
	fmt.Printf("   ✓ ControlPort: %d\n", cfg.ControlPort)
	fmt.Printf("   ✓ LogLevel: %s\n", cfg.LogLevel)
	fmt.Printf("   ✓ CircuitBuildTimeout: %s\n", cfg.CircuitBuildTimeout)
	fmt.Println()

	// Demo 3: Simple Validation
	fmt.Println("3. Validating Configuration...")
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Validation failed: %v", err)
	}
	fmt.Println("   ✓ Basic validation passed")
	fmt.Println()

	// Demo 4: Detailed Validation
	fmt.Println("4. Detailed Validation...")
	result := cfg.ValidateDetailed()
	if result.Valid {
		fmt.Println("   ✓ Detailed validation passed")
		fmt.Printf("   ✓ No errors\n")
		fmt.Printf("   ✓ %d warnings\n", len(result.Warnings))
		fmt.Println()
	}

	// Demo 5: Invalid Configuration
	fmt.Println("5. Testing Invalid Configuration...")
	invalidCfg := config.DefaultConfig()
	invalidCfg.SocksPort = 99999 // Invalid port
	result = invalidCfg.ValidateDetailed()
	if !result.Valid {
		fmt.Println("   ✓ Correctly detected invalid configuration")
		for _, err := range result.Errors {
			fmt.Printf("   ✗ Error: %s\n", err.Message)
			if err.Suggestion != "" {
				fmt.Printf("     → %s\n", err.Suggestion)
			}
		}
	}
	fmt.Println()

	// Demo 6: Configuration with Warnings
	fmt.Println("6. Testing Configuration with Warnings...")
	warnCfg := config.DefaultConfig()
	warnCfg.SocksPort = 80 // Privileged port
	result = warnCfg.ValidateDetailed()
	if result.Valid && len(result.Warnings) > 0 {
		fmt.Println("   ✓ Configuration valid but has warnings")
		for _, warn := range result.Warnings {
			fmt.Printf("   ⚠  Warning: %s\n", warn.Message)
			if warn.Suggestion != "" {
				fmt.Printf("     → %s\n", warn.Suggestion)
			}
		}
	}
	fmt.Println()

	// Demo 7: Available Templates
	fmt.Println("7. Available Configuration Templates:")
	templates := []string{
		"minimal     - Simplest working configuration",
		"production  - Production-ready with monitoring",
		"development - Development with debug logging",
		"high-security - Privacy-focused with strict isolation",
	}
	for _, tmpl := range templates {
		fmt.Printf("   • %s\n", tmpl)
	}
	fmt.Println()
	fmt.Println("   Generate a template:")
	fmt.Println("   $ tor-config-validator -template production -output torrc")
	fmt.Println()

	// Demo 8: Schema Export
	fmt.Println("8. Exporting JSON Schema...")
	jsonData, err := schema.ToJSON()
	if err != nil {
		log.Fatalf("Failed to export schema: %v", err)
	}
	
	// Write to temporary file for demonstration
	tmpFile := "/tmp/config-schema-demo.json"
	if err := os.WriteFile(tmpFile, jsonData, 0644); err != nil {
		log.Fatalf("Failed to write schema: %v", err)
	}
	fmt.Printf("   ✓ Schema exported to %s (%d bytes)\n", tmpFile, len(jsonData))
	fmt.Println("   ✓ Use with your IDE for autocomplete and validation")
	fmt.Println()

	// Demo 9: Usage Examples
	fmt.Println("9. Common Usage Patterns:")
	fmt.Println()
	fmt.Println("   Validate existing config:")
	fmt.Println("   $ tor-config-validator -config /etc/tor/torrc -verbose")
	fmt.Println()
	fmt.Println("   Generate minimal config:")
	fmt.Println("   $ tor-config-validator -template minimal -output torrc")
	fmt.Println()
	fmt.Println("   Generate JSON schema:")
	fmt.Println("   $ tor-config-validator -schema -output config-schema.json")
	fmt.Println()
	fmt.Println("   List all templates:")
	fmt.Println("   $ tor-config-validator -list-templates")
	fmt.Println()

	fmt.Println("=== Demo Complete ===")
	fmt.Println()
	fmt.Println("For more information:")
	fmt.Println("  - Configuration Guide: docs/CONFIGURATION.md")
	fmt.Println("  - API Documentation: docs/API.md")
	fmt.Println("  - Examples: examples/config-demo/")
	fmt.Println()
	fmt.Println("⚠️  Remember: go-tor is for educational purposes only.")
	fmt.Println("   For real anonymity, use official Tor Browser:")
	fmt.Println("   https://www.torproject.org/download/")
}

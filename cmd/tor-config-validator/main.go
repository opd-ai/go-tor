// Package main provides a configuration validation and generation tool for go-tor.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opd-ai/go-tor/pkg/config"
)

var (
	version   = "0.1.0-dev"
	buildTime = "unknown"
)

func main() {
	// Parse command-line flags
	configFile := flag.String("config", "", "Path to configuration file to validate")
	generateSample := flag.Bool("generate", false, "Generate sample configuration file")
	generateSchema := flag.Bool("schema", false, "Generate JSON schema for configuration")
	listTemplates := flag.Bool("list-templates", false, "List available configuration templates")
	template := flag.String("template", "", "Generate config from template (minimal, production, development, high-security)")
	outputFile := flag.String("output", "", "Output file for generated configuration (default: stdout)")
	showVersion := flag.Bool("version", false, "Show version information")
	verbose := flag.Bool("verbose", false, "Verbose output")
	flag.Parse()

	if *showVersion {
		fmt.Printf("tor-config-validator version %s (built %s)\n", version, buildTime)
		fmt.Println("Configuration validation and generation tool for go-tor")
		os.Exit(0)
	}

	// Generate JSON schema if requested
	if *generateSchema {
		if err := generateJSONSchema(*outputFile, *verbose); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating schema: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// List templates if requested
	if *listTemplates {
		listConfigTemplates(*verbose)
		os.Exit(0)
	}

	// Generate from template if requested
	if *template != "" {
		if err := generateFromTemplate(*template, *outputFile, *verbose); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating from template: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Generate sample config if requested
	if *generateSample {
		if err := generateSampleConfig(*outputFile, *verbose); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating sample config: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Validate config file if provided
	if *configFile != "" {
		if err := validateConfigFile(*configFile, *verbose); err != nil {
			fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Configuration is valid")
		os.Exit(0)
	}

	// No operation specified
	printUsage()
	os.Exit(1)
}

func printUsage() {
	fmt.Println("tor-config-validator - Configuration tool for go-tor")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  tor-config-validator [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -config <file>           Validate configuration file")
	fmt.Println("  -generate                Generate sample configuration file")
	fmt.Println("  -schema                  Generate JSON schema for configuration")
	fmt.Println("  -list-templates          List available configuration templates")
	fmt.Println("  -template <name>         Generate config from template")
	fmt.Println("                           (minimal, production, development, high-security)")
	fmt.Println("  -output <file>           Output file for generated config (default: stdout)")
	fmt.Println("  -verbose                 Show detailed validation information")
	fmt.Println("  -version                 Show version information")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Validate existing configuration")
	fmt.Println("  tor-config-validator -config /etc/tor/torrc")
	fmt.Println()
	fmt.Println("  # Validate with detailed feedback")
	fmt.Println("  tor-config-validator -config myconfig.conf -verbose")
	fmt.Println()
	fmt.Println("  # Generate sample configuration to stdout")
	fmt.Println("  tor-config-validator -generate")
	fmt.Println()
	fmt.Println("  # Generate JSON schema for IDE autocomplete")
	fmt.Println("  tor-config-validator -schema -output config-schema.json")
	fmt.Println()
	fmt.Println("  # List available templates")
	fmt.Println("  tor-config-validator -list-templates")
	fmt.Println()
	fmt.Println("  # Generate minimal config")
	fmt.Println("  tor-config-validator -template minimal -output torrc")
	fmt.Println()
	fmt.Println("  # Generate production config")
	fmt.Println("  tor-config-validator -template production -output torrc.prod")
	fmt.Println()
	fmt.Println("Templates:")
	fmt.Println("  minimal        - Simplest working configuration")
	fmt.Println("  production     - Production-ready with monitoring and tuning")
	fmt.Println("  development    - Development config with debug logging")
	fmt.Println("  high-security  - Privacy-focused with strict isolation")
}

func validateConfigFile(path string, verbose bool) error {
	if verbose {
		fmt.Printf("Validating configuration file: %s\n", path)
		fmt.Println()
	}

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("configuration file does not exist: %s", path)
	}

	// Create default config and load from file
	cfg := config.DefaultConfig()
	if err := config.LoadFromFile(path, cfg); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if verbose {
		fmt.Println("Configuration loaded successfully")
		fmt.Println()
		printConfigSummary(cfg)
	}

	// Perform detailed validation
	result := cfg.ValidateDetailed()

	if verbose {
		fmt.Println()
		printValidationResult(result)
	}

	if !result.Valid {
		return fmt.Errorf("configuration has %d error(s)", len(result.Errors))
	}

	if verbose && len(result.Warnings) > 0 {
		fmt.Printf("\n⚠  Configuration has %d warning(s) but is valid\n", len(result.Warnings))
	}

	return nil
}

func printValidationResult(result *config.ValidationResult) {
	if len(result.Errors) > 0 {
		fmt.Println("Errors:")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		for _, err := range result.Errors {
			fmt.Printf("✗ %s\n", err.Message)
			if err.Suggestion != "" {
				fmt.Printf("  → %s\n", err.Suggestion)
			}
		}
		fmt.Println()
	}

	if len(result.Warnings) > 0 {
		fmt.Println("Warnings:")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		for _, warn := range result.Warnings {
			fmt.Printf("⚠  %s\n", warn.Message)
			if warn.Suggestion != "" {
				fmt.Printf("  → %s\n", warn.Suggestion)
			}
		}
		fmt.Println()
	}

	if result.Valid && len(result.Errors) == 0 {
		fmt.Println("✓ All validation checks passed")
	}
}

func generateJSONSchema(outputPath string, verbose bool) error {
	if verbose {
		fmt.Println("Generating JSON schema...")
	}

	schema, err := config.GenerateJSONSchema()
	if err != nil {
		return fmt.Errorf("failed to generate schema: %w", err)
	}

	jsonData, err := schema.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to convert schema to JSON: %w", err)
	}

	// Write to file or stdout
	if outputPath != "" {
		// Create directory if it doesn't exist
		dir := filepath.Dir(outputPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Write file
		if err := os.WriteFile(outputPath, jsonData, 0o644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		if verbose {
			fmt.Printf("JSON schema written to: %s\n", outputPath)
			fmt.Println()
			fmt.Println("Use this schema with your IDE for autocomplete and validation.")
			fmt.Println()
			fmt.Println("For VS Code, add to .vscode/settings.json:")

			// Create a proper JSON example using a map
			exampleSettings := map[string]interface{}{
				"json.schemas": []map[string]interface{}{
					{
						"fileMatch": []string{"torrc", "*.torrc"},
						"url":       "./" + filepath.Base(outputPath),
					},
				},
			}

			exampleJSON, _ := json.MarshalIndent(exampleSettings, "", "  ")
			fmt.Println(string(exampleJSON))
		} else {
			fmt.Printf("JSON schema created: %s\n", outputPath)
		}
	} else {
		// Write to stdout
		fmt.Println(string(jsonData))
	}

	return nil
}

func listConfigTemplates(verbose bool) {
	templates := []struct {
		name        string
		file        string
		description string
	}{
		{
			name:        "minimal",
			file:        "configs/templates/minimal.torrc",
			description: "Simplest working configuration - just the essentials",
		},
		{
			name:        "production",
			file:        "configs/templates/production.torrc",
			description: "Production-ready with monitoring, performance tuning, and best practices",
		},
		{
			name:        "development",
			file:        "configs/templates/development.torrc",
			description: "Development config with debug logging, metrics, and relaxed timeouts",
		},
		{
			name:        "high-security",
			file:        "configs/templates/high-security.torrc",
			description: "Privacy-focused with strict circuit isolation and conservative settings",
		},
	}

	fmt.Println("Available Configuration Templates:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	for _, tmpl := range templates {
		fmt.Printf("  %s\n", tmpl.name)
		if verbose {
			fmt.Printf("    File: %s\n", tmpl.file)
		}
		fmt.Printf("    %s\n", tmpl.description)
		fmt.Println()
	}

	if !verbose {
		fmt.Println("Use -verbose to see template file paths")
	}

	fmt.Println("Generate a template:")
	fmt.Println("  tor-config-validator -template <name> -output torrc")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  tor-config-validator -template production -output /etc/tor/torrc")
}

func generateFromTemplate(templateName, outputPath string, verbose bool) error {
	// Map template names to files
	templateFiles := map[string]string{
		"minimal":       "configs/templates/minimal.torrc",
		"production":    "configs/templates/production.torrc",
		"development":   "configs/templates/development.torrc",
		"high-security": "configs/templates/high-security.torrc",
	}

	templateFile, ok := templateFiles[templateName]
	if !ok {
		return fmt.Errorf("unknown template: %s (available: minimal, production, development, high-security)", templateName)
	}

	if verbose {
		fmt.Printf("Generating configuration from template: %s\n", templateName)
	}

	// Read template file
	content, err := os.ReadFile(templateFile)
	if err != nil {
		return fmt.Errorf("failed to read template: %w", err)
	}

	// Write to file or stdout
	if outputPath != "" {
		// Create directory if it doesn't exist
		dir := filepath.Dir(outputPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Write file
		if err := os.WriteFile(outputPath, content, 0o644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		if verbose {
			fmt.Printf("Configuration written to: %s\n", outputPath)
			fmt.Println()
			fmt.Printf("Template: %s\n", templateName)
			fmt.Printf("Size: %d bytes\n", len(content))
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Println("  1. Review and customize the configuration")
			fmt.Println("  2. Validate: tor-config-validator -config " + outputPath)
			fmt.Println("  3. Run: tor-client -config " + outputPath)
		} else {
			fmt.Printf("Configuration file created: %s (template: %s)\n", outputPath, templateName)
		}
	} else {
		// Write to stdout
		fmt.Print(string(content))
	}

	return nil
}

func generateSampleConfig(outputPath string, verbose bool) error {
	cfg := config.DefaultConfig()

	// Build sample config content
	var sb strings.Builder

	sb.WriteString("# Sample Tor Configuration File\n")
	sb.WriteString("# Generated by tor-config-validator\n")
	sb.WriteString("# For more information, see the documentation at:\n")
	sb.WriteString("# https://github.com/opd-ai/go-tor/docs\n")
	sb.WriteString("\n")

	sb.WriteString("# Network Settings\n")
	sb.WriteString(fmt.Sprintf("SocksPort %d\n", cfg.SocksPort))
	sb.WriteString(fmt.Sprintf("ControlPort %d\n", cfg.ControlPort))
	sb.WriteString("\n")

	sb.WriteString("# Data Directory\n")
	sb.WriteString(fmt.Sprintf("DataDirectory %s\n", cfg.DataDirectory))
	sb.WriteString("\n")

	sb.WriteString("# Logging\n")
	sb.WriteString(fmt.Sprintf("LogLevel %s\n", cfg.LogLevel))
	sb.WriteString("\n")

	sb.WriteString("# Circuit Configuration\n")
	sb.WriteString(fmt.Sprintf("NumEntryGuards %d\n", cfg.NumEntryGuards))
	sb.WriteString(fmt.Sprintf("CircuitBuildTimeout %s\n", cfg.CircuitBuildTimeout))
	sb.WriteString(fmt.Sprintf("MaxCircuitDirtiness %s\n", cfg.MaxCircuitDirtiness))
	sb.WriteString(fmt.Sprintf("NewCircuitPeriod %s\n", cfg.NewCircuitPeriod))
	sb.WriteString("\n")

	sb.WriteString("# Connection Settings\n")
	sb.WriteString(fmt.Sprintf("ConnLimit %d\n", cfg.ConnLimit))
	sb.WriteString(fmt.Sprintf("DormantTimeout %s\n", cfg.DormantTimeout))
	sb.WriteString("\n")

	sb.WriteString("# Performance Tuning\n")
	sb.WriteString(fmt.Sprintf("# EnableConnectionPooling %t\n", cfg.EnableConnectionPooling))
	sb.WriteString(fmt.Sprintf("# ConnectionPoolMaxIdle %d\n", cfg.ConnectionPoolMaxIdle))
	sb.WriteString(fmt.Sprintf("# ConnectionPoolMaxLife %s\n", cfg.ConnectionPoolMaxLife))
	sb.WriteString(fmt.Sprintf("# EnableCircuitPrebuilding %t\n", cfg.EnableCircuitPrebuilding))
	sb.WriteString(fmt.Sprintf("# CircuitPoolMinSize %d\n", cfg.CircuitPoolMinSize))
	sb.WriteString(fmt.Sprintf("# CircuitPoolMaxSize %d\n", cfg.CircuitPoolMaxSize))
	sb.WriteString(fmt.Sprintf("# EnableBufferPooling %t\n", cfg.EnableBufferPooling))
	sb.WriteString("\n")

	sb.WriteString("# Circuit Isolation\n")
	sb.WriteString(fmt.Sprintf("# IsolationLevel %s\n", cfg.IsolationLevel))
	sb.WriteString(fmt.Sprintf("# IsolateDestinations %t\n", cfg.IsolateDestinations))
	sb.WriteString(fmt.Sprintf("# IsolateSOCKSAuth %t\n", cfg.IsolateSOCKSAuth))
	sb.WriteString(fmt.Sprintf("# IsolateClientPort %t\n", cfg.IsolateClientPort))
	sb.WriteString("\n")

	sb.WriteString("# HTTP Metrics (optional)\n")
	sb.WriteString("# EnableMetrics false\n")
	sb.WriteString("# MetricsPort 9052\n")
	sb.WriteString("\n")

	sb.WriteString("# Security Settings\n")
	sb.WriteString("# SafeLogging true\n")
	sb.WriteString("# UseEntryGuards true\n")
	sb.WriteString("\n")

	content := sb.String()

	// Write to file or stdout
	if outputPath != "" {
		// Create directory if it doesn't exist
		dir := filepath.Dir(outputPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Write file
		if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		if verbose {
			fmt.Printf("Sample configuration written to: %s\n", outputPath)
		} else {
			fmt.Printf("Configuration file created: %s\n", outputPath)
		}
	} else {
		// Write to stdout
		fmt.Print(content)
	}

	return nil
}

func printConfigSummary(cfg *config.Config) {
	fmt.Println("Configuration Summary:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	fmt.Println("\nNetwork Settings:")
	fmt.Printf("  SOCKS Port:       %d\n", cfg.SocksPort)
	fmt.Printf("  Control Port:     %d\n", cfg.ControlPort)
	if cfg.EnableMetrics {
		fmt.Printf("  Metrics Port:     %d\n", cfg.MetricsPort)
	}

	fmt.Println("\nPaths:")
	fmt.Printf("  Data Directory:   %s\n", cfg.DataDirectory)

	fmt.Println("\nLogging:")
	fmt.Printf("  Log Level:        %s\n", cfg.LogLevel)

	fmt.Println("\nCircuit Configuration:")
	fmt.Printf("  Num Entry Guards: %d\n", cfg.NumEntryGuards)
	fmt.Printf("  Build Timeout:    %s\n", cfg.CircuitBuildTimeout)
	fmt.Printf("  Max Dirtiness:    %s\n", cfg.MaxCircuitDirtiness)
	fmt.Printf("  New Circuit Per:  %s\n", cfg.NewCircuitPeriod)

	fmt.Println("\nConnection Settings:")
	fmt.Printf("  Conn Limit:       %d\n", cfg.ConnLimit)
	fmt.Printf("  Dormant Timeout:  %s\n", cfg.DormantTimeout)

	fmt.Println("\nPerformance:")
	fmt.Printf("  Conn Pooling:     %t\n", cfg.EnableConnectionPooling)
	if cfg.EnableConnectionPooling {
		fmt.Printf("  Pool Max Idle:    %d\n", cfg.ConnectionPoolMaxIdle)
		fmt.Printf("  Pool Max Life:    %s\n", cfg.ConnectionPoolMaxLife)
	}
	fmt.Printf("  Circuit Prebuild: %t\n", cfg.EnableCircuitPrebuilding)
	if cfg.EnableCircuitPrebuilding {
		fmt.Printf("  Pool Min Size:    %d\n", cfg.CircuitPoolMinSize)
		fmt.Printf("  Pool Max Size:    %d\n", cfg.CircuitPoolMaxSize)
	}
	fmt.Printf("  Buffer Pooling:   %t\n", cfg.EnableBufferPooling)

	fmt.Println("\nCircuit Isolation:")
	fmt.Printf("  Isolation Level:  %s\n", cfg.IsolationLevel)
	fmt.Printf("  Isolate Dest:     %t\n", cfg.IsolateDestinations)
	fmt.Printf("  Isolate SOCKS:    %t\n", cfg.IsolateSOCKSAuth)
	fmt.Printf("  Isolate Port:     %t\n", cfg.IsolateClientPort)

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

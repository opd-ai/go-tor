// Package main demonstrates obfs4 pluggable transport usage.
//
// This example shows how to use the obfs4 transport for both client and server scenarios.
// It demonstrates certificate generation, bridge line parsing, and connection establishment.
//
// ⚠️  EDUCATIONAL PURPOSE ONLY
// This is an experimental implementation. For actual Tor usage, use official Tor Browser.
//
// Requirements:
//   - obfs4proxy binary installed (apt-get install obfs4proxy on Debian/Ubuntu)
//
// Usage:
//
//	go run main.go
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/opd-ai/go-tor/pkg/pt"
	"github.com/opd-ai/go-tor/pkg/pt/obfs4"
)

func main() {
	fmt.Println("=== obfs4 Pluggable Transport Demo ===")
	fmt.Println()
	fmt.Println("⚠️  EDUCATIONAL PURPOSE ONLY")
	fmt.Println("This is an experimental implementation.")
	fmt.Println("For real Tor usage, use official Tor Browser.")
	fmt.Println()

	// Demonstrate PT discovery
	demonstratePTDiscovery()

	// Demonstrate client configuration
	demonstrateClientConfig()

	// Demonstrate server configuration
	demonstrateServerConfig()

	// Demonstrate bridge line parsing
	demonstrateBridgeLineParsing()

	fmt.Println("\n=== Demo Complete ===")
}

func demonstratePTDiscovery() {
	fmt.Println("--- PT Discovery ---")

	// Discover common PTs on the system
	discovered := pt.DiscoverCommonPTs()

	if len(discovered) == 0 {
		fmt.Println("⚠  No pluggable transports found")
		fmt.Println("   Install obfs4proxy: apt-get install obfs4proxy")
		fmt.Println()
		return
	}

	fmt.Printf("Found %d pluggable transport(s):\n", len(discovered))
	for name, path := range discovered {
		fmt.Printf("  - %s: %s\n", name, path)
	}
	fmt.Println()
}

func demonstrateClientConfig() {
	fmt.Println("--- obfs4 Client Configuration ---")

	// Example bridge line (from a real bridge)
	bridgeLine := "obfs4 192.0.2.1:1234 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA " +
		"cert=dGVzdGNlcnRpZmljYXRlMTIzNDU2Nzg5MDEyMzQ1Njc4OTA= iat-mode=0"

	// Parse the bridge line
	config, address, err := obfs4.ParseBridgeLine(bridgeLine)
	if err != nil {
		log.Printf("Failed to parse bridge line: %v", err)
		return
	}

	fmt.Printf("Bridge Address: %s\n", address)
	fmt.Printf("Certificate: %s\n", config.Certificate)
	fmt.Printf("IAT Mode: %d\n", config.IATMode)

	// Validate the certificate
	if err := obfs4.ValidateCertificate(config.Certificate); err != nil {
		log.Printf("Invalid certificate: %v", err)
		return
	}
	fmt.Println("✓ Certificate is valid")

	// Create client configuration
	tempDir, _ := os.MkdirTemp("", "obfs4-client-")
	defer os.RemoveAll(tempDir)

	clientConfig := obfs4.ClientConfig{
		BinaryPath:  "", // Auto-discover
		Cert:        config.Certificate,
		IATMode:     config.IATMode,
		StateDir:    tempDir,
		DialTimeout: 30 * time.Second,
	}

	// Create obfs4 client
	client, err := obfs4.NewClient(clientConfig)
	if err != nil {
		log.Printf("Failed to create client: %v", err)
		fmt.Println()
		return
	}
	defer client.Close()

	fmt.Printf("✓ Client created successfully\n")
	fmt.Printf("  Transport: %s\n", client.Name())
	fmt.Printf("  State Dir: %s\n", tempDir)
	fmt.Println()

	// Note: We don't actually start the client here as it requires obfs4proxy
	// In a real scenario, you would:
	// ctx := context.Background()
	// if err := client.Start(ctx); err != nil { ... }
	// conn, err := client.Dial(ctx, address)
}

func demonstrateServerConfig() {
	fmt.Println("--- obfs4 Server Configuration ---")

	tempDir, _ := os.MkdirTemp("", "obfs4-server-")
	defer os.RemoveAll(tempDir)

	serverConfig := obfs4.ServerConfig{
		BinaryPath: "", // Auto-discover
		BindAddr:   "127.0.0.1:0",
		StateDir:   tempDir,
		IATMode:    0,
	}

	// Create obfs4 server
	server, err := obfs4.NewServer(serverConfig)
	if err != nil {
		log.Printf("Failed to create server: %v", err)
		fmt.Println()
		return
	}
	defer server.Close()

	fmt.Printf("✓ Server created successfully\n")
	fmt.Printf("  Transport: %s\n", server.Name())
	fmt.Printf("  Bind Address: %s\n", serverConfig.BindAddr)
	fmt.Printf("  State Dir: %s\n", tempDir)
	fmt.Printf("  IAT Mode: %d\n", serverConfig.IATMode)

	// Demonstrate state file path
	stateFile := obfs4.GetStateFilePath(tempDir)
	fmt.Printf("  State File: %s\n", stateFile)

	// Generate example bridge line
	exampleCert := "dGVzdGNlcnRpZmljYXRlMTIzNDU2Nzg5MDEyMzQ1Njc4OTA="
	bridgeLine := obfs4.GetBridgeLineExample("192.0.2.1:1234", exampleCert, 0)
	fmt.Printf("\nExample Bridge Line:\n  %s\n", bridgeLine)
	fmt.Println()

	// Note: We don't actually start the server as it requires obfs4proxy
	// In a real scenario, you would:
	// ctx := context.Background()
	// if err := server.Start(ctx); err != nil { ... }
	// listener, err := server.Listen(ctx, "")
	// cert, err := server.GetCertificate()
}

func demonstrateBridgeLineParsing() {
	fmt.Println("--- Bridge Line Parsing ---")

	testCases := []string{
		"obfs4 192.0.2.1:1234 AAAA cert=dGVzdDEyMzQ= iat-mode=0",
		"Bridge obfs4 192.0.2.2:5678 BBBB cert=eHl6YWJjZGVm iat-mode=1",
		"obfs4 [2001:db8::1]:9001 CCCC cert=dGVzdGNlcnRpZmljYXRl iat-mode=2",
	}

	for i, bridgeLine := range testCases {
		fmt.Printf("Bridge %d:\n", i+1)
		fmt.Printf("  Line: %s\n", bridgeLine)

		config, address, err := obfs4.ParseBridgeLine(bridgeLine)
		if err != nil {
			log.Printf("  Error: %v\n", err)
			continue
		}

		fmt.Printf("  Address: %s\n", address)
		fmt.Printf("  Certificate: %s\n", config.Certificate)
		fmt.Printf("  IAT Mode: %d", config.IATMode)

		switch config.IATMode {
		case 0:
			fmt.Println(" (disabled)")
		case 1:
			fmt.Println(" (enabled)")
		case 2:
			fmt.Println(" (paranoid)")
		}

		fmt.Println()
	}
}

// demonstrateKeyManagement shows key export/import functionality
func demonstrateKeyManagement() {
	fmt.Println("--- Key Management ---")

	tempDir, _ := os.MkdirTemp("", "obfs4-keys-")
	defer os.RemoveAll(tempDir)

	// Create a mock state file
	stateFile := filepath.Join(tempDir, "obfs4_state.json")
	mockState := []byte(`{"test": "state", "keys": "generated"}`)
	if err := os.WriteFile(stateFile, mockState, 0o600); err != nil {
		log.Printf("Failed to create state: %v", err)
		return
	}

	// Export keys
	exportPath := filepath.Join(tempDir, "keys_backup.dat")
	if err := obfs4.ExportKeys(tempDir, exportPath); err != nil {
		log.Printf("Failed to export keys: %v", err)
		return
	}
	fmt.Printf("✓ Keys exported to: %s\n", exportPath)

	// Import to new directory
	importDir := filepath.Join(tempDir, "import")
	if err := obfs4.ImportKeys(exportPath, importDir); err != nil {
		log.Printf("Failed to import keys: %v", err)
		return
	}
	fmt.Printf("✓ Keys imported to: %s\n", importDir)
	fmt.Println()
}

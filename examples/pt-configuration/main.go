// Package main demonstrates pluggable transport configuration.
//
// This example shows how to configure go-tor to use pluggable transports
// like obfs4 for censorship resistance. It creates a sample torrc file with
// PT configuration and loads it.
//
// Note: This is an example only. You need to have the actual PT binary
// (e.g., obfs4proxy) installed to use pluggable transports.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/opd-ai/go-tor/pkg/config"
)

func main() {
	fmt.Println("=== Pluggable Transport Configuration Example ===\n")

	// Create a temporary directory for our example
	tmpDir, err := os.MkdirTemp("", "pt-config-example-*")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	torrcPath := filepath.Join(tmpDir, "torrc")

	// Example 1: Create a configuration with client-side pluggable transport
	fmt.Println("Example 1: Client-side Pluggable Transport (obfs4)")
	fmt.Println("--------------------------------------------------")

	clientTorrc := "# Tor configuration with pluggable transport\nSocksPort 9050\nDataDirectory " + tmpDir + "/tor-data\n\n# Use bridges with pluggable transport\nUseBridges 1\nBridge obfs4 192.0.2.1:1234 cert=AAAAAAAAAAAAAAAAAAAAAAAAAAAAA iat-mode=0\n\n# Configure pluggable transport\nClientTransportPlugin obfs4 exec /usr/bin/obfs4proxy\n\n# Optional: Use a SOCKS5 proxy for PT connections\n# TransportProxy socks5 127.0.0.1:9150\n"

	if err := os.WriteFile(torrcPath, []byte(clientTorrc), 0o600); err != nil {
		log.Fatalf("Failed to write torrc: %v", err)
	}

	// Load the configuration
	cfg := config.DefaultConfig()
	if err := config.LoadFromFile(torrcPath, cfg); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Display the loaded PT configuration
	fmt.Printf("Loaded configuration:\n")
	fmt.Printf("  UseBridges: %v\n", cfg.UseBridges)
	fmt.Printf("  Number of bridges: %d\n", len(cfg.BridgeAddresses))
	fmt.Printf("  Number of client transports: %d\n", len(cfg.ClientTransports))

	if len(cfg.ClientTransports) > 0 {
		for i, ct := range cfg.ClientTransports {
			fmt.Printf("\n  Client Transport %d:\n", i+1)
			fmt.Printf("    Name: %s\n", ct.Name)
			fmt.Printf("    Binary: %s\n", ct.BinaryPath)
			if len(ct.Options) > 0 {
				fmt.Printf("    Options:\n")
				for k, v := range ct.Options {
					fmt.Printf("      %s = %s\n", k, v)
				}
			}
		}
	}

	// Example 2: Server-side pluggable transport (for bridge relays)
	fmt.Println("\n\nExample 2: Server-side Pluggable Transport (Bridge Relay)")
	fmt.Println("----------------------------------------------------------")

	serverTorrc := "# Bridge relay configuration with pluggable transport\nSocksPort 0\nORPort 9001\nBridgeRelay 1\nDataDirectory " + tmpDir + "/bridge-data\n\n# Configure server-side pluggable transport\nServerTransportPlugin obfs4 exec /usr/bin/obfs4proxy\nServerTransportListenAddr obfs4 0.0.0.0:9443\nServerTransportOptions obfs4 iat-mode=1 drbg-seed=0123456789ABCDEF\n"

	if err := os.WriteFile(torrcPath, []byte(serverTorrc), 0o600); err != nil {
		log.Fatalf("Failed to write torrc: %v", err)
	}

	cfg2 := config.DefaultConfig()
	if err := config.LoadFromFile(torrcPath, cfg2); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Loaded configuration:\n")
	fmt.Printf("  Number of server transports: %d\n", len(cfg2.ServerTransports))

	if len(cfg2.ServerTransports) > 0 {
		for i, st := range cfg2.ServerTransports {
			fmt.Printf("\n  Server Transport %d:\n", i+1)
			fmt.Printf("    Name: %s\n", st.Name)
			fmt.Printf("    Binary: %s\n", st.BinaryPath)
			fmt.Printf("    Listen Address: %s\n", st.BindAddr)
			if len(st.Options) > 0 {
				fmt.Printf("    Options:\n")
				for k, v := range st.Options {
					fmt.Printf("      %s = %s\n", k, v)
				}
			}
		}
	}

	// Example 3: Programmatic configuration
	fmt.Println("\n\nExample 3: Programmatic PT Configuration")
	fmt.Println("------------------------------------------")

	cfg3 := config.DefaultConfig()
	cfg3.ClientTransports = []config.ClientTransportConfig{
		{
			Name:       "obfs4",
			BinaryPath: "/usr/bin/obfs4proxy",
			Options: map[string]string{
				"cert":     "AAAAAAAAAAAAAAAAAAAAAAAAA",
				"iat-mode": "0",
			},
		},
		{
			Name:       "meek",
			BinaryPath: "/usr/bin/meek-client",
			Options: map[string]string{
				"url":   "https://meek.example.com",
				"front": "www.example.com",
			},
		},
	}
	cfg3.TransportProxy = "socks5 127.0.0.1:9150"

	// Save this configuration
	savedPath := filepath.Join(tmpDir, "generated-torrc")
	if err := config.SaveToFile(savedPath, cfg3); err != nil {
		log.Fatalf("Failed to save config: %v", err)
	}

	fmt.Printf("Configuration saved to: %s\n\n", savedPath)

	// Read and display the generated file
	content, err := os.ReadFile(savedPath)
	if err != nil {
		log.Fatalf("Failed to read generated file: %v", err)
	}

	fmt.Println("Generated torrc content:")
	fmt.Println("------------------------")
	fmt.Print(string(content))

	fmt.Println("\n=== Example Complete ===")
	fmt.Println("\nNote: To actually use pluggable transports, you need:")
	fmt.Println("  1. The PT binary installed (e.g., obfs4proxy from torproject.org)")
	fmt.Println("  2. Bridge addresses from https://bridges.torproject.org/")
	fmt.Println("  3. Proper PT configuration matching your bridge line")
}

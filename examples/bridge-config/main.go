package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/opd-ai/go-tor/pkg/config"
)

func main() {
	fmt.Println("Bridge Configuration and PT Integration Example")
	fmt.Println("================================================")

	demonstrateBridgeParsing()
	fmt.Println()
	demonstrateBridgeConfiguration()
	fmt.Println()
	demonstrateTorrcLoading()
	fmt.Println()
	demonstratePTIntegration()
}

func demonstrateBridgeParsing() {
	fmt.Println("=== Bridge Line Parsing ===")

	bridgeLines := []string{
		"192.0.2.1:443",
		"192.0.2.1:9001 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"obfs4 192.0.2.1:1234 cert=abcd1234 iat-mode=0",
		"Bridge obfs4 192.0.2.1:1234 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA cert=xyz iat-mode=1",
		"meek_lite 192.0.2.1:443 url=https://meek.example.com front=www.google.com",
		"snowflake 192.0.2.1:1234 fingerprint=ABC ice=stun:stun.l.google.com:19302",
	}

	for i, line := range bridgeLines {
		fmt.Printf("\n%d. Parsing: %s\n", i+1, line)

		bridge, err := config.ParseBridge(line)
		if err != nil {
			fmt.Printf("   ERROR: %v\n", err)
			continue
		}

		fmt.Printf("   Transport: %s\n", bridge.GetTransportName())
		if bridge.GetTransportName() == "" {
			fmt.Printf("   Type: Vanilla Bridge\n")
		} else {
			fmt.Printf("   Type: Pluggable Transport\n")
		}
		fmt.Printf("   Address: %s\n", bridge.GetAddress())
		if bridge.Fingerprint != "" {
			fmt.Printf("   Fingerprint: %s\n", bridge.Fingerprint)
		}
		if len(bridge.Parameters) > 0 {
			fmt.Printf("   Parameters:\n")
			for k, v := range bridge.Parameters {
				if v != "" {
					fmt.Printf("     %s = %s\n", k, v)
				} else {
					fmt.Printf("     %s\n", k)
				}
			}
		}
	}
}

func demonstrateBridgeConfiguration() {
	fmt.Println("=== Programmatic Bridge Configuration ===")

	cfg := config.DefaultConfig()
	cfg.UseBridges = true
	cfg.UseEntryGuards = false
	cfg.BridgeAddresses = []string{
		"192.0.2.1:443",
		"obfs4 192.0.2.2:1234 cert=xyz iat-mode=1",
		"meek_lite 192.0.2.3:443 url=https://meek.example.com",
	}

	fmt.Printf("Bridge mode enabled: %v\n", cfg.UseBridges)
	fmt.Printf("Number of bridges configured: %d\n", len(cfg.BridgeAddresses))
	fmt.Println("\nConfigured bridges:")
	for i, bridge := range cfg.BridgeAddresses {
		fmt.Printf("  %d. %s\n", i+1, bridge)
	}
}

func demonstrateTorrcLoading() {
	fmt.Println("=== Loading Bridges from torrc ===")

	tmpDir := os.TempDir()
	torrcPath := tmpDir + "/example-bridges.torrc"

	torrcContent := `UseBridges 1
Bridge 192.0.2.1:443
Bridge obfs4 192.0.2.2:1234 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA cert=abcd1234 iat-mode=0
Bridge obfs4 192.0.2.3:9001 cert=xyz456 iat-mode=1
ClientTransportPlugin obfs4 exec /usr/bin/obfs4proxy
SocksPort 9050
DataDirectory /tmp/tor-bridge-test`

	err := os.WriteFile(torrcPath, []byte(torrcContent), 0o600)
	if err != nil {
		log.Printf("Failed to write torrc: %v\n", err)
		return
	}
	defer os.Remove(torrcPath)

	cfg := config.DefaultConfig()
	err = config.LoadFromFile(torrcPath, cfg)
	if err != nil {
		log.Printf("Failed to load torrc: %v\n", err)
		return
	}

	fmt.Printf("Successfully loaded torrc: %s\n", torrcPath)
	fmt.Printf("Bridge mode: %v\n", cfg.UseBridges)
	fmt.Printf("Configured bridges: %d\n", len(cfg.BridgeAddresses))
	fmt.Printf("Configured transports: %d\n", len(cfg.ClientTransports))

	fmt.Println("\nParsed bridges:")
	for i, bridge := range cfg.Bridges {
		fmt.Printf("  %d. %s", i+1, bridge.GetAddress())
		if bridge.IsPluggableTransport() {
			fmt.Printf(" (transport: %s)", bridge.GetTransportName())
		}
		fmt.Println()
	}

	fmt.Println("\nConfigured transports:")
	for i, transport := range cfg.ClientTransports {
		fmt.Printf("  %d. %s -> %s\n", i+1, transport.Name, transport.BinaryPath)
	}
}

func demonstratePTIntegration() {
	fmt.Println("=== PT Integration with Circuit Builder ===")

	cfg := config.DefaultConfig()
	cfg.UseBridges = true
	cfg.BridgeAddresses = []string{
		"obfs4 192.0.2.1:1234 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA cert=abcd1234 iat-mode=0",
		"obfs4 192.0.2.2:5678 cert=xyz456 iat-mode=1",
	}
	cfg.ClientTransports = []config.ClientTransportConfig{
		{
			Name:       "obfs4",
			BinaryPath: "/usr/bin/obfs4proxy",
			Options:    map[string]string{},
		},
	}

	fmt.Println("Configuration for PT-enabled bridge connections:")
	fmt.Printf("  Bridge mode: %v\n", cfg.UseBridges)
	fmt.Printf("  Bridges: %d\n", len(cfg.BridgeAddresses))
	fmt.Printf("  Transports: %d\n", len(cfg.ClientTransports))

	fmt.Println("\nCircuit building workflow with PT:")
	fmt.Println("  1. Client checks if bridge uses PT")
	fmt.Println("  2. If PT required, start PT process (obfs4proxy)")
	fmt.Println("  3. PT process provides SOCKS5 proxy")
	fmt.Println("  4. Connect to bridge via PT SOCKS5 proxy")
	fmt.Println("  5. Perform Tor handshake over PT connection")
	fmt.Println("  6. Build circuit through bridge to middle/exit")

	fmt.Println("\nPT Startup Sequence (simulated):")
	simulatePTStartup(cfg)
}

func simulatePTStartup(cfg *config.Config) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, transport := range cfg.ClientTransports {
		fmt.Printf("\n[PT] Starting %s transport...\n", transport.Name)
		fmt.Printf("[PT] Exec: %s\n", transport.BinaryPath)
		fmt.Printf("[PT] Environment variables:\n")
		fmt.Printf("     TOR_PT_CLIENT_TRANSPORTS=%s\n", transport.Name)
		fmt.Printf("     TOR_PT_MANAGED_TRANSPORT_VER=1\n")

		select {
		case <-ctx.Done():
			fmt.Println("[PT] Startup timeout")
			return
		case <-time.After(100 * time.Millisecond):
			fmt.Printf("[PT] %s ready on socks5://127.0.0.1:12345\n", transport.Name)
		}
	}

	fmt.Println("\n[PT] All transports ready for bridge connections")
}

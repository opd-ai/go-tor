// Package main demonstrates PT manager with automatic restart and discovery.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/opd-ai/go-tor/pkg/pt"
)

func main() {
	fmt.Println("=== Pluggable Transport Manager Demo ===\n")

	// 1. Discover available PTs
	fmt.Println("1. Discovering available PTs...")
	discoveredPTs := pt.DiscoverCommonPTs()
	if len(discoveredPTs) == 0 {
		fmt.Println("   No common PTs found. This demo requires obfs4proxy or other PTs.")
		fmt.Println("   Install with: apt-get install obfs4proxy")
		fmt.Println("\n   Continuing with mock demonstration...\n")
	} else {
		fmt.Printf("   Found %d PTs:\n", len(discoveredPTs))
		for name, path := range discoveredPTs {
			fmt.Printf("   - %s: %s\n", name, path)
		}
		fmt.Println()
	}

	// 2. Create PT manager with auto-restart
	fmt.Println("2. Creating PT manager...")
	mgr := pt.NewManager(pt.ManagerConfig{
		StateDir:     "/tmp/pt-demo",
		AutoRestart:  true,
		RestartDelay: 5 * time.Second,
		MaxRestarts:  3, // Limit restarts to 3 attempts
	})
	defer mgr.Close()
	fmt.Println("   ✓ Manager created with auto-restart enabled")
	fmt.Println()

	// 3. Add PTs to manager
	fmt.Println("3. Registering PTs...")

	if obfs4Path, ok := discoveredPTs["obfs4proxy"]; ok {
		if err := mgr.AddClient("obfs4", pt.TransportConfig{
			BinaryPath: obfs4Path,
		}); err != nil {
			log.Printf("   Failed to add obfs4: %v", err)
		} else {
			fmt.Println("   ✓ Registered obfs4proxy client")
		}
	}

	if snowflakePath, ok := discoveredPTs["snowflake-client"]; ok {
		if err := mgr.AddClient("snowflake", pt.TransportConfig{
			BinaryPath: snowflakePath,
		}); err != nil {
			log.Printf("   Failed to add snowflake: %v", err)
		} else {
			fmt.Println("   ✓ Registered snowflake-client")
		}
	}

	clients := mgr.Clients()
	if len(clients) == 0 {
		fmt.Println("   No PTs registered (none found on system)")
		fmt.Println("\n   Demo complete. Install PT binaries to see full functionality.")
		return
	}
	fmt.Printf("   Total registered: %d PTs\n\n", len(clients))

	// 4. Start all PTs (with monitoring)
	fmt.Println("4. Starting PTs (monitoring enabled)...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := mgr.StartAll(ctx); err != nil {
		log.Printf("   Some PTs failed to start: %v", err)
		fmt.Println("   (Manager will auto-restart failed PTs)")
	} else {
		fmt.Println("   ✓ All PTs started successfully")
	}
	fmt.Println()

	// 5. List active PTs
	fmt.Println("5. Active PTs:")
	for _, name := range mgr.Clients() {
		client, _ := mgr.GetClient(name)
		if client.IsRunning() {
			methods := client.Methods()
			fmt.Printf("   ✓ %s (methods: %v)\n", name, methods)
		} else {
			fmt.Printf("   ✗ %s (not running)\n", name)
		}
	}
	fmt.Println()

	// 6. Example: Using a PT for connections
	fmt.Println("6. PT usage example:")
	if len(clients) > 0 {
		firstPT := clients[0]
		fmt.Printf("   To connect through %s:\n", firstPT)
		fmt.Printf("   client, _ := mgr.GetClient(\"%s\")\n", firstPT)
		fmt.Printf("   conn, err := client.Dial(ctx, \"bridge.example.com:443\")\n")
		fmt.Printf("   if err != nil { ... }\n")
		fmt.Printf("   defer conn.Close()\n")
	}
	fmt.Println()

	// 7. Monitor for a bit
	fmt.Println("7. Monitoring PTs (press Ctrl+C to exit)...")
	fmt.Println("   - Manager will restart crashed PTs automatically")
	fmt.Println("   - Max restarts: 3 per PT")
	fmt.Println()

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sigChan:
			fmt.Println("\n8. Shutting down...")
			fmt.Println("   Stopping all PTs...")
			if err := mgr.Close(); err != nil {
				log.Printf("   Error during shutdown: %v", err)
			}
			fmt.Println("   ✓ All PTs stopped")
			fmt.Println("\n=== Demo Complete ===")
			return

		case <-ticker.C:
			// Periodic status check
			running := 0
			for _, name := range mgr.Clients() {
				client, _ := mgr.GetClient(name)
				if client.IsRunning() {
					running++
				}
			}
			fmt.Printf("   Status: %d/%d PTs running\n", running, len(clients))
		}
	}
}

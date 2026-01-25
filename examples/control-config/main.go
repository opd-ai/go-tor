// Example demonstrating GETCONF/SETCONF control protocol commands
package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/opd-ai/go-tor/pkg/client"
	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/logger"
)

func main() {
	fmt.Println("=== Control Protocol GETCONF/SETCONF Example ===")

	// Create configuration
	cfg := config.DefaultConfig()
	cfg.SocksPort = 19050
	cfg.ControlPort = 19051
	cfg.LogLevel = "info"
	cfg.EnableMetrics = true
	cfg.MetricsPort = 19090

	// Create Tor client
	log := logger.NewDefault()
	torClient, err := client.New(cfg, log)
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		return
	}

	// Start the client (this starts control server but we won't wait for full startup)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		torClient.Start(ctx)
	}()

	// Give the control server time to start
	time.Sleep(100 * time.Millisecond)

	// Connect to control port
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.ControlPort))
	if err != nil {
		fmt.Printf("Failed to connect to control port: %v\n", err)
		return
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Read greeting
	greeting, _ := reader.ReadString('\n')
	fmt.Printf("Server greeting: %s", greeting)

	// Authenticate (no password in this example)
	fmt.Println("\n--- Authentication ---")
	conn.Write([]byte("AUTHENTICATE\r\n"))
	authResp, _ := reader.ReadString('\n')
	fmt.Printf("Auth response: %s", authResp)

	// Get single configuration value
	fmt.Println("\n--- GETCONF Single Key ---")
	conn.Write([]byte("GETCONF SocksPort\r\n"))
	resp, _ := reader.ReadString('\n')
	fmt.Printf("GETCONF SocksPort: %s", resp)

	// Get multiple configuration values
	fmt.Println("\n--- GETCONF Multiple Keys ---")
	conn.Write([]byte("GETCONF SocksPort ControlPort LogLevel\r\n"))
	for i := 0; i < 3; i++ {
		resp, _ = reader.ReadString('\n')
		fmt.Printf("  %s", resp)
	}

	// Get current log level
	fmt.Println("\n--- Get Current LogLevel ---")
	conn.Write([]byte("GETCONF LogLevel\r\n"))
	resp, _ = reader.ReadString('\n')
	fmt.Printf("Current: %s", resp)

	// Set configuration value (LogLevel is writable)
	fmt.Println("\n--- SETCONF LogLevel ---")
	conn.Write([]byte("SETCONF LogLevel=debug\r\n"))
	resp, _ = reader.ReadString('\n')
	fmt.Printf("SETCONF response: %s", resp)

	// Verify the change
	conn.Write([]byte("GETCONF LogLevel\r\n"))
	resp, _ = reader.ReadString('\n')
	fmt.Printf("After change: %s", resp)

	// Try to set a read-only value
	fmt.Println("\n--- Attempt to Set Read-Only Key ---")
	conn.Write([]byte("SETCONF SocksPort=9999\r\n"))
	resp, _ = reader.ReadString('\n')
	fmt.Printf("SETCONF SocksPort (read-only): %s", resp)

	// Get unknown configuration key
	fmt.Println("\n--- Get Unknown Key ---")
	conn.Write([]byte("GETCONF UnknownKey\r\n"))
	resp, _ = reader.ReadString('\n')
	fmt.Printf("GETCONF UnknownKey: %s", resp)

	// Demonstrate PROTOCOLINFO with auth methods
	fmt.Println("\n--- PROTOCOLINFO ---")
	conn.Write([]byte("PROTOCOLINFO\r\n"))
	for {
		resp, _ = reader.ReadString('\n')
		fmt.Printf("  %s", resp)
		if strings.HasPrefix(resp, "250 ") {
			break
		}
	}

	// Close connection
	fmt.Println("\n--- Closing Connection ---")
	conn.Write([]byte("QUIT\r\n"))
	resp, _ = reader.ReadString('\n')
	fmt.Printf("QUIT response: %s", resp)

	// Give time for cleanup
	time.Sleep(100 * time.Millisecond)

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("\nSummary:")
	fmt.Println("✓ GETCONF retrieves configuration values")
	fmt.Println("✓ SETCONF updates writable configuration values")
	fmt.Println("✓ Read-only config values (ports, etc.) cannot be changed")
	fmt.Println("✓ Unknown keys return empty values per control-spec.txt")
	fmt.Println("✓ All commands require authentication")
}

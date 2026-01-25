// Package main demonstrates enhanced GETCONF/SETCONF control protocol functionality.
// This example shows how to query and modify Tor client configuration at runtime
// using the control protocol.
package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"github.com/opd-ai/go-tor/pkg/client"
	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/logger"
)

func main() {
	fmt.Println("=== Enhanced GETCONF/SETCONF Example ===")
	fmt.Println()

	// Create logger
	log := logger.New(slog.LevelInfo, os.Stdout)

	// Create configuration with specific settings
	cfg := &config.Config{
		SocksPort:                 9050,
		ControlPort:               9051,
		DataDirectory:             "/tmp/tor-getconf-example",
		CircuitBuildTimeout:       60 * time.Second,
		MaxCircuitDirtiness:       10 * time.Minute,
		NewCircuitPeriod:          30 * time.Second,
		EnableCircuitPadding:      true,
		PaddingStrategy:           "random",
		PaddingMinInterval:        3 * time.Second,
		PaddingMaxInterval:        10 * time.Second,
		SOCKSConnectionsPerSecond: 100.0,
		SOCKSConnectionsBurst:     50,
		EnableRateLimiting:        true,
		LogLevel:                  "info",
		TracingSampleRate:         0.1,
		EnableMemoryMonitoring:    false,
	}

	// Create and start client
	fmt.Println("Starting Tor client with control port on :9051...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	torClient, err := client.New(cfg, log)
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		os.Exit(1)
	}

	go func() {
		if err := torClient.Start(ctx); err != nil && err != context.Canceled {
			fmt.Printf("Client error: %v\n", err)
		}
	}()

	// Wait for control port to be ready
	time.Sleep(2 * time.Second)

	// Connect to control port
	fmt.Println("Connecting to control port...")
	conn, err := net.Dial("tcp", "127.0.0.1:9051")
	if err != nil {
		fmt.Printf("Failed to connect to control port: %v\n", err)
		torClient.Stop()
		os.Exit(1)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Authenticate (no password in this example)
	fmt.Println("Authenticating...")
	fmt.Fprintf(conn, "AUTHENTICATE\r\n")
	readResponse(reader)

	fmt.Println()
	fmt.Println("=== GETCONF Examples ===")
	fmt.Println()

	// Example 1: Query single configuration value
	fmt.Println("1. Query SocksPort:")
	fmt.Fprintf(conn, "GETCONF SocksPort\r\n")
	readResponse(reader)
	fmt.Println()

	// Example 2: Query multiple configuration values
	fmt.Println("2. Query multiple circuit settings:")
	fmt.Fprintf(conn, "GETCONF CircuitBuildTimeout MaxCircuitDirtiness NewCircuitPeriod\r\n")
	readResponse(reader)
	fmt.Println()

	// Example 3: Query padding configuration
	fmt.Println("3. Query circuit padding configuration:")
	fmt.Fprintf(conn, "GETCONF EnableCircuitPadding PaddingStrategy PaddingMinInterval PaddingMaxInterval\r\n")
	readResponse(reader)
	fmt.Println()

	// Example 4: Query rate limiting configuration
	fmt.Println("4. Query rate limiting settings:")
	fmt.Fprintf(conn, "GETCONF EnableRateLimiting SOCKSConnectionsPerSecond SOCKSConnectionsBurst\r\n")
	readResponse(reader)
	fmt.Println()

	// Example 5: Query tracing configuration
	fmt.Println("5. Query distributed tracing settings:")
	fmt.Fprintf(conn, "GETCONF EnableTracing TracingSampleRate\r\n")
	readResponse(reader)
	fmt.Println()

	fmt.Println("=== SETCONF Examples ===")
	fmt.Println()

	// Example 6: Modify circuit build timeout
	fmt.Println("6. Increase circuit build timeout to 90 seconds:")
	fmt.Fprintf(conn, "SETCONF CircuitBuildTimeout=90s\r\n")
	readResponse(reader)
	fmt.Fprintf(conn, "GETCONF CircuitBuildTimeout\r\n")
	readResponse(reader)
	fmt.Println()

	// Example 7: Change padding strategy
	fmt.Println("7. Change padding strategy to adaptive:")
	fmt.Fprintf(conn, "SETCONF PaddingStrategy=adaptive\r\n")
	readResponse(reader)
	fmt.Fprintf(conn, "GETCONF PaddingStrategy\r\n")
	readResponse(reader)
	fmt.Println()

	// Example 8: Adjust rate limiting
	fmt.Println("8. Adjust SOCKS connection rate limit to 50/second:")
	fmt.Fprintf(conn, "SETCONF SOCKSConnectionsPerSecond=50.0\r\n")
	readResponse(reader)
	fmt.Fprintf(conn, "GETCONF SOCKSConnectionsPerSecond\r\n")
	readResponse(reader)
	fmt.Println()

	// Example 9: Configure multiple settings at once
	fmt.Println("9. Configure multiple settings simultaneously:")
	fmt.Fprintf(conn, "SETCONF MaxCircuitDirtiness=15m NewCircuitPeriod=45s\r\n")
	readResponse(reader)
	fmt.Fprintf(conn, "GETCONF MaxCircuitDirtiness NewCircuitPeriod\r\n")
	readResponse(reader)
	fmt.Println()

	// Example 10: Enable memory monitoring
	fmt.Println("10. Enable memory monitoring:")
	fmt.Fprintf(conn, "SETCONF EnableMemoryMonitoring=1\r\n")
	readResponse(reader)
	fmt.Fprintf(conn, "GETCONF EnableMemoryMonitoring\r\n")
	readResponse(reader)
	fmt.Println()

	// Example 11: Attempt to modify setting that requires restart
	fmt.Println("11. Attempt to modify SocksPort (should fail - requires restart):")
	fmt.Fprintf(conn, "SETCONF SocksPort=9999\r\n")
	readResponse(reader)
	fmt.Println()

	// Example 12: Boolean value variations
	fmt.Println("12. Test different boolean value formats:")
	fmt.Fprintf(conn, "SETCONF EnableCircuitPadding=yes\r\n")
	readResponse(reader)
	fmt.Fprintf(conn, "SETCONF EnableCircuitPadding=TRUE\r\n")
	readResponse(reader)
	fmt.Fprintf(conn, "SETCONF EnableCircuitPadding=0\r\n")
	readResponse(reader)
	fmt.Fprintf(conn, "GETCONF EnableCircuitPadding\r\n")
	readResponse(reader)
	fmt.Println()

	fmt.Println("=== Summary ===")
	fmt.Println()
	fmt.Println("This example demonstrated:")
	fmt.Println("- Querying 70+ configuration parameters via GETCONF")
	fmt.Println("- Modifying 20+ runtime-configurable parameters via SETCONF")
	fmt.Println("- Flexible boolean value parsing (1/0, true/false, yes/no)")
	fmt.Println("- Parameter validation and constraint enforcement")
	fmt.Println("- Proper error handling for settings requiring restart")
	fmt.Println()
	fmt.Println("See docs/CONTROL_PROTOCOL_CONFIG.md for complete reference.")

	// Cleanup
	torClient.Stop()
	fmt.Println()
	fmt.Println("Example completed successfully!")
}

func readResponse(reader *bufio.Reader) {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading response: %v\n", err)
			return
		}

		line = strings.TrimSpace(line)
		fmt.Printf("  %s\n", line)

		// Check for end of response
		if strings.HasPrefix(line, "250 ") || strings.HasPrefix(line, "552 ") ||
			strings.HasPrefix(line, "514 ") || strings.HasPrefix(line, "551 ") {
			break
		}
	}
}

// Example demonstrating enhanced GETINFO command coverage in control protocol
package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
)

func main() {
	fmt.Println("Control GETINFO demo - demonstrating enhanced monitoring capabilities")
	fmt.Println("Note: This example requires a running go-tor client")
	fmt.Println()

	// Connect to control port
	fmt.Println("Attempting to connect to control port (127.0.0.1:9051)...")
	fmt.Println("Make sure to start the Tor client first with:")
	fmt.Println("  go run cmd/go-tor/main.go")
	fmt.Println()

	conn, err := net.DialTimeout("tcp", "127.0.0.1:9051", 5*time.Second)
	if err != nil {
		fmt.Printf("Cannot connect to control port (this is expected if client not running)\n")
		fmt.Printf("Error: %v\n\n", err)
		fmt.Println("This example demonstrates the GETINFO command interface.")
		fmt.Println("To test with a live client:")
		fmt.Println("  1. Start the go-tor client: go run cmd/go-tor/main.go")
		fmt.Println("  2. Run this example in another terminal")
		fmt.Println("\nExample GETINFO commands you can use:")
		printExampleCommands()
		return
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read greeting
	greeting, _ := reader.ReadString('\n')
	fmt.Printf("Server greeting: %s", greeting)

	// Authenticate
	writer.WriteString("AUTHENTICATE\r\n")
	writer.Flush()
	auth, _ := reader.ReadString('\n')
	fmt.Printf("Authentication: %s", auth)

	// Demonstrate various GETINFO keys
	fmt.Println("\n=== Basic Information ===")
	queryInfo(reader, writer, "version")
	queryInfo(reader, writer, "status/circuit-established")
	queryInfo(reader, writer, "status/enough-dir-info")

	fmt.Println("\n=== Circuit Statistics ===")
	queryInfo(reader, writer, "status/circuits")
	queryInfo(reader, writer, "status/circuit-builds")
	queryInfo(reader, writer, "status/circuit-build-success")
	queryInfo(reader, writer, "status/circuit-build-failure")

	fmt.Println("\n=== Guard Statistics ===")
	queryInfo(reader, writer, "status/guards/active")
	queryInfo(reader, writer, "status/guards/confirmed")

	fmt.Println("\n=== Network Information ===")
	queryInfo(reader, writer, "net/listeners/socks")
	queryInfo(reader, writer, "net/listeners/control")
	queryInfo(reader, writer, "status/connection-attempts")

	fmt.Println("\n=== System Information ===")
	queryInfo(reader, writer, "status/uptime")
	queryInfo(reader, writer, "config-file")

	fmt.Println("\n=== Multiple Keys at Once ===")
	queryMultiple(reader, writer, []string{
		"status/circuits",
		"status/guards/active",
		"status/uptime",
	})

	fmt.Println("\n=== Available Keys (info/names) ===")
	queryInfo(reader, writer, "info/names")

	fmt.Println("\n=== Traffic Statistics ===")
	queryInfo(reader, writer, "traffic/read")
	queryInfo(reader, writer, "traffic/written")

	fmt.Println("\nDemo completed successfully!")
}

func printExampleCommands() {
	commands := []string{
		"GETINFO version",
		"GETINFO status/circuits",
		"GETINFO status/circuit-builds",
		"GETINFO status/guards/active",
		"GETINFO status/uptime",
		"GETINFO net/listeners/socks",
		"GETINFO info/names",
	}

	fmt.Println("\nBasic Information:")
	for _, cmd := range commands {
		fmt.Printf("  %s\n", cmd)
	}

	fmt.Println("\nMultiple keys at once:")
	fmt.Println("  GETINFO status/circuits status/guards/active status/uptime")
}

// queryInfo sends a GETINFO command for a single key and displays the result
func queryInfo(reader *bufio.Reader, writer *bufio.Writer, key string) {
	writer.WriteString(fmt.Sprintf("GETINFO %s\r\n", key))
	writer.Flush()

	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(response)

	// Parse the response
	if strings.HasPrefix(response, "250 ") {
		// Extract value
		parts := strings.SplitN(response, "=", 2)
		if len(parts) == 2 {
			fmt.Printf("  %-35s %s\n", key+":", parts[1])
		} else {
			fmt.Printf("  %-35s %s\n", key+":", response)
		}
	} else {
		fmt.Printf("  %-35s ERROR: %s\n", key+":", response)
	}
}

// queryMultiple sends a GETINFO command for multiple keys and displays results
func queryMultiple(reader *bufio.Reader, writer *bufio.Writer, keys []string) {
	writer.WriteString(fmt.Sprintf("GETINFO %s\r\n", strings.Join(keys, " ")))
	writer.Flush()

	fmt.Printf("Requesting: %s\n", strings.Join(keys, ", "))

	// Read multi-line response
	for {
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(response)

		// Parse and display
		if strings.HasPrefix(response, "250-") || strings.HasPrefix(response, "250 ") {
			parts := strings.SplitN(response[4:], "=", 2)
			if len(parts) == 2 {
				fmt.Printf("  %-35s %s\n", parts[0]+":", parts[1])
			}

			// Last line
			if strings.HasPrefix(response, "250 ") {
				break
			}
		} else {
			fmt.Printf("  ERROR: %s\n", response)
			break
		}
	}
}

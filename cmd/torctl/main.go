// Package main provides a control utility for interacting with a running go-tor client.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

var (
	version   = "0.1.0-dev"
	buildTime = "unknown"
)

func main() {
	// Parse command-line flags
	controlAddr := flag.String("control", "127.0.0.1:9051", "Control protocol address")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("torctl version %s (built %s)\n", version, buildTime)
		fmt.Println("Control utility for go-tor client")
		os.Exit(0)
	}

	// Get command from arguments
	if len(flag.Args()) == 0 {
		printUsage()
		os.Exit(1)
	}

	command := flag.Args()[0]

	// Execute command
	if err := executeCommand(command, *controlAddr, flag.Args()[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("torctl - Control utility for go-tor client")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  torctl [options] <command> [args...]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -control <address>  Control protocol address (default: 127.0.0.1:9051)")
	fmt.Println("  -version            Show version information")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  status              Show current client status")
	fmt.Println("  circuits            List active circuits")
	fmt.Println("  streams             List active streams")
	fmt.Println("  info                Show detailed client information")
	fmt.Println("  config <key>        Get configuration value")
	fmt.Println("  signal <signal>     Send signal to client (SHUTDOWN, RELOAD, etc.)")
	fmt.Println("  version             Show client version")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  torctl status")
	fmt.Println("  torctl circuits")
	fmt.Println("  torctl signal SHUTDOWN")
	fmt.Println("  torctl config SocksPort")
}

func executeCommand(command, controlAddr string, args []string) error {
	// Validate arguments before connecting
	switch strings.ToLower(command) {
	case "config":
		if len(args) == 0 {
			return fmt.Errorf("config command requires a key argument")
		}
	case "signal":
		if len(args) == 0 {
			return fmt.Errorf("signal command requires a signal name")
		}
	case "status", "circuits", "streams", "info", "version":
		// These commands don't require arguments
	default:
		return fmt.Errorf("unknown command: %s", command)
	}

	// Connect to control port
	conn, err := connectControl(controlAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to control port: %w", err)
	}
	defer conn.Close()

	// Authenticate
	if err := authenticate(conn); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Execute specific command
	switch strings.ToLower(command) {
	case "status":
		return showStatus(conn)
	case "circuits":
		return listCircuits(conn)
	case "streams":
		return listStreams(conn)
	case "info":
		return showInfo(conn)
	case "config":
		return getConfig(conn, args[0])
	case "signal":
		return sendSignal(conn, args[0])
	case "version":
		return showVersion(conn)
	default:
		// Should never reach here due to validation above
		return fmt.Errorf("unknown command: %s", command)
	}
}

func connectControl(addr string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func authenticate(conn net.Conn) error {
	// Simple null authentication for now
	if _, err := fmt.Fprintf(conn, "AUTHENTICATE\r\n"); err != nil {
		return err
	}

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	if !strings.HasPrefix(response, "250") {
		return fmt.Errorf("authentication failed: %s", strings.TrimSpace(response))
	}

	return nil
}

func sendCommand(conn net.Conn, command string) ([]string, error) {
	if _, err := fmt.Fprintf(conn, "%s\r\n", command); err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)
	var lines []string
	
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		
		line = strings.TrimSpace(line)
		lines = append(lines, line)
		
		// Check for end of response
		if strings.HasPrefix(line, "250 ") {
			break
		}
		if strings.HasPrefix(line, "250-") {
			continue
		}
		if strings.HasPrefix(line, "5") {
			return lines, fmt.Errorf("command failed: %s", line)
		}
	}
	
	return lines, nil
}

func showStatus(conn net.Conn) error {
	fmt.Println("=== Tor Client Status ===")
	fmt.Println()

	// Get circuit count
	circuits, err := sendCommand(conn, "GETINFO circuit-status")
	if err != nil {
		return err
	}

	activeCircuits := 0
	for _, line := range circuits {
		if strings.HasPrefix(line, "250-") || strings.HasPrefix(line, "250+") {
			activeCircuits++
		}
	}

	fmt.Printf("Active Circuits: %d\n", activeCircuits)

	// Get stream count
	streams, err := sendCommand(conn, "GETINFO stream-status")
	if err != nil {
		return err
	}

	activeStreams := 0
	for _, line := range streams {
		if strings.HasPrefix(line, "250-") || strings.HasPrefix(line, "250+") {
			activeStreams++
		}
	}

	fmt.Printf("Active Streams: %d\n", activeStreams)

	// Get traffic stats
	traffic, err := sendCommand(conn, "GETINFO traffic/read traffic/written")
	if err == nil && len(traffic) > 0 {
		fmt.Println()
		fmt.Println("Traffic Statistics:")
		for _, line := range traffic {
			if strings.HasPrefix(line, "250-") {
				parts := strings.SplitN(line[4:], "=", 2)
				if len(parts) == 2 {
					fmt.Printf("  %s: %s bytes\n", parts[0], parts[1])
				}
			}
		}
	}

	fmt.Println()
	fmt.Println("Status: Running")
	
	return nil
}

func listCircuits(conn net.Conn) error {
	fmt.Println("=== Active Circuits ===")
	fmt.Println()

	circuits, err := sendCommand(conn, "GETINFO circuit-status")
	if err != nil {
		return err
	}

	if len(circuits) <= 1 {
		fmt.Println("No active circuits")
		return nil
	}

	for _, line := range circuits {
		if strings.HasPrefix(line, "250-circuit-status=") {
			line = strings.TrimPrefix(line, "250-circuit-status=")
		} else if strings.HasPrefix(line, "250+circuit-status=") {
			continue
		} else if strings.HasPrefix(line, "250 ") {
			break
		}
		
		// Parse circuit line format: ID STATUS PATH
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			fmt.Printf("Circuit %s: %s\n", parts[0], parts[1])
			if len(parts) >= 3 {
				fmt.Printf("  Path: %s\n", parts[2])
			}
		}
	}

	return nil
}

func listStreams(conn net.Conn) error {
	fmt.Println("=== Active Streams ===")
	fmt.Println()

	streams, err := sendCommand(conn, "GETINFO stream-status")
	if err != nil {
		return err
	}

	if len(streams) <= 1 {
		fmt.Println("No active streams")
		return nil
	}

	for _, line := range streams {
		if strings.HasPrefix(line, "250-stream-status=") {
			line = strings.TrimPrefix(line, "250-stream-status=")
		} else if strings.HasPrefix(line, "250+stream-status=") {
			continue
		} else if strings.HasPrefix(line, "250 ") {
			break
		}
		
		// Parse stream line
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			fmt.Printf("Stream %s: %s -> %s\n", parts[0], parts[1], parts[2])
		}
	}

	return nil
}

func showInfo(conn net.Conn) error {
	fmt.Println("=== Tor Client Information ===")
	fmt.Println()

	// Get version
	version, err := sendCommand(conn, "GETINFO version")
	if err == nil && len(version) > 0 {
		for _, line := range version {
			if strings.Contains(line, "version=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					fmt.Printf("Version: %s\n", parts[1])
				}
			}
		}
	}

	// Get SOCKS port
	socksPort, err := sendCommand(conn, "GETINFO net/listeners/socks")
	if err == nil && len(socksPort) > 0 {
		fmt.Println()
		fmt.Println("Network Listeners:")
		for _, line := range socksPort {
			if strings.Contains(line, "net/listeners/socks=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					fmt.Printf("  SOCKS: %s\n", parts[1])
				}
			}
		}
	}

	// Get data directory
	dataDir, err := sendCommand(conn, "GETINFO config-file")
	if err == nil && len(dataDir) > 0 {
		fmt.Println()
		for _, line := range dataDir {
			if strings.Contains(line, "config-file=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					fmt.Printf("Config File: %s\n", parts[1])
				}
			}
		}
	}

	return nil
}

func getConfig(conn net.Conn, key string) error {
	response, err := sendCommand(conn, fmt.Sprintf("GETCONF %s", key))
	if err != nil {
		return err
	}

	fmt.Printf("Configuration: %s\n", key)
	fmt.Println()

	for _, line := range response {
		if strings.HasPrefix(line, "250-") || strings.HasPrefix(line, "250 ") {
			config := strings.TrimPrefix(line, "250-")
			config = strings.TrimPrefix(config, "250 ")
			fmt.Println(config)
		}
	}

	return nil
}

func sendSignal(conn net.Conn, signal string) error {
	signal = strings.ToUpper(signal)
	
	response, err := sendCommand(conn, fmt.Sprintf("SIGNAL %s", signal))
	if err != nil {
		return err
	}

	for _, line := range response {
		if strings.HasPrefix(line, "250") {
			fmt.Printf("Signal %s sent successfully\n", signal)
			return nil
		}
	}

	return fmt.Errorf("unexpected response")
}

func showVersion(conn net.Conn) error {
	response, err := sendCommand(conn, "GETINFO version")
	if err != nil {
		return err
	}

	for _, line := range response {
		if strings.Contains(line, "version=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				fmt.Println(parts[1])
				return nil
			}
		}
	}

	return fmt.Errorf("version information not found")
}

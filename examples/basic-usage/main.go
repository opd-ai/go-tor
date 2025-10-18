// Example: Simple usage of go-tor packages
package main

import (
	"bytes"
	"fmt"
	"log"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/config"
)

func main() {
	fmt.Println("=== go-tor Package Examples ===\n")

	// Example 1: Working with cells
	fmt.Println("1. Cell Encoding/Decoding")
	demonstrateCells()
	fmt.Println()

	// Example 2: Circuit management
	fmt.Println("2. Circuit Management")
	demonstrateCircuits()
	fmt.Println()

	// Example 3: Configuration
	fmt.Println("3. Configuration")
	demonstrateConfig()
}

func demonstrateCells() {
	// Create a CREATE2 cell
	circID := uint32(12345)
	createCell := cell.NewCell(circID, cell.CmdCreate2)
	createCell.Payload = []byte("handshake data here")

	// Encode the cell
	var buf bytes.Buffer
	if err := createCell.Encode(&buf); err != nil {
		log.Fatalf("Failed to encode cell: %v", err)
	}
	fmt.Printf("  Created %s cell (ID: %d, size: %d bytes)\n",
		createCell.Command.String(), createCell.CircID, buf.Len())

	// Decode the cell
	decoded, err := cell.DecodeCell(&buf)
	if err != nil {
		log.Fatalf("Failed to decode cell: %v", err)
	}
	fmt.Printf("  Decoded cell: %s (ID: %d)\n", decoded.Command.String(), decoded.CircID)

	// Create a relay cell
	relayCell := cell.NewRelayCell(42, cell.RelayBegin, []byte("www.example.com:80"))
	payload, err := relayCell.Encode()
	if err != nil {
		log.Fatalf("Failed to encode relay cell: %v", err)
	}
	fmt.Printf("  Created relay cell: %s (stream: %d, payload size: %d)\n",
		cell.RelayCmdString(relayCell.Command), relayCell.StreamID, len(payload))
}

func demonstrateCircuits() {
	// Create a circuit manager
	manager := circuit.NewManager()

	// Create some circuits
	for i := 0; i < 3; i++ {
		circ, err := manager.CreateCircuit()
		if err != nil {
			log.Fatalf("Failed to create circuit: %v", err)
		}

		// Add hops to the circuit
		circ.AddHop(&circuit.Hop{
			Fingerprint: fmt.Sprintf("GUARD%d", i),
			Address:     fmt.Sprintf("10.0.0.%d:9001", i+1),
			IsGuard:     true,
		})
		circ.AddHop(&circuit.Hop{
			Fingerprint: fmt.Sprintf("MIDDLE%d", i),
			Address:     fmt.Sprintf("10.0.1.%d:9001", i+1),
		})
		circ.AddHop(&circuit.Hop{
			Fingerprint: fmt.Sprintf("EXIT%d", i),
			Address:     fmt.Sprintf("10.0.2.%d:9001", i+1),
			IsExit:      true,
		})

		// Mark circuit as open
		circ.SetState(circuit.StateOpen)

		fmt.Printf("  Circuit %d: %s, %d hops\n", circ.ID, circ.GetState(), circ.Length())
	}

	// List all circuits
	circuits := manager.ListCircuits()
	fmt.Printf("  Total circuits: %d\n", len(circuits))
}

func demonstrateConfig() {
	// Create default configuration
	cfg := config.DefaultConfig()

	fmt.Printf("  SOCKS Port: %d\n", cfg.SocksPort)
	fmt.Printf("  Control Port: %d\n", cfg.ControlPort)
	fmt.Printf("  Use Entry Guards: %v\n", cfg.UseEntryGuards)
	fmt.Printf("  Circuit Build Timeout: %s\n", cfg.CircuitBuildTimeout)

	// Customize configuration
	cfg.SocksPort = 9150
	cfg.LogLevel = "debug"
	cfg.OnionServices = []config.OnionServiceConfig{
		{
			ServiceDir:  "/var/lib/tor/hidden_service",
			VirtualPort: 80,
			TargetAddr:  "127.0.0.1:8080",
		},
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}
	fmt.Println("  Configuration validated successfully")
}

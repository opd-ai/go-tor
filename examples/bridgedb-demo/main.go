// Example demonstrating BridgeDB integration for educational purposes
package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/relay"
)

func main() {
	// WARNING: This is an educational implementation only!
	fmt.Println("=== BridgeDB Integration Demo (Educational Only) ===")
	fmt.Println("WARNING: Do NOT use for real anonymity needs!")
	fmt.Println()

	// Create logger
	log := logger.New(slog.LevelInfo, os.Stdout)

	// Create bridge distributor
	config := relay.DefaultDistributorConfig()
	distributor := relay.NewBridgeDistributor(config, log)

	// Add some example bridges
	bridges := []*relay.BridgeInfo{
		{
			Fingerprint: "A1B2C3D4E5F6A7B8C9D0E1F2A3B4C5D6E7F8A9B0",
			Address:     "192.0.2.1:9001",
			Transport:   "vanilla",
		},
		{
			Fingerprint: "B2C3D4E5F6A7B8C9D0E1F2A3B4C5D6E7F8A9B0C1",
			Address:     "192.0.2.2:9002",
			Transport:   "obfs4",
			Params:      "cert=abcd1234;iat-mode=0",
		},
		{
			Fingerprint: "C3D4E5F6A7B8C9D0E1F2A3B4C5D6E7F8A9B0C1D2",
			Address:     "192.0.2.3:9003",
			Transport:   "obfs4",
			Params:      "cert=efgh5678;iat-mode=1",
		},
		{
			Fingerprint: "D4E5F6A7B8C9D0E1F2A3B4C5D6E7F8A9B0C1D2E3",
			Address:     "192.0.2.4:9004",
			Transport:   "meek_lite",
			Params:      "url=https://example.com",
		},
	}

	fmt.Println("Adding bridges to distributor:")
	for _, bridge := range bridges {
		err := distributor.AddBridge(bridge)
		if err != nil {
			log.Error("Failed to add bridge", "error", err)
			continue
		}
		fmt.Printf("  ✓ Added %s bridge at %s\n", bridge.Transport, bridge.Address)
	}
	fmt.Println()

	// Show statistics
	fmt.Println("Bridge Distribution Statistics:")
	stats := distributor.GetStats()
	statsJSON, _ := json.MarshalIndent(stats, "  ", "  ")
	fmt.Printf("  %s\n", statsJSON)
	fmt.Println()

	// Demonstrate email responder
	emailResponder := relay.NewEmailResponder(distributor, log)

	fmt.Println("Email Responder Demo:")
	response, err := emailResponder.GenerateEmailResponse("user@example.com", "obfs4")
	if err != nil {
		log.Error("Failed to generate email response", "error", err)
	} else {
		fmt.Println("Email response for user@example.com:")
		fmt.Println("---")
		fmt.Println(response)
		fmt.Println("---")
	}
	fmt.Println()

	// Create HTTP server
	server := relay.NewBridgeDistributorServer(distributor, log)

	fmt.Println("Starting BridgeDB HTTP server on :8080")
	fmt.Println("Available endpoints:")
	fmt.Println("  GET /bridges          - Get all bridges")
	fmt.Println("  GET /bridges?transport=obfs4&count=2 - Get obfs4 bridges")
	fmt.Println("  GET /stats            - Get distribution statistics")
	fmt.Println()
	fmt.Println("Example: curl http://localhost:8080/bridges?transport=obfs4")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop")

	if err := http.ListenAndServe(":8080", server); err != nil {
		log.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
}

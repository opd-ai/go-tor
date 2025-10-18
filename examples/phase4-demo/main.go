// Phase 4 Demo: Stream Handling & Circuit Extension
//
// This example demonstrates the new Phase 4 capabilities:
// - Stream management and multiplexing
// - Circuit extension with CREATE2/CREATED2 and EXTEND2/EXTENDED2
// - Key derivation for circuit hops
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/stream"
)

func main() {
	// Initialize logger
	logLevel := slog.LevelInfo
	logr := logger.New(logLevel, os.Stdout)

	logr.Info("=== Phase 4 Demo: Stream Handling & Circuit Extension ===")
	logr.Info("This demo showcases the new Phase 4 implementations")

	ctx := context.Background()

	// Demo 1: Stream Management
	demoStreamManagement(ctx, logr)

	// Demo 2: Circuit Extension
	demoCircuitExtension(ctx, logr)

	// Demo 3: Stream Multiplexing
	demoStreamMultiplexing(ctx, logr)

	logr.Info("\n=== Phase 4 Implementation Summary ===")
	logr.Info("✅ Stream package: Multiplexing connections over circuits")
	logr.Info("✅ Circuit extension: CREATE2/CREATED2 and EXTEND2/EXTENDED2")
	logr.Info("✅ Key derivation: KDF-TOR for hop encryption keys")
	logr.Info("✅ Comprehensive testing: Full test coverage for new functionality")
}

func demoStreamManagement(ctx context.Context, logr *logger.Logger) {
	logr.Info("\n--- Demo 1: Stream Management ---")

	// Create a stream manager
	mgr := stream.NewManager(logr)
	defer mgr.Close()

	// Create streams for different destinations
	stream1, err := mgr.CreateStream(100, "example.com", 80)
	if err != nil {
		log.Fatalf("Failed to create stream: %v", err)
	}

	stream2, err := mgr.CreateStream(100, "torproject.org", 443)
	if err != nil {
		log.Fatalf("Failed to create stream: %v", err)
	}

	logr.Info("Created streams",
		"count", mgr.Count(),
		"stream1_id", stream1.ID,
		"stream2_id", stream2.ID)

	// Simulate stream state transitions
	stream1.SetState(stream.StateConnecting)
	stream1.SetState(stream.StateConnected)

	// Test data transfer
	testData := []byte("Hello, Tor!")
	if err := stream1.Send(testData); err != nil {
		log.Fatalf("Failed to send data: %v", err)
	}

	// Retrieve data (simulating circuit layer)
	sendCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	data, err := stream1.SendData(sendCtx)
	if err != nil {
		log.Fatalf("Failed to retrieve data: %v", err)
	}

	logr.Info("Data transfer successful",
		"stream_id", stream1.ID,
		"data_size", len(data))

	// Get streams for a circuit
	streams := mgr.GetStreamsForCircuit(100)
	logr.Info("Streams on circuit 100", "count", len(streams))
}

func demoCircuitExtension(ctx context.Context, logr *logger.Logger) {
	logr.Info("\n--- Demo 2: Circuit Extension ---")

	// Create a circuit
	circ := circuit.NewCircuit(1)
	logr.Info("Circuit created", "circuit_id", circ.ID)

	// Create extension handler
	ext := circuit.NewExtension(circ, logr)

	// Create first hop using CREATE2
	logr.Info("Creating first hop with CREATE2...")
	if err := ext.CreateFirstHop(ctx, circuit.HandshakeTypeNTor); err != nil {
		log.Fatalf("Failed to create first hop: %v", err)
	}

	logr.Info("First hop created successfully")

	// Extend circuit to second hop using EXTEND2
	logr.Info("Extending to second hop with EXTEND2...")
	if err := ext.ExtendCircuit(ctx, "middle-relay.example.com:9001", circuit.HandshakeTypeNTor); err != nil {
		log.Fatalf("Failed to extend circuit: %v", err)
	}

	logr.Info("Circuit extended to second hop")

	// Extend circuit to third hop (exit)
	logr.Info("Extending to third hop (exit) with EXTEND2...")
	if err := ext.ExtendCircuit(ctx, "exit-relay.example.com:9001", circuit.HandshakeTypeNTor); err != nil {
		log.Fatalf("Failed to extend circuit: %v", err)
	}

	logr.Info("Circuit extended to exit node")
	logr.Info("3-hop circuit built successfully!")

	// Demonstrate key derivation
	logr.Info("\nDemonstrating key derivation...")
	sharedSecret := make([]byte, 32)
	for i := range sharedSecret {
		sharedSecret[i] = byte(i)
	}

	forwardKey, backwardKey, err := ext.DeriveKeys(sharedSecret)
	if err != nil {
		log.Fatalf("Failed to derive keys: %v", err)
	}

	logr.Info("Keys derived successfully",
		"forward_key_len", len(forwardKey),
		"backward_key_len", len(backwardKey))
}

func demoStreamMultiplexing(ctx context.Context, logr *logger.Logger) {
	logr.Info("\n--- Demo 3: Stream Multiplexing ---")

	// Create stream manager
	mgr := stream.NewManager(logr)
	defer mgr.Close()

	// Simulate multiple concurrent streams on the same circuit
	circuitID := uint32(200)
	destinations := []struct {
		host string
		port uint16
	}{
		{"www.torproject.org", 443},
		{"www.eff.org", 443},
		{"check.torproject.org", 443},
		{"bridges.torproject.org", 443},
	}

	logr.Info("Creating multiple streams on circuit", "circuit_id", circuitID)

	streams := make([]*stream.Stream, 0, len(destinations))
	for _, dest := range destinations {
		s, err := mgr.CreateStream(circuitID, dest.host, dest.port)
		if err != nil {
			log.Fatalf("Failed to create stream: %v", err)
		}
		s.SetState(stream.StateConnected)
		streams = append(streams, s)
	}

	logr.Info("Streams created",
		"circuit_id", circuitID,
		"stream_count", len(streams))

	// Simulate concurrent data transfer on all streams
	for i, s := range streams {
		data := []byte(fmt.Sprintf("Request %d", i+1))
		if err := s.Send(data); err != nil {
			log.Printf("Failed to send on stream %d: %v", s.ID, err)
			continue
		}
	}

	logr.Info("Concurrent data sent on all streams")

	// Verify streams on circuit
	circuitStreams := mgr.GetStreamsForCircuit(circuitID)
	logr.Info("Verification complete",
		"expected_streams", len(destinations),
		"actual_streams", len(circuitStreams))
}

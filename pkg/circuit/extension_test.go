package circuit

import (
	"context"
	"testing"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestNewExtension(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	if ext == nil {
		t.Fatal("Expected extension to be created")
	}

	if ext.circuit.ID != 1 {
		t.Errorf("Expected circuit ID 1, got %d", ext.circuit.ID)
	}
}

func TestCreateFirstHop(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	ctx := context.Background()

	err := ext.CreateFirstHop(ctx, HandshakeTypeNTor)
	if err != nil {
		t.Fatalf("Failed to create first hop: %v", err)
	}
}

func TestCreateFirstHopTAP(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	ctx := context.Background()

	err := ext.CreateFirstHop(ctx, HandshakeTypeTAP)
	if err != nil {
		t.Fatalf("Failed to create first hop with TAP: %v", err)
	}
}

func TestExtendCircuit(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	ctx := context.Background()

	err := ext.ExtendCircuit(ctx, "relay.example.com:9001", HandshakeTypeNTor)
	if err != nil {
		t.Fatalf("Failed to extend circuit: %v", err)
	}
}

func TestGenerateHandshakeData(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	tests := []struct {
		name          string
		handshakeType HandshakeType
		expectedLen   int
	}{
		{"NTor", HandshakeTypeNTor, 32},
		{"TAP", HandshakeTypeTAP, 144},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := ext.generateHandshakeData(tt.handshakeType)
			if err != nil {
				t.Fatalf("Failed to generate handshake data: %v", err)
			}

			if len(data) != tt.expectedLen {
				t.Errorf("Expected %d bytes, got %d", tt.expectedLen, len(data))
			}
		})
	}
}

func TestGenerateHandshakeDataInvalidType(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	_, err := ext.generateHandshakeData(HandshakeType(0xFFFF))
	if err == nil {
		t.Error("Expected error for invalid handshake type")
	}
}

func TestBuildExtend2Data(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	handshakeData := make([]byte, 32)
	data := ext.buildExtend2Data("relay.example.com:9001", HandshakeTypeNTor, handshakeData)

	if len(data) == 0 {
		t.Error("Expected non-empty EXTEND2 data")
	}

	// Check NSPEC
	if data[0] != 1 {
		t.Errorf("Expected NSPEC=1, got %d", data[0])
	}
}

func TestProcessCreated2Valid(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	// Create a valid CREATED2 cell
	handshakeResponse := make([]byte, 32)
	payload := make([]byte, 2+len(handshakeResponse))
	payload[0] = 0
	payload[1] = 32 // hlen
	copy(payload[2:], handshakeResponse)

	created2Cell := &cell.Cell{
		CircID:  1,
		Command: cell.CmdCreated2,
		Payload: payload,
	}

	err := ext.ProcessCreated2(created2Cell)
	if err != nil {
		t.Fatalf("Failed to process CREATED2: %v", err)
	}
}

func TestProcessCreated2InvalidCommand(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	wrongCell := &cell.Cell{
		CircID:  1,
		Command: cell.CmdCreate2, // Wrong command
		Payload: make([]byte, 34),
	}

	err := ext.ProcessCreated2(wrongCell)
	if err == nil {
		t.Error("Expected error for wrong command")
	}
}

func TestProcessCreated2ShortPayload(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	shortCell := &cell.Cell{
		CircID:  1,
		Command: cell.CmdCreated2,
		Payload: make([]byte, 1), // Too short
	}

	err := ext.ProcessCreated2(shortCell)
	if err == nil {
		t.Error("Expected error for short payload")
	}
}

func TestProcessExtended2Valid(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	// Create a valid EXTENDED2 relay cell
	handshakeResponse := make([]byte, 32)
	payload := make([]byte, 2+len(handshakeResponse))
	payload[0] = 0
	payload[1] = 32 // hlen
	copy(payload[2:], handshakeResponse)

	extended2Cell := &cell.RelayCell{
		Command:  cell.RelayExtended2,
		StreamID: 0,
		Data:     payload,
	}

	err := ext.ProcessExtended2(extended2Cell)
	if err != nil {
		t.Fatalf("Failed to process EXTENDED2: %v", err)
	}
}

func TestProcessExtended2InvalidCommand(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	wrongCell := &cell.RelayCell{
		Command:  cell.RelayBegin, // Wrong command
		StreamID: 0,
		Data:     make([]byte, 34),
	}

	err := ext.ProcessExtended2(wrongCell)
	if err == nil {
		t.Error("Expected error for wrong command")
	}
}

func TestDeriveKeys(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	sharedSecret := make([]byte, 32)
	for i := range sharedSecret {
		sharedSecret[i] = byte(i)
	}

	forwardKey, backwardKey, err := ext.DeriveKeys(sharedSecret)
	if err != nil {
		t.Fatalf("Failed to derive keys: %v", err)
	}

	if len(forwardKey) != 16 {
		t.Errorf("Expected forward key length 16, got %d", len(forwardKey))
	}

	if len(backwardKey) != 16 {
		t.Errorf("Expected backward key length 16, got %d", len(backwardKey))
	}

	// Keys should be different
	if string(forwardKey) == string(backwardKey) {
		t.Error("Forward and backward keys should be different")
	}
}

func TestDeriveKeysEmptySecret(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	// Empty shared secret should still work (though not secure)
	sharedSecret := make([]byte, 0)

	_, _, err := ext.DeriveKeys(sharedSecret)
	if err != nil {
		t.Fatalf("Failed to derive keys with empty secret: %v", err)
	}
}

func TestHandshakeTypeConstants(t *testing.T) {
	if HandshakeTypeNTor != 0x0002 {
		t.Errorf("Expected HandshakeTypeNTor=0x0002, got 0x%04x", HandshakeTypeNTor)
	}

	if HandshakeTypeTAP != 0x0000 {
		t.Errorf("Expected HandshakeTypeTAP=0x0000, got 0x%04x", HandshakeTypeTAP)
	}
}

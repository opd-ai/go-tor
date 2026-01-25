package relay

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestParseLinkSpecifiers(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantCount int
		wantErr   bool
	}{
		{
			name: "single IPv4 specifier",
			data: []byte{
				1,                // NSPEC = 1
				0,                // Type = IPv4
				6,                // Length = 6
				192, 168, 1, 100, // IP
				0x1F, 0x90, // Port 8080
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "multiple specifiers",
			data: []byte{
				2,                // NSPEC = 2
				0,                // Type = IPv4
				6,                // Length = 6
				192, 168, 1, 100, // IP
				0x1F, 0x90, // Port 8080
				2,  // Type = Legacy ID
				20, // Length = 20
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, // 20 bytes
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "empty data",
			data:      []byte{},
			wantCount: 0,
			wantErr:   true,
		},
		{
			name: "truncated specifier",
			data: []byte{
				1, // NSPEC = 1
				0, // Type = IPv4
				6, // Length = 6
				// Missing data
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs, _, err := parseLinkSpecifiers(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseLinkSpecifiers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(specs) != tt.wantCount {
				t.Errorf("parseLinkSpecifiers() got %d specifiers, want %d", len(specs), tt.wantCount)
			}
		})
	}
}

func TestExtractAddressFromLinkSpecs(t *testing.T) {
	tests := []struct {
		name    string
		specs   []LinkSpecifier
		want    string
		wantErr bool
	}{
		{
			name: "IPv4 address",
			specs: []LinkSpecifier{
				{
					Type: 0, // IPv4
					Data: []byte{192, 168, 1, 100, 0x1F, 0x90}, // 192.168.1.100:8080
				},
			},
			want:    "192.168.1.100:8080",
			wantErr: false,
		},
		{
			name: "IPv6 address",
			specs: []LinkSpecifier{
				{
					Type: 1, // IPv6
					Data: append(
						net.ParseIP("2001:db8::1").To16(),
						[]byte{0x1F, 0x90}..., // Port 8080
					),
				},
			},
			want:    "[2001:db8::1]:8080",
			wantErr: false,
		},
		{
			name: "IPv4 preferred over IPv6",
			specs: []LinkSpecifier{
				{
					Type: 1, // IPv6
					Data: append(
						net.ParseIP("2001:db8::1").To16(),
						[]byte{0x1F, 0x90}...,
					),
				},
				{
					Type: 0, // IPv4
					Data: []byte{192, 168, 1, 100, 0x1F, 0x90},
				},
			},
			want:    "192.168.1.100:8080",
			wantErr: false,
		},
		{
			name: "no usable address",
			specs: []LinkSpecifier{
				{
					Type: 2, // Legacy ID (not an address)
					Data: make([]byte, 20),
				},
			},
			want:    "",
			wantErr: true,
		},
		{
			name:    "empty specifiers",
			specs:   []LinkSpecifier{},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractAddressFromLinkSpecs(tt.specs)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractAddressFromLinkSpecs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("extractAddressFromLinkSpecs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewExtensionHandler(t *testing.T) {
	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}

	log := logger.NewDefault()
	circuits := NewCircuitHandler(keys, log)

	handler := NewExtensionHandler(keys, circuits, log)
	if handler == nil {
		t.Error("NewExtensionHandler() returned nil")
	}

	if handler.keys != keys {
		t.Error("keys not set correctly")
	}

	if handler.circuits != circuits {
		t.Error("circuits not set correctly")
	}

	if handler.connPool == nil {
		t.Error("connPool not initialized")
	}
}

func TestExtensionHandlerClose(t *testing.T) {
	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}

	log := logger.NewDefault()
	circuits := NewCircuitHandler(keys, log)
	handler := NewExtensionHandler(keys, circuits, log)

	// Add a mock connection to the pool (simplified)
	// In a real test, would use a proper mock connection
	handler.connMutex.Lock()
	// handler.connPool["test"] = mockConn // Would need a mock
	handler.connMutex.Unlock()

	err = handler.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if len(handler.connPool) != 0 {
		t.Error("connPool not cleared after Close()")
	}
}

func TestHandleExtend2_ParseErrors(t *testing.T) {
	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}

	log := logger.NewDefault()
	circuits := NewCircuitHandler(keys, log)
	handler := NewExtensionHandler(keys, circuits, log)

	ctx := context.Background()
	circuitID := uint32(123)

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: true,
		},
		{
			name: "no link specifiers",
			data: []byte{
				0, // NSPEC = 0
				// No handshake data
			},
			wantErr: true,
		},
		{
			name: "truncated handshake header",
			data: []byte{
				1,             // NSPEC = 1
				0,             // Type = IPv4
				6,             // Length = 6
				127, 0, 0, 1,  // IP
				0x1F, 0x90,    // Port 8080
				0x00, // Only one byte of handshake type
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			relayCell := &cell.RelayCell{
				Command:  cell.RelayExtend2,
				StreamID: 0,
				Data:     tt.data,
			}

			err := handler.HandleExtend2(ctx, circuitID, relayCell)
			if (err != nil) != tt.wantErr {
				t.Errorf("HandleExtend2() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandleExtend2_UnsupportedHandshake(t *testing.T) {
	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}

	log := logger.NewDefault()
	circuits := NewCircuitHandler(keys, log)
	handler := NewExtensionHandler(keys, circuits, log)

	ctx := context.Background()
	circuitID := uint32(123)

	// Build EXTEND2 data with TAP handshake (unsupported)
	data := make([]byte, 0, 128)
	
	// Link specifier
	data = append(data, 1)             // NSPEC = 1
	data = append(data, 0)             // Type = IPv4
	data = append(data, 6)             // Length = 6
	data = append(data, 127, 0, 0, 1)  // IP
	data = append(data, 0x1F, 0x90)    // Port 8080

	// Handshake (TAP type 0x0000)
	data = append(data, 0x00, 0x00) // HTYPE = TAP
	data = append(data, 0x00, 0x04) // HLEN = 4
	data = append(data, 0x00, 0x01, 0x02, 0x03) // Dummy data

	relayCell := &cell.RelayCell{
		Command:  cell.RelayExtend2,
		StreamID: 0,
		Data:     data,
	}

	err = handler.HandleExtend2(ctx, circuitID, relayCell)
	if err == nil {
		t.Error("HandleExtend2() expected error for unsupported handshake type")
	}
}

func TestRegisterExtendedCircuit(t *testing.T) {
	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}

	log := logger.NewDefault()
	circuits := NewCircuitHandler(keys, log)
	handler := NewExtensionHandler(keys, circuits, log)

	// Create a test circuit first
	circuitID := uint32(123)
	circuit := &ServerCircuit{
		CircuitID:    circuitID,
		Created:      time.Now(),
		LastActivity: time.Now(),
		KeyMaterial:  make([]byte, 72),
	}

	circuits.mu.Lock()
	circuits.circuits[circuitID] = circuit
	circuits.mu.Unlock()

	// Test registering extension
	err = handler.registerExtendedCircuit(circuitID, 456, "127.0.0.1:9001", nil)
	if err != nil {
		t.Errorf("registerExtendedCircuit() error = %v", err)
	}

	// Test with non-existent circuit
	err = handler.registerExtendedCircuit(999, 456, "127.0.0.1:9001", nil)
	if err == nil {
		t.Error("registerExtendedCircuit() expected error for non-existent circuit")
	}
}

func TestSendExtended2(t *testing.T) {
	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}

	log := logger.NewDefault()
	circuits := NewCircuitHandler(keys, log)
	handler := NewExtensionHandler(keys, circuits, log)

	circuitID := uint32(123)
	handshakeResponse := make([]byte, 64)
	for i := range handshakeResponse {
		handshakeResponse[i] = byte(i)
	}

	err = handler.sendExtended2(circuitID, handshakeResponse)
	if err != nil {
		t.Errorf("sendExtended2() error = %v", err)
	}
}

func TestBuildExtend2Data(t *testing.T) {
	// Test building a complete EXTEND2 relay cell data
	handshakeData := make([]byte, 84) // ntor handshake size
	for i := range handshakeData {
		handshakeData[i] = byte(i)
	}

	// Build the data manually
	data := make([]byte, 0, 128)
	
	// Link specifier
	data = append(data, 1)             // NSPEC = 1
	data = append(data, 0)             // Type = IPv4
	data = append(data, 6)             // Length = 6
	data = append(data, 127, 0, 0, 1)  // IP
	data = append(data, 0x1F, 0x90)    // Port 8080

	// Handshake
	data = append(data, 0x00, 0x02) // HTYPE = ntor
	hlen := make([]byte, 2)
	binary.BigEndian.PutUint16(hlen, uint16(len(handshakeData)))
	data = append(data, hlen...)
	data = append(data, handshakeData...)

	// Verify we can parse it back
	specs, offset, err := parseLinkSpecifiers(data)
	if err != nil {
		t.Fatalf("parseLinkSpecifiers() error = %v", err)
	}

	if len(specs) != 1 {
		t.Errorf("Expected 1 link specifier, got %d", len(specs))
	}

	// Verify handshake data
	if offset+4 > len(data) {
		t.Fatal("Data truncated at handshake header")
	}

	htype := binary.BigEndian.Uint16(data[offset : offset+2])
	if htype != 0x0002 {
		t.Errorf("Expected handshake type 0x0002, got 0x%04x", htype)
	}

	hlenRead := binary.BigEndian.Uint16(data[offset+2 : offset+4])
	if hlenRead != uint16(len(handshakeData)) {
		t.Errorf("Expected handshake length %d, got %d", len(handshakeData), hlenRead)
	}

	extractedData := data[offset+4 : offset+4+int(hlenRead)]
	if !bytes.Equal(extractedData, handshakeData) {
		t.Error("Handshake data mismatch")
	}
}

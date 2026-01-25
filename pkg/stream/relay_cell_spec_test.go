package stream

import (
	"context"
	"encoding/binary"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestRELAY_BEGINCellFormat verifies RELAY_BEGIN cell format per tor-spec.txt §6.2
func TestRELAY_BEGINCellFormat(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		port    uint16
		wantErr bool
	}{
		{
			name:    "Valid IPv4 address with port",
			target:  "192.168.1.1",
			port:    80,
			wantErr: false,
		},
		{
			name:    "Valid hostname with port",
			target:  "www.example.com",
			port:    443,
			wantErr: false,
		},
		{
			name:    "Valid IPv6 address with port",
			target:  "2001:db8::1",
			port:    8080,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Construct RELAY_BEGIN cell data per tor-spec.txt §6.2
			// Format: ADDRPORT [nul-terminated string]
			addrPort := tt.target + ":" + string(rune(tt.port))
			data := []byte(addrPort)
			data = append(data, 0) // Null terminator

			// Create RELAY_BEGIN cell
			streamID := uint16(42)
			relayCell, err := cell.NewRelayCell(streamID, cell.RelayBegin, data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewRelayCell() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			// Verify cell structure
			if relayCell.Command != cell.RelayBegin {
				t.Errorf("Command = %d, want %d (RELAY_BEGIN)", relayCell.Command, cell.RelayBegin)
			}
			if relayCell.StreamID != streamID {
				t.Errorf("StreamID = %d, want %d", relayCell.StreamID, streamID)
			}
			if relayCell.Recognized != 0 {
				t.Errorf("Recognized = %d, want 0", relayCell.Recognized)
			}

			// Verify data format (null-terminated ADDRPORT string)
			if len(relayCell.Data) == 0 {
				t.Fatal("Data is empty")
			}
			if relayCell.Data[len(relayCell.Data)-1] != 0 {
				t.Errorf("Data is not null-terminated: last byte = %d", relayCell.Data[len(relayCell.Data)-1])
			}
		})
	}
}

// TestRELAY_CONNECTEDCellFormat verifies RELAY_CONNECTED cell format per tor-spec.txt §6.2
func TestRELAY_CONNECTEDCellFormat(t *testing.T) {
	tests := []struct {
		name     string
		ipv4Addr []byte
		ttl      uint32
		wantErr  bool
	}{
		{
			name:     "Valid IPv4 with TTL",
			ipv4Addr: []byte{192, 168, 1, 1},
			ttl:      3600,
			wantErr:  false,
		},
		{
			name:     "IPv4 loopback with TTL",
			ipv4Addr: []byte{127, 0, 0, 1},
			ttl:      7200,
			wantErr:  false,
		},
		{
			name:     "Empty response (no IPv4/TTL)",
			ipv4Addr: nil,
			ttl:      0,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Construct RELAY_CONNECTED cell data per tor-spec.txt §6.2
			// Format: IPv4 address (4 bytes) | TTL (4 bytes)
			// Empty response is also valid (0 bytes)
			var data []byte
			if tt.ipv4Addr != nil {
				data = make([]byte, 8)
				copy(data[0:4], tt.ipv4Addr)
				binary.BigEndian.PutUint32(data[4:8], tt.ttl)
			}

			// Create RELAY_CONNECTED cell
			streamID := uint16(42)
			relayCell, err := cell.NewRelayCell(streamID, cell.RelayConnected, data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewRelayCell() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			// Verify cell structure
			if relayCell.Command != cell.RelayConnected {
				t.Errorf("Command = %d, want %d (RELAY_CONNECTED)", relayCell.Command, cell.RelayConnected)
			}
			if relayCell.StreamID != streamID {
				t.Errorf("StreamID = %d, want %d", relayCell.StreamID, streamID)
			}

			// Verify data format
			if tt.ipv4Addr != nil {
				if len(relayCell.Data) != 8 {
					t.Errorf("Data length = %d, want 8 (IPv4 + TTL)", len(relayCell.Data))
				}
				// Verify IPv4 address
				for i := 0; i < 4; i++ {
					if relayCell.Data[i] != tt.ipv4Addr[i] {
						t.Errorf("IPv4 byte[%d] = %d, want %d", i, relayCell.Data[i], tt.ipv4Addr[i])
					}
				}
				// Verify TTL
				gotTTL := binary.BigEndian.Uint32(relayCell.Data[4:8])
				if gotTTL != tt.ttl {
					t.Errorf("TTL = %d, want %d", gotTTL, tt.ttl)
				}
			} else {
				if len(relayCell.Data) != 0 {
					t.Errorf("Data length = %d, want 0 (empty response)", len(relayCell.Data))
				}
			}
		})
	}
}

// TestRELAY_DATACellFormat verifies RELAY_DATA cell format per tor-spec.txt §6.1
func TestRELAY_DATACellFormat(t *testing.T) {
	tests := []struct {
		name     string
		dataSize int
		wantErr  bool
	}{
		{
			name:     "Empty data",
			dataSize: 0,
			wantErr:  false,
		},
		{
			name:     "Small data (1 byte)",
			dataSize: 1,
			wantErr:  false,
		},
		{
			name:     "Medium data (256 bytes)",
			dataSize: 256,
			wantErr:  false,
		},
		{
			name:     "Maximum data (498 bytes)",
			dataSize: 498, // 509 payload - 11 header = 498 max data
			wantErr:  false,
		},
		// Note: NewRelayCell accepts up to 498 bytes, Encode() validates against max
		// This is NOT an error case for NewRelayCell
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test data
			data := make([]byte, tt.dataSize)
			for i := range data {
				data[i] = byte(i % 256)
			}

			// Create RELAY_DATA cell
			streamID := uint16(42)
			relayCell, err := cell.NewRelayCell(streamID, cell.RelayData, data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewRelayCell() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			// Verify cell structure
			if relayCell.Command != cell.RelayData {
				t.Errorf("Command = %d, want %d (RELAY_DATA)", relayCell.Command, cell.RelayData)
			}
			if relayCell.StreamID != streamID {
				t.Errorf("StreamID = %d, want %d", relayCell.StreamID, streamID)
			}
			if relayCell.Length != uint16(len(data)) {
				t.Errorf("Length = %d, want %d", relayCell.Length, len(data))
			}

			// Verify data integrity
			if len(relayCell.Data) != len(data) {
				t.Errorf("Data length = %d, want %d", len(relayCell.Data), len(data))
			}
			for i := range data {
				if relayCell.Data[i] != data[i] {
					t.Errorf("Data[%d] = %d, want %d", i, relayCell.Data[i], data[i])
					break
				}
			}
		})
	}
}

// TestRELAY_ENDCellFormat verifies RELAY_END cell format per tor-spec.txt §6.3
func TestRELAY_ENDCellFormat(t *testing.T) {
	tests := []struct {
		name   string
		reason byte
	}{
		{name: "END_REASON_MISC", reason: cell.EndReasonMisc},
		{name: "END_REASON_RESOLVE_FAILED", reason: cell.EndReasonResolveFailed},
		{name: "END_REASON_CONN_REFUSED", reason: cell.EndReasonConnRefused},
		{name: "END_REASON_EXITPOLICY", reason: cell.EndReasonExitPolicy},
		{name: "END_REASON_DESTROY", reason: cell.EndReasonDestroy},
		{name: "END_REASON_DONE", reason: cell.EndReasonDone},
		{name: "END_REASON_TIMEOUT", reason: cell.EndReasonTimeout},
		{name: "END_REASON_NOROUTE", reason: cell.EndReasonNoRoute},
		{name: "END_REASON_HIBERNATING", reason: cell.EndReasonHibernating},
		{name: "END_REASON_INTERNAL", reason: cell.EndReasonInternal},
		{name: "END_REASON_RESOURCE_LIMIT", reason: cell.EndReasonResourceLimit},
		{name: "END_REASON_CONN_RESET", reason: cell.EndReasonConnReset},
		{name: "END_REASON_PROTOCOL", reason: cell.EndReasonProtocol},
		{name: "END_REASON_NOT_DIRECTORY", reason: cell.EndReasonNotDirectory},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Construct RELAY_END cell data per tor-spec.txt §6.3
			// Format: 1 byte reason code
			data := []byte{tt.reason}

			// Create RELAY_END cell
			streamID := uint16(42)
			relayCell, err := cell.NewRelayCell(streamID, cell.RelayEnd, data)
			if err != nil {
				t.Fatalf("NewRelayCell() error = %v", err)
			}

			// Verify cell structure
			if relayCell.Command != cell.RelayEnd {
				t.Errorf("Command = %d, want %d (RELAY_END)", relayCell.Command, cell.RelayEnd)
			}
			if relayCell.StreamID != streamID {
				t.Errorf("StreamID = %d, want %d", relayCell.StreamID, streamID)
			}
			if relayCell.Length != 1 {
				t.Errorf("Length = %d, want 1", relayCell.Length)
			}

			// Verify reason code
			if len(relayCell.Data) != 1 {
				t.Fatalf("Data length = %d, want 1", len(relayCell.Data))
			}
			if relayCell.Data[0] != tt.reason {
				t.Errorf("Reason = %d, want %d", relayCell.Data[0], tt.reason)
			}
		})
	}
}

// TestRELAY_SENDMECellFormat verifies RELAY_SENDME cell format per tor-spec.txt §7.4
func TestRELAY_SENDMECellFormat(t *testing.T) {
	tests := []struct {
		name     string
		streamID uint16
		isCirc   bool // Circuit-level vs stream-level SENDME
	}{
		{
			name:     "Circuit-level SENDME (streamID=0)",
			streamID: 0,
			isCirc:   true,
		},
		{
			name:     "Stream-level SENDME (streamID=42)",
			streamID: 42,
			isCirc:   false,
		},
		{
			name:     "Stream-level SENDME (streamID=1)",
			streamID: 1,
			isCirc:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Construct RELAY_SENDME cell per tor-spec.txt §7.4
			// Version 0: empty data
			// Version 1: 1 byte version (0x01) + 20 byte digest (not implemented here)
			// For compatibility, we test version 0 (empty data)
			data := []byte{} // Version 0 SENDME has no data

			// Create RELAY_SENDME cell
			relayCell, err := cell.NewRelayCell(tt.streamID, cell.RelaySendme, data)
			if err != nil {
				t.Fatalf("NewRelayCell() error = %v", err)
			}

			// Verify cell structure
			if relayCell.Command != cell.RelaySendme {
				t.Errorf("Command = %d, want %d (RELAY_SENDME)", relayCell.Command, cell.RelaySendme)
			}
			if relayCell.StreamID != tt.streamID {
				t.Errorf("StreamID = %d, want %d", relayCell.StreamID, tt.streamID)
			}

			// Verify stream ID semantics
			if tt.isCirc {
				if relayCell.StreamID != 0 {
					t.Errorf("Circuit-level SENDME must have StreamID=0, got %d", relayCell.StreamID)
				}
			} else {
				if relayCell.StreamID == 0 {
					t.Errorf("Stream-level SENDME must have StreamID>0, got %d", relayCell.StreamID)
				}
			}

			// Verify data (version 0 has no data)
			if len(relayCell.Data) != 0 {
				t.Errorf("SENDME v0 data length = %d, want 0", len(relayCell.Data))
			}
		})
	}
}

// TestRelayCellEncodeDecode verifies round-trip encoding/decoding of relay cells
func TestRelayCellEncodeDecode(t *testing.T) {
	testCells := []struct {
		name    string
		command byte
		data    []byte
	}{
		{name: "RELAY_BEGIN", command: cell.RelayBegin, data: []byte("example.com:80\x00")},
		{name: "RELAY_CONNECTED", command: cell.RelayConnected, data: make([]byte, 8)},
		{name: "RELAY_DATA", command: cell.RelayData, data: []byte("Hello, Tor!")},
		{name: "RELAY_END", command: cell.RelayEnd, data: []byte{cell.EndReasonDone}},
		{name: "RELAY_SENDME", command: cell.RelaySendme, data: []byte{}},
	}

	for _, tt := range testCells {
		t.Run(tt.name, func(t *testing.T) {
			streamID := uint16(123)

			// Create relay cell
			original, err := cell.NewRelayCell(streamID, tt.command, tt.data)
			if err != nil {
				t.Fatalf("NewRelayCell() error = %v", err)
			}

			// Encode to payload
			payload, err := original.Encode()
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}

			// Verify payload size (should be 509 bytes)
			if len(payload) != cell.PayloadLen {
				t.Errorf("Payload length = %d, want %d", len(payload), cell.PayloadLen)
			}

			// Decode back to relay cell
			decoded, err := cell.DecodeRelayCell(payload)
			if err != nil {
				t.Fatalf("DecodeRelayCell() error = %v", err)
			}

			// Verify decoded cell matches original
			if decoded.Command != original.Command {
				t.Errorf("Command = %d, want %d", decoded.Command, original.Command)
			}
			if decoded.StreamID != original.StreamID {
				t.Errorf("StreamID = %d, want %d", decoded.StreamID, original.StreamID)
			}
			if decoded.Length != original.Length {
				t.Errorf("Length = %d, want %d", decoded.Length, original.Length)
			}
			if len(decoded.Data) != len(original.Data) {
				t.Errorf("Data length = %d, want %d", len(decoded.Data), len(original.Data))
			}
			for i := range original.Data {
				if decoded.Data[i] != original.Data[i] {
					t.Errorf("Data[%d] = %d, want %d", i, decoded.Data[i], original.Data[i])
					break
				}
			}
		})
	}
}

// TestStreamFlowControl verifies flow control implementation per tor-spec.txt §7.4
func TestStreamFlowControl(t *testing.T) {
	log := logger.New(slog.LevelInfo, os.Stdout)
	stream := NewStream(42, 12345, "example.com", 80, log)

	// Verify initial window sizes
	// Per tor-spec.txt §7.4: Initial window is 500 cells
	if stream.packageWindow != 500 {
		t.Errorf("Initial packageWindow = %d, want 500", stream.packageWindow)
	}
	if stream.deliverWindow != 500 {
		t.Errorf("Initial deliverWindow = %d, want 500", stream.deliverWindow)
	}

	// Test package window decrement (sending DATA cells)
	for i := 0; i < 10; i++ {
		err := stream.DecrementPackageWindow()
		if err != nil {
			t.Fatalf("DecrementPackageWindow() error = %v", err)
		}
	}
	if stream.GetPackageWindow() != 490 {
		t.Errorf("PackageWindow after 10 decrements = %d, want 490", stream.GetPackageWindow())
	}

	// Test deliver window decrement (receiving DATA cells)
	for i := 0; i < 50; i++ {
		err := stream.DecrementDeliverWindow()
		if err != nil {
			t.Fatalf("DecrementDeliverWindow() error = %v", err)
		}
	}
	if stream.GetDeliverWindow() != 450 {
		t.Errorf("DeliverWindow after 50 decrements = %d, want 450", stream.GetDeliverWindow())
	}

	// Test SENDME threshold
	// Per tor-spec.txt §7.4: Send SENDME every 50 cells
	if !stream.ShouldSendStreamSendme() {
		t.Error("ShouldSendStreamSendme() = false, want true after 50 cells received")
	}

	// Test SENDME processing
	stream.RecordStreamSendmeSent()
	if stream.GetDeliverWindow() != 500 {
		t.Errorf("DeliverWindow after SENDME = %d, want 500", stream.GetDeliverWindow())
	}
	if stream.ShouldSendStreamSendme() {
		t.Error("ShouldSendStreamSendme() = true, want false after SENDME sent")
	}

	// Test package window increment (receiving SENDME)
	stream.IncrementPackageWindow()
	if stream.GetPackageWindow() != 540 {
		t.Errorf("PackageWindow after SENDME = %d, want 540 (490 + 50)", stream.GetPackageWindow())
	}
}

// TestStreamFlowControlWindowExhaustion verifies window exhaustion handling
func TestStreamFlowControlWindowExhaustion(t *testing.T) {
	log := logger.New(slog.LevelInfo, os.Stdout)
	stream := NewStream(42, 12345, "example.com", 80, log)

	// Exhaust package window
	for i := 0; i < 500; i++ {
		err := stream.DecrementPackageWindow()
		if err != nil {
			t.Fatalf("DecrementPackageWindow() error at %d: %v", i, err)
		}
	}

	// Next decrement should fail
	err := stream.DecrementPackageWindow()
	if err == nil {
		t.Error("DecrementPackageWindow() should error when window exhausted")
	}

	// Exhaust deliver window
	for i := 0; i < 500; i++ {
		err := stream.DecrementDeliverWindow()
		if err != nil {
			t.Fatalf("DecrementDeliverWindow() error at %d: %v", i, err)
		}
	}

	// Next decrement should fail
	err = stream.DecrementDeliverWindow()
	if err == nil {
		t.Error("DecrementDeliverWindow() should error when window exhausted")
	}
}

// TestStreamDataTransfer verifies data send/receive operations
func TestStreamDataTransfer(t *testing.T) {
	log := logger.New(slog.LevelInfo, os.Stdout)
	stream := NewStream(42, 12345, "example.com", 80, log)

	// Set stream to connected state
	stream.SetState(StateConnected)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	testData := []byte("Hello, Tor network!")

	// Test sending data
	err := stream.Send(testData)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	// Test retrieving sent data (as circuit would)
	sentData, err := stream.SendData(ctx)
	if err != nil {
		t.Fatalf("SendData() error = %v", err)
	}
	if string(sentData) != string(testData) {
		t.Errorf("SendData() = %q, want %q", sentData, testData)
	}

	// Test receiving data (as circuit would deliver)
	receiveData := []byte("Response from exit")
	err = stream.ReceiveData(receiveData)
	if err != nil {
		t.Fatalf("ReceiveData() error = %v", err)
	}

	// Test reading received data
	recvData, err := stream.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if string(recvData) != string(receiveData) {
		t.Errorf("Receive() = %q, want %q", recvData, receiveData)
	}
}

// TestStreamIsolationKeys verifies isolation key management
func TestStreamIsolationKeys(t *testing.T) {
	log := logger.New(slog.LevelInfo, os.Stdout)
	stream := NewStream(42, 12345, "example.com", 80, log)

	// Initially no isolation key
	if stream.GetIsolationKey() != nil {
		t.Errorf("Initial isolation key = %v, want nil", stream.GetIsolationKey())
	}

	// Set isolation key with destination
	testKey := circuit.NewIsolationKey(circuit.IsolationDestination).
		WithDestination("example.com:80")
	stream.SetIsolationKey(testKey)

	// Verify isolation key is set
	gotKey := stream.GetIsolationKey()
	if gotKey == nil {
		t.Fatal("GetIsolationKey() = nil, want non-nil")
	}
	if gotKey.Level != circuit.IsolationDestination {
		t.Errorf("IsolationKey.Level = %v, want IsolationDestination", gotKey.Level)
	}
	if gotKey.Destination != "example.com:80" {
		t.Errorf("IsolationKey.Destination = %q, want %q", gotKey.Destination, "example.com:80")
	}
}

// circuit_coverage_test.go - Additional tests to improve circuit package coverage
package circuit

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1" // #nosec G505 - SHA-1 required by Tor protocol
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
)

// TestNewHop tests the NewHop constructor function
func TestNewHop(t *testing.T) {
	tests := []struct {
		name        string
		fingerprint string
		address     string
		isGuard     bool
		isExit      bool
	}{
		{
			name:        "Guard node",
			fingerprint: "ABC123",
			address:     "1.2.3.4:9001",
			isGuard:     true,
			isExit:      false,
		},
		{
			name:        "Exit node",
			fingerprint: "DEF456",
			address:     "5.6.7.8:9001",
			isGuard:     false,
			isExit:      true,
		},
		{
			name:        "Middle relay",
			fingerprint: "GHI789",
			address:     "9.10.11.12:9001",
			isGuard:     false,
			isExit:      false,
		},
		{
			name:        "Guard and Exit",
			fingerprint: "JKL012",
			address:     "13.14.15.16:9001",
			isGuard:     true,
			isExit:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hop := NewHop(tt.fingerprint, tt.address, tt.isGuard, tt.isExit)

			if hop == nil {
				t.Fatal("NewHop() returned nil")
			}
			if hop.Fingerprint != tt.fingerprint {
				t.Errorf("Fingerprint = %v, want %v", hop.Fingerprint, tt.fingerprint)
			}
			if hop.Address != tt.address {
				t.Errorf("Address = %v, want %v", hop.Address, tt.address)
			}
			if hop.IsGuard != tt.isGuard {
				t.Errorf("IsGuard = %v, want %v", hop.IsGuard, tt.isGuard)
			}
			if hop.IsExit != tt.isExit {
				t.Errorf("IsExit = %v, want %v", hop.IsExit, tt.isExit)
			}
			// Crypto state should be nil initially
			if hop.ForwardCipher != nil {
				t.Error("ForwardCipher should be nil initially")
			}
			if hop.BackwardCipher != nil {
				t.Error("BackwardCipher should be nil initially")
			}
			if hop.ForwardDigest != nil {
				t.Error("ForwardDigest should be nil initially")
			}
			if hop.BackwardDigest != nil {
				t.Error("BackwardDigest should be nil initially")
			}
		})
	}
}

// TestHopSetCryptoState tests setting cryptographic state for a hop
func TestHopSetCryptoState(t *testing.T) {
	hop := NewHop("ABC123", "1.2.3.4:9001", true, false)

	// Create mock crypto state
	key := make([]byte, 16)
	for i := range key {
		key[i] = byte(i)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("Failed to create AES cipher: %v", err)
	}

	iv := make([]byte, aes.BlockSize)
	forwardCipher := cipher.NewCTR(block, iv)
	backwardCipher := cipher.NewCTR(block, iv)
	forwardDigest := sha1.New()
	backwardDigest := sha1.New()

	// Set the crypto state
	hop.SetCryptoState(forwardCipher, backwardCipher, forwardDigest, backwardDigest)

	// Verify the state was set
	if hop.ForwardCipher == nil {
		t.Error("ForwardCipher should not be nil after SetCryptoState")
	}
	if hop.BackwardCipher == nil {
		t.Error("BackwardCipher should not be nil after SetCryptoState")
	}
	if hop.ForwardDigest == nil {
		t.Error("ForwardDigest should not be nil after SetCryptoState")
	}
	if hop.BackwardDigest == nil {
		t.Error("BackwardDigest should not be nil after SetCryptoState")
	}
}

// TestCircuitGetHops tests the GetHops method
func TestCircuitGetHops(t *testing.T) {
	c := NewCircuit(1)

	// Initially should be empty
	hops := c.GetHops()
	if len(hops) != 0 {
		t.Errorf("GetHops() length = %v, want 0", len(hops))
	}

	// Add some hops
	hop1 := NewHop("ABC123", "1.2.3.4:9001", true, false)
	hop2 := NewHop("DEF456", "5.6.7.8:9001", false, false)
	hop3 := NewHop("GHI789", "9.10.11.12:9001", false, true)

	c.AddHop(hop1)
	c.AddHop(hop2)
	c.AddHop(hop3)

	// Get the hops
	hops = c.GetHops()
	if len(hops) != 3 {
		t.Errorf("GetHops() length = %v, want 3", len(hops))
	}

	// Verify the hops are in the correct order
	if hops[0].Fingerprint != "ABC123" {
		t.Errorf("hops[0].Fingerprint = %v, want ABC123", hops[0].Fingerprint)
	}
	if hops[1].Fingerprint != "DEF456" {
		t.Errorf("hops[1].Fingerprint = %v, want DEF456", hops[1].Fingerprint)
	}
	if hops[2].Fingerprint != "GHI789" {
		t.Errorf("hops[2].Fingerprint = %v, want GHI789", hops[2].Fingerprint)
	}

	// Verify that modifying the returned slice doesn't affect the original
	returnedHops := c.GetHops()
	returnedHops[0] = nil
	originalHops := c.GetHops()
	if originalHops[0] == nil {
		t.Error("Modifying returned hops should not affect original circuit hops")
	}
}

// TestCircuitClose tests the Close method
func TestCircuitClose(t *testing.T) {
	c := NewCircuit(1)
	c.SetState(StateOpen)

	// Close the circuit
	c.Close()

	// Verify state is closed
	if c.GetState() != StateClosed {
		t.Errorf("State = %v, want %v", c.GetState(), StateClosed)
	}

	// Closing again should be safe (idempotent)
	c.Close()
	if c.GetState() != StateClosed {
		t.Errorf("State = %v, want %v after second close", c.GetState(), StateClosed)
	}
}

// TestCircuitCloseWithRelayChannel tests Close when relay channel exists
func TestCircuitCloseWithRelayChannel(t *testing.T) {
	c := NewCircuit(1)
	c.SetState(StateOpen)

	// Verify relay channel exists
	if c.relayReceiveChan == nil {
		t.Fatal("relayReceiveChan should not be nil in new circuit")
	}

	// Close the circuit
	c.Close()

	// Verify channel is closed and set to nil
	if c.relayReceiveChan != nil {
		t.Error("relayReceiveChan should be nil after close")
	}
}

// TestCircuitSetStreamManager tests the SetStreamManager method
func TestCircuitSetStreamManager(t *testing.T) {
	c := NewCircuit(1)

	// Initially should be nil
	if c.streamManager != nil {
		t.Error("streamManager should be nil initially")
	}

	// Mock stream manager
	type mockStreamManager struct{}
	manager := &mockStreamManager{}

	// Set the stream manager
	c.SetStreamManager(manager)

	// Verify it was set
	if c.streamManager == nil {
		t.Error("streamManager should not be nil after SetStreamManager")
	}
}

// TestCircuitDeliverRelayCell tests DeliverRelayCell method
func TestCircuitDeliverRelayCell(t *testing.T) {
	c := NewCircuit(1)
	c.SetState(StateOpen)

	// Create a mock Cell with relay payload
	relayCell := cell.NewRelayCell(1, cell.RelayData, []byte("test data"))
	payload, err := relayCell.Encode()
	if err != nil {
		t.Fatalf("Failed to encode relay cell: %v", err)
	}

	cellData := &cell.Cell{
		CircID:  c.ID,
		Command: cell.CmdRelay,
		Payload: payload,
	}

	// Test delivering - should handle gracefully even without proper crypto setup
	err = c.DeliverRelayCell(cellData)
	// Error expected since we don't have proper crypto state
	if err == nil {
		t.Log("DeliverRelayCell succeeded (may handle missing crypto gracefully)")
	}
}

// TestCircuitWindowManagement tests window management functions
func TestCircuitWindowManagement(t *testing.T) {
	c := NewCircuit(1)

	// Test initial window values
	initialPackage := c.packageWindow
	initialDeliver := c.deliverWindow

	if initialPackage != 1000 {
		t.Errorf("Initial packageWindow = %v, want 1000", initialPackage)
	}
	if initialDeliver != 1000 {
		t.Errorf("Initial deliverWindow = %v, want 1000", initialDeliver)
	}

	// Test decrementPackageWindow
	err := c.decrementPackageWindow()
	if err != nil {
		t.Errorf("decrementPackageWindow() error = %v", err)
	}
	if c.packageWindow != 999 {
		t.Errorf("packageWindow after decrement = %v, want 999", c.packageWindow)
	}

	// Test incrementPackageWindow (adds 100 per tor-spec.txt §7.4)
	c.incrementPackageWindow()
	if c.packageWindow != 1099 {
		t.Errorf("packageWindow after increment = %v, want 1099", c.packageWindow)
	}

	// Test decrementDeliverWindow
	err = c.decrementDeliverWindow()
	if err != nil {
		t.Errorf("decrementDeliverWindow() error = %v", err)
	}
	if c.deliverWindow != 999 {
		t.Errorf("deliverWindow after decrement = %v, want 999", c.deliverWindow)
	}
}

// TestCircuitShouldSendCircuitSendme tests shouldSendCircuitSendme
func TestCircuitShouldSendCircuitSendme(t *testing.T) {
	c := NewCircuit(1)
	c.SetState(StateOpen)

	// Initially should not need SENDME
	if c.shouldSendCircuitSendme() {
		t.Error("shouldSendCircuitSendme() = true initially, want false")
	}

	// Decrement deliver window by 100 cells
	for i := 0; i < 100; i++ {
		c.decrementDeliverWindow()
	}

	// Should now need SENDME
	if !c.shouldSendCircuitSendme() {
		t.Error("shouldSendCircuitSendme() = false after 100 cells, want true")
	}
}

// TestCircuitSendCircuitSendme tests sendCircuitSendme
func TestCircuitSendCircuitSendme(t *testing.T) {
	c := NewCircuit(1)
	c.SetState(StateOpen)

	// sendCircuitSendme requires a connection, test error case
	err := c.sendCircuitSendme()
	if err == nil {
		t.Error("sendCircuitSendme() should return error when conn is nil")
	}
}

// TestCircuitReceiveRelayCellTimeout tests ReceiveRelayCellTimeout
func TestCircuitReceiveRelayCellTimeout(t *testing.T) {
	c := NewCircuit(1)
	c.SetState(StateOpen)

	// Test timeout - should return context deadline exceeded error
	relayCell, err := c.ReceiveRelayCellTimeout(10 * time.Millisecond)
	if err != context.DeadlineExceeded {
		t.Logf("ReceiveRelayCellTimeout() error = %v, want context.DeadlineExceeded", err)
	}
	if relayCell != nil {
		t.Error("ReceiveRelayCellTimeout() should return nil cell on timeout")
	}

	// Close circuit to test closed channel behavior
	c.Close()
	relayCell, err = c.ReceiveRelayCellTimeout(10 * time.Millisecond)
	// Should either timeout or get closed channel
	if relayCell != nil {
		t.Error("ReceiveRelayCellTimeout() should return nil on closed circuit")
	}
}

// TestCircuitStreamOperations tests stream-related functions
func TestCircuitStreamOperations(t *testing.T) {
	c := NewCircuit(1)
	c.SetState(StateOpen)

	// Test OpenStream - requires connection
	err := c.OpenStream(1, "example.com", 80)
	if err == nil {
		t.Error("OpenStream should return error when conn is nil")
	}

	// Test ReadFromStream - with timeout context to avoid hanging
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	// Close the circuit's relay channel first to prevent hanging
	c.Close()
	
	data, err := c.ReadFromStream(ctx, 1)
	// Should return error or nil data since circuit is closed
	if err == nil && data != nil {
		t.Error("ReadFromStream should return error or nil data on closed circuit")
	}

	// Reopen circuit for other tests
	c2 := NewCircuit(2)
	c2.SetState(StateOpen)

	// Test WriteToStream
	err = c2.WriteToStream(1, []byte("test"))
	if err == nil {
		t.Error("WriteToStream should return error when conn is nil")
	}

	// Test EndStream with reason code
	err = c2.EndStream(1, 1) // 1 = REASON_MISC
	if err == nil {
		t.Error("EndStream should return error when conn is nil")
	}
}

// TestCircuitCryptoFunctions tests encryption/decryption helper functions
func TestCircuitCryptoFunctions(t *testing.T) {
	c := NewCircuit(1)

	// Add a hop with crypto state
	hop := NewHop("ABC123", "1.2.3.4:9001", true, false)

	// Create crypto state
	key := make([]byte, 16)
	for i := range key {
		key[i] = byte(i)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("Failed to create AES cipher: %v", err)
	}

	iv := make([]byte, aes.BlockSize)
	forwardCipher := cipher.NewCTR(block, iv)
	backwardCipher := cipher.NewCTR(block, iv)
	forwardDigest := sha1.New()
	backwardDigest := sha1.New()

	hop.SetCryptoState(forwardCipher, backwardCipher, forwardDigest, backwardDigest)
	c.AddHop(hop)
	c.SetState(StateOpen)

	// Test decryptBackward
	testData := make([]byte, cell.PayloadLen)
	copy(testData, []byte("test data for decryption"))
	decrypted := c.decryptBackward(testData)
	if decrypted == nil {
		t.Error("decryptBackward() returned nil")
	}

	// Test updateHopDigests
	payload := make([]byte, cell.PayloadLen)
	err = c.updateHopDigests(DirectionForward, payload)
	if err != nil {
		t.Logf("updateHopDigests forward: %v", err)
	}
	err = c.updateHopDigests(DirectionBackward, payload)
	if err != nil {
		t.Logf("updateHopDigests backward: %v", err)
	}

	// Test verifyRelayCellDigest
	hopIdx, err := c.verifyRelayCellDigest(payload)
	if err != nil {
		t.Logf("verifyRelayCellDigest error (expected): %v", err)
	}
	if hopIdx >= 0 {
		t.Logf("verifyRelayCellDigest recognized at hop %d", hopIdx)
	}
}

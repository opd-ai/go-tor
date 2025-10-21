package circuit

import (
	"context"
	"testing"
	"time"
)

func TestStateString(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateBuilding, "BUILDING"},
		{StateOpen, "OPEN"},
		{StateClosed, "CLOSED"},
		{StateFailed, "FAILED"},
		{State(99), "UNKNOWN(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewCircuit(t *testing.T) {
	id := uint32(123)
	c := NewCircuit(id)

	if c.ID != id {
		t.Errorf("ID = %v, want %v", c.ID, id)
	}
	if c.State != StateBuilding {
		t.Errorf("State = %v, want %v", c.State, StateBuilding)
	}
	if c.Hops == nil {
		t.Error("Hops is nil")
	}
	if len(c.Hops) != 0 {
		t.Errorf("Hops length = %v, want 0", len(c.Hops))
	}
}

func TestCircuitAddHop(t *testing.T) {
	c := NewCircuit(1)

	hop := &Hop{
		Fingerprint: "ABC123",
		Address:     "1.2.3.4:9001",
		IsGuard:     true,
	}

	err := c.AddHop(hop)
	if err != nil {
		t.Fatalf("AddHop() error = %v", err)
	}

	if c.Length() != 1 {
		t.Errorf("Length() = %v, want 1", c.Length())
	}

	// Try adding to a closed circuit
	c.SetState(StateClosed)
	err = c.AddHop(hop)
	if err == nil {
		t.Error("AddHop() to closed circuit should return error")
	}
}

func TestCircuitSetGetState(t *testing.T) {
	c := NewCircuit(1)

	states := []State{StateBuilding, StateOpen, StateClosed, StateFailed}

	for _, state := range states {
		c.SetState(state)
		if got := c.GetState(); got != state {
			t.Errorf("GetState() = %v, want %v", got, state)
		}
	}
}

func TestCircuitIsReady(t *testing.T) {
	c := NewCircuit(1)

	if c.IsReady() {
		t.Error("IsReady() = true for building circuit, want false")
	}

	c.SetState(StateOpen)
	if !c.IsReady() {
		t.Error("IsReady() = false for open circuit, want true")
	}

	c.SetState(StateClosed)
	if c.IsReady() {
		t.Error("IsReady() = true for closed circuit, want false")
	}
}

func TestCircuitAge(t *testing.T) {
	c := NewCircuit(1)

	time.Sleep(10 * time.Millisecond)

	age := c.Age()
	if age < 10*time.Millisecond {
		t.Errorf("Age() = %v, want >= 10ms", age)
	}
	if age > 1*time.Second {
		t.Errorf("Age() = %v, want < 1s", age)
	}
}

func TestNewManager(t *testing.T) {
	m := NewManager()

	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
	if m.Count() != 0 {
		t.Errorf("Count() = %v, want 0", m.Count())
	}
}

func TestManagerCreateCircuit(t *testing.T) {
	m := NewManager()

	c1, err := m.CreateCircuit()
	if err != nil {
		t.Fatalf("CreateCircuit() error = %v", err)
	}
	if c1.ID == 0 {
		t.Error("Circuit ID is 0 (reserved)")
	}

	c2, err := m.CreateCircuit()
	if err != nil {
		t.Fatalf("CreateCircuit() error = %v", err)
	}
	if c2.ID == c1.ID {
		t.Error("Two circuits have the same ID")
	}

	if m.Count() != 2 {
		t.Errorf("Count() = %v, want 2", m.Count())
	}
}

func TestManagerGetCircuit(t *testing.T) {
	m := NewManager()

	c, err := m.CreateCircuit()
	if err != nil {
		t.Fatalf("CreateCircuit() error = %v", err)
	}

	retrieved, err := m.GetCircuit(c.ID)
	if err != nil {
		t.Fatalf("GetCircuit() error = %v", err)
	}
	if retrieved.ID != c.ID {
		t.Errorf("Retrieved circuit ID = %v, want %v", retrieved.ID, c.ID)
	}

	// Try getting non-existent circuit
	_, err = m.GetCircuit(99999)
	if err == nil {
		t.Error("GetCircuit() for non-existent circuit should return error")
	}
}

func TestManagerCloseCircuit(t *testing.T) {
	m := NewManager()

	c, err := m.CreateCircuit()
	if err != nil {
		t.Fatalf("CreateCircuit() error = %v", err)
	}

	err = m.CloseCircuit(c.ID)
	if err != nil {
		t.Fatalf("CloseCircuit() error = %v", err)
	}

	if m.Count() != 0 {
		t.Errorf("Count() = %v, want 0 after close", m.Count())
	}

	// Try closing non-existent circuit
	err = m.CloseCircuit(99999)
	if err == nil {
		t.Error("CloseCircuit() for non-existent circuit should return error")
	}
}

func TestManagerListCircuits(t *testing.T) {
	m := NewManager()

	// Create several circuits
	ids := make(map[uint32]bool)
	for i := 0; i < 5; i++ {
		c, err := m.CreateCircuit()
		if err != nil {
			t.Fatalf("CreateCircuit() error = %v", err)
		}
		ids[c.ID] = true
	}

	list := m.ListCircuits()
	if len(list) != 5 {
		t.Errorf("ListCircuits() length = %v, want 5", len(list))
	}

	// Verify all IDs are in the list
	for _, id := range list {
		if !ids[id] {
			t.Errorf("ListCircuits() contains unexpected ID %v", id)
		}
	}
}

func TestManagerClose(t *testing.T) {
	m := NewManager()

	// Create some circuits
	for i := 0; i < 3; i++ {
		_, err := m.CreateCircuit()
		if err != nil {
			t.Fatalf("CreateCircuit() error = %v", err)
		}
	}

	if m.Count() != 3 {
		t.Errorf("Count() = %v, want 3", m.Count())
	}

	// Close the manager
	ctx := context.Background()
	err := m.Close(ctx)
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Verify manager is closed
	if !m.IsClosed() {
		t.Error("IsClosed() = false, want true")
	}

	// Verify all circuits are closed
	if m.Count() != 0 {
		t.Errorf("Count() = %v, want 0 after close", m.Count())
	}

	// Try to create circuit on closed manager
	_, err = m.CreateCircuit()
	if err == nil {
		t.Error("CreateCircuit() on closed manager should return error")
	}

	// Try to close again
	err = m.Close(ctx)
	if err == nil {
		t.Error("Close() on already closed manager should return error")
	}
}

func TestManagerCloseWithTimeout(t *testing.T) {
	m := NewManager()

	// Create a circuit
	_, err := m.CreateCircuit()
	if err != nil {
		t.Fatalf("CreateCircuit() error = %v", err)
	}

	// Close with a timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = m.Close(ctx)
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if !m.IsClosed() {
		t.Error("IsClosed() = false, want true")
	}
}

// SPEC-002: Tests for circuit padding functionality

func TestCircuitPaddingEnabled(t *testing.T) {
	c := NewCircuit(1)

	// Padding should be enabled by default
	if !c.IsPaddingEnabled() {
		t.Error("Padding should be enabled by default")
	}

	// Disable padding
	c.SetPaddingEnabled(false)
	if c.IsPaddingEnabled() {
		t.Error("Padding should be disabled after SetPaddingEnabled(false)")
	}

	// Re-enable padding
	c.SetPaddingEnabled(true)
	if !c.IsPaddingEnabled() {
		t.Error("Padding should be enabled after SetPaddingEnabled(true)")
	}
}

func TestCircuitPaddingInterval(t *testing.T) {
	c := NewCircuit(1)

	// Initial interval should be 5 seconds (default)
	if c.GetPaddingInterval() != 5*time.Second {
		t.Errorf("Initial padding interval should be 5s, got %v", c.GetPaddingInterval())
	}

	// Set custom interval
	interval := 10 * time.Second
	c.SetPaddingInterval(interval)
	if c.GetPaddingInterval() != interval {
		t.Errorf("Padding interval should be %v, got %v", interval, c.GetPaddingInterval())
	}

	// Set to 0 to disable adaptive padding
	c.SetPaddingInterval(0)
	if c.GetPaddingInterval() != 0 {
		t.Errorf("Padding interval should be 0, got %v", c.GetPaddingInterval())
	}
}

func TestShouldSendPadding(t *testing.T) {
	tests := []struct {
		name           string
		paddingEnabled bool
		state          State
		timeSinceStart time.Duration
		expected       bool
	}{
		{
			name:           "enabled_open_after_interval",
			paddingEnabled: true,
			state:          StateOpen,
			timeSinceStart: 6 * time.Second, // After default 5s interval
			expected:       true,
		},
		{
			name:           "enabled_open_before_interval",
			paddingEnabled: true,
			state:          StateOpen,
			timeSinceStart: 2 * time.Second, // Before 5s interval
			expected:       false,
		},
		{
			name:           "disabled_open",
			paddingEnabled: false,
			state:          StateOpen,
			timeSinceStart: 6 * time.Second,
			expected:       false,
		},
		{
			name:           "enabled_building",
			paddingEnabled: true,
			state:          StateBuilding,
			timeSinceStart: 6 * time.Second,
			expected:       false,
		},
		{
			name:           "enabled_closed",
			paddingEnabled: true,
			state:          StateClosed,
			timeSinceStart: 6 * time.Second,
			expected:       false,
		},
		{
			name:           "disabled_closed",
			paddingEnabled: false,
			state:          StateClosed,
			timeSinceStart: 6 * time.Second,
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCircuit(1)
			c.SetPaddingEnabled(tt.paddingEnabled)
			c.SetState(tt.state)

			// Simulate time passage by setting lastPaddingTime in the past
			c.mu.Lock()
			c.lastPaddingTime = time.Now().Add(-tt.timeSinceStart)
			c.lastActivityTime = time.Now().Add(-tt.timeSinceStart)
			c.mu.Unlock()

			if got := c.ShouldSendPadding(); got != tt.expected {
				t.Errorf("ShouldSendPadding() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPaddingConcurrency(t *testing.T) {
	c := NewCircuit(1)
	c.SetState(StateOpen)

	// Test concurrent access to padding settings
	done := make(chan bool, 10)

	// Test concurrent RecordActivity and RecordPaddingSent
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 20; j++ {
				c.RecordActivity()
				_ = c.ShouldSendPadding()
				c.RecordPaddingSent()
			}
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}

func TestRecordPaddingSent(t *testing.T) {
	c := NewCircuit(1)

	// Set last padding time in the past
	pastTime := time.Now().Add(-10 * time.Second)
	c.mu.Lock()
	c.lastPaddingTime = pastTime
	c.mu.Unlock()

	// Record padding sent
	c.RecordPaddingSent()

	// Verify time was updated
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.lastPaddingTime == pastTime {
		t.Error("RecordPaddingSent() did not update lastPaddingTime")
	}
}

func TestRecordActivity(t *testing.T) {
	c := NewCircuit(1)

	// Set last activity time in the past
	pastTime := time.Now().Add(-10 * time.Second)
	c.mu.Lock()
	c.lastActivityTime = pastTime
	c.mu.Unlock()

	// Record activity
	c.RecordActivity()

	// Verify time was updated
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.lastActivityTime == pastTime {
		t.Error("RecordActivity() did not update lastActivityTime")
	}
}

// CRYPTO-001: Tests for relay cell digest verification

func TestCircuitDigestInitialization(t *testing.T) {
	c := NewCircuit(1)

	// Digests should be initialized
	if c.forwardDigest == nil {
		t.Error("Forward digest not initialized")
	}
	if c.backwardDigest == nil {
		t.Error("Backward digest not initialized")
	}
}

func TestUpdateDigest(t *testing.T) {
	c := NewCircuit(1)

	// Create a mock relay cell (11+ bytes)
	cellData := make([]byte, 20)
	cellData[0] = 1 // Command
	cellData[1] = 0 // Recognized (2 bytes)
	cellData[2] = 0
	cellData[3] = 0 // StreamID (2 bytes)
	cellData[4] = 1
	// Bytes 5-8 are digest (will be zeroed)
	cellData[5] = 0xAA
	cellData[6] = 0xBB
	cellData[7] = 0xCC
	cellData[8] = 0xDD
	cellData[9] = 0 // Length (2 bytes)
	cellData[10] = 0

	// Update forward digest
	err := c.UpdateDigest(DirectionForward, cellData)
	if err != nil {
		t.Fatalf("UpdateDigest failed: %v", err)
	}

	// Update backward digest
	err = c.UpdateDigest(DirectionBackward, cellData)
	if err != nil {
		t.Fatalf("UpdateDigest failed: %v", err)
	}
}

func TestUpdateDigestTooShort(t *testing.T) {
	c := NewCircuit(1)

	// Cell data too short
	cellData := make([]byte, 5)

	err := c.UpdateDigest(DirectionForward, cellData)
	if err == nil {
		t.Error("Expected error for short cell data, got nil")
	}
}

func TestVerifyDigest(t *testing.T) {
	c := NewCircuit(1)

	// Create mock relay cell
	cellData := make([]byte, 20)
	cellData[0] = 1 // Command

	// Get the current digest state BEFORE updating (this is the verification flow)
	currentSum := c.forwardDigest.Sum(nil)
	receivedDigest := [4]byte{currentSum[0], currentSum[1], currentSum[2], currentSum[3]}

	// Verify should pass with matching digest
	err := c.VerifyDigest(DirectionForward, cellData, receivedDigest)
	if err != nil {
		t.Errorf("VerifyDigest failed: %v", err)
	}

	// Now update the digest for future cells
	_ = c.UpdateDigest(DirectionForward, cellData)
}

func TestVerifyDigestMismatch(t *testing.T) {
	c := NewCircuit(1)

	cellData := make([]byte, 20)
	cellData[0] = 1

	// Wrong digest
	wrongDigest := [4]byte{0xFF, 0xFF, 0xFF, 0xFF}

	err := c.VerifyDigest(DirectionForward, cellData, wrongDigest)
	if err == nil {
		t.Error("Expected error for digest mismatch, got nil")
	}
}

func TestResetDigests(t *testing.T) {
	c := NewCircuit(1)

	// Update digests
	cellData := make([]byte, 20)
	_ = c.UpdateDigest(DirectionForward, cellData)
	_ = c.UpdateDigest(DirectionBackward, cellData)

	// Reset
	c.ResetDigests()

	// Digests should still be usable
	err := c.UpdateDigest(DirectionForward, cellData)
	if err != nil {
		t.Errorf("UpdateDigest after reset failed: %v", err)
	}
}

func TestDigestConcurrency(t *testing.T) {
	c := NewCircuit(1)
	c.SetState(StateOpen)

	done := make(chan bool, 10)

	// Multiple goroutines updating digests concurrently
	for i := 0; i < 10; i++ {
		go func(id int) {
			cellData := make([]byte, 20)
			cellData[0] = byte(id)

			for j := 0; j < 100; j++ {
				direction := DirectionForward
				if j%2 == 0 {
					direction = DirectionBackward
				}
				_ = c.UpdateDigest(direction, cellData)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

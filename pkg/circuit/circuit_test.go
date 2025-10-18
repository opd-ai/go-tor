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

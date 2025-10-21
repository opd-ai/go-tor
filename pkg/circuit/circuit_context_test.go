package circuit

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitWaitForState(t *testing.T) {
	t.Run("already in target state", func(t *testing.T) {
		circuit := NewCircuit(1)
		circuit.SetState(StateOpen)

		ctx := context.Background()
		err := circuit.WaitForState(ctx, StateOpen)
		if err != nil {
			t.Errorf("WaitForState failed: %v", err)
		}
	})

	t.Run("transition to target state", func(t *testing.T) {
		circuit := NewCircuit(1)
		circuit.SetState(StateBuilding)

		go func() {
			time.Sleep(50 * time.Millisecond)
			circuit.SetState(StateOpen)
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		err := circuit.WaitForState(ctx, StateOpen)
		if err != nil {
			t.Errorf("WaitForState failed: %v", err)
		}
	})

	t.Run("timeout waiting for state", func(t *testing.T) {
		circuit := NewCircuit(1)
		circuit.SetState(StateBuilding)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := circuit.WaitForState(ctx, StateOpen)
		if err == nil {
			t.Error("Expected timeout error")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		circuit := NewCircuit(1)
		circuit.SetState(StateBuilding)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := circuit.WaitForState(ctx, StateOpen)
		if err == nil {
			t.Error("Expected cancellation error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled, got: %v", err)
		}
	})
}

func TestCircuitWaitUntilReady(t *testing.T) {
	t.Run("circuit becomes ready", func(t *testing.T) {
		circuit := NewCircuit(1)
		circuit.SetState(StateBuilding)

		go func() {
			time.Sleep(50 * time.Millisecond)
			circuit.SetState(StateOpen)
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		err := circuit.WaitUntilReady(ctx)
		if err != nil {
			t.Errorf("WaitUntilReady failed: %v", err)
		}
	})

	t.Run("timeout waiting for ready", func(t *testing.T) {
		circuit := NewCircuit(1)
		circuit.SetState(StateBuilding)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := circuit.WaitUntilReady(ctx)
		if err == nil {
			t.Error("Expected timeout error")
		}
	})
}

func TestCircuitAgeWithContext(t *testing.T) {
	t.Run("get age with valid context", func(t *testing.T) {
		circuit := NewCircuit(1)
		time.Sleep(10 * time.Millisecond)

		ctx := context.Background()
		age, err := circuit.AgeWithContext(ctx)
		if err != nil {
			t.Errorf("AgeWithContext failed: %v", err)
		}
		if age < 10*time.Millisecond {
			t.Errorf("Expected age >= 10ms, got %v", age)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		circuit := NewCircuit(1)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := circuit.AgeWithContext(ctx)
		if err == nil {
			t.Error("Expected cancellation error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled, got: %v", err)
		}
	})
}

func TestCircuitIsOlderThan(t *testing.T) {
	t.Run("circuit is older", func(t *testing.T) {
		circuit := NewCircuit(1)
		time.Sleep(50 * time.Millisecond)

		if !circuit.IsOlderThan(10 * time.Millisecond) {
			t.Error("Circuit should be older than 10ms")
		}
	})

	t.Run("circuit is younger", func(t *testing.T) {
		circuit := NewCircuit(1)

		if circuit.IsOlderThan(100 * time.Millisecond) {
			t.Error("Circuit should not be older than 100ms")
		}
	})
}

func TestCircuitSetStateWithContext(t *testing.T) {
	t.Run("set state with valid context", func(t *testing.T) {
		circuit := NewCircuit(1)

		ctx := context.Background()
		err := circuit.SetStateWithContext(ctx, StateOpen)
		if err != nil {
			t.Errorf("SetStateWithContext failed: %v", err)
		}
		if circuit.GetState() != StateOpen {
			t.Errorf("Expected state %s, got %s", StateOpen, circuit.GetState())
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		circuit := NewCircuit(1)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := circuit.SetStateWithContext(ctx, StateOpen)
		if err == nil {
			t.Error("Expected cancellation error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled, got: %v", err)
		}
	})
}

func TestManagerCloseWithDeadline(t *testing.T) {
	t.Run("close with deadline", func(t *testing.T) {
		manager := NewManager()
		_, _ = manager.CreateCircuit()
		_, _ = manager.CreateCircuit()

		err := manager.CloseWithDeadline(100 * time.Millisecond)
		if err != nil {
			t.Errorf("CloseWithDeadline failed: %v", err)
		}

		if !manager.IsClosed() {
			t.Error("Manager should be closed")
		}
	})

	t.Run("close empty manager", func(t *testing.T) {
		manager := NewManager()

		err := manager.CloseWithDeadline(100 * time.Millisecond)
		if err != nil {
			t.Errorf("CloseWithDeadline failed: %v", err)
		}
	})
}

func TestManagerWaitForCircuitCount(t *testing.T) {
	t.Run("wait for circuits to reach count", func(t *testing.T) {
		manager := NewManager()

		go func() {
			time.Sleep(50 * time.Millisecond)
			for i := 0; i < 3; i++ {
				circuit, _ := manager.CreateCircuit()
				circuit.SetState(StateOpen)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		err := manager.WaitForCircuitCount(ctx, StateOpen, 3)
		if err != nil {
			t.Errorf("WaitForCircuitCount failed: %v", err)
		}
	})

	t.Run("timeout waiting for circuit count", func(t *testing.T) {
		manager := NewManager()

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := manager.WaitForCircuitCount(ctx, StateOpen, 3)
		if err == nil {
			t.Error("Expected timeout error")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
		}
	})

	t.Run("already have enough circuits", func(t *testing.T) {
		manager := NewManager()
		for i := 0; i < 5; i++ {
			circuit, _ := manager.CreateCircuit()
			circuit.SetState(StateOpen)
		}

		ctx := context.Background()
		err := manager.WaitForCircuitCount(ctx, StateOpen, 3)
		if err != nil {
			t.Errorf("WaitForCircuitCount failed: %v", err)
		}
	})
}

func TestManagerGetCircuitsByState(t *testing.T) {
	manager := NewManager()

	// Create circuits in different states
	c1, _ := manager.CreateCircuit()
	c1.SetState(StateOpen)

	c2, _ := manager.CreateCircuit()
	c2.SetState(StateOpen)

	c3, _ := manager.CreateCircuit()
	c3.SetState(StateBuilding)

	t.Run("get open circuits", func(t *testing.T) {
		circuits := manager.GetCircuitsByState(StateOpen)
		if len(circuits) != 2 {
			t.Errorf("Expected 2 open circuits, got %d", len(circuits))
		}
	})

	t.Run("get building circuits", func(t *testing.T) {
		circuits := manager.GetCircuitsByState(StateBuilding)
		if len(circuits) != 1 {
			t.Errorf("Expected 1 building circuit, got %d", len(circuits))
		}
	})

	t.Run("get closed circuits", func(t *testing.T) {
		circuits := manager.GetCircuitsByState(StateClosed)
		if len(circuits) != 0 {
			t.Errorf("Expected 0 closed circuits, got %d", len(circuits))
		}
	})
}

func TestManagerCountByState(t *testing.T) {
	manager := NewManager()

	// Create circuits in different states
	c1, _ := manager.CreateCircuit()
	c1.SetState(StateOpen)

	c2, _ := manager.CreateCircuit()
	c2.SetState(StateOpen)

	c3, _ := manager.CreateCircuit()
	c3.SetState(StateBuilding)

	tests := []struct {
		state    State
		expected int
	}{
		{StateOpen, 2},
		{StateBuilding, 1},
		{StateClosed, 0},
		{StateFailed, 0},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			count := manager.CountByState(tt.state)
			if count != tt.expected {
				t.Errorf("Expected %d circuits in state %s, got %d",
					tt.expected, tt.state, count)
			}
		})
	}
}

func TestManagerCloseCircuitWithContext(t *testing.T) {
	t.Run("close circuit with context", func(t *testing.T) {
		manager := NewManager()
		circuit, _ := manager.CreateCircuit()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := manager.CloseCircuitWithContext(ctx, circuit.ID)
		if err != nil {
			t.Errorf("CloseCircuitWithContext failed: %v", err)
		}
	})

	t.Run("close non-existent circuit", func(t *testing.T) {
		manager := NewManager()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := manager.CloseCircuitWithContext(ctx, 999)
		if err == nil {
			t.Error("Expected error closing non-existent circuit")
		}
	})
}

func TestManagerCreateCircuitWithContext(t *testing.T) {
	t.Run("create circuit with context", func(t *testing.T) {
		manager := NewManager()

		ctx := context.Background()
		circuit, err := manager.CreateCircuitWithContext(ctx)
		if err != nil {
			t.Errorf("CreateCircuitWithContext failed: %v", err)
		}
		if circuit == nil {
			t.Error("Expected circuit to be created")
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		manager := NewManager()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := manager.CreateCircuitWithContext(ctx)
		if err == nil {
			t.Error("Expected cancellation error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled, got: %v", err)
		}
	})
}

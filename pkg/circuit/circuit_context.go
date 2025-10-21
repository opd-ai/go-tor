// Package circuit provides context-aware operations for circuit management.
package circuit

import (
	"context"
	"fmt"
	"time"
)

// WaitForState waits for the circuit to reach a specific state or until the context is done.
// This is useful for waiting for circuit establishment or checking for closure.
//
// Example usage:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//	err := circuit.WaitForState(ctx, StateOpen)
func (c *Circuit) WaitForState(ctx context.Context, state State) error {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		if c.GetState() == state {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for state %s (current: %s): %w",
				state, c.GetState(), ctx.Err())
		case <-ticker.C:
			// Check state again on next iteration
		}
	}
}

// WaitUntilReady waits for the circuit to become ready (StateOpen) or until the context is done.
// This is a convenience method that wraps WaitForState.
//
// Example usage:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//	err := circuit.WaitUntilReady(ctx)
func (c *Circuit) WaitUntilReady(ctx context.Context) error {
	return c.WaitForState(ctx, StateOpen)
}

// AgeWithContext returns how long the circuit has existed, or an error if the context is done.
// This is useful for monitoring circuit age with cancellation support.
func (c *Circuit) AgeWithContext(ctx context.Context) (time.Duration, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
		return c.Age(), nil
	}
}

// IsOlderThan returns true if the circuit is older than the specified duration.
// This is useful for implementing circuit rotation policies.
func (c *Circuit) IsOlderThan(duration time.Duration) bool {
	return c.Age() > duration
}

// SetStateWithContext sets the circuit state with context support.
// This allows state changes to be cancelled if the context is done.
func (c *Circuit) SetStateWithContext(ctx context.Context, state State) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("state change cancelled: %w", ctx.Err())
	default:
		c.SetState(state)
		return nil
	}
}

// CloseWithDeadline closes all circuits in the manager with a deadline.
// This is a convenience wrapper around Close that adds deadline support.
//
// Example usage:
//
//	err := manager.CloseWithDeadline(5 * time.Second)
func (m *Manager) CloseWithDeadline(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return m.Close(ctx)
}

// WaitForCircuitCount waits until the manager has at least the specified number of circuits
// in the given state, or until the context is done.
//
// Example usage:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//	err := manager.WaitForCircuitCount(ctx, StateOpen, 3)
func (m *Manager) WaitForCircuitCount(ctx context.Context, state State, minCount int) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		m.mu.RLock()
		count := 0
		for _, circuit := range m.circuits {
			if circuit.GetState() == state {
				count++
			}
		}
		m.mu.RUnlock()

		if count >= minCount {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for %d circuits in state %s (current: %d): %w",
				minCount, state, count, ctx.Err())
		case <-ticker.C:
			// Check count again on next iteration
		}
	}
}

// GetCircuitsByState returns all circuits in the specified state.
// This is useful for monitoring or selecting circuits based on their state.
func (m *Manager) GetCircuitsByState(state State) []*Circuit {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var circuits []*Circuit
	for _, circuit := range m.circuits {
		if circuit.GetState() == state {
			circuits = append(circuits, circuit)
		}
	}
	return circuits
}

// CountByState returns the number of circuits in the specified state.
func (m *Manager) CountByState(state State) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, circuit := range m.circuits {
		if circuit.GetState() == state {
			count++
		}
	}
	return count
}

// CloseCircuitWithContext closes a circuit with context support for timeout/cancellation.
//
// Example usage:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	err := manager.CloseCircuitWithContext(ctx, circuitID)
func (m *Manager) CloseCircuitWithContext(ctx context.Context, id uint32) error {
	done := make(chan error, 1)
	go func() {
		done <- m.CloseCircuit(id)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// Try to force close even if context expires
		_ = m.CloseCircuit(id)
		return fmt.Errorf("close circuit timeout: %w", ctx.Err())
	}
}

// CreateCircuitWithContext creates a new circuit with context support.
// This allows circuit creation to be cancelled if needed.
func (m *Manager) CreateCircuitWithContext(ctx context.Context) (*Circuit, error) {
	done := make(chan struct {
		circuit *Circuit
		err     error
	}, 1)

	go func() {
		circuit, err := m.CreateCircuit()
		done <- struct {
			circuit *Circuit
			err     error
		}{circuit, err}
	}()

	select {
	case result := <-done:
		return result.circuit, result.err
	case <-ctx.Done():
		return nil, fmt.Errorf("create circuit cancelled: %w", ctx.Err())
	}
}

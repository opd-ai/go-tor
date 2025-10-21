// Package stream provides context-aware operations for Tor stream management.
package stream

import (
	"context"
	"fmt"
	"io"
	"time"
)

// SendWithContext sends data on the stream with context support for cancellation and timeout.
// This method provides better control over send operations compared to the basic Send method.
//
// Example usage:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	err := stream.SendWithContext(ctx, data)
func (s *Stream) SendWithContext(ctx context.Context, data []byte) error {
	if s.GetState() != StateConnected {
		return fmt.Errorf("stream not connected: state=%s", s.GetState())
	}

	select {
	case s.sendQueue <- data:
		return nil
	case <-s.closeChan:
		return io.EOF
	case <-ctx.Done():
		return fmt.Errorf("send cancelled: %w", ctx.Err())
	}
}

// ReceiveWithTimeout receives data from the stream with a timeout.
// This is a convenience method that wraps Receive with a timeout context.
//
// Example usage:
//
//	data, err := stream.ReceiveWithTimeout(5 * time.Second)
func (s *Stream) ReceiveWithTimeout(timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.Receive(ctx)
}

// SendWithTimeout sends data on the stream with a timeout.
// This is a convenience method that wraps SendWithContext with a timeout context.
//
// Example usage:
//
//	err := stream.SendWithTimeout(5*time.Second, data)
func (s *Stream) SendWithTimeout(timeout time.Duration, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.SendWithContext(ctx, data)
}

// WaitForState waits for the stream to reach a specific state or until the context is done.
// This is useful for waiting for connection establishment or checking for closure.
//
// Example usage:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//	err := stream.WaitForState(ctx, StateConnected)
func (s *Stream) WaitForState(ctx context.Context, state State) error {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		if s.GetState() == state {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for state %s (current: %s): %w",
				state, s.GetState(), ctx.Err())
		case <-ticker.C:
			// Check state again on next iteration
		case <-s.closeChan:
			if state == StateClosed {
				return nil
			}
			return fmt.Errorf("stream closed while waiting for state %s", state)
		}
	}
}

// CloseWithContext closes the stream gracefully, waiting for pending operations to complete
// or until the context is done. This provides better control over stream shutdown.
//
// Example usage:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	err := stream.CloseWithContext(ctx)
func (s *Stream) CloseWithContext(ctx context.Context) error {
	// Attempt to close the stream
	closeErr := make(chan error, 1)
	go func() {
		closeErr <- s.Close()
	}()

	// Wait for close or context cancellation
	select {
	case err := <-closeErr:
		return err
	case <-ctx.Done():
		// Force close even if context expires
		_ = s.Close()
		return fmt.Errorf("close timeout: %w", ctx.Err())
	}
}

// IsActive returns true if the stream is in an active state (CONNECTING or CONNECTED).
func (s *Stream) IsActive() bool {
	state := s.GetState()
	return state == StateConnecting || state == StateConnected
}

// IsClosed returns true if the stream has been closed or failed.
func (s *Stream) IsClosed() bool {
	state := s.GetState()
	return state == StateClosed || state == StateFailed
}

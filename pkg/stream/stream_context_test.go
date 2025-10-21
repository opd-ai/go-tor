package stream

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestSendWithContext(t *testing.T) {
	log := logger.NewDefault()

	t.Run("successful send", func(t *testing.T) {
		stream := NewStream(1, 100, "example.com", 80, log)
		stream.SetState(StateConnected)

		ctx := context.Background()
		data := []byte("test data")

		err := stream.SendWithContext(ctx, data)
		if err != nil {
			t.Errorf("SendWithContext failed: %v", err)
		}
	})

	t.Run("send with timeout", func(t *testing.T) {
		stream := NewStream(1, 100, "example.com", 80, log)
		stream.SetState(StateConnected)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Fill the send queue
		for i := 0; i < 32; i++ {
			_ = stream.Send([]byte("filling queue"))
		}

		// This should timeout since queue is full
		data := []byte("test data")
		err := stream.SendWithContext(ctx, data)
		if err == nil {
			t.Error("Expected timeout error, got nil")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
		}
	})

	t.Run("send on closed stream", func(t *testing.T) {
		stream := NewStream(1, 100, "example.com", 80, log)
		stream.SetState(StateConnected)
		_ = stream.Close()

		ctx := context.Background()
		data := []byte("test data")

		err := stream.SendWithContext(ctx, data)
		if err == nil {
			t.Error("Expected error when sending on closed stream")
		}
	})

	t.Run("send when not connected", func(t *testing.T) {
		stream := NewStream(1, 100, "example.com", 80, log)
		stream.SetState(StateNew)

		ctx := context.Background()
		data := []byte("test data")

		err := stream.SendWithContext(ctx, data)
		if err == nil {
			t.Error("Expected error when stream not connected")
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		stream := NewStream(1, 100, "example.com", 80, log)
		stream.SetState(StateConnected)

		// Fill the send queue to ensure select blocks
		for i := 0; i < 32; i++ {
			_ = stream.Send([]byte("filling queue"))
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		data := []byte("test data")
		err := stream.SendWithContext(ctx, data)
		if err == nil {
			t.Error("Expected error with cancelled context")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled, got: %v", err)
		}
	})
}

func TestReceiveWithTimeout(t *testing.T) {
	log := logger.NewDefault()

	t.Run("successful receive", func(t *testing.T) {
		stream := NewStream(1, 100, "example.com", 80, log)
		stream.SetState(StateConnected)

		testData := []byte("test data")
		go func() {
			time.Sleep(50 * time.Millisecond)
			_ = stream.ReceiveData(testData)
		}()

		data, err := stream.ReceiveWithTimeout(200 * time.Millisecond)
		if err != nil {
			t.Errorf("ReceiveWithTimeout failed: %v", err)
		}
		if string(data) != string(testData) {
			t.Errorf("Expected %s, got %s", testData, data)
		}
	})

	t.Run("timeout on receive", func(t *testing.T) {
		stream := NewStream(1, 100, "example.com", 80, log)
		stream.SetState(StateConnected)

		_, err := stream.ReceiveWithTimeout(100 * time.Millisecond)
		if err == nil {
			t.Error("Expected timeout error")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
		}
	})
}

func TestSendWithTimeout(t *testing.T) {
	log := logger.NewDefault()

	t.Run("successful send with timeout", func(t *testing.T) {
		stream := NewStream(1, 100, "example.com", 80, log)
		stream.SetState(StateConnected)

		data := []byte("test data")
		err := stream.SendWithTimeout(100*time.Millisecond, data)
		if err != nil {
			t.Errorf("SendWithTimeout failed: %v", err)
		}
	})

	t.Run("timeout on send", func(t *testing.T) {
		stream := NewStream(1, 100, "example.com", 80, log)
		stream.SetState(StateConnected)

		// Fill the send queue
		for i := 0; i < 32; i++ {
			_ = stream.Send([]byte("filling queue"))
		}

		data := []byte("test data")
		err := stream.SendWithTimeout(100*time.Millisecond, data)
		if err == nil {
			t.Error("Expected timeout error")
		}
	})
}

func TestWaitForState(t *testing.T) {
	log := logger.NewDefault()

	t.Run("already in target state", func(t *testing.T) {
		stream := NewStream(1, 100, "example.com", 80, log)
		stream.SetState(StateConnected)

		ctx := context.Background()
		err := stream.WaitForState(ctx, StateConnected)
		if err != nil {
			t.Errorf("WaitForState failed: %v", err)
		}
	})

	t.Run("transition to target state", func(t *testing.T) {
		stream := NewStream(1, 100, "example.com", 80, log)
		stream.SetState(StateConnecting)

		go func() {
			time.Sleep(50 * time.Millisecond)
			stream.SetState(StateConnected)
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		err := stream.WaitForState(ctx, StateConnected)
		if err != nil {
			t.Errorf("WaitForState failed: %v", err)
		}
	})

	t.Run("timeout waiting for state", func(t *testing.T) {
		stream := NewStream(1, 100, "example.com", 80, log)
		stream.SetState(StateConnecting)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := stream.WaitForState(ctx, StateConnected)
		if err == nil {
			t.Error("Expected timeout error")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
		}
	})

	t.Run("stream closed while waiting", func(t *testing.T) {
		stream := NewStream(1, 100, "example.com", 80, log)
		stream.SetState(StateConnecting)

		go func() {
			time.Sleep(50 * time.Millisecond)
			_ = stream.Close()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		err := stream.WaitForState(ctx, StateConnected)
		if err == nil {
			t.Error("Expected error when stream closed")
		}
	})

	t.Run("wait for closed state", func(t *testing.T) {
		stream := NewStream(1, 100, "example.com", 80, log)
		stream.SetState(StateConnected)

		go func() {
			time.Sleep(50 * time.Millisecond)
			_ = stream.Close()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		err := stream.WaitForState(ctx, StateClosed)
		if err != nil {
			t.Errorf("WaitForState failed: %v", err)
		}
	})
}

func TestCloseWithContext(t *testing.T) {
	log := logger.NewDefault()

	t.Run("successful close", func(t *testing.T) {
		stream := NewStream(1, 100, "example.com", 80, log)
		stream.SetState(StateConnected)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := stream.CloseWithContext(ctx)
		if err != nil {
			t.Errorf("CloseWithContext failed: %v", err)
		}

		if !stream.IsClosed() {
			t.Error("Stream should be closed")
		}
	})

	t.Run("close already closed stream", func(t *testing.T) {
		stream := NewStream(1, 100, "example.com", 80, log)
		stream.SetState(StateConnected)
		_ = stream.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := stream.CloseWithContext(ctx)
		if err != nil {
			t.Errorf("CloseWithContext on already closed stream failed: %v", err)
		}
	})
}

func TestIsActive(t *testing.T) {
	log := logger.NewDefault()
	stream := NewStream(1, 100, "example.com", 80, log)

	tests := []struct {
		state    State
		expected bool
	}{
		{StateNew, false},
		{StateConnecting, true},
		{StateConnected, true},
		{StateClosed, false},
		{StateFailed, false},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			stream.SetState(tt.state)
			if stream.IsActive() != tt.expected {
				t.Errorf("IsActive() = %v, expected %v for state %s",
					stream.IsActive(), tt.expected, tt.state)
			}
		})
	}
}

func TestIsClosed(t *testing.T) {
	log := logger.NewDefault()
	stream := NewStream(1, 100, "example.com", 80, log)

	tests := []struct {
		state    State
		expected bool
	}{
		{StateNew, false},
		{StateConnecting, false},
		{StateConnected, false},
		{StateClosed, true},
		{StateFailed, true},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			stream.SetState(tt.state)
			if stream.IsClosed() != tt.expected {
				t.Errorf("IsClosed() = %v, expected %v for state %s",
					stream.IsClosed(), tt.expected, tt.state)
			}
		})
	}
}

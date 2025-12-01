package cell

import (
	"fmt"
	"sync"
	"testing"
)

func TestNewReplayProtection(t *testing.T) {
	rp := NewReplayProtection()
	if rp == nil {
		t.Fatal("NewReplayProtection returned nil")
	}

	stats := rp.Stats()
	if stats.ForwardSequence != 0 {
		t.Errorf("Initial forward sequence = %d, want 0", stats.ForwardSequence)
	}
	if stats.BackwardSequence != 0 {
		t.Errorf("Initial backward sequence = %d, want 0", stats.BackwardSequence)
	}
}

func TestNewReplayProtectionWithWindow(t *testing.T) {
	tests := []uint64{1, 16, 32, 64, 128}
	for _, windowSize := range tests {
		t.Run(fmt.Sprintf("window_%d", windowSize), func(t *testing.T) {
			rp := NewReplayProtectionWithWindow(windowSize)
			if rp == nil {
				t.Fatal("NewReplayProtectionWithWindow returned nil")
			}
			if rp.windowSize != windowSize {
				t.Errorf("windowSize = %d, want %d", rp.windowSize, windowSize)
			}
		})
	}
}

func TestValidateAndTrack_NormalFlow(t *testing.T) {
	rp := NewReplayProtection()

	// Send cells in order
	for i := uint64(0); i < 10; i++ {
		cellData := []byte(fmt.Sprintf("cell data %d", i))
		err := rp.ValidateAndTrack(ReplayForward, i, cellData)
		if err != nil {
			t.Errorf("ValidateAndTrack(%d) error = %v", i, err)
		}
	}

	stats := rp.Stats()
	if stats.ForwardSequence != 10 {
		t.Errorf("ForwardSequence = %d, want 10", stats.ForwardSequence)
	}
	if stats.ReplayAttemptsForward != 0 {
		t.Errorf("ReplayAttemptsForward = %d, want 0", stats.ReplayAttemptsForward)
	}
}

func TestValidateAndTrack_DuplicateCell(t *testing.T) {
	rp := NewReplayProtection()

	cellData := []byte("test cell data")

	// First cell should succeed
	err := rp.ValidateAndTrack(ReplayForward, 0, cellData)
	if err != nil {
		t.Fatalf("First ValidateAndTrack error = %v", err)
	}

	// Same cell data with same sequence should fail (replay attempt)
	err = rp.ValidateAndTrack(ReplayForward, 0, cellData)
	if err == nil {
		t.Error("Expected error for duplicate cell, got nil")
	}

	stats := rp.Stats()
	if stats.ReplayAttemptsForward != 1 {
		t.Errorf("ReplayAttemptsForward = %d, want 1", stats.ReplayAttemptsForward)
	}
}

func TestValidateAndTrack_DuplicateSequence(t *testing.T) {
	rp := NewReplayProtection()

	// First cell
	err := rp.ValidateAndTrack(ReplayForward, 0, []byte("cell 0"))
	if err != nil {
		t.Fatalf("First ValidateAndTrack error = %v", err)
	}

	// Different data but same sequence should fail
	err = rp.ValidateAndTrack(ReplayForward, 0, []byte("different cell 0"))
	if err == nil {
		t.Error("Expected error for duplicate sequence, got nil")
	}

	stats := rp.Stats()
	if stats.ReplayAttemptsForward != 1 {
		t.Errorf("ReplayAttemptsForward = %d, want 1", stats.ReplayAttemptsForward)
	}
}

func TestValidateAndTrack_DuplicateDigest(t *testing.T) {
	rp := NewReplayProtection()

	cellData := []byte("unique cell data")

	// First cell at seq 0
	err := rp.ValidateAndTrack(ReplayForward, 0, cellData)
	if err != nil {
		t.Fatalf("First ValidateAndTrack error = %v", err)
	}

	// Same data at different sequence should fail (replay with same content)
	err = rp.ValidateAndTrack(ReplayForward, 1, cellData)
	if err == nil {
		t.Error("Expected error for duplicate digest at different sequence, got nil")
	}

	stats := rp.Stats()
	if stats.ReplayAttemptsForward != 1 {
		t.Errorf("ReplayAttemptsForward = %d, want 1", stats.ReplayAttemptsForward)
	}
}

func TestValidateAndTrack_OutOfOrder(t *testing.T) {
	rp := NewReplayProtection()

	// Send cells out of order but within window
	sequences := []uint64{0, 3, 1, 2, 5, 4}
	for _, seq := range sequences {
		cellData := []byte(fmt.Sprintf("cell %d", seq))
		err := rp.ValidateAndTrack(ReplayForward, seq, cellData)
		if err != nil {
			t.Errorf("ValidateAndTrack(%d) error = %v", seq, err)
		}
	}

	stats := rp.Stats()
	if stats.OutOfOrderForward == 0 {
		t.Error("Expected out-of-order cells to be tracked")
	}
}

func TestValidateAndTrack_TooOld(t *testing.T) {
	rp := NewReplayProtectionWithWindow(10)

	// Advance the sequence
	for i := uint64(0); i < 20; i++ {
		cellData := []byte(fmt.Sprintf("cell %d", i))
		err := rp.ValidateAndTrack(ReplayForward, i, cellData)
		if err != nil {
			t.Fatalf("ValidateAndTrack(%d) error = %v", i, err)
		}
	}

	// Now try to send a very old cell (should be rejected)
	oldCellData := []byte("old cell")
	err := rp.ValidateAndTrack(ReplayForward, 2, oldCellData)
	if err == nil {
		t.Error("Expected error for too-old sequence, got nil")
	}

	stats := rp.Stats()
	if stats.ReplayAttemptsForward == 0 {
		t.Error("Expected replay attempt to be recorded for too-old cell")
	}
}

func TestValidateAndTrack_TooFarAhead(t *testing.T) {
	rp := NewReplayProtectionWithWindow(10)

	// Send first cell
	err := rp.ValidateAndTrack(ReplayForward, 0, []byte("cell 0"))
	if err != nil {
		t.Fatalf("First ValidateAndTrack error = %v", err)
	}

	// Try to send a cell too far ahead
	err = rp.ValidateAndTrack(ReplayForward, 1000, []byte("far future cell"))
	if err == nil {
		t.Error("Expected error for sequence too far ahead, got nil")
	}
}

func TestValidateAndTrack_BackwardDirection(t *testing.T) {
	rp := NewReplayProtection()

	// Send cells in backward direction
	for i := uint64(0); i < 5; i++ {
		cellData := []byte(fmt.Sprintf("backward cell %d", i))
		err := rp.ValidateAndTrack(ReplayBackward, i, cellData)
		if err != nil {
			t.Errorf("ValidateAndTrack (backward) (%d) error = %v", i, err)
		}
	}

	stats := rp.Stats()
	if stats.BackwardSequence != 5 {
		t.Errorf("BackwardSequence = %d, want 5", stats.BackwardSequence)
	}

	// Forward should be unaffected
	if stats.ForwardSequence != 0 {
		t.Errorf("ForwardSequence = %d, want 0", stats.ForwardSequence)
	}
}

func TestValidateAndTrack_EmptyData(t *testing.T) {
	rp := NewReplayProtection()

	err := rp.ValidateAndTrack(ReplayForward, 0, []byte{})
	if err == nil {
		t.Error("Expected error for empty cell data, got nil")
	}

	err = rp.ValidateAndTrack(ReplayForward, 0, nil)
	if err == nil {
		t.Error("Expected error for nil cell data, got nil")
	}
}

func TestValidateAndTrack_BothDirections(t *testing.T) {
	rp := NewReplayProtection()

	// Send cells in both directions
	for i := uint64(0); i < 5; i++ {
		fwdData := []byte(fmt.Sprintf("forward %d", i))
		bwdData := []byte(fmt.Sprintf("backward %d", i))

		err := rp.ValidateAndTrack(ReplayForward, i, fwdData)
		if err != nil {
			t.Errorf("Forward ValidateAndTrack(%d) error = %v", i, err)
		}

		err = rp.ValidateAndTrack(ReplayBackward, i, bwdData)
		if err != nil {
			t.Errorf("Backward ValidateAndTrack(%d) error = %v", i, err)
		}
	}

	stats := rp.Stats()
	if stats.ForwardSequence != 5 {
		t.Errorf("ForwardSequence = %d, want 5", stats.ForwardSequence)
	}
	if stats.BackwardSequence != 5 {
		t.Errorf("BackwardSequence = %d, want 5", stats.BackwardSequence)
	}
}

func TestGetNextSequence(t *testing.T) {
	rp := NewReplayProtection()

	// Get forward sequences
	for i := uint64(0); i < 5; i++ {
		seq := rp.GetNextSequence(ReplayForward)
		if seq != i {
			t.Errorf("GetNextSequence (forward) = %d, want %d", seq, i)
		}
	}

	// Get backward sequences
	for i := uint64(0); i < 3; i++ {
		seq := rp.GetNextSequence(ReplayBackward)
		if seq != i {
			t.Errorf("GetNextSequence (backward) = %d, want %d", seq, i)
		}
	}

	stats := rp.Stats()
	if stats.ForwardSequence != 5 {
		t.Errorf("ForwardSequence = %d, want 5", stats.ForwardSequence)
	}
	if stats.BackwardSequence != 3 {
		t.Errorf("BackwardSequence = %d, want 3", stats.BackwardSequence)
	}
}

func TestReset(t *testing.T) {
	rp := NewReplayProtection()

	// Add some cells
	for i := uint64(0); i < 10; i++ {
		rp.ValidateAndTrack(ReplayForward, i, []byte(fmt.Sprintf("cell %d", i)))
		rp.ValidateAndTrack(ReplayBackward, i, []byte(fmt.Sprintf("back cell %d", i)))
	}

	// Trigger a replay to have non-zero stats
	rp.ValidateAndTrack(ReplayForward, 0, []byte("cell 0"))

	// Reset
	rp.Reset()

	stats := rp.Stats()
	if stats.ForwardSequence != 0 {
		t.Errorf("After reset ForwardSequence = %d, want 0", stats.ForwardSequence)
	}
	if stats.BackwardSequence != 0 {
		t.Errorf("After reset BackwardSequence = %d, want 0", stats.BackwardSequence)
	}
	if stats.ForwardWindowSize != 0 {
		t.Errorf("After reset ForwardWindowSize = %d, want 0", stats.ForwardWindowSize)
	}
	if stats.BackwardWindowSize != 0 {
		t.Errorf("After reset BackwardWindowSize = %d, want 0", stats.BackwardWindowSize)
	}

	// Stats should be preserved (for debugging)
	if stats.ReplayAttemptsForward != 1 {
		t.Errorf("After reset ReplayAttemptsForward = %d, want 1 (preserved)", stats.ReplayAttemptsForward)
	}
}

func TestTotalReplayAttempts(t *testing.T) {
	rp := NewReplayProtection()

	// Normal cell
	rp.ValidateAndTrack(ReplayForward, 0, []byte("cell 0"))
	rp.ValidateAndTrack(ReplayBackward, 0, []byte("back cell 0"))

	// Replay attempts
	rp.ValidateAndTrack(ReplayForward, 0, []byte("cell 0"))   // Replay
	rp.ValidateAndTrack(ReplayBackward, 0, []byte("back cell 0")) // Replay

	total := rp.TotalReplayAttempts()
	if total != 2 {
		t.Errorf("TotalReplayAttempts = %d, want 2", total)
	}
}

func TestWindowCleanup(t *testing.T) {
	rp := NewReplayProtectionWithWindow(5)

	// Add cells 0-9
	for i := uint64(0); i < 10; i++ {
		rp.ValidateAndTrack(ReplayForward, i, []byte(fmt.Sprintf("cell %d", i)))
	}

	stats := rp.Stats()

	// Window should only contain recent entries (within window size)
	// After processing 0-9 with window size 5, window should contain ~5 entries
	if stats.ForwardWindowSize > 6 {
		t.Errorf("ForwardWindowSize = %d, expected <= 6 after cleanup", stats.ForwardWindowSize)
	}

	// Old entries should be cleaned up, so we can't detect old replays
	// but should reject based on sequence being too old
	err := rp.ValidateAndTrack(ReplayForward, 0, []byte("new cell 0 data"))
	if err == nil {
		t.Error("Expected error for too-old sequence, got nil")
	}
}

func TestConcurrentAccess(t *testing.T) {
	rp := NewReplayProtection()
	var wg sync.WaitGroup

	// Launch multiple goroutines accessing replay protection
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				seq := uint64(gid*100 + i)
				cellData := []byte(fmt.Sprintf("goroutine %d cell %d", gid, i))

				direction := ReplayForward
				if gid%2 == 0 {
					direction = ReplayBackward
				}

				// We don't care about errors here, just checking for races
				rp.ValidateAndTrack(direction, seq, cellData)
			}
		}(g)
	}

	wg.Wait()

	// Also test concurrent stats reading
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				_ = rp.Stats()
				_ = rp.TotalReplayAttempts()
			}
		}()
	}

	wg.Wait()
}

func TestReplayDirectionConstants(t *testing.T) {
	// Ensure direction constants are distinct
	if ReplayForward == ReplayBackward {
		t.Error("ReplayForward and ReplayBackward should be distinct")
	}
}

func BenchmarkValidateAndTrack(b *testing.B) {
	rp := NewReplayProtection()
	cellData := []byte("benchmark cell data that represents a typical relay cell payload")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create unique data by appending iteration
		data := append(cellData, byte(i), byte(i>>8), byte(i>>16), byte(i>>24))
		rp.ValidateAndTrack(ReplayForward, uint64(i), data)
	}
}

func BenchmarkGetNextSequence(b *testing.B) {
	rp := NewReplayProtection()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rp.GetNextSequence(ReplayForward)
	}
}

func BenchmarkStats(b *testing.B) {
	rp := NewReplayProtection()

	// Add some data first
	for i := 0; i < 100; i++ {
		rp.ValidateAndTrack(ReplayForward, uint64(i), []byte(fmt.Sprintf("cell %d", i)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rp.Stats()
	}
}

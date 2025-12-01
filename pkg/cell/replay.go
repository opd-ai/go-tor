// Package cell provides replay protection for Tor protocol cells.
//
// Replay protection prevents attackers from capturing and replaying valid cells
// to cause duplicate actions, break anonymity through traffic correlation, or
// cause unauthorized behaviors.
//
// Implementation follows tor-spec.txt guidance on cell replay prevention using:
// - Sequence number tracking per circuit direction
// - Sliding window for accepting slightly out-of-order cells
// - Nonce/digest tracking for detecting duplicates
package cell

import (
	"crypto/sha256"
	"fmt"
	"sync"
)

// ReplayProtection provides replay protection for cells on a circuit.
// It tracks sequence numbers and cell digests to detect replayed cells.
//
// The implementation uses a sliding window approach that allows for:
// - Detection of duplicate cells via digest tracking
// - Detection of replayed sequence numbers
// - Acceptance of slightly out-of-order cells (within window size)
type ReplayProtection struct {
	mu sync.RWMutex

	// Forward direction (client -> exit)
	forwardSeq     uint64              // Next expected sequence number
	forwardWindow  map[uint64]struct{} // Seen sequence numbers in window
	forwardDigests map[[16]byte]uint64 // Cell digest -> sequence number

	// Backward direction (exit -> client)
	backwardSeq     uint64
	backwardWindow  map[uint64]struct{}
	backwardDigests map[[16]byte]uint64

	// Configuration
	windowSize uint64 // Sliding window size

	// Statistics
	replayAttemptsForward  uint64
	replayAttemptsBackward uint64
	outOfOrderForward      uint64
	outOfOrderBackward     uint64
}

// ReplayDirection indicates the direction of cell flow
type ReplayDirection int

const (
	// ReplayForward is client → exit direction
	ReplayForward ReplayDirection = iota
	// ReplayBackward is exit → client direction
	ReplayBackward
)

// DefaultWindowSize is the default sliding window size for sequence tracking.
// This allows for up to 32 cells to arrive out of order.
const DefaultWindowSize uint64 = 32

// NewReplayProtection creates a new replay protection instance with default settings.
func NewReplayProtection() *ReplayProtection {
	return NewReplayProtectionWithWindow(DefaultWindowSize)
}

// NewReplayProtectionWithWindow creates a replay protection instance with custom window size.
func NewReplayProtectionWithWindow(windowSize uint64) *ReplayProtection {
	return &ReplayProtection{
		forwardSeq:      0,
		forwardWindow:   make(map[uint64]struct{}),
		forwardDigests:  make(map[[16]byte]uint64),
		backwardSeq:     0,
		backwardWindow:  make(map[uint64]struct{}),
		backwardDigests: make(map[[16]byte]uint64),
		windowSize:      windowSize,
	}
}

// ValidateAndTrack validates a cell against replay attacks and tracks it.
// Returns an error if the cell appears to be replayed.
//
// The function:
// 1. Computes a truncated SHA-256 digest of the cell data
// 2. Checks if we've seen this exact digest before
// 3. Validates the sequence number is within acceptable window
// 4. Records the cell for future replay detection
func (r *ReplayProtection) ValidateAndTrack(direction ReplayDirection, seqNum uint64, cellData []byte) error {
	if len(cellData) == 0 {
		return fmt.Errorf("empty cell data")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Compute truncated digest (first 16 bytes of SHA-256)
	fullDigest := sha256.Sum256(cellData)
	var digest [16]byte
	copy(digest[:], fullDigest[:16])

	if direction == ReplayForward {
		return r.validateForward(seqNum, digest)
	}
	return r.validateBackward(seqNum, digest)
}

// validateForward validates and tracks a forward direction cell.
func (r *ReplayProtection) validateForward(seqNum uint64, digest [16]byte) error {
	// Check for duplicate digest
	if prevSeq, exists := r.forwardDigests[digest]; exists {
		r.replayAttemptsForward++
		return fmt.Errorf("replay detected: duplicate cell digest (original seq: %d, replay seq: %d)", prevSeq, seqNum)
	}

	// Check sequence number validity
	if err := r.validateSequence(seqNum, r.forwardSeq, r.forwardWindow, ReplayForward); err != nil {
		r.replayAttemptsForward++
		return err
	}

	// Track this cell
	r.forwardWindow[seqNum] = struct{}{}
	r.forwardDigests[digest] = seqNum

	// Track out-of-order cells
	if seqNum < r.forwardSeq {
		r.outOfOrderForward++
	}

	// Update expected sequence if this is the next one
	if seqNum >= r.forwardSeq {
		r.forwardSeq = seqNum + 1
	}

	// Cleanup old entries outside the window
	r.cleanupWindow(ReplayForward)

	return nil
}

// validateBackward validates and tracks a backward direction cell.
func (r *ReplayProtection) validateBackward(seqNum uint64, digest [16]byte) error {
	// Check for duplicate digest
	if prevSeq, exists := r.backwardDigests[digest]; exists {
		r.replayAttemptsBackward++
		return fmt.Errorf("replay detected: duplicate cell digest (original seq: %d, replay seq: %d)", prevSeq, seqNum)
	}

	// Check sequence number validity
	if err := r.validateSequence(seqNum, r.backwardSeq, r.backwardWindow, ReplayBackward); err != nil {
		r.replayAttemptsBackward++
		return err
	}

	// Track this cell
	r.backwardWindow[seqNum] = struct{}{}
	r.backwardDigests[digest] = seqNum

	// Track out-of-order cells
	if seqNum < r.backwardSeq {
		r.outOfOrderBackward++
	}

	// Update expected sequence if this is the next one
	if seqNum >= r.backwardSeq {
		r.backwardSeq = seqNum + 1
	}

	// Cleanup old entries outside the window
	r.cleanupWindow(ReplayBackward)

	return nil
}

// validateSequence checks if a sequence number is valid within the sliding window.
func (r *ReplayProtection) validateSequence(seqNum, expectedSeq uint64, window map[uint64]struct{}, _ ReplayDirection) error {
	// Check if sequence number was already seen
	if _, exists := window[seqNum]; exists {
		return fmt.Errorf("replay detected: duplicate sequence number %d", seqNum)
	}

	// Check if sequence number is too old (before the window)
	if expectedSeq > r.windowSize && seqNum < expectedSeq-r.windowSize {
		return fmt.Errorf("replay detected: sequence number %d too old (expected >= %d)", seqNum, expectedSeq-r.windowSize)
	}

	// Check if sequence number is too far ahead (possible attack or major network issue)
	maxAhead := r.windowSize * 2 // Allow some leeway for network conditions
	if seqNum > expectedSeq+maxAhead {
		return fmt.Errorf("sequence number %d too far ahead (expected near %d)", seqNum, expectedSeq)
	}

	return nil
}

// cleanupWindow removes old entries outside the sliding window.
func (r *ReplayProtection) cleanupWindow(direction ReplayDirection) {
	var window map[uint64]struct{}
	var digests map[[16]byte]uint64
	var expectedSeq uint64

	if direction == ReplayForward {
		window = r.forwardWindow
		digests = r.forwardDigests
		expectedSeq = r.forwardSeq
	} else {
		window = r.backwardWindow
		digests = r.backwardDigests
		expectedSeq = r.backwardSeq
	}

	// Calculate the minimum valid sequence number
	minValidSeq := uint64(0)
	if expectedSeq > r.windowSize {
		minValidSeq = expectedSeq - r.windowSize
	}

	// Remove old sequence numbers from window
	for seq := range window {
		if seq < minValidSeq {
			delete(window, seq)
		}
	}

	// Remove old digests
	for digest, seq := range digests {
		if seq < minValidSeq {
			delete(digests, digest)
		}
	}
}

// Stats returns replay protection statistics.
type Stats struct {
	ForwardSequence        uint64
	BackwardSequence       uint64
	ForwardWindowSize      int
	BackwardWindowSize     int
	ForwardDigests         int
	BackwardDigests        int
	ReplayAttemptsForward  uint64
	ReplayAttemptsBackward uint64
	OutOfOrderForward      uint64
	OutOfOrderBackward     uint64
}

// Stats returns the current replay protection statistics.
func (r *ReplayProtection) Stats() Stats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return Stats{
		ForwardSequence:        r.forwardSeq,
		BackwardSequence:       r.backwardSeq,
		ForwardWindowSize:      len(r.forwardWindow),
		BackwardWindowSize:     len(r.backwardWindow),
		ForwardDigests:         len(r.forwardDigests),
		BackwardDigests:        len(r.backwardDigests),
		ReplayAttemptsForward:  r.replayAttemptsForward,
		ReplayAttemptsBackward: r.replayAttemptsBackward,
		OutOfOrderForward:      r.outOfOrderForward,
		OutOfOrderBackward:     r.outOfOrderBackward,
	}
}

// Reset clears all replay protection state.
// This should be called when a circuit is torn down and recreated.
func (r *ReplayProtection) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.forwardSeq = 0
	r.forwardWindow = make(map[uint64]struct{})
	r.forwardDigests = make(map[[16]byte]uint64)

	r.backwardSeq = 0
	r.backwardWindow = make(map[uint64]struct{})
	r.backwardDigests = make(map[[16]byte]uint64)

	// Keep statistics for debugging purposes
}

// GetNextSequence returns the next sequence number to use for a direction.
// This is used by the sending side to assign sequence numbers to cells.
func (r *ReplayProtection) GetNextSequence(direction ReplayDirection) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	if direction == ReplayForward {
		seq := r.forwardSeq
		r.forwardSeq++
		return seq
	}

	seq := r.backwardSeq
	r.backwardSeq++
	return seq
}

// TotalReplayAttempts returns the total number of detected replay attempts.
func (r *ReplayProtection) TotalReplayAttempts() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.replayAttemptsForward + r.replayAttemptsBackward
}

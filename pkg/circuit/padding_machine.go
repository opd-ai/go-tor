// Package circuit provides padding machine state management per padding-spec.txt
package circuit

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
)

// PaddingMachineType identifies different padding machine implementations
type PaddingMachineType byte

const (
	// PaddingMachineNone indicates no padding machine is active
	PaddingMachineNone PaddingMachineType = 0
	// PaddingMachineAPE is the Adaptive Padding Engine from padding-spec.txt
	PaddingMachineAPE PaddingMachineType = 1
	// PaddingMachineCircuitSetup is for circuit setup phase padding
	PaddingMachineCircuitSetup PaddingMachineType = 2
)

// PaddingMachineState represents the current state of a padding machine
type PaddingMachineState byte

const (
	// MachineStateStart is the initial state
	MachineStateStart PaddingMachineState = 0
	// MachineStateBurst is when padding cells are sent in bursts
	MachineStateBurst PaddingMachineState = 1
	// MachineStateGap is the idle period between bursts
	MachineStateGap PaddingMachineState = 2
	// MachineStateEnd is the terminal state
	MachineStateEnd PaddingMachineState = 3
)

// String returns a human-readable state name
func (s PaddingMachineState) String() string {
	switch s {
	case MachineStateStart:
		return "START"
	case MachineStateBurst:
		return "BURST"
	case MachineStateGap:
		return "GAP"
	case MachineStateEnd:
		return "END"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", s)
	}
}

// StateMachine implements a formal padding state machine per padding-spec.txt
type StateMachine struct {
	mu          sync.RWMutex
	machineType PaddingMachineType
	state       PaddingMachineState
	circuit     *Circuit

	// State transition parameters
	burstMin      int           // Minimum cells in a burst
	burstMax      int           // Maximum cells in a burst
	gapMin        time.Duration // Minimum gap between bursts
	gapMax        time.Duration // Maximum gap between bursts
	cellDelay     time.Duration // Delay between cells within a burst

	// Runtime state
	cellsInBurst  int       // Cells sent in current burst
	burstTarget   int       // Target cells for current burst
	lastSentTime  time.Time // Last time we sent a padding cell
	nextEventTime time.Time // When next state transition should occur

	// Statistics
	totalPaddingSent uint64
	burstCount       uint64
}

// PaddingMachineParams contains configurable parameters for padding machines
// These can be set from consensus parameters via directory.GetPaddingParams()
type PaddingMachineParams struct {
	BurstMin      int           // Minimum cells in a burst
	BurstMax      int           // Maximum cells in a burst
	GapMin        time.Duration // Minimum gap between bursts
	GapMax        time.Duration // Maximum gap between bursts
	CellDelay     time.Duration // Delay between cells within a burst
}

// DefaultAPEParams returns default parameters for APE machine per padding-spec.txt §3
func DefaultAPEParams() *PaddingMachineParams {
	return &PaddingMachineParams{
		BurstMin:  2,
		BurstMax:  10,
		GapMin:    1500 * time.Millisecond,
		GapMax:    9500 * time.Millisecond,
		CellDelay: 20 * time.Millisecond,
	}
}

// DefaultCircuitSetupParams returns default parameters for circuit setup machine
func DefaultCircuitSetupParams() *PaddingMachineParams {
	return &PaddingMachineParams{
		BurstMin:  1,
		BurstMax:  5,
		GapMin:    500 * time.Millisecond,
		GapMax:    2000 * time.Millisecond,
		CellDelay: 50 * time.Millisecond,
	}
}

// NewAPEMachine creates an Adaptive Padding Engine state machine
// Parameters are based on padding-spec.txt recommendations
func NewAPEMachine(circuit *Circuit) *StateMachine {
	return NewAPEMachineWithParams(circuit, DefaultAPEParams())
}

// NewAPEMachineWithParams creates an APE machine with custom parameters from consensus
func NewAPEMachineWithParams(circuit *Circuit, params *PaddingMachineParams) *StateMachine {
	return &StateMachine{
		machineType: PaddingMachineAPE,
		state:       MachineStateStart,
		circuit:     circuit,
		burstMin:    params.BurstMin,
		burstMax:    params.BurstMax,
		gapMin:      params.GapMin,
		gapMax:      params.GapMax,
		cellDelay:   params.CellDelay,
	}
}

// NewCircuitSetupMachine creates a padding machine for circuit setup phase
func NewCircuitSetupMachine(circuit *Circuit) *StateMachine {
	return NewCircuitSetupMachineWithParams(circuit, DefaultCircuitSetupParams())
}

// NewCircuitSetupMachineWithParams creates a circuit setup machine with custom parameters
func NewCircuitSetupMachineWithParams(circuit *Circuit, params *PaddingMachineParams) *StateMachine {
	return &StateMachine{
		machineType: PaddingMachineCircuitSetup,
		state:       MachineStateStart,
		circuit:     circuit,
		burstMin:    params.BurstMin,
		burstMax:    params.BurstMax,
		gapMin:      params.GapMin,
		gapMax:      params.GapMax,
		cellDelay:   params.CellDelay,
	}
}

// Start transitions the machine from START to BURST state
func (sm *StateMachine) Start() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.state != MachineStateStart {
		return fmt.Errorf("cannot start from state %s", sm.state)
	}

	sm.state = MachineStateBurst
	sm.burstTarget = sm.randomRange(sm.burstMin, sm.burstMax)
	sm.cellsInBurst = 0
	sm.burstCount++
	return nil
}

// ProcessEvent handles state machine events (cell sent, timeout, etc.)
// Returns true if a padding cell should be sent
func (sm *StateMachine) ProcessEvent() (shouldPad bool, nextDelay time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()

	switch sm.state {
	case MachineStateStart:
		// Should not be in START state during processing
		return false, time.Hour

	case MachineStateBurst:
		// Check if we should send another cell in this burst
		if sm.cellsInBurst < sm.burstTarget {
			// Send a cell after cellDelay
			timeSinceLastCell := now.Sub(sm.lastSentTime)
			if timeSinceLastCell >= sm.cellDelay || sm.cellsInBurst == 0 {
				sm.cellsInBurst++
				sm.lastSentTime = now
				sm.totalPaddingSent++

				// Check if burst is complete
				if sm.cellsInBurst >= sm.burstTarget {
					sm.transitionToGap()
					return true, sm.randomDuration(sm.gapMin, sm.gapMax)
				}
				return true, sm.cellDelay
			}
			return false, sm.cellDelay - timeSinceLastCell
		}

		// Burst complete, transition to GAP
		sm.transitionToGap()
		return false, sm.randomDuration(sm.gapMin, sm.gapMax)

	case MachineStateGap:
		// Check if gap period is over
		if now.After(sm.nextEventTime) {
			sm.transitionToBurst()
			return false, sm.cellDelay // Start next burst soon
		}
		return false, time.Until(sm.nextEventTime)

	case MachineStateEnd:
		return false, time.Hour // Machine stopped

	default:
		return false, time.Hour
	}
}

// transitionToGap moves from BURST to GAP state
func (sm *StateMachine) transitionToGap() {
	sm.state = MachineStateGap
	gapDuration := sm.randomDuration(sm.gapMin, sm.gapMax)
	sm.nextEventTime = time.Now().Add(gapDuration)
}

// transitionToBurst moves from GAP to BURST state
func (sm *StateMachine) transitionToBurst() {
	sm.state = MachineStateBurst
	sm.burstTarget = sm.randomRange(sm.burstMin, sm.burstMax)
	sm.cellsInBurst = 0
	sm.burstCount++
}

// Stop transitions the machine to END state
func (sm *StateMachine) Stop() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.state = MachineStateEnd
}

// GetState returns the current state
func (sm *StateMachine) GetState() PaddingMachineState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

// GetStats returns statistics about the machine
func (sm *StateMachine) GetStats() StateMachineStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return StateMachineStats{
		State:            sm.state,
		TotalPaddingSent: sm.totalPaddingSent,
		BurstCount:       sm.burstCount,
	}
}

// StateMachineStats contains statistics about a padding machine
type StateMachineStats struct {
	State            PaddingMachineState
	TotalPaddingSent uint64
	BurstCount       uint64
}

// randomRange returns a random integer in [min, max]
func (sm *StateMachine) randomRange(min, max int) int {
	if min >= max {
		return min
	}
	rangeSize := uint32(max - min + 1)
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return min // Fallback on error
	}
	n := binary.BigEndian.Uint32(buf[:])
	return min + int(n%rangeSize)
}

// randomDuration returns a cryptographically random duration between min and max
func (sm *StateMachine) randomDuration(min, max time.Duration) time.Duration {
	if min >= max {
		return min
	}
	rangeSize := uint64(max - min)
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return min // Fallback on error
	}
	n := binary.BigEndian.Uint64(buf[:])
	return min + time.Duration(n%rangeSize)
}

// PaddingNegotiateRequest represents a PADDING_NEGOTIATE cell payload
type PaddingNegotiateRequest struct {
	Version     byte               // Protocol version (0 for now)
	Command     byte               // 1 = START, 2 = STOP
	MachineType PaddingMachineType // Type of padding machine to negotiate
}

// PaddingNegotiateResponse represents a PADDING_NEGOTIATED cell payload
type PaddingNegotiateResponse struct {
	Version     byte               // Protocol version (0 for now)
	Command     byte               // 1 = STARTED, 2 = STOPPED, 3 = ERROR
	MachineType PaddingMachineType // Type of padding machine negotiated
}

// Padding negotiate commands
const (
	PaddingCommandStart byte = 1
	PaddingCommandStop  byte = 2
)

// Padding negotiate responses
const (
	PaddingResponseStarted byte = 1
	PaddingResponseStopped byte = 2
	PaddingResponseError   byte = 3
)

// EncodePaddingNegotiate encodes a PADDING_NEGOTIATE request
func EncodePaddingNegotiate(req *PaddingNegotiateRequest) ([]byte, error) {
	if req == nil {
		return nil, errors.New("request cannot be nil")
	}
	// Version(1) + Command(1) + MachineType(1) = 3 bytes minimum
	payload := make([]byte, 3)
	payload[0] = req.Version
	payload[1] = req.Command
	payload[2] = byte(req.MachineType)
	return payload, nil
}

// DecodePaddingNegotiate decodes a PADDING_NEGOTIATE request
func DecodePaddingNegotiate(data []byte) (*PaddingNegotiateRequest, error) {
	if len(data) < 3 {
		return nil, fmt.Errorf("padding negotiate payload too short: %d < 3", len(data))
	}
	return &PaddingNegotiateRequest{
		Version:     data[0],
		Command:     data[1],
		MachineType: PaddingMachineType(data[2]),
	}, nil
}

// EncodePaddingNegotiated encodes a PADDING_NEGOTIATED response
func EncodePaddingNegotiated(resp *PaddingNegotiateResponse) ([]byte, error) {
	if resp == nil {
		return nil, errors.New("response cannot be nil")
	}
	payload := make([]byte, 3)
	payload[0] = resp.Version
	payload[1] = resp.Command
	payload[2] = byte(resp.MachineType)
	return payload, nil
}

// DecodePaddingNegotiated decodes a PADDING_NEGOTIATED response
func DecodePaddingNegotiated(data []byte) (*PaddingNegotiateResponse, error) {
	if len(data) < 3 {
		return nil, fmt.Errorf("padding negotiated payload too short: %d < 3", len(data))
	}
	return &PaddingNegotiateResponse{
		Version:     data[0],
		Command:     data[1],
		MachineType: PaddingMachineType(data[2]),
	}, nil
}

// SendPaddingNegotiate sends a PADDING_NEGOTIATE cell to negotiate padding
func (c *Circuit) SendPaddingNegotiate(machineType PaddingMachineType, start bool) error {
	cmd := PaddingCommandStop
	if start {
		cmd = PaddingCommandStart
	}

	req := &PaddingNegotiateRequest{
		Version:     0,
		Command:     cmd,
		MachineType: machineType,
	}

	payload, err := EncodePaddingNegotiate(req)
	if err != nil {
		return fmt.Errorf("failed to encode padding negotiate: %w", err)
	}

	relayCell, err := cell.NewRelayCell(0, cell.RelayPaddingNegotiate, payload)
	if err != nil {
		return fmt.Errorf("failed to create relay cell: %w", err)
	}

	return c.SendRelayCell(relayCell)
}

// HandlePaddingNegotiate processes an incoming PADDING_NEGOTIATE cell
func (c *Circuit) HandlePaddingNegotiate(data []byte) error {
	req, err := DecodePaddingNegotiate(data)
	if err != nil {
		return fmt.Errorf("failed to decode padding negotiate: %w", err)
	}

	// For now, we accept all padding negotiation requests
	// In a full implementation, we would check machine type support
	var responseCmd byte
	if req.Command == PaddingCommandStart {
		responseCmd = PaddingResponseStarted
	} else {
		responseCmd = PaddingResponseStopped
	}

	resp := &PaddingNegotiateResponse{
		Version:     0,
		Command:     responseCmd,
		MachineType: req.MachineType,
	}

	payload, err := EncodePaddingNegotiated(resp)
	if err != nil {
		return fmt.Errorf("failed to encode padding negotiated: %w", err)
	}

	relayCell, err := cell.NewRelayCell(0, cell.RelayPaddingNegotiated, payload)
	if err != nil {
		return fmt.Errorf("failed to create relay cell: %w", err)
	}

	return c.SendRelayCell(relayCell)
}

package circuit

import (
	"testing"
	"time"
)

func TestPaddingMachineState_String(t *testing.T) {
	tests := []struct {
		state    PaddingMachineState
		expected string
	}{
		{MachineStateStart, "START"},
		{MachineStateBurst, "BURST"},
		{MachineStateGap, "GAP"},
		{MachineStateEnd, "END"},
		{PaddingMachineState(99), "UNKNOWN(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewAPEMachine(t *testing.T) {
	circuit := NewCircuit(1)
	sm := NewAPEMachine(circuit)

	if sm == nil {
		t.Fatal("NewAPEMachine() returned nil")
	}
	if sm.machineType != PaddingMachineAPE {
		t.Errorf("machineType = %v, want %v", sm.machineType, PaddingMachineAPE)
	}
	if sm.state != MachineStateStart {
		t.Errorf("state = %v, want %v", sm.state, MachineStateStart)
	}
	if sm.circuit != circuit {
		t.Error("circuit not set correctly")
	}
}

func TestNewCircuitSetupMachine(t *testing.T) {
	circuit := NewCircuit(1)
	sm := NewCircuitSetupMachine(circuit)

	if sm == nil {
		t.Fatal("NewCircuitSetupMachine() returned nil")
	}
	if sm.machineType != PaddingMachineCircuitSetup {
		t.Errorf("machineType = %v, want %v", sm.machineType, PaddingMachineCircuitSetup)
	}
	if sm.state != MachineStateStart {
		t.Errorf("state = %v, want %v", sm.state, MachineStateStart)
	}
}

func TestStateMachine_Start(t *testing.T) {
	circuit := NewCircuit(1)
	sm := NewAPEMachine(circuit)

	// Start from START state
	err := sm.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if sm.GetState() != MachineStateBurst {
		t.Errorf("state = %v, want %v", sm.GetState(), MachineStateBurst)
	}

	// Starting again should fail
	err = sm.Start()
	if err == nil {
		t.Error("Start() from non-START state should return error")
	}
}

func TestStateMachine_Stop(t *testing.T) {
	circuit := NewCircuit(1)
	sm := NewAPEMachine(circuit)

	sm.Start()
	sm.Stop()

	if sm.GetState() != MachineStateEnd {
		t.Errorf("state = %v, want %v", sm.GetState(), MachineStateEnd)
	}
}

func TestStateMachine_ProcessEvent_Burst(t *testing.T) {
	circuit := NewCircuit(1)
	sm := NewAPEMachine(circuit)

	sm.Start()

	// First event in burst should trigger padding
	shouldPad, delay := sm.ProcessEvent()
	if !shouldPad {
		t.Error("First event in burst should trigger padding")
	}
	if delay <= 0 {
		t.Error("Delay should be positive")
	}

	// Record stats
	stats := sm.GetStats()
	if stats.TotalPaddingSent == 0 {
		t.Error("TotalPaddingSent should be > 0")
	}
	if stats.BurstCount == 0 {
		t.Error("BurstCount should be > 0")
	}
}

func TestStateMachine_ProcessEvent_Gap(t *testing.T) {
	circuit := NewCircuit(1)
	sm := NewAPEMachine(circuit)

	// Force machine into GAP state
	sm.mu.Lock()
	sm.state = MachineStateGap
	sm.nextEventTime = time.Now().Add(1 * time.Hour)
	sm.mu.Unlock()

	// During gap, should not pad
	shouldPad, delay := sm.ProcessEvent()
	if shouldPad {
		t.Error("Should not pad during gap")
	}
	if delay <= 0 {
		t.Error("Delay should be positive during gap")
	}
}

func TestStateMachine_ProcessEvent_End(t *testing.T) {
	circuit := NewCircuit(1)
	sm := NewAPEMachine(circuit)

	sm.Stop()

	shouldPad, _ := sm.ProcessEvent()
	if shouldPad {
		t.Error("Should not pad in END state")
	}
}

func TestStateMachine_BurstCompletion(t *testing.T) {
	circuit := NewCircuit(1)
	sm := NewAPEMachine(circuit)

	// Set small burst for testing
	sm.mu.Lock()
	sm.burstMin = 2
	sm.burstMax = 2
	sm.cellDelay = 1 * time.Millisecond
	sm.mu.Unlock()

	sm.Start()

	// Process events until burst completes
	maxIterations := 10
	for i := 0; i < maxIterations; i++ {
		shouldPad, delay := sm.ProcessEvent()
		if shouldPad {
			time.Sleep(delay)
		}

		if sm.GetState() == MachineStateGap {
			// Burst completed successfully
			return
		}
	}

	t.Error("Burst did not complete within expected iterations")
}

func TestStateMachine_GapToBurst(t *testing.T) {
	circuit := NewCircuit(1)
	sm := NewAPEMachine(circuit)

	// Force machine into GAP state with immediate transition
	sm.mu.Lock()
	sm.state = MachineStateGap
	sm.nextEventTime = time.Now().Add(-1 * time.Second) // In the past
	sm.mu.Unlock()

	// Process event should transition to BURST
	sm.ProcessEvent()

	if sm.GetState() != MachineStateBurst {
		t.Errorf("state = %v, want %v after gap expiry", sm.GetState(), MachineStateBurst)
	}
}

func TestStateMachine_RandomRange(t *testing.T) {
	circuit := NewCircuit(1)
	sm := NewAPEMachine(circuit)

	min, max := 5, 10
	for i := 0; i < 100; i++ {
		val := sm.randomRange(min, max)
		if val < min || val > max {
			t.Errorf("randomRange(%d, %d) = %d, out of range", min, max, val)
		}
	}

	// Test edge case where min == max
	val := sm.randomRange(5, 5)
	if val != 5 {
		t.Errorf("randomRange(5, 5) = %d, want 5", val)
	}

	// Test edge case where min > max
	val = sm.randomRange(10, 5)
	if val != 10 {
		t.Errorf("randomRange(10, 5) = %d, want 10", val)
	}
}

func TestStateMachine_RandomDuration(t *testing.T) {
	circuit := NewCircuit(1)
	sm := NewAPEMachine(circuit)

	min := 100 * time.Millisecond
	max := 500 * time.Millisecond

	for i := 0; i < 100; i++ {
		d := sm.randomDuration(min, max)
		if d < min || d >= max {
			t.Errorf("randomDuration(%v, %v) = %v, out of range", min, max, d)
		}
	}

	// Test edge case where min >= max
	d := sm.randomDuration(max, min)
	if d != max {
		t.Errorf("randomDuration(max, min) = %v, want %v", d, max)
	}
}

func TestEncodePaddingNegotiate(t *testing.T) {
	req := &PaddingNegotiateRequest{
		Version:     0,
		Command:     PaddingCommandStart,
		MachineType: PaddingMachineAPE,
	}

	payload, err := EncodePaddingNegotiate(req)
	if err != nil {
		t.Fatalf("EncodePaddingNegotiate() error = %v", err)
	}

	if len(payload) < 3 {
		t.Errorf("payload length = %d, want >= 3", len(payload))
	}
	if payload[0] != 0 {
		t.Errorf("version = %d, want 0", payload[0])
	}
	if payload[1] != PaddingCommandStart {
		t.Errorf("command = %d, want %d", payload[1], PaddingCommandStart)
	}
	if payload[2] != byte(PaddingMachineAPE) {
		t.Errorf("machineType = %d, want %d", payload[2], PaddingMachineAPE)
	}

	// Test nil request
	_, err = EncodePaddingNegotiate(nil)
	if err == nil {
		t.Error("EncodePaddingNegotiate(nil) should return error")
	}
}

func TestDecodePaddingNegotiate(t *testing.T) {
	payload := []byte{0, PaddingCommandStart, byte(PaddingMachineAPE)}

	req, err := DecodePaddingNegotiate(payload)
	if err != nil {
		t.Fatalf("DecodePaddingNegotiate() error = %v", err)
	}

	if req.Version != 0 {
		t.Errorf("Version = %d, want 0", req.Version)
	}
	if req.Command != PaddingCommandStart {
		t.Errorf("Command = %d, want %d", req.Command, PaddingCommandStart)
	}
	if req.MachineType != PaddingMachineAPE {
		t.Errorf("MachineType = %d, want %d", req.MachineType, PaddingMachineAPE)
	}

	// Test short payload
	_, err = DecodePaddingNegotiate([]byte{0, 1})
	if err == nil {
		t.Error("DecodePaddingNegotiate with short payload should return error")
	}
}

func TestEncodePaddingNegotiated(t *testing.T) {
	resp := &PaddingNegotiateResponse{
		Version:     0,
		Command:     PaddingResponseStarted,
		MachineType: PaddingMachineAPE,
	}

	payload, err := EncodePaddingNegotiated(resp)
	if err != nil {
		t.Fatalf("EncodePaddingNegotiated() error = %v", err)
	}

	if len(payload) < 3 {
		t.Errorf("payload length = %d, want >= 3", len(payload))
	}
	if payload[0] != 0 {
		t.Errorf("version = %d, want 0", payload[0])
	}
	if payload[1] != PaddingResponseStarted {
		t.Errorf("command = %d, want %d", payload[1], PaddingResponseStarted)
	}

	// Test nil response
	_, err = EncodePaddingNegotiated(nil)
	if err == nil {
		t.Error("EncodePaddingNegotiated(nil) should return error")
	}
}

func TestDecodePaddingNegotiated(t *testing.T) {
	payload := []byte{0, PaddingResponseStarted, byte(PaddingMachineAPE)}

	resp, err := DecodePaddingNegotiated(payload)
	if err != nil {
		t.Fatalf("DecodePaddingNegotiated() error = %v", err)
	}

	if resp.Version != 0 {
		t.Errorf("Version = %d, want 0", resp.Version)
	}
	if resp.Command != PaddingResponseStarted {
		t.Errorf("Command = %d, want %d", resp.Command, PaddingResponseStarted)
	}
	if resp.MachineType != PaddingMachineAPE {
		t.Errorf("MachineType = %d, want %d", resp.MachineType, PaddingMachineAPE)
	}

	// Test short payload
	_, err = DecodePaddingNegotiated([]byte{0, 1})
	if err == nil {
		t.Error("DecodePaddingNegotiated with short payload should return error")
	}
}

func TestPaddingNegotiateRoundTrip(t *testing.T) {
	req := &PaddingNegotiateRequest{
		Version:     0,
		Command:     PaddingCommandStart,
		MachineType: PaddingMachineCircuitSetup,
	}

	payload, err := EncodePaddingNegotiate(req)
	if err != nil {
		t.Fatalf("EncodePaddingNegotiate() error = %v", err)
	}

	decoded, err := DecodePaddingNegotiate(payload)
	if err != nil {
		t.Fatalf("DecodePaddingNegotiate() error = %v", err)
	}

	if decoded.Version != req.Version {
		t.Errorf("Version = %d, want %d", decoded.Version, req.Version)
	}
	if decoded.Command != req.Command {
		t.Errorf("Command = %d, want %d", decoded.Command, req.Command)
	}
	if decoded.MachineType != req.MachineType {
		t.Errorf("MachineType = %d, want %d", decoded.MachineType, req.MachineType)
	}
}

func TestPaddingNegotiatedRoundTrip(t *testing.T) {
	resp := &PaddingNegotiateResponse{
		Version:     0,
		Command:     PaddingResponseStopped,
		MachineType: PaddingMachineAPE,
	}

	payload, err := EncodePaddingNegotiated(resp)
	if err != nil {
		t.Fatalf("EncodePaddingNegotiated() error = %v", err)
	}

	decoded, err := DecodePaddingNegotiated(payload)
	if err != nil {
		t.Fatalf("DecodePaddingNegotiated() error = %v", err)
	}

	if decoded.Version != resp.Version {
		t.Errorf("Version = %d, want %d", decoded.Version, resp.Version)
	}
	if decoded.Command != resp.Command {
		t.Errorf("Command = %d, want %d", decoded.Command, resp.Command)
	}
	if decoded.MachineType != resp.MachineType {
		t.Errorf("MachineType = %d, want %d", decoded.MachineType, resp.MachineType)
	}
}

func TestStateMachine_GetStats(t *testing.T) {
	circuit := NewCircuit(1)
	sm := NewAPEMachine(circuit)

	initialStats := sm.GetStats()
	if initialStats.State != MachineStateStart {
		t.Errorf("initial state = %v, want %v", initialStats.State, MachineStateStart)
	}
	if initialStats.TotalPaddingSent != 0 {
		t.Errorf("initial TotalPaddingSent = %d, want 0", initialStats.TotalPaddingSent)
	}
	if initialStats.BurstCount != 0 {
		t.Errorf("initial BurstCount = %d, want 0", initialStats.BurstCount)
	}

	sm.Start()
	sm.ProcessEvent()

	stats := sm.GetStats()
	if stats.TotalPaddingSent == 0 {
		t.Error("TotalPaddingSent should be > 0 after processing event")
	}
	if stats.BurstCount == 0 {
		t.Error("BurstCount should be > 0 after start")
	}
}

func TestStateMachine_ConcurrentAccess(t *testing.T) {
	circuit := NewCircuit(1)
	sm := NewAPEMachine(circuit)

	sm.Start()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			sm.ProcessEvent()
		}
		close(done)
	}()

	for i := 0; i < 100; i++ {
		_ = sm.GetStats()
		_ = sm.GetState()
	}

	<-done
}

func TestAPEMachine_Parameters(t *testing.T) {
	circuit := NewCircuit(1)
	sm := NewAPEMachine(circuit)

	// Verify APE parameters match spec recommendations
	if sm.burstMin != 2 {
		t.Errorf("APE burstMin = %d, want 2", sm.burstMin)
	}
	if sm.burstMax != 10 {
		t.Errorf("APE burstMax = %d, want 10", sm.burstMax)
	}
	if sm.gapMin != 1500*time.Millisecond {
		t.Errorf("APE gapMin = %v, want 1500ms", sm.gapMin)
	}
	if sm.gapMax != 9500*time.Millisecond {
		t.Errorf("APE gapMax = %v, want 9500ms", sm.gapMax)
	}
}

func TestCircuitSetupMachine_Parameters(t *testing.T) {
	circuit := NewCircuit(1)
	sm := NewCircuitSetupMachine(circuit)

	// Verify circuit setup machine has more aggressive parameters
	if sm.burstMin < 1 {
		t.Errorf("CircuitSetup burstMin = %d, should be >= 1", sm.burstMin)
	}
	if sm.gapMax > 2*time.Second {
		t.Errorf("CircuitSetup gapMax = %v, should be <= 2s", sm.gapMax)
	}
}

package circuit

import (
	"sync"
	"testing"
	"time"
)

func TestStateMachine_StartFromInvalidStates(t *testing.T) {
	tests := []struct {
		name  string
		state PaddingMachineState
	}{
		{"from BURST", MachineStateBurst},
		{"from GAP", MachineStateGap},
		{"from END", MachineStateEnd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewAPEMachine(NewCircuit(1))
			sm.mu.Lock()
			sm.state = tt.state
			sm.mu.Unlock()

			if err := sm.Start(); err == nil {
				t.Errorf("Start() from %s should error", tt.state)
			}
		})
	}
}

func TestStateMachine_DoubleStart(t *testing.T) {
	sm := NewAPEMachine(NewCircuit(1))

	if err := sm.Start(); err != nil {
		t.Fatalf("first Start() error: %v", err)
	}
	if err := sm.Start(); err == nil {
		t.Error("second Start() should error")
	}
}

func TestStateMachine_ProcessEventInStartState(t *testing.T) {
	sm := NewAPEMachine(NewCircuit(1))

	shouldPad, delay := sm.ProcessEvent()
	if shouldPad {
		t.Error("ProcessEvent in START should not pad")
	}
	if delay < time.Minute {
		t.Errorf("delay = %v, want >= 1 minute (long delay)", delay)
	}
}

func TestStateMachine_ProcessEventUnknownState(t *testing.T) {
	sm := NewAPEMachine(NewCircuit(1))
	sm.mu.Lock()
	sm.state = PaddingMachineState(255)
	sm.mu.Unlock()

	shouldPad, delay := sm.ProcessEvent()
	if shouldPad {
		t.Error("ProcessEvent in unknown state should not pad")
	}
	if delay < time.Minute {
		t.Errorf("delay = %v, want >= 1 minute", delay)
	}
}

func TestStateMachine_FullLifecycle(t *testing.T) {
	sm := NewAPEMachine(NewCircuit(1))
	sm.mu.Lock()
	sm.burstMin = 2
	sm.burstMax = 2
	sm.cellDelay = time.Millisecond
	sm.gapMin = time.Millisecond
	sm.gapMax = 2 * time.Millisecond
	sm.mu.Unlock()

	if err := sm.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if sm.GetState() != MachineStateBurst {
		t.Fatalf("state after Start = %v, want BURST", sm.GetState())
	}

	// Process through first burst into GAP
	for i := 0; i < 20; i++ {
		sm.ProcessEvent()
		time.Sleep(time.Millisecond)
		if sm.GetState() == MachineStateGap {
			break
		}
	}
	if sm.GetState() != MachineStateGap {
		t.Fatalf("expected GAP state after burst, got %v", sm.GetState())
	}

	// Force gap expiry to trigger next burst
	sm.mu.Lock()
	sm.nextEventTime = time.Now().Add(-time.Second)
	sm.mu.Unlock()
	sm.ProcessEvent()

	if sm.GetState() != MachineStateBurst {
		t.Fatalf("expected BURST after gap, got %v", sm.GetState())
	}

	// Stop and verify END
	sm.Stop()
	if sm.GetState() != MachineStateEnd {
		t.Fatalf("expected END after Stop, got %v", sm.GetState())
	}
}

func TestStateMachine_StopFromEachState(t *testing.T) {
	tests := []struct {
		name  string
		state PaddingMachineState
	}{
		{"from START", MachineStateStart},
		{"from BURST", MachineStateBurst},
		{"from GAP", MachineStateGap},
		{"from END", MachineStateEnd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewAPEMachine(NewCircuit(1))
			sm.mu.Lock()
			sm.state = tt.state
			sm.mu.Unlock()

			sm.Stop()
			if sm.GetState() != MachineStateEnd {
				t.Errorf("after Stop from %s, state = %v, want END",
					tt.name, sm.GetState())
			}
		})
	}
}

func TestStateMachine_StatsAccuracyThroughLifecycle(t *testing.T) {
	sm := NewAPEMachine(NewCircuit(1))
	sm.mu.Lock()
	sm.burstMin = 3
	sm.burstMax = 3
	sm.cellDelay = time.Millisecond
	sm.gapMin = time.Millisecond
	sm.gapMax = 2 * time.Millisecond
	sm.mu.Unlock()

	stats := sm.GetStats()
	if stats.TotalPaddingSent != 0 || stats.BurstCount != 0 {
		t.Fatal("initial stats should be zero")
	}

	sm.Start()
	stats = sm.GetStats()
	if stats.BurstCount != 1 {
		t.Errorf("BurstCount after Start = %d, want 1", stats.BurstCount)
	}

	// Process cells in the burst
	var paddingSent uint64
	for i := 0; i < 20; i++ {
		shouldPad, _ := sm.ProcessEvent()
		if shouldPad {
			paddingSent++
		}
		time.Sleep(time.Millisecond)
		if sm.GetState() != MachineStateBurst {
			break
		}
	}

	stats = sm.GetStats()
	if stats.TotalPaddingSent != paddingSent {
		t.Errorf("TotalPaddingSent = %d, want %d",
			stats.TotalPaddingSent, paddingSent)
	}
}

func TestStateMachine_CustomParamsDeterministicBurst(t *testing.T) {
	params := &PaddingMachineParams{
		BurstMin:  5,
		BurstMax:  5,
		GapMin:    time.Millisecond,
		GapMax:    time.Millisecond,
		CellDelay: time.Millisecond,
	}
	sm := NewAPEMachineWithParams(NewCircuit(1), params)
	sm.Start()

	// burstTarget should always be 5 when min==max
	sm.mu.RLock()
	target := sm.burstTarget
	sm.mu.RUnlock()
	if target != 5 {
		t.Errorf("burstTarget = %d, want 5 (deterministic)", target)
	}
}

func TestStateMachine_CustomParamsDeterministicGap(t *testing.T) {
	params := &PaddingMachineParams{
		BurstMin:  1,
		BurstMax:  1,
		GapMin:    100 * time.Millisecond,
		GapMax:    100 * time.Millisecond,
		CellDelay: time.Millisecond,
	}
	sm := NewAPEMachineWithParams(NewCircuit(1), params)
	sm.Start()

	// Complete the single-cell burst
	sm.ProcessEvent()
	time.Sleep(time.Millisecond)

	if sm.GetState() != MachineStateGap {
		t.Fatalf("expected GAP after single-cell burst, got %v", sm.GetState())
	}
}

func TestStateMachine_CustomParamsSingleCellBurst(t *testing.T) {
	params := &PaddingMachineParams{
		BurstMin:  1,
		BurstMax:  1,
		GapMin:    time.Millisecond,
		GapMax:    2 * time.Millisecond,
		CellDelay: time.Millisecond,
	}
	sm := NewAPEMachineWithParams(NewCircuit(1), params)
	sm.Start()

	shouldPad, _ := sm.ProcessEvent()
	if !shouldPad {
		t.Error("first cell in burst should pad")
	}

	// After 1 cell, should transition to GAP
	if sm.GetState() != MachineStateGap {
		t.Errorf("state = %v, want GAP after single cell", sm.GetState())
	}
}

func TestDecodePaddingNegotiate_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{"exact 3 bytes", []byte{0, 1, 2}, false},
		{"extra bytes", []byte{0, 1, 2, 99, 100}, false},
		{"0 bytes", []byte{}, true},
		{"1 byte", []byte{0}, true},
		{"2 bytes", []byte{0, 1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodePaddingNegotiate(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodePaddingNegotiate() error = %v, wantErr %v",
					err, tt.wantErr)
			}
		})
	}
}

func TestDecodePaddingNegotiated_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{"exact 3 bytes", []byte{0, 1, 2}, false},
		{"extra bytes", []byte{0, 1, 2, 3, 4, 5}, false},
		{"0 bytes", []byte{}, true},
		{"1 byte", []byte{0}, true},
		{"2 bytes", []byte{0, 1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodePaddingNegotiated(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodePaddingNegotiated() error = %v, wantErr %v",
					err, tt.wantErr)
			}
		})
	}
}

func TestStateMachine_ConcurrentStartStop(t *testing.T) {
	const goroutines = 10

	sm := NewAPEMachine(NewCircuit(1))
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = sm.Start()
		}()
		go func() {
			defer wg.Done()
			sm.Stop()
		}()
	}

	wg.Wait()

	// Machine should be in a valid state
	state := sm.GetState()
	switch state {
	case MachineStateStart, MachineStateBurst, MachineStateEnd:
		// All valid outcomes
	default:
		t.Errorf("unexpected state %v after concurrent access", state)
	}
}

func TestPaddingMachineType_DistinctValues(t *testing.T) {
	types := []PaddingMachineType{
		PaddingMachineNone,
		PaddingMachineAPE,
		PaddingMachineCircuitSetup,
	}

	seen := make(map[PaddingMachineType]bool)
	for _, mt := range types {
		if seen[mt] {
			t.Errorf("duplicate PaddingMachineType value: %d", mt)
		}
		seen[mt] = true
	}
}

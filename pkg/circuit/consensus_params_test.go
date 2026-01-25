// Package circuit tests consensus parameter integration
package circuit

import (
	"testing"
	"time"
)

func TestAPEParamsFromConsensus(t *testing.T) {
	tests := []struct {
		name     string
		params   *ConsensusParams
		validate func(*testing.T, *PaddingMachineParams)
	}{
		{
			name:   "nil params returns defaults",
			params: nil,
			validate: func(t *testing.T, p *PaddingMachineParams) {
				if p.BurstMin != 2 {
					t.Errorf("expected BurstMin=2, got %d", p.BurstMin)
				}
				if p.BurstMax != 10 {
					t.Errorf("expected BurstMax=10, got %d", p.BurstMax)
				}
			},
		},
		{
			name: "disabled padding returns defaults",
			params: &ConsensusParams{
				PaddingDisabled: true,
				APEBurstMin:     5,
			},
			validate: func(t *testing.T, p *PaddingMachineParams) {
				if p.BurstMin != 2 {
					t.Errorf("disabled padding should use defaults, got BurstMin=%d", p.BurstMin)
				}
			},
		},
		{
			name: "custom valid params",
			params: &ConsensusParams{
				APEBurstMin:    3,
				APEBurstMax:    15,
				APEGapMinMS:    2000,
				APEGapMaxMS:    10000,
				APECellDelayMS: 30,
			},
			validate: func(t *testing.T, p *PaddingMachineParams) {
				if p.BurstMin != 3 {
					t.Errorf("expected BurstMin=3, got %d", p.BurstMin)
				}
				if p.BurstMax != 15 {
					t.Errorf("expected BurstMax=15, got %d", p.BurstMax)
				}
				if p.GapMin != 2000*time.Millisecond {
					t.Errorf("expected GapMin=2s, got %v", p.GapMin)
				}
				if p.GapMax != 10000*time.Millisecond {
					t.Errorf("expected GapMax=10s, got %v", p.GapMax)
				}
				if p.CellDelay != 30*time.Millisecond {
					t.Errorf("expected CellDelay=30ms, got %v", p.CellDelay)
				}
			},
		},
		{
			name: "invalid params get sanitized",
			params: &ConsensusParams{
				APEBurstMin:    0,   // Invalid: too low
				APEBurstMax:    1,   // Invalid: less than min after correction
				APEGapMinMS:    50,  // Invalid: too low
				APEGapMaxMS:    100, // Invalid: too low after correction
				APECellDelayMS: 5,   // Invalid: too low
			},
			validate: func(t *testing.T, p *PaddingMachineParams) {
				if p.BurstMin < 1 {
					t.Errorf("BurstMin should be at least 1, got %d", p.BurstMin)
				}
				if p.BurstMax < p.BurstMin {
					t.Errorf("BurstMax (%d) should be >= BurstMin (%d)", p.BurstMax, p.BurstMin)
				}
				if p.GapMin < 100*time.Millisecond {
					t.Errorf("GapMin should be at least 100ms after correction, got %v", p.GapMin)
				}
				if p.GapMax < p.GapMin {
					t.Errorf("GapMax (%v) should be >= GapMin (%v)", p.GapMax, p.GapMin)
				}
				if p.CellDelay < 10*time.Millisecond {
					t.Errorf("CellDelay should be at least 10ms after correction, got %v", p.CellDelay)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := APEParamsFromConsensus(tt.params)
			tt.validate(t, params)
		})
	}
}

func TestSetupParamsFromConsensus(t *testing.T) {
	tests := []struct {
		name     string
		params   *ConsensusParams
		validate func(*testing.T, *PaddingMachineParams)
	}{
		{
			name:   "nil params returns defaults",
			params: nil,
			validate: func(t *testing.T, p *PaddingMachineParams) {
				if p.BurstMin != 1 {
					t.Errorf("expected BurstMin=1, got %d", p.BurstMin)
				}
				if p.BurstMax != 5 {
					t.Errorf("expected BurstMax=5, got %d", p.BurstMax)
				}
			},
		},
		{
			name: "custom valid params",
			params: &ConsensusParams{
				SetupBurstMin:    2,
				SetupBurstMax:    8,
				SetupGapMinMS:    600,
				SetupGapMaxMS:    2500,
				SetupCellDelayMS: 60,
			},
			validate: func(t *testing.T, p *PaddingMachineParams) {
				if p.BurstMin != 2 {
					t.Errorf("expected BurstMin=2, got %d", p.BurstMin)
				}
				if p.BurstMax != 8 {
					t.Errorf("expected BurstMax=8, got %d", p.BurstMax)
				}
				if p.GapMin != 600*time.Millisecond {
					t.Errorf("expected GapMin=600ms, got %v", p.GapMin)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := SetupParamsFromConsensus(tt.params)
			tt.validate(t, params)
		})
	}
}

func TestNewAPEMachineWithParams(t *testing.T) {
	circuit := &Circuit{ID: 1}

	params := &PaddingMachineParams{
		BurstMin:  3,
		BurstMax:  7,
		GapMin:    2 * time.Second,
		GapMax:    8 * time.Second,
		CellDelay: 25 * time.Millisecond,
	}

	machine := NewAPEMachineWithParams(circuit, params)

	if machine.machineType != PaddingMachineAPE {
		t.Errorf("expected APE machine type")
	}
	if machine.burstMin != 3 {
		t.Errorf("expected burstMin=3, got %d", machine.burstMin)
	}
	if machine.burstMax != 7 {
		t.Errorf("expected burstMax=7, got %d", machine.burstMax)
	}
	if machine.gapMin != 2*time.Second {
		t.Errorf("expected gapMin=2s, got %v", machine.gapMin)
	}
}

func TestNewCircuitSetupMachineWithParams(t *testing.T) {
	circuit := &Circuit{ID: 1}

	params := &PaddingMachineParams{
		BurstMin:  2,
		BurstMax:  6,
		GapMin:    700 * time.Millisecond,
		GapMax:    2200 * time.Millisecond,
		CellDelay: 55 * time.Millisecond,
	}

	machine := NewCircuitSetupMachineWithParams(circuit, params)

	if machine.machineType != PaddingMachineCircuitSetup {
		t.Errorf("expected CircuitSetup machine type")
	}
	if machine.burstMin != 2 {
		t.Errorf("expected burstMin=2, got %d", machine.burstMin)
	}
	if machine.cellDelay != 55*time.Millisecond {
		t.Errorf("expected cellDelay=55ms, got %v", machine.cellDelay)
	}
}

func TestDefaultAPEParams(t *testing.T) {
	params := DefaultAPEParams()

	// Verify spec-compliant defaults
	if params.BurstMin != 2 {
		t.Errorf("expected default BurstMin=2, got %d", params.BurstMin)
	}
	if params.BurstMax != 10 {
		t.Errorf("expected default BurstMax=10, got %d", params.BurstMax)
	}
	if params.GapMin != 1500*time.Millisecond {
		t.Errorf("expected default GapMin=1.5s, got %v", params.GapMin)
	}
	if params.GapMax != 9500*time.Millisecond {
		t.Errorf("expected default GapMax=9.5s, got %v", params.GapMax)
	}
	if params.CellDelay != 20*time.Millisecond {
		t.Errorf("expected default CellDelay=20ms, got %v", params.CellDelay)
	}
}

func TestDefaultCircuitSetupParams(t *testing.T) {
	params := DefaultCircuitSetupParams()

	// Verify spec-compliant defaults
	if params.BurstMin != 1 {
		t.Errorf("expected default BurstMin=1, got %d", params.BurstMin)
	}
	if params.BurstMax != 5 {
		t.Errorf("expected default BurstMax=5, got %d", params.BurstMax)
	}
	if params.GapMin != 500*time.Millisecond {
		t.Errorf("expected default GapMin=500ms, got %v", params.GapMin)
	}
	if params.GapMax != 2000*time.Millisecond {
		t.Errorf("expected default GapMax=2s, got %v", params.GapMax)
	}
	if params.CellDelay != 50*time.Millisecond {
		t.Errorf("expected default CellDelay=50ms, got %v", params.CellDelay)
	}
}

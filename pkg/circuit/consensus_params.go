// Package circuit provides consensus parameter integration for padding machines
package circuit

import "time"

// ConsensusParams holds padding parameters extracted from network consensus
// This mirrors directory.PaddingParams but avoids import cycles
type ConsensusParams struct {
	APEBurstMin      int
	APEBurstMax      int
	APEGapMinMS      int
	APEGapMaxMS      int
	APECellDelayMS   int
	SetupBurstMin    int
	SetupBurstMax    int
	SetupGapMinMS    int
	SetupGapMaxMS    int
	SetupCellDelayMS int
	PaddingDisabled  bool
}

// APEParamsFromConsensus creates APE machine parameters from consensus
// If consensus params are nil, returns default parameters
func APEParamsFromConsensus(cp *ConsensusParams) *PaddingMachineParams {
	if cp == nil || cp.PaddingDisabled {
		return DefaultAPEParams()
	}

	params := &PaddingMachineParams{
		BurstMin:  cp.APEBurstMin,
		BurstMax:  cp.APEBurstMax,
		GapMin:    time.Duration(cp.APEGapMinMS) * time.Millisecond,
		GapMax:    time.Duration(cp.APEGapMaxMS) * time.Millisecond,
		CellDelay: time.Duration(cp.APECellDelayMS) * time.Millisecond,
	}

	// Validate parameters to prevent misconfiguration
	if params.BurstMin < 1 {
		params.BurstMin = 2
	}
	if params.BurstMax < params.BurstMin {
		params.BurstMax = params.BurstMin + 8
	}
	if params.GapMin < 100*time.Millisecond {
		params.GapMin = 1500 * time.Millisecond
	}
	if params.GapMax < params.GapMin {
		params.GapMax = params.GapMin + 8000*time.Millisecond
	}
	if params.CellDelay < 10*time.Millisecond {
		params.CellDelay = 20 * time.Millisecond
	}

	return params
}

// SetupParamsFromConsensus creates circuit setup machine parameters from consensus
func SetupParamsFromConsensus(cp *ConsensusParams) *PaddingMachineParams {
	if cp == nil || cp.PaddingDisabled {
		return DefaultCircuitSetupParams()
	}

	params := &PaddingMachineParams{
		BurstMin:  cp.SetupBurstMin,
		BurstMax:  cp.SetupBurstMax,
		GapMin:    time.Duration(cp.SetupGapMinMS) * time.Millisecond,
		GapMax:    time.Duration(cp.SetupGapMaxMS) * time.Millisecond,
		CellDelay: time.Duration(cp.SetupCellDelayMS) * time.Millisecond,
	}

	// Validate parameters
	if params.BurstMin < 1 {
		params.BurstMin = 1
	}
	if params.BurstMax < params.BurstMin {
		params.BurstMax = params.BurstMin + 4
	}
	if params.GapMin < 100*time.Millisecond {
		params.GapMin = 500 * time.Millisecond
	}
	if params.GapMax < params.GapMin {
		params.GapMax = params.GapMin + 1500*time.Millisecond
	}
	if params.CellDelay < 10*time.Millisecond {
		params.CellDelay = 50 * time.Millisecond
	}

	return params
}

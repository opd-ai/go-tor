// Package directory tests consensus parameter parsing
package directory

import (
	"testing"
)

func TestParseConsensusParams(t *testing.T) {
	tests := []struct {
		name        string
		paramsStr   string
		expected    map[string]int
		description string
	}{
		{
			name:      "empty params",
			paramsStr: "",
			expected:  map[string]int{},
		},
		{
			name:      "single parameter",
			paramsStr: "circpad_global_allowed_cells=1000",
			expected: map[string]int{
				"circpad_global_allowed_cells": 1000,
			},
		},
		{
			name:      "multiple parameters",
			paramsStr: "nf_ito_low=1500 nf_ito_high=9500 circpad_padding_disabled=0",
			expected: map[string]int{
				"nf_ito_low":               1500,
				"nf_ito_high":              9500,
				"circpad_padding_disabled": 0,
			},
		},
		{
			name:      "padding parameters",
			paramsStr: "circpad_ape_burst_min=2 circpad_ape_burst_max=10 circpad_ape_cell_delay=20",
			expected: map[string]int{
				"circpad_ape_burst_min":  2,
				"circpad_ape_burst_max":  10,
				"circpad_ape_cell_delay": 20,
			},
		},
		{
			name:      "circuit setup parameters",
			paramsStr: "circpad_setup_burst_min=1 circpad_setup_burst_max=5 circpad_setup_gap_min=500",
			expected: map[string]int{
				"circpad_setup_burst_min": 1,
				"circpad_setup_burst_max": 5,
				"circpad_setup_gap_min":   500,
			},
		},
		{
			name:      "malformed parameters ignored",
			paramsStr: "valid=100 invalid malformed=abc another=200",
			expected: map[string]int{
				"valid":   100,
				"another": 200,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := make(map[string]int)
			parseConsensusParams(tt.paramsStr, params)

			if len(params) != len(tt.expected) {
				t.Errorf("expected %d params, got %d", len(tt.expected), len(params))
			}

			for key, expectedValue := range tt.expected {
				if value, ok := params[key]; !ok {
					t.Errorf("expected key %q not found", key)
				} else if value != expectedValue {
					t.Errorf("key %q: expected value %d, got %d", key, expectedValue, value)
				}
			}
		})
	}
}

func TestGetPaddingParams(t *testing.T) {
	tests := []struct {
		name     string
		metadata *ConsensusMetadata
		validate func(*testing.T, *PaddingParams)
	}{
		{
			name:     "nil metadata returns defaults",
			metadata: nil,
			validate: func(t *testing.T, p *PaddingParams) {
				if p.APEBurstMin != 2 {
					t.Errorf("expected default APEBurstMin=2, got %d", p.APEBurstMin)
				}
				if p.APEBurstMax != 10 {
					t.Errorf("expected default APEBurstMax=10, got %d", p.APEBurstMax)
				}
				if p.APEGapMinMS != 1500 {
					t.Errorf("expected default APEGapMinMS=1500, got %d", p.APEGapMinMS)
				}
			},
		},
		{
			name: "empty params returns defaults",
			metadata: &ConsensusMetadata{
				Params: make(map[string]int),
			},
			validate: func(t *testing.T, p *PaddingParams) {
				if p.SetupBurstMin != 1 {
					t.Errorf("expected default SetupBurstMin=1, got %d", p.SetupBurstMin)
				}
				if p.SetupBurstMax != 5 {
					t.Errorf("expected default SetupBurstMax=5, got %d", p.SetupBurstMax)
				}
			},
		},
		{
			name: "global padding disabled",
			metadata: &ConsensusMetadata{
				Params: map[string]int{
					"circpad_padding_disabled": 1,
				},
			},
			validate: func(t *testing.T, p *PaddingParams) {
				if !p.PaddingDisabled {
					t.Error("expected PaddingDisabled=true")
				}
			},
		},
		{
			name: "custom APE parameters",
			metadata: &ConsensusMetadata{
				Params: map[string]int{
					"circpad_ape_burst_min":  3,
					"circpad_ape_burst_max":  15,
					"circpad_ape_cell_delay": 30,
					"nf_ito_low":             2000,
					"nf_ito_high":            10000,
				},
			},
			validate: func(t *testing.T, p *PaddingParams) {
				if p.APEBurstMin != 3 {
					t.Errorf("expected APEBurstMin=3, got %d", p.APEBurstMin)
				}
				if p.APEBurstMax != 15 {
					t.Errorf("expected APEBurstMax=15, got %d", p.APEBurstMax)
				}
				if p.APECellDelayMS != 30 {
					t.Errorf("expected APECellDelayMS=30, got %d", p.APECellDelayMS)
				}
				if p.APEGapMinMS != 2000 {
					t.Errorf("expected APEGapMinMS=2000, got %d", p.APEGapMinMS)
				}
				if p.APEGapMaxMS != 10000 {
					t.Errorf("expected APEGapMaxMS=10000, got %d", p.APEGapMaxMS)
				}
			},
		},
		{
			name: "custom setup parameters",
			metadata: &ConsensusMetadata{
				Params: map[string]int{
					"circpad_setup_burst_min":      2,
					"circpad_setup_burst_max":      8,
					"circpad_setup_gap_min":        600,
					"circpad_setup_gap_max":        2500,
					"circpad_setup_cell_delay":     60,
					"circpad_global_allowed_cells": 5000,
				},
			},
			validate: func(t *testing.T, p *PaddingParams) {
				if p.SetupBurstMin != 2 {
					t.Errorf("expected SetupBurstMin=2, got %d", p.SetupBurstMin)
				}
				if p.SetupBurstMax != 8 {
					t.Errorf("expected SetupBurstMax=8, got %d", p.SetupBurstMax)
				}
				if p.SetupGapMinMS != 600 {
					t.Errorf("expected SetupGapMinMS=600, got %d", p.SetupGapMinMS)
				}
				if p.SetupGapMaxMS != 2500 {
					t.Errorf("expected SetupGapMaxMS=2500, got %d", p.SetupGapMaxMS)
				}
				if p.SetupCellDelayMS != 60 {
					t.Errorf("expected SetupCellDelayMS=60, got %d", p.SetupCellDelayMS)
				}
				if p.GlobalAllowedCells != 5000 {
					t.Errorf("expected GlobalAllowedCells=5000, got %d", p.GlobalAllowedCells)
				}
			},
		},
		{
			name: "zero values are ignored",
			metadata: &ConsensusMetadata{
				Params: map[string]int{
					"circpad_ape_burst_min": 0, // Should be ignored, use default
					"circpad_ape_burst_max": 15,
				},
			},
			validate: func(t *testing.T, p *PaddingParams) {
				if p.APEBurstMin != 2 {
					t.Errorf("zero value should be ignored, expected default APEBurstMin=2, got %d", p.APEBurstMin)
				}
				if p.APEBurstMax != 15 {
					t.Errorf("expected APEBurstMax=15, got %d", p.APEBurstMax)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := GetPaddingParams(tt.metadata)
			tt.validate(t, params)
		})
	}
}

func TestPaddingParamsDefaults(t *testing.T) {
	params := GetPaddingParams(nil)

	// Verify all defaults match spec
	expected := map[string]int{
		"GlobalAllowedCells": 0,
		"APEBurstMin":        2,
		"APEBurstMax":        10,
		"APEGapMinMS":        1500,
		"APEGapMaxMS":        9500,
		"APECellDelayMS":     20,
		"SetupBurstMin":      1,
		"SetupBurstMax":      5,
		"SetupGapMinMS":      500,
		"SetupGapMaxMS":      2000,
		"SetupCellDelayMS":   50,
	}

	if params.GlobalAllowedCells != expected["GlobalAllowedCells"] {
		t.Errorf("GlobalAllowedCells: expected %d, got %d", expected["GlobalAllowedCells"], params.GlobalAllowedCells)
	}
	if params.APEBurstMin != expected["APEBurstMin"] {
		t.Errorf("APEBurstMin: expected %d, got %d", expected["APEBurstMin"], params.APEBurstMin)
	}
	if params.APEBurstMax != expected["APEBurstMax"] {
		t.Errorf("APEBurstMax: expected %d, got %d", expected["APEBurstMax"], params.APEBurstMax)
	}
	if params.APEGapMinMS != expected["APEGapMinMS"] {
		t.Errorf("APEGapMinMS: expected %d, got %d", expected["APEGapMinMS"], params.APEGapMinMS)
	}
	if params.APEGapMaxMS != expected["APEGapMaxMS"] {
		t.Errorf("APEGapMaxMS: expected %d, got %d", expected["APEGapMaxMS"], params.APEGapMaxMS)
	}
	if params.APECellDelayMS != expected["APECellDelayMS"] {
		t.Errorf("APECellDelayMS: expected %d, got %d", expected["APECellDelayMS"], params.APECellDelayMS)
	}
	if params.SetupBurstMin != expected["SetupBurstMin"] {
		t.Errorf("SetupBurstMin: expected %d, got %d", expected["SetupBurstMin"], params.SetupBurstMin)
	}
	if params.SetupBurstMax != expected["SetupBurstMax"] {
		t.Errorf("SetupBurstMax: expected %d, got %d", expected["SetupBurstMax"], params.SetupBurstMax)
	}
	if params.SetupGapMinMS != expected["SetupGapMinMS"] {
		t.Errorf("SetupGapMinMS: expected %d, got %d", expected["SetupGapMinMS"], params.SetupGapMinMS)
	}
	if params.SetupGapMaxMS != expected["SetupGapMaxMS"] {
		t.Errorf("SetupGapMaxMS: expected %d, got %d", expected["SetupGapMaxMS"], params.SetupGapMaxMS)
	}
	if params.SetupCellDelayMS != expected["SetupCellDelayMS"] {
		t.Errorf("SetupCellDelayMS: expected %d, got %d", expected["SetupCellDelayMS"], params.SetupCellDelayMS)
	}

	if params.PaddingDisabled {
		t.Error("default PaddingDisabled should be false")
	}
}

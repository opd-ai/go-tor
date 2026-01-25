// Example: Using consensus parameters for padding configuration
package main

import (
	"fmt"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/directory"
)

func main() {
	fmt.Println("=== Consensus Padding Parameters Example ===")

	// Simulate consensus parameters
	metadata := &directory.ConsensusMetadata{
		Params: map[string]int{
			"circpad_ape_burst_min":  3,
			"circpad_ape_burst_max":  12,
			"nf_ito_low":             1800,
			"nf_ito_high":            9000,
			"circpad_ape_cell_delay": 25,
		},
	}

	// Extract padding parameters from consensus
	paddingParams := directory.GetPaddingParams(metadata)

	fmt.Println("Padding parameters from consensus:")
	fmt.Printf("  APE Burst: %d-%d cells\n", paddingParams.APEBurstMin, paddingParams.APEBurstMax)
	fmt.Printf("  APE Gap: %d-%d ms\n", paddingParams.APEGapMinMS, paddingParams.APEGapMaxMS)
	fmt.Printf("  APE Cell Delay: %d ms\n", paddingParams.APECellDelayMS)

	// Convert to circuit package format
	consensusParams := &circuit.ConsensusParams{
		APEBurstMin:    paddingParams.APEBurstMin,
		APEBurstMax:    paddingParams.APEBurstMax,
		APEGapMinMS:    paddingParams.APEGapMinMS,
		APEGapMaxMS:    paddingParams.APEGapMaxMS,
		APECellDelayMS: paddingParams.APECellDelayMS,
	}

	// Create machine parameters from consensus
	apeParams := circuit.APEParamsFromConsensus(consensusParams)

	fmt.Println("\nConfigured APE machine:")
	fmt.Printf("  Bursts: %d-%d cells\n", apeParams.BurstMin, apeParams.BurstMax)
	fmt.Printf("  Gaps: %v-%v\n", apeParams.GapMin, apeParams.GapMax)
	fmt.Printf("  Cell Delay: %v\n", apeParams.CellDelay)

	// Create a circuit and padding machine
	circ := &circuit.Circuit{ID: 1}
	apeMachine := circuit.NewAPEMachineWithParams(circ, apeParams)

	fmt.Println("\n✅ Successfully created padding machine from consensus parameters")
	fmt.Printf("  Machine ready: %v\n", apeMachine != nil)

	// Show comparison with defaults
	fmt.Println("\n--- Comparison with defaults ---")
	defaultParams := circuit.DefaultAPEParams()
	fmt.Printf("Default: bursts=%d-%d, gaps=%v-%v\n",
		defaultParams.BurstMin, defaultParams.BurstMax,
		defaultParams.GapMin, defaultParams.GapMax)
	fmt.Printf("Consensus: bursts=%d-%d, gaps=%v-%v\n",
		apeParams.BurstMin, apeParams.BurstMax,
		apeParams.GapMin, apeParams.GapMax)
}

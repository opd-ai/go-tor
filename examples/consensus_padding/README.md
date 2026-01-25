# Consensus Padding Parameters Example

This example demonstrates how to use consensus parameters to configure circuit padding machines.

## What This Example Shows

1. Parsing padding parameters from consensus metadata
2. Converting consensus parameters to padding machine configuration
3. Creating padding machines with consensus-derived settings
4. Comparing consensus parameters with spec defaults

## Running the Example

```bash
cd examples/consensus_padding
go run main.go
```

## Expected Output

```
=== Consensus Padding Parameters Example ===

Padding parameters from consensus:
  APE Burst: 3-12 cells
  APE Gap: 1800-9000 ms
  APE Cell Delay: 25 ms

Configured APE machine:
  Bursts: 3-12 cells
  Gaps: 1.8s-9s
  Cell Delay: 25ms

✅ Successfully created padding machine from consensus parameters
  Machine ready: true

--- Comparison with defaults ---
Default: bursts=2-10, gaps=1.5s-9.5s
Consensus: bursts=3-12, gaps=1.8s-9s
```

## Key Concepts

### Consensus Parameters

The Tor network consensus includes a `params` line with network-wide configuration:

```
params circpad_ape_burst_min=3 circpad_ape_burst_max=12 nf_ito_low=1800 ...
```

### Parameter Flow

1. **Fetch Consensus** → Get network consensus document
2. **Parse Params** → Extract `params` line values
3. **Get Padding Params** → Convert to `PaddingParams` structure  
4. **Create Machine Params** → Generate `PaddingMachineParams`
5. **Build Machine** → Create padding state machine

### Supported Parameters

- `circpad_ape_burst_min/max` - Burst size range
- `nf_ito_low/high` - Gap timing range  
- `circpad_ape_cell_delay` - Cell spacing
- `circpad_padding_disabled` - Global disable flag

## Integration

In a real client, you would:

```go
// Fetch consensus with metadata
relays, metadata, err := dirClient.parseConsensusWithMetadata(reader)

// Extract padding parameters
paddingParams := directory.GetPaddingParams(metadata)

// Check if padding is disabled network-wide
if paddingParams.PaddingDisabled {
    // Don't create padding machines
    return
}

// Create consensus-configured machines
apeParams := circuit.APEParamsFromConsensus(consensusParams)
machine := circuit.NewAPEMachineWithParams(circuit, apeParams)
machine.Start()
```

## See Also

- [docs/CONSENSUS_PADDING_PARAMS.md](../../docs/CONSENSUS_PADDING_PARAMS.md) - Full documentation
- [docs/CIRCUIT_PADDING.md](../../docs/CIRCUIT_PADDING.md) - Circuit padding overview
- [pkg/directory/params_test.go](../../pkg/directory/params_test.go) - Parameter parsing tests
- [pkg/circuit/consensus_params_test.go](../../pkg/circuit/consensus_params_test.go) - Integration tests

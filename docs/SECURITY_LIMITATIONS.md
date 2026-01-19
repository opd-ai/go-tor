# Security Limitations

This document describes known security limitations of the go-tor implementation. Understanding these limitations is essential for making informed decisions about using this software.

> **IMPORTANT**: This software should NOT be used for anonymity, safety, or privacy-critical applications. Use official [Tor Browser](https://www.torproject.org/download/) or [Arti](https://gitlab.torproject.org/tpo/core/arti) for real anonymity needs.

## Table of Contents

- [Guard Node Fingerprinting](#guard-node-fingerprinting)
- [Traffic Analysis](#traffic-analysis)
- [Circuit Isolation Limitations](#circuit-isolation-limitations)
- [Recommended Mitigations](#recommended-mitigations)

## Guard Node Fingerprinting

**Severity**: LOW  
**Status**: Known limitation (documented)  
**Related Audit Finding**: AUDIT LOW-004

### Description

Path selection patterns in go-tor may be distinguishable from standard Tor client behavior. An adversary capable of observing network traffic may be able to fingerprint users based on their guard relay selection patterns.

### Technical Details

1. **Guard Selection Algorithm**: The guard node selection algorithm, while following Tor specification guidelines, may exhibit timing or behavioral differences from the reference C Tor implementation.

2. **Guard Rotation Patterns**: The timing and frequency of guard node rotation may differ from the official Tor client, potentially creating a fingerprinting vector.

3. **Relay Preference Patterns**: Bandwidth-weighted relay selection may produce slightly different statistical distributions compared to the reference implementation.

### Attack Vectors

- **Passive Traffic Analysis**: An adversary monitoring network traffic could potentially distinguish go-tor users from official Tor Browser users based on guard selection patterns.
- **Long-term Correlation**: Over time, consistent guard usage patterns may enable user tracking.

### Mitigations

1. **Guard Persistence**: go-tor implements guard persistence (see `pkg/path/guards.go`) to reduce guard rotation frequency, which limits the fingerprinting surface.

2. **Standard Guard Selection**: Guard nodes are selected from the same pool as the official Tor client (relays with Guard, Stable, Running, Valid flags).

3. **Cryptographic Randomness**: All random selections use `crypto/rand` for cryptographically secure randomness.

### Recommendations

- Do not rely on go-tor for anonymity-critical applications
- If fingerprinting resistance is critical, use the official Tor Browser
- Consider using go-tor only in environments where this fingerprinting vector is acceptable

## Traffic Analysis

### Description

While go-tor implements circuit padding (see `pkg/circuit/padding.go`), it may not provide the same level of traffic analysis resistance as the official Tor implementation.

### Limitations

1. **Timing Patterns**: Cell transmission timing may differ from the reference implementation
2. **Padding Strategy**: The padding implementation uses configurable strategies but may not match official Tor behavior exactly
3. **Burst Patterns**: Network burst patterns during circuit building may be distinguishable

### Mitigations

- Enable circuit padding in configuration
- Use adaptive padding strategy for better traffic analysis resistance
- Monitor timing metrics for anomalies

## Circuit Isolation Limitations

### Description

Circuit isolation prevents activity correlation but has inherent limitations.

### Limitations

1. **Guard Node Correlation**: All circuits share the same guard node, which can enable correlation at the guard level
2. **Timing Attacks**: Circuit-level timing is still observable by network adversaries
3. **Volume Analysis**: Traffic volume patterns remain visible despite circuit isolation
4. **Exit Node Surveillance**: Exit nodes can observe plaintext traffic for non-encrypted connections

For detailed information about circuit isolation, see [CIRCUIT_ISOLATION.md](./CIRCUIT_ISOLATION.md).

## Recommended Mitigations

### For All Users

1. **Use HTTPS**: Always use HTTPS connections through Tor
2. **Limit Session Duration**: Shorter sessions reduce correlation opportunities
3. **Regular Updates**: Keep go-tor updated for the latest security fixes

### For High-Security Environments

1. **Use Official Software**: For anonymity-critical applications, use Tor Browser or Arti
2. **Network Isolation**: Deploy go-tor in isolated network environments
3. **Monitoring**: Implement comprehensive logging and monitoring for anomaly detection

### Configuration Recommendations

```go
cfg := config.DefaultConfig()

// Enable circuit padding for traffic analysis resistance
cfg.EnableCircuitPadding = true
cfg.PaddingStrategy = "adaptive"

// Enable stream isolation by destination
cfg.IsolationLevel = "destination"

// Reduce fingerprinting through smaller circuit pools
cfg.CircuitPoolMaxSize = 5
```

## References

- [AUDIT.md](../AUDIT.md) - Security audit findings
- [CIRCUIT_ISOLATION.md](./CIRCUIT_ISOLATION.md) - Circuit isolation documentation
- [Tor Project Security](https://www.torproject.org/about/security/) - Official Tor security information
- [Tor Specification](https://spec.torproject.org/) - Official Tor protocol specifications

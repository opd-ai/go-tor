# Stream Protocol Implementation Required

**STATUS:** CRITICAL - SOCKS5 Proxy Non-Functional

## Problem

The SOCKS5 proxy server currently cannot relay connections through Tor because the stream protocol layer is not fully implemented. While the infrastructure exists (stream manager, relay cells, circuits), they are not integrated into a working data path.

### Current State

The SOCKS5 server can:
- ✅ Accept SOCKS5 connections
- ✅ Perform authentication handshake
- ✅ Parse connection requests
- ✅ Validate onion addresses
- ✅ Support circuit isolation
- ❌ **Cannot relay actual data through Tor circuits**

### What Was Removed

A previous implementation made **direct TCP connections** to target hosts, completely bypassing Tor. This was a critical security vulnerability that:
1. Exposed the user's real IP address
2. Defeated the entire purpose of using Tor
3. Provided no anonymity whatsoever

This dangerous code has been removed.

## Required Implementation

To make the SOCKS5 proxy functional, the following must be implemented:

### 1. Circuit-to-Stream Integration

**File:** `pkg/circuit/circuit.go`

Add methods to the `Circuit` struct:
```go
// OpenStream opens a new stream on this circuit
func (c *Circuit) OpenStream(streamID uint16, target string, port uint16) error

// SendRelayCell sends a relay cell through the circuit
func (c *Circuit) SendRelayCell(cell *cell.RelayCell) error

// ReceiveRelayCell receives a relay cell from the circuit
func (c *Circuit) ReceiveRelayCell() (*cell.RelayCell, error)
```

### 2. RELAY_BEGIN Protocol

**File:** `pkg/socks/socks.go` (replace TODO in `handleConnection`)

```go
// 1. Get a circuit from the pool
circ, err := s.circuitPool.Get(ctx)
if err != nil {
    s.logger.Error("Failed to get circuit", "error", err)
    s.sendReply(conn, replyGeneralFailure, nil)
    return
}
defer s.circuitPool.Put(circ)

// 2. Create a stream
stream, err := s.streamMgr.CreateStream(circ.ID, host, uint16(port))
if err != nil {
    s.logger.Error("Failed to create stream", "error", err)
    s.sendReply(conn, replyGeneralFailure, nil)
    return
}
defer s.streamMgr.RemoveStream(stream.ID)

// 3. Send RELAY_BEGIN cell
// Format: "host:port\0" as payload
beginPayload := []byte(fmt.Sprintf("%s:%d\x00", host, port))
beginCell := cell.NewRelayCell(stream.ID, cell.RelayBegin, beginPayload)
if err := circ.SendRelayCell(beginCell); err != nil {
    s.logger.Error("Failed to send RELAY_BEGIN", "error", err)
    s.sendReply(conn, replyGeneralFailure, nil)
    return
}

// 4. Wait for RELAY_CONNECTED
connectedCell, err := circ.ReceiveRelayCell()
if err != nil {
    s.logger.Error("Failed to receive RELAY_CONNECTED", "error", err)
    s.sendReply(conn, replyHostUnreachable, nil)
    return
}

if connectedCell.Command != cell.RelayConnected {
    s.logger.Error("Expected RELAY_CONNECTED, got", "command", cell.RelayCmdString(connectedCell.Command))
    s.sendReply(conn, replyHostUnreachable, nil)
    return
}

stream.SetState(stream.StateConnected)

// 5. Send SOCKS5 success reply
s.sendReply(conn, replySuccess, conn.LocalAddr())

// 6. Relay data bidirectionally
s.relayDataThroughCircuit(ctx, conn, circ, stream)
```

### 3. Bidirectional Data Relay

**File:** `pkg/socks/socks.go` (new method)

```go
// relayDataThroughCircuit relays data between SOCKS client and Tor circuit
func (s *Server) relayDataThroughCircuit(ctx context.Context, socksConn net.Conn, circ *circuit.Circuit, stream *stream.Stream) {
    var wg sync.WaitGroup
    wg.Add(2)
    
    // SOCKS client -> Tor circuit (RELAY_DATA)
    go func() {
        defer wg.Done()
        buf := make([]byte, 498) // Max relay cell data size
        for {
            n, err := socksConn.Read(buf)
            if err != nil {
                if err != io.EOF {
                    s.logger.Debug("SOCKS read error", "error", err)
                }
                // Send RELAY_END
                endCell := cell.NewRelayCell(stream.ID, cell.RelayEnd, []byte{6}) // REASON_DONE
                circ.SendRelayCell(endCell)
                return
            }
            
            // Send as RELAY_DATA
            dataCell := cell.NewRelayCell(stream.ID, cell.RelayData, buf[:n])
            if err := circ.SendRelayCell(dataCell); err != nil {
                s.logger.Error("Failed to send RELAY_DATA", "error", err)
                return
            }
        }
    }()
    
    // Tor circuit -> SOCKS client (RELAY_DATA)
    go func() {
        defer wg.Done()
        for {
            relayCell, err := circ.ReceiveRelayCell()
            if err != nil {
                s.logger.Debug("Circuit receive error", "error", err)
                return
            }
            
            // Filter for our stream
            if relayCell.StreamID != stream.ID {
                continue
            }
            
            switch relayCell.Command {
            case cell.RelayData:
                // Write to SOCKS client
                if _, err := socksConn.Write(relayCell.Data); err != nil {
                    s.logger.Error("Failed to write to SOCKS client", "error", err)
                    return
                }
                
            case cell.RelayEnd:
                s.logger.Debug("Received RELAY_END from circuit")
                return
                
            default:
                s.logger.Warn("Unexpected relay command", "command", cell.RelayCmdString(relayCell.Command))
            }
        }
    }()
    
    wg.Wait()
}
```

### 4. Relay Cell Encryption/Decryption

**File:** `pkg/circuit/circuit.go`

Relay cells must be encrypted/decrypted at each hop using the circuit's cryptographic state:

```go
// EncryptRelayCell encrypts a relay cell for all hops in the circuit
func (c *Circuit) EncryptRelayCell(relayCell *cell.RelayCell) (*cell.Cell, error) {
    // Update forward digest
    payload, _ := relayCell.Encode()
    c.forwardDigest.Write(payload)
    digest := c.forwardDigest.Sum(nil)[:4]
    copy(relayCell.Digest[:], digest)
    
    // Encrypt with each hop's key (in order for forward direction)
    // This is a simplified version - actual implementation needs per-hop crypto
    
    return cell.NewCell(cell.CmdRelay, c.ID, payload), nil
}

// DecryptRelayCell decrypts a relay cell from the circuit
func (c *Circuit) DecryptRelayCell(cellData *cell.Cell) (*cell.RelayCell, error) {
    // Decrypt with each hop's key (in reverse order for backward direction)
    // Verify backward digest
    // This is a simplified version - actual implementation needs per-hop crypto
    
    return cell.DecodeRelayCell(cellData.Payload)
}
```

### 5. Connection Layer Integration

**File:** `pkg/circuit/circuit.go`

Circuits need to maintain references to the underlying connection to actually send cells:

```go
type Circuit struct {
    // ... existing fields ...
    conn *connection.Connection // Connection to the entry guard
}

func (c *Circuit) SendRelayCell(relayCell *cell.RelayCell) error {
    // Encrypt the relay cell
    encryptedCell, err := c.EncryptRelayCell(relayCell)
    if err != nil {
        return fmt.Errorf("failed to encrypt relay cell: %w", err)
    }
    
    // Send through connection
    return c.conn.SendCell(encryptedCell)
}
```

## Dependencies

The stream protocol implementation requires:

1. **Per-hop Cryptography**
   - AES-CTR for stream cipher
   - SHA-1 for running digests (per tor-spec.txt §6.1)
   - Key material derived during circuit extension

2. **Stream Multiplexing**
   - Multiple concurrent streams per circuit
   - Stream ID management (uint16, 1-65535)
   - Per-stream state tracking

3. **Error Handling**
   - RELAY_END reason codes
   - Circuit-level failures
   - Stream-level failures
   - Proper cleanup and resource management

4. **Flow Control**
   - Circuit-level flow control (SENDME cells)
   - Stream-level flow control
   - Buffering and backpressure

## References

- **Tor Specification:** [tor-spec.txt section 6](https://gitweb.torproject.org/torspec.git/tree/tor-spec.txt) - Relay cells and stream protocol
- **Tor Specification:** [tor-spec.txt section 6.2](https://gitweb.torproject.org/torspec.git/tree/tor-spec.txt) - Opening streams
- **Tor Specification:** [tor-spec.txt section 7.3](https://gitweb.torproject.org/torspec.git/tree/tor-spec.txt) - Relay cell encryption

## Testing Plan

Once implemented:

1. Start tor-client
2. Test with curl: `curl --socks5 127.0.0.1:9049 https://check.torproject.org`
3. Verify connection goes through Tor (not direct)
4. Check exit IP matches Tor exit node
5. Test multiple concurrent connections
6. Test circuit isolation features

## Priority

**CRITICAL**: This is the core functionality that makes go-tor useful as a Tor client. Without it, the SOCKS5 proxy cannot function, and users cannot route traffic through Tor.

## Complexity Estimate

- **Medium-High**: Requires careful integration of multiple components
- **Estimated effort**: 3-5 days for a working implementation
- **Testing effort**: 2-3 days for comprehensive testing

## Current Workarounds

**None.** The SOCKS5 proxy will return an error for all connection attempts until this is implemented.

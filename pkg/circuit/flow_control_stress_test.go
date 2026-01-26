// Package circuit provides Tor circuit management and multi-hop routing.
package circuit

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestFlowControlHighThroughput tests flow control under high data volume
// This addresses PLAN.md line 1085: "⏳ Test with high-throughput scenarios"
func TestFlowControlHighThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high-throughput test in short mode")
	}

	// Create circuit
	circuitID := uint32(1)
	circ := NewCircuit(circuitID)
	circ.State = StateOpen

	// Track statistics
	var (
		cellsSent   int
		sendmesSent int
	)

	// Send 2000 cells (2x the initial window)
	// This forces multiple SENDME exchanges
	const numCells = 2000

	// Send cells and track window behavior
	for i := 0; i < numCells; i++ {
		// Attempt to decrement window (simulates sending a cell)
		err := circ.decrementPackageWindow()
		if err != nil {
			// Window exhausted, simulate receiving SENDME
			circ.incrementPackageWindow()
			sendmesSent++

			// Retry the cell
			err = circ.decrementPackageWindow()
			if err != nil {
				t.Fatalf("Failed to send cell even after SENDME at iteration %d: %v", i, err)
			}
		}

		cellsSent++

		// Check if we should send SENDME (every 100 cells received)
		if circ.shouldSendCircuitSendme() {
			sendmesSent++
		}
	}

	// Verify high throughput was achieved
	if cellsSent != numCells {
		t.Errorf("Only sent %d/%d cells, expected all cells to be sent", cellsSent, numCells)
	}

	// Verify SENDME mechanism was exercised
	expectedSendmes := (numCells / 100) + 1 // SENDME every 100 cells
	if sendmesSent < expectedSendmes/2 {
		t.Errorf("Only %d SENDMEs needed, expected at least %d", sendmesSent, expectedSendmes/2)
	}

	// Verify window is not exhausted at the end
	if circ.packageWindow < 0 {
		t.Errorf("Final packageWindow = %d, should not be negative", circ.packageWindow)
	}

	t.Logf("High-throughput test: sent %d cells, %d SENDMEs, final window: %d",
		cellsSent, sendmesSent, circ.packageWindow)
}

// TestFlowControlConcurrentStreams tests flow control with multiple concurrent streams
func TestFlowControlConcurrentStreams(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent streams test in short mode")
	}

	// Create circuit
	circuitID := uint32(1)
	circ := NewCircuit(circuitID)
	circ.State = StateOpen

	const numStreams = 10
	const cellsPerStream = 200

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Track results
	var (
		totalSent int
		mu        sync.Mutex
		wg        sync.WaitGroup
	)

	// Simulate SENDME processing in background
	go func() {
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Replenish circuit window if low
				if circ.packageWindow < 500 {
					circ.incrementPackageWindow()
				}
			}
		}
	}()

	// Launch concurrent streams
	for streamID := uint16(1); streamID <= numStreams; streamID++ {
		wg.Add(1)
		go func(sid uint16) {
			defer wg.Done()

			sent := 0
			for i := 0; i < cellsPerStream; i++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Try to decrement circuit window (simulates sending)
				err := circ.decrementPackageWindow()
				if err == nil {
					sent++
				} else {
					// Window exhausted, wait briefly
					time.Sleep(1 * time.Millisecond)
					i-- // Retry this cell
				}
			}

			mu.Lock()
			totalSent += sent
			mu.Unlock()
		}(streamID)
	}

	wg.Wait()

	// Verify concurrent streams work
	expectedCells := numStreams * cellsPerStream
	successRate := float64(totalSent) / float64(expectedCells) * 100

	if successRate < 80 {
		t.Errorf("Only sent %d/%d cells (%.1f%%), flow control may not support concurrent streams well",
			totalSent, expectedCells, successRate)
	}

	t.Logf("Concurrent streams test: %d streams, %d total cells (%.1f%% success rate)",
		numStreams, totalSent, successRate)
}

// TestFlowControlWindowRecovery tests that windows properly recover after exhaustion
func TestFlowControlWindowRecovery(t *testing.T) {
	circ := NewCircuit(1)
	circ.State = StateOpen

	initialWindow := circ.packageWindow

	// Exhaust the window
	for i := 0; i < initialWindow; i++ {
		err := circ.decrementPackageWindow()
		if err != nil {
			break
		}
	}

	// Window should be exhausted
	if circ.packageWindow != 0 {
		t.Errorf("Window not fully exhausted: packageWindow = %d, want 0", circ.packageWindow)
	}

	// Next decrement should fail
	err := circ.decrementPackageWindow()
	if err == nil {
		t.Error("decrementPackageWindow should fail when window exhausted")
	}

	// Simulate receiving SENDME (increment by 100)
	circ.incrementPackageWindow()

	// Window should be partially recovered
	if circ.packageWindow != 100 {
		t.Errorf("After SENDME, packageWindow = %d, want 100", circ.packageWindow)
	}

	// Decrement should succeed again
	err = circ.decrementPackageWindow()
	if err != nil {
		t.Errorf("decrementPackageWindow should succeed after SENDME: %v", err)
	}

	// Verify final window
	if circ.packageWindow != 99 {
		t.Errorf("After decrement, packageWindow = %d, want 99", circ.packageWindow)
	}

	t.Logf("Window recovery: initial=%d, exhausted=0, recovered=100, final=%d",
		initialWindow, circ.packageWindow)
}

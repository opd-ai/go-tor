package path

import (
	"context"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/directory"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// mockDirectoryClient creates a mock directory client with test data
type mockDirectoryClient struct {
	relays []*directory.Relay
}

func (m *mockDirectoryClient) FetchConsensus(ctx context.Context) ([]*directory.Relay, error) {
	return m.relays, nil
}

func newMockDirectoryClient() *mockDirectoryClient {
	return &mockDirectoryClient{
		relays: []*directory.Relay{
			{
				Nickname:    "GuardRelay1",
				Fingerprint: "AAAA1111",
				Address:     "192.168.1.1",
				ORPort:      9001,
				Flags:       []string{"Running", "Valid", "Guard", "Stable", "Fast"},
			},
			{
				Nickname:    "GuardRelay2",
				Fingerprint: "AAAA2222",
				Address:     "192.168.1.2",
				ORPort:      9001,
				Flags:       []string{"Running", "Valid", "Guard", "Stable"},
			},
			{
				Nickname:    "MiddleRelay1",
				Fingerprint: "BBBB1111",
				Address:     "192.168.2.1",
				ORPort:      9001,
				Flags:       []string{"Running", "Valid", "Fast"},
			},
			{
				Nickname:    "MiddleRelay2",
				Fingerprint: "BBBB2222",
				Address:     "192.168.2.2",
				ORPort:      9001,
				Flags:       []string{"Running", "Valid"},
			},
			{
				Nickname:    "ExitRelay1",
				Fingerprint: "CCCC1111",
				Address:     "192.168.3.1",
				ORPort:      9001,
				Flags:       []string{"Running", "Valid", "Exit", "Fast"},
			},
			{
				Nickname:    "ExitRelay2",
				Fingerprint: "CCCC2222",
				Address:     "192.168.3.2",
				ORPort:      9001,
				Flags:       []string{"Running", "Valid", "Exit"},
			},
			{
				Nickname:    "InvalidRelay",
				Fingerprint: "DDDD1111",
				Address:     "192.168.4.1",
				ORPort:      9001,
				Flags:       []string{"Running"}, // Not Valid
			},
		},
	}
}

func TestNewSelector(t *testing.T) {
	log := logger.NewDefault()
	mockDir := newMockDirectoryClient()
	dirClient := directory.NewClient(log)

	selector := NewSelector(dirClient, log)

	if selector == nil {
		t.Fatal("NewSelector returned nil")
	}

	if selector.logger == nil {
		t.Error("Selector logger is nil")
	}

	// Test with nil logger
	selector2 := NewSelector(dirClient, nil)
	if selector2.logger == nil {
		t.Error("Selector should create default logger when nil is passed")
	}

	_ = mockDir // Suppress unused warning
}

func TestUpdateConsensus(t *testing.T) {
	log := logger.NewDefault()
	mockDir := newMockDirectoryClient()

	selector := NewSelector(directory.NewClient(log), log)

	// Manually populate for test (simulating UpdateConsensus)
	selector.guards = mockDir.relays[:2]
	selector.relays = mockDir.relays

	if len(selector.guards) != 2 {
		t.Errorf("Expected 2 guard relays, got %d", len(selector.guards))
	}

	if len(selector.relays) != 7 {
		t.Errorf("Expected 7 total relays, got %d", len(selector.relays))
	}
}

func TestSelectPath(t *testing.T) {
	log := logger.NewDefault()
	mockDir := newMockDirectoryClient()

	selector := NewSelector(directory.NewClient(log), log)

	// Manually populate for test
	selector.guards = []*directory.Relay{
		mockDir.relays[0], // GuardRelay1
		mockDir.relays[1], // GuardRelay2
	}
	selector.relays = []*directory.Relay{
		mockDir.relays[0], // GuardRelay1
		mockDir.relays[1], // GuardRelay2
		mockDir.relays[2], // MiddleRelay1
		mockDir.relays[3], // MiddleRelay2
		mockDir.relays[4], // ExitRelay1
		mockDir.relays[5], // ExitRelay2
	}

	path, err := selector.SelectPath(80)
	if err != nil {
		t.Fatalf("SelectPath failed: %v", err)
	}

	if path == nil {
		t.Fatal("SelectPath returned nil path")
	}

	if path.Guard == nil {
		t.Error("Path guard is nil")
	}

	if path.Middle == nil {
		t.Error("Path middle is nil")
	}

	if path.Exit == nil {
		t.Error("Path exit is nil")
	}

	// Verify path diversity
	if path.Guard.Fingerprint == path.Middle.Fingerprint {
		t.Error("Guard and middle relay are the same")
	}

	if path.Guard.Fingerprint == path.Exit.Fingerprint {
		t.Error("Guard and exit relay are the same")
	}

	if path.Middle.Fingerprint == path.Exit.Fingerprint {
		t.Error("Middle and exit relay are the same")
	}
}

func TestSelectPathNoRelays(t *testing.T) {
	log := logger.NewDefault()
	selector := NewSelector(directory.NewClient(log), log)

	// Don't populate relays
	_, err := selector.SelectPath(80)
	if err == nil {
		t.Error("Expected error when no relays available")
	}
}

func TestSelectGuard(t *testing.T) {
	log := logger.NewDefault()
	mockDir := newMockDirectoryClient()

	selector := NewSelector(directory.NewClient(log), log)
	selector.guards = []*directory.Relay{
		mockDir.relays[0], // GuardRelay1
		mockDir.relays[1], // GuardRelay2
	}

	guard, err := selector.selectGuard()
	if err != nil {
		t.Fatalf("selectGuard failed: %v", err)
	}

	if guard == nil {
		t.Fatal("selectGuard returned nil")
	}

	// Should be one of the guard relays
	isGuard := guard.Fingerprint == "AAAA1111" || guard.Fingerprint == "AAAA2222"
	if !isGuard {
		t.Error("Selected relay is not a guard relay")
	}
}

func TestSelectGuardNoGuards(t *testing.T) {
	log := logger.NewDefault()
	selector := NewSelector(directory.NewClient(log), log)

	_, err := selector.selectGuard()
	if err == nil {
		t.Error("Expected error when no guards available")
	}
}

func TestSelectExit(t *testing.T) {
	log := logger.NewDefault()
	mockDir := newMockDirectoryClient()

	selector := NewSelector(directory.NewClient(log), log)
	selector.relays = mockDir.relays[:6] // Exclude invalid relay

	guard := mockDir.relays[0] // GuardRelay1

	exit, err := selector.selectExit(80, guard)
	if err != nil {
		t.Fatalf("selectExit failed: %v", err)
	}

	if exit == nil {
		t.Fatal("selectExit returned nil")
	}

	// Should not be the guard
	if exit.Fingerprint == guard.Fingerprint {
		t.Error("Exit relay is the same as guard")
	}
}

func TestSelectMiddle(t *testing.T) {
	log := logger.NewDefault()
	mockDir := newMockDirectoryClient()

	selector := NewSelector(directory.NewClient(log), log)
	selector.relays = mockDir.relays[:6]

	guard := mockDir.relays[0] // GuardRelay1
	exit := mockDir.relays[4]  // ExitRelay1

	middle, err := selector.selectMiddle(guard, exit)
	if err != nil {
		t.Fatalf("selectMiddle failed: %v", err)
	}

	if middle == nil {
		t.Fatal("selectMiddle returned nil")
	}

	// Should not be guard or exit
	if middle.Fingerprint == guard.Fingerprint {
		t.Error("Middle relay is the same as guard")
	}

	if middle.Fingerprint == exit.Fingerprint {
		t.Error("Middle relay is the same as exit")
	}
}

func TestSelectMiddleNoCandidates(t *testing.T) {
	log := logger.NewDefault()
	mockDir := newMockDirectoryClient()

	selector := NewSelector(directory.NewClient(log), log)
	selector.relays = []*directory.Relay{
		mockDir.relays[0], // Only two relays
		mockDir.relays[4],
	}

	guard := mockDir.relays[0]
	exit := mockDir.relays[4]

	_, err := selector.selectMiddle(guard, exit)
	if err == nil {
		t.Error("Expected error when no middle candidates available")
	}
}

func TestRandomIndex(t *testing.T) {
	// Test basic functionality
	idx, err := randomIndex(10)
	if err != nil {
		t.Fatalf("randomIndex failed: %v", err)
	}

	if idx < 0 || idx >= 10 {
		t.Errorf("randomIndex out of range: got %d, want [0, 10)", idx)
	}

	// Test edge cases
	idx, err = randomIndex(1)
	if err != nil {
		t.Fatalf("randomIndex(1) failed: %v", err)
	}
	if idx != 0 {
		t.Errorf("randomIndex(1) = %d, want 0", idx)
	}

	// Test invalid input
	_, err = randomIndex(0)
	if err == nil {
		t.Error("Expected error for randomIndex(0)")
	}

	_, err = randomIndex(-1)
	if err == nil {
		t.Error("Expected error for randomIndex(-1)")
	}
}

func TestPathDiversity(t *testing.T) {
	log := logger.NewDefault()
	mockDir := newMockDirectoryClient()

	selector := NewSelector(directory.NewClient(log), log)
	selector.guards = []*directory.Relay{
		mockDir.relays[0],
		mockDir.relays[1],
	}
	selector.relays = mockDir.relays[:6]

	// Select multiple paths and verify diversity
	paths := make([]*Path, 5)
	for i := 0; i < 5; i++ {
		path, err := selector.SelectPath(80)
		if err != nil {
			t.Fatalf("SelectPath failed: %v", err)
		}
		paths[i] = path
	}

	// Verify each path has unique relays
	for _, path := range paths {
		fingerprints := map[string]bool{
			path.Guard.Fingerprint:  true,
			path.Middle.Fingerprint: true,
			path.Exit.Fingerprint:   true,
		}

		if len(fingerprints) != 3 {
			t.Error("Path does not have unique relays")
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	log := logger.NewDefault()
	mockDir := newMockDirectoryClient()

	selector := NewSelector(directory.NewClient(log), log)
	selector.guards = []*directory.Relay{
		mockDir.relays[0],
		mockDir.relays[1],
	}
	selector.relays = mockDir.relays[:6]

	// Test concurrent path selection
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := selector.SelectPath(80)
			if err != nil {
				t.Errorf("SelectPath failed: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines with timeout
	timeout := time.After(5 * time.Second)
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// Success
		case <-timeout:
			t.Fatal("Test timed out")
		}
	}
}

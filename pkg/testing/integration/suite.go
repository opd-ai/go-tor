// Package integration provides comprehensive integration testing infrastructure
// for the go-tor client. It includes mock implementations for testing full client
// lifecycle, circuit operations, and network interactions without requiring a
// real Tor network connection.
//
// Tests in this package are designed to run with the "integration" build tag:
//
//	go test -tags=integration ./pkg/testing/integration/...
package integration

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// Suite provides the test harness for integration tests. It manages mock
// servers, test fixtures, and lifecycle operations.
type Suite struct {
	mu               sync.RWMutex
	logger           *logger.Logger
	mockServers      []*MockServer
	circuits         map[uint32]*circuit.Circuit
	circuitIDCounter uint32
	running          bool
	startedAt        time.Time
}

// NewSuite creates a new integration test suite with default configuration.
func NewSuite() *Suite {
	return &Suite{
		logger:   logger.NewDefault(),
		circuits: make(map[uint32]*circuit.Circuit),
	}
}

// Start initializes the test suite and mock infrastructure.
func (s *Suite) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("suite already running")
	}

	s.running = true
	s.startedAt = time.Now()
	s.logger.Info("Integration test suite started")
	return nil
}

// Stop shuts down the test suite and cleans up resources.
func (s *Suite) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	// Stop all mock servers
	for _, server := range s.mockServers {
		if err := server.Stop(); err != nil {
			s.logger.Warn("Failed to stop mock server", "error", err)
		}
	}

	// Close all circuits
	for _, circ := range s.circuits {
		circ.Close()
	}
	s.circuits = make(map[uint32]*circuit.Circuit)

	s.running = false
	s.logger.Info("Integration test suite stopped", "duration", time.Since(s.startedAt))
	return nil
}

// CreateMockCircuit creates a simulated circuit for testing purposes.
// The circuit will have the specified number of hops with mock relay data.
func (s *Suite) CreateMockCircuit(ctx context.Context, numHops int) (*circuit.Circuit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil, fmt.Errorf("suite not running")
	}

	id := atomic.AddUint32(&s.circuitIDCounter, 1)
	circ := &circuit.Circuit{
		ID:        id,
		CreatedAt: time.Now(),
	}
	circ.SetState(circuit.StateBuilding)

	// Add mock hops
	for i := 0; i < numHops; i++ {
		hop := circuit.NewHop(
			generateMockFingerprint(),
			fmt.Sprintf("192.168.1.%d:9001", i+1),
			i == 0,         // first hop is guard
			i == numHops-1, // last hop is exit
		)
		circ.AddHop(hop)
	}

	// Simulate build delay
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(10 * time.Millisecond):
	}

	circ.SetState(circuit.StateOpen)
	s.circuits[id] = circ
	return circ, nil
}

// GetCircuit returns a circuit by ID if it exists.
func (s *Suite) GetCircuit(id uint32) (*circuit.Circuit, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	circ, ok := s.circuits[id]
	return circ, ok
}

// CircuitCount returns the number of active circuits.
func (s *Suite) CircuitCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.circuits)
}

// MockServer represents a mock Tor relay for testing.
type MockServer struct {
	address     string
	listener    net.Listener
	connections []net.Conn
	mu          sync.Mutex
	running     bool
	closeCh     chan struct{}
	closeOnce   sync.Once
}

// NewMockServer creates a new mock server on a random available port.
func NewMockServer() (*MockServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	server := &MockServer{
		address:  listener.Addr().String(),
		listener: listener,
		closeCh:  make(chan struct{}),
		running:  true,
	}

	go server.acceptLoop()
	return server, nil
}

// Address returns the server's listen address.
func (s *MockServer) Address() string {
	return s.address
}

// ConnectionCount returns the number of active connections.
func (s *MockServer) ConnectionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.connections)
}

// Stop shuts down the mock server.
func (s *MockServer) Stop() error {
	var err error
	s.closeOnce.Do(func() {
		close(s.closeCh)
		s.running = false
		err = s.listener.Close()

		s.mu.Lock()
		for _, conn := range s.connections {
			conn.Close()
		}
		s.connections = nil
		s.mu.Unlock()
	})
	return err
}

func (s *MockServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.closeCh:
				return
			default:
				continue
			}
		}

		s.mu.Lock()
		s.connections = append(s.connections, conn)
		s.mu.Unlock()
	}
}

// TestResult represents the result of a single test case.
type TestResult struct {
	Name      string
	Passed    bool
	Duration  time.Duration
	Error     error
	StartedAt time.Time
}

// TestResults holds results for multiple tests.
type TestResults struct {
	mu      sync.Mutex
	results []TestResult
}

// NewTestResults creates a new TestResults container.
func NewTestResults() *TestResults {
	return &TestResults{}
}

// Add adds a test result.
func (r *TestResults) Add(result TestResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results = append(r.results, result)
}

// PassCount returns the number of passed tests.
func (r *TestResults) PassCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, result := range r.results {
		if result.Passed {
			count++
		}
	}
	return count
}

// FailCount returns the number of failed tests.
func (r *TestResults) FailCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, result := range r.results {
		if !result.Passed {
			count++
		}
	}
	return count
}

// TotalCount returns the total number of tests.
func (r *TestResults) TotalCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.results)
}

// AllPassed returns true if all tests passed.
func (r *TestResults) AllPassed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, result := range r.results {
		if !result.Passed {
			return false
		}
	}
	return true
}

// generateMockFingerprint creates a random fingerprint for testing.
func generateMockFingerprint() string {
	bytes := make([]byte, 20)
	if _, err := rand.Read(bytes); err != nil {
		return "0000000000000000000000000000000000000000"
	}
	return fmt.Sprintf("%x", bytes)
}

// Package path provides path selection algorithms for Tor circuits.
// This package implements guard, middle, and exit node selection.
package path

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"

	"github.com/opd-ai/go-tor/pkg/directory"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// Path represents a selected path through the Tor network
type Path struct {
	Guard  *directory.Relay
	Middle *directory.Relay
	Exit   *directory.Relay
}

// Selector provides path selection for Tor circuits
type Selector struct {
	logger       *logger.Logger
	dirClient    *directory.Client
	guardManager *GuardManager
	mu           sync.RWMutex
	guards       []*directory.Relay
	relays       []*directory.Relay
}

// NewSelector creates a new path selector
func NewSelector(dirClient *directory.Client, log *logger.Logger) *Selector {
	if log == nil {
		log = logger.NewDefault()
	}

	return &Selector{
		logger:    log.Component("path"),
		dirClient: dirClient,
		guards:    make([]*directory.Relay, 0),
		relays:    make([]*directory.Relay, 0),
	}
}

// NewSelectorWithGuards creates a new path selector with guard persistence
func NewSelectorWithGuards(dirClient *directory.Client, guardMgr *GuardManager, log *logger.Logger) *Selector {
	if log == nil {
		log = logger.NewDefault()
	}

	return &Selector{
		logger:       log.Component("path"),
		dirClient:    dirClient,
		guardManager: guardMgr,
		guards:       make([]*directory.Relay, 0),
		relays:       make([]*directory.Relay, 0),
	}
}

// UpdateConsensus fetches and updates the network consensus
func (s *Selector) UpdateConsensus(ctx context.Context) error {
	s.logger.Info("Updating network consensus")

	relays, err := s.dirClient.FetchConsensus(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch consensus: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Filter relays for guards (must be Guard, Running, Valid, Stable)
	guards := make([]*directory.Relay, 0)
	allRelays := make([]*directory.Relay, 0)

	for _, relay := range relays {
		if !relay.IsRunning() || !relay.IsValid() {
			continue // Skip non-running or invalid relays
		}

		allRelays = append(allRelays, relay)

		if relay.IsGuard() && relay.IsStable() {
			guards = append(guards, relay)
		}
	}

	s.guards = guards
	s.relays = allRelays

	s.logger.Info("Consensus updated",
		"total_relays", len(allRelays),
		"guard_relays", len(guards))

	return nil
}

// GetRelays returns all relays from the current consensus (for event publishing)
func (s *Selector) GetRelays() []*directory.Relay {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	relays := make([]*directory.Relay, len(s.relays))
	copy(relays, s.relays)
	return relays
}

// SelectPath selects a complete path (guard, middle, exit) for a circuit
func (s *Selector) SelectPath(exitPort int) (*Path, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.guards) == 0 || len(s.relays) == 0 {
		return nil, fmt.Errorf("no relays available, call UpdateConsensus first")
	}

	// Select guard
	guard, err := s.selectGuard()
	if err != nil {
		return nil, fmt.Errorf("failed to select guard: %w", err)
	}

	// Select exit (must allow the port and not be the guard)
	exit, err := s.selectExit(exitPort, guard)
	if err != nil {
		return nil, fmt.Errorf("failed to select exit: %w", err)
	}

	// Select middle (must not be guard or exit)
	middle, err := s.selectMiddle(guard, exit)
	if err != nil {
		return nil, fmt.Errorf("failed to select middle: %w", err)
	}

	s.logger.Info("Path selected",
		"guard", guard.Nickname,
		"middle", middle.Nickname,
		"exit", exit.Nickname)

	return &Path{
		Guard:  guard,
		Middle: middle,
		Exit:   exit,
	}, nil
}

// selectGuard selects a guard relay, preferring persistent guards
func (s *Selector) selectGuard() (*directory.Relay, error) {
	if len(s.guards) == 0 {
		return nil, fmt.Errorf("no guard relays available")
	}

	// If we have a guard manager, try to use persistent guards first
	if s.guardManager != nil {
		persistentGuards := s.guardManager.GetGuards()

		// Try to find a persistent guard that's still in the current consensus
		for _, pGuard := range persistentGuards {
			for _, relay := range s.guards {
				if relay.Fingerprint == pGuard.Fingerprint {
					s.logger.Debug("Using persistent guard", "nickname", relay.Nickname)
					return relay, nil
				}
			}
		}

		// If no persistent guards are available, select a new one and persist it
		s.logger.Debug("No persistent guards available, selecting new guard")
	}

	// Select a random guard from available guards
	idx, err := randomIndex(len(s.guards))
	if err != nil {
		return nil, err
	}

	guard := s.guards[idx]

	// Add to persistent guards if we have a guard manager
	if s.guardManager != nil {
		if err := s.guardManager.AddGuard(guard); err != nil {
			s.logger.Warn("Failed to persist guard", "error", err)
		} else if err := s.guardManager.Save(); err != nil {
			s.logger.Warn("Failed to save guard state", "error", err)
		}
	}

	return guard, nil
}

// ConfirmGuard marks a guard as confirmed after successful use
func (s *Selector) ConfirmGuard(fingerprint string) {
	if s.guardManager != nil {
		if err := s.guardManager.ConfirmGuard(fingerprint); err != nil {
			s.logger.Warn("Failed to confirm guard", "error", err)
			return
		}
		if err := s.guardManager.Save(); err != nil {
			s.logger.Warn("Failed to save guard state after confirmation", "error", err)
		}
	}
}

// selectExit selects an exit relay that allows the specified port
func (s *Selector) selectExit(port int, avoid *directory.Relay) (*directory.Relay, error) {
	// For now, select any exit that's not the guard
	// In production, this would check exit policies for the port
	exits := make([]*directory.Relay, 0)

	for _, relay := range s.relays {
		if relay.IsExit() && relay.Fingerprint != avoid.Fingerprint {
			exits = append(exits, relay)
		}
	}

	if len(exits) == 0 {
		// Fallback: any relay that's not the guard
		for _, relay := range s.relays {
			if relay.Fingerprint != avoid.Fingerprint {
				exits = append(exits, relay)
			}
		}
	}

	if len(exits) == 0 {
		return nil, fmt.Errorf("no suitable exit relays available")
	}

	idx, err := randomIndex(len(exits))
	if err != nil {
		return nil, err
	}

	return exits[idx], nil
}

// selectMiddle selects a middle relay that is neither guard nor exit
func (s *Selector) selectMiddle(guard, exit *directory.Relay) (*directory.Relay, error) {
	candidates := make([]*directory.Relay, 0)

	for _, relay := range s.relays {
		if relay.Fingerprint != guard.Fingerprint && relay.Fingerprint != exit.Fingerprint {
			candidates = append(candidates, relay)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no suitable middle relays available")
	}

	idx, err := randomIndex(len(candidates))
	if err != nil {
		return nil, err
	}

	return candidates[idx], nil
}

// randomIndex returns a cryptographically random index in [0, max)
func randomIndex(max int) (int, error) {
	if max <= 0 {
		return 0, fmt.Errorf("max must be positive")
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, fmt.Errorf("failed to generate random number: %w", err)
	}

	return int(n.Int64()), nil
}

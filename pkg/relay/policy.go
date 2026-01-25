// Package relay - Exit policy enforcement for relay servers
// This file implements reject-all exit policy per tor-spec.txt §6.2
package relay

import (
	"fmt"
	"sync/atomic"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// ExitPolicy defines the exit policy for a relay
type ExitPolicy struct {
	// AllowExit determines if the relay allows exit traffic
	// For non-exit relays, this is always false
	AllowExit bool

	// RejectedConnections tracks the number of rejected exit attempts
	rejectedConnections uint64

	logger *logger.Logger
}

// NewExitPolicy creates a new exit policy
// For bridge and non-exit relays, exit traffic is always rejected
func NewExitPolicy(log *logger.Logger) *ExitPolicy {
	if log == nil {
		log = logger.NewDefault()
	}
	return &ExitPolicy{
		AllowExit: false, // Always reject exit traffic
		logger:    log.Component("exit-policy"),
	}
}

// CheckExitAllowed checks if exit traffic is allowed
// Per tor-spec.txt §6.2, non-exit relays must reject all exit attempts
func (p *ExitPolicy) CheckExitAllowed(address string, port uint16) (bool, byte) {
	if p.AllowExit {
		// This should never be true for bridge/non-exit relays
		p.logger.Warn("Exit allowed flag is true - this should not happen for non-exit relays")
		return true, 0
	}

	// Always reject exit traffic for non-exit relays
	atomic.AddUint64(&p.rejectedConnections, 1)
	
	p.logger.Info("Rejected exit attempt",
		"address", address,
		"port", port,
		"total_rejected", atomic.LoadUint64(&p.rejectedConnections))

	// Return EXITPOLICY reason per tor-spec.txt §6.3
	return false, cell.EndReasonExitPolicy
}

// GetRejectedCount returns the number of rejected exit attempts
func (p *ExitPolicy) GetRejectedCount() uint64 {
	return atomic.LoadUint64(&p.rejectedConnections)
}

// String returns a human-readable representation of the exit policy
func (p *ExitPolicy) String() string {
	if p.AllowExit {
		return "accept *:*"
	}
	return "reject *:*"
}

// GetExitPolicyString returns the exit policy in torrc format
// For non-exit relays, this is always "reject *:*"
func (p *ExitPolicy) GetExitPolicyString() string {
	return "reject *:*"
}

// ValidateExitAttempt validates and rejects an exit attempt
// Returns an error if the exit attempt should be rejected
func (p *ExitPolicy) ValidateExitAttempt(command byte, address string, port uint16) error {
	// Only check for BEGIN and BEGIN_DIR commands
	if command != cell.RelayBegin && command != cell.RelayBeginDir {
		return nil
	}

	// Check if exit is allowed
	allowed, reason := p.CheckExitAllowed(address, port)
	if !allowed {
		return &ExitPolicyViolation{
			Address: address,
			Port:    port,
			Reason:  reason,
		}
	}

	return nil
}

// ExitPolicyViolation represents an exit policy violation error
type ExitPolicyViolation struct {
	Address string
	Port    uint16
	Reason  byte
}

// Error implements the error interface
func (e *ExitPolicyViolation) Error() string {
	return fmt.Sprintf("exit policy violation: %s:%d (reason: %s)",
		e.Address, e.Port, endReasonString(e.Reason))
}

// GetReason returns the END_REASON code for the violation
func (e *ExitPolicyViolation) GetReason() byte {
	return e.Reason
}

// endReasonString converts an END_REASON code to a string
func endReasonString(reason byte) string {
	switch reason {
	case cell.EndReasonMisc:
		return "MISC"
	case cell.EndReasonResolveFailed:
		return "RESOLVEFAILED"
	case cell.EndReasonConnRefused:
		return "CONNECTREFUSED"
	case cell.EndReasonExitPolicy:
		return "EXITPOLICY"
	case cell.EndReasonDestroy:
		return "DESTROY"
	case cell.EndReasonDone:
		return "DONE"
	case cell.EndReasonTimeout:
		return "TIMEOUT"
	case cell.EndReasonNoRoute:
		return "NOROUTE"
	case cell.EndReasonHibernating:
		return "HIBERNATING"
	case cell.EndReasonInternal:
		return "INTERNAL"
	case cell.EndReasonResourceLimit:
		return "RESOURCELIMIT"
	case cell.EndReasonConnReset:
		return "CONNRESET"
	case cell.EndReasonProtocol:
		return "TORPROTOCOL"
	case cell.EndReasonNotDirectory:
		return "NOTDIRECTORY"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", reason)
	}
}

// IsExitPolicyError checks if an error is an exit policy violation
func IsExitPolicyError(err error) bool {
	_, ok := err.(*ExitPolicyViolation)
	return ok
}

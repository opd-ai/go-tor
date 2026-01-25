// Package path provides path bias detection per path-spec.txt §5.3
// Path bias detection tracks circuit build and usage success rates to detect
// attacks where malicious guards or middle nodes manipulate circuit construction.
package path

import (
	"fmt"
	"sync"
	"time"
)

// BiasThresholds defines the thresholds for path bias detection per path-spec.txt §5.3
type BiasThresholds struct {
	// UseSuccessMin is minimum circuits that must succeed before we check use rate
	UseSuccessMin int
	// UseSuccessRate is minimum fraction of circuits that must complete successfully
	UseSuccessRate float64
	// BuildTimeoutCount is max consecutive timeouts before marking path as biased
	BuildTimeoutCount int
	// SuccessCount is number of circuits to track for success rate calculation
	SuccessCount int
}

// DefaultThresholds returns conservative thresholds matching Tor's defaults
func DefaultThresholds() BiasThresholds {
	return BiasThresholds{
		UseSuccessMin:     20,  // Need 20 circuits before checking
		UseSuccessRate:    0.7, // 70% must succeed
		BuildTimeoutCount: 3,   // 3 consecutive timeouts triggers alert
		SuccessCount:      20,  // Track last 20 circuits
	}
}

// CircuitOutcome represents the result of a circuit attempt
type CircuitOutcome int

const (
	// OutcomeBuildSuccess indicates circuit was built successfully
	OutcomeBuildSuccess CircuitOutcome = iota
	// OutcomeBuildTimeout indicates circuit build timed out
	OutcomeBuildTimeout
	// OutcomeBuildFailed indicates circuit build failed (other than timeout)
	OutcomeBuildFailed
	// OutcomeUseSuccess indicates circuit was used successfully
	OutcomeUseSuccess
	// OutcomeUseFailed indicates circuit failed during use
	OutcomeUseFailed
)

// String returns string representation of outcome
func (o CircuitOutcome) String() string {
	switch o {
	case OutcomeBuildSuccess:
		return "BUILD_SUCCESS"
	case OutcomeBuildTimeout:
		return "BUILD_TIMEOUT"
	case OutcomeBuildFailed:
		return "BUILD_FAILED"
	case OutcomeUseSuccess:
		return "USE_SUCCESS"
	case OutcomeUseFailed:
		return "USE_FAILED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", o)
	}
}

// CircuitRecord tracks a single circuit's outcome
type CircuitRecord struct {
	Timestamp   time.Time
	Outcome     CircuitOutcome
	Fingerprint string // Guard fingerprint
	CircuitID   uint32
}

// BiasDetector tracks circuit outcomes to detect path bias attacks
type BiasDetector struct {
	mu         sync.RWMutex
	thresholds BiasThresholds
	records    []CircuitRecord
	
	// Per-guard statistics
	guardStats map[string]*BiasGuardStats
	
	// Alert tracking
	alerts     []BiasAlert
	maxAlerts  int
}

// BiasGuardStats tracks statistics for a specific guard
type BiasGuardStats struct {
	Fingerprint       string
	TotalBuilds       int
	BuildSuccesses    int
	BuildTimeouts     int
	BuildFailures     int
	TotalUses         int
	UseSuccesses      int
	UseFailures       int
	ConsecutiveTimeouts int
	LastSeen          time.Time
}

// BiasAlert represents a detected path bias issue
type BiasAlert struct {
	Timestamp   time.Time
	Type        string
	Fingerprint string
	Message     string
	Stats       BiasGuardStats
}

// NewBiasDetector creates a new path bias detector
func NewBiasDetector(thresholds BiasThresholds) *BiasDetector {
	return &BiasDetector{
		thresholds: thresholds,
		records:    make([]CircuitRecord, 0, thresholds.SuccessCount),
		guardStats: make(map[string]*BiasGuardStats),
		alerts:     make([]BiasAlert, 0),
		maxAlerts:  100, // Keep last 100 alerts
	}
}

// RecordOutcome records a circuit outcome and checks for bias
func (bd *BiasDetector) RecordOutcome(circuitID uint32, fingerprint string, outcome CircuitOutcome) []BiasAlert {
	bd.mu.Lock()
	defer bd.mu.Unlock()
	
	// Create record
	record := CircuitRecord{
		Timestamp:   time.Now(),
		Outcome:     outcome,
		Fingerprint: fingerprint,
		CircuitID:   circuitID,
	}
	
	// Add to records (ring buffer behavior)
	if len(bd.records) >= bd.thresholds.SuccessCount {
		bd.records = bd.records[1:]
	}
	bd.records = append(bd.records, record)
	
	// Update guard stats
	stats := bd.getOrCreateGuardStats(fingerprint)
	bd.updateGuardStats(stats, outcome)
	
	// Check for bias and return any new alerts
	return bd.checkBias(stats)
}

// getOrCreateGuardStats gets or creates guard statistics
func (bd *BiasDetector) getOrCreateGuardStats(fingerprint string) *BiasGuardStats {
	stats, exists := bd.guardStats[fingerprint]
	if !exists {
		stats = &BiasGuardStats{
			Fingerprint: fingerprint,
			LastSeen:    time.Now(),
		}
		bd.guardStats[fingerprint] = stats
	}
	stats.LastSeen = time.Now()
	return stats
}

// updateGuardStats updates guard statistics based on outcome
func (bd *BiasDetector) updateGuardStats(stats *BiasGuardStats, outcome CircuitOutcome) {
	switch outcome {
	case OutcomeBuildSuccess:
		stats.TotalBuilds++
		stats.BuildSuccesses++
		stats.ConsecutiveTimeouts = 0
		
	case OutcomeBuildTimeout:
		stats.TotalBuilds++
		stats.BuildTimeouts++
		stats.ConsecutiveTimeouts++
		
	case OutcomeBuildFailed:
		stats.TotalBuilds++
		stats.BuildFailures++
		stats.ConsecutiveTimeouts = 0
		
	case OutcomeUseSuccess:
		stats.TotalUses++
		stats.UseSuccesses++
		
	case OutcomeUseFailed:
		stats.TotalUses++
		stats.UseFailures++
	}
}

// checkBias checks if the guard statistics indicate bias
func (bd *BiasDetector) checkBias(stats *BiasGuardStats) []BiasAlert {
	var newAlerts []BiasAlert
	
	// Check for consecutive timeout bias
	if stats.ConsecutiveTimeouts >= bd.thresholds.BuildTimeoutCount {
		alert := BiasAlert{
			Timestamp:   time.Now(),
			Type:        "CONSECUTIVE_TIMEOUTS",
			Fingerprint: stats.Fingerprint,
			Message: fmt.Sprintf("Guard has %d consecutive build timeouts (threshold: %d)",
				stats.ConsecutiveTimeouts, bd.thresholds.BuildTimeoutCount),
			Stats: *stats,
		}
		newAlerts = append(newAlerts, alert)
		bd.addAlert(alert)
	}
	
	// Check use success rate (only if we have minimum sample size)
	if stats.TotalUses >= bd.thresholds.UseSuccessMin {
		useRate := float64(stats.UseSuccesses) / float64(stats.TotalUses)
		if useRate < bd.thresholds.UseSuccessRate {
			alert := BiasAlert{
				Timestamp:   time.Now(),
				Type:        "LOW_USE_SUCCESS",
				Fingerprint: stats.Fingerprint,
				Message: fmt.Sprintf("Guard use success rate %.2f%% below threshold %.2f%% (%d/%d)",
					useRate*100, bd.thresholds.UseSuccessRate*100,
					stats.UseSuccesses, stats.TotalUses),
				Stats: *stats,
			}
			newAlerts = append(newAlerts, alert)
			bd.addAlert(alert)
		}
	}
	
	// Check build success rate (minimum sample)
	if stats.TotalBuilds >= bd.thresholds.UseSuccessMin {
		buildRate := float64(stats.BuildSuccesses) / float64(stats.TotalBuilds)
		if buildRate < bd.thresholds.UseSuccessRate {
			alert := BiasAlert{
				Timestamp:   time.Now(),
				Type:        "LOW_BUILD_SUCCESS",
				Fingerprint: stats.Fingerprint,
				Message: fmt.Sprintf("Guard build success rate %.2f%% below threshold %.2f%% (%d/%d)",
					buildRate*100, bd.thresholds.UseSuccessRate*100,
					stats.BuildSuccesses, stats.TotalBuilds),
				Stats: *stats,
			}
			newAlerts = append(newAlerts, alert)
			bd.addAlert(alert)
		}
	}
	
	return newAlerts
}

// addAlert adds an alert to the alert history (ring buffer)
func (bd *BiasDetector) addAlert(alert BiasAlert) {
	if len(bd.alerts) >= bd.maxAlerts {
		bd.alerts = bd.alerts[1:]
	}
	bd.alerts = append(bd.alerts, alert)
}

// GetGuardStats returns statistics for a specific guard
func (bd *BiasDetector) GetGuardStats(fingerprint string) (*BiasGuardStats, error) {
	bd.mu.RLock()
	defer bd.mu.RUnlock()
	
	stats, exists := bd.guardStats[fingerprint]
	if !exists {
		return nil, fmt.Errorf("no statistics for guard %s", fingerprint)
	}
	
	// Return a copy
	statsCopy := *stats
	return &statsCopy, nil
}

// GetAllGuardStats returns statistics for all tracked guards
func (bd *BiasDetector) GetAllGuardStats() map[string]BiasGuardStats {
	bd.mu.RLock()
	defer bd.mu.RUnlock()
	
	result := make(map[string]BiasGuardStats)
	for fp, stats := range bd.guardStats {
		result[fp] = *stats
	}
	return result
}

// GetAlerts returns recent bias alerts
func (bd *BiasDetector) GetAlerts(limit int) []BiasAlert {
	bd.mu.RLock()
	defer bd.mu.RUnlock()
	
	if limit <= 0 || limit > len(bd.alerts) {
		limit = len(bd.alerts)
	}
	
	// Return most recent alerts
	start := len(bd.alerts) - limit
	result := make([]BiasAlert, limit)
	copy(result, bd.alerts[start:])
	return result
}

// ClearGuard removes all statistics for a specific guard
// Used when a guard is no longer in use or has been replaced
func (bd *BiasDetector) ClearGuard(fingerprint string) {
	bd.mu.Lock()
	defer bd.mu.Unlock()
	
	delete(bd.guardStats, fingerprint)
}

// Reset clears all statistics and alerts
func (bd *BiasDetector) Reset() {
	bd.mu.Lock()
	defer bd.mu.Unlock()
	
	bd.records = make([]CircuitRecord, 0, bd.thresholds.SuccessCount)
	bd.guardStats = make(map[string]*BiasGuardStats)
	bd.alerts = make([]BiasAlert, 0)
}

// GetStats returns overall statistics
func (bd *BiasDetector) GetStats() BiasStats {
	bd.mu.RLock()
	defer bd.mu.RUnlock()
	
	var totalBuilds, buildSuccesses, buildTimeouts, buildFailures int
	var totalUses, useSuccesses, useFailures int
	
	for _, stats := range bd.guardStats {
		totalBuilds += stats.TotalBuilds
		buildSuccesses += stats.BuildSuccesses
		buildTimeouts += stats.BuildTimeouts
		buildFailures += stats.BuildFailures
		totalUses += stats.TotalUses
		useSuccesses += stats.UseSuccesses
		useFailures += stats.UseFailures
	}
	
	return BiasStats{
		TotalGuards:     len(bd.guardStats),
		TotalBuilds:     totalBuilds,
		BuildSuccesses:  buildSuccesses,
		BuildTimeouts:   buildTimeouts,
		BuildFailures:   buildFailures,
		TotalUses:       totalUses,
		UseSuccesses:    useSuccesses,
		UseFailures:     useFailures,
		TotalAlerts:     len(bd.alerts),
		RecordCount:     len(bd.records),
	}
}

// BiasStats contains overall bias detection statistics
type BiasStats struct {
	TotalGuards    int
	TotalBuilds    int
	BuildSuccesses int
	BuildTimeouts  int
	BuildFailures  int
	TotalUses      int
	UseSuccesses   int
	UseFailures    int
	TotalAlerts    int
	RecordCount    int
}

// IsBiased returns true if a guard appears to be biased based on current statistics
func (bd *BiasDetector) IsBiased(fingerprint string) bool {
	bd.mu.RLock()
	defer bd.mu.RUnlock()
	
	stats, exists := bd.guardStats[fingerprint]
	if !exists {
		return false
	}
	
	// Check consecutive timeouts
	if stats.ConsecutiveTimeouts >= bd.thresholds.BuildTimeoutCount {
		return true
	}
	
	// Check use success rate
	if stats.TotalUses >= bd.thresholds.UseSuccessMin {
		useRate := float64(stats.UseSuccesses) / float64(stats.TotalUses)
		if useRate < bd.thresholds.UseSuccessRate {
			return true
		}
	}
	
	// Check build success rate
	if stats.TotalBuilds >= bd.thresholds.UseSuccessMin {
		buildRate := float64(stats.BuildSuccesses) / float64(stats.TotalBuilds)
		if buildRate < bd.thresholds.UseSuccessRate {
			return true
		}
	}
	
	return false
}

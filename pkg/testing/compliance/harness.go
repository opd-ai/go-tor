// Package compliance provides a test harness for verifying specification
// compliance of the go-tor implementation against official Tor protocol
// specifications.
//
// # Design
//
// Each specification requirement is modelled as a [Requirement] identified by
// a spec name and section reference (e.g., "tor-spec §0.2").  Tests register
// [CheckFunc] functions against requirements; the [Harness] collects results
// and can report on coverage and failures.
//
// # Usage
//
//	func TestMyFeatureSpecCompliance(t *testing.T) {
//	    h := compliance.New(t)
//
//	    h.Check("tor-spec §0.2", "fixed-size cells are 514 bytes",
//	        func(t *testing.T) {
//	            // ... spec assertion ...
//	        })
//
//	    h.Check("tor-spec §0.2", "variable-length cell length field is big-endian uint16",
//	        func(t *testing.T) {
//	            // ...
//	        })
//
//	    h.Report() // prints summary to t.Log
//	}
package compliance

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// Requirement identifies a single protocol specification requirement.
type Requirement struct {
	// Spec is the specification document and section (e.g., "tor-spec §0.2").
	Spec string
	// Description is a short human-readable statement of the requirement.
	Description string
}

// String returns a human-readable representation of the requirement.
func (r Requirement) String() string {
	return fmt.Sprintf("[%s] %s", r.Spec, r.Description)
}

// Status represents the outcome of a compliance check.
type Status int

const (
	// StatusPass means the check passed.
	StatusPass Status = iota
	// StatusFail means the check failed.
	StatusFail
	// StatusSkip means the check was skipped (e.g., not applicable in env).
	StatusSkip
)

// String returns the status label.
func (s Status) String() string {
	switch s {
	case StatusPass:
		return "PASS"
	case StatusFail:
		return "FAIL"
	case StatusSkip:
		return "SKIP"
	default:
		return "UNKNOWN"
	}
}

// result holds the outcome for a single requirement check.
type result struct {
	Requirement Requirement
	Status      Status
}

// CheckFunc is a compliance check function that runs sub-tests against t.
type CheckFunc func(t *testing.T)

// Harness coordinates specification compliance checks for a single test function.
type Harness struct {
	t       *testing.T
	results []result
}

// New creates a new Harness associated with the given testing.T.
func New(t *testing.T) *Harness {
	t.Helper()
	return &Harness{t: t}
}

// Check runs fn as a sub-test named after req and records the outcome.
// If fn calls t.Skip, the requirement is recorded as [StatusSkip] rather than
// [StatusPass] or [StatusFail].
//
//	h.Check("tor-spec §0.2", "fixed-size cells are 514 bytes", func(t *testing.T) {
//	    // assertion
//	})
func (h *Harness) Check(spec, description string, fn CheckFunc) {
	h.t.Helper()
	req := Requirement{Spec: spec, Description: description}
	name := fmt.Sprintf("%s/%s", sanitise(spec), sanitise(description))

	var skipped bool
	passed := h.t.Run(name, func(t *testing.T) {
		t.Helper()
		// Use defer to capture t.Skipped() even when fn calls t.Skip(),
		// which triggers runtime.Goexit() and prevents code after fn(t) from running.
		defer func() { skipped = t.Skipped() }()
		fn(t)
	})

	status := StatusPass
	switch {
	case skipped:
		status = StatusSkip
	case !passed:
		status = StatusFail
	}
	h.results = append(h.results, result{Requirement: req, Status: status})
}

// Skip records a requirement as skipped without running any test function.
// Use this when a requirement cannot be verified in the current environment
// (e.g., requires live network, unavailable binary, etc.).
func (h *Harness) Skip(spec, description, reason string) {
	h.t.Helper()
	req := Requirement{Spec: spec, Description: description}
	h.t.Logf("SKIP [%s] %s: %s", spec, description, reason)
	h.results = append(h.results, result{Requirement: req, Status: StatusSkip})
}

// Report logs a summary of all registered checks grouped by spec section.
// Call this at the end of a compliance test function to surface aggregate
// pass/fail/skip counts.
func (h *Harness) Report() {
	h.t.Helper()

	bySpec := make(map[string][]result)
	for _, r := range h.results {
		bySpec[r.Requirement.Spec] = append(bySpec[r.Requirement.Spec], r)
	}

	specs := make([]string, 0, len(bySpec))
	for s := range bySpec {
		specs = append(specs, s)
	}
	sort.Strings(specs)

	var sb strings.Builder
	totalPass, totalFail, totalSkip := 0, 0, 0

	for _, spec := range specs {
		results := bySpec[spec]
		pass, fail, skip := 0, 0, 0
		for _, r := range results {
			switch r.Status {
			case StatusPass:
				pass++
				totalPass++
			case StatusFail:
				fail++
				totalFail++
			case StatusSkip:
				skip++
				totalSkip++
			}
		}
		fmt.Fprintf(&sb, "\n  %-20s  pass=%d fail=%d skip=%d", spec, pass, fail, skip)
	}

	h.t.Logf("Compliance report — total: pass=%d fail=%d skip=%d%s",
		totalPass, totalFail, totalSkip, sb.String())
}

// Summary returns counts of passing, failing, and skipped checks.
func (h *Harness) Summary() (pass, fail, skip int) {
	for _, r := range h.results {
		switch r.Status {
		case StatusPass:
			pass++
		case StatusFail:
			fail++
		case StatusSkip:
			skip++
		}
	}
	return pass, fail, skip
}

// sanitise replaces characters that are invalid in t.Run sub-test names.
func sanitise(s string) string {
	replacer := strings.NewReplacer(
		" ", "_",
		"/", "_",
		"§", "S",
		".", "_",
	)
	return replacer.Replace(s)
}

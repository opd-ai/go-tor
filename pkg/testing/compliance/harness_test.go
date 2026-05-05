package compliance_test

import (
	"testing"

	"github.com/opd-ai/go-tor/pkg/testing/compliance"
)

func TestHarness_PassingCheck(t *testing.T) {
	h := compliance.New(t)
	h.Check("tor-spec §0.2", "fixed-size cells are 514 bytes", func(t *testing.T) {
		// Example assertion: 4 (circID) + 1 (cmd) + 509 (payload) = 514
		const want = 514
		const got = 4 + 1 + 509
		if got != want {
			t.Errorf("cell size: got %d, want %d", got, want)
		}
	})
	pass, fail, skip := h.Summary()
	if pass != 1 || fail != 0 || skip != 0 {
		t.Errorf("Summary: pass=%d fail=%d skip=%d; want 1/0/0", pass, fail, skip)
	}
	h.Report()
}

func TestHarness_SummaryTracking(t *testing.T) {
	// Verify that Summary correctly counts results after mixed outcomes.
	h := compliance.New(t)

	h.Check("tor-spec §0.2", "spec check that passes", func(t *testing.T) {
		// deliberate no-op: passes
	})
	h.Check("tor-spec §0.2", "another passing check", func(t *testing.T) {
		// passes
	})
	h.Skip("tor-spec §5", "skipped requirement", "not applicable in unit test")

	pass, fail, skip := h.Summary()
	if pass != 2 || fail != 0 || skip != 1 {
		t.Errorf("Summary: pass=%d fail=%d skip=%d; want 2/0/1", pass, fail, skip)
	}
}

func TestHarness_SkipCheck(t *testing.T) {
	h := compliance.New(t)
	h.Skip("tor-spec §2", "TLS version negotiation", "requires live Tor relay")
	pass, fail, skip := h.Summary()
	if skip != 1 || pass != 0 || fail != 0 {
		t.Errorf("Summary: pass=%d fail=%d skip=%d; want 0/0/1", pass, fail, skip)
	}
}

func TestHarness_MultipleChecks(t *testing.T) {
	h := compliance.New(t)

	h.Check("tor-spec §0.2", "fixed-size cell format", func(t *testing.T) {})
	h.Check("tor-spec §0.2", "variable-length cell length is big-endian uint16", func(t *testing.T) {})
	h.Check("tor-spec §4", "CREATE2 HTYPE field is 2 bytes", func(t *testing.T) {})
	h.Check("rend-spec-v3 §2", "v3 onion address is 56 base32 chars", func(t *testing.T) {})
	h.Skip("tor-spec §2", "link-level auth", "requires TLS handshake setup")

	pass, fail, skip := h.Summary()
	if pass != 4 || fail != 0 || skip != 1 {
		t.Errorf("Summary: pass=%d fail=%d skip=%d; want 4/0/1", pass, fail, skip)
	}
	h.Report()
}

func TestHarness_Report_EmptyHarness(t *testing.T) {
	h := compliance.New(t)
	// Should not panic with empty results.
	h.Report()
}

func TestRequirement_String(t *testing.T) {
	req := compliance.Requirement{
		Spec:        "tor-spec §0.2",
		Description: "fixed-size cells",
	}
	got := req.String()
	if got == "" {
		t.Error("Requirement.String should not be empty")
	}
}

func TestStatus_String(t *testing.T) {
	tests := []struct {
		status compliance.Status
		want   string
	}{
		{compliance.StatusPass, "PASS"},
		{compliance.StatusFail, "FAIL"},
		{compliance.StatusSkip, "SKIP"},
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("Status %d String: got %q, want %q", tt.status, got, tt.want)
		}
	}
}

// TestHarness_SkipInCheckFunc verifies that calling t.Skip() inside a CheckFunc
// records the result as StatusSkip, not StatusPass or StatusFail.
func TestHarness_SkipInCheckFunc(t *testing.T) {
	h := compliance.New(t)
	h.Check("tor-spec §2", "TLS cert requirement", func(t *testing.T) {
		t.Skip("skipping: requires live TLS connection")
	})
	pass, fail, skip := h.Summary()
	if pass != 0 || fail != 0 || skip != 1 {
		t.Errorf("Summary: pass=%d fail=%d skip=%d; want 0/0/1", pass, fail, skip)
	}
}

// TestExampleHarness shows how to write spec compliance tests using the harness.
func TestExampleHarness(t *testing.T) {
	h := compliance.New(t)

	// tor-spec.txt §0.2 – Cell format
	h.Check("tor-spec §0.2", "fixed-size cells are 514 bytes", func(t *testing.T) {
		const fixedCellSize = 514
		if fixedCellSize != 514 {
			t.Errorf("cell size %d != 514", fixedCellSize)
		}
	})

	// rend-spec-v3 §6 – v3 onion address format
	h.Check("rend-spec-v3 §6", "v3 onion address is 56 base32 characters", func(t *testing.T) {
		// .onion suffix (6 chars) + 56 base32 chars = valid v3 address
		const addrLen = 56
		if addrLen != 56 {
			t.Errorf("expected 56 chars, got %d", addrLen)
		}
	})

	// dir-spec.txt §3 – consensus freshness
	h.Skip("dir-spec §3", "consensus freshness window", "requires live directory fetch")

	h.Report()

	pass, fail, _ := h.Summary()
	if fail > 0 {
		t.Errorf("%d compliance checks failed out of %d", fail, pass+fail)
	}
}

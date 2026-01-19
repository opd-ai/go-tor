package path

import (
	"fmt"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/directory"
	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestNewDiversityAnalyzer(t *testing.T) {
	// Test with nil logger
	da := NewDiversityAnalyzer(nil)
	if da == nil {
		t.Fatal("NewDiversityAnalyzer returned nil")
	}
	if da.logger == nil {
		t.Error("DiversityAnalyzer logger is nil")
	}
	if da.relayCache == nil {
		t.Error("DiversityAnalyzer relayCache is nil")
	}

	// Test with provided logger
	log := logger.NewDefault()
	da2 := NewDiversityAnalyzer(log)
	if da2 == nil {
		t.Fatal("NewDiversityAnalyzer with logger returned nil")
	}
}

func TestDiversityLevelString(t *testing.T) {
	tests := []struct {
		level DiversityLevel
		want  string
	}{
		{DiversityUnknown, "unknown"},
		{DiversityLow, "low"},
		{DiversityMedium, "medium"},
		{DiversityHigh, "high"},
		{DiversityExcellent, "excellent"},
		{DiversityLevel(99), "unknown"}, // Invalid value
	}

	for _, tt := range tests {
		got := tt.level.String()
		if got != tt.want {
			t.Errorf("DiversityLevel(%d).String() = %s, want %s", tt.level, got, tt.want)
		}
	}
}

func TestExtractRelayInfo(t *testing.T) {
	da := NewDiversityAnalyzer(nil)

	// Test nil relay
	info := da.ExtractRelayInfo(nil)
	if info != nil {
		t.Error("Expected nil for nil relay")
	}

	// Test valid relay
	relay := &directory.Relay{
		Fingerprint: "AAAA1111",
		Nickname:    "TestRelay",
		Address:     "192.168.1.1:9001",
	}

	info = da.ExtractRelayInfo(relay)
	if info == nil {
		t.Fatal("Expected non-nil RelayInfo")
	}

	if info.Fingerprint != relay.Fingerprint {
		t.Errorf("Fingerprint = %s, want %s", info.Fingerprint, relay.Fingerprint)
	}
	if info.Nickname != relay.Nickname {
		t.Errorf("Nickname = %s, want %s", info.Nickname, relay.Nickname)
	}
	if info.Address != relay.Address {
		t.Errorf("Address = %s, want %s", info.Address, relay.Address)
	}

	// Check caching
	da.mu.RLock()
	cachedInfo := da.relayCache[relay.Fingerprint]
	da.mu.RUnlock()
	if cachedInfo == nil {
		t.Error("Relay info was not cached")
	}
}

func TestAnalyzePathNil(t *testing.T) {
	da := NewDiversityAnalyzer(nil)

	// Test nil path
	score := da.AnalyzePath(nil)
	if score.Level != DiversityUnknown {
		t.Errorf("Expected DiversityUnknown for nil path, got %v", score.Level)
	}
	if score.Overall != 0 {
		t.Errorf("Expected 0 overall score for nil path, got %f", score.Overall)
	}

	// Test incomplete path - missing guard
	path := &Path{
		Guard:  nil,
		Middle: &directory.Relay{Fingerprint: "M1", Address: "10.0.0.1"},
		Exit:   &directory.Relay{Fingerprint: "E1", Address: "10.0.0.2"},
	}
	score = da.AnalyzePath(path)
	if score.Level != DiversityUnknown {
		t.Errorf("Expected DiversityUnknown for incomplete path, got %v", score.Level)
	}
}

func TestAnalyzePathDifferentSubnets(t *testing.T) {
	da := NewDiversityAnalyzer(nil)

	// All relays in different /16 subnets - should have high diversity
	path := &Path{
		Guard: &directory.Relay{
			Fingerprint: "G1",
			Nickname:    "Guard1",
			Address:     "192.168.1.1:9001",
		},
		Middle: &directory.Relay{
			Fingerprint: "M1",
			Nickname:    "Middle1",
			Address:     "10.0.1.1:9001",
		},
		Exit: &directory.Relay{
			Fingerprint: "E1",
			Nickname:    "Exit1",
			Address:     "172.16.1.1:9001",
		},
	}

	score := da.AnalyzePath(path)

	if score.Overall < 0.7 {
		t.Errorf("Expected high overall score for diverse subnets, got %f", score.Overall)
	}
	if score.ASScore < 0.8 {
		t.Errorf("Expected high AS score for different subnets, got %f", score.ASScore)
	}
	if score.Level < DiversityHigh {
		t.Errorf("Expected at least DiversityHigh, got %v", score.Level)
	}
}

func TestAnalyzePathSameSubnet(t *testing.T) {
	da := NewDiversityAnalyzer(nil)

	// All relays in the same /16 subnet - should have low diversity
	path := &Path{
		Guard: &directory.Relay{
			Fingerprint: "G1",
			Nickname:    "Guard1",
			Address:     "192.168.1.1:9001",
		},
		Middle: &directory.Relay{
			Fingerprint: "M1",
			Nickname:    "Middle1",
			Address:     "192.168.2.1:9001",
		},
		Exit: &directory.Relay{
			Fingerprint: "E1",
			Nickname:    "Exit1",
			Address:     "192.168.3.1:9001",
		},
	}

	score := da.AnalyzePath(path)

	if score.ASScore > 0.1 {
		t.Errorf("Expected low AS score for same subnet, got %f", score.ASScore)
	}
	if score.Level > DiversityMedium {
		t.Errorf("Expected at most DiversityMedium for same subnet, got %v", score.Level)
	}
}

func TestAnalyzePathMixedSubnets(t *testing.T) {
	da := NewDiversityAnalyzer(nil)

	// Two relays share subnet, one is different
	path := &Path{
		Guard: &directory.Relay{
			Fingerprint: "G1",
			Nickname:    "Guard1",
			Address:     "192.168.1.1:9001",
		},
		Middle: &directory.Relay{
			Fingerprint: "M1",
			Nickname:    "Middle1",
			Address:     "192.168.2.1:9001", // Same /16 as guard
		},
		Exit: &directory.Relay{
			Fingerprint: "E1",
			Nickname:    "Exit1",
			Address:     "10.0.1.1:9001", // Different /16
		},
	}

	score := da.AnalyzePath(path)

	// Should be medium - 2 unique subnets out of 3 relays
	if score.ASScore < 0.4 || score.ASScore > 0.6 {
		t.Errorf("Expected medium AS score (0.4-0.6) for mixed subnets, got %f", score.ASScore)
	}
}

func TestFamilyScore(t *testing.T) {
	da := NewDiversityAnalyzer(nil)

	// No families - should have perfect score
	guard := &RelayInfo{Fingerprint: "G1", Family: ""}
	middle := &RelayInfo{Fingerprint: "M1", Family: ""}
	exit := &RelayInfo{Fingerprint: "E1", Family: ""}

	score := da.calculateFamilyScore(guard, middle, exit)
	if score != 1.0 {
		t.Errorf("Expected family score 1.0 for no families, got %f", score)
	}

	// Shared family - should have zero score
	guard.Family = "MyFamily"
	middle.Family = "MyFamily"
	score = da.calculateFamilyScore(guard, middle, exit)
	if score != 0.0 {
		t.Errorf("Expected family score 0.0 for shared family, got %f", score)
	}

	// All different families - should have perfect score
	guard.Family = "FamilyA"
	middle.Family = "FamilyB"
	exit.Family = "FamilyC"
	score = da.calculateFamilyScore(guard, middle, exit)
	if score != 1.0 {
		t.Errorf("Expected family score 1.0 for unique families, got %f", score)
	}
}

func TestGetStats(t *testing.T) {
	da := NewDiversityAnalyzer(nil)

	// Initial stats
	stats := da.GetStats()
	if stats.PathsAnalyzed != 0 {
		t.Errorf("Expected 0 paths analyzed initially, got %d", stats.PathsAnalyzed)
	}

	// Analyze some paths
	path := &Path{
		Guard: &directory.Relay{
			Fingerprint: "G1",
			Address:     "192.168.1.1:9001",
		},
		Middle: &directory.Relay{
			Fingerprint: "M1",
			Address:     "10.0.1.1:9001",
		},
		Exit: &directory.Relay{
			Fingerprint: "E1",
			Address:     "172.16.1.1:9001",
		},
	}

	da.AnalyzePath(path)
	da.AnalyzePath(path)
	da.AnalyzePath(path)

	stats = da.GetStats()
	if stats.PathsAnalyzed != 3 {
		t.Errorf("Expected 3 paths analyzed, got %d", stats.PathsAnalyzed)
	}
	if stats.AverageScore <= 0 {
		t.Error("Expected non-zero average score after analysis")
	}
	if stats.LastAnalysisTime.IsZero() {
		t.Error("Expected non-zero last analysis time")
	}
}

func TestCheckPathDiversity(t *testing.T) {
	da := NewDiversityAnalyzer(nil)

	// High diversity path - should pass
	goodPath := &Path{
		Guard: &directory.Relay{
			Fingerprint: "G1",
			Address:     "192.168.1.1:9001",
		},
		Middle: &directory.Relay{
			Fingerprint: "M1",
			Address:     "10.0.1.1:9001",
		},
		Exit: &directory.Relay{
			Fingerprint: "E1",
			Address:     "172.16.1.1:9001",
		},
	}

	if !da.CheckPathDiversity(goodPath) {
		t.Error("Expected high diversity path to pass check")
	}
}

func TestSuggestAlternative(t *testing.T) {
	da := NewDiversityAnalyzer(nil)

	// Nil score - should suggest alternative
	if !da.SuggestAlternative(nil) {
		t.Error("Expected to suggest alternative for nil score")
	}

	// Low diversity score - should suggest alternative
	lowScore := &PathScore{Level: DiversityLow}
	if !da.SuggestAlternative(lowScore) {
		t.Error("Expected to suggest alternative for low diversity")
	}

	// Medium diversity - should not suggest alternative
	mediumScore := &PathScore{Level: DiversityMedium}
	if da.SuggestAlternative(mediumScore) {
		t.Error("Should not suggest alternative for medium diversity")
	}

	// High diversity - should not suggest alternative
	highScore := &PathScore{Level: DiversityHigh}
	if da.SuggestAlternative(highScore) {
		t.Error("Should not suggest alternative for high diversity")
	}
}

func TestScoreToLevel(t *testing.T) {
	tests := []struct {
		score float64
		want  DiversityLevel
	}{
		{1.0, DiversityExcellent},
		{0.95, DiversityExcellent},
		{0.9, DiversityExcellent},
		{0.85, DiversityHigh},
		{0.7, DiversityHigh},
		{0.65, DiversityMedium},
		{0.4, DiversityMedium},
		{0.3, DiversityLow},
		{0.0, DiversityLow},
		{-0.1, DiversityUnknown},
	}

	for _, tt := range tests {
		got := scoreToLevel(tt.score)
		if got != tt.want {
			t.Errorf("scoreToLevel(%f) = %v, want %v", tt.score, got, tt.want)
		}
	}
}

func TestExtractSubnet(t *testing.T) {
	tests := []struct {
		address string
		wantLen int // We just check if we get a result, not exact value
	}{
		{"192.168.1.1:9001", 2}, // IPv4 with port
		{"192.168.1.1", 2},      // IPv4 without port
		{"10.0.0.1", 2},         // IPv4
		{"invalid", 0},          // Invalid
		{"", 0},                 // Empty
		{"[::1]:9001", 4},       // IPv6 loopback (might not parse cleanly)
	}

	for _, tt := range tests {
		subnet := extractSubnet(tt.address)
		if tt.wantLen > 0 && subnet == "" {
			t.Errorf("extractSubnet(%s) returned empty, expected value", tt.address)
		}
		if tt.wantLen == 0 && subnet != "" {
			t.Errorf("extractSubnet(%s) = %s, expected empty", tt.address, subnet)
		}
	}
}

func TestExtractFirstOctet(t *testing.T) {
	tests := []struct {
		address   string
		wantEmpty bool
	}{
		{"192.168.1.1:9001", false},
		{"10.0.0.1", false},
		{"invalid", true},
		{"", true},
	}

	for _, tt := range tests {
		octet := extractFirstOctet(tt.address)
		if tt.wantEmpty && octet != "" {
			t.Errorf("extractFirstOctet(%s) = %s, expected empty", tt.address, octet)
		}
		if !tt.wantEmpty && octet == "" {
			t.Errorf("extractFirstOctet(%s) returned empty, expected value", tt.address)
		}
	}
}

func TestBuildDetails(t *testing.T) {
	da := NewDiversityAnalyzer(nil)

	// Test nil relays
	details := da.buildDetails(nil, nil, nil, 0, 0, 0)
	if details != "incomplete relay information" {
		t.Errorf("Expected 'incomplete relay information', got %s", details)
	}

	// Test with valid relays
	guard := &RelayInfo{Fingerprint: "G1", Address: "192.168.1.1"}
	middle := &RelayInfo{Fingerprint: "M1", Address: "10.0.1.1"}
	exit := &RelayInfo{Fingerprint: "E1", Address: "172.16.1.1"}

	// High scores
	details = da.buildDetails(guard, middle, exit, 0.95, 0.95, 0.95)
	if details == "" {
		t.Error("Expected non-empty details for high scores")
	}

	// Low scores
	details = da.buildDetails(guard, middle, exit, 0.1, 0.1, 0.1)
	if details == "" {
		t.Error("Expected non-empty details for low scores")
	}
}

func TestConcurrentAnalysis(t *testing.T) {
	da := NewDiversityAnalyzer(nil)

	path := &Path{
		Guard: &directory.Relay{
			Fingerprint: "G1",
			Address:     "192.168.1.1:9001",
		},
		Middle: &directory.Relay{
			Fingerprint: "M1",
			Address:     "10.0.1.1:9001",
		},
		Exit: &directory.Relay{
			Fingerprint: "E1",
			Address:     "172.16.1.1:9001",
		},
	}

	done := make(chan bool, 10)
	errChan := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			score := da.AnalyzePath(path)
			if score == nil {
				errChan <- fmt.Errorf("AnalyzePath returned nil")
				return
			}
			done <- true
		}()
	}

	// Wait with timeout
	timeout := time.After(5 * time.Second)
	successCount := 0
	for successCount < 10 {
		select {
		case <-done:
			successCount++
		case err := <-errChan:
			t.Error(err)
			successCount++ // Still count to exit the loop
		case <-timeout:
			t.Fatal("Test timed out")
		}
	}

	stats := da.GetStats()
	if stats.PathsAnalyzed != 10 {
		t.Errorf("Expected 10 paths analyzed, got %d", stats.PathsAnalyzed)
	}
}

func TestPathScoreDetails(t *testing.T) {
	da := NewDiversityAnalyzer(nil)

	path := &Path{
		Guard: &directory.Relay{
			Fingerprint: "G1",
			Nickname:    "GuardRelay",
			Address:     "192.168.1.1:9001",
		},
		Middle: &directory.Relay{
			Fingerprint: "M1",
			Nickname:    "MiddleRelay",
			Address:     "10.0.1.1:9001",
		},
		Exit: &directory.Relay{
			Fingerprint: "E1",
			Nickname:    "ExitRelay",
			Address:     "172.16.1.1:9001",
		},
	}

	score := da.AnalyzePath(path)
	if score.Details == "" {
		t.Error("Expected non-empty details in score")
	}

	// Verify all score components are populated
	if score.ASScore < 0 || score.ASScore > 1 {
		t.Errorf("ASScore out of range: %f", score.ASScore)
	}
	if score.GeoScore < 0 || score.GeoScore > 1 {
		t.Errorf("GeoScore out of range: %f", score.GeoScore)
	}
	if score.FamilyScore < 0 || score.FamilyScore > 1 {
		t.Errorf("FamilyScore out of range: %f", score.FamilyScore)
	}
	if score.Overall < 0 || score.Overall > 1 {
		t.Errorf("Overall score out of range: %f", score.Overall)
	}
}

func TestRelayInfoCaching(t *testing.T) {
	da := NewDiversityAnalyzer(nil)

	relay := &directory.Relay{
		Fingerprint: "CACHED123",
		Nickname:    "CachedRelay",
		Address:     "192.168.1.1:9001",
	}

	// First extraction
	info1 := da.ExtractRelayInfo(relay)

	// Verify it's cached
	da.mu.RLock()
	cached := da.relayCache["CACHED123"]
	da.mu.RUnlock()

	if cached == nil {
		t.Error("RelayInfo not cached")
	}
	if cached.Fingerprint != info1.Fingerprint {
		t.Error("Cached info doesn't match extracted info")
	}
}

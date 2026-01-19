// Package path provides path diversity analysis for Tor circuits.
// This file implements AS-level, geographic, and family diversity tracking
// to ensure circuit paths don't share common network infrastructure.
package path

import (
	"net"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/directory"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// DiversityLevel represents the level of path diversity for a circuit
type DiversityLevel int

const (
	// DiversityUnknown indicates diversity level could not be determined
	DiversityUnknown DiversityLevel = iota
	// DiversityLow indicates poor path diversity (shared infrastructure)
	DiversityLow
	// DiversityMedium indicates moderate path diversity
	DiversityMedium
	// DiversityHigh indicates good path diversity
	DiversityHigh
	// DiversityExcellent indicates optimal path diversity
	DiversityExcellent
)

// String returns a human-readable representation of the diversity level
func (d DiversityLevel) String() string {
	switch d {
	case DiversityLow:
		return "low"
	case DiversityMedium:
		return "medium"
	case DiversityHigh:
		return "high"
	case DiversityExcellent:
		return "excellent"
	default:
		return "unknown"
	}
}

// PathScore represents the quality score for a circuit path
type PathScore struct {
	// Overall is the combined diversity score (0.0 to 1.0)
	Overall float64

	// ASScore is the AS-level diversity score (0.0 to 1.0)
	// Higher means relays are in more diverse ASes
	ASScore float64

	// GeoScore is the geographic diversity score (0.0 to 1.0)
	// Higher means relays are more geographically distributed
	GeoScore float64

	// FamilyScore is the relay family diversity score (0.0 to 1.0)
	// 1.0 means no relays share the same family
	FamilyScore float64

	// Level is the categorized diversity level
	Level DiversityLevel

	// Details provides human-readable analysis details
	Details string
}

// RelayInfo holds extracted information about a relay for diversity analysis
type RelayInfo struct {
	Fingerprint string
	Nickname    string
	Address     string
	ASNumber    uint32 // Autonomous System number (0 if unknown)
	Country     string // ISO 3166-1 alpha-2 country code (empty if unknown)
	Family      string // Relay family identifier (empty if none)
	Subnet      string // /16 subnet for IP-based grouping
}

// DiversityAnalyzer analyzes path diversity for Tor circuits
type DiversityAnalyzer struct {
	logger *logger.Logger
	mu     sync.RWMutex

	// Cache of relay info for quick lookups
	relayCache map[string]*RelayInfo

	// Statistics tracking
	pathsAnalyzed    int64
	avgScore         float64
	lowDiversityPct  float64
	lastAnalysisTime time.Time
}

// NewDiversityAnalyzer creates a new diversity analyzer
func NewDiversityAnalyzer(log *logger.Logger) *DiversityAnalyzer {
	if log == nil {
		log = logger.NewDefault()
	}

	return &DiversityAnalyzer{
		logger:     log.Component("diversity"),
		relayCache: make(map[string]*RelayInfo),
	}
}

// ExtractRelayInfo extracts diversity-relevant information from a relay
func (da *DiversityAnalyzer) ExtractRelayInfo(relay *directory.Relay) *RelayInfo {
	if relay == nil {
		return nil
	}

	info := &RelayInfo{
		Fingerprint: relay.Fingerprint,
		Nickname:    relay.Nickname,
		Address:     relay.Address,
	}

	// Extract /16 subnet for IP-based grouping
	info.Subnet = extractSubnet(relay.Address)

	// Note: AS lookup and GeoIP are external services not implemented here
	// In production, these would use external databases like MaxMind GeoIP
	// or bgp.tools/rdap APIs. For now, we use subnet-based approximation.

	// Cache the info
	da.mu.Lock()
	da.relayCache[relay.Fingerprint] = info
	da.mu.Unlock()

	return info
}

// AnalyzePath analyzes the diversity of a circuit path
func (da *DiversityAnalyzer) AnalyzePath(path *Path) *PathScore {
	if path == nil || path.Guard == nil || path.Middle == nil || path.Exit == nil {
		return &PathScore{
			Overall: 0,
			Level:   DiversityUnknown,
			Details: "incomplete path",
		}
	}

	// Extract info for all relays
	guardInfo := da.ExtractRelayInfo(path.Guard)
	middleInfo := da.ExtractRelayInfo(path.Middle)
	exitInfo := da.ExtractRelayInfo(path.Exit)

	// Calculate individual diversity scores
	asScore := da.calculateASScore(guardInfo, middleInfo, exitInfo)
	geoScore := da.calculateGeoScore(guardInfo, middleInfo, exitInfo)
	familyScore := da.calculateFamilyScore(guardInfo, middleInfo, exitInfo)

	// Calculate overall score (weighted average)
	// AS diversity is weighted highest as it's most important for security
	overall := (asScore*0.4 + geoScore*0.3 + familyScore*0.3)

	level := scoreToLevel(overall)

	// Update statistics
	da.mu.Lock()
	da.pathsAnalyzed++
	// Running average calculation
	n := float64(da.pathsAnalyzed)
	da.avgScore = da.avgScore + (overall-da.avgScore)/n
	if level == DiversityLow {
		lowCount := da.lowDiversityPct * (n - 1)
		da.lowDiversityPct = (lowCount + 1) / n
	} else {
		lowCount := da.lowDiversityPct * (n - 1)
		da.lowDiversityPct = lowCount / n
	}
	da.lastAnalysisTime = time.Now()
	da.mu.Unlock()

	details := da.buildDetails(guardInfo, middleInfo, exitInfo, asScore, geoScore, familyScore)

	da.logger.Debug("Path analyzed",
		"guard", path.Guard.Nickname,
		"middle", path.Middle.Nickname,
		"exit", path.Exit.Nickname,
		"score", overall,
		"level", level.String())

	return &PathScore{
		Overall:     overall,
		ASScore:     asScore,
		GeoScore:    geoScore,
		FamilyScore: familyScore,
		Level:       level,
		Details:     details,
	}
}

// calculateASScore calculates AS-level diversity score
// Uses subnet as a proxy for AS when AS lookup is not available
func (da *DiversityAnalyzer) calculateASScore(guard, middle, exit *RelayInfo) float64 {
	if guard == nil || middle == nil || exit == nil {
		return 0
	}

	// Count unique subnets as proxy for AS diversity
	subnets := make(map[string]bool)
	if guard.Subnet != "" {
		subnets[guard.Subnet] = true
	}
	if middle.Subnet != "" {
		subnets[middle.Subnet] = true
	}
	if exit.Subnet != "" {
		subnets[exit.Subnet] = true
	}

	// 3 unique subnets = 1.0, 2 = 0.5, 1 = 0.0
	uniqueCount := len(subnets)
	if uniqueCount == 0 {
		return 0.5 // Unknown, assume moderate diversity
	}
	return float64(uniqueCount-1) / 2.0
}

// calculateGeoScore calculates geographic diversity score
// Uses IP address patterns as a heuristic when GeoIP is not available
func (da *DiversityAnalyzer) calculateGeoScore(guard, middle, exit *RelayInfo) float64 {
	if guard == nil || middle == nil || exit == nil {
		return 0
	}

	// Count unique countries if available
	countries := make(map[string]bool)
	if guard.Country != "" {
		countries[guard.Country] = true
	}
	if middle.Country != "" {
		countries[middle.Country] = true
	}
	if exit.Country != "" {
		countries[exit.Country] = true
	}

	if len(countries) > 0 {
		return float64(len(countries)-1) / 2.0
	}

	// Fallback: use first octet of IP as region proxy
	regions := make(map[string]bool)
	guardRegion := extractFirstOctet(guard.Address)
	middleRegion := extractFirstOctet(middle.Address)
	exitRegion := extractFirstOctet(exit.Address)

	if guardRegion != "" {
		regions[guardRegion] = true
	}
	if middleRegion != "" {
		regions[middleRegion] = true
	}
	if exitRegion != "" {
		regions[exitRegion] = true
	}

	if len(regions) == 0 {
		return 0.5 // Unknown, assume moderate diversity
	}
	return float64(len(regions)-1) / 2.0
}

// calculateFamilyScore calculates relay family diversity score
func (da *DiversityAnalyzer) calculateFamilyScore(guard, middle, exit *RelayInfo) float64 {
	if guard == nil || middle == nil || exit == nil {
		return 0
	}

	// If no family info, assume no shared families (best case)
	if guard.Family == "" && middle.Family == "" && exit.Family == "" {
		return 1.0
	}

	// Count unique non-empty families
	families := make(map[string]int)
	if guard.Family != "" {
		families[guard.Family]++
	}
	if middle.Family != "" {
		families[middle.Family]++
	}
	if exit.Family != "" {
		families[exit.Family]++
	}

	// Check for shared families
	for _, count := range families {
		if count > 1 {
			// Relays share a family - reduced score
			return 0.0
		}
	}

	return 1.0
}

// buildDetails builds a human-readable analysis description
func (da *DiversityAnalyzer) buildDetails(guard, middle, exit *RelayInfo, asScore, geoScore, familyScore float64) string {
	if guard == nil || middle == nil || exit == nil {
		return "incomplete relay information"
	}

	details := ""

	// AS/Subnet analysis
	if asScore >= 0.9 {
		details += "All relays in different network subnets. "
	} else if asScore >= 0.4 {
		details += "Some relays share network proximity. "
	} else {
		details += "Multiple relays in same subnet - potential AS overlap. "
	}

	// Geographic analysis
	if geoScore >= 0.9 {
		details += "Good geographic distribution. "
	} else if geoScore >= 0.4 {
		details += "Moderate geographic diversity. "
	} else {
		details += "Limited geographic diversity. "
	}

	// Family analysis
	if familyScore >= 0.9 {
		details += "No shared relay families."
	} else {
		details += "Relays share family membership - same operator."
	}

	return details
}

// GetStats returns current diversity analysis statistics
func (da *DiversityAnalyzer) GetStats() DiversityStats {
	da.mu.RLock()
	defer da.mu.RUnlock()

	return DiversityStats{
		PathsAnalyzed:    da.pathsAnalyzed,
		AverageScore:     da.avgScore,
		LowDiversityPct:  da.lowDiversityPct,
		LastAnalysisTime: da.lastAnalysisTime,
	}
}

// DiversityStats holds statistics about path diversity analysis
type DiversityStats struct {
	PathsAnalyzed    int64
	AverageScore     float64
	LowDiversityPct  float64
	LastAnalysisTime time.Time
}

// scoreToLevel converts a numeric score to a diversity level
func scoreToLevel(score float64) DiversityLevel {
	switch {
	case score >= 0.9:
		return DiversityExcellent
	case score >= 0.7:
		return DiversityHigh
	case score >= 0.4:
		return DiversityMedium
	case score >= 0.0:
		return DiversityLow
	default:
		return DiversityUnknown
	}
}

// extractSubnet extracts the /16 subnet from an IP address
func extractSubnet(address string) string {
	// Handle address with port
	host := address
	if h, _, err := net.SplitHostPort(address); err == nil {
		host = h
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return ""
	}

	// For IPv4, use /16 subnet
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0:2].String()
	}

	// For IPv6, use /32 prefix (first 4 bytes)
	if len(ip) >= 4 {
		return ip[0:4].String()
	}

	return ""
}

// extractFirstOctet extracts the first octet from an IP address as a string
func extractFirstOctet(address string) string {
	host := address
	if h, _, err := net.SplitHostPort(address); err == nil {
		host = h
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return ""
	}

	if ip4 := ip.To4(); ip4 != nil {
		return string(rune(ip4[0]))
	}

	return ""
}

// CheckPathDiversity performs a quick diversity check returning true if path meets minimum diversity requirements
func (da *DiversityAnalyzer) CheckPathDiversity(path *Path) bool {
	score := da.AnalyzePath(path)
	return score.Level >= DiversityMedium
}

// SuggestAlternative suggests whether an alternative path should be sought based on diversity score
func (da *DiversityAnalyzer) SuggestAlternative(score *PathScore) bool {
	if score == nil {
		return true
	}
	return score.Level == DiversityLow
}

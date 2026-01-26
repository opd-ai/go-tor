// Package relay implements BridgeDB integration for bridge distribution.
// This is an educational/research implementation of bridge distribution mechanisms.
package relay

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// BridgeInfo represents information about a bridge for distribution
type BridgeInfo struct {
	Fingerprint string    `json:"fingerprint"` // Bridge identity fingerprint
	Address     string    `json:"address"`     // IP:Port
	Transport   string    `json:"transport"`   // Transport type (vanilla, obfs4, etc.)
	Params      string    `json:"params"`      // Transport parameters
	AddedAt     time.Time `json:"added_at"`    // When bridge was added
}

// BridgeDistributor manages bridge distribution for educational purposes
type BridgeDistributor struct {
	bridges     map[string]*BridgeInfo // fingerprint -> BridgeInfo
	mu          sync.RWMutex
	logger      *logger.Logger
	rateLimiter map[string]time.Time // IP -> last request time
	rlMu        sync.Mutex
}

// DistributorConfig configures the bridge distributor
type DistributorConfig struct {
	RateLimitInterval time.Duration // Minimum time between requests from same IP (default: 1h)
}

// DefaultDistributorConfig returns default configuration
func DefaultDistributorConfig() DistributorConfig {
	return DistributorConfig{
		RateLimitInterval: 1 * time.Hour,
	}
}

// NewBridgeDistributor creates a new bridge distributor
func NewBridgeDistributor(config DistributorConfig, log *logger.Logger) *BridgeDistributor {
	if config.RateLimitInterval == 0 {
		config.RateLimitInterval = 1 * time.Hour
	}

	return &BridgeDistributor{
		bridges:     make(map[string]*BridgeInfo),
		logger:      log,
		rateLimiter: make(map[string]time.Time),
	}
}

// AddBridge adds a bridge to the distribution database
func (bd *BridgeDistributor) AddBridge(info *BridgeInfo) error {
	if info.Fingerprint == "" {
		return fmt.Errorf("bridge fingerprint is required")
	}
	if info.Address == "" {
		return fmt.Errorf("bridge address is required")
	}

	bd.mu.Lock()
	defer bd.mu.Unlock()

	// Set default transport to vanilla if not specified
	if info.Transport == "" {
		info.Transport = "vanilla"
	}

	// Set added time if not set
	if info.AddedAt.IsZero() {
		info.AddedAt = time.Now()
	}

	bd.bridges[info.Fingerprint] = info

	// Safe fingerprint truncation for logging
	fpDisplay := info.Fingerprint
	if len(fpDisplay) > 16 {
		fpDisplay = fpDisplay[:16] + "..."
	}
	bd.logger.Info("Added bridge to distributor",
		"fingerprint", fpDisplay,
		"transport", info.Transport)

	return nil
}

// RemoveBridge removes a bridge from distribution
func (bd *BridgeDistributor) RemoveBridge(fingerprint string) {
	bd.mu.Lock()
	defer bd.mu.Unlock()

	delete(bd.bridges, fingerprint)

	// Safe fingerprint truncation for logging
	fpDisplay := fingerprint
	if len(fpDisplay) > 16 {
		fpDisplay = fpDisplay[:16] + "..."
	}
	bd.logger.Info("Removed bridge from distributor", "fingerprint", fpDisplay)
}

// GetBridges returns bridges for a requestor (with rate limiting)
func (bd *BridgeDistributor) GetBridges(requestorIP, transport string, count int) ([]*BridgeInfo, error) {
	// Check rate limit
	if !bd.checkRateLimit(requestorIP) {
		return nil, fmt.Errorf("rate limit exceeded, try again later")
	}

	bd.mu.RLock()
	defer bd.mu.RUnlock()

	// Filter bridges by transport type
	var matching []*BridgeInfo
	for _, bridge := range bd.bridges {
		if transport == "" || bridge.Transport == transport {
			matching = append(matching, bridge)
		}
	}

	if len(matching) == 0 {
		return nil, fmt.Errorf("no bridges available for transport: %s", transport)
	}

	// Limit count
	if count <= 0 || count > 3 {
		count = 3 // Default: return 3 bridges
	}
	if count > len(matching) {
		count = len(matching)
	}

	// Use deterministic selection based on requestor IP hash
	// This ensures same IP gets same bridges (within rate limit window)
	selected := bd.selectBridges(matching, requestorIP, count)

	bd.logger.Info("Distributed bridges",
		"requestor_ip", requestorIP,
		"transport", transport,
		"count", len(selected))

	return selected, nil
}

// checkRateLimit checks if a requestor is within rate limits
func (bd *BridgeDistributor) checkRateLimit(ip string) bool {
	bd.rlMu.Lock()
	defer bd.rlMu.Unlock()

	lastRequest, exists := bd.rateLimiter[ip]
	if !exists || time.Since(lastRequest) >= 1*time.Hour {
		bd.rateLimiter[ip] = time.Now()
		return true
	}

	return false
}

// selectBridges deterministically selects bridges for a requestor
func (bd *BridgeDistributor) selectBridges(bridges []*BridgeInfo, ip string, count int) []*BridgeInfo {
	// Hash IP to get deterministic but distributed selection
	hash := sha256.Sum256([]byte(ip))
	offset := int(hash[0]) % len(bridges)

	result := make([]*BridgeInfo, 0, count)
	for i := 0; i < count && i < len(bridges); i++ {
		idx := (offset + i) % len(bridges)
		result = append(result, bridges[idx])
	}

	return result
}

// GetStats returns distributor statistics
func (bd *BridgeDistributor) GetStats() map[string]interface{} {
	bd.mu.RLock()
	defer bd.mu.RUnlock()

	transportCount := make(map[string]int)
	for _, bridge := range bd.bridges {
		transportCount[bridge.Transport]++
	}

	return map[string]interface{}{
		"total_bridges": len(bd.bridges),
		"by_transport":  transportCount,
	}
}

// BridgeDistributorServer provides HTTP API for bridge distribution
type BridgeDistributorServer struct {
	distributor *BridgeDistributor
	logger      *logger.Logger
}

// NewBridgeDistributorServer creates an HTTP server for bridge distribution
func NewBridgeDistributorServer(distributor *BridgeDistributor, log *logger.Logger) *BridgeDistributorServer {
	return &BridgeDistributorServer{
		distributor: distributor,
		logger:      log,
	}
}

// ServeHTTP implements http.Handler for bridge distribution
func (s *BridgeDistributorServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract client IP (simple version, no X-Forwarded-For handling for security)
	clientIP := strings.Split(r.RemoteAddr, ":")[0]

	switch r.URL.Path {
	case "/bridges":
		s.handleGetBridges(w, r, clientIP)
	case "/stats":
		s.handleGetStats(w, r)
	default:
		http.NotFound(w, r)
	}
}

// handleGetBridges handles GET /bridges?transport=obfs4&count=3
func (s *BridgeDistributorServer) handleGetBridges(w http.ResponseWriter, r *http.Request, clientIP string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	transport := r.URL.Query().Get("transport")
	count := 3
	if countStr := r.URL.Query().Get("count"); countStr != "" {
		fmt.Sscanf(countStr, "%d", &count)
	}

	bridges, err := s.distributor.GetBridges(clientIP, transport, count)
	if err != nil {
		s.logger.Warn("Failed to get bridges", "error", err, "ip", clientIP)
		http.Error(w, err.Error(), http.StatusTooManyRequests)
		return
	}

	// Format as bridge lines
	response := struct {
		Bridges []string `json:"bridges"`
		Count   int      `json:"count"`
	}{
		Bridges: make([]string, len(bridges)),
		Count:   len(bridges),
	}

	for i, b := range bridges {
		if b.Transport == "vanilla" {
			response.Bridges[i] = fmt.Sprintf("Bridge %s %s", b.Address, b.Fingerprint)
		} else {
			if b.Params != "" {
				response.Bridges[i] = fmt.Sprintf("Bridge %s %s %s %s", b.Transport, b.Address, b.Fingerprint, b.Params)
			} else {
				response.Bridges[i] = fmt.Sprintf("Bridge %s %s %s", b.Transport, b.Address, b.Fingerprint)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetStats handles GET /stats
func (s *BridgeDistributorServer) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := s.distributor.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// EmailResponder simulates bridge email distribution (for research/educational purposes only)
type EmailResponder struct {
	distributor *BridgeDistributor
	logger      *logger.Logger
}

// NewEmailResponder creates a simulated email responder
func NewEmailResponder(distributor *BridgeDistributor, log *logger.Logger) *EmailResponder {
	return &EmailResponder{
		distributor: distributor,
		logger:      log,
	}
}

// GenerateEmailResponse generates a simulated email response with bridges
func (er *EmailResponder) GenerateEmailResponse(senderEmail, transport string) (string, error) {
	// Use email as "IP" for rate limiting
	emailHash := sha256.Sum256([]byte(senderEmail))
	pseudoIP := hex.EncodeToString(emailHash[:8])

	bridges, err := er.distributor.GetBridges(pseudoIP, transport, 3)
	if err != nil {
		return "", err
	}

	// Format email response
	var response strings.Builder
	response.WriteString("Here are your bridge lines:\n\n")
	for _, b := range bridges {
		if b.Transport == "vanilla" {
			response.WriteString(fmt.Sprintf("Bridge %s %s\n", b.Address, b.Fingerprint))
		} else {
			if b.Params != "" {
				response.WriteString(fmt.Sprintf("Bridge %s %s %s %s\n", b.Transport, b.Address, b.Fingerprint, b.Params))
			} else {
				response.WriteString(fmt.Sprintf("Bridge %s %s %s\n", b.Transport, b.Address, b.Fingerprint))
			}
		}
	}
	response.WriteString("\nAdd these to your torrc file or Tor Browser.\n")
	response.WriteString("\nWARNING: This is an educational implementation. For real anonymity, use official Tor bridges from https://bridges.torproject.org/\n")

	er.logger.Info("Generated email response", "email", senderEmail, "bridge_count", len(bridges))

	return response.String(), nil
}

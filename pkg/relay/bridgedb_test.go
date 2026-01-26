package relay

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestNewBridgeDistributor(t *testing.T) {
	log := logger.New(slog.LevelInfo, os.Stdout)
	config := DefaultDistributorConfig()

	bd := NewBridgeDistributor(config, log)
	if bd == nil {
		t.Fatal("Expected non-nil distributor")
	}

	if len(bd.bridges) != 0 {
		t.Errorf("Expected empty bridge map, got %d bridges", len(bd.bridges))
	}
}

func TestDefaultDistributorConfig(t *testing.T) {
	config := DefaultDistributorConfig()
	if config.RateLimitInterval != 1*time.Hour {
		t.Errorf("Expected 1h rate limit, got %v", config.RateLimitInterval)
	}
}

func TestAddBridge(t *testing.T) {
	log := logger.New(slog.LevelInfo, os.Stdout)
	bd := NewBridgeDistributor(DefaultDistributorConfig(), log)

	tests := []struct {
		name    string
		bridge  *BridgeInfo
		wantErr bool
	}{
		{
			name: "valid vanilla bridge",
			bridge: &BridgeInfo{
				Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
				Address:     "192.0.2.1:9001",
			},
			wantErr: false,
		},
		{
			name: "valid obfs4 bridge",
			bridge: &BridgeInfo{
				Fingerprint: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
				Address:     "192.0.2.2:9002",
				Transport:   "obfs4",
				Params:      "cert=abcd1234;iat-mode=0",
			},
			wantErr: false,
		},
		{
			name: "missing fingerprint",
			bridge: &BridgeInfo{
				Address: "192.0.2.3:9003",
			},
			wantErr: true,
		},
		{
			name: "missing address",
			bridge: &BridgeInfo{
				Fingerprint: "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bd.AddBridge(tt.bridge)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddBridge() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// Verify bridge was added
				bd.mu.RLock()
				stored := bd.bridges[tt.bridge.Fingerprint]
				bd.mu.RUnlock()

				if stored == nil {
					t.Error("Bridge not stored")
				} else {
					if stored.Transport == "" {
						if stored.Transport != "vanilla" {
							t.Error("Expected default transport to be 'vanilla'")
						}
					}
					if stored.AddedAt.IsZero() {
						t.Error("AddedAt should be set")
					}
				}
			}
		})
	}
}

func TestRemoveBridge(t *testing.T) {
	log := logger.New(slog.LevelInfo, os.Stdout)
	bd := NewBridgeDistributor(DefaultDistributorConfig(), log)

	fingerprint := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	bridge := &BridgeInfo{
		Fingerprint: fingerprint,
		Address:     "192.0.2.1:9001",
	}

	err := bd.AddBridge(bridge)
	if err != nil {
		t.Fatalf("Failed to add bridge: %v", err)
	}

	bd.RemoveBridge(fingerprint)

	bd.mu.RLock()
	_, exists := bd.bridges[fingerprint]
	bd.mu.RUnlock()

	if exists {
		t.Error("Bridge should have been removed")
	}
}

func TestGetBridges(t *testing.T) {
	log := logger.New(slog.LevelInfo, os.Stdout)
	bd := NewBridgeDistributor(DefaultDistributorConfig(), log)

	// Add test bridges
	bridges := []*BridgeInfo{
		{
			Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			Address:     "192.0.2.1:9001",
			Transport:   "vanilla",
		},
		{
			Fingerprint: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
			Address:     "192.0.2.2:9002",
			Transport:   "obfs4",
			Params:      "cert=test",
		},
		{
			Fingerprint: "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC",
			Address:     "192.0.2.3:9003",
			Transport:   "obfs4",
			Params:      "cert=test2",
		},
	}

	for _, b := range bridges {
		err := bd.AddBridge(b)
		if err != nil {
			t.Fatalf("Failed to add bridge: %v", err)
		}
	}

	tests := []struct {
		name        string
		ip          string
		transport   string
		count       int
		wantCount   int
		wantErr     bool
		expectLimit bool
	}{
		{
			name:      "get all bridges",
			ip:        "198.51.100.1",
			transport: "",
			count:     5,
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:      "filter by obfs4",
			ip:        "198.51.100.2",
			transport: "obfs4",
			count:     3,
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "filter by vanilla",
			ip:        "198.51.100.3",
			transport: "vanilla",
			count:     3,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:        "rate limit",
			ip:          "198.51.100.1", // Same IP as first test
			transport:   "",
			count:       3,
			wantErr:     true,
			expectLimit: true,
		},
		{
			name:      "non-existent transport",
			ip:        "198.51.100.4",
			transport: "meek",
			count:     3,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := bd.GetBridges(tt.ip, tt.transport, tt.count)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetBridges() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(result) != tt.wantCount {
					t.Errorf("Expected %d bridges, got %d", tt.wantCount, len(result))
				}

				// Verify transport filter
				if tt.transport != "" {
					for _, b := range result {
						if b.Transport != tt.transport {
							t.Errorf("Expected transport %s, got %s", tt.transport, b.Transport)
						}
					}
				}
			}

			if tt.expectLimit && err != nil {
				if !strings.Contains(err.Error(), "rate limit") {
					t.Errorf("Expected rate limit error, got: %v", err)
				}
			}
		})
	}
}

func TestSelectBridges(t *testing.T) {
	log := logger.New(slog.LevelInfo, os.Stdout)
	bd := NewBridgeDistributor(DefaultDistributorConfig(), log)

	bridges := []*BridgeInfo{
		{Fingerprint: "AAA", Address: "192.0.2.1:9001"},
		{Fingerprint: "BBB", Address: "192.0.2.2:9002"},
		{Fingerprint: "CCC", Address: "192.0.2.3:9003"},
		{Fingerprint: "DDD", Address: "192.0.2.4:9004"},
		{Fingerprint: "EEE", Address: "192.0.2.5:9005"},
	}

	// Test deterministic selection
	ip := "198.51.100.1"
	selected1 := bd.selectBridges(bridges, ip, 3)
	selected2 := bd.selectBridges(bridges, ip, 3)

	if len(selected1) != 3 || len(selected2) != 3 {
		t.Fatalf("Expected 3 bridges, got %d and %d", len(selected1), len(selected2))
	}

	// Same IP should get same bridges
	for i := range selected1 {
		if selected1[i].Fingerprint != selected2[i].Fingerprint {
			t.Error("Selection should be deterministic for same IP")
		}
	}

	// Different IP should get potentially different bridges
	ip2 := "198.51.100.2"
	selected3 := bd.selectBridges(bridges, ip2, 3)
	// We can't guarantee they're different, but they should be valid
	if len(selected3) != 3 {
		t.Errorf("Expected 3 bridges, got %d", len(selected3))
	}
}

func TestGetStats(t *testing.T) {
	log := logger.New(slog.LevelInfo, os.Stdout)
	bd := NewBridgeDistributor(DefaultDistributorConfig(), log)

	// Add bridges with different transports
	bridges := []*BridgeInfo{
		{Fingerprint: "AAA", Address: "192.0.2.1:9001", Transport: "vanilla"},
		{Fingerprint: "BBB", Address: "192.0.2.2:9002", Transport: "obfs4"},
		{Fingerprint: "CCC", Address: "192.0.2.3:9003", Transport: "obfs4"},
		{Fingerprint: "DDD", Address: "192.0.2.4:9004", Transport: "meek"},
	}

	for _, b := range bridges {
		err := bd.AddBridge(b)
		if err != nil {
			t.Fatalf("Failed to add bridge: %v", err)
		}
	}

	stats := bd.GetStats()

	total, ok := stats["total_bridges"].(int)
	if !ok || total != 4 {
		t.Errorf("Expected 4 total bridges, got %v", stats["total_bridges"])
	}

	byTransport, ok := stats["by_transport"].(map[string]int)
	if !ok {
		t.Fatal("Expected by_transport map")
	}

	if byTransport["vanilla"] != 1 {
		t.Errorf("Expected 1 vanilla bridge, got %d", byTransport["vanilla"])
	}
	if byTransport["obfs4"] != 2 {
		t.Errorf("Expected 2 obfs4 bridges, got %d", byTransport["obfs4"])
	}
	if byTransport["meek"] != 1 {
		t.Errorf("Expected 1 meek bridge, got %d", byTransport["meek"])
	}
}

func TestBridgeDistributorServer(t *testing.T) {
	log := logger.New(slog.LevelInfo, os.Stdout)

	tests := []struct {
		name          string
		path          string
		method        string
		remoteAddr    string // Unique IP for each test to avoid rate limiting
		wantStatus    int
		checkResponse bool
		checkBridges  bool
		checkStats    bool
		expectedCount int
	}{
		{
			name:          "get bridges",
			path:          "/bridges",
			method:        http.MethodGet,
			remoteAddr:    "198.51.100.1:12345",
			wantStatus:    http.StatusOK,
			checkResponse: true,
			checkBridges:  true,
			expectedCount: 2,
		},
		{
			name:          "get obfs4 bridges",
			path:          "/bridges?transport=obfs4",
			method:        http.MethodGet,
			remoteAddr:    "198.51.100.2:12346",
			wantStatus:    http.StatusOK,
			checkResponse: true,
			checkBridges:  true,
			expectedCount: 1,
		},
		{
			name:          "get stats",
			path:          "/stats",
			method:        http.MethodGet,
			remoteAddr:    "198.51.100.3:12347",
			wantStatus:    http.StatusOK,
			checkResponse: true,
			checkStats:    true,
		},
		{
			name:       "invalid path",
			path:       "/invalid",
			method:     http.MethodGet,
			remoteAddr: "198.51.100.4:12348",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid method",
			path:       "/bridges",
			method:     http.MethodPost,
			remoteAddr: "198.51.100.5:12349",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create new distributor and server for each test to avoid rate limiting issues
			bd := NewBridgeDistributor(DefaultDistributorConfig(), log)
			bd.AddBridge(&BridgeInfo{
				Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
				Address:     "192.0.2.1:9001",
				Transport:   "vanilla",
			})
			bd.AddBridge(&BridgeInfo{
				Fingerprint: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
				Address:     "192.0.2.2:9002",
				Transport:   "obfs4",
				Params:      "cert=test",
			})
			server := NewBridgeDistributorServer(bd, log)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.RemoteAddr = tt.remoteAddr
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}

			if tt.checkResponse && w.Code == http.StatusOK {
				if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
					t.Error("Expected JSON content type")
				}
			}

			if tt.checkBridges {
				var response struct {
					Bridges []string `json:"bridges"`
					Count   int      `json:"count"`
				}
				err := json.NewDecoder(w.Body).Decode(&response)
				if err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response.Count != tt.expectedCount {
					t.Errorf("Expected %d bridges, got %d", tt.expectedCount, response.Count)
				}
				if len(response.Bridges) != tt.expectedCount {
					t.Errorf("Expected %d bridge lines, got %d", tt.expectedCount, len(response.Bridges))
				}

				// Verify bridge line format
				for _, line := range response.Bridges {
					if !strings.HasPrefix(line, "Bridge ") {
						t.Errorf("Invalid bridge line format: %s", line)
					}
				}
			}

			if tt.checkStats {
				var stats map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&stats)
				if err != nil {
					t.Fatalf("Failed to decode stats: %v", err)
				}

				if _, ok := stats["total_bridges"]; !ok {
					t.Error("Expected total_bridges in stats")
				}
				if _, ok := stats["by_transport"]; !ok {
					t.Error("Expected by_transport in stats")
				}
			}
		})
	}
}

func TestEmailResponder(t *testing.T) {
	log := logger.New(slog.LevelInfo, os.Stdout)
	bd := NewBridgeDistributor(DefaultDistributorConfig(), log)

	// Add test bridges
	bd.AddBridge(&BridgeInfo{
		Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Address:     "192.0.2.1:9001",
		Transport:   "vanilla",
	})
	bd.AddBridge(&BridgeInfo{
		Fingerprint: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
		Address:     "192.0.2.2:9002",
		Transport:   "obfs4",
		Params:      "cert=test",
	})

	responder := NewEmailResponder(bd, log)

	tests := []struct {
		name      string
		email     string
		transport string
		wantErr   bool
	}{
		{
			name:      "valid request",
			email:     "user@example.com",
			transport: "",
			wantErr:   false,
		},
		{
			name:      "obfs4 request",
			email:     "user2@example.com",
			transport: "obfs4",
			wantErr:   false,
		},
		{
			name:      "rate limited",
			email:     "user@example.com", // Same as first test
			transport: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := responder.GenerateEmailResponse(tt.email, tt.transport)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateEmailResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if !strings.Contains(response, "Here are your bridge lines:") {
					t.Error("Expected bridge lines header in response")
				}
				if !strings.Contains(response, "WARNING: This is an educational implementation") {
					t.Error("Expected warning in response")
				}
				if !strings.Contains(response, "Bridge ") {
					t.Error("Expected bridge lines in response")
				}

				// Count bridge lines
				lines := strings.Split(response, "\n")
				bridgeCount := 0
				for _, line := range lines {
					if strings.HasPrefix(line, "Bridge ") {
						bridgeCount++
					}
				}
				if bridgeCount == 0 {
					t.Error("Expected at least one bridge line")
				}
			}
		})
	}
}

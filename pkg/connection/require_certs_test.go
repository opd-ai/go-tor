package connection

import (
	"testing"
	
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestRequireCERTSGetter tests the RequireCERTS() getter method
func TestRequireCERTSGetter(t *testing.T) {
	tests := []struct {
		name          string
		requireCERTS  bool
		expectedValue bool
	}{
		{
			name:          "RequireCERTS enabled",
			requireCERTS:  true,
			expectedValue: true,
		},
		{
			name:          "RequireCERTS disabled (default)",
			requireCERTS:  false,
			expectedValue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Address:      "127.0.0.1:9001",
				RequireCERTS: tt.requireCERTS,
			}
			
			conn := New(cfg, logger.NewDefault())
			
			if got := conn.RequireCERTS(); got != tt.expectedValue {
				t.Errorf("RequireCERTS() = %v, want %v", got, tt.expectedValue)
			}
		})
	}
}

// TestDefaultConfigRequireCERTS tests that DefaultConfig returns RequireCERTS=false by default
func TestDefaultConfigRequireCERTS(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:9001")
	
	if cfg.RequireCERTS {
		t.Error("DefaultConfig().RequireCERTS should be false by default (backward compatible)")
	}
}

// TestConnectionStoresRequireCERTS tests that Connection properly stores RequireCERTS from Config
func TestConnectionStoresRequireCERTS(t *testing.T) {
	tests := []struct {
		name         string
		requireCERTS bool
	}{
		{name: "RequireCERTS true", requireCERTS: true},
		{name: "RequireCERTS false", requireCERTS: false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Address:      "127.0.0.1:9001",
				RequireCERTS: tt.requireCERTS,
			}
			
			conn := New(cfg, logger.NewDefault())
			
			if conn.requireCERTS != tt.requireCERTS {
				t.Errorf("Connection.requireCERTS = %v, want %v", conn.requireCERTS, tt.requireCERTS)
			}
			
			if conn.RequireCERTS() != tt.requireCERTS {
				t.Errorf("Connection.RequireCERTS() = %v, want %v", conn.RequireCERTS(), tt.requireCERTS)
			}
		})
	}
}

// Package protocol integration tests
// +build integration

package protocol

import (
"testing"
"time"

"github.com/opd-ai/go-tor/pkg/logger"
)

// TestIntegrationHandshakeTimeout tests handshake timeout behavior
func TestIntegrationHandshakeTimeout(t *testing.T) {
if testing.Short() {
t.Skip("Skipping integration test in short mode")
}

log := logger.NewDefault()
handshake := NewHandshake(nil, log)

// Test timeout configuration
tests := []struct {
name    string
timeout time.Duration
wantErr bool
}{
{"valid_short", 5 * time.Second, false},
{"valid_long", 30 * time.Second, false},
{"invalid_too_short", 1 * time.Second, true},
{"invalid_too_long", 120 * time.Second, true},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
err := handshake.SetTimeout(tt.timeout)
if (err != nil) != tt.wantErr {
t.Errorf("SetTimeout(%v) error = %v, wantErr %v", tt.timeout, err, tt.wantErr)
}
})
}
}

// TestIntegrationVersionSelection tests version selection logic
func TestIntegrationVersionSelection(t *testing.T) {
if testing.Short() {
t.Skip("Skipping integration test in short mode")
}

handshake := &Handshake{}

scenarios := []struct {
versions []int
expected int
}{
{[]int{3, 4, 5}, 5},
{[]int{3}, 3},
{[]int{1, 2, 6, 7}, 0},
}

for _, sc := range scenarios {
result := handshake.selectVersion(sc.versions)
if result != sc.expected {
t.Errorf("selectVersion(%v) = %d, want %d", sc.versions, result, sc.expected)
}
}
}

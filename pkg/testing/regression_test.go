// Package testing provides performance regression testing framework
//go:build regression
// +build regression

package testing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/client"
)

// PerformanceBaseline stores baseline performance metrics
type PerformanceBaseline struct {
	Version       string            `json:"version"`
	Timestamp     time.Time         `json:"timestamp"`
	ClientStartup PerformanceMetric `json:"client_startup"`
	CircuitBuild  PerformanceMetric `json:"circuit_build"`
}

// PerformanceMetric stores timing and statistical data
type PerformanceMetric struct {
	Mean time.Duration `json:"mean"`
	Min  time.Duration `json:"min"`
	Max  time.Duration `json:"max"`
	P95  time.Duration `json:"p95"`
}

// LoadBaseline loads performance baseline from file
func LoadBaseline(path string) (*PerformanceBaseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read baseline file: %w", err)
	}

	var baseline PerformanceBaseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, fmt.Errorf("failed to parse baseline: %w", err)
	}

	return &baseline, nil
}

// SaveBaseline saves performance baseline to file
func SaveBaseline(baseline *PerformanceBaseline, path string) error {
	data, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal baseline: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write baseline file: %w", err)
	}

	return nil
}

// TestRegressionEndToEnd tests full end-to-end performance
func TestRegressionEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping regression test in short mode")
	}

	// Create client
	torClient, err := client.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer torClient.Close()

	// Wait for readiness
	if err := torClient.WaitUntilReady(90 * time.Second); err != nil {
		t.Fatalf("Client not ready: %v", err)
	}

	// Measure various operations
	startTime := time.Now()

	// Get stats multiple times
	for i := 0; i < 100; i++ {
		_ = torClient.Stats()
	}

	statsTime := time.Since(startTime)

	// Check health
	startTime = time.Now()
	for i := 0; i < 10; i++ {
		_ = torClient.IsReady()
	}
	healthTime := time.Since(startTime)

	t.Logf("End-to-end performance:")
	t.Logf("  100 GetStats calls: %v (avg: %v)", statsTime, statsTime/100)
	t.Logf("  10 IsReady calls: %v (avg: %v)", healthTime, healthTime/10)
}

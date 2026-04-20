package profiling

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestNewProfiler_NilLogger(t *testing.T) {
	p := NewProfiler(DefaultConfig(), nil)
	if p == nil {
		t.Fatal("expected non-nil profiler")
	}
	defer p.Close()
	if p.logger == nil {
		t.Error("expected default logger to be assigned")
	}
}

func TestNewProfiler_AllProfilingDisabled(t *testing.T) {
	cfg := &Config{
		EnableCPUProfiling:  false,
		EnableHeapProfiling: false,
		EnableMutexProfile:  false,
		EnableBlockProfile:  false,
		PathPrefix:          "/debug/pprof",
	}
	p := NewProfiler(cfg, logger.NewDefault())
	defer p.Close()

	// Stats endpoint should still work
	req := httptest.NewRequest("GET", "/debug/pprof/stats", nil)
	w := httptest.NewRecorder()
	p.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleGC_MethodsNotAllowed(t *testing.T) {
	p := NewProfiler(DefaultConfig(), logger.NewDefault())
	defer p.Close()

	methods := []string{"GET", "PUT", "DELETE", "PATCH"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/debug/pprof/gc", nil)
			w := httptest.NewRecorder()
			p.handleGC(w, req)
			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected 405, got %d", w.Code)
			}
		})
	}
}

func TestDetectGoroutineLeaks_Thresholds(t *testing.T) {
	p := NewProfiler(DefaultConfig(), logger.NewDefault())
	defer p.Close()

	tests := []struct {
		name      string
		rate      float64
		threshold float64
		want      bool
	}{
		{"zero rate zero threshold", 0.0, 0.0, false},
		{"rate below threshold", 0.5, 1.0, false},
		{"rate equals threshold", 1.0, 1.0, false},
		{"rate above threshold", 2.0, 1.0, true},
		{"negative threshold", 0.1, -1.0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p.stats.mu.Lock()
			p.stats.GoroutineLeakRate = tt.rate
			p.stats.mu.Unlock()
			got := p.DetectGoroutineLeaks(context.Background(), tt.threshold)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProfiler_CustomPathPrefix(t *testing.T) {
	cfg := &Config{
		EnableCPUProfiling:  true,
		EnableHeapProfiling: true,
		PathPrefix:          "/custom/prof",
	}
	p := NewProfiler(cfg, logger.NewDefault())
	defer p.Close()

	tests := []struct {
		path   string
		status int
	}{
		{"/custom/prof/stats", http.StatusOK},
		{"/custom/prof/memory", http.StatusOK},
		{"/custom/prof/goroutine-leak", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			p.Handler().ServeHTTP(w, req)
			if w.Code != tt.status {
				t.Errorf("got %d, want %d", w.Code, tt.status)
			}
		})
	}
}

func TestProfiler_CloseIdempotent(t *testing.T) {
	p := NewProfiler(DefaultConfig(), logger.NewDefault())
	if err := p.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}
}

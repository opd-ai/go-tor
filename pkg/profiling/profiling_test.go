package profiling

import (
"context"
"encoding/json"
"net/http"
"net/http/httptest"
"runtime"
"strings"
"testing"
"time"

"github.com/opd-ai/go-tor/pkg/logger"
)

func TestNewProfiler(t *testing.T) {
tests := []struct {
name   string
config *Config
log    *logger.Logger
}{
{
name:   "with nil config uses defaults",
config: nil,
log:    logger.NewDefault(),
},
{
name:   "with custom config",
config: DefaultConfig(),
log:    logger.NewDefault(),
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
p := NewProfiler(tt.config, tt.log)
if p == nil {
t.Fatal("Expected profiler to be created")
}
defer p.Close()

if p.logger == nil {
t.Error("Expected logger to be set")
}
if p.mux == nil {
t.Error("Expected mux to be set")
}
if p.stats == nil {
t.Error("Expected stats to be initialized")
}
})
}
}

func TestDefaultConfig(t *testing.T) {
cfg := DefaultConfig()
if cfg == nil {
t.Fatal("Expected config to be created")
}

if !cfg.EnableCPUProfiling {
t.Error("Expected CPU profiling to be enabled by default")
}
if !cfg.EnableHeapProfiling {
t.Error("Expected heap profiling to be enabled by default")
}
if cfg.PathPrefix != "/debug/pprof" {
t.Errorf("Expected path prefix /debug/pprof, got %s", cfg.PathPrefix)
}
}

func TestProfilerHandlers(t *testing.T) {
p := NewProfiler(DefaultConfig(), logger.NewDefault())
defer p.Close()

tests := []struct {
name           string
path           string
method         string
expectedStatus int
}{
{
name:           "index page",
path:           "/debug/pprof/",
method:         "GET",
expectedStatus: http.StatusOK,
},
{
name:           "goroutine profile",
path:           "/debug/pprof/goroutine",
method:         "GET",
expectedStatus: http.StatusOK,
},
{
name:           "stats endpoint",
path:           "/debug/pprof/stats",
method:         "GET",
expectedStatus: http.StatusOK,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
req := httptest.NewRequest(tt.method, tt.path, nil)
w := httptest.NewRecorder()

p.Handler().ServeHTTP(w, req)

if w.Code != tt.expectedStatus {
t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
}
})
}
}

func TestHandleStats(t *testing.T) {
p := NewProfiler(DefaultConfig(), logger.NewDefault())
defer p.Close()

time.Sleep(100 * time.Millisecond)

req := httptest.NewRequest("GET", "/debug/pprof/stats", nil)
w := httptest.NewRecorder()

p.handleStats(w, req)

if w.Code != http.StatusOK {
t.Errorf("Expected status 200, got %d", w.Code)
}

var stats map[string]interface{}
if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
t.Fatalf("Failed to decode JSON: %v", err)
}

if _, ok := stats["num_goroutines"]; !ok {
t.Error("Expected num_goroutines field in stats JSON")
}
}

func TestHandleGC(t *testing.T) {
p := NewProfiler(DefaultConfig(), logger.NewDefault())
defer p.Close()

t.Run("GET method not allowed", func(t *testing.T) {
req := httptest.NewRequest("GET", "/debug/pprof/gc", nil)
w := httptest.NewRecorder()

p.handleGC(w, req)

if w.Code != http.StatusMethodNotAllowed {
t.Errorf("Expected status 405, got %d", w.Code)
}
})

t.Run("POST triggers GC", func(t *testing.T) {
req := httptest.NewRequest("POST", "/debug/pprof/gc", nil)
w := httptest.NewRecorder()

var beforeStats runtime.MemStats
runtime.ReadMemStats(&beforeStats)
beforeGC := beforeStats.NumGC

p.handleGC(w, req)

var afterStats runtime.MemStats
runtime.ReadMemStats(&afterStats)
afterGC := afterStats.NumGC

if w.Code != http.StatusOK {
t.Errorf("Expected status 200, got %d", w.Code)
}

if afterGC <= beforeGC {
t.Error("Expected GC count to increase after triggering GC")
}
})
}

func TestGetStats(t *testing.T) {
p := NewProfiler(DefaultConfig(), logger.NewDefault())
defer p.Close()

p.updateStats()

stats := p.GetStats()

if stats.NumGoroutines == 0 {
t.Error("Expected non-zero goroutine count")
}
if stats.HeapAllocBytes == 0 {
t.Error("Expected non-zero heap allocation")
}
}

func TestRegisterWithMux(t *testing.T) {
p := NewProfiler(DefaultConfig(), logger.NewDefault())
defer p.Close()

mux := http.NewServeMux()
p.RegisterWithMux(mux)

tests := []string{
"/debug/pprof/",
"/debug/pprof/stats",
}

for _, path := range tests {
req := httptest.NewRequest("GET", path, nil)
w := httptest.NewRecorder()

mux.ServeHTTP(w, req)

if w.Code == http.StatusNotFound {
t.Errorf("Expected endpoint %s to be registered, got 404", path)
}
}
}

func TestDetectGoroutineLeaks(t *testing.T) {
p := NewProfiler(DefaultConfig(), logger.NewDefault())
defer p.Close()

ctx := context.Background()

if p.DetectGoroutineLeaks(ctx, 1.0) {
t.Error("Should not detect leaks initially")
}

p.stats.mu.Lock()
p.stats.GoroutineLeakRate = 5.0
p.stats.mu.Unlock()

if !p.DetectGoroutineLeaks(ctx, 1.0) {
t.Error("Should detect leaks with high leak rate")
}
}

func TestTriggerHeapDump(t *testing.T) {
p := NewProfiler(DefaultConfig(), logger.NewDefault())
defer p.Close()

numObjects, heapBytes := p.TriggerHeapDump()

if numObjects == 0 {
t.Error("Expected non-zero heap objects")
}
if heapBytes == 0 {
t.Error("Expected non-zero heap bytes")
}
}

func TestProfilingEndpointsIntegration(t *testing.T) {
cfg := DefaultConfig()
p := NewProfiler(cfg, logger.NewDefault())
defer p.Close()

server := httptest.NewServer(p.Handler())
defer server.Close()

endpoints := []string{
"/debug/pprof/",
"/debug/pprof/stats",
"/debug/pprof/memory",
}

for _, endpoint := range endpoints {
t.Run("GET "+endpoint, func(t *testing.T) {
resp, err := http.Get(server.URL + endpoint)
if err != nil {
t.Fatalf("Failed to GET %s: %v", endpoint, err)
}
defer resp.Body.Close()

if resp.StatusCode != http.StatusOK {
t.Errorf("Expected status 200 for %s, got %d", endpoint, resp.StatusCode)
}
})
}

t.Run("POST /debug/pprof/gc", func(t *testing.T) {
resp, err := http.Post(server.URL+"/debug/pprof/gc", "application/json", strings.NewReader(""))
if err != nil {
t.Fatalf("Failed to POST /debug/pprof/gc: %v", err)
}
defer resp.Body.Close()

if resp.StatusCode != http.StatusOK {
t.Errorf("Expected status 200, got %d", resp.StatusCode)
}
})
}

// Package profiling provides runtime profiling capabilities for performance analysis.
// This package integrates Go's pprof profiling tools and provides HTTP endpoints
// for CPU profiling, heap profiling, goroutine analysis, and mutex/block profiling.
//
// Security Note: Profiling endpoints expose detailed runtime information and should
// only be enabled in controlled environments. Never expose profiling endpoints on
// public networks without proper authentication and access controls.
package profiling

import (
"context"
"fmt"
"net/http"
"net/http/pprof"
"runtime"
"runtime/debug"
"sync"
"time"

"github.com/opd-ai/go-tor/pkg/logger"
)

// Config holds profiling configuration options.
type Config struct {
// EnableCPUProfiling enables CPU profiling endpoints
EnableCPUProfiling bool
// EnableHeapProfiling enables heap profiling endpoints
EnableHeapProfiling bool
// EnableMutexProfile enables mutex contention profiling
EnableMutexProfile bool
// EnableBlockProfile enables blocking profiling
EnableBlockProfile bool
// MutexProfileRate sets the mutex profiling sampling rate (0 = disabled)
MutexProfileRate int
// BlockProfileRate sets the block profiling sampling rate in nanoseconds (0 = disabled)
BlockProfileRate int
// PathPrefix is the URL path prefix for pprof endpoints (default: "/debug/pprof")
PathPrefix string
}

// DefaultConfig returns default profiling configuration.
func DefaultConfig() *Config {
return &Config{
EnableCPUProfiling:  true,
EnableHeapProfiling: true,
EnableMutexProfile:  false,
EnableBlockProfile:  false,
MutexProfileRate:    0,
BlockProfileRate:    0,
PathPrefix:          "/debug/pprof",
}
}

// Profiler manages runtime profiling and provides profiling endpoints.
type Profiler struct {
config *Config
logger *logger.Logger
mux    *http.ServeMux
mu     sync.RWMutex

// Stats tracking
stats *Stats
}

// Stats tracks profiling statistics and goroutine information.
type Stats struct {
mu                sync.RWMutex
NumGoroutines     int       // Current number of goroutines
PeakGoroutines    int       // Peak number of goroutines observed
HeapAllocBytes    uint64    // Current heap allocation in bytes
PeakHeapBytes     uint64    // Peak heap allocation in bytes
TotalAllocBytes   uint64    // Total bytes allocated (cumulative)
NumGC             uint32    // Number of GC cycles completed
LastGCTime        time.Time // Time of last GC
GoroutineLeakRate float64   // Goroutine leak detection rate (goroutines/sec)
LastStatsUpdate   time.Time // Time when stats were last updated
}

// NewProfiler creates a new profiler instance.
func NewProfiler(cfg *Config, log *logger.Logger) *Profiler {
if cfg == nil {
cfg = DefaultConfig()
}
if log == nil {
log = logger.NewDefault()
}

p := &Profiler{
config: cfg,
logger: log.Component("profiling"),
mux:    http.NewServeMux(),
stats: &Stats{
LastStatsUpdate: time.Now(),
},
}

// Configure runtime profiling rates
if cfg.EnableMutexProfile && cfg.MutexProfileRate > 0 {
runtime.SetMutexProfileFraction(cfg.MutexProfileRate)
p.logger.Info("Mutex profiling enabled", "rate", cfg.MutexProfileRate)
}
if cfg.EnableBlockProfile && cfg.BlockProfileRate > 0 {
runtime.SetBlockProfileRate(cfg.BlockProfileRate)
p.logger.Info("Block profiling enabled", "rate_ns", cfg.BlockProfileRate)
}

// Register pprof handlers
p.registerHandlers()

// Start periodic stats collection
go p.collectStats()

return p
}

// registerHandlers registers all pprof HTTP handlers.
func (p *Profiler) registerHandlers() {
prefix := p.config.PathPrefix

// Index page showing all available profiles
p.mux.HandleFunc(prefix+"/", pprof.Index)

// CPU profiling endpoint
if p.config.EnableCPUProfiling {
p.mux.HandleFunc(prefix+"/profile", pprof.Profile)
p.logger.Debug("Registered CPU profiling endpoint", "path", prefix+"/profile")
}

// Heap profiling endpoints
if p.config.EnableHeapProfiling {
p.mux.HandleFunc(prefix+"/heap", pprof.Index)
p.mux.HandleFunc(prefix+"/allocs", pprof.Index)
p.logger.Debug("Registered heap profiling endpoints", "path", prefix+"/heap")
}

// Symbol and command-line info (always available)
p.mux.HandleFunc(prefix+"/symbol", pprof.Symbol)
p.mux.HandleFunc(prefix+"/cmdline", pprof.Cmdline)

// Goroutine profiling (always available - critical for debugging)
p.mux.HandleFunc(prefix+"/goroutine", pprof.Index)
p.logger.Debug("Registered goroutine profiling endpoint", "path", prefix+"/goroutine")

// Thread creation profiling (always available)
p.mux.HandleFunc(prefix+"/threadcreate", pprof.Index)

// Mutex profiling endpoint
if p.config.EnableMutexProfile {
p.mux.HandleFunc(prefix+"/mutex", pprof.Index)
p.logger.Debug("Registered mutex profiling endpoint", "path", prefix+"/mutex")
}

// Block profiling endpoint
if p.config.EnableBlockProfile {
p.mux.HandleFunc(prefix+"/block", pprof.Index)
p.logger.Debug("Registered block profiling endpoint", "path", prefix+"/block")
}

// Custom stats endpoint
p.mux.HandleFunc(prefix+"/stats", p.handleStats)
p.logger.Debug("Registered stats endpoint", "path", prefix+"/stats")

// Goroutine leak detection endpoint
p.mux.HandleFunc(prefix+"/goroutine-leak", p.handleGoroutineLeak)
p.logger.Debug("Registered goroutine leak detection endpoint", "path", prefix+"/goroutine-leak")

// Memory stats endpoint
p.mux.HandleFunc(prefix+"/memory", p.handleMemoryStats)
p.logger.Debug("Registered memory stats endpoint", "path", prefix+"/memory")

// Trigger manual GC endpoint (useful for debugging memory issues)
p.mux.HandleFunc(prefix+"/gc", p.handleGC)
p.logger.Debug("Registered manual GC endpoint", "path", prefix+"/gc")
}

// Handler returns the HTTP handler for profiling endpoints.
func (p *Profiler) Handler() http.Handler {
return p.mux
}

// collectStats periodically collects runtime statistics.
func (p *Profiler) collectStats() {
ticker := time.NewTicker(10 * time.Second)
defer ticker.Stop()

var lastGoroutineCount int
var lastUpdateTime time.Time

for range ticker.C {
p.updateStats()

// Calculate goroutine leak rate
p.stats.mu.Lock()
currentGoroutines := p.stats.NumGoroutines
currentTime := time.Now()

if !lastUpdateTime.IsZero() {
duration := currentTime.Sub(lastUpdateTime).Seconds()
if duration > 0 {
goroutineDelta := currentGoroutines - lastGoroutineCount
p.stats.GoroutineLeakRate = float64(goroutineDelta) / duration
}
}

lastGoroutineCount = currentGoroutines
lastUpdateTime = currentTime
p.stats.mu.Unlock()
}
}

// updateStats updates profiling statistics.
func (p *Profiler) updateStats() {
var m runtime.MemStats
runtime.ReadMemStats(&m)

var gcStats debug.GCStats
debug.ReadGCStats(&gcStats)

numGoroutines := runtime.NumGoroutine()

p.stats.mu.Lock()
defer p.stats.mu.Unlock()

p.stats.NumGoroutines = numGoroutines
p.stats.HeapAllocBytes = m.HeapAlloc
p.stats.TotalAllocBytes = m.TotalAlloc
p.stats.NumGC = m.NumGC
p.stats.LastStatsUpdate = time.Now()

// Track peak values
if numGoroutines > p.stats.PeakGoroutines {
p.stats.PeakGoroutines = numGoroutines
}
if m.HeapAlloc > p.stats.PeakHeapBytes {
p.stats.PeakHeapBytes = m.HeapAlloc
}

// Update last GC time
if len(gcStats.Pause) > 0 {
p.stats.LastGCTime = gcStats.LastGC
}
}

// GetStats returns a snapshot of current profiling statistics.
func (p *Profiler) GetStats() Stats {
p.stats.mu.RLock()
defer p.stats.mu.RUnlock()
return *p.stats
}

// handleStats serves runtime statistics as JSON.
func (p *Profiler) handleStats(w http.ResponseWriter, r *http.Request) {
stats := p.GetStats()

w.Header().Set("Content-Type", "application/json")
fmt.Fprintf(w, `{
  "num_goroutines": %d,
  "peak_goroutines": %d,
  "heap_alloc_bytes": %d,
  "peak_heap_bytes": %d,
  "total_alloc_bytes": %d,
  "num_gc": %d,
  "last_gc_time": "%s",
  "goroutine_leak_rate": %.2f,
  "last_stats_update": "%s"
}`, stats.NumGoroutines, stats.PeakGoroutines, stats.HeapAllocBytes,
stats.PeakHeapBytes, stats.TotalAllocBytes, stats.NumGC,
stats.LastGCTime.Format(time.RFC3339),
stats.GoroutineLeakRate,
stats.LastStatsUpdate.Format(time.RFC3339))
}

// handleGoroutineLeak analyzes goroutine growth for potential leaks.
func (p *Profiler) handleGoroutineLeak(w http.ResponseWriter, r *http.Request) {
stats := p.GetStats()

// Simple heuristic: if leak rate > 1 goroutine/sec, likely a leak
isLeaking := stats.GoroutineLeakRate > 1.0

w.Header().Set("Content-Type", "application/json")
fmt.Fprintf(w, `{
  "current_goroutines": %d,
  "peak_goroutines": %d,
  "leak_rate_per_second": %.2f,
  "likely_leaking": %t,
  "recommendation": "%s"
}`, stats.NumGoroutines, stats.PeakGoroutines, stats.GoroutineLeakRate, isLeaking,
func() string {
if isLeaking {
return "Potential goroutine leak detected. Check /debug/pprof/goroutine for details."
}
return "No goroutine leak detected. Goroutine count is stable."
}())
}

// handleMemoryStats serves detailed memory statistics.
func (p *Profiler) handleMemoryStats(w http.ResponseWriter, r *http.Request) {
var m runtime.MemStats
runtime.ReadMemStats(&m)

w.Header().Set("Content-Type", "application/json")
fmt.Fprintf(w, `{
  "heap_alloc_bytes": %d,
  "heap_sys_bytes": %d,
  "heap_idle_bytes": %d,
  "heap_inuse_bytes": %d,
  "heap_released_bytes": %d,
  "heap_objects": %d,
  "stack_inuse_bytes": %d,
  "stack_sys_bytes": %d,
  "mspan_inuse_bytes": %d,
  "mspan_sys_bytes": %d,
  "mcache_inuse_bytes": %d,
  "mcache_sys_bytes": %d,
  "gc_sys_bytes": %d,
  "other_sys_bytes": %d,
  "total_alloc_bytes": %d,
  "total_sys_bytes": %d,
  "num_gc": %d,
  "next_gc_bytes": %d,
  "last_gc_time_ns": %d,
  "pause_total_ns": %d
}`, m.HeapAlloc, m.HeapSys, m.HeapIdle, m.HeapInuse, m.HeapReleased,
m.HeapObjects, m.StackInuse, m.StackSys, m.MSpanInuse, m.MSpanSys,
m.MCacheInuse, m.MCacheSys, m.GCSys, m.OtherSys, m.TotalAlloc,
m.Sys, m.NumGC, m.NextGC, m.LastGC, m.PauseTotalNs)
}

// handleGC triggers a manual garbage collection.
func (p *Profiler) handleGC(w http.ResponseWriter, r *http.Request) {
// Only allow POST requests for GC trigger (prevents accidental triggers)
if r.Method != http.MethodPost {
http.Error(w, "Method not allowed. Use POST to trigger GC.", http.StatusMethodNotAllowed)
return
}

p.logger.Info("Manual GC triggered via profiling endpoint")

// Capture stats before GC
var beforeStats runtime.MemStats
runtime.ReadMemStats(&beforeStats)

// Trigger GC
runtime.GC()

// Capture stats after GC
var afterStats runtime.MemStats
runtime.ReadMemStats(&afterStats)

freedBytes := int64(beforeStats.HeapAlloc) - int64(afterStats.HeapAlloc)

w.Header().Set("Content-Type", "application/json")
fmt.Fprintf(w, `{
  "gc_triggered": true,
  "heap_before_bytes": %d,
  "heap_after_bytes": %d,
  "freed_bytes": %d,
  "num_gc": %d
}`, beforeStats.HeapAlloc, afterStats.HeapAlloc, freedBytes, afterStats.NumGC)
}

// Close stops the profiler and cleans up resources.
func (p *Profiler) Close() error {
p.logger.Info("Shutting down profiler")

// Disable profiling rates
if p.config.EnableMutexProfile {
runtime.SetMutexProfileFraction(0)
}
if p.config.EnableBlockProfile {
runtime.SetBlockProfileRate(0)
}

return nil
}

// RegisterWithMux registers profiling endpoints with an existing HTTP mux.
// This is useful for integrating profiling with the metrics server.
func (p *Profiler) RegisterWithMux(mux *http.ServeMux) {
prefix := p.config.PathPrefix

// Re-register all handlers with the provided mux
mux.HandleFunc(prefix+"/", pprof.Index)

if p.config.EnableCPUProfiling {
mux.HandleFunc(prefix+"/profile", pprof.Profile)
}
if p.config.EnableHeapProfiling {
mux.HandleFunc(prefix+"/heap", pprof.Index)
mux.HandleFunc(prefix+"/allocs", pprof.Index)
}

mux.HandleFunc(prefix+"/symbol", pprof.Symbol)
mux.HandleFunc(prefix+"/cmdline", pprof.Cmdline)
mux.HandleFunc(prefix+"/goroutine", pprof.Index)
mux.HandleFunc(prefix+"/threadcreate", pprof.Index)

if p.config.EnableMutexProfile {
mux.HandleFunc(prefix+"/mutex", pprof.Index)
}
if p.config.EnableBlockProfile {
mux.HandleFunc(prefix+"/block", pprof.Index)
}

mux.HandleFunc(prefix+"/stats", p.handleStats)
mux.HandleFunc(prefix+"/goroutine-leak", p.handleGoroutineLeak)
mux.HandleFunc(prefix+"/memory", p.handleMemoryStats)
mux.HandleFunc(prefix+"/gc", p.handleGC)

p.logger.Info("Profiling endpoints registered with HTTP mux", "prefix", prefix)
}

// DetectGoroutineLeaks performs a simple goroutine leak detection check.
// Returns true if a potential leak is detected.
func (p *Profiler) DetectGoroutineLeaks(ctx context.Context, threshold float64) bool {
stats := p.GetStats()
return stats.GoroutineLeakRate > threshold
}

// TriggerHeapDump triggers a heap dump and returns heap profiling information.
// This is useful for debugging memory leaks.
func (p *Profiler) TriggerHeapDump() (numObjects uint64, heapBytes uint64) {
var m runtime.MemStats
runtime.ReadMemStats(&m)
return m.HeapObjects, m.HeapAlloc
}

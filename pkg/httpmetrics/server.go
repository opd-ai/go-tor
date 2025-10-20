// Package httpmetrics provides HTTP-based metrics exposition for monitoring.
// This package implements HTTP endpoints for metrics in JSON and Prometheus formats,
// along with a simple HTML dashboard for real-time monitoring.
package httpmetrics

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/health"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/metrics"
)

// MetricsProvider interface for getting metrics
type MetricsProvider interface {
	Snapshot() *metrics.Snapshot
}

// HealthProvider interface for getting health status
type HealthProvider interface {
	Check(ctx context.Context) health.OverallHealth
}

// Server provides HTTP-based metrics exposition
type Server struct {
	address         string
	metricsProvider MetricsProvider
	healthProvider  HealthProvider
	logger          *logger.Logger
	server          *http.Server
	listener        net.Listener
	mux             *http.ServeMux

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewServer creates a new HTTP metrics server
func NewServer(address string, metricsProvider MetricsProvider, healthProvider HealthProvider, log *logger.Logger) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	mux := http.NewServeMux()

	s := &Server{
		address:         address,
		metricsProvider: metricsProvider,
		healthProvider:  healthProvider,
		logger:          log.Component("httpmetrics"),
		mux:             mux,
		ctx:             ctx,
		cancel:          cancel,
	}

	// Register handlers
	mux.HandleFunc("/metrics", s.handlePrometheusMetrics)
	mux.HandleFunc("/metrics/json", s.handleJSONMetrics)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/debug/metrics", s.handleDashboard)
	mux.HandleFunc("/", s.handleIndex)

	s.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// Start starts the HTTP metrics server
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.address, err)
	}

	s.listener = listener
	actualAddr := listener.Addr().String()
	s.logger.Info("HTTP metrics server listening", "address", actualAddr)

	// Serve in background
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.server.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully stops the HTTP metrics server
func (s *Server) Stop() error {
	s.logger.Info("Stopping HTTP metrics server")

	// Cancel context
	s.cancel()

	// Shutdown server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.Warn("HTTP server shutdown error", "error", err)
		return err
	}

	// Wait for goroutines
	s.wg.Wait()

	s.logger.Info("HTTP metrics server stopped")
	return nil
}

// GetAddress returns the actual listening address
func (s *Server) GetAddress() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.address
}

// handlePrometheusMetrics serves metrics in Prometheus text format
func (s *Server) handlePrometheusMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshot := s.metricsProvider.Snapshot()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)

	// Circuit metrics
	fmt.Fprintf(w, "# HELP tor_circuit_builds_total Total number of circuit build attempts\n")
	fmt.Fprintf(w, "# TYPE tor_circuit_builds_total counter\n")
	fmt.Fprintf(w, "tor_circuit_builds_total %d\n", snapshot.CircuitBuilds)

	fmt.Fprintf(w, "# HELP tor_circuit_build_success_total Total number of successful circuit builds\n")
	fmt.Fprintf(w, "# TYPE tor_circuit_build_success_total counter\n")
	fmt.Fprintf(w, "tor_circuit_build_success_total %d\n", snapshot.CircuitBuildSuccess)

	fmt.Fprintf(w, "# HELP tor_circuit_build_failures_total Total number of failed circuit builds\n")
	fmt.Fprintf(w, "# TYPE tor_circuit_build_failures_total counter\n")
	fmt.Fprintf(w, "tor_circuit_build_failures_total %d\n", snapshot.CircuitBuildFailure)

	fmt.Fprintf(w, "# HELP tor_circuit_build_duration_seconds_avg Average circuit build duration in seconds\n")
	fmt.Fprintf(w, "# TYPE tor_circuit_build_duration_seconds_avg gauge\n")
	fmt.Fprintf(w, "tor_circuit_build_duration_seconds_avg %.3f\n", snapshot.CircuitBuildTimeAvg.Seconds())

	fmt.Fprintf(w, "# HELP tor_circuit_build_duration_seconds_p95 95th percentile circuit build duration in seconds\n")
	fmt.Fprintf(w, "# TYPE tor_circuit_build_duration_seconds_p95 gauge\n")
	fmt.Fprintf(w, "tor_circuit_build_duration_seconds_p95 %.3f\n", snapshot.CircuitBuildTimeP95.Seconds())

	fmt.Fprintf(w, "# HELP tor_active_circuits Current number of active circuits\n")
	fmt.Fprintf(w, "# TYPE tor_active_circuits gauge\n")
	fmt.Fprintf(w, "tor_active_circuits %d\n", snapshot.ActiveCircuits)

	// Connection metrics
	fmt.Fprintf(w, "# HELP tor_connection_attempts_total Total number of connection attempts\n")
	fmt.Fprintf(w, "# TYPE tor_connection_attempts_total counter\n")
	fmt.Fprintf(w, "tor_connection_attempts_total %d\n", snapshot.ConnectionAttempts)

	fmt.Fprintf(w, "# HELP tor_connection_success_total Total number of successful connections\n")
	fmt.Fprintf(w, "# TYPE tor_connection_success_total counter\n")
	fmt.Fprintf(w, "tor_connection_success_total %d\n", snapshot.ConnectionSuccess)

	fmt.Fprintf(w, "# HELP tor_connection_failures_total Total number of failed connections\n")
	fmt.Fprintf(w, "# TYPE tor_connection_failures_total counter\n")
	fmt.Fprintf(w, "tor_connection_failures_total %d\n", snapshot.ConnectionFailures)

	fmt.Fprintf(w, "# HELP tor_active_connections Current number of active connections\n")
	fmt.Fprintf(w, "# TYPE tor_active_connections gauge\n")
	fmt.Fprintf(w, "tor_active_connections %d\n", snapshot.ActiveConnections)

	// Stream metrics
	fmt.Fprintf(w, "# HELP tor_streams_created_total Total number of streams created\n")
	fmt.Fprintf(w, "# TYPE tor_streams_created_total counter\n")
	fmt.Fprintf(w, "tor_streams_created_total %d\n", snapshot.StreamsCreated)

	fmt.Fprintf(w, "# HELP tor_streams_closed_total Total number of streams closed\n")
	fmt.Fprintf(w, "# TYPE tor_streams_closed_total counter\n")
	fmt.Fprintf(w, "tor_streams_closed_total %d\n", snapshot.StreamsClosed)

	fmt.Fprintf(w, "# HELP tor_active_streams Current number of active streams\n")
	fmt.Fprintf(w, "# TYPE tor_active_streams gauge\n")
	fmt.Fprintf(w, "tor_active_streams %d\n", snapshot.ActiveStreams)

	fmt.Fprintf(w, "# HELP tor_stream_data_bytes_total Total bytes transferred through streams\n")
	fmt.Fprintf(w, "# TYPE tor_stream_data_bytes_total counter\n")
	fmt.Fprintf(w, "tor_stream_data_bytes_total %d\n", snapshot.StreamData)

	// Guard metrics
	fmt.Fprintf(w, "# HELP tor_guards_active Current number of active guards\n")
	fmt.Fprintf(w, "# TYPE tor_guards_active gauge\n")
	fmt.Fprintf(w, "tor_guards_active %d\n", snapshot.GuardsActive)

	fmt.Fprintf(w, "# HELP tor_guards_confirmed Current number of confirmed guards\n")
	fmt.Fprintf(w, "# TYPE tor_guards_confirmed gauge\n")
	fmt.Fprintf(w, "tor_guards_confirmed %d\n", snapshot.GuardsConfirmed)

	// SOCKS metrics
	fmt.Fprintf(w, "# HELP tor_socks_connections_total Total number of SOCKS connections\n")
	fmt.Fprintf(w, "# TYPE tor_socks_connections_total counter\n")
	fmt.Fprintf(w, "tor_socks_connections_total %d\n", snapshot.SocksConnections)

	fmt.Fprintf(w, "# HELP tor_socks_requests_total Total number of SOCKS requests\n")
	fmt.Fprintf(w, "# TYPE tor_socks_requests_total counter\n")
	fmt.Fprintf(w, "tor_socks_requests_total %d\n", snapshot.SocksRequests)

	// System metrics
	fmt.Fprintf(w, "# HELP tor_uptime_seconds Client uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE tor_uptime_seconds gauge\n")
	fmt.Fprintf(w, "tor_uptime_seconds %d\n", snapshot.UptimeSeconds)
}

// handleJSONMetrics serves metrics in JSON format
func (s *Server) handleJSONMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshot := s.metricsProvider.Snapshot()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snapshot); err != nil {
		s.logger.Error("Failed to encode metrics", "error", err)
	}
}

// handleHealth serves health check information
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	healthStatus := s.healthProvider.Check(ctx)

	// Set HTTP status based on health
	statusCode := http.StatusOK
	if healthStatus.Status == health.StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	} else if healthStatus.Status == health.StatusDegraded {
		statusCode = http.StatusOK // Still 200, but degraded in response
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(healthStatus); err != nil {
		s.logger.Error("Failed to encode health status", "error", err)
	}
}

// handleDashboard serves a simple HTML dashboard
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshot := s.metricsProvider.Snapshot()

	tmpl := template.Must(template.New("dashboard").Parse(dashboardTemplate))

	data := struct {
		Metrics   *metrics.Snapshot
		Timestamp time.Time
	}{
		Metrics:   snapshot,
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if err := tmpl.Execute(w, data); err != nil {
		s.logger.Error("Failed to render dashboard", "error", err)
	}
}

// handleIndex serves the index page with links to available endpoints
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>go-tor Metrics</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        h1 { color: #333; }
        ul { list-style-type: none; padding: 0; }
        li { margin: 10px 0; }
        a { color: #7B68EE; text-decoration: none; }
        a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <h1>go-tor Metrics Server</h1>
    <p>Available endpoints:</p>
    <ul>
        <li><a href="/metrics">/metrics</a> - Prometheus format metrics</li>
        <li><a href="/metrics/json">/metrics/json</a> - JSON format metrics</li>
        <li><a href="/health">/health</a> - Health check status</li>
        <li><a href="/debug/metrics">/debug/metrics</a> - Real-time dashboard</li>
    </ul>
</body>
</html>`)
}

const dashboardTemplate = `<!DOCTYPE html>
<html>
<head>
    <title>go-tor Metrics Dashboard</title>
    <meta http-equiv="refresh" content="5">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            margin: 0;
            padding: 20px;
            background: #f5f5f5;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
        }
        h1 {
            color: #333;
            border-bottom: 3px solid #7B68EE;
            padding-bottom: 10px;
        }
        .timestamp {
            color: #666;
            font-size: 0.9em;
            margin-bottom: 20px;
        }
        .metrics-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 20px;
            margin-bottom: 20px;
        }
        .metric-card {
            background: white;
            border-radius: 8px;
            padding: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .metric-card h2 {
            margin-top: 0;
            color: #555;
            font-size: 1.2em;
            border-bottom: 2px solid #eee;
            padding-bottom: 10px;
        }
        .metric-row {
            display: flex;
            justify-content: space-between;
            padding: 8px 0;
            border-bottom: 1px solid #f0f0f0;
        }
        .metric-row:last-child {
            border-bottom: none;
        }
        .metric-label {
            color: #666;
            font-weight: 500;
        }
        .metric-value {
            color: #333;
            font-weight: bold;
        }
        .success { color: #28a745; }
        .warning { color: #ffc107; }
        .danger { color: #dc3545; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🧅 go-tor Metrics Dashboard</h1>
        <div class="timestamp">Last updated: {{.Timestamp.Format "2006-01-02 15:04:05 MST"}} (auto-refresh every 5s)</div>

        <div class="metrics-grid">
            <!-- Circuit Metrics -->
            <div class="metric-card">
                <h2>Circuit Metrics</h2>
                <div class="metric-row">
                    <span class="metric-label">Active Circuits:</span>
                    <span class="metric-value">{{.Metrics.ActiveCircuits}}</span>
                </div>
                <div class="metric-row">
                    <span class="metric-label">Total Builds:</span>
                    <span class="metric-value">{{.Metrics.CircuitBuilds}}</span>
                </div>
                <div class="metric-row">
                    <span class="metric-label">Successful:</span>
                    <span class="metric-value success">{{.Metrics.CircuitBuildSuccess}}</span>
                </div>
                <div class="metric-row">
                    <span class="metric-label">Failed:</span>
                    <span class="metric-value danger">{{.Metrics.CircuitBuildFailure}}</span>
                </div>
                <div class="metric-row">
                    <span class="metric-label">Avg Build Time:</span>
                    <span class="metric-value">{{printf "%.2fs" .Metrics.CircuitBuildTimeAvg.Seconds}}</span>
                </div>
                <div class="metric-row">
                    <span class="metric-label">P95 Build Time:</span>
                    <span class="metric-value">{{printf "%.2fs" .Metrics.CircuitBuildTimeP95.Seconds}}</span>
                </div>
            </div>

            <!-- Connection Metrics -->
            <div class="metric-card">
                <h2>Connection Metrics</h2>
                <div class="metric-row">
                    <span class="metric-label">Active Connections:</span>
                    <span class="metric-value">{{.Metrics.ActiveConnections}}</span>
                </div>
                <div class="metric-row">
                    <span class="metric-label">Total Attempts:</span>
                    <span class="metric-value">{{.Metrics.ConnectionAttempts}}</span>
                </div>
                <div class="metric-row">
                    <span class="metric-label">Successful:</span>
                    <span class="metric-value success">{{.Metrics.ConnectionSuccess}}</span>
                </div>
                <div class="metric-row">
                    <span class="metric-label">Failed:</span>
                    <span class="metric-value danger">{{.Metrics.ConnectionFailures}}</span>
                </div>
                <div class="metric-row">
                    <span class="metric-label">Retries:</span>
                    <span class="metric-value">{{.Metrics.ConnectionRetries}}</span>
                </div>
                <div class="metric-row">
                    <span class="metric-label">Avg TLS Handshake:</span>
                    <span class="metric-value">{{printf "%.2fs" .Metrics.TLSHandshakeAvg.Seconds}}</span>
                </div>
            </div>

            <!-- Stream Metrics -->
            <div class="metric-card">
                <h2>Stream Metrics</h2>
                <div class="metric-row">
                    <span class="metric-label">Active Streams:</span>
                    <span class="metric-value">{{.Metrics.ActiveStreams}}</span>
                </div>
                <div class="metric-row">
                    <span class="metric-label">Created:</span>
                    <span class="metric-value">{{.Metrics.StreamsCreated}}</span>
                </div>
                <div class="metric-row">
                    <span class="metric-label">Closed:</span>
                    <span class="metric-value">{{.Metrics.StreamsClosed}}</span>
                </div>
                <div class="metric-row">
                    <span class="metric-label">Failures:</span>
                    <span class="metric-value danger">{{.Metrics.StreamFailures}}</span>
                </div>
                <div class="metric-row">
                    <span class="metric-label">Data Transferred:</span>
                    <span class="metric-value">{{.Metrics.StreamData}} bytes</span>
                </div>
            </div>

            <!-- Guard & SOCKS Metrics -->
            <div class="metric-card">
                <h2>Guard & SOCKS Metrics</h2>
                <div class="metric-row">
                    <span class="metric-label">Active Guards:</span>
                    <span class="metric-value">{{.Metrics.GuardsActive}}</span>
                </div>
                <div class="metric-row">
                    <span class="metric-label">Confirmed Guards:</span>
                    <span class="metric-value success">{{.Metrics.GuardsConfirmed}}</span>
                </div>
                <div class="metric-row">
                    <span class="metric-label">SOCKS Connections:</span>
                    <span class="metric-value">{{.Metrics.SocksConnections}}</span>
                </div>
                <div class="metric-row">
                    <span class="metric-label">SOCKS Requests:</span>
                    <span class="metric-value">{{.Metrics.SocksRequests}}</span>
                </div>
                <div class="metric-row">
                    <span class="metric-label">SOCKS Errors:</span>
                    <span class="metric-value danger">{{.Metrics.SocksErrors}}</span>
                </div>
            </div>

            <!-- System Metrics -->
            <div class="metric-card">
                <h2>System Metrics</h2>
                <div class="metric-row">
                    <span class="metric-label">Uptime:</span>
                    <span class="metric-value">{{.Metrics.UptimeSeconds}}s</span>
                </div>
            </div>
        </div>
    </div>
</body>
</html>`

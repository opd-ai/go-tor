package pt

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/errors"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// Manager manages multiple pluggable transports with automatic restarts.
type Manager struct {
	clients map[string]*ManagedClient
	servers map[string]*ManagedServer
	config  ManagerConfig
	mu      sync.RWMutex
	log     *logger.Logger
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// ManagerConfig configures the PT manager.
type ManagerConfig struct {
	// StateDir is the root state directory for all PTs.
	StateDir string

	// AutoRestart enables automatic PT process restart on failure.
	AutoRestart bool

	// RestartDelay is the delay between restart attempts.
	RestartDelay time.Duration

	// MaxRestarts is the maximum number of restart attempts (0 = unlimited).
	MaxRestarts int
}

// DefaultManagerConfig returns sensible defaults for PT management.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		StateDir:     filepath.Join(os.TempDir(), "go-tor-pt"),
		AutoRestart:  true,
		RestartDelay: 5 * time.Second,
		MaxRestarts:  0, // Unlimited
	}
}

// NewManager creates a new pluggable transport manager.
func NewManager(config ManagerConfig) *Manager {
	if config.StateDir == "" {
		config.StateDir = DefaultManagerConfig().StateDir
	}
	if config.RestartDelay == 0 {
		config.RestartDelay = DefaultManagerConfig().RestartDelay
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		clients: make(map[string]*ManagedClient),
		servers: make(map[string]*ManagedServer),
		config:  config,
		log:     logger.New(slog.LevelInfo, os.Stdout),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// AddClient registers a client PT with the manager.
func (m *Manager) AddClient(name string, config TransportConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[name]; exists {
		return errors.New(errors.CategoryConfiguration, errors.SeverityMedium, "PT client already registered: "+name)
	}

	if config.StateDir == "" {
		config.StateDir = filepath.Join(m.config.StateDir, "client", name)
	}

	client, err := NewManagedClient(config)
	if err != nil {
		return err
	}

	m.clients[name] = client
	m.log.Info("Registered PT client", "name", name, "binary", config.BinaryPath)

	return nil
}

// AddServer registers a server PT with the manager.
func (m *Manager) AddServer(name string, config TransportConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.servers[name]; exists {
		return errors.New(errors.CategoryConfiguration, errors.SeverityMedium, "PT server already registered: "+name)
	}

	if config.StateDir == "" {
		config.StateDir = filepath.Join(m.config.StateDir, "server", name)
	}

	server, err := NewManagedServer(config)
	if err != nil {
		return err
	}

	m.servers[name] = server
	m.log.Info("Registered PT server", "name", name, "binary", config.BinaryPath)

	return nil
}

// StartAll starts all registered PTs.
func (m *Manager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, client := range m.clients {
		if err := client.Start(ctx); err != nil {
			m.log.Warn("Failed to start PT client", "name", name, "error", err)
			if !m.config.AutoRestart {
				return err
			}
		}

		if m.config.AutoRestart {
			m.wg.Add(1)
			go m.monitorClient(name, client)
		}
	}

	for name, server := range m.servers {
		if err := server.Start(ctx); err != nil {
			m.log.Warn("Failed to start PT server", "name", name, "error", err)
			if !m.config.AutoRestart {
				return err
			}
		}

		if m.config.AutoRestart {
			m.wg.Add(1)
			go m.monitorServer(name, server)
		}
	}

	return nil
}

// monitorClient monitors a client PT and restarts it on failure.
func (m *Manager) monitorClient(name string, client *ManagedClient) {
	defer m.wg.Done()

	restartCount := 0
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			client.mu.RLock()
			running := client.running
			cmd := client.cmd
			client.mu.RUnlock()

			if !running {
				continue
			}

			if cmd != nil && cmd.ProcessState != nil && cmd.ProcessState.Exited() {
				m.log.Warn("PT client exited", "name", name, "exit_code", cmd.ProcessState.ExitCode())

				if m.config.MaxRestarts > 0 && restartCount >= m.config.MaxRestarts {
					m.log.Error("PT client max restarts exceeded", "name", name, "count", restartCount)
					return
				}

				restartCount++
				m.log.Info("Restarting PT client", "name", name, "attempt", restartCount)

				time.Sleep(m.config.RestartDelay)

				if err := client.Start(m.ctx); err != nil {
					m.log.Error("Failed to restart PT client", "name", name, "error", err)
				} else {
					m.log.Info("PT client restarted successfully", "name", name)
				}
			}
		}
	}
}

// monitorServer monitors a server PT and restarts it on failure.
func (m *Manager) monitorServer(name string, server *ManagedServer) {
	defer m.wg.Done()

	restartCount := 0
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			server.mu.RLock()
			running := server.running
			cmd := server.cmd
			server.mu.RUnlock()

			if !running {
				continue
			}

			if cmd != nil && cmd.ProcessState != nil && cmd.ProcessState.Exited() {
				m.log.Warn("PT server exited", "name", name, "exit_code", cmd.ProcessState.ExitCode())

				if m.config.MaxRestarts > 0 && restartCount >= m.config.MaxRestarts {
					m.log.Error("PT server max restarts exceeded", "name", name, "count", restartCount)
					return
				}

				restartCount++
				m.log.Info("Restarting PT server", "name", name, "attempt", restartCount)

				time.Sleep(m.config.RestartDelay)

				if err := server.Start(m.ctx); err != nil {
					m.log.Error("Failed to restart PT server", "name", name, "error", err)
				} else {
					m.log.Info("PT server restarted successfully", "name", name)
				}
			}
		}
	}
}

// GetClient returns a client PT by name.
func (m *Manager) GetClient(name string) (*ManagedClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, ok := m.clients[name]
	if !ok {
		return nil, fmt.Errorf("PT client not found: %s", name)
	}

	return client, nil
}

// GetServer returns a server PT by name.
func (m *Manager) GetServer(name string) (*ManagedServer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	server, ok := m.servers[name]
	if !ok {
		return nil, fmt.Errorf("PT server not found: %s", name)
	}

	return server, nil
}

// Clients returns all registered client names.
func (m *Manager) Clients() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.clients))
	for name := range m.clients {
		names = append(names, name)
	}
	return names
}

// Servers returns all registered server names.
func (m *Manager) Servers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	return names
}

// Close shuts down all PTs and stops monitoring.
func (m *Manager) Close() error {
	m.cancel()
	m.wg.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()

	for name, client := range m.clients {
		if err := client.Close(); err != nil {
			m.log.Warn("Failed to close PT client", "name", name, "error", err)
		}
	}

	for name, server := range m.servers {
		if err := server.Close(); err != nil {
			m.log.Warn("Failed to close PT server", "name", name, "error", err)
		}
	}

	return nil
}

package pt

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/errors"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// ManagedServer represents a server-side pluggable transport managed as an external process.
// It implements the PT version 2 IPC protocol per pt-spec.txt for bridge relays.
type ManagedServer struct {
	config TransportConfig
	cmd    *exec.Cmd
	stdout io.ReadCloser
	stderr io.ReadCloser

	methods   map[string]*ServerMethodInfo
	listeners map[string]net.Listener
	mu        sync.RWMutex
	running   bool

	log *logger.Logger
}

// ServerMethodInfo describes a server transport method.
type ServerMethodInfo struct {
	Name    string
	Address string
	Options map[string]string
}

// NewManagedServer creates a new managed PT server for bridge relays.
func NewManagedServer(config TransportConfig) (*ManagedServer, error) {
	if config.BinaryPath == "" {
		return nil, errors.New(errors.CategoryConfiguration, errors.SeverityHigh, "PT binary path is required")
	}

	if config.StateDir == "" {
		config.StateDir = os.TempDir() + "/go-tor-pt-server"
	}

	if config.TorSOCKSPort == 0 {
		config.TorSOCKSPort = 9050 // Default Tor SOCKS port
	}

	return &ManagedServer{
		config:    config,
		methods:   make(map[string]*ServerMethodInfo),
		listeners: make(map[string]net.Listener),
		log:       logger.New(slog.LevelInfo, os.Stdout),
	}, nil
}

// Name returns the transport name (derived from binary name).
func (ms *ManagedServer) Name() string {
	name := filepath.Base(ms.config.BinaryPath)
	return strings.TrimSuffix(name, ".exe")
}

// Start launches the PT server process and performs IPC handshake.
func (ms *ManagedServer) Start(ctx context.Context) error {
	ms.mu.Lock()
	
	if ms.running {
		ms.mu.Unlock()
		return nil
	}

	if err := os.MkdirAll(ms.config.StateDir, 0o700); err != nil {
		ms.mu.Unlock()
		return errors.Wrap(errors.CategoryNetwork, errors.SeverityHigh, "failed to create state directory", err)
	}

	ms.cmd = exec.CommandContext(ctx, ms.config.BinaryPath)
	ms.cmd.Env = ms.buildEnvironment()

	stdout, err := ms.cmd.StdoutPipe()
	if err != nil {
		ms.mu.Unlock()
		return errors.Wrap(errors.CategoryNetwork, errors.SeverityHigh, "failed to create stdout pipe", err)
	}
	ms.stdout = stdout

	stderr, err := ms.cmd.StderrPipe()
	if err != nil {
		ms.mu.Unlock()
		return errors.Wrap(errors.CategoryNetwork, errors.SeverityHigh, "failed to create stderr pipe", err)
	}
	ms.stderr = stderr

	if err := ms.cmd.Start(); err != nil {
		ms.mu.Unlock()
		return errors.Wrap(errors.CategoryNetwork, errors.SeverityHigh, "failed to start PT server process", err)
	}

	ms.running = true
	ms.log.Info("PT server process started", "binary", ms.config.BinaryPath, "pid", ms.cmd.Process.Pid)
	
	// Unlock before handshake to avoid deadlock when parseSMethod tries to acquire lock
	ms.mu.Unlock()

	go ms.readStderr()

	if err := ms.performHandshake(ctx); err != nil {
		ms.mu.Lock()
		ms.cmd.Process.Kill()
		ms.running = false
		ms.mu.Unlock()
		return errors.Wrap(errors.CategoryProtocol, errors.SeverityHigh, "PT server handshake failed", err)
	}

	return nil
}

// buildEnvironment constructs the environment variables for the PT server process per pt-spec.txt §3.3.
func (ms *ManagedServer) buildEnvironment() []string {
	env := []string{
		"TOR_PT_MANAGED_TRANSPORT_VER=1",
		"TOR_PT_STATE_LOCATION=" + ms.config.StateDir,
		"TOR_PT_SERVER_TRANSPORTS=*",                        // Request all available transports
		fmt.Sprintf("TOR_PT_SERVER_BINDADDR=*-127.0.0.1:0"), // Bind to any available port
	}

	// Extended ORPort for communication with Tor (optional)
	if ms.config.TorSOCKSPort > 0 {
		env = append(env, fmt.Sprintf("TOR_PT_EXTENDED_SERVER_PORT=127.0.0.1:%d", ms.config.TorSOCKSPort))
	}

	// PT-specific options
	for key, value := range ms.config.Options {
		env = append(env, fmt.Sprintf("TOR_PT_SERVER_TRANSPORT_OPTIONS=%s:%s=%s",
			"*", strings.ToLower(key), value))
	}

	return append(os.Environ(), env...)
}

// performHandshake reads the PT server's stdout and parses SMETHOD lines.
func (ms *ManagedServer) performHandshake(ctx context.Context) error {
	scanner := bufio.NewScanner(ms.stdout)
	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return err
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}

		line := scanner.Text()
		ms.log.Debug("PT server stdout", "line", line)

		if strings.HasPrefix(line, "SMETHOD-ERROR") {
			return fmt.Errorf("PT server reported error: %s", line)
		}

		if strings.HasPrefix(line, "SMETHOD") {
			if err := ms.parseSMethod(line); err != nil {
				ms.log.Warn("Failed to parse SMETHOD", "line", line, "error", err)
			}
		}

		if strings.HasPrefix(line, "SMETHODS DONE") {
			ms.log.Info("PT server handshake complete", "methods", len(ms.methods))
			return nil
		}
	}

	return fmt.Errorf("PT server handshake timeout")
}

// parseSMethod parses a SMETHOD line: SMETHOD <methodname> <address> [options]
// Example: SMETHOD obfs4 127.0.0.1:1234 ARGS:cert=...,iat-mode=0
func (ms *ManagedServer) parseSMethod(line string) error {
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return fmt.Errorf("invalid SMETHOD format: %s", line)
	}

	method := &ServerMethodInfo{
		Name:    parts[1],
		Address: parts[2],
		Options: make(map[string]string),
	}

	// Parse optional ARGS field
	for i := 3; i < len(parts); i++ {
		if strings.HasPrefix(parts[i], "ARGS:") {
			argsStr := strings.TrimPrefix(parts[i], "ARGS:")
			argPairs := strings.Split(argsStr, ",")
			for _, pair := range argPairs {
				kv := strings.SplitN(pair, "=", 2)
				if len(kv) == 2 {
					method.Options[kv[0]] = kv[1]
				}
			}
		}
	}

	ms.mu.Lock()
	ms.methods[method.Name] = method
	ms.mu.Unlock()

	ms.log.Info("Registered PT server method", "name", method.Name, "address", method.Address, "options", len(method.Options))
	return nil
}

// readStderr consumes stderr output in background.
func (ms *ManagedServer) readStderr() {
	scanner := bufio.NewScanner(ms.stderr)
	for scanner.Scan() {
		ms.log.Debug("PT server stderr", "line", scanner.Text())
	}
}

// Listen starts accepting connections on the PT server for the given bind address.
// The PT process handles the actual listening; this returns a connection to the PT's listener.
func (ms *ManagedServer) Listen(ctx context.Context, bindAddr string) (net.Listener, error) {
	ms.mu.RLock()
	if !ms.running || len(ms.methods) == 0 {
		ms.mu.RUnlock()
		return nil, fmt.Errorf("PT server not ready")
	}

	// Get the first available method
	var method *ServerMethodInfo
	for _, m := range ms.methods {
		method = m
		break
	}
	ms.mu.RUnlock()

	// The PT binary is already listening on method.Address
	// We create a wrapper listener that accepts connections from the PT
	listener := &ptServerListener{
		addr:   method.Address,
		ctx:    ctx,
		method: method.Name,
		log:    ms.log,
	}

	ms.mu.Lock()
	ms.listeners[method.Name] = listener
	ms.mu.Unlock()

	ms.log.Info("PT server listener ready", "method", method.Name, "address", method.Address)
	return listener, nil
}

// Methods returns available server transport method names.
func (ms *ManagedServer) Methods() []string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	methods := make([]string, 0, len(ms.methods))
	for name := range ms.methods {
		methods = append(methods, name)
	}
	return methods
}

// GetMethod returns full method info for a server transport method.
func (ms *ManagedServer) GetMethod(name string) *ServerMethodInfo {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.methods[name]
}

// GetAllMethods returns all server method info.
func (ms *ManagedServer) GetAllMethods() []*ServerMethodInfo {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	methods := make([]*ServerMethodInfo, 0, len(ms.methods))
	for _, method := range ms.methods {
		methods = append(methods, method)
	}
	return methods
}

// Dial is not supported for server transports.
func (ms *ManagedServer) Dial(ctx context.Context, address string) (net.Conn, error) {
	return nil, fmt.Errorf("Dial not supported for server transports")
}

// Close terminates the PT server process and all listeners.
func (ms *ManagedServer) Close() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if !ms.running {
		return nil
	}

	// Close all listeners
	for name, listener := range ms.listeners {
		if listener != nil {
			listener.Close()
			ms.log.Debug("Closed PT listener", "method", name)
		}
	}
	ms.listeners = make(map[string]net.Listener)

	ms.running = false

	if ms.cmd != nil && ms.cmd.Process != nil {
		ms.cmd.Process.Kill()
		ms.cmd.Wait()
		ms.log.Info("PT server process terminated", "binary", ms.config.BinaryPath)
	}

	return nil
}

// ptServerListener wraps a PT server's listening address.
// Note: The actual listening is done by the PT binary.
// This is a placeholder for future implementation that would properly integrate
// with the PT's Extended ORPort or direct TCP listener.
type ptServerListener struct {
	addr   string
	ctx    context.Context
	method string
	log    *logger.Logger
	closed bool
	mu     sync.Mutex
}

// Accept waits for and returns the next connection to the listener.
// This is a simplified implementation; production use would require
// proper integration with the PT's connection forwarding mechanism.
func (l *ptServerListener) Accept() (net.Conn, error) {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil, fmt.Errorf("listener closed")
	}
	l.mu.Unlock()

	// In a real implementation, this would:
	// 1. Accept connections from the PT binary's listener
	// 2. Unwrap the PT protocol to get the raw OR connection
	// 3. Return the connection for Tor processing
	//
	// For now, return an error indicating this needs external integration
	return nil, fmt.Errorf("PT server Accept requires external integration with bridge relay")
}

// Close closes the listener.
func (l *ptServerListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
	return nil
}

// Addr returns the listener's network address.
func (l *ptServerListener) Addr() net.Addr {
	addr, _ := net.ResolveTCPAddr("tcp", l.addr)
	return addr
}

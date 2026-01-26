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

// ManagedClient represents a client-side pluggable transport managed as an external process.
// It implements the PT version 2 IPC protocol per pt-spec.txt.
type ManagedClient struct {
	config TransportConfig
	cmd    *exec.Cmd
	stdout io.ReadCloser
	stderr io.ReadCloser

	methods map[string]*MethodInfo
	mu      sync.RWMutex
	running bool

	log *logger.Logger
}

// NewManagedClient creates a new managed PT client.
func NewManagedClient(config TransportConfig) (*ManagedClient, error) {
	if config.BinaryPath == "" {
		return nil, errors.New(errors.CategoryConfiguration, errors.SeverityHigh, "PT binary path is required")
	}

	if config.StateDir == "" {
		config.StateDir = os.TempDir() + "/go-tor-pt"
	}

	return &ManagedClient{
		config:  config,
		methods: make(map[string]*MethodInfo),
		log:     logger.New(slog.LevelInfo, os.Stdout),
	}, nil
}

// Name returns the transport name (derived from binary name).
func (mc *ManagedClient) Name() string {
	name := filepath.Base(mc.config.BinaryPath)
	return strings.TrimSuffix(name, ".exe")
}

// Start launches the PT process and performs IPC handshake.
func (mc *ManagedClient) Start(ctx context.Context) error {
	mc.mu.Lock()

	if mc.running {
		mc.mu.Unlock()
		return nil
	}

	if err := os.MkdirAll(mc.config.StateDir, 0o700); err != nil {
		mc.mu.Unlock()
		return errors.Wrap(errors.CategoryNetwork, errors.SeverityHigh, "failed to create state directory", err)
	}

	mc.cmd = exec.CommandContext(ctx, mc.config.BinaryPath)
	mc.cmd.Env = mc.buildEnvironment()

	stdout, err := mc.cmd.StdoutPipe()
	if err != nil {
		mc.mu.Unlock()
		return errors.Wrap(errors.CategoryNetwork, errors.SeverityHigh, "failed to create stdout pipe", err)
	}
	mc.stdout = stdout

	stderr, err := mc.cmd.StderrPipe()
	if err != nil {
		mc.mu.Unlock()
		return errors.Wrap(errors.CategoryNetwork, errors.SeverityHigh, "failed to create stderr pipe", err)
	}
	mc.stderr = stderr

	if err := mc.cmd.Start(); err != nil {
		mc.mu.Unlock()
		return errors.Wrap(errors.CategoryNetwork, errors.SeverityHigh, "failed to start PT process", err)
	}

	mc.running = true
	mc.log.Info("PT process started", "binary", mc.config.BinaryPath, "pid", mc.cmd.Process.Pid)

	// Unlock before handshake to avoid deadlock when parseCMethod tries to acquire lock
	mc.mu.Unlock()

	go mc.readStderr()

	if err := mc.performHandshake(ctx); err != nil {
		mc.mu.Lock()
		mc.cmd.Process.Kill()
		mc.running = false
		mc.mu.Unlock()
		return errors.Wrap(errors.CategoryProtocol, errors.SeverityHigh, "PT handshake failed", err)
	}

	return nil
}

// buildEnvironment constructs the environment variables for the PT process per pt-spec.txt.
func (mc *ManagedClient) buildEnvironment() []string {
	env := []string{
		"TOR_PT_MANAGED_TRANSPORT_VER=1",
		"TOR_PT_STATE_LOCATION=" + mc.config.StateDir,
		"TOR_PT_CLIENT_TRANSPORTS=*", // Request all available transports
	}

	if mc.config.ProxyURL != "" {
		env = append(env, "TOR_PT_PROXY="+mc.config.ProxyURL)
	}

	for key, value := range mc.config.Options {
		env = append(env, fmt.Sprintf("TOR_PT_OPT_%s=%s", strings.ToUpper(key), value))
	}

	return append(os.Environ(), env...)
}

// performHandshake reads the PT's stdout and parses CMETHOD lines.
func (mc *ManagedClient) performHandshake(ctx context.Context) error {
	scanner := bufio.NewScanner(mc.stdout)
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
		mc.log.Debug("PT stdout", "line", line)

		if strings.HasPrefix(line, "CMETHOD-ERROR") {
			return fmt.Errorf("PT reported error: %s", line)
		}

		if strings.HasPrefix(line, "CMETHOD") {
			if err := mc.parseCMethod(line); err != nil {
				mc.log.Warn("Failed to parse CMETHOD", "line", line, "error", err)
			}
		}

		if strings.HasPrefix(line, "CMETHODS DONE") {
			mc.log.Info("PT handshake complete", "methods", len(mc.methods))
			return nil
		}
	}

	return fmt.Errorf("PT handshake timeout")
}

// parseCMethod parses a CMETHOD line: CMETHOD <methodname> socks5 <address>
func (mc *ManagedClient) parseCMethod(line string) error {
	parts := strings.Fields(line)
	if len(parts) < 4 {
		return fmt.Errorf("invalid CMETHOD format: %s", line)
	}

	method := &MethodInfo{
		Name:    parts[1],
		Address: parts[3],
		Args:    make(map[string]string),
		Options: make(map[string]string),
	}

	socksType := parts[2]
	switch socksType {
	case "socks4":
		method.SOCKSVersion = 4
	case "socks5":
		method.SOCKSVersion = 5
	default:
		return fmt.Errorf("unsupported SOCKS version: %s", socksType)
	}

	for i := 4; i < len(parts); i++ {
		kv := strings.SplitN(parts[i], "=", 2)
		if len(kv) == 2 {
			method.Args[kv[0]] = kv[1]
		}
	}

	mc.mu.Lock()
	mc.methods[method.Name] = method
	mc.mu.Unlock()

	mc.log.Info("Registered PT method", "name", method.Name, "socks", method.SOCKSVersion, "address", method.Address)
	return nil
}

// readStderr consumes stderr output in background.
func (mc *ManagedClient) readStderr() {
	scanner := bufio.NewScanner(mc.stderr)
	for scanner.Scan() {
		mc.log.Debug("PT stderr", "line", scanner.Text())
	}
}

// Dial connects through the pluggable transport.
func (mc *ManagedClient) Dial(ctx context.Context, address string) (net.Conn, error) {
	mc.mu.RLock()
	if !mc.running || len(mc.methods) == 0 {
		mc.mu.RUnlock()
		return nil, fmt.Errorf("PT not ready")
	}

	var method *MethodInfo
	for _, m := range mc.methods {
		method = m
		break
	}
	mc.mu.RUnlock()

	dialer := &net.Dialer{}
	socksConn, err := dialer.DialContext(ctx, "tcp", method.Address)
	if err != nil {
		return nil, errors.Wrap(errors.CategoryNetwork, errors.SeverityHigh, "failed to connect to PT SOCKS", err)
	}

	if method.SOCKSVersion == 5 {
		if err := mc.socks5Handshake(socksConn, address); err != nil {
			socksConn.Close()
			return nil, err
		}
	}

	return socksConn, nil
}

// socks5Handshake performs SOCKS5 handshake to connect through PT.
func (mc *ManagedClient) socks5Handshake(conn net.Conn, address string) error {
	conn.Write([]byte{0x05, 0x01, 0x00})

	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return err
	}
	if resp[0] != 0x05 || resp[1] != 0x00 {
		return fmt.Errorf("SOCKS5 auth failed")
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}

	portNum := 0
	fmt.Sscanf(port, "%d", &portNum)

	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
	req = append(req, []byte(host)...)
	req = append(req, byte(portNum>>8), byte(portNum&0xff))

	if _, err := conn.Write(req); err != nil {
		return err
	}

	reply := make([]byte, 10)
	if _, err := io.ReadFull(conn, reply[:4]); err != nil {
		return err
	}
	if reply[1] != 0x00 {
		return fmt.Errorf("SOCKS5 connect failed: %d", reply[1])
	}

	return nil
}

// Methods returns available transport method names.
func (mc *ManagedClient) Methods() []string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	methods := make([]string, 0, len(mc.methods))
	for name := range mc.methods {
		methods = append(methods, name)
	}
	return methods
}

// GetMethod returns full method info for a transport method.
func (mc *ManagedClient) GetMethod(name string) *MethodInfo {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.methods[name]
}

// GetAllMethods returns all method info.
func (mc *ManagedClient) GetAllMethods() []*MethodInfo {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	methods := make([]*MethodInfo, 0, len(mc.methods))
	for _, method := range mc.methods {
		methods = append(methods, method)
	}
	return methods
}

// IsRunning reports whether the PT process is running.
func (mc *ManagedClient) IsRunning() bool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.running
}

// Close terminates the PT process.
func (mc *ManagedClient) Close() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if !mc.running {
		return nil
	}

	mc.running = false

	if mc.cmd != nil && mc.cmd.Process != nil {
		mc.cmd.Process.Kill()
		mc.cmd.Wait()
		mc.log.Info("PT process terminated", "binary", mc.config.BinaryPath)
	}

	return nil
}

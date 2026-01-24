// Package main demonstrates control protocol password authentication
package main

import (
	"bufio"
	"fmt"
	"net"
	"time"

	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/control"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// Example shows how to use control protocol with password authentication
func main() {
	// Create config with control password
	cfg := config.DefaultConfig()
	cfg.ControlPassword = "my-secret-password"
	cfg.ControlPort = 9051

	log := logger.NewDefault()

	// Create mock client info getter for demo
	mockClient := &mockClientGetter{
		activeCircuits: 5,
		socksPort:      9050,
		controlPort:    9051,
	}

	// Create control server with password
	server := control.NewServerWithPassword(
		fmt.Sprintf("127.0.0.1:%d", cfg.ControlPort),
		mockClient,
		cfg.ControlPassword,
		log,
	)

	// Start the server
	if err := server.Start(); err != nil {
		panic(err)
	}
	defer server.Stop()

	fmt.Printf("Control server started on port %d\n", cfg.ControlPort)
	fmt.Printf("Password authentication enabled\n")

	// Example client connection
	time.Sleep(100 * time.Millisecond)
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.ControlPort))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read greeting
	greeting, _ := reader.ReadString('\n')
	fmt.Printf("Server: %s", greeting)

	// Check PROTOCOLINFO
	writer.WriteString("PROTOCOLINFO\r\n")
	writer.Flush()
	for {
		line, _ := reader.ReadString('\n')
		fmt.Printf("Server: %s", line)
		if line[0:4] == "250 " {
			break
		}
	}

	// Authenticate with password
	writer.WriteString(fmt.Sprintf("AUTHENTICATE %s\r\n", cfg.ControlPassword))
	writer.Flush()
	response, _ := reader.ReadString('\n')
	fmt.Printf("Server: %s", response)

	// Now we can use authenticated commands
	writer.WriteString("GETINFO version\r\n")
	writer.Flush()
	response, _ = reader.ReadString('\n')
	fmt.Printf("Server: %s", response)

	fmt.Println("Authentication successful!")
}

// mockClientGetter implements control.ClientInfoGetter for demo
type mockClientGetter struct {
	activeCircuits      int
	socksPort           int
	controlPort         int
	circuitBuilds       int64
	circuitBuildSuccess int64
	circuitBuildFailure int64
	guardsActive        int
	guardsConfirmed     int
	uptimeSeconds       int64
	connectionAttempts  int64
	dataDir             string
}

func (m *mockClientGetter) GetStats() control.StatsProvider {
	return m
}

func (m *mockClientGetter) GetActiveCircuits() int {
	return m.activeCircuits
}

func (m *mockClientGetter) GetSocksPort() int {
	return m.socksPort
}

func (m *mockClientGetter) GetControlPort() int {
	return m.controlPort
}

func (m *mockClientGetter) GetCircuitBuilds() int64 {
	return m.circuitBuilds
}

func (m *mockClientGetter) GetCircuitBuildSuccess() int64 {
	return m.circuitBuildSuccess
}

func (m *mockClientGetter) GetCircuitBuildFailure() int64 {
	return m.circuitBuildFailure
}

func (m *mockClientGetter) GetGuardsActive() int {
	return m.guardsActive
}

func (m *mockClientGetter) GetGuardsConfirmed() int {
	return m.guardsConfirmed
}

func (m *mockClientGetter) GetUptimeSeconds() int64 {
	return m.uptimeSeconds
}

func (m *mockClientGetter) GetConnectionAttempts() int64 {
	return m.connectionAttempts
}

func (m *mockClientGetter) GetDataDir() string {
	return m.dataDir
}

func (m *mockClientGetter) GetConfig() control.ConfigProvider {
	return nil // No config needed for this demo
}

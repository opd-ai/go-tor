package control

import (
	"bufio"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestEventIntegration tests the complete event flow
func TestEventIntegration(t *testing.T) {
	// Create mock client
	mockClient := &mockClientGetter{
		activeCircuits: 0,
		socksPort:      9050,
		controlPort:    9051,
	}

	// Create server
	log := logger.NewDefault()
	server := NewServer("127.0.0.1:0", mockClient, log)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Get the actual address
	addr := server.listener.Addr().String()

	// Connect to server
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read greeting
	greeting, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read greeting: %v", err)
	}
	if !strings.HasPrefix(greeting, "250") {
		t.Errorf("Unexpected greeting: %s", greeting)
	}

	// Authenticate
	writer.WriteString("AUTHENTICATE\r\n")
	writer.Flush()
	authResp, _ := reader.ReadString('\n')
	if !strings.HasPrefix(authResp, "250") {
		t.Errorf("Authentication failed: %s", authResp)
	}

	// Subscribe to all events
	writer.WriteString("SETEVENTS CIRC STREAM BW ORCONN\r\n")
	writer.Flush()
	eventResp, _ := reader.ReadString('\n')
	if !strings.HasPrefix(eventResp, "250") {
		t.Errorf("Event subscription failed: %s", eventResp)
	}

	// Set up event collection
	var receivedEvents []string
	var eventsMu sync.Mutex
	eventChan := make(chan string, 10)

	// Start event reader goroutine
	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if strings.HasPrefix(line, "650 ") {
				eventChan <- strings.TrimSpace(line)
			}
		}
	}()

	// Wait a bit for setup
	time.Sleep(50 * time.Millisecond)

	// Publish circuit event
	server.GetEventDispatcher().Dispatch(&CircuitEvent{
		CircuitID: 100,
		Status:    "BUILT",
		Path:      "$ABC~NodeA,$DEF~NodeB,$GHI~NodeC",
		Purpose:   "GENERAL",
	})

	// Publish stream event
	server.GetEventDispatcher().Dispatch(&StreamEvent{
		StreamID:  200,
		Status:    "NEW",
		CircuitID: 100,
		Target:    "example.com:80",
	})

	// Publish bandwidth event
	server.GetEventDispatcher().Dispatch(&BWEvent{
		BytesRead:    1024,
		BytesWritten: 2048,
	})

	// Publish OR connection event
	server.GetEventDispatcher().Dispatch(&ORConnEvent{
		Target:   "127.0.0.1:9001",
		Status:   "CONNECTED",
		NumCircs: 1,
		ID:       12345,
	})

	// Collect events with timeout
	timeout := time.After(1 * time.Second)
	expectedEventCount := 4

eventLoop:
	for {
		select {
		case event := <-eventChan:
			eventsMu.Lock()
			receivedEvents = append(receivedEvents, event)
			if len(receivedEvents) >= expectedEventCount {
				eventsMu.Unlock()
				break eventLoop
			}
			eventsMu.Unlock()
		case <-timeout:
			t.Logf("Timeout waiting for events. Received %d events", len(receivedEvents))
			break eventLoop
		}
	}

	// Verify we received all events
	eventsMu.Lock()
	defer eventsMu.Unlock()

	if len(receivedEvents) < expectedEventCount {
		t.Errorf("Expected at least %d events, got %d", expectedEventCount, len(receivedEvents))
	}

	// Verify event content
	foundCirc := false
	foundStream := false
	foundBW := false
	foundORConn := false

	for _, event := range receivedEvents {
		t.Logf("Received event: %s", event)

		if strings.Contains(event, "CIRC 100 BUILT") {
			foundCirc = true
			if !strings.Contains(event, "$ABC~NodeA") {
				t.Error("Circuit event missing path information")
			}
		}
		if strings.Contains(event, "STREAM 200 NEW 100") {
			foundStream = true
			if !strings.Contains(event, "example.com:80") {
				t.Error("Stream event missing target information")
			}
		}
		if strings.Contains(event, "BW 1024 2048") {
			foundBW = true
		}
		if strings.Contains(event, "ORCONN 127.0.0.1:9001 CONNECTED") {
			foundORConn = true
		}
	}

	if !foundCirc {
		t.Error("Did not receive CIRC event")
	}
	if !foundStream {
		t.Error("Did not receive STREAM event")
	}
	if !foundBW {
		t.Error("Did not receive BW event")
	}
	if !foundORConn {
		t.Error("Did not receive ORCONN event")
	}
}

// TestEventFiltering tests that only subscribed events are received
func TestEventFiltering(t *testing.T) {
	mockClient := &mockClientGetter{
		activeCircuits: 0,
		socksPort:      9050,
		controlPort:    9051,
	}

	log := logger.NewDefault()
	server := NewServer("127.0.0.1:0", mockClient, log)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().String()

	// Connect
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read greeting
	reader.ReadString('\n')

	// Authenticate
	writer.WriteString("AUTHENTICATE\r\n")
	writer.Flush()
	reader.ReadString('\n')

	// Subscribe only to CIRC events
	writer.WriteString("SETEVENTS CIRC\r\n")
	writer.Flush()
	reader.ReadString('\n')

	// Set up event collection
	var receivedEvents []string
	var eventsMu sync.Mutex
	eventChan := make(chan string, 10)

	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if strings.HasPrefix(line, "650 ") {
				eventChan <- strings.TrimSpace(line)
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish various events
	server.GetEventDispatcher().Dispatch(&CircuitEvent{
		CircuitID: 100,
		Status:    "BUILT",
	})

	server.GetEventDispatcher().Dispatch(&StreamEvent{
		StreamID:  200,
		Status:    "NEW",
		CircuitID: 100,
		Target:    "example.com:80",
	})

	server.GetEventDispatcher().Dispatch(&BWEvent{
		BytesRead:    1024,
		BytesWritten: 2048,
	})

	// Wait for events
	timeout := time.After(500 * time.Millisecond)

eventLoop:
	for {
		select {
		case event := <-eventChan:
			eventsMu.Lock()
			receivedEvents = append(receivedEvents, event)
			eventsMu.Unlock()
		case <-timeout:
			break eventLoop
		}
	}

	// Should only receive CIRC event, not STREAM or BW
	eventsMu.Lock()
	defer eventsMu.Unlock()

	if len(receivedEvents) != 1 {
		t.Errorf("Expected exactly 1 event, got %d: %v", len(receivedEvents), receivedEvents)
	}

	if len(receivedEvents) > 0 && !strings.Contains(receivedEvents[0], "CIRC") {
		t.Errorf("Expected CIRC event, got: %s", receivedEvents[0])
	}
}

// TestMultipleSubscribers tests that multiple clients can subscribe to events
func TestMultipleSubscribers(t *testing.T) {
	mockClient := &mockClientGetter{
		activeCircuits: 0,
		socksPort:      9050,
		controlPort:    9051,
	}

	log := logger.NewDefault()
	server := NewServer("127.0.0.1:0", mockClient, log)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().String()

	// Create two connections
	conn1, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect 1: %v", err)
	}
	defer conn1.Close()

	conn2, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect 2: %v", err)
	}
	defer conn2.Close()

	// Set up both connections
	setupConnection := func(conn net.Conn) (*bufio.Reader, *bufio.Writer, chan string) {
		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)
		eventChan := make(chan string, 10)

		// Read greeting
		reader.ReadString('\n')

		// Authenticate
		writer.WriteString("AUTHENTICATE\r\n")
		writer.Flush()
		reader.ReadString('\n')

		// Subscribe to CIRC events
		writer.WriteString("SETEVENTS CIRC\r\n")
		writer.Flush()
		reader.ReadString('\n')

		// Start event reader
		go func() {
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if strings.HasPrefix(line, "650 ") {
					eventChan <- strings.TrimSpace(line)
				}
			}
		}()

		return reader, writer, eventChan
	}

	_, _, events1 := setupConnection(conn1)
	_, _, events2 := setupConnection(conn2)

	time.Sleep(50 * time.Millisecond)

	// Publish a circuit event
	server.GetEventDispatcher().Dispatch(&CircuitEvent{
		CircuitID: 100,
		Status:    "BUILT",
		Path:      "$ABC~NodeA,$DEF~NodeB,$GHI~NodeC",
	})

	// Both connections should receive the event
	timeout := time.After(1 * time.Second)
	var event1, event2 string

	select {
	case event1 = <-events1:
	case <-timeout:
		t.Fatal("Timeout waiting for event on connection 1")
	}

	select {
	case event2 = <-events2:
	case <-timeout:
		t.Fatal("Timeout waiting for event on connection 2")
	}

	// Both should have received the same event
	if !strings.Contains(event1, "CIRC 100 BUILT") {
		t.Errorf("Connection 1 got unexpected event: %s", event1)
	}
	if !strings.Contains(event2, "CIRC 100 BUILT") {
		t.Errorf("Connection 2 got unexpected event: %s", event2)
	}
}

// TestEventUnsubscribe tests that unsubscribing stops event delivery
func TestEventUnsubscribe(t *testing.T) {
	mockClient := &mockClientGetter{
		activeCircuits: 0,
		socksPort:      9050,
		controlPort:    9051,
	}

	log := logger.NewDefault()
	server := NewServer("127.0.0.1:0", mockClient, log)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().String()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	eventChan := make(chan string, 10)

	// Read greeting
	reader.ReadString('\n')

	// Authenticate
	writer.WriteString("AUTHENTICATE\r\n")
	writer.Flush()
	reader.ReadString('\n')

	// Subscribe to CIRC events
	writer.WriteString("SETEVENTS CIRC\r\n")
	writer.Flush()
	reader.ReadString('\n')

	// Start event reader
	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if strings.HasPrefix(line, "650 ") {
				eventChan <- strings.TrimSpace(line)
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish event - should be received
	server.GetEventDispatcher().Dispatch(&CircuitEvent{
		CircuitID: 100,
		Status:    "BUILT",
	})

	timeout := time.After(500 * time.Millisecond)
	select {
	case event := <-eventChan:
		if !strings.Contains(event, "CIRC 100") {
			t.Errorf("Unexpected event: %s", event)
		}
	case <-timeout:
		t.Fatal("Did not receive expected event")
	}

	// Unsubscribe by sending empty SETEVENTS
	writer.WriteString("SETEVENTS\r\n")
	writer.Flush()
	reader.ReadString('\n')

	time.Sleep(50 * time.Millisecond)

	// Publish another event - should NOT be received
	server.GetEventDispatcher().Dispatch(&CircuitEvent{
		CircuitID: 101,
		Status:    "BUILT",
	})

	// Wait and verify no event received
	timeout = time.After(300 * time.Millisecond)
	select {
	case event := <-eventChan:
		t.Errorf("Received unexpected event after unsubscribe: %s", event)
	case <-timeout:
		// Expected - no event should be received
	}
}

// BenchmarkEventDispatchMultipleSubscribers benchmarks event dispatch with many subscribers
func BenchmarkEventDispatchMultipleSubscribers(b *testing.B) {
	dispatcher := NewEventDispatcher()

	// Create 1000 mock connections with minimal initialization
	// The connections have nil conn/writer which is checked by Dispatch
	for i := 0; i < 1000; i++ {
		conn := &connection{
			events: make(map[string]bool),
			conn:   nil, // nil is fine - Dispatch checks before writing
		}
		dispatcher.Subscribe(conn, []EventType{EventCirc, EventStream, EventBW})
	}

	event := &CircuitEvent{
		CircuitID: 123,
		Status:    "BUILT",
		Path:      "$ABC~A,$DEF~B,$GHI~C",
		Purpose:   "GENERAL",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dispatcher.Dispatch(event)
	}
}

// BenchmarkEventFormatting benchmarks event formatting
func BenchmarkEventFormatting(b *testing.B) {
	event := &CircuitEvent{
		CircuitID:   123,
		Status:      "BUILT",
		Path:        "$ABC123~NodeA,$DEF456~NodeB,$GHI789~NodeC",
		BuildFlags:  "NEED_CAPACITY",
		Purpose:     "GENERAL",
		TimeCreated: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = event.Format()
	}
}

// TestNewEventTypesIntegration tests the new event types (NEWDESC, GUARD, NS)
func TestNewEventTypesIntegration(t *testing.T) {
	// Create mock client
	mockClient := &mockClientGetter{
		activeCircuits: 1,
		socksPort:      9050,
		controlPort:    9051,
	}

	// Create server
	log := logger.NewDefault()
	server := NewServer("127.0.0.1:0", mockClient, log)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Get the actual address
	addr := server.listener.Addr().String()

	// Connect to server
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read greeting
	greeting, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read greeting: %v", err)
	}
	if !strings.HasPrefix(greeting, "250") {
		t.Errorf("Unexpected greeting: %s", greeting)
	}

	// Authenticate
	writer.WriteString("AUTHENTICATE\r\n")
	writer.Flush()
	authResp, _ := reader.ReadString('\n')
	if !strings.HasPrefix(authResp, "250") {
		t.Errorf("Authentication failed: %s", authResp)
	}

	// Subscribe to new event types
	writer.WriteString("SETEVENTS NEWDESC GUARD NS\r\n")
	writer.Flush()
	eventResp, _ := reader.ReadString('\n')
	if !strings.HasPrefix(eventResp, "250") {
		t.Errorf("Event subscription failed: %s", eventResp)
	}

	// Set up event collection
	var receivedEvents []string
	var eventsMu sync.Mutex
	eventChan := make(chan string, 10)

	// Start event reader goroutine
	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if strings.HasPrefix(line, "650 ") {
				eventChan <- strings.TrimSpace(line)
			}
		}
	}()

	// Wait a bit for setup
	time.Sleep(50 * time.Millisecond)

	// Publish NEWDESC event
	server.GetEventDispatcher().Dispatch(&NewDescEvent{
		Descriptors: []string{"$ABC123~NodeA", "$DEF456~NodeB"},
	})

	// Publish GUARD event
	server.GetEventDispatcher().Dispatch(&GuardEvent{
		GuardType: "ENTRY",
		Name:      "$GHI789~GuardNode",
		Status:    "NEW",
	})

	// Publish NS event
	server.GetEventDispatcher().Dispatch(&NSEvent{
		LongName:    "$JKL012~ExitNode",
		Fingerprint: "$JKL012",
		Published:   "2024-01-01T12:00:00Z",
		IP:          "192.168.1.1",
		ORPort:      9001,
		DirPort:     9030,
		Flags:       []string{"Fast", "Exit", "Running", "Valid"},
	})

	// Collect events with timeout
	timeout := time.After(2 * time.Second)
	expectedEvents := 3
	for len(receivedEvents) < expectedEvents {
		select {
		case event := <-eventChan:
			eventsMu.Lock()
			receivedEvents = append(receivedEvents, event)
			eventsMu.Unlock()
		case <-timeout:
			t.Fatalf("Timeout waiting for events. Received %d/%d events", len(receivedEvents), expectedEvents)
		}
	}

	// Verify events
	eventsMu.Lock()
	defer eventsMu.Unlock()

	if len(receivedEvents) != expectedEvents {
		t.Errorf("Expected %d events, got %d", expectedEvents, len(receivedEvents))
	}

	// Check for NEWDESC event
	foundNewDesc := false
	for _, event := range receivedEvents {
		if strings.HasPrefix(event, "650 NEWDESC") && strings.Contains(event, "$ABC123~NodeA") {
			foundNewDesc = true
			break
		}
	}
	if !foundNewDesc {
		t.Errorf("NEWDESC event not received. Got: %v", receivedEvents)
	}

	// Check for GUARD event
	foundGuard := false
	for _, event := range receivedEvents {
		if strings.HasPrefix(event, "650 GUARD") && strings.Contains(event, "ENTRY") && strings.Contains(event, "$GHI789~GuardNode") {
			foundGuard = true
			break
		}
	}
	if !foundGuard {
		t.Errorf("GUARD event not received. Got: %v", receivedEvents)
	}

	// Check for NS event
	foundNS := false
	for _, event := range receivedEvents {
		if strings.HasPrefix(event, "650 NS") && strings.Contains(event, "$JKL012~ExitNode") && strings.Contains(event, "192.168.1.1") {
			foundNS = true
			break
		}
	}
	if !foundNS {
		t.Errorf("NS event not received. Got: %v", receivedEvents)
	}
}

// TestMixedEventSubscription tests subscribing to both old and new event types
func TestMixedEventSubscription(t *testing.T) {
	// Create mock client
	mockClient := &mockClientGetter{
		activeCircuits: 1,
		socksPort:      9050,
		controlPort:    9051,
	}

	// Create server
	log := logger.NewDefault()
	server := NewServer("127.0.0.1:0", mockClient, log)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Get the actual address
	addr := server.listener.Addr().String()

	// Connect to server
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read greeting
	greeting, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read greeting: %v", err)
	}
	if !strings.HasPrefix(greeting, "250") {
		t.Errorf("Unexpected greeting: %s", greeting)
	}

	// Authenticate
	writer.WriteString("AUTHENTICATE\r\n")
	writer.Flush()
	authResp, _ := reader.ReadString('\n')
	if !strings.HasPrefix(authResp, "250") {
		t.Errorf("Authentication failed: %s", authResp)
	}

	// Subscribe to mix of old and new event types
	writer.WriteString("SETEVENTS CIRC GUARD BW NEWDESC\r\n")
	writer.Flush()
	eventResp, _ := reader.ReadString('\n')
	if !strings.HasPrefix(eventResp, "250") {
		t.Errorf("Event subscription failed: %s", eventResp)
	}

	// Set up event collection
	eventChan := make(chan string, 10)

	// Start event reader goroutine
	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if strings.HasPrefix(line, "650 ") {
				eventChan <- strings.TrimSpace(line)
			}
		}
	}()

	// Wait a bit for setup
	time.Sleep(50 * time.Millisecond)

	// Publish old event (CIRC)
	server.GetEventDispatcher().Dispatch(&CircuitEvent{
		CircuitID: 100,
		Status:    "BUILT",
		Purpose:   "GENERAL",
	})

	// Publish new event (GUARD)
	server.GetEventDispatcher().Dispatch(&GuardEvent{
		GuardType: "ENTRY",
		Name:      "$ABC~Guard",
		Status:    "UP",
	})

	// Publish another old event (BW)
	server.GetEventDispatcher().Dispatch(&BWEvent{
		BytesRead:    1024,
		BytesWritten: 2048,
	})

	// Publish another new event (NEWDESC)
	server.GetEventDispatcher().Dispatch(&NewDescEvent{
		Descriptors: []string{"$XYZ~Relay"},
	})

	// Publish event that should NOT be received (NS - not subscribed)
	server.GetEventDispatcher().Dispatch(&NSEvent{
		LongName:    "$Test~Node",
		Fingerprint: "$Test",
		Published:   "2024-01-01T00:00:00Z",
		IP:          "1.2.3.4",
		ORPort:      9001,
		DirPort:     0,
		Flags:       []string{},
	})

	// Collect events
	var receivedEvents []string
	timeout := time.After(2 * time.Second)
	expectedEvents := 4 // CIRC, GUARD, BW, NEWDESC (not NS)

	// Collect events with timeout
	done := false
	for len(receivedEvents) < expectedEvents && !done {
		select {
		case event := <-eventChan:
			receivedEvents = append(receivedEvents, event)
		case <-timeout:
			// Timeout is OK - we might have received all we need
			done = true
		}
	}

	// Extra wait to ensure NS event doesn't arrive
	time.Sleep(100 * time.Millisecond)

	// Drain any extra events
	for {
		select {
		case event := <-eventChan:
			receivedEvents = append(receivedEvents, event)
		default:
			goto doneCollecting
		}
	}
doneCollecting:

	// Verify we got exactly the expected events
	if len(receivedEvents) != expectedEvents {
		t.Errorf("Expected %d events, got %d: %v", expectedEvents, len(receivedEvents), receivedEvents)
	}

	// Verify we got the expected event types
	eventTypes := make(map[string]bool)
	for _, event := range receivedEvents {
		if strings.HasPrefix(event, "650 CIRC") {
			eventTypes["CIRC"] = true
		} else if strings.HasPrefix(event, "650 GUARD") {
			eventTypes["GUARD"] = true
		} else if strings.HasPrefix(event, "650 BW") {
			eventTypes["BW"] = true
		} else if strings.HasPrefix(event, "650 NEWDESC") {
			eventTypes["NEWDESC"] = true
		} else if strings.HasPrefix(event, "650 NS") {
			t.Errorf("Received NS event which should not be subscribed: %s", event)
		}
	}

	// Verify all expected types present
	expectedTypes := []string{"CIRC", "GUARD", "BW", "NEWDESC"}
	for _, et := range expectedTypes {
		if !eventTypes[et] {
			t.Errorf("Missing expected event type: %s", et)
		}
	}
}

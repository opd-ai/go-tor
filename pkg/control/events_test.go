package control

import (
	"testing"
	"time"
)

func TestCircuitEventFormat(t *testing.T) {
	tests := []struct {
		name     string
		event    *CircuitEvent
		expected string
	}{
		{
			name: "basic circuit built event",
			event: &CircuitEvent{
				CircuitID: 123,
				Status:    "BUILT",
			},
			expected: "650 CIRC 123 BUILT",
		},
		{
			name: "circuit with path",
			event: &CircuitEvent{
				CircuitID: 456,
				Status:    "EXTENDED",
				Path:      "$ABC123~NodeA,$DEF456~NodeB",
			},
			expected: "650 CIRC 456 EXTENDED $ABC123~NodeA,$DEF456~NodeB",
		},
		{
			name: "circuit with purpose",
			event: &CircuitEvent{
				CircuitID: 789,
				Status:    "BUILT",
				Path:      "$ABC~A,$DEF~B,$GHI~C",
				Purpose:   "GENERAL",
			},
			expected: "650 CIRC 789 BUILT $ABC~A,$DEF~B,$GHI~C PURPOSE=GENERAL",
		},
		{
			name: "circuit with all fields",
			event: &CircuitEvent{
				CircuitID:   999,
				Status:      "BUILT",
				Path:        "$ABC~A,$DEF~B,$GHI~C",
				BuildFlags:  "NEED_CAPACITY",
				Purpose:     "GENERAL",
				TimeCreated: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			expected: "650 CIRC 999 BUILT $ABC~A,$DEF~B,$GHI~C BUILD_FLAGS=NEED_CAPACITY PURPOSE=GENERAL TIME_CREATED=2024-01-01T12:00:00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.event.Format()
			if result != tt.expected {
				t.Errorf("Format() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestStreamEventFormat(t *testing.T) {
	tests := []struct {
		name     string
		event    *StreamEvent
		expected string
	}{
		{
			name: "basic stream event",
			event: &StreamEvent{
				StreamID:  100,
				Status:    "NEW",
				CircuitID: 200,
				Target:    "example.com:80",
			},
			expected: "650 STREAM 100 NEW 200 example.com:80",
		},
		{
			name: "stream with reason",
			event: &StreamEvent{
				StreamID:  101,
				Status:    "FAILED",
				CircuitID: 201,
				Target:    "test.onion:443",
				Reason:    "TIMEOUT",
			},
			expected: "650 STREAM 101 FAILED 201 test.onion:443 REASON=TIMEOUT",
		},
		{
			name: "stream succeeded",
			event: &StreamEvent{
				StreamID:  102,
				Status:    "SUCCEEDED",
				CircuitID: 202,
				Target:    "example.org:443",
			},
			expected: "650 STREAM 102 SUCCEEDED 202 example.org:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.event.Format()
			if result != tt.expected {
				t.Errorf("Format() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBWEventFormat(t *testing.T) {
	tests := []struct {
		name     string
		event    *BWEvent
		expected string
	}{
		{
			name: "zero bandwidth",
			event: &BWEvent{
				BytesRead:    0,
				BytesWritten: 0,
			},
			expected: "650 BW 0 0",
		},
		{
			name: "non-zero bandwidth",
			event: &BWEvent{
				BytesRead:    1024,
				BytesWritten: 2048,
			},
			expected: "650 BW 1024 2048",
		},
		{
			name: "large bandwidth",
			event: &BWEvent{
				BytesRead:    1073741824, // 1GB
				BytesWritten: 2147483648, // 2GB
			},
			expected: "650 BW 1073741824 2147483648",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.event.Format()
			if result != tt.expected {
				t.Errorf("Format() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestORConnEventFormat(t *testing.T) {
	tests := []struct {
		name     string
		event    *ORConnEvent
		expected string
	}{
		{
			name: "basic connection",
			event: &ORConnEvent{
				Target: "127.0.0.1:9001",
				Status: "CONNECTED",
			},
			expected: "650 ORCONN 127.0.0.1:9001 CONNECTED",
		},
		{
			name: "connection with reason",
			event: &ORConnEvent{
				Target: "192.168.1.1:9001",
				Status: "FAILED",
				Reason: "TIMEOUT",
			},
			expected: "650 ORCONN 192.168.1.1:9001 FAILED REASON=TIMEOUT",
		},
		{
			name: "connection with circuits",
			event: &ORConnEvent{
				Target:   "10.0.0.1:9001",
				Status:   "CONNECTED",
				NumCircs: 5,
				ID:       12345,
			},
			expected: "650 ORCONN 10.0.0.1:9001 CONNECTED NCIRCS=5 ID=12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.event.Format()
			if result != tt.expected {
				t.Errorf("Format() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestEventDispatcher(t *testing.T) {
	dispatcher := NewEventDispatcher()

	// Create mock connections
	conn1 := &connection{
		events: make(map[string]bool),
	}
	conn2 := &connection{
		events: make(map[string]bool),
	}

	// Subscribe conn1 to CIRC events
	dispatcher.Subscribe(conn1, []EventType{EventCirc})

	// Subscribe conn2 to STREAM and BW events
	dispatcher.Subscribe(conn2, []EventType{EventStream, EventBW})

	// Test subscriber counts
	if count := dispatcher.GetSubscriberCount(EventCirc); count != 1 {
		t.Errorf("GetSubscriberCount(EventCirc) = %d, want 1", count)
	}
	if count := dispatcher.GetSubscriberCount(EventStream); count != 1 {
		t.Errorf("GetSubscriberCount(EventStream) = %d, want 1", count)
	}
	if count := dispatcher.GetSubscriberCount(EventBW); count != 1 {
		t.Errorf("GetSubscriberCount(EventBW) = %d, want 1", count)
	}
	if count := dispatcher.GetSubscriberCount(EventORConn); count != 0 {
		t.Errorf("GetSubscriberCount(EventORConn) = %d, want 0", count)
	}

	// Update conn1 subscription to include BW
	dispatcher.Subscribe(conn1, []EventType{EventCirc, EventBW})

	// Now both should be subscribed to BW
	if count := dispatcher.GetSubscriberCount(EventBW); count != 2 {
		t.Errorf("GetSubscriberCount(EventBW) after update = %d, want 2", count)
	}

	// Unsubscribe conn2
	dispatcher.Unsubscribe(conn2)

	// Check counts after unsubscribe
	if count := dispatcher.GetSubscriberCount(EventStream); count != 0 {
		t.Errorf("GetSubscriberCount(EventStream) after unsubscribe = %d, want 0", count)
	}
	if count := dispatcher.GetSubscriberCount(EventBW); count != 1 {
		t.Errorf("GetSubscriberCount(EventBW) after unsubscribe = %d, want 1", count)
	}
}

func TestEventDispatcherConcurrent(t *testing.T) {
	dispatcher := NewEventDispatcher()

	// Create multiple connections
	conns := make([]*connection, 10)
	for i := range conns {
		conns[i] = &connection{
			events: make(map[string]bool),
		}
		// Subscribe to different events
		if i%2 == 0 {
			dispatcher.Subscribe(conns[i], []EventType{EventCirc})
		} else {
			dispatcher.Subscribe(conns[i], []EventType{EventBW})
		}
	}

	// Dispatch events concurrently
	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			dispatcher.Dispatch(&CircuitEvent{
				CircuitID: uint32(i),
				Status:    "BUILT",
			})
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			dispatcher.Dispatch(&BWEvent{
				BytesRead:    uint64(i),
				BytesWritten: uint64(i * 2),
			})
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Verify subscriber counts
	if count := dispatcher.GetSubscriberCount(EventCirc); count != 5 {
		t.Errorf("GetSubscriberCount(EventCirc) = %d, want 5", count)
	}
	if count := dispatcher.GetSubscriberCount(EventBW); count != 5 {
		t.Errorf("GetSubscriberCount(EventBW) = %d, want 5", count)
	}
}

func TestEventTypes(t *testing.T) {
	tests := []struct {
		event    Event
		expected EventType
	}{
		{&CircuitEvent{}, EventCirc},
		{&StreamEvent{}, EventStream},
		{&BWEvent{}, EventBW},
		{&ORConnEvent{}, EventORConn},
		{&NewDescEvent{}, EventNewDesc},
		{&GuardEvent{}, EventGuard},
		{&NSEvent{}, EventNS},
	}

	for _, tt := range tests {
		t.Run(string(tt.expected), func(t *testing.T) {
			if result := tt.event.Type(); result != tt.expected {
				t.Errorf("Type() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNewDescEventFormat(t *testing.T) {
	tests := []struct {
		name     string
		event    *NewDescEvent
		expected string
	}{
		{
			name:     "empty descriptors",
			event:    &NewDescEvent{},
			expected: "650 NEWDESC",
		},
		{
			name: "single descriptor",
			event: &NewDescEvent{
				Descriptors: []string{"$ABC123~NodeA"},
			},
			expected: "650 NEWDESC $ABC123~NodeA",
		},
		{
			name: "multiple descriptors",
			event: &NewDescEvent{
				Descriptors: []string{"$ABC123~NodeA", "$DEF456~NodeB", "$GHI789~NodeC"},
			},
			expected: "650 NEWDESC $ABC123~NodeA $DEF456~NodeB $GHI789~NodeC",
		},
		{
			name: "fingerprints only",
			event: &NewDescEvent{
				Descriptors: []string{"$ABC123", "$DEF456"},
			},
			expected: "650 NEWDESC $ABC123 $DEF456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.event.Format()
			if result != tt.expected {
				t.Errorf("Format() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGuardEventFormat(t *testing.T) {
	tests := []struct {
		name     string
		event    *GuardEvent
		expected string
	}{
		{
			name: "guard new",
			event: &GuardEvent{
				GuardType: "ENTRY",
				Name:      "$ABC123~GuardNode",
				Status:    "NEW",
			},
			expected: "650 GUARD ENTRY $ABC123~GuardNode NEW",
		},
		{
			name: "guard up",
			event: &GuardEvent{
				GuardType: "ENTRY",
				Name:      "$DEF456~MyGuard",
				Status:    "UP",
			},
			expected: "650 GUARD ENTRY $DEF456~MyGuard UP",
		},
		{
			name: "guard down",
			event: &GuardEvent{
				GuardType: "ENTRY",
				Name:      "$GHI789~DownGuard",
				Status:    "DOWN",
			},
			expected: "650 GUARD ENTRY $GHI789~DownGuard DOWN",
		},
		{
			name: "guard dropped",
			event: &GuardEvent{
				GuardType: "ENTRY",
				Name:      "OldGuard",
				Status:    "DROPPED",
			},
			expected: "650 GUARD ENTRY OldGuard DROPPED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.event.Format()
			if result != tt.expected {
				t.Errorf("Format() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestNSEventFormat(t *testing.T) {
	tests := []struct {
		name     string
		event    *NSEvent
		expected string
	}{
		{
			name: "basic NS event",
			event: &NSEvent{
				LongName:    "$ABC123~NodeA",
				Fingerprint: "$ABC123",
				Published:   "2024-01-01T12:00:00Z",
				IP:          "192.168.1.1",
				ORPort:      9001,
				DirPort:     9030,
				Flags:       []string{},
			},
			expected: "650 NS $ABC123~NodeA $ABC123 2024-01-01T12:00:00Z 192.168.1.1 9001 9030 ",
		},
		{
			name: "NS event with flags",
			event: &NSEvent{
				LongName:    "$DEF456~GuardNode",
				Fingerprint: "$DEF456",
				Published:   "2024-01-02T13:00:00Z",
				IP:          "10.0.0.1",
				ORPort:      443,
				DirPort:     80,
				Flags:       []string{"Fast", "Guard", "Running", "Stable", "Valid"},
			},
			expected: "650 NS $DEF456~GuardNode $DEF456 2024-01-02T13:00:00Z 10.0.0.1 443 80 Fast Guard Running Stable Valid",
		},
		{
			name: "NS event exit node",
			event: &NSEvent{
				LongName:    "$GHI789~ExitNode",
				Fingerprint: "$GHI789",
				Published:   "2024-01-03T14:00:00Z",
				IP:          "172.16.0.1",
				ORPort:      9001,
				DirPort:     0,
				Flags:       []string{"Exit", "Fast", "Running", "Valid"},
			},
			expected: "650 NS $GHI789~ExitNode $GHI789 2024-01-03T14:00:00Z 172.16.0.1 9001 0 Exit Fast Running Valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.event.Format()
			if result != tt.expected {
				t.Errorf("Format() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func BenchmarkCircuitEventFormat(b *testing.B) {
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

func BenchmarkEventDispatch(b *testing.B) {
	dispatcher := NewEventDispatcher()

	// Create 100 connections subscribed to CIRC events
	for i := 0; i < 100; i++ {
		conn := &connection{
			events: make(map[string]bool),
		}
		dispatcher.Subscribe(conn, []EventType{EventCirc})
	}

	event := &CircuitEvent{
		CircuitID: 123,
		Status:    "BUILT",
		Path:      "$ABC~A,$DEF~B,$GHI~C",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dispatcher.Dispatch(event)
	}
}

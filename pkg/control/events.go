// Package control - Event notification system
package control

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// EventType represents the type of event
type EventType string

const (
	// EventCirc indicates circuit status changes
	EventCirc EventType = "CIRC"
	// EventStream indicates stream status changes
	EventStream EventType = "STREAM"
	// EventBW indicates bandwidth usage updates
	EventBW EventType = "BW"
	// EventORConn indicates OR connection status changes
	EventORConn EventType = "ORCONN"
	// EventNewDesc indicates new descriptor availability
	EventNewDesc EventType = "NEWDESC"
	// EventGuard indicates guard status changes
	EventGuard EventType = "GUARD"
)

// Event represents a control protocol event
type Event interface {
	Type() EventType
	Format() string
}

// CircuitEvent represents a circuit status change event
// Format: 650 CIRC <CircuitID> <Status> [<Path>] [BUILD_FLAGS=<Flags>] [PURPOSE=<Purpose>] [HS_STATE=<State>] [REND_QUERY=<Query>] [TIME_CREATED=<Time>]
type CircuitEvent struct {
	CircuitID   uint32
	Status      string // LAUNCHED, BUILT, EXTENDED, FAILED, CLOSED
	Path        string // $fingerprint1~nickname1,$fingerprint2~nickname2,...
	BuildFlags  string
	Purpose     string
	TimeCreated time.Time
}

// Type returns the event type
func (e *CircuitEvent) Type() EventType {
	return EventCirc
}

// Format formats the event for transmission
func (e *CircuitEvent) Format() string {
	parts := []string{
		fmt.Sprintf("650 CIRC %d %s", e.CircuitID, e.Status),
	}
	
	if e.Path != "" {
		parts = append(parts, e.Path)
	}
	
	if e.BuildFlags != "" {
		parts = append(parts, fmt.Sprintf("BUILD_FLAGS=%s", e.BuildFlags))
	}
	
	if e.Purpose != "" {
		parts = append(parts, fmt.Sprintf("PURPOSE=%s", e.Purpose))
	}
	
	if !e.TimeCreated.IsZero() {
		parts = append(parts, fmt.Sprintf("TIME_CREATED=%s", e.TimeCreated.Format(time.RFC3339)))
	}
	
	return strings.Join(parts, " ")
}

// StreamEvent represents a stream status change event
// Format: 650 STREAM <StreamID> <Status> <CircuitID> <Target>
type StreamEvent struct {
	StreamID  uint16
	Status    string // NEW, NEWRESOLVE, REMAP, SENTCONNECT, SENTRESOLVE, SUCCEEDED, FAILED, CLOSED, DETACHED
	CircuitID uint32
	Target    string // host:port
	Reason    string // Optional reason for FAILED/CLOSED
}

// Type returns the event type
func (e *StreamEvent) Type() EventType {
	return EventStream
}

// Format formats the event for transmission
func (e *StreamEvent) Format() string {
	parts := []string{
		fmt.Sprintf("650 STREAM %d %s %d %s", e.StreamID, e.Status, e.CircuitID, e.Target),
	}
	
	if e.Reason != "" {
		parts = append(parts, fmt.Sprintf("REASON=%s", e.Reason))
	}
	
	return strings.Join(parts, " ")
}

// BWEvent represents a bandwidth usage event
// Format: 650 BW <BytesRead> <BytesWritten>
type BWEvent struct {
	BytesRead    uint64
	BytesWritten uint64
}

// Type returns the event type
func (e *BWEvent) Type() EventType {
	return EventBW
}

// Format formats the event for transmission
func (e *BWEvent) Format() string {
	return fmt.Sprintf("650 BW %d %d", e.BytesRead, e.BytesWritten)
}

// ORConnEvent represents an OR connection status change event
// Format: 650 ORCONN <Target> <Status> [REASON=<Reason>] [NCIRCS=<NumCircuits>] [ID=<ID>]
type ORConnEvent struct {
	Target    string // address:port
	Status    string // NEW, LAUNCHED, CONNECTED, FAILED, CLOSED
	Reason    string // Optional reason
	NumCircs  int    // Number of circuits on this connection
	ID        uint64 // Connection ID
}

// Type returns the event type
func (e *ORConnEvent) Type() EventType {
	return EventORConn
}

// Format formats the event for transmission
func (e *ORConnEvent) Format() string {
	parts := []string{
		fmt.Sprintf("650 ORCONN %s %s", e.Target, e.Status),
	}
	
	if e.Reason != "" {
		parts = append(parts, fmt.Sprintf("REASON=%s", e.Reason))
	}
	
	if e.NumCircs > 0 {
		parts = append(parts, fmt.Sprintf("NCIRCS=%d", e.NumCircs))
	}
	
	if e.ID > 0 {
		parts = append(parts, fmt.Sprintf("ID=%d", e.ID))
	}
	
	return strings.Join(parts, " ")
}

// EventDispatcher manages event subscriptions and dispatching
type EventDispatcher struct {
	mu          sync.RWMutex
	subscribers map[*connection]map[EventType]bool
}

// NewEventDispatcher creates a new event dispatcher
func NewEventDispatcher() *EventDispatcher {
	return &EventDispatcher{
		subscribers: make(map[*connection]map[EventType]bool),
	}
}

// Subscribe subscribes a connection to specific event types
func (d *EventDispatcher) Subscribe(conn *connection, events []EventType) {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if d.subscribers[conn] == nil {
		d.subscribers[conn] = make(map[EventType]bool)
	}
	
	// Clear existing subscriptions for this connection
	d.subscribers[conn] = make(map[EventType]bool)
	
	// Add new subscriptions
	for _, event := range events {
		d.subscribers[conn][event] = true
	}
}

// Unsubscribe removes all subscriptions for a connection
func (d *EventDispatcher) Unsubscribe(conn *connection) {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	delete(d.subscribers, conn)
}

// Dispatch sends an event to all subscribed connections
func (d *EventDispatcher) Dispatch(event Event) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	eventType := event.Type()
	formatted := event.Format()
	
	for conn, subscriptions := range d.subscribers {
		if subscriptions[eventType] {
			// Send event asynchronously to avoid blocking
			go func(c *connection, msg string) {
				c.mu.Lock()
				defer c.mu.Unlock()
				
				// Check if connection is still valid
				if c.conn != nil {
					c.writer.WriteString(msg + "\r\n")
					c.writer.Flush()
				}
			}(conn, formatted)
		}
	}
}

// GetSubscriberCount returns the number of subscribers for an event type
func (d *EventDispatcher) GetSubscriberCount(eventType EventType) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	count := 0
	for _, subscriptions := range d.subscribers {
		if subscriptions[eventType] {
			count++
		}
	}
	return count
}

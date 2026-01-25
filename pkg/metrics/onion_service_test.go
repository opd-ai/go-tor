// Package metrics - Onion Service Metrics Tests (Phase 9.3.3)
package metrics

import (
	"testing"
	"time"
)

// TestOnionServiceMetrics tests all onion service metric operations
func TestOnionServiceMetrics(t *testing.T) {
	m := New()

	// Test stream metrics
	t.Run("stream_creation", func(t *testing.T) {
		initial := m.OnionServiceStreamsActive.Value()
		m.RecordOnionServiceStream(true)

		if got := m.OnionServiceStreamCreated.Value(); got != 1 {
			t.Errorf("StreamCreated = %d, want 1", got)
		}
		if got := m.OnionServiceStreamsActive.Value(); got != initial+1 {
			t.Errorf("StreamsActive = %d, want %d", got, initial+1)
		}
	})

	t.Run("stream_closure", func(t *testing.T) {
		active := m.OnionServiceStreamsActive.Value()
		m.RecordOnionServiceStream(false)

		if got := m.OnionServiceStreamClosed.Value(); got != 1 {
			t.Errorf("StreamClosed = %d, want 1", got)
		}
		if got := m.OnionServiceStreamsActive.Value(); got != active-1 {
			t.Errorf("StreamsActive = %d, want %d", got, active-1)
		}
	})

	t.Run("stream_data", func(t *testing.T) {
		m.RecordOnionServiceStreamData(1024)
		m.RecordOnionServiceStreamData(2048)

		if got := m.OnionServiceStreamData.Value(); got != 3072 {
			t.Errorf("StreamData = %d, want 3072", got)
		}
	})

	// Test descriptor publication metrics
	t.Run("descriptor_publish_success", func(t *testing.T) {
		m.RecordOnionServiceDescriptorPublish(true)

		if got := m.OnionServiceDescriptorPublished.Value(); got != 1 {
			t.Errorf("DescriptorPublished = %d, want 1", got)
		}
	})

	t.Run("descriptor_publish_failure", func(t *testing.T) {
		m.RecordOnionServiceDescriptorPublish(false)

		if got := m.OnionServiceDescriptorFailed.Value(); got != 1 {
			t.Errorf("DescriptorFailed = %d, want 1", got)
		}
	})

	// Test introduction point metrics
	t.Run("intro_establish_success", func(t *testing.T) {
		m.RecordOnionServiceIntroEstablish(true)

		if got := m.OnionServiceIntroEstablished.Value(); got != 1 {
			t.Errorf("IntroEstablished = %d, want 1", got)
		}
	})

	t.Run("intro_establish_failure", func(t *testing.T) {
		m.RecordOnionServiceIntroEstablish(false)

		if got := m.OnionServiceIntroFailed.Value(); got != 1 {
			t.Errorf("IntroFailed = %d, want 1", got)
		}
	})

	t.Run("intro_received", func(t *testing.T) {
		m.RecordOnionServiceIntroReceived()
		m.RecordOnionServiceIntroReceived()

		if got := m.OnionServiceIntroReceived.Value(); got != 2 {
			t.Errorf("IntroReceived = %d, want 2", got)
		}
	})

	t.Run("set_intro_points", func(t *testing.T) {
		m.SetOnionServiceIntroPoints(3)

		if got := m.OnionServiceActiveIntroPoints.Value(); got != 3 {
			t.Errorf("ActiveIntroPoints = %d, want 3", got)
		}
	})

	// Test rendezvous metrics
	t.Run("rendezvous_success", func(t *testing.T) {
		m.RecordOnionServiceRendezvous(true)

		if got := m.OnionServiceRendezvousSuccess.Value(); got != 1 {
			t.Errorf("RendezvousSuccess = %d, want 1", got)
		}
	})

	t.Run("rendezvous_failure", func(t *testing.T) {
		m.RecordOnionServiceRendezvous(false)

		if got := m.OnionServiceRendezvousFailed.Value(); got != 1 {
			t.Errorf("RendezvousFailed = %d, want 1", got)
		}
	})

	// Test service duration
	t.Run("service_duration", func(t *testing.T) {
		duration := 5 * time.Minute
		m.RecordOnionServiceDuration(duration)

		if count := m.OnionServiceActiveDuration.Count(); count != 1 {
			t.Errorf("Duration observations = %d, want 1", count)
		}

		if mean := m.OnionServiceActiveDuration.Mean(); mean != duration {
			t.Errorf("Duration mean = %v, want %v", mean, duration)
		}
	})
}

// TestOnionServiceMetricsIntegration tests realistic usage patterns
func TestOnionServiceMetricsIntegration(t *testing.T) {
	m := New()

	// Simulate service lifecycle
	// 1. Establish 3 intro points
	m.SetOnionServiceIntroPoints(3)
	m.RecordOnionServiceIntroEstablish(true)
	m.RecordOnionServiceIntroEstablish(true)
	m.RecordOnionServiceIntroEstablish(true)

	if got := m.OnionServiceIntroEstablished.Value(); got != 3 {
		t.Errorf("After establishing 3 intros: got %d, want 3", got)
	}

	// 2. Publish descriptor
	m.RecordOnionServiceDescriptorPublish(true)

	// 3. Receive introduction
	m.RecordOnionServiceIntroReceived()

	// 4. Complete rendezvous
	m.RecordOnionServiceRendezvous(true)

	// 5. Handle 2 streams
	m.RecordOnionServiceStream(true)
	m.RecordOnionServiceStreamData(1024)
	m.RecordOnionServiceStream(true)
	m.RecordOnionServiceStreamData(2048)

	if got := m.OnionServiceStreamsActive.Value(); got != 2 {
		t.Errorf("Active streams = %d, want 2", got)
	}

	// 6. Close one stream
	m.RecordOnionServiceStream(false)

	if got := m.OnionServiceStreamsActive.Value(); got != 1 {
		t.Errorf("Active streams after close = %d, want 1", got)
	}

	// 7. Record service lifetime
	m.RecordOnionServiceDuration(10 * time.Minute)

	// Verify final state
	if got := m.OnionServiceDescriptorPublished.Value(); got != 1 {
		t.Errorf("Descriptors published = %d, want 1", got)
	}
	if got := m.OnionServiceIntroReceived.Value(); got != 1 {
		t.Errorf("Intros received = %d, want 1", got)
	}
	if got := m.OnionServiceRendezvousSuccess.Value(); got != 1 {
		t.Errorf("Rendezvous successes = %d, want 1", got)
	}
	if got := m.OnionServiceStreamData.Value(); got != 3072 {
		t.Errorf("Stream data = %d, want 3072", got)
	}
}

// TestOnionServiceMetricsErrorCases tests error scenarios
func TestOnionServiceMetricsErrorCases(t *testing.T) {
	m := New()

	// Intro establishment failures
	m.RecordOnionServiceIntroEstablish(false)
	m.RecordOnionServiceIntroEstablish(false)

	if got := m.OnionServiceIntroFailed.Value(); got != 2 {
		t.Errorf("Intro failures = %d, want 2", got)
	}

	// Descriptor publication failures
	m.RecordOnionServiceDescriptorPublish(false)

	if got := m.OnionServiceDescriptorFailed.Value(); got != 1 {
		t.Errorf("Descriptor failures = %d, want 1", got)
	}

	// Rendezvous failures
	m.RecordOnionServiceRendezvous(false)
	m.RecordOnionServiceRendezvous(false)

	if got := m.OnionServiceRendezvousFailed.Value(); got != 2 {
		t.Errorf("Rendezvous failures = %d, want 2", got)
	}
}

// TestOnionServiceMetricsConcurrency tests concurrent metric updates
func TestOnionServiceMetricsConcurrency(t *testing.T) {
	m := New()

	// Simulate concurrent stream operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			m.RecordOnionServiceStream(true)
			m.RecordOnionServiceStreamData(100)
			m.RecordOnionServiceStream(false)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// All streams should be closed
	if got := m.OnionServiceStreamsActive.Value(); got != 0 {
		t.Errorf("Active streams = %d, want 0", got)
	}

	// Should have processed 10 streams
	if got := m.OnionServiceStreamCreated.Value(); got != 10 {
		t.Errorf("Streams created = %d, want 10", got)
	}
	if got := m.OnionServiceStreamClosed.Value(); got != 10 {
		t.Errorf("Streams closed = %d, want 10", got)
	}

	// Total data should be 1000 bytes
	if got := m.OnionServiceStreamData.Value(); got != 1000 {
		t.Errorf("Stream data = %d, want 1000", got)
	}
}

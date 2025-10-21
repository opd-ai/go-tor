package trace

import (
	"testing"
	"time"
)

func TestAlwaysSample(t *testing.T) {
	sampler := AlwaysSample()

	tests := []string{
		"operation1",
		"operation2",
		"operation3",
	}

	for _, name := range tests {
		if !sampler.ShouldSample(name) {
			t.Errorf("AlwaysSample should sample '%s'", name)
		}
	}
}

func TestNeverSample(t *testing.T) {
	sampler := NeverSample()

	tests := []string{
		"operation1",
		"operation2",
		"operation3",
	}

	for _, name := range tests {
		if sampler.ShouldSample(name) {
			t.Errorf("NeverSample should not sample '%s'", name)
		}
	}
}

func TestProbabilitySample(t *testing.T) {
	tests := []struct {
		name        string
		probability float64
		samples     int
		tolerance   float64
	}{
		{"always", 1.0, 1000, 0.0},
		{"never", 0.0, 1000, 0.0},
		{"half", 0.5, 1000, 0.1},
		{"quarter", 0.25, 1000, 0.1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampler := ProbabilitySample(tt.probability)

			sampled := 0
			for i := 0; i < tt.samples; i++ {
				if sampler.ShouldSample("test-operation") {
					sampled++
				}
			}

			actualRate := float64(sampled) / float64(tt.samples)
			diff := actualRate - tt.probability
			if diff < 0 {
				diff = -diff
			}

			if diff > tt.tolerance {
				t.Errorf("Expected sample rate ~%f, got %f (diff: %f)", tt.probability, actualRate, diff)
			}
		})
	}
}

func TestProbabilitySampleBounds(t *testing.T) {
	// Test negative probability
	sampler := ProbabilitySample(-0.5)
	sampled := 0
	for i := 0; i < 100; i++ {
		if sampler.ShouldSample("test") {
			sampled++
		}
	}
	if sampled != 0 {
		t.Error("Negative probability should never sample")
	}

	// Test probability > 1
	sampler = ProbabilitySample(1.5)
	sampled = 0
	for i := 0; i < 100; i++ {
		if sampler.ShouldSample("test") {
			sampled++
		}
	}
	if sampled != 100 {
		t.Error("Probability > 1 should always sample")
	}
}

func TestRateLimitSample(t *testing.T) {
	maxPerSecond := 10
	sampler := RateLimitSample(maxPerSecond)

	// Sample rapidly
	sampled := 0
	for i := 0; i < 100; i++ {
		if sampler.ShouldSample("test-operation") {
			sampled++
		}
	}

	if sampled > maxPerSecond {
		t.Errorf("Expected at most %d samples in first burst, got %d", maxPerSecond, sampled)
	}

	// Wait for reset and try again
	time.Sleep(1100 * time.Millisecond)

	sampled = 0
	for i := 0; i < 100; i++ {
		if sampler.ShouldSample("test-operation") {
			sampled++
		}
	}

	if sampled > maxPerSecond {
		t.Errorf("Expected at most %d samples after reset, got %d", maxPerSecond, sampled)
	}
}

func TestRateLimitSampleMultipleSeconds(t *testing.T) {
	maxPerSecond := 5
	sampler := RateLimitSample(maxPerSecond)

	totalSampled := 0

	// Test over 3 seconds
	for sec := 0; sec < 3; sec++ {
		sampled := 0
		for i := 0; i < 20; i++ {
			if sampler.ShouldSample("test-operation") {
				sampled++
				totalSampled++
			}
			time.Sleep(10 * time.Millisecond)
		}

		if sampled > maxPerSecond {
			t.Errorf("Second %d: expected at most %d samples, got %d", sec, maxPerSecond, sampled)
		}

		// Wait for next second
		time.Sleep(time.Second - 200*time.Millisecond)
	}
}

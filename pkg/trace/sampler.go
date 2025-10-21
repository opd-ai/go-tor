package trace

import (
	"math/rand"
	"sync"
	"time"
)

// alwaysSampler always samples
type alwaysSampler struct{}

func (s *alwaysSampler) ShouldSample(name string) bool {
	return true
}

// AlwaysSample returns a sampler that samples all traces
func AlwaysSample() Sampler {
	return &alwaysSampler{}
}

// neverSampler never samples
type neverSampler struct{}

func (s *neverSampler) ShouldSample(name string) bool {
	return false
}

// NeverSample returns a sampler that never samples
func NeverSample() Sampler {
	return &neverSampler{}
}

// probabilitySampler samples based on probability
type probabilitySampler struct {
	probability float64
	mu          sync.Mutex
	rng         *rand.Rand
}

func (s *probabilitySampler) ShouldSample(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rng.Float64() < s.probability
}

// ProbabilitySample returns a sampler that samples based on probability (0.0-1.0)
func ProbabilitySample(probability float64) Sampler {
	if probability < 0 {
		probability = 0
	}
	if probability > 1 {
		probability = 1
	}
	return &probabilitySampler{
		probability: probability,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// rateLimitSampler samples with rate limiting
type rateLimitSampler struct {
	maxPerSecond int
	count        int
	resetTime    time.Time
	mu           sync.Mutex
}

func (s *rateLimitSampler) ShouldSample(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if now.Sub(s.resetTime) >= time.Second {
		s.count = 0
		s.resetTime = now
	}

	if s.count < s.maxPerSecond {
		s.count++
		return true
	}

	return false
}

// RateLimitSample returns a sampler that limits sampling rate to N per second
func RateLimitSample(maxPerSecond int) Sampler {
	return &rateLimitSampler{
		maxPerSecond: maxPerSecond,
		resetTime:    time.Now(),
	}
}

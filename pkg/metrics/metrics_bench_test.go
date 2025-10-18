package metrics

import (
	"testing"
	"time"
)

// BenchmarkCounterInc benchmarks counter increment operations
func BenchmarkCounterInc(b *testing.B) {
	c := NewCounter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Inc()
	}
}

// BenchmarkCounterAdd benchmarks counter add operations
func BenchmarkCounterAdd(b *testing.B) {
	c := NewCounter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Add(5)
	}
}

// BenchmarkCounterValue benchmarks counter value reads
func BenchmarkCounterValue(b *testing.B) {
	c := NewCounter()
	c.Add(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Value()
	}
}

// BenchmarkGaugeSet benchmarks gauge set operations
func BenchmarkGaugeSet(b *testing.B) {
	g := NewGauge()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Set(int64(i))
	}
}

// BenchmarkGaugeInc benchmarks gauge increment operations
func BenchmarkGaugeInc(b *testing.B) {
	g := NewGauge()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Inc()
	}
}

// BenchmarkHistogramObserve benchmarks histogram observations
func BenchmarkHistogramObserve(b *testing.B) {
	h := NewHistogram()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Observe(time.Duration(i) * time.Millisecond)
	}
}

// BenchmarkHistogramMean benchmarks histogram mean calculation
func BenchmarkHistogramMean(b *testing.B) {
	h := NewHistogram()
	// Pre-populate with data
	for i := 0; i < 100; i++ {
		h.Observe(time.Duration(i) * time.Millisecond)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.Mean()
	}
}

// BenchmarkHistogramPercentile benchmarks histogram percentile calculation
func BenchmarkHistogramPercentile(b *testing.B) {
	h := NewHistogram()
	// Pre-populate with data
	for i := 0; i < 100; i++ {
		h.Observe(time.Duration(i) * time.Millisecond)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.Percentile(0.95)
	}
}

// BenchmarkMetricsSnapshot benchmarks full metrics snapshot
func BenchmarkMetricsSnapshot(b *testing.B) {
	m := New()
	// Add some data
	m.RecordCircuitBuild(true, 2*time.Second)
	m.RecordCircuitBuild(false, 1*time.Second)
	m.RecordConnection(true, 1)
	m.ActiveCircuits.Set(3)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Snapshot()
	}
}

// BenchmarkCounterIncParallel benchmarks parallel counter increments
func BenchmarkCounterIncParallel(b *testing.B) {
	c := NewCounter()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Inc()
		}
	})
}

// BenchmarkGaugeSetParallel benchmarks parallel gauge sets
func BenchmarkGaugeSetParallel(b *testing.B) {
	g := NewGauge()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			g.Set(42)
		}
	})
}

// BenchmarkHistogramObserveParallel benchmarks parallel histogram observations
func BenchmarkHistogramObserveParallel(b *testing.B) {
	h := NewHistogram()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			h.Observe(100 * time.Millisecond)
		}
	})
}

// BenchmarkRecordCircuitBuild benchmarks recording circuit builds
func BenchmarkRecordCircuitBuild(b *testing.B) {
	m := New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordCircuitBuild(true, 2*time.Second)
	}
}

// BenchmarkRecordConnection benchmarks recording connections
func BenchmarkRecordConnection(b *testing.B) {
	m := New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordConnection(true, 2)
	}
}

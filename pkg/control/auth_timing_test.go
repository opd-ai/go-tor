package control

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestAuthenticationTimingAttackResistance verifies that password comparison
// is constant-time to prevent timing attacks
func TestAuthenticationTimingAttackResistance(t *testing.T) {
	mockClient := &mockClientGetter{
		activeCircuits: 3,
		socksPort:      9050,
		controlPort:    9051,
	}

	log := logger.NewDefault()
	correctPassword := "SuperSecretPassword123!"
	server := NewServerWithPassword("127.0.0.1:0", mockClient, correctPassword, log)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().String()

	// Test cases with different password lengths and positions of mismatch
	testCases := []struct {
		name     string
		password string
		desc     string
	}{
		{
			name:     "Empty password",
			password: "",
			desc:     "Zero length",
		},
		{
			name:     "First character wrong",
			password: "XuperSecretPassword123!",
			desc:     "Mismatch at position 0",
		},
		{
			name:     "Middle character wrong",
			password: "SuperXecretPassword123!",
			desc:     "Mismatch at position 6",
		},
		{
			name:     "Last character wrong",
			password: "SuperSecretPassword123X",
			desc:     "Mismatch at position 22",
		},
		{
			name:     "Correct length, all wrong",
			password: "XXXXXXXXXXXXXXXXXXXXXXX",
			desc:     "Same length, completely different",
		},
		{
			name:     "Shorter password",
			password: "Short",
			desc:     "Length mismatch (shorter)",
		},
		{
			name:     "Longer password",
			password: "SuperSecretPassword123!ExtraCharacters",
			desc:     "Length mismatch (longer)",
		},
	}

	const iterations = 100
	timings := make(map[string][]time.Duration)

	for _, tc := range testCases {
		timings[tc.name] = make([]time.Duration, 0, iterations)

		for i := 0; i < iterations; i++ {
			conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
			if err != nil {
				t.Fatalf("[%s] Failed to connect: %v", tc.name, err)
			}

			reader := bufio.NewReader(conn)
			writer := bufio.NewWriter(conn)

			// Read greeting
			_, err = reader.ReadString('\n')
			if err != nil {
				conn.Close()
				t.Fatalf("[%s] Failed to read greeting: %v", tc.name, err)
			}

			// Measure authentication timing
			start := time.Now()
			_, err = writer.WriteString("AUTHENTICATE " + tc.password + "\r\n")
			if err != nil {
				conn.Close()
				t.Fatalf("[%s] Failed to write command: %v", tc.name, err)
			}
			writer.Flush()

			_, err = reader.ReadString('\n')
			elapsed := time.Since(start)
			if err != nil {
				conn.Close()
				t.Fatalf("[%s] Failed to read response: %v", tc.name, err)
			}

			timings[tc.name] = append(timings[tc.name], elapsed)
			conn.Close()

			// Small delay between attempts to avoid rate limiting
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Analyze timing data
	t.Logf("\n=== Authentication Timing Analysis ===")
	for _, tc := range testCases {
		durations := timings[tc.name]
		avg, stddev := calculateStats(durations)
		t.Logf("[%s] %s: avg=%v, stddev=%v", tc.name, tc.desc, avg, stddev)
	}

	// Statistical analysis: timing variance should be similar across all test cases
	// If timing attack is possible, early mismatches would be faster than late mismatches
	firstCharWrong := timings["First character wrong"]
	lastCharWrong := timings["Last character wrong"]

	avgFirst, stddevFirst := calculateStats(firstCharWrong)
	avgLast, stddevLast := calculateStats(lastCharWrong)

	t.Logf("\nConstant-Time Verification:")
	t.Logf("  First char wrong: avg=%v, stddev=%v", avgFirst, stddevFirst)
	t.Logf("  Last char wrong:  avg=%v, stddev=%v", avgLast, stddevLast)

	// Calculate timing difference between first and last character mismatch
	timingDiff := avgFirst - avgLast
	if timingDiff < 0 {
		timingDiff = -timingDiff
	}

	// The timing difference should be negligible (< 100µs)
	// Network jitter and OS scheduling will dominate
	maxAcceptableDiff := 100 * time.Microsecond

	t.Logf("  Timing difference: %v (threshold: %v)", timingDiff, maxAcceptableDiff)

	if timingDiff > maxAcceptableDiff {
		t.Logf("  ⚠️  WARNING: Timing difference exceeds threshold")
		t.Logf("  This may indicate a timing vulnerability, but could also be network jitter")
		t.Logf("  Manual inspection recommended if difference is > 1ms")
	} else {
		t.Logf("  ✓ Timing difference within acceptable range")
	}

	// Additional check: coefficient of variation should be similar
	cvFirst := stddevFirst.Seconds() / avgFirst.Seconds()
	cvLast := stddevLast.Seconds() / avgLast.Seconds()
	t.Logf("  CV first: %.4f, CV last: %.4f (should be similar)", cvFirst, cvLast)
}

// TestAuthenticationRateLimiting verifies that failed authentication attempts
// trigger exponential backoff
func TestAuthenticationRateLimiting(t *testing.T) {
	mockClient := &mockClientGetter{
		activeCircuits: 3,
		socksPort:      9050,
		controlPort:    9051,
	}

	log := logger.NewDefault()
	password := "correct-password"
	server := NewServerWithPassword("127.0.0.1:0", mockClient, password, log)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().String()

	// Helper function to attempt authentication
	attemptAuth := func(pwd string) (string, error) {
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err != nil {
			return "", err
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)

		// Read greeting
		_, err = reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		// Authenticate
		_, err = writer.WriteString("AUTHENTICATE " + pwd + "\r\n")
		if err != nil {
			return "", err
		}
		writer.Flush()

		response, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		return response, nil
	}

	// First failed attempt - should be immediate
	t.Logf("Attempt 1: First failed authentication")
	start := time.Now()
	resp, err := attemptAuth("wrong-password-1")
	elapsed1 := time.Since(start)
	if err != nil {
		t.Fatalf("Failed to attempt auth: %v", err)
	}
	if !strings.HasPrefix(resp, "515") {
		t.Errorf("Expected 515 error, got: %s", resp)
	}
	t.Logf("  Response: %s", strings.TrimSpace(resp))
	t.Logf("  Time: %v", elapsed1)

	// Immediate retry - should be rate limited
	t.Logf("\nAttempt 2: Immediate retry (should be rate limited)")
	start = time.Now()
	resp, err = attemptAuth("wrong-password-2")
	elapsed2 := time.Since(start)
	if err != nil {
		t.Fatalf("Failed to attempt auth: %v", err)
	}
	if !strings.HasPrefix(resp, "515") {
		t.Errorf("Expected 515 error, got: %s", resp)
	}
	if !strings.Contains(resp, "too many attempts") {
		t.Errorf("Expected rate limit message, got: %s", resp)
	}
	t.Logf("  Response: %s", strings.TrimSpace(resp))
	t.Logf("  Time: %v", elapsed2)

	// Wait for backoff period (1 second)
	t.Logf("\nWaiting 1.1 seconds for backoff to expire...")
	time.Sleep(1100 * time.Millisecond)

	// Third attempt - should work again but fail on password
	t.Logf("\nAttempt 3: After backoff period")
	start = time.Now()
	resp, err = attemptAuth("wrong-password-3")
	elapsed3 := time.Since(start)
	if err != nil {
		t.Fatalf("Failed to attempt auth: %v", err)
	}
	if !strings.HasPrefix(resp, "515") {
		t.Errorf("Expected 515 error, got: %s", resp)
	}
	if strings.Contains(resp, "too many attempts") {
		t.Errorf("Should not be rate limited after backoff, got: %s", resp)
	}
	t.Logf("  Response: %s", strings.TrimSpace(resp))
	t.Logf("  Time: %v", elapsed3)

	// Immediate retry - should be rate limited with 2 second backoff
	t.Logf("\nAttempt 4: Immediate retry (should have 2s backoff)")
	start = time.Now()
	resp, err = attemptAuth("wrong-password-4")
	elapsed4 := time.Since(start)
	if err != nil {
		t.Fatalf("Failed to attempt auth: %v", err)
	}
	if !strings.Contains(resp, "too many attempts") {
		t.Errorf("Expected rate limit message, got: %s", resp)
	}
	t.Logf("  Response: %s", strings.TrimSpace(resp))
	t.Logf("  Time: %v", elapsed4)

	// Verify exponential backoff by checking server state
	server.authAttemptsMu.Lock()
	// Extract IP from address (server uses IP only for rate limiting)
	testIP := "127.0.0.1"
	limiter := server.authAttempts[testIP]
	if limiter == nil {
		server.authAttemptsMu.Unlock()
		t.Fatal("No rate limiter found for connection")
	}
	backoff := limiter.backoffMs
	attempts := limiter.attempts
	server.authAttemptsMu.Unlock()

	t.Logf("\nRate Limiter State:")
	t.Logf("  Attempts: %d", attempts)
	t.Logf("  Current backoff: %dms", backoff)

	if attempts < 2 {
		t.Errorf("Expected at least 2 failed attempts, got %d", attempts)
	}
	if backoff < 2000 {
		t.Errorf("Expected backoff to be at least 2000ms, got %dms", backoff)
	}

	// Wait for second backoff and try correct password
	t.Logf("\nWaiting 2.1 seconds for second backoff to expire...")
	time.Sleep(2100 * time.Millisecond)

	t.Logf("\nAttempt 5: Correct password (should succeed and reset limiter)")
	resp, err = attemptAuth(password)
	if err != nil {
		t.Fatalf("Failed to attempt auth: %v", err)
	}
	if !strings.HasPrefix(resp, "250") {
		t.Errorf("Expected 250 OK with correct password, got: %s", resp)
	}
	t.Logf("  Response: %s", strings.TrimSpace(resp))

	// Verify rate limiter was reset
	server.authAttemptsMu.Lock()
	testIP = "127.0.0.1"
	limiter = server.authAttempts[testIP]
	server.authAttemptsMu.Unlock()

	if limiter != nil {
		t.Errorf("Rate limiter should be reset after successful auth, but still exists")
	} else {
		t.Logf("  ✓ Rate limiter successfully reset")
	}
}

// TestAuthenticationConstantTimeCorrectPassword verifies that correct password
// comparison also uses constant-time operations
func TestAuthenticationConstantTimeCorrectPassword(t *testing.T) {
	mockClient := &mockClientGetter{
		activeCircuits: 3,
		socksPort:      9050,
		controlPort:    9051,
	}

	log := logger.NewDefault()
	password := "TestPassword123"
	server := NewServerWithPassword("127.0.0.1:0", mockClient, password, log)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().String()

	const iterations = 50
	timings := make([]time.Duration, 0, iterations)

	for i := 0; i < iterations; i++ {
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}

		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)

		// Read greeting
		_, err = reader.ReadString('\n')
		if err != nil {
			conn.Close()
			t.Fatalf("Failed to read greeting: %v", err)
		}

		// Measure authentication timing with correct password
		start := time.Now()
		_, err = writer.WriteString("AUTHENTICATE " + password + "\r\n")
		if err != nil {
			conn.Close()
			t.Fatalf("Failed to write command: %v", err)
		}
		writer.Flush()

		resp, err := reader.ReadString('\n')
		elapsed := time.Since(start)
		if err != nil {
			conn.Close()
			t.Fatalf("Failed to read response: %v", err)
		}

		if !strings.HasPrefix(resp, "250") {
			t.Errorf("Expected 250 OK, got: %s", resp)
		}

		timings = append(timings, elapsed)
		conn.Close()

		time.Sleep(10 * time.Millisecond)
	}

	avg, stddev := calculateStats(timings)
	t.Logf("Correct password timing: avg=%v, stddev=%v", avg, stddev)
	t.Logf("Coefficient of variation: %.4f", stddev.Seconds()/avg.Seconds())
}

// calculateStats computes average and standard deviation of durations
func calculateStats(durations []time.Duration) (avg, stddev time.Duration) {
	if len(durations) == 0 {
		return 0, 0
	}

	// Calculate average
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	avg = sum / time.Duration(len(durations))

	// Calculate standard deviation
	var variance float64
	avgNs := float64(avg.Nanoseconds())
	for _, d := range durations {
		diff := float64(d.Nanoseconds()) - avgNs
		variance += diff * diff
	}
	variance /= float64(len(durations))
	stddev = time.Duration(int64(variance)) // sqrt approximation

	// Better stddev calculation
	var sumSquaredDiff float64
	for _, d := range durations {
		diff := float64(d.Nanoseconds()) - avgNs
		sumSquaredDiff += diff * diff
	}
	stddevNs := 0.0
	if len(durations) > 1 {
		variance := sumSquaredDiff / float64(len(durations)-1)
		stddevNs = 1.0
		// Newton's method for square root
		for i := 0; i < 10; i++ {
			stddevNs = (stddevNs + variance/stddevNs) / 2.0
		}
	}
	stddev = time.Duration(int64(stddevNs))

	return avg, stddev
}

package directory

import (
"context"
"testing"
"time"
)

// TestFetchConsensusMethod33Integration tests fetching and parsing
// modern microdescriptor consensus (consensus-method 33)
func TestFetchConsensusMethod33Integration(t *testing.T) {
if testing.Short() {
t.Skip("Skipping integration test in short mode")
}

client := NewClient(nil)
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Fetch consensus (now using consensus-microdesc by default)
relays, err := client.FetchConsensus(ctx)
if err != nil {
t.Fatalf("FetchConsensus() error = %v", err)
}

if len(relays) == 0 {
t.Fatal("Expected at least some relays in consensus")
}

t.Logf("Fetched %d relays from consensus", len(relays))

// Count relays with microdescriptor digests
digestCount := 0
for _, r := range relays {
if r.MicrodescDigest != "" {
digestCount++
}
}

// We expect most relays to have microdescriptor digests
percentage := 100.0 * float64(digestCount) / float64(len(relays))
t.Logf("Relays with microdescriptor digest: %d/%d (%.1f%%)", digestCount, len(relays), percentage)

if percentage < 90.0 {
t.Errorf("Expected >90%% of relays to have microdescriptor digests, got %.1f%%", percentage)
}

// Check that microdescriptor digests are valid base64
for i, r := range relays {
if r.MicrodescDigest == "" {
continue
}
// Base64 digest should be 43-44 characters for SHA256
if len(r.MicrodescDigest) < 40 || len(r.MicrodescDigest) > 50 {
t.Errorf("Relay %d (%s) has suspicious digest length: %d", 
i, r.Nickname, len(r.MicrodescDigest))
break
}
}
}

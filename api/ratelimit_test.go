package api

import (
	"testing"
	"time"
)

// TestIPRateLimiterBurstThenThrottle verifies that a client gets `burst`
// immediate attempts and is then throttled until tokens refill.
func TestIPRateLimiterBurstThenThrottle(t *testing.T) {
	// 1 token/sec, burst of 3.
	l := newIPRateLimiter(1.0, 3)
	now := time.Unix(1_700_000_000, 0)

	// First 3 requests in the same instant are allowed (the burst).
	for i := 0; i < 3; i++ {
		if !l.allow("1.2.3.4", now) {
			t.Fatalf("request %d in burst should be allowed", i+1)
		}
	}
	// 4th in the same instant is throttled.
	if l.allow("1.2.3.4", now) {
		t.Fatalf("request beyond burst should be throttled")
	}

	// After 1 second, one token refills → exactly one more request allowed.
	now = now.Add(time.Second)
	if !l.allow("1.2.3.4", now) {
		t.Fatalf("one token should have refilled after 1s")
	}
	if l.allow("1.2.3.4", now) {
		t.Fatalf("only one token should refill per second")
	}
}

// TestIPRateLimiterIsolatesClients verifies one IP exhausting its bucket does
// not throttle a different IP.
func TestIPRateLimiterIsolatesClients(t *testing.T) {
	l := newIPRateLimiter(1.0, 2)
	now := time.Unix(1_700_000_000, 0)

	// Exhaust IP A.
	if !l.allow("10.0.0.1", now) || !l.allow("10.0.0.1", now) {
		t.Fatalf("IP A burst should be allowed")
	}
	if l.allow("10.0.0.1", now) {
		t.Fatalf("IP A should be throttled after burst")
	}

	// IP B is unaffected.
	if !l.allow("10.0.0.2", now) {
		t.Fatalf("IP B should be allowed regardless of IP A")
	}
}

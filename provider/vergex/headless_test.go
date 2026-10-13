package vergex

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRelayRetryDelay(t *testing.T) {
	const m = time.Minute
	cases := []struct {
		failures int
		want     time.Duration
	}{
		{0, m}, {-1, m}, {1, m}, {2, 2 * m}, {3, 4 * m}, {4, 5 * m}, {5, 5 * m},
		{10, 5 * m}, {64, 5 * m}, {1 << 30, 5 * m},
	}
	for _, c := range cases {
		if got := relayRetryDelay(c.failures, m); got != c.want {
			t.Errorf("relayRetryDelay(%d, 1m) = %s, want %s", c.failures, got, c.want)
		}
	}
	// An interval already above the cap is clamped once failing.
	if got := relayRetryDelay(2, 10*time.Minute); got != relayMaxBackoff {
		t.Errorf("large interval: got %s, want %s", got, relayMaxBackoff)
	}
}

func TestRelayHealthAlertOncePerOutage(t *testing.T) {
	start := time.Date(2026, 10, 13, 3, 0, 0, 0, time.UTC)
	h := newRelayHealth(start)
	boom := errors.New("boom")

	// Success shortly after start, then failures.
	if a, r := h.record(true, nil, start.Add(time.Minute)); a || r {
		t.Fatalf("success w/o outage: alert=%v recovered=%v", a, r)
	}
	last := start.Add(time.Minute)

	if a, _ := h.record(false, boom, last.Add(relayTTL-time.Second)); a {
		t.Fatalf("alerted before threshold")
	}
	if h.failures != 1 {
		t.Fatalf("failures = %d, want 1", h.failures)
	}
	if a, r := h.record(false, boom, last.Add(relayTTL)); !a || r {
		t.Fatalf("expected alert at threshold, got alert=%v recovered=%v", a, r)
	}
	if a, _ := h.record(false, boom, last.Add(relayTTL+time.Minute)); a {
		t.Fatalf("repeated alert in same outage")
	}
	if h.failures != 3 {
		t.Fatalf("failures = %d, want 3", h.failures)
	}

	back := last.Add(relayTTL + 5*time.Minute)
	if a, r := h.record(true, nil, back); a || !r {
		t.Fatalf("expected recovery, got alert=%v recovered=%v", a, r)
	}
	if h.lastOutage != back.Sub(last) || h.failures != 0 {
		t.Fatalf("lastOutage=%s failures=%d", h.lastOutage, h.failures)
	}
	if _, r := h.record(true, nil, back.Add(time.Minute)); r {
		t.Fatalf("recovery reported twice")
	}

	// A new outage alerts again.
	if a, _ := h.record(false, boom, back.Add(time.Minute+relayTTL)); !a {
		t.Fatalf("expected alert for second outage")
	}
}

func TestRelayHealthNeverSucceeded(t *testing.T) {
	start := time.Date(2026, 10, 13, 3, 0, 0, 0, time.UTC)
	h := newRelayHealth(start)
	boom := errors.New("navigation failed")
	if a, _ := h.record(false, boom, start.Add(relayTTL-time.Minute)); a {
		t.Fatalf("alerted too early")
	}
	if a, _ := h.record(false, boom, start.Add(relayTTL)); !a {
		t.Fatalf("expected alert for never-succeeded relay")
	}
	if msg := h.alertMessage(); !strings.Contains(msg, "启动后从未成功") || !strings.Contains(msg, "30 分钟") {
		t.Fatalf("unexpected alert message: %s", msg)
	}
}

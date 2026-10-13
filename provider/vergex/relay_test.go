package vergex

import (
	"testing"
	"time"
)

// Relay-fed payloads must serve GetAI500Symbols without any network access —
// the backend's direct vergex requests are Cloudflare-challenged.
func TestRelayFeedsAI500(t *testing.T) {
	const relayedPath = "/trending-category?key=ai500&lang=en"
	body := `{"category":{"key":"ai500","assets":[
		{"symbol":"USELESS","pair":"USELESSUSDT","score":69.3},
		{"symbol":"VIRTUAL","pair":"VIRTUALUSDT","score":55.1},
		{"symbol":"not a symbol 🦞","pair":"","score":1}
	]},"tableRows":[{"symbol":"FET","pair":"FETUSDT","score":40.2}]}`

	c := NewClient()
	// Without relay data the fetch would hit the (challenged) upstream.
	FeedRelay(relayedPath, []byte(body))

	symbols, err := c.GetAI500Symbols(10)
	if err != nil {
		t.Fatalf("GetAI500Symbols: %v", err)
	}
	want := []string{"USELESSUSDT", "VIRTUALUSDT", "FETUSDT"}
	if len(symbols) != len(want) {
		t.Fatalf("symbols = %v, want %v", symbols, want)
	}
	for i := range want {
		if symbols[i] != want[i] {
			t.Fatalf("symbols[%d] = %s, want %s", i, symbols[i], want[i])
		}
	}
}

// Entries past relayTTL must be dropped so strategies don't silently trade on
// outdated candidate lists.
func TestRelayExpiry(t *testing.T) {
	const p = "/trending-crypto?tab=oi&duration=1h&limit=100"
	FeedRelay(p, []byte(`{"top":[]}`))

	if body, ok := relayLookup(p); !ok || len(body) == 0 {
		t.Fatalf("fresh relay entry missing")
	}

	key := canonicalRelayKey(p)
	relayMu.Lock()
	relayCache[key] = relayEntry{body: []byte(`{}`), expires: time.Now().Add(-time.Minute)}
	relayMu.Unlock()

	if _, ok := relayLookup(p); ok {
		t.Fatalf("stale relay entry still served")
	}
}

// Malformed relay payloads must be dropped at feed time.
func TestRelayRejectsInvalidBody(t *testing.T) {
	FeedRelay("/trending-hl?category=crypto", []byte("not json"))
	if _, ok := relayLookup("/trending-hl?category=crypto"); ok {
		t.Fatalf("invalid body was stored")
	}
}

// Kernel requests must hit browser-relayed payloads even when the parameter
// order differs and the limits don't match (kernel asks limit=20, browser
// relays limit=100).
func TestRelayCanonicalMatch(t *testing.T) {
	FeedRelay("/trending-crypto?tab=oi&duration=1h&limit=100", []byte(`{"top":[]}`))
	if _, ok := relayLookup("/trending-crypto?duration=1h&limit=20&tab=oi"); !ok {
		t.Fatalf("canonical relay lookup missed for reordered params/limit")
	}
	if _, ok := relayLookup("/trending-crypto?tab=oi&duration=4h&limit=20"); ok {
		t.Fatalf("relay entry for a different duration must not be served")
	}
}

// A relay-fed limit=100 payload must be sliced down to the caller's limit.
func TestGetOIRankingRelayTruncatesToLimit(t *testing.T) {
	rows := func() string {
		s := ""
		for i := 0; i < 100; i++ {
			if i > 0 {
				s += ","
			}
			s += `{"rank":1,"symbol":"COINUSDT","price":1,"current_oi":1,"oi_delta":1,"oi_delta_percent":1,"oi_delta_value":1,"price_delta_percent":1}`
		}
		return s
	}
	body := `{"top":[` + rows() + `], "low":[` + rows() + `]}`
	FeedRelay("/trending-crypto?tab=oi&duration=1h&limit=100", []byte(body))

	c := NewClient()
	result, err := c.GetOIRanking("1h", 20)
	if err != nil {
		t.Fatalf("GetOIRanking: %v", err)
	}
	if len(result.LowPositions) != 20 || len(result.TopPositions) != 20 {
		t.Fatalf("positions = top %d / low %d, want 20/20",
			len(result.TopPositions), len(result.LowPositions))
	}
}

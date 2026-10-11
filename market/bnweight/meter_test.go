package bnweight

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// newTestMeter builds a meter on a fake clock whose sleep just advances the
// clock (and records the requested waits).
func newTestMeter(start time.Time) (*Meter, *fakeClock, *[]time.Duration) {
	clk := &fakeClock{t: start}
	m := NewMeter()
	m.now = clk.Now
	var mu sync.Mutex
	waits := []time.Duration{}
	m.sleep = func(ctx context.Context, d time.Duration) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		mu.Lock()
		waits = append(waits, d)
		mu.Unlock()
		clk.Advance(d)
		return nil
	}
	m.jitter = func() time.Duration { return 500 * time.Millisecond }
	return m, clk, &waits
}

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func resp(status int, hdr map[string]string) *http.Response {
	h := http.Header{}
	for k, v := range hdr {
		h.Set(k, v)
	}
	return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(strings.NewReader("{}"))}
}

func get(t *testing.T, rt http.RoundTripper, rawURL string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	return rt.RoundTrip(req)
}

func TestClassifyWeights(t *testing.T) {
	cases := []struct {
		method, rawURL string
		weight         int
		bucket         Bucket
		endpoint       string
	}{
		{"GET", "/fapi/v1/klines?symbol=BTCUSDT&interval=1m&limit=1", 1, BucketIP1M, "/fapi/v1/klines"},
		{"GET", "/fapi/v1/klines?symbol=BTCUSDT&limit=99", 1, BucketIP1M, "/fapi/v1/klines"},
		{"GET", "/fapi/v1/klines?symbol=BTCUSDT&limit=100", 2, BucketIP1M, "/fapi/v1/klines"},
		{"GET", "/fapi/v1/klines?symbol=BTCUSDT&limit=499", 2, BucketIP1M, "/fapi/v1/klines"},
		{"GET", "/fapi/v1/klines?symbol=BTCUSDT&limit=500", 5, BucketIP1M, "/fapi/v1/klines"},
		{"GET", "/fapi/v1/klines?symbol=BTCUSDT&limit=865", 5, BucketIP1M, "/fapi/v1/klines"},
		{"GET", "/fapi/v1/klines?symbol=BTCUSDT&limit=1000", 5, BucketIP1M, "/fapi/v1/klines"},
		{"GET", "/fapi/v1/klines?symbol=BTCUSDT&limit=1500", 10, BucketIP1M, "/fapi/v1/klines"},
		{"GET", "/fapi/v1/klines?symbol=BTCUSDT", 5, BucketIP1M, "/fapi/v1/klines"},
		{"GET", "/fapi/v1/depth?symbol=BTCUSDT&limit=20", 2, BucketIP1M, "/fapi/v1/depth"},
		{"GET", "/fapi/v1/depth?symbol=BTCUSDT&limit=100", 5, BucketIP1M, "/fapi/v1/depth"},
		{"GET", "/fapi/v1/depth?symbol=BTCUSDT&limit=500", 10, BucketIP1M, "/fapi/v1/depth"},
		{"GET", "/fapi/v1/depth?symbol=BTCUSDT&limit=1000", 20, BucketIP1M, "/fapi/v1/depth"},
		{"GET", "/fapi/v1/userTrades?symbol=BTCUSDT", 5, BucketIP1M, "/fapi/v1/userTrades"},
		{"GET", "/fapi/v1/income?incomeType=COMMISSION", 30, BucketIP1M, "/fapi/v1/income"},
		{"GET", "/fapi/v2/positionRisk", 5, BucketIP1M, "/fapi/v2/positionRisk"},
		{"GET", "/fapi/v2/account", 5, BucketIP1M, "/fapi/v2/account"},
		{"GET", "/fapi/v1/openOrders?symbol=BTCUSDT", 1, BucketIP1M, "/fapi/v1/openOrders"},
		{"GET", "/fapi/v1/openOrders", 40, BucketIP1M, "/fapi/v1/openOrders"},
		{"GET", "/fapi/v1/allOrders?symbol=BTCUSDT", 5, BucketIP1M, "/fapi/v1/allOrders"},
		{"GET", "/fapi/v1/ticker/24hr?symbol=BTCUSDT", 1, BucketIP1M, "/fapi/v1/ticker/24hr"},
		{"GET", "/fapi/v1/ticker/24hr", 40, BucketIP1M, "/fapi/v1/ticker/24hr"},
		{"GET", "/fapi/v1/exchangeInfo", 1, BucketIP1M, "/fapi/v1/exchangeInfo"},
		{"GET", "/fapi/v1/premiumIndex?symbol=BTCUSDT", 1, BucketIP1M, "/fapi/v1/premiumIndex"},
		{"GET", "/fapi/v1/premiumIndex", 10, BucketIP1M, "/fapi/v1/premiumIndex"},
		{"GET", "/fapi/v1/fundingRate?symbol=BTCUSDT&limit=4", 0, BucketFundingRate, "/fapi/v1/fundingRate"},
		{"GET", "/futures/data/openInterestHist?symbol=BTCUSDT&period=5m", 0, BucketData, "/futures/data/openInterestHist"},
		{"POST", "/fapi/v1/order", 0, BucketIP1M, "/fapi/v1/order"},
		{"DELETE", "/fapi/v1/order", 1, BucketIP1M, "/fapi/v1/order"},
		{"GET", "/fapi/v1/somethingNew/", 1, BucketIP1M, "/fapi/v1/somethingNew"},
	}
	for _, c := range cases {
		u, err := url.Parse("https://fapi.binance.com" + c.rawURL)
		if err != nil {
			t.Fatal(err)
		}
		ep, w, b := Classify(c.method, u.Path, u.Query())
		if ep != c.endpoint || w != c.weight || b != c.bucket {
			t.Errorf("%s %s: got (%s, %d, %v), want (%s, %d, %v)", c.method, c.rawURL, ep, w, b, c.endpoint, c.weight, c.bucket)
		}
	}
}

func TestOnlyFuturesHostsAreMetered(t *testing.T) {
	m, _, _ := newTestMeter(time.Date(2026, 10, 11, 12, 0, 10, 0, time.UTC))
	calls := 0
	rt := m.Wrap(rtFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return resp(200, map[string]string{HeaderUsedWeight1M: "3000"}), nil
	}), Low)
	for _, u := range []string{
		"https://api.binance.com/api/v3/klines?symbol=BTCUSDT",
		"https://query1.finance.yahoo.com/v8/finance/chart/AAPL",
	} {
		if _, err := get(t, rt, u); err != nil {
			t.Fatal(err)
		}
	}
	s := m.Snapshot()
	if calls != 2 || s.UsedWeight != 0 || s.EstLastMinute != 0 || len(s.Endpoints) != 0 {
		t.Fatalf("non-futures traffic must pass through untouched: calls=%d snap=%+v", calls, s)
	}
	for _, h := range []string{"fapi.binance.com", "fapi1.binance.com", "FAPI.binance.com:443", "testnet.binancefuture.com"} {
		if !m.isFuturesHost(h) {
			t.Errorf("%s should be a futures host", h)
		}
	}
	if m.isFuturesHost("api.binance.com") || m.isFuturesHost("example.com") {
		t.Error("spot/other host misclassified")
	}
	m.RegisterHost("mirror.example.com:8443")
	if !m.isFuturesHost("mirror.example.com:8443") || !m.isFuturesHost("mirror.example.com") {
		t.Error("registered host not recognised")
	}
	t.Setenv("BINANCE_FAPI_BASE", "https://fapi-proxy.internal:9000/")
	if !m.isFuturesHost("fapi-proxy.internal:9000") {
		t.Error("BINANCE_FAPI_BASE host not recognised")
	}
}

func TestCountsHeaderAndSummary(t *testing.T) {
	m, clk, _ := newTestMeter(time.Date(2026, 10, 11, 12, 0, 10, 0, time.UTC))
	rt := m.Wrap(rtFunc(func(r *http.Request) (*http.Response, error) {
		return resp(200, map[string]string{HeaderUsedWeight1M: "993"}), nil
	}), Low)
	crit := m.Wrap(rtFunc(func(r *http.Request) (*http.Response, error) {
		return resp(200, nil), nil
	}), Critical)

	for i := 0; i < 3; i++ {
		get(t, rt, "https://fapi.binance.com/fapi/v1/klines?symbol=BTCUSDT&interval=15m&limit=865")
	}
	get(t, rt, "https://fapi.binance.com/fapi/v1/ticker/24hr")
	get(t, rt, "https://fapi.binance.com/futures/data/openInterestHist?symbol=BTCUSDT")
	get(t, rt, "https://fapi.binance.com/fapi/v1/fundingRate?symbol=BTCUSDT")
	get(t, crit, "https://fapi.binance.com/fapi/v2/positionRisk")
	clk.Advance(5 * time.Second)

	s := m.Snapshot()
	if s.UsedWeight != 993 || !s.UsedFresh {
		t.Fatalf("header not captured: %+v", s)
	}
	wantEst := 3*5 + 40 + 5
	if s.EstLastMinute != wantEst {
		t.Fatalf("EstLastMinute = %d, want %d", s.EstLastMinute, wantEst)
	}
	if s.Data5m != 1 || s.FundingRate5m != 1 {
		t.Fatalf("separate pools: data=%d funding=%d", s.Data5m, s.FundingRate5m)
	}
	if s.ByPriority[Low].Weight != 55 || s.ByPriority[Low].Requests != 6 || s.ByPriority[Critical].Weight != 5 || s.ByPriority[Critical].Requests != 1 {
		t.Fatalf("per-priority aggregation wrong: %+v", s.ByPriority)
	}
	if len(s.Endpoints) == 0 || s.Endpoints[0].Endpoint != "/fapi/v1/ticker/24hr" || s.Endpoints[0].Weight != 40 {
		t.Fatalf("top endpoint should be ticker/24hr: %+v", s.Endpoints)
	}
	line := m.SummaryLine()
	for _, want := range []string{"header=993/2400", "ownEst1m=60", "/fapi/v1/ticker/24hr=40w/1x", "/fapi/v1/klines=15w/3x", "low=55w/6r", "critical=5w/1r"} {
		if !strings.Contains(line, want) {
			t.Errorf("summary missing %q:\n%s", want, line)
		}
	}
	// Events older than a minute leave the rolling window but stay in totals.
	clk.Advance(2 * time.Minute)
	s = m.Snapshot()
	if s.EstLastMinute != 0 || s.Totals[Low].Requests != 6 {
		t.Fatalf("rolling window / totals wrong: %+v", s)
	}
}

func TestSummaryTopEightOnly(t *testing.T) {
	m, _, _ := newTestMeter(time.Date(2026, 10, 11, 12, 0, 10, 0, time.UTC))
	rt := m.Wrap(rtFunc(func(*http.Request) (*http.Response, error) { return resp(200, nil), nil }), Normal)
	for i := 0; i < 12; i++ {
		get(t, rt, "https://fapi.binance.com/fapi/v1/ep"+string(rune('a'+i)))
	}
	line := m.SummaryLine()
	if got := strings.Count(line, "/fapi/v1/ep"); got != 8 {
		t.Fatalf("summary should list 8 endpoints, got %d:\n%s", got, line)
	}
}

func TestGuardLowWaitsNormalAndCriticalDoNot(t *testing.T) {
	start := time.Date(2026, 10, 11, 12, 0, 20, 0, time.UTC)
	m, clk, waits := newTestMeter(start)
	// Seed: header says 1600 used this minute.
	seed := m.Wrap(rtFunc(func(*http.Request) (*http.Response, error) {
		return resp(200, map[string]string{HeaderUsedWeight1M: "1600"}), nil
	}), Critical)
	get(t, seed, "https://fapi.binance.com/fapi/v1/time")

	sent := 0
	base := rtFunc(func(*http.Request) (*http.Response, error) {
		sent++
		return resp(200, map[string]string{HeaderUsedWeight1M: "10"}), nil
	})
	// Critical: never waits.
	if _, err := get(t, m.Wrap(base, Critical), "https://fapi.binance.com/fapi/v2/positionRisk"); err != nil {
		t.Fatal(err)
	}
	if len(*waits) != 0 {
		t.Fatalf("Critical must not wait, waits=%v", *waits)
	}
	// Normal threshold is 2000: 1600 passes.
	if _, err := get(t, m.Wrap(base, Normal), "https://fapi.binance.com/fapi/v1/ticker/24hr?symbol=BTCUSDT"); err != nil {
		t.Fatal(err)
	}
	if len(*waits) != 0 {
		t.Fatalf("Normal below 2000 must not wait, waits=%v", *waits)
	}
	// Low: >=1500 -> waits to the next minute boundary (+jitter), then goes.
	// (Critical positionRisk above added 5 own weight, header seed already 1600.)
	before := clk.Now()
	if _, err := get(t, m.Wrap(base, Low), "https://fapi.binance.com/fapi/v1/klines?symbol=BTCUSDT&limit=50"); err != nil {
		t.Fatal(err)
	}
	if len(*waits) != 1 {
		t.Fatalf("Low must wait exactly once, waits=%v", *waits)
	}
	wantWait := 40*time.Second + 500*time.Millisecond // 12:00:20 -> 12:01:00 + jitter
	if (*waits)[0] != wantWait {
		t.Fatalf("wait = %v, want %v", (*waits)[0], wantWait)
	}
	if clk.Now().Sub(before) != wantWait || sent != 3 {
		t.Fatalf("clock/sent wrong: advanced=%v sent=%d", clk.Now().Sub(before), sent)
	}

	// Normal at >=2000 also waits.
	m2, _, waits2 := newTestMeter(start)
	hot := m2.Wrap(rtFunc(func(*http.Request) (*http.Response, error) {
		return resp(200, map[string]string{HeaderUsedWeight1M: "2100"}), nil
	}), Critical)
	get(t, hot, "https://fapi.binance.com/fapi/v1/time")
	if _, err := get(t, m2.Wrap(base, Normal), "https://fapi.binance.com/fapi/v1/exchangeInfo"); err != nil {
		t.Fatal(err)
	}
	if len(*waits2) != 1 {
		t.Fatalf("Normal at 2100 must wait, waits=%v", *waits2)
	}
}

func TestGuardIgnoresStaleMinuteAndCountsOwnSends(t *testing.T) {
	m, clk, waits := newTestMeter(time.Date(2026, 10, 11, 12, 0, 50, 0, time.UTC))
	seed := m.Wrap(rtFunc(func(*http.Request) (*http.Response, error) {
		return resp(200, map[string]string{HeaderUsedWeight1M: "1499"}), nil
	}), Critical)
	get(t, seed, "https://fapi.binance.com/fapi/v1/time")

	base := rtFunc(func(*http.Request) (*http.Response, error) { return resp(200, nil), nil })
	low := m.Wrap(base, Low)
	// 1499 + own weight sent since (a 1000-limit kline = 5) crosses 1500 without a new header.
	get(t, low, "https://fapi.binance.com/fapi/v1/klines?symbol=A&limit=1000") // 1499 < 1500: passes, adds 5
	if len(*waits) != 0 {
		t.Fatalf("unexpected wait: %v", *waits)
	}
	get(t, low, "https://fapi.binance.com/fapi/v1/klines?symbol=B&limit=1000") // 1504 >= 1500: waits
	if len(*waits) != 1 {
		t.Fatalf("own sends since the last header must count toward the guard, waits=%v", *waits)
	}
	// The wait ran into the next minute, so the stale header no longer blocks.
	_ = clk
	get(t, low, "https://fapi.binance.com/fapi/v1/klines?symbol=C&limit=1000")
	if len(*waits) != 1 {
		t.Fatalf("new minute must not wait again, waits=%v", *waits)
	}
}

func TestGuardHonoursContextCancel(t *testing.T) {
	m, _, _ := newTestMeter(time.Date(2026, 10, 11, 12, 0, 20, 0, time.UTC))
	seed := m.Wrap(rtFunc(func(*http.Request) (*http.Response, error) {
		return resp(200, map[string]string{HeaderUsedWeight1M: "2300"}), nil
	}), Critical)
	get(t, seed, "https://fapi.binance.com/fapi/v1/time")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://fapi.binance.com/fapi/v1/klines?symbol=A", nil)
	called := false
	_, err := m.Wrap(rtFunc(func(*http.Request) (*http.Response, error) { called = true; return resp(200, nil), nil }), Low).RoundTrip(req)
	if !errors.Is(err, context.Canceled) || called {
		t.Fatalf("cancelled ctx must abort the wait without sending: err=%v called=%v", err, called)
	}
}

func TestGuardWaitIsCappedAt60s(t *testing.T) {
	m, _, waits := newTestMeter(time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC))
	// Sleep that does NOT advance the clock simulates a stuck minute.
	m.sleep = func(ctx context.Context, d time.Duration) error { *waits = append(*waits, d); return nil }
	seed := m.Wrap(rtFunc(func(*http.Request) (*http.Response, error) {
		return resp(200, map[string]string{HeaderUsedWeight1M: "2300"}), nil
	}), Critical)
	get(t, seed, "https://fapi.binance.com/fapi/v1/time")
	sent := false
	m.Wrap(rtFunc(func(*http.Request) (*http.Response, error) { sent = true; return resp(200, nil), nil }), Low).
		RoundTrip(mustReq(t, "https://fapi.binance.com/fapi/v1/klines?symbol=A"))
	var total time.Duration
	for _, w := range *waits {
		total += w
	}
	if !sent || total > maxGuardWait {
		t.Fatalf("total wait %v must be <= %v and request must eventually go (sent=%v)", total, maxGuardWait, sent)
	}
}

func mustReq(t *testing.T, u string) *http.Request {
	t.Helper()
	r, err := http.NewRequest("GET", u, nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestCooldownAfter429(t *testing.T) {
	m, clk, _ := newTestMeter(time.Date(2026, 10, 11, 12, 0, 10, 0, time.UTC))
	status := 429
	sent := 0
	base := rtFunc(func(*http.Request) (*http.Response, error) {
		sent++
		if status == 200 {
			return resp(200, nil), nil
		}
		return resp(status, map[string]string{"Retry-After": "30"}), nil
	})
	low := m.Wrap(base, Low)
	normal := m.Wrap(base, Normal)
	crit := m.Wrap(base, Critical)

	// The 429 itself reaches the caller.
	r, err := get(t, low, "https://fapi.binance.com/fapi/v1/klines?symbol=A")
	if err != nil || r.StatusCode != 429 {
		t.Fatalf("first request should return the 429, got %v %v", r, err)
	}
	until, in := m.InCooldown()
	if !in || until != clk.Now().Add(30*time.Second) {
		t.Fatalf("cooldown should last Retry-After=30s: until=%v in=%v", until, in)
	}
	before := sent
	for _, rt := range []http.RoundTripper{low, normal} {
		_, err := get(t, rt, "https://fapi.binance.com/fapi/v1/klines?symbol=A")
		if !errors.Is(err, ErrCooldown) {
			t.Fatalf("Low/Normal must fail fast with ErrCooldown, got %v", err)
		}
		var ce *CooldownError
		if !errors.As(err, &ce) || ce.Status != 429 {
			t.Fatalf("typed error expected: %v", err)
		}
	}
	if sent != before {
		t.Fatalf("rejected requests must not be sent (sent %d -> %d)", before, sent)
	}
	// Critical still goes out during the cooldown.
	status = 200
	if r, err := get(t, crit, "https://fapi.binance.com/fapi/v1/order"); err != nil || r.StatusCode != 200 {
		t.Fatalf("Critical must pass during cooldown: %v %v", r, err)
	}
	if sent != before+1 {
		t.Fatalf("Critical not sent")
	}
	// Through http.Client the error is wrapped in *url.Error but still matches.
	cl := &http.Client{Transport: low}
	if _, err := cl.Get("https://fapi.binance.com/fapi/v1/klines?symbol=A"); !errors.Is(err, ErrCooldown) {
		t.Fatalf("errors.Is through http.Client failed: %v", err)
	}
	// After Retry-After elapses everything flows again.
	clk.Advance(31 * time.Second)
	if _, err := get(t, low, "https://fapi.binance.com/fapi/v1/klines?symbol=A"); err != nil {
		t.Fatalf("cooldown should be over: %v", err)
	}
}

func TestCooldownDefaultsAndCriticalTriggers(t *testing.T) {
	m, clk, _ := newTestMeter(time.Date(2026, 10, 11, 12, 0, 30, 0, time.UTC))
	rt := m.Wrap(rtFunc(func(*http.Request) (*http.Response, error) { return resp(418, nil), nil }), Critical)
	get(t, rt, "https://fapi.binance.com/fapi/v1/order")
	until, in := m.InCooldown()
	if !in || until.Sub(clk.Now()) != 5*time.Minute {
		t.Fatalf("418 without Retry-After defaults to 5 minutes: %v %v", until, in)
	}
	// A second 429 while cooling down extends (never shortens) and does not re-announce.
	clk.Advance(time.Minute)
	m2, clk2, _ := newTestMeter(time.Date(2026, 10, 11, 12, 0, 30, 0, time.UTC))
	rt2 := m2.Wrap(rtFunc(func(*http.Request) (*http.Response, error) { return resp(429, nil), nil }), Normal)
	get(t, rt2, "https://fapi.binance.com/fapi/v1/klines?symbol=A")
	u2, _ := m2.InCooldown()
	if u2.Sub(clk2.Now()) != 32*time.Second { // rest of the minute (30s) + 2s
		t.Fatalf("429 without Retry-After -> next minute + 2s, got %v", u2.Sub(clk2.Now()))
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	if d := parseRetryAfter("120", now); d != 120*time.Second {
		t.Errorf("seconds: %v", d)
	}
	if d := parseRetryAfter(now.Add(90*time.Second).Format(http.TimeFormat), now); d != 90*time.Second {
		t.Errorf("http date: %v", d)
	}
	if parseRetryAfter("", now) != 0 || parseRetryAfter("garbage", now) != 0 || parseRetryAfter("-5", now) != 0 {
		t.Error("invalid values must yield 0")
	}
}

// End-to-end through a real httptest server and http.Client.
func TestWrapAgainstHTTPTestServer(t *testing.T) {
	var mu sync.Mutex
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		n := hits
		mu.Unlock()
		if n == 3 {
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set(HeaderUsedWeight1M, "321")
		w.Write([]byte("[]"))
	}))
	defer srv.Close()
	m := NewMeter()
	u, _ := url.Parse(srv.URL)
	m.RegisterHost(u.Host)
	cl := &http.Client{Transport: m.Wrap(nil, Low)}
	for i := 0; i < 2; i++ {
		resp, err := cl.Get(srv.URL + "/fapi/v1/klines?symbol=BTCUSDT&limit=200")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	s := m.Snapshot()
	if s.UsedWeight != 321 || s.EstLastMinute != 4 {
		t.Fatalf("snapshot: %+v", s)
	}
	resp, err := cl.Get(srv.URL + "/fapi/v1/klines?symbol=BTCUSDT&limit=200")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if _, err := cl.Get(srv.URL + "/fapi/v1/klines?symbol=BTCUSDT&limit=200"); !errors.Is(err, ErrCooldown) {
		t.Fatalf("expected ErrCooldown after 429, got %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if hits != 3 {
		t.Fatalf("server should have seen exactly 3 requests, saw %d", hits)
	}
}

func TestWrapIsIdempotent(t *testing.T) {
	m := NewMeter()
	inner := rtFunc(func(*http.Request) (*http.Response, error) { return resp(200, nil), nil })
	w1 := m.Wrap(inner, Low)
	w2 := m.Wrap(w1, Critical).(*transport)
	if _, double := w2.base.(*transport); double || w2.prio != Critical {
		t.Fatal("re-wrapping must replace, not stack, the meter")
	}
	get(t, w2, "https://fapi.binance.com/fapi/v1/exchangeInfo")
	if m.Snapshot().Totals[Critical].Requests != 1 || m.Snapshot().Totals[Low].Requests != 0 {
		t.Fatalf("counted wrong: %+v", m.Snapshot().Totals)
	}
}

func TestWithPriorityOverridesClientPriority(t *testing.T) {
	m, _, waits := newTestMeter(time.Date(2026, 10, 11, 12, 0, 20, 0, time.UTC))
	hot := m.Wrap(rtFunc(func(*http.Request) (*http.Response, error) {
		return resp(200, map[string]string{HeaderUsedWeight1M: "2300"}), nil
	}), Critical)
	get(t, hot, "https://fapi.binance.com/fapi/v1/time")

	// A Critical client: an unmarked request never waits even at 2300 ...
	crit := m.Wrap(rtFunc(func(*http.Request) (*http.Response, error) { return resp(200, nil), nil }), Critical)
	get(t, crit, "https://fapi.binance.com/fapi/v2/positionRisk")
	if len(*waits) != 0 {
		t.Fatalf("Critical must not wait: %v", *waits)
	}
	// ... but the same client's request marked Normal (history reads) does, and is counted as Normal.
	req, _ := http.NewRequestWithContext(WithPriority(context.Background(), Normal), "GET", "https://fapi.binance.com/fapi/v1/income", nil)
	if _, err := crit.RoundTrip(req); err != nil {
		t.Fatal(err)
	}
	if len(*waits) != 1 {
		t.Fatalf("Normal-marked request on a Critical client must be guarded: %v", *waits)
	}
	if s := m.Snapshot(); s.Totals[Normal].Requests != 1 || s.Totals[Normal].Weight != 30 {
		t.Fatalf("override not accounted as Normal: %+v", s.Totals)
	}
}

package usstock

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func resetFundamentals() {
	fundamentalsCache.Lock()
	fundamentalsCache.entries = make(map[string]fundamentalsEntry)
	fundamentalsCache.Unlock()
	yahooGate <- struct{}{}
	yahooSession.cookie, yahooSession.crumb = "", ""
	yahooSession.loaded, yahooSession.failedAt, yahooSession.failedWith = time.Time{}, time.Time{}, nil
	<-yahooGate
}

type fundamentalsServer struct {
	cookies, crumbs, summaries atomic.Int32
	crumbStatus                int
	rejectFirstCrumb           bool
	body                       string
}

func (s *fundamentalsServer) handler(t *testing.T) roundTripFunc {
	return func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Host == "fc.test":
			s.cookies.Add(1)
			resp := response(404, "")
			resp.Header.Add("Set-Cookie", "A3=abc; Path=/; Domain=.yahoo.com")
			return resp, nil
		case r.URL.Path == "/v1/test/getcrumb":
			n := s.crumbs.Add(1)
			if !strings.Contains(r.Header.Get("Cookie"), "A3=abc") {
				return response(401, "Unauthorized"), nil
			}
			if s.crumbStatus != 0 {
				return response(s.crumbStatus, "Too Many Requests"), nil
			}
			return response(200, "crumb"+string(rune('0'+n))), nil
		case strings.HasPrefix(r.URL.Path, "/v10/finance/quoteSummary/"):
			n := s.summaries.Add(1)
			if s.rejectFirstCrumb && n == 1 {
				return response(401, `{"finance":{"error":{"code":"Unauthorized"}}}`), nil
			}
			if r.URL.Query().Get("crumb") == "" || !strings.Contains(r.Header.Get("Cookie"), "A3=abc") {
				return response(401, "{}"), nil
			}
			if !strings.HasSuffix(r.URL.Path, "/AAPL") {
				return response(404, `{"quoteSummary":{"result":null,"error":{"code":"Not Found","description":"No fundamentals data found"}}}`), nil
			}
			return response(200, s.body), nil
		}
		t.Errorf("unexpected request: %s", r.URL)
		return response(400, "{}"), nil
	}
}

func setupFundamentals(t *testing.T, now time.Time, s *fundamentalsServer) {
	t.Helper()
	setupHTTP(t, now, s.handler(t))
	oldCookie := yahooCookieURL
	yahooCookieURL = "https://fc.test"
	resetFundamentals()
	t.Cleanup(func() {
		yahooCookieURL = oldCookie
		resetFundamentals()
	})
}

const aaplSummary = `{"quoteSummary":{"result":[{
 "calendarEvents":{"earnings":{"earningsDate":[{"raw":1761868800,"fmt":"2025-10-31"},{"raw":1793404800,"fmt":"2026-10-30"}]}},
 "assetProfile":{"sector":"Technology"},
 "quoteType":{"quoteType":"EQUITY"}}],"error":null}}`

func TestGetFundamentalsCrumbFlowAndCache(t *testing.T) {
	now := time.Date(2026, 10, 15, 14, 0, 0, 0, time.UTC)
	s := &fundamentalsServer{body: aaplSummary}
	setupFundamentals(t, now, s)
	f, err := GetFundamentals(context.Background(), "aapl")
	if err != nil {
		t.Fatal(err)
	}
	if f.QuoteType != "EQUITY" || f.Sector != "Technology" {
		t.Fatalf("fundamentals = %+v", f)
	}
	// 1761868800 is in the past relative to now; the next future date wins.
	if want := time.Unix(1793404800, 0).UTC(); !f.NextEarnings.Equal(want) {
		t.Fatalf("next earnings = %v want %v", f.NextEarnings, want)
	}
	if s.cookies.Load() != 1 || s.crumbs.Load() != 1 || s.summaries.Load() != 1 {
		t.Fatalf("calls cookie/crumb/summary = %d/%d/%d", s.cookies.Load(), s.crumbs.Load(), s.summaries.Load())
	}
	// Cached for 24h: no more traffic, same value (and the crumb is reused).
	if _, err := GetFundamentals(context.Background(), "AAPL"); err != nil || s.summaries.Load() != 1 {
		t.Fatalf("expected cache hit, err=%v summaries=%d", err, s.summaries.Load())
	}
	nowFunc = func() time.Time { return now.Add(25 * time.Hour) }
	if _, err := GetFundamentals(context.Background(), "AAPL"); err != nil || s.summaries.Load() != 2 {
		t.Fatalf("expected refetch after TTL, err=%v summaries=%d", err, s.summaries.Load())
	}
	// 25h later the 12h crumb has expired and is fetched again.
	if s.cookies.Load() != 2 || s.crumbs.Load() != 2 {
		t.Fatalf("crumb should refresh after its TTL: cookies=%d crumbs=%d", s.cookies.Load(), s.crumbs.Load())
	}
}

func TestGetFundamentalsNegativeCache(t *testing.T) {
	now := time.Date(2026, 10, 15, 14, 0, 0, 0, time.UTC)
	s := &fundamentalsServer{body: aaplSummary}
	setupFundamentals(t, now, s)
	if _, err := GetFundamentals(context.Background(), "ZZZZ"); err == nil {
		t.Fatal("expected error for unknown ticker")
	}
	if _, err := GetFundamentals(context.Background(), "ZZZZ"); err == nil || s.summaries.Load() != 1 {
		t.Fatalf("negative result should be cached: err=%v summaries=%d", err, s.summaries.Load())
	}
	nowFunc = func() time.Time { return now.Add(7 * time.Hour) }
	if _, err := GetFundamentals(context.Background(), "ZZZZ"); err == nil || s.summaries.Load() != 2 {
		t.Fatalf("negative cache should expire after 6h: err=%v summaries=%d", err, s.summaries.Load())
	}
}

func TestGetFundamentalsRefreshesRejectedCrumb(t *testing.T) {
	now := time.Date(2026, 10, 15, 14, 0, 0, 0, time.UTC)
	s := &fundamentalsServer{body: aaplSummary, rejectFirstCrumb: true}
	setupFundamentals(t, now, s)
	f, err := GetFundamentals(context.Background(), "AAPL")
	if err != nil || f.Sector != "Technology" {
		t.Fatalf("f=%+v err=%v", f, err)
	}
	if s.crumbs.Load() != 2 {
		t.Fatalf("crumb fetched %d times, want 2", s.crumbs.Load())
	}
}

func TestGetFundamentalsCrumbFailureBacksOff(t *testing.T) {
	now := time.Date(2026, 10, 15, 14, 0, 0, 0, time.UTC)
	s := &fundamentalsServer{body: aaplSummary, crumbStatus: 429}
	setupFundamentals(t, now, s)
	for _, ticker := range []string{"AAPL", "MSFT", "NVDA"} {
		if _, err := GetFundamentals(context.Background(), ticker); err == nil {
			t.Fatalf("%s: expected error", ticker)
		}
	}
	if s.crumbs.Load() != 1 || s.summaries.Load() != 0 {
		t.Fatalf("crumb failure should back off: crumbs=%d summaries=%d", s.crumbs.Load(), s.summaries.Load())
	}
}

func TestGetFundamentalsETFWithoutProfile(t *testing.T) {
	now := time.Date(2026, 10, 15, 14, 0, 0, 0, time.UTC)
	s := &fundamentalsServer{body: `{"quoteSummary":{"result":[{"calendarEvents":{},"quoteType":{"quoteType":"ETF"}}],"error":null}}`}
	setupFundamentals(t, now, s)
	f, err := GetFundamentals(context.Background(), "AAPL")
	if err != nil || f.QuoteType != "ETF" || f.Sector != "" || !f.NextEarnings.IsZero() {
		t.Fatalf("f=%+v err=%v", f, err)
	}
}

func TestGetFundamentalsCanceledContext(t *testing.T) {
	now := time.Date(2026, 10, 15, 14, 0, 0, 0, time.UTC)
	s := &fundamentalsServer{body: aaplSummary}
	setupFundamentals(t, now, s)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := GetFundamentals(ctx, "AAPL"); err == nil {
		t.Fatal("expected context error")
	}
	if s.cookies.Load() != 0 {
		t.Fatal("canceled call must not hit the network")
	}
}

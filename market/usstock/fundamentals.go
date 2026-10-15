package usstock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Fundamentals is the best-effort company profile of an underlying from Yahoo
// quoteSummary. Any field may be zero/empty when Yahoo does not know it.
type Fundamentals struct {
	QuoteType    string    // "EQUITY", "ETF", ...
	Sector       string    // "" for ETFs and unknown
	NextEarnings time.Time // zero when unknown; the earliest listed date not in the past
}

const (
	fundamentalsTTL     = 24 * time.Hour
	fundamentalsNegTTL  = 6 * time.Hour
	crumbTTL            = 12 * time.Hour
	crumbFailBackoff    = 10 * time.Minute
	fundamentalsModules = "calendarEvents,assetProfile,quoteType"
)

// yahooCookieURL is the host that hands out the session cookie the crumb
// endpoint requires. A variable so tests can point it at a fake.
var yahooCookieURL = "https://fc.yahoo.com"

type fundamentalsEntry struct {
	f      Fundamentals
	err    error
	loaded time.Time
}

var fundamentalsCache = struct {
	sync.Mutex
	entries map[string]fundamentalsEntry
}{entries: make(map[string]fundamentalsEntry)}

// yahooSession is the cookie+crumb pair; guarded by yahooGate (every use holds it).
var yahooSession struct {
	cookie     string
	crumb      string
	loaded     time.Time
	failedAt   time.Time
	failedWith error
}

// GetFundamentals returns the quote type, sector and next earnings date of a
// Yahoo ticker. Results are cached for 24h, failures for 6h, so a broken
// endpoint or an unknown ticker is not hammered. Callers must treat an error
// as "unknown" — fundamentals are never required.
func GetFundamentals(ctx context.Context, underlying string) (Fundamentals, error) {
	ticker := strings.ToUpper(strings.TrimSpace(underlying))
	if ticker == "" {
		return Fundamentals{}, errors.New("empty underlying")
	}
	if err := ctx.Err(); err != nil {
		return Fundamentals{}, err
	}
	if f, err, ok := cachedFundamentals(ticker); ok {
		return f, err
	}
	select {
	case yahooGate <- struct{}{}:
	case <-ctx.Done():
		return Fundamentals{}, ctx.Err()
	}
	defer func() { <-yahooGate }()
	// Another caller may have filled the entry while this one queued.
	if f, err, ok := cachedFundamentals(ticker); ok {
		return f, err
	}
	f, err := fetchFundamentals(ctx, ticker)
	if err != nil && ctx.Err() != nil {
		return Fundamentals{}, ctx.Err() // cancellation is not a verdict on the ticker
	}
	fundamentalsCache.Lock()
	fundamentalsCache.entries[ticker] = fundamentalsEntry{f: f, err: err, loaded: nowFunc()}
	fundamentalsCache.Unlock()
	return f, err
}

func cachedFundamentals(ticker string) (Fundamentals, error, bool) {
	fundamentalsCache.Lock()
	defer fundamentalsCache.Unlock()
	e, ok := fundamentalsCache.entries[ticker]
	if !ok {
		return Fundamentals{}, nil, false
	}
	ttl := fundamentalsTTL
	if e.err != nil {
		ttl = fundamentalsNegTTL
	}
	if nowFunc().Sub(e.loaded) >= ttl {
		return Fundamentals{}, nil, false
	}
	return e.f, e.err, true
}

type quoteSummaryResponse struct {
	QuoteSummary struct {
		Result []struct {
			CalendarEvents struct {
				Earnings struct {
					EarningsDate []struct {
						Raw int64 `json:"raw"`
					} `json:"earningsDate"`
				} `json:"earnings"`
			} `json:"calendarEvents"`
			AssetProfile struct {
				Sector string `json:"sector"`
			} `json:"assetProfile"`
			QuoteType struct {
				QuoteType string `json:"quoteType"`
			} `json:"quoteType"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"quoteSummary"`
}

// fetchFundamentals runs with yahooGate held. A rejected crumb (401/403) is
// refreshed once.
func fetchFundamentals(ctx context.Context, ticker string) (Fundamentals, error) {
	for attempt := 0; attempt < 2; attempt++ {
		if err := ensureYahooSession(ctx); err != nil {
			return Fundamentals{}, err
		}
		q := url.Values{"modules": {fundamentalsModules}, "crumb": {yahooSession.crumb}}
		body, status, err := yahooGet(ctx, yahooBaseURL, "/v10/finance/quoteSummary/"+url.PathEscape(ticker), q, yahooSession.cookie)
		if err != nil {
			return Fundamentals{}, err
		}
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			yahooSession.crumb, yahooSession.cookie, yahooSession.loaded = "", "", time.Time{}
			continue
		}
		if status != http.StatusOK {
			return Fundamentals{}, fmt.Errorf("Yahoo quoteSummary %s: HTTP %d", ticker, status)
		}
		var resp quoteSummaryResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return Fundamentals{}, fmt.Errorf("Yahoo quoteSummary %s: %w", ticker, err)
		}
		if e := resp.QuoteSummary.Error; e != nil {
			return Fundamentals{}, fmt.Errorf("Yahoo quoteSummary %s: %s: %s", ticker, e.Code, e.Description)
		}
		if len(resp.QuoteSummary.Result) == 0 {
			return Fundamentals{}, fmt.Errorf("Yahoo quoteSummary %s: no result", ticker)
		}
		r := resp.QuoteSummary.Result[0]
		f := Fundamentals{QuoteType: r.QuoteType.QuoteType, Sector: r.AssetProfile.Sector}
		now := nowFunc()
		for _, d := range r.CalendarEvents.Earnings.EarningsDate {
			if d.Raw <= 0 {
				continue
			}
			t := time.Unix(d.Raw, 0).UTC()
			if t.Before(now) {
				continue
			}
			if f.NextEarnings.IsZero() || t.Before(f.NextEarnings) {
				f.NextEarnings = t
			}
		}
		return f, nil
	}
	return Fundamentals{}, fmt.Errorf("Yahoo quoteSummary %s: crumb rejected", ticker)
}

// ensureYahooSession obtains the cookie and crumb (cached for crumbTTL). After
// a failure it keeps returning that failure for crumbFailBackoff without
// touching the network.
func ensureYahooSession(ctx context.Context) error {
	now := nowFunc()
	if yahooSession.crumb != "" && now.Sub(yahooSession.loaded) < crumbTTL {
		return nil
	}
	if yahooSession.failedWith != nil && now.Sub(yahooSession.failedAt) < crumbFailBackoff {
		return yahooSession.failedWith
	}
	cookie, crumb, err := fetchYahooCrumb(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		yahooSession.failedAt, yahooSession.failedWith = now, err
		return err
	}
	yahooSession.cookie, yahooSession.crumb, yahooSession.loaded, yahooSession.failedWith = cookie, crumb, now, nil
	return nil
}

func fetchYahooCrumb(ctx context.Context) (cookie, crumb string, err error) {
	// The cookie host usually answers 404 — only its Set-Cookie matters.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, yahooCookieURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("Yahoo cookie: %w", err)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	var parts []string
	for _, c := range resp.Cookies() {
		parts = append(parts, c.Name+"="+c.Value)
	}
	if len(parts) == 0 {
		return "", "", errors.New("Yahoo cookie: no cookie returned")
	}
	cookie = strings.Join(parts, "; ")
	body, status, err := yahooGet(ctx, yahooBaseURL, "/v1/test/getcrumb", nil, cookie)
	if err != nil {
		return "", "", fmt.Errorf("Yahoo crumb: %w", err)
	}
	if status != http.StatusOK {
		return "", "", fmt.Errorf("Yahoo crumb: HTTP %d", status)
	}
	crumb = strings.TrimSpace(string(body))
	// An HTML or JSON error page, or an empty body, is not a crumb.
	if crumb == "" || len(crumb) > 64 || strings.ContainsAny(crumb, "<{ \n") {
		return "", "", errors.New("Yahoo crumb: unexpected response")
	}
	return cookie, crumb, nil
}

// yahooGet performs a GET with the Yahoo headers and an optional cookie and
// returns the (size-limited) body and status; non-200 is not an error here.
func yahooGet(ctx context.Context, base, path string, query url.Values, cookie string) ([]byte, int, error) {
	endpoint := strings.TrimRight(base, "/") + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return body, resp.StatusCode, err
}

package breakout

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The package's other tests swap binanceHTTP per test and expect every call to
// reach it; the response caches are exercised explicitly below.
func init() { cacheEnabled = false }

type cacheRT func(*http.Request) (*http.Response, error)

func (f cacheRT) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func cacheResp(body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}

// withCache enables the caches with a controllable clock and fake HTTP client.
func withCache(t *testing.T, now *time.Time, rt cacheRT) {
	t.Helper()
	oldHTTP, oldNow := binanceHTTP, cacheNow
	cacheEnabled = true
	resetResponseCaches()
	binanceHTTP = &http.Client{Transport: rt}
	cacheNow = func() time.Time { return *now }
	t.Cleanup(func() {
		cacheEnabled = false
		binanceHTTP, cacheNow = oldHTTP, oldNow
		resetResponseCaches()
	})
}

// klineRow builds one Binance kline row.
func klineRow(openMs, closeMs int64) string {
	return fmt.Sprintf(`[%d,"1","2","0.5","1.5","10",%d,"15","5","4","6","0"]`, openMs, closeMs)
}

func TestKlineCacheServesUntilFormingBarCloses(t *testing.T) {
	// Real-clock based because fetchKlines filters the forming bar with time.Now().
	barMs := int64(15 * 60 * 1000)
	nowMs := time.Now().UnixMilli()
	open := nowMs - nowMs%barMs // current 15m bar (forming)
	closeFormingMs := open + barMs - 1
	var calls atomic.Int32
	body := "[" + klineRow(open-barMs, open-1) + "," + klineRow(open, closeFormingMs) + "]"
	clock := time.Now()
	withCache(t, &clock, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return cacheResp(body), nil
	})

	first, err := fetchKlines("https://fapi.binance.com/fapi/v1/klines", "BTCUSDT", "15m", 2)
	if err != nil || len(first) != 1 {
		t.Fatalf("first fetch: %v %v (forming bar must be dropped)", first, err)
	}
	// Same request (another scanner / next pass inside the bar): cache hit, and a defensive copy.
	first[0].Close = -1
	second, err := fetchKlines("https://fapi.binance.com/fapi/v1/klines", "BTCUSDT", "15m", 2)
	if err != nil || calls.Load() != 1 {
		t.Fatalf("second fetch must hit the cache: calls=%d err=%v", calls.Load(), err)
	}
	if second[0].Close == -1 {
		t.Fatal("cache returned aliased slice; callers must not be able to corrupt it")
	}
	// Different limit / symbol are different keys.
	fetchKlines("https://fapi.binance.com/fapi/v1/klines", "BTCUSDT", "15m", 3)
	fetchKlines("https://fapi.binance.com/fapi/v1/klines", "ETHUSDT", "15m", 2)
	if calls.Load() != 3 {
		t.Fatalf("distinct keys must fetch: calls=%d", calls.Load())
	}
	// Past the forming bar's close (+slack) the entry is stale.
	clock = time.UnixMilli(closeFormingMs).Add(klineCacheSlack + time.Millisecond)
	fetchKlines("https://fapi.binance.com/fapi/v1/klines", "BTCUSDT", "15m", 2)
	if calls.Load() != 4 {
		t.Fatalf("expired entry must refetch: calls=%d", calls.Load())
	}
}

func TestKlineCacheSkipsErrorsAndSeriesWithoutFormingBar(t *testing.T) {
	var calls atomic.Int32
	clock := time.Now()
	fail := true
	closedOnly := "[" + klineRow(1, 2) + "]"
	withCache(t, &clock, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if fail {
			return &http.Response{StatusCode: 429, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"code":-1003}`))}, nil
		}
		return cacheResp(closedOnly), nil
	})
	if _, err := fetchKlines("https://fapi.binance.com/fapi/v1/klines", "AUSDT", "1h", 5); err == nil {
		t.Fatal("429 must surface as an error")
	}
	fail = false
	for i := 0; i < 2; i++ {
		if _, err := fetchKlines("https://fapi.binance.com/fapi/v1/klines", "AUSDT", "1h", 5); err != nil {
			t.Fatal(err)
		}
	}
	// 1 failed + 2 successful fetches: an all-closed series has no known expiry, so it is never cached.
	if calls.Load() != 3 {
		t.Fatalf("errors / no-forming-bar results must not be cached: calls=%d", calls.Load())
	}
}

func TestBodyCacheTTLHitMissAndErrors(t *testing.T) {
	var calls atomic.Int32
	clock := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	status := 200
	withCache(t, &clock, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if status != 200 {
			return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		}
		return cacheResp(`[{"symbol":"BTCUSDT"}]`), nil
	})
	var out []struct {
		Symbol string `json:"symbol"`
	}
	const url = "https://fapi.binance.com/fapi/v1/ticker/24hr"
	for i := 0; i < 3; i++ {
		out = nil
		if err := fetchJSONCached(url, 2*time.Minute, &out); err != nil || len(out) != 1 || out[0].Symbol != "BTCUSDT" {
			t.Fatalf("call %d: %v %v", i, out, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 upstream call within TTL, got %d", calls.Load())
	}
	clock = clock.Add(2*time.Minute - time.Second)
	fetchJSONCached(url, 2*time.Minute, &out)
	if calls.Load() != 1 {
		t.Fatalf("still inside TTL: calls=%d", calls.Load())
	}
	clock = clock.Add(2 * time.Second)
	fetchJSONCached(url, 2*time.Minute, &out)
	if calls.Load() != 2 {
		t.Fatalf("past TTL must refetch: calls=%d", calls.Load())
	}
	// A failing refresh surfaces the error (no stale serve) and caches nothing.
	status = 429
	clock = clock.Add(3 * time.Minute)
	if err := fetchJSONCached(url, 2*time.Minute, &out); err == nil {
		t.Fatal("error must propagate")
	}
	status = 200
	if err := fetchJSONCached(url, 2*time.Minute, &out); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 4 {
		t.Fatalf("error response was cached: calls=%d", calls.Load())
	}
	// Distinct URLs do not share entries.
	fetchJSONCached(url+"?symbol=X", 2*time.Minute, &out)
	if calls.Load() != 5 {
		t.Fatalf("calls=%d", calls.Load())
	}
}

func TestCacheSweepBoundsMemory(t *testing.T) {
	clock := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	withCache(t, &clock, func(r *http.Request) (*http.Response, error) { return cacheResp("[]"), nil })
	for i := 0; i < cacheMaxEntries+50; i++ {
		bodyCachePut(fmt.Sprintf("k%d", i), []byte("[]"), time.Hour)
	}
	respCacheMu.Lock()
	n := len(bodyCache) + len(klineCache)
	respCacheMu.Unlock()
	if n > cacheMaxEntries {
		t.Fatalf("cache grew past its bound: %d", n)
	}
}

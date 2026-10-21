package kernel

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"nofx/store"
)

// allowLoopback swaps the SSRF guard and the SSRF-safe client for plain ones so httptest servers on
// 127.0.0.1 are reachable; restored when the test ends.
func allowLoopback(t *testing.T, timeout time.Duration) {
	t.Helper()
	oldValidate, oldClient, oldTimeout := externalValidateURL, externalHTTPClient, externalDataTimeout
	externalValidateURL = func(string) error { return nil }
	externalHTTPClient = func(d time.Duration) *http.Client { return &http.Client{Timeout: d} }
	externalDataTimeout = timeout
	t.Cleanup(func() {
		externalValidateURL, externalHTTPClient, externalDataTimeout = oldValidate, oldClient, oldTimeout
	})
}

func engineWith(sources ...store.ExternalDataSource) *StrategyEngine {
	cfg := store.GetDefaultStrategyConfig("en")
	cfg.Indicators.ExternalDataSources = sources
	return NewStrategyEngine(&cfg)
}

func TestFetchExternalDataHeadersMethodAndDataPath(t *testing.T) {
	allowLoopback(t, 2*time.Second)
	var gotKey, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey, gotMethod = r.Header.Get("X-Api-Key"), r.Method
		w.Write([]byte(`{"data":{"ranked":["BTC"],"coins":{"BTC":{"funding_ann_pct":7.19}}}}`))
	}))
	defer srv.Close()

	e := engineWith(store.ExternalDataSource{Name: "board", URL: srv.URL, Headers: map[string]string{"X-Api-Key": "test-key"}, DataPath: "data"})
	data, err := e.FetchExternalData()
	if err != nil {
		t.Fatal(err)
	}
	if gotKey != "test-key" || gotMethod != http.MethodGet {
		t.Fatalf("header %q method %q, want test-key and GET (empty method)", gotKey, gotMethod)
	}
	board, ok := data["board"].(map[string]interface{})
	if !ok || board["coins"] == nil {
		t.Fatalf("data_path did not unwrap the envelope: %#v", data["board"])
	}
	if _, wrapped := board["data"]; wrapped {
		t.Fatalf("envelope leaked through data_path")
	}
}

func TestFetchExternalDataRefreshSecsIsCacheTTL(t *testing.T) {
	allowLoopback(t, 2*time.Second)
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write([]byte(`{"n":1}`))
	}))
	defer srv.Close()

	cached := engineWith(store.ExternalDataSource{Name: "a", URL: srv.URL, RefreshSecs: 300})
	cached.FetchExternalData()
	cached.FetchExternalData()
	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Fatalf("refresh_secs=300: %d fetches in two cycles, want 1", n)
	}

	atomic.StoreInt32(&hits, 0)
	every := engineWith(store.ExternalDataSource{Name: "b", URL: srv.URL})
	every.FetchExternalData()
	every.FetchExternalData()
	if n := atomic.LoadInt32(&hits); n != 2 {
		t.Fatalf("refresh_secs=0: %d fetches in two cycles, want 2", n)
	}

	// an expired entry is fetched again
	atomic.StoreInt32(&hits, 0)
	cached.externalMu.Lock()
	for k, v := range cached.externalCache {
		v.fetchedAt = v.fetchedAt.Add(-301 * time.Second)
		cached.externalCache[k] = v
	}
	cached.externalMu.Unlock()
	cached.FetchExternalData()
	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Fatalf("expired entry: %d fetches, want 1", n)
	}
}

func TestFetchExternalDataRejectsNon2xxAndDoesNotCacheFailures(t *testing.T) {
	allowLoopback(t, 2*time.Second)
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusPaymentRequired)
		w.Write([]byte(`{"error":"key balance too low"}`))
	}))
	defer srv.Close()

	e := engineWith(store.ExternalDataSource{Name: "a", URL: srv.URL, RefreshSecs: 300})
	data, _ := e.FetchExternalData()
	if _, ok := data["a"]; ok {
		t.Fatalf("a 402 body was passed on as data: %#v", data["a"])
	}
	e.FetchExternalData()
	if n := atomic.LoadInt32(&hits); n != 2 {
		t.Fatalf("a failure was cached: %d fetches, want 2", n)
	}
}

func TestFetchExternalDataCapsTheBody(t *testing.T) {
	allowLoopback(t, 2*time.Second)
	big := `{"x":"` + strings.Repeat("a", externalDataMaxBody) + `"}`
	fits := `{"x":"` + strings.Repeat("a", externalDataMaxBody-8) + `"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/big" {
			w.Write([]byte(big))
			return
		}
		w.Write([]byte(fits))
	}))
	defer srv.Close()

	e := engineWith(
		store.ExternalDataSource{Name: "big", URL: srv.URL + "/big"},
		store.ExternalDataSource{Name: "fits", URL: srv.URL + "/fits"},
	)
	data, _ := e.FetchExternalData()
	if _, ok := data["big"]; ok {
		t.Fatalf("a body over %d bytes was accepted", externalDataMaxBody)
	}
	if _, ok := data["fits"]; !ok {
		t.Fatalf("a body of exactly %d bytes was refused", len(fits))
	}
}

func TestFetchExternalDataFixedTimeout(t *testing.T) {
	allowLoopback(t, 200*time.Millisecond)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Second)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	// refresh_secs is a cache TTL now, not the timeout: a large value must not stretch the request
	e := engineWith(store.ExternalDataSource{Name: "slow", URL: srv.URL, RefreshSecs: 600})
	start := time.Now()
	data, _ := e.FetchExternalData()
	if _, ok := data["slow"]; ok {
		t.Fatalf("a response slower than the timeout was accepted")
	}
	if took := time.Since(start); took > 900*time.Millisecond {
		t.Fatalf("fetch took %v, the fixed timeout is 200ms", took)
	}
}

func TestFetchExternalDataKeepsSSRFGuard(t *testing.T) {
	// no allowLoopback: the production guard must refuse a loopback URL
	e := engineWith(store.ExternalDataSource{Name: "local", URL: "http://127.0.0.1:1/x"})
	data, _ := e.FetchExternalData()
	if _, ok := data["local"]; ok {
		t.Fatalf("a loopback source passed the SSRF guard")
	}
}

func TestBuildUserPromptIncludesExternalDataUnderOwnHeading(t *testing.T) {
	e := engineWith(
		store.ExternalDataSource{Name: "market_board", URL: "https://example.invalid/a"},
		store.ExternalDataSource{Name: "btc_signal", URL: "https://example.invalid/b"},
	)
	ctx := &Context{
		CurrentTime: "2026-10-13 12:00:00 UTC",
		Account:     AccountInfo{TotalEquity: 1000, AvailableBalance: 1000},
		ExternalData: map[string]interface{}{
			"btc_signal":   map[string]interface{}{"coin": "BTC", "flags": []interface{}{"liq_spike"}},
			"market_board": map[string]interface{}{"ranked": []interface{}{"BTC", "ETH"}},
		},
	}
	prompt := e.BuildUserPrompt(ctx)
	head := strings.Index(prompt, "## External Data Sources")
	board := strings.Index(prompt, "### market_board\n{\"ranked\":[\"BTC\",\"ETH\"]}")
	btc := strings.Index(prompt, "### btc_signal\n{\"coin\":\"BTC\",\"flags\":[\"liq_spike\"]}")
	end := strings.Index(prompt, "---\n\nNow please analyze")
	if head < 0 || board < 0 || btc < 0 || end < 0 {
		t.Fatalf("external data missing from the user prompt:\n%s", prompt)
	}
	if !(head < board && board < btc && btc < end) {
		t.Fatalf("external data not in strategy order before the decision request:\n%s", prompt)
	}

	empty := e.BuildUserPrompt(&Context{Account: AccountInfo{TotalEquity: 1000}})
	if strings.Contains(empty, "## External Data Sources") {
		t.Fatalf("heading printed with no external data")
	}
}

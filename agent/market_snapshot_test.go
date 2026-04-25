package agent

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestToolGetMarketSnapshotReturnsRealtimeAnalysisContext(t *testing.T) {
	prevBaseURL := binanceFuturesAPIBaseURL
	prevClient := marketDataHTTPClient
	binanceFuturesAPIBaseURL = "https://example.test"
	marketDataHTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := ""
			switch {
			case strings.HasPrefix(req.URL.Path, "/fapi/v1/ticker/24hr"):
				body = `{"symbol":"BTCUSDT","lastPrice":"65000","priceChange":"1200","priceChangePercent":"1.88","highPrice":"66000","lowPrice":"63800","volume":"12345","quoteVolume":"800000000","count":98765}`
			case strings.HasPrefix(req.URL.Path, "/fapi/v1/premiumIndex"):
				body = `{"symbol":"BTCUSDT","markPrice":"65010","indexPrice":"64990","lastFundingRate":"0.00010000","nextFundingTime":1710000000000}`
			case strings.HasPrefix(req.URL.Path, "/fapi/v1/openInterest"):
				body = `{"symbol":"BTCUSDT","openInterest":"45678.9","time":1710000000000}`
			case strings.HasPrefix(req.URL.Path, "/fapi/v1/klines"):
				body = `[[1710000000000,"64000","65100","63900","64500","100",1710000899999],[1710000900000,"64500","65500","64400","65000","120",1710001799999]]`
			default:
				body = `{"error":"not found"}`
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}
	defer func() {
		binanceFuturesAPIBaseURL = prevBaseURL
		marketDataHTTPClient = prevClient
	}()

	a := New(nil, nil, DefaultConfig(), nil)
	raw := a.toolGetMarketSnapshot(`{"symbol":"BTC","interval":"15m","limit":2}`)

	var resp struct {
		Symbol    string  `json:"symbol"`
		Price     float64 `json:"price"`
		Ticker24h struct {
			PriceChangePercent float64 `json:"price_change_percent"`
		} `json:"ticker_24h"`
		PerpMetrics struct {
			FundingRate  float64 `json:"funding_rate"`
			OpenInterest float64 `json:"open_interest"`
		} `json:"perp_metrics"`
		KlineSnapshot struct {
			Interval            string           `json:"interval"`
			Limit               int              `json:"limit"`
			PeriodChangePercent float64          `json:"period_change_percent"`
			RecentKlines        []map[string]any `json:"recent_klines"`
		} `json:"kline_snapshot"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("failed to parse tool response: %v\nraw=%s", err, raw)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected tool error: %s", resp.Error)
	}
	if resp.Symbol != "BTCUSDT" {
		t.Fatalf("expected normalized symbol BTCUSDT, got %s", resp.Symbol)
	}
	if resp.Price != 65000 {
		t.Fatalf("expected price 65000, got %v", resp.Price)
	}
	if resp.Ticker24h.PriceChangePercent != 1.88 {
		t.Fatalf("expected 24h change 1.88, got %v", resp.Ticker24h.PriceChangePercent)
	}
	if resp.PerpMetrics.FundingRate != 0.0001 {
		t.Fatalf("expected funding rate 0.0001, got %v", resp.PerpMetrics.FundingRate)
	}
	if resp.PerpMetrics.OpenInterest != 45678.9 {
		t.Fatalf("expected open interest 45678.9, got %v", resp.PerpMetrics.OpenInterest)
	}
	if resp.KlineSnapshot.Interval != "15m" || resp.KlineSnapshot.Limit != 2 {
		t.Fatalf("unexpected kline snapshot metadata: %+v", resp.KlineSnapshot)
	}
	if len(resp.KlineSnapshot.RecentKlines) != 2 {
		t.Fatalf("expected 2 klines, got %d", len(resp.KlineSnapshot.RecentKlines))
	}
	if resp.KlineSnapshot.PeriodChangePercent <= 0 {
		t.Fatalf("expected positive period change, got %v", resp.KlineSnapshot.PeriodChangePercent)
	}
}

func TestToolGetMarketSnapshotRejectsStockSymbols(t *testing.T) {
	a := New(nil, nil, DefaultConfig(), nil)
	raw := a.toolGetMarketSnapshot(`{"symbol":"AAPL"}`)
	if !strings.Contains(raw, "currently supports crypto symbols only") {
		t.Fatalf("expected stock rejection, got: %s", raw)
	}
}

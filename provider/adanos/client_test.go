package adanos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompareFetchesCryptoSentiment(t *testing.T) {
	var gotPath string
	var gotQuery string
	var gotAPIKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("symbols")
		gotAPIKey = r.Header.Get("X-API-Key")

		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"symbol":          "BTC",
				"source":          "reddit",
				"buzz_score":      82.5,
				"sentiment_score": 0.41,
				"mentions":        123,
				"trend":           "rising",
				"bullish_pct":     67.2,
				"bearish_pct":     18.4,
			},
		})
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk_live_test",
		Source:  SourceRedditCrypto,
		Days:    3,
	})

	result, err := client.Compare(context.Background(), []string{"BTCUSDT", "BTCUSDT"})
	if err != nil {
		t.Fatalf("Compare returned error: %v", err)
	}

	if gotPath != "/reddit/crypto/v1/compare" {
		t.Fatalf("path = %q, want crypto compare endpoint", gotPath)
	}
	if gotQuery != "BTC" {
		t.Fatalf("symbols query = %q, want BTC", gotQuery)
	}
	if gotAPIKey != "sk_live_test" {
		t.Fatalf("API key header = %q, want sk_live_test", gotAPIKey)
	}

	sentiment := result["BTCUSDT"]
	if sentiment == nil {
		t.Fatal("expected BTCUSDT sentiment mapped back from BTC response")
	}
	if sentiment.Source != "reddit" {
		t.Fatalf("source = %q, want reddit", sentiment.Source)
	}
	if sentiment.BuzzScore == nil || *sentiment.BuzzScore != 82.5 {
		t.Fatalf("buzz score = %v, want 82.5", sentiment.BuzzScore)
	}
	if sentiment.Mentions == nil || *sentiment.Mentions != 123 {
		t.Fatalf("mentions = %v, want 123", sentiment.Mentions)
	}
}

func TestCompareSupportsStockSources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/x/stocks/v1/compare" {
			t.Fatalf("path = %q, want x stocks compare endpoint", r.URL.Path)
		}
		if got := r.URL.Query().Get("tickers"); got != "AAPL,NVDA" {
			t.Fatalf("tickers query = %q, want AAPL,NVDA", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"ticker": "AAPL", "sentiment": "0.12", "total_mentions": "42"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk_live_test",
		Source:  SourceXStocks,
	})

	result, err := client.Compare(context.Background(), []string{"AAPL", "NVDA"})
	if err != nil {
		t.Fatalf("Compare returned error: %v", err)
	}
	if _, ok := result["AAPL"]; !ok {
		t.Fatal("expected AAPL sentiment")
	}
}

func TestCompareRequiresAPIKey(t *testing.T) {
	client := NewClient(Config{APIKey: ""})

	result, err := client.Compare(context.Background(), []string{"BTCUSDT"})
	if err == nil {
		t.Fatal("expected missing API key error")
	}
	if len(result) != 0 {
		t.Fatalf("result length = %d, want 0", len(result))
	}
}

func TestCompareReturnsStatusErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk_live_test",
	})

	result, err := client.Compare(context.Background(), []string{"BTCUSDT"})
	if err == nil {
		t.Fatal("expected non-2xx status error")
	}
	if len(result) != 0 {
		t.Fatalf("result length = %d, want 0", len(result))
	}
}

func TestNormalizeSourceDefaultsToRedditCrypto(t *testing.T) {
	if got := NormalizeSource(""); got != SourceRedditCrypto {
		t.Fatalf("empty source = %q, want %q", got, SourceRedditCrypto)
	}
	if got := NormalizeSource("x-stocks"); got != SourceXStocks {
		t.Fatalf("x-stocks source = %q, want %q", got, SourceXStocks)
	}
}

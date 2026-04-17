package kernel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"nofx/market"
	"nofx/provider/adanos"
	"nofx/store"
	"strings"
	"testing"
)

func TestFetchAdanosSentimentBatchDisabled(t *testing.T) {
	config := store.GetDefaultStrategyConfig("en")
	config.Indicators.EnableAdanosSentiment = false
	engine := NewStrategyEngine(&config)

	got := engine.FetchAdanosSentimentBatch([]string{"BTCUSDT"})
	if len(got) != 0 {
		t.Fatalf("disabled Adanos sentiment returned %d entries, want 0", len(got))
	}
}

func TestFetchAdanosSentimentBatchUsesOptionalAPIKeyAndLimit(t *testing.T) {
	var gotPath string
	var gotSymbols string
	var gotDays string
	var gotAPIKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotSymbols = r.URL.Query().Get("symbols")
		gotDays = r.URL.Query().Get("days")
		gotAPIKey = r.Header.Get("X-API-Key")

		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"symbol":          "BTC",
				"source":          "reddit_crypto",
				"buzz_score":      88.5,
				"sentiment_score": 0.42,
				"mentions":        144,
				"trend":           "rising",
			},
		})
	}))
	defer server.Close()

	t.Setenv("ADANOS_BASE_URL", server.URL)

	config := store.GetDefaultStrategyConfig("en")
	config.Indicators.EnableAdanosSentiment = true
	config.Indicators.AdanosAPIKey = "sk_live_test"
	config.Indicators.AdanosSource = adanos.SourceRedditCrypto
	config.Indicators.AdanosDays = 3
	config.Indicators.AdanosMaxSymbols = 1
	engine := NewStrategyEngine(&config)

	got := engine.FetchAdanosSentimentBatch([]string{"BTCUSDT", "ETHUSDT"})
	if gotPath != "/reddit/crypto/v1/compare" {
		t.Fatalf("path = %q, want Adanos crypto compare endpoint", gotPath)
	}
	if gotSymbols != "BTC" {
		t.Fatalf("symbols query = %q, want only BTC because max symbols is 1", gotSymbols)
	}
	if gotDays != "3" {
		t.Fatalf("days query = %q, want 3", gotDays)
	}
	if gotAPIKey != "sk_live_test" {
		t.Fatalf("API key header = %q, want configured key", gotAPIKey)
	}
	if got["BTCUSDT"] == nil {
		t.Fatal("expected BTCUSDT sentiment mapped from BTC response")
	}
}

func TestFetchAdanosSentimentBatchIgnoresBlankSymbols(t *testing.T) {
	var gotSymbols string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSymbols = r.URL.Query().Get("symbols")
		_ = json.NewEncoder(w).Encode([]map[string]any{{"symbol": "BTC", "buzz_score": 72.0}})
	}))
	defer server.Close()

	t.Setenv("ADANOS_BASE_URL", server.URL)

	config := store.GetDefaultStrategyConfig("en")
	config.Indicators.EnableAdanosSentiment = true
	config.Indicators.AdanosAPIKey = "sk_live_test"
	engine := NewStrategyEngine(&config)

	got := engine.FetchAdanosSentimentBatch([]string{"", "  ", "$BTC"})
	if gotSymbols != "BTC" {
		t.Fatalf("symbols query = %q, want only BTC", gotSymbols)
	}
	if got["BTCUSDT"] == nil {
		t.Fatal("expected BTCUSDT sentiment")
	}
}

func TestFetchAdanosSentimentBatchNormalizesCryptoPairs(t *testing.T) {
	var gotSymbols string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSymbols = r.URL.Query().Get("symbols")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"symbol": "BTC", "buzz_score": 72.0},
			{"symbol": "ETH", "buzz_score": 71.0},
			{"symbol": "SOL", "buzz_score": 70.0},
		})
	}))
	defer server.Close()

	t.Setenv("ADANOS_BASE_URL", server.URL)

	config := store.GetDefaultStrategyConfig("en")
	config.Indicators.EnableAdanosSentiment = true
	config.Indicators.AdanosAPIKey = "sk_live_test"
	engine := NewStrategyEngine(&config)

	got := engine.FetchAdanosSentimentBatch([]string{"BTC/USD", "ETH-USDC", "SOL_USDT"})
	if gotSymbols != "BTC,ETH,SOL" {
		t.Fatalf("symbols query = %q, want normalized crypto bases", gotSymbols)
	}
	for _, symbol := range []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"} {
		if got[symbol] == nil {
			t.Fatalf("expected %s sentiment", symbol)
		}
	}
}

func TestFetchAdanosSentimentBatchFallsBackToEnvAPIKey(t *testing.T) {
	var gotAPIKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-API-Key")
		_ = json.NewEncoder(w).Encode([]map[string]any{{"symbol": "BTC", "buzz_score": 72.0}})
	}))
	defer server.Close()

	t.Setenv("ADANOS_BASE_URL", server.URL)
	t.Setenv("ADANOS_API_KEY", "sk_live_env")

	config := store.GetDefaultStrategyConfig("en")
	config.Indicators.EnableAdanosSentiment = true
	config.Indicators.AdanosAPIKey = ""
	engine := NewStrategyEngine(&config)

	got := engine.FetchAdanosSentimentBatch([]string{"BTCUSDT"})
	if gotAPIKey != "sk_live_env" {
		t.Fatalf("API key header = %q, want env key", gotAPIKey)
	}
	if got["BTCUSDT"] == nil {
		t.Fatal("expected BTCUSDT sentiment")
	}
}

func TestFetchAdanosSentimentBatchKeepsStockTickers(t *testing.T) {
	var gotTickers string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTickers = r.URL.Query().Get("tickers")
		_ = json.NewEncoder(w).Encode([]map[string]any{{"ticker": "AAPL", "buzz_score": 72.0}})
	}))
	defer server.Close()

	t.Setenv("ADANOS_BASE_URL", server.URL)

	config := store.GetDefaultStrategyConfig("en")
	config.Indicators.EnableAdanosSentiment = true
	config.Indicators.AdanosAPIKey = "sk_live_test"
	config.Indicators.AdanosSource = adanos.SourceXStocks
	engine := NewStrategyEngine(&config)

	got := engine.FetchAdanosSentimentBatch([]string{"AAPL", "xyz:MSFT", "NVDAUSDT"})
	if gotTickers != "AAPL,MSFT,NVDA" {
		t.Fatalf("tickers query = %q, want stock tickers without crypto quote suffixes", gotTickers)
	}
	if got["AAPL"] == nil {
		t.Fatal("expected AAPL sentiment")
	}
}

func TestFetchAdanosSentimentForContextPrioritizesPositions(t *testing.T) {
	var gotSymbols string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSymbols = r.URL.Query().Get("symbols")
		_ = json.NewEncoder(w).Encode([]map[string]any{{"symbol": "ETH", "buzz_score": 72.0}})
	}))
	defer server.Close()

	t.Setenv("ADANOS_BASE_URL", server.URL)

	config := store.GetDefaultStrategyConfig("en")
	config.Indicators.EnableAdanosSentiment = true
	config.Indicators.AdanosAPIKey = "sk_live_test"
	config.Indicators.AdanosMaxSymbols = 1
	engine := NewStrategyEngine(&config)

	ctx := &Context{
		Positions:      []PositionInfo{{Symbol: "ETHUSDT"}},
		CandidateCoins: []CandidateCoin{{Symbol: "BTCUSDT", Sources: []string{"static"}}},
		MarketDataMap: map[string]*market.Data{
			"ADAUSDT": {Symbol: "ADAUSDT"},
		},
	}

	got := engine.FetchAdanosSentimentForContext(ctx)
	if gotSymbols != "ETH" {
		t.Fatalf("symbols query = %q, want position symbol ETH first", gotSymbols)
	}
	if got["ETHUSDT"] == nil {
		t.Fatal("expected ETHUSDT sentiment")
	}
}

func TestBuildUserPromptIncludesAdanosSentimentWithoutAPIKey(t *testing.T) {
	config := store.GetDefaultStrategyConfig("en")
	config.Indicators.EnableAdanosSentiment = true
	config.Indicators.AdanosAPIKey = "sk_live_secret"
	engine := NewStrategyEngine(&config)

	ctx := &Context{
		CurrentTime:    "2026-04-17 08:00:00 UTC",
		RuntimeMinutes: 10,
		CallCount:      2,
		Account: AccountInfo{
			TotalEquity:      1000,
			AvailableBalance: 800,
			PositionCount:    0,
		},
		CandidateCoins: []CandidateCoin{{Symbol: "BTCUSDT", Sources: []string{"static"}}},
		MarketDataMap: map[string]*market.Data{
			"BTCUSDT": {
				Symbol:       "BTCUSDT",
				CurrentPrice: 65000,
			},
		},
		AdanosSentimentMap: map[string]*adanos.Sentiment{
			"BTCUSDT": {
				Symbol:         "BTCUSDT",
				Source:         adanos.SourceRedditCrypto,
				BuzzScore:      floatPtr(88.5),
				SentimentScore: floatPtr(0.42),
				Mentions:       intPtr(144),
				Trend:          "rising",
			},
		},
	}

	prompt := engine.BuildUserPrompt(ctx)
	for _, want := range []string{
		"Adanos Market Sentiment",
		"buzz_score=88.50",
		"sentiment_score=0.420",
		"mentions=144",
		"trend=rising",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "sk_live_secret") {
		t.Fatal("prompt must not include Adanos API keys")
	}
}

func TestBuildUserPromptFindsStockSentimentForXyzSymbols(t *testing.T) {
	config := store.GetDefaultStrategyConfig("en")
	config.Indicators.EnableAdanosSentiment = true
	config.Indicators.AdanosSource = adanos.SourceXStocks
	engine := NewStrategyEngine(&config)

	ctx := &Context{
		CurrentTime:    "2026-04-17 08:00:00 UTC",
		RuntimeMinutes: 10,
		CallCount:      2,
		Account: AccountInfo{
			TotalEquity:      1000,
			AvailableBalance: 800,
		},
		CandidateCoins: []CandidateCoin{{Symbol: "xyz:AAPL", Sources: []string{"static"}}},
		MarketDataMap: map[string]*market.Data{
			"xyz:AAPL": {
				Symbol:       "xyz:AAPL",
				CurrentPrice: 180,
			},
		},
		AdanosSentimentMap: map[string]*adanos.Sentiment{
			"AAPL": {
				Symbol:    "AAPL",
				Source:    adanos.SourceXStocks,
				BuzzScore: floatPtr(70.5),
			},
		},
	}

	prompt := engine.BuildUserPrompt(ctx)
	if !strings.Contains(prompt, "AAPL Adanos Market Sentiment") {
		t.Fatalf("prompt missing stock Adanos sentiment:\n%s", prompt)
	}
}

func floatPtr(value float64) *float64 {
	return &value
}

func intPtr(value int) *int {
	return &value
}

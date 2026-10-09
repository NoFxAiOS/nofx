package vergex

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestWinrateInvalidInputsNeverReachPayment(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusPaymentRequired)
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	for _, modify := range []func(*WinrateQuery){
		func(q *WinrateQuery) { q.MarketType = "typo" },
		func(q *WinrateQuery) { q.CostMin, q.CostMax = math.NaN(), 108 },
		func(q *WinrateQuery) { q.CostMin, q.CostMax = 92, math.Inf(1) },
		func(q *WinrateQuery) { q.CostMin, q.CostMax = 92.5, 108 },
		func(q *WinrateQuery) { q.MinRoundTrips = -1 },
		func(q *WinrateQuery) { q.MinRoundTrips = 10001 },
		func(q *WinrateQuery) { q.Chain = "testnet" },
	} {
		q := WinrateQuery{MarketType: "hip3_perp", Symbol: "NVDA"}
		modify(&q)
		_, err := client.GetHolderWinrateMap(context.Background(), q)
		var validation *WinrateValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("expected typed validation error, got %v", err)
		}
	}
	for _, modify := range []func(*WinrateHoldersQuery){
		func(q *WinrateHoldersQuery) { q.RowEnd = 20 },
		func(q *WinrateHoldersQuery) { q.ColumnEnd = 18 },
		func(q *WinrateHoldersQuery) { q.Offset = -1 },
		func(q *WinrateHoldersQuery) { q.Offset = 1000001 },
		func(q *WinrateHoldersQuery) { q.Limit = 500 },
		func(q *WinrateHoldersQuery) { q.Limit = -1 },
	} {
		q := WinrateHoldersQuery{WinrateQuery: WinrateQuery{Symbol: "BTC", MarketType: "core_perp"}, SnapshotID: "XGSZMBVFLOZRFYMWGC44OBBQNX", RowEnd: 19, ColumnEnd: 17}
		modify(&q)
		_, err := client.GetHolderWinrateHolders(context.Background(), q)
		var validation *WinrateValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("expected typed validation error, got %v", err)
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("invalid input made %d upstream requests", requests.Load())
	}
}

func TestWinrateQualityIncludedInAIPrompt(t *testing.T) {
	now := time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC)
	data := map[string]any{"coverage": "complete", "asOf": now.Format(time.RFC3339), "positionsAsOf": now.Format(time.RFC3339), "priceAsOf": now.Format(time.RFC3339), "historyMode": "rolling", "staleHistoryCount": 0, "minRoundTrips": 3}
	fresh := formatWinrateQuality(data, now)
	if strings.Contains(fresh, "WARNING") {
		t.Fatalf("fresh complete data warned: %s", fresh)
	}
	data["coverage"], data["staleHistoryCount"] = "partial", 12
	data["positionsAsOf"] = now.Add(-time.Hour).Format(time.RFC3339)
	out := formatWinrateQuality(data, now)
	for _, want := range []string{"coverage=partial", "12 stale holder histories", "positionsAsOf is older than 15 minutes", "MinRoundTrips", "HistoryMode", "not a forecast"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
	for _, cells := range []any{[]any{}, []any{map[string]any{"row": 0, "column": 1, "long": map[string]any{"notional": "5", "count": 1}}}} {
		data["cells"] = cells
		raw, _ := json.Marshal(map[string]any{"data": data})
		prompt := FormatAnalysisForAI(&MarketAnalysis{Symbol: "NVDA", Winrate: raw})
		if !strings.Contains(prompt, "DATA QUALITY WARNING") || !strings.Contains(prompt, "coverage=partial") {
			t.Fatalf("quality lost in final AI prompt: %s", prompt)
		}
	}
	unknown := formatWinrateQuality(map[string]any{}, now)
	if !strings.Contains(unknown, "coverage=unknown") || !strings.Contains(unknown, "history freshness is unknown") {
		t.Fatal(unknown)
	}
}

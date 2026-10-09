package vergex

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestParseDirectionChangeLeaderboard(t *testing.T) {
	body := []byte(`{
		"band":15,"universeSize":30,"rankBy":"directionScore","asOf":1786781428957,
		"items":[
			{"market":{"marketType":"hip3_perp","symbol":"xyz:NVDA"},"symbol":"xyz:NVDA","bias":"bullish","directionScore":4,"bullishCount":4,"bearishCount":0,"neutralCount":1,"markPrice":224.5,"rank":1,"oiRank":14},
			{"market":{"marketType":"core_perp","symbol":"BTC"},"symbol":"BTC","bias":"bearish","directionScore":-4,"bullishCount":0,"bearishCount":4,"neutralCount":1,"markPrice":63047,"rank":2,"oiRank":1}
		]
	}`)

	board, err := ParseDirectionChangeLeaderboard(body)
	if err != nil {
		t.Fatal(err)
	}
	if board.Band != 15 || board.UniverseSize != 30 || board.RankBy != "directionScore" || len(board.Items) != 2 {
		t.Fatalf("unexpected board metadata/items: %+v", board)
	}
	nvda := board.Items[0]
	if nvda.Symbol != "NVDA" || nvda.APISymbol != "xyz:NVDA" || nvda.MarketType != "hip3_perp" || nvda.Score != 4 || nvda.BullishCount != 4 || nvda.MarkPrice != 224.5 || nvda.OIRank != 14 {
		t.Fatalf("unexpected parsed NVDA item: %+v", nvda)
	}
	filtered := FilterDirectionChangeItems(board.Items, "all", 30)
	if len(filtered) != 2 || filtered[0].Category != "stock" || filtered[1].Category != "crypto" {
		t.Fatalf("unexpected filtered items: %+v", filtered)
	}
}

func TestDirectionChangeRequestsUseExactPathsAndParams(t *testing.T) {
	type seenRequest struct {
		path  string
		query url.Values
	}
	var mu sync.Mutex
	var seen []seenRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, seenRequest{path: r.URL.Path, query: r.URL.Query()})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case DirectionLeaderboardPath:
			fmt.Fprint(w, `{"band":15,"universeSize":0,"rankBy":"directionScore","asOf":1,"items":[]}`)
		case DirectionCurrentPath:
			fmt.Fprint(w, `{"symbol":"BTC","direction":"bearish"}`)
		case DirectionHistoryPath:
			fmt.Fprint(w, `{"items":[],"pagination":{"current_page":2,"page_size":100,"total_pages":0,"total_items":0}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := testClient(t, server.URL)
	if _, err := client.GetDirectionChangeLeaderboard(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetDirectionChangeCurrent(context.Background(), " BTC "); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetDirectionChangeHistory(context.Background(), "BTC", "reversal", 2, 500); err != nil {
		t.Fatal(err)
	}

	if len(seen) != 3 {
		t.Fatalf("requests=%d, want 3", len(seen))
	}
	if seen[0].path != DirectionLeaderboardPath || len(seen[0].query) != 0 {
		t.Fatalf("leaderboard request=%+v", seen[0])
	}
	if seen[1].path != DirectionCurrentPath || seen[1].query.Get("symbol") != "BTC" || len(seen[1].query) != 1 {
		t.Fatalf("current request=%+v", seen[1])
	}
	q := seen[2].query
	if seen[2].path != DirectionHistoryPath || q.Get("symbol") != "BTC" || q.Get("type") != "reversal" || q.Get("page") != "2" || q.Get("page_size") != "100" || len(q) != 4 {
		t.Fatalf("history request=%+v", seen[2])
	}
}

func TestDirectionChangeValidationAndHistoryDefaults(t *testing.T) {
	client := testClient(t, "http://127.0.0.1:1")
	if _, err := client.GetDirectionChangeCurrent(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "symbol is required") {
		t.Fatalf("current validation err=%v", err)
	}
	if _, err := client.GetDirectionChangeHistory(context.Background(), "BTC", "bad", 1, 20); err == nil || !strings.Contains(err.Error(), "type must be") {
		t.Fatalf("history type err=%v", err)
	}
}

func TestDirectionChangeLiveIntegration(t *testing.T) {
	if os.Getenv("VERGEX_INTEGRATION") != "1" {
		t.Skip("set VERGEX_INTEGRATION=1 to run paid claw402 integration")
	}
	client, err := NewClient("", os.Getenv("CLAW402_WALLET_KEY"), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	board, err := client.GetDirectionChangeLeaderboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if board.Band <= 0 || board.UniverseSize <= 0 || len(board.Items) == 0 || len(board.Items) > MaxDirectionChangeItems {
		t.Fatalf("invalid live leaderboard metadata: band=%d universe=%d items=%d", board.Band, board.UniverseSize, len(board.Items))
	}
	symbol := board.Items[0].APISymbol
	currentRaw, err := client.GetDirectionChangeCurrent(ctx, symbol)
	if err != nil {
		t.Fatalf("current(%s): %v", symbol, err)
	}
	var current struct {
		Symbol    string `json:"symbol"`
		Direction string `json:"direction"`
	}
	if err := json.Unmarshal(currentRaw, &current); err != nil {
		t.Fatal(err)
	}
	if current.Symbol == "" || current.Direction == "" {
		t.Fatalf("invalid current response: %s", currentRaw)
	}
	historyRaw, err := client.GetDirectionChangeHistory(ctx, symbol, "all", 1, 2)
	if err != nil {
		t.Fatalf("history(%s): %v", symbol, err)
	}
	var history struct {
		Items      []json.RawMessage `json:"items"`
		Pagination struct {
			CurrentPage int `json:"current_page"`
			PageSize    int `json:"page_size"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(historyRaw, &history); err != nil {
		t.Fatal(err)
	}
	if history.Pagination.CurrentPage != 1 || history.Pagination.PageSize != 2 {
		t.Fatalf("invalid history pagination: %s", historyRaw)
	}
}

func TestFormatAnalysisForAIUsesDirectionChangeData(t *testing.T) {
	text := FormatAnalysisForAI(&MarketAnalysis{
		Symbol: "BTC", QuerySymbol: "BTC", MarketType: "core_perp",
		Ranking:          &DirectionChangeItem{Rank: 25, Bias: "bearish", Score: -4, BearishCount: 4, NeutralCount: 1, OIRank: 1, MarkPrice: 63047, Category: "crypto"},
		DirectionCurrent: json.RawMessage(`{"symbol":"BTC","direction":"bearish"}`),
		DirectionHistory: json.RawMessage(`{"items":[{"prev_bias":"bullish","new_bias":"bearish"}]}`),
	})
	for _, want := range []string{"Direction leaderboard", "direction_score=-4", "Current Bull/Bear Direction", "Bull/Bear Direction History", `"new_bias":"bearish"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Signal Lab") {
		t.Fatalf("old Signal Lab label remains:\n%s", text)
	}
}

func testClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return &Client{baseURL: baseURL, privateKey: (*ecdsa.PrivateKey)(key), httpClient: http.DefaultClient}
}

func TestHeatmapCanonicalMarketTypeAndLocalValidation(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != CostLiquidationHeatmapPath || r.URL.Query().Get("marketType") != "core_perp" {
			t.Errorf("wrong request: %s", r.URL)
		}
		fmt.Fprint(w, `{"data":{"bins":[]}}`)
	}))
	defer s.Close()
	c := testClient(t, s.URL)
	if _, err := c.GetCostLiquidationHeatmap(context.Background(), Query{MarketType: "perp", Symbol: "SOL"}); err != nil {
		t.Fatal(err)
	}
	_, err := c.GetCostLiquidationHeatmap(context.Background(), Query{MarketType: "bad", Symbol: "SOL"})
	if _, ok := err.(*WinrateValidationError); !ok {
		t.Fatalf("expected local validation: %v", err)
	}
	if calls != 1 {
		t.Fatalf("invalid request reached payment: %d calls", calls)
	}
}

func TestHolderWinrateRequestsUseExactPathsAndParams(t *testing.T) {
	type seenRequest struct {
		path  string
		query url.Values
	}
	var mu sync.Mutex
	var seen []seenRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, seenRequest{path: r.URL.Path, query: r.URL.Query()})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case HolderWinrateMapPath:
			fmt.Fprint(w, `{"data":{"snapshotId":"XGSZMBVFLOZRFYMWGC44OBBQNX","cells":[]}}`)
		case HolderWinrateHoldersPath:
			fmt.Fprint(w, `{"data":{"items":[],"total":0,"nextOffset":null}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := testClient(t, server.URL)
	if _, err := client.GetHolderWinrateMap(context.Background(), WinrateQuery{
		MarketType: "hip3_perp", Symbol: "NVDA", Chain: "hyperliquid",
		WinMin: 0, WinMax: 100, CostMin: 92, CostMax: 108, MinRoundTrips: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetHolderWinrateHolders(context.Background(), WinrateHoldersQuery{
		WinrateQuery: WinrateQuery{MarketType: "core_perp", Symbol: "BTC"},
		SnapshotID:   "XGSZMBVFLOZRFYMWGC44OBBQNX",
		Row:          0, RowEnd: 19, Column: 1, ColumnEnd: 16,
		Side: "short", Offset: 20, Limit: 100,
	}); err != nil {
		t.Fatal(err)
	}

	if len(seen) != 2 {
		t.Fatalf("requests=%d, want 2", len(seen))
	}
	q := seen[0].query
	if seen[0].path != HolderWinrateMapPath ||
		q.Get("marketType") != "hip3_perp" || q.Get("symbol") != "xyz:NVDA" ||
		q.Get("chain") != "mainnet" ||
		q.Get("winMin") != "0" || q.Get("winMax") != "100" ||
		q.Get("costMin") != "92" || q.Get("costMax") != "108" {
		t.Fatalf("map request=%+v", seen[0])
	}
	h := seen[1].query
	if seen[1].path != HolderWinrateHoldersPath ||
		h.Get("marketType") != "core_perp" || h.Get("symbol") != "BTC" ||
		h.Get("snapshotId") != "XGSZMBVFLOZRFYMWGC44OBBQNX" ||
		h.Get("row") != "0" || h.Get("rowEnd") != "19" ||
		h.Get("column") != "1" || h.Get("columnEnd") != "16" ||
		h.Get("side") != "short" || h.Get("offset") != "20" || h.Get("limit") != "100" {
		t.Fatalf("holders request=%+v", seen[1])
	}
}

func TestHolderWinrateValidation(t *testing.T) {
	client := testClient(t, "http://127.0.0.1:1")
	cases := []struct {
		name string
		call func() error
	}{
		{"map missing symbol", func() error {
			_, err := client.GetHolderWinrateMap(context.Background(), WinrateQuery{MarketType: "hip3_perp"})
			return err
		}},
		{"map inverted win window", func() error {
			_, err := client.GetHolderWinrateMap(context.Background(), WinrateQuery{MarketType: "hip3_perp", Symbol: "NVDA", WinMin: 80, WinMax: 20})
			return err
		}},
		{"map inverted cost window", func() error {
			_, err := client.GetHolderWinrateMap(context.Background(), WinrateQuery{MarketType: "hip3_perp", Symbol: "NVDA", CostMin: 140, CostMax: 60})
			return err
		}},
		{"holders malformed snapshot", func() error {
			_, err := client.GetHolderWinrateHolders(context.Background(), WinrateHoldersQuery{
				WinrateQuery: WinrateQuery{MarketType: "core_perp", Symbol: "BTC"},
				SnapshotID:   "not_a_snapshot", Row: 0, RowEnd: 19, Column: 1, ColumnEnd: 16,
			})
			return err
		}},
		{"holders inverted row range", func() error {
			_, err := client.GetHolderWinrateHolders(context.Background(), WinrateHoldersQuery{
				WinrateQuery: WinrateQuery{MarketType: "core_perp", Symbol: "BTC"},
				SnapshotID:   "XGSZMBVFLOZRFYMWGC44OBBQNX", Row: 19, RowEnd: 0, Column: 1, ColumnEnd: 16,
			})
			return err
		}},
		{"holders bad side", func() error {
			_, err := client.GetHolderWinrateHolders(context.Background(), WinrateHoldersQuery{
				WinrateQuery: WinrateQuery{MarketType: "core_perp", Symbol: "BTC"},
				SnapshotID:   "XGSZMBVFLOZRFYMWGC44OBBQNX", Row: 0, RowEnd: 19, Column: 1, ColumnEnd: 16,
				Side: "both",
			})
			return err
		}},
	}
	for _, tc := range cases {
		if err := tc.call(); err == nil {
			t.Errorf("%s: expected validation error, got nil", tc.name)
		}
	}
}

func TestFormatWinrateMarkdownSummaries(t *testing.T) {
	// 20 win rows × 18 slots, viewport 92..108 → slot width 1, price slot = 9
	// (lower edge 100%). Rows 18-19 = 90-100% win band; rows 0-5 = 0-30%.
	var cells []string
	mk := func(row, col int, side, notional string, count int) string {
		return fmt.Sprintf(`{"row":%d,"column":%d,"%s":{"count":%d,"notional":"%s"}}`, row, col, side, count, notional)
	}
	cells = append(cells,
		mk(19, 3, "long", "1000000", 10),  // long: 90-100% win, entry 94-95% (below price)
		mk(19, 12, "long", "3000000", 20), // long: 90-100% win, entry 103-104% (above price)
		mk(2, 3, "long", "1000000", 5),    // long: 0-30% win band, below price
		mk(10, 3, "short", "2000000", 7),  // short: mid win band
	)
	body := fmt.Sprintf(`{"data":{"markPrice":"100","included":{"count":42,"notional":"7000000"},
		"excluded":{"no_samples":{"count":9,"notional":"100000"}},
		"viewport":{"winMin":0,"winMax":100,"costMin":92,"costMax":108},
		"winBins":20,"costBins":16,
		"cells":[%s],
		"water":{"long":{"aboveWater":{"count":30,"notional":"3500000"},"belowWater":{"count":5,"notional":"500000"}},
		         "short":{"aboveWater":{"count":2,"notional":"200000"},"belowWater":{"count":7,"notional":"1800000"}}}}}`,
		strings.Join(cells, ","))

	out := FormatWinrateMarkdown(json.RawMessage(body))
	for _, want := range []string{
		"Included: 42 addrs / $7.00M",
		"Excluded (no verified round trip): 9 addrs",
		// long total 5M: topWin = 1M+3M = 80%, bottomWin = 1M = 20%, abovePx = 3M = 60%
		"long $5.00M / 35 addrs: 80% held by 90-100% win-rate holders, 20% by 0-30%; 60% entered above current price",
		"in profit $3.50M vs underwater $500.00K",
		"short $2.00M / 7 addrs: 0% held by 90-100%",
		"Largest cell: long 95-100% win-rate × entry 103-104% of price ($3.00M / 20 addrs)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q\n got:\n%s", want, out)
		}
	}
}

func TestFormatAnalysisForAIIncludesWinrateSection(t *testing.T) {
	analysis := &MarketAnalysis{
		Symbol:  "BTC",
		Winrate: json.RawMessage(`{"data":{"markPrice":"100","cells":[{"row":0,"column":1,"long":{"count":1,"notional":"5"}}],"viewport":{"costMin":92,"costMax":108},"winBins":20,"costBins":16}}`),
	}
	out := FormatAnalysisForAI(analysis)
	if !strings.Contains(out, "#### Holder Win-Rate Matrix") {
		t.Fatalf("missing win-rate section:\n%s", out)
	}
	analysis = &MarketAnalysis{Symbol: "BTC", WinrateError: "boom"}
	if out := FormatAnalysisForAI(analysis); !strings.Contains(out, "Holder Win-Rate Matrix: unavailable (boom)") {
		t.Fatalf("missing win-rate error line:\n%s", out)
	}
}

func TestFormatWinrateMarkdownIncludesJointGrid(t *testing.T) {
	// Same win bin, opposite sides of price — the joint grid must show them in
	// different rows, which the marginal summary cannot express. Row 19 is the
	// raw 95-100% win bin (full resolution, no 10% merge).
	cells := []string{
		`{"row":19,"column":3,"long":{"count":7,"notional":"1000000"}}`,  // 95-100% win, entry 94-95
		`{"row":19,"column":12,"long":{"count":2,"notional":"2000000"}}`, // 95-100% win, entry 103-104
	}
	body := fmt.Sprintf(`{"data":{"markPrice":"100","viewport":{"winMin":0,"winMax":100,"costMin":92,"costMax":108},
		"winBins":20,"costBins":16,"cells":[%s]}}`, strings.Join(cells, ","))
	out := FormatWinrateMarkdown(json.RawMessage(body))
	for _, want := range []string{
		"long grid (cells = notional/addrs",
		"| 95-100 |", // full-resolution win header
		"| **100-101** |",
		"| 92-96 |", "| 102-104 |",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("grid missing %q\n got:\n%s", want, out)
		}
	}
	// the merged 92-96 row must carry 1M/7 in the 95-100 win column (last col)
	if !strings.Contains(out, "| 92-96 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1M/7 |") {
		t.Errorf("92-96 row should carry 1M/7 in the 95-100 column:\n%s", out)
	}
	if !strings.Contains(out, "| 102-104 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 2M/2 |") {
		t.Errorf("102-104 row should carry 2M/2 in the 95-100 column:\n%s", out)
	}
}

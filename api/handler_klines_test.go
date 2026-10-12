package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"nofx/market/usstock"
)

func stubKlineSeams(t *testing.T, known map[string]bool) (lookups *[]string, charts *[]string) {
	t.Helper()
	oldLookup, oldChart := usstockLookupSymbol, usstockChartKlines
	t.Cleanup(func() { usstockLookupSymbol, usstockChartKlines = oldLookup, oldChart })
	lookups, charts = &[]string{}, &[]string{}
	usstockLookupSymbol = func(_ context.Context, symbol string) (usstock.SymbolInfo, bool) {
		*lookups = append(*lookups, symbol)
		return usstock.SymbolInfo{Symbol: symbol}, known[symbol]
	}
	usstockChartKlines = func(_ context.Context, symbol, interval string, limit int) ([]usstock.ChartKline, error) {
		*charts = append(*charts, symbol+"/"+interval)
		return []usstock.ChartKline{{OpenTime: 1700000000000, CloseTime: 1700000299999, Open: 1, High: 3, Low: 0.5, Close: 2, Volume: 10, QuoteVolume: 20}}, nil
	}
	return lookups, charts
}

func getKlines(url string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	s := &Server{}
	r.GET("/api/klines", s.handleKlines)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	return rec
}

func TestKlinesBStockUsesBinanceSpot(t *testing.T) {
	lookups, charts := stubKlineSeams(t, map[string]bool{"AMCBUSDT": true})
	rec := getKlines("/api/klines?symbol=amcbusdt&interval=5m&limit=1500&exchange=Binance")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var body []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || body[0]["openTime"] != float64(1700000000000) || body[0]["close"] != float64(2) ||
		body[0]["quoteVolume"] != float64(20) || body[0]["closeTime"] != float64(1700000299999) {
		t.Fatalf("unexpected payload %s", rec.Body.String())
	}
	if len(*lookups) != 1 || (*lookups)[0] != "AMCBUSDT" || len(*charts) != 1 || (*charts)[0] != "AMCBUSDT/5m" {
		t.Fatalf("lookups=%v charts=%v", *lookups, *charts)
	}
}

func TestKlinesNonBStockSkipsRegistry(t *testing.T) {
	// BTCUSDT does not end in BUSDT: the registry must not be touched.
	// ARBUSDT does, but is unknown, so it must fall through to CoinAnk (not the spot path).
	// An unsupported interval makes the CoinAnk path fail offline before any network call.
	lookups, charts := stubKlineSeams(t, nil)
	getKlines("/api/klines?symbol=BTCUSDT&interval=7m&exchange=binance")
	if len(*lookups) != 0 || len(*charts) != 0 {
		t.Fatalf("registry touched for BTCUSDT: lookups=%v charts=%v", *lookups, *charts)
	}
	getKlines("/api/klines?symbol=ARBUSDT&interval=7m&exchange=binance")
	if len(*lookups) != 1 || len(*charts) != 0 {
		t.Fatalf("ARBUSDT: lookups=%v charts=%v", *lookups, *charts)
	}
	getKlines("/api/klines?symbol=AMCBUSDT&interval=7m&exchange=bybit")
	if len(*lookups) != 1 {
		t.Fatalf("non-binance exchange must skip registry: %v", *lookups)
	}
}

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"nofx/kernel/stockengine"
	"nofx/market/usstock"
)

func TestUSStockSymbolsShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := usstockListSymbols
	t.Cleanup(func() { usstockListSymbols = old })
	usstockListSymbols = func(context.Context) ([]usstock.SymbolInfo, error) {
		return []usstock.SymbolInfo{{Symbol: "AAPLBUSDT", BaseAsset: "AAPLB", Underlying: "AAPL", TickSize: 0.01}}, nil
	}
	r := gin.New()
	s := &Server{}
	r.GET("/api/usstock/symbols", s.handleUSStockSymbols)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/usstock/symbols", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string][]map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	got := body["symbols"]
	if len(got) != 1 || got[0]["symbol"] != "AAPLBUSDT" || got[0]["underlying"] != "AAPL" || got[0]["base_asset"] != "AAPLB" || len(got[0]) != 3 {
		t.Fatalf("unexpected payload %s", rec.Body.String())
	}
}

func TestUSStockPaperEndpointIsOwnerScoped(t *testing.T) {
	srv, r, tokenA, _, traderA := idorFixture(t)
	protected := r.Group("/p", srv.authMiddleware())
	protected.GET("/paper/:trader_id", srv.handleUSStockPaper)
	if _, err := srv.store.StockPaper().EnsureAccount(traderA, 5000); err != nil {
		t.Fatal(err)
	}
	get := func(id string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/p/paper/"+id, nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	if rec := get(traderA); rec.Code != http.StatusOK {
		t.Fatalf("own trader: %d %s", rec.Code, rec.Body.String())
	}
	if rec := get("trader-B"); rec.Code == http.StatusOK {
		t.Fatal("another user's paper ledger must not be readable")
	}
}

func TestUSStockScanEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldCached, oldWarm := usstockCachedScan, usstockWarmScan
	t.Cleanup(func() { usstockCachedScan, usstockWarmScan = oldCached, oldWarm })
	r := gin.New()
	r.GET("/api/usstock/scan", (&Server{}).handleUSStockScan)
	get := func() map[string]any {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/usstock/scan", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body
	}

	// No cached report: a background scan is started and the page is told to poll.
	var report *stockengine.ScanReport
	warmed := []time.Duration{}
	usstockCachedScan = func() *stockengine.ScanReport { return report }
	usstockWarmScan = func(age time.Duration) { warmed = append(warmed, age) }
	body := get()
	if body["scanning"] != true || body["generated_at"] != nil || len(body["results"].([]any)) != 0 || len(body["skipped"].(map[string]any)) != 0 {
		t.Fatalf("scanning payload %v", body)
	}
	if len(warmed) != 1 || warmed[0] != 0 {
		t.Fatalf("warm calls %v", warmed)
	}

	// Cached report: results and skipped are returned verbatim.
	at := time.Date(2026, 10, 14, 14, 0, 0, 0, time.UTC)
	report = &stockengine.ScanReport{GeneratedAt: at, Results: []stockengine.ScanResult{{Symbol: "AAPLBUSDT", Underlying: "AAPL", Source: "bstock",
		Cycle: stockengine.CycleAssessment{Phase: stockengine.PhaseWedgePop, LongSetup: true, Score: 71.5}}}}
	body = get()
	if body["scanning"] != false || body["generated_at"] != "2026-10-14T14:00:00Z" || len(warmed) != 1 {
		t.Fatalf("report payload %v (warm calls %v)", body, warmed)
	}
	results := body["results"].([]any)
	first := results[0].(map[string]any)
	cycle := first["cycle"].(map[string]any)
	if len(results) != 1 || first["symbol"] != "AAPLBUSDT" || cycle["phase"] != "wedge_pop" || cycle["long_setup"] != true || cycle["score"] != 71.5 {
		t.Fatalf("results %v", results)
	}
	if skipped, ok := body["skipped"].(map[string]any); !ok || len(skipped) != 0 {
		t.Fatalf("skipped must be an object: %v", body["skipped"])
	}
}

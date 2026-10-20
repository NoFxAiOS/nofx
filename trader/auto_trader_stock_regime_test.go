package trader

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"nofx/kernel/stockengine"
	"nofx/store"
)

// Regime scaler (basket_vol) cycle tests. The scan report carries one momentum
// name whose closes alternate ±10% a day, so σ ≈ 162% and the scale is pinned
// to the configured floor.

func regimeScanReport() *stockengine.ScanReport {
	closes := make([]stockengine.DatedClose, 22)
	c := 100.0
	for i := range closes {
		if i > 0 {
			if i%2 == 1 {
				c *= 1.1
			} else {
				c *= 0.9
			}
		}
		closes[i] = stockengine.DatedClose{Date: stockTestNow.AddDate(0, 0, i-22).Format("2006-01-02"), Close: c}
	}
	r := momRes("AAPLBUSDT", "uptrend", false, 50, 40, true)
	r.RecentCloses = closes
	return scanReportOf(r)
}

// regimeFixture is a paper hybrid-mode fixture with the regime scaler on at
// the given floor (which becomes s), holding 10 AAPL.
func regimeFixture(t *testing.T, floor float64, mutate func(*store.StockConfig), now time.Time) *stockFixture {
	t.Helper()
	fx := newStockFixtureCfg(t, true, now, func(int) string { return holdAAPL }, func(sc *store.StockConfig) {
		hybridMode(1, []string{"AAPLBUSDT"}, nil)(sc)
		sc.RegimeScaler, sc.RegimeFloor = store.StockRegimeBasketVol, floor
		if mutate != nil {
			mutate(sc)
		}
	})
	useScanReport(regimeScanReport(), nil)
	if _, err := fx.at.store.StockPaper().EnsureAccount("t1", 10000); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.at.store.StockPaper().OpenOrAdd("t1", "AAPLBUSDT", 10, 120, 115, 130, now); err != nil {
		t.Fatal(err)
	}
	return fx
}

func paperQty(t *testing.T, fx *stockFixture) float64 {
	t.Helper()
	open, err := fx.at.store.StockPaper().ListOpen("t1")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range open {
		if p.Symbol == "AAPLBUSDT" {
			return p.Quantity
		}
	}
	return 0
}

// scaleKey is the store key the fixture's mode reads/writes applied scales under.
func scaleKey(fx *stockFixture) string { return fx.at.stockRegimeScaleKey(fx.at.stock.cfg.IsPaper()) }

func appliedScales(t *testing.T, fx *stockFixture) map[string]float64 {
	t.Helper()
	m, err := fx.at.store.StockRegimeScales().List(scaleKey(fx))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func regimeTrimActions(rec *store.DecisionRecord) []store.DecisionAction {
	var out []store.DecisionAction
	for _, a := range rec.Decisions {
		if strings.HasPrefix(a.Reasoning, "REGIME_TRIM") {
			out = append(out, a)
		}
	}
	return out
}

func TestStockRegimeTrimsOwnedQtyWhenScaleFalls(t *testing.T) {
	fx := regimeFixture(t, 0.5, nil, stockTestNow)
	if err := fx.at.store.StockRegimeScales().Set(scaleKey(fx), "AAPLBUSDT", 1); err != nil {
		t.Fatal(err)
	}
	fx.cycle(t)
	if q := paperQty(t, fx); !near(q, 5) {
		t.Fatalf("paper qty %v, want half sold", q)
	}
	if m := appliedScales(t, fx); len(m) != 1 || !near(m["AAPLBUSDT"], 0.5) {
		t.Fatalf("applied scales %v", m)
	}
	rec := fx.lastRecord(t)
	trims := regimeTrimActions(rec)
	if !rec.Success || len(trims) != 1 || !trims[0].Success || trims[0].Action != "reduce_long" || trims[0].Symbol != "AAPLBUSDT" ||
		!near(trims[0].Quantity, 5) || !near(trims[0].Price, 120) || !strings.Contains(trims[0].Reasoning, "s=0.50 applied=1.00 σ=") {
		t.Fatalf("decision record %+v", rec.Decisions)
	}
	joined := strings.Join(rec.ExecutionLog, "\n")
	if !strings.Contains(joined, "regime basket_vol s=0.50 σ=") || !strings.Contains(joined, "basket=[AAPLBUSDT]") || !strings.Contains(joined, "breadth=1/1") {
		t.Fatalf("regime log line missing:\n%s", joined)
	}
	// The prompt shows s and post-trim numbers (5 shares left = 600 exposure).
	if !strings.Contains(fx.ai.user, "市场状态仓位系数 s=0.50") || !strings.Contains(fx.ai.user, "qty=5.000000") || !strings.Contains(fx.ai.user, "exposure=600.00") {
		t.Fatalf("prompt not built from post-trim state:\n%s", fx.ai.user)
	}
	// A second cycle at the same scale does nothing more.
	fx.cycle(t)
	if q := paperQty(t, fx); !near(q, 5) || len(regimeTrimActions(fx.lastRecord(t))) != 0 {
		t.Fatalf("repeated trim: qty %v", q)
	}
}

func TestStockRegimeNoTrimInsideBand(t *testing.T) {
	fx := regimeFixture(t, 0.9, nil, stockTestNow) // s = 0.9: only 0.1 below applied 1.0
	fx.cycle(t)
	if q := paperQty(t, fx); !near(q, 10) {
		t.Fatalf("qty %v, want untouched", q)
	}
	if m := appliedScales(t, fx); len(m) != 0 {
		t.Fatalf("applied scale written without a trade: %v", m)
	}
	if n := len(regimeTrimActions(fx.lastRecord(t))); n != 0 {
		t.Fatalf("%d trim actions", n)
	}
}

func TestStockRegimeNoTrimOffHours(t *testing.T) {
	pre := time.Date(2026, 10, 14, 8, 0, 0, 0, stockET) // pre-market
	fx := regimeFixture(t, 0.5, nil, pre)
	fx.cycle(t)
	if q := paperQty(t, fx); !near(q, 10) {
		t.Fatalf("off-hours qty %v, want untouched", q)
	}
	if n := len(regimeTrimActions(fx.lastRecord(t))); n != 0 {
		t.Fatalf("%d trim actions off-hours", n)
	}
}

func TestStockRegimeTrimOffKeepsHoldings(t *testing.T) {
	off := false
	fx := regimeFixture(t, 0.5, func(sc *store.StockConfig) { sc.RegimeTrim = &off }, stockTestNow)
	fx.cycle(t)
	if q := paperQty(t, fx); !near(q, 10) {
		t.Fatalf("regime_trim=false qty %v, want untouched", q)
	}
	if !strings.Contains(fx.ai.user, "市场状态仓位系数 s=0.50") {
		t.Fatal("sizing/prompt scale must still apply with trimming off")
	}
}

func TestStockRegimeDisabledIsInert(t *testing.T) {
	fx := regimeFixture(t, 0.5, func(sc *store.StockConfig) { sc.RegimeScaler = "" }, stockTestNow)
	// A leftover row must be neither read nor deleted while the scaler is off.
	if err := fx.at.store.StockRegimeScales().Set(scaleKey(fx), "GONEBUSDT", 0.4); err != nil {
		t.Fatal(err)
	}
	fx.cycle(t)
	if q := paperQty(t, fx); !near(q, 10) {
		t.Fatalf("disabled qty %v", q)
	}
	if m := appliedScales(t, fx); len(m) != 1 || m["GONEBUSDT"] != 0.4 {
		t.Fatalf("disabled regime touched the store: %v", m)
	}
	if strings.Contains(fx.ai.user, "市场状态") {
		t.Fatalf("disabled regime must not reach the prompt:\n%s", fx.ai.user)
	}
	for _, line := range fx.lastRecord(t).ExecutionLog {
		if strings.Contains(line, "regime") {
			t.Fatalf("disabled regime logged: %q", line)
		}
	}
}

func TestStockRegimeScalesEntrySizingAndRecordsApplied(t *testing.T) {
	respond := func(int) string { return openAAPL }
	run := func(floor float64, scaler string) (qty float64, fx *stockFixture) {
		fx = newStockFixtureCfg(t, true, stockTestNow, respond, func(sc *store.StockConfig) {
			hybridMode(1, []string{"AAPLBUSDT"}, nil)(sc)
			sc.RegimeScaler, sc.RegimeFloor = scaler, floor
		})
		useScanReport(regimeScanReport(), nil)
		fx.cycle(t)
		return paperQty(t, fx), fx
	}
	base, _ := run(0, "")
	if !near(base, 16.666) {
		t.Fatalf("baseline qty %v", base)
	}
	scaled, fx := run(0.5, store.StockRegimeBasketVol)
	if !near(scaled, 8.333) { // risk budget 1% × 0.5 → half the baseline
		t.Fatalf("scaled qty %v, want %v", scaled, base/2)
	}
	if m := appliedScales(t, fx); len(m) != 1 || !near(m["AAPLBUSDT"], 0.5) {
		t.Fatalf("applied scale after entry: %v", m)
	}
}

func TestStockRegimeCloseDeletesAppliedAndCleansGonePositions(t *testing.T) {
	fx := regimeFixture(t, 0.9, nil, stockTestNow)
	fx.ai.respond = func(int) string { return closeAAPL }
	rs := fx.at.store.StockRegimeScales()
	for _, sym := range []string{"AAPLBUSDT", "GONEBUSDT"} {
		if err := rs.Set(scaleKey(fx), sym, 0.9); err != nil {
			t.Fatal(err)
		}
	}
	fx.cycle(t)
	if q := paperQty(t, fx); q != 0 {
		t.Fatalf("close_long left qty %v", q)
	}
	if m := appliedScales(t, fx); len(m) != 0 {
		t.Fatalf("closed/gone rows must be deleted: %v", m)
	}
}

func TestStockRegimeTrimSkipsQuarantinedHolding(t *testing.T) {
	fx := newStockFixtureCfg(t, false, stockTestNow, func(int) string { return holdAAPL }, func(sc *store.StockConfig) {
		hybridMode(1, []string{"AAPLBUSDT"}, nil)(sc)
		sc.RegimeScaler, sc.RegimeFloor = store.StockRegimeBasketVol, 0.5
	})
	useScanReport(regimeScanReport(), nil)
	ctx := context.Background()
	fx.openLiveOwned(t, 10, 10)
	if err := fx.at.stockPlaceProtection(ctx, "AAPLBUSDT", 10, 115, 130, 120); err != nil {
		t.Fatal(err)
	}
	fx.spot.setProt, fx.spot.sells = nil, nil
	fx.spot.protErrors = map[string]error{"AAPLBUSDT": fmt.Errorf("open orders unavailable")}
	fx.cycle(t)
	rec := fx.lastRecord(t)
	if !rec.Success || len(fx.spot.sells) != 0 || len(regimeTrimActions(rec)) != 0 {
		t.Fatalf("quarantined holding was trimmed: sells=%v record=%+v", fx.spot.sells, rec.Decisions)
	}
	if !strings.Contains(fx.ai.user, "PROTECTION UNVERIFIED") {
		t.Fatalf("holding should be quarantined in the prompt:\n%s", fx.ai.user)
	}
}

func TestStockRegimeLiveTrimSellsOwnedOnly(t *testing.T) {
	fx := newStockFixtureCfg(t, false, stockTestNow, func(int) string { return holdAAPL }, func(sc *store.StockConfig) {
		hybridMode(1, []string{"AAPLBUSDT"}, nil)(sc)
		sc.RegimeScaler, sc.RegimeFloor = store.StockRegimeBasketVol, 0.5
	})
	useScanReport(regimeScanReport(), nil)
	ctx := context.Background()
	fx.openLiveOwned(t, 10, 14) // 4 manual shares on top of the 10 owned
	if err := fx.at.stockPlaceProtection(ctx, "AAPLBUSDT", 10, 115, 130, 120); err != nil {
		t.Fatal(err)
	}
	fx.spot.setProt, fx.spot.sells = nil, nil
	fx.cycle(t)
	if len(fx.spot.sells) != 1 || fx.spot.sells[0].Symbol != "AAPLBUSDT" || !near(fx.spot.sells[0].Qty, 5) {
		t.Fatalf("sells %+v, want 5 of the 10 owned shares", fx.spot.sells)
	}
	if fx.spot.hold["AAPLBUSDT"] != 9 {
		t.Fatalf("balance %v: manual shares must be untouched", fx.spot.hold["AAPLBUSDT"])
	}
	rows, err := fx.at.store.Position().StockOwnedRows("t1", "AAPLBUSDT")
	if err != nil || len(rows) != 1 || !near(rows[0].Quantity, 5) {
		t.Fatalf("owned rows %+v err %v", rows, err)
	}
	if len(fx.spot.setProt) != 1 || !near(fx.spot.setProt[0].Qty, 5) {
		t.Fatalf("remainder protection %+v", fx.spot.setProt)
	}
	if m := appliedScales(t, fx); !near(m["AAPLBUSDT"], 0.5) {
		t.Fatalf("applied scales %v", m)
	}
	// The cycle's prompt reflects the post-trim live state.
	if !strings.Contains(fx.ai.user, "qty=5.000000") {
		t.Fatalf("prompt not built from post-trim state:\n%s", fx.ai.user)
	}
}

func TestStockRegimeTrimTooSmallIsRecordedOnce(t *testing.T) {
	fx := regimeFixture(t, 0.5, nil, stockTestNow)
	// Swap the 10-share position for one too small to halve above min notional.
	if _, _, err := fx.at.store.StockPaper().Reduce("t1", "AAPLBUSDT", 0, 120, stockTestNow, "seed"); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.at.store.StockPaper().OpenOrAdd("t1", "AAPLBUSDT", 0.04, 120, 115, 130, stockTestNow); err != nil { // 0.04 × 120 = 4.8 USDT
		t.Fatal(err)
	}
	fx.cycle(t)
	rec := fx.lastRecord(t)
	trims := regimeTrimActions(rec)
	if len(trims) != 1 || trims[0].Success || !strings.Contains(trims[0].Error, "REDUCE_TOO_SMALL") {
		t.Fatalf("trim actions %+v", trims)
	}
	if !near(paperQty(t, fx), 0.04) {
		t.Fatal("too-small position must stay untouched")
	}
	if m := appliedScales(t, fx); !near(m["AAPLBUSDT"], 0.5) {
		t.Fatalf("too-small trim must record the scale to stop retrying: %v", m)
	}
	fx.cycle(t)
	if n := len(regimeTrimActions(fx.lastRecord(t))); n != 0 {
		t.Fatalf("retried a too-small trim: %d actions", n)
	}
}

func TestStockCycleRegimeStaleOrMissingReportIsScaleOne(t *testing.T) {
	fx := regimeFixture(t, 0.5, nil, stockTestNow)
	var lines []string
	logf := func(f string, a ...interface{}) { lines = append(lines, fmt.Sprintf(f, a...)) }
	stale := regimeScanReport()
	stale.GeneratedAt = stockTestNow.Add(-stockRegimeMaxReportAge - time.Hour)
	for name, report := range map[string]*stockengine.ScanReport{"stale": stale, "nil": nil} {
		got := fx.at.stockCycleRegime(fx.at.stock.cfg, report, stockTestNow, logf)
		if !got.Enabled || got.Scale != 1 || got.Reason == "" {
			t.Fatalf("%s: %+v", name, got)
		}
	}
	if got := fx.at.stockCycleRegime(&store.StockConfig{}, regimeScanReport(), stockTestNow, logf); got.Enabled || got.Scale != 1 {
		t.Fatalf("disabled: %+v", got)
	}
}

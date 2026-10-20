package trader

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"nofx/store"
	"nofx/trader/types"
)

// Regression tests for the 2026-10-20 us_stock review (area3).

func ownedQty(t *testing.T, fx *stockFixture) (qty, entry float64) {
	t.Helper()
	rows, err := fx.at.store.Position().StockOwnedRows("t1", "AAPLBUSDT")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		qty += r.Quantity
		entry = r.EntryPrice
	}
	return
}

// #1: canceling with newClientOrderId replaces the order's clientOrderId on the
// exchange; reconciliation must still accept the (our own) canceled order.
func TestStockCanceledProtectionWithReplacedClientIDReconciles(t *testing.T) {
	fx := newStockFixture(t, false, stockTestNow, func(int) string { return holdAAPL })
	fx.openLiveOwned(t, 5, 5)
	ctx := context.Background()
	if err := fx.at.stockPlaceProtection(ctx, "AAPLBUSDT", 5, 115, 0, 120); err != nil {
		t.Fatal(err)
	}
	id := fx.spot.prot["AAPLBUSDT"].StopOrderID
	// The exchange-side cancel (done inside CancelProtection) rewrote the ID.
	fx.spot.onCancel = func() { fx.spot.statuses[id]["clientOrderId"] = "nxbs_cancel_77" }
	if _, err := fx.at.stockPrepareProtectionChange("AAPLBUSDT", stockTestNow); err != nil {
		t.Fatalf("a canceled order whose ID became our cancel ID must reconcile: %v", err)
	}
	rows, _ := fx.at.store.StockPaper().ListProtectionOrders("t1", "AAPLBUSDT")
	if len(rows) != 1 || !rows[0].Retired || rows[0].ClientOrderID != "nxbs_cancel_77" || rows[0].Status != "CANCELED" {
		t.Fatalf("new client ID must be persisted and the set retired: %+v", rows)
	}
	// The relaxation is narrow: a live order, or a non-program ID, still mismatches.
	if err := fx.at.stockPlaceProtection(ctx, "AAPLBUSDT", 5, 115, 0, 120); err != nil {
		t.Fatal(err)
	}
	id = fx.spot.prot["AAPLBUSDT"].StopOrderID
	fx.spot.statuses[id]["clientOrderId"] = "nxbs_other"
	if _, err := fx.at.stockCheckProtection("AAPLBUSDT", stockTestNow); err == nil || !strings.Contains(err.Error(), "client ID mismatch") {
		t.Fatalf("a live order with a changed ID must stay a mismatch, got %v", err)
	}
	fx.spot.statuses[id]["clientOrderId"], fx.spot.statuses[id]["status"] = "manual-123", "CANCELED"
	if _, err := fx.at.stockCheckProtection("AAPLBUSDT", stockTestNow); err == nil || !strings.Contains(err.Error(), "client ID mismatch") {
		t.Fatalf("a non-program ID must stay a mismatch, got %v", err)
	}
}

func pendingFixture(t *testing.T, qty float64) (*stockFixture, *stockPending) {
	t.Helper()
	fx := newStockFixture(t, false, stockTestNow, func(int) string { return holdAAPL })
	p := &stockPending{Symbol: "AAPLBUSDT", OrderID: "pend1", Action: "open_long", Qty: qty, Limit: 100, Stop: 90, TakeProfit: 130, PlacedAt: stockTestNow}
	fx.at.stock.pendings[p.Symbol] = p
	fx.at.savePending(p)
	return fx, p
}

func pendingStatus(status string, executed, quote, fee float64) map[string]interface{} {
	m := map[string]interface{}{"status": status, "executedQty": executed, "cummulativeQuoteQty": quote, "commission": fee,
		"clientOrderId": "nxbs_x"}
	if executed > 0 {
		m["avgPrice"] = quote / executed
	}
	return m
}

// #10: later fills of a limit entry are priced from the cumulative DELTAS.
func TestStockPendingIncrementUsesCumulativeDeltas(t *testing.T) {
	fx, _ := pendingFixture(t, 2)
	fx.spot.hold["AAPLBUSDT"] = 1
	fx.spot.statuses["pend1"] = pendingStatus("PARTIALLY_FILLED", 1, 100, 0.1)
	fx.at.stockProtectionTick(context.Background())
	fx.spot.hold["AAPLBUSDT"] = 2
	fx.spot.statuses["pend1"] = pendingStatus("FILLED", 2, 180, 0.4) // 2nd share cost 80, cumulative avg 90
	fx.at.stockProtectionTick(context.Background())
	qty, entry := ownedQty(t, fx)
	if !near(qty, 2) || !near(entry, 90) {
		t.Fatalf("owned %v @ %v, want 2 @ 90 (a cumulative-average increment would give 95)", qty, entry)
	}
	rows, _ := fx.at.store.Position().StockOwnedRows("t1", "AAPLBUSDT")
	if len(rows) != 1 || !near(rows[0].Fee, 0.4) {
		t.Fatalf("fee must be the sum of the deltas (0.1 + 0.3), got %+v", rows)
	}
}

// #2: the fill watermark and the position increment commit together.
func TestStockPendingFillAndWatermarkCommitAtomically(t *testing.T) {
	fx, _ := pendingFixture(t, 2)
	fx.spot.hold["AAPLBUSDT"] = 2
	fx.spot.statuses["pend1"] = pendingStatus("PARTIALLY_FILLED", 2, 200, 0)
	db := fx.at.store.GormDB()
	if err := db.Exec(`CREATE TRIGGER fail_pending_watermark BEFORE UPDATE ON stock_pending_entries BEGIN SELECT RAISE(ABORT, 'injected'); END`).Error; err != nil {
		t.Fatal(err)
	}
	fx.at.stockProtectionTick(context.Background())
	if qty, _ := ownedQty(t, fx); qty != 0 {
		t.Fatalf("position escaped the failed watermark write: %v", qty)
	}
	if p := fx.at.stock.pendings["AAPLBUSDT"]; p.FilledQty != 0 {
		t.Fatalf("in-memory watermark advanced without the commit: %v", p.FilledQty)
	}
	if err := db.Exec(`DROP TRIGGER fail_pending_watermark`).Error; err != nil {
		t.Fatal(err)
	}
	fx.at.stockProtectionTick(context.Background())
	fx.at.stockProtectionTick(context.Background()) // replay of the same cumulative status
	if qty, _ := ownedQty(t, fx); !near(qty, 2) {
		t.Fatalf("owned %v, want exactly one booking of 2", qty)
	}
	// A restart restores the committed watermark: no second claim of the 2 shares.
	fx.at.stock.pendings = map[string]*stockPending{}
	fx.at.restoreStockPendings()
	if p := fx.at.stock.pendings["AAPLBUSDT"]; p == nil || !near(p.FilledQty, 2) || !near(p.FilledQuote, 200) {
		t.Fatalf("restored pending %+v", p)
	}
	fx.at.stockProtectionTick(context.Background())
	if qty, _ := ownedQty(t, fx); !near(qty, 2) {
		t.Fatalf("restart re-claimed the fill: owned %v", qty)
	}
}

// #3: a first protection that failed must be retried, not quarantined forever;
// after repeated failures the naked position is exited.
func TestStockFirstProtectionFailureIsRetriedThenBounded(t *testing.T) {
	fx := newStockFixture(t, false, stockTestNow, func(int) string { return openAAPL })
	fx.spot.setProtErr = fmt.Errorf("NOTIONAL: below minimum after the stop-limit buffer")
	fx.cycle(t)
	if qty, _ := ownedQty(t, fx); qty <= 0 {
		t.Fatal("the buy must have been booked")
	}
	if in, _ := fx.at.store.StockPaper().ListProtectionIntents("t1"); !in["AAPLBUSDT"] {
		t.Fatalf("a booked buy with unconfirmed protection must be remembered: %v", in)
	}
	// The protection tick retries instead of quarantining the symbol.
	before := len(fx.spot.setProt)
	fx.at.stockProtectionTick(context.Background())
	if len(fx.spot.setProt) <= before {
		t.Fatalf("no retry of the protection: %v", fx.spot.setProt)
	}
	for _, a := range fx.alert {
		if strings.Contains(a, "无已记录的保护单 ID") {
			t.Fatalf("symbol quarantined as unknown provenance: %s", a)
		}
	}
	// The failure clears: the next tick places it and the intent goes away.
	fx.spot.setProtErr = nil
	fx.at.stockProtectionTick(context.Background())
	if fx.spot.prot["AAPLBUSDT"] == nil {
		t.Fatal("protection not re-placed once possible")
	}
	if in, _ := fx.at.store.StockPaper().ListProtectionIntents("t1"); in["AAPLBUSDT"] {
		t.Fatalf("intent must clear once protection stands: %v", in)
	}

	// Bounded: a protection that never places ends in a market exit.
	fx2 := newStockFixture(t, false, stockTestNow, func(int) string { return openAAPL })
	fx2.spot.setProtErr = fmt.Errorf("NOTIONAL")
	fx2.cycle(t)
	for i := 0; i < stockFirstProtectionMaxAttempts+1; i++ {
		fx2.at.stockProtectionTick(context.Background())
	}
	if len(fx2.spot.sells) != 1 {
		t.Fatalf("expected one bounded emergency exit, sells=%v", fx2.spot.sells)
	}
	if qty, _ := ownedQty(t, fx2); qty != 0 {
		t.Fatalf("position still owned after the exit: %v", qty)
	}
	if in, _ := fx2.at.store.StockPaper().ListProtectionIntents("t1"); in["AAPLBUSDT"] {
		t.Fatalf("closed position keeps its intent: %v", in)
	}
}

// #6: a full exit retires the resting entry and sells what it already filled.
func TestStockFullExitCancelsRestingEntryAndSellsItsFills(t *testing.T) {
	fx := newStockFixture(t, false, stockTestNow, func(int) string { return closeAAPL })
	fx.openLiveOwned(t, 4, 4)
	if err := fx.at.stockPlaceProtection(context.Background(), "AAPLBUSDT", 4, 115, 0, 120); err != nil {
		t.Fatal(err)
	}
	p := &stockPending{Symbol: "AAPLBUSDT", OrderID: "pend1", Action: "add_long", Qty: 10, FilledQty: 0, Limit: 119, Stop: 115, PlacedAt: stockTestNow}
	fx.at.stock.pendings[p.Symbol] = p
	fx.at.savePending(p)
	// 2 more shares fill between the cycle snapshot and the cancel.
	fx.spot.statuses["pend1"] = pendingStatus("PARTIALLY_FILLED", 2, 238, 0)
	fx.spot.hold["AAPLBUSDT"] = 6
	fx.cycle(t)
	if len(fx.spot.cancelled) == 0 || fx.spot.cancelled[0] != "AAPLBUSDT:pend1" {
		t.Fatalf("the resting entry was not cancelled: %v", fx.spot.cancelled)
	}
	if fx.at.stock.pendings["AAPLBUSDT"] != nil {
		t.Fatal("pending still tracked after the exit")
	}
	if pend, _ := fx.at.store.StockPaper().ListPending("t1"); len(pend) != 0 {
		t.Fatalf("pending row survived the exit: %+v", pend)
	}
	if n := len(fx.spot.sells); n == 0 || !near(fx.spot.sells[n-1].Qty, 6) {
		t.Fatalf("the exit must sell the 4 owned + 2 just filled shares: %v", fx.spot.sells)
	}
	if qty, _ := ownedQty(t, fx); qty != 0 {
		t.Fatalf("owned %v after a full exit", qty)
	}
}

// #6: protection closing the position also retires the entry remainder.
func TestStockProtectionExitCancelsRestingEntry(t *testing.T) {
	fx := newStockFixture(t, false, stockTestNow, func(int) string { return holdAAPL })
	fx.openLiveOwned(t, 5, 5)
	if err := fx.at.stockPlaceProtection(context.Background(), "AAPLBUSDT", 5, 115, 0, 120); err != nil {
		t.Fatal(err)
	}
	id := fx.spot.prot["AAPLBUSDT"].StopOrderID
	fx.spot.statuses[id]["status"], fx.spot.statuses[id]["executedQty"], fx.spot.statuses[id]["cummulativeQuoteQty"] = "FILLED", 5.0, 570.0
	fx.spot.cancelProtection("AAPLBUSDT")
	fx.spot.hold["AAPLBUSDT"] = 0
	p := &stockPending{Symbol: "AAPLBUSDT", OrderID: "pend1", Action: "add_long", Qty: 6, Limit: 110, Stop: 100, PlacedAt: stockTestNow}
	fx.at.stock.pendings[p.Symbol] = p
	fx.at.savePending(p)
	fx.spot.statuses["pend1"] = pendingStatus("NEW", 0, 0, 0)
	fx.at.stockProtectionTick(context.Background())
	if len(fx.spot.cancelled) != 1 || fx.at.stock.pendings["AAPLBUSDT"] != nil {
		t.Fatalf("entry remainder must be cancelled after the stop exit: cancelled=%v pend=%v", fx.spot.cancelled, fx.at.stock.pendings["AAPLBUSDT"])
	}
}

// #5: a kept entry that FILLED must not be forgotten when the mode switches to paper.
func TestStockOrphanCancelKeepsEntryWithUnbookedFills(t *testing.T) {
	limit := `[{"symbol":"AAPLBUSDT","action":"open_long","entry_type":"limit","limit_price":119,"stop_loss":115,"take_profit":130,"confidence":80,"thesis":"t","invalidation":"i","reasoning":"r"}]`
	fx := newStockFixture(t, false, stockTestNow, func(int) string { return limit })
	fx.cycle(t)
	pend, _ := fx.at.store.StockPaper().ListPending("t1")
	if len(pend) != 1 {
		t.Fatalf("pending not placed: %+v", pend)
	}
	// It filled while the instance was down.
	fx.spot.statuses[pend[0].OrderID] = pendingStatus("FILLED", 3, 357, 0)
	fx.at.StopForRestart()
	paper := true
	fx.at.config.StrategyConfig.StockConfig.PaperTrading = &paper
	fx.at.stockTrader = nil
	old := stockOrphanExecutor
	t.Cleanup(func() { stockOrphanExecutor = old })
	stockOrphanExecutor = func(*AutoTrader) (types.SpotStockTrader, error) { return fx.spot, nil }
	fx.at.isRunning = true
	if _, err := fx.at.prepareStockRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	if after, _ := fx.at.store.StockPaper().ListPending("t1"); len(after) != 1 {
		t.Fatalf("a filled entry must stay recorded until its fills are booked: %+v", after)
	}
	alerted := false
	for _, a := range fx.alert {
		alerted = alerted || strings.Contains(a, "filled but not booked")
	}
	if !alerted {
		t.Fatalf("no alert about the unbooked fills: %v", fx.alert)
	}
}

func liveRegimeFixture(t *testing.T) *stockFixture {
	t.Helper()
	fx := newStockFixtureCfg(t, false, stockTestNow, func(int) string { return holdAAPL }, func(sc *store.StockConfig) {
		hybridMode(1, []string{"AAPLBUSDT"}, nil)(sc)
		sc.RegimeScaler, sc.RegimeFloor = store.StockRegimeBasketVol, 0.5
	})
	useScanReport(regimeScanReport(), nil)
	fx.openLiveOwned(t, 10, 10)
	if err := fx.at.stockPlaceProtection(context.Background(), "AAPLBUSDT", 10, 115, 130, 120); err != nil {
		t.Fatal(err)
	}
	fx.spot.setProt, fx.spot.sells = nil, nil
	return fx
}

// #7: a partial regime sell is not "applied"; the trim resumes towards the same
// TARGET remainder instead of halving what is left.
func TestStockRegimeTrimPartialFillResumesTowardsTarget(t *testing.T) {
	fx := liveRegimeFixture(t)
	fx.spot.sellRatio = 0.4 // 5 requested, 2 executed
	fx.cycle(t)
	if qty, _ := ownedQty(t, fx); !near(qty, 8) {
		t.Fatalf("owned %v after the partial sell, want 8", qty)
	}
	if m := appliedScales(t, fx); len(m) != 1 || !near(m["AAPLBUSDT"], 1) {
		t.Fatalf("a partial fill must not mark the scale applied: %v", m)
	}
	plans, _ := fx.at.store.StockRegimeScales().ListPlans(scaleKey(fx))
	if pl := plans["AAPLBUSDT"]; !near(pl.TargetQty, 5) || !near(pl.TargetScale, 0.5) {
		t.Fatalf("plan %+v, want target 5 @ 0.5", plans)
	}
	fx.spot.sellRatio = 0
	fx.cycle(t)
	if qty, _ := ownedQty(t, fx); !near(qty, 5) {
		t.Fatalf("owned %v, want the target 5 (not 4 from halving the remainder)", qty)
	}
	if m := appliedScales(t, fx); !near(m["AAPLBUSDT"], 0.5) {
		t.Fatalf("applied scales %v", m)
	}
	if plans, _ := fx.at.store.StockRegimeScales().ListPlans(scaleKey(fx)); len(plans) != 0 {
		t.Fatalf("plan must clear once applied: %+v", plans)
	}
}

// #8: the sell succeeded but the applied-scale write was lost (crash): the
// persisted plan means the restart does NOT halve the remainder again.
func TestStockRegimeTrimRestartAfterSellDoesNotTrimAgain(t *testing.T) {
	fx := liveRegimeFixture(t)
	fx.spot.hold["AAPLBUSDT"] = 5
	rows, _ := fx.at.store.Position().StockOwnedRows("t1", "AAPLBUSDT")
	if err := fx.at.store.Position().ReducePositionQuantity(rows[0].ID, 5, 120, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := fx.at.store.StockRegimeScales().SetPlan(scaleKey(fx), "AAPLBUSDT", 0.5, 5); err != nil {
		t.Fatal(err)
	}
	fx.cycle(t)
	if len(fx.spot.sells) != 0 {
		t.Fatalf("restart re-trimmed an already trimmed position: %v", fx.spot.sells)
	}
	if m := appliedScales(t, fx); !near(m["AAPLBUSDT"], 0.5) {
		t.Fatalf("applied scales %v", m)
	}
}

// #12: paper and live applied scales are independent books.
func TestStockRegimeScalesAreSeparatePerMode(t *testing.T) {
	fx := regimeFixture(t, 0.9, nil, stockTestNow) // paper
	liveKey := fx.at.stockRegimeScaleKey(false)
	if liveKey == fx.at.stockRegimeScaleKey(true) {
		t.Fatal("paper and live must not share a key")
	}
	rs := fx.at.store.StockRegimeScales()
	if err := rs.Set(liveKey, "AAPLBUSDT", 0.5); err != nil { // a live position the paper book knows nothing about
		t.Fatal(err)
	}
	if err := rs.Set(liveKey, "GONEBUSDT", 0.4); err != nil {
		t.Fatal(err)
	}
	fx.cycle(t) // paper cleanup must not collect the live rows
	live, _ := rs.List(liveKey)
	if len(live) != 2 || !near(live["AAPLBUSDT"], 0.5) || !near(live["GONEBUSDT"], 0.4) {
		t.Fatalf("paper cycle touched the live applied scales: %v", live)
	}
}

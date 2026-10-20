package trader

import (
	"math"
	"path/filepath"
	"strings"
	"testing"

	"nofx/kernel"
	"nofx/market"
	"nofx/store"
)

// 2026-10-20 review (areas 2 and 4) pins for risk / loop / rules.

// area2 #5: a MARKET open's risk reservation uses the live price, not the
// limit anchor stored in GateState.
func TestReview1020MarketReservationUsesLivePriceNotLimitAnchor(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{
		LimitEntryEnabled: true, RiskPerTradePct: 1.5, MaxNetDirectionalRiskPct: -1,
	})
	at.exchange = "binance"
	at.cycleGateStates = map[string]*kernel.GateState{
		"SYMUSDT": {ShortAllowed: true, ShortEntryPrice: 101, ShortLimitAllowed: true, ShortMarketException: true},
	}
	ctx := &kernel.Context{
		Account:        kernel.AccountInfo{TotalEquity: 10000},
		CandidateCoins: []kernel.CandidateCoin{{Symbol: "SYMUSDT"}},
		MarketDataMap:  map[string]*market.Data{"SYMUSDT": {Symbol: "SYMUSDT", CurrentPrice: 100}},
		// Existing LONG stop-risk 26 x 100 x 5% = 130 of the 150 per-symbol budget.
		Positions: []kernel.PositionInfo{{Symbol: "SYMUSDT", Side: "long", EntryPrice: 100, Quantity: 26, StopLossPrice: 95}},
	}
	d := kernel.Decision{Symbol: "SYMUSDT", Action: "open_short", Price: 0, StopLoss: 103, TakeProfit: 94, PositionSizeUSD: 1000, Confidence: 95}
	if px := at.entryReferencePrice(&d, ctx); px != 100 {
		t.Fatalf("market open reference price = %v, want live 100 (not anchor 101)", px)
	}
	lim := d
	lim.Action = "open_short_limit"
	if px := at.entryReferencePrice(&lim, ctx); px != 101 {
		t.Fatalf("limit open reference price = %v, want anchor 101", px)
	}
	// Priced at 100 the candidate adds 30 -> 160 > 150 symbol budget -> blocked.
	// (At the old anchor price it added 19.8 -> 149.8 and slipped through.)
	if got := at.applyHardRiskGates([]kernel.Decision{d}, ctx); len(got) != 0 {
		t.Fatalf("market short whose live-priced stop-risk breaches the symbol budget passed: %+v", got)
	}
	// A smaller existing risk lets it through, and the booked reservation is the live-priced 30.
	ctx.Positions[0].Quantity = 10 // 50
	got := at.applyHardRiskGates([]kernel.Decision{d}, ctx)
	if len(got) != 1 || math.Abs(got[0].CycleReservedRiskUSD-30) > 1e-6 {
		t.Fatalf("expected one decision reserving 30 (live price), got %+v", got)
	}
}

// area2 #5: market->limit degrade re-books the reservation at the anchor.
func TestReview1020RepriceReservationAfterMarketToLimitConversion(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{LimitEntryEnabled: true, RiskPerTradePct: 1.5})
	at.cycleGateStates = map[string]*kernel.GateState{"SYMUSDT": {ShortEntryPrice: 101}}
	ctx := &kernel.Context{Account: kernel.AccountInfo{TotalEquity: 10000},
		MarketDataMap: map[string]*market.Data{"SYMUSDT": {CurrentPrice: 100}}}
	at.cycleRiskReservedUSD, at.cycleShortRiskReservedUSD = 30, 30
	at.cycleSymbolRiskReservedUSD = map[string]float64{"SYMUSDT": 30}
	d := kernel.Decision{Symbol: "SYMUSDT", Action: "open_short", StopLoss: 103, PositionSizeUSD: 1000, CycleReservedRiskUSD: 30}

	at.repriceCycleReservation(&d, "open_short", ctx) // not converted: no-op
	if d.CycleReservedRiskUSD != 30 {
		t.Fatalf("unconverted decision was re-priced: %v", d.CycleReservedRiskUSD)
	}
	d.Action = "open_short_limit" // marketExceptionGate degrade
	d.Price = 101
	at.repriceCycleReservation(&d, "open_short", ctx)
	want := 1000 * 2.0 / 101
	if math.Abs(d.CycleReservedRiskUSD-want) > 1e-6 || math.Abs(at.cycleRiskReservedUSD-want) > 1e-6 ||
		math.Abs(at.cycleShortRiskReservedUSD-want) > 1e-6 || math.Abs(at.cycleSymbolRiskReservedUSD["SYMUSDT"]-want) > 1e-6 {
		t.Fatalf("reservation not re-booked at anchor (want %.4f): dec=%v total=%v short=%v sym=%v",
			want, d.CycleReservedRiskUSD, at.cycleRiskReservedUSD, at.cycleShortRiskReservedUSD, at.cycleSymbolRiskReservedUSD)
	}
}

// area2 #6: a daily-loss breach seen on an equity snapshot latches even when
// no open decision exists, and survives an equity rebound.
func TestReview1020DailyLossLatchedWithoutOpenDecision(t *testing.T) {
	ResetDailyBaselineRegistryForTest()
	rc := store.RiskControlConfig{DailyMaxLossPct: 10}
	at := riskTestTrader(rc)
	at.anchorDailyBaseline(1000)
	if halt := at.evaluateDailyLossLatch(890); halt == "" {
		t.Fatal("11% intraday loss must latch the halt on the equity snapshot")
	}
	// Equity rebounds; the next cycle's open is still halted.
	if halt := at.dailyLossHaltBlocks(rc, 950); halt == "" {
		t.Fatal("halt latched in a hold-only cycle was released after equity rebounded")
	}
	// Invalid snapshots never latch or panic; unconfigured strategy is a no-op.
	if halt := riskTestTrader(rc).evaluateDailyLossLatch(0); halt != "" {
		t.Fatalf("zero equity snapshot must not halt: %q", halt)
	}
	if halt := (&AutoTrader{}).evaluateDailyLossLatch(500); halt != "" {
		t.Fatalf("no strategy config must be a no-op: %q", halt)
	}
}

func netDirTrader() *AutoTrader {
	return riskTestTrader(store.RiskControlConfig{MaxAccountRiskPct: 20, MaxNetDirectionalRiskPct: 3, RiskPerTradePct: 2})
}

// area4 #1: this-cycle opposite-side reservations (not yet executed) must not
// offset net risk - if the earlier long fails, the shorts still execute.
func TestReview1020UnexecutedOppositeReservationDoesNotHedge(t *testing.T) {
	at := netDirTrader()
	ctx := &kernel.Context{Account: kernel.AccountInfo{TotalEquity: 1000}}
	// A (long) and B (short) already reserved this cycle at 20 (2%) each.
	at.cycleRiskReservedUSD, at.cycleLongRiskReservedUSD, at.cycleShortRiskReservedUSD = 40, 20, 20
	short := &kernel.Decision{Action: "open_short", Symbol: "CUSDT", PositionSizeUSD: 1000, StopLoss: 102}
	blocked, reason, _ := at.accountRiskExposureBlocks(short, 100, ctx)
	if !blocked || !strings.Contains(reason, "max_net_directional_risk_pct") {
		t.Fatalf("C short must be blocked (B+C = 4%% short if A never fills): blocked=%v %s", blocked, reason)
	}
	// With the long actually FILLED (a position) the same short is a real hedge.
	at.cycleRiskReservedUSD, at.cycleLongRiskReservedUSD, at.cycleShortRiskReservedUSD = 20, 0, 20
	ctx.Positions = []kernel.PositionInfo{{Symbol: "AUSDT", Side: "long", EntryPrice: 100, Quantity: 10, StopLossPrice: 98}} // 20U filled long
	if blocked, reason, _ := at.accountRiskExposureBlocks(short, 100, ctx); blocked {
		t.Fatalf("filled long hedges the short: %s", reason)
	}
}

// area4 #1: resting opposite-side limits do not hedge either.
func TestReview1020PendingOppositeLimitDoesNotHedge(t *testing.T) {
	at := netDirTrader()
	ctx := &kernel.Context{Account: kernel.AccountInfo{TotalEquity: 1000}}
	at.setPendingEntry(&pendingEntry{Symbol: "AUSDT", Side: "short", Price: 100, Quantity: 12.5, StopLoss: 102}) // 25U short
	at.setPendingEntry(&pendingEntry{Symbol: "BUSDT", Side: "long", Price: 100, Quantity: 12.5, StopLoss: 98})   // 25U long
	short := &kernel.Decision{Action: "open_short", Symbol: "CUSDT", PositionSizeUSD: 1000, StopLoss: 102}       // 20U
	if blocked, _, _ := at.accountRiskExposureBlocks(short, 100, ctx); !blocked {
		t.Fatal("pending long must not offset 25U pending short + 20U candidate (4.5% > 3%)")
	}
}

// area4 #2: a candidate the engine filtered (market data unavailable) is not
// an entry candidate; an open that cannot be priced is refused.
func TestReview1020FilteredCandidateAndUnpriceableOpensRejected(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{})
	d := kernel.Decision{Symbol: "AUSDT", Action: "open_long", StopLoss: 95, PositionSizeUSD: 100, Confidence: 95}
	ctx := &kernel.Context{
		Account:            kernel.AccountInfo{TotalEquity: 1000},
		CandidateCoins:     []kernel.CandidateCoin{{Symbol: "AUSDT"}},
		FilteredCandidates: map[string]string{"AUSDT": "market data unavailable/stale"},
	}
	if got := at.applyHardRiskGates([]kernel.Decision{d}, ctx); len(got) != 0 {
		t.Fatalf("filtered candidate still opened: %+v", got)
	}
	if r := at.popFilterReason(d); !strings.Contains(r, "FILTERED_CANDIDATE") {
		t.Fatalf("filter reason = %q", r)
	}
	// Not filtered but no price anywhere (no market data, model omitted price): refuse.
	ctx.FilteredCandidates = nil
	if got := at.applyHardRiskGates([]kernel.Decision{d}, ctx); len(got) != 0 {
		t.Fatalf("unpriceable open passed with zero reservation: %+v", got)
	}
	// Valid market data restores the open.
	ctx.MarketDataMap = map[string]*market.Data{"AUSDT": {Symbol: "AUSDT", CurrentPrice: 100}}
	if got := at.applyHardRiskGates([]kernel.Decision{d}, ctx); len(got) != 1 {
		t.Fatalf("priced open rejected: %+v", got)
	}
}

func review1020RuleTrader(t *testing.T, cond string) *AutoTrader {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "rules.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if _, err := st.Rule().CreateRule("r1020", &store.RuleInput{RuleType: "hard", Name: "rule", Condition: cond, OnViolation: "block"}); err != nil {
		t.Fatal(err)
	}
	return &AutoTrader{id: "r1020", store: st}
}

// area4 #5: review rules see the program-determined entry price, not the
// model's (omittable) price.
func TestReview1020RulesUseProgramEntryPrice(t *testing.T) {
	at := review1020RuleTrader(t, `{"field":"stop_loss_pct","op":">","value":4}`)
	ctx := &kernel.Context{MarketDataMap: map[string]*market.Data{"XUSDT": {CurrentPrice: 100}}}
	wide := kernel.Decision{Symbol: "XUSDT", Action: "open_long", StopLoss: 95, TakeProfit: 120, Leverage: 2}
	if got := at.preTradeRuleCheckWithContext([]kernel.Decision{wide}, 1000, ctx); len(got) != 0 {
		t.Fatalf("5%% stop with omitted price bypassed stop_loss_pct>4 block: %+v", got)
	}
	tight := wide
	tight.StopLoss = 98
	if got := at.preTradeRuleCheckWithContext([]kernel.Decision{tight}, 1000, ctx); len(got) != 1 {
		t.Fatalf("2%% stop must pass: %+v", got)
	}
	// A model-supplied price that makes the stop look tight must not be trusted for a market open.
	spoof := wide
	spoof.Price = 94
	if got := at.preTradeRuleCheckWithContext([]kernel.Decision{spoof}, 1000, ctx); len(got) != 0 {
		t.Fatalf("model price overrode the program price: %+v", got)
	}
	// No entry price at all + price-based rule: fail closed.
	if got := at.preTradeRuleCheckWithContext([]kernel.Decision{wide}, 1000, &kernel.Context{}); len(got) != 0 {
		t.Fatalf("no entry price + price-based rule must refuse new risk: %+v", got)
	}
	// Closes always pass.
	closeD := kernel.Decision{Symbol: "XUSDT", Action: "close_long"}
	if got := at.preTradeRuleCheckWithContext([]kernel.Decision{closeD}, 1000, &kernel.Context{}); len(got) != 1 {
		t.Fatal("close was blocked")
	}
	// A rule that does not need price facts is unaffected by a missing price.
	lev := review1020RuleTrader(t, `{"field":"leverage","op":">","value":10}`)
	if got := lev.preTradeRuleCheckWithContext([]kernel.Decision{wide}, 1000, &kernel.Context{}); len(got) != 1 {
		t.Fatalf("price-independent rule must not fail closed on missing price: %+v", got)
	}
}

func review1020Short1d() *market.Data {
	md := uptrend1dData()
	// Too little history: 5 daily bars -> trend "unknown".
	md.TimeframeData["1d"].Klines = md.TimeframeData["1d"].Klines[:5]
	return md
}

// area4 #7 (trader side): block_short_1d_uptrend fails closed when the 1d
// series is missing or too short.
func TestReview1020Block1dUptrendShortFailsClosedWithoutDailyData(t *testing.T) {
	if blocked, _ := short1dUptrendBlocks(nil); !blocked {
		t.Fatal("nil market data must block")
	}
	noDaily := &market.Data{Symbol: "XUSDT", CurrentPrice: 100, TimeframeData: map[string]*market.TimeframeSeriesData{}}
	if blocked, why := short1dUptrendBlocks(noDaily); !blocked || !strings.Contains(why, "unavailable") {
		t.Fatalf("missing 1d must block: %v %q", blocked, why)
	}
	if blocked, _ := short1dUptrendBlocks(review1020Short1d()); !blocked {
		t.Fatal("insufficient 1d history must block")
	}
	if blocked, _ := short1dUptrendBlocks(uptrend1dData()); !blocked {
		t.Fatal("1d uptrend must block")
	}

	for _, tc := range []struct {
		name string
		md   *market.Data
		rc   store.RiskControlConfig
		want int
	}{
		{"enabled, no 1d", noDaily, store.RiskControlConfig{BlockShort1dUptrend: true}, 0},
		{"disabled, no 1d", noDaily, store.RiskControlConfig{}, 1},
	} {
		at := riskTestTrader(tc.rc)
		at.cycleGateStates = map[string]*kernel.GateState{"XUSDT": {ShortAllowed: true}}
		ctx := &kernel.Context{Account: kernel.AccountInfo{TotalEquity: 1000}, CandidateCoins: []kernel.CandidateCoin{{Symbol: "XUSDT"}},
			MarketDataMap: map[string]*market.Data{"XUSDT": tc.md}}
		d := kernel.Decision{Symbol: "XUSDT", Action: "open_short", StopLoss: 105, TakeProfit: 90, PositionSizeUSD: 100, Confidence: 90}
		if got := at.applyHardRiskGates([]kernel.Decision{d}, ctx); len(got) != tc.want {
			t.Errorf("%s: kept %d, want %d", tc.name, len(got), tc.want)
		}
	}
}

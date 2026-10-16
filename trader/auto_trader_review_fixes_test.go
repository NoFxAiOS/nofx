package trader

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"nofx/kernel"
	"nofx/market"
	"nofx/store"
)

// ============================================================================
// 09-28 deep-review fix regressions:
//   ⑥ executor ATR must exclude the forming bar (kernel parity)
//   ③ vendor-divergence hard-gate code enforced on ALL open paths
//   ④ daily-loss-halt baseline anchored per cycle (not at first open decision)
//   ① 1R/ROE trim markers persist across restarts (no double reduction)
// ============================================================================

// The executor's stop-band yardstick must quote the same ATR as the kernel's
// closed-bars-only computation: a just-opened forming bar (tiny range) used
// to drag Wilder ATR down ~6% and let through stops the methodology calls
// below-noise.
func TestATRPercentFromSeriesExcludesFormingBar(t *testing.T) {
	now := time.Now()
	dur := market.TimeframeDuration("1h")
	tfd := &market.TimeframeSeriesData{Timeframe: "1h"}
	for i := 0; i < 30; i++ {
		p := 100 + float64(i)*0.01
		openTime := now.Add(time.Duration(i-30)*dur + dur/2).UnixMilli() // closed
		tfd.Klines = append(tfd.Klines, market.KlineBar{
			Time: openTime, Open: p * 0.995, High: p * 1.005, Low: p * 0.995, Close: p,
		})
	}
	closedOnly := atrPercentFromSeries(tfd)

	// Append a just-opened forming bar with a tiny range (opened 5% into
	// its window). Old behavior: Wilder seed drags ATR ~6% low.
	formingOpen := now.Add(-dur / 20).UnixMilli()
	tfd.Klines = append(tfd.Klines, market.KlineBar{
		Time: formingOpen, Open: 100.3, High: 100.31, Low: 100.30, Close: 100.30,
	})
	withForming := atrPercentFromSeries(tfd)

	if math.Abs(withForming-closedOnly) > closedOnly*0.02 {
		t.Fatalf("forming bar leaked into executor ATR: %.4f vs closed-only %.4f (>2%% drift)", withForming, closedOnly)
	}
	if closedOnly <= 0 {
		t.Fatal("fixture broken: closed-only ATR must be positive")
	}
}

func vendorGateTrader() *AutoTrader {
	at := riskTestTrader(store.RiskControlConfig{})
	at.cycleGateStates = map[string]*kernel.GateState{}
	return at
}

// The kernel writes VENDOR_DIVERGENCE_* into GateState.Failed; until this
// fix the executor was the one place the code was NOT enforced — a market
// order with exception evidence or a resting limit executed anyway.
func TestVendorDivergenceBlocksAllOpenPaths(t *testing.T) {
	at := vendorGateTrader()
	at.cycleGateStates["TESTUSDT"] = &kernel.GateState{
		LongFailed:  []string{"VENDOR_DIVERGENCE_1.36"},
		ShortFailed: []string{"VENDOR_DIVERGENCE_UNKNOWN"},
	}
	ctx := &kernel.Context{Account: kernel.AccountInfo{TotalEquity: 1000}, CandidateCoins: []kernel.CandidateCoin{{Symbol: "TESTUSDT"}, {Symbol: "CLEANUSDT"}}}

	out := at.applyHardRiskGates([]kernel.Decision{
		{Symbol: "TESTUSDT", Action: "open_long", Confidence: 95, Reasoning: "breakout"},
		{Symbol: "TESTUSDT", Action: "open_short", Confidence: 95, Reasoning: "breakdown"},
	}, ctx)
	if len(out) != 0 {
		t.Fatalf("vendor-diverged opens must be rejected on both paths, got %d through: %+v", len(out), out)
	}

	// Market-exception evidence does NOT override this code (prompt
	// contract: VENDOR_DIVERGENCE has no exception path).
	at.cycleGateStates["TESTUSDT"].LongMarketException = true
	out = at.applyHardRiskGates([]kernel.Decision{
		{Symbol: "TESTUSDT", Action: "open_long", Confidence: 95, Reasoning: "confirmed breakout"},
	}, ctx)
	if len(out) != 0 {
		t.Fatalf("market exception must not bypass the vendor gate, got %d through", len(out))
	}

	// A symbol without the code passes untouched.
	at.cycleGateStates["CLEANUSDT"] = &kernel.GateState{LongFailed: []string{"RR_MAX_0.90"}}
	out = at.applyHardRiskGates([]kernel.Decision{
		{Symbol: "CLEANUSDT", Action: "open_long", Confidence: 95, Reasoning: "x"},
	}, ctx)
	if len(out) != 1 {
		t.Fatalf("non-vendor codes must not be touched by this gate, got %d", len(out))
	}
}

// Anti-chase codes block a MARKET open at the executor (it buys at the live
// price the gate evaluated) but are not absolute bans for resting limits.
func TestAntiChaseCodesBlockMarketOpen(t *testing.T) {
	at := vendorGateTrader()
	at.cycleGateStates["CHASEUSDT"] = &kernel.GateState{
		LongFailed:  []string{"CHASE_EMA20_1H_7.0_GT_6"},
		ShortFailed: []string{"RSI1H_EXHAUSTED_15"},
	}
	ctx := &kernel.Context{Account: kernel.AccountInfo{TotalEquity: 1000}, CandidateCoins: []kernel.CandidateCoin{{Symbol: "CHASEUSDT"}}}
	out := at.applyHardRiskGates([]kernel.Decision{
		{Symbol: "CHASEUSDT", Action: "open_long", Confidence: 95, Reasoning: "x"},
		{Symbol: "CHASEUSDT", Action: "open_short", Confidence: 95, Reasoning: "x"},
	}, ctx)
	if len(out) != 0 {
		t.Fatalf("market chase opens must be refused, got %d through: %+v", len(out), out)
	}
	if absoluteBanCode("CHASE_EMA20_1H_7.0_GT_6") {
		t.Fatal("anti-chase codes must stay out of absoluteBanCode (pending recheck would cancel resting pullback limits)")
	}
}

// Daily-loss-halt baseline: losses between midnight and the day's first
// open DECISION used to escape the baseline (lazy anchor inside the open
// branch), letting the account lose ~1.6× the configured cap. The baseline
// is now anchored every cycle via anchorDailyBaseline.
func TestDailyLossHaltBaselineAnchoredPerCycle(t *testing.T) {
	ResetDailyBaselineRegistryForTest() // the day anchor is process-shared (F8)
	at := riskTestTrader(store.RiskControlConfig{DailyMaxLossPct: 10})

	// 00:05 UTC: two stop-outs take equity 1000 → 932.5 (−6.75%). No open
	// decision is emitted (positions are managing themselves).
	at.anchorDailyBaseline(1000)
	if halt := at.dailyLossHaltBlocks(at.config.StrategyConfig.RiskControl, 932.5); halt != "" {
		t.Fatalf("−6.75%% must not halt a 10%% cap: %s", halt)
	}
	// 03:05 UTC: the AI's first open decision of the day — the halt must
	// measure from the 1000 anchor, NOT from 932.5 (which would allow
	// losing all the way to 839.25 = 16.75% from the true day start).
	if halt := at.dailyLossHaltBlocks(at.config.StrategyConfig.RiskControl, 895); halt == "" {
		t.Fatal("equity 895 is −10.5% from the cycle-anchored baseline — must halt")
	}
}

// 1R/ROE trim markers survive restarts: the OPEN row's persisted flags are
// read as OR alongside the in-memory maps, and a successful trim writes
// through. Without this, a restart while a position still sat ≥1R re-fired
// the 50% reduction (and, with tp_trim_yields_to_lock=false, the ROE 1/3
// tier too).
func TestTrimFlagsPersistAcrossRestart(t *testing.T) {
	st, err := store.NewWithConfig(store.DBConfig{Type: store.DBTypeSQLite, Path: filepath.Join(t.TempDir(), "trim.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	traderID := "t1"
	if err := st.Position().CreateOpenPosition(&store.TraderPosition{
		TraderID: traderID, Symbol: "AAAUSDT", Side: "LONG",
		Quantity: 1, EntryQuantity: 2, EntryPrice: 100, Status: "OPEN",
	}); err != nil {
		t.Fatal(err)
	}

	at := &AutoTrader{id: traderID, store: st}
	if at.storeR1TrimDone("AAAUSDT", "long") {
		t.Fatal("fresh row must read not-trimmed")
	}

	// The restart boundary: everything the process knew is gone; the row is
	// what a new process reads.
	at.persistTrimFlags("AAAUSDT", "long")
	at2 := &AutoTrader{id: traderID, store: st}
	if !at2.storeR1TrimDone("AAAUSDT", "long") {
		t.Fatal("persisted r1_trim_done must survive the process boundary")
	}

	// Store-level mark methods are idempotent and side-scoped.
	if err := st.Position().MarkTPTrimDone(traderID, "AAAUSDT", "long"); err != nil {
		t.Fatal(err)
	}
	if err := st.Position().MarkTPTrimDone(traderID, "AAAUSDT", "long"); err != nil {
		t.Fatal(err)
	}
	row, err := st.Position().GetOpenPositionBySymbol(traderID, "AAAUSDT", "LONG")
	if err != nil || row == nil {
		t.Fatalf("open row missing: %v", err)
	}
	if !row.R1TrimDone || !row.TPTrimDone {
		t.Fatalf("row flags = r1 %v tp %v, want true/true", row.R1TrimDone, row.TPTrimDone)
	}
}

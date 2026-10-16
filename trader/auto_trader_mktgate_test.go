package trader

import (
	"strings"
	"testing"

	"nofx/kernel"
	"nofx/store"
)

// Pins for the market-exception enforcement at the open dispatch
// (QUANT_REVIEW_2026-09-22 B1): in limit-entry mode a market open needs
// program evidence (GateState.MarketException) plus confidence ≥ the
// published bar; without it the decision degrades to the anchor limit, and
// with no anchor it is rejected. Market-default strategies are exempt.
func TestMarketExceptionGate(t *testing.T) {
	on := true
	rc := store.RiskControlConfig{LimitEntryEnabled: on}
	at := riskTestTrader(rc)
	at.cycleGateStates = map[string]*kernel.GateState{
		"TESTUSDT": {
			LongAllowed:       true,
			LongLimitAllowed:  true,
			LongEntryPrice:    99.5,
			ShortLimitAllowed: false,
		},
	}
	rec := &store.DecisionAction{}

	// 1. No evidence anywhere, live long anchor → degrade to the anchor limit.
	d := &kernel.Decision{Action: "open_long", Symbol: "TESTUSDT", Confidence: 90}
	out, err := at.marketExceptionGate(d, rec, true)
	if err != nil {
		t.Fatalf("degrade path returned error: %v", err)
	}
	if out.Action != "open_long_limit" || out.Price != 99.5 || !out.MarketDegraded {
		t.Fatalf("expected degrade to open_long_limit @99.5, got %s @%.2f degraded=%v", out.Action, out.Price, out.MarketDegraded)
	}

	// 2. No evidence, no anchor (short) → fail-closed rejection.
	d = &kernel.Decision{Action: "open_short", Symbol: "TESTUSDT", Confidence: 90}
	_, err = at.marketExceptionGate(d, rec, false)
	if err == nil || !strings.Contains(err.Error(), "no live anchor") {
		t.Fatalf("expected fail-closed rejection without anchor, got err=%v", err)
	}

	// 3./4. Short exception present (anchor still dead).
	at.cycleGateStates["TESTUSDT"] = &kernel.GateState{ShortMarketException: true}

	// 3. Evidence but confidence below the bar → still fail-closed.
	d = &kernel.Decision{Action: "open_short", Symbol: "TESTUSDT", Confidence: kernel.MarketExceptionMinScore - 1}
	_, err = at.marketExceptionGate(d, rec, false)
	if err == nil || !strings.Contains(err.Error(), "no live anchor") {
		t.Fatalf("short exception with low confidence must not pass and short has no anchor, got err=%v", err)
	}

	// 4. Evidence + confidence → market open passes untouched.
	d = &kernel.Decision{Action: "open_short", Symbol: "TESTUSDT", Confidence: kernel.MarketExceptionMinScore}
	out, err = at.marketExceptionGate(d, rec, false)
	if err != nil || out.Action != "open_short" || out.MarketDegraded {
		t.Fatalf("bona-fide exception must pass untouched, got action=%s err=%v", out.Action, err)
	}

	// 5. Market-default strategy (limit entry off) is exempt entirely.
	atFree := riskTestTrader(store.RiskControlConfig{LimitEntryEnabled: false})
	d = &kernel.Decision{Action: "open_long", Symbol: "TESTUSDT", Confidence: 10}
	out, err = atFree.marketExceptionGate(d, rec, true)
	if err != nil || out.Action != "open_long" {
		t.Fatalf("market-default strategy must bypass the gate, got action=%s err=%v", out.Action, err)
	}

	// 6. Missing gate state → fail-closed (decisions execute in the same
	// cycle their state was captured; absence is an anomaly).
	at.cycleGateStates = nil
	d = &kernel.Decision{Action: "open_long", Symbol: "TESTUSDT", Confidence: 90}
	_, err = at.marketExceptionGate(d, rec, true)
	if err == nil || !strings.Contains(err.Error(), "no hard-gate state") {
		t.Fatalf("missing gate state must fail closed, got err=%v", err)
	}
}

// Pins for the D2 account risk-exposure gate: existing stop-risk + new
// stop-risk vs max_account_risk_pct of equity; unprotected positions
// worst-cased at the stop-band cap.
func TestAccountRiskExposureBlocks(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{MaxAccountRiskPct: 10})
	ctx := &kernel.Context{
		Account: kernel.AccountInfo{TotalEquity: 1000},
		Positions: []kernel.PositionInfo{
			{Symbol: "AUSDT", Side: "long", EntryPrice: 100, Quantity: 1, StopLossPrice: 95},   // risk 5
			{Symbol: "BUSDT", Side: "short", EntryPrice: 50, Quantity: 2, StopLossPrice: 51.5}, // risk 3
		},
	}

	// Existing 0.8% + new 1% = 1.8% ≤ 10% → allowed.
	d := &kernel.Decision{Symbol: "CUSDT", Price: 200, StopLoss: 196, PositionSizeUSD: 500}
	if blocked, _, _ := at.accountRiskExposureBlocks(d, 200, ctx); blocked {
		t.Fatal("1.8% total risk must pass a 10% cap")
	}

	// R8-clamp semantics (2026-09-26 NILUSDT): an oversized candidate is
	// CLAMPED to the risk budget (1.5% default) and PASSES — the executor
	// would size it identically; hard-rejecting what sizing would shrink is
	// inconsistent. 10.4% raw → 1.5% clamped → 0.8+1.5 = 2.3% ≤ 10%.
	d = &kernel.Decision{Symbol: "CUSDT", Price: 100, StopLoss: 94.8, PositionSizeUSD: 2000}
	blocked, _, cand := at.accountRiskExposureBlocks(d, 100, ctx)
	if blocked {
		t.Fatalf("oversized candidate must be clamped into compliance, got blocked: %v", blocked)
	}
	if cand > 15.0+1e-9 {
		t.Fatalf("booked risk must be the clamped 1.5%% (15U of 1000), got %.2f", cand)
	}

	// The account cap now binds via EXISTING exposure: two big positions
	// (4.75%+4.75%) + a clamped 1.5% candidate = 11% > 10% → blocked.
	ctx.Positions = []kernel.PositionInfo{
		{Symbol: "AUSDT", Side: "long", EntryPrice: 100, Quantity: 9.5, StopLossPrice: 95},
		{Symbol: "BUSDT", Side: "long", EntryPrice: 100, Quantity: 9.5, StopLossPrice: 95},
	}
	d = &kernel.Decision{Symbol: "CUSDT", Price: 100, StopLoss: 94.8, PositionSizeUSD: 2000}
	blocked, reason, _ := at.accountRiskExposureBlocks(d, 100, ctx)
	if !blocked {
		t.Fatal("existing 9.5%% + candidate 1.5%% must block at the 10%% cap")
	}
	if !strings.Contains(reason, "10.0%") {
		t.Fatalf("reason should quote the cap, got: %s", reason)
	}

	// Unprotected position worst-cased at 8%: 300 notional → 24 risk (2.4%).
	ctx.Positions = append(ctx.Positions, kernel.PositionInfo{Symbol: "DUSDT", Side: "long", EntryPrice: 300, Quantity: 1})
	d = &kernel.Decision{Symbol: "CUSDT", Price: 200, StopLoss: 190, PositionSizeUSD: 1500} // 7.5%
	blocked, reason, _ = at.accountRiskExposureBlocks(d, 200, ctx)
	if !blocked {
		t.Fatal("unprotected worst-casing should push this over the cap")
	}
	if !strings.Contains(reason, "unprotected") {
		t.Fatalf("reason should disclose the unprotected worst-case, got: %s", reason)
	}

	// Disabled (negative config) → never blocks.
	atOff := riskTestTrader(store.RiskControlConfig{MaxAccountRiskPct: -1})
	if atOff.config.StrategyConfig.RiskControl.EffectiveMaxAccountRiskPct() != 0 {
		t.Fatal("negative max_account_risk_pct must disable the cap")
	}

	// Missing stop on the DECISION → not this gate's problem (mandatory-SL gate).
	d = &kernel.Decision{Symbol: "CUSDT", Price: 200, StopLoss: 0, PositionSizeUSD: 9000}
	if blocked, _, _ := at.accountRiskExposureBlocks(d, 200, ctx); blocked {
		t.Fatal("unpriceable decision must be left to the mandatory-SL gate")
	}
}

func TestNetDirectionalRiskExposureBlocks(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{
		MaxAccountRiskPct:        20,
		MaxNetDirectionalRiskPct: 3,
		RiskPerTradePct:          2,
	})
	ctx := &kernel.Context{
		Account: kernel.AccountInfo{TotalEquity: 1000},
		Positions: []kernel.PositionInfo{
			{Symbol: "AUSDT", Side: "long", EntryPrice: 100, Quantity: 5, StopLossPrice: 95}, // 25U long risk
		},
	}
	long := &kernel.Decision{Action: "open_long", Symbol: "BUSDT", PositionSizeUSD: 500, StopLoss: 98}
	if blocked, reason, _ := at.accountRiskExposureBlocks(long, 100, ctx); !blocked || !strings.Contains(reason, "max_net_directional_risk_pct") {
		t.Fatalf("projected 3.5%% net long risk must block: blocked=%v reason=%s", blocked, reason)
	}
	short := &kernel.Decision{Action: "open_short", Symbol: "BUSDT", PositionSizeUSD: 500, StopLoss: 102}
	if blocked, reason, _ := at.accountRiskExposureBlocks(short, 100, ctx); blocked {
		t.Fatalf("opposite-side candidate lowers net concentration and must pass: %s", reason)
	}
	if got := (store.RiskControlConfig{}).EffectiveMaxNetDirectionalRiskPct(); got != 6 {
		t.Fatalf("default net directional risk cap = %.1f, want 6", got)
	}
}

func TestBinanceSymbolGrossStopRiskIncludesBothSidesAndPending(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{MaxAccountRiskPct: 10, RiskPerTradePct: 1.5})
	at.exchange = "binance"
	ctx := &kernel.Context{
		Account: kernel.AccountInfo{TotalEquity: 1000},
		Positions: []kernel.PositionInfo{{
			Symbol: "SOLUSDT", Side: "long", EntryPrice: 100, Quantity: 1, StopLossPrice: 95,
		}}, // 5 USDT
	}
	at.setPendingEntry(&pendingEntry{
		Symbol: "SOLUSDT", Side: "short", Price: 100, Quantity: 1, StopLoss: 105,
	}) // 5 USDT reserved
	d := &kernel.Decision{Action: "open_short", Symbol: "SOLUSDT", Price: 100, StopLoss: 105, PositionSizeUSD: 120}
	if blocked, _, _ := at.accountRiskExposureBlocks(d, 100, ctx); blocked {
		t.Fatal("replacing the same-side pending entry must not count it twice")
	}
	d.Action = "open_long"
	d.StopLoss = 95
	blocked, reason, _ := at.accountRiskExposureBlocks(d, 100, ctx)
	if !blocked || !strings.Contains(reason, "gross long+short stop-risk") {
		t.Fatalf("opposite-side pending risk must count against the symbol budget: blocked=%v reason=%s", blocked, reason)
	}
	at.dropPendingEntry("SOLUSDT", "short")
	at.cycleSymbolRiskReservedUSD = map[string]float64{"SOLUSDT": 7}
	if blocked, reason, _ := at.accountRiskExposureBlocks(d, 100, ctx); !blocked || !strings.Contains(reason, "gross long+short stop-risk") {
		t.Fatalf("5 existing + 6 candidate + 7 reserved exceeds 15: blocked=%v reason=%s", blocked, reason)
	}
	ctx.Account.TotalEquity = 0
	if blocked, _, _ := at.accountRiskExposureBlocks(d, 100, ctx); !blocked {
		t.Fatal("unknown equity must block Binance opens because the risk budget cannot be verified")
	}
}

// Pins for the D2 daily-loss halt: anchored at the first equity of each UTC
// day, halting opens past the threshold, never blocking without config.
func TestDailyLossHaltBlocks(t *testing.T) {
	ResetDailyBaselineRegistryForTest() // the day anchor is process-shared (F8)
	at := &AutoTrader{}
	rc := store.RiskControlConfig{DailyMaxLossPct: 10}

	// First sighting anchors the baseline and never halts same-call.
	if halt := at.dailyLossHaltBlocks(rc, 1000); halt != "" {
		t.Fatalf("baseline anchor must not halt: %s", halt)
	}
	if halt := at.dailyLossHaltBlocks(rc, 950); halt != "" {
		t.Fatalf("−5%% must not halt a 10%% cap: %s", halt)
	}
	if halt := at.dailyLossHaltBlocks(rc, 895); halt == "" {
		t.Fatalf("−10.5%% must halt, got %q", halt)
	}

	// Next UTC day re-anchors (simulated by rewinding the anchor day — the
	// registry bucket is per-day, so a rolled-over day finds no anchor and
	// re-anchors fresh; the reset mirrors that rollover).
	at.dayStartDay = "2000-01-01"
	at.dailyHaltDay = "2000-01-01" // the halt latches per UTC day: a rolled-over day clears it
	ResetDailyBaselineRegistryForTest()
	if halt := at.dailyLossHaltBlocks(rc, 900); halt != "" {
		t.Fatalf("new day must re-anchor, got %s", halt)
	}

	// Disabled (negative) → always empty.
	rcOff := store.RiskControlConfig{DailyMaxLossPct: -1}
	if halt := (&AutoTrader{}).dailyLossHaltBlocks(rcOff, 1); halt != "" {
		t.Fatalf("disabled halt must stay empty, got %s", halt)
	}
	if store.DefaultMaxAccountRiskPct != 10 || store.DefaultDailyMaxLossPct != 10 {
		t.Fatal("defaults moved — the prompt renders these constants, keep them in sync with the review")
	}
}

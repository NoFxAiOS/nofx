package trader

import (
	"strings"
	"testing"
	"time"

	"nofx/kernel"
	"nofx/market"
	"nofx/store"
)

func riskTestTrader(rc store.RiskControlConfig) *AutoTrader {
	return &AutoTrader{
		config:                AutoTraderConfig{StrategyConfig: &store.StrategyConfig{RiskControl: rc}},
		positionFirstSeenTime: make(map[string]int64),
		positionStopLoss:      make(map[string]float64),
	}
}

func TestValidateOpenRisk(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{MinRiskRewardRatio: 3})

	// Missing stop loss → reject (entry would be unprotected).
	err := at.validateOpenRisk(&kernel.Decision{Action: "open_short", Symbol: "BRUSDT", StopLoss: 0, TakeProfit: 0.24}, 0.2933, 3.5, 3.5)
	if err == nil {
		t.Fatal("open without stop_loss must be rejected")
	}

	// Wrong-side stop loss (short SL below entry) → reject.
	err = at.validateOpenRisk(&kernel.Decision{Action: "open_short", Symbol: "BRUSDT", StopLoss: 0.28, TakeProfit: 0.24}, 0.2933, 3.5, 3.5)
	if err == nil {
		t.Fatal("short with stop_loss below entry must be rejected")
	}

	// Wrong-side take profit (long TP below entry) → reject.
	err = at.validateOpenRisk(&kernel.Decision{Action: "open_long", Symbol: "INJUSDT", StopLoss: 5.0, TakeProfit: 4.8}, 5.2, 3.5, 3.5)
	if err == nil {
		t.Fatal("long with take_profit below entry must be rejected")
	}

	// Risk-reward below minimum → reject (short: risk 0.0127, reward 0.0533 ≈ 1:4.2 passes;
	// here risk 0.02, reward 0.03 → 1:1.5 fails).
	err = at.validateOpenRisk(&kernel.Decision{Action: "open_short", Symbol: "EDGEUSDT", StopLoss: 0.56, TakeProfit: 0.51}, 0.54, 3.5, 3.5)
	if err == nil {
		t.Fatal("open with risk-reward 1:1.5 below min 1:3 must be rejected")
	}

	// Valid short (entry 0.5358, SL 0.559, TP 0.4635 → RR ≈ 3.1) → pass.
	if err := at.validateOpenRisk(&kernel.Decision{Action: "open_short", Symbol: "EDGEUSDT", StopLoss: 0.559, TakeProfit: 0.46}, 0.5358, 3.5, 3.5); err != nil {
		t.Fatalf("valid short rejected: %v", err)
	}

	// Valid long (entry 100, SL 97, TP 109 → RR = 3.0) → pass.
	if err := at.validateOpenRisk(&kernel.Decision{Action: "open_long", Symbol: "BTCUSDT", StopLoss: 97, TakeProfit: 110}, 100, 3.5, 3.5); err != nil {
		t.Fatalf("valid long rejected: %v", err)
	}

	// No strategy config → no enforcement.
	at2 := &AutoTrader{}
	if err := at2.validateOpenRisk(&kernel.Decision{Action: "open_short", Symbol: "X"}, 100, 3.5, 3.5); err != nil {
		t.Fatalf("without strategy config the gate must be a no-op: %v", err)
	}
}

func TestValidateOpenRiskRejectsBStockWithoutDailyATR(t *testing.T) {
	market.SetEquityClassificationForTesting(
		map[string]bool{"AAPLUSDT": true},
		map[string]bool{"AAPLUSDT": true},
	)
	defer market.SetEquityClassificationForTesting(nil, nil)

	at := riskTestTrader(store.RiskControlConfig{MinRiskRewardRatio: 1.5, SLMinATRMult: 1.5})
	d := &kernel.Decision{
		Action: "open_long", Symbol: "AAPLUSDT",
		StopLoss: 95, TakeProfit: 110, Leverage: 3,
	}
	if err := at.validateOpenRisk(d, 100, 0, 0); err == nil || !strings.Contains(err.Error(), "BSTOCK_DAILY_DATA_UNAVAILABLE") {
		t.Fatalf("bstock without daily ATR must fail closed, got %v", err)
	}
	if err := at.validateOpenRisk(d, 100, 2, 2); err != nil {
		t.Fatalf("bstock with daily ATR should use normal risk validation: %v", err)
	}
}

func TestMinHoldBlocksClose(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{MinHoldMinutes: 10})

	posKey := "BRUSDT_short"
	at.positionFirstSeenTime[posKey] = time.Now().Add(-4 * time.Minute).UnixMilli()
	at.SetRecordedStopLoss("BRUSDT", "short", 0.306)

	// Held 4min < 10min, price far from SL → block.
	if blocked, reason := at.minHoldBlocksClose("BRUSDT", "short", 0.295); !blocked {
		t.Fatal("close inside the observation period must be blocked")
	} else if reason == "" {
		t.Fatal("block reason must not be empty")
	}

	// Held 4min but price already crossed the stop-loss → hard exit allowed.
	if blocked, _ := at.minHoldBlocksClose("BRUSDT", "short", 0.308); blocked {
		t.Fatal("close must be allowed once price crossed the stop-loss")
	}

	// Held 15min ≥ 10min → allowed.
	at.positionFirstSeenTime[posKey] = time.Now().Add(-15 * time.Minute).UnixMilli()
	if blocked, _ := at.minHoldBlocksClose("BRUSDT", "short", 0.295); blocked {
		t.Fatal("close past the min-hold period must be allowed")
	}

	// Unknown position age → never block.
	if blocked, _ := at.minHoldBlocksClose("NEWUSDT", "long", 100); blocked {
		t.Fatal("unknown position age must not block closes")
	}

	// Gate disabled (min_hold_minutes = 0) → never block.
	at2 := riskTestTrader(store.RiskControlConfig{})
	at2.positionFirstSeenTime[posKey] = time.Now().UnixMilli()
	if blocked, _ := at2.minHoldBlocksClose("BRUSDT", "short", 0.295); blocked {
		t.Fatal("disabled min-hold gate must not block closes")
	}
}

func uptrend1dData() *market.Data {
	now := time.Now()
	tfData := &market.TimeframeSeriesData{Timeframe: "1d"}
	p := 50.0
	for i := 0; i < 60; i++ {
		step := 0.008
		if (i%5) == 4 && i != 59 {
			step = -0.003
		}
		p *= 1 + step
		barStart := now.Add(time.Duration(i-60) * 24 * time.Hour)
		tfData.Klines = append(tfData.Klines, market.KlineBar{
			Time: barStart.UnixMilli(), Open: p * (1 - step), High: p * 1.002,
			Low: p * 0.998, Close: p, Volume: 1000,
		})
	}
	return &market.Data{
		Symbol: "BRUSDT", CurrentPrice: p,
		TimeframeData: map[string]*market.TimeframeSeriesData{"1d": tfData},
	}
}

func TestApplyHardRiskGates(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{
		BlockShort1dUptrend: true,
		MinHoldMinutes:      30,
	})
	at.positionFirstSeenTime["BRUSDT_short"] = time.Now().Add(-2 * time.Minute).UnixMilli()
	at.SetRecordedStopLoss("BRUSDT", "short", 0.32)

	ctx := &kernel.Context{
		CandidateCoins: []kernel.CandidateCoin{{Symbol: "BRUSDT"}},
		MarketDataMap:  map[string]*market.Data{"BRUSDT": uptrend1dData()},
		Positions: []kernel.PositionInfo{
			{Symbol: "BRUSDT", Side: "short", MarkPrice: 0.30},
		},
	}
	decisions := []kernel.Decision{
		{Action: "open_short", Symbol: "BRUSDT"},
		{Action: "close_short", Symbol: "BRUSDT"},
		{Action: "hold", Symbol: "BTCUSDT"},
	}

	gated := at.applyHardRiskGates(decisions, ctx)
	if len(gated) != 1 {
		t.Fatalf("gates should drop the counter-trend short and the too-early close, kept %d: %+v", len(gated), gated)
	}
	if gated[0].Action != "hold" {
		t.Fatalf("only the hold decision should survive, got %s", gated[0].Action)
	}

	// Same-trend long opens and SL-hit closes must pass untouched.
	at.positionFirstSeenTime["BRUSDT_short"] = time.Now().Add(-2 * time.Minute).UnixMilli()
	ctx.Positions[0].MarkPrice = 0.33 // beyond the recorded stop-loss 0.32
	gated = at.applyHardRiskGates([]kernel.Decision{
		{Action: "open_long", Symbol: "BRUSDT"},
		{Action: "close_short", Symbol: "BRUSDT"},
	}, ctx)
	if len(gated) != 2 {
		t.Fatalf("long opens and stop-loss-hit closes must pass the gates, kept %d", len(gated))
	}

	// Gates disabled → pass-through.
	at2 := riskTestTrader(store.RiskControlConfig{})
	if got := at2.applyHardRiskGates(decisions, ctx); len(got) != len(decisions) {
		t.Fatalf("disabled gates must pass all decisions through")
	}
}

func TestApplyHardRiskGatesBlocksLowConfidenceOpen(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{MinConfidence: 70, MaxNetDirectionalRiskPct: -1})
	ctx := &kernel.Context{Account: kernel.AccountInfo{TotalEquity: 1000}, MarketDataMap: map[string]*market.Data{}, CandidateCoins: []kernel.CandidateCoin{{Symbol: "LOWUSDT"}, {Symbol: "OKUSDT"}}}
	got := at.applyHardRiskGates([]kernel.Decision{
		{Action: "open_long", Symbol: "LOWUSDT", Confidence: 69},
		{Action: "open_short", Symbol: "OKUSDT", Confidence: 70},
		{Action: "wait", Symbol: "LOWUSDT", Confidence: 0},
	}, ctx)
	if len(got) != 2 || got[0].Symbol != "OKUSDT" || got[1].Action != "wait" {
		t.Fatalf("confidence gate kept %+v, want threshold open + non-open only", got)
	}
}

// Outlier wide stops are capped at max(2×ATR(1h), 8%): volatile coins get
// proportionally more room, but never unbounded (an 8.9% stop at 3x leverage
// is -27% margin on one trade).
func TestValidateOpenRiskStopDistanceCap(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{MinRiskRewardRatio: 1.5})

	// Short entry 100, SL 108.9 (8.9%), TP 73 → RR 3.03 passes the RR gate.
	wide := &kernel.Decision{Action: "open_short", Symbol: "HEMIUSDT", StopLoss: 108.9, TakeProfit: 73, Leverage: 3}

	// ATR 3% → cap = max(6%, 8%) = 8% → 8.9% rejected.
	if err := at.validateOpenRisk(wide, 100, 3.0, 3.0); err == nil {
		t.Fatal("8.9% stop with cap 8% must be rejected")
	}
	// ATR 6% → cap = 12% → the same stop passes (volatility-proportional room).
	if err := at.validateOpenRisk(wide, 100, 6.0, 6.0); err != nil {
		t.Fatalf("8.9%% stop with cap 12%% must pass: %v", err)
	}
	// ATR unknown (0) → flat 8% cap still applies.
	if err := at.validateOpenRisk(wide, 100, 0, 0); err == nil {
		t.Fatal("8.9% stop with unknown ATR (8% cap) must be rejected")
	}
	// Exactly at the cap → allowed.
	ok := &kernel.Decision{Action: "open_short", Symbol: "EDGEUSDT", StopLoss: 108, TakeProfit: 87, Leverage: 3}
	if err := at.validateOpenRisk(ok, 100, 3.0, 3.0); err != nil {
		t.Fatalf("8%% stop exactly at cap must pass: %v", err)
	}
}

// Repeated gate blocks for the same symbol merge into one push: first block
// notifies, follow-ups inside the 30-minute window only bump the counter.
func TestGateNotifyDedup(t *testing.T) {
	at := &AutoTrader{}
	base := time.Now()

	// First block → notify (streak 1).
	if n, push := at.gateNotifyRecord("4USDT", base); n != 1 || !push {
		t.Fatalf("first block: streak=%d push=%v, want 1/true", n, push)
	}
	// Follow-ups inside the window → silent, counter climbs.
	for i := 2; i <= 4; i++ {
		if n, push := at.gateNotifyRecord("4USDT", base.Add(time.Duration(i)*5*time.Minute)); push {
			t.Fatalf("block %d should be silent", i)
		} else if n != i {
			t.Fatalf("block %d: streak=%d", i, n)
		}
	}
	// Another symbol has its own counter.
	if n, push := at.gateNotifyRecord("BRUSDT", base.Add(time.Minute)); n != 1 || !push {
		t.Fatalf("second symbol: streak=%d push=%v, want 1/true", n, push)
	}
	// Outside the window → notify again, counter resets.
	if n, push := at.gateNotifyRecord("4USDT", base.Add(31*time.Minute)); n != 1 || !push {
		t.Fatalf("after window: streak=%d push=%v, want 1/true", n, push)
	}
}

// Entry timing gate: the finest sub-hour TF must align with direction —
// longs blocked when 15m is down/range, shorts blocked when 15m is up/range.
func TestEntryTimingGate(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{EntryTimingGate: true})

	sub := func(trend string) *market.Data {
		// Reuse kernel trend classification via a real series: piggyback on
		// the 1d trend builder pattern but for 15m.
		now := time.Now()
		tfData := &market.TimeframeSeriesData{Timeframe: "15m"}
		p := 100.0
		for i := 0; i < 40; i++ {
			step := 0.004
			if trend == "down" {
				step = -step
			} else if trend == "range" {
				// oscillate around the same level
				if i%4 == 2 {
					step = -0.004
				} else {
					step = 0.004
				}
			}
			p *= 1 + step
			tfData.Klines = append(tfData.Klines, market.KlineBar{
				Time: now.Add(time.Duration(i-40) * 15 * time.Minute).UnixMilli(),
				Open: p * (1 - step), High: p * 1.002, Low: p * 0.998, Close: p, Volume: 1000,
			})
		}
		return &market.Data{
			Symbol: "ZECUSDT", CurrentPrice: p,
			TimeframeData: map[string]*market.TimeframeSeriesData{"15m": tfData},
		}
	}

	mkGate := func(trend string) *kernel.Context {
		return &kernel.Context{
			CandidateCoins: []kernel.CandidateCoin{{Symbol: "ZECUSDT"}},
			MarketDataMap:  map[string]*market.Data{"ZECUSDT": sub(trend)},
		}
	}

	// 15m downtrend: open_long blocked, open_short allowed.
	if got := at.applyHardRiskGates([]kernel.Decision{{Action: "open_long", Symbol: "ZECUSDT"}}, mkGate("down")); len(got) != 0 {
		t.Fatal("long in a 15m downtrend must be blocked")
	}
	if got := at.applyHardRiskGates([]kernel.Decision{{Action: "open_short", Symbol: "ZECUSDT"}}, mkGate("down")); len(got) != 1 {
		t.Fatal("short in a 15m downtrend must pass")
	}

	// 15m uptrend: mirror.
	if got := at.applyHardRiskGates([]kernel.Decision{{Action: "open_long", Symbol: "ZECUSDT"}}, mkGate("up")); len(got) != 1 {
		t.Fatal("long in a 15m uptrend must pass")
	}
	if got := at.applyHardRiskGates([]kernel.Decision{{Action: "open_short", Symbol: "ZECUSDT"}}, mkGate("up")); len(got) != 0 {
		t.Fatal("short in a 15m uptrend must be blocked")
	}

	// Gate disabled: pass-through.
	at2 := riskTestTrader(store.RiskControlConfig{})
	if got := at2.applyHardRiskGates([]kernel.Decision{{Action: "open_long", Symbol: "ZECUSDT"}}, mkGate("down")); len(got) != 1 {
		t.Fatal("disabled gate must pass everything")
	}
}

// The RR gate must anchor at BOTH the AI's decision price and the live
// execution price: a below-floor plan passing on a lucky ticker (SOL case)
// is still a below-floor plan.
func TestValidateOpenRiskDualAnchor(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{MinRiskRewardRatio: 3})
	// SOL case: AI planned RR 2.08 at its own 103.15 reference; the live
	// ticker dipped to 102.6 where RR = 4.3 would pass.
	d := &kernel.Decision{Action: "open_long", Symbol: "SOLUSDT", Price: 103.15, StopLoss: 101.85, TakeProfit: 105.85}
	if err := at.validateOpenRisk(d, 102.6, 1.5, 1.5); err == nil {
		t.Fatal("decision-price RR 2.08 below floor 3.0 must be rejected despite lucky ticker")
	}
	// Same plan, but AI's own price respects the floor → both anchors pass.
	d2 := &kernel.Decision{Action: "open_long", Symbol: "SOLUSDT", Price: 102.7, StopLoss: 101.85, TakeProfit: 106.2}
	if err := at.validateOpenRisk(d2, 102.6, 3, 3); err != nil {
		t.Fatalf("both anchors above floor must pass: %v", err)
	}
	// Legacy calls without a decision price still enforce the live anchor.
	d3 := &kernel.Decision{Action: "open_long", Symbol: "X", StopLoss: 99, TakeProfit: 104}
	if err := at.validateOpenRisk(d3, 100, 3, 3); err != nil {
		t.Fatalf("no-decision-price path: %v", err)
	}
}

// ATR noise floor: stops closer than SLMinATRMult × ATR(1h) are rejected —
// SOL's 1.26% stop inside a 1.5% ATR was the motivating case.
func TestValidateOpenRiskATRFloor(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{MinRiskRewardRatio: 1.5, SLMinATRMult: 1.5})
	// Entry 100, SL 101.85-equivalent for a short: dist 1.26%, ATR 1.5% → floor 2.25% → reject.
	tight := &kernel.Decision{Action: "open_short", Symbol: "SOLUSDT", StopLoss: 101.26, TakeProfit: 97.5}
	if err := at.validateOpenRisk(tight, 100, 1.5, 1.5); err == nil {
		t.Fatal("1.26% stop below 2.25% floor must be rejected")
	}
	// Wider stop inside the cap passes.
	ok := &kernel.Decision{Action: "open_short", Symbol: "SOLUSDT", StopLoss: 102.5, TakeProfit: 95.0}
	if err := at.validateOpenRisk(ok, 100, 1.5, 1.5); err != nil {
		t.Fatalf("2.5%% stop above floor must pass: %v", err)
	}
	// Floor disabled (0) → tight stop passes.
	at2 := riskTestTrader(store.RiskControlConfig{MinRiskRewardRatio: 1.5})
	if err := at2.validateOpenRisk(tight, 100, 1.5, 1.5); err != nil {
		t.Fatalf("disabled floor must pass: %v", err)
	}
}

// Risk-based sizing: position value capped at equity × risk% ÷ stop-distance%.
func TestClampSizeToRisk(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{RiskPerTradePct: 1.5})
	d := &kernel.Decision{StopLoss: 101.85}

	// equity 80, risk 1.5%, dist 1.24% → cap = 80×1.5/1.24 ≈ 96.8; size 150 → clamped.
	got := at.clampSizeToRisk(d, 150, 80, 103.13)
	if got < 83 || got > 84 {
		t.Fatalf("clamp = %.2f, want ≈83.27 including 20bps cost", got)
	}
	// Smaller size than the cap passes untouched.
	if got := at.clampSizeToRisk(d, 50, 80, 103.13); got != 50 {
		t.Fatalf("size under cap must pass: %.2f", got)
	}
	// Default risk 1.5% when unset.
	at2 := riskTestTrader(store.RiskControlConfig{})
	if got := at2.clampSizeToRisk(d, 999, 80, 103.13); got > 98 {
		t.Fatalf("default 1.5%% budget not applied: %.2f", got)
	}
}

// Direction detection must treat open_long_limit/open_short_limit like their
// market counterparts — a long limit misread as a short inverted every side
// check and rejected every long limit entry.
func TestValidateOpenRiskLimitActions(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{MinRiskRewardRatio: 1.5, SLMinATRMult: 0})

	// Long limit: SL below trigger, TP above — the natural long shape.
	// The anchor is the LIMIT price (the exact fill), ATR 0.5% → floor 0.75%.
	longLimit := &kernel.Decision{Action: "open_long_limit", Symbol: "ZECUSDT", Price: 1010, StopLoss: 1000, TakeProfit: 1045}
	if err := at.validateOpenRisk(longLimit, 1010, 0.5, 0.5); err != nil {
		t.Fatalf("valid long limit rejected: %v", err)
	}

	// Short limit: SL above trigger, TP below.
	shortLimit := &kernel.Decision{Action: "open_short_limit", Symbol: "4USDT", Price: 0.0290, StopLoss: 0.0300, TakeProfit: 0.0270}
	if err := at.validateOpenRisk(shortLimit, 0.0290, 0.5, 0.5); err != nil {
		t.Fatalf("valid short limit rejected: %v", err)
	}

	// A long limit with an inverted (short-shaped) SL/TP must be rejected.
	bad := &kernel.Decision{Action: "open_long_limit", Symbol: "X", Price: 100, StopLoss: 104, TakeProfit: 96}
	if err := at.validateOpenRisk(bad, 100, 1.5, 1.5); err == nil {
		t.Fatal("inverted SL/TP for a long limit must be rejected")
	}
}

// The stop window rides TWO yardsticks (user directive 2026-09-10): the
// noise floor reads the 1h series (flat), the wide-stop cap reads the 4h
// series (fat) — a fat 4h series must not leak into the floor and a flat 1h
// series must not shrink the cap.
func TestStopLossATRYardsticks(t *testing.T) {
	now := time.Now()
	// 4h series with 2% per-bar range; 1h series tiny ranges.
	k4h := make([]market.KlineBar, 0, 40)
	p := 100.0
	for i := 0; i < 40; i++ {
		p *= 1.01
		k4h = append(k4h, market.KlineBar{Time: now.Add(time.Duration(i) * time.Hour).UnixMilli(), Open: p * 0.98, High: p * 1.02, Low: p * 0.97, Close: p, Volume: 10})
	}
	k1h := make([]market.KlineBar, 0, 40)
	p = 100.0
	for i := 0; i < 40; i++ {
		p *= 1.001
		k1h = append(k1h, market.KlineBar{Time: now.Add(time.Duration(i) * time.Hour).UnixMilli(), Open: p * 0.9995, High: p * 1.0005, Low: p * 0.9994, Close: p, Volume: 10})
	}
	data := &market.Data{
		Symbol: "TESTUSDT",
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"4h": {Klines: k4h},
			"1h": {Klines: k1h},
		},
	}
	floor := oneHourATRPct(data)
	cap := fourHourATRPct(data)
	if floor > 0.5 {
		t.Fatalf("floor yardstick (%.4f) must come from the flat 1h series, not the fat 4h one", floor)
	}
	if cap <= floor {
		t.Fatalf("cap yardstick (%.4f) must come from the fat 4h series, not the flat 1h one (%.4f)", cap, floor)
	}
	// 1h missing → deterministic fallback toward longer TFs, not a random map
	// pick: only 4h present → the floor falls back to the 4h ATR.
	only4h := &market.Data{Symbol: "TESTUSDT", TimeframeData: map[string]*market.TimeframeSeriesData{"4h": {Klines: k4h}}}
	if got := oneHourATRPct(only4h); got <= 0.5 {
		t.Fatalf("floor fallback without 1h must use 4h ATR (%.4f)", got)
	}
}

// 09-19 PONS case: the model adopted the gated stop_plan verbatim
// (SL 0.705709, validated in-band at the snapshot basis 0.6712 / d 5.14%),
// then hung it on a BETTER short entry (limit anchor 0.679254, +1.2%) — the
// same structure stop read 3.89% at that basis, under the 4.44% floor. A
// plan-equal stop is exempt from the floor: the gate basis already
// validated it, and the anchor only shifts the entry in the favorable
// direction. RR is still re-checked at this basis.
func TestValidateOpenRiskGatedPlanExemptFromFloor(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{SLMinATRMult: 1.5, MinRiskRewardRatio: 1.5})
	at.cycleGateStates = map[string]*kernel.GateState{
		"PONSUSDT": {ShortStopPlanPrice: 0.705709, LongStopPlanPrice: 0.601333},
	}

	dec := &kernel.Decision{
		Symbol: "PONSUSDT", Action: "open_short_limit", Price: 0.679254,
		StopLoss: 0.705709, TakeProfit: 0.6099,
	}
	// entry = the limit anchor; ATR(1h) 2.96% → floor 4.44%; plan stop reads
	// 3.89% at this basis.
	if err := at.validateOpenRisk(dec, 0.679254, 2.96, 6.0); err != nil {
		t.Fatalf("gated plan stop rejected: %v", err)
	}

	// A NON-plan stop below the floor must still be rejected.
	dec2 := &kernel.Decision{
		Symbol: "OTHERUSDT", Action: "open_short_limit", Price: 0.679254,
		StopLoss: 0.705709, TakeProfit: 0.6099,
	}
	if err := at.validateOpenRisk(dec2, 0.679254, 2.96, 6.0); err == nil {
		t.Fatal("non-plan below-floor stop passed — the exemption leaked")
	}
}

// The plan-parity exemption (0d7b4584): a stop that EQUALS the gated
// stop_plan passes the noise floor even at the limit-anchor basis (a deeper
// limit anchor shrinks the plan's percentage — the plan was validated
// in-band at the gate's snapshot basis and the anchor only improves the
// entry). Regression for the 2026-09-20 wiring bug: runCycle assigned
// at.cycleGateStates BEFORE the prompt build had created the map, so the
// exemption read nil and every plan-equal stop was rejected all night
// (AVAXUSDT 9.60461 == stop_plan, still rejected at 2.72% < 3.57%).
func TestNoiseFloorExemptsPlanEqualStop(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{
		MinRiskRewardRatio: 1.5,
		SLMinATRMult:       1.5,
	})
	// Gate state as the cycle's prompt build would have captured it
	// (post-fix wiring: assigned AFTER computeCoinSignal filled the map).
	at.cycleGateStates = map[string]*kernel.GateState{
		"AVAXUSDT": {LongStopPlanPrice: 9.60461, LongAllowed: true},
	}

	// Limit anchor 9.87308 (1.2% under live), stop == plan 9.60461 →
	// anchor-basis distance 2.72% < 1.5×ATR(1h) 3.57% — exempt, allowed.
	err := at.validateOpenRisk(&kernel.Decision{
		Action: "open_long_limit", Symbol: "AVAXUSDT",
		Price: 9.87308, StopLoss: 9.60461, TakeProfit: 10.823,
	}, 9.87308, 2.38, 3.87)
	if err != nil {
		t.Fatalf("plan-equal stop must be exempt from the noise floor: %v", err)
	}

	// A stop that does NOT match the gated plan stays rejected below the
	// floor (the exemption must not become a hole).
	err = at.validateOpenRisk(&kernel.Decision{
		Action: "open_long_limit", Symbol: "AVAXUSDT",
		Price: 9.87308, StopLoss: 9.65, TakeProfit: 10.823,
	}, 9.87308, 2.38, 3.87)
	if err == nil || !strings.Contains(err.Error(), "noise floor") {
		t.Fatalf("non-plan stop below floor must be rejected, got: %v", err)
	}

	// Missing gate state (the old wiring bug's runtime shape) → exemption
	// cannot fire; the floor rejects even a plan-equal stop.
	at.cycleGateStates = nil
	err = at.validateOpenRisk(&kernel.Decision{
		Action: "open_long_limit", Symbol: "AVAXUSDT",
		Price: 9.87308, StopLoss: 9.60461, TakeProfit: 10.823,
	}, 9.87308, 2.38, 3.87)
	if err == nil || !strings.Contains(err.Error(), "noise floor") {
		t.Fatalf("nil gate states must keep the floor active, got: %v", err)
	}
}

// D3 (QUANT_REVIEW 2026-09-22): the stop must sit comfortably inside the
// liquidation distance — ~1/leverage of notional, haircut 10%. At 10x the
// liq distance ≈ 9%, so a 8% stop (84% of the way to liquidation) is
// rejected; the same stop at 3x (liq ≈ 27%) passes easily.
func TestValidateOpenRiskLiquidationDistance(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{MinRiskRewardRatio: 1.5})

	// 10x, stop 8% away → stop sits at 88% of the liquidation move → reject.
	err := at.validateOpenRisk(&kernel.Decision{
		Action: "open_long", Symbol: "HIUSDT", Leverage: 10,
		Price: 100, StopLoss: 92, TakeProfit: 120, PositionSizeUSD: 50,
	}, 100, 2, 2)
	if err == nil || !strings.Contains(err.Error(), "liquidation") {
		t.Fatalf("8%% stop at 10x must be rejected vs liquidation, got err=%v", err)
	}

	// 3x, stop 8% away → liq distance ≈ 27%, stop at 30% of it → pass.
	err = at.validateOpenRisk(&kernel.Decision{
		Action: "open_long", Symbol: "HIUSDT", Leverage: 3,
		Price: 100, StopLoss: 92, TakeProfit: 120, PositionSizeUSD: 50,
	}, 100, 2, 2)
	if err != nil {
		t.Fatalf("8%% stop at 3x must pass, got err=%v", err)
	}
}

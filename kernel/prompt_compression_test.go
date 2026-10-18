package kernel

import (
	"math"
	"strings"
	"testing"
	"time"

	"nofx/market"
	"nofx/store"
)

// Regression tests for the 09-18 token-audit batch:
// ① double-blocked candidates compress to one line (GateStates survive),
// ② close-locked positions render one management line,
// ④ signal JSON numbers are precision-capped (6 sig digits / 2-dec pct).

func TestBlockedCandidateCompressesToOneLine(t *testing.T) {
	engine := NewStrategyEngine(&store.StrategyConfig{})
	ctx := &Context{MarketDataMap: map[string]*market.Data{}}
	// Fully data-less candidate → DATA_INSUFFICIENT on both directions →
	// the mechanical-wait compression.
	blocked := CandidateCoin{Symbol: "BADUSDT", Sources: []string{"ai500"}}
	ctx.CandidateCoins = append(ctx.CandidateCoins, blocked)
	data := &market.Data{Symbol: "BADUSDT", CurrentPrice: 1.0}
	ctx.MarketDataMap["BADUSDT"] = data

	prompt := engine.BuildUserPrompt(ctx)
	if !strings.Contains(prompt, "双向硬门拦截") {
		t.Fatalf("blocked candidate not compressed:\n%s", prompt)
	}
	if !strings.Contains(prompt, "DATA_INSUFFICIENT") {
		t.Fatalf("failed codes missing from the compressed line:\n%s", prompt)
	}
	if strings.Contains(prompt, `"hard_entry_gate"`) {
		t.Fatalf("full JSON still rendered for a double-blocked candidate:\n%s", prompt)
	}
	// The program-side bookkeeping must survive the compression: wait_state
	// derivation reads GateStates, the RR-decay dataset reads RRCeilings.
	if gs := ctx.GateStates[market.Normalize("BADUSDT")]; gs == nil {
		t.Fatal("GateStates not recorded for the compressed candidate")
	} else if gs.LongAllowed || gs.ShortAllowed {
		t.Fatalf("GateStates should be double-blocked, got %+v", gs)
	}
	if ctx.RRCeilings == nil || ctx.RRCeilings[market.Normalize("BADUSDT")] == nil {
		t.Fatal("RRCeilings not recorded for the compressed candidate")
	}
}

func TestAllowedCandidateKeepsFullJSON(t *testing.T) {
	cfg := &store.StrategyConfig{}
	// The monotone fixture has RSI=100 and a distant live-price anchor;
	// keep this rendering test independent of the anti-chase caps.
	cfg.RiskControl.CounterMaxEMA20_1hPct = -1
	cfg.RiskControl.EntryRSI1hMax = -1
	engine := NewStrategyEngine(cfg)
	ctx := &Context{MarketDataMap: map[string]*market.Data{}}
	now := time.Now()
	ctx.CandidateCoins = append(ctx.CandidateCoins, CandidateCoin{Symbol: "OKUSDT", Sources: []string{"ai500"}})
	ctx.MarketDataMap["OKUSDT"] = withVendor(&market.Data{Symbol: "OKUSDT", CurrentPrice: 1.0,
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"15m": buildTF("15m", now, 80, 1.0, false),
			"1h":  buildTF("1h", now, 80, 1.0, false),
			"4h":  buildTF("4h", now, 80, 1.0, false),
		}}, 0.05)
	prompt := engine.BuildUserPrompt(ctx)
	if !strings.Contains(prompt, `"hard_entry_gate"`) {
		t.Fatalf("renderable candidate lost its full JSON:\n%s", prompt)
	}
}

// buildDecliningTF: buildTF's mirror — falling closes (bearish bars), which
// are FAVORABLE for a short, so the early-close evidence counter stays at 0
// and the close gate locks (the KORU 09-18 shape).
func buildDecliningTF(tf string, now time.Time, bars int, base float64) *market.TimeframeSeriesData {
	dur := marketTFDuration(tf)
	data := &market.TimeframeSeriesData{Timeframe: tf}
	p := base
	for i := 0; i < bars; i++ {
		p *= 0.999
		barTime := now.Add(time.Duration(i-(bars-1)) * dur)
		data.Klines = append(data.Klines, market.KlineBar{
			Time: barTime.UnixMilli(), Open: p * 1.001, High: p * 1.002,
			Low: p * 0.998, Close: p, Volume: 1000,
		})
		data.EMA20Values = append(data.EMA20Values, p*1.005)
		data.EMA50Values = append(data.EMA50Values, p*1.01)
		data.MACDValues = append(data.MACDValues, -0.001)
		data.RSI14Values = append(data.RSI14Values, 45)
	}
	data.ATR14 = base * 0.004
	return data
}

func TestLockedPositionCompresses(t *testing.T) {
	engine := NewStrategyEngine(&store.StrategyConfig{})
	now := time.Now()
	tfs := map[string]*market.TimeframeSeriesData{
		"15m": buildDecliningTF("15m", now, 80, 1.0),
		"1h":  buildDecliningTF("1h", now, 80, 1.0), // falling → bearish closes → favorable to the short → 0 against-candles
		"4h":  buildDecliningTF("4h", now, 80, 1.0),
	}
	ctx := &Context{MarketDataMap: map[string]*market.Data{
		"KORUUSDT": {Symbol: "KORUUSDT", CurrentPrice: 19.28, TimeframeData: tfs},
	}}
	pos := PositionInfo{
		Symbol: "KORUUSDT", Side: "short", Managed: true,
		EntryPrice: 19.23, MarkPrice: 19.28, Quantity: 3.62,
		Leverage: 5, UpdateTime: now.Add(-46 * time.Minute).UnixMilli(),
		StopLossPrice: 19.85, TakeProfitPrice: 18.40,
	}
	out := engine.formatPositionInfo(1, pos, ctx)
	// 46min < 4h, rising 1h closes → zero against-direction candles for a
	// short → close/partial are locked this cycle; the 4-TF JSON is noise.
	if !strings.Contains(out, "平仓门锁定") {
		t.Fatalf("locked position not compressed:\n%s", out)
	}
	for _, want := range []string{"SL 19.8500", "TP 18.4000", "adjust_stop_loss"} {
		if !strings.Contains(out, want) {
			t.Fatalf("locked-position line missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, `"hard_entry_gate"`) {
		t.Fatalf("full JSON still rendered for a locked position:\n%s", out)
	}

	// Past the early-close window → full data again.
	pos.UpdateTime = now.Add(-5 * time.Hour).UnixMilli()
	out = engine.formatPositionInfo(1, pos, ctx)
	if !strings.Contains(out, `"hard_entry_gate"`) {
		t.Fatal("position past the early-close window must render full JSON")
	}

	// Near-stop danger (price within 0.5×ATR(1h) of the SL) → full data even
	// while the close gate is locked.
	pos.UpdateTime = now.Add(-46 * time.Minute).UnixMilli()
	pos.StopLossPrice = pos.MarkPrice * 1.001 // ~0.1% away, well inside 0.5×ATR
	out = engine.formatPositionInfo(1, pos, ctx)
	if !strings.Contains(out, `"hard_entry_gate"`) {
		t.Fatal("near-stop position must keep full JSON")
	}
}

func TestRenderSignalJSONPrecision(t *testing.T) {
	// Raw decimals must keep their precision (funding_rate is a raw decimal,
	// 0.00033705 = 0.033705%/settle).
	if got := roundSignificant(1.5343639999999998, 6); math.Abs(got-1.53436) > 1e-9 {
		t.Errorf("roundSignificant price = %v, want 1.53436", got)
	}
	if got := roundSignificant(0.00033705, 6); math.Abs(got-0.00033705) > 1e-12 {
		t.Errorf("roundSignificant funding = %v, want 0.00033705 (precision must survive)", got)
	}
	walkedMap := roundJSONNumbers(map[string]interface{}{
		"funding_annualized_pct":        72.48024000000001,
		"vendor_vs_live_divergence_pct": -19.394719896973594,
	}, "").(map[string]interface{})
	if math.Abs(walkedMap["funding_annualized_pct"].(float64)-72.48) > 1e-9 {
		t.Errorf("pct rounding = %v, want 72.48", walkedMap["funding_annualized_pct"])
	}
	if math.Abs(walkedMap["vendor_vs_live_divergence_pct"].(float64)-(-19.39)) > 1e-9 {
		t.Errorf("divergence rounding = %v, want -19.39", walkedMap["vendor_vs_live_divergence_pct"])
	}

	now := time.Date(2026, 9, 18, 16, 0, 0, 0, time.UTC)
	data := gateTestMarket(now, 100.0)
	data.FundingRate = 0.00033705
	data.FundingRateOK = true
	sig, err := ComputeSymbolSignals("TUSDT", data, SignalOptions{Now: now, PrimaryTF: "1h"})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	j := RenderSignalJSON(sig)
	if !strings.Contains(j, `"funding_rate":0.00033705`) {
		t.Errorf("raw funding_rate lost precision:\n%s", j[:minInt(len(j), 600)])
	}
	if strings.Contains(j, "9999999") || strings.Contains(j, "00000001") {
		t.Errorf("full-precision float junk still present in signal JSON")
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

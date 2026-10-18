package kernel

import (
	"math"
	"nofx/store"
	"strings"
	"testing"
	"time"

	"nofx/market"
)

// fp: float-pointer literal helper for optional signal fields.
func fp(v float64) *float64 { return &v }

// withVendor: stamp a vendor-vs-live measurement on a hand-built market.Data.
// Production always measures (market/data.go); fixtures bypassing that layer
// must simulate the measurement or the fail-closed vendor gate (default on,
// 1%) blocks both directions as VENDOR_DIVERGENCE_UNKNOWN.
func withVendor(d *market.Data, pct float64) *market.Data {
	d.VendorStalenessPct = fp(pct)
	return d
}

func buildTF(tf string, now time.Time, bars int, base float64, lastBarForming bool) *market.TimeframeSeriesData {
	dur := marketTFDuration(tf)
	data := &market.TimeframeSeriesData{Timeframe: tf}
	// EMA/MACD/RSI series so the signal layer can read them
	closeprices := make([]float64, bars)
	for i := 0; i < bars; i++ {
		p := base * (1 + float64(i)*0.001) // mild uptrend
		closeprices[i] = p
		barTime := now.Add(time.Duration(i-(bars-1)) * dur) // last bar starts AT now → forming
		data.Klines = append(data.Klines, market.KlineBar{
			Time: barTime.UnixMilli(), Open: p * 0.999, High: p * 1.002,
			Low: p * 0.998, Close: p, Volume: 1000,
		})
		data.EMA20Values = append(data.EMA20Values, p*0.995)
		data.EMA50Values = append(data.EMA50Values, p*0.99)
		data.MACDValues = append(data.MACDValues, 0.001)
		data.RSI14Values = append(data.RSI14Values, 55)
		data.BOLLUpper = append(data.BOLLUpper, p*1.01)
		data.BOLLLower = append(data.BOLLLower, p*0.99)
	}
	data.ATR14 = base * 0.005
	_ = lastBarForming
	return data
}

func TestSignalLayerClosedCandleAndFeatures(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	data := &market.Data{
		Symbol: "INJUSDT", CurrentPrice: 5.47, // consistent with the rising series (within its bands)
		PriceChange1h: -1.31,
		FundingRate:   -0.000101, FundingRateOK: true,
		OpenInterest:  &market.OIData{Latest: 101.9, Average: 100.0},
		TimeframeData: map[string]*market.TimeframeSeriesData{},
	}
	// 5m: 40 bars ending with a FORMING candle (its openTime + 5m > now)
	tf5 := buildTF("5m", now, 41, 5.30, false)
	tf5.ATR14 = 5.3 * 0.0082
	data.TimeframeData["5m"] = tf5
	// 2h: 40 bars, last CLOSED
	tf2 := buildTF("2h", now, 60, 5.10, false)
	tf2.ATR14 = 5.1 * 0.01
	data.TimeframeData["2h"] = tf2

	sig, err := ComputeSymbolSignals("INJUSDT", data, SignalOptions{
		Now:                  now,
		PrimaryTF:            "2h",
		LimitEntryOffsetMode: "fixed", // pin the fixed-offset contract; ATR mode has its own tests
	})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}

	// Pre-computed limit-entry anchors: snapshot price ±0.5% default offset
	// (the model copies these into `price` for open_*_limit — it must never do
	// its own math).
	if want := 5.47 * 0.995; math.Abs(sig.LimitBuyPrice-want) > 1e-9 {
		t.Errorf("limit_buy_price = %.6f, want %.6f (price×0.995)", sig.LimitBuyPrice, want)
	}
	if want := 5.47 * 1.005; math.Abs(sig.LimitSellPrice-want) > 1e-9 {
		t.Errorf("limit_sell_price = %.6f, want %.6f (price×1.005)", sig.LimitSellPrice, want)
	}

	// Strategy-configurable offset (risk_control.limit_entry_offset_pct).
	sigCustom, err := ComputeSymbolSignals("INJUSDT", data, SignalOptions{
		Now:                  now,
		PrimaryTF:            "2h",
		LimitEntryOffsetPct:  0.8,
		LimitEntryOffsetMode: "fixed",
	})
	if err != nil {
		t.Fatalf("compute custom offset: %v", err)
	}
	if want := 5.47 * 0.992; math.Abs(sigCustom.LimitBuyPrice-want) > 1e-9 {
		t.Errorf("custom offset limit_buy_price = %.6f, want %.6f (price×0.992)", sigCustom.LimitBuyPrice, want)
	}
	if want := 5.47 * 1.008; math.Abs(sigCustom.LimitSellPrice-want) > 1e-9 {
		t.Errorf("custom offset limit_sell_price = %.6f, want %.6f (price×1.008)", sigCustom.LimitSellPrice, want)
	}
	// Zero/negative offset falls back to the 0.5% default.
	sigZero, err := ComputeSymbolSignals("INJUSDT", data, SignalOptions{
		Now:                  now,
		PrimaryTF:            "2h",
		LimitEntryOffsetPct:  -1,
		LimitEntryOffsetMode: "fixed",
	})
	if err != nil {
		t.Fatalf("compute zero offset: %v", err)
	}
	if want := 5.47 * 0.995; math.Abs(sigZero.LimitBuyPrice-want) > 1e-9 {
		t.Errorf("fallback limit_buy_price = %.6f, want %.6f", sigZero.LimitBuyPrice, want)
	}

	// 5m blocks are DROPPED (09-19 audit 八: sub-15m JSON is dead weight —
	// role_tfs is 15m/1h/4h, no rule reads it). The settlement checks that
	// used to run on the 5m block are pinned on the 15m block instead.
	if _, ok := sig.Timeframes["5m"]; ok {
		t.Fatal("5m block must be dropped from the signal JSON")
	}
	tf2h := sig.Timeframes["2h"] // the fixture's primary TF
	if tf2h == nil {
		t.Fatal("2h signal missing")
	}
	if !tf2h.UnclosedDropped {
		t.Error("2h: forming candle should be dropped")
	}

	// 2h trend should be "up" on a rising series with fast EMA above slow
	tf2s := sig.Timeframes["2h"]
	if tf2s.Trend != "up" {
		t.Errorf("2h trend expected up, got %s", tf2s.Trend)
	}
	if tf2s.LastClosedCandle != "bullish" {
		t.Errorf("2h candle expected bullish, got %s", tf2s.LastClosedCandle)
	}
	if tf2s.Support == nil || len(tf2s.Support) == 0 {
		t.Error("2h support should not be empty")
	}

	// Derivatives
	if sig.Derivatives == nil || sig.Derivatives.FundingRate == nil {
		t.Error("derivatives missing")
	}
	// OI vs average = (101.9-100)/100*100 = 1.9%
	if sig.Derivatives.OIVsAvgPct == nil || *sig.Derivatives.OIVsAvgPct < 1.89 || *sig.Derivatives.OIVsAvgPct > 1.91 {
		t.Errorf("OI vs avg wrong: %v", sig.Derivatives.OIVsAvgPct)
	}

	// Data complete + JSON renders
	if !sig.DataComplete {
		t.Errorf("data should be complete: %v", sig.Warnings)
	}
	j := RenderSignalJSON(sig)
	if len(j) < 200 || !strings.Contains(j, "\"symbol\":\"INJUSDT\"") {
		t.Errorf("JSON block wrong: %s", j[:120])
	}
}

// Insufficient bars → data incomplete → prohibited.
func TestSignalLayerIncomplete(t *testing.T) {
	now := time.Now()
	data := &market.Data{
		Symbol: "XUSDT", CurrentPrice: 1.0,
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"5m": buildTF("5m", now, 5, 1.0, false), // only 5 bars
		},
	}
	sig, err := ComputeSymbolSignals("XUSDT", data, SignalOptions{Now: now})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if sig.DataComplete {
		t.Error("5 bars should be incomplete")
	}
	if len(sig.Warnings) == 0 {
		t.Error("expected warnings")
	}
}

// Full 60+ bar history on every ANALYZED timeframe (15m/1h/4h) must yield
// sufficient=true. A required-but-never-fetched timeframe (the old static
// "1d" entry) must not pin the flag false forever — review 2026-09-07.
func TestSignalLayerSufficientWithFullBars(t *testing.T) {
	now := time.Now()
	data := &market.Data{
		Symbol: "XUSDT", CurrentPrice: 1.0,
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"15m": buildTF("15m", now, 100, 1.0, false),
			"1h":  buildTF("1h", now, 100, 1.0, false),
			"4h":  buildTF("4h", now, 100, 1.0, false),
			// deliberately NO "1d" — the live pipeline never fetches it
		},
	}
	sig, err := ComputeSymbolSignals("XUSDT", data, SignalOptions{Now: now, PrimaryTF: "15m"})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if !sig.DataQuality.Sufficient {
		t.Fatalf("100 bars on 15m/1h/4h must be sufficient: %+v", sig.DataQuality)
	}
	if _, has1d := sig.DataQuality.AvailableBars["1d"]; has1d {
		t.Errorf("unanalyzed timeframe 1d must not appear in available_bars: %+v", sig.DataQuality.AvailableBars)
	}

	// A genuinely short required timeframe still flips the flag.
	short := &market.Data{
		Symbol: "XUSDT", CurrentPrice: 1.0,
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"15m": buildTF("15m", now, 100, 1.0, false),
			"1h":  buildTF("1h", now, 100, 1.0, false),
			"4h":  buildTF("4h", now, 30, 1.0, false), // < 60
		},
	}
	sigShort, err := ComputeSymbolSignals("XUSDT", short, SignalOptions{Now: now, PrimaryTF: "15m"})
	if err != nil {
		t.Fatalf("compute short: %v", err)
	}
	if sigShort.DataQuality.Sufficient {
		t.Error("4h with 30 bars must be insufficient")
	}
	if len(sigShort.DataQuality.Shortfall) == 0 {
		t.Error("expected a shortfall entry for 4h")
	}
}

// Round-4 review R4-3 regression: a strategy whose fetch list has no 4h
// (default template ["5m","15m","1h"], aggressive preset ["3m","15m","1h"])
// must NOT be pinned to DATA_INSUFFICIENT by the static 4h requirement —
// GetWithTimeframes fetches only the configured list. A configured-but-
// missing timeframe still fails closed.
func TestDataQualityRespectsConfiguredTimeframes(t *testing.T) {
	now := time.Now()
	// No 4h configured → no 4h requirement; the strategy trades.
	no4h := &market.Data{
		Symbol: "XUSDT", CurrentPrice: 1.0,
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"5m":  buildTF("5m", now, 100, 1.0, false),
			"15m": buildTF("15m", now, 100, 1.0, false),
			"1h":  buildTF("1h", now, 100, 1.0, false),
		},
	}
	sig, err := ComputeSymbolSignals("XUSDT", no4h, SignalOptions{
		Now: now, PrimaryTF: "5m",
		ConfiguredTimeframes: []string{"5m", "15m", "1h"},
	})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if !sig.DataQuality.Sufficient {
		t.Fatalf("configured {5m,15m,1h} with 100 bars each must be sufficient: %+v", sig.DataQuality)
	}
	if _, has := sig.DataQuality.MinimumBars["4h"]; has {
		t.Errorf("unconfigured 4h must not appear in minimum_bars: %+v", sig.DataQuality.MinimumBars)
	}

	// Configured but unfetched → still fail closed (fetch failed mid-cycle).
	broken := &market.Data{
		Symbol: "XUSDT", CurrentPrice: 1.0,
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"15m": buildTF("15m", now, 100, 1.0, false),
			"1h":  buildTF("1h", now, 100, 1.0, false),
			// 4h configured by the strategy but the fetch failed
		},
	}
	sigBroken, err := ComputeSymbolSignals("XUSDT", broken, SignalOptions{
		Now: now, PrimaryTF: "15m",
		ConfiguredTimeframes: []string{"15m", "1h", "4h"},
	})
	if err != nil {
		t.Fatalf("compute broken: %v", err)
	}
	if sigBroken.DataQuality.Sufficient {
		t.Error("configured-but-missing 4h must stay insufficient")
	}
	found4hShortfall := false
	for _, s := range sigBroken.DataQuality.Shortfall {
		if strings.Contains(s, "4h") {
			found4hShortfall = true
		}
	}
	if !found4hShortfall {
		t.Errorf("shortfall must name the missing 4h: %+v", sigBroken.DataQuality.Shortfall)
	}

	// Legacy callers (execution-side recompute, no config plumbed) keep the
	// historical {15m,1h,4h} requirement.
	legacy, err := ComputeSymbolSignals("XUSDT", broken, SignalOptions{Now: now, PrimaryTF: "15m"})
	if err != nil {
		t.Fatalf("compute legacy: %v", err)
	}
	if legacy.DataQuality.Sufficient {
		t.Error("legacy path without 4h data must stay insufficient")
	}
}

// A steady uptrend should pin StochRSI in the overbought band (K > 80), and
// the values must exist on timeframes with enough closed bars.
func TestStochRSIUptrendOverbought(t *testing.T) {
	n := 60
	c := make([]float64, n)
	base := 100.0
	// Realistic uptrend: net gains with periodic pullbacks so the RSI series
	// has variation (a perfectly monotonic series pins RSI at 100 and
	// degenerates the stochastic range).
	deltas := []float64{0.02, 0.01, 0.03, -0.004}
	for i := 0; i < n-3; i++ {
		base *= 1 + deltas[i%len(deltas)]
		c[i] = base
	}
	// Finish with three strong up bars: the latest RSI prints near its
	// 14-bar high, so StochRSI must be elevated.
	for i := n - 3; i < n; i++ {
		base *= 1.02
		c[i] = base
	}
	k, d, ok := stochRSI(c, 14, 14, 3, 3)
	if !ok {
		t.Fatal("stochRSI should be computable with 60 bars")
	}
	if k <= 70 || d <= 60 {
		t.Fatalf("uptrend stochRSI should be elevated, got K=%.1f D=%.1f", k, d)
	}

	// Oscillating series: K/D stay mid-range, never pinned.
	osc := make([]float64, n)
	for i := 0; i < n; i++ {
		osc[i] = 100 + 5*math.Sin(float64(i)/2)
	}
	if k, d, ok := stochRSI(osc, 14, 14, 3, 3); !ok || k > 95 || d > 95 {
		t.Fatalf("oscillating series stochRSI should be mid-range, got K=%.1f D=%.1f ok=%v", k, d, ok)
	}
}

// buildTrendTF builds a closed-bar series for one timeframe following a steady
// up or down trend (with mild alternation so RSI stays non-degenerate).
func buildTrendTF(tf string, now time.Time, bars int, base float64, up bool) *market.TimeframeSeriesData {
	dur := marketTFDuration(tf)
	data := &market.TimeframeSeriesData{Timeframe: tf}
	p := base
	for i := 0; i < bars; i++ {
		step := 0.006
		if (i%5) == 4 && i != bars-1 { // counter-move, never on the final bar
			step = -0.002
		}
		if !up {
			step = -step
		}
		p *= 1 + step
		barStart := now.Add(time.Duration(i-bars) * dur) // last bar starts one dur before now → closed
		data.Klines = append(data.Klines, market.KlineBar{
			Time: barStart.UnixMilli(), Open: p * (1 - step), High: p * 1.002,
			Low: p * 0.998, Close: p, Volume: 1000,
		})
	}
	return data
}

// The deterministic entry/exit rules must be driven by the strategy's
// configured primary timeframe, not blindly by the longest one available.
func TestPrimaryTFDrivesEntryExitRules(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	newData := func() *market.Data {
		return &market.Data{
			Symbol: "TESTUSDT", CurrentPrice: 100.0,
			TimeframeData: map[string]*market.TimeframeSeriesData{},
		}
	}

	// 15m is in a downtrend with a bearish close; 1d is in an uptrend.
	// With primary_timeframe=15m the exit rule (down trend + bearish candle)
	// must fire from the 15m block — the old longest-TF behavior (1d) hid it.
	// The exit rule is direction-aware (2026-10-09 ONUSDT): this rule is the
	// LONG side's invalidation, so the assertions pass PositionSide "long".
	data := newData()
	data.TimeframeData["15m"] = buildTrendTF("15m", now, 60, 105, false)
	data.TimeframeData["1d"] = buildTrendTF("1d", now, 60, 60, true)
	sig, err := ComputeSymbolSignals("TESTUSDT", data, SignalOptions{Now: now, PrimaryTF: "15m", PositionSide: "long"})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if !sig.ExitTriggered {
		t.Fatalf("exit rule should fire from the configured 15m primary TF (down+bearish)")
	}
	if sig.EntryTriggered {
		t.Fatalf("entry rule should not fire while the primary TF is in a downtrend")
	}

	// Same market, but the configured primary TF is missing from the block →
	// the rules must fall back to the longest timeframe: entry/exit must match
	// what a block containing ONLY that longest timeframe would produce.
	data2 := newData()
	data2.TimeframeData["15m"] = buildTrendTF("15m", now, 60, 105, false)
	data2.TimeframeData["1d"] = buildTrendTF("1d", now, 60, 60, true)
	sig2, err := ComputeSymbolSignals("TESTUSDT", data2, SignalOptions{Now: now, PrimaryTF: "4h", PositionSide: "long"})
	if err != nil {
		t.Fatalf("compute fallback: %v", err)
	}
	dataOnly1d := newData()
	dataOnly1d.TimeframeData["1d"] = buildTrendTF("1d", now, 60, 60, true)
	sigOnly1d, err := ComputeSymbolSignals("TESTUSDT", dataOnly1d, SignalOptions{Now: now, PrimaryTF: "1d", PositionSide: "long"})
	if err != nil {
		t.Fatalf("compute 1d-only: %v", err)
	}
	if sig2.ExitTriggered != sigOnly1d.ExitTriggered || sig2.EntryTriggered != sigOnly1d.EntryTriggered {
		t.Fatalf("fallback should evaluate on the longest TF: got exit=%v entry=%v, want exit=%v entry=%v",
			sig2.ExitTriggered, sig2.EntryTriggered, sigOnly1d.ExitTriggered, sigOnly1d.EntryTriggered)
	}
}

// TimeframeTrend exposes the deterministic trend classification to the trader's
// hard risk gates.
func TestTimeframeTrend(t *testing.T) {
	now := time.Now()
	upData := &market.Data{
		Symbol: "UPUSDT", CurrentPrice: 100.0,
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"1d": buildTrendTF("1d", now, 60, 60, true),
		},
	}
	if got := TimeframeTrend(upData, "1d"); got != "up" {
		t.Fatalf("1d trend = %q, want up", got)
	}
	downData := &market.Data{
		Symbol: "DOWNUSDT", CurrentPrice: 50.0,
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"1d": buildTrendTF("1d", now, 60, 120, false),
		},
	}
	if got := TimeframeTrend(downData, "1d"); got != "down" {
		t.Fatalf("1d trend = %q, want down", got)
	}
	if got := TimeframeTrend(nil, "1d"); got != "" {
		t.Fatalf("nil data trend = %q, want empty", got)
	}
	if got := TimeframeTrend(upData, "4h"); got != "" {
		t.Fatalf("missing timeframe trend = %q, want empty", got)
	}
}

// Regression for the support/resistance anchor bug: pivot levels must be
// classified against the LIVE price, not each timeframe's last closed close.
// FFUSDT-style case: daily close 0.10241 but live price 0.11037 already
// cleared the 0.107 swing high — listing it as "resistance" is wrong.
func TestSupportResistanceAnchorUsesLivePrice(t *testing.T) {
	now := time.Now()
	dur := marketTFDuration("1d")
	bars := func() []market.KlineBar {
		var out []market.KlineBar
		for i := 0; i < 20; i++ {
			c := 0.095 + float64(i)*0.0005
			out = append(out, market.KlineBar{
				Time: now.Add(time.Duration(i-20) * dur).UnixMilli(),
				Open: c - 0.0005, High: c + 0.001, Low: c - 0.001, Close: c,
				Volume: 1000,
			})
		}
		// A clean swing high at bar 5: 0.107 with strictly lower shoulders.
		out[5].High, out[5].Close = 0.107, 0.106
		// Last closed bar mimics the daily close of the reported case.
		out[19].Close, out[19].High = 0.10241, 0.103
		return out
	}

	build := func(live float64, optPrice float64) *SymbolSignal {
		data := &market.Data{
			Symbol: "FFUSDT", CurrentPrice: live,
			TimeframeData: map[string]*market.TimeframeSeriesData{
				"1d": {Timeframe: "1d", Klines: bars()},
			},
		}
		sig, err := ComputeSymbolSignals("FFUSDT", data, SignalOptions{Now: now, PrimaryTF: "1d", CurrentPrice: optPrice})
		if err != nil {
			t.Fatalf("compute: %v", err)
		}
		return sig
	}

	// Live price above the swing high → it must NOT be listed as resistance.
	sig := build(0.11037, 0.11037)
	for _, r := range sig.Timeframes["1d"].Resistance {
		if r <= 0.11037 {
			t.Fatalf("broken level %.5f still listed as resistance (live price 0.11037)", r)
		}
	}

	// Fallback: no live price supplied → anchor is data.CurrentPrice (0.10241),
	// and the 0.107 swing high is legitimately above it → listed.
	sig2 := build(0.10241, 0)
	found := false
	for _, r := range sig2.Timeframes["1d"].Resistance {
		if r == 0.107 {
			found = true
		}
	}
	if !found {
		t.Fatalf("swing high 0.107 should be resistance when price is 0.10241, got %v",
			sig2.Timeframes["1d"].Resistance)
	}
}

// Distances are pre-computed from the live anchor and index-aligned with the
// level arrays, so the model never does its own arithmetic.
func TestSupportResistanceDistances(t *testing.T) {
	now := time.Now()
	dur := marketTFDuration("1d")
	var bars []market.KlineBar
	for i := 0; i < 20; i++ {
		c := 0.095 + float64(i)*0.0005
		bars = append(bars, market.KlineBar{
			Time: now.Add(time.Duration(i-20) * dur).UnixMilli(),
			Open: c - 0.0005, High: c + 0.001, Low: c - 0.001, Close: c, Volume: 1000,
		})
	}
	bars[5].High, bars[5].Close = 0.107, 0.106 // swing high
	bars[8].Low, bars[8].Close = 0.090, 0.091  // swing low
	data := &market.Data{
		Symbol: "FFUSDT", CurrentPrice: 0.10241,
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"1d": {Timeframe: "1d", Klines: bars},
		},
	}
	sig, err := ComputeSymbolSignals("FFUSDT", data, SignalOptions{Now: now, PrimaryTF: "1d"})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	tf := sig.Timeframes["1d"]
	if len(tf.ResistanceDistPct) != len(tf.Resistance) || len(tf.SupportDistPct) != len(tf.Support) {
		t.Fatalf("distance arrays misaligned: res=%d/%d sup=%d/%d",
			len(tf.ResistanceDistPct), len(tf.Resistance), len(tf.SupportDistPct), len(tf.Support))
	}
	found := false
	for i, lvl := range tf.Resistance {
		if lvl == 0.107 {
			found = true
			want := (0.107 - 0.10241) / 0.10241 * 100
			if math.Abs(tf.ResistanceDistPct[i]-want) > 0.01 {
				t.Fatalf("resistance dist = %.4f, want %.4f", tf.ResistanceDistPct[i], want)
			}
		}
	}
	if !found {
		t.Fatal("expected 0.107 in resistance")
	}
	for i, lvl := range tf.Support {
		if lvl == 0.090 {
			want := (0.090 - 0.10241) / 0.10241 * 100
			if math.Abs(tf.SupportDistPct[i]-want) > 0.01 {
				t.Fatalf("support dist = %.4f, want %.4f", tf.SupportDistPct[i], want)
			}
		}
	}
}

// ATR percentile must be a 0-100 rank of the current ATR% in its own history.
func TestATRPercentile(t *testing.T) {
	now := time.Now()
	dur := marketTFDuration("1h")
	var bars []market.KlineBar
	p := 5.0
	for i := 0; i < 60; i++ {
		range_ := 0.01
		if i >= 50 {
			range_ = 0.08 // volatile tail
		}
		bars = append(bars, market.KlineBar{
			Time: now.Add(time.Duration(i-60) * dur).UnixMilli(),
			Open: p, High: p + range_, Low: p - range_, Close: p, Volume: 1000,
		})
	}
	data := &market.Data{
		Symbol: "TUSDT", CurrentPrice: 5.0,
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"1h": {Timeframe: "1h", Klines: bars},
		},
	}
	sig, err := ComputeSymbolSignals("TUSDT", data, SignalOptions{Now: now, PrimaryTF: "1h"})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	pct := sig.Timeframes["1h"].ATRPercentile
	if pct == nil || *pct < 0 || *pct > 100 {
		t.Fatalf("atr percentile out of range: %v", pct)
	}
	if *pct < 50 {
		t.Fatalf("tail volatility should rank high, got %.1f", *pct)
	}
}

// Correlation/beta against BTC: a perfect linear clone must give 1.0 / 1.0.
func TestBTCCorrelationAndFreshness(t *testing.T) {
	now := time.Now()
	dur := marketTFDuration("1h")
	btc := []float64{50000.0}
	for i := 1; i < 60; i++ {
		factor := 1.01
		if i%2 == 0 {
			factor = 0.995
		}
		btc = append(btc, btc[i-1]*factor)
	}
	var bars []market.KlineBar
	sym := 5.0
	for i := 0; i < 59; i++ {
		sym = sym * (btc[i+1] / btc[i])
		c := sym
		bars = append(bars, market.KlineBar{
			Time: now.Add(time.Duration(i-60) * dur).UnixMilli(),
			Open: c * 0.999, High: c * 1.001, Low: c * 0.999, Close: c, Volume: 1000,
		})
	}
	data := &market.Data{
		Symbol: "ALTPERP", CurrentPrice: sym,
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"1h": {Timeframe: "1h", Klines: bars},
		},
	}
	sig, err := ComputeSymbolSignals("ALTPERP", data, SignalOptions{
		Now: now, PrimaryTF: "1h",
		BtcCloses:          btc,
		QuoteVolume24hUsd:  42_000_000,
		VendorStalenessPct: func() *float64 { v := 6.5; return &v }(),
		TraderHistory:      &TraderHistoryStat{ClosedTrades: 3, Wins: 1, WinRatePct: 33.3, RealizedPnL: -2.15},
	})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	btcSig := sig.BTC
	if btcSig == nil {
		t.Fatal("btc correlation missing")
	}
	if math.Abs(btcSig.Pearson-1) > 1e-6 || math.Abs(btcSig.Beta-1) > 1e-6 {
		t.Fatalf("pearson=%.4f beta=%.4f, want 1.0/1.0", btcSig.Pearson, btcSig.Beta)
	}
	fresh := sig.DataFreshness
	if fresh == nil || fresh.SettlementAgeSeconds < 3590 || fresh.SettlementAgeSeconds > 3610 {
		t.Fatalf("settlement age wrong: %v", fresh)
	}
	if fresh.VendorDivergencePct == nil || *fresh.VendorDivergencePct != 6.5 {
		t.Fatalf("vendor divergence missing: %v", fresh.VendorDivergencePct)
	}
	if sig.Liquidity == nil || sig.Liquidity.QuoteVolume24hUsd == nil || *sig.Liquidity.QuoteVolume24hUsd != 42_000_000 {
		t.Fatal("quote volume missing")
	}
	if sig.TraderHistory == nil || sig.TraderHistory.ClosedTrades != 3 {
		t.Fatal("trader history missing")
	}
	if sig.Derivatives.LongShortAccountRatio != nil {
		t.Fatal("long/short not provided → must stay nil")
	}
}

// data_complete covers core OHLC only; missing derivatives (tokenized-stock
// perps etc.) must be flagged as warnings — not silently read as "no crowding".
func TestDerivativesCoverageWarnings(t *testing.T) {
	now := time.Now()
	dur := marketTFDuration("1h")
	var bars []market.KlineBar
	for i := 0; i < 30; i++ {
		c := 5.0 + float64(i)*0.001
		bars = append(bars, market.KlineBar{
			Time: now.Add(time.Duration(i-30) * dur).UnixMilli(),
			Open: c - 0.0005, High: c + 0.001, Low: c - 0.001, Close: c, Volume: 1000,
		})
	}
	data := &market.Data{
		Symbol: "SNDKUSDT", CurrentPrice: 5.03, FundingRate: 0, // no funding data
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"1h": {Timeframe: "1h", Klines: bars},
		},
	}
	sig, err := ComputeSymbolSignals("SNDKUSDT", data, SignalOptions{Now: now, PrimaryTF: "1h"})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if !sig.DataComplete {
		t.Fatal("core OHLC is complete → data_complete must stay true")
	}
	joined := strings.Join(sig.Warnings, "\n")
	if !strings.Contains(joined, "funding_rate fetch FAILED") {
		t.Fatalf("missing funding_rate should be flagged in warnings, got: %v", sig.Warnings)
	}
	if !strings.Contains(joined, "long/short") {
		t.Fatalf("missing long/short should be flagged in warnings, got: %v", sig.Warnings)
	}
}

// Regression: price_change_1h_pct must be a self-computed rolling ~60min
// change from the symbol's own klines — it previously collapsed into a copy
// of vendor_vs_live_divergence_pct because both anchored on the vendor's
// frozen forming-candle close. Now the anchors differ and the values diverge.
func TestPriceChange1hNotVendorDivergence(t *testing.T) {
	now := time.Now()
	dur := marketTFDuration("15m")
	// 15m closes stepping DOWN so the vendor-frozen anchor (previous close)
	// and the rolling-60m anchor (4 bars ago) genuinely differ.
	var bars []market.KlineBar
	p := 1.20
	for i := 0; i < 30; i++ {
		p -= 0.005 // -0.42% per bar
		bars = append(bars, market.KlineBar{
			Time: now.Add(time.Duration(i-30) * dur).UnixMilli(),
			Open: p + 0.004, High: p + 0.001, Low: p - 0.001, Close: p, Volume: 1000,
		})
	}
	// The forming candle close is frozen at the previous candle's close
	// (vendor behavior) — 60 minutes of movement have happened since.
	data := &market.Data{
		Symbol: "ZECUSDT", CurrentPrice: p * 0.99,
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"15m": {Timeframe: "15m", Klines: bars},
		},
	}
	opt := SignalOptions{Now: now, PrimaryTF: "15m", CurrentPrice: data.CurrentPrice}
	opt.VendorStalenessPct = func() *float64 { v := -0.10805076421357795; return &v }()

	sig, err := ComputeSymbolSignals("ZECUSDT", data, opt)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	pc := *sig.Derivatives.PriceChange1hLivePct
	if pc == *sig.DataFreshness.VendorDivergencePct {
		t.Fatalf("price_change_1h_pct (%.6f) still equals vendor divergence", pc)
	}
	// Rolling anchor: the close 4 bars ago (60-75 min back), vs live price.
	wantAnchor := 1.20 - 0.005*26 // 26 closed bars before the last 4
	want := (p*0.99/wantAnchor - 1) * 100
	if math.Abs(pc-want) > 0.01 {
		t.Fatalf("price_change_1h_pct = %.4f, want %.4f (live vs close 60m ago)", pc, want)
	}
}

// XYZ-prefixed symbols (Hyperliquid tokenized stock/commodity perps — SKHX,
// XAU, CL, ...) are dropped from every coin source at the exclusion choke
// point: their derivatives metrics don't map to the crypto context.
func TestXYZSymbolsExcluded(t *testing.T) {
	cfg := &store.StrategyConfig{}
	e := NewStrategyEngine(cfg)
	in := []CandidateCoin{
		{Symbol: "BTCUSDT", Sources: []string{"ai500"}},
		{Symbol: "XYZ:SKHXUSDT", Sources: []string{"ai500"}},
		{Symbol: "xyz:XAUUSDT", Sources: []string{"short_scan"}},
		{Symbol: "ETHUSDT", Sources: []string{"short_scan"}},
	}
	out := e.filterExcludedCoins(in)
	if len(out) != 2 {
		t.Fatalf("want 2 candidates after XYZ filter, got %d: %v", len(out), out)
	}
	for _, c := range out {
		if strings.Contains(strings.ToUpper(c.Symbol), ":") {
			t.Fatalf("XYZ symbol leaked: %s", c.Symbol)
		}
	}
}

// The USD OI delta must share its sign with the percentage delta: it is the
// base-unit change valued at the latest series price, not the raw notional
// difference (which flips sign when price moves hard inside the window).
func TestOIDeltasSignConsistency(t *testing.T) {
	// OI -10.47% while price +12%: the old notional diff was positive.
	curBase, prevBase := 8.953, 10.0
	curValue := curBase * 1.12 // implied mark rose 12%
	prevValue := prevBase * 1.0
	db, dv, dp := oiDeltas(curBase, prevBase, curValue, prevValue)
	if dp >= 0 {
		t.Fatalf("pct should be negative, got %.2f", dp)
	}
	if dv > 0 {
		t.Fatalf("USD delta must share the pct sign, got %+.0f", dv)
	}
	if math.Abs(dv-dp*0.01*prevBase*(curValue/curBase)) > 1e-6 {
		t.Fatalf("usd delta inconsistent with pct: %v vs %v%%", dv, dp)
	}

	// Plain case: everything negative.
	db, dv, dp = oiDeltas(9, 10, 900, 1000)
	if db != -1 || dv != -100 || math.Abs(dp-(-10)) > 1e-9 {
		t.Fatalf("plain deltas wrong: %v %v %v", db, dv, dp)
	}
} // Scanner output must be neutral EVIDENCE, not a prescriptive conclusion:
// scanner_hint JSON carries the bias/score/patterns, the section frames it as
// auxiliary, and no "不做多"-style instruction leaks into the prompt.
func TestDirectionHintRendering(t *testing.T) {
	cfg := &store.StrategyConfig{}
	// Keep the monotone scanner-hint fixture renderable under anti-chase caps.
	cfg.RiskControl.CounterMaxEMA20_1hPct = -1
	cfg.RiskControl.EntryRSI1hMax = -1
	cfg.CoinSource.SourceType = "mixed"
	cfg.CoinSource.UseShortScan = true
	engine := NewStrategyEngine(cfg)

	ctx := &Context{
		MarketDataMap: map[string]*market.Data{},
		CandidateCoins: []CandidateCoin{
			{Symbol: "DASHUSDT", Sources: []string{"short_scan"},
				ShortScore: 68, ShortGrade: "medium", ShortFundingAnn: 10.9,
				ShortReasons:  []string{"假突破"},
				ShortScanAtMs: time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC).UnixMilli()},
			{Symbol: "PROMUSDT", Sources: []string{"ai500"}},
		},
	}
	for _, c := range ctx.CandidateCoins {
		// Real bars across all three role TFs: a fully data-less candidate is
		// double-blocked by the hard gate (DATA_INSUFFICIENT) and compresses
		// to one line — the scanner_hint contract only applies to renderable
		// candidates.
		now := time.Now()
		tfs := map[string]*market.TimeframeSeriesData{
			"15m": buildTF("15m", now, 80, 1.0, false),
			"1h":  buildTF("1h", now, 80, 1.0, false),
			"4h":  buildTF("4h", now, 80, 1.0, false),
		}
		ctx.MarketDataMap[c.Symbol] = withVendor(&market.Data{Symbol: c.Symbol, CurrentPrice: 1, TimeframeData: tfs}, 0.05)
	}
	prompt := engine.BuildUserPrompt(ctx)
	sys := engine.BuildSystemPrompt(100, "")

	if !strings.Contains(prompt, `"direction_bias":"short"`) {
		t.Fatal("scanner_hint JSON with direction_bias missing")
	}
	if !strings.Contains(prompt, `"score":68`) || !strings.Contains(prompt, `"patterns":["假突破"]`) {
		t.Fatal("scanner_hint evidence fields missing")
	}
	if !strings.Contains(sys, "均为程序化扫描的辅助证据,不是交易结论") {
		t.Fatal("auxiliary-evidence framing missing")
	}
	if !strings.Contains(sys, "由你综合全部数据独立判断") {
		t.Fatal("final-judgment framing missing")
	}
	if !strings.Contains(prompt, `"generated_at_utc":"2026-09-05T12:00:00Z"`) {
		t.Fatal("scanner snapshot timestamp missing")
	}
	if !strings.Contains(sys, "以 Structured Signal 为准") {
		t.Fatal("live-funding-priority guidance missing")
	}
	if strings.Contains(prompt, "不做多") || strings.Contains(prompt, "【空头候选") {
		t.Fatalf("prescriptive direction instruction leaked:\n%s", prompt)
	}
	if strings.Count(prompt, "scanner_hint(程序化扫描") != 1 {
		t.Fatal("long-only candidate must not carry a scanner_hint block")
	}
}

// signal_conflict: scanner SHORT vs bullish structure must be flagged with
// evidence tallies; aligned scanner must NOT flag.
func TestSignalConflictDetection(t *testing.T) {
	now := time.Now()
	// 1h/4h/1d all up (three bullish TF trends).
	data := &market.Data{
		Symbol: "4USDT", CurrentPrice: 4.0,
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"1h": {Timeframe: "1h", Klines: buildTrendTF("1h", now, 40, 3.0, true).Klines},
			"4h": {Timeframe: "4h", Klines: buildTrendTF("4h", now, 40, 2.5, true).Klines},
			"1d": {Timeframe: "1d", Klines: buildTrendTF("1d", now, 40, 2.0, true).Klines},
		},
	}
	sig, err := ComputeSymbolSignals("4USDT", data, SignalOptions{Now: now, PrimaryTF: "1h", ScannerBias: "short"})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	sc := sig.SignalConflict
	if sc == nil || !sc.DirectionalConflict {
		t.Fatalf("scanner short vs 3x up structure must conflict: %+v", sc)
	}
	if sc.BullishEvidence < 3 || sc.BearishEvidence < 1 {
		t.Fatalf("evidence tally wrong: %+v", sc)
	}
	if !strings.Contains(sc.Note, "SHORT") {
		t.Fatalf("note should name the scanner bias: %q", sc.Note)
	}

	// No scanner → pure structure; with all-up there is no conflict.
	sig2, err := ComputeSymbolSignals("4USDT", data, SignalOptions{Now: now, PrimaryTF: "1h"})
	if err != nil {
		t.Fatalf("compute2: %v", err)
	}
	if sig2.SignalConflict == nil || sig2.SignalConflict.DirectionalConflict {
		t.Fatalf("aligned structure must not conflict: %+v", sig2.SignalConflict)
	}
	if sig2.SignalConflict.BullishEvidence < 3 {
		t.Fatalf("bullish tally wrong: %+v", sig2.SignalConflict)
	}
}

// flatTF builds a sideways series — trend classifies as "range".
func flatTF(tf string) *market.TimeframeSeriesData {
	d := &market.TimeframeSeriesData{Timeframe: tf}
	for i := 0; i < 60; i++ {
		d.Klines = append(d.Klines, market.KlineBar{Open: 100, High: 100.05, Low: 99.95, Close: 100, Volume: 1000})
	}
	return d
}

func TestSignalConflictScoreAndTypes(t *testing.T) {
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	// 1h up + 4h down → genuine TIMEFRAME_SPLIT.
	data := &market.Data{Symbol: "T1USDT", CurrentPrice: 100, TimeframeData: map[string]*market.TimeframeSeriesData{
		"15m": flatTF("15m"),
		"1h":  buildTrendTF("1h", now, 60, 100, true),
		"4h":  buildTrendTF("4h", now, 60, 100, false),
	}}

	sig, err := ComputeSymbolSignals("T1USDT", data, SignalOptions{Now: now, PrimaryTF: "15m"})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if sig.SignalConflict == nil || !sig.SignalConflict.DirectionalConflict {
		t.Fatal("1h/4h opposition must flag directional conflict")
	}
	hasSplit := false
	for _, ty := range sig.SignalConflict.Types {
		if ty == "TIMEFRAME_SPLIT" {
			hasSplit = true
		}
	}
	if !hasSplit {
		t.Fatalf("TIMEFRAME_SPLIT type missing: %v", sig.SignalConflict.Types)
	}
	// bull=2 (1h up, live+) bear=2 (4h down, scanner short) → score 0
	if sig.SignalConflict.DirectionalScore != 0 {
		t.Fatalf("directional_score = %d, want 0 for 2:2", sig.SignalConflict.DirectionalScore)
	}

	// ⑦ range-heavy split (no 1h/4h opposition) must NOT flag conflict.
	data2 := &market.Data{Symbol: "T2USDT", CurrentPrice: 100, TimeframeData: map[string]*market.TimeframeSeriesData{
		"15m": flatTF("15m"),
		"5m":  flatTF("5m"),
		"1h":  buildTrendTF("1h", now, 60, 100, true),
		"4h":  buildTrendTF("4h", now, 60, 100, true),
	}}
	sig2, err := ComputeSymbolSignals("T2USDT", data2, SignalOptions{Now: now, PrimaryTF: "15m"})
	if err != nil {
		t.Fatalf("compute2: %v", err)
	}
	if sig2.SignalConflict != nil && sig2.SignalConflict.DirectionalConflict {
		t.Fatalf("range-heavy split must not flag conflict: %+v", sig2.SignalConflict)
	}
}

func TestExecutionFilterAndRoles(t *testing.T) {
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	data := &market.Data{Symbol: "T3USDT", CurrentPrice: 100, TimeframeData: map[string]*market.TimeframeSeriesData{
		"15m": flatTF("15m"),
		"1h":  buildTrendTF("1h", now, 60, 100, true),
		"4h":  buildTrendTF("4h", now, 60, 100, true),
	}}

	sig, err := ComputeSymbolSignals("T3USDT", data, SignalOptions{Now: now, PrimaryTF: "15m"})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if sig.ExecutionFilter == nil {
		t.Fatal("execution filter missing")
	}
	if sig.ExecutionFilter.MicroTF != "15m" || sig.ExecutionFilter.MicroTrend != "range" {
		t.Fatalf("micro tf/trend wrong: %+v", sig.ExecutionFilter)
	}
	if sig.ExecutionFilter.LongAllowed || sig.ExecutionFilter.ShortAllowed {
		t.Fatal("range must block both directions")
	}
	if sig.RoleTFs.ExecutionTF != "15m" || sig.RoleTFs.TrendTF != "1h" || sig.RoleTFs.RegimeTF != "4h" {
		t.Fatalf("role TFs wrong: %+v", sig.RoleTFs)
	}
}

func TestFundingAnnualizedPrecomputed(t *testing.T) {
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	rate := 0.0001
	data := &market.Data{
		Symbol: "T4USDT", CurrentPrice: 100, FundingRate: rate, FundingRateOK: true,
		TimeframeData: map[string]*market.TimeframeSeriesData{},
	}
	t15 := buildTF("15m", now, 60, 100, false)
	data.TimeframeData["15m"] = t15
	sig, err := ComputeSymbolSignals("T4USDT", data, SignalOptions{Now: now, PrimaryTF: "15m"})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if sig.Derivatives == nil || sig.Derivatives.FundingAnnualizedPct == nil {
		t.Fatal("funding_annualized_pct missing")
	}
	want := rate * 3 * 365 * 100 // 10.95
	if *sig.Derivatives.FundingAnnualizedPct < want-0.01 || *sig.Derivatives.FundingAnnualizedPct > want+0.01 {
		t.Fatalf("annualized = %.2f, want %.2f", *sig.Derivatives.FundingAnnualizedPct, want)
	}
}

// TestFundingAnnualizedRealInterval guards the settlement-interval contract:
// a 4h-settlement listing (e.g. VVVUSDT) must annualize with ×6/day, not the
// 8h ×3 fallback that made the structured signal half the scanner's figure.
func TestFundingAnnualizedRealInterval(t *testing.T) {
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	rate := 0.0001
	data := &market.Data{
		Symbol: "VVVUSDT", CurrentPrice: 100, FundingRate: rate, FundingRateOK: true,
		FundingSettleHours: 4, // measured from settlement timestamps
		TimeframeData:      map[string]*market.TimeframeSeriesData{},
	}
	t15 := buildTF("15m", now, 60, 100, false)
	data.TimeframeData["15m"] = t15
	sig, err := ComputeSymbolSignals("VVVUSDT", data, SignalOptions{Now: now, PrimaryTF: "15m"})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if sig.Derivatives == nil || sig.Derivatives.FundingAnnualizedPct == nil {
		t.Fatal("funding_annualized_pct missing")
	}
	want := rate * 6 * 365 * 100 // 21.9
	if *sig.Derivatives.FundingAnnualizedPct < want-0.01 || *sig.Derivatives.FundingAnnualizedPct > want+0.01 {
		t.Fatalf("annualized = %.2f, want %.2f", *sig.Derivatives.FundingAnnualizedPct, want)
	}
	if sig.Derivatives.FundingSettleHours == nil || *sig.Derivatives.FundingSettleHours != 4 {
		t.Fatalf("funding_settle_hours missing or wrong: %v", sig.Derivatives.FundingSettleHours)
	}
}

func TestBreakoutStateBelowAndConfirmed(t *testing.T) {
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	mk := func(lastClose float64) *market.Data {
		data := &market.Data{Symbol: "T5USDT", CurrentPrice: lastClose, TimeframeData: map[string]*market.TimeframeSeriesData{}}
		t15 := buildTF("15m", now, 60, lastClose, false)
		// 1h: uptrend closing just below the structure high (vol normal).
		k1h := make([]market.KlineBar, 0, 60)
		p := lastClose * 0.9
		for i := 0; i < 59; i++ {
			p *= 1.002
			k1h = append(k1h, market.KlineBar{Open: p * 0.999, High: p * 1.001, Low: p * 0.998, Close: p, Volume: 100})
		}
		// final closed bar pushes beyond the prior swing high with volume
		high := lastClose * 1.02
		k1h = append(k1h, market.KlineBar{Open: p, High: high * 1.005, Low: p * 0.999, Close: lastClose, Volume: 500})
		tf1h := &market.TimeframeSeriesData{Klines: k1h}
		data.TimeframeData["15m"] = t15
		data.TimeframeData["1h"] = tf1h
		return data
	}
	sig, err := ComputeSymbolSignals("T5USDT", mk(103.0), SignalOptions{Now: now, PrimaryTF: "15m"})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if sig.Breakout == nil {
		t.Fatal("breakout state missing")
	}
	if sig.Breakout.Direction != "breakout" || sig.Breakout.Level <= 0 {
		t.Fatalf("direction/level wrong: %+v", sig.Breakout)
	}
	// confirmed requires volume+OI: OI absent here → at most broken_unconfirmed
	if sig.Breakout.Status == "confirmed" {
		t.Fatal("without OI data status must not be confirmed")
	}
	switch sig.Breakout.Status {
	// review 2026-10-07 B2-3: a cross inside the last 8 bars is now detected
	// (close transition), so this steadily rising series legitimately yields
	// retest_hold/extended (its lows hug the level) — still never confirmed.
	case "broken_unconfirmed", "approach", "below", "fake_break", "retest_hold", "extended":
	default:
		t.Fatalf("unexpected status %s", sig.Breakout.Status)
	}
}

// ① Anchor cross-validation: a buy limit parked within the supply-zone
// threshold of overhead resistance must be zeroed (BULLA prototype —
// anchor 0.46% under 15m resistance got rejected at execution anyway).
func TestAnchorSuppressionAgainstStructure(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	// Build a 1h series whose resistance sits 0.46% above price×0.995.
	// Flat series so the pivot placement is deterministic (buildTF drifts
	// +0.1%/bar, which would put the trend tail far above the anchor).
	// The raw-pivot scan runs on 1h only (2026-09-10): 15m minors inside the
	// pullback corridor are noise at the 1h-scale threshold.
	base := 0.0800
	anchor := base * 0.995        // 0.0796
	resistance := anchor * 1.0046 // 0.46% above the anchor — inside the zone
	data := &market.Data{Symbol: "BULLAUSDT", CurrentPrice: base, TimeframeData: map[string]*market.TimeframeSeriesData{}}
	t1h := &market.TimeframeSeriesData{Timeframe: "1h"}
	for i := 0; i < 60; i++ {
		t1h.Klines = append(t1h.Klines, market.KlineBar{Open: base, High: base * 1.0005, Low: base * 0.9995, Close: base, Volume: 1000})
	}
	// A swing high at `resistance`: bars 28-32 strictly below it (2-bar
	// shoulders) so swingPivots recognizes the pivot.
	for i := 28; i <= 32; i++ {
		t1h.Klines[i].High = resistance * 0.995
		t1h.Klines[i].Close = resistance * 0.99
	}
	t1h.Klines[30].High = resistance
	t1h.Klines[30].Close = resistance * 0.999
	data.TimeframeData["1h"] = t1h

	sig, err := ComputeSymbolSignals("BULLAUSDT", data, SignalOptions{
		Now: now, PrimaryTF: "15m",
		LimitEntryOffsetPct:  0.5,
		LimitEntryOffsetMode: "fixed",
		SupplyZonePct:        0.5,
	})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if sig.LimitBuyPrice != 0 {
		t.Fatalf("buy anchor %.6f should be suppressed (resistance %.6f within 0.5%%)",
			sig.LimitBuyPrice, resistance)
	}
	suppressed := false
	for _, w := range sig.Warnings {
		if strings.Contains(w, "limit_buy_price suppressed") {
			suppressed = true
		}
	}
	if !suppressed {
		t.Fatal("suppression warning missing")
	}
}

// A swing high that lives ONLY on the 15m series must not zero the anchor:
// under the 1h-scale breathing threshold the pullback corridor (0.3-1.2%)
// always contains 15m minors, and scanning them zeroed 63% of anchors and
// blocked every open for a whole day (2026-09-10 regression).
func TestAnchorSuppressionIgnores15mPivots(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	base := 0.0800
	anchor := base * 0.995
	minor := anchor * 1.0046 // 0.46% above the anchor — inside the zone
	data := &market.Data{Symbol: "BULLAUSDT", CurrentPrice: base, TimeframeData: map[string]*market.TimeframeSeriesData{}}
	t15 := &market.TimeframeSeriesData{Timeframe: "15m"}
	for i := 0; i < 60; i++ {
		t15.Klines = append(t15.Klines, market.KlineBar{Open: base, High: base * 1.0005, Low: base * 0.9995, Close: base, Volume: 1000})
	}
	for i := 28; i <= 32; i++ {
		t15.Klines[i].High = minor * 0.995
		t15.Klines[i].Close = minor * 0.99
	}
	t15.Klines[30].High = minor
	t15.Klines[30].Close = minor * 0.999
	data.TimeframeData["15m"] = t15

	sig, err := ComputeSymbolSignals("BULLAUSDT", data, SignalOptions{
		Now: now, PrimaryTF: "15m",
		LimitEntryOffsetPct:  0.5,
		LimitEntryOffsetMode: "fixed",
		SupplyZonePct:        0.5,
	})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if sig.LimitBuyPrice == 0 {
		t.Fatal("buy anchor must survive a 15m-only minor pivot")
	}
}

// With the gate disabled (SupplyZonePct=0) anchors pass through untouched.
func TestAnchorSuppressionDisabled(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	base := 0.0800
	data := &market.Data{Symbol: "BULLAUSDT", CurrentPrice: base, TimeframeData: map[string]*market.TimeframeSeriesData{
		"15m": buildTF("15m", now, 60, base, false),
	}}
	sig, err := ComputeSymbolSignals("BULLAUSDT", data, SignalOptions{
		Now: now, PrimaryTF: "15m",
		LimitEntryOffsetPct:  0.5,
		LimitEntryOffsetMode: "fixed",
	})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if want := base * 0.995; sig.LimitBuyPrice != want {
		t.Fatalf("gate disabled: anchor = %.6f, want %.6f", sig.LimitBuyPrice, want)
	}
}

// ATR-scaled anchor offsets (review 2026-09-07): offset = 0.5×ATR(execution
// TF) clamped to [0.15%, 1.2%], falling back to the fixed percent when ATR
// is unavailable.
func TestAnchorOffsetVolatilityScaled(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	price := 1.0

	// Flat closes with symmetric High/Low wicks → TR = High-Low = 2×wick on
	// every bar, so Wilder ATR ≈ 2×wickPct% of price — a directly controlled
	// ATR(14).
	buildRangeTF := func(bars int, wickPct float64) *market.TimeframeSeriesData {
		dur := marketTFDuration("15m")
		tfData := &market.TimeframeSeriesData{Timeframe: "15m"}
		for i := 0; i < bars; i++ {
			barTime := now.Add(time.Duration(i-(bars-1)) * dur)
			tfData.Klines = append(tfData.Klines, market.KlineBar{
				Time: barTime.UnixMilli(), Open: price,
				High: price * (1 + wickPct/100), Low: price * (1 - wickPct/100),
				Close: price, Volume: 1000,
			})
		}
		return tfData
	}

	compute := func(t *testing.T, tfData *market.TimeframeSeriesData, mode string) *SymbolSignal {
		t.Helper()
		data := &market.Data{Symbol: "XUSDT", CurrentPrice: price, TimeframeData: map[string]*market.TimeframeSeriesData{}}
		if tfData != nil {
			data.TimeframeData["15m"] = tfData
		}
		sig, err := ComputeSymbolSignals("XUSDT", data, SignalOptions{
			Now: now, PrimaryTF: "15m",
			LimitEntryOffsetPct:  0.3,
			LimitEntryOffsetMode: mode,
		})
		if err != nil {
			t.Fatalf("compute: %v", err)
		}
		return sig
	}

	// Quiet book (SKHYNIX-like, ATR 0.27%): 0.5×0.27=0.135% → floor 0.15%.
	low := compute(t, buildRangeTF(60, 0.135), "")
	if math.Abs(low.LimitEntryOffsetPct-0.15) > 1e-9 {
		t.Errorf("low-ATR offset = %.4f%%, want floor 0.15%%", low.LimitEntryOffsetPct)
	}
	if want := price * (1 - 0.0015); math.Abs(low.LimitBuyPrice-want) > 1e-9 {
		t.Errorf("low-ATR limit_buy = %.6f, want %.6f", low.LimitBuyPrice, want)
	}

	// Violent mover (ATR 3%): 0.5×3=1.5% → ceiling 1.2%.
	high := compute(t, buildRangeTF(60, 1.5), "")
	if math.Abs(high.LimitEntryOffsetPct-1.2) > 1e-9 {
		t.Errorf("high-ATR offset = %.4f%%, want ceiling 1.2%%", high.LimitEntryOffsetPct)
	}

	// Mid (ATR 0.8%): 0.5×0.8=0.4%, no clamp.
	mid := compute(t, buildRangeTF(60, 0.4), "")
	if math.Abs(mid.LimitEntryOffsetPct-0.4) > 1e-9 {
		t.Errorf("mid-ATR offset = %.4f%%, want 0.4%%", mid.LimitEntryOffsetPct)
	}

	// Fixed mode ignores ATR entirely.
	fixed := compute(t, buildRangeTF(60, 1.5), "fixed")
	if math.Abs(fixed.LimitEntryOffsetPct-0.3) > 1e-9 {
		t.Errorf("fixed-mode offset = %.4f%%, want configured 0.3%%", fixed.LimitEntryOffsetPct)
	}

	// ATR unavailable (<10 closed bars → TFSignal short-circuits) → fixed fallback.
	fallback := compute(t, buildRangeTF(5, 1.5), "")
	if math.Abs(fallback.LimitEntryOffsetPct-0.3) > 1e-9 {
		t.Errorf("ATR-less fallback offset = %.4f%%, want fixed 0.3%%", fallback.LimitEntryOffsetPct)
	}
}

// Audit regression 2026-09-11: stochRSI must not read the unset zero head of
// the RSI series; a perfectly flat series yields RSI 50 (not 100); the trend
// classifier's pullback branch must be reachable.
func TestAuditFixes(t *testing.T) {
	// Flat closes: RSI series all 50; stochRSI ok with %K=50-ish (hi==lo → 50).
	flat := make([]float64, 60)
	for i := range flat {
		flat[i] = 100
	}
	rsi := wilderRSISeries(flat, 14)
	if got := rsi[len(rsi)-1]; got != 50 {
		t.Fatalf("flat series RSI = %v, want 50", got)
	}
	k, d, ok := stochRSI(flat, 14, 14, 3, 3)
	if !ok || k != 50 || d != 50 {
		t.Fatalf("flat stochRSI = %v/%v ok=%v, want 50/50", k, d, ok)
	}

	// Thin history (25 bars): stochRSI must refuse rather than read zeros.
	thin := make([]float64, 25)
	for i := range thin {
		thin[i] = 100 + float64(i%3)
	}
	if _, _, ok := stochRSI(thin, 14, 14, 3, 3); ok {
		t.Fatal("25 bars cannot feed rsi14+stoch14+smooth — must be ok=false")
	}

	// classifyTrend: uptrend structure with price under the fast EMA is a
	// PULLBACK (branch was dead — slope-sign bug fed "range" to the gates).
	c := make([]float64, 60)
	for i := range c {
		c[i] = 100 + float64(i)*0.5 // steady climb
	}
	c[len(c)-1] = c[len(c)-2] - 1               // dip under the fast EMA
	got := classifyTrend(c, 128.0, 120.0, true) // fast>slow, last<fast
	if got != "pullback" {
		t.Fatalf("uptrend dip classified %q, want pullback", got)
	}
}

// Rally label (audit 09-13): a downtrend bounce must classify as rally (the
// short-side entry window), and the execution filter must allow shorts —
// never longs — in that state.
func TestRallyClassificationAndFilter(t *testing.T) {
	// Steady decline, last bar bounces back ABOVE the fast EMA (but still
	// below the slow EMA) — the "sell the bounce" window of a downtrend.
	c := make([]float64, 60)
	for i := range c {
		c[i] = 200 - float64(i)*0.5 // steady fall → last ≈ 170.5
	}
	c[len(c)-1] = 181                           // bounce above fast(180), still under slow(185)
	got := classifyTrend(c, 180.0, 185.0, true) // fast<slow, last>=fast
	if got != "rally" {
		t.Fatalf("downtrend bounce classified %q, want rally", got)
	}
	// The same series with the bounce absent stays "down" (below both EMAs).
	falling := make([]float64, 60)
	for i := range falling {
		falling[i] = 200 - float64(i)*0.5
	}
	if got := classifyTrend(falling, 180.0, 185.0, true); got != "down" {
		t.Fatalf("pre-bounce decline classified %q, want down", got)
	}

}

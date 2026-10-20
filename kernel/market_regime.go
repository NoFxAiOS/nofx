package kernel

import (
	"math"
	"time"

	"nofx/market"
)

const (
	regimeADXTrend1h       = 22.0
	regimeADXTrend4h       = 18.0
	regimeSlopeLookback    = 3
	regimeATRHighPct       = 80.0
	regimeATRLowPct        = 25.0
	regimeBBHighPct        = 80.0
	regimeBBLowPct         = 25.0
	regimeBBExpansionPct   = 15.0
	regimeBBContractionPct = -15.0
)

// withRequiredRegimeTimeframes keeps user-selected execution frames and adds
// the two closed-bar frames required by the programmatic regime. It returns a
// fresh de-duplicated slice so strategy configuration is never mutated.
func withRequiredRegimeTimeframes(timeframes []string) []string {
	out := make([]string, 0, len(timeframes)+2)
	seen := make(map[string]bool, len(timeframes)+2)
	for _, tf := range append(append([]string{}, timeframes...), "1h", "4h") {
		if tf == "" || seen[tf] {
			continue
		}
		seen[tf] = true
		out = append(out, tf)
	}
	return out
}

// withRequiredSymbolTimeframes adds symbol-specific risk frames after the
// common regime frames. Bstock stops and targets use the 4h-1d structure
// horizon with ATR(1d), so their prompt path must always fetch 1d instead of
// silently falling back to the crypto 1h/4h scale. Intraday frames remain in
// place for entry timing, including weekday pre/after-market trading.
// WithRequiredSymbolTimeframes is the exported form — the trader's pending
// recheck must expand its timeframe list exactly like the AI cycle does
// (incident 2026-10-07: divergent expansion fed DataQuality a different bar
// map than the decision was approved with).
func WithRequiredSymbolTimeframes(timeframes []string, symbol string) []string {
	return withRequiredSymbolTimeframes(timeframes, symbol)
}

func withRequiredSymbolTimeframes(timeframes []string, symbol string) []string {
	return withRequiredSymbolTimeframesFor(timeframes, symbol, false)
}

// WithRequiredSymbolTimeframesFor is the exported form of
// withRequiredSymbolTimeframesFor — the trader's pending recheck passes
// RiskControl.BlockShort1dUptrend so it fetches the same 1d frame the AI
// cycle does.
func WithRequiredSymbolTimeframesFor(timeframes []string, symbol string, require1d bool) []string {
	return withRequiredSymbolTimeframesFor(timeframes, symbol, require1d)
}

// withRequiredSymbolTimeframesFor is withRequiredSymbolTimeframes plus an
// explicit require1d switch (review 2026-10-20 area4 #7): the "no shorts
// against a 1d uptrend" gate reads the 1d trend, which was only ever fetched
// for bstocks — for ordinary crypto the frame was missing, TimeframeTrend
// returned "" and the gate silently never fired. require1d appends 1d for
// every symbol; it is a fetch requirement only (DataQuality minimum-bar
// accounting is unchanged for non-bstocks, so a short daily history never
// blocks longs).
func withRequiredSymbolTimeframesFor(timeframes []string, symbol string, require1d bool) []string {
	out := withRequiredRegimeTimeframes(timeframes)
	if !require1d && !market.IsBStockSymbol(symbol) {
		return out
	}
	for _, tf := range out {
		if tf == "1d" {
			return out
		}
	}
	return append(out, "1d")
}

// MarketRegime is a deterministic, auditable trend x volatility state. The
// enum is intentionally small because it feeds execution gates; Inputs retain
// the measurements so the verdict never becomes an opaque replacement for
// price structure.
type MarketRegime struct {
	TrendState      string             `json:"trend_state"`      // UP | DOWN | RANGE | UNKNOWN
	TrendStrength   float64            `json:"trend_strength"`   // 0..100, derived from the weaker 1h/4h ADX
	VolatilityState string             `json:"volatility_state"` // LOW | NORMAL | HIGH | EXPANDING | CONTRACTING | UNKNOWN
	Regime          string             `json:"regime"`           // TREND_UP | TREND_DOWN | RANGE_LOW_VOL | RANGE_NORMAL | CHOP_HIGH_VOL | UNKNOWN
	Confidence      float64            `json:"confidence"`       // 0..100 deterministic classification confidence
	ConfirmedBars   int                `json:"confirmed_bars"`   // consecutive closed 1h evaluations with the same regime, capped at 2
	AsOfUTC         string             `json:"as_of_utc,omitempty"`
	BarsClosed      bool               `json:"bars_closed"`
	Inputs          MarketRegimeInputs `json:"inputs"`
}

type MarketRegimeInputs struct {
	EMAAlignment1h      string   `json:"ema_alignment_1h"`
	EMAAlignment4h      string   `json:"ema_alignment_4h"`
	EMASlope1hPct       *float64 `json:"ema_slope_1h_pct,omitempty"`
	EMASlope4hPct       *float64 `json:"ema_slope_4h_pct,omitempty"`
	ADX1h               *float64 `json:"adx_1h,omitempty"`
	ADX4h               *float64 `json:"adx_4h,omitempty"`
	ATRPercentile1h     *float64 `json:"atr_percentile_1h,omitempty"`
	BBWidthPct1h        *float64 `json:"bb_width_pct_1h,omitempty"`
	BBWidthPercentile1h *float64 `json:"bb_width_percentile_1h,omitempty"`
	BBWidthChangePct1h  *float64 `json:"bb_width_change_pct_1h,omitempty"`
}

type regimeMeasurement struct {
	state *MarketRegime
	valid bool
}

// computeMarketRegime classifies the current closed-bar state and confirms it
// against the immediately preceding closed 1h evaluation. The previous
// evaluation uses a real timestamp cutoff, so a 4h candle that had not closed
// at that time cannot leak into confirmation.
func computeMarketRegime(data *market.Data, now time.Time) *MarketRegime {
	current := measureMarketRegime(data, now)
	if !current.valid {
		return current.state
	}
	current.state.ConfirmedBars = 1

	tf1h := data.TimeframeData["1h"]
	closed1h := closedKlinesAt(tf1h, now, time.Hour)
	if len(closed1h) < 2 {
		return current.state
	}
	currentClose := time.UnixMilli(closed1h[len(closed1h)-1].Time).Add(time.Hour)
	previousCutoff := currentClose.Add(-time.Nanosecond)
	previous := measureMarketRegime(data, previousCutoff)
	if previous.valid && previous.state.Regime == current.state.Regime {
		current.state.ConfirmedBars = 2
	}
	return current.state
}

func measureMarketRegime(data *market.Data, cutoff time.Time) regimeMeasurement {
	r := &MarketRegime{
		TrendState: "UNKNOWN", VolatilityState: "UNKNOWN", Regime: "UNKNOWN",
		Inputs: MarketRegimeInputs{EMAAlignment1h: "UNKNOWN", EMAAlignment4h: "UNKNOWN"},
	}
	if data == nil || data.TimeframeData == nil {
		return regimeMeasurement{state: r}
	}
	b1 := closedKlinesAt(data.TimeframeData["1h"], cutoff, time.Hour)
	b4 := closedKlinesAt(data.TimeframeData["4h"], cutoff, 4*time.Hour)
	if len(b1) < 50 || len(b4) < 50 {
		return regimeMeasurement{state: r}
	}

	k1, k4 := barsToKlines(b1), barsToKlines(b4)
	a1, s1, ok1 := emaRegimeEvidence(k1)
	a4, s4, ok4 := emaRegimeEvidence(k4)
	adx1, adx4 := wilderADX(k1, 14), wilderADX(k4, 14)
	atrPct, atrOK := atrPercentile(k1, 14)
	bbWidth, bbPct, bbChange, bbOK := bollingerWidthEvidence(k1, 20, regimeSlopeLookback)
	if !ok1 || !ok4 || math.IsNaN(adx1) || math.IsNaN(adx4) || !atrOK || !bbOK {
		return regimeMeasurement{state: r}
	}

	r.Inputs.EMAAlignment1h, r.Inputs.EMAAlignment4h = a1, a4
	r.Inputs.EMASlope1hPct, r.Inputs.EMASlope4hPct = floatPtr(s1), floatPtr(s4)
	r.Inputs.ADX1h, r.Inputs.ADX4h = floatPtr(round2(adx1)), floatPtr(round2(adx4))
	r.Inputs.ATRPercentile1h = floatPtr(round2(atrPct))
	r.Inputs.BBWidthPct1h = floatPtr(round2(bbWidth))
	r.Inputs.BBWidthPercentile1h = floatPtr(round2(bbPct))
	r.Inputs.BBWidthChangePct1h = floatPtr(round2(bbChange))
	r.BarsClosed = true
	lastClose := time.UnixMilli(b1[len(b1)-1].Time).Add(time.Hour)
	r.AsOfUTC = lastClose.UTC().Format(time.RFC3339)

	weakADX := math.Min(adx1, adx4)
	r.TrendStrength = round2(clamp(weakADX*2, 0, 100))
	up := a1 == "UP" && a4 == "UP" && s1 > 0 && s4 > 0 && adx1 >= regimeADXTrend1h && adx4 >= regimeADXTrend4h
	down := a1 == "DOWN" && a4 == "DOWN" && s1 < 0 && s4 < 0 && adx1 >= regimeADXTrend1h && adx4 >= regimeADXTrend4h
	switch {
	case up:
		r.TrendState, r.Regime = "UP", "TREND_UP"
	case down:
		r.TrendState, r.Regime = "DOWN", "TREND_DOWN"
	default:
		r.TrendState = "RANGE"
	}

	switch {
	case bbChange >= regimeBBExpansionPct && atrPct >= 60:
		r.VolatilityState = "EXPANDING"
	case bbChange <= regimeBBContractionPct:
		r.VolatilityState = "CONTRACTING"
	case atrPct >= regimeATRHighPct || bbPct >= regimeBBHighPct:
		r.VolatilityState = "HIGH"
	case atrPct <= regimeATRLowPct && bbPct <= regimeBBLowPct:
		r.VolatilityState = "LOW"
	default:
		r.VolatilityState = "NORMAL"
	}

	if r.TrendState == "RANGE" {
		switch r.VolatilityState {
		case "HIGH", "EXPANDING":
			r.Regime = "CHOP_HIGH_VOL"
		case "LOW", "CONTRACTING":
			r.Regime = "RANGE_LOW_VOL"
		default:
			r.Regime = "RANGE_NORMAL"
		}
	}
	if r.TrendState == "RANGE" {
		r.Confidence = round2(clamp(45+(regimeADXTrend1h-adx1)*1.5, 35, 90))
	} else {
		r.Confidence = round2(clamp(50+(weakADX-regimeADXTrend4h)*2, 50, 100))
	}
	return regimeMeasurement{state: r, valid: true}
}

func marketRegimeAllowsException(sig *SymbolSignal, isLong bool) bool {
	if sig == nil || sig.MarketRegime == nil || sig.MarketRegime.ConfirmedBars < 2 {
		return false
	}
	if isLong {
		return sig.MarketRegime.Regime == "TREND_UP"
	}
	return sig.MarketRegime.Regime == "TREND_DOWN"
}

func closedKlinesAt(tfData *market.TimeframeSeriesData, cutoff time.Time, dur time.Duration) []market.KlineBar {
	if tfData == nil {
		return nil
	}
	out := make([]market.KlineBar, 0, len(tfData.Klines))
	for _, b := range tfData.Klines {
		if !time.UnixMilli(b.Time).Add(dur).After(cutoff) {
			out = append(out, b)
		}
	}
	return out
}

func emaRegimeEvidence(bars []market.Kline) (string, float64, bool) {
	if len(bars) < 50+regimeSlopeLookback {
		return "UNKNOWN", 0, false
	}
	fast := market.ExportCalculateEMA(bars, 20)
	slow := market.ExportCalculateEMA(bars, 50)
	previousFast := market.ExportCalculateEMA(bars[:len(bars)-regimeSlopeLookback], 20)
	if fast <= 0 || slow <= 0 || previousFast <= 0 {
		return "UNKNOWN", 0, false
	}
	alignment := "DOWN"
	if fast > slow {
		alignment = "UP"
	} else if fast == slow {
		alignment = "FLAT"
	}
	slope := (fast/previousFast - 1) * 100
	return alignment, round2(slope), true
}

func atrPercentile(bars []market.Kline, period int) (float64, bool) {
	series := wilderATRSeries(bars, period)
	if len(series) < 6 {
		return 0, false
	}
	values := make([]float64, 0, len(series))
	for i, atr := range series {
		barIdx := i + period
		if barIdx < len(bars) && bars[barIdx].Close > 0 {
			values = append(values, atr/bars[barIdx].Close*100)
		}
	}
	if len(values) < 6 {
		return 0, false
	}
	current := values[len(values)-1]
	less, equal := 0, 0
	for _, v := range values {
		switch {
		case v < current:
			less++
		case math.Abs(v-current) <= 1e-12:
			equal++
		}
	}
	return (float64(less) + 0.5*float64(equal)) / float64(len(values)) * 100, true
}

func bollingerWidthEvidence(bars []market.Kline, period, changeLookback int) (width, percentile, change float64, ok bool) {
	if len(bars) < period+changeLookback+1 {
		return 0, 0, 0, false
	}
	widths := make([]float64, 0, len(bars)-period+1)
	for end := period; end <= len(bars); end++ {
		upper, middle, lower := market.ExportCalculateBOLL(bars[:end], period, 2)
		if middle > 0 {
			widths = append(widths, (upper-lower)/middle*100)
		}
	}
	if len(widths) <= changeLookback {
		return 0, 0, 0, false
	}
	width = widths[len(widths)-1]
	less, equal := 0, 0
	for _, v := range widths {
		switch {
		case v < width:
			less++
		case math.Abs(v-width) <= 1e-12:
			equal++
		}
	}
	percentile = (float64(less) + 0.5*float64(equal)) / float64(len(widths)) * 100
	previous := widths[len(widths)-1-changeLookback]
	if previous > 0 {
		change = (width/previous - 1) * 100
	}
	return width, percentile, change, true
}

// wilderADX computes standard Wilder ADX from closed OHLC bars.
func wilderADX(bars []market.Kline, period int) float64 {
	if period <= 0 || len(bars) < 2*period+1 {
		return 0
	}
	tr := make([]float64, len(bars))
	plusDM := make([]float64, len(bars))
	minusDM := make([]float64, len(bars))
	for i := 1; i < len(bars); i++ {
		upMove := bars[i].High - bars[i-1].High
		downMove := bars[i-1].Low - bars[i].Low
		if upMove > downMove && upMove > 0 {
			plusDM[i] = upMove
		}
		if downMove > upMove && downMove > 0 {
			minusDM[i] = downMove
		}
		tr[i] = math.Max(bars[i].High-bars[i].Low,
			math.Max(math.Abs(bars[i].High-bars[i-1].Close), math.Abs(bars[i].Low-bars[i-1].Close)))
	}
	var smTR, smPlus, smMinus float64
	for i := 1; i <= period; i++ {
		smTR += tr[i]
		smPlus += plusDM[i]
		smMinus += minusDM[i]
	}
	dx := make([]float64, 0, len(bars)-period)
	appendDX := func() {
		if smTR <= 0 {
			dx = append(dx, 0)
			return
		}
		pdi := 100 * smPlus / smTR
		mdi := 100 * smMinus / smTR
		if pdi+mdi == 0 {
			dx = append(dx, 0)
			return
		}
		dx = append(dx, 100*math.Abs(pdi-mdi)/(pdi+mdi))
	}
	appendDX()
	for i := period + 1; i < len(bars); i++ {
		smTR = smTR - smTR/float64(period) + tr[i]
		smPlus = smPlus - smPlus/float64(period) + plusDM[i]
		smMinus = smMinus - smMinus/float64(period) + minusDM[i]
		appendDX()
	}
	if len(dx) < period {
		return 0
	}
	adx := 0.0
	for _, v := range dx[:period] {
		adx += v
	}
	adx /= float64(period)
	for _, v := range dx[period:] {
		adx = (adx*float64(period-1) + v) / float64(period)
	}
	return adx
}

func floatPtr(v float64) *float64 { return &v }

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

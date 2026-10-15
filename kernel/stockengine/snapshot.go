package stockengine

import (
	"nofx/market/usstock"
	"time"
)

// Raw candles shown to the model per timeframe, and the RSI period.
const (
	promptCandles1w = 26
	promptCandles1d = 30
	promptCandles4h = 24
	rsiPeriod       = 14
)

var promptCandleCounts = map[string]int{usstock.TF1w: promptCandles1w, usstock.TF1d: promptCandles1d, usstock.TF4h: promptCandles4h}

// BuildSnapshot computes indicators from closed bars only. The contract has no
// preset parameter: TrendEntry uses supplied 1h bars, or daily bars when 1h is
// absent (position callers should supply daily/weekly series). MissingTF remains
// independent of that choice; Validate gates the resolved preset's timeframes.
// now is injected for API consistency; freshness comes from the supplied quote.
func BuildSnapshot(symbol, underlying string, series map[string]*usstock.Series, quote *usstock.Quote, now time.Time) *SymbolSnapshot {
	s := &SymbolSnapshot{Symbol: symbol, Underlying: underlying, Sources: make(map[string]string), Quote: quote,
		TrendDaily: "unknown", TrendWeekly: "unknown", TrendEntry: "unknown"}
	needs := map[string]int{usstock.TF15m: 60, usstock.TF1h: 60, usstock.TF4h: 60, usstock.TF1d: 200, usstock.TF1w: 30}
	for _, tf := range []string{usstock.TF15m, usstock.TF1h, usstock.TF4h, usstock.TF1d, usstock.TF1w} {
		data := series[tf]
		if data != nil {
			s.Sources[tf] = data.Source
		}
		if data == nil || len(data.Bars) < needs[tf] {
			s.MissingTF = append(s.MissingTF, tf)
		}
		if s.Price == 0 && data != nil && len(data.Bars) > 0 {
			s.Price = data.Bars[len(data.Bars)-1].Close
		}
	}
	// Preserve sources for any additional caller-supplied timeframe as well.
	for tf, data := range series {
		if data != nil {
			s.Sources[tf] = data.Source
		}
	}
	if quote != nil {
		s.Price = quote.BStockPrice
	}
	if daily := series[usstock.TF1d]; daily != nil && len(daily.Bars) > 0 {
		bars := daily.Bars
		values := closes(bars)
		n := len(bars)
		s.EMA20, s.EMA50, s.EMA200 = last(ema(values, 20)), last(ema(values, 50)), last(ema(values, 200))
		if n >= 200 {
			f := ema(values, 50)
			i := n - 1
			s.TrendDaily = "range"
			if values[i] > s.EMA50 && s.EMA50 > s.EMA200 && f[i] > f[i-10] {
				s.TrendDaily = "up"
			}
			if values[i] < s.EMA50 && s.EMA50 < s.EMA200 && f[i] < f[i-10] {
				s.TrendDaily = "down"
			}
		}
		s.ATR1d = atr(bars, 14)
		if last(values) > 0 {
			s.ATR1dPct = s.ATR1d / last(values) * 100
		}
		start := n - 252
		if start < 0 {
			start = 0
		}
		s.High52w, s.Low52w = bars[start].High, bars[start].Low
		for _, b := range bars[start:] {
			if b.High > s.High52w {
				s.High52w = b.High
			}
			if b.Low < s.Low52w {
				s.Low52w = b.Low
			}
		}
		if s.High52w > 0 {
			s.PctFrom52wHigh = (last(values)/s.High52w - 1) * 100
		}
		if v := averageVolume(bars, 50); v > 0 {
			s.VolumeRatio20_50 = averageVolume(bars, 20) / v
		}
		if n > 20 && values[n-21] > 0 {
			s.Return20dPct = (values[n-1]/values[n-21] - 1) * 100
		}
		s.SwingLow, s.SwingHigh = pivots(bars, 3)
	}
	if weekly := series[usstock.TF1w]; weekly != nil {
		s.TrendWeekly = alignedTrend(closes(weekly.Bars), 10, 30, 0)
	}
	entry := series[usstock.TF1h]
	if entry == nil {
		entry = series[usstock.TF1d]
	}
	if entry != nil {
		s.TrendEntry = alignedTrend(closes(entry.Bars), 20, 50, 10)
	}
	for tf, n := range promptCandleCounts {
		data := series[tf]
		if data == nil || len(data.Bars) == 0 {
			continue
		}
		if len(data.Bars) >= rsiPeriod+1 {
			if s.RSI == nil {
				s.RSI = make(map[string]float64)
			}
			s.RSI[tf] = rsi(closes(data.Bars), rsiPeriod)
		}
		tail := data.Bars
		if len(tail) > n {
			tail = tail[len(tail)-n:]
		}
		if s.Candles == nil {
			s.Candles = make(map[string][]usstock.Bar)
		}
		s.Candles[tf] = append([]usstock.Bar(nil), tail...)
	}
	return s
}

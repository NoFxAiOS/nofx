package stockengine

import (
	"math"
	"nofx/market/usstock"
)

// ema is aligned with values; zero entries precede the period-bar SMA seed.
func ema(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	if period <= 0 || len(values) < period {
		return out
	}
	for _, v := range values[:period] {
		out[period-1] += v / float64(period)
	}
	alpha := 2 / float64(period+1)
	for i := period; i < len(values); i++ {
		out[i] = alpha*values[i] + (1-alpha)*out[i-1]
	}
	return out
}

// atr uses Wilder's smoothing, seeded with the first period true ranges.
// The first true range is high-low because no previous close is available.
func atr(bars []usstock.Bar, period int) float64 {
	if period <= 0 || len(bars) < period {
		return 0
	}
	value := 0.0
	for i, b := range bars {
		tr := b.High - b.Low
		if i > 0 {
			tr = math.Max(tr, math.Max(math.Abs(b.High-bars[i-1].Close), math.Abs(b.Low-bars[i-1].Close)))
		}
		if i < period {
			value += tr / float64(period)
		} else {
			value = (value*float64(period-1) + tr) / float64(period)
		}
	}
	return value
}

// pivots returns the latest strictly confirmed low/high with width bars on each side.
func pivots(bars []usstock.Bar, width int) (low, high float64) {
	if width < 1 {
		return
	}
	for i := width; i+width < len(bars); i++ {
		isLow, isHigh := true, true
		for j := i - width; j <= i+width; j++ {
			if j == i {
				continue
			}
			isLow = isLow && bars[i].Low < bars[j].Low
			isHigh = isHigh && bars[i].High > bars[j].High
		}
		if isLow {
			low = bars[i].Low
		}
		if isHigh {
			high = bars[i].High
		}
	}
	return
}

func closes(bars []usstock.Bar) []float64 {
	values := make([]float64, len(bars))
	for i, b := range bars {
		values[i] = b.Close
	}
	return values
}
func last(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return values[len(values)-1]
}
func averageVolume(bars []usstock.Bar, period int) float64 {
	if len(bars) < period {
		return 0
	}
	total := 0.0
	for _, b := range bars[len(bars)-period:] {
		total += b.Volume
	}
	return total / float64(period)
}
func alignedTrend(values []float64, fast, slow, slope int) string {
	if len(values) < slow+slope {
		return "unknown"
	}
	f, s := ema(values, fast), ema(values, slow)
	i := len(values) - 1
	if values[i] > f[i] && f[i] > s[i] && (slope == 0 || f[i] > f[i-slope]) {
		return "up"
	}
	if values[i] < f[i] && f[i] < s[i] && (slope == 0 || f[i] < f[i-slope]) {
		return "down"
	}
	return "range"
}

// rsi is Wilder's RSI on values, last value. It returns 50 when undefined
// (fewer than period+1 values) and 100 when there were no losses.
func rsi(values []float64, period int) float64 {
	if period <= 0 || len(values) < period+1 {
		return 50
	}
	var gain, loss float64
	for i := 1; i <= period; i++ {
		d := values[i] - values[i-1]
		if d > 0 {
			gain += d
		} else {
			loss -= d
		}
	}
	gain /= float64(period)
	loss /= float64(period)
	for i := period + 1; i < len(values); i++ {
		d := values[i] - values[i-1]
		g, l := 0.0, 0.0
		if d > 0 {
			g = d
		} else {
			l = -d
		}
		gain = (gain*float64(period-1) + g) / float64(period)
		loss = (loss*float64(period-1) + l) / float64(period)
	}
	if loss == 0 {
		return 100
	}
	return 100 - 100/(1+gain/loss)
}

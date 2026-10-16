package main

import (
	"math"

	"nofx/market/usstock"
)

// All rule thresholds are in bars of the execution timeframe. Momentum always
// uses daily bars. Rolling event windows include today; prior-low/high tests
// exclude today. Strict new highs must exceed all preceding HighLookback closes.
const (
	FastPeriod          = 20
	SlowPeriod          = 60
	TrendPeriod         = 200
	MomentumSkip        = 21
	MomentumLookback    = 252
	ATRPeriod           = 14
	SlopeLookback       = 5
	TouchTolerance      = 0.005
	SmallBreakTolerance = 0.02
	EntryLookback       = 3
	OversoldATR         = 2.5
	OverextendedATR     = 3.0
	ExtensionLookback   = 20
	HighLookback        = 20
	X2NoHighLookback    = 5
	X3NoHighLookback    = 10
	BreakLowLookback    = 5
	StopATR             = 2.0
)

type indicator struct {
	Fast, Slow, Trend, ATR float64
	NewHigh                bool
}

// SMA and Wilder ATR are calculated forward only, with unavailable values NaN.
func indicators(bars []usstock.Bar) []indicator {
	out := make([]indicator, len(bars))
	var fast, slow, trend, seed, atr float64
	for i, b := range bars {
		fast += b.Close
		slow += b.Close
		trend += b.Close
		if i >= FastPeriod {
			fast -= bars[i-FastPeriod].Close
		}
		if i >= SlowPeriod {
			slow -= bars[i-SlowPeriod].Close
		}
		if i >= TrendPeriod {
			trend -= bars[i-TrendPeriod].Close
		}
		v := indicator{Fast: math.NaN(), Slow: math.NaN(), Trend: math.NaN(), ATR: math.NaN()}
		if i+1 >= FastPeriod {
			v.Fast = fast / FastPeriod
		}
		if i+1 >= SlowPeriod {
			v.Slow = slow / SlowPeriod
		}
		if i+1 >= TrendPeriod {
			v.Trend = trend / TrendPeriod
		}
		tr := b.High - b.Low
		if i > 0 {
			tr = math.Max(tr, math.Max(math.Abs(b.High-bars[i-1].Close), math.Abs(b.Low-bars[i-1].Close)))
		}
		if i < ATRPeriod {
			seed += tr
			atr = seed / ATRPeriod
		} else {
			atr = (atr*(ATRPeriod-1) + tr) / ATRPeriod
		}
		if i+1 >= ATRPeriod {
			v.ATR = atr
		}
		if i >= HighLookback {
			v.NewHigh = true
			for j := i - HighLookback; j < i; j++ {
				if b.Close <= bars[j].Close {
					v.NewHigh = false
					break
				}
			}
		}
		out[i] = v
	}
	return out
}

func golden(v []indicator, i int) bool {
	return i > 0 && v[i-1].Fast <= v[i-1].Slow && v[i].Fast > v[i].Slow
}

func death(v []indicator, i int) bool {
	return i > 0 && v[i-1].Fast >= v[i-1].Slow && v[i].Fast < v[i].Slow
}

func ready(v []indicator, i int) bool {
	return i >= 0 && i < len(v) && finite(v[i].Fast) && finite(v[i].Slow) && finite(v[i].ATR)
}

func bullishKey(bars []usstock.Bar, i int) bool {
	if i < 1 {
		return false
	}
	b, p := bars[i], bars[i-1]
	engulf := p.Close < p.Open && b.Close > b.Open && b.Open <= p.Close && b.Close >= p.Open
	return b.Close > p.High || engulf
}

// Entry attribution uses the first matching rule in E1, E2, E3, E4 order.
func entryRule(bars []usstock.Bar, v []indicator, i int) string {
	if !ready(v, i) {
		return ""
	}
	b, a := bars[i], v[i]
	if golden(v, i) {
		return "E1"
	}
	rising := i >= SlopeLookback && a.Slow > v[i-SlopeLookback].Slow
	if a.Fast > a.Slow && rising {
		if b.Low <= a.Fast*(1+TouchTolerance) && b.Close >= a.Slow && b.Close > b.Open {
			return "E2"
		}
		if b.Close > a.Fast {
			for j := max(0, i-EntryLookback+1); j <= i; j++ {
				if bars[j].Close < v[j].Fast && bars[j].Close >= v[j].Slow*(1-SmallBreakTolerance) {
					return "E3"
				}
			}
		}
	}
	if bullishKey(bars, i) {
		for j := max(0, i-EntryLookback+1); j <= i; j++ {
			if finite(v[j].Fast) && finite(v[j].ATR) && bars[j].Close/v[j].Fast-1 <= -OversoldATR*v[j].ATR/bars[j].Close {
				return "E4"
			}
		}
	}
	return ""
}

func noNewHigh(v []indicator, i, lookback int) bool {
	if i-lookback+1 < HighLookback {
		return false
	}
	for j := i - lookback + 1; j <= i; j++ {
		if v[j].NewHigh {
			return false
		}
	}
	return true
}

// X1..X4 take precedence over an E4 target, and exits take precedence over
// entries on the same close. The stop uses the actual fill price and signal ATR
// (the last ATR observable before the entry open).
func exitRule(bars []usstock.Bar, v []indicator, i int, entryPrice, entryATR float64, bounce bool) string {
	if !ready(v, i) {
		return ""
	}
	b, a := bars[i], v[i]
	if death(v, i) {
		return "X1"
	}
	extended := false
	for j := max(0, i-ExtensionLookback+1); j <= i; j++ {
		if finite(v[j].Fast) && finite(v[j].ATR) && bars[j].Close/v[j].Fast-1 >= OverextendedATR*v[j].ATR/bars[j].Close {
			extended = true
		}
	}
	if extended && noNewHigh(v, i, X2NoHighLookback) && i >= BreakLowLookback {
		low := math.Inf(1)
		for j := i - BreakLowLookback; j < i; j++ {
			low = math.Min(low, bars[j].Low)
		}
		if b.Close < low {
			return "X2"
		}
	}
	if noNewHigh(v, i, X3NoHighLookback) && b.Close < math.Min(a.Fast, a.Slow) {
		return "X3"
	}
	if b.Close < entryPrice-StopATR*entryATR {
		return "X4"
	}
	if bounce && b.Close >= a.Fast {
		return "TARGET"
	}
	return ""
}

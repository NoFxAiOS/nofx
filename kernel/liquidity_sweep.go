package kernel

import (
	"math"
	"sort"
	"time"

	"nofx/market"
)

// LiquiditySweep is optional research evidence only. No gate or execution
// decision reads it. Each side carries its latest event across 15m and 1h.
type LiquiditySweep struct {
	SellsideSwept *LiquiditySweepEvent `json:"sellside_swept"`
	BuysideSwept  *LiquiditySweepEvent `json:"buyside_swept"`
}

type LiquiditySweepEvent struct {
	TF           string  `json:"tf"`
	BarsAgo      int     `json:"bars_ago"` // latest closed bar = 0, on this event's TF
	Level        float64 `json:"level"`
	Extreme      float64 `json:"extreme"`
	DepthPct     float64 `json:"depth_pct"`
	MSSConfirmed bool    `json:"mss_confirmed"`
	Reclaimed    bool    `json:"reclaimed"`
}

type indexedSweepPivot struct {
	index int
	price float64
}

// indexedSweepPivots preserves swingPivots' strict fractal definition without
// changing that helper's callers. A pivot at i is confirmed at i+shoulder.
func indexedSweepPivots(bars []market.KlineBar, shoulder int) (highs, lows []indexedSweepPivot) {
	for i := shoulder; i < len(bars)-shoulder; i++ {
		high, low := true, true
		for j := i - shoulder; j <= i+shoulder; j++ {
			if j == i {
				continue
			}
			high = high && bars[i].High > bars[j].High
			low = low && bars[i].Low < bars[j].Low
		}
		if high {
			highs = append(highs, indexedSweepPivot{i, bars[i].High})
		}
		if low {
			lows = append(lows, indexedSweepPivot{i, bars[i].Low})
		}
	}
	return highs, lows
}

func computeLiquiditySweep(data *market.Data, now time.Time) *LiquiditySweep {
	if data == nil {
		return nil
	}
	result := &LiquiditySweep{}
	var sellsideTime, buysideTime time.Time
	for _, cfg := range []struct {
		tf                         string
		shoulder, lookback, recent int
	}{
		{"15m", 2, 40, 8},
		{"1h", 3, 48, 4},
	} {
		td := data.TimeframeData[cfg.tf]
		if td == nil {
			continue
		}
		dur := tfDuration(cfg.tf)
		// Historical snapshots may include multiple future bars. Trim those
		// first, then use the signal layer's settlement rule for the forming bar.
		snapshot := *td
		end := sort.Search(len(td.Klines), func(i int) bool { return td.Klines[i].Time > now.UnixMilli() })
		snapshot.Klines = td.Klines[:end]
		bars := ClosedKlines(&snapshot, now, dur)
		highs, lows := indexedSweepPivots(bars, cfg.shoulder)
		long := liquiditySweepSide(bars, lows, highs, cfg.tf, cfg.shoulder, cfg.lookback, cfg.recent, true)
		short := liquiditySweepSide(bars, highs, lows, cfg.tf, cfg.shoulder, cfg.lookback, cfg.recent, false)
		// Compare actual close times, not bars_ago across different timeframes.
		// Equal-time events prefer 15m (the first timeframe above).
		if long != nil {
			at := time.UnixMilli(bars[len(bars)-1-long.BarsAgo].Time).Add(dur)
			if result.SellsideSwept == nil || at.After(sellsideTime) {
				result.SellsideSwept, sellsideTime = long, at
			}
		}
		if short != nil {
			at := time.UnixMilli(bars[len(bars)-1-short.BarsAgo].Time).Add(dur)
			if result.BuysideSwept == nil || at.After(buysideTime) {
				result.BuysideSwept, buysideTime = short, at
			}
		}
	}
	if result.SellsideSwept == nil && result.BuysideSwept == nil {
		return nil
	}
	return result
}

// liquiditySweepSide mirrors ict.py's _sweeps: select the latest unconsumed
// pivot confirmed BEFORE s, then consume EVERY pierced level, including
// non-selected levels and piercings that close beyond (rather than reclaim) it.
func liquiditySweepSide(bars []market.KlineBar, refs, opposite []indexedSweepPivot, tf string, shoulder, lookback, recent int, long bool) *LiquiditySweepEvent {
	active := make([]indexedSweepPivot, 0)
	next := 0
	var latest *LiquiditySweepEvent
	sign := 1.0
	if !long {
		sign = -1
	}
	for s, bar := range bars {
		for next < len(refs) && refs[next].index+shoulder < s {
			active = append(active, refs[next])
			next++
		}
		kept := active[:0]
		for _, p := range active {
			if s-p.index <= lookback {
				kept = append(kept, p)
			}
		}
		active = kept
		extreme := bar.Low
		if !long {
			extreme = bar.High
		}
		if len(active) > 0 {
			ref := active[len(active)-1]
			if len(bars)-1-s < recent && ref.price > 0 &&
				sign*(extreme-ref.price) < 0 && sign*(bar.Close-ref.price) > 0 {
				latest = &LiquiditySweepEvent{
					TF: tf, BarsAgo: len(bars) - 1 - s,
					Level: ref.price, Extreme: extreme,
					DepthPct:     math.Abs(extreme-ref.price) / ref.price * 100,
					MSSConfirmed: liquiditySweepMSS(bars, opposite, s, shoulder, lookback, sign),
					Reclaimed:    true,
				}
			}
		}
		kept = active[:0]
		for _, p := range active {
			if sign*(extreme-p.price) >= 0 {
				kept = append(kept, p)
			}
		}
		active = kept
	}
	return latest
}

func liquiditySweepMSS(bars []market.KlineBar, opposite []indexedSweepPivot, s, shoulder, lookback int, sign float64) bool {
	for i := len(opposite) - 1; i >= 0; i-- {
		p := opposite[i]
		if p.index+shoulder > s {
			continue
		}
		if s-p.index > lookback {
			break
		}
		// Match ict.py's MSS lookup: the opposite pivot must predate s,
		// may be confirmed at s's close, and need not be unconsumed.
		for _, bar := range bars[s+1:] {
			if sign*(bar.Close-p.price) > 0 {
				return true
			}
		}
		return false
	}
	return false
}

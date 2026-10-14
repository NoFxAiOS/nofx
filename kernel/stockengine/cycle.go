package stockengine

import (
	"fmt"
	"math"
	"time"

	"nofx/market/usstock"
)

// Daily cycle heuristics. Keep periods, thresholds and score weights together
// so tuning does not change the phase precedence below.
const (
	cycleMinBars             = 60
	cycleFastEMA             = 10
	cycleSlowEMA             = 20
	cycleTrendSMA            = 50
	cycleLongSMA             = 200
	cycleATRPeriod           = 14
	cycleVolumePeriod        = 50
	cycleRSPeriod            = 63
	cycleEventAge            = 2
	cycleExtensionATR        = 2.5
	cycleDropLookback        = 5
	cycleConfirmVolume       = 1.2
	cycleReclaimWindow       = 10
	cycleReclaimBelow        = 5
	cyclePopStretchAge       = 12
	cyclePopStretchEndAge    = 3
	cycleMATightATR          = 1.0
	cycleResistanceBars      = 5
	cycleCrossbackMaxAge     = 25
	cycleCrossbackMinAge     = 4
	cycleTouchATR            = 0.5
	cycleSupportATR          = 0.25
	cycleCrossbackExtension  = 1.0
	cycleBaseMinBars         = 10
	cycleBaseMaxBars         = 30
	cycleBaseRangeATR        = 4.0
	cycleBreakVolume         = 1.3
	cycleReversalLookback    = 10
	cycleStrongVolume        = 1.5
	cycleReversalRecoveryATR = -1.0
	cycleFormingBars         = 10
	cycleFormingRangeATR     = 3.0
	cycleFormingHighATR      = 1.0
	cycleFormingDistanceATR  = 1.5
	cycleTightBars           = 5
	cycleTightRangeATR       = 1.5
	cyclePopScore            = 40.0
	cycleCrossbackScore      = 40.0
	cycleBreakScore          = 35.0
	cycleFormingScore        = 25.0
	cycleReversalScore       = 20.0
	cycleRSScoreDivisor      = 2.0
	cycleRSScoreMin          = -10.0
	cycleRSScoreMax          = 20.0
	cycleAboveSMAScore       = 8.0
	cycleSMAStructureScore   = 7.0
	cycleStrongVolumeScore   = 10.0
	cycleConfirmVolumeScore  = 5.0
	cycleNearDistanceATR     = 1.0
	cycleFarDistanceATR      = 2.0
	cycleNearDistanceScore   = 10.0
	cycleFarDistanceScore    = 5.0
	cycleTightScore          = 5.0
	cycleMaxScore            = 100.0
)

type CyclePhase string

const (
	PhaseReversalExtension   CyclePhase = "reversal_extension"
	PhaseWedgePop            CyclePhase = "wedge_pop"
	PhaseEMACrossback        CyclePhase = "ema_crossback"
	PhaseBaseNBreak          CyclePhase = "base_n_break"
	PhaseBaseForming         CyclePhase = "base_forming"
	PhaseUptrend             CyclePhase = "uptrend"
	PhaseExhaustionExtension CyclePhase = "exhaustion_extension"
	PhaseWedgeDrop           CyclePhase = "wedge_drop"
	PhaseDowntrend           CyclePhase = "downtrend"
	PhaseNeutral             CyclePhase = "neutral"
	PhaseInsufficient        CyclePhase = "insufficient_data"
)

// CycleAssessment is the daily-bar read of one symbol.
type CycleAssessment struct {
	Phase         CyclePhase `json:"phase"`
	LongSetup     bool       `json:"long_setup"`
	Score         float64    `json:"score"`
	Ext10ATR      float64    `json:"ext10_atr"`
	Dist20ATR     float64    `json:"dist20_atr"`
	RS63Pct       float64    `json:"rs63_pct"`
	VolRatio      float64    `json:"vol_ratio"`
	AboveSMA50    bool       `json:"above_sma50"`
	SMA50Above200 bool       `json:"sma50_above_200"`
	Bars          int        `json:"bars"`
	Notes         []string   `json:"notes,omitempty"`
}

// ClassifyCycle reads the latest closed daily bar of bars (ascending) against
// spy (SPY daily bars, may be nil). Pure function, no IO.
func ClassifyCycle(bars, spy []usstock.Bar) CycleAssessment {
	a := CycleAssessment{Phase: PhaseInsufficient, Bars: len(bars)}
	if len(bars) < cycleMinBars {
		a.Notes = []string{fmt.Sprintf("need %d daily bars; have %d", cycleMinBars, len(bars))}
		return a
	}
	i := len(bars) - 1
	values := closes(bars)
	fast, slow := ema(values, cycleFastEMA), ema(values, cycleSlowEMA)
	// Cache only requested historical ATRs; atr retains the established Wilder
	// seed and each event is judged without looking ahead.
	atrs := make(map[int]float64)
	atrAt := func(j int) float64 {
		if v, ok := atrs[j]; ok {
			return v
		}
		v := atr(bars[:j+1], cycleATRPeriod)
		atrs[j] = v
		return v
	}
	currentATR := atrAt(i)
	if currentATR <= 0 || !cycleFinite(currentATR) || !cycleFinite(fast[i]) || !cycleFinite(slow[i]) {
		a.Notes = []string{"daily ATR is unavailable"}
		return a
	}
	extAt := func(j int) float64 {
		if v := atrAt(j); v > 0 {
			return (bars[j].Close - fast[j]) / v
		}
		return 0
	}
	aboveBoth := func(j int) bool { return bars[j].Close > fast[j] && bars[j].Close > slow[j] }
	sma50 := cycleSMA(values, cycleTrendSMA)
	a.Ext10ATR = extAt(i)
	a.Dist20ATR = (bars[i].Close - slow[i]) / currentATR
	a.RS63Pct = cycleRelativeStrength(bars, spy)
	a.AboveSMA50 = bars[i].Close > sma50
	if len(bars) >= cycleLongSMA {
		a.SMA50Above200 = sma50 > cycleSMA(values, cycleLongSMA)
	}
	a.Phase = PhaseNeutral
	event := i

	// First match wins, including the bearish vetoes before any long setup.
	maxExt := math.Inf(-1)
	for j := i - cycleEventAge; j <= i; j++ {
		maxExt = math.Max(maxExt, extAt(j))
	}
	if a.AboveSMA50 && maxExt >= cycleExtensionATR {
		a.Phase = PhaseExhaustionExtension
		a.Notes = []string{fmt.Sprintf("recent extension %.1f ATR above EMA10", maxExt)}
		return cycleFinish(a, bars, event, currentATR)
	}

	if bars[i].Close < fast[i] && bars[i].Close < slow[i] {
		// Use the actual loss bar's volume, even when it precedes today's bar.
		for j := i - 1; j >= i-cycleDropLookback; j-- {
			if !aboveBoth(j) {
				continue
			}
			breakBar := -1
			for k := j + 1; k <= i; k++ {
				if bars[k].Close < fast[k] && bars[k].Close < slow[k] {
					breakBar = k
					break
				}
			}
			if breakBar >= 0 && (fast[i] < slow[i] || cycleVolumeRatio(bars, breakBar) >= cycleConfirmVolume) {
				a.Phase, event = PhaseWedgeDrop, breakBar
				a.Notes = []string{"fresh loss of EMA10 and EMA20", fmt.Sprintf("break bar vol %.1fx", cycleVolumeRatio(bars, event))}
				return cycleFinish(a, bars, event, currentATR)
			}
			break
		}
	}
	if fast[i] < slow[i] && bars[i].Close < slow[i] && bars[i].Close < sma50 {
		a.Phase = PhaseDowntrend
		a.Notes = []string{"below falling EMA structure and SMA50"}
		return cycleFinish(a, bars, event, currentATR)
	}

	if aboveBoth(i) {
		below, lastBelow := 0, -1
		for j := i - cyclePopStretchAge; j <= i-cyclePopStretchEndAge; j++ {
			if bars[j].Close < slow[j] {
				below++
				lastBelow = j
			}
		}
		if below >= cycleReclaimBelow {
			// The first reclaim following the below-EMA stretch is the pop;
			// subsequent closes above the averages are not new pop events.
			pop := -1
			for j := lastBelow + 1; j <= i; j++ {
				if aboveBoth(j) {
					pop = j
					break
				}
			}
			if pop >= i-cycleEventAge && atrAt(pop) > 0 && math.Abs(fast[pop]-slow[pop])/atrAt(pop) <= cycleMATightATR {
				hi, _ := cycleRange(bars[pop-cycleResistanceBars : pop])
				if bars[pop].Close > hi {
					a.Phase, a.LongSetup, event = PhaseWedgePop, true, pop
					a.Notes = []string{fmt.Sprintf("first EMA reclaim after %d below-EMA20 bars", below), fmt.Sprintf("pop bar vol %.1fx", cycleVolumeRatio(bars, pop))}
					return cycleFinish(a, bars, event, currentATR)
				}
			}
		}
	}

	if fast[i] >= slow[i] && bars[i].Close >= slow[i]-cycleSupportATR*currentATR && a.Ext10ATR < cycleCrossbackExtension {
		touch := i - cycleEventAge
		for j := touch + 1; j <= i; j++ {
			if bars[j].Low <= bars[touch].Low {
				touch = j
			}
		}
		touchATR := atrAt(touch)
		distance := math.Min(math.Abs(bars[touch].Low-fast[touch]), math.Abs(bars[touch].Low-slow[touch]))
		if touchATR > 0 && distance <= cycleTouchATR*touchATR {
			for pop := i - cycleCrossbackMinAge; pop >= i-cycleCrossbackMaxAge; pop-- {
				if !cycleReclaim(bars, fast, slow, pop) {
					continue
				}
				a.Phase, a.LongSetup, event = PhaseEMACrossback, true, touch
				a.Notes = []string{fmt.Sprintf("EMA reclaim %d bars ago", i-pop), fmt.Sprintf("pullback low %.1f ATR from EMA support", distance/touchATR)}
				return cycleFinish(a, bars, event, currentATR)
			}
		}
	}

	if a.AboveSMA50 && fast[i] >= slow[i] {
		for breakout := i; breakout >= i-cycleEventAge; breakout-- {
			for length := cycleBaseMaxBars; length >= cycleBaseMinBars; length-- {
				hi, lo := cycleRange(bars[breakout-length : breakout])
				if hi-lo > cycleBaseRangeATR*atrAt(breakout) {
					continue
				}
				// Judge the longest qualifying base, not a shorter convenient one.
				if bars[breakout].Close > hi && cycleVolumeRatio(bars, breakout) >= cycleBreakVolume {
					a.Phase, a.LongSetup, event = PhaseBaseNBreak, true, breakout
					a.Notes = []string{fmt.Sprintf("breakout above %d-bar base", length), fmt.Sprintf("breakout vol %.1fx", cycleVolumeRatio(bars, breakout))}
					return cycleFinish(a, bars, event, currentATR)
				}
				break
			}
		}
	}

	if !aboveBoth(i) && a.Ext10ATR > cycleReversalRecoveryATR {
		for j := i; j >= i-cycleReversalLookback; j-- {
			if extAt(j) > -cycleExtensionATR || bars[i].Close <= bars[j].Low {
				continue
			}
			volumeConfirmed := false
			for k := max(0, j-1); k <= min(i, j+1); k++ {
				volumeConfirmed = volumeConfirmed || cycleVolumeRatio(bars, k) >= cycleStrongVolume
			}
			if volumeConfirmed {
				a.Phase, a.LongSetup, event = PhaseReversalExtension, true, j
				a.Notes = []string{fmt.Sprintf("capitulation %.1f ATR below EMA10", extAt(j)), "recovering toward EMAs on confirmed volume"}
				return cycleFinish(a, bars, event, currentATR)
			}
		}
	}

	hi, lo := cycleRange(bars[len(bars)-cycleFormingBars:])
	if bars[i].Close > slow[i] && fast[i] >= slow[i] && hi-lo <= cycleFormingRangeATR*currentATR && hi-bars[i].Close <= cycleFormingHighATR*currentATR && math.Abs(a.Dist20ATR) <= cycleFormingDistanceATR {
		a.Phase, a.LongSetup = PhaseBaseForming, true
		a.Notes = []string{fmt.Sprintf("%d-bar base range %.1f ATR", cycleFormingBars, (hi-lo)/currentATR), "tightening above EMA20"}
	} else if bars[i].Close > slow[i] && slow[i] > sma50 {
		a.Phase = PhaseUptrend
		a.Notes = []string{"price above EMA20 above SMA50"}
	} else {
		a.Notes = []string{"no defined daily cycle entry"}
	}
	return cycleFinish(a, bars, event, currentATR)
}

func cycleReclaim(bars []usstock.Bar, fast, slow []float64, j int) bool {
	if bars[j].Close <= fast[j] || bars[j].Close <= slow[j] || (bars[j-1].Close > fast[j-1] && bars[j-1].Close > slow[j-1]) {
		return false
	}
	below := 0
	for k := j - cycleReclaimWindow; k < j; k++ {
		if bars[k].Close < slow[k] {
			below++
		}
	}
	return below >= cycleReclaimBelow
}

func cycleFinish(a CycleAssessment, bars []usstock.Bar, event int, currentATR float64) CycleAssessment {
	a.VolRatio = cycleVolumeRatio(bars, event)
	if !a.LongSetup {
		return a
	}
	score := map[CyclePhase]float64{
		PhaseWedgePop: cyclePopScore, PhaseEMACrossback: cycleCrossbackScore,
		PhaseBaseNBreak: cycleBreakScore, PhaseBaseForming: cycleFormingScore,
		PhaseReversalExtension: cycleReversalScore,
	}[a.Phase]
	score += math.Max(cycleRSScoreMin, math.Min(cycleRSScoreMax, a.RS63Pct/cycleRSScoreDivisor))
	if a.AboveSMA50 {
		score += cycleAboveSMAScore
	}
	if a.SMA50Above200 {
		score += cycleSMAStructureScore
	}
	if a.VolRatio >= cycleStrongVolume {
		score += cycleStrongVolumeScore
	} else if a.VolRatio >= cycleConfirmVolume {
		score += cycleConfirmVolumeScore
	}
	if math.Abs(a.Dist20ATR) <= cycleNearDistanceATR {
		score += cycleNearDistanceScore
	} else if math.Abs(a.Dist20ATR) <= cycleFarDistanceATR {
		score += cycleFarDistanceScore
	}
	hi, lo := cycleRange(bars[len(bars)-cycleTightBars:])
	if hi-lo <= cycleTightRangeATR*currentATR {
		score += cycleTightScore
	}
	a.Score = math.Round(math.Max(0, math.Min(cycleMaxScore, score))*10) / 10
	return a
}

func cycleVolumeRatio(bars []usstock.Bar, j int) float64 {
	if avg := averageVolume(bars[:j], cycleVolumePeriod); avg > 0 {
		return bars[j].Volume / avg
	}
	return 0
}

func cycleSMA(values []float64, period int) float64 {
	if len(values) < period {
		return 0
	}
	total := 0.0
	for _, v := range values[len(values)-period:] {
		total += v
	}
	return total / float64(period)
}

func cycleRange(bars []usstock.Bar) (high, low float64) {
	high, low = math.Inf(-1), math.Inf(1)
	for _, b := range bars {
		high, low = math.Max(high, b.High), math.Min(low, b.Low)
	}
	return
}

// Align sessions by UTC calendar date, not by slice position or intraday
// timestamps (Yahoo session opens and bStock midnight can differ). Measure
// both returns over the latest 64 common dates; missing sessions cannot shift
// one leg of the comparison to a different date.
func cycleRelativeStrength(bars, spy []usstock.Bar) float64 {
	if len(bars) <= cycleRSPeriod || len(spy) <= cycleRSPeriod {
		return 0
	}
	benchmark := make(map[time.Time]float64, len(spy))
	for _, b := range spy {
		benchmark[cycleDate(b.OpenTime)] = b.Close
	}
	type pair struct{ stock, spy float64 }
	aligned := make([]pair, 0, cycleRSPeriod+1)
	for j := len(bars) - 1; j >= 0 && len(aligned) <= cycleRSPeriod; j-- {
		if value, ok := benchmark[cycleDate(bars[j].OpenTime)]; ok {
			aligned = append(aligned, pair{bars[j].Close, value})
		}
	}
	if len(aligned) <= cycleRSPeriod {
		return 0
	}
	end, start := aligned[0], aligned[cycleRSPeriod]
	if start.stock <= 0 || start.spy <= 0 {
		return 0
	}
	rs := (end.stock/start.stock - end.spy/start.spy) * 100
	if !cycleFinite(rs) {
		return 0
	}
	return rs
}

func cycleDate(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func cycleFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

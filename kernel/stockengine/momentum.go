package stockengine

import (
	"math"

	"nofx/market/usstock"
)

const (
	momentumMinBars  = 253 // close[n-253] is the 12-month anchor
	momentumSMAWin   = 200
	momentumSkipBars = 21 // the most recent month is skipped (12-1 momentum)
)

// MomentumAssessment is the trend-filtered 12-1 momentum read of one symbol
// (validated 2026-10-15: docs/research/2026-10-15-trend-momentum-ic.md).
type MomentumAssessment struct {
	Valid       bool    `json:"valid"`        // ≥ 253 daily bars
	Mom12_1Pct  float64 `json:"mom12_1_pct"`  // (close[t−21]/close[t−252] − 1)×100
	AboveSMA200 bool    `json:"above_sma200"` // close[t] > simple mean of the last 200 closes
}

// ClassifyMomentum reads bars (ascending, closed). With n bars the momentum is
// close[n−22]/close[n−253] − 1 and the trend filter compares close[n−1] with
// the mean of close[n−200:]; this is the cmd/cycleic "sma200_mom" definition.
// Fewer than 253 bars, or a non-positive / non-finite price in the used
// window, yields Valid=false with the other fields zero.
func ClassifyMomentum(bars []usstock.Bar) MomentumAssessment {
	n := len(bars)
	if n < momentumMinBars {
		return MomentumAssessment{}
	}
	usable := func(p float64) bool { return p > 0 && !math.IsNaN(p) && !math.IsInf(p, 0) }
	var sum float64
	for _, b := range bars[n-momentumSMAWin:] {
		if !usable(b.Close) {
			return MomentumAssessment{}
		}
		sum += b.Close
	}
	last := bars[n-1].Close
	recent, anchor := bars[n-1-momentumSkipBars].Close, bars[n-momentumMinBars].Close
	if !usable(recent) || !usable(anchor) {
		return MomentumAssessment{}
	}
	return MomentumAssessment{
		Valid:       true,
		Mom12_1Pct:  (recent/anchor - 1) * 100,
		AboveSMA200: last > sum/momentumSMAWin,
	}
}

package market

import (
	"fmt"
	"math"
	"strings"
)

// CandidateQuality holds per-dimension percentile scores for a candidate coin.
// All score fields are in [0, 1] where 1 means highest rank within the cohort.
type CandidateQuality struct {
	Passed       bool     `json:"passed"`
	Reasons      []string `json:"reasons,omitempty"`
	Liquidity    float64  `json:"liquidity_score,omitempty"`
	OpenInterest float64  `json:"open_interest_score,omitempty"`
	Activity     float64  `json:"activity_score,omitempty"`
	Momentum     float64  `json:"momentum_score,omitempty"`
	OIGrowth     float64  `json:"oi_growth_score,omitempty"`
	FundingEdge  float64  `json:"funding_edge_score,omitempty"`
	Reliability  float64  `json:"reliability_score,omitempty"`
	Tradability  float64  `json:"tradability_score,omitempty"`
	RiskPenalty  float64  `json:"risk_penalty,omitempty"`
}

// candidateInput holds raw metrics for a single candidate used in batch scoring.
type candidateInput struct {
	symbol      string
	volumeUSD   float64
	oiUSD       float64
	absChgPct   float64 // abs(priceChangePct)
	activity    float64 // vol/oi*100
	oiGrowthPct float64 // OI delta% vs previous snapshot; math.NaN() if unavailable
	fundingRate float64 // raw funding rate; math.NaN() if unavailable
}

// percentileRank returns the fraction of other values strictly less than values[index].
// Result is in [0, 1]; returns 0.5 when the slice has <= 1 element.
func percentileRank(values []float64, index int) float64 {
	if len(values) <= 1 {
		return 0.5
	}
	count := 0
	for i, v := range values {
		if i != index && v < values[index] {
			count++
		}
	}
	return float64(count) / float64(len(values)-1)
}

// scoreCandidatesPercentile performs batch percentile-based scoring for a slice of
// candidate inputs. It returns one CandidateQuality per input in the same order.
// Hard-filter violations (low_volume, low_open_interest, extreme_price_change) are
// still applied before percentile scoring so disqualified coins do not distort the
// distribution; they are excluded from the cohort that feeds percentile math.
func scoreCandidatesPercentile(inputs []candidateInput) []CandidateQuality {
	n := len(inputs)
	results := make([]CandidateQuality, n)

	if n == 0 {
		return results
	}

	// --- Pass 1: apply hard filters, mark failures ---
	for i, inp := range inputs {
		q := &results[i]
		q.Passed = true
		if inp.volumeUSD < hotCoinMinVolume*0.5 {
			q.Passed = false
			q.Reasons = append(q.Reasons, "low_volume")
		}
		if inp.oiUSD < hotCoinMinOI*0.5 {
			q.Passed = false
			q.Reasons = append(q.Reasons, "low_open_interest")
		}
		if inp.absChgPct > hotCoinMaxPriceChg {
			q.Passed = false
			q.Reasons = append(q.Reasons, "extreme_price_change")
		}
	}

	// --- Pass 2: collect dimension slices from *passing* candidates only ---
	// We build index maps so that percentile rank only uses the valid cohort.
	passingIdx := make([]int, 0, n)
	for i := range inputs {
		if results[i].Passed {
			passingIdx = append(passingIdx, i)
		}
	}
	m := len(passingIdx)
	if m == 0 {
		return results
	}

	vols := make([]float64, m)
	ois := make([]float64, m)
	chgs := make([]float64, m)
	acts := make([]float64, m)

	// For optional dimensions we fall back to 0.5 when all values are NaN.
	oiGrowths := make([]float64, m)
	oiGrowthValid := make([]bool, m)
	fundingEdges := make([]float64, m) // abs(funding); lower = less crowded = better
	fundingEdgeValid := make([]bool, m)

	for j, gi := range passingIdx {
		inp := inputs[gi]
		vols[j] = inp.volumeUSD
		ois[j] = inp.oiUSD
		chgs[j] = inp.absChgPct
		acts[j] = inp.activity

		if !math.IsNaN(inp.oiGrowthPct) {
			oiGrowths[j] = inp.oiGrowthPct
			oiGrowthValid[j] = true
		}
		if !math.IsNaN(inp.fundingRate) {
			fundingEdges[j] = math.Abs(inp.fundingRate)
			fundingEdgeValid[j] = true
		}
	}

	// Check whether optional dimensions have at least one valid value.
	hasOIGrowth := false
	hasFunding := false
	for j := range passingIdx {
		if oiGrowthValid[j] {
			hasOIGrowth = true
		}
		if fundingEdgeValid[j] {
			hasFunding = true
		}
	}

	// --- Pass 3: score each passing candidate using percentile rank ---
	for j, gi := range passingIdx {
		q := &results[gi]
		inp := inputs[gi]

		q.Liquidity = percentileRank(vols, j)
		q.OpenInterest = percentileRank(ois, j)
		q.Momentum = percentileRank(chgs, j) // higher abs change = higher momentum rank
		q.Activity = percentileRank(acts, j)

		// OI Growth: percentile of oiGrowthPct (higher growth = better)
		if hasOIGrowth && oiGrowthValid[j] {
			q.OIGrowth = percentileRank(oiGrowths, j)
		} else {
			q.OIGrowth = 0.5 // neutral when unavailable
		}

		// Funding Edge: inverted percentile of abs(funding) — lower funding = less crowded = higher score
		if hasFunding && fundingEdgeValid[j] {
			q.FundingEdge = 1.0 - percentileRank(fundingEdges, j)
		} else {
			q.FundingEdge = 0.5 // neutral when unavailable
		}

		// Risk penalty based on abs price change relative to max threshold.
		q.RiskPenalty = clamp01(math.Max(0, (inp.absChgPct-12)/(hotCoinMaxPriceChg-12)))

		// Reliability: stability of liquidity + OI, penalised by extreme moves.
		q.Reliability = clamp01(0.45*q.Liquidity + 0.35*q.OpenInterest + 0.20*(1-math.Min(inp.absChgPct/hotCoinMaxPriceChg, 1)))

		// Tradability composite.
		q.Tradability = clamp01(
			0.30*q.Liquidity +
				0.22*q.OpenInterest +
				0.18*q.Activity +
				0.13*q.Reliability +
				0.10*q.OIGrowth +
				0.07*q.FundingEdge +
				0.05*q.Momentum -
				0.20*q.RiskPenalty,
		)
	}

	return results
}

// scoreCandidateQuality is retained for backward compatibility with callers that
// still supply precomputed max values. It converts to percentile by treating the
// single candidate against a synthetic 2-element cohort [0, value] (equivalent to
// value/maxValue but expressed as a rank). Where max==0 it returns 0.5.
// Prefer scoreCandidatesPercentile for batch use.
func scoreCandidateQuality(volumeUSD, oiUSD, priceChangePct, activityProxy, maxVol, maxOI, maxChg, maxActivity float64) CandidateQuality {
	absChg := math.Abs(priceChangePct)
	inp := candidateInput{
		volumeUSD:   volumeUSD,
		oiUSD:       oiUSD,
		absChgPct:   absChg,
		activity:    activityProxy,
		oiGrowthPct: math.NaN(),
		fundingRate: math.NaN(),
	}
	// Build a synthetic 2-element cohort so percentileRank gives value/max behaviour.
	// The "max" element represents the best observed value in the batch.
	synth := []candidateInput{
		inp,
		{
			volumeUSD:   maxVol,
			oiUSD:       maxOI,
			absChgPct:   maxChg,
			activity:    maxActivity,
			oiGrowthPct: math.NaN(),
			fundingRate: math.NaN(),
		},
	}
	// Hard-filter the synthetic "max" entry so it is always passing.
	// Mark it as passing by using values that pass the hard filter.
	qs := scoreCandidatesPercentile(synth)
	return qs[0]
}

// compositeHotScore computes the final hot-list score from a CandidateQuality.
// Weights: Tradability 45%, Liquidity 20%, Activity 13%, OIGrowth 10%, Momentum 7%, FundingEdge 5%.
func compositeHotScore(q CandidateQuality) float64 {
	return clamp01(
		0.45*q.Tradability +
			0.20*q.Liquidity +
			0.13*q.Activity +
			0.10*q.OIGrowth +
			0.07*q.Momentum +
			0.05*q.FundingEdge,
	)
}

// compositeOIRankScore scores a coin for the OI-ranking list (ascending/descending).
func compositeOIRankScore(q CandidateQuality, oiActivity float64, maxOIActivity float64, directionSign float64) float64 {
	// Convert oiActivity to a percentile against the single max reference.
	activity := clamp01(safeNorm(oiActivity, maxOIActivity))
	base := clamp01(0.50*activity + 0.25*q.Tradability + 0.15*q.OpenInterest + 0.10*q.Liquidity)
	if directionSign < 0 {
		return -base
	}
	return base
}

// qualityLogLine formats a one-line percentile summary for a coin, e.g.:
// "📊 Coin ranking: SOLUSDT [Liq=P92 OI=P88 Act=P75 Mom=P60 OIG=P95 Fund=P70] composite=0.82"
func qualityLogLine(symbol string, q CandidateQuality, composite float64) string {
	pct := func(v float64) string {
		return fmt.Sprintf("P%d", int(math.Round(v*100)))
	}
	parts := []string{
		"Liq=" + pct(q.Liquidity),
		"OI=" + pct(q.OpenInterest),
		"Act=" + pct(q.Activity),
		"Mom=" + pct(q.Momentum),
		"OIG=" + pct(q.OIGrowth),
		"Fund=" + pct(q.FundingEdge),
	}
	return fmt.Sprintf("Coin ranking: %s [%s] composite=%.2f",
		symbol, strings.Join(parts, " "), composite)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func scoreOIDeltaCandidate(deltaPct float64, q CandidateQuality) float64 {
	// OI delta is the primary signal. Tradability only adjusts whether the move is worth trading.
	deltaSignal := clamp01(math.Abs(deltaPct) / 2.0) // 2%+ over the local window is already very meaningful.
	score := 0.70*deltaSignal + 0.20*q.Tradability + 0.10*q.OpenInterest
	if deltaPct < 0 {
		return -score
	}
	return score
}

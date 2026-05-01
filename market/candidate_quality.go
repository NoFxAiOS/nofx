package market

import "math"

type CandidateQuality struct {
	Passed       bool     `json:"passed"`
	Reasons      []string `json:"reasons,omitempty"`
	Liquidity    float64  `json:"liquidity_score,omitempty"`
	OpenInterest float64  `json:"open_interest_score,omitempty"`
	Activity     float64  `json:"activity_score,omitempty"`
	Momentum     float64  `json:"momentum_score,omitempty"`
	Reliability  float64  `json:"reliability_score,omitempty"`
	Tradability  float64  `json:"tradability_score,omitempty"`
	RiskPenalty  float64  `json:"risk_penalty,omitempty"`
}

func scoreCandidateQuality(volumeUSD, oiUSD, priceChangePct, activityProxy, maxVol, maxOI, maxChg, maxActivity float64) CandidateQuality {
	q := CandidateQuality{Passed: true}
	absChg := math.Abs(priceChangePct)
	if volumeUSD < hotCoinMinVolume*0.5 {
		q.Passed = false
		q.Reasons = append(q.Reasons, "low_volume")
	}
	if oiUSD < hotCoinMinOI*0.5 {
		q.Passed = false
		q.Reasons = append(q.Reasons, "low_open_interest")
	}
	if absChg > hotCoinMaxPriceChg {
		q.Passed = false
		q.Reasons = append(q.Reasons, "extreme_price_change")
	}
	q.Liquidity = clamp01(safeNorm(volumeUSD, maxVol))
	q.OpenInterest = clamp01(safeNorm(oiUSD, maxOI))
	q.Momentum = clamp01(safeNorm(absChg, maxChg))
	q.Activity = clamp01(safeNorm(activityProxy, maxActivity))
	q.RiskPenalty = clamp01(math.Max(0, (absChg-12)/(hotCoinMaxPriceChg-12)))
	q.Reliability = clamp01(0.45*q.Liquidity + 0.35*q.OpenInterest + 0.20*(1-math.Min(absChg/hotCoinMaxPriceChg, 1)))
	q.Tradability = clamp01(0.35*q.Liquidity + 0.25*q.OpenInterest + 0.20*q.Activity + 0.15*q.Reliability + 0.05*q.Momentum - 0.20*q.RiskPenalty)
	return q
}

func compositeHotScore(q CandidateQuality) float64 {
	// Hot list should surface tradable opportunity, not merely noisy movers.
	return clamp01(0.50*q.Tradability + 0.20*q.Liquidity + 0.15*q.Activity + 0.10*q.Momentum + 0.05*q.OpenInterest)
}

func compositeOIRankScore(q CandidateQuality, oiActivity float64, maxOIActivity float64, directionSign float64) float64 {
	activity := clamp01(safeNorm(oiActivity, maxOIActivity))
	base := clamp01(0.50*activity + 0.25*q.Tradability + 0.15*q.OpenInterest + 0.10*q.Liquidity)
	if directionSign < 0 {
		return -base
	}
	return base
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

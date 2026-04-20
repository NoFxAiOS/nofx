package kernel

import "math"

// recomputeRiskRewardWithExecutionConstraints returns a compact, deterministic
// RR recomputation using only execution constraints already present in the AI
// rationale. It never fetches live exchange data.
func recomputeRiskRewardWithExecutionConstraints(action string, rr AIRiskRewardRationale, c AIEntryExecutionConstraints) (grossRR, netRR float64, ok bool) {
	return RuntimeRecomputeRiskRewardWithExecutionConstraints(action, rr, c)
}

// RuntimeRecomputeRiskRewardWithExecutionConstraints is the runtime-facing form
// of the compact RR recomputation used by kernel validation. It is deterministic
// and never fetches live exchange data.
func RuntimeRecomputeRiskRewardWithExecutionConstraints(action string, rr AIRiskRewardRationale, c AIEntryExecutionConstraints) (grossRR, netRR float64, ok bool) {
	entry := roundPriceForRR(rr.Entry, c)
	invalidation := roundPriceForRR(rr.Invalidation, c)
	firstTarget := roundPriceForRR(rr.FirstTarget, c)
	if entry <= 0 || invalidation <= 0 || firstTarget <= 0 {
		return 0, 0, false
	}

	riskDistance := absFloat(entry - invalidation)
	rewardDistance := absFloat(firstTarget - entry)
	if riskDistance <= 0 || rewardDistance <= 0 {
		return 0, 0, false
	}
	grossRR = rewardDistance / riskDistance
	netRR = grossRR

	costBps := 0.0
	if c.EstimatedSlippageBps > 0 {
		// Entry plus exit estimate; intentionally simple and auditable.
		costBps += c.EstimatedSlippageBps * 2
	}
	feeRate := c.TakerFeeRate
	if feeRate <= 0 {
		feeRate = c.MakerFeeRate
	}
	if feeRate > 0 {
		costBps += feeRate * 2 * 10000
	}
	if costBps > 0 {
		costDistance := entry * costBps / 10000
		netReward := rewardDistance - costDistance
		if netReward < 0 {
			netReward = 0
		}
		netRR = netReward / riskDistance
	}

	if math.IsNaN(grossRR) || math.IsInf(grossRR, 0) || math.IsNaN(netRR) || math.IsInf(netRR, 0) {
		return 0, 0, false
	}
	_ = action // action is retained for future side-aware rounding without widening behavior now.
	return grossRR, netRR, true
}

func roundPriceForRR(price float64, c AIEntryExecutionConstraints) float64 {
	if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
		return 0
	}
	if c.TickSize > 0 && !math.IsNaN(c.TickSize) && !math.IsInf(c.TickSize, 0) {
		return math.Round(price/c.TickSize) * c.TickSize
	}
	if c.PricePrecision > 0 && c.PricePrecision <= 12 {
		factor := math.Pow10(c.PricePrecision)
		return math.Round(price*factor) / factor
	}
	return price
}

func hasRiskRewardExecutionConstraints(c AIEntryExecutionConstraints) bool {
	return c.TickSize > 0 || c.PricePrecision > 0 || c.TakerFeeRate > 0 || c.MakerFeeRate > 0 || c.EstimatedSlippageBps > 0
}

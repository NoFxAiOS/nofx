package trader

import "nofx/store"

type ExecutionQualityContext struct {
	Grade                string  `json:"grade,omitempty"`
	SpreadBps            float64 `json:"spread_bps,omitempty"`
	EstimatedSlippageBps float64 `json:"estimated_slippage_bps,omitempty"`
	MinOrderNotionalUSDT float64 `json:"min_order_notional_usdt,omitempty"`
	PartialCloseFeasible bool    `json:"partial_close_feasible,omitempty"`
	LadderTiersFeasible  int     `json:"ladder_tiers_feasible,omitempty"`
	Reason               string  `json:"reason,omitempty"`
}

func buildExecutionQualityContext(snapshot *ExecutionConstraintsSnapshot, plannedPositionUSD float64, plannedLadderTiers int) *ExecutionQualityContext {
	if snapshot == nil {
		return nil
	}
	ctx := &ExecutionQualityContext{
		SpreadBps:            snapshot.SpreadBps,
		EstimatedSlippageBps: snapshot.EstimatedSlippageBps,
		Grade:                "unknown",
	}
	minNotional := snapshot.MinNotional
	if minNotional <= 0 && snapshot.MinQty > 0 {
		price := firstPositive(snapshot.LastPrice, snapshot.MarkPrice, snapshot.BestAsk, snapshot.BestBid)
		if price > 0 {
			minNotional = snapshot.MinQty * price
			if snapshot.ContractValue > 0 {
				minNotional *= snapshot.ContractValue
			}
		}
	}
	ctx.MinOrderNotionalUSDT = minNotional
	if plannedLadderTiers <= 0 {
		plannedLadderTiers = 1
	}
	if minNotional > 0 && plannedPositionUSD > 0 {
		ctx.LadderTiersFeasible = int(plannedPositionUSD / minNotional)
		if ctx.LadderTiersFeasible > plannedLadderTiers {
			ctx.LadderTiersFeasible = plannedLadderTiers
		}
		ctx.PartialCloseFeasible = ctx.LadderTiersFeasible >= 2
	}
	switch {
	case snapshot.SpreadBps > 20 || snapshot.EstimatedSlippageBps > 30:
		ctx.Grade = "D"
		ctx.Reason = "wide spread/slippage"
	case snapshot.SpreadBps > 10 || snapshot.EstimatedSlippageBps > 15:
		ctx.Grade = "C"
		ctx.Reason = "moderate execution cost"
	case snapshot.SpreadBps > 0 || snapshot.EstimatedSlippageBps > 0 || minNotional > 0:
		ctx.Grade = "B"
		ctx.Reason = "execution data available"
	default:
		ctx.Grade = "unknown"
		ctx.Reason = "insufficient execution data"
	}
	if minNotional > 0 && plannedPositionUSD > 0 && plannedPositionUSD < minNotional*2 {
		ctx.Grade = worseGrade(ctx.Grade, "C")
		ctx.Reason = "position too small for reliable partial protection"
	}
	return ctx
}

func attachExecutionQualityToReview(review *store.DecisionActionReviewContext, ctx *ExecutionQualityContext) {
	if review == nil || ctx == nil {
		return
	}
	if review.Extra == nil {
		review.Extra = map[string]interface{}{}
	}
	review.Extra["execution_quality"] = ctx
}

func worseGrade(a, b string) string {
	rank := map[string]int{"A": 1, "B": 2, "C": 3, "D": 4, "unknown": 0, "": 0}
	if rank[b] > rank[a] {
		return b
	}
	return a
}

func firstPositive(values ...float64) float64 {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

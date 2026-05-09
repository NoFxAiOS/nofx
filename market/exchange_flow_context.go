package market

import "fmt"

func BuildExchangeFlowContext(data *Data) *ExchangeFlowContext {
	if data == nil {
		return &ExchangeFlowContext{DataQuality: "missing"}
	}
	ctx := &ExchangeFlowContext{DataQuality: "partial"}
	ctx.FundingBias = classifyFundingBias(data.FundingRate)
	if data.LongShortRatio != nil {
		ctx.LongShortSkew = classifyRatioSkew(*data.LongShortRatio, "long_crowded", "short_crowded")
	}
	if data.TakerBuySellRatio != nil {
		ctx.TakerFlowBias = classifyRatioSkew(*data.TakerBuySellRatio, "aggressive_buy", "aggressive_sell")
	}
	if data.DepthBidTotal != nil && data.DepthAskTotal != nil {
		ctx.DepthTotalUSDT = *data.DepthBidTotal + *data.DepthAskTotal
	}
	if data.DepthImbalance != nil {
		ctx.DepthImbalance = *data.DepthImbalance
		switch {
		case ctx.DepthImbalance >= 0.2:
			ctx.DepthBias = "bid_heavy"
		case ctx.DepthImbalance <= -0.2:
			ctx.DepthBias = "ask_heavy"
		default:
			ctx.DepthBias = "balanced"
		}
	}
	ctx.CrowdingRisk = classifyExchangeCrowding(ctx)
	ctx.Interpretation = fmt.Sprintf("funding=%s long_short=%s taker=%s depth=%s crowding=%s", ctx.FundingBias, ctx.LongShortSkew, ctx.TakerFlowBias, ctx.DepthBias, ctx.CrowdingRisk)
	if ctx.FundingBias != "unknown" || ctx.LongShortSkew != "" || ctx.TakerFlowBias != "" || ctx.DepthBias != "" {
		ctx.DataQuality = "ok"
	}
	return ctx
}

func classifyRatioSkew(v float64, highLabel, lowLabel string) string {
	switch {
	case v >= 1.2:
		return highLabel
	case v <= 0.8 && v > 0:
		return lowLabel
	case v == 0:
		return "unknown"
	default:
		return "neutral"
	}
}

func classifyExchangeCrowding(ctx *ExchangeFlowContext) string {
	if ctx == nil {
		return "unknown"
	}
	score := 0
	for _, v := range []string{ctx.FundingBias, ctx.LongShortSkew, ctx.TakerFlowBias} {
		switch v {
		case "long_crowded", "short_crowded", "aggressive_buy", "aggressive_sell":
			score++
		}
	}
	if (ctx.LongShortSkew == "long_crowded" && ctx.TakerFlowBias == "aggressive_buy") || (ctx.LongShortSkew == "short_crowded" && ctx.TakerFlowBias == "aggressive_sell") {
		score++
	}
	switch {
	case score >= 3:
		return "high"
	case score >= 2:
		return "medium"
	case score == 0 && ctx.DataQuality == "partial":
		return "unknown"
	default:
		return "low"
	}
}

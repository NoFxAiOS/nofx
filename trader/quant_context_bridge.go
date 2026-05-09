package trader

import (
	"nofx/kernel"
	"nofx/market"
)

func attachQuantContextToMarketData(marketData map[string]*market.Data, quantData map[string]*kernel.QuantData) {
	if len(marketData) == 0 || len(quantData) == 0 {
		return
	}
	for symbol, q := range quantData {
		m := marketData[symbol]
		if m == nil || q == nil {
			continue
		}
		m.QuantContext = buildMarketQuantContextFromKernel(q)
	}
}

func buildMarketQuantContextFromKernel(q *kernel.QuantData) *market.QuantContext {
	if q == nil {
		return nil
	}
	ctx := &market.QuantContext{DataQuality: "partial"}
	if q.Netflow != nil {
		if q.Netflow.Institution != nil {
			ctx.InstitutionFuture1h = flowAtKernel(q.Netflow.Institution.Future, "1h")
			ctx.InstitutionFuture4h = flowAtKernel(q.Netflow.Institution.Future, "4h")
			ctx.InstitutionSpot1h = flowAtKernel(q.Netflow.Institution.Spot, "1h")
		}
		if q.Netflow.Personal != nil {
			ctx.RetailFuture1h = flowAtKernel(q.Netflow.Personal.Future, "1h")
		}
	}
	for _, oi := range q.OI {
		if oi == nil || oi.Delta == nil {
			continue
		}
		if d := oi.Delta["1h"]; d != nil && absFloatTrader(d.OIDeltaPercent) > absFloatTrader(ctx.OIChange1hPct) {
			ctx.OIChange1hPct = d.OIDeltaPercent
			ctx.OIChange1hValue = d.OIDeltaValue
		}
		if d := oi.Delta["4h"]; d != nil && absFloatTrader(d.OIDeltaPercent) > absFloatTrader(ctx.OIChange4hPct) {
			ctx.OIChange4hPct = d.OIDeltaPercent
		}
	}
	ctx.FlowBias = classifyMarketFlowBias(ctx.InstitutionFuture1h, ctx.InstitutionSpot1h, ctx.RetailFuture1h)
	ctx.CrowdingRisk = classifyMarketCrowding(ctx.OIChange1hPct, ctx.InstitutionFuture1h, ctx.RetailFuture1h)
	ctx.Interpretation = ctx.FlowBias
	if ctx.FlowBias != "unknown" || ctx.OIChange1hPct != 0 || ctx.InstitutionFuture1h != 0 || ctx.InstitutionSpot1h != 0 {
		ctx.DataQuality = "ok"
	}
	return ctx
}

func flowAtKernel(m map[string]float64, key string) float64 {
	if m == nil {
		return 0
	}
	return m[key]
}

func classifyMarketFlowBias(instFuture1h, instSpot1h, retailFuture1h float64) string {
	combined := instFuture1h + instSpot1h*0.7
	switch {
	case combined > 0 && retailFuture1h < 0:
		return "institution_accumulation_retail_fade"
	case combined > 0:
		return "institution_inflow"
	case combined < 0 && retailFuture1h > 0:
		return "institution_distribution_retail_chase"
	case combined < 0:
		return "institution_outflow"
	default:
		return "unknown"
	}
}

func classifyMarketCrowding(oiChange1hPct, instFuture1h, retailFuture1h float64) string {
	absOI := absFloatTrader(oiChange1hPct)
	sameDirectionFlow := (instFuture1h > 0 && retailFuture1h > 0) || (instFuture1h < 0 && retailFuture1h < 0)
	switch {
	case absOI >= 8 || (absOI >= 4 && sameDirectionFlow):
		return "high"
	case absOI >= 3 || sameDirectionFlow:
		return "medium"
	case absOI == 0 && instFuture1h == 0 && retailFuture1h == 0:
		return "unknown"
	default:
		return "low"
	}
}

func absFloatTrader(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

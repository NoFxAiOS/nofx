package market

import (
	"fmt"

	"nofx/provider/nofxos"
)

func BuildQuantContextFromNofxOS(data *nofxos.QuantData) *QuantContext {
	if data == nil {
		return nil
	}
	ctx := &QuantContext{DataQuality: "partial"}
	if data.Netflow != nil {
		if data.Netflow.Institution != nil {
			ctx.InstitutionFuture1h = flowAt(data.Netflow.Institution.Future, "1h")
			ctx.InstitutionFuture4h = flowAt(data.Netflow.Institution.Future, "4h")
			ctx.InstitutionSpot1h = flowAt(data.Netflow.Institution.Spot, "1h")
		}
		if data.Netflow.Personal != nil {
			ctx.RetailFuture1h = flowAt(data.Netflow.Personal.Future, "1h")
		}
	}
	for _, oi := range data.OI {
		if oi == nil || oi.Delta == nil {
			continue
		}
		if d := oi.Delta["1h"]; d != nil {
			if absFloat64(d.OIDeltaPercent) > absFloat64(ctx.OIChange1hPct) {
				ctx.OIChange1hPct = d.OIDeltaPercent
				ctx.OIChange1hValue = d.OIDeltaValue
			}
		}
		if d := oi.Delta["4h"]; d != nil && absFloat64(d.OIDeltaPercent) > absFloat64(ctx.OIChange4hPct) {
			ctx.OIChange4hPct = d.OIDeltaPercent
		}
	}
	ctx.FlowBias = classifyQuantFlowBias(ctx.InstitutionFuture1h, ctx.InstitutionSpot1h, ctx.RetailFuture1h)
	ctx.CrowdingRisk = classifyQuantCrowding(ctx.OIChange1hPct, ctx.InstitutionFuture1h, ctx.RetailFuture1h)
	ctx.Interpretation = explainQuantContext(ctx)
	if ctx.FlowBias != "unknown" || ctx.OIChange1hPct != 0 || ctx.InstitutionFuture1h != 0 || ctx.InstitutionSpot1h != 0 {
		ctx.DataQuality = "ok"
	}
	return ctx
}

func flowAt(m map[string]float64, k string) float64 {
	if m == nil {
		return 0
	}
	return m[k]
}

func classifyQuantFlowBias(instFuture1h, instSpot1h, retailFuture1h float64) string {
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

func classifyQuantCrowding(oiChange1hPct, instFuture1h, retailFuture1h float64) string {
	absOI := absFloat64(oiChange1hPct)
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

func explainQuantContext(ctx *QuantContext) string {
	if ctx == nil {
		return ""
	}
	return fmt.Sprintf("flow_bias=%s crowding=%s oi_1h=%.2f%% inst_future_1h=%.0f retail_future_1h=%.0f", ctx.FlowBias, ctx.CrowdingRisk, ctx.OIChange1hPct, ctx.InstitutionFuture1h, ctx.RetailFuture1h)
}

func absFloat64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

package trader

import "strings"

func reconcileResultForUnmaterializedPlan(openOrders []OpenOrder, positionSide string, protectionConfigured bool) protectionReconcileResult {
	if !protectionConfigured {
		return protectionReconcileResult{Summary: "no protection configured"}
	}
	hasStop := hasAnyProtectionOrder(openOrders, positionSide, false)
	hasProfit := hasAnyProtectionOrder(openOrders, positionSide, true)
	result := protectionReconcileResult{ExchangeVerified: hasStop, Summary: "configured protection plan not materialized"}
	switch {
	case hasStop && hasProfit:
		result.Summary = "degraded_exchange_stop_and_profit_present_without_materialized_plan"
	case hasStop:
		result.Summary = "degraded_exchange_stop_present_without_materialized_plan"
	case hasProfit:
		result.Summary = "exchange_profit_present_but_stop_missing_without_materialized_plan"
	default:
		result.Summary = "configured protection plan not materialized and no exchange stop present"
	}
	return result
}

func hasProtectionConfiguredForReconcile(configured bool, summary string) bool {
	return configured || strings.TrimSpace(summary) != ""
}

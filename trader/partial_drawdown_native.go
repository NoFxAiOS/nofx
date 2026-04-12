package trader

import "nofx/store"

// buildManagedPartialDrawdownPlanCandidate converts a partial drawdown rule into a managed
// protection plan representation. This is NOT a native trailing order: it precomputes a fixed
// trigger/take-profit price from the drawdown rule and places a standard TP-style protection order.
func buildManagedPartialDrawdownPlanCandidate(entryPrice float64, action string, rule store.DrawdownTakeProfitRule) *ProtectionPlan {
	if entryPrice <= 0 || rule.MinProfitPct <= 0 || rule.MaxDrawdownPct <= 0 {
		return nil
	}
	if rule.CloseRatioPct <= 0 || rule.CloseRatioPct >= 99.999 {
		return nil
	}

	isLong := action == "open_long"
	isShort := action == "open_short"
	if !isLong && !isShort {
		return nil
	}

	peakMove := rule.MinProfitPct / 100.0
	drawdownMove := rule.MaxDrawdownPct / 100.0
	price := entryPrice

	if isLong {
		price = entryPrice * (1 + peakMove) * (1 - drawdownMove)
	} else {
		price = entryPrice * (1 - peakMove) * (1 + drawdownMove)
	}

	if price <= 0 {
		return nil
	}

	return &ProtectionPlan{
		Mode:                 "drawdown_partial_managed",
		NeedsTakeProfit:      true,
		TakeProfitPrice:      price,
		TakeProfitOrders:     []ProtectionOrder{{Price: price, CloseRatioPct: rule.CloseRatioPct}},
		RequiresNativeOrders: true,
		RequiresPartialClose: true,
	}
}

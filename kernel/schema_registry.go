package kernel

// Canonical schema alias registry for open-action AI decision fields.
// Keep this file as the single source of truth for field-name compatibility.

var decisionAliasRegistry = map[string][]string{
	// key_levels
	"key_levels.support":                      {"support_levels"},
	"key_levels.resistance":                   {"resistance_levels"},
	"key_levels.fibonacci.levels":             {"fib_levels", "fibonacci_levels"},
	"key_levels.fibonacci.swing_high":         {"swing_high"},
	"key_levels.fibonacci.swing_low":          {"swing_low"},

	// volatility_adjustment
	"volatility_adjustment.atr14_pct":         {"atr_pct", "atr14"},
	"volatility_adjustment.boll_width_pct":    {"bollinger_width_pct"},
	"volatility_adjustment.market_regime":     {"regime"},
	"volatility_adjustment.widening_pct":      {"buffer_pct"},

	// risk_reward
	"risk_reward.entry":                       {"entry_price"},
	"risk_reward.invalidation":                {"invalidation_price"},
	"risk_reward.first_target":                {"first_target_price"},
	"risk_reward.gross_estimated_rr":          {"gross_rr"},
	"risk_reward.net_estimated_rr":            {"net_rr"},
	"risk_reward.min_required_rr":             {"min_rr"},

	// execution_constraints
	"execution_constraints.best_bid":          {"bid"},
	"execution_constraints.best_ask":          {"ask"},
	"execution_constraints.estimated_slippage_bps": {"slippage_bps"},
	"execution_constraints.tick_size":         {"price_step"},
	"execution_constraints.qty_step_size":     {"quantity_step_size"},

	// derivatives_context
	"derivatives_context.oi_current":          {"open_interest"},
	"derivatives_context.funding_rate_current": {"funding_rate"},
	"derivatives_context.mark_index_basis_bps": {"basis_bps"},
	"derivatives_context.orderbook_imbalance": {"depth_imbalance"},
	"derivatives_context.top5_bid_notional":   {"bid_notional_top5"},
	"derivatives_context.top5_ask_notional":   {"ask_notional_top5"},

	// protection_plan break-even
	"protection_plan.break_even_trigger_mode": {"breakeven_trigger"},
	"protection_plan.break_even_trigger_value": {"break_even_value", "breakeven_value"},
	"protection_plan.break_even_offset_pct":   {"break_even_offset", "breakeven_offset_pct"},
	"protection_plan.break_even_reason_anchor": {"break_even_reason", "breakeven_reason_anchor"},

	// drawdown_rules
	"drawdown_rules.close_ratio_pct":          {"close_ratio"},

	// ladder_rules
	"ladder_rules.take_profit_pct":            {"tp_pct", "tp_level"},
	"ladder_rules.stop_loss_pct":              {"sl_pct", "sl_level"},
	"ladder_rules.take_profit_close_ratio_pct": {"tp_close_ratio_pct"},
	"ladder_rules.stop_loss_close_ratio_pct":  {"sl_close_ratio_pct"},
}

func schemaAliases(canonical string) []string {
	return decisionAliasRegistry[canonical]
}

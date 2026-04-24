package kernel

// Canonical schema registry for open-action AI decision fields.
// This is the single source of truth for field compatibility metadata.

type SchemaFieldMeta struct {
	Canonical      string
	Aliases        []string
	Required       bool
	AutoFill       bool
	AutoFillSource []string
	RepairPolicy   string // e.g. alias_only, alias_then_autofill, strict_required
}

var decisionSchemaRegistry = map[string]SchemaFieldMeta{
	// key_levels
	"key_levels.support": {
		Canonical:      "key_levels.support",
		Aliases:        []string{"support_levels"},
		Required:       true,
		AutoFill:       true,
		AutoFillSource: []string{"structural_key_levels[type=support]", "anchors[type contains support]"},
		RepairPolicy:   "alias_then_autofill",
	},
	"key_levels.resistance": {
		Canonical:      "key_levels.resistance",
		Aliases:        []string{"resistance_levels"},
		Required:       true,
		AutoFill:       true,
		AutoFillSource: []string{"structural_key_levels[type=resistance]", "anchors[type contains resistance]"},
		RepairPolicy:   "alias_then_autofill",
	},
	"key_levels.fibonacci.levels": {
		Canonical:      "key_levels.fibonacci.levels",
		Aliases:        []string{"fib_levels", "fibonacci_levels"},
		Required:       false,
		AutoFill:       false,
		RepairPolicy:   "alias_only",
	},
	"key_levels.fibonacci.swing_high": {
		Canonical:    "key_levels.fibonacci.swing_high",
		Aliases:      []string{"swing_high"},
		Required:     false,
		RepairPolicy: "alias_only",
	},
	"key_levels.fibonacci.swing_low": {
		Canonical:    "key_levels.fibonacci.swing_low",
		Aliases:      []string{"swing_low"},
		Required:     false,
		RepairPolicy: "alias_only",
	},

	// volatility_adjustment
	"volatility_adjustment.atr14_pct": {
		Canonical:    "volatility_adjustment.atr14_pct",
		Aliases:      []string{"atr_pct", "atr14"},
		RepairPolicy: "alias_only",
	},
	"volatility_adjustment.boll_width_pct": {
		Canonical:    "volatility_adjustment.boll_width_pct",
		Aliases:      []string{"bollinger_width_pct"},
		RepairPolicy: "alias_only",
	},
	"volatility_adjustment.market_regime": {
		Canonical:    "volatility_adjustment.market_regime",
		Aliases:      []string{"regime"},
		RepairPolicy: "alias_only",
	},
	"volatility_adjustment.widening_pct": {
		Canonical:    "volatility_adjustment.widening_pct",
		Aliases:      []string{"buffer_pct"},
		RepairPolicy: "alias_only",
	},

	// risk_reward
	"risk_reward.entry": {
		Canonical:    "risk_reward.entry",
		Aliases:      []string{"entry_price"},
		Required:     true,
		RepairPolicy: "alias_only",
	},
	"risk_reward.invalidation": {
		Canonical:    "risk_reward.invalidation",
		Aliases:      []string{"invalidation_price"},
		Required:     true,
		RepairPolicy: "alias_only",
	},
	"risk_reward.first_target": {
		Canonical:    "risk_reward.first_target",
		Aliases:      []string{"first_target_price"},
		Required:     true,
		RepairPolicy: "alias_only",
	},
	"risk_reward.gross_estimated_rr": {
		Canonical:    "risk_reward.gross_estimated_rr",
		Aliases:      []string{"gross_rr"},
		Required:     true,
		RepairPolicy: "alias_only",
	},
	"risk_reward.net_estimated_rr": {
		Canonical:    "risk_reward.net_estimated_rr",
		Aliases:      []string{"net_rr"},
		Required:     false,
		RepairPolicy: "alias_only",
	},
	"risk_reward.min_required_rr": {
		Canonical:    "risk_reward.min_required_rr",
		Aliases:      []string{"min_rr"},
		Required:     false,
		RepairPolicy: "alias_only",
	},

	// execution_constraints
	"execution_constraints.best_bid": {Canonical: "execution_constraints.best_bid", Aliases: []string{"bid"}, RepairPolicy: "alias_only"},
	"execution_constraints.best_ask": {Canonical: "execution_constraints.best_ask", Aliases: []string{"ask"}, RepairPolicy: "alias_only"},
	"execution_constraints.estimated_slippage_bps": {Canonical: "execution_constraints.estimated_slippage_bps", Aliases: []string{"slippage_bps"}, RepairPolicy: "alias_only"},
	"execution_constraints.tick_size": {Canonical: "execution_constraints.tick_size", Aliases: []string{"price_step"}, RepairPolicy: "alias_only"},
	"execution_constraints.qty_step_size": {Canonical: "execution_constraints.qty_step_size", Aliases: []string{"quantity_step_size"}, RepairPolicy: "alias_only"},

	// derivatives_context
	"derivatives_context.oi_current": {Canonical: "derivatives_context.oi_current", Aliases: []string{"open_interest"}, RepairPolicy: "alias_only"},
	"derivatives_context.funding_rate_current": {Canonical: "derivatives_context.funding_rate_current", Aliases: []string{"funding_rate"}, RepairPolicy: "alias_only"},
	"derivatives_context.mark_index_basis_bps": {Canonical: "derivatives_context.mark_index_basis_bps", Aliases: []string{"basis_bps"}, RepairPolicy: "alias_only"},
	"derivatives_context.orderbook_imbalance": {Canonical: "derivatives_context.orderbook_imbalance", Aliases: []string{"depth_imbalance"}, RepairPolicy: "alias_only"},
	"derivatives_context.top5_bid_notional": {Canonical: "derivatives_context.top5_bid_notional", Aliases: []string{"bid_notional_top5"}, RepairPolicy: "alias_only"},
	"derivatives_context.top5_ask_notional": {Canonical: "derivatives_context.top5_ask_notional", Aliases: []string{"ask_notional_top5"}, RepairPolicy: "alias_only"},

	// protection_plan break-even
	"protection_plan.break_even_trigger_mode": {Canonical: "protection_plan.break_even_trigger_mode", Aliases: []string{"breakeven_trigger"}, Required: false, RepairPolicy: "alias_only"},
	"protection_plan.break_even_trigger_value": {Canonical: "protection_plan.break_even_trigger_value", Aliases: []string{"break_even_value", "breakeven_value"}, Required: false, RepairPolicy: "alias_only"},
	"protection_plan.break_even_offset_pct": {Canonical: "protection_plan.break_even_offset_pct", Aliases: []string{"break_even_offset", "breakeven_offset_pct"}, Required: false, RepairPolicy: "alias_only"},
	"protection_plan.break_even_reason_anchor": {Canonical: "protection_plan.break_even_reason_anchor", Aliases: []string{"break_even_reason", "breakeven_reason_anchor"}, Required: false, RepairPolicy: "alias_only"},

	// drawdown_rules
	"drawdown_rules.close_ratio_pct": {Canonical: "drawdown_rules.close_ratio_pct", Aliases: []string{"close_ratio"}, Required: true, RepairPolicy: "alias_only"},

	// ladder_rules
	"ladder_rules.take_profit_pct": {Canonical: "ladder_rules.take_profit_pct", Aliases: []string{"tp_pct", "tp_level"}, RepairPolicy: "alias_only"},
	"ladder_rules.stop_loss_pct": {Canonical: "ladder_rules.stop_loss_pct", Aliases: []string{"sl_pct", "sl_level"}, RepairPolicy: "alias_only"},
	"ladder_rules.take_profit_close_ratio_pct": {Canonical: "ladder_rules.take_profit_close_ratio_pct", Aliases: []string{"tp_close_ratio_pct"}, RepairPolicy: "alias_only"},
	"ladder_rules.stop_loss_close_ratio_pct": {Canonical: "ladder_rules.stop_loss_close_ratio_pct", Aliases: []string{"sl_close_ratio_pct"}, RepairPolicy: "alias_only"},
}

func schemaAliases(canonical string) []string {
	if meta, ok := decisionSchemaRegistry[canonical]; ok {
		return meta.Aliases
	}
	return nil
}

func schemaMeta(canonical string) (SchemaFieldMeta, bool) {
	meta, ok := decisionSchemaRegistry[canonical]
	return meta, ok
}

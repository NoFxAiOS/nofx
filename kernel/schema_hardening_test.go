package kernel

import (
	"encoding/json"
	"testing"

	"nofx/store"
)

func TestSchemaRegistryContainsCoreAliases(t *testing.T) {
	cases := map[string][]string{
		"drawdown_rules.close_ratio_pct": {"close_ratio"},
		"protection_plan.break_even_trigger_mode": {"breakeven_trigger"},
		"key_levels.support": {"support_levels"},
		"risk_reward.entry": {"entry_price"},
	}
	for canonical, expected := range cases {
		got := schemaAliases(canonical)
		meta, ok := schemaMeta(canonical)
		if !ok || meta.Canonical != canonical {
			t.Fatalf("expected schema meta for %s, got %+v", canonical, meta)
		}
		if len(got) == 0 {
			t.Fatalf("expected aliases for %s", canonical)
		}
		for _, want := range expected {
			found := false
			for _, v := range got {
				if v == want {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected alias %q for %s, got %#v", want, canonical, got)
			}
		}
	}
}

func TestSchemaMetadataCarriesRequiredAndAutofillPolicy(t *testing.T) {
	supportMeta, ok := schemaMeta("key_levels.support")
	if !ok || !supportMeta.Required || !supportMeta.AutoFill || supportMeta.RepairPolicy != "alias_then_autofill" {
		t.Fatalf("unexpected support schema metadata: %+v", supportMeta)
	}
	closeRatioMeta, ok := schemaMeta("drawdown_rules.close_ratio_pct")
	if !ok || !closeRatioMeta.Required || closeRatioMeta.AutoFill || closeRatioMeta.RepairPolicy != "alias_only" {
		t.Fatalf("unexpected close_ratio schema metadata: %+v", closeRatioMeta)
	}
}

func TestVolatilityAdjustmentAcceptsAliases(t *testing.T) {
	var v AIEntryVolatilityAdjustment
	if err := json.Unmarshal([]byte(`{"atr_pct":1.2,"bollinger_width_pct":3.4,"regime":"wide","buffer_pct":0.5}`), &v); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if v.ATR14Pct != 1.2 || v.BollWidthPct != 3.4 || v.MarketRegime != "wide" || v.WideningPct != 0.5 {
		t.Fatalf("unexpected volatility alias mapping: %+v", v)
	}
}

func TestExecutionConstraintsAcceptAliases(t *testing.T) {
	var e AIEntryExecutionConstraints
	if err := json.Unmarshal([]byte(`{"bid":100,"ask":101,"slippage_bps":8,"price_step":0.1,"quantity_step_size":0.001}`), &e); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if e.BestBid != 100 || e.BestAsk != 101 || e.EstimatedSlippageBps != 8 || e.TickSize != 0.1 || e.QtyStepSize != 0.001 {
		t.Fatalf("unexpected execution constraint alias mapping: %+v", e)
	}
}

func TestDerivativesContextAcceptsAliases(t *testing.T) {
	var d AIEntryDerivativesContext
	if err := json.Unmarshal([]byte(`{"open_interest":12345,"funding_rate":0.01,"basis_bps":12,"depth_imbalance":0.2,"bid_notional_top5":1000,"ask_notional_top5":900}`), &d); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if d.OICurrent != 12345 || d.FundingRateCurrent != 0.01 || d.MarkIndexBasisBps != 12 || d.OrderbookImbalance != 0.2 || d.Top5BidNotional != 1000 || d.Top5AskNotional != 900 {
		t.Fatalf("unexpected derivatives alias mapping: %+v", d)
	}
}

func TestRiskRewardAcceptsAliases(t *testing.T) {
	var rr AIRiskRewardRationale
	if err := json.Unmarshal([]byte(`{"entry_price":100,"invalidation_price":95,"first_target_price":110,"gross_rr":2.0,"net_rr":1.7,"min_rr":1.5}`), &rr); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if rr.Entry != 100 || rr.Invalidation != 95 || rr.FirstTarget != 110 || rr.GrossEstimatedRR != 2.0 || rr.NetEstimatedRR != 1.7 || rr.MinRequiredRR != 1.5 {
		t.Fatalf("unexpected risk reward mapping: %+v", rr)
	}
}

func TestLadderRuleAcceptsAliases(t *testing.T) {
	var rule AIProtectionLadderRule
	if err := json.Unmarshal([]byte(`{"tp_pct":3,"tp_close_ratio_pct":40,"sl_pct":1.5,"sl_close_ratio_pct":25}`), &rule); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if rule.TakeProfitPct != 3 || rule.TakeProfitCloseRatioPct != 40 || rule.StopLossPct != 1.5 || rule.StopLossCloseRatioPct != 25 {
		t.Fatalf("unexpected ladder alias mapping: %+v", rule)
	}
}

func TestProtectionPlanAcceptsBreakevenAliases(t *testing.T) {
	var plan AIProtectionPlan
	if err := json.Unmarshal([]byte(`{"mode":"break_even","breakeven_trigger":"price_change_pct","breakeven_value":1.2,"breakeven_offset_pct":0.15,"breakeven_reason_anchor":"15m support flip"}`), &plan); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if plan.BreakEvenTrigger != "price_change_pct" || plan.BreakEvenValue != 1.2 || plan.BreakEvenOffset != 0.15 || plan.BreakEvenAnchor == "" {
		t.Fatalf("unexpected mapped breakeven plan: %+v", plan)
	}
}

func TestEntryKeyLevelsAcceptAliases(t *testing.T) {
	var levels AIEntryKeyLevels
	if err := json.Unmarshal([]byte(`{"support_levels":[100,99],"resistance_levels":[110],"fib_levels":[0.382,0.618],"swing_high":120,"swing_low":90}`), &levels); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if len(levels.Support) != 2 || len(levels.Resistance) != 1 {
		t.Fatalf("expected support/resistance aliases to map, got %+v", levels)
	}
	if levels.Fibonacci == nil || len(levels.Fibonacci.Levels) != 2 || levels.Fibonacci.SwingHigh != 120 || levels.Fibonacci.SwingLow != 90 {
		t.Fatalf("expected fib aliases to map, got %+v", levels.Fibonacci)
	}
}

func TestEntryKeyLevelsAcceptStructuredObjects(t *testing.T) {
	var levels AIEntryKeyLevels
	if err := json.Unmarshal([]byte(`{"support":[{"timeframe":"15m","price":100,"type":"fibonacci","reason":"support"},{"timeframe":"15m","price":99,"type":"swing_low","reason":"deeper support"}],"resistance":[{"timeframe":"15m","price":110,"type":"swing_high","reason":"target"}]}`), &levels); err != nil {
		t.Fatalf("unexpected structured key_levels unmarshal error: %v", err)
	}
	if len(levels.Support) != 2 || levels.Support[0] != 100 || levels.Support[1] != 99 {
		t.Fatalf("expected structured support objects to collapse into prices, got %+v", levels.Support)
	}
	if len(levels.Resistance) != 1 || levels.Resistance[0] != 110 {
		t.Fatalf("expected structured resistance objects to collapse into prices, got %+v", levels.Resistance)
	}
}

func TestDrawdownRuleAcceptsCloseRatioAlias(t *testing.T) {
	var rule AIProtectionDrawdownRule
	if err := json.Unmarshal([]byte(`{"min_profit_pct":2.5,"max_drawdown_pct":35,"close_ratio":40}`), &rule); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if rule.CloseRatioPct != 40 {
		t.Fatalf("expected close_ratio alias to map to CloseRatioPct=40, got %.2f", rule.CloseRatioPct)
	}
}

func TestBackfillEntryProtectionKeyLevelsFromStructuralLevels(t *testing.T) {
	ep := &AIEntryProtectionRationale{
		StructuralKeyLevels: []AIStructuralKeyLevel{
			{Price: 100, Type: "support", Timeframe: "15m", Source: "auto_detected"},
			{Price: 110, Type: "resistance", Timeframe: "15m", Source: "auto_detected"},
			{Price: 99, Type: "support", Timeframe: "5m", Source: "fibonacci_0.618"},
		},
	}
	backfillEntryProtectionKeyLevels(ep)
	if len(ep.KeyLevels.Support) != 2 {
		t.Fatalf("expected 2 support levels, got %d", len(ep.KeyLevels.Support))
	}
	if len(ep.KeyLevels.Resistance) != 1 {
		t.Fatalf("expected 1 resistance level, got %d", len(ep.KeyLevels.Resistance))
	}
	if ep.KeyLevels.Support[0] != 100 || ep.KeyLevels.Support[1] != 99 {
		t.Fatalf("expected support sorted descending [100,99], got %#v", ep.KeyLevels.Support)
	}
	if ep.KeyLevels.Resistance[0] != 110 {
		t.Fatalf("expected resistance [110], got %#v", ep.KeyLevels.Resistance)
	}
}

func TestNormalizeAndRepairOpenDecisionBackfillsStructuralKeyLevels(t *testing.T) {
	decisions := []Decision{{
		Symbol: " btcusdt ",
		Action: " OPEN_LONG ",
		EntryProtection: &AIEntryProtectionRationale{
			TimeframeContext: AIEntryTimeframeContext{Primary: " 15m "},
			KeyLevels: AIEntryKeyLevels{Support: []float64{100}, Resistance: []float64{110}},
		},
	}}
	normalizeAndRepairOpenDecisions(decisions)
	if decisions[0].Action != "open_long" {
		t.Fatalf("expected normalized action open_long, got %q", decisions[0].Action)
	}
	if decisions[0].Symbol != "BTCUSDT" {
		t.Fatalf("expected normalized symbol BTCUSDT, got %q", decisions[0].Symbol)
	}
	ep := decisions[0].EntryProtection
	if ep == nil || len(ep.StructuralKeyLevels) != 2 {
		t.Fatalf("expected 2 backfilled structural key levels, got %#v", ep)
	}
	if ep.StructuralKeyLevels[0].Timeframe != "15m" {
		t.Fatalf("expected trimmed primary timeframe 15m, got %q", ep.StructuralKeyLevels[0].Timeframe)
	}
}

func TestNormalizeAndRepairOpenDecisionKeepsKeyLevelsWithinConfigCapsAfterBackfill(t *testing.T) {
	ep := &AIEntryProtectionRationale{
		TimeframeContext: AIEntryTimeframeContext{Primary: "15m", Lower: []string{"5m"}, Higher: []string{"1h"}},
		KeyLevels: AIEntryKeyLevels{
			Support:    []float64{100, 99, 98},
			Resistance: []float64{110, 111, 112, 113},
		},
		Anchors: []AIEntryProtectionAnchor{{Type: "support", Timeframe: "15m", Price: 100, Reason: "invalidation"}, {Type: "resistance", Timeframe: "1h", Price: 113, Reason: "first target"}},
		RiskReward: AIRiskRewardRationale{Entry: 105, Invalidation: 100, FirstTarget: 113, GrossEstimatedRR: 1.6, NetEstimatedRR: 1.5, MinRequiredRR: 1.5, Passed: true},
	}
	decisions := []Decision{{Symbol: "ZECUSDT", Action: "open_long", EntryProtection: ep}}
	normalizeAndRepairOpenDecisions(decisions)
	trimEntryProtectionToConfigLimits(decisions[0].EntryProtection, store.EntryStructureConfig{Enabled: true, MaxSupportLevels: 3, MaxResistanceLevels: 3})
	if got := len(decisions[0].EntryProtection.KeyLevels.Resistance); got != 3 {
		t.Fatalf("expected resistance levels trimmed to 3 after repair, got %d", got)
	}
}

func TestBackfillEntryProtectionKeyLevelsFromAnchors(t *testing.T) {
	ep := &AIEntryProtectionRationale{
		Anchors: []AIEntryProtectionAnchor{
			{Type: "support_invalidation", Timeframe: "15m", Price: 95, Reason: "base"},
			{Type: "resistance_target", Timeframe: "1h", Price: 120, Reason: "target"},
		},
	}
	backfillEntryProtectionKeyLevels(ep)
	if len(ep.KeyLevels.Support) != 1 || ep.KeyLevels.Support[0] != 95 {
		t.Fatalf("expected support [95], got %#v", ep.KeyLevels.Support)
	}
	if len(ep.KeyLevels.Resistance) != 1 || ep.KeyLevels.Resistance[0] != 120 {
		t.Fatalf("expected resistance [120], got %#v", ep.KeyLevels.Resistance)
	}
}

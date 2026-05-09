package kernel

import "testing"

func TestValidateDecisionFormatRejectsCloseActionWithProtectionPlan(t *testing.T) {
	decisions := []Decision{{
		Symbol:    "ETHUSDT",
		Action:    "close_long",
		Reasoning: "test",
		ProtectionPlan: &AIProtectionPlan{
			Mode:          "full",
			TakeProfitPct: 8,
			StopLossPct:   3,
		},
	}}

	if err := ValidateDecisionFormat(decisions); err == nil {
		t.Fatal("expected close action carrying protection_plan to be rejected")
	}
}

func TestValidateDecisionFormatRejectsFullModeWithLadderRules(t *testing.T) {
	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 100,
		Reasoning:       "test",
		ProtectionPlan: &AIProtectionPlan{
			Mode:          "full",
			TakeProfitPct: 8,
			StopLossPct:   3,
			LadderRules: []AIProtectionLadderRule{{
				TakeProfitPct: 3,
			}},
		},
	}}

	if err := ValidateDecisionFormat(decisions); err == nil {
		t.Fatal("expected full mode carrying ladder_rules to be rejected")
	}
}

func TestValidateDecisionFormatRejectsLadderModeWithoutRules(t *testing.T) {
	decisions := []Decision{{
		Symbol:          "SOLUSDT",
		Action:          "open_short",
		Leverage:        2,
		PositionSizeUSD: 100,
		Reasoning:       "test",
		ProtectionPlan: &AIProtectionPlan{
			Mode: "ladder",
		},
	}}

	if err := ValidateDecisionFormat(decisions); err == nil {
		t.Fatal("expected ladder mode without ladder_rules to be rejected")
	}
}

func TestValidateDecisionFormatRejectsFullModeWithoutAnyThresholds(t *testing.T) {
	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 100,
		Reasoning:       "test",
		ProtectionPlan: &AIProtectionPlan{
			Mode: "full",
		},
	}}

	if err := ValidateDecisionFormat(decisions); err == nil {
		t.Fatal("expected full mode without TP/SL thresholds to be rejected")
	}
}

func TestValidateDecisionFormatRejectsUnknownProtectionPlanMode(t *testing.T) {
	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 100,
		Reasoning:       "test",
		ProtectionPlan: &AIProtectionPlan{
			Mode: "weird_mode",
		},
	}}

	if err := ValidateDecisionFormat(decisions); err == nil {
		t.Fatal("expected unknown protection_plan mode to be rejected")
	}
}

func TestValidateDecisionFormatRejectsLadderRuleWithInvalidRatios(t *testing.T) {
	decisions := []Decision{{
		Symbol:          "SOLUSDT",
		Action:          "open_short",
		Leverage:        2,
		PositionSizeUSD: 100,
		Reasoning:       "test",
		ProtectionPlan: &AIProtectionPlan{
			Mode: "ladder",
			LadderRules: []AIProtectionLadderRule{{
				TakeProfitPct:           3,
				TakeProfitCloseRatioPct: 0,
				StopLossPct:             1.5,
				StopLossCloseRatioPct:   125,
			}},
		},
	}}

	if err := ValidateDecisionFormat(decisions); err == nil {
		t.Fatal("expected invalid ladder rule ratios to be rejected")
	}
}

func TestValidateDecisionFormatRejectsBreakEvenModeWithoutTrigger(t *testing.T) {
	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 100,
		Reasoning:       "test",
		ProtectionPlan:  &AIProtectionPlan{Mode: "break_even"},
	}}

	if err := ValidateDecisionFormat(decisions); err == nil {
		t.Fatal("expected break_even mode without trigger to be rejected")
	}
}

func TestValidateDecisionFormatRejectsBreakEvenModeWithMixedFields(t *testing.T) {
	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 100,
		Reasoning:       "test",
		ProtectionPlan: &AIProtectionPlan{
			Mode:             "break_even",
			BreakEvenTrigger: "profit_pct",
			BreakEvenValue:   3,
			TakeProfitPct:    8,
		},
	}}

	if err := ValidateDecisionFormat(decisions); err == nil {
		t.Fatal("expected break_even mode with mixed full fields to be rejected")
	}
}

func TestValidateDecisionFormatRejectsDrawdownModeWithoutRules(t *testing.T) {
	decisions := []Decision{{
		Symbol:          "SOLUSDT",
		Action:          "open_short",
		Leverage:        2,
		PositionSizeUSD: 100,
		Reasoning:       "test",
		ProtectionPlan: &AIProtectionPlan{
			Mode: "drawdown",
		},
	}}

	if err := ValidateDecisionFormat(decisions); err == nil {
		t.Fatal("expected drawdown mode without drawdown_rules to be rejected")
	}
}

func TestValidateDecisionFormatRejectsDrawdownModeWithMixedFields(t *testing.T) {
	decisions := []Decision{{
		Symbol:          "SOLUSDT",
		Action:          "open_short",
		Leverage:        2,
		PositionSizeUSD: 100,
		Reasoning:       "test",
		ProtectionPlan: &AIProtectionPlan{
			Mode:          "drawdown",
			TakeProfitPct: 3,
			DrawdownRules: []AIProtectionDrawdownRule{{MinProfitPct: 5, MaxDrawdownPct: 40, CloseRatioPct: 100}},
		},
	}}

	if err := ValidateDecisionFormat(decisions); err == nil {
		t.Fatal("expected drawdown mode with mixed full fields to be rejected")
	}
}

func TestValidateDecisionFormatRejectsDrawdownRuleWithInvalidThresholds(t *testing.T) {
	decisions := []Decision{{
		Symbol:          "SOLUSDT",
		Action:          "open_short",
		Leverage:        2,
		PositionSizeUSD: 100,
		Reasoning:       "test",
		ProtectionPlan: &AIProtectionPlan{
			Mode:          "drawdown",
			DrawdownRules: []AIProtectionDrawdownRule{{MinProfitPct: 0, MaxDrawdownPct: 120, CloseRatioPct: 0, PollIntervalSeconds: 3}},
		},
	}}

	if err := ValidateDecisionFormat(decisions); err == nil {
		t.Fatal("expected invalid drawdown rules to be rejected")
	}
}

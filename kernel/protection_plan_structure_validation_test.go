package kernel

import "testing"

func TestValidateAIProtectionPlanCompletenessAndStructureRejectsPercentOnlyLadderFallback(t *testing.T) {
	d := Decision{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		EntryProtection: validEntryProtectionForTest("open_long"),
		ProtectionPlan: &AIProtectionPlan{
			Mode: "ladder",
			LadderRules: []AIProtectionLadderRule{{
				TakeProfitPct:           3,
				StopLossPct:             0.9,
				TakeProfitCloseRatioPct: 50,
				StopLossCloseRatioPct:   50,
				StructuralAnchor:        "15m support",
			}},
		},
	}
	if err := validateAIProtectionPlanCompletenessAndStructure(d); err == nil {
		t.Fatal("expected percent-only ladder fallback to be rejected")
	}
}

func TestValidateAIProtectionPlanCompletenessAndStructureRejectsDrawdownWithoutAnchorOrTf(t *testing.T) {
	d := Decision{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		EntryProtection: validEntryProtectionForTest("open_long"),
		ProtectionPlan: &AIProtectionPlan{
			Mode: "drawdown",
			DrawdownRules: []AIProtectionDrawdownRule{{
				MinProfitPct:   0.0885,
				MaxDrawdownPct: 68,
				CloseRatioPct:  100,
				StageName:      "outer_exit",
			}},
		},
	}
	if err := validateAIProtectionPlanCompletenessAndStructure(d); err == nil {
		t.Fatal("expected drawdown rule without anchor/timeframe to be rejected")
	}
}

func TestValidateAIProtectionPlanCompletenessAndStructureAllowsAnchoredDrawdown(t *testing.T) {
	d := Decision{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		EntryProtection: validEntryProtectionForTest("open_long"),
		ProtectionPlan: &AIProtectionPlan{
			Mode: "drawdown",
			DrawdownRules: []AIProtectionDrawdownRule{{
				Timeframe:      "15m",
				MinProfitPct:   1.8,
				MaxDrawdownPct: 60,
				CloseRatioPct:  50,
				ReasonAnchor:   "15m primary resistance",
				StageName:      "partial_profit_lock",
			}},
		},
	}
	if err := validateAIProtectionPlanCompletenessAndStructure(d); err != nil {
		t.Fatalf("expected anchored drawdown to pass, got %v", err)
	}
}

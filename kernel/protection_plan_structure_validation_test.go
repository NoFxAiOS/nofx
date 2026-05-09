package kernel

import (
	"strings"
	"testing"
)

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
				VolatilityBufferReason:  "15m ATR buffer",
			}},
		},
	}
	if err := validateAIProtectionPlanCompletenessAndStructure(d); err == nil {
		t.Fatal("expected percent-only ladder fallback to be rejected")
	}
}

func TestValidateAIProtectionPlanCompletenessAndStructureRejectsLadderWithoutBuffer(t *testing.T) {
	d := Decision{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		EntryProtection: validEntryProtectionForTest("open_long"),
		ProtectionPlan: &AIProtectionPlan{
			Mode: "ladder",
			LadderRules: []AIProtectionLadderRule{{
				TakeProfitPrice:         110,
				TakeProfitCloseRatioPct: 40,
				StopLossPrice:           95,
				StopLossCloseRatioPct:   50,
				StructuralAnchor:        "15m support / 15m resistance",
			}},
		},
	}
	if err := validateAIProtectionPlanCompletenessAndStructure(d); err == nil || !strings.Contains(err.Error(), "volatility_buffer") {
		t.Fatalf("expected missing volatility buffer to be rejected, got %v", err)
	}
}

func TestValidateAIProtectionPlanCompletenessAndStructureRejectsWrongSideLadder(t *testing.T) {
	d := Decision{
		Symbol:          "BTCUSDT",
		Action:          "open_short",
		EntryProtection: validEntryProtectionForTest("open_short"),
		ProtectionPlan: &AIProtectionPlan{
			Mode: "ladder",
			LadderRules: []AIProtectionLadderRule{{
				TakeProfitPrice:         105,
				TakeProfitCloseRatioPct: 40,
				StopLossPrice:           95,
				StopLossCloseRatioPct:   50,
				StructuralAnchor:        "15m resistance / 15m support",
				VolatilityBufferReason:  "15m ATR buffer",
			}},
		},
	}
	if err := validateAIProtectionPlanCompletenessAndStructure(d); err == nil || !strings.Contains(err.Error(), "take_profit_price must be below entry for short") {
		t.Fatalf("expected wrong-side short TP to be rejected, got %v", err)
	}
}

func TestValidateAIProtectionPlanCompletenessAndStructureAllowsAnchoredAbsoluteLadder(t *testing.T) {
	d := Decision{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		EntryProtection: validEntryProtectionForTest("open_long"),
		ProtectionPlan: &AIProtectionPlan{
			Mode: "ladder",
			LadderRules: []AIProtectionLadderRule{{
				TakeProfitPrice:         110,
				TakeProfitCloseRatioPct: 40,
				StopLossPrice:           95,
				StopLossCloseRatioPct:   50,
				StructuralAnchor:        "15m support / 15m resistance",
				VolatilityBufferReason:  "15m ATR buffer",
			}},
		},
	}
	if err := validateAIProtectionPlanCompletenessAndStructure(d); err != nil {
		t.Fatalf("expected anchored absolute ladder to pass, got %v", err)
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

func TestValidateDrawdownRulesStructureRejectsAllFullCloseTiers(t *testing.T) {
	err := validateDrawdownRulesStructure([]AIProtectionDrawdownRule{
		{Timeframe: "15m", StageName: "outer_exit", MinProfitPct: 0.28, MaxDrawdownPct: 55, CloseRatioPct: 100, ReasonAnchor: "15m target"},
		{Timeframe: "15m", StageName: "outer_exit", MinProfitPct: 0.6, MaxDrawdownPct: 48, CloseRatioPct: 100, ReasonAnchor: "15m target"},
		{Timeframe: "15m", StageName: "outer_exit", MinProfitPct: 0.98, MaxDrawdownPct: 42, CloseRatioPct: 100, ReasonAnchor: "15m target"},
	})
	if err == nil || !strings.Contains(err.Error(), "closes 100% before final stage") {
		t.Fatalf("expected all/full-close drawdown tiers to be rejected, got %v", err)
	}
}

func TestValidateDrawdownRulesStructureAcceptsTimeframeRunnerTiers(t *testing.T) {
	err := validateDrawdownRulesStructure([]AIProtectionDrawdownRule{
		{Timeframe: "15m", StageName: "partial_profit_lock", MinProfitPct: 0.3, MaxDrawdownPct: 60, CloseRatioPct: 35, ReasonAnchor: "15m first target"},
		{Timeframe: "1h", StageName: "higher_timeframe_runner", MinProfitPct: 1.0, MaxDrawdownPct: 50, CloseRatioPct: 50, RunnerKeepPct: 30, ReasonAnchor: "1h resistance"},
		{Timeframe: "1h", StageName: "extension_exhaustion", MinProfitPct: 2.0, MaxDrawdownPct: 40, CloseRatioPct: 70, RunnerKeepPct: 20, ReasonAnchor: "1h extension"},
	})
	if err != nil {
		t.Fatalf("expected structured runner tiers to pass, got %v", err)
	}
}

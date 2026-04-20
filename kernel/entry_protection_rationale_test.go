package kernel

import (
	"strings"
	"testing"

	"nofx/store"
)

func validEntryProtectionForTest(action string) *AIEntryProtectionRationale {
	if action == "open_short" {
		return &AIEntryProtectionRationale{
			RiskReward: AIRiskRewardRationale{Entry: 100, Invalidation: 110, FirstTarget: 80, GrossEstimatedRR: 2.0, NetEstimatedRR: 1.8, MinRequiredRR: 1.5, Passed: true},
		}
	}
	return &AIEntryProtectionRationale{
		RiskReward: AIRiskRewardRationale{Entry: 100, Invalidation: 95, FirstTarget: 110, GrossEstimatedRR: 2.0, NetEstimatedRR: 1.8, MinRequiredRR: 1.5, Passed: true},
	}
}

func validTighterLongEntryProtectionForTest() *AIEntryProtectionRationale {
	return &AIEntryProtectionRationale{
		RiskReward: AIRiskRewardRationale{Entry: 100, Invalidation: 96, FirstTarget: 108, GrossEstimatedRR: 2.0, NetEstimatedRR: 1.8, MinRequiredRR: 1.5, Passed: true},
	}
}

func TestValidateAIDecisionsWithStrategyRequiresEntryProtectionRationale(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 1.5

	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 500,
		Reasoning:       "setup looks good",
	}}

	err := ValidateAIDecisionsWithStrategy(decisions, cfg)
	if err == nil || !strings.Contains(err.Error(), "entry_protection_rationale") {
		t.Fatalf("expected entry_protection_rationale validation error, got %v", err)
	}
}

func TestValidateAIDecisionsWithStrategyRejectsWrongLongDirection(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 1.5

	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 500,
		Reasoning:       "setup looks good",
		EntryProtection: &AIEntryProtectionRationale{
			RiskReward: AIRiskRewardRationale{
				Entry:            100,
				Invalidation:     105,
				FirstTarget:      120,
				GrossEstimatedRR: 4.0,
			},
		},
	}}

	err := ValidateAIDecisionsWithStrategy(decisions, cfg)
	if err == nil || !strings.Contains(err.Error(), "direction mismatch") {
		t.Fatalf("expected direction mismatch error, got %v", err)
	}
}

func TestValidateAIDecisionsWithStrategyRejectsRRBelowMinimum(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 2.0

	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_short",
		Leverage:        3,
		PositionSizeUSD: 500,
		Reasoning:       "setup looks good",
		EntryProtection: &AIEntryProtectionRationale{
			RiskReward: AIRiskRewardRationale{
				Entry:            100,
				Invalidation:     110,
				FirstTarget:      75,
				GrossEstimatedRR: 2.5,
				NetEstimatedRR:   1.8,
				MinRequiredRR:    2.0,
				Passed:           false,
			},
		},
	}}

	err := ValidateAIDecisionsWithStrategy(decisions, cfg)
	if err == nil || !strings.Contains(err.Error(), "below min") {
		t.Fatalf("expected RR below min error, got %v", err)
	}
}

func TestValidateAIDecisionsWithStrategyAllowsWaitAndEmpty(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 1.5

	if err := ValidateAIDecisionsWithStrategy([]Decision{}, cfg); err != nil {
		t.Fatalf("expected empty decisions to be valid, got %v", err)
	}

	decisions := []Decision{{
		Symbol:    "BTCUSDT",
		Action:    "wait",
		Reasoning: "no clean setup",
	}}
	if err := ValidateAIDecisionsWithStrategy(decisions, cfg); err != nil {
		t.Fatalf("expected wait decision to be valid, got %v", err)
	}
}

func TestValidateDecisionFormatRejectsEntryProtectionOnCloseAction(t *testing.T) {
	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "close_long",
		Reasoning:       "take profit",
		EntryProtection: validEntryProtectionForTest("open_long"),
	}}

	err := ValidateDecisionFormat(decisions)
	if err == nil || !strings.Contains(err.Error(), "entry_protection_rationale is only allowed for open actions") {
		t.Fatalf("expected close-action entry_protection_rationale error, got %v", err)
	}
}

func TestValidateAIDecisionsWithStrategyRejectsGrossRRMismatch(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 1.5

	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 500,
		Reasoning:       "setup looks good",
		EntryProtection: &AIEntryProtectionRationale{
			RiskReward: AIRiskRewardRationale{
				Entry:            100,
				Invalidation:     95,
				FirstTarget:      110,
				GrossEstimatedRR: 1.5,
				NetEstimatedRR:   1.6,
				MinRequiredRR:    1.5,
				Passed:           true,
			},
		},
	}}

	err := ValidateAIDecisionsWithStrategy(decisions, cfg)
	if err == nil || !strings.Contains(err.Error(), "gross_estimated_rr") {
		t.Fatalf("expected gross_estimated_rr mismatch error, got %v", err)
	}
}

func TestValidateAIDecisionsWithStrategyRejectsMinRequiredRRMismatch(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 1.5

	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 500,
		Reasoning:       "setup looks good",
		EntryProtection: validEntryProtectionForTest("open_long"),
	}}
	decisions[0].EntryProtection.RiskReward.MinRequiredRR = 2.0

	err := ValidateAIDecisionsWithStrategy(decisions, cfg)
	if err == nil || !strings.Contains(err.Error(), "min_required_rr") {
		t.Fatalf("expected min_required_rr mismatch error, got %v", err)
	}
}

func TestValidateAIDecisionsWithStrategyRejectsPassedFlagMismatch(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 1.5

	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 500,
		Reasoning:       "setup looks good",
		EntryProtection: validEntryProtectionForTest("open_long"),
	}}
	decisions[0].EntryProtection.RiskReward.Passed = false

	err := ValidateAIDecisionsWithStrategy(decisions, cfg)
	if err == nil || !strings.Contains(err.Error(), "passed=false") {
		t.Fatalf("expected passed flag mismatch error, got %v", err)
	}
}

func TestValidateAIDecisionsWithStrategyRejectsFullProtectionPlanRationaleMismatch(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 1.5

	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 500,
		Reasoning:       "setup looks good",
		EntryProtection: validEntryProtectionForTest("open_long"),
		ProtectionPlan: &AIProtectionPlan{
			Mode:          "full",
			StopLossPct:   2,
			TakeProfitPct: 10,
		},
	}}

	err := ValidateAIDecisionsWithStrategy(decisions, cfg)
	if err == nil || !strings.Contains(err.Error(), "stop_loss_pct") {
		t.Fatalf("expected stop_loss_pct alignment error, got %v", err)
	}
}

func TestValidateAIDecisionsWithStrategyRejectsBreakEvenProfitTriggerBeyondFirstTarget(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 1.5

	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 500,
		Reasoning:       "setup looks good",
		EntryProtection: validEntryProtectionForTest("open_long"),
		ProtectionPlan: &AIProtectionPlan{
			Mode:             "break_even",
			BreakEvenTrigger: "profit_pct",
			BreakEvenValue:   12,
			BreakEvenOffset:  0.1,
		},
	}}

	err := ValidateAIDecisionsWithStrategy(decisions, cfg)
	if err == nil || !strings.Contains(err.Error(), "break_even_trigger_value") {
		t.Fatalf("expected break_even trigger alignment error, got %v", err)
	}
}

func TestValidateAIDecisionsWithStrategyAllowsBreakEvenRMultipleAtFirstTargetRR(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 1.5

	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 500,
		Reasoning:       "setup looks good",
		EntryProtection: validEntryProtectionForTest("open_long"),
		ProtectionPlan: &AIProtectionPlan{
			Mode:             "break_even",
			BreakEvenTrigger: "r_multiple",
			BreakEvenValue:   2.0,
			BreakEvenOffset:  0.1,
		},
	}}

	if err := ValidateAIDecisionsWithStrategy(decisions, cfg); err != nil {
		t.Fatalf("expected break_even r_multiple at first target RR to validate, got %v", err)
	}
}

func TestValidateAIDecisionsWithStrategyRejectsFallbackMaxLossInsideInvalidationEnvelope(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 1.5
	cfg.Protection.FullTPSL = store.FullTPSLConfig{
		Enabled:                true,
		Mode:                   store.ProtectionModeManual,
		FallbackMaxLossEnabled: true,
		FallbackMaxLoss:        store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 4.0},
	}

	err := validateFallbackMaxLossAlignment("open_long", validEntryProtectionForTest("open_long").RiskReward, nil, cfg)
	if err == nil || !strings.Contains(err.Error(), "fallback_max_loss") {
		t.Fatalf("expected fallback_max_loss alignment error, got %v", err)
	}
}

func TestValidateAIDecisionsWithStrategyAllowsFallbackMaxLossOutsideInvalidationEnvelope(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 1.5
	cfg.Protection.FullTPSL = store.FullTPSLConfig{
		Enabled:                true,
		Mode:                   store.ProtectionModeManual,
		FallbackMaxLossEnabled: true,
		FallbackMaxLoss:        store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 6.0},
	}

	if err := validateFallbackMaxLossAlignment("open_long", validEntryProtectionForTest("open_long").RiskReward, nil, cfg); err != nil {
		t.Fatalf("expected fallback_max_loss outside invalidation envelope to validate, got %v", err)
	}
}

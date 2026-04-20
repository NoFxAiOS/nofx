package kernel

import (
	"strings"
	"testing"

	"nofx/store"
)

func validEntryProtectionForTest(action string) *AIEntryProtectionRationale {
	if action == "open_short" {
		return &AIEntryProtectionRationale{
			RiskReward: AIRiskRewardRationale{Entry: 100, Invalidation: 110, FirstTarget: 90, GrossEstimatedRR: 2.0, NetEstimatedRR: 1.8},
		}
	}
	return &AIEntryProtectionRationale{
		RiskReward: AIRiskRewardRationale{Entry: 100, Invalidation: 95, FirstTarget: 110, GrossEstimatedRR: 2.0, NetEstimatedRR: 1.8},
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
				GrossEstimatedRR: 2.0,
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
				FirstTarget:      90,
				GrossEstimatedRR: 2.5,
				NetEstimatedRR:   1.8,
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

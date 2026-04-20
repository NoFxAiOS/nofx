package kernel

import (
	"strings"
	"testing"

	"nofx/store"
)

func TestValidateAIDecisionsWithStrategyUsesRoundedNetRRFromExecutionConstraints(t *testing.T) {
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
				Entry:            100.04,
				Invalidation:     95.01,
				FirstTarget:      110.06,
				GrossEstimatedRR: 2.0,
				NetEstimatedRR:   1.96,
				MinRequiredRR:    1.5,
				Passed:           true,
			},
			ExecutionConstraints: AIEntryExecutionConstraints{
				TickSize:             0.1,
				TakerFeeRate:         0.0004,
				EstimatedSlippageBps: 1,
			},
		},
	}}

	if err := ValidateAIDecisionsWithStrategy(decisions, cfg); err != nil {
		t.Fatalf("expected rounded net RR to validate, got %v", err)
	}
}

func TestValidateAIDecisionsWithStrategyRejectsNetRRMismatchWithExecutionConstraints(t *testing.T) {
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
				GrossEstimatedRR: 2.0,
				NetEstimatedRR:   1.5,
				MinRequiredRR:    1.5,
				Passed:           true,
			},
			ExecutionConstraints: AIEntryExecutionConstraints{
				PricePrecision:       2,
				TakerFeeRate:         0.0004,
				EstimatedSlippageBps: 1,
			},
		},
	}}

	err := ValidateAIDecisionsWithStrategy(decisions, cfg)
	if err == nil || !strings.Contains(err.Error(), "net_estimated_rr") {
		t.Fatalf("expected net_estimated_rr mismatch, got %v", err)
	}
}

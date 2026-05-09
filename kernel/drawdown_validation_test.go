package kernel

import (
	"strings"
	"testing"
)

func TestValidateDecisionFormatRequiresAtLeastTwoDrawdownRules(t *testing.T) {
	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Reasoning:       "valid setup",
		Leverage:        1,
		PositionSizeUSD: 100,
		ProtectionPlan: &AIProtectionPlan{
			Mode: "drawdown",
			DrawdownRules: []AIProtectionDrawdownRule{{
				MinProfitPct:        1.0,
				MaxDrawdownPct:      60,
				CloseRatioPct:       50,
				PollIntervalSeconds: 20,
			}},
		},
	}}

	err := ValidateDecisionFormat(decisions)
	if err == nil || !strings.Contains(err.Error(), "at least 2 drawdown_rules") {
		t.Fatalf("expected at least two drawdown rules validation error, got %v", err)
	}
}

func TestValidateDecisionFormatAcceptsTwoDrawdownRules(t *testing.T) {
	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Reasoning:       "valid setup",
		Leverage:        1,
		PositionSizeUSD: 100,
		ProtectionPlan: &AIProtectionPlan{
			Mode: "drawdown",
			DrawdownRules: []AIProtectionDrawdownRule{{
				Timeframe:           "15m",
				MinProfitPct:        1.0,
				MaxDrawdownPct:      70,
				CloseRatioPct:       35,
				PollIntervalSeconds: 20,
				ReasonAnchor:        "first primary-timeframe resistance",
			}, {
				Timeframe:           "1h",
				MinProfitPct:        2.2,
				MaxDrawdownPct:      55,
				CloseRatioPct:       65,
				PollIntervalSeconds: 20,
				ReasonAnchor:        "higher-timeframe trend extension",
			}},
		},
	}}

	if err := ValidateDecisionFormat(decisions); err != nil {
		t.Fatalf("expected two drawdown rules to pass, got %v", err)
	}
}

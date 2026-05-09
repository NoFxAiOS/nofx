package api

import (
	"encoding/json"
	"strings"
	"testing"

	"nofx/kernel"
	"nofx/store"
)

func validEntryProtectionForAPITest(action string) *kernel.AIEntryProtectionRationale {
	if action == "open_short" {
		return &kernel.AIEntryProtectionRationale{
			RiskReward: kernel.AIRiskRewardRationale{Entry: 100, Invalidation: 110, FirstTarget: 80, GrossEstimatedRR: 2.0, NetEstimatedRR: 1.8, MinRequiredRR: 1.5, Passed: true},
		}
	}
	return &kernel.AIEntryProtectionRationale{
		RiskReward: kernel.AIRiskRewardRationale{Entry: 100, Invalidation: 95, FirstTarget: 110, GrossEstimatedRR: 2.0, NetEstimatedRR: 1.8, MinRequiredRR: 1.5, Passed: true},
	}
}

func validAlignedFullProtectionPlanForAPITest() *kernel.AIProtectionPlan {
	return &kernel.AIProtectionPlan{
		Mode:          "full",
		TakeProfitPct: 10,
		StopLossPct:   5,
	}
}

func TestRouteAwareDrawdownProtectionErrorsExposeParseErrorInEnvelope(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 1.5
	cfg.Protection.DrawdownTakeProfit = store.DrawdownTakeProfitConfig{Enabled: true, Mode: store.ProtectionModeAI}
	cfg.Protection.FullTPSL = store.FullTPSLConfig{Enabled: true, Mode: store.ProtectionModeDisabled}
	cfg.Protection.LadderTPSL = store.LadderTPSLConfig{Enabled: true, Mode: store.ProtectionModeDisabled}

	decisions := []kernel.Decision{{
		Symbol:          "XRPUSDT",
		Action:          "open_long",
		Leverage:        2,
		PositionSizeUSD: 100,
		Reasoning:       "test",
		EntryProtection: validEntryProtectionForAPITest("open_long"),
		ProtectionPlan:  validAlignedFullProtectionPlanForAPITest(),
	}}
	validationErr := kernel.ValidateAIDecisionsWithStrategy(decisions, cfg)
	if validationErr == nil {
		t.Fatal("expected route-aware validator to fail")
	}
	if !strings.Contains(validationErr.Error(), "current strategy route requires drawdown protection_plan") {
		t.Fatalf("unexpected validation error: %v", validationErr)
	}
}

func TestRouteAwareProtectionErrorsExposeParseErrorInEnvelope(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 1.5
	cfg.Protection.LadderTPSL = store.LadderTPSLConfig{Enabled: true, Mode: store.ProtectionModeAI}
	cfg.Protection.FullTPSL = store.FullTPSLConfig{Enabled: true, Mode: store.ProtectionModeDisabled}

	decisions := []kernel.Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 100,
		Reasoning:       "test",
		EntryProtection: validEntryProtectionForAPITest("open_long"),
		ProtectionPlan:  validAlignedFullProtectionPlanForAPITest(),
	}}
	validationErr := kernel.ValidateAIDecisionsWithStrategy(decisions, cfg)
	if validationErr == nil {
		t.Fatal("expected route-aware validator to fail")
	}
	if !strings.Contains(validationErr.Error(), "current strategy route requires ladder protection_plan") {
		t.Fatalf("unexpected validation error: %v", validationErr)
	}

	payload := map[string]any{
		"ai_response":      decisions,
		"parsed_decisions": decisions,
		"parse_error":      validationErr.Error(),
	}
	blob, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal envelope failed: %v", err)
	}
	var decoded struct {
		ParseError string `json:"parse_error"`
	}
	if err := json.Unmarshal(blob, &decoded); err != nil {
		t.Fatalf("unmarshal envelope failed: %v", err)
	}
	if !strings.Contains(decoded.ParseError, "current strategy route requires ladder protection_plan") {
		t.Fatalf("expected parse_error to expose route-aware failure, got %q", decoded.ParseError)
	}
}

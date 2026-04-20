package kernel

import (
	"testing"

	"nofx/store"
)

func TestValidateProtectionReasoningContractRejectsMissingDrawdownAcknowledgement(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Protection.DrawdownTakeProfit.Enabled = true
	cfg.Protection.DrawdownTakeProfit.Rules = []store.DrawdownTakeProfitRule{{MinProfitPct: 5, MaxDrawdownPct: 30, CloseRatioPct: 100, PollIntervalSeconds: 60}}

	err := ValidateProtectionReasoningContract("this reasoning only talks about trend and entry", cfg)
	if err == nil {
		t.Fatal("expected missing drawdown acknowledgement to fail")
	}
}

func TestValidateProtectionReasoningContractRejectsMissingBreakEvenAcknowledgement(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Protection.BreakEvenStop.Enabled = true

	err := ValidateProtectionReasoningContract("this reasoning talks about tp/sl only", cfg)
	if err == nil {
		t.Fatal("expected missing break-even acknowledgement to fail")
	}
}

func TestValidateProtectionReasoningContractAcceptsAcknowledgement(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Protection.DrawdownTakeProfit.Enabled = true
	cfg.Protection.DrawdownTakeProfit.Rules = []store.DrawdownTakeProfitRule{{MinProfitPct: 5, MaxDrawdownPct: 30, CloseRatioPct: 100, PollIntervalSeconds: 60}}
	cfg.Protection.BreakEvenStop.Enabled = true

	reasoning := "drawdown will remain the primary profit-protection path and break-even adds an extra stop layer after profit trigger"
	if err := ValidateProtectionReasoningContract(reasoning, cfg); err != nil {
		t.Fatalf("expected reasoning contract to pass, got %v", err)
	}
}

func TestValidateProtectionReasoningContractRejectsDrawdownAIWithoutTimeframeAndStructureAnchors(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Protection.DrawdownTakeProfit.Enabled = true
	cfg.Protection.DrawdownTakeProfit.Mode = store.ProtectionModeAI
	cfg.Protection.DrawdownTakeProfit.Rules = []store.DrawdownTakeProfitRule{{MinProfitPct: 5, MaxDrawdownPct: 30, CloseRatioPct: 100, PollIntervalSeconds: 60}}

	err := ValidateProtectionReasoningContract("drawdown profit-protection will be used but details are omitted", cfg)
	if err == nil {
		t.Fatal("expected drawdown ai reasoning without timeframe/structure anchors to fail")
	}
}

func TestValidateProtectionReasoningContractAcceptsDrawdownAITimeframeAndStructureAnchors(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Protection.DrawdownTakeProfit.Enabled = true
	cfg.Protection.DrawdownTakeProfit.Mode = store.ProtectionModeAI
	cfg.Protection.DrawdownTakeProfit.Rules = []store.DrawdownTakeProfitRule{{MinProfitPct: 5, MaxDrawdownPct: 30, CloseRatioPct: 100, PollIntervalSeconds: 60}}

	reasoning := "drawdown remains the profit-protection owner; on the 15m primary timeframe with 5m confirmation and 1h structure, support/resistance, fibonacci, and volatility justify staged exits"
	if err := ValidateProtectionReasoningContract(reasoning, cfg); err != nil {
		t.Fatalf("expected drawdown ai reasoning contract to pass, got %v", err)
	}
}

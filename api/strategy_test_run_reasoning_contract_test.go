package api

import (
	"encoding/json"
	"strings"
	"testing"

	"nofx/kernel"
	"nofx/store"
)

func TestBreakEvenRouteRequiresStructuredOutput(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Protection.BreakEvenStop.Enabled = true

	raw := `[{"symbol":"BTCUSDT","action":"open_long","leverage":3,"position_size_usd":100,"reasoning":"15m primary timeframe, 5m support/resistance, fibonacci and volatility anchors"}]`
	decisions, err := kernel.ParseAIDecisions(raw)
	if err != nil {
		t.Fatalf("parser should succeed, got %v", err)
	}
	validationErr := kernel.ValidateAIDecisionsWithStrategy(decisions, cfg)
	if validationErr == nil {
		t.Fatal("expected break-even structured output route to fail")
	}
	if !strings.Contains(validationErr.Error(), "requires break-even protection output") {
		t.Fatalf("expected break-even route failure, got %v", validationErr)
	}
}

func TestReasoningContractErrorsExposeParseErrorInEnvelope(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Protection.DrawdownTakeProfit.Enabled = true
	cfg.Protection.DrawdownTakeProfit.Rules = []store.DrawdownTakeProfitRule{{MinProfitPct: 5, MaxDrawdownPct: 30, CloseRatioPct: 100, PollIntervalSeconds: 60}}
	cfg.Protection.BreakEvenStop.Enabled = true

	raw := `<reasoning>Only discuss entry trend and stop loss, ignore all runtime protection.</reasoning><decision>[{"symbol":"BTCUSDT","action":"wait"}]</decision>`
	decisions, err := kernel.ParseAIDecisions(raw)
	if err != nil {
		t.Fatalf("parser should succeed, got %v", err)
	}
	validationErr := kernel.ValidateProtectionReasoningContract("Only discuss entry trend and stop loss, ignore all runtime protection.", cfg)
	if validationErr == nil {
		t.Fatal("expected reasoning contract to fail")
	}
	payload := map[string]any{
		"ai_response":      raw,
		"parsed_decisions": decisions,
		"parse_error":      validationErr.Error(),
	}
	blob, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded struct {
		ParseError string `json:"parse_error"`
	}
	if err := json.Unmarshal(blob, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !strings.Contains(decoded.ParseError, "drawdown") {
		t.Fatalf("expected parse_error to expose reasoning contract failure, got %q", decoded.ParseError)
	}
}

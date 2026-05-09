package api

import (
	"encoding/json"
	"testing"

	"nofx/kernel"
)

func TestStrategyTestRunStructuredDecisionEnvelopeShape(t *testing.T) {
	decisions := []kernel.Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 100,
		Reasoning:       "test",
		ProtectionPlan: &kernel.AIProtectionPlan{
			Mode:          "full",
			TakeProfitPct: 8,
			StopLossPct:   2,
		},
	}}

	payload := map[string]any{
		"ai_response":      "raw",
		"parsed_decisions": decisions,
		"parse_error":      "",
	}

	blob, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(blob, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := decoded["parsed_decisions"]; !ok {
		t.Fatal("expected parsed_decisions key in test-run response envelope")
	}
	if _, ok := decoded["parse_error"]; !ok {
		t.Fatal("expected parse_error key in test-run response envelope")
	}
}

func TestStrategyTestRunNonRealAIEnvelopeIncludesParsedDecisionFields(t *testing.T) {
	payload := map[string]any{
		"ai_response":      "Please select an AI model and click 'Run Test' to perform real AI analysis.",
		"parsed_decisions": []any{},
		"parse_error":      "",
	}

	blob, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(blob, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := decoded["parsed_decisions"]; !ok {
		t.Fatal("expected parsed_decisions in non-real-AI envelope")
	}
	if _, ok := decoded["parse_error"]; !ok {
		t.Fatal("expected parse_error in non-real-AI envelope")
	}
}

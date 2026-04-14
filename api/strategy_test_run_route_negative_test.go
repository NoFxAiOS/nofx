package api

import (
	"encoding/json"
	"strings"
	"testing"

	"nofx/kernel"
	"nofx/store"
)

func TestRouteAwareProtectionErrorsExposeParseErrorInEnvelope(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Protection.LadderTPSL = store.LadderTPSLConfig{Enabled: true, Mode: store.ProtectionModeAI}
	cfg.Protection.FullTPSL = store.FullTPSLConfig{Enabled: true, Mode: store.ProtectionModeDisabled}

	raw := `[{"symbol":"BTCUSDT","action":"open_long","leverage":3,"position_size_usd":100,"reasoning":"test","protection_plan":{"mode":"full","take_profit_pct":8,"stop_loss_pct":3}}]`
	decisions, parseErr := kernel.ParseAIDecisions(raw)
	if parseErr != nil {
		t.Fatalf("expected parser to succeed before route-aware validation, got %v", parseErr)
	}
	validationErr := kernel.ValidateAIDecisionsWithStrategy(decisions, cfg)
	if validationErr == nil {
		t.Fatal("expected route-aware validator to fail")
	}
	if !strings.Contains(validationErr.Error(), "current strategy route requires ladder protection_plan") {
		t.Fatalf("unexpected validation error: %v", validationErr)
	}

	payload := map[string]any{
		"ai_response":      raw,
		"parsed_decisions": decisions,
		"parse_error":      validationErr.Error(),
	}
	blob, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal envelope failed: %v", err)
	}
	var decoded struct{ ParseError string `json:"parse_error"` }
	if err := json.Unmarshal(blob, &decoded); err != nil {
		t.Fatalf("unmarshal envelope failed: %v", err)
	}
	if !strings.Contains(decoded.ParseError, "current strategy route requires ladder protection_plan") {
		t.Fatalf("expected parse_error to expose route-aware failure, got %q", decoded.ParseError)
	}
}

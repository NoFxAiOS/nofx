package kernel

import (
	"testing"

	"nofx/market"
	"nofx/store"
)

func TestParseMacroResponse_ValidJSON(t *testing.T) {
	input := `{"trend":"bullish","risk_level":"medium","focus_reason":"test","symbols_for_deep_dive":["BTCUSDT","ETHUSDT"],"check_positions":true}`
	out, err := ParseMacroResponse(input)
	if err != nil {
		t.Fatalf("ParseMacroResponse failed: %v", err)
	}
	if out == nil {
		t.Fatal("expected non-nil MacroOutput")
	}
	if out.Trend != "bullish" {
		t.Errorf("Trend = %q, want bullish", out.Trend)
	}
	if out.RiskLevel != "medium" {
		t.Errorf("RiskLevel = %q, want medium", out.RiskLevel)
	}
	if out.FocusReason != "test" {
		t.Errorf("FocusReason = %q, want test", out.FocusReason)
	}
	if !out.CheckPositions {
		t.Error("CheckPositions = false, want true")
	}
	if len(out.SymbolsForDeepDive) != 2 {
		t.Fatalf("SymbolsForDeepDive len = %d, want 2", len(out.SymbolsForDeepDive))
	}
	if out.SymbolsForDeepDive[0] != "BTCUSDT" || out.SymbolsForDeepDive[1] != "ETHUSDT" {
		t.Errorf("SymbolsForDeepDive = %v, want [BTCUSDT ETHUSDT]", out.SymbolsForDeepDive)
	}
}

func TestParseMacroResponse_WithCodeFence(t *testing.T) {
	input := "```json\n{\"trend\":\"bearish\",\"risk_level\":\"high\",\"focus_reason\":\"x\",\"symbols_for_deep_dive\":[\"SOLUSDT\"],\"check_positions\":false}\n```"
	out, err := ParseMacroResponse(input)
	if err != nil {
		t.Fatalf("ParseMacroResponse failed: %v", err)
	}
	if out.Trend != "bearish" {
		t.Errorf("Trend = %q, want bearish", out.Trend)
	}
	if out.RiskLevel != "high" {
		t.Errorf("RiskLevel = %q, want high", out.RiskLevel)
	}
	if len(out.SymbolsForDeepDive) != 1 || out.SymbolsForDeepDive[0] != "SOLUSDT" {
		t.Errorf("SymbolsForDeepDive = %v, want [SOLUSDT]", out.SymbolsForDeepDive)
	}
}

func TestParseMacroResponse_Invalid(t *testing.T) {
	_, err := ParseMacroResponse("not valid json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestValidateAndMergeMacroOutput_PositionsIncluded(t *testing.T) {
	config := &store.StrategyConfig{MacroDeepDiveLimit: 5}
	ctx := &Context{
		Positions: []PositionInfo{{Symbol: "BTCUSDT"}},
	}
	out := &MacroOutput{
		Trend:              "neutral",
		RiskLevel:          "medium",
		SymbolsForDeepDive: []string{"SOLUSDT"},
		CheckPositions:     true,
	}
	result := ValidateAndMergeMacroOutput(out, ctx, config)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// BTCUSDT (position) must be included; SOLUSDT from macro
	hasBTC := false
	hasSOL := false
	for _, s := range result.SymbolsForDeepDive {
		n := market.Normalize(s)
		if n == "BTCUSDT" {
			hasBTC = true
		}
		if n == "SOLUSDT" {
			hasSOL = true
		}
	}
	if !hasBTC {
		t.Error("SymbolsForDeepDive must include position BTCUSDT")
	}
	if !hasSOL {
		t.Error("SymbolsForDeepDive must include macro symbol SOLUSDT")
	}
}

func TestValidateAndMergeMacroOutput_CoercesEnums(t *testing.T) {
	config := &store.StrategyConfig{MacroDeepDiveLimit: 5}
	ctx := &Context{}
	out := &MacroOutput{
		Trend:              "invalid",
		RiskLevel:          "unknown",
		SymbolsForDeepDive: []string{"BTCUSDT"},
	}
	result := ValidateAndMergeMacroOutput(out, ctx, config)
	if result.Trend != "neutral" {
		t.Errorf("invalid Trend should be coerced to neutral, got %q", result.Trend)
	}
	if result.RiskLevel != "medium" {
		t.Errorf("invalid RiskLevel should be coerced to medium, got %q", result.RiskLevel)
	}
}

func TestValidateAndMergeMacroOutput_CapsTotal(t *testing.T) {
	config := &store.StrategyConfig{MacroDeepDiveLimit: 3}
	ctx := &Context{
		Positions: []PositionInfo{{Symbol: "BTCUSDT"}},
	}
	// macro returns 5 symbols; with 1 position and limit 3, maxTotal = 1+3 = 4
	out := &MacroOutput{
		Trend:              "neutral",
		RiskLevel:          "medium",
		SymbolsForDeepDive: []string{"ETHUSDT", "SOLUSDT", "AVAXUSDT", "DOGEUSDT", "XRPUSDT"},
	}
	result := ValidateAndMergeMacroOutput(out, ctx, config)
	maxTotal := 1 + 3 // positions + limit
	if len(result.SymbolsForDeepDive) > maxTotal {
		t.Errorf("SymbolsForDeepDive len = %d, want <= %d", len(result.SymbolsForDeepDive), maxTotal)
	}
}

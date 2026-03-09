package kernel

import (
	"testing"

	"nofx/market"
	"nofx/store"
)

func TestParseMacroResponse_ValidJSON(t *testing.T) {
	// New format with per-symbol bias/risk/conviction
	input := `{"trend":"bullish","risk_level":"medium","focus_reason":"test","symbols_for_deep_dive":[{"symbol":"BTCUSDT","bias":"bullish","risk":"low","conviction":0.8},{"symbol":"ETHUSDT","bias":"bearish","risk":"medium","conviction":0.6}],"check_positions":true}`
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
	if out.SymbolsForDeepDive[0].Symbol != "BTCUSDT" || out.SymbolsForDeepDive[1].Symbol != "ETHUSDT" {
		t.Errorf("SymbolsForDeepDive symbols = %v %v", out.SymbolsForDeepDive[0].Symbol, out.SymbolsForDeepDive[1].Symbol)
	}
	if out.SymbolsForDeepDive[0].Bias != "bullish" || out.SymbolsForDeepDive[0].Conviction != 0.8 {
		t.Errorf("BTCUSDT bias=%q conviction=%f", out.SymbolsForDeepDive[0].Bias, out.SymbolsForDeepDive[0].Conviction)
	}
	if out.SymbolsForDeepDive[1].Bias != "bearish" || out.SymbolsForDeepDive[1].Risk != "medium" {
		t.Errorf("ETHUSDT bias=%q risk=%q", out.SymbolsForDeepDive[1].Bias, out.SymbolsForDeepDive[1].Risk)
	}
}

func TestParseMacroResponse_WithCodeFence(t *testing.T) {
	// Legacy format (string array) still supported
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
	if len(out.SymbolsForDeepDive) != 1 || out.SymbolsForDeepDive[0].Symbol != "SOLUSDT" {
		t.Errorf("SymbolsForDeepDive = %v, want [SOLUSDT]", out.SymbolsForDeepDive)
	}
}

func TestParseMacroResponse_LegacyStringArray(t *testing.T) {
	// Legacy format (symbols_for_deep_dive as string array) should still parse
	input := `{"trend":"neutral","risk_level":"medium","focus_reason":"x","symbols_for_deep_dive":["BTCUSDT","ETHUSDT"],"check_positions":false}`
	out, err := ParseMacroResponse(input)
	if err != nil {
		t.Fatalf("ParseMacroResponse failed: %v", err)
	}
	if len(out.SymbolsForDeepDive) != 2 {
		t.Fatalf("SymbolsForDeepDive len = %d, want 2", len(out.SymbolsForDeepDive))
	}
	if out.SymbolsForDeepDive[0].Symbol != "BTCUSDT" || out.SymbolsForDeepDive[1].Symbol != "ETHUSDT" {
		t.Errorf("symbols = %v", out.SymbolsForDeepDive)
	}
	// Legacy parse uses defaults for bias/risk/conviction
	if out.SymbolsForDeepDive[0].Bias != "neutral" || out.SymbolsForDeepDive[0].Conviction != 0.5 {
		t.Errorf("legacy entries should have default bias/conviction")
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
		SymbolsForDeepDive: macroSymbolsForDeepDive{{Symbol: "SOLUSDT", Bias: "bullish", Risk: "medium", Conviction: 0.7}},
		CheckPositions:     true,
	}
	result := ValidateAndMergeMacroOutput(out, ctx, config)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// BTCUSDT (position) must be included; SOLUSDT from macro
	hasBTC := false
	hasSOL := false
	for _, e := range result.SymbolsForDeepDive {
		n := market.Normalize(e.Symbol)
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
		SymbolsForDeepDive: macroSymbolsForDeepDive{{Symbol: "BTCUSDT", Bias: "neutral", Risk: "medium", Conviction: 0.5}},
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
		Trend: "neutral",
		RiskLevel: "medium",
		SymbolsForDeepDive: macroSymbolsForDeepDive{
			{Symbol: "ETHUSDT"}, {Symbol: "SOLUSDT"}, {Symbol: "AVAXUSDT"}, {Symbol: "DOGEUSDT"}, {Symbol: "XRPUSDT"},
		},
	}
	result := ValidateAndMergeMacroOutput(out, ctx, config)
	maxTotal := 1 + 3 // positions + limit
	if len(result.SymbolsForDeepDive) > maxTotal {
		t.Errorf("SymbolsForDeepDive len = %d, want <= %d", len(result.SymbolsForDeepDive), maxTotal)
	}
}

func TestValidateAndMergeMacroOutput_ExcludedCoinsFiltered(t *testing.T) {
	config := &store.StrategyConfig{
		MacroDeepDiveLimit: 5,
		CoinSource: store.CoinSourceConfig{
			ExcludedCoins: []string{"SOLUSDT", "AVAXUSDT"},
		},
	}
	ctx := &Context{
		Positions: []PositionInfo{{Symbol: "SOLUSDT"}}, // position in excluded symbol
	}
	out := &MacroOutput{
		Trend: "neutral",
		RiskLevel: "medium",
		SymbolsForDeepDive: macroSymbolsForDeepDive{
			{Symbol: "BTCUSDT"}, {Symbol: "SOLUSDT"}, {Symbol: "ETHUSDT"}, {Symbol: "AVAXUSDT"},
		},
		CheckPositions: true,
	}
	result := ValidateAndMergeMacroOutput(out, ctx, config)
	// SOLUSDT must be included (position - we need to manage it)
	// BTCUSDT, ETHUSDT must be included (not excluded)
	// AVAXUSDT must NOT be included (excluded, not a position)
	hasSOL := false
	hasAVAX := false
	hasBTC := false
	hasETH := false
	for _, e := range result.SymbolsForDeepDive {
		n := market.Normalize(e.Symbol)
		switch n {
		case "SOLUSDT":
			hasSOL = true
		case "AVAXUSDT":
			hasAVAX = true
		case "BTCUSDT":
			hasBTC = true
		case "ETHUSDT":
			hasETH = true
		}
	}
	if !hasSOL {
		t.Error("SymbolsForDeepDive must include SOLUSDT (position, even if excluded)")
	}
	if hasAVAX {
		t.Error("SymbolsForDeepDive must not include AVAXUSDT (excluded)")
	}
	if !hasBTC || !hasETH {
		t.Error("SymbolsForDeepDive must include BTCUSDT and ETHUSDT (not excluded)")
	}
}

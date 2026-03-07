package kernel

import (
	"errors"
	"strings"
	"testing"
	"time"

	"nofx/market"
	"nofx/mcp"
	"nofx/store"
)

// mockAIClient implements mcp.AIClient for tests. Returns responses in sequence.
// Used for tests that invoke GetFullDecisionMacroMicroWithTrace with mocked AI.
type mockAIClient struct {
	responses []string
	callCount int
}

func (m *mockAIClient) SetAPIKey(apiKey, customURL, customModel string) {}
func (m *mockAIClient) SetTimeout(timeout time.Duration)                {}

func (m *mockAIClient) CallWithMessages(systemPrompt, userPrompt string) (string, error) {
	if m.callCount >= len(m.responses) {
		return "", errors.New("no more responses")
	}
	r := m.responses[m.callCount]
	m.callCount++
	return r, nil
}

func (m *mockAIClient) CallWithRequest(req *mcp.Request) (string, error) {
	return "", errors.New("CallWithRequest not implemented in mock")
}

var _ mcp.AIClient = (*mockAIClient)(nil)

func TestRestrictDeepDiveSymbolsToContext_PreFilled(t *testing.T) {
	ctx := &Context{
		MarketDataMap: map[string]*market.Data{
			"BTCUSDT": {Symbol: "BTCUSDT", CurrentPrice: 70000},
			"ETHUSDT": {Symbol: "ETHUSDT", CurrentPrice: 3500},
		},
		Positions: []PositionInfo{},
	}
	macroOut := &MacroOutput{
		SymbolsForDeepDive: []string{"SOLUSDT", "BTCUSDT"},
	}
	restrictDeepDiveSymbolsToContext(ctx, macroOut, 5, nil)
	// SOLUSDT not in context → dropped; BTCUSDT in context → kept; then fills from context up to maxDeepDives
	if len(macroOut.SymbolsForDeepDive) < 1 {
		t.Fatalf("SymbolsForDeepDive should not be empty")
	}
	hasBTC := false
	hasSOL := false
	for _, s := range macroOut.SymbolsForDeepDive {
		n := market.Normalize(s)
		if n == "BTCUSDT" {
			hasBTC = true
		}
		if n == "SOLUSDT" {
			hasSOL = true
		}
	}
	if !hasBTC {
		t.Error("SymbolsForDeepDive must include BTCUSDT (was in macro and context)")
	}
	if hasSOL {
		t.Error("SymbolsForDeepDive must not include SOLUSDT (not in context)")
	}
}

func TestRestrictDeepDiveSymbolsToContext_EmptyContext(t *testing.T) {
	ctx := &Context{
		MarketDataMap: map[string]*market.Data{},
		Positions:     []PositionInfo{},
	}
	macroOut := &MacroOutput{
		SymbolsForDeepDive: []string{"BTCUSDT", "ETHUSDT"},
	}
	restrictDeepDiveSymbolsToContext(ctx, macroOut, 5, nil)
	// Empty context: function returns early, macroOut unchanged
	if len(macroOut.SymbolsForDeepDive) != 2 {
		t.Errorf("empty context should leave SymbolsForDeepDive unchanged, got len %d", len(macroOut.SymbolsForDeepDive))
	}
}

func TestRestrictDeepDiveSymbolsToContext_WithPositions(t *testing.T) {
	ctx := &Context{
		MarketDataMap: map[string]*market.Data{
			"BTCUSDT": {Symbol: "BTCUSDT", CurrentPrice: 70000},
			"ETHUSDT": {Symbol: "ETHUSDT", CurrentPrice: 3500},
		},
		Positions: []PositionInfo{{Symbol: "ETHUSDT"}},
	}
	macroOut := &MacroOutput{
		SymbolsForDeepDive: []string{"SOLUSDT"},
	}
	restrictDeepDiveSymbolsToContext(ctx, macroOut, 5, nil)
	// SOLUSDT not in context → dropped; ETHUSDT is position in context → added; then may fill from context
	if len(macroOut.SymbolsForDeepDive) < 1 {
		t.Fatalf("SymbolsForDeepDive should not be empty")
	}
	hasETH := false
	hasSOL := false
	for _, s := range macroOut.SymbolsForDeepDive {
		n := market.Normalize(s)
		if n == "ETHUSDT" {
			hasETH = true
		}
		if n == "SOLUSDT" {
			hasSOL = true
		}
	}
	if !hasETH {
		t.Error("SymbolsForDeepDive must include ETHUSDT (position in context)")
	}
	if hasSOL {
		t.Error("SymbolsForDeepDive must not include SOLUSDT (not in context)")
	}
}

func TestRestrictDeepDiveSymbolsToContext_FallbackRespectsMaxDeepDives(t *testing.T) {
	ctx := &Context{
		MarketDataMap: map[string]*market.Data{
			"BTCUSDT":  {Symbol: "BTCUSDT", CurrentPrice: 70000},
			"ETHUSDT":  {Symbol: "ETHUSDT", CurrentPrice: 3500},
			"SOLUSDT":  {Symbol: "SOLUSDT", CurrentPrice: 150},
			"XRPUSDT":  {Symbol: "XRPUSDT", CurrentPrice: 2},
			"DOGEUSDT": {Symbol: "DOGEUSDT", CurrentPrice: 0.3},
		},
		Positions: []PositionInfo{},
	}
	macroOut := &MacroOutput{
		SymbolsForDeepDive: []string{},
	}
	restrictDeepDiveSymbolsToContext(ctx, macroOut, 3, nil)
	if len(macroOut.SymbolsForDeepDive) != 3 {
		t.Errorf("fallback with maxDeepDives=3 and 5 context symbols should return 3, got %d", len(macroOut.SymbolsForDeepDive))
	}
}

func TestBuildMacroMicroCombinedUserPrompt_ContainsMacroAndPerSymbol(t *testing.T) {
	config := store.GetDefaultStrategyConfig("en")
	config.EnableMacroMicroFlow = true
	engine := NewStrategyEngine(&config)
	ctx := &Context{
		MarketDataMap: map[string]*market.Data{
			"BTCUSDT": {
				Symbol:       "BTCUSDT",
				CurrentPrice: 70000,
			},
		},
	}
	macroOut := &MacroOutput{
		Trend:              "bullish",
		RiskLevel:          "medium",
		FocusReason:        "test focus",
		SymbolsForDeepDive: []string{"BTCUSDT"},
	}
	prompt := engine.BuildMacroMicroCombinedUserPrompt(ctx, "## Macro brief\nTest brief", macroOut)
	if !strings.Contains(prompt, "## Macro brief") {
		t.Error("prompt should contain macro brief")
	}
	if !strings.Contains(prompt, "Trend: bullish") {
		t.Error("prompt should contain macro trend")
	}
	if !strings.Contains(prompt, "=== BTCUSDT ===") {
		t.Error("prompt should contain per-symbol data block for BTCUSDT")
	}
	if !strings.Contains(prompt, "Output your trading decisions") {
		t.Error("prompt should contain output instructions")
	}
}

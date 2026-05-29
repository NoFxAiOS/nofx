package agent

import (
	"encoding/json"
	"testing"

	"nofx/mcp"
)

// plannerToolsForText now always returns the FULL toolset (no per-domain
// trimming) so the LLM can cross-domain reason. The old "if market intent,
// hide manage_trader" filter was making cross-domain questions like "BTC
// dropped, how much am I losing?" impossible to answer because the agent
// couldn't see both market AND position tools in the same turn.
//
// We still trim the giant strategy schema for non-mutation intents because
// that one is genuinely huge and uninformative for read-only use.

func TestPlannerToolsExposeFullSetForMarketIntent(t *testing.T) {
	tools := plannerToolsForText("看一下 BTCUSDT 行情和 K线")
	names := toolNamesForTest(tools)

	// Market tools must be present.
	for _, expected := range []string{"get_market_snapshot", "get_market_price", "get_kline"} {
		if !containsString(names, expected) {
			t.Fatalf("expected market tool %q in %v", expected, names)
		}
	}
	// Cross-domain tools (positions, balance, trader management) must ALSO be
	// present so the agent can answer "how much am I losing" follow-ups
	// without losing the market context.
	for _, expected := range []string{"get_positions", "get_balance", "manage_trader"} {
		if !containsString(names, expected) {
			t.Fatalf("expected cross-domain tool %q in market context %v", expected, names)
		}
	}
}

func TestPlannerToolsExposeFullSetForExchangeIntent(t *testing.T) {
	tools := plannerToolsForText("帮我添加 okx 交易所 API key")
	names := toolNamesForTest(tools)

	// At least the exchange management tools must show up.
	for _, expected := range []string{"get_exchange_configs", "manage_exchange_config"} {
		if !containsString(names, expected) {
			t.Fatalf("expected exchange tool %q in %v", expected, names)
		}
	}
	// And the agent still has the broader surface available — adding an
	// exchange often leads to "now create a trader" so trader/strategy tools
	// must be reachable in the same turn.
	for _, expected := range []string{"manage_trader", "get_strategies"} {
		if !containsString(names, expected) {
			t.Fatalf("expected adjacent tool %q in exchange context %v", expected, names)
		}
	}
}

func TestPlannerToolsUseCompactManageStrategyForReadIntent(t *testing.T) {
	tools := plannerToolsForText("列出我的策略")
	tool := findToolForTest(tools, "manage_strategy")
	if tool == nil {
		t.Fatalf("expected manage_strategy in strategy tools")
	}

	raw, _ := json.Marshal(tool.Function.Parameters)
	if len(raw) > 900 {
		t.Fatalf("expected compact strategy schema, got %d bytes", len(raw))
	}
	if string(raw) == "" || !json.Valid(raw) {
		t.Fatalf("expected valid strategy schema JSON")
	}
}

func TestPlannerToolsKeepFullManageStrategyForMutationIntent(t *testing.T) {
	tools := plannerToolsForText("创建一个 BTC 网格策略")
	tool := findToolForTest(tools, "manage_strategy")
	if tool == nil {
		t.Fatalf("expected manage_strategy in strategy tools")
	}

	raw, _ := json.Marshal(tool.Function.Parameters)
	if len(raw) < 1500 {
		t.Fatalf("expected full strategy schema for mutation intent, got %d bytes", len(raw))
	}
}

func toolNamesForTest(tools []mcp.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Function.Name)
	}
	return names
}

func findToolForTest(tools []mcp.Tool, name string) *mcp.Tool {
	for i := range tools {
		if tools[i].Function.Name == name {
			return &tools[i]
		}
	}
	return nil
}

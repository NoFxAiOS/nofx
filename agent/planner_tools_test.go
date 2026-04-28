package agent

import (
	"encoding/json"
	"testing"

	"nofx/mcp"
)

func TestPlannerToolsForMarketIntentAreTrimmed(t *testing.T) {
	tools := plannerToolsForText("看一下 BTCUSDT 行情和 K线")
	names := toolNamesForTest(tools)

	for _, expected := range []string{"get_market_snapshot", "get_market_price", "get_kline"} {
		if !containsString(names, expected) {
			t.Fatalf("expected market tool %q in %v", expected, names)
		}
	}
	for _, unexpected := range []string{"manage_strategy", "manage_trader", "manage_exchange_config", "manage_model_config"} {
		if containsString(names, unexpected) {
			t.Fatalf("did not expect management tool %q in market tools %v", unexpected, names)
		}
	}
}

func TestPlannerToolsForExchangeIntentAreTrimmed(t *testing.T) {
	tools := plannerToolsForText("帮我添加 okx 交易所 API key")
	names := toolNamesForTest(tools)

	if len(names) != 2 {
		t.Fatalf("expected two exchange tools, got %v", names)
	}
	for _, expected := range []string{"get_exchange_configs", "manage_exchange_config"} {
		if !containsString(names, expected) {
			t.Fatalf("expected exchange tool %q in %v", expected, names)
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

package api

import "testing"

func TestGetTraderConfigResponse_IncludesStrategyName(t *testing.T) {
	result := map[string]interface{}{
		"trader_id":     "trader-1",
		"trader_name":   "Demo Trader",
		"strategy_id":   "strategy-1",
		"strategy_name": "Trend Following",
	}

	if _, exists := result["strategy_name"]; !exists {
		t.Fatal("expected strategy_name in trader config response")
	}
	if result["strategy_name"] != "Trend Following" {
		t.Fatalf("expected strategy_name %q, got %v", "Trend Following", result["strategy_name"])
	}
	if result["strategy_id"] != "strategy-1" {
		t.Fatalf("expected strategy_id %q, got %v", "strategy-1", result["strategy_id"])
	}
}

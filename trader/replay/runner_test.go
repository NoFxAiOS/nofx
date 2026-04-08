package replay

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunScenarioSmoke(t *testing.T) {
	scenarioPath := filepath.Join("..", "..", "fixtures", "replay", "scenario-btc-long-protection-smoke.json")
	scenario, err := LoadScenario(scenarioPath)
	if err != nil {
		t.Fatalf("expected scenario load success, got %v", err)
	}

	result, err := RunScenario(scenario)
	if err != nil {
		t.Fatalf("expected scenario run success, got %v", err)
	}
	if err := ValidateResult(scenario, result); err != nil {
		t.Fatalf("expected scenario validation success, got %v", err)
	}
}

func TestRunScenarioBlockedByFundingFilter(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "scenario.json")
	content := `{
  "name": "blocked-by-funding",
  "symbol": "BTCUSDT",
  "initial_price": 100,
  "prices": [100, 101, 102],
  "funding_rates": [0.02],
  "actions": [{"type": "open_long", "quantity": 1, "leverage": 5}],
  "regime_filter": {
    "enabled": true,
    "allowed_regimes": ["standard", "trending"],
    "block_high_funding": true,
    "max_funding_rate_abs": 0.01,
    "require_trend_alignment": false
  },
  "expected": {
    "protection_orders": 0,
    "final_position_count": 0,
    "blocked": true
  }
}`
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp scenario: %v", err)
	}
	scenario, err := LoadScenario(tmp)
	if err != nil {
		t.Fatalf("expected scenario load success, got %v", err)
	}
	result, err := RunScenario(scenario)
	if err != nil {
		t.Fatalf("expected scenario run success, got %v", err)
	}
	if err := ValidateResult(scenario, result); err != nil {
		t.Fatalf("expected blocked scenario validation success, got %v", err)
	}
}

func TestRunScenarioOpenCloseLifecycle(t *testing.T) {
	scenarioPath := filepath.Join("..", "..", "fixtures", "replay", "scenario-btc-long-open-close-smoke.json")
	scenario, err := LoadScenario(scenarioPath)
	if err != nil {
		t.Fatalf("expected scenario load success, got %v", err)
	}

	result, err := RunScenario(scenario)
	if err != nil {
		t.Fatalf("expected scenario run success, got %v", err)
	}
	if err := ValidateResult(scenario, result); err != nil {
		t.Fatalf("expected open-close scenario validation success, got %v", err)
	}
}

func TestRunScenarioCloseNotBlockedByRegimeFilter(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "scenario-close-not-blocked.json")
	content := `{
  "name": "close-not-blocked-by-regime-filter",
  "symbol": "BTCUSDT",
  "initial_price": 100,
  "prices": [100, 90],
  "actions": [
    {"type": "open_long", "quantity": 1, "leverage": 5, "price": 100},
    {"type": "close_long", "quantity": 0, "price": 90}
  ],
  "regime_filter": {
    "enabled": true,
    "allowed_regimes": ["standard", "trending"],
    "block_high_funding": false,
    "require_trend_alignment": true
  },
  "expected": {
    "protection_orders": 0,
    "final_position_count": 0,
    "closed_pnl_count": 1,
    "realized_pnl": -10,
    "blocked": false
  }
}`
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp scenario: %v", err)
	}
	scenario, err := LoadScenario(tmp)
	if err != nil {
		t.Fatalf("expected scenario load success, got %v", err)
	}
	result, err := RunScenario(scenario)
	if err != nil {
		t.Fatalf("expected scenario run success, got %v", err)
	}
	if err := ValidateResult(scenario, result); err != nil {
		t.Fatalf("expected close-not-blocked scenario validation success, got %v", err)
	}
}

// --- New deep scenarios ---

func TestRunScenarioEthShortOpenClose(t *testing.T) {
	scenarioPath := filepath.Join("..", "..", "fixtures", "replay", "scenario-eth-short-open-close.json")
	scenario, err := LoadScenario(scenarioPath)
	if err != nil {
		t.Fatalf("expected scenario load success, got %v", err)
	}
	result, err := RunScenario(scenario)
	if err != nil {
		t.Fatalf("expected scenario run success, got %v", err)
	}
	if err := ValidateResult(scenario, result); err != nil {
		t.Fatalf("expected eth short open-close validation success, got %v", err)
	}
}

func TestRunScenarioMultiStepProgression(t *testing.T) {
	scenarioPath := filepath.Join("..", "..", "fixtures", "replay", "scenario-multi-step-progression.json")
	scenario, err := LoadScenario(scenarioPath)
	if err != nil {
		t.Fatalf("expected scenario load success, got %v", err)
	}
	result, err := RunScenario(scenario)
	if err != nil {
		t.Fatalf("expected scenario run success, got %v", err)
	}
	if err := ValidateResult(scenario, result); err != nil {
		t.Fatalf("expected multi-step progression validation success, got %v", err)
	}
}

func TestRunScenarioNegativePnLLong(t *testing.T) {
	scenarioPath := filepath.Join("..", "..", "fixtures", "replay", "scenario-negative-pnl-long.json")
	scenario, err := LoadScenario(scenarioPath)
	if err != nil {
		t.Fatalf("expected scenario load success, got %v", err)
	}
	result, err := RunScenario(scenario)
	if err != nil {
		t.Fatalf("expected scenario run success, got %v", err)
	}
	if err := ValidateResult(scenario, result); err != nil {
		t.Fatalf("expected negative pnl validation success, got %v", err)
	}
}

func TestRunScenarioOpenWithProtection(t *testing.T) {
	scenarioPath := filepath.Join("..", "..", "fixtures", "replay", "scenario-open-with-protection.json")
	scenario, err := LoadScenario(scenarioPath)
	if err != nil {
		t.Fatalf("expected scenario load success, got %v", err)
	}
	result, err := RunScenario(scenario)
	if err != nil {
		t.Fatalf("expected scenario run success, got %v", err)
	}
	if err := ValidateResult(scenario, result); err != nil {
		t.Fatalf("expected open-with-protection validation success, got %v", err)
	}
}

func TestRunScenarioShortWithProtection(t *testing.T) {
	scenarioPath := filepath.Join("..", "..", "fixtures", "replay", "scenario-short-with-protection.json")
	scenario, err := LoadScenario(scenarioPath)
	if err != nil {
		t.Fatalf("expected scenario load success, got %v", err)
	}
	result, err := RunScenario(scenario)
	if err != nil {
		t.Fatalf("expected scenario run success, got %v", err)
	}
	if err := ValidateResult(scenario, result); err != nil {
		t.Fatalf("expected short-with-protection validation success, got %v", err)
	}
}

func TestRunScenarioRegimeTrendBlock(t *testing.T) {
	scenarioPath := filepath.Join("..", "..", "fixtures", "replay", "scenario-regime-trend-block.json")
	scenario, err := LoadScenario(scenarioPath)
	if err != nil {
		t.Fatalf("expected scenario load success, got %v", err)
	}
	result, err := RunScenario(scenario)
	if err != nil {
		t.Fatalf("expected scenario run success, got %v", err)
	}
	if err := ValidateResult(scenario, result); err != nil {
		t.Fatalf("expected regime trend block validation success, got %v", err)
	}
}

// --- Edge case / error path scenarios ---

func TestRunScenarioNilScenario(t *testing.T) {
	_, err := RunScenario(nil)
	if err == nil {
		t.Fatal("expected error for nil scenario")
	}
}

func TestRunScenarioEmptySymbol(t *testing.T) {
	s := &Scenario{Name: "empty-symbol", Actions: []ScenarioAction{{Type: "open_long", Quantity: 1, Leverage: 5}}}
	_, err := RunScenario(s)
	if err == nil {
		t.Fatal("expected error for empty symbol")
	}
}

func TestRunScenarioInvalidAction(t *testing.T) {
	s := &Scenario{
		Name:         "invalid-action",
		Symbol:       "BTCUSDT",
		InitialPrice: 100,
		Prices:       []float64{100},
		Actions:      []ScenarioAction{{Type: "invalid_action", Quantity: 1, Leverage: 5}},
		Expected:     ScenarioExpected{},
	}
	_, err := RunScenario(s)
	if err == nil {
		t.Fatal("expected error for invalid action type")
	}
}

func TestRunScenarioMissingPrice(t *testing.T) {
	s := &Scenario{
		Name:    "missing-price",
		Symbol:  "NOPRICE",
		Prices:  []float64{},
		Actions: []ScenarioAction{{Type: "open_long", Quantity: 1, Leverage: 5}},
	}
	_, err := RunScenario(s)
	if err == nil {
		t.Fatal("expected error for missing price")
	}
}

func TestRunScenarioCloseWithoutOpen(t *testing.T) {
	s := &Scenario{
		Name:         "close-without-open",
		Symbol:       "BTCUSDT",
		InitialPrice: 100,
		Prices:       []float64{100},
		Actions:      []ScenarioAction{{Type: "close_long", Quantity: 0, Price: 100}},
	}
	_, err := RunScenario(s)
	if err == nil {
		t.Fatal("expected error for close without open position")
	}
}

func TestLoadScenarioInvalidPath(t *testing.T) {
	_, err := LoadScenario("/nonexistent/path/scenario.json")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestLoadScenarioInvalidJSON(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(tmp, []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	_, err := LoadScenario(tmp)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestValidateResultNil(t *testing.T) {
	err := ValidateResult(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil scenario/result")
	}
}

func TestValidateResultMismatchProtectionOrders(t *testing.T) {
	s := &Scenario{Expected: ScenarioExpected{ProtectionOrders: 2}}
	r := &Result{ProtectionOrders: 0}
	err := ValidateResult(s, r)
	if err == nil {
		t.Fatal("expected validation error for protection order mismatch")
	}
}

func TestValidateResultMismatchPnL(t *testing.T) {
	s := &Scenario{Expected: ScenarioExpected{RealizedPnL: 100}}
	r := &Result{RealizedPnL: 50}
	err := ValidateResult(s, r)
	if err == nil {
		t.Fatal("expected validation error for realized pnl mismatch")
	}
}

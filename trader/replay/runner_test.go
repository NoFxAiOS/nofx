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

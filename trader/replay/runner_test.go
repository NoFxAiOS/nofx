package replay

import (
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

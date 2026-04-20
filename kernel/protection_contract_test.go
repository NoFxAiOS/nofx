package kernel

import (
	"encoding/json"
	"os"
	"testing"

	"nofx/store"
)

func TestProtectionFixturePromptMentionsDrawdownAndBreakEvenContracts(t *testing.T) {
	blob, err := os.ReadFile("../docs/fixtures/protection-test-run-fixture.json")
	if err != nil {
		t.Fatalf("read fixture failed: %v", err)
	}
	var payload struct {
		Config store.StrategyConfig `json:"config"`
	}
	if err := json.Unmarshal(blob, &payload); err != nil {
		t.Fatalf("unmarshal fixture failed: %v", err)
	}
	payload.Config.Protection.DrawdownTakeProfit.Enabled = true
	payload.Config.Protection.DrawdownTakeProfit.Rules = []store.DrawdownTakeProfitRule{{
		MinProfitPct: 5, MaxDrawdownPct: 30, CloseRatioPct: 100, PollIntervalSeconds: 60,
	}}
	payload.Config.Protection.BreakEvenStop.Enabled = true

	engine := NewStrategyEngine(&payload.Config)
	systemPrompt := engine.BuildSystemPrompt(1000, "balanced")
	mustContain := []string{
		"Drawdown Take Profit",
		"drawdown, trailing, or profit-protection ownership",
		"Break-even Stop",
		"mode=break_even",
		"break_even_trigger_mode/value/offset",
		"break-even or acknowledge that an additional stop layer exists",
	}
	for _, needle := range mustContain {
		if !contains(systemPrompt, needle) {
			t.Fatalf("system prompt should contain %q when drawdown/break-even enabled", needle)
		}
	}
}

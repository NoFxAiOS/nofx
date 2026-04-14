package kernel

import (
	"encoding/json"
	"os"
	"testing"

	"nofx/store"
)

func TestProtectionFixtureBuildsPromptWithProtectionPlanGuidance(t *testing.T) {
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

	engine := NewStrategyEngine(&payload.Config)
	systemPrompt := engine.BuildSystemPrompt(1000, "balanced")
	if systemPrompt == "" {
		t.Fatal("system prompt is empty")
	}

	mustContain := []string{
		"protection_plan",
		"mode=full",
		"mode=ladder",
		"Do NOT output protection_plan for hold/wait/close actions",
		"open_long",
		"close_long",
	}
	for _, needle := range mustContain {
		if !contains(systemPrompt, needle) {
			t.Fatalf("system prompt should contain %q", needle)
		}
	}
}
